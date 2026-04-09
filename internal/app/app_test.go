package app

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/matt-wright86/mardi-gras/internal/data"
	"github.com/matt-wright86/mardi-gras/internal/gastown"
)

func testIssue(id string, status data.Status) data.Issue {
	now := time.Now()
	return data.Issue{
		ID:        id,
		Title:     id,
		Status:    status,
		Priority:  data.PriorityMedium,
		IssueType: data.TypeTask,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func TestFileChangedMsgPreservesSelectionAndClosedState(t *testing.T) {
	issues := []data.Issue{
		testIssue("open-1", data.StatusOpen),
		testIssue("open-2", data.StatusOpen),
		testIssue("closed-1", data.StatusClosed),
	}

	m := New(issues, data.Source{}, data.DefaultBlockingTypes)
	m.startedAt = time.Now().Add(-time.Second) // bypass startup guard
	model, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	got := model.(Model)

	// Move selection to second open issue.
	model, _ = got.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	got = model.(Model)
	if got.parade.SelectedIssue == nil || got.parade.SelectedIssue.ID != "open-2" {
		t.Fatalf("expected selected issue open-2 before refresh, got %+v", got.parade.SelectedIssue)
	}

	// Expand closed section.
	model, _ = got.Update(tea.KeyPressMsg{Code: 'c', Text: "c"})
	got = model.(Model)
	if !got.parade.ShowClosed {
		t.Fatal("expected closed section expanded before refresh")
	}

	// Simulate file refresh with same issues.
	model, _ = got.Update(data.FileChangedMsg{Issues: issues})
	got = model.(Model)

	if !got.parade.ShowClosed {
		t.Fatal("expected closed section to remain expanded after refresh")
	}
	if got.parade.SelectedIssue == nil || got.parade.SelectedIssue.ID != "open-2" {
		t.Fatalf("expected selected issue open-2 after refresh, got %+v", got.parade.SelectedIssue)
	}
}

func TestFileChangedMsgAppliesPendingSelectionOverride(t *testing.T) {
	issues := []data.Issue{
		testIssue("open-1", data.StatusOpen),
		testIssue("open-2", data.StatusOpen),
	}

	m := New(issues, data.Source{}, data.DefaultBlockingTypes)
	m.startedAt = time.Now().Add(-time.Second)
	model, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	got := model.(Model)

	if got.parade.SelectedIssue == nil || got.parade.SelectedIssue.ID != "open-1" {
		t.Fatalf("expected initial selection open-1, got %+v", got.parade.SelectedIssue)
	}

	got.pendingSelectID = "open-3"
	refreshed := []data.Issue{
		testIssue("open-2", data.StatusOpen),
		testIssue("open-3", data.StatusOpen),
	}
	model, _ = got.Update(data.FileChangedMsg{Issues: refreshed})
	got = model.(Model)

	if got.parade.SelectedIssue == nil || got.parade.SelectedIssue.ID != "open-3" {
		t.Fatalf("expected pending selection open-3 after refresh, got %+v", got.parade.SelectedIssue)
	}
	if got.pendingSelectID != "" {
		t.Fatalf("expected pendingSelectID to be cleared, got %q", got.pendingSelectID)
	}
}

func TestInitialLayoutExcludesHiddenTypesButKeepsDetailIssueMap(t *testing.T) {
	issues := []data.Issue{
		testIssue("open-1", data.StatusOpen),
		testIssue("epic-1", data.StatusOpen),
	}
	issues[1].IssueType = data.TypeEpic

	m := New(issues, data.Source{}, data.DefaultBlockingTypes, map[string]bool{"epic": true})
	model, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	got := model.(Model)

	for _, item := range got.parade.Items {
		if item.Issue != nil && item.Issue.ID == "epic-1" {
			t.Fatal("expected excluded epic to be hidden from parade")
		}
	}
	if _, ok := got.detail.IssueMap["epic-1"]; !ok {
		t.Fatal("expected excluded epic to remain in detail issue map")
	}
}

func TestFileChangedMsgQueuesDetailRefetchesForPendingSelection(t *testing.T) {
	issues := []data.Issue{
		testIssue("open-1", data.StatusOpen),
		testIssue("open-2", data.StatusOpen),
	}

	m := New(issues, data.Source{}, data.DefaultBlockingTypes)
	m.startedAt = time.Now().Add(-time.Second)
	model, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	got := model.(Model)

	got.pendingSelectID = "open-2"
	got.gtEnv.Available = true
	got.gtPollInFlight = true     // suppress gt status polling
	got.patrolScanInFlight = true // suppress patrol scan polling
	got.activeAgents["open-2"] = "Toast"
	got.detail.CommentsIssueID = "open-1"
	got.detail.Comments = []gastown.Comment{{Author: "alpha", Body: "cached"}}
	got.detail.MoleculeIssueID = "open-1"
	got.detail.MoleculeDAG = &gastown.DAGInfo{}
	got.detail.MoleculeProgress = &gastown.MoleculeProgress{}
	got.detail.RichIssueID = "open-1"

	model, cmd := got.Update(data.FileChangedMsg{Issues: issues})
	got = model.(Model)

	if got.parade.SelectedIssue == nil || got.parade.SelectedIssue.ID != "open-2" {
		t.Fatalf("expected selection open-2 after refresh, got %+v", got.parade.SelectedIssue)
	}
	if got.detail.CommentsIssueID != "" {
		t.Fatalf("expected CommentsIssueID to be cleared on selection change, got %q", got.detail.CommentsIssueID)
	}
	if got.detail.MoleculeIssueID != "" {
		t.Fatalf("expected MoleculeIssueID to be cleared on selection change, got %q", got.detail.MoleculeIssueID)
	}
	if got.detail.RichIssueID != "" {
		t.Fatalf("expected RichIssueID to be cleared on selection change, got %q", got.detail.RichIssueID)
	}
	if cmd == nil {
		t.Fatal("expected reload command batch")
	}

	msg := cmd()
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		t.Fatalf("expected tea.BatchMsg, got %T", msg)
	}

	nonNil := 0
	for _, subcmd := range batch {
		if subcmd != nil {
			nonNil++
		}
	}
	if nonNil != 3 {
		t.Fatalf("expected 3 detail refetch commands, got %d", nonNil)
	}
}

func TestFilteringModeAcceptsTypedInput(t *testing.T) {
	issues := []data.Issue{
		testIssue("alpha-1", data.StatusOpen),
		testIssue("beta-1", data.StatusOpen),
	}

	m := New(issues, data.Source{}, data.DefaultBlockingTypes)
	m.startedAt = time.Now().Add(-time.Second) // bypass startup guard
	model, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	got := model.(Model)

	model, _ = got.Update(tea.KeyPressMsg{Code: '/', Text: "/"})
	got = model.(Model)
	if !got.filtering {
		t.Fatal("expected filtering mode to be active after pressing /")
	}

	model, _ = got.Update(tea.KeyPressMsg{Code: 'b', Text: "b"})
	got = model.(Model)
	if got.filterInput.Value() != "b" {
		t.Fatalf("expected filter input value %q, got %q", "b", got.filterInput.Value())
	}
}

func TestFilteringModeQStillQuits(t *testing.T) {
	issues := []data.Issue{
		testIssue("alpha-1", data.StatusOpen),
	}

	m := New(issues, data.Source{}, data.DefaultBlockingTypes)
	m.startedAt = time.Now().Add(-time.Second) // bypass startup guard
	model, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	got := model.(Model)

	model, _ = got.Update(tea.KeyPressMsg{Code: '/', Text: "/"})
	got = model.(Model)
	if !got.filtering {
		t.Fatal("expected filtering mode to be active after pressing /")
	}

	_, cmd := got.Update(tea.KeyPressMsg{Code: 'q', Text: "q"})
	if cmd == nil {
		t.Fatal("expected quit command when pressing q in filtering mode")
	}

	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Fatalf("expected tea.QuitMsg from quit command, got %T", msg)
	}
}

