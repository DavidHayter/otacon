package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/fatih/color"
	"github.com/merthan/otacon/internal/engine"
	"github.com/merthan/otacon/internal/engine/audit"
	"github.com/spf13/cobra"
)

func newScanCommand() *cobra.Command {
	var (
		explain           bool
		categories        []string
		excludeCategories []string
		severityFilter    string
		outputFile        string
		outputFmt         string
		topN              int
		minScore          float64
	)

	cmd := &cobra.Command{
		Use:   "scan",
		Short: "Run a full cluster health scan with scorecard",
		Long: `Performs a comprehensive audit of your Kubernetes cluster, checking
50+ rules across Security, Resource Management, Reliability, Best Practices,
and Network Policies. Produces an A-F graded scorecard with actionable findings.

Categories: Security, Resource Management, Reliability, Best Practices, Network Policies

Examples:
  otacon scan                                         Full cluster scan
  otacon scan -n production                           Scan specific namespace
  otacon scan --explain                               Include detailed explanations
  otacon scan --categories Security,Reliability        Only these categories
  otacon scan --exclude-categories "Network Policies"  Skip network policy checks
  otacon scan --severity critical                      Only show critical findings
  otacon scan --min-score 70                           Exit code 1 if below threshold
  otacon scan --export report.html                     Export as HTML report
  otacon scan --export report.json                     Export as JSON`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runScan(explain, categories, excludeCategories, severityFilter, outputFile, outputFmt, topN, minScore)
		},
	}

	cmd.Flags().BoolVar(&explain, "explain", false, "Include detailed explanations for each finding")
	cmd.Flags().StringSliceVar(&categories, "categories", nil, "Include only these categories (comma-separated)")
	cmd.Flags().StringSliceVar(&excludeCategories, "exclude-categories", nil, "Exclude these categories (comma-separated)")
	cmd.Flags().StringVar(&severityFilter, "severity", "", "Show only findings of this severity: critical, warning, info")
	cmd.Flags().StringVar(&outputFile, "export", "", "Export report to file (.html or .json)")
	cmd.Flags().StringVarP(&outputFmt, "output", "o", "table", "Output format: table, json, yaml")
	cmd.Flags().IntVar(&topN, "top", 10, "Number of top findings to display")
	cmd.Flags().Float64Var(&minScore, "min-score", 0, "Minimum passing score (exit 1 if below, useful for CI)")

	return cmd
}

