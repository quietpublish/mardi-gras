package views

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
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
		Agents:  []gastown.AgentRuntime{},
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

func TestGasTownViewWithRigs(t *testing.T) {
	g := NewGasTown(100, 30)
	status := &gastown.TownStatus{
		Agents: []gastown.AgentRuntime{},
		Rigs: []gastown.RigStatus{
			{Name: "my-project", PolecatCount: 3, CrewCount: 1, HasWitness: true, HasRefinery: false},
		},
	}
	env := gastown.Env{Available: true}
	g.SetStatus(status, env)

	view := g.View()
	if !strings.Contains(view, "my-project") {
		t.Fatalf("view should contain rig name, got: %s", view)
	}
	if !strings.Contains(view, "3 polecats") {
		t.Fatalf("view should contain polecat count, got: %s", view)
	}
	if !strings.Contains(view, "witness") {
		t.Fatalf("view should contain witness badge, got: %s", view)
	}
}

func TestGasTownAgentCursor(t *testing.T) {
	g := NewGasTown(100, 30)
	agents := []gastown.AgentRuntime{
		{Name: "alpha", Role: "polecat", State: "working", Address: "addr-1"},
		{Name: "bravo", Role: "polecat", State: "idle", Address: "addr-2"},
		{Name: "charlie", Role: "crew", State: "idle", Address: "addr-3"},
	}
	status := &gastown.TownStatus{Agents: agents}
	g.SetStatus(status, gastown.Env{Available: true})

	// Initial cursor at 0
	if g.agentCursor != 0 {
		t.Fatalf("initial cursor = %d, want 0", g.agentCursor)
	}

	// Move down
	g, _ = g.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if g.agentCursor != 1 {
		t.Fatalf("after j, cursor = %d, want 1", g.agentCursor)
	}

	// Move down again
	g, _ = g.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if g.agentCursor != 2 {
		t.Fatalf("after j j, cursor = %d, want 2", g.agentCursor)
	}

	// Can't go past end
	g, _ = g.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if g.agentCursor != 2 {
		t.Fatalf("cursor should clamp at end, got %d", g.agentCursor)
	}

	// Move up
	g, _ = g.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if g.agentCursor != 1 {
		t.Fatalf("after k, cursor = %d, want 1", g.agentCursor)
	}

	// Jump to top
	g, _ = g.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	if g.agentCursor != 0 {
		t.Fatalf("after g, cursor = %d, want 0", g.agentCursor)
	}

	// Jump to bottom
	g, _ = g.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	if g.agentCursor != 2 {
		t.Fatalf("after G, cursor = %d, want 2", g.agentCursor)
	}
}

func TestGasTownSelectedAgent(t *testing.T) {
	g := NewGasTown(100, 30)

	// No status → nil
	if g.SelectedAgent() != nil {
		t.Fatal("expected nil agent when no status")
	}

	agents := []gastown.AgentRuntime{
		{Name: "alpha", Role: "polecat", Address: "addr-1"},
		{Name: "bravo", Role: "crew", Address: "addr-2"},
	}
	status := &gastown.TownStatus{Agents: agents}
	g.SetStatus(status, gastown.Env{Available: true})

	// Cursor at 0 → first agent
	a := g.SelectedAgent()
	if a == nil || a.Name != "alpha" {
		t.Fatalf("expected agent 'alpha', got %v", a)
	}

	// Move to 1
	g, _ = g.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	a = g.SelectedAgent()
	if a == nil || a.Name != "bravo" {
		t.Fatalf("expected agent 'bravo', got %v", a)
	}
}

