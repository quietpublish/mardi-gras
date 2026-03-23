package gastown

import (
	"testing"

	"github.com/matt-wright86/mardi-gras/internal/data"
)

func TestComputeScorecards(t *testing.T) {
	issues := []data.Issue{
		{ID: "a", Status: data.StatusClosed, Assignee: "Toast"},
		{ID: "b", Status: data.StatusClosed, Assignee: "Toast"},
		{ID: "c", Status: data.StatusClosed, Assignee: "Muffin"},
		// Open issue — should not count
		{ID: "d", Status: data.StatusOpen, Assignee: "Toast"},
		// Closed with no assignee — skipped
		{ID: "e", Status: data.StatusClosed},
		// Another closed for Toast
		{ID: "f", Status: data.StatusClosed, Assignee: "Toast"},
	}

	cards := ComputeScorecards(issues)

	if len(cards) != 2 {
		t.Fatalf("expected 2 scorecards, got %d", len(cards))
	}

	// Should be sorted by issues closed descending
	// Toast: 3 closed, Muffin: 1 closed
	if cards[0].Name != "Toast" {
		t.Fatalf("expected Toast first (most closed), got %q", cards[0].Name)
	}
	if cards[0].IssuesClosed != 3 {
		t.Fatalf("Toast IssuesClosed = %d, want 3", cards[0].IssuesClosed)
	}

	if cards[1].Name != "Muffin" {
		t.Fatalf("expected Muffin second, got %q", cards[1].Name)
	}
	if cards[1].IssuesClosed != 1 {
		t.Fatalf("Muffin IssuesClosed = %d, want 1", cards[1].IssuesClosed)
	}
}

func TestComputeScorecardsEmpty(t *testing.T) {
	cards := ComputeScorecards(nil)
	if len(cards) != 0 {
		t.Fatalf("expected 0 scorecards, got %d", len(cards))
	}
}

func TestComputeScorecardsNoClosedIssues(t *testing.T) {
	issues := []data.Issue{
		{ID: "a", Status: data.StatusOpen, Assignee: "Toast"},
		{ID: "b", Status: data.StatusInProgress, Assignee: "Muffin"},
	}
	cards := ComputeScorecards(issues)
	if len(cards) != 0 {
		t.Fatalf("expected 0 scorecards for no closed issues, got %d", len(cards))
	}
}
