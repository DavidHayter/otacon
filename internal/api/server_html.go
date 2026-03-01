package api

import (
	"fmt"
	"time"

	"github.com/merthan/otacon/internal/engine"
)

func generateFullHTMLReport(scorecard *engine.Scorecard) string {
	html := fmt.Sprintf(`<!DOCTYPE html><html><head><meta charset="UTF-8">
<title>Otacon Report - %s</title>
<style>*{margin:0;padding:0;box-sizing:border-box}body{font-family:system-ui;background:#000000;color:#c9d1d9;padding:40px}.c{max-width:900px;margin:0 auto}h1{color:#3b82f6}
.g{font-size:64px;font-weight:bold;color:#3b82f6;text-align:center;padding:24px}.s{text-align:center;color:#8b949e;font-size:20px;margin-bottom:32px}
.cat{background:#0d1420;border:1px solid #1e2a3a;border-radius:8px;padding:16px;margin-bottom:12px}
.bar{background:#21262d;border-radius:4px;height:8px;margin:8px 0}.fill{height:100%%;border-radius:4px}
.f{padding:6px 0;font-size:13px;border-bottom:1px solid #21262d}
.cr{color:#ef4444}.wa{color:#eab308}.in{color:#3b82f6}
.ft{text-align:center;color:#484f58;margin-top:32px;font-size:12px}</style></head>
<body><div class="c"><h1>Otacon Audit Report</h1><p style="color:#8b949e">Cluster: %s | %s</p>
<div class="g">%s</div><div class="s">%.0f / 100</div>`,
		scorecard.ClusterName, scorecard.ClusterName, scorecard.ScanTime.Format("2006-01-02 15:04"), scorecard.Grade, scorecard.OverallScore)

	for _, cat := range scorecard.Categories {
		pct := cat.Percentage()
		color := "#22c55e"
		if pct < 60 { color = "#ef4444" } else if pct < 80 { color = "#eab308" }

		html += fmt.Sprintf(`<div class="cat"><b>%s</b> — %.0f%%<div class="bar"><div class="fill" style="width:%.0f%%;background:%s"></div></div>`, cat.Name, pct, pct, color)
		for _, f := range cat.Findings {
			cls := "in"
			if f.Severity == engine.SeverityCritical { cls = "cr" } else if f.Severity == engine.SeverityWarning { cls = "wa" }
			html += fmt.Sprintf(`<div class="f"><span class="%s">[%s]</span> %s</div>`, cls, f.Severity.String(), f.Message)
		}
		html += `</div>`
	}

	html += fmt.Sprintf(`<div class="ft">Otacon Intelligence Platform — %s</div></div></body></html>`, time.Now().Format("2006-01-02 15:04:05"))
	return html
}
