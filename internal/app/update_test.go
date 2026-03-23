package app

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/matt-wright86/mardi-gras/internal/components"
	"github.com/matt-wright86/mardi-gras/internal/data"
)

// ---------------------------------------------------------------------------
// TestToastDismissMsg
// ---------------------------------------------------------------------------

func TestToastDismissMsg(t *testing.T) {
	issues := []data.Issue{testIssue("open-1", data.StatusOpen)}
	m := New(issues, data.Source{}, data.DefaultBlockingTypes)
	model, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	got := model.(Model)

	// Set up a toast so there's something to dismiss.
	got.toast.Message = "hello"

	model, _ = got.Update(components.ToastDismissMsg{})
	got = model.(Model)

	if got.toast.Message != "" {
		t.Fatalf("expected toast.Message to be empty after ToastDismissMsg, got %q", got.toast.Message)
	}
}

// ---------------------------------------------------------------------------
// TestMutateResultSuccess
// ---------------------------------------------------------------------------

func TestMutateResultSuccess(t *testing.T) {
	issues := []data.Issue{testIssue("open-1", data.StatusOpen)}
	m := New(issues, data.Source{}, data.DefaultBlockingTypes)
	model, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	got := model.(Model)

	model, _ = got.Update(mutateResultMsg{issueID: "open-1", action: "in_progress"})
	got = model.(Model)

	if got.toast.Message == "" {
		t.Fatal("expected toast.Message to be non-empty after successful mutateResultMsg")
	}
}

// ---------------------------------------------------------------------------
// TestMutateResultClosed
// ---------------------------------------------------------------------------

func TestMutateResultClosed(t *testing.T) {
	issues := []data.Issue{testIssue("open-1", data.StatusOpen)}
	m := New(issues, data.Source{}, data.DefaultBlockingTypes)
	model, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	got := model.(Model)

	model, _ = got.Update(mutateResultMsg{issueID: "open-1", action: "closed"})
	got = model.(Model)

	if !got.confetti.Active() {
		t.Fatal("expected confetti to be active after closing an issue")
	}
}

func TestMutateResultClosedNoAnimations(t *testing.T) {
	issues := []data.Issue{testIssue("open-1", data.StatusOpen)}
	m := NewWithGuard(issues, data.Source{}, data.DefaultBlockingTypes, nil, true)
	model, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	got := model.(Model)

	model, _ = got.Update(mutateResultMsg{issueID: "open-1", action: "closed"})
	got = model.(Model)

	if got.confetti.Active() {
		t.Fatal("expected confetti to be inactive when noAnimations is true")
	}
	if got.toast.Message == "" {
		t.Fatal("expected toast notification after closing an issue with noAnimations")
	}
}

func TestMutateClaimNextResultSuccess(t *testing.T) {
	issues := []data.Issue{testIssue("open-1", data.StatusOpen)}
	m := New(issues, data.Source{}, data.DefaultBlockingTypes)
	model, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	got := model.(Model)
	got.detail.RichIssueID = "open-1"

	model, _ = got.Update(mutateResultMsg{issueID: "open-1", action: "closed → claimed open-2", claimedID: "open-2"})
	got = model.(Model)

	if got.pendingSelectID != "open-2" {
		t.Fatalf("pendingSelectID = %q, want %q", got.pendingSelectID, "open-2")
	}
	if got.detail.RichIssueID != "" {
		t.Fatalf("RichIssueID = %q, want empty string", got.detail.RichIssueID)
	}
	if !got.confetti.Active() {
		t.Fatal("expected confetti to be active after claim-next close")
	}
	if !strings.Contains(got.toast.Message, "open-1") || !strings.Contains(got.toast.Message, "claimed open-2") {
		t.Fatalf("unexpected toast message: %q", got.toast.Message)
	}
}

