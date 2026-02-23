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

func TestGasTownSectionToggle(t *testing.T) {
	g := NewGasTown(100, 30)
	agents := []gastown.AgentRuntime{
		{Name: "alpha", Role: "polecat"},
	}
	status := &gastown.TownStatus{Agents: agents}
	g.SetStatus(status, gastown.Env{Available: true})

	// Add convoy details so tab toggle is possible
	g.SetConvoyDetails([]gastown.ConvoyDetail{
		{ID: "cv-1", Title: "Sprint 1", Status: "open", Completed: 2, Total: 5},
	})

	if g.Section() != SectionAgents {
		t.Fatal("initial section should be SectionAgents")
	}

	// Tab to convoys
	g, _ = g.Update(tea.KeyMsg{Type: tea.KeyTab})
	if g.Section() != SectionConvoys {
		t.Fatal("after tab, section should be SectionConvoys")
	}

	// Tab back to agents
	g, _ = g.Update(tea.KeyMsg{Type: tea.KeyTab})
	if g.Section() != SectionAgents {
		t.Fatal("after second tab, section should be SectionAgents")
	}
}

func TestGasTownConvoyCursor(t *testing.T) {
	g := NewGasTown(100, 30)
	status := &gastown.TownStatus{Agents: []gastown.AgentRuntime{}}
	g.SetStatus(status, gastown.Env{Available: true})

	convoys := []gastown.ConvoyDetail{
		{ID: "cv-1", Title: "Alpha", Status: "open", Completed: 1, Total: 3},
		{ID: "cv-2", Title: "Bravo", Status: "open", Completed: 0, Total: 2},
		{ID: "cv-3", Title: "Charlie", Status: "closed", Completed: 4, Total: 4},
	}
	g.SetConvoyDetails(convoys)

	// Switch to convoy section
	g.section = SectionConvoys

	if g.convoyCursor != 0 {
		t.Fatalf("initial convoy cursor = %d, want 0", g.convoyCursor)
	}

	// Move down
	g, _ = g.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if g.convoyCursor != 1 {
		t.Fatalf("after j, convoy cursor = %d, want 1", g.convoyCursor)
	}

	// Move down
	g, _ = g.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if g.convoyCursor != 2 {
		t.Fatalf("after j j, convoy cursor = %d, want 2", g.convoyCursor)
	}

	// Can't go past end
	g, _ = g.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if g.convoyCursor != 2 {
		t.Fatalf("cursor should clamp at end, got %d", g.convoyCursor)
	}

	// Move up
	g, _ = g.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if g.convoyCursor != 1 {
		t.Fatalf("after k, convoy cursor = %d, want 1", g.convoyCursor)
	}

	// Jump to top
	g, _ = g.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	if g.convoyCursor != 0 {
		t.Fatalf("after g, convoy cursor = %d, want 0", g.convoyCursor)
	}

	// Jump to bottom
	g, _ = g.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	if g.convoyCursor != 2 {
		t.Fatalf("after G, convoy cursor = %d, want 2", g.convoyCursor)
	}
}

func TestGasTownConvoyExpandCollapse(t *testing.T) {
	g := NewGasTown(100, 30)
	status := &gastown.TownStatus{Agents: []gastown.AgentRuntime{}}
	g.SetStatus(status, gastown.Env{Available: true})

	convoys := []gastown.ConvoyDetail{
		{
			ID: "cv-1", Title: "Sprint", Status: "open", Completed: 1, Total: 2,
			Tracked: []gastown.TrackedIssueInfo{
				{ID: "bd-1", Title: "Fix bug", Status: "closed"},
				{ID: "bd-2", Title: "Add feature", Status: "in_progress", Worker: "toast"},
			},
		},
	}
	g.SetConvoyDetails(convoys)
	g.section = SectionConvoys

	if g.expandedConvoy != -1 {
		t.Fatalf("initial expanded = %d, want -1", g.expandedConvoy)
	}

	// Enter expands
	g, _ = g.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if g.expandedConvoy != 0 {
		t.Fatalf("after enter, expanded = %d, want 0", g.expandedConvoy)
	}

	// Enter again collapses
	g, _ = g.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if g.expandedConvoy != -1 {
		t.Fatalf("after second enter, expanded = %d, want -1", g.expandedConvoy)
	}
}

