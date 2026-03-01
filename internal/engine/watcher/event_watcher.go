package watcher

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/merthan/otacon/internal/engine"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
)

// EventHandler is called when an event is processed
type EventHandler func(event engine.Event)

// FilterConfig defines which events to capture
type FilterConfig struct {
	Namespaces       []string          // Empty = all
	ExcludeNamespaces []string          // kube-system etc.
	Severities       []engine.Severity // Empty = all
	Reasons          []string          // Empty = all
	ResourceKinds    []string          // Empty = all
}

// Watcher monitors Kubernetes events in real-time
type Watcher struct {
	client   kubernetes.Interface
	filter   FilterConfig
	handlers []EventHandler
	mu       sync.RWMutex
	stopCh   chan struct{}
	running  bool
}

// NewWatcher creates a new event watcher
func NewWatcher(client kubernetes.Interface, filter FilterConfig) *Watcher {
	// Default excluded namespaces
	if len(filter.ExcludeNamespaces) == 0 {
		filter.ExcludeNamespaces = []string{"kube-system", "kube-public", "kube-node-lease"}
	}

	return &Watcher{
		client: client,
		filter: filter,
		stopCh: make(chan struct{}),
	}
}

// OnEvent registers a handler for processed events
func (w *Watcher) OnEvent(handler EventHandler) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.handlers = append(w.handlers, handler)
}

// Start begins watching Kubernetes events
func (w *Watcher) Start(ctx context.Context) error {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return fmt.Errorf("watcher already running")
	}
	w.running = true
	w.mu.Unlock()

	log.Println("[watcher] Starting Kubernetes event watcher...")

	go w.watchLoop(ctx)
	return nil
}

// Stop halts the watcher
func (w *Watcher) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.running {
		close(w.stopCh)
		w.running = false
		log.Println("[watcher] Event watcher stopped")
	}
}

func (w *Watcher) watchLoop(ctx context.Context) {
	for {
		err := w.doWatch(ctx)
		if err != nil {
			log.Printf("[watcher] Watch error: %v, reconnecting in 5s...", err)
		}

		select {
		case <-ctx.Done():
			return
		case <-w.stopCh:
			return
		case <-time.After(5 * time.Second):
			// Reconnect
		}
	}
}

func (w *Watcher) doWatch(ctx context.Context) error {
	namespace := ""
	if len(w.filter.Namespaces) == 1 {
		namespace = w.filter.Namespaces[0]
	}

	watcher, err := w.client.CoreV1().Events(namespace).Watch(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to create watch: %w", err)
	}
	defer watcher.Stop()

	log.Printf("[watcher] Connected, watching events (namespace: %s)", displayNs(namespace))

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-w.stopCh:
			return nil
		case event, ok := <-watcher.ResultChan():
			if !ok {
				return fmt.Errorf("watch channel closed")
			}
			if event.Type == watch.Added || event.Type == watch.Modified {
				if k8sEvent, ok := event.Object.(*corev1.Event); ok {
					w.processEvent(k8sEvent)
				}
			}
		}
	}
}

