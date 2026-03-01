package cli

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/fatih/color"
	"github.com/merthan/otacon/internal/api"
	"github.com/merthan/otacon/internal/engine"
	"github.com/merthan/otacon/internal/engine/audit"
	"github.com/merthan/otacon/internal/engine/intelligence"
	"github.com/merthan/otacon/internal/engine/watcher"
	"github.com/merthan/otacon/internal/notification"
	"github.com/merthan/otacon/internal/store"
	"github.com/spf13/cobra"
)

func newGuardianCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "guardian",
		Short: "Run Otacon in guardian (daemon) mode inside the cluster",
		Long: `Starts Otacon as a long-running process that continuously monitors
cluster events, runs periodic audits, and sends notifications.

This mode is designed to run as a Kubernetes Deployment via Helm chart.

Examples:
  otacon guardian start                Start guardian mode
  otacon guardian status               Check guardian status`,
	}

	cmd.AddCommand(newGuardianStartCommand())
	cmd.AddCommand(newGuardianStatusCommand())

	return cmd
}

func newGuardianStartCommand() *cobra.Command {
	var (
		port        int
		metricsPort int
		uiEnabled   bool
	)

	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start guardian mode",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGuardianStart(port, metricsPort, uiEnabled)
		},
	}

	cmd.Flags().IntVar(&port, "port", 8080, "API server port")
	cmd.Flags().IntVar(&metricsPort, "metrics-port", 9090, "Metrics port")
	cmd.Flags().BoolVar(&uiEnabled, "ui", true, "Enable web UI")

	return cmd
}

func newGuardianStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Check guardian status",
		Run: func(cmd *cobra.Command, args []string) {
			dim := color.New(color.FgHiBlack)
			dim.Println(" Guardian mode status check — not yet implemented")
			dim.Println(" This will be available in a future release")
		},
	}
}

func runGuardianStart(port, metricsPort int, uiEnabled bool) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cyan := color.New(color.FgCyan)
	white := color.New(color.FgWhite)

	cyan.Println(banner)
	white.Printf("  Guardian Mode — Continuous Cluster Intelligence\n\n")

	// 1. Connect to cluster
	kubeCfg := engine.KubeConfig{
		Kubeconfig: globalFlags.Kubeconfig,
		Context:    globalFlags.Context,
	}
	client, _, err := engine.NewKubeClient(kubeCfg)
	if err != nil {
		return fmt.Errorf("failed to connect to cluster: %w", err)
	}
	white.Printf(" ✓ Connected to cluster: %s\n", engine.GetClusterName(kubeCfg))

	// 2. Initialize storage
	db, err := store.New(store.DefaultConfig())
	if err != nil {
		return fmt.Errorf("failed to init storage: %w", err)
	}
	defer db.Close()
	go db.StartRetentionWorker(ctx)
	white.Println(" ✓ Storage initialized (SQLite, 7-day retention)")

	// 3. Intelligence engine
	correlator := intelligence.NewCorrelator()
	dedup := intelligence.NewDeduplicator(intelligence.DefaultDeduplicationConfig())
	cooldown := intelligence.NewCooldownManager(intelligence.DefaultCooldownConfig())
	white.Println(" ✓ Intelligence engine ready (correlator, dedup, cooldown)")

	// 4. Notification router
	router := notification.NewRouter(cooldown)
	white.Println(" ✓ Notification router ready")

	// 5. Wire: correlator → store + router
	correlator.OnIncident(func(inc engine.CorrelatedIncident) {
		db.SaveIncident(inc)
		router.RouteIncident(ctx, inc)
	})

	// 6. Wire: dedup → router
	dedup.OnGroup(func(g intelligence.DeduplicatedGroup) {
		// Log deduplicated groups
		fmt.Printf(" [dedup] %s\n", g.Summary())
	})

	// 7. Event watcher → correlator + dedup + store + metrics
	w := watcher.NewWatcher(client, watcher.FilterConfig{})
	w.OnEvent(func(event engine.Event) {
		db.SaveEvent(event)
		correlator.Ingest(event)
		dedup.Ingest(event)

		// Update Prometheus metrics
		api.GlobalMetrics.EventsTotal.Add(1)
		switch event.Severity {
		case engine.SeverityCritical:
			api.GlobalMetrics.EventsCritical.Add(1)
		case engine.SeverityWarning:
			api.GlobalMetrics.EventsWarning.Add(1)
		default:
			api.GlobalMetrics.EventsInfo.Add(1)
		}
	})
	w.Start(ctx)
	white.Println(" ✓ Event watcher started")

	// 8. Periodic audit scan — runs immediately then every 6 hours
	scanner := audit.NewScanner(client)
	go func() {
		runAuditScan(ctx, scanner, db, engine.GetClusterName(kubeCfg))
		ticker := time.NewTicker(6 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				runAuditScan(ctx, scanner, db, engine.GetClusterName(kubeCfg))
			case <-ctx.Done():
				return
			}
		}
	}()
	white.Println(" ✓ Periodic audit scan scheduled (every 6h, first scan running now)")

	// 9. API server
	apiServer := api.NewServer(api.ServerConfig{
		Port:       port,
		Store:      db,
		Correlator: correlator,
		Dedup:      dedup,
		Cooldown:   cooldown,
		Router:     router,
	})
	white.Printf(" ✓ API server on :%d\n", port)
	if uiEnabled {
		white.Printf(" ✓ Web UI enabled at http://localhost:%d\n", port)
	}

	white.Printf("\n 🛡️  Otacon guardian is running. Press Ctrl+C to stop.\n\n")

	// Block on API server
	return apiServer.Start()
}

// runAuditScan performs a full cluster audit and stores the result
func runAuditScan(ctx context.Context, scanner *audit.Scanner, db *store.Store, clusterName string) {
	log.Println("[guardian] Running periodic audit scan...")
	scanCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	scorecard, err := scanner.Scan(scanCtx, audit.ScanOptions{
		Verbose: false,
		Workers: 10,
	})
	if err != nil {
		log.Printf("[guardian] Audit scan failed: %v", err)
		return
	}

	scorecard.ClusterName = clusterName

	// Gather cluster info
	info := scanner.GatherClusterInfo(scanCtx, "")
	scorecard.NodeCount = info.NodeCount
	scorecard.PodCount = info.PodCount
	scorecard.NamespaceCount = info.NamespaceCount

	// Store in SQLite
	if err := db.SaveAuditReport(scorecard); err != nil {
		log.Printf("[guardian] Failed to save audit report: %v", err)
		return
	}

	// Update Prometheus metrics
	api.GlobalMetrics.AuditScore.Store(int64(scorecard.OverallScore * 100))
	api.GlobalMetrics.AuditGrade.Store(scorecard.Grade)

	log.Printf("[guardian] Audit complete: %s (%.0f/100) — %d findings (%d critical, %d warning)",
		scorecard.Grade, scorecard.OverallScore,
		scorecard.TotalFindings, scorecard.TotalCritical, scorecard.TotalWarning)
}
