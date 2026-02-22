package data

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSampleIssues(t *testing.T) {
	// Find sample data relative to project root
	path := filepath.Join("..", "..", "testdata", "sample.jsonl")
	issues, err := LoadIssues(path)
	if err != nil {
		t.Fatalf("LoadIssues: %v", err)
	}

	if len(issues) != 14 {
		t.Fatalf("expected 14 issues, got %d", len(issues))
	}

	// Verify sorting: active issues come first
	for i, issue := range issues {
		if issue.Status == StatusClosed {
			// All remaining should be closed
			for j := i; j < len(issues); j++ {
				if issues[j].Status != StatusClosed {
					t.Errorf("issue %d (%s) is active but comes after closed issue %d", j, issues[j].ID, i)
				}
			}
			break
		}
	}
}

func TestGroupByParade(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "sample.jsonl")
	issues, err := LoadIssues(path)
	if err != nil {
		t.Fatalf("LoadIssues: %v", err)
	}

	groups := GroupByParade(issues, DefaultBlockingTypes)

	rolling := groups[ParadeRolling]
	linedUp := groups[ParadeLinedUp]
	stalled := groups[ParadeStalled]
	passed := groups[ParadePastTheStand]

	if len(rolling) != 2 {
		t.Errorf("expected 2 rolling, got %d: %v", len(rolling), issueIDs(rolling))
	}
	// mg-006 (open, blocked by mg-001), mg-011 (dangling dep), mg-012 (in_progress, blocked)
	if len(stalled) != 3 {
		t.Errorf("expected 3 stalled, got %d: %v", len(stalled), issueIDs(stalled))
	}
	// mg-013 (resolved dep) + mg-014 (non-blocking dep) join original 4
	if len(linedUp) != 6 {
		t.Errorf("expected 6 lined up, got %d: %v", len(linedUp), issueIDs(linedUp))
	}
	if len(passed) != 3 {
		t.Errorf("expected 3 past the stand, got %d: %v", len(passed), issueIDs(passed))
	}
}

func TestIsBlocked(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "sample.jsonl")
	issues, err := LoadIssues(path)
	if err != nil {
		t.Fatalf("LoadIssues: %v", err)
	}

	issueMap := BuildIssueMap(issues)

	// mg-006 is blocked by mg-001 (in_progress)
	mg006 := issueMap["mg-006"]
	if mg006 == nil {
		t.Fatal("mg-006 not found")
	}
	if !mg006.IsBlocked(issueMap) {
		t.Error("mg-006 should be blocked")
	}

	blockers := mg006.BlockedByIDs(issueMap)
	if len(blockers) != 1 || blockers[0] != "mg-001" {
		t.Errorf("expected blockers [mg-001], got %v", blockers)
	}

	// mg-001 blocks mg-006 and mg-012
	mg001 := issueMap["mg-001"]
	blocks := mg001.BlocksIDs(issues, DefaultBlockingTypes)
	if len(blocks) != 2 {
		t.Errorf("expected mg-001 blocks 2 issues, got %v", blocks)
	}
}

func issueIDs(issues []Issue) []string {
	ids := make([]string, len(issues))
	for i, iss := range issues {
		ids[i] = iss.ID
	}
	return ids
}

func TestEvaluateDependencies(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "sample.jsonl")
	issues, err := LoadIssues(path)
	if err != nil {
		t.Fatalf("LoadIssues: %v", err)
	}
	issueMap := BuildIssueMap(issues)

	t.Run("blocking", func(t *testing.T) {
		// mg-006 depends on mg-001 (in_progress) with type "blocks"
		eval := issueMap["mg-006"].EvaluateDependencies(issueMap, DefaultBlockingTypes)
		if !eval.IsBlocked {
			t.Error("mg-006 should be blocked")
		}
		if len(eval.BlockingIDs) != 1 || eval.BlockingIDs[0] != "mg-001" {
			t.Errorf("expected BlockingIDs [mg-001], got %v", eval.BlockingIDs)
		}
		if eval.NextBlockerID != "mg-001" {
			t.Errorf("expected NextBlockerID mg-001, got %s", eval.NextBlockerID)
		}
	})

	t.Run("missing", func(t *testing.T) {
		// mg-011 depends on mg-999 (does not exist)
		eval := issueMap["mg-011"].EvaluateDependencies(issueMap, DefaultBlockingTypes)
		if !eval.IsBlocked {
			t.Error("mg-011 should be blocked (dangling dep)")
		}
		if len(eval.MissingIDs) != 1 || eval.MissingIDs[0] != "mg-999" {
			t.Errorf("expected MissingIDs [mg-999], got %v", eval.MissingIDs)
		}
		if eval.NextBlockerID != "mg-999" {
			t.Errorf("expected NextBlockerID mg-999, got %s", eval.NextBlockerID)
		}
	})

	t.Run("resolved", func(t *testing.T) {
		// mg-013 depends on mg-008 (closed)
		eval := issueMap["mg-013"].EvaluateDependencies(issueMap, DefaultBlockingTypes)
		if eval.IsBlocked {
			t.Error("mg-013 should not be blocked (resolved dep)")
		}
		if len(eval.ResolvedIDs) != 1 || eval.ResolvedIDs[0] != "mg-008" {
			t.Errorf("expected ResolvedIDs [mg-008], got %v", eval.ResolvedIDs)
		}
	})

	t.Run("nonblocking", func(t *testing.T) {
		// mg-014 depends on mg-003 with type "discovered-from"
		eval := issueMap["mg-014"].EvaluateDependencies(issueMap, DefaultBlockingTypes)
		if eval.IsBlocked {
			t.Error("mg-014 should not be blocked (non-blocking type)")
		}
		if len(eval.NonBlocking) != 1 {
			t.Errorf("expected 1 NonBlocking edge, got %d", len(eval.NonBlocking))
		}
		if len(eval.NonBlocking) > 0 && eval.NonBlocking[0].Type != "discovered-from" {
			t.Errorf("expected NonBlocking type 'discovered-from', got %s", eval.NonBlocking[0].Type)
		}
	})
}

