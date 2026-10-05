package app

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"

	"github.com/matt-wright86/mardi-gras/internal/components"
)

func TestOSCGuardAllowsNormalKeys(t *testing.T) {
	filter, sleep := newTestFilter()

	// Normal key presses with human-speed gaps should pass through.
	msg := tea.KeyPressMsg{Code: 'j', Text: "j"}
	if filter(nil, msg) == nil {
		t.Fatal("expected normal key 'j' to pass through")
	}

	sleep(50 * time.Millisecond)
	msg = tea.KeyPressMsg{Code: 'k', Text: "k"}
	if filter(nil, msg) == nil {
		t.Fatal("expected normal key 'k' to pass through")
	}
}

func TestOSCGuardSuppressesFastBurst(t *testing.T) {
	filter, _ := newTestFilter()

	// Simulate control-sequence tail burst: chars arriving with no
	// sleep between calls (effectively 0ms gap).
	burstChars := []struct {
		code rune
		text string
	}{
		{';', ";"},
		{'r', "r"},
		{'g', "g"},
		{'b', "b"},
		{':', ":"},
		{'1', "1"},
		{'f', "f"},
	}

	var suppressed int
	for _, ch := range burstChars {
		msg := tea.KeyPressMsg{Code: ch.code, Text: ch.text}
		if filter(nil, msg) == nil {
			suppressed++
		}
	}

	// The first char passes (no prior timing reference), but all subsequent
	// chars should be suppressed via burst detection + window.
	if suppressed < 5 {
		t.Fatalf("expected at least 5 suppressed keys in burst, got %d", suppressed)
	}
}

func TestOSCGuardWindowSuppressesSlowFollowers(t *testing.T) {
	filter, sleep := newTestFilter()

	// Two fast chars to trigger burst detection.
	filter(nil, tea.KeyPressMsg{Code: '1', Text: "1"})
	filter(nil, tea.KeyPressMsg{Code: ';', Text: ";"})

	// Now wait 50ms (within the 500ms window) and send another char.
	sleep(50 * time.Millisecond)
	msg := tea.KeyPressMsg{Code: ':', Text: ":"}
	if filter(nil, msg) != nil {
		t.Fatal("expected ':' to be suppressed within window")
	}
}

func TestOSCGuardAlwaysAllowsCtrlC(t *testing.T) {
	filter, _ := newTestFilter()

	// Even during a burst, ctrl+c should pass through.
	filter(nil, tea.KeyPressMsg{Code: '1', Text: "1"})
	filter(nil, tea.KeyPressMsg{Code: ';', Text: ";"})

	msg := tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
	if filter(nil, msg) == nil {
		t.Fatal("expected ctrl+c to pass through even during burst")
	}
}

func TestOSCGuardSuppressesModifierTailsDuringWindow(t *testing.T) {
	filter, _ := newTestFilter()

	// Trigger burst.
	filter(nil, tea.KeyPressMsg{Code: '1', Text: "1"})
	filter(nil, tea.KeyPressMsg{Code: ';', Text: ";"})

	// alt+\ (OSC string terminator tail) should be suppressed during window.
	msg := tea.KeyPressMsg{Code: '\\', Mod: tea.ModAlt}
	if filter(nil, msg) != nil {
		t.Fatal("expected alt+\\ to be suppressed during window")
	}

	// shift+R (CPR response tail byte) should be suppressed during window.
	msg = tea.KeyPressMsg{Code: 'r', Mod: tea.ModShift}
	if filter(nil, msg) != nil {
		t.Fatal("expected shift+r to be suppressed during window")
	}

	// shift+B (split CSI tail byte) should be suppressed during window.
	msg = tea.KeyPressMsg{Code: 'b', Mod: tea.ModShift}
	if filter(nil, msg) != nil {
		t.Fatal("expected shift+b to be suppressed during window")
	}
}

func TestOSCGuardAllowsModifierKeysOutsideWindow(t *testing.T) {
	filter, _ := newTestFilter()

	// Outside any burst window, modified keys should pass through.
	msg := tea.KeyPressMsg{Code: 'x', Mod: tea.ModAlt}
	if filter(nil, msg) == nil {
		t.Fatal("expected alt+x to pass through outside window")
	}

	msg = tea.KeyPressMsg{Code: 'r', Mod: tea.ModShift}
	if filter(nil, msg) == nil {
		t.Fatal("expected shift+r to pass through outside window")
	}
}

