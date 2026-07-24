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
		{"spike", "spike", ColorSpike},
		{"story", "story", ColorStory},
		{"milestone", "milestone", ColorMilestone},
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
		{"propelled", "propelled", StatePropelled},
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
		"propelled":     AgentStateColor("propelled"),
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
		{"propelled", "working"},
		{"propelled", "idle"},
		{"propelled", "spawning"},
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

func TestRoleColor(t *testing.T) {
	tests := []struct {
		name string
		role string
		want color.Color
	}{
		{"mayor", "mayor", RoleMayor},
		{"coordinator alias of mayor", "coordinator", RoleMayor},
		{"deacon", "deacon", RoleDeacon},
		{"health-check alias of deacon", "health-check", RoleDeacon},
		{"polecat", "polecat", RolePolecat},
		{"crew", "crew", RoleCrew},
		{"witness", "witness", RoleWitness},
		{"refinery", "refinery", RoleRefinery},
		{"dog", "dog", RoleDog},
		{"unknown falls back to default", "frobnicator", RoleDefault},
		{"empty falls back to default", "", RoleDefault},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := RoleColor(tc.role)
			if got != tc.want {
				t.Errorf("RoleColor(%q) = %v, want %v", tc.role, got, tc.want)
			}
		})
	}
}

func TestAgentStateColorPatrolling(t *testing.T) {
	// Patrolling state was added 2026-05-13; verify it maps to the sky-blue
	// StatePatrolling color and not the default Idle fallback.
	if got := AgentStateColor("patrolling"); got != StatePatrolling {
		t.Errorf("AgentStateColor(\"patrolling\") = %v, want StatePatrolling", got)
	}
	if AgentStateColor("patrolling") == AgentStateColor("idle") {
		t.Error("patrolling and idle must map to distinct colors")
	}
}

// TestSetThemeRebakes guards the SetTheme contract: switching themes must
// rebake derived artifacts (pre-rendered strings, gradients), not just the
// palette vars.
func TestSetThemeRebakes(t *testing.T) {
	t.Cleanup(func() { SetTheme(ThemeDark) })

	darkBadge := BadgeP0
	darkHeat := GradientHeat.At(50).Render("█")

	SetTheme(ThemeLight)
	if BadgeP0 == darkBadge {
		t.Error("SetTheme(ThemeLight) did not rebake pre-rendered BadgeP0")
	}
	if GradientHeat.At(50).Render("█") == darkHeat {
		t.Error("SetTheme(ThemeLight) did not rebuild GradientHeat")
	}
}
