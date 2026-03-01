package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/merthan/otacon/internal/engine"
	"github.com/merthan/otacon/internal/engine/audit"
	"github.com/spf13/cobra"
)

func newAuditCommand() *cobra.Command {
	var (
		severity   string
		explain    bool
		outputFile string
	)

	cmd := &cobra.Command{
		Use:   "audit",
		Short: "Run best practice compliance audit",
		Long: `Checks your cluster against 50+ best practice rules and reports
compliance violations grouped by severity. Unlike 'scan' which gives an
overview scorecard, 'audit' provides detailed per-resource findings.

Examples:
  otacon audit                          Full audit
  otacon audit -n production            Audit specific namespace
  otacon audit --severity critical      Show only critical findings
  otacon audit --explain                Include remediation details
  otacon audit --export report.json     Export findings`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAudit(severity, explain, outputFile)
		},
	}

	cmd.Flags().StringVar(&severity, "severity", "", "Filter by severity: critical, warning, info")
	cmd.Flags().BoolVar(&explain, "explain", false, "Include detailed explanations and remediation")
	cmd.Flags().StringVar(&outputFile, "export", "", "Export findings to file (.json)")

	return cmd
}

func runAudit(severityFilter string, explain bool, outputFile string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	cyan := color.New(color.FgCyan)
	white := color.New(color.FgWhite)
	yellow := color.New(color.FgYellow)
	red := color.New(color.FgRed)
	green := color.New(color.FgGreen)
	blue := color.New(color.FgBlue)
	dim := color.New(color.FgHiBlack)

	cyan.Printf("\n 🔍 Otacon Audit — Best Practice Compliance Check\n\n")

	kubeCfg := engine.KubeConfig{
		Kubeconfig: globalFlags.Kubeconfig,
		Context:    globalFlags.Context,
	}

	client, _, err := engine.NewKubeClient(kubeCfg)
	if err != nil {
		return fmt.Errorf("failed to connect to cluster: %w", err)
	}

	scanner := audit.NewScanner(client)
	scorecard, err := scanner.Scan(ctx, audit.ScanOptions{
		Namespace: globalFlags.Namespace,
		Verbose:   globalFlags.Verbose,
		Explain:   explain,
		Workers:   10,
	})
	if err != nil {
		return fmt.Errorf("audit failed: %w", err)
	}

	// Group findings by category
	for _, cat := range scorecard.Categories {
		findings := cat.Findings

		// Filter by severity if specified
		if severityFilter != "" {
			var filtered []engine.Finding
			for _, f := range findings {
				if strings.EqualFold(f.Severity.String(), severityFilter) {
					filtered = append(filtered, f)
				}
			}
			findings = filtered
		}

		if len(findings) == 0 {
			continue
		}

		// Category header
		pct := cat.Percentage()
		white.Printf(" ━━━ %s ", cat.Name)
		dim.Printf("(%d findings, %.0f%% score)\n", len(findings), pct)

		for _, f := range findings {
			var c *color.Color
			var icon string
			switch f.Severity {
			case engine.SeverityCritical:
				c = red
				icon = "CRIT"
			case engine.SeverityWarning:
				c = yellow
				icon = "WARN"
			case engine.SeverityInfo:
				c = blue
				icon = "INFO"
			}

			c.Printf("   [%s] ", icon)
			white.Printf("%s\n", f.Message)
			dim.Printf("         Resource: %s\n", f.Resource)

			if explain {
				if f.Explain != "" {
					dim.Printf("         Why: %s\n", f.Explain)
				}
				if f.Remediation != "" {
					green.Printf("         Fix: %s\n", f.Remediation)
				}
			}
			fmt.Println()
		}
	}

	// Summary
	fmt.Println()
	dim.Printf(" ━━━ Summary\n")
	red.Printf("   Critical: %d  ", scorecard.TotalCritical)
	yellow.Printf("Warning: %d  ", scorecard.TotalWarning)
	blue.Printf("Info: %d  ", scorecard.TotalInfo)
	white.Printf("Total: %d\n", scorecard.TotalFindings)
	dim.Printf("   Overall Grade: %s (%.0f/100)\n\n", scorecard.Grade, scorecard.OverallScore)

	if outputFile != "" {
		data, _ := json.MarshalIndent(scorecard, "", "  ")
		os.WriteFile(outputFile, data, 0644)
		green.Printf(" 📄 Audit report exported to: %s\n", outputFile)
	}

	return nil
}