func TestOSCGuardAllowsNonCharKeys(t *testing.T) {
	filter, _ := newTestFilter()

	// Trigger burst.
	filter(nil, tea.KeyPressMsg{Code: '1', Text: "1"})
	filter(nil, tea.KeyPressMsg{Code: ';', Text: ";"})

	// Arrow keys should pass through even during burst.
	msg := tea.KeyPressMsg{Code: tea.KeyDown}
	if filter(nil, msg) == nil {
		t.Fatal("expected down arrow to pass through even during burst")
	}
}

func TestOSCGuardWindowExpires(t *testing.T) {
	filter, sleep := newTestFilter()

	// Trigger burst.
	filter(nil, tea.KeyPressMsg{Code: '1', Text: "1"})
	filter(nil, tea.KeyPressMsg{Code: ';', Text: ";"})

	// Wait for window to expire.
	sleep(600 * time.Millisecond)

	msg := tea.KeyPressMsg{Code: '1', Text: "1"}
	if filter(nil, msg) == nil {
		t.Fatal("expected '1' to pass through after window expired")
	}
}

func TestOSCGuardSuppressesAllCharsInWindow(t *testing.T) {
	filter, sleep := newTestFilter()

	// Trigger burst with two fast chars.
	filter(nil, tea.KeyPressMsg{Code: '1', Text: "1"})
	filter(nil, tea.KeyPressMsg{Code: ';', Text: ";"})

	// All printable chars should be suppressed during window.
	sleep(5 * time.Millisecond)

	for _, ch := range []struct {
		code rune
		text string
	}{
		{'r', "r"},
		{'g', "g"},
		{'b', "b"},
		{':', ":"},
		{'x', "x"},
		{'z', "z"},
	} {
		msg := tea.KeyPressMsg{Code: ch.code, Text: ch.text}
		if filter(nil, msg) != nil {
			t.Fatalf("expected %q to be suppressed within window", ch.text)
		}
	}
}

func TestOSCGuardSuppressesCharAfterNavKey(t *testing.T) {
	filter, sleep := newTestFilter()

	// Simulate: user presses 'down', then ']' arrives ~20ms later as
	// the first leaked byte of a torn control sequence.
	filter(nil, tea.KeyPressMsg{Code: tea.KeyDown})

	// The ']' arrives faster than a human could type after pressing down.
	sleep(20 * time.Millisecond)
	msg := tea.KeyPressMsg{Code: ']', Text: "]"}
	if filter(nil, msg) != nil {
		t.Fatal("expected ']' to be suppressed after nav key")
	}

	// Subsequent chars should also be suppressed (window is now active).
	msg = tea.KeyPressMsg{Code: '1', Text: "1"}
	if filter(nil, msg) != nil {
		t.Fatal("expected '1' to be suppressed in window after nav-triggered suppression")
	}
}

func TestOSCGuardAllowsCharLongAfterNavKey(t *testing.T) {
	filter, sleep := newTestFilter()

	// User presses 'down', then types 'j' 100ms later — normal usage.
	filter(nil, tea.KeyPressMsg{Code: tea.KeyDown})

	sleep(100 * time.Millisecond)
	msg := tea.KeyPressMsg{Code: 'j', Text: "j"}
	if filter(nil, msg) == nil {
		t.Fatal("expected 'j' to pass through 100ms after nav key")
	}
}

func TestOSCGuardAllowsCtrlCombosInWindow(t *testing.T) {
	filter, _ := newTestFilter()

	// Trigger burst.
	filter(nil, tea.KeyPressMsg{Code: '1', Text: "1"})
	filter(nil, tea.KeyPressMsg{Code: ';', Text: ";"})

	// ctrl+k should pass through during window (user shortcut).
	msg := tea.KeyPressMsg{Code: 'k', Mod: tea.ModCtrl}
	if filter(nil, msg) == nil {
		t.Fatal("expected ctrl+k to pass through during window")
	}

	// ctrl+n should pass through during window.
	msg = tea.KeyPressMsg{Code: 'n', Mod: tea.ModCtrl}
	if filter(nil, msg) == nil {
		t.Fatal("expected ctrl+n to pass through during window")
	}
}

