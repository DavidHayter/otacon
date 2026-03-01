package notification

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/merthan/otacon/internal/engine"
	"github.com/merthan/otacon/internal/engine/intelligence"
)

// MockChannel records sent notifications for testing
type MockChannel struct {
	name     string
	mu       sync.Mutex
	sent     []engine.NotificationPayload
	digests  int
	failNext bool
}

func NewMockChannel(name string) *MockChannel {
	return &MockChannel{name: name}
}

func (m *MockChannel) Name() string { return m.name }

func (m *MockChannel) Send(ctx context.Context, payload engine.NotificationPayload) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failNext {
		m.failNext = false
		return context.DeadlineExceeded
	}
	m.sent = append(m.sent, payload)
	return nil
}

func (m *MockChannel) SendDigest(ctx context.Context, digest *intelligence.Digest) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.digests++
	return nil
}

func (m *MockChannel) SentCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.sent)
}

func TestRouterRouteEvent(t *testing.T) {
	cooldown := intelligence.NewCooldownManager(intelligence.CooldownConfig{
		DefaultDuration: 1 * time.Millisecond,
		PerSeverity:     map[string]time.Duration{},
		PerGroup:        true,
		MaxSuppressed:   100,
	})
	router := NewRouter(cooldown)

	mock := NewMockChannel("test-slack")
	router.RegisterChannel(mock)

	router.AddRule(RoutingRule{
		Name:       "prod-critical",
		Namespaces: []string{"production"},
		Severities: []engine.Severity{engine.SeverityCritical},
		Channel:    "test-slack",
		Target:     "#alerts",
	})

	ctx := context.Background()

	// Event matching the rule
	router.RouteEvent(ctx, engine.Event{
		Reason:    "OOMKilled",
		Namespace: "production",
		Severity:  engine.SeverityCritical,
		LastSeen:  time.Now(),
	})

	time.Sleep(50 * time.Millisecond) // async dispatch

	if mock.SentCount() != 1 {
		t.Errorf("Expected 1 notification sent, got %d", mock.SentCount())
	}

	// Event NOT matching (wrong namespace)
	router.RouteEvent(ctx, engine.Event{
		Reason:    "OOMKilled",
		Namespace: "staging",
		Severity:  engine.SeverityCritical,
		LastSeen:  time.Now(),
	})

	time.Sleep(50 * time.Millisecond)

	if mock.SentCount() != 1 {
		t.Errorf("Expected still 1 notification (staging not matched), got %d", mock.SentCount())
	}
}

func TestRouterSeverityFilter(t *testing.T) {
	cooldown := intelligence.NewCooldownManager(intelligence.CooldownConfig{
		DefaultDuration: 1 * time.Millisecond,
		PerSeverity:     map[string]time.Duration{},
		PerGroup:        true,
		MaxSuppressed:   100,
	})
	router := NewRouter(cooldown)

	mock := NewMockChannel("slack")
	router.RegisterChannel(mock)

	// Only critical
	router.AddRule(RoutingRule{
		Name:       "critical-only",
		Severities: []engine.Severity{engine.SeverityCritical},
		Channel:    "slack",
	})

	ctx := context.Background()

	// Warning event — should NOT be routed
	router.RouteEvent(ctx, engine.Event{
		Reason: "Unhealthy", Namespace: "default",
		Severity: engine.SeverityWarning, LastSeen: time.Now(),
	})

	time.Sleep(50 * time.Millisecond)

	if mock.SentCount() != 0 {
		t.Errorf("Warning event should not match critical-only rule, got %d sent", mock.SentCount())
	}

	// Critical event — should be routed
	router.RouteEvent(ctx, engine.Event{
		Reason: "OOMKilled", Namespace: "default",
		Severity: engine.SeverityCritical, LastSeen: time.Now(),
	})

	time.Sleep(50 * time.Millisecond)

	if mock.SentCount() != 1 {
		t.Errorf("Critical event should match, got %d sent", mock.SentCount())
	}
}

func TestRouterAllSeverities(t *testing.T) {
	cooldown := intelligence.NewCooldownManager(intelligence.CooldownConfig{
		DefaultDuration: 1 * time.Millisecond,
		PerSeverity:     map[string]time.Duration{},
		PerGroup:        true,
		MaxSuppressed:   100,
	})
	router := NewRouter(cooldown)

	mock := NewMockChannel("slack")
	router.RegisterChannel(mock)

	// Empty severities = match all
	router.AddRule(RoutingRule{
		Name:    "catch-all",
		Channel: "slack",
	})

	ctx := context.Background()

	router.RouteEvent(ctx, engine.Event{
		Reason: "Pulled", Namespace: "default",
		Severity: engine.SeverityInfo, LastSeen: time.Now(),
	})

	time.Sleep(50 * time.Millisecond)

	if mock.SentCount() != 1 {
		t.Errorf("Info event should match catch-all rule, got %d sent", mock.SentCount())
	}
}

func TestRouterDigest(t *testing.T) {
	router := NewRouter(nil)

	mock := NewMockChannel("slack")
	router.RegisterChannel(mock)

	digest := &intelligence.Digest{
		ID:   "DIG-001",
		Type: intelligence.DigestDaily,
	}

	router.RouteDigest(context.Background(), digest, []string{"slack"})

	time.Sleep(50 * time.Millisecond)

	if mock.digests != 1 {
		t.Errorf("Expected 1 digest sent, got %d", mock.digests)
	}
}
