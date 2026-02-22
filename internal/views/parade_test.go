package views

import (
	"testing"

	"github.com/matt-wright86/mardi-gras/internal/data"
)

// paradeIssues returns a mix of open, in_progress, and closed issues for testing.
func paradeIssues() []data.Issue {
	return []data.Issue{
		{ID: "mg-001", Title: "Rolling issue", Status: data.StatusInProgress, Priority: data.PriorityHigh, IssueType: data.TypeTask},
		{ID: "mg-002", Title: "Lined up issue", Status: data.StatusOpen, Priority: data.PriorityMedium, IssueType: data.TypeFeature},
		{ID: "mg-003", Title: "Another open issue", Status: data.StatusOpen, Priority: data.PriorityLow, IssueType: data.TypeBug},
		{ID: "mg-004", Title: "Closed issue", Status: data.StatusClosed, Priority: data.PriorityMedium, IssueType: data.TypeTask},
		{ID: "mg-005", Title: "Another closed", Status: data.StatusClosed, Priority: data.PriorityBacklog, IssueType: data.TypeChore},
	}
}

func newTestParade() Parade {
	return NewParade(paradeIssues(), 80, 20, data.DefaultBlockingTypes)
}

func TestNewParadeInitialCursor(t *testing.T) {
	p := newTestParade()

	if len(p.Items) == 0 {
		t.Fatal("expected items, got none")
	}

	item := p.Items[p.Cursor]
	if item.IsHeader || item.IsFooter {
		t.Fatalf("cursor at %d is on a non-selectable item (header=%v, footer=%v)", p.Cursor, item.IsHeader, item.IsFooter)
	}
	if item.Issue == nil {
		t.Fatal("cursor item has nil issue")
	}
}

func TestMoveDownUp(t *testing.T) {
	p := newTestParade()
	startCursor := p.Cursor
	startIssue := p.SelectedIssue

	p.MoveDown()
	if p.Cursor == startCursor {
		t.Fatal("MoveDown did not advance cursor")
	}
	if p.Items[p.Cursor].IsHeader || p.Items[p.Cursor].IsFooter {
		t.Fatal("MoveDown landed on non-selectable item")
	}
	if p.SelectedIssue == startIssue {
		t.Fatal("MoveDown did not update SelectedIssue")
	}

	downIssue := p.SelectedIssue
	p.MoveUp()
	if p.SelectedIssue == downIssue {
		t.Fatal("MoveUp did not change SelectedIssue")
	}
	if p.SelectedIssue.ID != startIssue.ID {
		t.Fatalf("MoveUp did not return to original issue: got %s, want %s", p.SelectedIssue.ID, startIssue.ID)
	}
}

func TestMoveDownClampsAtEnd(t *testing.T) {
	p := newTestParade()

	// Move to the very end
	for i := 0; i < len(p.Items)+10; i++ {
		p.MoveDown()
	}
	lastCursor := p.Cursor
	lastIssue := p.SelectedIssue

	p.MoveDown()
	if p.Cursor != lastCursor {
		t.Fatalf("MoveDown past end changed cursor: got %d, want %d", p.Cursor, lastCursor)
	}
	if p.SelectedIssue.ID != lastIssue.ID {
		t.Fatal("MoveDown past end changed SelectedIssue")
	}
}

func TestMoveUpClampsAtTop(t *testing.T) {
	p := newTestParade()
	firstCursor := p.Cursor
	firstIssue := p.SelectedIssue

	p.MoveUp()
	if p.Cursor != firstCursor {
		t.Fatalf("MoveUp past top changed cursor: got %d, want %d", p.Cursor, firstCursor)
	}
	if p.SelectedIssue.ID != firstIssue.ID {
		t.Fatal("MoveUp past top changed SelectedIssue")
	}
}

