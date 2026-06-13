package app

import (
	"testing"

	"github.com/matt-wright86/mardi-gras/internal/data"
	"github.com/matt-wright86/mardi-gras/internal/gastown"
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
	if len(c.particles) != confettiParticles {
		t.Errorf("expected %d particles, got %d", confettiParticles, len(c.particles))
	}
	if len(c.necklaces) != necklaceCount {
		t.Errorf("expected %d necklaces, got %d", necklaceCount, len(c.necklaces))
	}
}

func TestConfettiDeactivatesAfterFrames(t *testing.T) {
	c := NewConfetti(80, 20)
	for i := 0; i < confettiFrames; i++ {
		c.Update()
	}
	if c.Active() {
		t.Errorf("expected confetti to be inactive after %d updates", confettiFrames)
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

func TestBuildOrphanedIDsNil(t *testing.T) {
	if got := buildOrphanedIDs(nil); got != nil {
		t.Errorf("nil status should yield nil map, got %v", got)
	}
}

func TestBuildOrphanedIDsHealthyRig(t *testing.T) {
	status := &gastown.TownStatus{
		Rigs: []gastown.RigStatus{{Name: "mardi_gras", PolecatCount: 2}},
		Agents: []gastown.AgentRuntime{
			{Name: "obsidian", Role: "polecat", Rig: "mardi_gras", Running: true, HasWork: true, State: "working"},
		},
	}
	if got := buildOrphanedIDs(status); got != nil {
		t.Errorf("healthy rig should yield nil orphans, got %v", got)
	}
}

func TestBuildOrphanedIDsDeadRig(t *testing.T) {
	status := &gastown.TownStatus{
		Rigs: []gastown.RigStatus{{Name: "mardi_gras", PolecatCount: 0}},
		Agents: []gastown.AgentRuntime{
			{Name: "obsidian", Role: "polecat", Rig: "mardi_gras", Running: false, HookBead: "mg-001"},
			{Name: "quartz", Role: "polecat", Rig: "mardi_gras", Running: false, HookBead: "mg-002"},
		},
	}
	got := buildOrphanedIDs(status)
	if len(got) != 2 {
		t.Fatalf("expected 2 orphan IDs, got %d (%v)", len(got), got)
	}
	if !got["mg-001"] || !got["mg-002"] {
		t.Errorf("expected mg-001 and mg-002 in orphan set, got %v", got)
	}
}

func TestBuildZombieIDsNil(t *testing.T) {
	if got := buildZombieIDs(nil, nil); got != nil {
		t.Errorf("nil status should yield nil map, got %v", got)
	}
}

func TestBuildZombieIDsDeadHookedAgent(t *testing.T) {
	status := &gastown.TownStatus{
		Agents: []gastown.AgentRuntime{
			// Dead but hooked — should be a zombie.
			{Name: "obsidian", Role: "polecat", Rig: "live_rig", Running: false, HookBead: "lr-001"},
			// Running — not a zombie.
			{Name: "quartz", Role: "polecat", Rig: "live_rig", Running: true, HookBead: "lr-002", HasWork: true, State: "working"},
			// Dead but unhooked — not a zombie.
			{Name: "granite", Role: "polecat", Rig: "live_rig", Running: false},
		},
	}
	got := buildZombieIDs(status, nil)
	if len(got) != 1 {
		t.Fatalf("expected 1 zombie ID, got %d (%v)", len(got), got)
	}
	if !got["lr-001"] {
		t.Errorf("expected lr-001 in zombie set, got %v", got)
	}
}

func TestBuildZombieIDsSkipsOrphans(t *testing.T) {
	// Agent on a dead rig is an orphan, not a zombie — the dead-rig path
	// handles it. buildZombieIDs must skip these to avoid double counting.
	status := &gastown.TownStatus{
		Agents: []gastown.AgentRuntime{
			{Name: "obsidian", Role: "polecat", Rig: "dead_rig", Running: false, HookBead: "dr-001"},
			{Name: "stale", Role: "polecat", Rig: "live_rig", Running: false, HookBead: "lr-001"},
		},
	}
	orphans := map[string]bool{"dr-001": true}
	got := buildZombieIDs(status, orphans)
	if len(got) != 1 {
		t.Fatalf("expected 1 zombie (dr-001 suppressed by orphans), got %d (%v)", len(got), got)
	}
	if got["dr-001"] {
		t.Errorf("dr-001 should be suppressed by orphan set, but appeared as zombie: %v", got)
	}
	if !got["lr-001"] {
		t.Errorf("lr-001 should be a zombie, got %v", got)
	}
}

func TestBuildZombieIDsNoZombies(t *testing.T) {
	status := &gastown.TownStatus{
		Agents: []gastown.AgentRuntime{
			{Name: "obsidian", Running: true, HookBead: "mg-001", HasWork: true, State: "working"},
		},
	}
	if got := buildZombieIDs(status, nil); got != nil {
		t.Errorf("healthy agents should yield nil zombies, got %v", got)
	}
}

func TestOrchestratorAvailable(t *testing.T) {
	gc, err := gastown.NewGCDriver("http://127.0.0.1:8080", "")
	if err != nil {
		t.Fatalf("NewGCDriver: %v", err)
	}
	cases := []struct {
		name string
		m    Model
		want bool
	}{
		{"gt on PATH", Model{gtEnv: gastown.Env{Available: true}, driver: gastown.NewGTDriver()}, true},
		{"no orchestrator", Model{gtEnv: gastown.Env{Available: false}, driver: gastown.NewGTDriver()}, false},
		{"gas city only (no gt)", Model{gtEnv: gastown.Env{Available: false}, driver: gc}, true},
		{"both gt and gc", Model{gtEnv: gastown.Env{Available: true}, driver: gc}, true},
	}
	for _, c := range cases {
		if got := c.m.orchestratorAvailable(); got != c.want {
			t.Errorf("%s: orchestratorAvailable() = %v, want %v", c.name, got, c.want)
		}
	}
}