func TestGasTownSelectedConvoy(t *testing.T) {
	g := NewGasTown(100, 30)
	status := &gastown.TownStatus{Agents: []gastown.AgentRuntime{}}
	g.SetStatus(status, gastown.Env{Available: true})

	// No convoys → nil
	if g.SelectedConvoy() != nil {
		t.Fatal("expected nil convoy when none set")
	}

	convoys := []gastown.ConvoyDetail{
		{ID: "cv-1", Title: "Alpha"},
		{ID: "cv-2", Title: "Bravo"},
	}
	g.SetConvoyDetails(convoys)

	// Still nil when in agents section
	if g.SelectedConvoy() != nil {
		t.Fatal("expected nil convoy when in agents section")
	}

	// Switch to convoy section
	g.section = SectionConvoys
	c := g.SelectedConvoy()
	if c == nil || c.ID != "cv-1" {
		t.Fatalf("expected convoy cv-1, got %v", c)
	}

	g, _ = g.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	c = g.SelectedConvoy()
	if c == nil || c.ID != "cv-2" {
		t.Fatalf("expected convoy cv-2, got %v", c)
	}
}

func TestGasTownActionConvoyLand(t *testing.T) {
	g := NewGasTown(100, 30)
	status := &gastown.TownStatus{Agents: []gastown.AgentRuntime{}}
	g.SetStatus(status, gastown.Env{Available: true})

	convoys := []gastown.ConvoyDetail{
		{ID: "cv-1", Title: "Sprint", Status: "open"},
	}
	g.SetConvoyDetails(convoys)
	g.section = SectionConvoys

	g, cmd := g.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	if cmd == nil {
		t.Fatal("expected cmd from convoy land action")
	}
	msg := cmd()
	action, ok := msg.(GasTownActionMsg)
	if !ok {
		t.Fatalf("expected GasTownActionMsg, got %T", msg)
	}
	if action.Type != "convoy_land" {
		t.Fatalf("expected type 'convoy_land', got %q", action.Type)
	}
	if action.ConvoyID != "cv-1" {
		t.Fatalf("expected convoy ID 'cv-1', got %q", action.ConvoyID)
	}
}

func TestGasTownActionConvoyClose(t *testing.T) {
	g := NewGasTown(100, 30)
	status := &gastown.TownStatus{Agents: []gastown.AgentRuntime{}}
	g.SetStatus(status, gastown.Env{Available: true})

	convoys := []gastown.ConvoyDetail{
		{ID: "cv-2", Title: "Cleanup", Status: "open"},
	}
	g.SetConvoyDetails(convoys)
	g.section = SectionConvoys

	g, cmd := g.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if cmd == nil {
		t.Fatal("expected cmd from convoy close action")
	}
	msg := cmd()
	action, ok := msg.(GasTownActionMsg)
	if !ok {
		t.Fatalf("expected GasTownActionMsg, got %T", msg)
	}
	if action.Type != "convoy_close" {
		t.Fatalf("expected type 'convoy_close', got %q", action.Type)
	}
	if action.ConvoyID != "cv-2" {
		t.Fatalf("expected convoy ID 'cv-2', got %q", action.ConvoyID)
	}
}

func TestGasTownConvoyCursorClamp(t *testing.T) {
	g := NewGasTown(100, 30)
	g.section = SectionConvoys

	// Start with 3 convoys, cursor at 2
	convoys := []gastown.ConvoyDetail{
		{ID: "cv-1"}, {ID: "cv-2"}, {ID: "cv-3"},
	}
	g.SetConvoyDetails(convoys)
	g.convoyCursor = 2

	// Now reduce to 1 convoy — cursor should clamp
	g.SetConvoyDetails([]gastown.ConvoyDetail{{ID: "cv-1"}})
	if g.convoyCursor != 0 {
		t.Fatalf("convoy cursor should clamp to 0, got %d", g.convoyCursor)
	}
}