func runScan(explain bool, categories, excludeCategories []string, severityFilter, outputFile, outputFmt string, topN int, minScore float64) error {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// Print banner
	cyan := color.New(color.FgCyan)
	white := color.New(color.FgWhite)
	yellow := color.New(color.FgYellow)
	red := color.New(color.FgRed)
	green := color.New(color.FgGreen)
	blue := color.New(color.FgBlue)
	dim := color.New(color.FgHiBlack)

	cyan.Println(banner)
	dim.Printf("  Kubernetes Intelligence Platform\n\n")

	// Connect to cluster
	kubeCfg := engine.KubeConfig{
		Kubeconfig: globalFlags.Kubeconfig,
		Context:    globalFlags.Context,
	}

	client, _, err := engine.NewKubeClient(kubeCfg)
	if err != nil {
		return fmt.Errorf("failed to connect to cluster: %w", err)
	}

	clusterName := engine.GetClusterName(kubeCfg)
	namespace := globalFlags.Namespace

	// Show scan target
	if namespace != "" {
		yellow.Printf(" ⏳ Scanning namespace: %s (cluster: %s)...\n\n", namespace, clusterName)
	} else {
		scanner := audit.NewScanner(client)
		info := scanner.GatherClusterInfo(ctx, namespace)
		yellow.Printf(" ⏳ Scanning cluster: %s (%d nodes, %d pods)...\n\n",
			clusterName, info.NodeCount, info.PodCount)
	}

	// Merge include + exclude into effective categories
	effectiveCategories := categories
	if len(excludeCategories) > 0 && len(effectiveCategories) == 0 {
		// If only exclude is specified, start with all and remove excluded
		effectiveCategories = []string{
			"Security", "Resource Management", "Reliability", "Best Practices", "Network Policies",
		}
	}
	if len(excludeCategories) > 0 {
		var filtered []string
		excludeMap := make(map[string]bool)
		for _, ex := range excludeCategories {
			excludeMap[ex] = true
		}
		for _, cat := range effectiveCategories {
			if !excludeMap[cat] {
				filtered = append(filtered, cat)
			}
		}
		effectiveCategories = filtered
	}

	// Run scan
	scanner := audit.NewScanner(client)
	scorecard, err := scanner.Scan(ctx, audit.ScanOptions{
		Namespace:  namespace,
		Categories: effectiveCategories,
		Verbose:    globalFlags.Verbose,
		Explain:    explain,
		Workers:    10,
	})
	if err != nil {
		return fmt.Errorf("scan failed: %w", err)
	}

	// Handle JSON output
	if outputFmt == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(scorecard)
	}

	// Display scorecard
	fmt.Print(audit.FormatScorecardText(scorecard))

	// Display top findings
	topFindings := audit.GetTopFindings(scorecard, topN)

	// Apply severity filter if specified
	if severityFilter != "" {
		var filtered []engine.Finding
		for _, f := range topFindings {
			switch severityFilter {
			case "critical", "CRITICAL":
				if f.Severity == engine.SeverityCritical {
					filtered = append(filtered, f)
				}
			case "warning", "WARNING":
				if f.Severity == engine.SeverityWarning {
					filtered = append(filtered, f)
				}
			case "info", "INFO":
				if f.Severity == engine.SeverityInfo {
					filtered = append(filtered, f)
				}
			}
		}
		topFindings = filtered
	}

	if len(topFindings) > 0 {
		fmt.Printf("\n TOP FINDINGS:\n")
		for _, f := range topFindings {
			var c *color.Color
			var icon string
			switch f.Severity {
			case engine.SeverityCritical:
				c = red
				icon = "🔴 CRIT"
			case engine.SeverityWarning:
				c = yellow
				icon = "🟡 WARN"
			case engine.SeverityInfo:
				c = blue
				icon = "🔵 INFO"
			}

			c.Printf(" %s  ", icon)
			white.Printf("%s", f.Message)
			dim.Printf(" [%s]\n", f.Category)

			if explain && f.Explain != "" {
				printExplainBox(f)
			}
		}
	} else {
		green.Printf("\n ✅ No findings! Your cluster is in excellent shape.\n")
	}

	// Summary
	fmt.Println()
	dim.Printf(" Scan completed in %.1fs | %d rules checked | %d findings\n",
		time.Since(scorecard.ScanTime).Seconds(),
		len(audit.BuiltinRules()),
		scorecard.TotalFindings)
	dim.Printf(" 💡 Run 'otacon scan --explain' for detailed explanations\n")
	dim.Printf(" 📊 Run 'otacon scan --export report.html' for exportable report\n")

	// Export if requested
	if outputFile != "" {
		// Apply severity filter to exported scorecard
		exportCard := scorecard
		if severityFilter != "" {
			exportCard = filterScorecardBySeverity(scorecard, severityFilter)
		}
		if err := exportReport(exportCard, outputFile); err != nil {
			return err
		}
	}

	// CI gate: exit with error if score below threshold
	if minScore > 0 && scorecard.OverallScore < minScore {
		return fmt.Errorf("cluster score %.0f is below minimum threshold %.0f", scorecard.OverallScore, minScore)
	}

	return nil
}

