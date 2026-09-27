package data

import (
	"slices"
	"testing"
	"time"
)

func baseIssue(id string) Issue {
	return Issue{
		ID:       id,
		Title:    "Title " + id,
		Status:   StatusOpen,
		Priority: PriorityMedium,
		Labels:   []string{"a", "b"},
		Dependencies: []Dependency{
			{IssueID: id, DependsOnID: "x", Type: "blocks"},
			{IssueID: id, DependsOnID: "y", Type: "related"},
		},
		Metadata: map[string]any{"team": "core"},
	}
}

func TestDiffIssues(t *testing.T) {
	due := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name   string
		mutate func(*Issue)
		want   bool // whether the issue counts as changed
	}{
		{"unchanged", func(*Issue) {}, false},
		{"status", func(i *Issue) { i.Status = StatusClosed }, true},
		{"title", func(i *Issue) { i.Title = "Renamed" }, true},
		{"description", func(i *Issue) { i.Description = "More detail" }, true},
		{"priority", func(i *Issue) { i.Priority = PriorityHigh }, true},
		{"type", func(i *Issue) { i.IssueType = "bug" }, true},
		{"assignee", func(i *Issue) { i.Assignee = "someone" }, true},
		{"notes", func(i *Issue) { i.Notes = "note" }, true},
		{"comment count", func(i *Issue) { i.CommentCount = 3 }, true},
		{"label added", func(i *Issue) { i.Labels = append(i.Labels, "c") }, true},
		{"labels reordered", func(i *Issue) { i.Labels = []string{"b", "a"} }, false},
		{"dep added", func(i *Issue) {
			i.Dependencies = append(i.Dependencies, Dependency{DependsOnID: "z", Type: "blocks"})
		}, true},
		{"dep type changed", func(i *Issue) { i.Dependencies[1].Type = "blocks" }, true},
		{"deps reordered", func(i *Issue) {
			i.Dependencies[0], i.Dependencies[1] = i.Dependencies[1], i.Dependencies[0]
		}, false},
		{"due date set", func(i *Issue) { i.DueAt = &due }, true},
		{"metadata", func(i *Issue) { i.Metadata = map[string]any{"team": "infra"} }, true},
		{"updated_at only", func(i *Issue) { i.UpdatedAt = due }, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prev := []Issue{baseIssue("a")}
			next := []Issue{baseIssue("a")}
			tt.mutate(&next[0])
			changed, removed := DiffIssues(prev, next)
			if got := slices.Contains(changed, "a"); got != tt.want {
				t.Errorf("changed = %v, want a changed: %v", changed, tt.want)
			}
			if len(removed) != 0 {
				t.Errorf("removed = %v, want none", removed)
			}
		})
	}
}

func TestDiffIssuesAddedAndRemoved(t *testing.T) {
	prev := []Issue{baseIssue("a"), baseIssue("b"), baseIssue("c")}
	next := []Issue{baseIssue("a"), baseIssue("d"), baseIssue("e")}
	changed, removed := DiffIssues(prev, next)
	if !slices.Equal(changed, []string{"d", "e"}) {
		t.Errorf("changed = %v, want [d e]", changed)
	}
	if !slices.Equal(removed, []string{"b", "c"}) {
		t.Errorf("removed = %v, want [b c]", removed)
	}
}

func TestDiffIssuesTimePointers(t *testing.T) {
	// Equal instants in different locations are the same due date.
	utc := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	local := utc.In(time.FixedZone("CDT", -5*3600))
	prev := []Issue{baseIssue("a")}
	next := []Issue{baseIssue("a")}
	prev[0].DueAt = &utc
	next[0].DueAt = &local
	if changed, _ := DiffIssues(prev, next); len(changed) != 0 {
		t.Errorf("changed = %v, want none for the same instant", changed)
	}
}
