package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/matt-wright86/mardi-gras/internal/components"
	"github.com/matt-wright86/mardi-gras/internal/data"
)

// ---------------------------------------------------------------------------
// Key state-transition tests
// ---------------------------------------------------------------------------

// setupModel creates a standard model with open and closed issues, sized to
// 100x20, ready for key dispatch tests.
func setupModel(t *testing.T) Model {
	t.Helper()
	issues := []data.Issue{
		testIssue("open-1", data.StatusOpen),
		testIssue("open-2", data.StatusOpen),
		testIssue("open-3", data.StatusOpen),
		testIssue("closed-1", data.StatusClosed),
	}
	m := New(issues, "", false, data.DefaultBlockingTypes)
	model, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	return model.(Model)
}

// ---------------------------------------------------------------------------
// 1. q quits
// ---------------------------------------------------------------------------

func TestKeyQQuits(t *testing.T) {
	got := setupModel(t)

	_, cmd := got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("expected quit command from pressing q")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Fatalf("expected tea.QuitMsg, got %T", msg)
	}
}

// ---------------------------------------------------------------------------
// 2. ? opens help
// ---------------------------------------------------------------------------

func TestKeyQuestionOpensHelp(t *testing.T) {
	got := setupModel(t)

	model, _ := got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	got = model.(Model)

	if !got.showHelp {
		t.Fatal("expected showHelp to be true after pressing ?")
	}
}

// ---------------------------------------------------------------------------
// 3. / enters filter mode
// ---------------------------------------------------------------------------

func TestKeySlashEntersFilter(t *testing.T) {
	got := setupModel(t)

	model, _ := got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	got = model.(Model)

	if !got.filtering {
		t.Fatal("expected filtering to be true after pressing /")
	}
}

// ---------------------------------------------------------------------------
// 4. tab toggles panes
// ---------------------------------------------------------------------------

func TestKeyTabTogglesPanes(t *testing.T) {
	got := setupModel(t)

	if got.activPane != PaneParade {
		t.Fatalf("expected initial activPane to be PaneParade, got %d", got.activPane)
	}

	// Tab: Parade -> Detail
	model, _ := got.Update(tea.KeyMsg{Type: tea.KeyTab})
	got = model.(Model)
	if got.activPane != PaneDetail {
		t.Fatalf("expected activPane PaneDetail after first tab, got %d", got.activPane)
	}

	// Tab: Detail -> Parade
	model, _ = got.Update(tea.KeyMsg{Type: tea.KeyTab})
	got = model.(Model)
	if got.activPane != PaneParade {
		t.Fatalf("expected activPane PaneParade after second tab, got %d", got.activPane)
	}
}

// ---------------------------------------------------------------------------
// 5. esc exits focus mode
// ---------------------------------------------------------------------------

func TestKeyEscExitsFocusMode(t *testing.T) {
	got := setupModel(t)
	got.focusMode = true

	model, _ := got.Update(tea.KeyMsg{Type: tea.KeyEsc})
	got = model.(Model)

	if got.focusMode {
		t.Fatal("expected focusMode to be false after pressing esc")
	}
}

// ---------------------------------------------------------------------------
// 6. esc moves from detail to parade
// ---------------------------------------------------------------------------

func TestKeyEscDetailToParade(t *testing.T) {
	got := setupModel(t)
	got.activPane = PaneDetail

	model, _ := got.Update(tea.KeyMsg{Type: tea.KeyEsc})
	got = model.(Model)

	if got.activPane != PaneParade {
		t.Fatalf("expected activPane PaneParade after esc from detail, got %d", got.activPane)
	}
}

// ---------------------------------------------------------------------------
// 7. f toggles focus mode
// ---------------------------------------------------------------------------

func TestKeyFTogglesFocus(t *testing.T) {
	got := setupModel(t)

	if got.focusMode {
		t.Fatal("expected focusMode to be false initially")
	}

	// Toggle ON
	model, cmd := got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	got = model.(Model)
	if !got.focusMode {
		t.Fatal("expected focusMode to be true after pressing f")
	}
	if cmd == nil {
		t.Fatal("expected non-nil cmd (toast) after pressing f")
	}

	// Toggle OFF
	model, cmd = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	got = model.(Model)
	if got.focusMode {
		t.Fatal("expected focusMode to be false after pressing f again")
	}
	if cmd == nil {
		t.Fatal("expected non-nil cmd (toast) after pressing f again")
	}
}

// ---------------------------------------------------------------------------
// 8. c toggles closed section
// ---------------------------------------------------------------------------