func TestFileChangedMsgDeletedSelectedIssue(t *testing.T) {
	issues := []data.Issue{
		testIssue("alpha-1", data.StatusOpen),
		testIssue("alpha-2", data.StatusOpen),
		testIssue("alpha-3", data.StatusOpen),
	}

	m := New(issues, data.Source{}, data.DefaultBlockingTypes)
	m.startedAt = time.Now().Add(-time.Second) // bypass startup guard
	model, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	got := model.(Model)

	// Move to alpha-2.
	model, _ = got.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	got = model.(Model)
	if got.parade.SelectedIssue == nil || got.parade.SelectedIssue.ID != "alpha-2" {
		t.Fatalf("expected selected issue alpha-2 before refresh, got %+v", got.parade.SelectedIssue)
	}

	// Simulate a file refresh that removes alpha-2.
	reduced := []data.Issue{
		testIssue("alpha-1", data.StatusOpen),
		testIssue("alpha-3", data.StatusOpen),
	}
	model, _ = got.Update(data.FileChangedMsg{Issues: reduced})
	got = model.(Model)

	// Selection must have moved to a valid, non-nil issue that is not alpha-2.
	if got.parade.SelectedIssue == nil {
		t.Fatal("expected non-nil selection after deleted-issue refresh")
	}
	if got.parade.SelectedIssue.ID == "alpha-2" {
		t.Fatal("expected selection to move away from deleted alpha-2")
	}

	// The transient flag must be cleared after the handler fires the toast.
	if got.selectionLost {
		t.Fatal("expected selectionLost to be cleared after FileChangedMsg handling")
	}
	if got.lostIssueID != "" {
		t.Fatalf("expected lostIssueID to be cleared, got %q", got.lostIssueID)
	}
}

func TestHelpCanOpenFromFilteringMode(t *testing.T) {
	issues := []data.Issue{
		testIssue("alpha-1", data.StatusOpen),
	}

	m := New(issues, data.Source{}, data.DefaultBlockingTypes)
	m.startedAt = time.Now().Add(-time.Second) // bypass startup guard
	model, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	got := model.(Model)

	model, _ = got.Update(tea.KeyPressMsg{Code: '/', Text: "/"})
	got = model.(Model)
	if !got.filtering {
		t.Fatal("expected filtering mode to be active after pressing /")
	}

	model, _ = got.Update(tea.KeyPressMsg{Code: '?', Text: "?"})
	got = model.(Model)
	if !got.showHelp {
		t.Fatal("expected help overlay to open from filtering mode")
	}
	if !got.filtering {
		t.Fatal("expected filtering mode state to be preserved while help is open")
	}

	// Closing help should return to prior mode.
	model, _ = got.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	got = model.(Model)
	if got.showHelp {
		t.Fatal("expected help overlay to close on esc")
	}
	if !got.filtering {
		t.Fatal("expected filtering mode to resume after closing help")
	}
}
