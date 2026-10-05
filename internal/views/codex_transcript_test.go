package views

import (
	"encoding/json"
	"maps"
	"strings"
	"testing"

	"github.com/matt-wright86/mardi-gras/internal/codexmcp"
)

func mkEvent(msgType string, payload any) codexmcp.CodexEvent {
	body := map[string]any{"type": msgType}
	if payload != nil {
		b, _ := json.Marshal(payload)
		var m map[string]any
		_ = json.Unmarshal(b, &m)
		maps.Copy(body, m)
	}
	raw, _ := json.Marshal(body)
	return codexmcp.CodexEvent{Msg: raw}
}

func TestAppendEventAgentMessageSplitsBody(t *testing.T) {
	state := &CodexTranscriptState{IssueID: "i1"}
	ok := state.AppendEvent(mkEvent("agent_message", map[string]string{
		"message": "first line\nsecond line\nthird",
	}))
	if !ok {
		t.Fatal("expected event to be appended")
	}
	if len(state.Entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(state.Entries))
	}
	e := state.Entries[0]
	if e.Kind != "agent" {
		t.Fatalf("kind = %q", e.Kind)
	}
	if e.Title != "first line" {
		t.Fatalf("title = %q", e.Title)
	}
	if !strings.Contains(e.Body, "second line") || !strings.Contains(e.Body, "third") {
		t.Fatalf("body = %q", e.Body)
	}
}

func TestAppendEventExecCommandPair(t *testing.T) {
	state := &CodexTranscriptState{}
	_ = state.AppendEvent(mkEvent("exec_command_begin", map[string]any{
		"call_id": "c1",
		"command": []string{"ls", "-la"},
	}))
	_ = state.AppendEvent(mkEvent("exec_command_end", map[string]any{
		"call_id":   "c1",
		"exit_code": 2,
	}))
	if len(state.Entries) != 2 {
		t.Fatalf("entries = %d, want 2", len(state.Entries))
	}
	if state.Entries[0].Title != "ls -la" {
		t.Fatalf("begin title = %q", state.Entries[0].Title)
	}
	if state.Entries[1].Title != "exit 2" {
		t.Fatalf("end title = %q", state.Entries[1].Title)
	}
	if !state.Entries[1].Error {
		t.Fatal("non-zero exit should mark error")
	}
}

func TestAppendEventSessionConfiguredCapturesMeta(t *testing.T) {
	state := &CodexTranscriptState{}
	_ = state.AppendEvent(mkEvent("session_configured", map[string]any{
		"thread_id":       "tid-1",
		"model":           "gpt-5.2",
		"approval_policy": "never",
		"cwd":             "/proj",
	}))
	if state.ThreadID != "tid-1" {
		t.Fatalf("ThreadID = %q", state.ThreadID)
	}
	if state.Model != "gpt-5.2" {
		t.Fatalf("Model = %q", state.Model)
	}
}

func TestAppendEventUnknownTypeIsDropped(t *testing.T) {
	state := &CodexTranscriptState{}
	ok := state.AppendEvent(mkEvent("agent_reasoning_section_break", nil))
	if ok {
		t.Fatal("unknown event type should be dropped")
	}
	if len(state.Entries) != 0 {
		t.Fatalf("entries = %d, want 0", len(state.Entries))
	}
}

func TestViewWithoutStateShowsPlaceholder(t *testing.T) {
	v := NewCodexTranscript(80, 24)
	out := v.View()
	if !strings.Contains(out, "CODEX (MCP)") {
		t.Fatalf("missing title: %q", out)
	}
	if !strings.Contains(out, "No Codex session") {
		t.Fatalf("missing placeholder: %q", out)
	}
}

func TestViewWithoutStateOffersStartOnlyWithCodex(t *testing.T) {
	v := NewCodexTranscript(80, 24)
	v.SetIdle("mg-1", "")
	if out := v.View(); !strings.Contains(out, "mg-1") || !strings.Contains(out, "enter start session") {
		t.Fatalf("with codex: want the issue and a start hint: %q", out)
	}
	v.SetIdle("mg-1", "codex is not on PATH")
	if out := v.View(); strings.Contains(out, "enter start") || !strings.Contains(out, "Cannot start one: codex is not on PATH") {
		t.Fatalf("without codex: want no start hint: %q", out)
	}
}

func TestViewWithEntriesIncludesTitles(t *testing.T) {
	state := &CodexTranscriptState{
		IssueID: "bd-1",
		Status:  "running",
	}
	_ = state.AppendEvent(mkEvent("agent_message", map[string]any{
		"message": "hello world",
	}))
	v := NewCodexTranscript(80, 24)
	v.SetState(state)
	out := v.View()
	if !strings.Contains(out, "hello world") {
		t.Fatalf("missing agent message: %q", out)
	}
}