func TestGasTownActionNudge(t *testing.T) {
	g := NewGasTown(100, 30)
	agents := []gastown.AgentRuntime{
		{Name: "toast", Role: "polecat", Address: "beads/toast"},
	}
	status := &gastown.TownStatus{Agents: agents}
	g.SetStatus(status, gastown.Env{Available: true})

	g, cmd := g.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if cmd == nil {
		t.Fatal("expected cmd from nudge action")
	}
	msg := cmd()
	action, ok := msg.(GasTownActionMsg)
	if !ok {
		t.Fatalf("expected GasTownActionMsg, got %T", msg)
	}
	if action.Type != "nudge" {
		t.Fatalf("expected type 'nudge', got %q", action.Type)
	}
	if action.Agent.Name != "toast" {
		t.Fatalf("expected agent 'toast', got %q", action.Agent.Name)
	}
}

func TestGasTownActionHandoff(t *testing.T) {
	g := NewGasTown(100, 30)
	agents := []gastown.AgentRuntime{
		{Name: "toast", Role: "polecat", Address: "beads/toast"},
	}
	status := &gastown.TownStatus{Agents: agents}
	g.SetStatus(status, gastown.Env{Available: true})

	g, cmd := g.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	if cmd == nil {
		t.Fatal("expected cmd from handoff action")
	}
	msg := cmd()
	action, ok := msg.(GasTownActionMsg)
	if !ok {
		t.Fatalf("expected GasTownActionMsg, got %T", msg)
	}
	if action.Type != "handoff" {
		t.Fatalf("expected type 'handoff', got %q", action.Type)
	}
}

func TestGasTownActionDecommissionOnlyPolecat(t *testing.T) {
	g := NewGasTown(100, 30)
	agents := []gastown.AgentRuntime{
		{Name: "witness", Role: "witness", Address: "beads/witness"},
	}
	status := &gastown.TownStatus{Agents: agents}
	g.SetStatus(status, gastown.Env{Available: true})

	// K on non-polecat should not produce a cmd
	_, cmd := g.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'K'}})
	if cmd != nil {
		t.Fatal("expected no cmd for decommission on non-polecat")
	}

	// Now try with a polecat
	g2 := NewGasTown(100, 30)
	agents2 := []gastown.AgentRuntime{
		{Name: "toast", Role: "polecat", Address: "beads/toast"},
	}
	status2 := &gastown.TownStatus{Agents: agents2}
	g2.SetStatus(status2, gastown.Env{Available: true})

	_, cmd2 := g2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'K'}})
	if cmd2 == nil {
		t.Fatal("expected cmd for decommission on polecat")
	}
	msg := cmd2()
	action, ok := msg.(GasTownActionMsg)
	if !ok {
		t.Fatalf("expected GasTownActionMsg, got %T", msg)
	}
	if action.Type != "decommission" {
		t.Fatalf("expected type 'decommission', got %q", action.Type)
	}
}

func TestGasTownCursorClampOnStatusChange(t *testing.T) {
	g := NewGasTown(100, 30)

	// Start with 5 agents, cursor at 4
	agents := make([]gastown.AgentRuntime, 5)
	for i := range agents {
		agents[i] = gastown.AgentRuntime{Name: "agent-" + string(rune('0'+i)), Role: "polecat"}
	}
	status := &gastown.TownStatus{Agents: agents}
	g.SetStatus(status, gastown.Env{Available: true})
	g.agentCursor = 4

	// Now status changes to 2 agents — cursor should clamp
	agents2 := agents[:2]
	status2 := &gastown.TownStatus{Agents: agents2}
	g.SetStatus(status2, gastown.Env{Available: true})

	if g.agentCursor != 1 {
		t.Fatalf("cursor should clamp to %d, got %d", 1, g.agentCursor)
	}
}

func TestGasTownHints(t *testing.T) {
	g := NewGasTown(100, 30)
	status := &gastown.TownStatus{Agents: []gastown.AgentRuntime{
		{Name: "test", Role: "polecat"},
	}}
	g.SetStatus(status, gastown.Env{Available: true})

	view := g.View()
	if !strings.Contains(view, "nudge") {
		t.Fatal("view should contain hint bar with 'nudge'")
	}
	if !strings.Contains(view, "handoff") {
		t.Fatal("view should contain hint bar with 'handoff'")
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
