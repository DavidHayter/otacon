package enricher

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/merthan/otacon/internal/engine"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

// LogEnricher adds pod log context to notifications
type LogEnricher struct {
	client    kubernetes.Interface
	tailLines int64
}

// NewLogEnricher creates a new log enricher
func NewLogEnricher(client kubernetes.Interface, tailLines int64) *LogEnricher {
	if tailLines <= 0 {
		tailLines = 20
	}
	return &LogEnricher{client: client, tailLines: tailLines}
}

func (e *LogEnricher) Name() string { return "logs" }

func (e *LogEnricher) Enrich(ctx context.Context, event engine.Event) (*engine.Enrichment, error) {
	if event.ResourceKind != "Pod" || event.ResourceName == "" || event.Namespace == "" {
		return nil, nil
	}

	// Try current logs first, then previous
	logs := e.getLogs(ctx, event.Namespace, event.ResourceName, "", false)
	if logs == "" {
		logs = e.getLogs(ctx, event.Namespace, event.ResourceName, "", true)
	}

	if logs == "" {
		return nil, nil
	}

	return &engine.Enrichment{
		Type:    "logs",
		Title:   fmt.Sprintf("Last %d log lines", e.tailLines),
		Content: logs,
	}, nil
}

func (e *LogEnricher) getLogs(ctx context.Context, namespace, pod, container string, previous bool) string {
	opts := &corev1.PodLogOptions{
		TailLines: &e.tailLines,
		Previous:  previous,
	}
	if container != "" {
		opts.Container = container
	}

	req := e.client.CoreV1().Pods(namespace).GetLogs(pod, opts)
	stream, err := req.Stream(ctx)
	if err != nil {
		return ""
	}
	defer stream.Close()

	var buf bytes.Buffer
	io.Copy(&buf, io.LimitReader(stream, 4096))
	return strings.TrimSpace(buf.String())
}

// GrafanaEnricher adds Grafana dashboard links to notifications
type GrafanaEnricher struct {
	baseURL      string
	dashboardUID string
}

// NewGrafanaEnricher creates a new Grafana link enricher
func NewGrafanaEnricher(baseURL, dashboardUID string) *GrafanaEnricher {
	return &GrafanaEnricher{
		baseURL:      strings.TrimSuffix(baseURL, "/"),
		dashboardUID: dashboardUID,
	}
}

func (e *GrafanaEnricher) Name() string { return "grafana" }

func (e *GrafanaEnricher) Enrich(ctx context.Context, event engine.Event) (*engine.Enrichment, error) {
	if e.baseURL == "" || e.dashboardUID == "" {
		return nil, nil
	}

	// Build Grafana URL with time range (15min window around the event)
	from := event.LastSeen.Add(-15 * 60 * 1e9).UnixMilli() // 15min before
	to := event.LastSeen.Add(15 * 60 * 1e9).UnixMilli()    // 15min after

	link := fmt.Sprintf("%s/d/%s?var-namespace=%s&var-pod=%s&from=%d&to=%d",
		e.baseURL, e.dashboardUID,
		event.Namespace, event.ResourceName,
		from, to,
	)

	return &engine.Enrichment{
		Type:    "link",
		Title:   "Grafana Dashboard",
		Content: link,
	}, nil
}

// RunbookEnricher adds runbook links based on event reason
type RunbookEnricher struct {
	baseURL  string
	runbooks map[string]string // reason → runbook path
}

// NewRunbookEnricher creates a new runbook enricher
func NewRunbookEnricher(baseURL string) *RunbookEnricher {
	return &RunbookEnricher{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		runbooks: map[string]string{
			"OOMKilled":        "/runbooks/oom-killed",
			"CrashLoopBackOff": "/runbooks/crash-loop",
			"NodeNotReady":     "/runbooks/node-not-ready",
			"ImagePullBackOff": "/runbooks/image-pull-failure",
			"FailedScheduling": "/runbooks/failed-scheduling",
			"FailedMount":      "/runbooks/volume-mount-failure",
			"Evicted":          "/runbooks/pod-eviction",
			"Unhealthy":        "/runbooks/probe-failure",
		},
	}
}

func (e *RunbookEnricher) Name() string { return "runbook" }

func (e *RunbookEnricher) Enrich(ctx context.Context, event engine.Event) (*engine.Enrichment, error) {
	if e.baseURL == "" {
		return nil, nil
	}

	path, ok := e.runbooks[event.Reason]
	if !ok {
		return nil, nil
	}

	return &engine.Enrichment{
		Type:    "link",
		Title:   fmt.Sprintf("Runbook: %s", event.Reason),
		Content: e.baseURL + path,
	}, nil
}

// AddRunbook registers a custom runbook mapping
func (e *RunbookEnricher) AddRunbook(reason, path string) {
	e.runbooks[reason] = path
}
