package app

import (
	"encoding/json"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/matt-wright86/mardi-gras/internal/agent"
	"github.com/matt-wright86/mardi-gras/internal/codexmcp"
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
	ev := codexmcp.CodexEvent{Msg: raw}
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

func TestKeyROpensCodexReplyWhenSessionTerminal(t *testing.T) {
	got := setupModel(t)
	issueID := got.parade.SelectedIssue.ID

	// Open the codex overlay manually and prime a terminal session.
	got.showCodex = true
	got.codexSessions[issueID] = &codexSession{
		state: &views.CodexTranscriptState{
			IssueID:  issueID,
			ThreadID: "thr-test",
			Status:   "done",
			StartAt:  time.Now(),
		},
		handle: &agent.CodexMCPHandle{}, // gate only checks non-nil
	}

	model, _ := got.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
	got = model.(Model)

	if !got.codexReplying {
		t.Fatal("expected codexReplying=true after r on terminal session")
	}
	if got.codexReplyID != issueID {
		t.Fatalf("codexReplyID = %q", got.codexReplyID)
	}
}

func TestKeyRRefusesWhileSessionRunning(t *testing.T) {
	got := setupModel(t)
	issueID := got.parade.SelectedIssue.ID

	got.showCodex = true
	got.codexSessions[issueID] = &codexSession{
		state: &views.CodexTranscriptState{
			IssueID:  issueID,
			ThreadID: "thr-test",
			Status:   "running",
			StartAt:  time.Now(),
		},
		handle: &agent.CodexMCPHandle{},
	}

	model, _ := got.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
	got = model.(Model)

	if got.codexReplying {
		t.Fatal("codexReplying should remain false while session is running")
	}
}

func TestKeyRRefusesWhenNoThreadIDYet(t *testing.T) {
	got := setupModel(t)
	issueID := got.parade.SelectedIssue.ID

	got.showCodex = true
	got.codexSessions[issueID] = &codexSession{
		state: &views.CodexTranscriptState{
			IssueID: issueID,
			Status:  "done",
			// no ThreadID
			StartAt: time.Now(),
		},
		handle: &agent.CodexMCPHandle{},
	}

	model, _ := got.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
	got = model.(Model)

	if got.codexReplying {
		t.Fatal("codexReplying should be false when threadID is empty")
	}
}

func TestKeyRWithoutOverlayActsAsComment(t *testing.T) {
	got := setupModel(t)

	// Codex overlay closed -> r should NOT trigger codexReplying.
	model, _ := got.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
	got = model.(Model)

	if got.codexReplying {
		t.Fatal("codexReplying should remain false when overlay is closed")
	}
	if got.qaMode != "comment" {
		t.Fatalf("expected r to fall through to comment quick-action; qaMode = %q", got.qaMode)
	}
}

func TestCodexReplyEnterCallsHandleReply(t *testing.T) {
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
		// nil handle on purpose — codexReplyCmd will fail at handle.Reply
		// because the gate only confirms handle != nil; we want to exercise
		// the input enter path without spawning a real subprocess. The
		// error lands as codexReplyErrorMsg which we don't drive here.
		handle: &agent.CodexMCPHandle{},
	}

	// Open the reply input.
	model, _ := got.Update(tea.KeyPressMsg{Code: 'r', Text: "r"})
	got = model.(Model)
	if !got.codexReplying {
		t.Fatal("codexReplying not set")
	}

	// Type something.
	got.codexReplyInput.SetValue("follow up")

	// Press enter — KeyPressMsg's String() returns "enter" for KeyEnter.
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
		result:  codexmcp.SessionResult{ThreadID: "tid", Content: "all done"},
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