func TestKeyCTogglesClosed(t *testing.T) {
	got := setupModel(t)

	if got.parade.ShowClosed {
		t.Fatal("expected ShowClosed to be false initially")
	}

	model, _ := got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	got = model.(Model)
	if !got.parade.ShowClosed {
		t.Fatal("expected ShowClosed to be true after pressing c")
	}

	model, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	got = model.(Model)
	if got.parade.ShowClosed {
		t.Fatal("expected ShowClosed to be false after pressing c again")
	}
}

// ---------------------------------------------------------------------------
// 9. N opens create form
// ---------------------------------------------------------------------------

func TestKeyNOpensCreateForm(t *testing.T) {
	got := setupModel(t)

	model, _ := got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'N'}})
	got = model.(Model)

	if !got.creating {
		t.Fatal("expected creating to be true after pressing N")
	}
}

// ---------------------------------------------------------------------------
// 10. enter switches to detail pane
// ---------------------------------------------------------------------------

func TestKeyEnterSwitchesToDetail(t *testing.T) {
	got := setupModel(t)

	if got.activPane != PaneParade {
		t.Fatal("expected activPane to be PaneParade initially")
	}

	model, _ := got.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got = model.(Model)

	if got.activPane != PaneDetail {
		t.Fatalf("expected activPane PaneDetail after enter, got %d", got.activPane)
	}
}

// ---------------------------------------------------------------------------
// 11. g jumps to top (first selectable item)
// ---------------------------------------------------------------------------

func TestKeyGJumpsToTop(t *testing.T) {
	got := setupModel(t)

	// Move cursor down a few times first
	model, _ := got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	got = model.(Model)
	model, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	got = model.(Model)

	// Press g to jump to top
	model, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	got = model.(Model)

	// The cursor should be on the first non-header item
	if got.parade.Cursor >= len(got.parade.Items) {
		t.Fatal("cursor out of range")
	}
	for i := 0; i < got.parade.Cursor; i++ {
		if !got.parade.Items[i].IsHeader {
			t.Fatalf("expected cursor at first non-header item, but item %d is not a header", i)
		}
	}
	if got.parade.Items[got.parade.Cursor].IsHeader {
		t.Fatal("expected cursor to be on a non-header item after pressing g")
	}
}

// ---------------------------------------------------------------------------
// 12. G jumps to bottom (last selectable item)
// ---------------------------------------------------------------------------

func TestKeyGGJumpsToBottom(t *testing.T) {
	got := setupModel(t)

	// First expand closed so we have more items
	model, _ := got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	got = model.(Model)

	// Press G to jump to bottom
	model, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	got = model.(Model)

	// The cursor should be on the last non-header item
	if got.parade.Cursor >= len(got.parade.Items) {
		t.Fatal("cursor out of range")
	}
	if got.parade.Items[got.parade.Cursor].IsHeader {
		t.Fatal("expected cursor to be on a non-header item after pressing G")
	}
	// Verify no non-header items exist after the cursor
	for i := got.parade.Cursor + 1; i < len(got.parade.Items); i++ {
		if !got.parade.Items[i].IsHeader {
			t.Fatalf("expected no non-header items after cursor at %d, but item %d is selectable", got.parade.Cursor, i)
		}
	}
}

// ---------------------------------------------------------------------------
// 13. j/k navigation
// ---------------------------------------------------------------------------

func TestKeyJKNavigation(t *testing.T) {
	got := setupModel(t)

	startCursor := got.parade.Cursor

	// j moves down
	model, _ := got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	got = model.(Model)
	afterJ := got.parade.Cursor
	if afterJ <= startCursor {
		t.Fatalf("expected cursor to move down with j: start=%d, after=%d", startCursor, afterJ)
	}

	// k moves back up
	model, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	got = model.(Model)
	afterK := got.parade.Cursor
	if afterK >= afterJ {
		t.Fatalf("expected cursor to move up with k: afterJ=%d, afterK=%d", afterJ, afterK)
	}
}

// ---------------------------------------------------------------------------
// 14. space toggles selection
// ---------------------------------------------------------------------------

func TestKeySpaceTogglesSelect(t *testing.T) {
	got := setupModel(t)

	model, _ := got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	got = model.(Model)

	if len(got.parade.Selected) == 0 {
		t.Fatal("expected parade.Selected to be non-empty after pressing space")
	}
}

// ---------------------------------------------------------------------------
// 15. X clears selection
// ---------------------------------------------------------------------------