func (w *Watcher) processEvent(k8sEvent *corev1.Event) {
	// Skip old events (only process events from last 5 minutes)
	eventTime := k8sEvent.LastTimestamp.Time
	if eventTime.IsZero() {
		eventTime = k8sEvent.EventTime.Time
	}
	if eventTime.IsZero() {
		eventTime = time.Now()
	}
	if time.Since(eventTime) > 5*time.Minute {
		return
	}

	// Apply filters
	if !w.shouldProcess(k8sEvent) {
		return
	}

	// Classify severity
	severity := ClassifySeverity(k8sEvent.Reason, k8sEvent.Type, k8sEvent.Message)

	// Severity filter
	if len(w.filter.Severities) > 0 {
		found := false
		for _, s := range w.filter.Severities {
			if s == severity {
				found = true
				break
			}
		}
		if !found {
			return
		}
	}

	// Build Otacon event
	otaconEvent := engine.Event{
		ID:            fmt.Sprintf("%s-%s-%s-%d", k8sEvent.Namespace, k8sEvent.InvolvedObject.Name, k8sEvent.Reason, k8sEvent.Count),
		Type:          k8sEvent.Type,
		Reason:        k8sEvent.Reason,
		Message:       k8sEvent.Message,
		Namespace:     k8sEvent.Namespace,
		ResourceKind:  k8sEvent.InvolvedObject.Kind,
		ResourceName:  k8sEvent.InvolvedObject.Name,
		NodeName:      k8sEvent.Source.Host,
		Severity:      severity,
		Count:         k8sEvent.Count,
		FirstSeen:     k8sEvent.FirstTimestamp.Time,
		LastSeen:      eventTime,
	}

	// Dispatch to all handlers
	w.mu.RLock()
	handlers := make([]EventHandler, len(w.handlers))
	copy(handlers, w.handlers)
	w.mu.RUnlock()

	for _, handler := range handlers {
		handler(otaconEvent)
	}
}

func (w *Watcher) shouldProcess(event *corev1.Event) bool {
	// Check namespace exclusion
	for _, ns := range w.filter.ExcludeNamespaces {
		if event.Namespace == ns {
			return false
		}
	}

	// Check namespace inclusion
	if len(w.filter.Namespaces) > 0 {
		found := false
		for _, ns := range w.filter.Namespaces {
			if event.Namespace == ns || matchWildcard(ns, event.Namespace) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check reason filter
	if len(w.filter.Reasons) > 0 {
		found := false
		for _, r := range w.filter.Reasons {
			if event.Reason == r {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check resource kind filter
	if len(w.filter.ResourceKinds) > 0 {
		found := false
		for _, k := range w.filter.ResourceKinds {
			if event.InvolvedObject.Kind == k {
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

// ClassifySeverity determines event severity from reason and type
func ClassifySeverity(reason, eventType, message string) engine.Severity {
	criticalReasons := map[string]bool{
		"OOMKilled": true, "CrashLoopBackOff": true, "NodeNotReady": true,
		"EvictionThresholdMet": true, "SystemOOM": true, "ContainerGCFailed": true,
		"FreeDiskSpaceFailed": true, "NetworkNotReady": true, "NodeHasInsufficientMemory": true,
		"NodeHasInsufficientPID": true, "NodeHasDiskPressure": true,
		"FailedAttachVolume": true, "FailedMount": true,
	}
	warningReasons := map[string]bool{
		"BackOff": true, "Unhealthy": true, "FailedScheduling": true,
		"FailedCreate": true, "FailedDelete": true, "ImagePullBackOff": true,
		"ErrImagePull": true, "Evicted": true, "Preempting": true,
		"FailedValidation": true, "FailedPostStartHook": true,
		"FailedPreStopHook": true, "ProbeWarning": true,
		"DNSConfigForming": true,
	}

	if criticalReasons[reason] {
		return engine.SeverityCritical
	}
	if warningReasons[reason] || eventType == "Warning" {
		return engine.SeverityWarning
	}

	// Check message patterns for additional classification
	lowerMsg := strings.ToLower(message)
	if strings.Contains(lowerMsg, "oom") || strings.Contains(lowerMsg, "out of memory") ||
		strings.Contains(lowerMsg, "crash") || strings.Contains(lowerMsg, "fatal") {
		return engine.SeverityCritical
	}
	if strings.Contains(lowerMsg, "error") || strings.Contains(lowerMsg, "failed") ||
		strings.Contains(lowerMsg, "timeout") || strings.Contains(lowerMsg, "exceeded") {
		return engine.SeverityWarning
	}

	return engine.SeverityInfo
}

func matchWildcard(pattern, s string) bool {
	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(s, prefix)
	}
	return pattern == s
}

func displayNs(ns string) string {
	if ns == "" {
		return "all namespaces"
	}
	return ns
}
