package components

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/matt-wright86/mardi-gras/internal/gastown"
)

func TestRecoveryDialogCancel(t *testing.T) {
	orphans := []gastown.OrphanedIssue{
		{IssueID: "mg-001", Title: "Fix auth", AgentName: "obsidian"},
	}
	rd := NewRecoveryDialog("mardi_gras", orphans, 80, 24)

	_, cmd := rd.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("expected cmd from esc")
	}
	msg := cmd()
	result, ok := msg.(RecoveryDialogResult)
	if !ok {
		t.Fatalf("expected RecoveryDialogResult, got %T", msg)
	}
	if !result.Cancelled {
		t.Fatal("expected Cancelled=true")
	}
}

func TestRecoveryDialogCancelQ(t *testing.T) {
	rd := NewRecoveryDialog("test_rig", nil, 80, 24)
	_, cmd := rd.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	if cmd == nil {
		t.Fatal("expected cmd from q")
	}
	msg := cmd()
	result := msg.(RecoveryDialogResult)
	if !result.Cancelled {
		t.Fatal("expected Cancelled=true from q")
	}
}

func TestRecoveryDialogDefaultResling(t *testing.T) {
	orphans := []gastown.OrphanedIssue{
		{IssueID: "mg-001", Title: "Fix auth", AgentName: "obsidian"},
		{IssueID: "mg-002", Title: "Add tests", AgentName: "quartz"},
	}
	rd := NewRecoveryDialog("mardi_gras", orphans, 80, 24)

	// Enter on default selection = resling
	_, cmd := rd.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected cmd from enter")
	}
	msg := cmd()
	result, ok := msg.(RecoveryDialogResult)
	if !ok {
		t.Fatalf("expected RecoveryDialogResult, got %T", msg)
	}
	if result.Cancelled {
		t.Fatal("should not be cancelled")
	}
	if result.Mode != gastown.RecoveryResling {
		t.Fatalf("expected RecoveryResling, got %d", result.Mode)
	}
	if result.RigName != "mardi_gras" {
		t.Fatalf("expected rig mardi_gras, got %q", result.RigName)
	}
	if len(result.Orphans) != 2 {
		t.Fatalf("expected 2 orphans, got %d", len(result.Orphans))
	}
}

func TestRecoveryDialogSelectReleaseOnly(t *testing.T) {
	orphans := []gastown.OrphanedIssue{
		{IssueID: "mg-001", Title: "Fix auth"},
	}
	rd := NewRecoveryDialog("test_rig", orphans, 80, 24)

	// Move down to "Release only"
	rd, _ = rd.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if rd.modeIdx != 1 {
		t.Fatalf("modeIdx = %d, want 1", rd.modeIdx)
	}

	// Confirm
	_, cmd := rd.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected cmd from enter")
	}
	result := cmd().(RecoveryDialogResult)
	if result.Mode != gastown.RecoveryReleaseOnly {
		t.Fatalf("expected RecoveryReleaseOnly, got %d", result.Mode)
	}
}

func TestRecoveryDialogCursorClamp(t *testing.T) {
	rd := NewRecoveryDialog("test_rig", nil, 80, 24)

	// Can't go above 0
	rd, _ = rd.Update(tea.KeyPressMsg{Code: 'k', Text: "k"})
	if rd.modeIdx != 0 {
		t.Fatalf("modeIdx = %d, want 0", rd.modeIdx)
	}

	// Go to bottom
	rd, _ = rd.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if rd.modeIdx != 1 {
		t.Fatalf("modeIdx = %d, want 1", rd.modeIdx)
	}

	// Can't go past bottom
	rd, _ = rd.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if rd.modeIdx != 1 {
		t.Fatalf("modeIdx should clamp at 1, got %d", rd.modeIdx)
	}
}

func TestRecoveryDialogView(t *testing.T) {
	orphans := []gastown.OrphanedIssue{
		{IssueID: "mg-001", Title: "Fix auth", AgentName: "obsidian"},
		{IssueID: "mg-002", Title: "Add tests"},
	}
	rd := NewRecoveryDialog("mardi_gras", orphans, 100, 30)
	view := rd.View()

	checks := []struct {
		needle string
		desc   string
	}{
		{"mardi_gras", "rig name"},
		{"mg-001", "first orphan ID"},
		{"mg-002", "second orphan ID"},
		{"Fix auth", "first orphan title"},
		{"Add tests", "second orphan title"},
		{"obsidian", "dead agent name"},
		{"Re-sling", "resling mode label"},
		{"Release only", "release-only mode label"},
	}
	for _, c := range checks {
		if !strings.Contains(view, c.needle) {
			t.Fatalf("view should contain %s (%q)", c.desc, c.needle)
		}
	}
}

func TestRecoveryDialogNonKeyMsg(t *testing.T) {
	rd := NewRecoveryDialog("test_rig", nil, 80, 24)
	rd2, cmd := rd.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	if cmd != nil {
		t.Fatal("expected no cmd from non-key msg")
	}
	if rd2.modeIdx != rd.modeIdx {
		t.Fatal("state should not change on non-key msg")
	}
}