func TestGasTownConvoyDetailsRender(t *testing.T) {
	g := NewGasTown(100, 30)
	status := &gastown.TownStatus{Agents: []gastown.AgentRuntime{}}
	g.SetStatus(status, gastown.Env{Available: true})

	convoys := []gastown.ConvoyDetail{
		{
			ID: "cv-1", Title: "Sprint delivery", Status: "open",
			Completed: 3, Total: 5,
			Tracked: []gastown.TrackedIssueInfo{
				{ID: "bd-1", Title: "Fix login", Status: "closed"},
			},
		},
	}
	g.SetConvoyDetails(convoys)

	view := g.View()
	if !strings.Contains(view, "Sprint delivery") {
		t.Fatal("view should contain convoy detail title")
	}
	if !strings.Contains(view, "3/5") {
		t.Fatal("view should contain progress 3/5")
	}
}

func TestGasTownSetMailMessages(t *testing.T) {
	g := NewGasTown(100, 30)
	status := &gastown.TownStatus{Agents: []gastown.AgentRuntime{}}
	g.SetStatus(status, gastown.Env{Available: true})

	msgs := []gastown.MailMessage{
		{ID: "msg-1", From: "gastown/Toast", Subject: "Status update", Read: false},
		{ID: "msg-2", From: "mayor/", Subject: "Deploy approved", Read: true},
	}
	g.SetMailMessages(msgs)

	if len(g.mailMessages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(g.mailMessages))
	}
}

func TestGasTownSelectedMail(t *testing.T) {
	g := NewGasTown(100, 30)
	status := &gastown.TownStatus{Agents: []gastown.AgentRuntime{}}
	g.SetStatus(status, gastown.Env{Available: true})

	// No messages → nil
	if g.SelectedMail() != nil {
		t.Fatal("expected nil mail when none set")
	}

	msgs := []gastown.MailMessage{
		{ID: "msg-1", Subject: "First"},
		{ID: "msg-2", Subject: "Second"},
	}
	g.SetMailMessages(msgs)

	// Still nil in agents section
	if g.SelectedMail() != nil {
		t.Fatal("expected nil mail when in agents section")
	}

	g.section = SectionMail
	m := g.SelectedMail()
	if m == nil || m.ID != "msg-1" {
		t.Fatalf("expected msg-1, got %v", m)
	}

	g, _ = g.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = g.SelectedMail()
	if m == nil || m.ID != "msg-2" {
		t.Fatalf("expected msg-2, got %v", m)
	}
}

func TestGasTownMailCursor(t *testing.T) {
	g := NewGasTown(100, 30)
	status := &gastown.TownStatus{Agents: []gastown.AgentRuntime{}}
	g.SetStatus(status, gastown.Env{Available: true})

	msgs := []gastown.MailMessage{
		{ID: "msg-1"}, {ID: "msg-2"}, {ID: "msg-3"},
	}
	g.SetMailMessages(msgs)
	g.section = SectionMail

	// Move down
	g, _ = g.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if g.mailCursor != 1 {
		t.Fatalf("after j, mail cursor = %d, want 1", g.mailCursor)
	}

	// Can't go past end
	g, _ = g.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	g, _ = g.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if g.mailCursor != 2 {
		t.Fatalf("cursor should clamp at end, got %d", g.mailCursor)
	}

	// Jump to top
	g, _ = g.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	if g.mailCursor != 0 {
		t.Fatalf("after g, mail cursor = %d, want 0", g.mailCursor)
	}

	// Jump to bottom
	g, _ = g.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	if g.mailCursor != 2 {
		t.Fatalf("after G, mail cursor = %d, want 2", g.mailCursor)
	}
}

func TestGasTownMailExpandCollapse(t *testing.T) {
	g := NewGasTown(100, 30)
	status := &gastown.TownStatus{Agents: []gastown.AgentRuntime{}}
	g.SetStatus(status, gastown.Env{Available: true})

	msgs := []gastown.MailMessage{
		{ID: "msg-1", Subject: "Test", Body: "Hello world", Read: true},
	}
	g.SetMailMessages(msgs)
	g.section = SectionMail

	if g.expandedMail != -1 {
		t.Fatalf("initial expanded = %d, want -1", g.expandedMail)
	}

	// Enter expands (already-read message, no cmd emitted)
	g, cmd := g.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if g.expandedMail != 0 {
		t.Fatalf("after enter, expanded = %d, want 0", g.expandedMail)
	}
	if cmd != nil {
		t.Fatal("expected no cmd for already-read message expand")
	}

	// Enter again collapses
	g, _ = g.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if g.expandedMail != -1 {
		t.Fatalf("after second enter, expanded = %d, want -1", g.expandedMail)
	}
}

