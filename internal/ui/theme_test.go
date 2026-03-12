package ui

import (
	"image/color"
	"strings"
	"testing"
)

func TestPriorityColor(t *testing.T) {
	tests := []struct {
		name     string
		priority int
		want     color.Color
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
		want      color.Color
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

func TestAgentStateColor(t *testing.T) {
	tests := []struct {
		name  string
		state string
		want  color.Color
	}{
		{"working", "working", StateWorking},
		{"spawning", "spawning", StateSpawn},
		{"idle", "idle", StateIdle},
		{"backoff", "backoff", StateBackoff},
		{"degraded maps to backoff", "degraded", StateBackoff},
		{"stuck", "stuck", StateStuck},
		{"awaiting-gate", "awaiting-gate", StateGate},
		{"fix_needed", "fix_needed", StateFixNeeded},
		{"paused", "paused", Dim},
		{"muted", "muted", Dim},
		{"unknown falls back to idle", "unknown", StateIdle},
		{"empty falls back to idle", "", StateIdle},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AgentStateColor(tt.state)
			if got != tt.want {
				t.Errorf("AgentStateColor(%q) = %v, want %v", tt.state, got, tt.want)
			}
		})
	}
}

func TestAgentStateColorDistinctCategories(t *testing.T) {
	// Each state category should map to a distinct color
	colors := map[string]color.Color{
		"working":       AgentStateColor("working"),
		"spawning":      AgentStateColor("spawning"),
		"idle":          AgentStateColor("idle"),
		"backoff":       AgentStateColor("backoff"),
		"stuck":         AgentStateColor("stuck"),
		"awaiting-gate": AgentStateColor("awaiting-gate"),
		"fix_needed":    AgentStateColor("fix_needed"),
		"paused":        AgentStateColor("paused"),
	}

	// Verify distinct colors across categories (some share intentionally)
	pairs := [][2]string{
		{"working", "idle"},
		{"working", "backoff"},
		{"working", "stuck"},
		{"idle", "backoff"},
		{"idle", "stuck"},
		{"stuck", "backoff"},
		{"fix_needed", "stuck"},
		{"fix_needed", "backoff"},
		{"fix_needed", "working"},
		{"spawning", "idle"},
	}
	for _, pair := range pairs {
		if colors[pair[0]] == colors[pair[1]] {
			t.Errorf("AgentStateColor(%q) == AgentStateColor(%q), should be distinct", pair[0], pair[1])
		}
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
