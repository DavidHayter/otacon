package audit

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/merthan/otacon/internal/engine"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// ScanOptions configures the audit scan
type ScanOptions struct {
	Namespace  string
	Categories []string // Empty = all categories
	Verbose    bool
	Explain    bool
	Workers    int
}

// Scanner is the main audit orchestrator
type Scanner struct {
	client kubernetes.Interface
	rules  []Rule
}

// NewScanner creates a new audit scanner
func NewScanner(client kubernetes.Interface) *Scanner {
	return &Scanner{
		client: client,
		rules:  BuiltinRules(),
	}
}

// ClusterInfo gathers basic cluster information
type ClusterInfo struct {
	NodeCount      int
	PodCount       int
	NamespaceCount int
	DeploymentCount int
	ServiceCount   int
}

// GatherClusterInfo collects basic cluster stats
func (s *Scanner) GatherClusterInfo(ctx context.Context, namespace string) ClusterInfo {
	info := ClusterInfo{}

	nodes, err := s.client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err == nil && nodes != nil {
		info.NodeCount = len(nodes.Items)
	}

	if namespace != "" {
		pods, _ := s.client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
		if pods != nil {
			info.PodCount = len(pods.Items)
		}
		deploys, _ := s.client.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
		if deploys != nil {
			info.DeploymentCount = len(deploys.Items)
		}
		svcs, _ := s.client.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
		if svcs != nil {
			info.ServiceCount = len(svcs.Items)
		}
		info.NamespaceCount = 1
	} else {
		pods, _ := s.client.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
		if pods != nil {
			info.PodCount = len(pods.Items)
		}
		deploys, _ := s.client.AppsV1().Deployments("").List(ctx, metav1.ListOptions{})
		if deploys != nil {
			info.DeploymentCount = len(deploys.Items)
		}
		svcs, _ := s.client.CoreV1().Services("").List(ctx, metav1.ListOptions{})
		if svcs != nil {
			info.ServiceCount = len(svcs.Items)
		}
		nsList, _ := s.client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
		if nsList != nil {
			info.NamespaceCount = len(nsList.Items)
		}
	}

	return info
}

// Scan runs all audit rules and produces a scorecard
func (s *Scanner) Scan(ctx context.Context, opts ScanOptions) (*engine.Scorecard, error) {
	startTime := time.Now()

	// Determine which rules to run
	activeRules := s.filterRules(opts.Categories)

	// Run all rules concurrently
	type ruleResult struct {
		rule     Rule
		findings []engine.Finding
	}

	workers := opts.Workers
	if workers <= 0 {
		workers = 10
	}

	results := make(chan ruleResult, len(activeRules))
	sem := make(chan struct{}, workers)
	var wg sync.WaitGroup

	for _, rule := range activeRules {
		wg.Add(1)
		go func(r Rule) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			findings := r.Check(ctx, s.client, opts.Namespace)
			results <- ruleResult{rule: r, findings: findings}
		}(rule)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect all findings
	var allFindings []engine.Finding
	for result := range results {
		if opts.Explain {
			for i := range result.findings {
				result.findings[i].Explain = result.rule.Explain
			}
		}
		allFindings = append(allFindings, result.findings...)
	}

	// Build scorecard
	scorecard := s.buildScorecard(allFindings, opts.Namespace, startTime)

	// Gather cluster info
	clusterInfo := s.GatherClusterInfo(ctx, opts.Namespace)
	scorecard.NodeCount = clusterInfo.NodeCount
	scorecard.PodCount = clusterInfo.PodCount
	scorecard.NamespaceCount = clusterInfo.NamespaceCount

	return scorecard, nil
}

func (s *Scanner) filterRules(categories []string) []Rule {
	if len(categories) == 0 {
		return s.rules
	}

	catMap := make(map[string]bool)
	for _, c := range categories {
		catMap[c] = true
	}

	var filtered []Rule
	for _, rule := range s.rules {
		if catMap[rule.Category] {
			filtered = append(filtered, rule)
		}
	}
	return filtered
}

