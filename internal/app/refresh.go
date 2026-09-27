package app

import (
	"fmt"
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
}

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

func (m Model) refreshInterval() time.Duration {
	if m.usingCLI() {
		return data.CLIPollInterval
	}
	return data.WatchInterval
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
	return m.refreshTimer(m.refresh.tickGen)
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
	if m.refresh.dirty {
		m.refresh.dirty = false
		return m.requestRefresh()
	}
	return m.scheduleRefresh()
}

// restartRefresh abandons any fetch in flight, since its result belongs to
// the source mg just switched away from, and re-arms the timer for the new
// one.
func (m *Model) restartRefresh() tea.Cmd {
	m.refresh.fetchGen++
	m.refresh.inFlight = false
	m.refresh.dirty = false
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
// It returns the Cmds the update produced; continuing the refresh loop is the
// caller's job.
func (m *Model) applyIssues(msg data.FileChangedMsg) []tea.Cmd {
	var cmds []tea.Cmd

	// Warn if malformed lines were skipped
	if msg.Skipped > 0 {
		toast, toastCmd := components.ShowToast(
			fmt.Sprintf("Skipped %d malformed line(s)", msg.Skipped),
			components.ToastWarn, toastDuration,
		)
		m.toast = toast
		cmds = append(cmds, toastCmd)
	}

	// Diff against previous state for change indicators
	changes := m.diffIssues(msg.Issues)
	if changes > 0 {
		m.changedAt = time.Now()
		toast, toastCmd := components.ShowToast(
			fmt.Sprintf("File reloaded — %d issue%s changed", changes, plural(changes)),
			components.ToastInfo, toastDuration,
		)
		m.toast = toast
		cmds = append(cmds, toastCmd)
		cmds = append(cmds, tea.Tick(changeIndicatorDuration, func(time.Time) tea.Msg {
			return changeIndicatorExpiredMsg{}
		}))
	}

	// Update snapshot for next diff
	m.prevIssueMap = make(map[string]data.Status, len(msg.Issues))
	for _, iss := range msg.Issues {
		m.prevIssueMap[iss.ID] = iss.Status
	}

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
	return cmds
}
