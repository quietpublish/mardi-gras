package components

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/matt-wright86/mardi-gras/internal/data"
)

func TestParsePriority(t *testing.T) {
	tests := []struct {
		input  string
		expect data.Priority
	}{
		{"0", data.PriorityCritical},
		{"1", data.PriorityHigh},
		{"2", data.PriorityMedium},
		{"3", data.PriorityLow},
		{"4", data.PriorityBacklog},
		{"", data.PriorityMedium},
		{"999", data.PriorityMedium},
		{"abc", data.PriorityMedium},
	}

	for _, tc := range tests {
		t.Run("input_"+tc.input, func(t *testing.T) {
			got := ParsePriority(tc.input)
			if got != tc.expect {
				t.Fatalf("ParsePriority(%q) = %d, want %d", tc.input, got, tc.expect)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// CreateForm state machine tests
// ---------------------------------------------------------------------------

func newTestForm() CreateForm {
	return NewCreateForm(80, 24)
}

func TestCreateFormDefaults(t *testing.T) {
	cf := newTestForm()
	if cf.activeField != 0 {
		t.Fatalf("expected activeField 0, got %d", cf.activeField)
	}
	if cf.typeIdx != 0 {
		t.Fatalf("expected typeIdx 0 (task), got %d", cf.typeIdx)
	}
	if cf.prioIdx != 2 {
		t.Fatalf("expected prioIdx 2 (P2 Medium), got %d", cf.prioIdx)
	}
}

// ---------------------------------------------------------------------------
// Tab cycles fields forward: 0 → 1 → 2 → 0
// ---------------------------------------------------------------------------

func TestCreateFormTabCyclesForward(t *testing.T) {
	cf := newTestForm()

	cf, _ = cf.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if cf.activeField != 1 {
		t.Fatalf("expected activeField 1 after first tab, got %d", cf.activeField)
	}

	cf, _ = cf.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if cf.activeField != 2 {
		t.Fatalf("expected activeField 2 after second tab, got %d", cf.activeField)
	}

	cf, _ = cf.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if cf.activeField != 0 {
		t.Fatalf("expected activeField 0 after third tab (wrap), got %d", cf.activeField)
	}
}

// ---------------------------------------------------------------------------
// Shift+Tab cycles fields backward: 0 → 2 → 1 → 0
// ---------------------------------------------------------------------------

func TestCreateFormShiftTabCyclesBackward(t *testing.T) {
	cf := newTestForm()

	cf, _ = cf.Update(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
	if cf.activeField != 2 {
		t.Fatalf("expected activeField 2 after shift+tab from 0, got %d", cf.activeField)
	}

	cf, _ = cf.Update(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
	if cf.activeField != 1 {
		t.Fatalf("expected activeField 1 after shift+tab from 2, got %d", cf.activeField)
	}

	cf, _ = cf.Update(tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
	if cf.activeField != 0 {
		t.Fatalf("expected activeField 0 after shift+tab from 1, got %d", cf.activeField)
	}
}

// ---------------------------------------------------------------------------
// Enter on field 0 or 1 advances to next field
// ---------------------------------------------------------------------------

func TestCreateFormEnterAdvancesField(t *testing.T) {
	cf := newTestForm()
	// activeField == 0 (title)
	cf, _ = cf.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cf.activeField != 1 {
		t.Fatalf("expected activeField 1 after enter on title, got %d", cf.activeField)
	}

	// activeField == 1 (type)
	cf, _ = cf.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cf.activeField != 2 {
		t.Fatalf("expected activeField 2 after enter on type, got %d", cf.activeField)
	}
}

// ---------------------------------------------------------------------------
// Enter on field 2 submits when title is non-empty
// ---------------------------------------------------------------------------

func TestCreateFormEnterSubmits(t *testing.T) {
	cf := newTestForm()

	// Type a title
	for _, r := range "My issue" {
		cf, _ = cf.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}

	// Advance to priority field
	cf, _ = cf.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	cf, _ = cf.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if cf.activeField != 2 {
		t.Fatalf("expected activeField 2, got %d", cf.activeField)
	}

	// Enter should submit
	_, cmd := cf.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected non-nil cmd on submit")
	}
	msg := cmd()
	result, ok := msg.(CreateFormResult)
	if !ok {
		t.Fatalf("expected CreateFormResult, got %T", msg)
	}
	if result.Cancelled {
		t.Fatal("expected Cancelled to be false")
	}
	if result.Title != "My issue" {
		t.Fatalf("expected title 'My issue', got %q", result.Title)
	}
	if result.Type != "task" {
		t.Fatalf("expected type 'task', got %q", result.Type)
	}
	if result.Priority != "2" {
		t.Fatalf("expected priority '2', got %q", result.Priority)
	}
}

// ---------------------------------------------------------------------------
// Enter on field 2 with empty title does NOT submit
// ---------------------------------------------------------------------------

func TestCreateFormEnterEmptyTitleNoSubmit(t *testing.T) {
	cf := newTestForm()

	// Advance to priority field without typing a title
	cf, _ = cf.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	cf, _ = cf.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if cf.activeField != 2 {
		t.Fatalf("expected activeField 2, got %d", cf.activeField)
	}

	_, cmd := cf.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		t.Fatal("expected nil cmd when submitting with empty title")
	}
}

// ---------------------------------------------------------------------------
// Esc cancels the form
// ---------------------------------------------------------------------------

func TestCreateFormEscCancels(t *testing.T) {
	cf := newTestForm()

	_, cmd := cf.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("expected non-nil cmd from esc")
	}
	msg := cmd()
	result, ok := msg.(CreateFormResult)
	if !ok {
		t.Fatalf("expected CreateFormResult, got %T", msg)
	}
	if !result.Cancelled {
		t.Fatal("expected Cancelled to be true")
	}
}

// ---------------------------------------------------------------------------
// j/down on type field increments typeIdx with bounds
// ---------------------------------------------------------------------------

func TestCreateFormJDownOnTypeField(t *testing.T) {
	cf := newTestForm()
	// Move to type field
	cf, _ = cf.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if cf.activeField != 1 {
		t.Fatalf("expected activeField 1, got %d", cf.activeField)
	}

	// j increments typeIdx
	cf, _ = cf.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if cf.typeIdx != 1 {
		t.Fatalf("expected typeIdx 1 after j, got %d", cf.typeIdx)
	}

	// down arrow also works
	cf, _ = cf.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if cf.typeIdx != 2 {
		t.Fatalf("expected typeIdx 2 after down, got %d", cf.typeIdx)
	}

	// Navigate to last option
	for i := cf.typeIdx; i < len(typeOptions)-1; i++ {
		cf, _ = cf.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	}
	if cf.typeIdx != len(typeOptions)-1 {
		t.Fatalf("expected typeIdx at last option %d, got %d", len(typeOptions)-1, cf.typeIdx)
	}

	// j at last option stays clamped
	cf, _ = cf.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if cf.typeIdx != len(typeOptions)-1 {
		t.Fatalf("expected typeIdx clamped at %d, got %d", len(typeOptions)-1, cf.typeIdx)
	}
}

// ---------------------------------------------------------------------------
// k/up on type field decrements typeIdx with bounds
// ---------------------------------------------------------------------------

func TestCreateFormKUpOnTypeField(t *testing.T) {
	cf := newTestForm()
	// Move to type field and navigate down first
	cf, _ = cf.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	cf, _ = cf.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	cf, _ = cf.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if cf.typeIdx != 2 {
		t.Fatalf("expected typeIdx 2, got %d", cf.typeIdx)
	}

	// k decrements
	cf, _ = cf.Update(tea.KeyPressMsg{Code: 'k', Text: "k"})
	if cf.typeIdx != 1 {
		t.Fatalf("expected typeIdx 1 after k, got %d", cf.typeIdx)
	}

	// up arrow also works
	cf, _ = cf.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if cf.typeIdx != 0 {
		t.Fatalf("expected typeIdx 0 after up, got %d", cf.typeIdx)
	}

	// k at 0 stays clamped
	cf, _ = cf.Update(tea.KeyPressMsg{Code: 'k', Text: "k"})
	if cf.typeIdx != 0 {
		t.Fatalf("expected typeIdx clamped at 0, got %d", cf.typeIdx)
	}
}

// ---------------------------------------------------------------------------
// j/down on priority field increments prioIdx with bounds
// ---------------------------------------------------------------------------

func TestCreateFormJDownOnPriorityField(t *testing.T) {
	cf := newTestForm()
	// Move to priority field
	cf, _ = cf.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	cf, _ = cf.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	if cf.activeField != 2 {
		t.Fatalf("expected activeField 2, got %d", cf.activeField)
	}

	// Default prioIdx is 2
	if cf.prioIdx != 2 {
		t.Fatalf("expected prioIdx 2, got %d", cf.prioIdx)
	}

	cf, _ = cf.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if cf.prioIdx != 3 {
		t.Fatalf("expected prioIdx 3 after j, got %d", cf.prioIdx)
	}

	cf, _ = cf.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if cf.prioIdx != 4 {
		t.Fatalf("expected prioIdx 4 after down, got %d", cf.prioIdx)
	}

	// Clamped at last
	cf, _ = cf.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if cf.prioIdx != len(priorityOptions)-1 {
		t.Fatalf("expected prioIdx clamped at %d, got %d", len(priorityOptions)-1, cf.prioIdx)
	}
}

// ---------------------------------------------------------------------------
// k/up on priority field decrements prioIdx with bounds
// ---------------------------------------------------------------------------

func TestCreateFormKUpOnPriorityField(t *testing.T) {
	cf := newTestForm()
	// Move to priority field
	cf, _ = cf.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	cf, _ = cf.Update(tea.KeyPressMsg{Code: tea.KeyTab})

	// prioIdx starts at 2, move up
	cf, _ = cf.Update(tea.KeyPressMsg{Code: 'k', Text: "k"})
	if cf.prioIdx != 1 {
		t.Fatalf("expected prioIdx 1 after k, got %d", cf.prioIdx)
	}

	cf, _ = cf.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if cf.prioIdx != 0 {
		t.Fatalf("expected prioIdx 0 after up, got %d", cf.prioIdx)
	}

	// Clamped at 0
	cf, _ = cf.Update(tea.KeyPressMsg{Code: 'k', Text: "k"})
	if cf.prioIdx != 0 {
		t.Fatalf("expected prioIdx clamped at 0, got %d", cf.prioIdx)
	}
}

// ---------------------------------------------------------------------------
// View contains expected labels
// ---------------------------------------------------------------------------

func TestCreateFormViewContainsLabels(t *testing.T) {
	cf := newTestForm()
	view := cf.View()

	for _, label := range []string{"Title", "Type", "Priority"} {
		if !strings.Contains(view, label) {
			t.Fatalf("expected view to contain %q", label)
		}
	}
}

// ---------------------------------------------------------------------------
// Submit reflects changed type and priority selections
// ---------------------------------------------------------------------------

func TestCreateFormSubmitReflectsSelections(t *testing.T) {
	cf := newTestForm()

	// Type title
	for _, r := range "Bug report" {
		cf, _ = cf.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}

	// Move to type field and select "bug" (index 1)
	cf, _ = cf.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	cf, _ = cf.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if cf.typeIdx != 1 {
		t.Fatalf("expected typeIdx 1, got %d", cf.typeIdx)
	}

	// Move to priority field and select P0 Critical (move up from default 2 to 0)
	cf, _ = cf.Update(tea.KeyPressMsg{Code: tea.KeyTab})
	cf, _ = cf.Update(tea.KeyPressMsg{Code: 'k', Text: "k"})
	cf, _ = cf.Update(tea.KeyPressMsg{Code: 'k', Text: "k"})
	if cf.prioIdx != 0 {
		t.Fatalf("expected prioIdx 0, got %d", cf.prioIdx)
	}

	// Submit
	_, cmd := cf.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected non-nil cmd on submit")
	}
	msg := cmd()
	result, ok := msg.(CreateFormResult)
	if !ok {
		t.Fatalf("expected CreateFormResult, got %T", msg)
	}
	if result.Title != "Bug report" {
		t.Fatalf("expected title 'Bug report', got %q", result.Title)
	}
	if result.Type != "bug" {
		t.Fatalf("expected type 'bug', got %q", result.Type)
	}
	if result.Priority != "0" {
		t.Fatalf("expected priority '0' (Critical), got %q", result.Priority)
	}
}

// ---------------------------------------------------------------------------
// Crew member field appears when gtAvailable is true
// ---------------------------------------------------------------------------

func TestCreateFormWithCrewField(t *testing.T) {
	cf := NewCreateFormWithGT(80, 24)
	// Should have 4 fields: title(0), type(1), priority(2), crew(3)
	// Tab through all fields
	cf, _ = cf.Update(tea.KeyPressMsg{Code: tea.KeyTab}) // -> type
	if cf.activeField != 1 {
		t.Fatalf("expected activeField 1, got %d", cf.activeField)
	}
	cf, _ = cf.Update(tea.KeyPressMsg{Code: tea.KeyTab}) // -> priority
	if cf.activeField != 2 {
		t.Fatalf("expected activeField 2, got %d", cf.activeField)
	}
	cf, _ = cf.Update(tea.KeyPressMsg{Code: tea.KeyTab}) // -> crew
	if cf.activeField != 3 {
		t.Fatalf("expected activeField 3, got %d", cf.activeField)
	}
	cf, _ = cf.Update(tea.KeyPressMsg{Code: tea.KeyTab}) // -> wrap to title
	if cf.activeField != 0 {
		t.Fatalf("expected activeField 0 after wrap, got %d", cf.activeField)
	}
}

func TestCreateFormCrewFieldSubmit(t *testing.T) {
	cf := NewCreateFormWithGT(80, 24)

	// Type a title
	for _, r := range "Fix auth" {
		cf, _ = cf.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}

	// Tab to crew field (field 3)
	cf, _ = cf.Update(tea.KeyPressMsg{Code: tea.KeyTab}) // type
	cf, _ = cf.Update(tea.KeyPressMsg{Code: tea.KeyTab}) // priority
	cf, _ = cf.Update(tea.KeyPressMsg{Code: tea.KeyTab}) // crew
	if cf.activeField != 3 {
		t.Fatalf("expected activeField 3, got %d", cf.activeField)
	}

	// Type crew member name
	for _, r := range "monet" {
		cf, _ = cf.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}

	// Enter on last field should submit
	_, cmd := cf.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected non-nil cmd on submit")
	}
	msg := cmd()
	result, ok := msg.(CreateFormResult)
	if !ok {
		t.Fatalf("expected CreateFormResult, got %T", msg)
	}
	if result.Title != "Fix auth" {
		t.Fatalf("expected title 'Fix auth', got %q", result.Title)
	}
	if result.CrewMember != "monet" {
		t.Fatalf("expected crew 'monet', got %q", result.CrewMember)
	}
}

func TestCreateFormCrewFieldOptional(t *testing.T) {
	cf := NewCreateFormWithGT(80, 24)

	// Type a title
	for _, r := range "Quick task" {
		cf, _ = cf.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	}

	// Tab to crew field, leave empty, submit
	cf, _ = cf.Update(tea.KeyPressMsg{Code: tea.KeyTab}) // type
	cf, _ = cf.Update(tea.KeyPressMsg{Code: tea.KeyTab}) // priority
	cf, _ = cf.Update(tea.KeyPressMsg{Code: tea.KeyTab}) // crew
	_, cmd := cf.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected non-nil cmd on submit with empty crew")
	}
	msg := cmd()
	result := msg.(CreateFormResult)
	if result.CrewMember != "" {
		t.Fatalf("expected empty crew, got %q", result.CrewMember)
	}
	if result.Title != "Quick task" {
		t.Fatalf("expected title 'Quick task', got %q", result.Title)
	}
}

func TestCreateFormWithoutGTHasNoCrewField(t *testing.T) {
	cf := NewCreateForm(80, 24) // no GT
	// Should have 3 fields: title(0), type(1), priority(2)
	cf, _ = cf.Update(tea.KeyPressMsg{Code: tea.KeyTab}) // -> type
	cf, _ = cf.Update(tea.KeyPressMsg{Code: tea.KeyTab}) // -> priority
	cf, _ = cf.Update(tea.KeyPressMsg{Code: tea.KeyTab}) // -> wrap to title
	if cf.activeField != 0 {
		t.Fatalf("expected activeField 0 after wrap (3 fields), got %d", cf.activeField)
	}
}

func TestCreateFormViewContainsCrewLabel(t *testing.T) {
	cf := NewCreateFormWithGT(80, 24)
	view := cf.View()
	if !strings.Contains(view, "Crew") {
		t.Fatal("expected view to contain 'Crew' label when GT available")
	}
}

// ---------------------------------------------------------------------------
// j/k on title field (activeField 0) do not change type/priority indices
// ---------------------------------------------------------------------------

func TestCreateFormJKOnTitleFieldNoEffect(t *testing.T) {
	cf := newTestForm()
	// On title field, j/k should go to text input, not change indices
	origType := cf.typeIdx
	origPrio := cf.prioIdx

	cf, _ = cf.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	cf, _ = cf.Update(tea.KeyPressMsg{Code: 'k', Text: "k"})

	if cf.typeIdx != origType {
		t.Fatalf("expected typeIdx unchanged at %d, got %d", origType, cf.typeIdx)
	}
	if cf.prioIdx != origPrio {
		t.Fatalf("expected prioIdx unchanged at %d, got %d", origPrio, cf.prioIdx)
	}
}
