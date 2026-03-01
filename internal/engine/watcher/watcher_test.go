package watcher

import (
	"testing"

	"github.com/merthan/otacon/internal/engine"
)

func TestClassifySeverity(t *testing.T) {
	tests := []struct {
		reason   string
		evType   string
		message  string
		expected engine.Severity
	}{
		{"OOMKilled", "Warning", "", engine.SeverityCritical},
		{"CrashLoopBackOff", "Warning", "", engine.SeverityCritical},
		{"NodeNotReady", "Warning", "", engine.SeverityCritical},
		{"FailedMount", "Warning", "", engine.SeverityCritical},
		{"NodeHasDiskPressure", "Warning", "", engine.SeverityCritical},

		{"BackOff", "Warning", "", engine.SeverityWarning},
		{"Unhealthy", "Warning", "", engine.SeverityWarning},
		{"FailedScheduling", "Warning", "", engine.SeverityWarning},
		{"ImagePullBackOff", "Warning", "", engine.SeverityWarning},
		{"Evicted", "Warning", "", engine.SeverityWarning},
		{"SomethingRandom", "Warning", "", engine.SeverityWarning},

		{"Pulled", "Normal", "", engine.SeverityInfo},
		{"Started", "Normal", "", engine.SeverityInfo},
		{"Created", "Normal", "", engine.SeverityInfo},
		{"Scheduled", "Normal", "", engine.SeverityInfo},

		// Message-based classification
		{"Unknown", "Normal", "out of memory error occurred", engine.SeverityCritical},
		{"Unknown", "Normal", "connection failed to backend", engine.SeverityWarning},
		{"Unknown", "Normal", "everything is fine", engine.SeverityInfo},
	}

	for _, tt := range tests {
		t.Run(tt.reason+"_"+tt.evType, func(t *testing.T) {
			got := ClassifySeverity(tt.reason, tt.evType, tt.message)
			if got != tt.expected {
				t.Errorf("ClassifySeverity(%q, %q, %q) = %s, want %s",
					tt.reason, tt.evType, tt.message, got, tt.expected)
			}
		})
	}
}

func TestMatchWildcard(t *testing.T) {
	tests := []struct {
		pattern string
		input   string
		match   bool
	}{
		{"prod*", "production", true},
		{"prod*", "prod-eu", true},
		{"prod*", "staging", false},
		{"default", "default", true},
		{"default", "kube-system", false},
	}

	for _, tt := range tests {
		got := matchWildcard(tt.pattern, tt.input)
		if got != tt.match {
			t.Errorf("matchWildcard(%q, %q) = %v, want %v", tt.pattern, tt.input, got, tt.match)
		}
	}
}
