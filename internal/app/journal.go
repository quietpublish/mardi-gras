package app

import (
	"os"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/matt-wright86/mardi-gras/internal/components"
	"github.com/matt-wright86/mardi-gras/internal/data"
)

// Live updates from the bd events journal. data.JournalFollower is the state
// machine; this file runs its bd calls and timer and turns its actions into
// reloads. The journal only ever triggers a reload: issue state always comes
// from bd list, through the refresh loop.
//
// It is idle unless the workspace has the journal on (`bd config set
// events-journal true`). mg never turns it on: that edits a git-tracked file
// and journals every writer in the workspace, agents included.

// journalLoop is the app's side of the events journal.
type journalLoop struct {
	follower data.JournalFollower
	gen      uint64 // identifies the one live journal timer
	busy     bool   // a journal bd call is in flight; its result re-arms the timer
	// freshAt is when a probe last found nothing pending, which makes the
	// issues on screen current as of then, whatever the last bd list.
	freshAt time.Time
	// pending maps issue IDs from probed records to when they were probed,
	// until a reload that started after that shows whether bd list returns
	// them. ignore holds the ones it did not (templates, gates and other
	// types bd list hides), so their records stop triggering reloads.
	pending map[string]time.Time
	ignore  map[string]bool
}

type journalEnabledMsg struct {
	on  bool
	err error
}

type journalHeadMsg struct {
	anchor data.JournalAnchor
	err    error
}

type journalProbeMsg struct {
	records   []data.JournalRecord
	err       error
	startedAt time.Time // when the probe read the journal
	// afterMutation marks the probe run right after mg's own write, which
	// must reload whatever the probe finds.
	afterMutation bool
}

type journalTickMsg struct{ gen uint64 }

// journalOptedOut reports MG_EVENTS=off, which keeps mg on plain polling.
func journalOptedOut() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("MG_EVENTS")), "off")
}

// newJournalLoop starts the follower for a bd-backed source. Init launches
// the first check, so the loop starts busy (Init cannot mutate the model).
func newJournalLoop(mode data.SourceMode, optedOut bool) journalLoop {
	if mode != data.SourceCLI {
		return journalLoop{}
	}
	f, act := data.NewJournalFollower(optedOut)
	return journalLoop{follower: f, busy: act == data.JournalCheckEnabled}
}

func checkJournalEnabled() tea.Msg {
	on, err := data.JournalEnabled()
	return journalEnabledMsg{on: on, err: err}
}

func findJournalHead() tea.Msg {
	anchor, err := data.FindJournalHead()
	return journalHeadMsg{anchor: anchor, err: err}
}

func (m Model) probeJournal(afterMutation bool) tea.Cmd {
	anchor := m.journal.follower.Anchor
	return func() tea.Msg {
		startedAt := time.Now()
		records, err := data.ProbeJournal(anchor, data.JournalProbeLimit)
		return journalProbeMsg{records: records, err: err, startedAt: startedAt, afterMutation: afterMutation}
	}
}

// journalCall runs one journal bd call. Only one is ever in flight.
func (m *Model) journalCall(cmd tea.Cmd) tea.Cmd {
	m.journal.busy = true
	return cmd
}

// journalDo carries out a follower action, re-arming the timer when the
// action is not itself a bd call.
func (m *Model) journalDo(act data.JournalAction) tea.Cmd {
	switch act {
	case data.JournalCheckEnabled:
		return m.journalCall(checkJournalEnabled)
	case data.JournalFindHead:
		return m.journalCall(findJournalHead)
	case data.JournalRefresh:
		return tea.Batch(m.journalRefresh(), m.scheduleJournal())
	}
	return m.scheduleJournal()
}

// scheduleJournal arms the journal timer: the next probe while following,
// or the retry while backing off or waiting to re-check a disabled journal.
func (m *Model) scheduleJournal() tea.Cmd {
	f := m.journal.follower
	var d time.Duration
	switch {
	case f.Live():
		d = data.JournalProbeInterval
	case (f.Phase == data.JournalBackoff || f.Phase == data.JournalOff) && !f.RetryAt.IsZero():
		d = max(time.Until(f.RetryAt), 0)
	default:
		return nil // waiting on a bd call, or off for good
	}
	m.journal.gen++
	gen := m.journal.gen
	return tea.Tick(d, func(time.Time) tea.Msg {
		return journalTickMsg{gen: gen}
	})
}

// journalRefresh reloads on the journal's behalf, but no more often than the
// old poll ran: a burst of agent writes costs at most one bd list per
// CLIPollInterval.
func (m *Model) journalRefresh() tea.Cmd {
	return m.refreshWithin(data.CLIPollInterval - time.Since(m.refresh.startedAt))
}

// refreshAfterMutation reloads after mg's own write. While following the
// journal it probes first, so the anchor moves past mg's own record and that
// record does not trigger a second reload.
func (m *Model) refreshAfterMutation() tea.Cmd {
	if !m.journal.follower.Live() || m.journal.busy || !m.usingCLI() {
		return m.requestRefresh()
	}
	return m.journalCall(m.probeJournal(true))
}

