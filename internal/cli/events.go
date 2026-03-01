package cli

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/merthan/otacon/internal/engine"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"github.com/spf13/cobra"
)

func newEventsCommand() *cobra.Command {
	var (
		since     string
		correlate bool
		severity  string
	)

	cmd := &cobra.Command{
		Use:   "events",
		Short: "View cluster events with optional correlation",
		Long: `Displays Kubernetes events as a timeline. With --correlate, groups
related events into incidents showing root cause analysis.

Examples:
  otacon events                        All recent events
  otacon events --since 1h             Last hour
  otacon events --correlate            Group related events
  otacon events --severity critical    Critical events only`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runEvents(since, correlate, severity)
		},
	}

	cmd.Flags().StringVar(&since, "since", "1h", "Show events since duration (e.g., 30m, 1h, 24h)")
	cmd.Flags().BoolVar(&correlate, "correlate", false, "Enable event correlation")
	cmd.Flags().StringVar(&severity, "severity", "", "Filter by severity: critical, warning, info")

	return cmd
}

func runEvents(since string, correlate bool, severityFilter string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cyan := color.New(color.FgCyan)
	green := color.New(color.FgGreen)
	dim := color.New(color.FgHiBlack)

	cyan.Printf("\n 📋 Otacon Event Timeline\n\n")

	kubeCfg := engine.KubeConfig{
		Kubeconfig: globalFlags.Kubeconfig,
		Context:    globalFlags.Context,
	}

	client, _, err := engine.NewKubeClient(kubeCfg)
	if err != nil {
		return fmt.Errorf("failed to connect to cluster: %w", err)
	}

	namespace := globalFlags.Namespace

	// Parse since duration
	duration, err := time.ParseDuration(since)
	if err != nil {
		duration = 1 * time.Hour
	}
	sinceTime := time.Now().Add(-duration)

	events, err := client.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list events: %w", err)
	}

	// Filter and classify events
	var processed []engine.Event
	for _, event := range events.Items {
		eventTime := event.LastTimestamp.Time
		if eventTime.IsZero() {
			eventTime = event.EventTime.Time
		}
		if eventTime.Before(sinceTime) {
			continue
		}

		severity := classifySeverity(event.Reason, event.Type)

		if severityFilter != "" {
			if !strings.EqualFold(severity.String(), severityFilter) {
				continue
			}
		}

		processed = append(processed, engine.Event{
			Type:         event.Type,
			Reason:       event.Reason,
			Message:      event.Message,
			Namespace:    event.Namespace,
			ResourceKind: event.InvolvedObject.Kind,
			ResourceName: event.InvolvedObject.Name,
			NodeName:     event.Source.Host,
			Severity:     severity,
			Count:        event.Count,
			FirstSeen:    event.FirstTimestamp.Time,
			LastSeen:     eventTime,
		})
	}

	// Sort by time
	sort.Slice(processed, func(i, j int) bool {
		return processed[i].LastSeen.After(processed[j].LastSeen)
	})

	if len(processed) == 0 {
		green.Printf(" ✅ No events found in the last %s\n\n", since)
		return nil
	}

	if correlate {
		printCorrelatedEvents(processed)
	} else {
		printEventTimeline(processed)
	}

	fmt.Println()
	dim.Printf(" Found %d events in the last %s\n", len(processed), since)
	if !correlate {
		dim.Printf(" 💡 Run 'otacon events --correlate' to group related events\n")
	}
	fmt.Println()

	return nil
}

