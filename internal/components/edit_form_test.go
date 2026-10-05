package components

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/matt-wright86/mardi-gras/internal/data"
)

func TestEditFormDefaults(t *testing.T) {
	issue := data.Issue{
		ID:       "mg-42",
		Title:    "Fix login bug",
		Priority: data.PriorityHigh,
	}
	ef := NewEditForm(80, 24, &issue)
	if ef.issueID != "mg-42" {
		t.Fatalf("issueID = %q, want mg-42", ef.issueID)
	}
	if ef.titleInput.Value() != "Fix login bug" {
		t.Fatalf("title = %q, want Fix login bug", ef.titleInput.Value())
	}
	if ef.prioIdx != 1 { // P1 = index 1
		t.Fatalf("prioIdx = %d, want 1 (P1 High)", ef.prioIdx)
	}
	if ef.activeField != 0 {
		t.Fatalf("activeField = %d, want 0", ef.activeField)
	}
}

func TestEditFormPrePopulatesPriority(t *testing.T) {
	tests := []struct {
		priority data.Priority
		wantIdx  int
	}{
		{data.PriorityCritical, 0},
		{data.PriorityHigh, 1},
		{data.PriorityMedium, 2},
		{data.PriorityLow, 3},
		{data.PriorityBacklog, 4},
	}
	for _, tt := range tests {
		issue := data.Issue{ID: "mg-1", Title: "Test", Priority: tt.priority}
		ef := NewEditForm(80, 24, &issue)
		if ef.prioIdx != tt.wantIdx {
			t.Errorf("priority %d: prioIdx = %d, want %d", tt.priority, ef.prioIdx, tt.wantIdx)
		}
	}
}

func TestEditFormTabCycles(t *testing.T) {
	issue := data.Issue{ID: "mg-1", Title: "Test", Priority: data.PriorityMedium}
	ef := NewEditForm(80, 24, &issue)

	// Title → Type → Priority → Status → Description → Title (mg-nd2)
	for _, want := range []int{editFieldType, editFieldPriority, editFieldStatus, editFieldDescription, editFieldTitle} {
		ef, _ = ef.Update(tea.KeyPressMsg{Code: -2, Text: "tab"})
		if ef.activeField != want {
			t.Fatalf("activeField = %d, want %d", ef.activeField, want)
		}
	}
}

func TestEditFormShiftTabCycles(t *testing.T) {
	issue := data.Issue{ID: "mg-1", Title: "Test", Priority: data.PriorityMedium}
	ef := NewEditForm(80, 24, &issue)

	ef, _ = ef.Update(tea.KeyPressMsg{Code: -2, Text: "shift+tab"})
	if ef.activeField != editFieldDescription {
		t.Fatalf("after shift+tab, activeField = %d, want the description (wrap)", ef.activeField)
	}
}

func TestEditFormEscCancels(t *testing.T) {
	issue := data.Issue{ID: "mg-1", Title: "Test", Priority: data.PriorityMedium}
	ef := NewEditForm(80, 24, &issue)

	_, cmd := ef.Update(tea.KeyPressMsg{Code: -2, Text: "esc"})
	if cmd == nil {
		t.Fatal("expected cmd from esc")
	}
	msg := cmd()
	result, ok := msg.(EditFormResult)
	if !ok {
		t.Fatalf("expected EditFormResult, got %T", msg)
	}
	if !result.Cancelled {
		t.Fatal("expected Cancelled=true")
	}
}

func TestEditFormSubmitNoChanges(t *testing.T) {
	issue := data.Issue{ID: "mg-1", Title: "Test", Priority: data.PriorityMedium}
	ef := NewEditForm(80, 24, &issue)

	// Move to priority field, then submit
	ef.focus(editFieldPriority)
	_, cmd := ef.Update(tea.KeyPressMsg{Code: -2, Text: "enter"})
	if cmd == nil {
		t.Fatal("expected cmd from enter on last field")
	}
	msg := cmd()
	result, ok := msg.(EditFormResult)
	if !ok {
		t.Fatalf("expected EditFormResult, got %T", msg)
	}
	if result.Cancelled {
		t.Fatal("should not be cancelled")
	}
	if result.IssueID != "mg-1" {
		t.Fatalf("IssueID = %q, want mg-1", result.IssueID)
	}
}

func TestEditFormSubmitWithChanges(t *testing.T) {
	issue := data.Issue{ID: "mg-1", Title: "Old title", Priority: data.PriorityLow}
	ef := NewEditForm(80, 24, &issue)

	// Change title by setting value directly (simulating typing)
	ef.titleInput.SetValue("New title")

	// Move to priority, change it
	ef.focus(editFieldPriority)
	ef, _ = ef.Update(tea.KeyPressMsg{Code: 'k', Text: "k"}) // move priority up (P3→P2)

	// Submit
	ef, cmd := ef.Update(tea.KeyPressMsg{Code: -2, Text: "enter"})
	msg := cmd()
	result := msg.(EditFormResult)
	if result.Title != "New title" {
		t.Fatalf("Title = %q, want New title", result.Title)
	}
	if result.Priority != "2" {
		t.Fatalf("Priority = %q, want 2", result.Priority)
	}
	if strings.Join(result.Changed, ",") != "title,priority" {
		t.Fatalf("Changed = %v, want only title and priority", result.Changed)
	}
}

