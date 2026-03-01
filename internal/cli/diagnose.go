package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/fatih/color"
	"github.com/merthan/otacon/internal/engine"
	"github.com/merthan/otacon/internal/engine/diagnostics"
	"github.com/spf13/cobra"
)

func newDiagnoseCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "diagnose",
		Short: "Run deep diagnostics on cluster components",
		Long: `Performs targeted diagnostic checks on specific cluster aspects.
Use subcommands to focus on network, logs, or node issues.

Examples:
  otacon diagnose network              Network & DNS diagnostics
  otacon diagnose logs                 Log pattern analysis
  otacon diagnose nodes                Node pressure detection
  otacon diagnose all                  Run all diagnostics`,
	}

	cmd.AddCommand(newDiagnoseNetworkCommand())
	cmd.AddCommand(newDiagnoseLogsCommand())
	cmd.AddCommand(newDiagnoseNodesCommand())
	cmd.AddCommand(newDiagnoseAllCommand())

	return cmd
}

func newDiagnoseNetworkCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "network",
		Short: "Diagnose network issues (DNS, connectivity, NetworkPolicy)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDiagnose("network")
		},
	}
}

func newDiagnoseLogsCommand() *cobra.Command {
	var status string

	cmd := &cobra.Command{
		Use:   "logs",
		Short: "Analyze pod logs for error patterns",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDiagnose("logs")
		},
	}

	cmd.Flags().StringVar(&status, "status", "", "Filter by pod status (CrashLoopBackOff, Error, OOMKilled)")
	return cmd
}

func newDiagnoseNodesCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "nodes",
		Short: "Detect node pressure conditions (disk, memory, PID)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDiagnose("nodes")
		},
	}
}

func newDiagnoseAllCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "all",
		Short: "Run all diagnostic checks",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDiagnose("all")
		},
	}
}

func runDiagnose(target string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	cyan := color.New(color.FgCyan)
	white := color.New(color.FgWhite)
	yellow := color.New(color.FgYellow)
	dim := color.New(color.FgHiBlack)

	cyan.Printf("\n 🔬 Otacon Diagnostics — Deep Analysis\n\n")

	kubeCfg := engine.KubeConfig{
		Kubeconfig: globalFlags.Kubeconfig,
		Context:    globalFlags.Context,
	}

	client, _, err := engine.NewKubeClient(kubeCfg)
	if err != nil {
		return fmt.Errorf("failed to connect to cluster: %w", err)
	}

	diag := diagnostics.NewDiagnosticEngine(client)
	namespace := globalFlags.Namespace

	var results []engine.DiagnosticResult

	switch target {
	case "network":
		yellow.Printf(" ⏳ Running network diagnostics...\n\n")
		results = diag.RunNetworkDiagnostics(ctx, namespace)
	case "logs":
		yellow.Printf(" ⏳ Analyzing pod logs for error patterns...\n\n")
		results = diag.RunLogDiagnostics(ctx, namespace)
	case "nodes":
		yellow.Printf(" ⏳ Checking node health and pressure conditions...\n\n")
		results = diag.RunNodeDiagnostics(ctx)
	case "all":
		yellow.Printf(" ⏳ Running all diagnostics...\n\n")

		white.Printf(" ━━━ Network Diagnostics\n")
		netResults := diag.RunNetworkDiagnostics(ctx, namespace)
		printDiagResults(netResults)

		white.Printf("\n ━━━ Log Analysis\n")
		logResults := diag.RunLogDiagnostics(ctx, namespace)
		printDiagResults(logResults)

		white.Printf("\n ━━━ Node Health\n")
		nodeResults := diag.RunNodeDiagnostics(ctx)
		printDiagResults(nodeResults)

		total := len(netResults) + len(logResults) + len(nodeResults)
		failed := countFailed(netResults) + countFailed(logResults) + countFailed(nodeResults)

		fmt.Println()
		dim.Printf(" Diagnostics complete: %d checks, %d issues found\n\n", total, failed)
		return nil
	}

	printDiagResults(results)

	failed := countFailed(results)
	fmt.Println()
	dim.Printf(" Diagnostics complete: %d checks, %d issues found\n\n", len(results), failed)

	return nil
}

func printDiagResults(results []engine.DiagnosticResult) {
	green := color.New(color.FgGreen)
	red := color.New(color.FgRed)
	yellow := color.New(color.FgYellow)
	white := color.New(color.FgWhite)
	dim := color.New(color.FgHiBlack)

	for _, r := range results {
		var icon string
		var c *color.Color

		switch r.Status {
		case "pass":
			icon = "✅"
			c = green
		case "fail":
			icon = "❌"
			c = red
		case "warn":
			icon = "⚠️"
			c = yellow
		default:
			icon = "ℹ️"
			c = white
		}

		c.Printf("   %s %s\n", icon, r.Check)
		white.Printf("      %s\n", r.Message)

		if len(r.Details) > 0 {
			for _, d := range r.Details {
				dim.Printf("      • %s\n", d)
			}
		}

		if r.Remediation != "" && r.Status != "pass" {
			green.Printf("      Fix: %s\n", r.Remediation)
		}
		fmt.Println()
	}
}

func countFailed(results []engine.DiagnosticResult) int {
	count := 0
	for _, r := range results {
		if r.Status == "fail" || r.Status == "warn" {
			count++
		}
	}
	return count
}
