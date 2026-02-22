package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestPriorityColor(t *testing.T) {
	tests := []struct {
		name     string
		priority int
		want     lipgloss.Color
	}{
		{"P0", 0, PrioP0},
		{"P1", 1, PrioP1},
		{"P2", 2, PrioP2},
		{"P3", 3, PrioP3},
		{"P4", 4, PrioP4},
		{"negative falls back to Muted", -1, Muted},
		{"out of range falls back to Muted", 5, Muted},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PriorityColor(tt.priority)
			if got != tt.want {
				t.Errorf("PriorityColor(%d) = %v, want %v", tt.priority, got, tt.want)
			}
		})
	}
}

func TestIssueTypeColor(t *testing.T) {
	tests := []struct {
		name      string
		issueType string
		want      lipgloss.Color
	}{
		{"bug", "bug", ColorBug},
		{"feature", "feature", ColorFeature},
		{"task", "task", ColorTask},
		{"chore", "chore", ColorChore},
		{"epic", "epic", ColorEpic},
		{"empty string falls back to Muted", "", Muted},
		{"unknown falls back to Muted", "unknown", Muted},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IssueTypeColor(tt.issueType)
			if got != tt.want {
				t.Errorf("IssueTypeColor(%q) = %v, want %v", tt.issueType, got, tt.want)
			}
		})
	}
}

func TestApplyMardiGrasGradientEmpty(t *testing.T) {
	result := ApplyMardiGrasGradient("")
	if result != "" {
		t.Errorf("ApplyMardiGrasGradient(\"\") = %q, want \"\"", result)
	}
}

func TestApplyMardiGrasGradientNonEmpty(t *testing.T) {
	input := "hello"
	result := ApplyMardiGrasGradient(input)

	if result == "" {
		t.Fatal("ApplyMardiGrasGradient(\"hello\") returned empty string")
	}

	for _, r := range input {
		if !strings.Contains(result, string(r)) {
			t.Errorf("result missing character %q from input", string(r))
		}
	}
}

func TestApplyPartialGradientZeroLength(t *testing.T) {
	result := ApplyPartialMardiGrasGradient("hello", 0)
	if result != "" {
		t.Errorf("ApplyPartialMardiGrasGradient(\"hello\", 0) = %q, want \"\"", result)
	}
}

func TestApplyPartialGradientNonEmpty(t *testing.T) {
	result := ApplyPartialMardiGrasGradient("hello", 10)
	if result == "" {
		t.Fatal("ApplyPartialMardiGrasGradient(\"hello\", 10) returned empty string")
	}
}
