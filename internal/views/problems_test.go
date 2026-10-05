package views

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/matt-wright86/mardi-gras/internal/gastown"
)

func TestNewProblems(t *testing.T) {
	p := NewProblems(80, 24)
	if p.width != 80 {
		t.Fatalf("width = %d, want 80", p.width)
	}
	if p.Count() != 0 {
		t.Fatalf("count = %d, want 0", p.Count())
	}
}

func TestProblemsSetSize(t *testing.T) {
	p := NewProblems(80, 24)
	p.SetSize(120, 40)
	if p.width != 120 {
		t.Fatalf("width = %d, want 120", p.width)
	}
	if p.height != 40 {
		t.Fatalf("height = %d, want 40", p.height)
	}
}

func TestProblemsViewNoProblems(t *testing.T) {
	p := NewProblems(80, 24)
	view := p.View()
	if !strings.Contains(view, "No problems detected") {
		t.Fatal("should show 'No problems detected' when empty")
	}
}

func TestProblemsViewWithProblems(t *testing.T) {
	p := NewProblems(100, 30)
	problems := []gastown.Problem{
		{Type: "stalled", Agent: gastown.AgentRuntime{Name: "Toast", Role: "polecat"}, Detail: "Has work but idle", Severity: "warn"},
		{Type: "zombie", Agent: gastown.AgentRuntime{Name: "Stale", Role: "polecat"}, Detail: "Not running but has hooked work", Severity: "error"},
	}
	p.SetProblems(problems)

	view := p.View()
	if !strings.Contains(view, "PROBLEMS (2 detected)") {
		t.Fatal("should show problem count in header")
	}
	if !strings.Contains(view, "STALLED") {
		t.Fatal("should show STALLED type")
	}
	if !strings.Contains(view, "ZOMBIE") {
		t.Fatal("should show ZOMBIE type")
	}
	if !strings.Contains(view, "Toast") {
		t.Fatal("should show agent name Toast")
	}
	if !strings.Contains(view, "Stale") {
		t.Fatal("should show agent name Stale")
	}
}

func TestProblemsCursor(t *testing.T) {
	p := NewProblems(100, 30)
	problems := []gastown.Problem{
		{Type: "stalled", Agent: gastown.AgentRuntime{Name: "A"}, Severity: "warn"},
		{Type: "backoff", Agent: gastown.AgentRuntime{Name: "B"}, Severity: "warn"},
		{Type: "zombie", Agent: gastown.AgentRuntime{Name: "C"}, Severity: "error"},
	}
	p.SetProblems(problems)

	if p.cursor != 0 {
		t.Fatalf("initial cursor = %d, want 0", p.cursor)
	}

	// Move down
	p, _ = p.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if p.cursor != 1 {
		t.Fatalf("after j, cursor = %d, want 1", p.cursor)
	}

	// Move down
	p, _ = p.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if p.cursor != 2 {
		t.Fatalf("after j j, cursor = %d, want 2", p.cursor)
	}

	// Can't go past end
	p, _ = p.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if p.cursor != 2 {
		t.Fatalf("cursor should clamp at end, got %d", p.cursor)
	}

	// Move up
	p, _ = p.Update(tea.KeyPressMsg{Code: 'k', Text: "k"})
	if p.cursor != 1 {
		t.Fatalf("after k, cursor = %d, want 1", p.cursor)
	}

	// Jump to top
	p, _ = p.Update(tea.KeyPressMsg{Code: 'g', Text: "g"})
	if p.cursor != 0 {
		t.Fatalf("after g, cursor = %d, want 0", p.cursor)
	}

	// Jump to bottom
	p, _ = p.Update(tea.KeyPressMsg{Code: 'G', Text: "G"})
	if p.cursor != 2 {
		t.Fatalf("after G, cursor = %d, want 2", p.cursor)
	}
}

func TestProblemsCursorClamp(t *testing.T) {
	p := NewProblems(100, 30)
	problems := []gastown.Problem{
		{Type: "a", Agent: gastown.AgentRuntime{Name: "1"}, Severity: "warn"},
		{Type: "b", Agent: gastown.AgentRuntime{Name: "2"}, Severity: "warn"},
		{Type: "c", Agent: gastown.AgentRuntime{Name: "3"}, Severity: "warn"},
	}
	p.SetProblems(problems)
	p.cursor = 2

	// Reduce to 1 problem — cursor should clamp
	p.SetProblems([]gastown.Problem{
		{Type: "a", Agent: gastown.AgentRuntime{Name: "1"}, Severity: "warn"},
	})
	if p.cursor != 0 {
		t.Fatalf("cursor should clamp to 0, got %d", p.cursor)
	}
}

func TestProblemsActionNudge(t *testing.T) {
	p := NewProblems(100, 30)
	problems := []gastown.Problem{
		{Type: "stalled", Agent: gastown.AgentRuntime{Name: "Toast", Role: "polecat", Address: "beads/toast"}, Severity: "warn"},
	}
	p.SetProblems(problems)

	p, cmd := p.Update(tea.KeyPressMsg{Code: 'n', Text: "n"})
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
	if action.Agent.Name != "Toast" {
		t.Fatalf("expected agent 'Toast', got %q", action.Agent.Name)
	}
}

