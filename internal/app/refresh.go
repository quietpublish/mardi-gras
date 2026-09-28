package app

import (
	"fmt"
	"slices"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/matt-wright86/mardi-gras/internal/components"
	"github.com/matt-wright86/mardi-gras/internal/data"
)

// refreshLoop is the single owner of issue reloads (bd list, or a JSONL
// re-stat). Exactly one loop runs per session: one armed timer, and at most
// one fetch in flight.
//
// It replaces a design where every post-mutation refresh started a fresh
// poll chain that re-armed itself forever, alongside the chain already
// running, so each claim, close or comment permanently added another
// `bd list` every 5s.
type refreshLoop struct {
	// tickGen identifies the one live timer. Arming a new timer, or starting
	// an immediate fetch, bumps it so an older timer fires into nothing.
	tickGen uint64
	// fetchGen identifies the source a fetch was started against. It is
	// bumped when the source switches, so a result from the old source is
	// dropped rather than applied over the new one.
	fetchGen uint64
	inFlight bool
	// dirty records a refresh request that arrived while a fetch was in
	// flight. It is honoured by exactly one follow-up fetch, so a burst of
	// mutations coalesces instead of queueing.
	dirty bool
	// spaced records a reload a trigger (the journal, the orchestrator's
	// event stream) asked for while a fetch was in flight. refreshDone
	// honours it no sooner than spacedGap after that fetch lands, unlike
	// dirty, so a stream of writes never reloads back to back.
	spaced    bool
	spacedGap time.Duration
	// startedAt is when the latest fetch started, landedAt when the latest
	// one finished, and dueAt when the armed timer fires (zero for the first
	// timer, which Init arms).
	startedAt time.Time
	landedAt  time.Time
	dueAt     time.Time
}

// journalBackstopInterval is how often bd list still runs while the events
// journal drives reloads: a safety net for writes the journal cannot see.
const journalBackstopInterval = 30 * time.Second

// refreshTickMsg is the refresh timer firing.
type refreshTickMsg struct{ gen uint64 }

// refreshResultMsg wraps a fetch result (data.FileChangedMsg,
// data.FileUnchangedMsg or data.FileWatchErrorMsg) with the source
// generation it was fetched against.
type refreshResultMsg struct {
	gen uint64
	msg tea.Msg
}

// usingCLI reports whether reloads currently go through bd list.
func (m Model) usingCLI() bool {
	return !m.sourceHealth.InFallback() && m.sourceMode == data.SourceCLI
}

// refreshInterval is the timer's period. While the events journal drives
// reloads, bd list runs only as a backstop, unless the journal has been seen
// missing writes, or bd list itself is failing and needs retrying at the
// usual pace.
func (m Model) refreshInterval() time.Duration {
	if !m.usingCLI() {
		return data.WatchInterval
	}
	if f := m.journal.follower; f.Live() && !f.Partial &&
		m.sourceHealth.ConsecFailures == 0 && !m.sourceHealth.IsDegraded() {
		return journalBackstopInterval
	}
	return data.CLIPollInterval
}

// refreshTimer arms the timer for gen without touching the model, so Init
// (a value receiver) can arm the first one. It returns nil when there is
// nothing to reload from (a JSONL source with no file), as the old watcher
// did.
func (m Model) refreshTimer(gen uint64) tea.Cmd {
	if !m.usingCLI() && m.watchPath == "" {
		return nil
	}
	return tea.Tick(m.refreshInterval(), func(time.Time) tea.Msg {
		return refreshTickMsg{gen: gen}
	})
}

// scheduleRefresh arms the refresh timer, retiring any timer already armed.
func (m *Model) scheduleRefresh() tea.Cmd {
	m.refresh.tickGen++
	m.refresh.dueAt = time.Now().Add(m.refreshInterval())
	return m.refreshTimer(m.refresh.tickGen)
}

