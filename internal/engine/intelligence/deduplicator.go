package intelligence

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/merthan/otacon/internal/engine"
)

// DeduplicationConfig configures the deduplication behavior
type DeduplicationConfig struct {
	WindowDuration time.Duration // Group events within this window (default 15min)
	GroupBy        []string      // Fields to group by: namespace, reason, resourceKind, nodeName
	MaxDigestSize  int           // Max events in a single digest (default 100)
}

// DefaultDeduplicationConfig returns sensible defaults
func DefaultDeduplicationConfig() DeduplicationConfig {
	return DeduplicationConfig{
		WindowDuration: 15 * time.Minute,
		GroupBy:        []string{"namespace", "reason"},
		MaxDigestSize:  100,
	}
}

// DeduplicatedGroup represents a group of similar events
type DeduplicatedGroup struct {
	Key           string         `json:"key"`
	Reason        string         `json:"reason"`
	Namespace     string         `json:"namespace"`
	Severity      engine.Severity `json:"severity"`
	Count         int            `json:"count"`
	AffectedPods  []string       `json:"affectedPods"`
	AffectedNodes []string       `json:"affectedNodes"`
	FirstSeen     time.Time      `json:"firstSeen"`
	LastSeen      time.Time      `json:"lastSeen"`
	SampleMessage string         `json:"sampleMessage"`
	Events        []engine.Event `json:"events"`
}

// Summary returns a human-readable summary of the group
func (g *DeduplicatedGroup) Summary() string {
	podInfo := ""
	if len(g.AffectedPods) > 3 {
		podInfo = fmt.Sprintf("%s, %s, %s (+%d more)",
			g.AffectedPods[0], g.AffectedPods[1], g.AffectedPods[2],
			len(g.AffectedPods)-3)
	} else if len(g.AffectedPods) > 0 {
		podInfo = strings.Join(g.AffectedPods, ", ")
	}

	nodeInfo := ""
	if len(g.AffectedNodes) > 0 {
		nodeInfo = fmt.Sprintf(" across nodes: %s", strings.Join(g.AffectedNodes, ", "))
	}

	duration := g.LastSeen.Sub(g.FirstSeen)
	durationStr := ""
	if duration > time.Second {
		durationStr = fmt.Sprintf(" over %s", formatDuration(duration))
	}

	return fmt.Sprintf("%s %s: %d occurrences in %s [%s]%s%s",
		g.Severity.Icon(), g.Reason, g.Count, g.Namespace,
		podInfo, nodeInfo, durationStr)
}

// Deduplicator groups and deduplicates similar events
type Deduplicator struct {
	config   DeduplicationConfig
	groups   map[string]*DeduplicatedGroup
	handlers []func(DeduplicatedGroup)
	mu       sync.Mutex
}

// NewDeduplicator creates a new deduplication engine
func NewDeduplicator(config DeduplicationConfig) *Deduplicator {
	return &Deduplicator{
		config: config,
		groups: make(map[string]*DeduplicatedGroup),
	}
}

// OnGroup registers a handler for when a dedup group is emitted
func (d *Deduplicator) OnGroup(handler func(DeduplicatedGroup)) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.handlers = append(d.handlers, handler)
}

// Ingest processes a new event into the deduplication engine
func (d *Deduplicator) Ingest(event engine.Event) {
	d.mu.Lock()
	defer d.mu.Unlock()

	key := d.buildKey(event)

	group, exists := d.groups[key]
	if !exists {
		group = &DeduplicatedGroup{
			Key:           key,
			Reason:        event.Reason,
			Namespace:     event.Namespace,
			Severity:      event.Severity,
			Count:         0,
			AffectedPods:  []string{},
			AffectedNodes: []string{},
			FirstSeen:     event.LastSeen,
			LastSeen:      event.LastSeen,
			SampleMessage: event.Message,
		}
		d.groups[key] = group
	}

	// Check if event is within the dedup window
	if event.LastSeen.Sub(group.LastSeen) > d.config.WindowDuration {
		// Window expired — emit the old group and start fresh
		if group.Count > 0 {
			d.emitGroup(*group)
		}

		group.Count = 0
		group.AffectedPods = []string{}
		group.AffectedNodes = []string{}
		group.Events = []engine.Event{}
		group.FirstSeen = event.LastSeen
	}

	// Update group
	group.Count++
	group.LastSeen = event.LastSeen

	// Track affected pods (unique)
	podName := fmt.Sprintf("%s/%s", event.Namespace, event.ResourceName)
	if !containsStr(group.AffectedPods, podName) {
		group.AffectedPods = append(group.AffectedPods, podName)
	}

	// Track affected nodes (unique)
	if event.NodeName != "" && !containsStr(group.AffectedNodes, event.NodeName) {
		group.AffectedNodes = append(group.AffectedNodes, event.NodeName)
	}

	// Keep events up to max digest size
	if len(group.Events) < d.config.MaxDigestSize {
		group.Events = append(group.Events, event)
	}

	// Emit if this is a significant group (multiple events)
	if group.Count == 5 || group.Count == 10 || group.Count == 25 || group.Count == 50 || group.Count == 100 {
		d.emitGroup(*group)
	}
}

// Flush emits all pending groups
func (d *Deduplicator) Flush() {
	d.mu.Lock()
	defer d.mu.Unlock()

	for _, group := range d.groups {
		if group.Count > 0 {
			d.emitGroup(*group)
		}
	}
}

// GetActiveGroups returns all currently active dedup groups
func (d *Deduplicator) GetActiveGroups() []DeduplicatedGroup {
	d.mu.Lock()
	defer d.mu.Unlock()

	var result []DeduplicatedGroup
	cutoff := time.Now().Add(-d.config.WindowDuration)
	for _, group := range d.groups {
		if group.LastSeen.After(cutoff) && group.Count > 0 {
			result = append(result, *group)
		}
	}
	return result
}

// Cleanup removes expired groups
func (d *Deduplicator) Cleanup() {
	d.mu.Lock()
	defer d.mu.Unlock()

	cutoff := time.Now().Add(-d.config.WindowDuration * 2)
	for key, group := range d.groups {
		if group.LastSeen.Before(cutoff) {
			delete(d.groups, key)
		}
	}
}

func (d *Deduplicator) buildKey(event engine.Event) string {
	parts := []string{}
	for _, field := range d.config.GroupBy {
		switch field {
		case "namespace":
			parts = append(parts, event.Namespace)
		case "reason":
			parts = append(parts, event.Reason)
		case "resourceKind":
			parts = append(parts, event.ResourceKind)
		case "nodeName":
			parts = append(parts, event.NodeName)
		}
	}
	return strings.Join(parts, ":")
}

func (d *Deduplicator) emitGroup(group DeduplicatedGroup) {
	log.Printf("[dedup] Group emitted: %s (%d events)", group.Key, group.Count)
	for _, handler := range d.handlers {
		go handler(group)
	}
}

func containsStr(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.0fs", d.Seconds())
	}
	if d < time.Hour {
		return fmt.Sprintf("%.0fm", d.Minutes())
	}
	return fmt.Sprintf("%.1fh", d.Hours())
}