func TestMutateClaimNextResultNoReadyWork(t *testing.T) {
	issues := []data.Issue{testIssue("open-1", data.StatusOpen)}
	m := New(issues, data.Source{}, data.DefaultBlockingTypes)
	model, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	got := model.(Model)

	model, _ = got.Update(mutateResultMsg{issueID: "open-1", action: "closed (no ready work)"})
	got = model.(Model)

	if got.pendingSelectID != "" {
		t.Fatalf("pendingSelectID = %q, want empty string", got.pendingSelectID)
	}
	if !strings.Contains(got.toast.Message, "no ready work") {
		t.Fatalf("unexpected toast message: %q", got.toast.Message)
	}
}

// ---------------------------------------------------------------------------
// TestMutateResultError
// ---------------------------------------------------------------------------

func TestMutateResultError(t *testing.T) {
	issues := []data.Issue{testIssue("open-1", data.StatusOpen)}
	m := New(issues, data.Source{}, data.DefaultBlockingTypes)
	model, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	got := model.(Model)

	model, _ = got.Update(mutateResultMsg{issueID: "open-1", action: "test", err: fmt.Errorf("fail")})
	got = model.(Model)

	if got.toast.Level != components.ToastError {
		t.Fatalf("expected toast.Level to be ToastError, got %d", got.toast.Level)
	}
}

// ---------------------------------------------------------------------------
// TestChangeIndicatorExpired
// ---------------------------------------------------------------------------

func TestChangeIndicatorExpired(t *testing.T) {
	issues := []data.Issue{testIssue("open-1", data.StatusOpen)}
	m := New(issues, data.Source{}, data.DefaultBlockingTypes)
	model, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	got := model.(Model)

	// Populate changedIDs with some data.
	got.changedIDs["open-1"] = true

	model, _ = got.Update(changeIndicatorExpiredMsg{})
	got = model.(Model)

	if len(got.changedIDs) != 0 {
		t.Fatalf("expected changedIDs to be empty after changeIndicatorExpiredMsg, got %d entries", len(got.changedIDs))
	}
}

// ---------------------------------------------------------------------------
// TestCreateFormResultCancelled
// ---------------------------------------------------------------------------

func TestCreateFormResultCancelled(t *testing.T) {
	issues := []data.Issue{testIssue("open-1", data.StatusOpen)}
	m := New(issues, data.Source{}, data.DefaultBlockingTypes)
	model, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	got := model.(Model)

	model, _ = got.Update(components.CreateFormResult{Cancelled: true})
	got = model.(Model)

	if got.creating {
		t.Fatal("expected creating to be false after cancelled CreateFormResult")
	}
}

// ---------------------------------------------------------------------------
// TestCreateFormResultSubmit
// ---------------------------------------------------------------------------

func TestCreateFormResultSubmit(t *testing.T) {
	issues := []data.Issue{testIssue("open-1", data.StatusOpen)}
	m := New(issues, data.Source{}, data.DefaultBlockingTypes)
	model, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	got := model.(Model)

	got.creating = true

	model, cmd := got.Update(components.CreateFormResult{Title: "T"})
	got = model.(Model)

	if got.creating {
		t.Fatal("expected creating to be false after submitted CreateFormResult")
	}
	if cmd == nil {
		t.Fatal("expected non-nil cmd after submitted CreateFormResult")
	}
}

// ---------------------------------------------------------------------------
// TestViewNotReady
// ---------------------------------------------------------------------------

func TestViewNotReady(t *testing.T) {
	issues := []data.Issue{testIssue("open-1", data.StatusOpen)}
	m := New(issues, data.Source{}, data.DefaultBlockingTypes)

	// Do NOT send WindowSizeMsg, so ready remains false.
	output := m.View()
	if !strings.Contains(output.Content, "Loading...") {
		t.Fatalf("expected View() to contain %q when not ready, got %q", "Loading...", output.Content)
	}
}

// ---------------------------------------------------------------------------
// TestViewReady
// ---------------------------------------------------------------------------

