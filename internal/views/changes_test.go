package views

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/matt-wright86/mardi-gras/internal/data"
)

var changesNow = time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)

func change(seq int64, op, id, actor string, ago time.Duration) data.JournalRecord {
	return data.JournalRecord{
		Seq: seq, Op: op, IssueID: id, Actor: actor,
		TS:    changesNow.Add(-ago).Format(time.RFC3339),
		Issue: &data.JournalIssue{Title: "Title " + id, Status: data.StatusOpen},
	}
}

func TestDescribeChange(t *testing.T) {
	blocked := change(1, "update", "a", "", 0)
	blocked.Issue.IsBlocked = true
	comment := change(2, "comment", "a", "you", 0)
	comment.Comment = &data.JournalComment{Text: "looks  good\nto me"}
	depAdd := change(3, "dep_add", "a", "you", 0)
	depAdd.Dep = &data.JournalDep{Kind: "blocks", Target: "b"}
	parent := change(4, "dep_remove", "a", "you", 0)
	parent.Dep = &data.JournalDep{Kind: "parent-child", Target: "p"}
	related := change(5, "dep_add", "a", "you", 0)
	related.Dep = &data.JournalDep{Kind: "related", Target: "c"}

	tests := []struct {
		r    data.JournalRecord
		want string
	}{
		{change(1, "create", "a", "you", 0), "created"},
		{change(1, "close", "a", "you", 0), "closed"},
		{data.JournalRecord{Op: "delete"}, "deleted"},
		{change(1, "update", "a", "you", 0), "updated (open)"},
		{change(1, "update", "a", "", 0), "unblocked"},
		{blocked, "became blocked"},
		{comment, "commented: looks good to me"},
		{depAdd, "now waits on b"},
		{parent, "moved out of p"},
		{related, "linked to c (related)"},
	}
	for _, tt := range tests {
		if got := DescribeChange(tt.r); got != tt.want {
			t.Errorf("%s: got %q, want %q", tt.r.Op, got, tt.want)
		}
	}
}

func TestChangesView(t *testing.T) {
	c := NewChanges(100, 20)
	c.now = func() time.Time { return changesNow }
	deleted := data.JournalRecord{Seq: 3, Op: "delete", IssueID: "gone-1", Actor: "you", TS: changesNow.Format(time.RFC3339)}
	c.SetRecords([]data.JournalRecord{
		change(1, "create", "old-1", "alice", time.Hour),
		change(2, "close", "new-1", "bob", 5*time.Second),
		deleted,
	}, map[string]*data.Issue{"gone-1": {ID: "gone-1", Title: "Deleted issue title"}}, "")

	view := c.View()
	newest, oldest := strings.Index(view, "gone-1"), strings.Index(view, "old-1")
	if newest < 0 || oldest < 0 || newest > oldest {
		t.Fatalf("expected newest first:\n%s", view)
	}
	for _, want := range []string{"closed", "· bob", "5s ago", "Deleted issue title"} {
		if !strings.Contains(view, want) {
			t.Errorf("view missing %q:\n%s", want, view)
		}
	}

	// Scrolling moves the window off the newest.
	c, _ = c.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if strings.Contains(c.View(), "gone-1") {
		t.Error("expected j to scroll past the newest record")
	}
}

func TestChangesViewNote(t *testing.T) {
	var c Changes // zero value: no clock set
	c.SetSize(80, 12)
	c.SetRecords(nil, nil, "The bd events journal is off for this workspace.")
	if view := c.View(); !strings.Contains(view, "journal is off") {
		t.Fatalf("expected the note:\n%s", view)
	}
	c.SetRecords([]data.JournalRecord{change(1, "create", "a", "you", time.Minute)}, nil, "")
	if view := c.View(); !strings.Contains(view, "created") {
		t.Fatalf("expected a record from a zero-value panel:\n%s", view)
	}
}