func TestEvaluateDependencies_DeDupe(t *testing.T) {
	issue := &Issue{
		ID:     "dup-1",
		Status: StatusOpen,
		Dependencies: []Dependency{
			{IssueID: "dup-1", DependsOnID: "dep-1", Type: "blocks"},
			{IssueID: "dup-1", DependsOnID: "dep-1", Type: "blocks"}, // duplicate
		},
	}
	dep := &Issue{ID: "dep-1", Status: StatusInProgress}
	issueMap := map[string]*Issue{"dup-1": issue, "dep-1": dep}

	eval := issue.EvaluateDependencies(issueMap, DefaultBlockingTypes)
	if len(eval.BlockingIDs) != 1 {
		t.Errorf("expected 1 blocking ID after dedup, got %d", len(eval.BlockingIDs))
	}
	if len(eval.Edges) != 1 {
		t.Errorf("expected 1 edge after dedup, got %d", len(eval.Edges))
	}
}

func TestParadeGroup_InProgressBlocked(t *testing.T) {
	// mg-012: in_progress but blocked by mg-001 → should be Stalled, not Rolling
	path := filepath.Join("..", "..", "testdata", "sample.jsonl")
	issues, err := LoadIssues(path)
	if err != nil {
		t.Fatalf("LoadIssues: %v", err)
	}
	issueMap := BuildIssueMap(issues)

	mg012 := issueMap["mg-012"]
	if mg012 == nil {
		t.Fatal("mg-012 not found")
	}
	if mg012.Status != StatusInProgress {
		t.Fatalf("expected mg-012 to be in_progress, got %s", mg012.Status)
	}

	group := mg012.ParadeGroup(issueMap, DefaultBlockingTypes)
	if group != ParadeStalled {
		t.Errorf("expected mg-012 to be Stalled, got %d", group)
	}
}

func TestParadeGroup_DanglingDep(t *testing.T) {
	// mg-011: open, depends on mg-999 (not found) → Stalled
	path := filepath.Join("..", "..", "testdata", "sample.jsonl")
	issues, err := LoadIssues(path)
	if err != nil {
		t.Fatalf("LoadIssues: %v", err)
	}
	issueMap := BuildIssueMap(issues)

	mg011 := issueMap["mg-011"]
	if mg011 == nil {
		t.Fatal("mg-011 not found")
	}

	group := mg011.ParadeGroup(issueMap, DefaultBlockingTypes)
	if group != ParadeStalled {
		t.Errorf("expected mg-011 to be Stalled (dangling dep), got %d", group)
	}
}

func TestParadeGroup_CustomBlockTypes(t *testing.T) {
	// When "discovered-from" is added to blocking types, mg-014 should be Stalled
	path := filepath.Join("..", "..", "testdata", "sample.jsonl")
	issues, err := LoadIssues(path)
	if err != nil {
		t.Fatalf("LoadIssues: %v", err)
	}
	issueMap := BuildIssueMap(issues)

	customTypes := map[string]bool{"blocks": true, "discovered-from": true}

	mg014 := issueMap["mg-014"]
	if mg014 == nil {
		t.Fatal("mg-014 not found")
	}

	// With default types: Lined Up (non-blocking dep type)
	group := mg014.ParadeGroup(issueMap, DefaultBlockingTypes)
	if group != ParadeLinedUp {
		t.Errorf("expected mg-014 to be LinedUp with default types, got %d", group)
	}

	// With custom types including "discovered-from": should be Stalled
	group = mg014.ParadeGroup(issueMap, customTypes)
	if group != ParadeStalled {
		t.Errorf("expected mg-014 to be Stalled with custom block types, got %d", group)
	}
}

func TestLoadRealBeads(t *testing.T) {
	// Use BEADS_JSONL env var if set, otherwise try common local path
	path := os.Getenv("BEADS_JSONL")
	if path == "" {
		// Fallback for local development only
		home, _ := os.UserHomeDir()
		candidates := []string{
			filepath.Join(home, "Work", "voice-vault", ".beads", "issues.jsonl"),
			".beads/issues.jsonl",
		}
		for _, c := range candidates {
			if _, err := os.Stat(c); err == nil {
				path = c
				break
			}
		}
	}
	if path == "" {
		t.Skip("no real beads data available (set BEADS_JSONL to override)")
	}

	issues, err := LoadIssues(path)
	if err != nil {
		t.Fatalf("LoadIssues (real data): %v", err)
	}

	if len(issues) == 0 {
		t.Error("expected at least 1 issue from real data")
	}

	groups := GroupByParade(issues, DefaultBlockingTypes)
	t.Logf("Real data: %d total, %d rolling, %d lined up, %d stalled, %d passed",
		len(issues),
		len(groups[ParadeRolling]),
		len(groups[ParadeLinedUp]),
		len(groups[ParadeStalled]),
		len(groups[ParadePastTheStand]),
	)
}
