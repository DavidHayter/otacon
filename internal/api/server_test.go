package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthzEndpoint(t *testing.T) {
	s := &Server{}
	req := httptest.NewRequest("GET", "/healthz", nil)
	w := httptest.NewRecorder()

	s.handleHealthz(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}

	var body map[string]string
	json.NewDecoder(w.Body).Decode(&body)
	if body["status"] != "ok" {
		t.Errorf("Expected status ok, got %s", body["status"])
	}
}

func TestReadyzEndpoint(t *testing.T) {
	s := &Server{}
	req := httptest.NewRequest("GET", "/readyz", nil)
	w := httptest.NewRecorder()

	s.handleReadyz(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}
}

func TestStatusEndpoint(t *testing.T) {
	s := &Server{}
	req := httptest.NewRequest("GET", "/api/v1/status", nil)
	w := httptest.NewRecorder()

	s.handleStatus(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}

	var body map[string]interface{}
	json.NewDecoder(w.Body).Decode(&body)
	if body["status"] != "running" {
		t.Errorf("Expected status running, got %v", body["status"])
	}
	if body["mode"] != "guardian" {
		t.Errorf("Expected mode guardian, got %v", body["mode"])
	}
}

func TestParseDuration(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"1h", "1h0m0s"},
		{"30m", "30m0s"},
		{"24h", "24h0m0s"},
		{"", "1h0m0s"}, // default
		{"invalid", "1h0m0s"}, // default
	}

	for _, tt := range tests {
		d := parseDuration(tt.input, 3600000000000) // 1h in nanoseconds
		if d.String() != tt.expected {
			t.Errorf("parseDuration(%q) = %s, want %s", tt.input, d.String(), tt.expected)
		}
	}
}

func TestParseSeverity(t *testing.T) {
	tests := []struct {
		input string
		expected int
	}{
		{"critical", 2},
		{"CRITICAL", 2},
		{"warning", 1},
		{"WARNING", 1},
		{"info", 0},
		{"unknown", 0},
	}

	for _, tt := range tests {
		got := parseSeverity(tt.input)
		if int(got) != tt.expected {
			t.Errorf("parseSeverity(%q) = %d, want %d", tt.input, got, tt.expected)
		}
	}
}

func TestCorsMiddleware(t *testing.T) {
	handler := corsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("OPTIONS", "/api/v1/events", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("OPTIONS should return 200, got %d", w.Code)
	}
	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("Missing CORS Allow-Origin header")
	}
	if w.Header().Get("Access-Control-Allow-Methods") == "" {
		t.Error("Missing CORS Allow-Methods header")
	}
}