func (m Model) handleJournalTick(msg journalTickMsg) (tea.Model, tea.Cmd) {
	if msg.gen != m.journal.gen || m.journal.busy {
		return m, nil
	}
	before := m.journal.follower
	f, act := before.Tick(time.Now())
	m.journal.follower = f
	cmds := m.journalTransition(before)
	switch {
	case act != data.JournalNoAction:
		cmds = append(cmds, m.journalDo(act))
	case f.Live() && !m.sourceHealth.IsDegraded():
		cmds = append(cmds, m.journalCall(m.probeJournal(false)))
	default:
		// Not following, or bd list is failing too: the refresh loop and
		// its health checks own recovery. Keep the timer going.
		cmds = append(cmds, m.scheduleJournal())
	}
	return m, tea.Batch(cmds...)
}

func (m Model) handleJournalEnabled(msg journalEnabledMsg) (tea.Model, tea.Cmd) {
	m.journal.busy = false
	before := m.journal.follower
	f, act := before.Enabled(msg.on, msg.err, time.Now())
	m.journal.follower = f
	cmds := m.journalTransition(before)
	cmds = append(cmds, m.journalDo(act))
	return m, tea.Batch(cmds...)
}

func (m Model) handleJournalHead(msg journalHeadMsg) (tea.Model, tea.Cmd) {
	m.journal.busy = false
	before := m.journal.follower
	f, act := before.HeadFound(msg.anchor, msg.err, time.Now())
	m.journal.follower = f
	cmds := m.journalTransition(before)
	cmds = append(cmds, m.journalDo(act))
	return m, tea.Batch(cmds...)
}

func (m Model) handleJournalProbe(msg journalProbeMsg) (tea.Model, tea.Cmd) {
	m.journal.busy = false
	now := time.Now()
	before := m.journal.follower
	f, act := before.Probed(msg.records, msg.err, msg.startedAt, now)
	m.journal.follower = f
	relevant := m.noteJournalRecords(msg.records, now)

	var cmds []tea.Cmd
	if msg.afterMutation {
		// mg's own write reloads now, whatever the probe found.
		cmds = append(cmds, m.requestRefresh())
		if act == data.JournalRefresh {
			act = data.JournalNoAction
		}
	} else if act == data.JournalRefresh && !relevant {
		act = data.JournalNoAction
	}
	if msg.err == nil && act == data.JournalNoAction && len(m.journal.pending) == 0 &&
		!m.refresh.inFlight && !m.refresh.dirty {
		m.journal.freshAt = now
	}
	cmds = append(cmds, m.journalTransition(before)...)
	cmds = append(cmds, m.journalDo(act))
	return m, tea.Batch(cmds...)
}

// noteJournalRecords marks the issues probed records name as pending until a
// reload shows them, and reports whether any could change the parade: wisps
// and IDs bd list is known not to return cannot.
func (m *Model) noteJournalRecords(records []data.JournalRecord, now time.Time) bool {
	if len(records) == 0 {
		return false
	}
	onScreen := make(map[string]bool, len(m.issues))
	for i := range m.issues {
		onScreen[m.issues[i].ID] = true
	}
	relevant := false
	for _, r := range records {
		if r.Ephemeral() || (!onScreen[r.IssueID] && m.journal.ignore[r.IssueID]) {
			continue
		}
		relevant = true
		if m.journal.pending == nil {
			m.journal.pending = make(map[string]time.Time)
		}
		m.journal.pending[r.IssueID] = now
	}
	return relevant
}

// journalReloaded tells the journal side about a completed reload: which
// pending IDs bd list turned out not to return, and which issues changed,
// so the follower can spot writes the journal never recorded.
func (m *Model) journalReloaded(issues []data.Issue, touched []string) {
	if !m.journal.follower.Live() {
		return
	}
	startedAt := m.refresh.startedAt
	if len(m.journal.pending) > 0 {
		present := make(map[string]bool, len(issues))
		for i := range issues {
			present[issues[i].ID] = true
		}
		for id, at := range m.journal.pending {
			if at.After(startedAt) {
				continue // this reload may predate the write; wait for the next
			}
			if !present[id] {
				if m.journal.ignore == nil {
					m.journal.ignore = make(map[string]bool)
				}
				m.journal.ignore[id] = true
			}
			delete(m.journal.pending, id)
		}
	}
	m.journal.follower = m.journal.follower.Reloaded(touched, startedAt, time.Now())
}

// journalTransition reacts to the follower changing state: the backstop
// rate follows whether the journal is live and complete, and losing a live
// journal to repeated failures is worth one toast.
func (m *Model) journalTransition(before data.JournalFollower) []tea.Cmd {
	after := m.journal.follower
	var cmds []tea.Cmd
	if (before.Live() != after.Live() || before.Partial != after.Partial) && !m.refresh.inFlight {
		cmds = append(cmds, m.scheduleRefresh())
	}
	if before.Live() && after.Phase == data.JournalBackoff {
		toast, cmd := components.ShowToast(
			"Live updates paused (bd events journal unreachable); polling every 5s",
			components.ToastWarn, toastDuration,
		)
		m.toast = toast
		cmds = append(cmds, cmd)
	}
	return cmds
}

// dataFreshAt is when the issues on screen were last known current: the
// last bd list, or while the journal is live and complete, the last probe
// that found nothing pending.
func (m Model) dataFreshAt() time.Time {
	if f := m.journal.follower; f.Live() && !f.Partial && m.journal.freshAt.After(m.lastFileMod) {
		return m.journal.freshAt
	}
	return m.lastFileMod
}
