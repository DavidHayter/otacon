package api

import (
	"fmt"
	"net/http"
	"sync/atomic"
	"time"
)

// Metrics holds Otacon runtime metrics for Prometheus scraping
type Metrics struct {
	EventsTotal      atomic.Int64
	EventsCritical   atomic.Int64
	EventsWarning    atomic.Int64
	EventsInfo       atomic.Int64
	IncidentsTotal   atomic.Int64
	NotifsSent       atomic.Int64
	NotifsSuppressed atomic.Int64
	NotifsErrors     atomic.Int64
	AuditScore       atomic.Int64 // *100 for precision (8542 = 85.42)
	AuditGrade       atomic.Value // string
	UptimeStart      time.Time
}

// GlobalMetrics is the singleton metrics instance
var GlobalMetrics = &Metrics{
	UptimeStart: time.Now(),
}

func init() {
	GlobalMetrics.AuditGrade.Store("N/A")
}

// MetricsHandler serves Prometheus-compatible /metrics endpoint
func MetricsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		m := GlobalMetrics
		uptime := time.Since(m.UptimeStart).Seconds()
		grade, _ := m.AuditGrade.Load().(string)
		score := float64(m.AuditScore.Load()) / 100.0

		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")

		fmt.Fprintf(w, "# HELP otacon_events_total Total events processed\n")
		fmt.Fprintf(w, "# TYPE otacon_events_total counter\n")
		fmt.Fprintf(w, "otacon_events_total %d\n", m.EventsTotal.Load())

		fmt.Fprintf(w, "# HELP otacon_events_by_severity Events by severity level\n")
		fmt.Fprintf(w, "# TYPE otacon_events_by_severity counter\n")
		fmt.Fprintf(w, "otacon_events_by_severity{severity=\"critical\"} %d\n", m.EventsCritical.Load())
		fmt.Fprintf(w, "otacon_events_by_severity{severity=\"warning\"} %d\n", m.EventsWarning.Load())
		fmt.Fprintf(w, "otacon_events_by_severity{severity=\"info\"} %d\n", m.EventsInfo.Load())

		fmt.Fprintf(w, "# HELP otacon_incidents_total Total correlated incidents\n")
		fmt.Fprintf(w, "# TYPE otacon_incidents_total counter\n")
		fmt.Fprintf(w, "otacon_incidents_total %d\n", m.IncidentsTotal.Load())

		fmt.Fprintf(w, "# HELP otacon_notifications_sent_total Notifications delivered\n")
		fmt.Fprintf(w, "# TYPE otacon_notifications_sent_total counter\n")
		fmt.Fprintf(w, "otacon_notifications_sent_total %d\n", m.NotifsSent.Load())

		fmt.Fprintf(w, "# HELP otacon_notifications_suppressed_total Notifications suppressed by cooldown\n")
		fmt.Fprintf(w, "# TYPE otacon_notifications_suppressed_total counter\n")
		fmt.Fprintf(w, "otacon_notifications_suppressed_total %d\n", m.NotifsSuppressed.Load())

		fmt.Fprintf(w, "# HELP otacon_notifications_errors_total Notification delivery failures\n")
		fmt.Fprintf(w, "# TYPE otacon_notifications_errors_total counter\n")
		fmt.Fprintf(w, "otacon_notifications_errors_total %d\n", m.NotifsErrors.Load())

		fmt.Fprintf(w, "# HELP otacon_audit_score Latest audit score (0-100)\n")
		fmt.Fprintf(w, "# TYPE otacon_audit_score gauge\n")
		fmt.Fprintf(w, "otacon_audit_score %.2f\n", score)

		fmt.Fprintf(w, "# HELP otacon_audit_grade Latest audit letter grade\n")
		fmt.Fprintf(w, "# TYPE otacon_audit_grade gauge\n")
		fmt.Fprintf(w, "otacon_audit_grade{grade=%q} 1\n", grade)

		fmt.Fprintf(w, "# HELP otacon_uptime_seconds Seconds since guardian started\n")
		fmt.Fprintf(w, "# TYPE otacon_uptime_seconds gauge\n")
		fmt.Fprintf(w, "otacon_uptime_seconds %.0f\n", uptime)

		fmt.Fprintf(w, "# HELP otacon_info Otacon version info\n")
		fmt.Fprintf(w, "# TYPE otacon_info gauge\n")
		fmt.Fprintf(w, "otacon_info{version=\"0.1.0\"} 1\n")
	}
}
