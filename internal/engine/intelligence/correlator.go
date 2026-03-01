package intelligence

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/merthan/otacon/internal/engine"
)

// CorrelationRule defines how events are related
type CorrelationRule struct {
	Name          string
	Description   string
	TriggerReason []string        // Reasons that start a correlation
	RelatedReasons []string       // Reasons that get grouped under the trigger
	TimeWindow    time.Duration   // Max time gap between trigger and related
	SameNode      bool            // Must be on same node
	SameNamespace bool            // Must be in same namespace
	Impact        func(trigger engine.Event, related []engine.Event) string
	Suggestion    func(trigger engine.Event, related []engine.Event) string
}

// Correlator groups related events into incidents
type Correlator struct {
	rules       []CorrelationRule
	buffer      []engine.Event          // Recent events buffer
	incidents   []engine.CorrelatedIncident
	handlers    []func(engine.CorrelatedIncident)
	bufferSize  int
	mu          sync.Mutex
	incidentSeq int
}

// NewCorrelator creates a new correlation engine with built-in rules
func NewCorrelator() *Correlator {
	return &Correlator{
		rules:      BuiltinCorrelationRules(),
		bufferSize: 5000,
	}
}

// OnIncident registers a handler for correlated incidents
func (c *Correlator) OnIncident(handler func(engine.CorrelatedIncident)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.handlers = append(c.handlers, handler)
}

// Ingest processes a new event and checks for correlations
func (c *Correlator) Ingest(event engine.Event) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Add to buffer
	c.buffer = append(c.buffer, event)

	// Trim buffer if too large
	if len(c.buffer) > c.bufferSize {
		c.buffer = c.buffer[len(c.buffer)-c.bufferSize:]
	}

	// Check if this event triggers any correlation rule
	for _, rule := range c.rules {
		if c.matchesTrigger(event, rule) {
			incident := c.buildIncident(event, rule)
			if incident != nil && len(incident.Events) > 1 {
				c.incidents = append(c.incidents, *incident)
				log.Printf("[correlator] Incident detected: %s (%d events)", incident.Title, len(incident.Events))

				// Dispatch to handlers
				for _, handler := range c.handlers {
					go handler(*incident)
				}
			}
		}
	}
}

// GetRecentIncidents returns recent correlated incidents
func (c *Correlator) GetRecentIncidents(since time.Duration) []engine.CorrelatedIncident {
	c.mu.Lock()
	defer c.mu.Unlock()

	cutoff := time.Now().Add(-since)
	var result []engine.CorrelatedIncident
	for _, inc := range c.incidents {
		if inc.StartTime.After(cutoff) {
			result = append(result, inc)
		}
	}
	return result
}

// GetBufferedEvents returns events from the buffer
func (c *Correlator) GetBufferedEvents(since time.Duration) []engine.Event {
	c.mu.Lock()
	defer c.mu.Unlock()

	cutoff := time.Now().Add(-since)
	var result []engine.Event
	for _, e := range c.buffer {
		if e.LastSeen.After(cutoff) {
			result = append(result, e)
		}
	}
	return result
}

func (c *Correlator) matchesTrigger(event engine.Event, rule CorrelationRule) bool {
	for _, reason := range rule.TriggerReason {
		if event.Reason == reason {
			return true
		}
	}
	return false
}