func classifySeverity(reason, eventType string) engine.Severity {
	criticalReasons := map[string]bool{
		"OOMKilled": true, "CrashLoopBackOff": true, "NodeNotReady": true,
		"EvictionThresholdMet": true, "SystemOOM": true, "ContainerGCFailed": true,
		"FreeDiskSpaceFailed": true, "FailedMount": true, "NetworkNotReady": true,
	}
	warningReasons := map[string]bool{
		"BackOff": true, "Unhealthy": true, "FailedScheduling": true,
		"FailedCreate": true, "FailedDelete": true, "ImagePullBackOff": true,
		"ErrImagePull": true, "NodeHasDiskPressure": true, "NodeHasMemoryPressure": true,
		"Evicted": true, "Preempting": true,
	}

	if criticalReasons[reason] {
		return engine.SeverityCritical
	}
	if warningReasons[reason] || eventType == "Warning" {
		return engine.SeverityWarning
	}
	return engine.SeverityInfo
}

func printEventTimeline(events []engine.Event) {
	red := color.New(color.FgRed)
	yellow := color.New(color.FgYellow)
	blue := color.New(color.FgBlue)
	white := color.New(color.FgWhite)
	dim := color.New(color.FgHiBlack)

	for _, e := range events {
		var c *color.Color
		switch e.Severity {
		case engine.SeverityCritical:
			c = red
		case engine.SeverityWarning:
			c = yellow
		default:
			c = blue
		}

		timeStr := e.LastSeen.Format("15:04:05")
		c.Printf(" %s %s ", e.Severity.Icon(), timeStr)
		white.Printf("[%s] ", e.Reason)
		dim.Printf("%s/%s: ", e.Namespace, e.ResourceName)
		white.Printf("%s", truncateMsg(e.Message, 80))
		if e.Count > 1 {
			dim.Printf(" (x%d)", e.Count)
		}
		fmt.Println()
	}
}

func printCorrelatedEvents(events []engine.Event) {
	white := color.New(color.FgWhite)
	yellow := color.New(color.FgYellow)
	red := color.New(color.FgRed)
	dim := color.New(color.FgHiBlack)

	// Simple correlation: group events by time window + node
	type group struct {
		trigger engine.Event
		related []engine.Event
	}

	var groups []group
	used := make(map[int]bool)

	for i, e := range events {
		if used[i] || e.Severity != engine.SeverityCritical {
			continue
		}
		used[i] = true
		g := group{trigger: e}

		// Find related events within 5 minutes
		for j, other := range events {
			if used[j] || i == j {
				continue
			}
			timeDiff := e.LastSeen.Sub(other.LastSeen)
			if timeDiff < 0 {
				timeDiff = -timeDiff
			}
			if timeDiff <= 5*time.Minute {
				// Same node or same namespace
				if e.NodeName == other.NodeName || e.Namespace == other.Namespace {
					g.related = append(g.related, other)
					used[j] = true
				}
			}
		}
		groups = append(groups, g)
	}

	// Print correlated incidents
	if len(groups) > 0 {
		white.Printf(" ━━━ Correlated Incidents\n\n")
		for i, g := range groups {
			red.Printf(" 🔴 Incident #%d: %s\n", i+1, g.trigger.Reason)
			white.Printf("    Trigger: %s/%s — %s\n", g.trigger.Namespace, g.trigger.ResourceName, truncateMsg(g.trigger.Message, 70))
			dim.Printf("    Time: %s\n", g.trigger.LastSeen.Format("15:04:05"))

			if len(g.related) > 0 {
				yellow.Printf("    Related events (%d):\n", len(g.related))
				for _, r := range g.related {
					dim.Printf("      %s %s [%s] %s/%s: %s\n",
						r.Severity.Icon(), r.LastSeen.Format("15:04:05"),
						r.Reason, r.Namespace, r.ResourceName,
						truncateMsg(r.Message, 60))
				}
			}
			fmt.Println()
		}
	}

	// Print uncorrelated events
	var uncorrelated []engine.Event
	for i, e := range events {
		if !used[i] {
			uncorrelated = append(uncorrelated, e)
		}
	}

	if len(uncorrelated) > 0 {
		white.Printf(" ━━━ Other Events (%d)\n\n", len(uncorrelated))
		printEventTimeline(uncorrelated)
	}
}

func truncateMsg(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > maxLen {
		return s[:maxLen] + "..."
	}
	return s
}
