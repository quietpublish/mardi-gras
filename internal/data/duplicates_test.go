package data

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func dupIssue(id, title string, status Status, updated time.Time) Issue {
	return Issue{ID: id, Title: title, Status: status, UpdatedAt: updated, CreatedAt: updated}
}

func TestDuplicateCandidates(t *testing.T) {
	now := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	recent := now.Add(-3 * 24 * time.Hour)
	old := now.Add(-30 * 24 * time.Hour)
	closedRecent := dupIssue("mg-5", "CI pipeline timeout on main", StatusClosed, recent)
	closedRecent.ClosedAt = &recent
	closedOld := dupIssue("mg-6", "CI pipeline timeout flake", StatusClosed, old)
	closedOld.ClosedAt = &old
	issues := []Issue{
		dupIssue("mg-1", "Login loop on Safari after 2FA", StatusOpen, recent),
		dupIssue("mg-2", "Fix CI pipeline timeout", StatusInProgress, recent),
		dupIssue("mg-3", "Flaky integration stage in the pipeline", StatusOpen, old),
		dupIssue("mg-4", "Update README", StatusOpen, recent),
		closedRecent,
		closedOld,
	}

	got := DuplicateCandidates(issues, "CI pipeline times out", now, 8)
	ids := make([]string, len(got))
	for i, iss := range got {
		ids[i] = iss.ID
	}
	joined := strings.Join(ids, ",")
	if len(got) == 0 || got[0].ID != "mg-2" {
		t.Fatalf("best candidate should be the near-identical title, got %s", joined)
	}
	if !strings.Contains(joined, "mg-5") {
		t.Errorf("a recently closed match should be a candidate: %s", joined)
	}
	for _, excluded := range []string{"mg-1", "mg-4", "mg-6"} {
		if strings.Contains(joined, excluded) {
			t.Errorf("%s should not be a candidate: %s", excluded, joined)
		}
	}
}

func TestDuplicateCandidatesEdgeCaseCapAndOrder(t *testing.T) {
	now := time.Now()
	var issues []Issue
	for i := range 12 {
		issues = append(issues, dupIssue(string(rune('a'+i)), "parade render glitch", StatusOpen, now.Add(-time.Duration(i)*time.Hour)))
	}
	got := DuplicateCandidates(issues, "parade render glitch", now, 8)
	if len(got) != 8 {
		t.Fatalf("got %d candidates, want the cap of 8", len(got))
	}
	if got[0].ID != "a" {
		t.Fatalf("ties should prefer the most recently updated, got %s", got[0].ID)
	}
}

func TestDuplicateCandidatesEdgeCaseStopwordsOnly(t *testing.T) {
	issues := []Issue{dupIssue("x", "Fix the thing", StatusOpen, time.Now())}
	if got := DuplicateCandidates(issues, "fix the", time.Now(), 8); got != nil {
		t.Fatalf("a title of stopwords has no candidates, got %v", got)
	}
	if got := DuplicateCandidates(issues, "thing", time.Now(), 0); got != nil {
		t.Fatalf("max 0 means none, got %v", got)
	}
}

func TestAddDependencyTyped(t *testing.T) {
	calls, restore := mockExecCapture(nil)
	defer restore()
	if err := AddDependencyTyped("mg-9", "mg-2", "duplicates"); err != nil {
		t.Fatal(err)
	}
	want := "bd dep add --type=duplicates mg-9 -- mg-2"
	if got := strings.Join((*calls)[0], " "); got != want {
		t.Fatalf("ran %q, want %q", got, want)
	}
}

func TestAddDependencyTypedEdgeCaseRejectsBadInput(t *testing.T) {
	calls, restore := mockExecCapture(nil)
	defer restore()
	for _, in := range [][3]string{{"", "mg-2", "duplicates"}, {"mg-9", "", "duplicates"}, {"mg-9", "mg-2", "--flag"}, {"mg-9", "mg-2", "dup licates"}} {
		if err := AddDependencyTyped(in[0], in[1], in[2]); err == nil {
			t.Errorf("AddDependencyTyped(%q, %q, %q) accepted bad input", in[0], in[1], in[2])
		}
	}
	if len(*calls) != 0 {
		t.Fatal("bad input must not reach bd")
	}
}

func TestAddDependencyTypedEdgeCaseBdError(t *testing.T) {
	_, restore := mockExecCapture(errors.New("exit status 1"))
	defer restore()
	if err := AddDependencyTyped("mg-9", "mg-2", "related"); err == nil {
		t.Fatal("bd failure should surface")
	}
}