// refreshSpaced reloads for the journal, but no more often than the old
// poll ran: CLIPollInterval after the last reload landed.
func (m *Model) refreshSpaced() tea.Cmd {
	return m.refreshSpacedBy(data.CLIPollInterval)
}

// refreshSpacedBy reloads for a trigger no sooner than gap after the last
// reload landed, pulling the timer in (never pushing it out). While a fetch
// is in flight it only notes the request, and refreshDone honours it on the
// same terms.
func (m *Model) refreshSpacedBy(gap time.Duration) tea.Cmd {
	if m.refresh.inFlight {
		if !m.refresh.spaced || gap < m.refresh.spacedGap {
			m.refresh.spacedGap = gap
		}
		m.refresh.spaced = true
		return nil
	}
	return m.refreshBy(gap - time.Since(m.refresh.landedAt))
}

// refreshBy makes the next reload start no later than d from now:
// immediately when d has passed, otherwise by pulling the timer in. It never
// postpones a timer that is already due sooner. Call it only with no fetch
// in flight.
func (m *Model) refreshBy(d time.Duration) tea.Cmd {
	if d <= 0 {
		return m.requestRefresh()
	}
	due := time.Now().Add(d)
	if !m.refresh.dueAt.IsZero() && !m.refresh.dueAt.After(due) {
		return nil
	}
	m.refresh.tickGen++
	m.refresh.dueAt = due
	gen := m.refresh.tickGen
	return tea.Tick(d, func(time.Time) tea.Msg {
		return refreshTickMsg{gen: gen}
	})
}

// requestRefresh reloads issues now, or right after the fetch in flight
// lands. Mutation handlers call it instead of starting a poll of their own.
func (m *Model) requestRefresh() tea.Cmd {
	if m.refresh.inFlight {
		m.refresh.dirty = true
		return nil
	}
	m.refresh.tickGen++ // the fetch result re-arms the timer
	return m.startFetch()
}

// startFetch runs one reload against the current source.
func (m *Model) startFetch() tea.Cmd {
	var fetch tea.Cmd
	if m.usingCLI() {
		fetch = data.FetchIssuesNow(m.projectDir)
	} else {
		fetch = data.CheckFile(m.watchPath, m.lastFileMod)
	}
	if fetch == nil {
		// No file to watch; the loop stops, as the old watcher did.
		return nil
	}
	m.refresh.inFlight = true
	m.refresh.startedAt = time.Now()
	gen := m.refresh.fetchGen
	return func() tea.Msg {
		return refreshResultMsg{gen: gen, msg: fetch()}
	}
}

// refreshDone ends the fetch that just landed and keeps the loop going:
// straight into another fetch if a refresh was requested meanwhile, otherwise
// back to the timer. Call it after the model has absorbed the result, so the
// next fetch sees the updated lastFileMod.
func (m *Model) refreshDone() tea.Cmd {
	m.refresh.inFlight = false
	m.refresh.landedAt = time.Now()
	if m.refresh.dirty {
		m.refresh.dirty = false
		m.refresh.spaced = false
		return m.requestRefresh()
	}
	timer := m.scheduleRefresh()
	if m.refresh.spaced {
		m.refresh.spaced = false
		if sooner := m.refreshBy(m.refresh.spacedGap); sooner != nil {
			return sooner
		}
	}
	return timer
}

// restartRefresh abandons any fetch in flight, since its result belongs to
// the source mg just switched away from, and re-arms the timer for the new
// one.
func (m *Model) restartRefresh() tea.Cmd {
	m.refresh.fetchGen++
	m.refresh.inFlight = false
	m.refresh.dirty = false
	m.refresh.spaced = false
	return m.scheduleRefresh()
}

// handleRefreshTick starts the timed fetch unless a newer timer or an
// immediate fetch has superseded this tick.
func (m Model) handleRefreshTick(msg refreshTickMsg) (tea.Model, tea.Cmd) {
	if msg.gen != m.refresh.tickGen || m.refresh.inFlight {
		return m, nil
	}
	fetch := m.startFetch()
	return m, fetch
}