func (c *Correlator) buildIncident(trigger engine.Event, rule CorrelationRule) *engine.CorrelatedIncident {
	var related []engine.Event
	earliest := trigger.LastSeen
	latest := trigger.LastSeen

	for _, event := range c.buffer {
		if event.ID == trigger.ID {
			continue
		}

		// Check time window
		timeDiff := trigger.LastSeen.Sub(event.LastSeen)
		if timeDiff < 0 {
			timeDiff = -timeDiff
		}
		if timeDiff > rule.TimeWindow {
			continue
		}

		// Check same node constraint
		if rule.SameNode && trigger.NodeName != "" && event.NodeName != trigger.NodeName {
			continue
		}

		// Check same namespace constraint
		if rule.SameNamespace && trigger.Namespace != event.Namespace {
			continue
		}

		// Check if event reason matches any related reason
		isRelated := false
		for _, reason := range rule.RelatedReasons {
			if event.Reason == reason {
				isRelated = true
				break
			}
		}

		if isRelated {
			related = append(related, event)
			if event.LastSeen.Before(earliest) {
				earliest = event.LastSeen
			}
			if event.LastSeen.After(latest) {
				latest = event.LastSeen
			}
		}
	}

	if len(related) == 0 {
		return nil
	}

	c.incidentSeq++
	allEvents := append([]engine.Event{trigger}, related...)

	impact := fmt.Sprintf("%d related events detected", len(related))
	if rule.Impact != nil {
		impact = rule.Impact(trigger, related)
	}

	suggestion := "Investigate the root cause event"
	if rule.Suggestion != nil {
		suggestion = rule.Suggestion(trigger, related)
	}

	return &engine.CorrelatedIncident{
		ID:         fmt.Sprintf("INC-%06d", c.incidentSeq),
		Title:      fmt.Sprintf("%s: %s", rule.Name, trigger.Message),
		RootCause:  rule.Description,
		Severity:   trigger.Severity,
		Events:     allEvents,
		Impact:     impact,
		Suggestion: suggestion,
		StartTime:  earliest,
		EndTime:    latest,
	}
}

