package app

import (
	"strings"
	"sync/atomic"
	"time"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
)

const (
	// oscGuardWindow is the suppression window opened when a burst of
	// fragmented terminal reply traffic is detected. Empirically, OSC 11
	// + DECRPM reply bursts complete within ~60ms. The previous 500ms
	// value ate real keypresses typed at normal speed (~120ms+ apart).
	// See: charmbracelet/bubbletea#1590, docs/internal/osc-leak-investigation.md
	oscGuardWindow     = 80 * time.Millisecond
	oscGuardTailWindow = 40 * time.Millisecond
)

// OSCGuard coordinates control-sequence suppression across the Bubble Tea
// filter and app-level deferred key handling.
type OSCGuard struct {
	lastPrintableTime time.Time
	lastAnyKeyTime    time.Time
	lastControlTime   time.Time
	lastSuspiciousAt  time.Time
	suppressUntil     time.Time
	seqBuf            [16]byte
	seqLen            int

	// textEntry is set by the app while a text field has focus; see
	// SetTextEntry. Atomic because the filter and Update are separate
	// call sites.
	textEntry atomic.Bool

	// now is the guard's clock; nil means time.Now. Its heuristics work on
	// gaps of 5-50ms, too fine to test with real sleeps on a busy machine.
	now func() time.Time
}

func (g *OSCGuard) clock() time.Time {
	if g.now != nil {
		return g.now()
	}
	return time.Now()
}

// NewOSCGuard constructs a shared guard instance for a program run.
func NewOSCGuard() *OSCGuard {
	return &OSCGuard{}
}

// NewOSCGuardFilter returns a standalone tea.WithFilter function.
//
// This is preserved for tests and callers that only need the filter.
func NewOSCGuardFilter() func(tea.Model, tea.Msg) tea.Msg {
	return NewOSCGuard().Filter()
}

// Filter returns a tea.WithFilter-compatible function.
//
// BubbleTea v2 performs more terminal capability negotiation than v1
// (DECRPM, Kitty keyboard, modifyOtherKeys). In the VS Code integrated
// terminal, reply traffic (OSC 11 background color, CPR cursor position,
// DECRPM mode reports) sometimes arrives fragmented. Ultraviolet's parser
// cannot reassemble the fragments before its ESC timeout expires, so the
// tail bytes degrade into individual tea.KeyPressMsg events.
//
// The guard uses three layers of defense:
//
//  1. Unknown event filter: uv.UnknownEvent messages (opaque fragments
//     that ultraviolet kept intact but could not identify) are dropped.
//
//  2. Timing heuristics: two printable-char keys arriving < 15ms apart,
//     or a printable char within 50ms of a non-char key, opens a 500ms
//     suppression window. During the window all printable keys are
//     dropped regardless of shift/alt modifiers.
//
//  3. Content-aware pattern detection: an accumulator tracks recent
//     printable chars that passed through the timing guard. When the
//     accumulated tail matches a known control-sequence fragment
//     (";rg", "rgb:", "[?", "$y") or a likely CSI/OSC prefix
//     ("[2", "]1", "[A"), the triggering char is suppressed and a
//     500ms window opens. This catches slow-dripped fragments that
//     arrive with human-scale gaps (18-200ms).
//
// Ctrl combos and non-character keys always pass through.
//
// See docs/internal/osc-leak-investigation.md for full analysis.
func (g *OSCGuard) Filter() func(tea.Model, tea.Msg) tea.Msg {
	return func(_ tea.Model, msg tea.Msg) tea.Msg {
		return g.filterMsg(msg)
	}
}

