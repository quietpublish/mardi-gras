package app

import (
	"testing"

	"github.com/matt-wright86/mardi-gras/internal/data"
)

// ---------------------------------------------------------------------------
// plural
// ---------------------------------------------------------------------------

func TestPlural(t *testing.T) {
	tests := []struct {
		n    int
		want string
	}{
		{0, "s"},
		{1, ""},
		{5, "s"},
	}
	for _, tt := range tests {
		got := plural(tt.n)
		if got != tt.want {
			t.Errorf("plural(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// splitLines
// ---------------------------------------------------------------------------

func TestSplitLines(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{"empty string", "", []string{""}},
		{"single line", "abc", []string{"abc"}},
		{"multi-line", "a\nb\nc", []string{"a", "b", "c"}},
		{"trailing newline", "a\nb\n", []string{"a", "b", ""}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitLines(tt.in)
			if len(got) != len(tt.want) {
				t.Fatalf("splitLines(%q) len = %d, want %d", tt.in, len(got), len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("splitLines(%q)[%d] = %q, want %q", tt.in, i, got[i], tt.want[i])
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// joinLines
// ---------------------------------------------------------------------------

func TestJoinLines(t *testing.T) {
	// Roundtrip: splitLines → joinLines should recover original string.
	original := "a\nb\nc"
	if got := joinLines(splitLines(original)); got != original {
		t.Errorf("roundtrip failed: got %q, want %q", got, original)
	}

	// Single element → no newline.
	if got := joinLines([]string{"abc"}); got != "abc" {
		t.Errorf("single element: got %q, want %q", got, "abc")
	}

	// Empty slice → empty string.
	if got := joinLines([]string{}); got != "" {
		t.Errorf("empty slice: got %q, want %q", got, "")
	}
}

// ---------------------------------------------------------------------------
// overlayStrings
// ---------------------------------------------------------------------------

func TestOverlayStrings(t *testing.T) {
	// Space chars in overlay don't overwrite base.
	base := "hello"
	overlay := "  X  "
	got := overlayStrings(base, overlay)
	if got != "heXlo" {
		t.Errorf("space passthrough: got %q, want %q", got, "heXlo")
	}

	// Non-space chars do overwrite.
	got = overlayStrings("abcde", "12345")
	if got != "12345" {
		t.Errorf("full overwrite: got %q, want %q", got, "12345")
	}

	// Overlay shorter than base.
	got = overlayStrings("abcde", "XY")
	if got != "XYcde" {
		t.Errorf("shorter overlay: got %q, want %q", got, "XYcde")
	}

	// Overlay longer than base (truncated to base length).
	got = overlayStrings("ab", "XYZW")
	if got != "XY" {
		t.Errorf("longer overlay: got %q, want %q", got, "XY")
	}
}

// ---------------------------------------------------------------------------
// diffIssues
// ---------------------------------------------------------------------------

func TestDiffIssuesEmptyPrev(t *testing.T) {
	m := Model{
		prevIssueMap: map[string]data.Status{},
		changedIDs:   make(map[string]bool),
	}
	issues := []data.Issue{testIssue("a", data.StatusOpen)}
	if got := m.diffIssues(issues); got != 0 {
		t.Errorf("empty prevIssueMap: got %d changes, want 0", got)
	}
}

func TestDiffIssuesStatusChanged(t *testing.T) {
	m := Model{
		prevIssueMap: map[string]data.Status{
			"a": data.StatusOpen,
		},
		changedIDs: make(map[string]bool),
	}
	issues := []data.Issue{testIssue("a", data.StatusInProgress)}
	got := m.diffIssues(issues)
	if got != 1 {
		t.Errorf("status changed: got %d changes, want 1", got)
	}
	if !m.changedIDs["a"] {
		t.Error("expected changedIDs to contain 'a'")
	}
}

func TestDiffIssuesNewAndRemoved(t *testing.T) {
	m := Model{
		prevIssueMap: map[string]data.Status{
			"old": data.StatusOpen,
		},
		changedIDs: make(map[string]bool),
	}
	issues := []data.Issue{testIssue("new", data.StatusOpen)}
	got := m.diffIssues(issues)
	// 1 new issue + 1 removed issue = 2
	if got != 2 {
		t.Errorf("new+removed: got %d changes, want 2", got)
	}
	if !m.changedIDs["new"] {
		t.Error("expected changedIDs to contain 'new'")
	}
}

func TestDiffIssuesNoChange(t *testing.T) {
	m := Model{
		prevIssueMap: map[string]data.Status{
			"a": data.StatusOpen,
			"b": data.StatusClosed,
		},
		changedIDs: make(map[string]bool),
	}
	issues := []data.Issue{
		testIssue("a", data.StatusOpen),
		testIssue("b", data.StatusClosed),
	}
	if got := m.diffIssues(issues); got != 0 {
		t.Errorf("no change: got %d changes, want 0", got)
	}
}

// ---------------------------------------------------------------------------
// Confetti
// ---------------------------------------------------------------------------

func TestNewConfettiActive(t *testing.T) {
	c := NewConfetti(80, 20)
	if !c.Active() {
		t.Error("expected new confetti to be active")
	}
	if len(c.particles) != 35 {
		t.Errorf("expected 35 particles, got %d", len(c.particles))
	}
}

func TestConfettiDeactivatesAfterFrames(t *testing.T) {
	c := NewConfetti(80, 20)
	for i := 0; i < 18; i++ {
		c.Update()
	}
	if c.Active() {
		t.Error("expected confetti to be inactive after 18 updates")
	}
}

func TestConfettiTickNilWhenInactive(t *testing.T) {
	c := Confetti{active: false}
	if cmd := c.Tick(); cmd != nil {
		t.Error("expected nil Tick command for inactive confetti")
	}
}

func TestConfettiViewEmptyWhenInactive(t *testing.T) {
	c := Confetti{active: false, width: 80, height: 20}
	if got := c.View(); got != "" {
		t.Errorf("expected empty view for inactive confetti, got %q", got)
	}
}

func TestConfettiViewNonEmptyWhenActive(t *testing.T) {
	c := NewConfetti(80, 20)
	got := c.View()
	if got == "" {
		t.Error("expected non-empty view for active confetti with w/h > 0")
	}
}
