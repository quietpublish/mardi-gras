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

	// Start at field 0 (title)
	ef, _ = ef.Update(tea.KeyPressMsg{Code: -2, Text: "tab"})
	if ef.activeField != 1 {
		t.Fatalf("after tab, activeField = %d, want 1", ef.activeField)
	}
	ef, _ = ef.Update(tea.KeyPressMsg{Code: -2, Text: "tab"})
	if ef.activeField != 0 {
		t.Fatalf("after tab tab, activeField = %d, want 0 (wrap)", ef.activeField)
	}
}

func TestEditFormShiftTabCycles(t *testing.T) {
	issue := data.Issue{ID: "mg-1", Title: "Test", Priority: data.PriorityMedium}
	ef := NewEditForm(80, 24, &issue)

	ef, _ = ef.Update(tea.KeyPressMsg{Code: -2, Text: "shift+tab"})
	if ef.activeField != 1 {
		t.Fatalf("after shift+tab, activeField = %d, want 1", ef.activeField)
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

	// Move to priority field (field 1), then submit
	ef, _ = ef.Update(tea.KeyPressMsg{Code: -2, Text: "tab"})
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
	ef, _ = ef.Update(tea.KeyPressMsg{Code: -2, Text: "tab"})
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
}

func TestEditFormPriorityBounds(t *testing.T) {
	issue := data.Issue{ID: "mg-1", Title: "Test", Priority: data.PriorityCritical}
	ef := NewEditForm(80, 24, &issue)

	// Move to priority field
	ef, _ = ef.Update(tea.KeyPressMsg{Code: -2, Text: "tab"})

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
