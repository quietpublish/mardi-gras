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

func TestClaimNextReadyArgsAndParse(t *testing.T) {
	calls, restore := mockRunCapture([]byte(`[{"id":"mg-77","title":"Make the parade roll faster"}]`), nil)
	defer restore()

	issue, err := ClaimNextReady()
	if err != nil {
		t.Fatalf("ClaimNextReady() error = %v", err)
	}
	if issue == nil || issue.ID != "mg-77" || issue.Title != "Make the parade roll faster" {
		t.Fatalf("issue = %+v, want mg-77 with title", issue)
	}

	args := (*calls)[0]
	if len(args) != 4 || args[0] != "bd" || args[1] != "ready" || args[2] != "--claim" || args[3] != "--json" {
		t.Errorf("args = %v", args)
	}
}

func TestClaimNextReadyEmpty(t *testing.T) {
	defer mockRun([]byte(`[]`), nil)()

	issue, err := ClaimNextReady()
	if err != nil {
		t.Fatalf("ClaimNextReady() error = %v", err)
	}
	if issue != nil {
		t.Fatalf("issue = %+v, want nil", issue)
	}
}

func TestClaimNextReadyExecError(t *testing.T) {
	defer mockRun(nil, errors.New("unknown flag: --claim"))()

	_, err := ClaimNextReady()
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

func TestFirstLine(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"whitespace only", "   \n\t  \n", ""},
		{"single line", "hello", "hello"},
		{"leading whitespace", "  hello  ", "hello"},
		{"trailing CR trimmed", "hello\r", "hello"},
		{"multiline", "first line\nsecond\nthird", "first line"},
		{"leading blank lines", "\n\nactual content\nmore\n", "actual content"},
		{"trailing newline", "summary line\n", "summary line"},
		{"trailing whitespace on first line", "summary  \nignored", "summary"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := firstLine(tc.in)
			if got != tc.want {
				t.Errorf("firstLine(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestPrunePreviewArgs(t *testing.T) {
	calls, restore := mockRunCapture([]byte("Would delete 5 closed issues.\n"), nil)
	defer restore()

	got, err := PrunePreview("14d")
	if err != nil {
		t.Fatalf("PrunePreview() error = %v", err)
	}
	if got != "Would delete 5 closed issues." {
		t.Errorf("PrunePreview() = %q, want first line", got)
	}

	args := (*calls)[0]
	if len(args) != 5 || args[0] != "bd" || args[1] != "prune" || args[2] != "--older-than" || args[3] != "14d" || args[4] != "--dry-run" {
		t.Errorf("args = %v", args)
	}
}

func TestPrunePreviewDefaultsTo30d(t *testing.T) {
	calls, restore := mockRunCapture([]byte("ok"), nil)
	defer restore()
	if _, err := PrunePreview(""); err != nil {
		t.Fatal(err)
	}
	args := (*calls)[0]
	if args[3] != "30d" {
		t.Errorf("default olderThan = %q, want 30d", args[3])
	}
}

func TestPruneClosedArgs(t *testing.T) {
	calls, restore := mockRunCapture([]byte("Deleted 3 issues.\n"), nil)
	defer restore()

	got, err := PruneClosed("30d")
	if err != nil {
		t.Fatalf("PruneClosed() error = %v", err)
	}
	if got != "Deleted 3 issues." {
		t.Errorf("PruneClosed() = %q, want first line", got)
	}

	args := (*calls)[0]
	if len(args) != 5 || args[0] != "bd" || args[1] != "prune" || args[2] != "--older-than" || args[3] != "30d" || args[4] != "--force" {
		t.Errorf("args = %v", args)
	}
}

func TestUpdateTitleArgs(t *testing.T) {
	calls, restore := mockExecCapture(nil)
	defer restore()

	if err := UpdateTitle("mg-42", "Switch parade to MQ4"); err != nil {
		t.Fatalf("UpdateTitle() error = %v", err)
	}

	args := (*calls)[0]
	if len(args) != 4 || args[0] != "bd" || args[1] != "update" || args[2] != "mg-42" || args[3] != "--title=Switch parade to MQ4" {
		t.Errorf("args = %v", args)
	}
}

func TestUpdateTitleRejectsBadID(t *testing.T) {
	_, restore := mockExecCapture(nil)
	defer restore()
	if err := UpdateTitle("", "title"); err == nil {
		t.Fatal("expected ValidateIssueID error, got nil")
	}
}

func TestCloseAndClaimNextEdgeCaseUnreadableOutput(t *testing.T) {
	// bd exited 0, so the issue is closed; only the claim is unknown. That
	// used to read as a failed close (mg-299).
	defer mockRun([]byte(``), nil)()
	_, err := CloseAndClaimNext("mg-42")
	if !errors.Is(err, ErrClaimUnreadable) {
		t.Fatalf("err = %v, want ErrClaimUnreadable", err)
	}
}