// filterScorecardBySeverity returns a copy of the scorecard with findings filtered to the given severity
func filterScorecardBySeverity(sc *engine.Scorecard, severity string) *engine.Scorecard {
	var minSev engine.Severity
	switch severity {
	case "critical", "CRITICAL":
		minSev = engine.SeverityCritical
	case "warning", "WARNING":
		minSev = engine.SeverityWarning
	default:
		return sc // info = show all
	}

	filtered := *sc
	filtered.Categories = nil
	filtered.TotalFindings = 0
	filtered.TotalCritical = 0
	filtered.TotalWarning = 0
	filtered.TotalInfo = 0

	for _, cat := range sc.Categories {
		newCat := engine.CategoryScore{
			Name:     cat.Name,
			Score:    cat.Score,
			MaxScore: cat.MaxScore,
			Weight:   cat.Weight,
		}
		for _, f := range cat.Findings {
			if f.Severity >= minSev {
				newCat.Findings = append(newCat.Findings, f)
				switch f.Severity {
				case engine.SeverityCritical:
					newCat.Critical++
					filtered.TotalCritical++
				case engine.SeverityWarning:
					newCat.Warning++
					filtered.TotalWarning++
				case engine.SeverityInfo:
					newCat.Info++
					filtered.TotalInfo++
				}
				filtered.TotalFindings++
			}
		}
		filtered.Categories = append(filtered.Categories, newCat)
	}

	return &filtered
}

func printExplainBox(f engine.Finding) {
	dim := color.New(color.FgHiBlack)
	white := color.New(color.FgWhite)
	green := color.New(color.FgGreen)

	dim.Printf(" ┌──────────────────────────────────────────────────────┐\n")
	dim.Printf(" │ ")
	white.Printf("WHAT: %s", f.Message)
	dim.Printf("\n │\n")
	dim.Printf(" │ ")
	white.Printf("WHY: %s", f.Explain)
	dim.Printf("\n │\n")
	if f.Remediation != "" {
		dim.Printf(" │ ")
		green.Printf("FIX: %s", f.Remediation)
		dim.Printf("\n")
	}
	if f.Resource != "" {
		dim.Printf(" │ ")
		white.Printf("AFFECTED: %s", f.Resource)
		dim.Printf("\n")
	}
	dim.Printf(" └──────────────────────────────────────────────────────┘\n")
}

func exportReport(scorecard *engine.Scorecard, filename string) error {
	var data []byte
	var err error

	// Auto-detect format by extension
	isHTML := len(filename) > 5 && filename[len(filename)-5:] == ".html"
	isJSON := len(filename) > 5 && filename[len(filename)-5:] == ".json"

	if isHTML {
		data = []byte(generateHTMLReport(scorecard))
	} else if isJSON {
		data, err = json.MarshalIndent(scorecard, "", "  ")
		if err != nil {
			return err
		}
	} else {
		// Default: HTML
		data = []byte(generateHTMLReport(scorecard))
	}

	err = os.WriteFile(filename, data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write report: %w", err)
	}

	green := color.New(color.FgGreen)
	green.Printf(" 📄 Report exported to: %s\n", filename)
	return nil
}