func (g *OSCGuard) filterMsg(msg tea.Msg) tea.Msg {
	now := g.clock()

	// Layer 1: drop opaque unknown events (torn sequences UV kept
	// intact but could not identify).
	if _, ok := msg.(uv.UnknownEvent); ok {
		dbg("  GUARD-UNKNOWN dropped: %q", msg)
		g.markSuspicious(now, oscGuardWindow)
		g.lastControlTime = now
		g.seqLen = 0
		return nil
	}

	// Cleanly parsed control replies are useful, but they also tell us
	// fragmented tail bytes may still be in flight. Open a short follow-up
	// window so a split final byte (for example bare "A" from ESC[A) does
	// not reach Update as a real key.
	switch msg.(type) {
	case tea.BackgroundColorMsg, tea.CursorPositionMsg, tea.ModeReportMsg:
		g.lastControlTime = now
		g.markSuspicious(now, oscGuardTailWindow)
		g.seqLen = 0
		return msg
	}

	kp, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return msg
	}

	// Some artifacts are specific enough that suppressing them outright is
	// safer than letting them hit Update as phantom shortcuts.
	if isKnownArtifactKey(kp) {
		dbg("  GUARD-ARTIFACT suppressed: %q", kp.String())
		g.markSuspicious(now, oscGuardWindow)
		g.seqLen = 0
		return nil
	}

	// Ctrl combos always pass through (ctrl+c, ctrl+k, etc.).
	if kp.Mod&tea.ModCtrl != 0 {
		g.lastAnyKeyTime = now
		g.seqLen = 0
		dbg("  GUARD-PASS ctrl combo: %q", kp.String())
		return msg
	}

	// A reply parsed cleanly, but a final byte still leaked immediately
	// after it. Treat the leaked tail as artifact traffic.
	if isPrintableKey(kp) && now.Sub(g.lastControlTime) > 0 && now.Sub(g.lastControlTime) < oscGuardTailWindow && isReplyTailKey(kp) {
		g.markSuspicious(now, oscGuardWindow)
		g.seqLen = 0
		dbg("  GUARD-TAIL suppressed: %q (gap from control=%v)", kp.String(), now.Sub(g.lastControlTime))
		return nil
	}

	// Non-printable keys (arrows, enter, esc, tab, function keys,
	// shift+down, etc.) always pass through. Track timing and
	// reset accumulator.
	if !isPrintableKey(kp) {
		g.lastAnyKeyTime = now
		g.seqLen = 0
		dbg("  GUARD-PASS non-printable: %q", kp.String())
		return msg
	}

	// In a text field a dropped key silently loses what the user typed,
	// while a leaked fragment only shows up as text they can see and
	// delete. Fast typing, dictation and unbracketed paste all look like a
	// burst, so the timing layers stand down there; the content patterns
	// still catch reply fragments (mg-4ko).
	if g.textEntry.Load() {
		charGap := now.Sub(g.lastPrintableTime)
		g.lastPrintableTime = now
		return g.matchContent(msg, kp, now, charGap)
	}

	// Layer 2: timing-based suppression window. During the window
	// drop all printable keys including shift/alt-modified ones.
	if now.Before(g.suppressUntil) {
		g.lastPrintableTime = now
		g.markSuspicious(now, oscGuardWindow)
		dbg("  GUARD-WINDOW suppressed: %q (mod=%d)", kp.String(), kp.Mod)
		return nil
	}

	// Outside the window, modified printable keys pass through normally.
	// Reset the accumulator since this is real user interaction.
	if kp.Mod != 0 {
		g.lastAnyKeyTime = now
		g.seqLen = 0
		return msg
	}

	// Timing-based burst/nav detection (unmodified printable).
	charGap := now.Sub(g.lastPrintableTime)
	g.lastPrintableTime = now

	if charGap > 0 && charGap < 15*time.Millisecond {
		g.markSuspicious(now, oscGuardWindow)
		g.seqLen = 0
		dbg("  GUARD-BURST suppressed: %q (gap=%v)", kp.String(), charGap)
		return nil
	}

	navGap := now.Sub(g.lastAnyKeyTime)
	if navGap > 0 && navGap < 50*time.Millisecond {
		g.markSuspicious(now, oscGuardWindow)
		g.seqLen = 0
		dbg("  GUARD-NAV suppressed: %q (gap from nav=%v)", kp.String(), navGap)
		return nil
	}

	return g.matchContent(msg, kp, now, charGap)
}

