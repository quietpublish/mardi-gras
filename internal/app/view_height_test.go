package app

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/matt-wright86/mardi-gras/internal/data"
)

// TestScreenHeightMatchesTerminal guards the screen height contract: the
// composed view must render exactly the terminal height, or the footer is
// pushed off-screen (regression: layout() rebuilt the detail viewport at full
// pane height, ignoring the scroll-cue row reserved by Detail.SetSize).
func TestScreenHeightMatchesTerminal(t *testing.T) {
	issues := []data.Issue{
		{ID: "x-1", Title: "One", Status: data.StatusInProgress, Priority: 0, IssueType: data.TypeTask, Description: strings.Repeat("line\n", 80)},
		{ID: "x-2", Title: "Two", Status: data.StatusOpen, Priority: 2, IssueType: data.TypeBug},
	}
	m := New(issues, data.Source{Mode: data.SourceJSONL, Path: "/tmp/x.jsonl"}, nil)
	nm, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	model := nm.(Model)
	if got := len(strings.Split(model.View().Content, "\n")); got != 40 {
		t.Fatalf("screen = %d lines, want 40", got)
	}
}