func TestGasTownMailExpandUnreadEmitsMarkRead(t *testing.T) {
	g := NewGasTown(100, 30)
	status := &gastown.TownStatus{Agents: []gastown.AgentRuntime{}}
	g.SetStatus(status, gastown.Env{Available: true})

	msgs := []gastown.MailMessage{
		{ID: "msg-1", Subject: "Unread", Read: false},
	}
	g.SetMailMessages(msgs)
	g.section = SectionMail

	g, cmd := g.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if g.expandedMail != 0 {
		t.Fatalf("expected expand, got %d", g.expandedMail)
	}
	if cmd == nil {
		t.Fatal("expected mark-read cmd for unread message")
	}
	msg := cmd()
	action, ok := msg.(GasTownActionMsg)
	if !ok {
		t.Fatalf("expected GasTownActionMsg, got %T", msg)
	}
	if action.Type != "mail_read" {
		t.Fatalf("expected type 'mail_read', got %q", action.Type)
	}
	if action.Mail.ID != "msg-1" {
		t.Fatalf("expected mail ID 'msg-1', got %q", action.Mail.ID)
	}
}

func TestGasTownMailActionReply(t *testing.T) {
	g := NewGasTown(100, 30)
	status := &gastown.TownStatus{Agents: []gastown.AgentRuntime{}}
	g.SetStatus(status, gastown.Env{Available: true})

	msgs := []gastown.MailMessage{
		{ID: "msg-1", From: "gastown/Toast", Subject: "Help"},
	}
	g.SetMailMessages(msgs)
	g.section = SectionMail

	g, cmd := g.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	if cmd == nil {
		t.Fatal("expected cmd from reply action")
	}
	msg := cmd()
	action, ok := msg.(GasTownActionMsg)
	if !ok {
		t.Fatalf("expected GasTownActionMsg, got %T", msg)
	}
	if action.Type != "mail_reply" {
		t.Fatalf("expected type 'mail_reply', got %q", action.Type)
	}
	if action.Mail.ID != "msg-1" {
		t.Fatalf("expected mail ID 'msg-1', got %q", action.Mail.ID)
	}
}

func TestGasTownMailActionArchive(t *testing.T) {
	g := NewGasTown(100, 30)
	status := &gastown.TownStatus{Agents: []gastown.AgentRuntime{}}
	g.SetStatus(status, gastown.Env{Available: true})

	msgs := []gastown.MailMessage{
		{ID: "msg-2", Subject: "Done"},
	}
	g.SetMailMessages(msgs)
	g.section = SectionMail

	g, cmd := g.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	if cmd == nil {
		t.Fatal("expected cmd from archive action")
	}
	msg := cmd()
	action, ok := msg.(GasTownActionMsg)
	if !ok {
		t.Fatalf("expected GasTownActionMsg, got %T", msg)
	}
	if action.Type != "mail_archive" {
		t.Fatalf("expected type 'mail_archive', got %q", action.Type)
	}
}

func TestGasTownMailRender(t *testing.T) {
	g := NewGasTown(100, 30)
	status := &gastown.TownStatus{Agents: []gastown.AgentRuntime{}}
	g.SetStatus(status, gastown.Env{Available: true})

	msgs := []gastown.MailMessage{
		{ID: "msg-1", From: "gastown/Toast", Subject: "Build complete", Read: false, Type: "notification"},
		{ID: "msg-2", From: "mayor/", Subject: "Deploy request", Read: true, Type: "task"},
	}
	g.SetMailMessages(msgs)

	view := g.View()
	if !strings.Contains(view, "MAIL") {
		t.Fatal("view should contain MAIL section")
	}
	if !strings.Contains(view, "1 unread") {
		t.Fatal("view should contain unread count")
	}
	if !strings.Contains(view, "Build complete") {
		t.Fatal("view should contain message subject")
	}
}

