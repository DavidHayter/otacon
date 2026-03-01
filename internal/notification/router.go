package notification

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/merthan/otacon/internal/engine"
	"github.com/merthan/otacon/internal/engine/intelligence"
)

// Channel represents a notification delivery channel
type Channel interface {
	Name() string
	Send(ctx context.Context, payload engine.NotificationPayload) error
	SendDigest(ctx context.Context, digest *intelligence.Digest) error
}

// Enricher adds additional context to notifications
type Enricher interface {
	Name() string
	Enrich(ctx context.Context, event engine.Event) (*engine.Enrichment, error)
}

// RoutingRule defines when and where to send notifications
type RoutingRule struct {
	Name           string
	Namespaces     []string          // Match namespaces (empty = all)
	Severities     []engine.Severity // Match severities (empty = all)
	EventReasons   []string          // Match specific reasons (empty = all)
	Channel        string            // Target channel name
	Target         string            // Channel-specific target (e.g., Slack channel)
	Enrichers      []string          // Enricher names to apply
}

// Router manages notification routing and delivery
type Router struct {
	channels   map[string]Channel
	enrichers  map[string]Enricher
	rules      []RoutingRule
	cooldown   *intelligence.CooldownManager
	mu         sync.RWMutex
	stats      RouterStats
}

// RouterStats tracks notification statistics
type RouterStats struct {
	TotalReceived   int64
	TotalRouted     int64
	TotalSuppressed int64
	TotalErrors     int64
	ByChannel       map[string]int64
}

// NewRouter creates a new notification router
func NewRouter(cooldown *intelligence.CooldownManager) *Router {
	return &Router{
		channels:  make(map[string]Channel),
		enrichers: make(map[string]Enricher),
		cooldown:  cooldown,
		stats: RouterStats{
			ByChannel: make(map[string]int64),
		},
	}
}

// RegisterChannel adds a notification channel
func (r *Router) RegisterChannel(channel Channel) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.channels[channel.Name()] = channel
	log.Printf("[router] Registered channel: %s", channel.Name())
}

// RegisterEnricher adds an enrichment plugin
func (r *Router) RegisterEnricher(enricher Enricher) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.enrichers[enricher.Name()] = enricher
	log.Printf("[router] Registered enricher: %s", enricher.Name())
}

// AddRule adds a routing rule
func (r *Router) AddRule(rule RoutingRule) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rules = append(r.rules, rule)
	log.Printf("[router] Added rule: %s → %s/%s", rule.Name, rule.Channel, rule.Target)
}

// RouteEvent processes a single event through routing rules
func (r *Router) RouteEvent(ctx context.Context, event engine.Event) {
	r.mu.RLock()
	rules := make([]RoutingRule, len(r.rules))
	copy(rules, r.rules)
	r.mu.RUnlock()

	r.stats.TotalReceived++

	for _, rule := range rules {
		if !r.matchesRule(event, rule) {
			continue
		}

		// Check cooldown
		cooldownKey := fmt.Sprintf("%s:%s:%s", rule.Channel, event.Namespace, event.Reason)
		if r.cooldown != nil && !r.cooldown.ShouldNotify(cooldownKey, event.Severity.String()) {
			r.stats.TotalSuppressed++
			continue
		}

		// Build payload
		payload := engine.NotificationPayload{
			Title:     fmt.Sprintf("%s %s in %s/%s", event.Severity.Icon(), event.Reason, event.Namespace, event.ResourceName),
			Severity:  event.Severity,
			Body:      event.Message,
			Timestamp: event.LastSeen,
			Fields: map[string]string{
				"Namespace": event.Namespace,
				"Resource":  fmt.Sprintf("%s/%s", event.ResourceKind, event.ResourceName),
				"Reason":    event.Reason,
				"Node":      event.NodeName,
			},
		}

		// Apply enrichers
		for _, enricherName := range rule.Enrichers {
			if enricher, ok := r.enrichers[enricherName]; ok {
				enrichment, err := enricher.Enrich(ctx, event)
				if err == nil && enrichment != nil {
					payload.Enrichments = append(payload.Enrichments, *enrichment)
				}
			}
		}

		// Send to channel
		r.mu.RLock()
		channel, ok := r.channels[rule.Channel]
		r.mu.RUnlock()

		if !ok {
			log.Printf("[router] Channel not found: %s", rule.Channel)
			continue
		}

		go func(ch Channel, p engine.NotificationPayload) {
			if err := ch.Send(ctx, p); err != nil {
				log.Printf("[router] Failed to send to %s: %v", ch.Name(), err)
				r.mu.Lock()
				r.stats.TotalErrors++
				r.mu.Unlock()
			} else {
				r.mu.Lock()
				r.stats.TotalRouted++
				r.stats.ByChannel[ch.Name()]++
				r.mu.Unlock()
			}
		}(channel, payload)
	}
}

