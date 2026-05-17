package app

import (
	"context"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/matt-wright86/mardi-gras/internal/agent"
	"github.com/matt-wright86/mardi-gras/internal/codexmcp"
	"github.com/matt-wright86/mardi-gras/internal/views"
)

// codexSession is one in-flight (or terminated) codex MCP session attached
// to a specific issue. State holds the transcript surface; handle owns the
// subprocess.
type codexSession struct {
	handle *agent.CodexMCPHandle
	state  *views.CodexTranscriptState
}

// Codex MCP message types. They are scoped to the codex feature so app.go's
// existing message dispatch stays uncluttered.

// codexLaunchedMsg lands when LaunchCodexMCP has returned successfully and
// the session is ready to stream events.
type codexLaunchedMsg struct {
	issueID string
	sess    *codexSession
}

// codexLaunchErrorMsg lands when LaunchCodexMCP fails.
type codexLaunchErrorMsg struct {
	issueID string
	err     error
}

// codexEventMsg carries one streamed event for the given issue's session.
// The handler appends to state and re-issues codexNextEventCmd.
type codexEventMsg struct {
	issueID string
	ev      codexmcp.CodexEvent
}

// codexDoneMsg carries the terminal SessionResult.
type codexDoneMsg struct {
	issueID string
	result  codexmcp.SessionResult
}

// codexNextEventCmd returns a tea.Cmd that reads the next event or terminal
// result from the session.
//
// Why the closed-Events branch falls through to Done: at session termination
// both channels become ready (awaitResponse pushes the result then signalStop
// closes events). Go picks pseudo-randomly; if the closed-events branch wins,
// we must still surface the terminal result instead of returning a sentinel
// the handler would have to interpret.
func codexNextEventCmd(issueID string, sess *codexmcp.Session) tea.Cmd {
	return func() tea.Msg {
		select {
		case ev, ok := <-sess.Events():
			if !ok {
				res := <-sess.Done()
				return codexDoneMsg{issueID: issueID, result: res}
			}
			return codexEventMsg{issueID: issueID, ev: ev}
		case res := <-sess.Done():
			return codexDoneMsg{issueID: issueID, result: res}
		}
	}
}

// codexLaunchCmd kicks off the LaunchCodexMCP call in a goroutine. Codex's
// initial handshake (the mcp_startup of sub-MCP servers) can take many
// seconds — return codexLaunchedMsg only after the session is ready to
// stream events.
func codexLaunchCmd(issueID, prompt, projectDir, clientVersion string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		handle, err := agent.LaunchCodexMCP(ctx, agent.LaunchCodexMCPOptions{
			Prompt:        prompt,
			ProjectDir:    projectDir,
			ClientVersion: clientVersion,
		})
		if err != nil {
			return codexLaunchErrorMsg{issueID: issueID, err: err}
		}
		return codexLaunchedMsg{
			issueID: issueID,
			sess: &codexSession{
				handle: handle,
				state: &views.CodexTranscriptState{
					IssueID: issueID,
					Status:  "running",
					StartAt: time.Now(),
				},
			},
		}
	}
}

// applyCodexEvent updates a session's transcript state when an event lands.
// Returns true if the event was actually consumed (display-worthy).
func applyCodexEvent(sess *codexSession, ev codexmcp.CodexEvent) bool {
	if sess == nil || sess.state == nil {
		return false
	}
	return sess.state.AppendEvent(ev)
}

// finalizeCodexSession marks a session as terminated and copies relevant
// fields from the SessionResult into the transcript state.
func finalizeCodexSession(sess *codexSession, res codexmcp.SessionResult) {
	if sess == nil || sess.state == nil {
		return
	}
	now := time.Now()
	sess.state.EndAt = now
	if res.Err != nil {
		sess.state.Status = "errored"
		sess.state.AppendEntry(views.CodexTranscriptEntry{
			At:    now,
			Kind:  "agent",
			Title: "session error: " + res.Err.Error(),
			Error: true,
		})
		return
	}
	sess.state.Status = "done"
	if res.Content != "" {
		sess.state.AppendEntry(views.CodexTranscriptEntry{
			At:    now,
			Kind:  "agent",
			Title: "final",
			Body:  res.Content,
		})
	}
	if res.ThreadID != "" && sess.state.ThreadID == "" {
		sess.state.ThreadID = res.ThreadID
	}
}

// closeAllCodexSessions terminates every active codex MCP subprocess. Called
// on app quit so we don't leak background processes.
func closeAllCodexSessions(sessions map[string]*codexSession) {
	for _, sess := range sessions {
		if sess == nil || sess.handle == nil {
			continue
		}
		_ = sess.handle.Close()
	}
}

// Cleanup terminates all codex MCP subprocesses owned by this model. Safe to
// call after tea.Program.Run returns. Idempotent.
func (m *Model) Cleanup() {
	closeAllCodexSessions(m.codexSessions)
}

// toggleCodexTranscript is the M-key handler. It opens the codex transcript
// for the selected issue; if no session exists it spawns one, then routes
// subsequent events through the transcript view.
func (m Model) toggleCodexTranscript() (tea.Model, tea.Cmd) {
	if m.showCodex {
		m.showCodex = false
		return m, nil
	}

	issue := m.parade.SelectedIssue
	if issue == nil {
		return m, nil
	}

	m.showCodex = true
	m.showGasTown = false
	m.showProblems = false
	m.showDoctor = false

	if sess, ok := m.codexSessions[issue.ID]; ok && sess != nil {
		m.codexTranscript.SetState(sess.state)
		return m, nil
	}

	// No session yet — spawn one.
	deps := issue.EvaluateDependencies(m.detail.IssueMap, m.blockingTypes)
	prompt := agent.BuildPrompt(*issue, deps, m.detail.IssueMap)
	m.codexTranscript.SetState(&views.CodexTranscriptState{
		IssueID: issue.ID,
		Status:  "running",
		StartAt: time.Now(),
	})
	return m, codexLaunchCmd(issue.ID, prompt, m.projectDir, "")
}
