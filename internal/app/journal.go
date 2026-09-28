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
	// own maps issues mg itself just wrote to when. The reload that write
	// started already covers them, so their records must not ask for a
	// second one.
	own map[string]time.Time
	// recent is the Recent changes feed: the newest records, oldest first,
	// wisps left out.
	recent []data.JournalRecord
	// serveURL is MG_BD_SERVE: a bd serve whose event stream replaces the
	// CLI probes while it's up. serve is that stream's state.
	serveURL string
	serve    serveStream
}

// journalRecentCap bounds the Recent changes feed.
const journalRecentCap = 200

// journalBackfill is how many records before the head the feed loads when the
// journal goes live, so it isn't empty at startup.
const journalBackfill = 100

type journalBackfillMsg struct {
	records []data.JournalRecord
}

// addRecent appends records newer than the feed's newest, leaving out wisps.
func (j *journalLoop) addRecent(records []data.JournalRecord) {
	var newest int64
	if n := len(j.recent); n > 0 {
		newest = j.recent[n-1].Seq
	}
	for _, r := range records {
		if r.Seq > newest && !r.Ephemeral() {
			j.recent = append(j.recent, r)
			newest = r.Seq
		}
	}
	if extra := len(j.recent) - journalRecentCap; extra > 0 {
		j.recent = append(j.recent[:0:0], j.recent[extra:]...)
	}
}

// addBackfill puts records older than the feed's oldest in front of it.
func (j *journalLoop) addBackfill(records []data.JournalRecord) {
	oldest := int64(1<<63 - 1)
	if len(j.recent) > 0 {
		oldest = j.recent[0].Seq
	}
	var older []data.JournalRecord
	for _, r := range records {
		if r.Seq < oldest && !r.Ephemeral() {
			older = append(older, r)
		}
	}
	j.recent = append(older, j.recent...)
	if extra := len(j.recent) - journalRecentCap; extra > 0 {
		j.recent = j.recent[extra:]
	}
}

// recentFor returns the feed's records for one issue, oldest first.
func (j journalLoop) recentFor(id string) []data.JournalRecord {
	var out []data.JournalRecord
	for _, r := range j.recent {
		if r.IssueID == id {
			out = append(out, r)
		}
	}
	return out
}

func backfillJournal(head int64) tea.Cmd {
	return func() tea.Msg {
		records, err := data.TailRecent(head, journalBackfill)
		if err != nil {
			return journalBackfillMsg{} // unreachable history just stays empty
		}
		return journalBackfillMsg{records: records}
	}
}

func (m Model) handleJournalBackfill(msg journalBackfillMsg) (tea.Model, tea.Cmd) {
	m.journal.addBackfill(msg.records)
	m.syncRecentChanges()
	return m, nil
}

// syncRecentChanges pushes the feed to the Recent changes overlay and the
// selected issue's ACTIVITY section.
func (m *Model) syncRecentChanges() {
	m.changes.SetRecords(m.journal.recent, m.detail.IssueMap, m.journalNote())
	if sel := m.parade.SelectedIssue; sel != nil && m.detail.Issue != nil && m.detail.Issue.ID == sel.ID {
		m.detail.RecentChanges = m.journal.recentFor(sel.ID)
		m.detail.SetIssue(m.detail.Issue)
	}
}

// journalNote explains, for the Recent changes overlay, why the journal isn't
// feeding it; "" when it is.
func (m Model) journalNote() string {
	f := m.journal.follower
	switch {
	case f.Live():
		return ""
	case m.sourceMode != data.SourceCLI:
		return "Recent changes come from the bd events journal, and mg is reading issues.jsonl directly, without bd."
	case journalOptedOut():
		return "Recent changes come from the bd events journal, which MG_EVENTS=off turns off."
	case m.orchestratorAvailable():
		return "Recent changes come from the bd events journal, which mg doesn't use under Gas Town or Gas City: orchestrator writes can bypass it."
	case f.Phase == data.JournalBackoff:
		return "The bd events journal isn't answering; mg will try again shortly."
	case f.Phase == data.JournalDetecting || f.Phase == data.JournalBaselining:
		return "Connecting to the bd events journal..."
	}
	return "Recent changes come from the bd events journal, which is off for this workspace, or needs bd 1.2.1 or newer. To turn it on: bd config set events-journal true. That edits .beads/config.yaml and journals every writer in the workspace; see the README's Live updates section."
}

// journalOwnWindow is how long mg waits for its own write's record before
// forgetting it (a write that was never journaled).
const journalOwnWindow = 30 * time.Second

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
}

type journalTickMsg struct{ gen uint64 }

// journalOptedOut reports MG_EVENTS=off, which keeps mg on plain polling.
func journalOptedOut() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv("MG_EVENTS")), "off")
}

// newJournalLoop starts the follower for a bd-backed source. Init launches
// the first check, so the loop starts busy (Init cannot mutate the model).
// Under an orchestrator the caller opts out: orchestrator writes can bypass
// the journal, so mg keeps the 5s poll there, and probing on top of it would
// only add bd processes.
func newJournalLoop(mode data.SourceMode, optedOut bool) journalLoop {
	if mode != data.SourceCLI {
		return journalLoop{}
	}
	f, act := data.NewJournalFollower(optedOut)
	j := journalLoop{follower: f, busy: act == data.JournalCheckEnabled}
	if !optedOut {
		j.serveURL = strings.TrimSpace(os.Getenv("MG_BD_SERVE"))
	}
	return j
}

func checkJournalEnabled() tea.Msg {
	on, err := data.JournalEnabled()
	return journalEnabledMsg{on: on, err: err}
}

func findJournalHead() tea.Msg {
	anchor, err := data.FindJournalHead()
	return journalHeadMsg{anchor: anchor, err: err}
}