func TestOSCGuardSuppressesKnownArtifactKeys(t *testing.T) {
	filter, _ := newTestFilter()

	msg := tea.KeyPressMsg{Code: '\\', Mod: tea.ModAlt}
	if filter(nil, msg) != nil {
		t.Fatal("expected alt+\\ artifact to be suppressed")
	}

	msg = tea.KeyPressMsg{Code: tea.KeyF3, Mod: tea.ModAlt | tea.ModMeta}
	if filter(nil, msg) != nil {
		t.Fatal("expected alt+meta+f3 artifact to be suppressed")
	}
}

// --- Layer 1: UnknownEvent suppression ---

func TestOSCGuardDropsUnknownEvent(t *testing.T) {
	filter, _ := newTestFilter()

	// uv.UnknownEvent from a torn sequence ultraviolet kept intact.
	msg := uv.UnknownEvent("\x1b]11;rgb:1f1f/2323/3535\\[5;6R")
	if filter(nil, msg) != nil {
		t.Fatal("expected uv.UnknownEvent to be dropped")
	}
}

func TestOSCGuardUnknownEventOpensWindow(t *testing.T) {
	filter, sleep := newTestFilter()

	// UnknownEvent should open a suppression window.
	filter(nil, uv.UnknownEvent("\x1b]11;rgb:1f1f/2323/3535\\"))

	// A char arriving soon after should be caught by the window.
	sleep(5 * time.Millisecond)
	msg := tea.KeyPressMsg{Code: '\\', Text: "\\"}
	if filter(nil, msg) != nil {
		t.Fatal("expected '\\' to be suppressed after UnknownEvent")
	}
}

func TestOSCGuardSuppressesTailAfterControlReply(t *testing.T) {
	filter, sleep := newTestFilter()

	// A parsed reply should open a short tail window.
	if filter(nil, tea.CursorPositionMsg{X: 5, Y: 6}) == nil {
		t.Fatal("expected cursor position reply to pass through")
	}

	sleep(5 * time.Millisecond)
	msg := tea.KeyPressMsg{Code: 'a', Mod: tea.ModShift}
	if filter(nil, msg) != nil {
		t.Fatal("expected shift+a tail to be suppressed after control reply")
	}
}

// --- Layer 3: content-aware pattern detection ---

func TestOSCGuardPatternSemiRG(t *testing.T) {
	filter, sleep := newTestFilter()

	// Simulate slow-dripped ;rgb: with human-scale gaps.
	// ';' and 'r' pass through (no pattern yet).
	// 'g' completes ";rg" and should be suppressed.
	filter(nil, tea.KeyPressMsg{Code: ';', Text: ";"})
	sleep(50 * time.Millisecond)
	filter(nil, tea.KeyPressMsg{Code: 'r', Text: "r"})
	sleep(50 * time.Millisecond)

	msg := tea.KeyPressMsg{Code: 'g', Text: "g"}
	if filter(nil, msg) != nil {
		t.Fatal("expected 'g' to be suppressed by ';rg' pattern")
	}

	// 'b' should be caught by the window opened by pattern match.
	sleep(50 * time.Millisecond)
	msg = tea.KeyPressMsg{Code: 'b', Text: "b"}
	if filter(nil, msg) != nil {
		t.Fatal("expected 'b' to be suppressed in window after pattern")
	}
}

func TestOSCGuardPatternBracket1(t *testing.T) {
	filter, sleep := newTestFilter()

	// Simulate slow-dripped ]11; from a torn OSC 11 introducer.
	// ']' passes through, '1' completes "]1" and should be suppressed.
	filter(nil, tea.KeyPressMsg{Code: ']', Text: "]"})
	sleep(50 * time.Millisecond)

	msg := tea.KeyPressMsg{Code: '1', Text: "1"}
	if filter(nil, msg) != nil {
		t.Fatal("expected '1' to be suppressed by ']1' pattern")
	}
}

func TestOSCGuardPatternBracketQuestion(t *testing.T) {
	filter, sleep := newTestFilter()

	// Simulate torn CSI private parameter: [?2026;2$y
	// '[' passes through, '?' completes "[?" and should be suppressed.
	filter(nil, tea.KeyPressMsg{Code: '[', Text: "["})
	sleep(50 * time.Millisecond)

	msg := tea.KeyPressMsg{Code: '?', Text: "?"}
	if filter(nil, msg) != nil {
		t.Fatal("expected '?' to be suppressed by '[?' pattern")
	}
}

