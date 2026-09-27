package data

import (
	"reflect"
	"slices"
	"time"
)

// DiffIssues compares two loads of the same workspace. changed lists the IDs
// in next that are new or differ from prev in a field mg shows; removed lists
// the IDs in prev that are missing from next. Both keep their input order.
func DiffIssues(prev, next []Issue) (changed, removed []string) {
	prevByID := make(map[string]*Issue, len(prev))
	for i := range prev {
		prevByID[prev[i].ID] = &prev[i]
	}
	seen := make(map[string]bool, len(next))
	for i := range next {
		iss := &next[i]
		seen[iss.ID] = true
		if old, ok := prevByID[iss.ID]; !ok || !sameVisibleFields(old, iss) {
			changed = append(changed, iss.ID)
		}
	}
	for i := range prev {
		if !seen[prev[i].ID] {
			removed = append(removed, prev[i].ID)
		}
	}
	return changed, removed
}

// sameVisibleFields reports whether two versions of an issue look the same
// in the parade and detail panel. It compares fields rather than UpdatedAt,
// because bd does not bump updated_at for label or comment changes.
// Timestamps that only move with status (started, closed) are left out.
func sameVisibleFields(a, b *Issue) bool {
	return a.Title == b.Title &&
		a.Description == b.Description &&
		a.Status == b.Status &&
		a.Priority == b.Priority &&
		a.IssueType == b.IssueType &&
		a.Owner == b.Owner &&
		a.Assignee == b.Assignee &&
		a.CloseReason == b.CloseReason &&
		a.Notes == b.Notes &&
		a.Design == b.Design &&
		a.AcceptanceCriteria == b.AcceptanceCriteria &&
		a.CommentCount == b.CommentCount &&
		sameTime(a.DueAt, b.DueAt) &&
		sameTime(a.DeferUntil, b.DeferUntil) &&
		sameSet(a.Labels, b.Labels) &&
		sameSet(depKeys(a.Dependencies), depKeys(b.Dependencies)) &&
		reflect.DeepEqual(a.Metadata, b.Metadata)
}

func sameTime(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Equal(*b)
}

// sameSet compares two string lists as sets, so a reorder is not a change.
func sameSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	a, b = slices.Clone(a), slices.Clone(b)
	slices.Sort(a)
	slices.Sort(b)
	return slices.Equal(a, b)
}

func depKeys(deps []Dependency) []string {
	keys := make([]string, len(deps))
	for i, d := range deps {
		keys[i] = d.Type + "|" + d.DependsOnID
	}
	return keys
}