// matchContent is layer 3: content-aware pattern detection.
func (g *OSCGuard) matchContent(msg tea.Msg, kp tea.KeyPressMsg, now time.Time, charGap time.Duration) tea.Msg {
	// Track recent printable chars that passed through timing checks.
	// If the accumulated string matches a known control-sequence
	// fragment, suppress the triggering character and open a window.
	//
	// Patterns and what they catch:
	//   ";rg"  - OSC 11 ";rgb:..." (suppresses 'g' before it acts)
	//   "rgb:" - OSC 11 color value (catches ':' which opens palette)
	//   "[?"   - torn CSI private parameter "[?2026..."
	//   "$y"   - DECRPM response terminator
	//   "[2"   - CPR / CSI parameter prefix ([28;135R)
	//   "[A"   - split CSI final byte from ESC[A / ESC[B / ... tails
	//   "]1"   - OSC parameter prefix (]11;...)

	if charGap > 2*time.Second {
		g.seqLen = 0
	}

	if g.seqLen >= len(g.seqBuf) {
		copy(g.seqBuf[:], g.seqBuf[8:])
		g.seqLen = 8
	}
	g.seqBuf[g.seqLen] = byte(kp.Code)
	g.seqLen++

	acc := string(g.seqBuf[:g.seqLen])
	if pat, ok := matchSeqPattern(acc); ok {
		g.markSuspicious(now, oscGuardWindow)
		dbg("  GUARD-PATTERN suppressed: %q (matched %q in %q)", kp.String(), pat, acc)
		g.seqLen = 0
		return nil
	}

	dbg("  GUARD-PASS printable: %q (charGap=%v acc=%q)", kp.String(), charGap, acc)
	return msg
}

// SetTextEntry tells the guard whether a text field has focus. The app
// sets it after every Update.
func (g *OSCGuard) SetTextEntry(on bool) {
	g.textEntry.Store(on)
}

// SuspiciousSince reports whether the guard observed suppressed or parsed
// control-sequence activity after a deferred key was staged.
func (g *OSCGuard) SuspiciousSince(t time.Time) bool {
	return !g.lastSuspiciousAt.IsZero() && (g.lastSuspiciousAt.After(t) || g.lastSuspiciousAt.Equal(t))
}

// NoteAppSuppression lets app-level deferred key logic open the same window the
// filter uses once it identifies a torn fragment pair.
func (g *OSCGuard) NoteAppSuppression() {
	now := g.clock()
	g.markSuspicious(now, oscGuardWindow)
	g.seqLen = 0
}

func (g *OSCGuard) markSuspicious(now time.Time, window time.Duration) {
	g.lastSuspiciousAt = now
	until := now.Add(window)
	if until.After(g.suppressUntil) {
		g.suppressUntil = until
	}
}

func matchSeqPattern(acc string) (string, bool) {
	for _, pat := range seqPatterns {
		if strings.Contains(acc, pat) {
			return pat, true
		}
	}

	if len(acc) >= 2 {
		tail := acc[len(acc)-2:]
		if isLikelySeqPrefix(tail) {
			return tail, true
		}
	}

	return "", false
}

func isLikelySeqPrefix(tail string) bool {
	if len(tail) != 2 {
		return false
	}

	switch tail[0] {
	case '[':
		return isDigitASCII(tail[1]) || isCSIFinalByte(tail[1])
	case ']':
		return isDigitASCII(tail[1])
	}

	return false
}

func isKnownArtifactKey(kp tea.KeyPressMsg) bool {
	switch kp.String() {
	case "alt+\\", "alt+meta+f3":
		return true
	default:
		return false
	}
}

func isPrintableKey(kp tea.KeyPressMsg) bool {
	return kp.Code >= 0x20 && kp.Code <= 0x7E
}

func isReplyTailKey(kp tea.KeyPressMsg) bool {
	if kp.Code == '\\' {
		return true
	}

	if kp.Mod&tea.ModShift == 0 {
		return false
	}

	return isCSIFinalByte(byte(kp.Code))
}

func isCSIFinalByte(b byte) bool {
	switch b {
	case 'A', 'B', 'C', 'D', 'R':
		return true
	default:
		return false
	}
}

func isDigitASCII(b byte) bool {
	return b >= '0' && b <= '9'
}

// seqPatterns are substrings of torn terminal control sequences. When any
// pattern appears in the recent-character accumulator, the triggering
// character is suppressed and a 500ms window opens.
var seqPatterns = []string{
	";rg",  // OSC 11 color prefix (;rgb:...) - catches 'g' early
	"rgb:", // OSC 11 color value - catches ':' if ';' wasn't accumulated
	"[?",   // torn CSI private parameter ([?2026...)
	"$y",   // DECRPM response terminator
}