func TestOSCGuardPatternBracketDigit(t *testing.T) {
	filter, sleep := newTestFilter()

	// Slow-dripped CPR prefix: [28;135R
	// '[' passes through, '2' is enough to identify a CSI parameter prefix.
	filter(nil, tea.KeyPressMsg{Code: '[', Text: "["})
	sleep(50 * time.Millisecond)

	msg := tea.KeyPressMsg{Code: '2', Text: "2"}
	if filter(nil, msg) != nil {
		t.Fatal("expected '2' to be suppressed by '[2' prefix")
	}
}

func TestOSCGuardPatternDollarY(t *testing.T) {
	filter, sleep := newTestFilter()

	// Simulate DECRPM terminator.
	filter(nil, tea.KeyPressMsg{Code: '$', Text: "$"})
	sleep(50 * time.Millisecond)

	msg := tea.KeyPressMsg{Code: 'y', Text: "y"}
	if filter(nil, msg) != nil {
		t.Fatal("expected 'y' to be suppressed by '$y' pattern")
	}
}

func TestOSCGuardPatternRGBColon(t *testing.T) {
	filter, sleep := newTestFilter()

	// "rgb:" matches even without leading ";".
	filter(nil, tea.KeyPressMsg{Code: 'r', Text: "r"})
	sleep(50 * time.Millisecond)
	filter(nil, tea.KeyPressMsg{Code: 'g', Text: "g"})
	sleep(50 * time.Millisecond)
	filter(nil, tea.KeyPressMsg{Code: 'b', Text: "b"})
	sleep(50 * time.Millisecond)

	msg := tea.KeyPressMsg{Code: ':', Text: ":"}
	if filter(nil, msg) != nil {
		t.Fatal("expected ':' to be suppressed by 'rgb:' pattern")
	}
}

func TestOSCGuardPatternResetsOnNavKey(t *testing.T) {
	filter, sleep := newTestFilter()

	// Start accumulating suspicious chars.
	filter(nil, tea.KeyPressMsg{Code: ';', Text: ";"})
	sleep(50 * time.Millisecond)

	// A nav key resets the accumulator.
	filter(nil, tea.KeyPressMsg{Code: tea.KeyDown})
	sleep(100 * time.Millisecond)

	// 'r' and 'g' no longer complete ";rg" because ';' was flushed.
	filter(nil, tea.KeyPressMsg{Code: 'r', Text: "r"})
	sleep(50 * time.Millisecond)

	msg := tea.KeyPressMsg{Code: 'g', Text: "g"}
	if filter(nil, msg) == nil {
		t.Fatal("expected 'g' to pass through — nav key reset accumulator")
	}
}

func TestOSCGuardPatternResetsOnLongGap(t *testing.T) {
	filter, sleep := newTestFilter()

	// Accumulate some chars.
	filter(nil, tea.KeyPressMsg{Code: ';', Text: ";"})
	filter(nil, tea.KeyPressMsg{Code: 'r', Text: "r"})

	// Wait > 2 seconds to reset the accumulator.
	sleep(2100 * time.Millisecond)

	// 'g' should NOT match ";rg" because accumulator was reset.
	msg := tea.KeyPressMsg{Code: 'g', Text: "g"}
	if filter(nil, msg) == nil {
		t.Fatal("expected 'g' to pass through — long gap reset accumulator")
	}
}

func TestOSCGuardPatternNoFalsePositiveOnNormalTyping(t *testing.T) {
	filter, sleep := newTestFilter()

	// Normal typing: individual chars with human-speed gaps
	// that don't form control-sequence patterns.
	chars := []struct {
		code rune
		text string
	}{
		{'h', "h"},
		{'e', "e"},
		{'l', "l"},
		{'p', "p"},
	}

	for _, ch := range chars {
		sleep(50 * time.Millisecond)
		msg := tea.KeyPressMsg{Code: ch.code, Text: ch.text}
		if filter(nil, msg) == nil {
			t.Fatalf("expected %q to pass through during normal typing", ch.text)
		}
	}
}

func TestOSCGuardSuppressesDigitSoonAfterNavKey(t *testing.T) {
	filter, sleep := newTestFilter()

	filter(nil, tea.KeyPressMsg{Code: tea.KeyDown})

	// 40ms is still too fast to be intentional "down then 1" input.
	sleep(40 * time.Millisecond)
	msg := tea.KeyPressMsg{Code: '1', Text: "1"}
	if filter(nil, msg) != nil {
		t.Fatal("expected '1' to be suppressed immediately after nav key")
	}
}

