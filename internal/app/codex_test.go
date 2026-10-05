package app

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/matt-wright86/mardi-gras/internal/agent"
	"github.com/matt-wright86/mardi-gras/internal/codexapp"
	"github.com/matt-wright86/mardi-gras/internal/views"
)

func TestKeyMTogglesCodexOverlay(t *testing.T) {
	got := setupModel(t)
	if got.showCodex {
		t.Fatal("expected showCodex=false initially")
	}

	// First press: should open the overlay.
	model, _ := got.Update(tea.KeyPressMsg{Code: 'M', Text: "M"})
	got = model.(Model)
	if !got.showCodex {
		t.Fatal("expected showCodex=true after M")
	}

	// Second press: should close the overlay.
	model, _ = got.Update(tea.KeyPressMsg{Code: 'M', Text: "M"})
	got = model.(Model)
	if got.showCodex {
		t.Fatal("expected showCodex=false after second M")
	}
}

func TestKeyMOpeningClearsOtherOverlays(t *testing.T) {
	got := setupModel(t)
	got.showProblems = true
	got.showGasTown = true
	got.showDoctor = true

	model, _ := got.Update(tea.KeyPressMsg{Code: 'M', Text: "M"})
	got = model.(Model)
	if !got.showCodex {
		t.Fatal("showCodex should be true")
	}
	if got.showProblems || got.showGasTown || got.showDoctor {
		t.Fatal("other overlays should be cleared")
	}
}

func TestCodexEventMsgAppendsToTranscript(t *testing.T) {
	got := setupModel(t)
	issueID := got.parade.SelectedIssue.ID

	got.codexSessions[issueID] = &codexSession{
		state: &views.CodexTranscriptState{
			IssueID: issueID,
			Status:  "running",
			StartAt: time.Now(),
		},
	}

	raw, _ := json.Marshal(map[string]string{
		"type":    "agent_message",
		"message": "hi from codex",
	})
	ev := codexapp.CodexEvent{Msg: raw}
	model, _ := got.Update(codexEventMsg{issueID: issueID, ev: ev})
	got = model.(Model)

	sess := got.codexSessions[issueID]
	if len(sess.state.Entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(sess.state.Entries))
	}
	if sess.state.Entries[0].Title != "hi from codex" {
		t.Fatalf("title = %q", sess.state.Entries[0].Title)
	}
}

func TestKeyRGate(t *testing.T) {
	const issueID = "open-1"
	tests := []struct {
		name         string
		overlay      bool
		sess         *codexSession
		wantReplying bool
		wantQAMode   string
	}{
		{
			name:    "terminal session opens reply",
			overlay: true,
			sess: &codexSession{
				state:  &views.CodexTranscriptState{IssueID: issueID, ThreadID: "thr-test", Status: "done"},
				handle: &agent.CodexAppHandle{},
			},
			wantReplying: true,
		},
		{
			name:    "running session refuses",
			overlay: true,
			sess: &codexSession{
				state:  &views.CodexTranscriptState{IssueID: issueID, ThreadID: "thr-test", Status: "running"},
				handle: &agent.CodexAppHandle{},
			},
			wantReplying: false,
		},
		{
			name:    "no threadID yet refuses",
			overlay: true,
			sess: &codexSession{
				state:  &views.CodexTranscriptState{IssueID: issueID, Status: "done"},
				handle: &agent.CodexAppHandle{},
			},
			wantReplying: false,
		},
		{
			name:         "overlay closed falls through to comment",
			overlay:      false,
			sess:         nil,
			wantReplying: false,
			wantQAMode:   "comment",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := setupModel(t)
			m.showCodex = tc.overlay
			if tc.sess != nil {
				m.codexSessions[m.parade.SelectedIssue.ID] = tc.sess
			}
			model, _ := m.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
			got := model.(Model)
			if got.codexReplying != tc.wantReplying {
				t.Errorf("codexReplying = %v, want %v", got.codexReplying, tc.wantReplying)
			}
			if got.qaMode != tc.wantQAMode {
				t.Errorf("qaMode = %q, want %q", got.qaMode, tc.wantQAMode)
			}
		})
	}
}

