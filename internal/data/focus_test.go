package data

import (
	"os"
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

	mine := focusTestIssue("mine-1", StatusInProgress, PriorityHigh)
	mine.Assignee = "alice"

	other := focusTestIssue("other-1", StatusOpen, PriorityMedium)

	result := FocusFilter([]Issue{other, mine}, DefaultBlockingTypes, "alice")

	if len(result) == 0 {
		t.Fatal("expected at least one result")
	}
	if result[0].ID != "mine-1" {
		t.Errorf("expected first result to be mine-1, got %s", result[0].ID)
	}
}

func TestFocusFilterNoUser(t *testing.T) {
	ip1 := focusTestIssue("ip-1", StatusInProgress, PriorityHigh)
	ip2 := focusTestIssue("ip-2", StatusInProgress, PriorityMedium)

	result := FocusFilter([]Issue{ip1, ip2}, DefaultBlockingTypes, "")

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

	low := focusTestIssue("low", StatusOpen, PriorityLow)
	crit := focusTestIssue("crit", StatusOpen, PriorityCritical)
	med := focusTestIssue("med", StatusOpen, PriorityMedium)
	high := focusTestIssue("high", StatusOpen, PriorityHigh)

	result := FocusFilter([]Issue{low, crit, med, high}, DefaultBlockingTypes, "testuser")

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

	var issues []Issue
	for i := 0; i < 8; i++ {
		issues = append(issues, focusTestIssue(
			"ready-"+string(rune('a'+i)),
			StatusOpen,
			PriorityMedium,
		))
	}

	result := FocusFilter(issues, DefaultBlockingTypes, "testuser")

	// No myWork, no blocked — result should be exactly the 5 ready cap.
	if len(result) != 5 {
		t.Errorf("expected 5 ready issues, got %d", len(result))
	}
}

func TestFocusFilterBlockedCappedAt3(t *testing.T) {

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

	result := FocusFilter(issues, DefaultBlockingTypes, "testuser")

	// All issues are blocked; cap is 3.
	if len(result) != 3 {
		t.Errorf("expected 3 blocked issues, got %d", len(result))
	}
}

func TestFocusFilterExcludesClosed(t *testing.T) {

	closed := focusTestIssue("closed-1", StatusClosed, PriorityCritical)
	closed.Assignee = "alice"

	open := focusTestIssue("open-1", StatusOpen, PriorityMedium)

	result := FocusFilter([]Issue{closed, open}, DefaultBlockingTypes, "alice")

	for _, iss := range result {
		if iss.ID == "closed-1" {
			t.Error("closed issue should not appear in focus filter results")
		}
	}
}

func TestFocusFilterOrdering(t *testing.T) {

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

	result := FocusFilter([]Issue{blocked, ready, myWork}, DefaultBlockingTypes, "alice")

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

func TestFocusFilterUnassignedInProgress(t *testing.T) {
	// An agent marking an issue in progress without claiming it leaves no
	// assignee; that work must not vanish from focus mode.
	unclaimed := focusTestIssue("unclaimed", StatusInProgress, PriorityHigh)
	theirs := focusTestIssue("theirs", StatusInProgress, PriorityHigh)
	theirs.Assignee = "bob"

	result := FocusFilter([]Issue{theirs, unclaimed}, DefaultBlockingTypes, "alice")
	if len(result) != 1 || result[0].ID != "unclaimed" {
		t.Fatalf("got %v, want only the unclaimed in-progress issue", ids(result))
	}
}

func TestFocusFilterMatchesActorLikeBd(t *testing.T) {
	for _, assignee := range []string{"matt-wright86", "matt_wright86", "Matt-Wright86", "matt.wright86"} {
		mine := focusTestIssue("mine", StatusInProgress, PriorityHigh)
		mine.Assignee = assignee
		if result := FocusFilter([]Issue{mine}, DefaultBlockingTypes, "matt-wright86"); len(result) != 1 {
			t.Errorf("assignee %q: not recognised as matt-wright86", assignee)
		}
	}
}

func TestSameActor(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"alice", "alice", true},
		{"matt-wright86", "matt_wright86", true},
		{"a.b-c", "a_b_c", true},
		{"gastown--mayor", "gastown/mayor", true},
		{"gastown--mayor", "gastown-mayor", false}, // "--" is '/', not a separator
		{"Alice", "alice", true},
		{"alice", "", false},
		{"", "", false},
		{"alice", "alice/claude", false},
	}
	for _, tt := range tests {
		if got := SameActor(tt.a, tt.b); got != tt.want {
			t.Errorf("SameActor(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestCurrentActor(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".beads"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".beads", "config.yaml"), []byte("actor: \"from-config\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// git reads user.name from these, over every config file.
	t.Setenv("GIT_CONFIG_COUNT", "1")
	t.Setenv("GIT_CONFIG_KEY_0", "user.name")
	t.Setenv("GIT_CONFIG_VALUE_0", "From Git")
	t.Setenv("USER", "from-user")
	t.Setenv("BEADS_ACTOR", "")
	t.Setenv("BD_ACTOR", "")

	steps := []struct {
		name string
		set  func()
		want string
	}{
		{"BEADS_ACTOR wins", func() { t.Setenv("BEADS_ACTOR", "from-beads-actor"); t.Setenv("BD_ACTOR", "from-bd-actor") }, "from-beads-actor"},
		{"then BD_ACTOR", func() { t.Setenv("BEADS_ACTOR", "") }, "from-bd-actor"},
		{"then the workspace config", func() { t.Setenv("BD_ACTOR", "") }, "from-config"},
		{"then git user.name", func() { dir = t.TempDir() }, "From Git"},
		{"then $USER", func() { t.Setenv("GIT_CONFIG_VALUE_0", "") }, "from-user"},
	}
	for _, step := range steps {
		step.set()
		if got := CurrentActor(dir); got != step.want {
			t.Fatalf("%s: CurrentActor() = %q, want %q", step.name, got, step.want)
		}
	}
}

func ids(issues []Issue) []string {
	out := make([]string, len(issues))
	for i, iss := range issues {
		out[i] = iss.ID
	}
	return out
}
