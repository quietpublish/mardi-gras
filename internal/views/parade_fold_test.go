package views

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/matt-wright86/mardi-gras/internal/data"
	"github.com/matt-wright86/mardi-gras/internal/ui"
)

// foldFixture is a small hierarchy: epic "ep" with an open child "ep.1"
// (which has its own child "ep.1.1") and an in-progress child "ep.2" that
// lands in Rolling, plus an unrelated root "solo". This repo's own data has
// no parent-child edges, so hierarchy tests use fixtures like this one.
func foldFixture() []data.Issue {
	child := func(id, parent string, status data.Status) data.Issue {
		iss := testIssue(id, status)
		iss.Dependencies = []data.Dependency{{IssueID: id, DependsOnID: parent, Type: "parent-child"}}
		return iss
	}
	return []data.Issue{
		testIssue("ep", data.StatusOpen),
		child("ep.1", "ep", data.StatusOpen),
		child("ep.1.1", "ep.1", data.StatusOpen),
		child("ep.2", "ep", data.StatusInProgress),
		testIssue("solo", data.StatusOpen),
	}
}

func foldParade(t *testing.T) Parade {
	t.Helper()
	return NewParade(foldFixture(), 100, 40, data.DefaultBlockingTypes)
}

// visibleIDs lists the issue rows the parade shows, in order.
func visibleIDs(p Parade) []string {
	var ids []string
	for _, it := range p.Items {
		if it.Issue != nil {
			ids = append(ids, it.Issue.ID)
		}
	}
	return ids
}

func selectID(t *testing.T, p *Parade, id string) {
	t.Helper()
	for i, it := range p.Items {
		if it.Issue != nil && it.Issue.ID == id {
			p.Cursor, p.SelectedIssue = i, it.Issue
			return
		}
	}
	t.Fatalf("%s is not a visible row: %v", id, visibleIDs(*p))
}

func itemFor(p Parade, id string) *ParadeItem {
	for i := range p.Items {
		if p.Items[i].Issue != nil && p.Items[i].Issue.ID == id {
			return &p.Items[i]
		}
	}
	return nil
}

func TestParadeToggleFold(t *testing.T) {
	p := foldParade(t)
	selectID(t, &p, "ep")
	if !p.ToggleFold() {
		t.Fatal("ToggleFold on a parent = false")
	}
	got := strings.Join(visibleIDs(p), " ")
	// ep.2 sits in Rolling, a root there: folding ep must not hide it.
	if got != "ep.2 ep solo" {
		t.Fatalf("visible after folding ep = %q, want %q", got, "ep.2 ep solo")
	}
	ep := itemFor(p, "ep")
	if ep.Hidden != 2 {
		t.Errorf("ep.Hidden = %d, want 2 (ep.1 and ep.1.1)", ep.Hidden)
	}
	if row := ansi.Strip(p.renderIssue(*ep, true, 0)); !strings.Contains(row, ui.SymFolded+"+2") {
		t.Errorf("collapsed row = %q, want the %s+2 badge", row, ui.SymFolded)
	}
	if p.SelectedIssue.ID != "ep" {
		t.Errorf("selection = %s, want it to stay on ep", p.SelectedIssue.ID)
	}

	if !p.ToggleFold() {
		t.Fatal("second ToggleFold = false")
	}
	if got := strings.Join(visibleIDs(p), " "); got != "ep.2 ep ep.1 ep.1.1 solo" {
		t.Fatalf("visible after unfolding = %q", got)
	}
	if itemFor(p, "ep").Hidden != 0 {
		t.Error("an expanded parent should carry no hidden count")
	}
}

func TestParadeToggleFoldEdgeCaseNoChildren(t *testing.T) {
	p := foldParade(t)
	selectID(t, &p, "solo")
	before := strings.Join(visibleIDs(p), " ")
	if p.ToggleFold() {
		t.Fatal("ToggleFold on an issue with no children = true")
	}
	if got := strings.Join(visibleIDs(p), " "); got != before || len(p.Collapsed) != 0 {
		t.Errorf("rows changed to %q (collapsed %v), want no change", got, p.Collapsed)
	}
	// ep.2's parent is in another section, so here it has nothing to fold.
	selectID(t, &p, "ep.2")
	if p.ToggleFold() {
		t.Error("ToggleFold on a Rolling root whose parent is Lined Up = true")
	}
}

func TestParadeToggleAllFolds(t *testing.T) {
	p := foldParade(t)
	selectID(t, &p, "ep.1.1")
	collapsed, ok := p.ToggleAllFolds()
	if !ok || !collapsed {
		t.Fatalf("ToggleAllFolds = %v, %v; want collapsed", collapsed, ok)
	}
	if got := strings.Join(visibleIDs(p), " "); got != "ep.2 ep solo" {
		t.Fatalf("visible after collapsing all = %q", got)
	}
	// The cursor was on a row that just folded away; it moves to the parent
	// that now stands in for it.
	if p.SelectedIssue == nil || p.SelectedIssue.ID != "ep" {
		t.Errorf("selection = %v, want ep", p.SelectedIssue)
	}

	collapsed, ok = p.ToggleAllFolds()
	if !ok || collapsed {
		t.Fatalf("second ToggleAllFolds = %v, %v; want expanded", collapsed, ok)
	}
	if len(p.Collapsed) != 0 || len(visibleIDs(p)) != 5 {
		t.Errorf("after expanding all: collapsed %v, rows %v", p.Collapsed, visibleIDs(p))
	}
}

func TestParadeRevealIssue(t *testing.T) {
	p := foldParade(t)
	p.ToggleAllFolds() // folds ep and ep.1
	if !p.RevealIssue("ep.1.1") {
		t.Fatal("RevealIssue on a folded issue = false")
	}
	if itemFor(p, "ep.1.1") == nil {
		t.Fatalf("ep.1.1 still hidden: %v", visibleIDs(p))
	}
	if p.Collapsed["ep"] || p.Collapsed["ep.1"] {
		t.Errorf("collapsed = %v, want both ancestors opened", p.Collapsed)
	}
	if p.RevealIssue("solo") {
		t.Error("RevealIssue on a visible issue = true")
	}
}

func TestParadeSetFolds(t *testing.T) {
	p := foldParade(t)
	p.SetFolds(map[string]bool{"ep": true}, false)
	if got := strings.Join(visibleIDs(p), " "); got != "ep.2 ep solo" {
		t.Fatalf("visible with carried-over fold = %q", got)
	}

	// Suspended (filter or focus mode): everything shows, and z declines.
	p = foldParade(t)
	p.SetFolds(map[string]bool{"ep": true}, true)
	if len(visibleIDs(p)) != 5 {
		t.Fatalf("suspended folds still hide rows: %v", visibleIDs(p))
	}
	selectID(t, &p, "ep")
	if p.ToggleFold() {
		t.Error("ToggleFold while suspended = true")
	}
	if !p.Collapsed["ep"] {
		t.Error("suspending folds must keep them for when the filter clears")
	}
}
