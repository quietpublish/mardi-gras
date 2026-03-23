package data

import (
	"errors"
	"testing"
)

func TestBranchName(t *testing.T) {
	tests := []struct {
		name     string
		issue    Issue
		expected string
	}{
		{
			name:     "Bug issue",
			issue:    Issue{ID: "bd-a1b2", Title: "Fix login token expiry", IssueType: TypeBug},
			expected: "fix/bd-a1b2-fix-login-token-expiry",
		},
		{
			name:     "Feature issue",
			issue:    Issue{ID: "bd-c3d4", Title: "Add search feature", IssueType: TypeFeature},
			expected: "feat/bd-c3d4-add-search-feature",
		},
		{
			name:     "Task issue",
			issue:    Issue{ID: "bd-e5f6", Title: "Update documentation", IssueType: TypeTask},
			expected: "task/bd-e5f6-update-documentation",
		},
		{
			name:     "Chore issue",
			issue:    Issue{ID: "bd-g7h8", Title: "Clean up CI config", IssueType: TypeChore},
			expected: "chore/bd-g7h8-clean-up-ci-config",
		},
		{
			name:     "Special characters stripped",
			issue:    Issue{ID: "bd-i9j0", Title: "Handle @mentions & #tags (v2)", IssueType: TypeFeature},
			expected: "feat/bd-i9j0-handle-mentions-tags-v2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BranchName(tt.issue)
			if got != tt.expected {
				t.Errorf("BranchName() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestSlugify(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Hello World", "hello-world"},
		{"Fix login/auth bug", "fix-login-auth-bug"},
		{"UPPER CASE", "upper-case"},
		{"   spaces   ", "spaces"},
		{"no-change", "no-change"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := slugify(tt.input)
			if got != tt.expected {
				t.Errorf("slugify(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestSetStatusArgs(t *testing.T) {
	calls, restore := mockExecCapture(nil)
	defer restore()
	err := SetStatus("mg-42", StatusInProgress)
	if err != nil {
		t.Fatalf("SetStatus() error = %v", err)
	}
	args := (*calls)[0]
	// Should be: bd update mg-42 --status=in_progress
	if len(args) != 4 || args[0] != "bd" || args[1] != "update" || args[2] != "mg-42" || args[3] != "--status=in_progress" {
		t.Errorf("args = %v", args)
	}
}

func TestClaimIssueArgs(t *testing.T) {
	calls, restore := mockExecCapture(nil)
	defer restore()
	err := ClaimIssue("mg-42")
	if err != nil {
		t.Fatalf("ClaimIssue() error = %v", err)
	}
	args := (*calls)[0]
	// Should be: bd update mg-42 --claim
	if len(args) != 4 || args[1] != "update" || args[2] != "mg-42" || args[3] != "--claim" {
		t.Errorf("args = %v", args)
	}
}

func TestCloseIssueArgs(t *testing.T) {
	calls, restore := mockExecCapture(nil)
	defer restore()
	err := CloseIssue("mg-42")
	if err != nil {
		t.Fatalf("CloseIssue() error = %v", err)
	}
	args := (*calls)[0]
	// Should be: bd close mg-42
	if len(args) != 3 || args[0] != "bd" || args[1] != "close" || args[2] != "mg-42" {
		t.Errorf("args = %v", args)
	}
}

func TestCloseAndClaimNextArgsAndParse(t *testing.T) {
	calls, restore := mockRunCapture([]byte(`{"claimed":{"id":"mg-99"},"closed":[{"id":"mg-42"}]}`), nil)
	defer restore()

	claimedID, err := CloseAndClaimNext("mg-42")
	if err != nil {
		t.Fatalf("CloseAndClaimNext() error = %v", err)
	}
	if claimedID != "mg-99" {
		t.Fatalf("claimedID = %q, want %q", claimedID, "mg-99")
	}

	args := (*calls)[0]
	if len(args) != 5 || args[0] != "bd" || args[1] != "close" || args[2] != "--claim-next" || args[3] != "--json" || args[4] != "mg-42" {
		t.Errorf("args = %v", args)
	}
}

func TestCloseAndClaimNextNoClaimedIssue(t *testing.T) {
	defer mockRun([]byte(`{"claimed":null,"closed":[{"id":"mg-42"}]}`), nil)()

	claimedID, err := CloseAndClaimNext("mg-42")
	if err != nil {
		t.Fatalf("CloseAndClaimNext() error = %v", err)
	}
	if claimedID != "" {
		t.Fatalf("claimedID = %q, want empty string", claimedID)
	}
}

func TestCloseAndClaimNextParseError(t *testing.T) {
	defer mockRun([]byte(`{bad json`), nil)()

	_, err := CloseAndClaimNext("mg-42")
	if err == nil {
		t.Fatal("expected parse error, got nil")
	}
}

func TestCloseAndClaimNextExecError(t *testing.T) {
	defer mockRun(nil, errors.New("bd not found"))()

	_, err := CloseAndClaimNext("mg-42")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestSetPriorityArgs(t *testing.T) {
	calls, restore := mockExecCapture(nil)
	defer restore()
	err := SetPriority("mg-42", PriorityHigh)
	if err != nil {
		t.Fatalf("SetPriority() error = %v", err)
	}
	args := (*calls)[0]
	// Should be: bd update mg-42 --priority=1
	if len(args) != 4 || args[3] != "--priority=1" {
		t.Errorf("args = %v", args)
	}
}

func TestCreateIssueHappy(t *testing.T) {
	defer mockRun([]byte("mg-99\n"), nil)()
	id, err := CreateIssue("New feature", TypeFeature, PriorityMedium)
	if err != nil {
		t.Fatalf("CreateIssue() error = %v", err)
	}
	if id != "mg-99" {
		t.Errorf("ID = %q, want mg-99", id)
	}
}

func TestCreateIssueExecError(t *testing.T) {
	defer mockRun(nil, errors.New("database locked"))()
	_, err := CreateIssue("New feature", TypeFeature, PriorityMedium)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- AddComment tests ---

func TestAddCommentArgs(t *testing.T) {
	calls, restore := mockRunCapture([]byte("ok\n"), nil)
	defer restore()
	err := AddComment("mg-42", "Fixed the auth bug")
	if err != nil {
		t.Fatalf("AddComment() error = %v", err)
	}
	args := (*calls)[0]
	// Should be: bd comments add mg-42 -- "Fixed the auth bug"
	if len(args) != 6 || args[0] != "bd" || args[1] != "comments" || args[2] != "add" || args[3] != "mg-42" || args[4] != "--" || args[5] != "Fixed the auth bug" {
		t.Errorf("args = %v", args)
	}
}

func TestAddCommentExecError(t *testing.T) {
	defer mockRun(nil, errors.New("bd not found"))()
	err := AddComment("mg-42", "comment")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestAddNoteArgs(t *testing.T) {
	calls, restore := mockRunCapture([]byte("ok\n"), nil)
	defer restore()

	err := AddNote("mg-42", "Remember to backfill fixtures")
	if err != nil {
		t.Fatalf("AddNote() error = %v", err)
	}

	args := (*calls)[0]
	if len(args) != 5 || args[0] != "bd" || args[1] != "note" || args[2] != "mg-42" || args[3] != "--" || args[4] != "Remember to backfill fixtures" {
		t.Errorf("args = %v", args)
	}
}

func TestAddNoteExecError(t *testing.T) {
	defer mockRun(nil, errors.New("bd not found"))()

	err := AddNote("mg-42", "note")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- SetAssignee tests ---

func TestSetAssigneeArgs(t *testing.T) {
	calls, restore := mockExecCapture(nil)
	defer restore()
	err := SetAssignee("mg-42", "alice")
	if err != nil {
		t.Fatalf("SetAssignee() error = %v", err)
	}
	args := (*calls)[0]
	// Should be: bd update mg-42 --assignee=alice
	if len(args) != 4 || args[0] != "bd" || args[1] != "update" || args[2] != "mg-42" || args[3] != "--assignee=alice" {
		t.Errorf("args = %v", args)
	}
}

func TestSetAssigneeError(t *testing.T) {
	_, restore := mockExecCapture(errors.New("not found"))
	defer restore()
	err := SetAssignee("mg-42", "alice")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- AddLabel tests ---

func TestAddLabelArgs(t *testing.T) {
	calls, restore := mockExecCapture(nil)
	defer restore()
	err := AddLabel("mg-42", "backend")
	if err != nil {
		t.Fatalf("AddLabel() error = %v", err)
	}
	args := (*calls)[0]
	// Should be: bd label add mg-42 -- backend
	if len(args) != 6 || args[0] != "bd" || args[1] != "label" || args[2] != "add" || args[3] != "mg-42" || args[4] != "--" || args[5] != "backend" {
		t.Errorf("args = %v", args)
	}
}

func TestAddLabelError(t *testing.T) {
	_, restore := mockExecCapture(errors.New("not found"))
	defer restore()
	err := AddLabel("mg-42", "backend")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// --- AddDependency tests ---

func TestAddDependencyArgs(t *testing.T) {
	calls, restore := mockExecCapture(nil)
	defer restore()
	err := AddDependency("mg-42", "mg-10")
	if err != nil {
		t.Fatalf("AddDependency() error = %v", err)
	}
	args := (*calls)[0]
	// Should be: bd dep add mg-42 -- mg-10
	if len(args) != 6 || args[0] != "bd" || args[1] != "dep" || args[2] != "add" || args[3] != "mg-42" || args[4] != "--" || args[5] != "mg-10" {
		t.Errorf("args = %v", args)
	}
}

func TestAddDependencyError(t *testing.T) {
	_, restore := mockExecCapture(errors.New("not found"))
	defer restore()
	err := AddDependency("mg-42", "mg-10")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
