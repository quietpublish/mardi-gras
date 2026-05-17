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
				handle: &agent.CodexMCPHandle{},
			},
			wantReplying: true,
		},
		{
			name:    "running session refuses",
			overlay: true,
			sess: &codexSession{
				state:  &views.CodexTranscriptState{IssueID: issueID, ThreadID: "thr-test", Status: "running"},
				handle: &agent.CodexMCPHandle{},
			},
			wantReplying: false,
		},
		{
			name:    "no threadID yet refuses",
			overlay: true,
			sess: &codexSession{
				state:  &views.CodexTranscriptState{IssueID: issueID, Status: "done"},
				handle: &agent.CodexMCPHandle{},
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
		handle: &agent.CodexMCPHandle{},
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