func TestProblemsActionHandoff(t *testing.T) {
	p := NewProblems(100, 30)
	problems := []gastown.Problem{
		{Type: "stalled", Agent: gastown.AgentRuntime{Name: "Toast", Role: "polecat"}, Severity: "warn"},
	}
	p.SetProblems(problems)

	_, cmd := p.Update(tea.KeyPressMsg{Code: 'h', Text: "h"})
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

func TestProblemsActionDecommission(t *testing.T) {
	p := NewProblems(100, 30)
	problems := []gastown.Problem{
		{Type: "zombie", Agent: gastown.AgentRuntime{Name: "Toast", Role: "polecat"}, Severity: "error"},
	}
	p.SetProblems(problems)

	_, cmd := p.Update(tea.KeyPressMsg{Code: 'K', Text: "K"})
	if cmd == nil {
		t.Fatal("expected cmd from decommission action")
	}
	msg := cmd()
	action, ok := msg.(GasTownActionMsg)
	if !ok {
		t.Fatalf("expected GasTownActionMsg, got %T", msg)
	}
	if action.Type != "decommission" {
		t.Fatalf("expected type 'decommission', got %q", action.Type)
	}
}

func TestProblemsDecommissionOnlyPolecat(t *testing.T) {
	p := NewProblems(100, 30)
	problems := []gastown.Problem{
		{Type: "stalled", Agent: gastown.AgentRuntime{Name: "witness", Role: "witness"}, Severity: "warn"},
	}
	p.SetProblems(problems)

	_, cmd := p.Update(tea.KeyPressMsg{Code: 'K', Text: "K"})
	if cmd != nil {
		t.Fatal("expected no cmd for decommission on non-polecat")
	}
}

func TestProblemsViewDeadRig(t *testing.T) {
	p := NewProblems(100, 30)
	problems := []gastown.Problem{
		{
			Type:     "dead_rig",
			Detail:   "Rig has 0 polecats — 2 issues left without an agent",
			Severity: "error",
			RigName:  "mardi_gras",
			Orphans: []gastown.OrphanedIssue{
				{IssueID: "mg-001", Title: "Fix auth", AgentName: "obsidian"},
				{IssueID: "mg-002", Title: "Add tests", AgentName: "quartz"},
			},
		},
	}
	p.SetProblems(problems)

	view := p.View()
	if !strings.Contains(view, "DEAD_RIG") {
		t.Fatal("should show DEAD_RIG type label")
	}
	if !strings.Contains(view, "mardi_gras") {
		t.Fatal("should show rig name")
	}
	if !strings.Contains(view, "mg-001") {
		t.Fatal("should show orphan issue ID mg-001")
	}
	if !strings.Contains(view, "mg-002") {
		t.Fatal("should show orphan issue ID mg-002")
	}
	if !strings.Contains(view, "obsidian") {
		t.Fatal("should show dead agent name")
	}
	if !strings.Contains(view, "R recover rig") {
		t.Fatal("should show recovery hint when dead_rig present")
	}
}

func TestProblemsRecoveryAction(t *testing.T) {
	p := NewProblems(100, 30)
	orphans := []gastown.OrphanedIssue{
		{IssueID: "mg-001", Title: "Fix auth", AgentName: "obsidian"},
	}
	problems := []gastown.Problem{
		{Type: "dead_rig", Severity: "error", RigName: "mardi_gras", Orphans: orphans},
	}
	p.SetProblems(problems)

	_, cmd := p.Update(tea.KeyPressMsg{Code: 'R', Text: "R"})
	if cmd == nil {
		t.Fatal("expected cmd from R on dead_rig")
	}
	msg := cmd()
	action, ok := msg.(RecoveryActionMsg)
	if !ok {
		t.Fatalf("expected RecoveryActionMsg, got %T", msg)
	}
	if action.RigName != "mardi_gras" {
		t.Fatalf("expected rig mardi_gras, got %q", action.RigName)
	}
	if len(action.Orphans) != 1 {
		t.Fatalf("expected 1 orphan, got %d", len(action.Orphans))
	}
}

func TestProblemsRecoveryActionOnlyDeadRig(t *testing.T) {
	p := NewProblems(100, 30)
	problems := []gastown.Problem{
		{Type: "stalled", Agent: gastown.AgentRuntime{Name: "Toast"}, Severity: "warn"},
	}
	p.SetProblems(problems)

	_, cmd := p.Update(tea.KeyPressMsg{Code: 'R', Text: "R"})
	if cmd != nil {
		t.Fatal("expected no cmd from R on non-dead_rig problem")
	}
}

func TestProblemsNoActionWhenEmpty(t *testing.T) {
	p := NewProblems(100, 30)

	_, cmd := p.Update(tea.KeyPressMsg{Code: 'n', Text: "n"})
	if cmd != nil {
		t.Fatal("expected no cmd when no problems")
	}
}

func TestProblemsHints(t *testing.T) {
	p := NewProblems(100, 30)
	problems := []gastown.Problem{
		{Type: "stalled", Agent: gastown.AgentRuntime{Name: "Toast"}, Severity: "warn"},
	}
	p.SetProblems(problems)

	view := p.View()
	if !strings.Contains(view, "nudge") {
		t.Fatal("view should contain hint 'nudge'")
	}
	if !strings.Contains(view, "handoff") {
		t.Fatal("view should contain hint 'handoff'")
	}
	if !strings.Contains(view, "decommission") {
		t.Fatal("view should contain hint 'decommission'")
	}
}

func TestProblemsEnterJumpsToIssue(t *testing.T) {
	// A zombie's hooked work could not be reached from the problem (mg-6ia).
	p := NewProblems(80, 20)
	p.SetProblems([]gastown.Problem{{Type: "zombie", Agent: gastown.AgentRuntime{Name: "quartz", HookBead: "mg-004"}}})
	_, cmd := p.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("enter should jump to the problem's issue")
	}
	if msg, ok := cmd().(ProblemJumpMsg); !ok || msg.IssueID != "mg-004" {
		t.Fatalf("msg = %+v", msg)
	}
}
