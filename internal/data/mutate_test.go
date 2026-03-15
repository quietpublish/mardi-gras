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