func TestViewReady(t *testing.T) {
	issues := []data.Issue{testIssue("open-1", data.StatusOpen)}
	m := New(issues, data.Source{}, data.DefaultBlockingTypes)
	model, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	got := model.(Model)

	output := got.View()
	if output.Content == "" {
		t.Fatal("expected View() to return non-empty output when ready")
	}
	if strings.Contains(output.Content, "Loading...") {
		t.Fatal("expected View() NOT to contain \"Loading...\" when ready")
	}
}

// ---------------------------------------------------------------------------
// TestViewWithHelp
// ---------------------------------------------------------------------------

func TestViewWithHelp(t *testing.T) {
	issues := []data.Issue{testIssue("open-1", data.StatusOpen)}
	m := New(issues, data.Source{}, data.DefaultBlockingTypes)
	model, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	got := model.(Model)

	normalView := got.View()

	got.showHelp = true
	helpView := got.View()

	if normalView.Content == helpView.Content {
		t.Fatal("expected View() output to differ when showHelp is true")
	}
}

// ---------------------------------------------------------------------------
// TestBuildPaletteCommandsGasTown
// ---------------------------------------------------------------------------

func TestBuildPaletteCommandsGasTown(t *testing.T) {
	issues := []data.Issue{testIssue("open-1", data.StatusOpen)}
	m := New(issues, data.Source{}, data.DefaultBlockingTypes)
	model, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	got := model.(Model)

	// Baseline without Gas Town.
	got.gtEnv.Available = false
	got.agentAvail = false
	baseCmds := got.buildPaletteCommands()
	baseLen := len(baseCmds)

	// Enable Gas Town.
	got.gtEnv.Available = true
	gtCmds := got.buildPaletteCommands()

	if len(gtCmds) <= baseLen {
		t.Fatalf("expected more palette commands with gastown (got %d, base %d)", len(gtCmds), baseLen)
	}

	// Verify sling/nudge actions are present.
	foundSling := false
	foundNudge := false
	foundAssign := false
	for _, cmd := range gtCmds {
		if cmd.Action == components.ActionSlingFormula {
			foundSling = true
		}
		if cmd.Action == components.ActionNudgeAgent {
			foundNudge = true
		}
		if cmd.Action == components.ActionAssign {
			foundAssign = true
		}
	}
	if !foundSling {
		t.Error("expected ActionSlingFormula in gastown palette commands")
	}
	if !foundNudge {
		t.Error("expected ActionNudgeAgent in gastown palette commands")
	}
	if !foundAssign {
		t.Error("expected ActionAssign in gastown palette commands")
	}
}

// ---------------------------------------------------------------------------
// TestExecutePaletteActions
// ---------------------------------------------------------------------------