func TestGasTownSectionCycleWithMail(t *testing.T) {
	g := NewGasTown(100, 30)
	status := &gastown.TownStatus{Agents: []gastown.AgentRuntime{
		{Name: "test", Role: "polecat"},
	}}
	g.SetStatus(status, gastown.Env{Available: true})

	g.SetConvoyDetails([]gastown.ConvoyDetail{{ID: "cv-1", Title: "Sprint"}})
	g.SetMailMessages([]gastown.MailMessage{{ID: "msg-1", Subject: "Hello"}})

	// Agents → Convoys
	g, _ = g.Update(tea.KeyMsg{Type: tea.KeyTab})
	if g.Section() != SectionConvoys {
		t.Fatalf("expected SectionConvoys, got %d", g.Section())
	}

	// Convoys → Mail
	g, _ = g.Update(tea.KeyMsg{Type: tea.KeyTab})
	if g.Section() != SectionMail {
		t.Fatalf("expected SectionMail, got %d", g.Section())
	}

	// Mail → Agents
	g, _ = g.Update(tea.KeyMsg{Type: tea.KeyTab})
	if g.Section() != SectionAgents {
		t.Fatalf("expected SectionAgents, got %d", g.Section())
	}
}

func TestGasTownMailNoActionInOtherSection(t *testing.T) {
	g := NewGasTown(100, 30)
	status := &gastown.TownStatus{Agents: []gastown.AgentRuntime{
		{Name: "test", Role: "polecat"},
	}}
	g.SetStatus(status, gastown.Env{Available: true})

	g.SetMailMessages([]gastown.MailMessage{{ID: "msg-1", Subject: "Test"}})

	// r and d should not produce cmd in agents section
	_, cmd := g.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	if cmd != nil {
		t.Fatal("r should not produce cmd in agents section")
	}
	_, cmd = g.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	if cmd != nil {
		t.Fatal("d should not produce cmd in agents section")
	}
}

func TestGasTownMailCursorClamp(t *testing.T) {
	g := NewGasTown(100, 30)
	g.section = SectionMail

	msgs := []gastown.MailMessage{{ID: "msg-1"}, {ID: "msg-2"}, {ID: "msg-3"}}
	g.SetMailMessages(msgs)
	g.mailCursor = 2

	// Reduce to 1 message — cursor should clamp
	g.SetMailMessages([]gastown.MailMessage{{ID: "msg-1"}})
	if g.mailCursor != 0 {
		t.Fatalf("mail cursor should clamp to 0, got %d", g.mailCursor)
	}
}

func TestGasTownConvoyNoActionInAgentSection(t *testing.T) {
	g := NewGasTown(100, 30)
	status := &gastown.TownStatus{Agents: []gastown.AgentRuntime{
		{Name: "test", Role: "polecat"},
	}}
	g.SetStatus(status, gastown.Env{Available: true})

	convoys := []gastown.ConvoyDetail{{ID: "cv-1", Title: "Test"}}
	g.SetConvoyDetails(convoys)

	// l and x should not produce cmd when in agents section
	_, cmd := g.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	if cmd != nil {
		t.Fatal("l should not produce cmd in agents section")
	}
	_, cmd = g.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if cmd != nil {
		t.Fatal("x should not produce cmd in agents section")
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

func TestGasTownSetCosts(t *testing.T) {
	g := NewGasTown(100, 30)
	status := &gastown.TownStatus{Agents: []gastown.AgentRuntime{}}
	g.SetStatus(status, gastown.Env{Available: true})

	costs := &gastown.CostsOutput{
		Period:   "today",
		Total:    gastown.CostTotal{InputTokens: 150000, OutputTokens: 50000, Cost: 47.23},
		Sessions: 20,
		ByRole: []gastown.RoleCost{
			{Role: "polecat", Sessions: 12, Cost: 12.30},
			{Role: "witness", Sessions: 3, Cost: 3.20},
		},
	}
	g.SetCosts(costs)

	view := g.View()
	if !strings.Contains(view, "COSTS") {
		t.Fatal("view should contain COSTS section")
	}
	if !strings.Contains(view, "47.23") {
		t.Fatal("view should contain total cost")
	}
	if !strings.Contains(view, "polecat") {
		t.Fatal("view should contain role 'polecat'")
	}
	if !strings.Contains(view, "12.30") {
		t.Fatal("view should contain polecat cost")
	}
}

func TestGasTownNoCostsSection(t *testing.T) {
	g := NewGasTown(100, 30)
	status := &gastown.TownStatus{Agents: []gastown.AgentRuntime{}}
	g.SetStatus(status, gastown.Env{Available: true})

	// No costs set
	view := g.View()
	if strings.Contains(view, "COSTS") {
		t.Fatal("view should not contain COSTS section when no data")
	}
}