func generateHTMLReport(sc *engine.Scorecard) string {
	gradeColor := "#3fb950"
	if sc.OverallScore < 60 {
		gradeColor = "#f85149"
	} else if sc.OverallScore < 80 {
		gradeColor = "#d29922"
	}

	html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Otacon Report — %s</title>
<style>
:root{--bg:#000000;--card:#0d1420;--border:#1e2a3a;--text:#c9d1d9;--muted:#8b949e;--accent:#3b82f6;--green:#22c55e;--yellow:#eab308;--red:#ef4444}
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;background:var(--bg);color:var(--text);line-height:1.6}
.container{max-width:960px;margin:0 auto;padding:40px 24px}
.header{text-align:center;margin-bottom:40px;padding-bottom:24px;border-bottom:1px solid var(--border)}
.header h1{font-size:28px;color:var(--accent);font-family:monospace;letter-spacing:3px;margin-bottom:4px}
.header p{color:var(--muted);font-size:14px}
.grade-card{text-align:center;background:var(--card);border:1px solid var(--border);border-radius:12px;padding:32px;margin-bottom:32px}
.grade{font-size:80px;font-weight:800;font-family:monospace;line-height:1}
.score{font-size:20px;color:var(--muted);margin-top:8px}
.stats{display:flex;justify-content:center;gap:32px;margin-top:20px;font-size:14px}
.stats div{text-align:center}
.stats .num{font-size:24px;font-weight:700;font-family:monospace}
.stats .lbl{color:var(--muted);font-size:11px;text-transform:uppercase;letter-spacing:0.5px}
.cat{background:var(--card);border:1px solid var(--border);border-radius:8px;margin-bottom:16px;overflow:hidden}
.cat-header{padding:16px 20px;display:flex;justify-content:space-between;align-items:center;cursor:pointer}
.cat-header h3{font-size:15px;font-weight:600}
.cat-pct{font-family:monospace;font-size:14px;color:var(--muted)}
.bar{background:#21262d;border-radius:4px;height:6px;margin:0 20px 16px}
.bar-fill{height:100%%;border-radius:4px;transition:width 0.4s}
.findings{border-top:1px solid var(--border)}
.finding{padding:10px 20px;font-size:13px;border-bottom:1px solid #21262d;display:flex;gap:10px;align-items:flex-start}
.finding:last-child{border-bottom:none}
.badge{padding:2px 6px;border-radius:3px;font-size:10px;font-weight:700;font-family:monospace;white-space:nowrap}
.badge-crit{background:rgba(248,81,73,0.15);color:var(--red)}
.badge-warn{background:rgba(210,153,34,0.15);color:var(--yellow)}
.badge-info{background:rgba(88,166,255,0.15);color:var(--accent)}
.finding-msg{flex:1}
.finding-res{font-size:11px;color:var(--muted);font-family:monospace;margin-top:2px}
.footer{text-align:center;color:#484f58;font-size:11px;margin-top:40px;padding-top:20px;border-top:1px solid var(--border)}
</style>
</head>
<body>
<div class="container">
<div class="header">
  <h1>OTACON</h1>
  <p>Cluster: %s · Scanned: %s · %d nodes · %d pods</p>
</div>

<div class="grade-card">
  <div class="grade" style="color:%s">%s</div>
  <div class="score">%.0f / 100</div>
  <div class="stats">
    <div><div class="num" style="color:var(--red)">%d</div><div class="lbl">Critical</div></div>
    <div><div class="num" style="color:var(--yellow)">%d</div><div class="lbl">Warning</div></div>
    <div><div class="num" style="color:var(--accent)">%d</div><div class="lbl">Info</div></div>
    <div><div class="num">%d</div><div class="lbl">Total</div></div>
  </div>
</div>
`,
		sc.ClusterName,
		sc.ClusterName, sc.ScanTime.Format("2006-01-02 15:04"), sc.NodeCount, sc.PodCount,
		gradeColor, sc.Grade, sc.OverallScore,
		sc.TotalCritical, sc.TotalWarning, sc.TotalInfo, sc.TotalFindings)

	// Categories
	for _, cat := range sc.Categories {
		pct := cat.Percentage()
		barColor := "var(--green)"
		if pct < 60 {
			barColor = "var(--red)"
		} else if pct < 80 {
			barColor = "var(--yellow)"
		}
		findingCount := cat.Critical + cat.Warning + cat.Info

		html += fmt.Sprintf(`<div class="cat">
  <div class="cat-header">
    <h3>%s</h3>
    <span class="cat-pct">%.0f%% · %d findings</span>
  </div>
  <div class="bar"><div class="bar-fill" style="width:%.0f%%;background:%s"></div></div>
`, cat.Name, pct, findingCount, pct, barColor)

		if len(cat.Findings) > 0 {
			html += `  <div class="findings">`
			for _, f := range cat.Findings {
				badgeClass := "badge-info"
				if f.Severity == engine.SeverityCritical {
					badgeClass = "badge-crit"
				} else if f.Severity == engine.SeverityWarning {
					badgeClass = "badge-warn"
				}
				html += fmt.Sprintf(`    <div class="finding">
      <span class="badge %s">%s</span>
      <div class="finding-msg">%s<div class="finding-res">%s</div></div>
    </div>
`, badgeClass, f.Severity.String(), f.Message, f.Resource)
			}
			html += `  </div>`
		}
		html += `</div>`
	}

	html += fmt.Sprintf(`
<div class="footer">Otacon Intelligence Platform · Generated %s</div>
</div>
</body>
</html>`, time.Now().Format("2006-01-02 15:04:05"))

	return html
}