func (s *Scanner) buildScorecard(findings []engine.Finding, namespace string, scanTime time.Time) *engine.Scorecard {
	// Define categories with weights
	type catDef struct {
		name   string
		weight float64
	}

	categories := []catDef{
		{name: "Security", weight: 0.25},
		{name: "Resource Management", weight: 0.20},
		{name: "Reliability", weight: 0.25},
		{name: "Best Practices", weight: 0.15},
		{name: "Network Policies", weight: 0.15},
	}

	// Group findings by category
	catFindings := make(map[string][]engine.Finding)
	for _, f := range findings {
		catFindings[f.Category] = append(catFindings[f.Category], f)
	}

	var catScores []engine.CategoryScore
	totalScore := 0.0
	totalCritical, totalWarning, totalInfo := 0, 0, 0

	for _, cat := range categories {
		cf := catFindings[cat.name]
		critical, warning, info := 0, 0, 0

		for _, f := range cf {
			switch f.Severity {
			case engine.SeverityCritical:
				critical++
			case engine.SeverityWarning:
				warning++
			case engine.SeverityInfo:
				info++
			}
		}

		// Score calculation:
		// Start at 100, deduct per finding severity
		// Critical: -10 each (max deduction 60)
		// Warning:  -5 each (max deduction 30)
		// Info:     -2 each (max deduction 10)
		score := 100.0
		critDeduct := float64(critical) * 10.0
		if critDeduct > 60 {
			critDeduct = 60
		}
		warnDeduct := float64(warning) * 5.0
		if warnDeduct > 30 {
			warnDeduct = 30
		}
		infoDeduct := float64(info) * 2.0
		if infoDeduct > 10 {
			infoDeduct = 10
		}
		score -= critDeduct + warnDeduct + infoDeduct
		if score < 0 {
			score = 0
		}

		catScores = append(catScores, engine.CategoryScore{
			Name:     cat.name,
			Score:    score,
			MaxScore: 100,
			Weight:   cat.weight,
			Findings: cf,
			Critical: critical,
			Warning:  warning,
			Info:     info,
		})

		totalScore += score * cat.weight
		totalCritical += critical
		totalWarning += warning
		totalInfo += info
	}

	// Sort findings by severity for display
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].Severity != findings[j].Severity {
			return findings[i].Severity > findings[j].Severity
		}
		return findings[i].Category < findings[j].Category
	})

	return &engine.Scorecard{
		ClusterName:   engine.GetClusterName(engine.KubeConfig{}),
		ScanTime:      scanTime,
		OverallScore:  totalScore,
		Grade:         engine.CalculateGrade(totalScore),
		Categories:    catScores,
		TotalFindings: len(findings),
		TotalCritical: totalCritical,
		TotalWarning:  totalWarning,
		TotalInfo:     totalInfo,
	}
}

// GetTopFindings returns the most important findings, sorted by severity
func GetTopFindings(scorecard *engine.Scorecard, limit int) []engine.Finding {
	var all []engine.Finding
	for _, cat := range scorecard.Categories {
		all = append(all, cat.Findings...)
	}

	sort.Slice(all, func(i, j int) bool {
		if all[i].Severity != all[j].Severity {
			return all[i].Severity > all[j].Severity
		}
		return all[i].Category < all[j].Category
	})

	if limit > 0 && len(all) > limit {
		return all[:limit]
	}
	return all
}

// FormatScorecardText formats the scorecard for terminal display
func FormatScorecardText(sc *engine.Scorecard) string {
	output := ""

	// Header
	output += fmt.Sprintf("\n ┌─────────────────────────────────────────────────────┐\n")
	output += fmt.Sprintf(" │              CLUSTER HEALTH SCORECARD                │\n")
	output += fmt.Sprintf(" │                                                      │\n")
	output += fmt.Sprintf(" │           Overall Score: %s (%s/100)%s│\n",
		sc.Grade, fmt.Sprintf("%.0f", sc.OverallScore), padding(sc.Grade, sc.OverallScore))
	output += fmt.Sprintf(" │                                                      │\n")

	// Category bars
	for _, cat := range sc.Categories {
		pct := cat.Percentage()
		bar := engine.ProgressBar(pct, 12)
		status := "✓ Clean"
		if cat.Critical > 0 {
			status = fmt.Sprintf("✗ %d findings", cat.Critical+cat.Warning+cat.Info)
		} else if cat.Warning > 0 {
			status = fmt.Sprintf("⚠ %d findings", cat.Warning+cat.Info)
		} else if cat.Info > 0 {
			status = fmt.Sprintf("ℹ %d findings", cat.Info)
		}

		name := fmt.Sprintf("%-18s", cat.Name)
		output += fmt.Sprintf(" │  %s %s  %3.0f%%  %s%s│\n",
			name, bar, pct, status, catPadding(cat.Name, pct, status))
	}

	output += fmt.Sprintf(" │                                                      │\n")
	output += fmt.Sprintf(" └─────────────────────────────────────────────────────┘\n")

	return output
}

func padding(grade string, score float64) string {
	totalLen := len(grade) + len(fmt.Sprintf("%.0f", score)) + 1 // grade + score + "/"
	padNeeded := 18 - totalLen
	if padNeeded < 0 {
		padNeeded = 0
	}
	result := ""
	for i := 0; i < padNeeded; i++ {
		result += " "
	}
	return result
}

func catPadding(name string, pct float64, status string) string {
	// Just ensure it fits
	return " "
}