func TestEditFormPriorityBounds(t *testing.T) {
	issue := data.Issue{ID: "mg-1", Title: "Test", Priority: data.PriorityCritical}
	ef := NewEditForm(80, 24, &issue)

	// Move to priority field
	ef.focus(editFieldPriority)

	// Try to go above P0 (should stay at 0)
	ef, _ = ef.Update(tea.KeyPressMsg{Code: 'k', Text: "k"})
	if ef.prioIdx != 0 {
		t.Fatalf("prioIdx should stay at 0, got %d", ef.prioIdx)
	}
}

func TestEditFormViewContainsLabels(t *testing.T) {
	issue := data.Issue{ID: "mg-1", Title: "Test", Priority: data.PriorityMedium}
	ef := NewEditForm(80, 24, &issue)
	view := ef.View()
	if !strings.Contains(view, "Title") {
		t.Fatal("view should contain Title label")
	}
	if !strings.Contains(view, "Priority") {
		t.Fatal("view should contain Priority label")
	}
	if !strings.Contains(view, "mg-1") {
		t.Fatal("view should contain issue ID")
	}
}

func TestEditFormViewShowsEditHeader(t *testing.T) {
	issue := data.Issue{ID: "mg-42", Title: "Test", Priority: data.PriorityMedium}
	ef := NewEditForm(80, 24, &issue)
	view := ef.View()
	if !strings.Contains(view, "EDIT") {
		t.Fatal("view should contain EDIT in header")
	}
}

func TestEditFormEnterOnTitleSaves(t *testing.T) {
	// Enter on the title used to move to priority; the hint says it saves
	// (mg-vtc).
	issue := data.Issue{ID: "mg-1", Title: "Old title", Priority: data.PriorityMedium}
	ef := NewEditForm(80, 24, &issue)
	_, cmd := ef.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("enter on the title should save")
	}
	if res, ok := cmd().(EditFormResult); !ok || res.Title != "Old title" || res.IssueID != "mg-1" {
		t.Fatalf("result = %+v", res)
	}
}

func TestEditFormEnterEdgeCaseEmptyTitleSaysWhy(t *testing.T) {
	issue := data.Issue{ID: "mg-1", Title: "x", Priority: data.PriorityMedium}
	ef := NewEditForm(80, 24, &issue)
	ef, _ = ef.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	ef, cmd := ef.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil || !strings.Contains(ef.View(), "Title is required") {
		t.Fatal("an empty title should not save and should say why")
	}
}

func TestEditFormEditsTypeStatusDescription(t *testing.T) {
	// The edit form covered only title and priority; description, type and
	// status needed the bd CLI (mg-nd2).
	issue := data.Issue{ID: "mg-1", Title: "T", IssueType: data.TypeTask, Status: data.StatusOpen, Priority: data.PriorityMedium, Description: "old"}
	ef := NewEditForm(80, 24, &issue)
	ef.focus(editFieldType)
	ef, _ = ef.Update(tea.KeyPressMsg{Code: tea.KeyRight}) // Task → Bug
	ef.focus(editFieldStatus)
	ef, _ = ef.Update(tea.KeyPressMsg{Code: 'l', Text: "l"}) // Open → In progress
	ef.focus(editFieldDescription)
	ef.descInput.SetValue("line one")
	ef, _ = ef.Update(tea.KeyPressMsg{Code: tea.KeyEnter}) // a newline, not a save
	ef, _ = ef.Update(tea.KeyPressMsg{Code: 't', Text: "t"})
	_, cmd := ef.Update(tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl})
	if cmd == nil {
		t.Fatal("ctrl+s should save from the description")
	}
	res := cmd().(EditFormResult)
	if res.Type != "bug" || res.Status != "in_progress" || res.Description != "line one\nt" {
		t.Fatalf("result = %+v", res)
	}
	if strings.Join(res.Changed, ",") != "type,status,description" {
		t.Fatalf("Changed = %v", res.Changed)
	}
}

func TestEditFormEdgeCaseCustomTypeKept(t *testing.T) {
	issue := data.Issue{ID: "mg-1", Title: "T", IssueType: "decision", Priority: data.PriorityMedium}
	ef := NewEditForm(80, 24, &issue)
	if got := ef.values().Type; got != "decision" {
		t.Fatalf("a custom type should stay selected, got %q", got)
	}
}