// handleRefreshResult unwraps a fetch result, dropping it when it was fetched
// from a source mg has since switched away from.
func (m Model) handleRefreshResult(msg refreshResultMsg) (tea.Model, tea.Cmd) {
	if msg.gen != m.refresh.fetchGen {
		return m, nil
	}
	return m.Update(msg.msg)
}

// applyIssues installs a freshly loaded issue set: change indicators, the
// parade rebuild, selection, and the detail refetches that depend on them.
// It returns the Cmds the update produced and the IDs of every issue that
// was added, changed or removed; continuing the refresh loop is the caller's
// job.
func (m *Model) applyIssues(msg data.FileChangedMsg) (cmds []tea.Cmd, touched []string) {

	// Warn if malformed lines were skipped
	if msg.Skipped > 0 {
		toast, toastCmd := components.ShowToast(
			fmt.Sprintf("Skipped %d malformed line(s)", msg.Skipped),
			components.ToastWarn, toastDuration,
		)
		m.toast = toast
		cmds = append(cmds, toastCmd)
	}

	// Diff against the issues on screen for change indicators
	changed, removed := m.diffIssues(msg.Issues)
	if changes := len(changed) + len(removed); changes > 0 {
		// The ◈ marks carry the news. Only toast when nothing else is
		// showing: the reload right after mg's own write would otherwise
		// replace that write's confirmation or error almost at once.
		if !m.toast.Active() {
			toast, toastCmd := components.ShowToast(
				fmt.Sprintf("%d issue%s changed", changes, plural(changes)),
				components.ToastInfo, toastDuration,
			)
			m.toast = toast
			cmds = append(cmds, toastCmd)
		}
		cmds = append(cmds, tea.Tick(changeIndicatorDuration, func(time.Time) tea.Msg {
			return changeIndicatorExpiredMsg{}
		}))
	}
	m.dropStaleDetail(changed, msg.Issues)

	m.issues = msg.Issues
	m.groups = data.GroupByParade(msg.Issues, m.blockingTypes)
	if !msg.LastMod.IsZero() {
		m.lastFileMod = msg.LastMod
	}
	m.rebuildParade()
	if m.selectionLost {
		lostID := m.lostIssueID
		m.selectionLost = false
		m.lostIssueID = ""
		toast, toastCmd := components.ShowToast(
			fmt.Sprintf("Issue %s removed — moved to next", lostID),
			components.ToastInfo, toastDuration,
		)
		m.toast = toast
		cmds = append(cmds, toastCmd)
	}
	m.recomputeVelocity()
	cmds = append(cmds, m.detailFetchBatch()...)
	return cmds, append(changed, removed...)
}

// handleIssuesLoaded absorbs a successful reload and continues the refresh loop.
func (m Model) handleIssuesLoaded(msg data.FileChangedMsg) (tea.Model, tea.Cmd) {
	m.sourceHealth = m.sourceHealth.RecordSuccess()
	cmds, touched := m.applyIssues(msg)
	m.journalReloaded(msg.Issues, touched)
	cmds = append(cmds, m.refreshDone(), m.gatedPollAgentState())
	return m, tea.Batch(cmds...)
}

// handleIssuesUnchanged continues the refresh loop after a JSONL check found no change.
func (m Model) handleIssuesUnchanged(msg data.FileUnchangedMsg) (tea.Model, tea.Cmd) {
	if !msg.LastMod.IsZero() {
		m.lastFileMod = msg.LastMod
	}
	refresh := m.refreshDone()
	return m, tea.Batch(refresh, m.gatedPollAgentState())
}

