package engine

import "testing"

func TestCalculateGrade(t *testing.T) {
	tests := []struct{ score float64; grade string }{
		{100, "A+"}, {95, "A+"}, {92, "A"}, {90, "A"}, {87, "A-"}, {85, "A-"},
		{82, "B+"}, {80, "B+"}, {77, "B"}, {75, "B"}, {72, "B-"}, {70, "B-"},
		{65, "C"}, {60, "C"}, {55, "D"}, {50, "D"}, {45, "F"}, {0, "F"},
	}
	for _, tt := range tests {
		if got := CalculateGrade(tt.score); got != tt.grade {
			t.Errorf("CalculateGrade(%.0f) = %s, want %s", tt.score, got, tt.grade)
		}
	}
}

func TestProgressBar(t *testing.T) {
	if b := ProgressBar(50, 10); len(b) == 0 { t.Error("empty bar") }
	if ProgressBar(100, 10) == ProgressBar(0, 10) { t.Error("100 and 0 should differ") }
	if b := ProgressBar(-10, 10); b == "" { t.Error("neg should produce bar") }
	if b := ProgressBar(150, 10); b == "" { t.Error("over 100 should produce bar") }
}

func TestSeverityString(t *testing.T) {
	if SeverityInfo.String() != "INFO" { t.Error("info string") }
	if SeverityWarning.String() != "WARNING" { t.Error("warning string") }
	if SeverityCritical.String() != "CRITICAL" { t.Error("critical string") }
	if SeverityInfo.Icon() != "🔵" { t.Error("info icon") }
	if SeverityCritical.Icon() != "🔴" { t.Error("critical icon") }
}

func TestCategoryScorePercentage(t *testing.T) {
	cs := CategoryScore{Score: 85, MaxScore: 100}
	if cs.Percentage() != 85 { t.Errorf("expected 85, got %.0f", cs.Percentage()) }
	cs0 := CategoryScore{Score: 0, MaxScore: 0}
	if cs0.Percentage() != 0 { t.Errorf("expected 0 for zero max, got %.0f", cs0.Percentage()) }
}