// BuiltinCorrelationRules returns all built-in correlation rules
func BuiltinCorrelationRules() []CorrelationRule {
	return []CorrelationRule{
		{
			Name:           "Node Cascade Failure",
			Description:    "Node failure causing cascading pod evictions and rescheduling",
			TriggerReason:  []string{"NodeNotReady", "NodeHasInsufficientMemory", "NodeHasDiskPressure"},
			RelatedReasons: []string{"Evicted", "NodeLost", "FailedScheduling", "Killing", "Preempting", "Rescheduled"},
			TimeWindow:     10 * time.Minute,
			SameNode:       true,
			Impact: func(trigger engine.Event, related []engine.Event) string {
				podCount := 0
				namespaces := make(map[string]bool)
				for _, e := range related {
					if e.ResourceKind == "Pod" {
						podCount++
						namespaces[e.Namespace] = true
					}
				}
				return fmt.Sprintf("%d pods affected across %d namespaces on node %s",
					podCount, len(namespaces), trigger.NodeName)
			},
			Suggestion: func(trigger engine.Event, related []engine.Event) string {
				if strings.Contains(trigger.Reason, "Memory") {
					return "Check node memory pressure: kubectl describe node " + trigger.NodeName + " | grep -A5 Conditions. Consider adding more nodes or setting memory limits."
				}
				if strings.Contains(trigger.Reason, "Disk") {
					return "Check disk usage on node " + trigger.NodeName + ": ssh to node and run df -h. Clean up container images: crictl rmi --prune"
				}
				return "Check node status: kubectl describe node " + trigger.NodeName
			},
		},
		{
			Name:           "OOM Cascade",
			Description:    "Out-of-memory kills triggering crash loops and restarts",
			TriggerReason:  []string{"OOMKilled", "SystemOOM"},
			RelatedReasons: []string{"CrashLoopBackOff", "BackOff", "Killing", "Started", "Created"},
			TimeWindow:     15 * time.Minute,
			SameNamespace:  true,
			Impact: func(trigger engine.Event, related []engine.Event) string {
				crashCount := 0
				for _, e := range related {
					if e.Reason == "CrashLoopBackOff" {
						crashCount++
					}
				}
				return fmt.Sprintf("OOM in %s/%s triggered %d crash loops",
					trigger.Namespace, trigger.ResourceName, crashCount)
			},
			Suggestion: func(trigger engine.Event, related []engine.Event) string {
				return fmt.Sprintf("Increase memory limits for %s/%s. Check current usage: kubectl top pod -n %s %s",
					trigger.Namespace, trigger.ResourceName, trigger.Namespace, trigger.ResourceName)
			},
		},
		{
			Name:           "Image Pull Failure Chain",
			Description:    "Image pull failures causing pod scheduling failures",
			TriggerReason:  []string{"ErrImagePull", "ImagePullBackOff"},
			RelatedReasons: []string{"BackOff", "FailedCreate", "FailedScheduling", "Failed"},
			TimeWindow:     10 * time.Minute,
			SameNamespace:  true,
			Impact: func(trigger engine.Event, related []engine.Event) string {
				return fmt.Sprintf("Image pull failure for %s/%s blocking %d dependent operations",
					trigger.Namespace, trigger.ResourceName, len(related))
			},
			Suggestion: func(trigger engine.Event, related []engine.Event) string {
				return "Verify image name and tag, check registry credentials (imagePullSecrets), and network connectivity to registry"
			},
		},
		{
			Name:           "Volume Mount Failure Chain",
			Description:    "PVC/Volume mount failures preventing pod startup",
			TriggerReason:  []string{"FailedMount", "FailedAttachVolume"},
			RelatedReasons: []string{"FailedScheduling", "BackOff", "ContainerCreating"},
			TimeWindow:     10 * time.Minute,
			SameNamespace:  true,
			Impact: func(trigger engine.Event, related []engine.Event) string {
				return fmt.Sprintf("Volume mount failure for %s/%s — %d pods waiting",
					trigger.Namespace, trigger.ResourceName, len(related))
			},
			Suggestion: func(trigger engine.Event, related []engine.Event) string {
				return "Check PVC status: kubectl get pvc -n " + trigger.Namespace + ". Verify storage class and provisioner health."
			},
		},
		{
			Name:           "Scheduling Pressure",
			Description:    "Multiple pods failing to schedule — possible resource exhaustion",
			TriggerReason:  []string{"FailedScheduling"},
			RelatedReasons: []string{"FailedScheduling", "FailedCreate", "Pending"},
			TimeWindow:     5 * time.Minute,
			SameNamespace:  false,
			Impact: func(trigger engine.Event, related []engine.Event) string {
				uniquePods := make(map[string]bool)
				uniquePods[trigger.ResourceName] = true
				for _, e := range related {
					uniquePods[e.ResourceName] = true
				}
				return fmt.Sprintf("%d pods unable to schedule — cluster may need scaling", len(uniquePods))
			},
			Suggestion: func(trigger engine.Event, related []engine.Event) string {
				return "Check cluster capacity: kubectl describe nodes | grep -A5 'Allocated resources'. Consider adding nodes or adjusting resource requests."
			},
		},
		{
			Name:           "DNS Resolution Failure",
			Description:    "DNS issues causing service communication failures",
			TriggerReason:  []string{"DNSConfigForming"},
			RelatedReasons: []string{"Unhealthy", "BackOff", "FailedCreate"},
			TimeWindow:     5 * time.Minute,
			SameNamespace:  true,
			Impact: func(trigger engine.Event, related []engine.Event) string {
				return fmt.Sprintf("DNS issue in %s affecting %d workloads", trigger.Namespace, len(related)+1)
			},
			Suggestion: func(trigger engine.Event, related []engine.Event) string {
				return "Check CoreDNS health: kubectl -n kube-system get pods -l k8s-app=kube-dns. Verify DNS config: kubectl exec <pod> -- nslookup kubernetes.default"
			},
		},
		{
			Name:           "Probe Failure Cascade",
			Description:    "Health probe failures causing restarts and service disruption",
			TriggerReason:  []string{"Unhealthy"},
			RelatedReasons: []string{"Killing", "BackOff", "CrashLoopBackOff", "Started"},
			TimeWindow:     10 * time.Minute,
			SameNamespace:  true,
			Impact: func(trigger engine.Event, related []engine.Event) string {
				restarts := 0
				for _, e := range related {
					if e.Reason == "Killing" || e.Reason == "CrashLoopBackOff" {
						restarts++
					}
				}
				return fmt.Sprintf("Probe failures in %s/%s caused %d restarts",
					trigger.Namespace, trigger.ResourceName, restarts)
			},
			Suggestion: func(trigger engine.Event, related []engine.Event) string {
				return fmt.Sprintf("Review probe configuration for %s/%s. Increase initialDelaySeconds, consider adding startupProbe for slow-starting containers.",
					trigger.Namespace, trigger.ResourceName)
			},
		},
	}
}
