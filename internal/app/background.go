package app

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/matt-wright86/mardi-gras/internal/components"
	"github.com/matt-wright86/mardi-gras/internal/data"
)

// updateBackground handles the results of background work before anything
// else sees them. While a modal (palette, form, dialog, text input) is open,
// Update forwards every other message to it, and a modal drops what it does
// not understand. Each message here closes an in-flight gate or re-arms a
// timer, so losing one would stall its loop for the rest of the session: no
// more reloads, agent captures, journal probes, status polls or animation.
func (m Model) updateBackground(msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	var model tea.Model
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case refreshTickMsg:
		model, cmd = m.handleRefreshTick(msg)
	case refreshResultMsg:
		model, cmd = m.handleRefreshResult(msg)
	case data.FileChangedMsg:
		model, cmd = m.handleIssuesLoaded(msg)
	case data.FileUnchangedMsg:
		model, cmd = m.handleIssuesUnchanged(msg)
	case data.FileWatchErrorMsg:
		model, cmd = m.handleLoadError(msg)
	case data.CLIHealthCheckMsg:
		model, cmd = m.handleHealthCheck(msg)
	case townStatusMsg:
		model, cmd = m.handleTownStatus(msg)
	case patrolScanMsg:
		model, cmd = m.handlePatrolScan(msg)
	case gasTownTickMsg:
		model, cmd = m.handleGasTownTick()
	case headerShimmerMsg:
		model, cmd = m.handleHeaderShimmer()
	case components.ToastDismissMsg:
		model, cmd = m.handleToastDismiss()
	case changeIndicatorExpiredMsg:
		model, cmd = m.handleChangeIndicatorExpired()
	case agentStatusMsg:
		model, cmd = m.handleAgentStatus(msg)
	case agentOutputMsg:
		model, cmd = m.handleAgentOutput(msg)
	case journalTickMsg:
		model, cmd = m.handleJournalTick(msg)
	case journalEnabledMsg:
		model, cmd = m.handleJournalEnabled(msg)
	case journalHeadMsg:
		model, cmd = m.handleJournalHead(msg)
	case journalProbeMsg:
		model, cmd = m.handleJournalProbe(msg)
	case journalBackfillMsg:
		model, cmd = m.handleJournalBackfill(msg)
	case beadStreamStartedMsg:
		model, cmd = m.handleBeadStreamStarted(msg)
	case beadStreamMsg:
		model, cmd = m.handleBeadStream(msg)
	case serveMsg:
		model, cmd = m.handleServe(msg)
	default:
		return m, nil, false
	}
	return model, cmd, true
}

// handleTownStatus absorbs an orchestrator status poll.
func (m Model) handleTownStatus(msg townStatusMsg) (tea.Model, tea.Cmd) {
	m.gtPollInFlight = false
	if msg.err != nil {
		// Record the failure so the panel can report it. Previously this
		// error was dropped on the floor, and with no status to fall back
		// on the panel showed its loading line forever — an unreachable
		// orchestrator was indistinguishable from a slow one. A successful
		// poll that already populated townStatus keeps its data; the error
		// only surfaces when there is nothing to show.
		m.townStatusErr = msg.err
		m.gasTown.SetStatusError(msg.err, m.driver.Backend())
		return m, nil
	}
	if msg.status != nil {
		m.townStatusErr = nil
		m.gasTown.SetStatusError(nil, "")
		m.townStatus = msg.status
		m.setActiveAgents(msg.status.ActiveAgentMap())
		if m.showGasTown {
			m.gasTown.SetStatus(m.townStatus, m.gtEnv)
			m.recomputeVelocity()
		}
		if m.showProblems {
			m.problems.SetProblems(m.allProblems())
		}
		// Check if selected issue now has an agent → fetch molecule
		if cmd := m.maybeFetchMolecule(); cmd != nil {
			return m, cmd
		}
	}
	return m, nil
}

// handlePatrolScan absorbs a gt patrol scan.
func (m Model) handlePatrolScan(msg patrolScanMsg) (tea.Model, tea.Cmd) {
	m.patrolScanInFlight = false
	if msg.err != nil {
		// Clear stale patrol data and update TTL to prevent hot-loop retries
		m.patrolScan = nil
		m.lastPatrolScan = time.Now()
		m.header.ProblemCount = len(m.allProblems())
		if m.showProblems {
			m.problems.SetProblems(m.allProblems())
		}
		return m, nil
	}
	if msg.scan != nil {
		m.patrolScan = msg.scan
		m.lastPatrolScan = time.Now()
		m.header.ProblemCount = len(m.allProblems())
		if m.showProblems {
			m.problems.SetProblems(m.allProblems())
		}
	}
	return m, nil
}

// handleGasTownTick advances the Gas Town panel while it is visible.
func (m Model) handleGasTownTick() (tea.Model, tea.Cmd) {
	m.gasTown.Tick()
	// Keep ticking while panel is visible
	if m.showGasTown {
		return m, gasTownTickCmd()
	}
	m.gasTownTicking = false
	return m, nil
}

// handleHeaderShimmer advances the header shimmer animation.
func (m Model) handleHeaderShimmer() (tea.Model, tea.Cmd) {
	m.beadOffset++
	return m, headerShimmerCmd()
}

// handleToastDismiss clears the toast when its timer ends.
func (m Model) handleToastDismiss() (tea.Model, tea.Cmd) {
	m.toast = components.Toast{}
	return m, nil
}

// handleChangeIndicatorExpired clears change indicators that have reached
// their lifetime.
func (m Model) handleChangeIndicatorExpired() (tea.Model, tea.Cmd) {
	cutoff := time.Now().Add(-changeIndicatorDuration)
	for id, at := range m.changedIDs {
		if !at.After(cutoff) {
			delete(m.changedIDs, id)
		}
	}
	m.parade.ChangedIDs = m.changedSet()
	return m, nil
}

// handleAgentStatus absorbs a tmux agent poll.
func (m Model) handleAgentStatus(msg agentStatusMsg) (tea.Model, tea.Cmd) {
	m.setActiveAgents(msg.activeAgents)
	return m, nil
}

// handleAgentOutput shows a finished pane capture. A capture that lands after
// the selection moved on or the agent went away is dropped; Update issues a
// fresh one if one is still wanted.
func (m Model) handleAgentOutput(msg agentOutputMsg) (tea.Model, tea.Cmd) {
	m.captureInFlight = false
	if sel := m.parade.SelectedIssue; sel != nil && sel.ID == msg.issueID && m.capturable(msg.issueID) {
		m.detail.SetAgentOutput(msg.issueID, msg.lines)
	}
	return m, nil
}
