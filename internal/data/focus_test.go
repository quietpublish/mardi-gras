package data

import (
	"path/filepath"
	"testing"
	"time"
)

func focusTestIssue(id string, status Status, priority Priority) Issue {
	now := time.Now()
	return Issue{
		ID:        id,
		Title:     id,
		Status:    status,
		Priority:  priority,
		IssueType: TypeTask,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func TestFocusFilterMyWork(t *testing.T) {
	t.Setenv("USER", "alice")

	mine := focusTestIssue("mine-1", StatusInProgress, PriorityHigh)
	mine.Assignee = "alice"

	other := focusTestIssue("other-1", StatusOpen, PriorityMedium)

	result := FocusFilter([]Issue{other, mine}, DefaultBlockingTypes)

	if len(result) == 0 {
		t.Fatal("expected at least one result")
	}
	if result[0].ID != "mine-1" {
		t.Errorf("expected first result to be mine-1, got %s", result[0].ID)
	}
}

func TestFocusFilterNoUser(t *testing.T) {
	// Set USER to empty so os.Getenv("USER") returns "".
	// Suppress all git config sources so `git config user.name` fails fast.
	t.Setenv("USER", "")
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("HOME", filepath.Join(t.TempDir(), "nohome"))
	t.Setenv("GIT_DIR", filepath.Join(t.TempDir(), "nogit"))

	ip1 := focusTestIssue("ip-1", StatusInProgress, PriorityHigh)
	ip2 := focusTestIssue("ip-2", StatusInProgress, PriorityMedium)

	result := FocusFilter([]Issue{ip1, ip2}, DefaultBlockingTypes)

	// With user="" all in_progress issues should be included in myWork bucket.
	var found int
	for _, iss := range result {
		if iss.ID == "ip-1" || iss.ID == "ip-2" {
			found++
		}
	}
	if found != 2 {
		t.Errorf("expected both in_progress issues in result, found %d", found)
	}
}

func TestFocusFilterReadySortedByPriority(t *testing.T) {
	t.Setenv("USER", "testuser")

	low := focusTestIssue("low", StatusOpen, PriorityLow)
	crit := focusTestIssue("crit", StatusOpen, PriorityCritical)
	med := focusTestIssue("med", StatusOpen, PriorityMedium)
	high := focusTestIssue("high", StatusOpen, PriorityHigh)

	result := FocusFilter([]Issue{low, crit, med, high}, DefaultBlockingTypes)

	expected := []string{"crit", "high", "med", "low"}
	if len(result) < len(expected) {
		t.Fatalf("expected at least %d results, got %d", len(expected), len(result))
	}
	for i, want := range expected {
		if result[i].ID != want {
			t.Errorf("result[%d] = %s, want %s", i, result[i].ID, want)
		}
	}
}

func TestFocusFilterReadyCappedAt5(t *testing.T) {
	t.Setenv("USER", "testuser")

	var issues []Issue
	for i := 0; i < 8; i++ {
		issues = append(issues, focusTestIssue(
			"ready-"+string(rune('a'+i)),
			StatusOpen,
			PriorityMedium,
		))
	}

	result := FocusFilter(issues, DefaultBlockingTypes)

	// No myWork, no blocked — result should be exactly the 5 ready cap.
	if len(result) != 5 {
		t.Errorf("expected 5 ready issues, got %d", len(result))
	}
}

func TestFocusFilterBlockedCappedAt3(t *testing.T) {
	t.Setenv("USER", "testuser")

	var issues []Issue
	for i := 0; i < 5; i++ {
		iss := focusTestIssue(
			"blocked-"+string(rune('a'+i)),
			StatusOpen,
			PriorityMedium,
		)
		iss.Dependencies = []Dependency{{
			Type:        "blocks",
			DependsOnID: "nonexistent",
			IssueID:     iss.ID,
		}}
		issues = append(issues, iss)
	}

	result := FocusFilter(issues, DefaultBlockingTypes)

	// All issues are blocked; cap is 3.
	if len(result) != 3 {
		t.Errorf("expected 3 blocked issues, got %d", len(result))
	}
}

func TestFocusFilterExcludesClosed(t *testing.T) {
	t.Setenv("USER", "alice")

	closed := focusTestIssue("closed-1", StatusClosed, PriorityCritical)
	closed.Assignee = "alice"

	open := focusTestIssue("open-1", StatusOpen, PriorityMedium)

	result := FocusFilter([]Issue{closed, open}, DefaultBlockingTypes)

	for _, iss := range result {
		if iss.ID == "closed-1" {
			t.Error("closed issue should not appear in focus filter results")
		}
	}
}

func TestFocusFilterOrdering(t *testing.T) {
	t.Setenv("USER", "alice")

	// myWork: in_progress assigned to alice
	myWork := focusTestIssue("my-1", StatusInProgress, PriorityHigh)
	myWork.Assignee = "alice"

	// ready: open, not blocked
	ready := focusTestIssue("ready-1", StatusOpen, PriorityMedium)

	// blocked: open with unresolvable dependency
	blocked := focusTestIssue("blocked-1", StatusOpen, PriorityLow)
	blocked.Dependencies = []Dependency{{
		Type:        "blocks",
		DependsOnID: "nonexistent",
		IssueID:     "blocked-1",
	}}

	result := FocusFilter([]Issue{blocked, ready, myWork}, DefaultBlockingTypes)

	if len(result) != 3 {
		t.Fatalf("expected 3 results, got %d", len(result))
	}
	if result[0].ID != "my-1" {
		t.Errorf("result[0] = %s, want my-1 (myWork)", result[0].ID)
	}
	if result[1].ID != "ready-1" {
		t.Errorf("result[1] = %s, want ready-1 (ready)", result[1].ID)
	}
	if result[2].ID != "blocked-1" {
		t.Errorf("result[2] = %s, want blocked-1 (blocked)", result[2].ID)
	}
}