func TestToggleClosed(t *testing.T) {
	p := newTestParade()
	if p.ShowClosed {
		t.Fatal("ShowClosed should default to false")
	}

	// Count items before toggle (closed section collapsed)
	countBefore := len(p.Items)

	p.ToggleClosed()
	if !p.ShowClosed {
		t.Fatal("ToggleClosed did not flip ShowClosed to true")
	}
	if len(p.Items) <= countBefore {
		t.Fatalf("toggling closed on should add items: before=%d, after=%d", countBefore, len(p.Items))
	}

	p.ToggleClosed()
	if p.ShowClosed {
		t.Fatal("second ToggleClosed did not flip ShowClosed back to false")
	}
	if len(p.Items) != countBefore {
		t.Fatalf("toggling closed off should restore count: got=%d, want=%d", len(p.Items), countBefore)
	}
}

func TestToggleClosedPreservesSelection(t *testing.T) {
	p := newTestParade()

	// Select second issue
	p.MoveDown()
	selectedID := p.SelectedIssue.ID

	p.ToggleClosed()
	if p.SelectedIssue == nil || p.SelectedIssue.ID != selectedID {
		got := "<nil>"
		if p.SelectedIssue != nil {
			got = p.SelectedIssue.ID
		}
		t.Fatalf("ToggleClosed changed selection: got %s, want %s", got, selectedID)
	}
}

func TestToggleSelect(t *testing.T) {
	p := newTestParade()
	issueID := p.SelectedIssue.ID

	p.ToggleSelect()
	if !p.Selected[issueID] {
		t.Fatalf("ToggleSelect did not add %s to Selected", issueID)
	}
}

func TestToggleSelectDeselects(t *testing.T) {
	p := newTestParade()
	issueID := p.SelectedIssue.ID

	p.ToggleSelect()
	p.ToggleSelect()
	if p.Selected[issueID] {
		t.Fatalf("second ToggleSelect did not remove %s from Selected", issueID)
	}
}

func TestClearSelection(t *testing.T) {
	p := newTestParade()

	p.ToggleSelect()
	p.MoveDown()
	p.ToggleSelect()

	if len(p.Selected) != 2 {
		t.Fatalf("expected 2 selected, got %d", len(p.Selected))
	}

	p.ClearSelection()
	if len(p.Selected) != 0 {
		t.Fatalf("ClearSelection did not empty Selected: got %d", len(p.Selected))
	}
}

func TestSelectedIssues(t *testing.T) {
	p := newTestParade()
	firstID := p.SelectedIssue.ID
	p.ToggleSelect()

	p.MoveDown()
	secondID := p.SelectedIssue.ID
	p.ToggleSelect()

	result := p.SelectedIssues()
	if len(result) != 2 {
		t.Fatalf("expected 2 selected issues, got %d", len(result))
	}

	ids := map[string]bool{}
	for _, iss := range result {
		ids[iss.ID] = true
	}
	if !ids[firstID] || !ids[secondID] {
		t.Fatalf("SelectedIssues missing expected IDs: want %s and %s, got %v", firstID, secondID, ids)
	}
}

func TestSelectionCount(t *testing.T) {
	p := newTestParade()

	if p.SelectionCount() != 0 {
		t.Fatalf("expected 0, got %d", p.SelectionCount())
	}

	p.ToggleSelect()
	if p.SelectionCount() != 1 {
		t.Fatalf("expected 1, got %d", p.SelectionCount())
	}

	p.MoveDown()
	p.ToggleSelect()
	if p.SelectionCount() != 2 {
		t.Fatalf("expected 2, got %d", p.SelectionCount())
	}
}

func TestEnsureVisible(t *testing.T) {
	// Use a very small viewport height so scrolling is triggered
	issues := paradeIssues()
	p := NewParade(issues, 80, 3, data.DefaultBlockingTypes)

	// Move down enough to go past the viewport
	for i := 0; i < 10; i++ {
		p.MoveDown()
	}

	// Cursor should be visible: within [ScrollOffset, ScrollOffset+Height)
	if p.Cursor < p.ScrollOffset || p.Cursor >= p.ScrollOffset+p.Height {
		t.Fatalf("cursor %d not visible in viewport [%d, %d)", p.Cursor, p.ScrollOffset, p.ScrollOffset+p.Height)
	}
}
