package intelligence

import (
	"testing"
	"time"

	"github.com/merthan/otacon/internal/engine"
)

// ============================================================
// CORRELATOR TESTS
// ============================================================

func TestCorrelatorNodeCascade(t *testing.T) {
	c := NewCorrelator()
	var incidents []engine.CorrelatedIncident
	c.OnIncident(func(inc engine.CorrelatedIncident) {
		incidents = append(incidents, inc)
	})

	now := time.Now()
	c.Ingest(engine.Event{ID: "e1", Reason: "Evicted", Namespace: "prod", ResourceName: "pod-1", NodeName: "node-1", Severity: engine.SeverityWarning, LastSeen: now.Add(-3 * time.Minute)})
	c.Ingest(engine.Event{ID: "e2", Reason: "Evicted", Namespace: "prod", ResourceName: "pod-2", NodeName: "node-1", Severity: engine.SeverityWarning, LastSeen: now.Add(-2 * time.Minute)})
	c.Ingest(engine.Event{ID: "trigger", Reason: "NodeNotReady", Namespace: "kube-system", ResourceName: "node-1", NodeName: "node-1", Severity: engine.SeverityCritical, LastSeen: now})

	if len(incidents) == 0 {
		t.Error("expected correlated incident for node cascade")
	} else if len(incidents[0].Events) < 2 {
		t.Errorf("expected multiple events, got %d", len(incidents[0].Events))
	}
}

func TestCorrelatorOOMCascade(t *testing.T) {
	c := NewCorrelator()
	var incidents []engine.CorrelatedIncident
	c.OnIncident(func(inc engine.CorrelatedIncident) { incidents = append(incidents, inc) })

	now := time.Now()
	c.Ingest(engine.Event{ID: "clb", Reason: "CrashLoopBackOff", Namespace: "prod", ResourceName: "svc", Severity: engine.SeverityWarning, LastSeen: now.Add(-5 * time.Minute)})
	c.Ingest(engine.Event{ID: "oom", Reason: "OOMKilled", Namespace: "prod", ResourceName: "svc", Severity: engine.SeverityCritical, LastSeen: now})

	if len(incidents) == 0 {
		t.Error("expected OOM cascade incident")
	}
}

func TestCorrelatorNoFalsePositive(t *testing.T) {
	c := NewCorrelator()
	var incidents []engine.CorrelatedIncident
	c.OnIncident(func(inc engine.CorrelatedIncident) { incidents = append(incidents, inc) })
	c.Ingest(engine.Event{ID: "n1", Reason: "Pulled", Severity: engine.SeverityInfo, LastSeen: time.Now()})
	if len(incidents) > 0 {
		t.Error("normal events should not trigger incidents")
	}
}

func TestCorrelatorBufferLimit(t *testing.T) {
	c := NewCorrelator()
	c.bufferSize = 50
	for i := 0; i < 100; i++ {
		c.Ingest(engine.Event{ID: "x", Reason: "Pulled", Severity: engine.SeverityInfo, LastSeen: time.Now()})
	}
	events := c.GetBufferedEvents(1 * time.Hour)
	if len(events) > 50 {
		t.Errorf("buffer exceeded limit: %d", len(events))
	}
}

// ============================================================
// DEDUPLICATOR TESTS
// ============================================================

func TestDeduplicatorGrouping(t *testing.T) {
	d := NewDeduplicator(DeduplicationConfig{WindowDuration: 15 * time.Minute, GroupBy: []string{"namespace", "reason"}, MaxDigestSize: 50})
	var emitted []DeduplicatedGroup
	d.OnGroup(func(g DeduplicatedGroup) { emitted = append(emitted, g) })

	now := time.Now()
	for i := 0; i < 5; i++ {
		d.Ingest(engine.Event{ID: "e", Reason: "CrashLoopBackOff", Namespace: "prod", ResourceName: "p-" + string(rune('a'+i)), Severity: engine.SeverityWarning, LastSeen: now.Add(time.Duration(i) * time.Second)})
	}

	if len(emitted) == 0 {
		t.Error("expected group emission at count=5")
	}
	groups := d.GetActiveGroups()
	if len(groups) == 0 {
		t.Fatal("expected active groups")
	}
	if groups[0].Count != 5 {
		t.Errorf("expected count=5, got %d", groups[0].Count)
	}
}

func TestDeduplicatorFlush(t *testing.T) {
	d := NewDeduplicator(DefaultDeduplicationConfig())
	var flushed int
	d.OnGroup(func(g DeduplicatedGroup) { flushed++ })
	d.Ingest(engine.Event{ID: "e1", Reason: "BackOff", Namespace: "default", Severity: engine.SeverityWarning, LastSeen: time.Now()})
	d.Flush()
	if flushed == 0 {
		t.Error("flush should emit pending groups")
	}
}

func TestDeduplicatorSummary(t *testing.T) {
	g := DeduplicatedGroup{
		Reason: "CrashLoopBackOff", Namespace: "prod", Severity: engine.SeverityCritical, Count: 42,
		AffectedPods: []string{"prod/a", "prod/b", "prod/c", "prod/d"},
		FirstSeen: time.Now().Add(-10 * time.Minute), LastSeen: time.Now(),
	}
	s := g.Summary()
	if s == "" {
		t.Error("summary empty")
	}
}

// ============================================================
// COOLDOWN TESTS
// ============================================================

func TestCooldownBasic(t *testing.T) {
	cm := NewCooldownManager(CooldownConfig{DefaultDuration: 100 * time.Millisecond, PerGroup: true, MaxSuppressed: 100})
	if !cm.ShouldNotify("k", "WARNING") {
		t.Error("first should pass")
	}
	if cm.ShouldNotify("k", "WARNING") {
		t.Error("repeat should be suppressed")
	}
	time.Sleep(150 * time.Millisecond)
	if !cm.ShouldNotify("k", "WARNING") {
		t.Error("should pass after cooldown")
	}
}

func TestCooldownSafetyValve(t *testing.T) {
	cm := NewCooldownManager(CooldownConfig{DefaultDuration: 1 * time.Hour, PerGroup: true, MaxSuppressed: 3})
	cm.ShouldNotify("v", "WARNING")
	for i := 0; i < 3; i++ {
		cm.ShouldNotify("v", "WARNING")
	}
	if !cm.ShouldNotify("v", "WARNING") {
		t.Error("safety valve should trigger")
	}
}

func TestCooldownStats(t *testing.T) {
	cm := NewCooldownManager(DefaultCooldownConfig())
	cm.ShouldNotify("a", "WARNING")
	cm.ShouldNotify("a", "WARNING")
	cm.ShouldNotify("b", "CRITICAL")
	stats := cm.GetStats()
	if stats.TotalReceived != 3 || stats.TotalPassed != 2 || stats.TotalSuppressed != 1 {
		t.Errorf("unexpected stats: %+v", stats)
	}
}

func TestCooldownReset(t *testing.T) {
	cm := NewCooldownManager(CooldownConfig{DefaultDuration: 1 * time.Hour, PerGroup: true, MaxSuppressed: 100})
	cm.ShouldNotify("r", "WARNING")
	cm.Reset("r")
	if !cm.ShouldNotify("r", "WARNING") {
		t.Error("should pass after reset")
	}
}
