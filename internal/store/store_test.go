package store

import (
	"os"
	"testing"
	"time"

	"github.com/merthan/otacon/internal/engine"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	tmpFile := t.TempDir() + "/test.db"
	cfg := DefaultConfig()
	cfg.DSN = tmpFile
	s, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	t.Cleanup(func() {
		s.Close()
		os.Remove(tmpFile)
	})
	return s
}

func TestStoreSaveAndGetEvents(t *testing.T) {
	s := testStore(t)

	event := engine.Event{
		ID:           "evt-001",
		Type:         "Warning",
		Reason:       "OOMKilled",
		Message:      "Container killed due to OOM",
		Namespace:    "production",
		ResourceKind: "Pod",
		ResourceName: "api-pod-1",
		NodeName:     "node-01",
		Severity:     engine.SeverityCritical,
		Count:        3,
		FirstSeen:    time.Now().Add(-5 * time.Minute),
		LastSeen:     time.Now(),
	}

	err := s.SaveEvent(event)
	if err != nil {
		t.Fatalf("SaveEvent failed: %v", err)
	}

	events, err := s.GetEvents(1*time.Hour, "", nil)
	if err != nil {
		t.Fatalf("GetEvents failed: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("Expected 1 event, got %d", len(events))
	}

	got := events[0]
	if got.ID != "evt-001" {
		t.Errorf("Expected ID evt-001, got %s", got.ID)
	}
	if got.Reason != "OOMKilled" {
		t.Errorf("Expected reason OOMKilled, got %s", got.Reason)
	}
	if got.Severity != engine.SeverityCritical {
		t.Errorf("Expected Critical severity, got %v", got.Severity)
	}
}

func TestStoreEventNamespaceFilter(t *testing.T) {
	s := testStore(t)

	s.SaveEvent(engine.Event{ID: "e1", Reason: "A", Namespace: "prod", LastSeen: time.Now()})
	s.SaveEvent(engine.Event{ID: "e2", Reason: "B", Namespace: "staging", LastSeen: time.Now()})

	events, _ := s.GetEvents(1*time.Hour, "prod", nil)
	if len(events) != 1 {
		t.Errorf("Expected 1 event for prod, got %d", len(events))
	}
}

func TestStoreEventSeverityFilter(t *testing.T) {
	s := testStore(t)

	s.SaveEvent(engine.Event{ID: "e1", Severity: engine.SeverityCritical, LastSeen: time.Now()})
	s.SaveEvent(engine.Event{ID: "e2", Severity: engine.SeverityInfo, LastSeen: time.Now()})

	crit := engine.SeverityCritical
	events, _ := s.GetEvents(1*time.Hour, "", &crit)
	if len(events) != 1 {
		t.Errorf("Expected 1 critical event, got %d", len(events))
	}
}

func TestStoreSaveAndGetIncidents(t *testing.T) {
	s := testStore(t)

	inc := engine.CorrelatedIncident{
		ID:        "INC-001",
		Title:     "Node cascade failure",
		RootCause: "Node memory exhaustion",
		Severity:  engine.SeverityCritical,
		Events: []engine.Event{
			{ID: "e1", Reason: "NodeNotReady"},
			{ID: "e2", Reason: "Evicted"},
		},
		Impact:     "3 pods affected",
		Suggestion: "Add more memory",
		StartTime:  time.Now().Add(-5 * time.Minute),
		EndTime:    time.Now(),
	}

	err := s.SaveIncident(inc)
	if err != nil {
		t.Fatalf("SaveIncident failed: %v", err)
	}

	incidents, err := s.GetIncidents(1 * time.Hour)
	if err != nil {
		t.Fatalf("GetIncidents failed: %v", err)
	}

	if len(incidents) != 1 {
		t.Fatalf("Expected 1 incident, got %d", len(incidents))
	}

	if incidents[0].ID != "INC-001" {
		t.Errorf("Expected INC-001, got %s", incidents[0].ID)
	}
	if len(incidents[0].Events) != 2 {
		t.Errorf("Expected 2 events in incident, got %d", len(incidents[0].Events))
	}
}

func TestStoreSaveAndGetAudit(t *testing.T) {
	s := testStore(t)

	sc := &engine.Scorecard{
		ClusterName:  "test-cluster",
		OverallScore: 85.5,
		Grade:        "A-",
		ScanTime:     time.Now(),
		Categories: []engine.CategoryScore{
			{Name: "Security", Score: 80, MaxScore: 100, Weight: 0.25},
		},
	}

	err := s.SaveAuditReport(sc)
	if err != nil {
		t.Fatalf("SaveAuditReport failed: %v", err)
	}

	latest, err := s.GetLatestAuditReport()
	if err != nil {
		t.Fatalf("GetLatestAuditReport failed: %v", err)
	}

	if latest.Grade != "A-" {
		t.Errorf("Expected grade A-, got %s", latest.Grade)
	}
	if latest.OverallScore != 85.5 {
		t.Errorf("Expected score 85.5, got %f", latest.OverallScore)
	}
}

func TestStoreAuditHistory(t *testing.T) {
	s := testStore(t)

	for i := 0; i < 3; i++ {
		s.SaveAuditReport(&engine.Scorecard{
			ClusterName:  "test",
			OverallScore: float64(80 + i),
			Grade:        "B+",
			ScanTime:     time.Now(),
		})
	}

	history, err := s.GetAuditHistory(1 * time.Hour)
	if err != nil {
		t.Fatalf("GetAuditHistory failed: %v", err)
	}

	if len(history) != 3 {
		t.Errorf("Expected 3 reports, got %d", len(history))
	}
}

func TestStoreStats(t *testing.T) {
	s := testStore(t)

	s.SaveEvent(engine.Event{ID: "e1", LastSeen: time.Now()})
	s.SaveEvent(engine.Event{ID: "e2", LastSeen: time.Now()})

	stats := s.Stats()
	if stats["events"] != 2 {
		t.Errorf("Expected 2 events in stats, got %d", stats["events"])
	}
}
