package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/merthan/otacon/internal/engine"
	"github.com/merthan/otacon/internal/engine/intelligence"
	"github.com/merthan/otacon/internal/notification"
	"github.com/merthan/otacon/internal/store"
)

// Server is the HTTP API server
type Server struct {
	store      *store.Store
	correlator *intelligence.Correlator
	dedup      *intelligence.Deduplicator
	cooldown   *intelligence.CooldownManager
	router     *notification.Router
	httpServer *http.Server
}

// ServerConfig configures the API server
type ServerConfig struct {
	Port       int
	Store      *store.Store
	Correlator *intelligence.Correlator
	Dedup      *intelligence.Deduplicator
	Cooldown   *intelligence.CooldownManager
	Router     *notification.Router
}

// NewServer creates a new API server
func NewServer(config ServerConfig) *Server {
	s := &Server{
		store:      config.Store,
		correlator: config.Correlator,
		dedup:      config.Dedup,
		cooldown:   config.Cooldown,
		router:     config.Router,
	}

	mux := http.NewServeMux()

	// Health
	mux.HandleFunc("/healthz", s.handleHealthz)
	mux.HandleFunc("/readyz", s.handleReadyz)
	mux.HandleFunc("/metrics", MetricsHandler())

	// API v1
	mux.HandleFunc("/api/v1/events", s.handleEvents)
	mux.HandleFunc("/api/v1/incidents", s.handleIncidents)
	mux.HandleFunc("/api/v1/audit/reports", s.handleAuditReports)
	mux.HandleFunc("/api/v1/audit/reports/latest", s.handleLatestAuditReport)
	mux.HandleFunc("/api/v1/audit/history", s.handleAuditHistory)
	mux.HandleFunc("/api/v1/dedup/groups", s.handleDedupGroups)
	mux.HandleFunc("/api/v1/stats", s.handleStats)
	mux.HandleFunc("/api/v1/status", s.handleStatus)
	mux.HandleFunc("/api/v1/alertmanager/webhook", s.handleAlertmanagerWebhook)

	// UI — serve embedded React app for all non-API routes
	uiHandler := UIHandler()
	mux.Handle("/", uiHandler)

	handler := corsMiddleware(mux)

	s.httpServer = &http.Server{
		Addr:         fmt.Sprintf(":%d", config.Port),
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return s
}

// Start begins listening
func (s *Server) Start() error {
	log.Printf("[api] Server listening on %s", s.httpServer.Addr)
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully stops the server
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

// ---- Handlers ----

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleReadyz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	since := parseDuration(r.URL.Query().Get("since"), 1*time.Hour)
	namespace := r.URL.Query().Get("namespace")
	var severity *engine.Severity
	if sv := r.URL.Query().Get("severity"); sv != "" {
		sev := parseSeverity(sv)
		severity = &sev
	}
	events, err := s.store.GetEvents(since, namespace, severity)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"events": events, "total": len(events)})
}

func (s *Server) handleIncidents(w http.ResponseWriter, r *http.Request) {
	since := parseDuration(r.URL.Query().Get("since"), 24*time.Hour)
	incidents, err := s.store.GetIncidents(since)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"incidents": incidents, "total": len(incidents)})
}

func (s *Server) handleAuditReports(w http.ResponseWriter, r *http.Request) {
	since := parseDuration(r.URL.Query().Get("since"), 7*24*time.Hour)
	reports, err := s.store.GetAuditHistory(since)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"reports": reports, "total": len(reports)})
}

func (s *Server) handleLatestAuditReport(w http.ResponseWriter, r *http.Request) {
	report, err := s.store.GetLatestAuditReport()
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no audit reports found"})
		return
	}
	format := r.URL.Query().Get("format")
	if format == "html" {
		w.Header().Set("Content-Type", "text/html")
		w.Header().Set("Content-Disposition", "attachment; filename=otacon-audit-report.html")
		w.Write([]byte(generateFullHTMLReport(report)))
		return
	}
	writeJSON(w, http.StatusOK, report)
}

func (s *Server) handleAuditHistory(w http.ResponseWriter, r *http.Request) {
	since := parseDuration(r.URL.Query().Get("since"), 7*24*time.Hour)
	reports, err := s.store.GetAuditHistory(since)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	type scorePoint struct {
		Time  time.Time `json:"time"`
		Score float64   `json:"score"`
		Grade string    `json:"grade"`
	}
	var trend []scorePoint
	for _, rp := range reports {
		trend = append(trend, scorePoint{Time: rp.ScanTime, Score: rp.OverallScore, Grade: rp.Grade})
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"trend": trend, "total": len(trend)})
}

func (s *Server) handleDedupGroups(w http.ResponseWriter, r *http.Request) {
	groups := s.dedup.GetActiveGroups()
	writeJSON(w, http.StatusOK, map[string]interface{}{"groups": groups, "total": len(groups)})
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"database": s.store.Stats(),
		"cooldown": s.cooldown.GetStats(),
		"router":   s.router.GetStats(),
	})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status": "running", "mode": "guardian", "version": "0.1.0",
	})
}

func (s *Server) handleAlertmanagerWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "POST required"})
		return
	}
	var payload struct {
		Alerts []struct {
			Status      string            `json:"status"`
			Labels      map[string]string `json:"labels"`
			Annotations map[string]string `json:"annotations"`
			StartsAt    time.Time         `json:"startsAt"`
		} `json:"alerts"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid payload"})
		return
	}
	for _, alert := range payload.Alerts {
		s.correlator.Ingest(engine.Event{
			ID:           fmt.Sprintf("am-%s-%s", alert.Labels["alertname"], alert.Labels["instance"]),
			Type:         "AlertManager",
			Reason:       alert.Labels["alertname"],
			Message:      alert.Annotations["summary"],
			Namespace:    alert.Labels["namespace"],
			ResourceName: alert.Labels["pod"],
			NodeName:     alert.Labels["instance"],
			Severity:     alertmanagerSeverity(alert.Labels["severity"]),
			FirstSeen:    alert.StartsAt,
			LastSeen:     alert.StartsAt,
		})
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "accepted", "count": strconv.Itoa(len(payload.Alerts))})
}

// ---- Helpers ----

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func parseDuration(s string, d time.Duration) time.Duration {
	if s == "" {
		return d
	}
	v, err := time.ParseDuration(s)
	if err != nil {
		return d
	}
	return v
}

func parseSeverity(s string) engine.Severity {
	switch s {
	case "critical", "CRITICAL":
		return engine.SeverityCritical
	case "warning", "WARNING":
		return engine.SeverityWarning
	default:
		return engine.SeverityInfo
	}
}

func alertmanagerSeverity(s string) engine.Severity {
	switch s {
	case "critical":
		return engine.SeverityCritical
	case "warning":
		return engine.SeverityWarning
	default:
		return engine.SeverityInfo
	}
}