func TestKeyXClearsSelection(t *testing.T) {
	got := setupModel(t)

	// Select an item first
	model, _ := got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	got = model.(Model)
	if len(got.parade.Selected) == 0 {
		t.Fatal("expected parade.Selected to be non-empty after space")
	}

	// X clears all selections
	model, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'X'}})
	got = model.(Model)
	if len(got.parade.Selected) != 0 {
		t.Fatalf("expected parade.Selected to be empty after X, got %d", len(got.parade.Selected))
	}
}

// ---------------------------------------------------------------------------
// 16. J (shift+j) select + move down
// ---------------------------------------------------------------------------

func TestKeyJShiftSelectMoves(t *testing.T) {
	got := setupModel(t)

	startCursor := got.parade.Cursor

	model, _ := got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'J'}})
	got = model.(Model)

	if len(got.parade.Selected) == 0 {
		t.Fatal("expected parade.Selected to be non-empty after J")
	}
	if got.parade.Cursor <= startCursor {
		t.Fatalf("expected cursor to move down after J: start=%d, got=%d", startCursor, got.parade.Cursor)
	}
}

// ---------------------------------------------------------------------------
// 17. executePaletteAction returns cmd for valid action
// ---------------------------------------------------------------------------

func TestQuickActionReturnsCmd(t *testing.T) {
	got := setupModel(t)

	// Verify we have a selected issue
	if got.parade.SelectedIssue == nil {
		t.Fatal("expected a selected issue in the parade")
	}

	_, cmd := got.executePaletteAction(components.ActionSetInProgress)
	if cmd == nil {
		t.Fatal("expected non-nil cmd from executePaletteAction(ActionSetInProgress) with selected issue")
	}
}

// ---------------------------------------------------------------------------
// 18. quickAction with no issues is a no-op
// ---------------------------------------------------------------------------

func TestQuickActionNilIssueNoop(t *testing.T) {
	m := New([]data.Issue{}, "", false, data.DefaultBlockingTypes)
	model, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	got := model.(Model)

	_, cmd := got.quickAction(data.StatusInProgress, "in_progress")
	if cmd != nil {
		t.Fatal("expected nil cmd from quickAction with no issues")
	}
}

// ---------------------------------------------------------------------------
// 19. closeSelectedIssue returns cmd for open issue
// ---------------------------------------------------------------------------

func TestCloseSelectedIssueReturnsCmd(t *testing.T) {
	got := setupModel(t)

	if got.parade.SelectedIssue == nil {
		t.Fatal("expected a selected issue")
	}
	if got.parade.SelectedIssue.Status == data.StatusClosed {
		t.Fatal("expected selected issue to not already be closed")
	}

	_, cmd := got.closeSelectedIssue()
	if cmd == nil {
		t.Fatal("expected non-nil cmd from closeSelectedIssue with open issue selected")
	}
}

// ---------------------------------------------------------------------------
// 20. setPriority returns cmd when priority differs
// ---------------------------------------------------------------------------

func TestSetPriorityReturnsCmd(t *testing.T) {
	got := setupModel(t)

	if got.parade.SelectedIssue == nil {
		t.Fatal("expected a selected issue")
	}
	// testIssue sets PriorityMedium, so setting High should produce a cmd
	if got.parade.SelectedIssue.Priority != data.PriorityMedium {
		t.Fatalf("expected selected issue priority to be PriorityMedium, got %d", got.parade.SelectedIssue.Priority)
	}

	_, cmd := got.setPriority(data.PriorityHigh)
	if cmd == nil {
		t.Fatal("expected non-nil cmd from setPriority(PriorityHigh) with medium priority issue")
	}
}

// ---------------------------------------------------------------------------
// 21. 's' key with Gas Town spawns formula list fetch
// ---------------------------------------------------------------------------

func TestKeySGasTownFetchesFormulas(t *testing.T) {
	got := setupModel(t)
	got.gtEnv.Available = true

	model, cmd := got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	got = model.(Model)

	if cmd == nil {
		t.Fatal("expected non-nil cmd from pressing s with Gas Town available")
	}
	// The formulaTarget should be set to the selected issue ID
	if got.formulaTarget == "" {
		t.Fatal("expected formulaTarget to be set after pressing s")
	}
}

// ---------------------------------------------------------------------------
// 22. 's' key without Gas Town is a no-op
// ---------------------------------------------------------------------------

func TestKeySNoGasTownNoop(t *testing.T) {
	got := setupModel(t)
	got.gtEnv.Available = false

	_, cmd := got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	if cmd != nil {
		t.Fatal("expected nil cmd from pressing s without Gas Town")
	}
}

