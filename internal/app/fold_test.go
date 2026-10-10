package app

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/matt-wright86/mardi-gras/internal/data"
	"github.com/matt-wright86/mardi-gras/internal/gastown"
)

// foldModel builds a model over a small hierarchy (epic "ep" > "ep.1" >
// "ep.1.1", plus an unrelated "solo") with the cursor on ep.
func foldModel(t *testing.T) Model {
	t.Helper()
	child := func(id, parent string) data.Issue {
		iss := testIssue(id, data.StatusOpen)
		iss.Dependencies = []data.Dependency{{IssueID: id, DependsOnID: parent, Type: "parent-child"}}
		return iss
	}
	issues := []data.Issue{testIssue("ep", data.StatusOpen), child("ep.1", "ep"), child("ep.1.1", "ep.1"), testIssue("solo", data.StatusOpen)}
	m := New(issues, data.Source{}, data.DefaultBlockingTypes)
	m.startedAt = time.Now().Add(-time.Second) // bypass startup guard
	m.driver = gastown.NewGTDriver()           // host-independent, as setupModel does
	model, _ := m.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
	m = model.(Model)
	if !m.restoreParadeSelection("ep") {
		t.Fatal("ep not selectable")
	}
	return m
}

func paradeRows(m Model) string {
	var ids []string
	for _, it := range m.parade.Items {
		if it.Issue != nil {
			ids = append(ids, it.Issue.ID)
		}
	}
	return strings.Join(ids, " ")
}

func pressKey(m Model, r rune) (Model, tea.Cmd) {
	model, cmd := m.Update(tea.KeyPressMsg{Code: r, Text: string(r)})
	return model.(Model), cmd
}

// A fold must outlive the parade rebuild every reload does, pause under a
// filter, and come back when the filter clears.
func TestFoldSurvivesRebuild(t *testing.T) {
	m := foldModel(t)
	m, _ = pressKey(m, 'z')
	if got := paradeRows(m); got != "ep solo" {
		t.Fatalf("after z: rows = %q, want ep solo", got)
	}

	m.rebuildParade()
	if got := paradeRows(m); got != "ep solo" {
		t.Fatalf("after a reload: rows = %q, want the fold kept", got)
	}
	if m.parade.SelectedIssue == nil || m.parade.SelectedIssue.ID != "ep" {
		t.Errorf("after a reload: selection = %v, want ep", m.parade.SelectedIssue)
	}

	m.filterInput.SetValue("ep")
	m.rebuildParade()
	if got := paradeRows(m); !strings.Contains(got, "ep.1.1") {
		t.Fatalf("with a filter: rows = %q, want folds paused so matches show", got)
	}
	m.filterInput.SetValue("")
	m.rebuildParade()
	if got := paradeRows(m); got != "ep solo" {
		t.Errorf("filter cleared: rows = %q, want the fold back", got)
	}
}

// Selecting an issue mg just claimed or created unfolds whatever hides it.
func TestFoldRevealsPendingSelection(t *testing.T) {
	m := foldModel(t)
	m, _ = pressKey(m, 'Z')
	if got := paradeRows(m); got != "ep solo" {
		t.Fatalf("after Z: rows = %q", got)
	}
	m.pendingSelectID = "ep.1.1"
	m.rebuildParade()
	if m.parade.SelectedIssue == nil || m.parade.SelectedIssue.ID != "ep.1.1" {
		t.Fatalf("selection = %v, want the revealed ep.1.1 (rows %q)", m.parade.SelectedIssue, paradeRows(m))
	}
}

func TestFoldKeyEdgeCaseNoChildrenSaysWhy(t *testing.T) {
	m := foldModel(t)
	m.restoreParadeSelection("solo")
	m, _ = pressKey(m, 'z')
	if !strings.Contains(m.toast.Message, "solo has no children") {
		t.Errorf("toast = %q, want it to say solo has nothing to fold", m.toast.Message)
	}
}