// handleLoadError records a failed reload, falls back to JSONL when bd has
// been failing long enough, and continues the refresh loop.
func (m Model) handleLoadError(msg data.FileWatchErrorMsg) (tea.Model, tea.Cmd) {
	m.sourceHealth = m.sourceHealth.RecordFailure(msg.Err)
	cmds := []tea.Cmd{m.gatedPollAgentState()}

	// Toast suppression: only show on the first failure.
	if m.sourceHealth.ShouldShowToast() {
		label := fmt.Sprintf("Load failed: %s", msg.Err)
		if m.sourceMode == data.SourceCLI {
			label = fmt.Sprintf("bd list failed: %s", msg.Err)
		}
		toast, toastCmd := components.ShowToast(label, components.ToastError, toastDuration)
		m.toast = toast
		cmds = append(cmds, toastCmd)
	}

	// On entering degraded: probe for a fresh JSONL fallback file.
	if m.sourceHealth.State == data.HealthDegraded && m.sourceHealth.ConsecFailures == data.DegradeThreshold {
		if path, _, ok := data.ProbeJSONLFallback(m.projectDir); ok {
			m.sourceHealth.State = data.HealthFallback
			m.jsonlPath = path
			m.sourceMode = data.SourceJSONL
			m.watchPath = path
			if !m.healthChecking {
				m.healthChecking = true
				cmds = append(cmds, data.CLIHealthCheck(m.projectDir))
			}
			toast, toastCmd := components.ShowToast(
				"Switched to issues.jsonl fallback (bd unavailable)",
				components.ToastWarn, toastDuration,
			)
			m.toast = toast
			cmds = append(cmds, toastCmd)
		}
		// If JSONL probe fails: stay degraded with last-good data.
	}
	// Continue the loop after any switch above, so the next fetch
	// targets the source now in effect.
	cmds = append(cmds, m.refreshDone())
	return m, tea.Batch(cmds...)
}

// handleHealthCheck handles a bd probe made while in JSONL fallback,
// switching back to the CLI once bd has recovered.
func (m Model) handleHealthCheck(msg data.CLIHealthCheckMsg) (tea.Model, tea.Cmd) {
	if msg.Err != nil {
		m.sourceHealth = m.sourceHealth.RecordFailure(msg.Err)
		// Keep probing until CLI recovers.
		return m, data.CLIHealthCheck(m.projectDir)
	}
	m.sourceHealth = m.sourceHealth.RecordSuccess()
	if m.sourceHealth.State == data.HealthHealthy {
		// Recovery complete: switch back to CLI. Restarting the loop
		// drops any JSONL fetch still in flight and retires its timer.
		m.sourceMode = data.SourceCLI
		m.watchPath = ""
		m.healthChecking = false
		cmds, _ := m.applyIssues(data.FileChangedMsg{Issues: msg.Issues, LastMod: time.Now()})
		toast, toastCmd := components.ShowToast(
			"bd recovered \u2014 switched back to CLI",
			components.ToastSuccess, toastDuration,
		)
		m.toast = toast
		cmds = append(cmds, m.restartRefresh(), m.gatedPollAgentState(), toastCmd)
		return m, tea.Batch(cmds...)
	}
	// Still recovering (1 success counted); keep probing.
	return m, data.CLIHealthCheck(m.projectDir)
}

// dropStaleDetail clears the detail panel's caches for the selected issue
// when a reload shows it changed, so detailFetchBatch refetches them: rich
// detail on any change, comments when the comment count moved. It must run
// before the parade rebuild, while SelectedIssue still holds the old version.
func (m *Model) dropStaleDetail(changed []string, next []data.Issue) {
	sel := m.parade.SelectedIssue
	if sel == nil || !slices.Contains(changed, sel.ID) {
		return
	}
	if m.detail.RichIssueID == sel.ID {
		m.detail.RichIssueID = ""
	}
	for i := range next {
		if next[i].ID == sel.ID && next[i].CommentCount != sel.CommentCount && m.detail.CommentsIssueID == sel.ID {
			m.detail.CommentsIssueID = ""
		}
	}
}