func TestExecutePaletteActions(t *testing.T) {
	tests := []struct {
		name   string
		action components.PaletteAction
		check  func(t *testing.T, m Model, cmd tea.Cmd)
	}{
		{
			name:   "ActionHelp sets showHelp",
			action: components.ActionHelp,
			check: func(t *testing.T, m Model, cmd tea.Cmd) {
				if !m.showHelp {
					t.Fatal("expected showHelp to be true after ActionHelp")
				}
			},
		},
		{
			name:   "ActionFilter sets filtering",
			action: components.ActionFilter,
			check: func(t *testing.T, m Model, cmd tea.Cmd) {
				if !m.filtering {
					t.Fatal("expected filtering to be true after ActionFilter")
				}
			},
		},
		{
			name:   "ActionToggleFocus flips focusMode",
			action: components.ActionToggleFocus,
			check: func(t *testing.T, m Model, cmd tea.Cmd) {
				if !m.focusMode {
					t.Fatal("expected focusMode to be true after ActionToggleFocus (was false)")
				}
			},
		},
		{
			name:   "ActionQuit produces QuitMsg",
			action: components.ActionQuit,
			check: func(t *testing.T, m Model, cmd tea.Cmd) {
				if cmd == nil {
					t.Fatal("expected non-nil cmd from ActionQuit")
				}
				msg := cmd()
				if _, ok := msg.(tea.QuitMsg); !ok {
					t.Fatalf("expected tea.QuitMsg, got %T", msg)
				}
			},
		},
		{
			name:   "ActionNewIssue sets creating",
			action: components.ActionNewIssue,
			check: func(t *testing.T, m Model, cmd tea.Cmd) {
				if !m.creating {
					t.Fatal("expected creating to be true after ActionNewIssue")
				}
			},
		},
		{
			name:   "ActionAddNote sets quick action mode",
			action: components.ActionAddNote,
			check: func(t *testing.T, m Model, cmd tea.Cmd) {
				if m.qaMode != "note" {
					t.Fatalf("expected qaMode %q after ActionAddNote, got %q", "note", m.qaMode)
				}
			},
		},
		{
			name:   "ActionAssign sets creating",
			action: components.ActionAssign,
			check: func(t *testing.T, m Model, cmd tea.Cmd) {
				if !m.creating {
					t.Fatal("expected creating to be true after ActionAssign")
				}
			},
		},
		{
			name:   "ActionToggleClosed flips ShowClosed",
			action: components.ActionToggleClosed,
			check: func(t *testing.T, m Model, cmd tea.Cmd) {
				if !m.parade.ShowClosed {
					t.Fatal("expected parade.ShowClosed to be true after ActionToggleClosed (was false)")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issues := []data.Issue{testIssue("open-1", data.StatusOpen)}
			m := New(issues, data.Source{}, data.DefaultBlockingTypes)
			model, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
			got := model.(Model)
			if tt.action == components.ActionAssign {
				got.gtEnv.Available = true
			}

			result, cmd := got.executePaletteAction(tt.action)
			got = result.(Model)

			tt.check(t, got, cmd)
		})
	}
}

// ---------------------------------------------------------------------------
// TestConvoyCreateFromEpic
// ---------------------------------------------------------------------------

func TestConvoyCreateFromEpic(t *testing.T) {
	epic := testIssue("mg-100", data.StatusOpen)
	epic.IssueType = data.TypeEpic
	issues := []data.Issue{epic, testIssue("open-1", data.StatusOpen)}
	m := New(issues, data.Source{}, data.DefaultBlockingTypes)
	model, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	got := model.(Model)
	got.gtEnv.Available = true

	// Simulate pressing C on the epic
	got.parade.SelectedIssue = &issues[0]
	result, _ := got.handleKey(tea.KeyPressMsg{Code: 'C', Text: "C"})
	got = result.(Model)

	if !got.convoyCreating {
		t.Fatal("expected convoyCreating to be true")
	}
	if got.convoyEpicID != "mg-100" {
		t.Fatalf("expected convoyEpicID = %q, got %q", "mg-100", got.convoyEpicID)
	}
	if len(got.convoyIssueIDs) != 0 {
		t.Fatalf("expected empty convoyIssueIDs for epic, got %v", got.convoyIssueIDs)
	}
	if !strings.Contains(got.convoyInput.Placeholder, "epic") {
		t.Fatalf("expected placeholder to mention epic, got %q", got.convoyInput.Placeholder)
	}
}

func TestConvoyCreateFromNonEpic(t *testing.T) {
	issues := []data.Issue{testIssue("open-1", data.StatusOpen)}
	m := New(issues, data.Source{}, data.DefaultBlockingTypes)
	model, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	got := model.(Model)
	got.gtEnv.Available = true

	// Simulate pressing C on a regular issue
	got.parade.SelectedIssue = &issues[0]
	result, _ := got.handleKey(tea.KeyPressMsg{Code: 'C', Text: "C"})
	got = result.(Model)

	if !got.convoyCreating {
		t.Fatal("expected convoyCreating to be true")
	}
	if got.convoyEpicID != "" {
		t.Fatalf("expected empty convoyEpicID for non-epic, got %q", got.convoyEpicID)
	}
	if len(got.convoyIssueIDs) != 1 || got.convoyIssueIDs[0] != "open-1" {
		t.Fatalf("expected convoyIssueIDs = [open-1], got %v", got.convoyIssueIDs)
	}
}