// RouteIncident processes a correlated incident through routing rules
func (r *Router) RouteIncident(ctx context.Context, incident engine.CorrelatedIncident) {
	payload := engine.NotificationPayload{
		Title:     fmt.Sprintf("%s Otacon Intelligence Report", incident.Severity.Icon()),
		Severity:  incident.Severity,
		Body:      formatIncidentBody(incident),
		Timestamp: incident.EndTime,
		Fields: map[string]string{
			"Incident":   incident.ID,
			"Root Cause": incident.RootCause,
			"Impact":     incident.Impact,
			"Suggestion": incident.Suggestion,
		},
	}

	// Send to all channels matching the severity
	r.mu.RLock()
	rules := make([]RoutingRule, len(r.rules))
	copy(rules, r.rules)
	r.mu.RUnlock()

	for _, rule := range rules {
		if !r.matchesSeverity(incident.Severity, rule.Severities) {
			continue
		}

		cooldownKey := fmt.Sprintf("incident:%s:%s", rule.Channel, incident.ID)
		if r.cooldown != nil && !r.cooldown.ShouldNotify(cooldownKey, incident.Severity.String()) {
			continue
		}

		r.mu.RLock()
		channel, ok := r.channels[rule.Channel]
		r.mu.RUnlock()

		if !ok {
			continue
		}

		go func(ch Channel, p engine.NotificationPayload) {
			if err := ch.Send(ctx, p); err != nil {
				log.Printf("[router] Failed to send incident to %s: %v", ch.Name(), err)
			}
		}(channel, payload)
	}
}

// RouteDigest sends a digest to configured channels
func (r *Router) RouteDigest(ctx context.Context, digest *intelligence.Digest, channelNames []string) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, name := range channelNames {
		if channel, ok := r.channels[name]; ok {
			go func(ch Channel) {
				if err := ch.SendDigest(ctx, digest); err != nil {
					log.Printf("[router] Failed to send digest to %s: %v", ch.Name(), err)
				} else {
					log.Printf("[router] Digest %s sent to %s", digest.ID, ch.Name())
				}
			}(channel)
		}
	}
}

// GetStats returns routing statistics
func (r *Router) GetStats() RouterStats {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.stats
}

func (r *Router) matchesRule(event engine.Event, rule RoutingRule) bool {
	// Check namespace
	if len(rule.Namespaces) > 0 {
		found := false
		for _, ns := range rule.Namespaces {
			if event.Namespace == ns {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check severity
	if !r.matchesSeverity(event.Severity, rule.Severities) {
		return false
	}

	// Check reasons
	if len(rule.EventReasons) > 0 {
		found := false
		for _, reason := range rule.EventReasons {
			if event.Reason == reason {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

func (r *Router) matchesSeverity(severity engine.Severity, severities []engine.Severity) bool {
	if len(severities) == 0 {
		return true
	}
	for _, s := range severities {
		if severity == s {
			return true
		}
	}
	return false
}

func formatIncidentBody(incident engine.CorrelatedIncident) string {
	body := fmt.Sprintf("*Incident:* %s\n", incident.Title)
	body += fmt.Sprintf("*Root Cause:* %s\n", incident.RootCause)
	body += fmt.Sprintf("*Impact:* %s\n", incident.Impact)
	body += fmt.Sprintf("*Events:* %d correlated\n", len(incident.Events))
	body += fmt.Sprintf("*Duration:* %s\n", incident.EndTime.Sub(incident.StartTime).Round(time.Second))
	body += fmt.Sprintf("*Suggestion:* %s", incident.Suggestion)
	return body
}
