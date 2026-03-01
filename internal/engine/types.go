package engine

import (
	"fmt"
	"time"
)

// Severity represents the importance level of a finding or event
type Severity int

const (
	SeverityInfo Severity = iota
	SeverityWarning
	SeverityCritical
)

func (s Severity) String() string {
	switch s {
	case SeverityInfo:
		return "INFO"
	case SeverityWarning:
		return "WARNING"
	case SeverityCritical:
		return "CRITICAL"
	default:
		return "UNKNOWN"
	}
}

func (s Severity) Icon() string {
	switch s {
	case SeverityInfo:
		return "🔵"
	case SeverityWarning:
		return "🟡"
	case SeverityCritical:
		return "🔴"
	default:
		return "⚪"
	}
}

// Finding represents a single audit finding
type Finding struct {
	ID          string    `json:"id"`
	Category    string    `json:"category"`
	Rule        string    `json:"rule"`
	Severity    Severity  `json:"severity"`
	Resource    string    `json:"resource"`
	Namespace   string    `json:"namespace"`
	Kind        string    `json:"kind"`
	Message     string    `json:"message"`
	Remediation string    `json:"remediation"`
	Explain     string    `json:"explain"`
	Timestamp   time.Time `json:"timestamp"`
}

// CategoryScore represents a score for a single audit category
type CategoryScore struct {
	Name       string    `json:"name"`
	Score      float64   `json:"score"`
	MaxScore   float64   `json:"maxScore"`
	Weight     float64   `json:"weight"`
	Findings   []Finding `json:"findings"`
	Critical   int       `json:"critical"`
	Warning    int       `json:"warning"`
	Info       int       `json:"info"`
}

func (cs CategoryScore) Percentage() float64 {
	if cs.MaxScore == 0 {
		return 0
	}
	return (cs.Score / cs.MaxScore) * 100
}

// Scorecard represents the overall cluster audit scorecard
type Scorecard struct {
	ClusterName    string          `json:"clusterName"`
	ScanTime       time.Time       `json:"scanTime"`
	OverallScore   float64         `json:"overallScore"`
	Grade          string          `json:"grade"`
	Categories     []CategoryScore `json:"categories"`
	TotalFindings  int             `json:"totalFindings"`
	TotalCritical  int             `json:"totalCritical"`
	TotalWarning   int             `json:"totalWarning"`
	TotalInfo      int             `json:"totalInfo"`
	NamespaceCount int             `json:"namespaceCount"`
	PodCount       int             `json:"podCount"`
	NodeCount      int             `json:"nodeCount"`
}

// CalculateGrade converts a numeric score to a letter grade
func CalculateGrade(score float64) string {
	switch {
	case score >= 95:
		return "A+"
	case score >= 90:
		return "A"
	case score >= 85:
		return "A-"
	case score >= 80:
		return "B+"
	case score >= 75:
		return "B"
	case score >= 70:
		return "B-"
	case score >= 60:
		return "C"
	case score >= 50:
		return "D"
	default:
		return "F"
	}
}

// ProgressBar generates a text-based progress bar
func ProgressBar(percentage float64, width int) string {
	filled := int(percentage / 100 * float64(width))
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}
	empty := width - filled
	return fmt.Sprintf("%s%s", repeat("█", filled), repeat("░", empty))
}

func repeat(s string, n int) string {
	result := ""
	for i := 0; i < n; i++ {
		result += s
	}
	return result
}

// Event represents a Kubernetes event processed by Otacon
type Event struct {
	ID            string    `json:"id"`
	Type          string    `json:"type"`
	Reason        string    `json:"reason"`
	Message       string    `json:"message"`
	Namespace     string    `json:"namespace"`
	ResourceKind  string    `json:"resourceKind"`
	ResourceName  string    `json:"resourceName"`
	NodeName      string    `json:"nodeName"`
	Severity      Severity  `json:"severity"`
	Count         int32     `json:"count"`
	FirstSeen     time.Time `json:"firstSeen"`
	LastSeen      time.Time `json:"lastSeen"`
	CorrelationID string    `json:"correlationId,omitempty"`
}

// CorrelatedIncident represents a group of related events
type CorrelatedIncident struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	RootCause   string    `json:"rootCause"`
	Severity    Severity  `json:"severity"`
	Events      []Event   `json:"events"`
	Impact      string    `json:"impact"`
	Suggestion  string    `json:"suggestion"`
	StartTime   time.Time `json:"startTime"`
	EndTime     time.Time `json:"endTime"`
}

// DiagnosticResult represents the output of a diagnostic check
type DiagnosticResult struct {
	Check       string   `json:"check"`
	Status      string   `json:"status"` // "pass", "fail", "warn"
	Message     string   `json:"message"`
	Details     []string `json:"details,omitempty"`
	Remediation string   `json:"remediation,omitempty"`
}

// ResourceAnalysis represents resource usage analysis for a workload
type ResourceAnalysis struct {
	Namespace      string  `json:"namespace"`
	Name           string  `json:"name"`
	Kind           string  `json:"kind"`
	Container      string  `json:"container"`
	CPURequest     string  `json:"cpuRequest"`
	CPULimit       string  `json:"cpuLimit"`
	MemRequest     string  `json:"memRequest"`
	MemLimit       string  `json:"memLimit"`
	CPUUsage       string  `json:"cpuUsage,omitempty"`
	MemUsage       string  `json:"memUsage,omitempty"`
	CPUEfficiency  float64 `json:"cpuEfficiency"`
	MemEfficiency  float64 `json:"memEfficiency"`
	Recommendation string  `json:"recommendation"`
}

// NotificationPayload represents a notification to be sent
type NotificationPayload struct {
	Title       string            `json:"title"`
	Severity    Severity          `json:"severity"`
	Body        string            `json:"body"`
	Fields      map[string]string `json:"fields,omitempty"`
	Links       map[string]string `json:"links,omitempty"`
	Enrichments []Enrichment      `json:"enrichments,omitempty"`
	Timestamp   time.Time         `json:"timestamp"`
}

// Enrichment represents additional context added to a notification
type Enrichment struct {
	Type    string `json:"type"`
	Title   string `json:"title"`
	Content string `json:"content"`
}
