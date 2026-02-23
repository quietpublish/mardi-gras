package views

import (
	"strings"
	"testing"

	"github.com/matt-wright86/mardi-gras/internal/gastown"
)

func TestNewGasTown(t *testing.T) {
	g := NewGasTown(80, 24)
	if g.width != 80 {
		t.Fatalf("width = %d, want 80", g.width)
	}
	if g.height != 24 {
		t.Fatalf("height = %d, want 24", g.height)
	}
}

func TestGasTownSetSize(t *testing.T) {
	g := NewGasTown(80, 24)
	g.SetSize(120, 40)
	if g.width != 120 {
		t.Fatalf("width = %d, want 120", g.width)
	}
	if g.height != 40 {
		t.Fatalf("height = %d, want 40", g.height)
	}
	if g.viewport.Width != 118 {
		t.Fatalf("viewport.Width = %d, want 118", g.viewport.Width)
	}
	if g.viewport.Height != 40 {
		t.Fatalf("viewport.Height = %d, want 40", g.viewport.Height)
	}
}

func TestGasTownSetStatus(t *testing.T) {
	g := NewGasTown(80, 24)
	status := &gastown.TownStatus{
		Agents: []gastown.AgentRuntime{
			{Name: "polecat-1", Role: "polecat", State: "working", Running: true, HasWork: true, WorkTitle: "Fix bug"},
		},
	}
	env := gastown.Env{Available: true, Role: "mayor", Rig: "test-rig"}
	g.SetStatus(status, env)

	if g.status != status {
		t.Fatal("status not set")
	}
	if g.env.Role != "mayor" {
		t.Fatalf("env.Role = %q, want %q", g.env.Role, "mayor")
	}
}

func TestGasTownViewNoStatus(t *testing.T) {
	g := NewGasTown(80, 24)
	view := g.View()
	if !strings.Contains(view, "not available") {
		t.Fatalf("nil status should show 'not available', got: %s", view)
	}
}

func TestGasTownViewEmptyAgents(t *testing.T) {
	g := NewGasTown(80, 24)
	status := &gastown.TownStatus{Agents: []gastown.AgentRuntime{}}
	env := gastown.Env{Available: true}
	g.SetStatus(status, env)

	view := g.View()
	if !strings.Contains(view, "No agents") {
		t.Fatalf("empty agents should show placeholder, got: %s", view)
	}
}

func TestGasTownViewWithAgents(t *testing.T) {
	g := NewGasTown(100, 30)
	status := &gastown.TownStatus{
		Agents: []gastown.AgentRuntime{
			{Name: "polecat-1", Role: "polecat", State: "working", Running: true, HasWork: true, WorkTitle: "Fix the login bug"},
			{Name: "crew-alpha", Role: "crew", State: "idle", Running: true},
		},
	}
	env := gastown.Env{Available: true, Role: "mayor", Rig: "my-project"}
	g.SetStatus(status, env)

	view := g.View()
	if !strings.Contains(view, "polecat-1") {
		t.Fatalf("view should contain agent name 'polecat-1', got: %s", view)
	}
	if !strings.Contains(view, "crew-alpha") {
		t.Fatalf("view should contain agent name 'crew-alpha', got: %s", view)
	}
}

func TestGasTownViewWithConvoys(t *testing.T) {
	g := NewGasTown(100, 30)
	status := &gastown.TownStatus{
		Agents: []gastown.AgentRuntime{},
		Convoys: []gastown.ConvoyInfo{
			{ID: "conv-1", Title: "Sprint delivery", Status: "active", Done: 3, Total: 10},
		},
	}
	env := gastown.Env{Available: true}
	g.SetStatus(status, env)

	view := g.View()
	if !strings.Contains(view, "Sprint delivery") {
		t.Fatalf("view should contain convoy title, got: %s", view)
	}
	if !strings.Contains(view, "3/10") {
		t.Fatalf("view should contain progress label '3/10', got: %s", view)
	}
}

func TestProgressBar(t *testing.T) {
	tests := []struct {
		name     string
		done     int
		total    int
		width    int
		wantLen  int
		wantFull bool // all filled
	}{
		{name: "zero total", done: 0, total: 0, width: 10, wantLen: 10},
		{name: "half done", done: 5, total: 10, width: 20, wantLen: 20},
		{name: "all done", done: 10, total: 10, width: 10, wantLen: 10, wantFull: true},
		{name: "zero width", done: 5, total: 10, width: 0, wantLen: 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			bar := progressBar(tc.done, tc.total, tc.width)
			// The bar contains ANSI escape codes from lipgloss, so we can't check raw length.
			// But we can check the content is not empty for non-zero widths.
			if tc.width > 0 && len(bar) == 0 {
				t.Fatal("expected non-empty progress bar")
			}
			if tc.width == 0 && bar != "" {
				t.Fatalf("expected empty bar for zero width, got %q", bar)
			}
		})
	}
}

func TestTruncateGT(t *testing.T) {
	tests := []struct {
		input  string
		maxLen int
		expect string
	}{
		{"short", 10, "short"},
		{"hello world long string", 10, "hello w..."},
		{"ab", 2, "ab"},
		{"abc", 2, "ab"},
	}

	for _, tc := range tests {
		got := truncateGT(tc.input, tc.maxLen)
		if got != tc.expect {
			t.Fatalf("truncateGT(%q, %d) = %q, want %q", tc.input, tc.maxLen, got, tc.expect)
		}
	}
}
