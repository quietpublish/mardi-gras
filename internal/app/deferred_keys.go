package app

import (
	"time"

	tea "charm.land/bubbletea/v2"
)

// deferredKeyDelay is how long printable keys are held before delivery,
// allowing the OSC guard to detect fragment companions. The OSC 11 reply
// head ("]") arrives ~90ms after the triggering key; the burst follows
// within 2ms. 30ms is enough for fragment detection without adding
// perceptible latency to navigation.
const deferredKeyDelay = 30 * time.Millisecond

type deferredKeyMsg struct {
	id uint64
}

type pendingDeferredKey struct {
	key      tea.KeyPressMsg
	id       uint64
	stagedAt time.Time
}

func deferredKeyCmd(id uint64) tea.Cmd {
	return func() tea.Msg {
		time.Sleep(deferredKeyDelay)
		return deferredKeyMsg{id: id}
	}
}

func (m Model) handleKeyPress(msg tea.KeyPressMsg, allowDeferredBuffer bool) (tea.Model, tea.Cmd) {
	// BubbleTea v2's terminal capability negotiation (DECRPM, Kitty
	// keyboard, etc.) can produce reply traffic that arrives fragmented.
	// Suppress all keys during the startup window to avoid phantom
	// keypresses from torn control-sequence tails.
	if time.Since(m.startedAt) < 500*time.Millisecond {
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		dbg("  SUPPRESSED startup key: %q (age=%v)", msg.String(), time.Since(m.startedAt))
		return m, nil
	}

	if msg.String() == "ctrl+c" {
		logRoute("global ctrl+c -> quit")
		return m, tea.Quit
	}

	if m.showHelp {
		logRoute("handleHelpKey")
		return m.handleHelpKey(msg)
	}

	if m.filtering {
		logRoute("handleFilteringKey")
		return m.handleFilteringKey(msg)
	}

	if allowDeferredBuffer && m.shouldDeferKey(msg) {
		dbg("  DEFER staging key: %q (pendingCount=%d)", msg.String(), len(m.pendingKeys))
		return m.handleDeferredKeyPress(msg)
	}

	logRoute("handleKey")
	logState(m)
	return m.handleKey(msg)
}

// inTextEntry reports whether keystrokes are going into a text field rather
// than acting as shortcuts. There a dropped key loses what the user typed,
// so the deferred buffer and the guard's timing layers stand down (mg-4ko).
func (m Model) inTextEntry() bool {
	return m.filtering || m.creating || m.editing || m.qaMode != "" ||
		m.nudging || m.slingTargeting || m.convoyCreating ||
		m.mailReplying || m.mailComposing || m.codexReplying || m.showPalette
}

func (m Model) shouldDeferKey(msg tea.KeyPressMsg) bool {
	if m.oscGuard == nil || m.inTextEntry() {
		return false
	}
	if msg.Mod&tea.ModCtrl != 0 {
		return false
	}
	return isPrintableKey(msg)
}

func (m Model) handleDeferredKeyPress(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	now := time.Now()

	if len(m.pendingKeys) == 0 {
		return m.stageDeferredKey(msg, now)
	}

	pending := m.pendingKeys[0]

	if m.oscGuard != nil && (m.oscGuard.SuspiciousSince(pending.stagedAt) || isLikelyDeferredFragmentPair(pending.key, msg)) {
		dbg("  DEFER-PAIR dropped: pending=%q + new=%q (suspicious=%v fragmentPair=%v)",
			pending.key.String(), msg.String(),
			m.oscGuard.SuspiciousSince(pending.stagedAt),
			isLikelyDeferredFragmentPair(pending.key, msg))
		m.clearPendingKeys()
		m.oscGuard.NoteAppSuppression()
		return m, nil
	}

	// Quick-action digits are dangerous if they leak, so keep them queued
	// until their own timer instead of flushing them on the next printable.
	if shouldKeepPendingKeyQueued(pending.key) {
		return m.stageDeferredKey(msg, now)
	}

	m.pendingKeys = m.pendingKeys[1:]

	nextModel, cmd1 := m.handleKeyPress(pending.key, false)
	next := nextModel.(Model)
	finalModel, cmd2 := next.handleKeyPress(msg, true)
	return finalModel, batchCmds(cmd1, cmd2)
}

func (m Model) resolveDeferredKey(msg deferredKeyMsg) (Model, tea.KeyPressMsg, bool) {
	idx := m.findPendingKeyIndex(msg.id)
	if idx < 0 {
		dbg("  DEFER-RESOLVE id=%d: not found (already consumed)", msg.id)
		return m, tea.KeyPressMsg{}, false
	}

	pending := m.pendingKeys[idx]
	age := time.Since(pending.stagedAt)
	m.pendingKeys = append(m.pendingKeys[:idx], m.pendingKeys[idx+1:]...)

	if m.oscGuard != nil && m.oscGuard.SuspiciousSince(pending.stagedAt) {
		dbg("  DEFER-RESOLVE id=%d key=%q: DROPPED by guard (age=%v)", msg.id, pending.key.String(), age)
		return m, tea.KeyPressMsg{}, false
	}

	dbg("  DEFER-RESOLVE id=%d key=%q: DELIVERED (age=%v)", msg.id, pending.key.String(), age)
	return m, pending.key, true
}

func (m Model) stageDeferredKey(msg tea.KeyPressMsg, stagedAt time.Time) (Model, tea.Cmd) {
	m.pendingKeyID++
	m.pendingKeys = append(m.pendingKeys, pendingDeferredKey{
		key:      msg,
		id:       m.pendingKeyID,
		stagedAt: stagedAt,
	})
	return m, deferredKeyCmd(m.pendingKeyID)
}

func (m Model) findPendingKeyIndex(id uint64) int {
	for i, pending := range m.pendingKeys {
		if pending.id == id {
			return i
		}
	}
	return -1
}

func (m *Model) clearPendingKeys() {
	m.pendingKeys = nil
}

func shouldKeepPendingKeyQueued(msg tea.KeyPressMsg) bool {
	if msg.Mod != 0 {
		return false
	}

	switch msg.Code {
	case '1', '2', '3':
		return true
	default:
		return false
	}
}

func isLikelyDeferredFragmentPair(first, second tea.KeyPressMsg) bool {
	if !isPrintableKey(first) || !isPrintableKey(second) {
		return false
	}

	if first.Mod&tea.ModCtrl != 0 || second.Mod&tea.ModCtrl != 0 {
		return false
	}

	switch {
	case first.Code == '[':
		return isDigitASCII(byte(second.Code)) || isLikelyShiftCSITail(second)
	case first.Code == ']':
		return isDigitASCII(byte(second.Code))
	case isDigitRune(first.Code):
		return second.Code == ';'
	case first.Code == ';':
		return second.Code == 'r' || second.Code == 'g'
	default:
		return false
	}
}

func isLikelyShiftCSITail(msg tea.KeyPressMsg) bool {
	return msg.Mod&tea.ModShift != 0 && isCSIFinalByte(byte(msg.Code))
}

func isDigitRune(r rune) bool {
	return r >= '0' && r <= '9'
}

func batchCmds(cmds ...tea.Cmd) tea.Cmd {
	filtered := make([]tea.Cmd, 0, len(cmds))
	for _, cmd := range cmds {
		if cmd != nil {
			filtered = append(filtered, cmd)
		}
	}

	switch len(filtered) {
	case 0:
		return nil
	case 1:
		return filtered[0]
	default:
		return tea.Batch(filtered...)
	}
}
