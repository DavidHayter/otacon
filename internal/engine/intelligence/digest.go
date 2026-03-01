package intelligence

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/merthan/otacon/internal/engine"
	"github.com/merthan/otacon/internal/engine/audit"
	"k8s.io/client-go/kubernetes"
)

// DigestType represents the kind of digest
type DigestType string

const (
	DigestDaily         DigestType = "daily"
	DigestWeekly        DigestType = "weekly"
	DigestComprehensive DigestType = "comprehensive"
	DigestEventsOnly    DigestType = "events-only"
	DigestAuditOnly     DigestType = "audit-only"
)

// DigestConfig configures digest generation
type DigestConfig struct {
	Type              DigestType
	IncludeScorecard  bool
	IncludeResources  bool
	IncludeTrending   bool
	IncludeTopEvents  int // Number of top events to include
}

// Digest represents a periodic cluster health summary
type Digest struct {
	ID            string                      `json:"id"`
	Type          DigestType                  `json:"type"`
	GeneratedAt   time.Time                   `json:"generatedAt"`
	PeriodStart   time.Time                   `json:"periodStart"`
	PeriodEnd     time.Time                   `json:"periodEnd"`
	ClusterName   string                      `json:"clusterName"`
	Summary       DigestSummary               `json:"summary"`
	Scorecard     *engine.Scorecard           `json:"scorecard,omitempty"`
	TopEvents     []engine.Event              `json:"topEvents,omitempty"`
	Incidents     []engine.CorrelatedIncident  `json:"incidents,omitempty"`
	TopGroups     []DeduplicatedGroup         `json:"topGroups,omitempty"`
	Trending      *TrendingAnalysis           `json:"trending,omitempty"`
}

// DigestSummary is the high-level overview
type DigestSummary struct {
	TotalEvents     int     `json:"totalEvents"`
	CriticalEvents  int     `json:"criticalEvents"`
	WarningEvents   int     `json:"warningEvents"`
	InfoEvents      int     `json:"infoEvents"`
	IncidentCount   int     `json:"incidentCount"`
	OverallGrade    string  `json:"overallGrade"`
	OverallScore    float64 `json:"overallScore"`
	ScoreChange     float64 `json:"scoreChange"` // vs previous period
	HealthStatus    string  `json:"healthStatus"` // "improving", "stable", "degrading"
}

// TrendingAnalysis compares current vs previous period
type TrendingAnalysis struct {
	ScorePrevious    float64           `json:"scorePrevious"`
	ScoreCurrent     float64           `json:"scoreCurrent"`
	ScoreDirection   string            `json:"scoreDirection"` // "up", "down", "stable"
	NewFindings      []engine.Finding  `json:"newFindings"`
	ResolvedFindings []engine.Finding  `json:"resolvedFindings"`
	PersistentIssues int               `json:"persistentIssues"`
}

// DigestBuilder creates periodic digest reports
type DigestBuilder struct {
	client     kubernetes.Interface
	correlator *Correlator
	dedup      *Deduplicator
	mu         sync.Mutex
	lastDigest *Digest
	digestSeq  int
}

// NewDigestBuilder creates a new digest builder
func NewDigestBuilder(client kubernetes.Interface, correlator *Correlator, dedup *Deduplicator) *DigestBuilder {
	return &DigestBuilder{
		client:     client,
		correlator: correlator,
		dedup:      dedup,
	}
}

// BuildDigest generates a cluster health digest
func (db *DigestBuilder) BuildDigest(ctx context.Context, config DigestConfig) (*Digest, error) {
	db.mu.Lock()
	defer db.mu.Unlock()

	db.digestSeq++

	// Determine period
	now := time.Now()
	var periodStart time.Time
	switch config.Type {
	case DigestDaily:
		periodStart = now.Add(-24 * time.Hour)
	case DigestWeekly:
		periodStart = now.Add(-7 * 24 * time.Hour)
	default:
		periodStart = now.Add(-24 * time.Hour)
	}

	log.Printf("[digest] Building %s digest for period %s to %s",
		config.Type, periodStart.Format("2006-01-02 15:04"), now.Format("2006-01-02 15:04"))

	digest := &Digest{
		ID:          fmt.Sprintf("DIG-%06d", db.digestSeq),
		Type:        config.Type,
		GeneratedAt: now,
		PeriodStart: periodStart,
		PeriodEnd:   now,
		ClusterName: engine.GetClusterName(engine.KubeConfig{}),
	}

	// Gather events from the period
	events := db.correlator.GetBufferedEvents(now.Sub(periodStart))
	incidents := db.correlator.GetRecentIncidents(now.Sub(periodStart))
	groups := db.dedup.GetActiveGroups()

	// Build summary
	summary := DigestSummary{
		TotalEvents:   len(events),
		IncidentCount: len(incidents),
	}

	for _, e := range events {
		switch e.Severity {
		case engine.SeverityCritical:
			summary.CriticalEvents++
		case engine.SeverityWarning:
			summary.WarningEvents++
		case engine.SeverityInfo:
			summary.InfoEvents++
		}
	}

	// Run audit for scorecard
	if config.IncludeScorecard {
		scanner := audit.NewScanner(db.client)
		scorecard, err := scanner.Scan(ctx, audit.ScanOptions{Workers: 10})
		if err == nil {
			digest.Scorecard = scorecard
			summary.OverallGrade = scorecard.Grade
			summary.OverallScore = scorecard.OverallScore

			// Compare with previous digest
			if db.lastDigest != nil && db.lastDigest.Scorecard != nil {
				summary.ScoreChange = scorecard.OverallScore - db.lastDigest.Scorecard.OverallScore
				if summary.ScoreChange > 2 {
					summary.HealthStatus = "improving"
				} else if summary.ScoreChange < -2 {
					summary.HealthStatus = "degrading"
				} else {
					summary.HealthStatus = "stable"
				}
			} else {
				summary.HealthStatus = "baseline"
			}
		}
	}

	digest.Summary = summary

	// Top events (sorted by severity)
	if config.IncludeTopEvents > 0 {
		topEvents := sortEventsBySeverity(events)
		if len(topEvents) > config.IncludeTopEvents {
			topEvents = topEvents[:config.IncludeTopEvents]
		}
		digest.TopEvents = topEvents
	}

	// Incidents
	digest.Incidents = incidents

	// Top dedup groups
	digest.TopGroups = groups

	// Trending analysis
	if config.IncludeTrending && db.lastDigest != nil {
		digest.Trending = db.buildTrending(digest)
	}

	db.lastDigest = digest

	log.Printf("[digest] Digest %s generated: %d events, %d incidents, grade: %s",
		digest.ID, summary.TotalEvents, summary.IncidentCount, summary.OverallGrade)

	return digest, nil
}