// fakeClock drives an OSCGuard deterministically. The guard's heuristics work
// on gaps of 5-50ms; with real sleeps a busy runner oversleeps and flips the
// "within the window" cases (a 40ms sleep against a 50ms window failed on CI).
// Each reading moves it on 100µs, as real time does between two events; the
// guard reads a zero gap as "no previous key", so a frozen clock would hide
// every burst.
type fakeClock struct{ t time.Time }

func (c *fakeClock) now() time.Time {
	c.t = c.t.Add(100 * time.Microsecond)
	return c.t
}

func (c *fakeClock) advance(d time.Duration) { c.t = c.t.Add(d) }

// newTestFilter returns a guard filter on a fake clock, and the sleep that
// advances it.
func newTestFilter() (filter func(tea.Model, tea.Msg) tea.Msg, sleep func(time.Duration)) {
	clk := &fakeClock{t: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	g := &OSCGuard{now: clk.now}
	return g.Filter(), clk.advance
}

// typeBurst feeds text through the guard 2ms per character, as dictation,
// a keyboard macro or an unbracketed paste delivers it, and returns what
// passed.
func typeBurst(g *OSCGuard, clk *fakeClock, text string) string {
	var passed []rune
	for _, r := range text {
		kp := tea.KeyPressMsg{Code: r, Text: string(r)}
		if r >= 'A' && r <= 'Z' {
			kp = tea.KeyPressMsg{Code: r + ('a' - 'A'), Text: string(r), Mod: tea.ModShift}
		}
		if g.filterMsg(kp) != nil {
			passed = append(passed, r)
		}
		clk.advance(2 * time.Millisecond)
	}
	return string(passed)
}

func TestOSCGuardTextEntryPassesBurst(t *testing.T) {
	// A fast burst into a text field used to keep only its first
	// character: GUARD-NAV dropped "u" after shift+B, then the window ate
	// the rest (mg-4ko).
	clk := &fakeClock{t: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	g := &OSCGuard{now: clk.now}
	g.SetTextEntry(true)
	if got := typeBurst(g, clk, "Burst typed title"); got != "Burst typed title" {
		t.Fatalf("passed %q, want the whole burst", got)
	}
}

func TestOSCGuardTextEntryEdgeCaseStillCatchesReplyFragment(t *testing.T) {
	// The content layer still applies in a text field: a torn OSC 11
	// reply must not be typed in whole.
	clk := &fakeClock{t: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	g := &OSCGuard{now: clk.now}
	g.SetTextEntry(true)
	if got := typeBurst(g, clk, "]11;rgb:1e1e/1e1e/1e1e"); got == "]11;rgb:1e1e/1e1e/1e1e" {
		t.Fatal("a reply fragment passed whole through a text field")
	}
}

func TestOSCGuardEdgeCaseBurstStillDroppedOutsideTextEntry(t *testing.T) {
	// Outside a text field a burst is still treated as reply traffic, so
	// leaked bytes cannot fire parade shortcuts.
	clk := &fakeClock{t: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	g := &OSCGuard{now: clk.now}
	if got := typeBurst(g, clk, "Burst"); got == "Burst" {
		t.Fatalf("passed %q outside text entry, want the burst suppressed", got)
	}
}

func TestUpdateSyncsTextEntryToGuard(t *testing.T) {
	m := setupModel(t)
	m.oscGuard = NewOSCGuard()
	// With a guard, printable shortcuts go through the deferred buffer:
	// deliver the held key the way the runtime would.
	model, cmd := m.Update(tea.KeyPressMsg{Code: 'N', Text: "N", Mod: tea.ModShift})
	if deferred, ok := cmd().(deferredKeyMsg); ok {
		model, _ = model.(Model).Update(deferred)
	}
	if !model.(Model).creating || !m.oscGuard.textEntry.Load() {
		t.Fatal("opening the create form should put the guard in text-entry mode")
	}
	model, _ = model.(Model).Update(components.CreateFormResult{Cancelled: true})
	if model.(Model).creating || m.oscGuard.textEntry.Load() {
		t.Fatal("leaving the form should take the guard out of text-entry mode")
	}
}