// ---------------------------------------------------------------------------
// 23. 'n' key opens nudge input when agent active
// ---------------------------------------------------------------------------

func TestKeyNOpensNudgeInput(t *testing.T) {
	got := setupModel(t)
	got.gtEnv.Available = true
	issueID := got.parade.SelectedIssue.ID
	got.activeAgents[issueID] = "Toast"

	model, cmd := got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	got = model.(Model)

	if !got.nudging {
		t.Fatal("expected nudging to be true after pressing n with active agent")
	}
	if got.nudgeTarget != "Toast" {
		t.Fatalf("expected nudgeTarget 'Toast', got %q", got.nudgeTarget)
	}
	if cmd == nil {
		t.Fatal("expected blink cmd from nudge input")
	}
}

// ---------------------------------------------------------------------------
// 24. 'n' key no-op when no active agent
// ---------------------------------------------------------------------------

func TestKeyNNoActiveAgentNoop(t *testing.T) {
	got := setupModel(t)
	got.gtEnv.Available = true

	model, cmd := got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	got = model.(Model)

	if got.nudging {
		t.Fatal("expected nudging to be false when no active agent")
	}
	if cmd != nil {
		t.Fatal("expected nil cmd when no active agent for nudge")
	}
}

// ---------------------------------------------------------------------------
// 25. 'A' key with Gas Town dispatches unsling
// ---------------------------------------------------------------------------

func TestKeyAGasTownUnsling(t *testing.T) {
	got := setupModel(t)
	got.gtEnv.Available = true
	issueID := got.parade.SelectedIssue.ID
	got.activeAgents[issueID] = "Toast"

	_, cmd := got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'A'}})
	if cmd == nil {
		t.Fatal("expected non-nil cmd from pressing A with Gas Town and active agent")
	}
	// Execute the cmd and verify it returns unslingResultMsg
	msg := cmd()
	if _, ok := msg.(unslingResultMsg); !ok {
		t.Fatalf("expected unslingResultMsg, got %T", msg)
	}
}

// ---------------------------------------------------------------------------
// 26. 'A' key no-op when no active agent
// ---------------------------------------------------------------------------

func TestKeyANoActiveAgentNoop(t *testing.T) {
	got := setupModel(t)
	got.gtEnv.Available = true

	_, cmd := got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'A'}})
	if cmd != nil {
		t.Fatal("expected nil cmd when no active agent for A key")
	}
}

// ---------------------------------------------------------------------------
// 27. formulaListMsg opens formula palette
// ---------------------------------------------------------------------------

func TestFormulaListMsgOpensPalette(t *testing.T) {
	got := setupModel(t)
	got.formulaTarget = "open-1"

	model, cmd := got.Update(formulaListMsg{formulas: []string{"shiny", "basic"}, err: nil})
	got = model.(Model)

	if !got.formulaPicking {
		t.Fatal("expected formulaPicking to be true")
	}
	if !got.showPalette {
		t.Fatal("expected showPalette to be true")
	}
	if cmd == nil {
		t.Fatal("expected palette init cmd")
	}
}

// ---------------------------------------------------------------------------
// 28. formulaListMsg with empty formulas falls back to plain sling
// ---------------------------------------------------------------------------

func TestFormulaListMsgEmptyFallback(t *testing.T) {
	got := setupModel(t)
	got.formulaTarget = "open-1"

	model, cmd := got.Update(formulaListMsg{formulas: nil, err: nil})
	got = model.(Model)

	if got.formulaPicking {
		t.Fatal("expected formulaPicking to be false on fallback")
	}
	if cmd == nil {
		t.Fatal("expected non-nil cmd (toast + sling) on fallback")
	}
}

// ---------------------------------------------------------------------------
// 29. multi-sling 'a' key with Gas Town and selection
// ---------------------------------------------------------------------------

func TestKeyAMultiSlingWithSelection(t *testing.T) {
	got := setupModel(t)
	got.gtEnv.Available = true

	// Select current item
	model, _ := got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	got = model.(Model)
	if got.parade.SelectionCount() == 0 {
		t.Fatal("expected items to be selected")
	}

	model, cmd := got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	got = model.(Model)

	if cmd == nil {
		t.Fatal("expected non-nil cmd from multi-sling")
	}
	// Selection should be cleared
	if got.parade.SelectionCount() != 0 {
		t.Fatal("expected selection to be cleared after multi-sling")
	}
}
