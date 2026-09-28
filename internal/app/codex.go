package app

import (
	"context"
	"sort"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/matt-wright86/mardi-gras/internal/agent"
	"github.com/matt-wright86/mardi-gras/internal/codexmcp"
	"github.com/matt-wright86/mardi-gras/internal/components"
	"github.com/matt-wright86/mardi-gras/internal/ui"
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

type codexReplyDispatchedMsg struct {
	issueID string
	sess    *codexmcp.Session
}

// codexApprovalRequestMsg lands when codex sends a server-initiated approval
// request (exec or patch). ok is false when the request isn't a supported
// exec/patch approval, in which case the handler auto-denies.
type codexApprovalRequestMsg struct {
	issueID  string
	req      codexmcp.ServerRequest
	approval codexmcp.ElicitApproval
	ok       bool
}

// codexApprovalResolvedMsg lands after mg has written an approval response back
// to codex (or failed to).
type codexApprovalResolvedMsg struct {
	issueID string
	err     error
}

type codexReplyErrorMsg struct {
	issueID string
	err     error
}

// codexNextEventCmd returns a tea.Cmd that reads the next event or terminal
// result from the session.
//
// Why the closed-Events branch falls through to Done: at session termination
// both channels become ready (awaitResponse pushes the result then signalStop
// closes events). Go picks pseudo-randomly; if the closed-events branch wins,
// we must still surface the terminal result instead of returning a sentinel
// the handler would have to interpret.
func codexNextEventCmd(issueID string, sess *codexmcp.Session, serverReqCh <-chan codexmcp.ServerRequest) tea.Cmd {
	return func() tea.Msg {
		select {
		case ev, ok := <-sess.Events():
			if !ok {
				res := <-sess.Done()
				return codexDoneMsg{issueID: issueID, result: res}
			}
			return codexEventMsg{issueID: issueID, ev: ev}
		case req, ok := <-serverReqCh:
			if !ok {
				// Client shut down — serverReqCh closes with the transport, at
				// which point the session is terminating and Done() has a result.
				res := <-sess.Done()
				return codexDoneMsg{issueID: issueID, result: res}
			}
			approval, valid := codexmcp.ParseElicitApproval(req.Params)
			return codexApprovalRequestMsg{issueID: issueID, req: req, approval: approval, ok: valid}
		case res := <-sess.Done():
			return codexDoneMsg{issueID: issueID, result: res}
		}
	}
}

// codexApprovalDecisionResult builds the JSON-RPC result object for an
// elicitation/create approval reply. Decision is a codex ReviewDecision value
// (e.g. "approved", "approved_for_session", "denied", "abort").
func codexApprovalDecisionResult(decision string) map[string]any {
	return map[string]any{"decision": decision}
}

// codexRespondCmd writes an approval decision back to codex on the request's
// RawID, in a goroutine, surfacing the outcome as codexApprovalResolvedMsg.
func codexRespondCmd(issueID string, handle *agent.CodexMCPHandle, req codexmcp.ServerRequest, decision string) tea.Cmd {
	return func() tea.Msg {
		err := handle.Respond(req.RawID, codexApprovalDecisionResult(decision))
		return codexApprovalResolvedMsg{issueID: issueID, err: err}
	}
}

// handleCodexApprovalRequest routes an inbound exec/patch approval request. It
// always re-issues the event pump so the transcript keeps streaming while a modal
// is up (codex emits events while waiting on the approval). Unsupported requests
// are auto-denied; a second request that arrives while a modal is open is queued.
func (m Model) handleCodexApprovalRequest(msg codexApprovalRequestMsg) (tea.Model, tea.Cmd) {
	sess := m.codexSessions[msg.issueID]
	if sess == nil || sess.handle == nil {
		// Session gone — nothing to reply on, and re-pumping would be pointless.
		return m, nil
	}
	rePump := codexNextEventCmd(msg.issueID, sess.handle.Session(), sess.handle.ServerRequests())

	// Unknown / unsupported elicitation: deny so the agent loop doesn't stall.
	if !msg.ok {
		deny := codexRespondCmd(msg.issueID, sess.handle, msg.req, "denied")
		toast, tcmd := components.ShowToast(
			"Codex sent an unsupported approval request — auto-denied.",
			components.ToastWarn, toastDuration,
		)
		m.toast = toast
		return m, tea.Batch(rePump, deny, tcmd)
	}

	// A modal is already up — queue this one behind it.
	if m.approving {
		m.pendingApprovals = append(m.pendingApprovals, msg)
		return m, rePump
	}

	m.openApprovalDialog(msg)
	return m, rePump
}

// handleApprovalDialogResult replies to codex with the chosen decision (cancel is
// treated as a denial), then opens the next queued approval or closes the modal.
func (m Model) handleApprovalDialogResult(res components.ApprovalDialogResult) (tea.Model, tea.Cmd) {
	decision := res.Decision
	if res.Cancelled || decision == "" {
		decision = "denied"
	}

	cur := m.currentApproval
	sess := m.codexSessions[cur.issueID]
	var respond tea.Cmd
	if sess != nil && sess.handle != nil {
		respond = codexRespondCmd(cur.issueID, sess.handle, cur.req, decision)
	}

	if len(m.pendingApprovals) > 0 {
		next := m.pendingApprovals[0]
		m.pendingApprovals = m.pendingApprovals[1:]
		m.openApprovalDialog(next)
	} else {
		m.approving = false
		m.currentApproval = codexApprovalRequestMsg{}
	}
	return m, respond
}

// openApprovalDialog builds the modal for an approval request and marks the model
// as approving. Pointer receiver — mutates dialog state in place.
func (m *Model) openApprovalDialog(msg codexApprovalRequestMsg) {
	a := msg.approval
	var files []string
	if a.Kind == "patch" {
		files = make([]string, 0, len(a.Changes))
		for p := range a.Changes {
			files = append(files, p)
		}
		sort.Strings(files)
	}
	m.approving = true
	m.currentApproval = msg
	m.approvalDialog = components.NewApprovalDialog(
		a.Kind, a.Message, a.Command, a.Cwd, a.Reason, files, m.width, m.height,
	)
}

// codexReplyCmd invokes Handle.Reply in a goroutine and returns the
// resulting tea.Msg (either codexReplyDispatchedMsg with the new session
// or codexReplyErrorMsg).
func codexReplyCmd(issueID, prompt string, handle *agent.CodexMCPHandle) tea.Cmd {
	return func() tea.Msg {
		// Use a generous context for the reply tools/call. Like the initial
		// launch, the session itself uses context.Background() internally so
		// this ctx only bounds the request setup, not the session lifetime.
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		sess, err := handle.Reply(ctx, prompt)
		if err != nil {
			return codexReplyErrorMsg{issueID: issueID, err: err}
		}
		return codexReplyDispatchedMsg{issueID: issueID, sess: sess}
	}
}

// codexLaunchCmd kicks off the LaunchCodexMCP call in a goroutine. Codex's
// initial handshake (the mcp_startup of sub-MCP servers) can take many
// seconds — return codexLaunchedMsg only after the session is ready to
// stream events.
func codexLaunchCmd(issueID, prompt, projectDir, clientVersion, approvalPolicy string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		handle, err := agent.LaunchCodexMCP(ctx, agent.LaunchCodexMCPOptions{
			Prompt:         prompt,
			ProjectDir:     projectDir,
			ClientVersion:  clientVersion,
			ApprovalPolicy: approvalPolicy,
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

// isCodexShownFor returns true when the codex transcript overlay is visible
// and currently showing the session for issueID. Used as a guard before
// updating the displayed transcript state.
func (m *Model) isCodexShownFor(issueID string) bool {
	return m.showCodex &&
		m.parade.SelectedIssue != nil &&
		m.parade.SelectedIssue.ID == issueID
}

// dismissCodexReply clears any in-flight reply input state. Called whenever
// the overlay closes or a sibling overlay opens, so a stranded codexReplying
// flag can't intercept all keypresses after the user has moved on.
func (m *Model) dismissCodexReply() {
	m.codexReplying = false
	m.codexReplyID = ""
}

// startCodexReply initializes the reply input bar for the given session's
// issue. Returns a textinput.Blink cmd to start the cursor blinking.
// Mirrors the mail-reply setup pattern from views.ActionMailReply handling.
func (m *Model) startCodexReply(issueID string) tea.Cmd {
	m.codexReplying = true
	m.codexReplyID = issueID
	m.codexReplyInput = textinput.New()
	m.codexReplyInput.Prompt = ui.InputPrompt.Render("reply> ")
	m.codexReplyInput.Placeholder = "Send a follow-up to codex..."
	m.codexReplyInput.SetWidth(50)
	m.codexReplyInput.Focus()
	return textinput.Blink
}

// openCodexReply is the `r`-key handler when the codex overlay is visible.
// It validates that a session exists, is terminal (not mid-turn), and has
// a usable threadID, then opens the reply input. On gate failure it surfaces
// a toast and leaves state untouched.
func (m Model) openCodexReply() (tea.Model, tea.Cmd) {
	issue := m.parade.SelectedIssue
	if issue == nil {
		return m, nil
	}
	reason := codexReplyGateReason(m.codexSessions[issue.ID])
	if reason != "" {
		cmd := m.codexReplyToast(reason)
		return m, cmd
	}
	cmd := m.startCodexReply(issue.ID)
	return m, cmd
}

// codexReplyGateReason returns a user-facing message explaining why the
// reply gate is blocking, or "" if the reply is allowed.
func codexReplyGateReason(sess *codexSession) string {
	if sess == nil {
		return "No codex session for this issue."
	}
	if sess.state == nil || sess.state.ThreadID == "" {
		return "Codex session not ready yet."
	}
	if sess.state.Status == "running" {
		return "Codex turn still running."
	}
	if sess.handle == nil {
		return "Session handle is closed — press M to relaunch."
	}
	return ""
}

func (m *Model) codexReplyToast(text string) tea.Cmd {
	toast, cmd := components.ShowToast(text, components.ToastWarn, toastDuration)
	m.toast = toast
	return cmd
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
	m.stopBeadStream()
	m.stopServe()
}

// toggleCodexTranscript is the M-key handler. It opens the codex transcript
// for the selected issue; if no session exists it spawns one, then routes
// subsequent events through the transcript view.
func (m Model) toggleCodexTranscript() (tea.Model, tea.Cmd) {
	if m.showCodex {
		m.showCodex = false
		m.dismissCodexReply()
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
	m.showChanges = false

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
	// M-key launches are human-present: use on-request so codex surfaces exec
	// and apply-patch approvals through mg's modal instead of auto-approving.
	// Polecat/gt-sling and tmux launches keep "never" (no human at the terminal).
	return m, codexLaunchCmd(issue.ID, prompt, m.projectDir, "", "on-request")
}