// TestCodexReplyEnterDispatchesAndFlipsStatus drives the codexReplying
// enter path: type a body, hit enter, observe state.Status flip to
// "running" and codexReplying clear. Doesn't drive Handle.Reply itself —
// that's covered by TestReplyRotatesSession in internal/agent.
func TestCodexReplyEnterDispatchesAndFlipsStatus(t *testing.T) {
	got := setupModel(t)
	issueID := got.parade.SelectedIssue.ID

	got.showCodex = true
	got.codexSessions[issueID] = &codexSession{
		state: &views.CodexTranscriptState{
			IssueID:  issueID,
			ThreadID: "thr-x",
			Status:   "done",
			StartAt:  time.Now(),
		},
		handle: &agent.CodexAppHandle{},
	}

	model, _ := got.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
	got = model.(Model)
	if !got.codexReplying {
		t.Fatal("codexReplying not set")
	}

	got.codexReplyInput.SetValue("follow up")

	model, _ = got.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	got = model.(Model)

	if got.codexReplying {
		t.Fatal("codexReplying should be false after enter")
	}
	if got.codexSessions[issueID].state.Status != "running" {
		t.Fatalf("session state should flip to running after enter; got %q",
			got.codexSessions[issueID].state.Status)
	}
}

// TestDismissCodexReplyClearsState asserts dismissCodexReply zeroes the
// reply input triad. The helper is called from every showCodex=false site
// so an in-flight reply input never leaks past overlay close.
func TestDismissCodexReplyClearsState(t *testing.T) {
	m := setupModel(t)
	m.codexReplying = true
	m.codexReplyID = "open-1"
	m.dismissCodexReply()
	if m.codexReplying {
		t.Error("codexReplying should be false")
	}
	if m.codexReplyID != "" {
		t.Errorf("codexReplyID = %q, want empty", m.codexReplyID)
	}
}

func TestCodexDoneMsgMarksSessionTerminal(t *testing.T) {
	got := setupModel(t)
	issueID := got.parade.SelectedIssue.ID

	got.codexSessions[issueID] = &codexSession{
		state: &views.CodexTranscriptState{
			IssueID: issueID,
			Status:  "running",
			StartAt: time.Now(),
		},
	}

	model, _ := got.Update(codexDoneMsg{
		issueID: issueID,
		result:  codexapp.SessionResult{ThreadID: "tid", Content: "all done"},
	})
	got = model.(Model)
	sess := got.codexSessions[issueID]
	if sess.state.Status != "done" {
		t.Fatalf("status = %q, want done", sess.state.Status)
	}
	if sess.state.EndAt.IsZero() {
		t.Fatal("EndAt should be set")
	}
}

// fakeCodexOnPath puts an executable named codex on PATH so
// agent.CodexAvailable passes; nothing ever runs it.
func fakeCodexOnPath(t *testing.T) {
	t.Helper()
	t.Setenv(agent.AgentCommandEnv, "") // no wrapper: resolve codex itself
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "codex"), []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
}

func TestKeyMDoesNotLaunchCodex(t *testing.T) {
	// M used to spawn codex mcp-server when the issue had no session: one
	// keystroke set an agent working on the repo (mg-ney).
	fakeCodexOnPath(t)
	got := setupModel(t)

	model, cmd := got.Update(tea.KeyPressMsg{Code: 'M', Text: "M"})
	got = model.(Model)
	if !got.showCodex {
		t.Fatal("M should open the transcript")
	}
	if cmd != nil || len(got.codexSessions) != 0 {
		t.Fatalf("M started a session: cmd %v sessions %d", cmd != nil, len(got.codexSessions))
	}
	if view := got.codexTranscript.View(); !strings.Contains(view, "enter start session") {
		t.Fatalf("transcript should say how to start a session:\n%s", view)
	}
}

func TestKeyMEdgeCaseNoCodexOnPath(t *testing.T) {
	t.Setenv(agent.AgentCommandEnv, "")
	t.Setenv("PATH", t.TempDir())
	got := setupModel(t)

	model, _ := got.Update(tea.KeyPressMsg{Code: 'M', Text: "M"})
	got = model.(Model)
	view := got.codexTranscript.View()
	if !strings.Contains(view, "codex is not on PATH") || strings.Contains(view, "enter start") {
		t.Fatalf("without codex the transcript should say so and offer no start:\n%s", view)
	}
	model, cmd := got.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	got = model.(Model)
	if len(got.codexSessions) != 0 {
		t.Fatal("enter must not start a session without codex")
	}
	_ = cmd
}

func TestCodexEnterStartsAndLaunchErrorShowsErrored(t *testing.T) {
	fakeCodexOnPath(t)
	got := setupModel(t)
	issueID := got.parade.SelectedIssue.ID
	model, _ := got.Update(tea.KeyPressMsg{Code: 'M', Text: "M"})
	got = model.(Model)

	model, cmd := got.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	got = model.(Model)
	if cmd == nil {
		t.Fatal("enter in the transcript should launch codex")
	}
	sess := got.codexSessions[issueID]
	if sess == nil || !sess.launching {
		t.Fatalf("expected a launching placeholder, got %+v", sess)
	}
	// A second enter while launching must not launch twice.
	if _, again := got.Update(tea.KeyPressMsg{Code: tea.KeyEnter}); again != nil {
		t.Fatal("enter while launching started a second launch")
	}

	model, _ = got.Update(codexLaunchErrorMsg{issueID: issueID, err: errors.New("spawn codex mcp-server: boom")})
	got = model.(Model)
	sess = got.codexSessions[issueID]
	if sess.state.Status != "errored" || sess.launching {
		t.Fatalf("status %q launching %v, want errored and not launching", sess.state.Status, sess.launching)
	}
	if view := got.codexTranscript.View(); strings.Contains(view, "running") || !strings.Contains(view, "launch failed") {
		t.Fatalf("transcript should show the failed launch, not running:\n%s", view)
	}
	if !codexRestartable(sess) {
		t.Fatal("an errored launch should allow a new session")
	}
}