func (m Model) probeJournal() tea.Cmd {
	anchor := m.journal.follower.Anchor
	return func() tea.Msg {
		startedAt := time.Now()
		records, err := data.ProbeJournal(anchor, data.JournalProbeLimit)
		return journalProbeMsg{records: records, err: err, startedAt: startedAt}
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
		return tea.Batch(m.refreshSpaced(), m.scheduleJournal())
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
		d = f.ProbeInterval(time.Now())
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

// refreshAfterMutation reloads right away after mg's own write to ownID
// (empty when the write has no single issue). While following the journal
// it remembers ownID, so the write's own record, which that reload already
// covers, does not ask for a second one, and probes now to move the anchor
// past it.
func (m *Model) refreshAfterMutation(ownID string) tea.Cmd {
	refresh := m.requestRefresh()
	if !m.journal.follower.Live() || !m.usingCLI() {
		return refresh
	}
	if ownID != "" {
		if m.journal.own == nil {
			m.journal.own = make(map[string]time.Time)
		}
		m.journal.own[ownID] = time.Now()
	}
	if m.journal.busy {
		return refresh
	}
	return tea.Batch(refresh, m.journalCall(m.probeJournal()))
}

func (m Model) handleJournalTick(msg journalTickMsg) (tea.Model, tea.Cmd) {
	if msg.gen != m.journal.gen || m.journal.busy {
		return m, nil
	}
	before := m.journal.follower
	f, act := before.Tick(time.Now())
	m.journal.follower = f
	cmds := m.journalTransition(before)
	if cmd := m.maybeStartServe(); cmd != nil {
		cmds = append(cmds, cmd)
	}
	switch {
	case act != data.JournalNoAction:
		cmds = append(cmds, m.journalDo(act))
	case f.Live() && m.journal.serve.connected:
		// The bd serve stream is feeding the follower: no CLI probe.
		cmds = append(cmds, m.scheduleJournal())
	case f.Live() && !m.sourceHealth.IsDegraded():
		cmds = append(cmds, m.journalCall(m.probeJournal()))
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
	if f.Live() && len(m.journal.recent) == 0 && msg.anchor.Seq > 0 {
		cmds = append(cmds, backfillJournal(msg.anchor.Seq))
	}
	if cmd := m.maybeStartServe(); cmd != nil {
		cmds = append(cmds, cmd)
	}
	m.syncRecentChanges()
	return m, tea.Batch(cmds...)
}

func (m Model) handleJournalProbe(msg journalProbeMsg) (tea.Model, tea.Cmd) {
	m.journal.busy = false
	return m, tea.Batch(m.absorbJournal(msg.records, msg.err, msg.startedAt)...)
}

// absorbJournal feeds records (or a read error) to the follower, whichever
// path read them: a CLI probe or the bd serve stream. Records at or below
// the anchor are dropped: when both paths have run, one may deliver what the
// other already has, and replaying it would move the anchor backwards.
func (m *Model) absorbJournal(records []data.JournalRecord, err error, startedAt time.Time) []tea.Cmd {
	now := time.Now()
	before := m.journal.follower
	records = recordsAfter(records, before.Anchor.Seq)
	f, act := before.Probed(records, err, startedAt, now)
	m.journal.follower = f
	relevant := m.noteJournalRecords(records, now)
	if len(records) > 0 && err == nil {
		m.journal.addRecent(records)
		m.syncRecentChanges()
	}

	var cmds []tea.Cmd
	if act == data.JournalRefresh && !relevant {
		act = data.JournalNoAction
	}
	if err == nil && act == data.JournalNoAction && len(m.journal.pending) == 0 &&
		!m.refresh.inFlight && !m.refresh.dirty {
		m.journal.freshAt = now
	}
	cmds = append(cmds, m.journalTransition(before)...)
	cmds = append(cmds, m.journalDo(act))
	return cmds
}

// recordsAfter drops records at or below seq.
func recordsAfter(records []data.JournalRecord, seq int64) []data.JournalRecord {
	for i, r := range records {
		if r.Seq > seq {
			return records[i:]
		}
	}
	return nil
}

// noteJournalRecords marks the issues probed records name as pending until a
// reload shows them, and reports whether any could change the parade. Wisps
// cannot (their IDs join ignore, so a wisp's delete record, whose issue is
// null, is skipped too), nor can IDs bd list is known not to return, nor mg's
// own recent writes, whose reload is already under way.
func (m *Model) noteJournalRecords(records []data.JournalRecord, now time.Time) bool {
	if len(records) == 0 {
		return false
	}
	covered := make(map[string]bool)
	for id, at := range m.journal.own {
		if now.Sub(at) < journalOwnWindow {
			covered[id] = true
		} else {
			delete(m.journal.own, id)
		}
	}
	relevant := false
	for _, r := range records {
		if r.Ephemeral() {
			if m.journal.ignore == nil {
				m.journal.ignore = make(map[string]bool)
			}
			m.journal.ignore[r.IssueID] = true
			continue
		}
		if covered[r.IssueID] {
			delete(m.journal.own, r.IssueID)
			continue
		}
		if m.detail.IssueMap[r.IssueID] == nil && m.journal.ignore[r.IssueID] {
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
	if !m.journal.follower.Live() || !m.usingCLI() {
		// A JSONL fallback reload differs from bd list for reasons that
		// have nothing to do with the journal.
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
	if before.Live() && !after.Live() {
		m.stopServe()
	}
	if (before.Live() != after.Live() || before.Partial != after.Partial) && !m.refresh.inFlight {
		// Bring the backstop in to the new interval, but never push out a
		// timer already due sooner (one refreshSpaced pulled in).
		cmds = append(cmds, m.refreshBy(m.refreshInterval()))
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