// FormatDigestText formats a digest for terminal/text output
func FormatDigestText(d *Digest) string {
	var sb strings.Builder

	digestType := string(d.Type)
	if len(digestType) > 0 {
		digestType = strings.ToUpper(digestType[:1]) + digestType[1:]
	}
	sb.WriteString(fmt.Sprintf("\n━━━ Otacon %s Digest ━━━\n", digestType))
	sb.WriteString(fmt.Sprintf("Cluster: %s | Period: %s to %s\n\n",
		d.ClusterName,
		d.PeriodStart.Format("Jan 02 15:04"),
		d.PeriodEnd.Format("Jan 02 15:04")))

	// Summary
	sb.WriteString(fmt.Sprintf("📊 Overview\n"))
	if d.Summary.OverallGrade != "" {
		sb.WriteString(fmt.Sprintf("   Grade: %s (%.0f/100)", d.Summary.OverallGrade, d.Summary.OverallScore))
		if d.Summary.ScoreChange != 0 {
			direction := "↑"
			if d.Summary.ScoreChange < 0 {
				direction = "↓"
			}
			sb.WriteString(fmt.Sprintf(" %s %.1f", direction, d.Summary.ScoreChange))
		}
		sb.WriteString("\n")
	}
	sb.WriteString(fmt.Sprintf("   Events: %d total (🔴 %d critical, 🟡 %d warning, 🔵 %d info)\n",
		d.Summary.TotalEvents, d.Summary.CriticalEvents, d.Summary.WarningEvents, d.Summary.InfoEvents))
	sb.WriteString(fmt.Sprintf("   Incidents: %d correlated\n", d.Summary.IncidentCount))

	if d.Summary.HealthStatus != "" && d.Summary.HealthStatus != "baseline" {
		icon := "→"
		switch d.Summary.HealthStatus {
		case "improving":
			icon = "📈"
		case "degrading":
			icon = "📉"
		case "stable":
			icon = "➡️"
		}
		sb.WriteString(fmt.Sprintf("   Trend: %s %s\n", icon, d.Summary.HealthStatus))
	}

	// Top incidents
	if len(d.Incidents) > 0 {
		sb.WriteString(fmt.Sprintf("\n🚨 Incidents (%d)\n", len(d.Incidents)))
		for i, inc := range d.Incidents {
			if i >= 5 {
				sb.WriteString(fmt.Sprintf("   ... and %d more\n", len(d.Incidents)-5))
				break
			}
			sb.WriteString(fmt.Sprintf("   %s %s — %s (%d events)\n",
				inc.Severity.Icon(), inc.ID, inc.Title, len(inc.Events)))
		}
	}

	// Top event groups
	if len(d.TopGroups) > 0 {
		sb.WriteString(fmt.Sprintf("\n📋 Top Event Groups\n"))
		for i, g := range d.TopGroups {
			if i >= 5 {
				break
			}
			sb.WriteString(fmt.Sprintf("   %s\n", g.Summary()))
		}
	}

	sb.WriteString("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	return sb.String()
}

func (db *DigestBuilder) buildTrending(current *Digest) *TrendingAnalysis {
	if db.lastDigest == nil || db.lastDigest.Scorecard == nil || current.Scorecard == nil {
		return nil
	}

	trending := &TrendingAnalysis{
		ScorePrevious: db.lastDigest.Scorecard.OverallScore,
		ScoreCurrent:  current.Scorecard.OverallScore,
	}

	diff := trending.ScoreCurrent - trending.ScorePrevious
	if diff > 2 {
		trending.ScoreDirection = "up"
	} else if diff < -2 {
		trending.ScoreDirection = "down"
	} else {
		trending.ScoreDirection = "stable"
	}

	return trending
}

func sortEventsBySeverity(events []engine.Event) []engine.Event {
	sorted := make([]engine.Event, len(events))
	copy(sorted, events)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Severity != sorted[j].Severity {
			return sorted[i].Severity > sorted[j].Severity
		}
		return sorted[i].LastSeen.After(sorted[j].LastSeen)
	})
	return sorted
}