func TestCodexKStopsSession(t *testing.T) {
	got := setupModel(t)
	issueID := got.parade.SelectedIssue.ID
	got.codexSessions[issueID] = &codexSession{
		handle:  &agent.CodexAppHandle{},
		state:   &views.CodexTranscriptState{IssueID: issueID, Status: "running", StartAt: time.Now()},
		pumping: true,
	}
	model, _ := got.Update(tea.KeyPressMsg{Code: 'M', Text: "M"})
	got = model.(Model)

	model, _ = got.Update(tea.KeyPressMsg{Code: 'K', Text: "K"})
	got = model.(Model)
	sess := got.codexSessions[issueID]
	if !sess.closed || sess.state.Status != "canceled" {
		t.Fatalf("closed %v status %q, want a canceled session", sess.closed, sess.state.Status)
	}
	if codexRestartable(sess) {
		t.Fatal("restart must wait for the event pump to drain")
	}
	if reason := codexReplyGateReason(sess); reason == "" {
		t.Fatal("a stopped session must not accept replies")
	}

	// The drained pump arrives; the stop sticks and a restart is now safe.
	model, _ = got.Update(codexDoneMsg{issueID: issueID, result: codexapp.SessionResult{Err: errors.New("canceled")}})
	got = model.(Model)
	sess = got.codexSessions[issueID]
	if sess.state.Status != "canceled" || !codexRestartable(sess) {
		t.Fatalf("status %q restartable %v after drain", sess.state.Status, codexRestartable(sess))
	}
}

func TestApprovalDialogEnterClosesModal(t *testing.T) {
	// While approving, update() forwarded every message to the dialog,
	// including the dialog's own ApprovalDialogResult, so Enter and esc
	// could never close it: found against a real codex session.
	for _, key := range []tea.KeyPressMsg{{Code: tea.KeyEnter}, {Code: tea.KeyEscape}} {
		got := setupModel(t)
		got.codexSessions["open-1"] = &codexSession{state: &views.CodexTranscriptState{IssueID: "open-1"}}
		got.openApprovalDialog(execApproval(`7`, "curl", "-sI", "https://example.com"))

		model, cmd := got.Update(key)
		got = model.(Model)
		if cmd == nil {
			t.Fatalf("%s: dialog produced no result", key.String())
		}
		model, _ = got.Update(cmd())
		if model.(Model).approving {
			t.Fatalf("%s: modal still open after its result", key.String())
		}
	}
}

func TestCodexApprovalDecisionValues(t *testing.T) {
	// app-server's decisions, not codex MCP's (mg-xge.2).
	for in, want := range map[string]string{
		"approved": "accept", "approved_for_session": "acceptForSession",
		"denied": "decline", "abort": "cancel", "": "decline", "bogus": "decline",
	} {
		if got := codexApprovalDecisionResult(in)["decision"]; got != want {
			t.Errorf("%q -> %v, want %s", in, got, want)
		}
	}
}

func TestResolvedApprovalClosesModalAndQueue(t *testing.T) {
	// When the server stops waiting (turn interrupted, answered elsewhere),
	// a modal for that request would answer into the void (mg-xge.2).
	got := setupModel(t)
	issueID := got.parade.SelectedIssue.ID
	got.codexSessions[issueID] = &codexSession{state: &views.CodexTranscriptState{IssueID: issueID}}
	first := execApproval(`7`, "curl", "x")
	first.issueID = issueID
	second := execApproval(`"s8"`, "ls")
	second.issueID = issueID
	got.openApprovalDialog(first)
	got.pendingApprovals = append(got.pendingApprovals, second)

	got.dropResolvedApproval(issueID, json.RawMessage(`"s8"`))
	if len(got.pendingApprovals) != 0 || !got.approving {
		t.Fatalf("queued request should go, the open one stay: pending %d approving %v", len(got.pendingApprovals), got.approving)
	}
	got.dropResolvedApproval(issueID, json.RawMessage(`7`))
	if got.approving {
		t.Fatal("the open modal should close when its request is resolved")
	}
}
