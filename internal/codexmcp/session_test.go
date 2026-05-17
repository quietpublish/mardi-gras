package codexmcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestStartSessionRequiresPrompt(t *testing.T) {
	c, _ := newFakeServer(t)
	_, err := c.StartSession(context.Background(), SessionOptions{})
	if err == nil {
		t.Fatal("expected error for empty prompt")
	}
}

func TestStartSessionForwardsArgs(t *testing.T) {
	c, fs := newFakeServer(t)

	opts := SessionOptions{
		Prompt:         "do the thing",
		Cwd:            "/tmp/x",
		Sandbox:        "workspace-write",
		ApprovalPolicy: "on-request",
		Model:          "gpt-5.2",
	}
	go func() {
		_, err := c.StartSession(context.Background(), opts)
		if err != nil {
			t.Errorf("StartSession: %v", err)
		}
	}()
	req := fs.Expect(t, methodToolsCall, 2*time.Second)

	var p toolsCallParams
	if err := json.Unmarshal(req.Params, &p); err != nil {
		t.Fatalf("decode params: %v", err)
	}
	if p.Name != codexToolName {
		t.Fatalf("tool name: %q", p.Name)
	}
	want := map[string]string{
		"prompt":          "do the thing",
		"cwd":             "/tmp/x",
		"sandbox":         "workspace-write",
		"approval-policy": "on-request",
		"model":           "gpt-5.2",
	}
	for k, v := range want {
		got, _ := p.Arguments[k].(string)
		if got != v {
			t.Errorf("arg %s = %q, want %q", k, got, v)
		}
	}
	// Respond so the caller goroutine completes.
	fs.Respond(req.ID, json.RawMessage(`{"structuredContent":{"threadId":"t1","content":"done"}}`))
}

func TestSessionFiltersEventsByRequestID(t *testing.T) {
	c, fs := newFakeServer(t)
	sess, err := c.StartSession(context.Background(), SessionOptions{Prompt: "p"})
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}
	req := fs.Expect(t, methodToolsCall, 2*time.Second)
	// Wrong requestID — should be filtered out.
	fs.SendEvent(999, "tx", `{"type":"agent_message","message":"unrelated"}`)
	// Right requestID.
	fs.SendEvent(req.ID, "t1", `{"type":"agent_message","message":"mine"}`)
	select {
	case ev := <-sess.Events():
		if ev.Meta.RequestID != req.ID {
			t.Fatalf("got event for foreign request %d", ev.Meta.RequestID)
		}
		var am AgentMessageEvent
		_ = json.Unmarshal(ev.Msg, &am)
		if am.Message != "mine" {
			t.Fatalf("got %q want \"mine\"", am.Message)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout")
	}
	if sess.ThreadID() != "t1" {
		t.Fatalf("threadID = %q, want t1", sess.ThreadID())
	}
	// Drain so the session terminates cleanly.
	fs.Respond(req.ID, json.RawMessage(`{"structuredContent":{"threadId":"t1","content":"ok"}}`))
	_ = waitDone(t, sess)
}

func TestSessionDoneReceivesResult(t *testing.T) {
	c, fs := newFakeServer(t)
	sess, err := c.StartSession(context.Background(), SessionOptions{Prompt: "p"})
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}
	req := fs.Expect(t, methodToolsCall, 2*time.Second)
	fs.SendEvent(req.ID, "t-final", `{"type":"task_complete","last_agent_message":"hi"}`)
	fs.Respond(req.ID, json.RawMessage(`{"structuredContent":{"threadId":"t-final","content":"final answer"}}`))

	res := waitDone(t, sess)
	if res.Err != nil {
		t.Fatalf("Done.Err: %v", res.Err)
	}
	if res.Content != "final answer" {
		t.Fatalf("Content = %q", res.Content)
	}
	if res.ThreadID != "t-final" {
		t.Fatalf("ThreadID = %q", res.ThreadID)
	}
}

func TestSessionDoneReceivesError(t *testing.T) {
	c, fs := newFakeServer(t)
	sess, err := c.StartSession(context.Background(), SessionOptions{Prompt: "p"})
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}
	req := fs.Expect(t, methodToolsCall, 2*time.Second)
	fs.RespondError(req.ID, -32000, "rate limited")
	res := waitDone(t, sess)
	if res.Err == nil || !strings.Contains(res.Err.Error(), "rate limited") {
		t.Fatalf("Err = %v", res.Err)
	}
}

func TestStartReplySessionRequiresPromptAndThread(t *testing.T) {
	c, _ := newFakeServer(t)
	if _, err := c.StartReplySession(context.Background(), "tid", ""); err == nil {
		t.Fatal("expected error for empty prompt")
	}
	if _, err := c.StartReplySession(context.Background(), "", "hi"); err == nil {
		t.Fatal("expected error for empty threadID")
	}
}

func TestStartReplySessionForwardsArgs(t *testing.T) {
	c, fs := newFakeServer(t)

	go func() {
		_, err := c.StartReplySession(context.Background(), "thr-1", "what about Y?")
		if err != nil {
			t.Errorf("StartReplySession: %v", err)
		}
	}()
	req := fs.Expect(t, methodToolsCall, 2*time.Second)

	var p toolsCallParams
	if err := json.Unmarshal(req.Params, &p); err != nil {
		t.Fatalf("decode params: %v", err)
	}
	if p.Name != codexReplyToolName {
		t.Fatalf("tool name: %q, want %q", p.Name, codexReplyToolName)
	}
	if got, _ := p.Arguments["threadId"].(string); got != "thr-1" {
		t.Errorf("threadId = %q", got)
	}
	if got, _ := p.Arguments["prompt"].(string); got != "what about Y?" {
		t.Errorf("prompt = %q", got)
	}
	if _, hasCwd := p.Arguments["cwd"]; hasCwd {
		t.Error("codex-reply args should not carry cwd — inherited from prior conversation")
	}
	fs.Respond(req.ID, json.RawMessage(`{"structuredContent":{"threadId":"thr-1","content":"continued"}}`))
}

func TestStartReplySessionStreamsEventsAndDone(t *testing.T) {
	c, fs := newFakeServer(t)
	sess, err := c.StartReplySession(context.Background(), "thr-2", "follow up")
	if err != nil {
		t.Fatalf("StartReplySession: %v", err)
	}
	req := fs.Expect(t, methodToolsCall, 2*time.Second)
	fs.SendEvent(req.ID, "thr-2", `{"type":"agent_message","message":"the reply"}`)
	fs.Respond(req.ID, json.RawMessage(`{"structuredContent":{"threadId":"thr-2","content":"the reply"}}`))

	select {
	case ev, ok := <-sess.Events():
		if !ok {
			t.Fatal("events closed early")
		}
		var am AgentMessageEvent
		_ = json.Unmarshal(ev.Msg, &am)
		if am.Message != "the reply" {
			t.Fatalf("message = %q", am.Message)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no event")
	}
	res := waitDone(t, sess)
	if res.Err != nil {
		t.Fatalf("Done.Err: %v", res.Err)
	}
	if res.ThreadID != "thr-2" {
		t.Fatalf("ThreadID = %q", res.ThreadID)
	}
}

func TestSessionCancelStopsBeforeResponse(t *testing.T) {
	c, fs := newFakeServer(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sess, err := c.StartSession(ctx, SessionOptions{Prompt: "p"})
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}
	_ = fs.Expect(t, methodToolsCall, 2*time.Second)
	sess.Cancel()
	res := waitDone(t, sess)
	if res.Err == nil {
		t.Fatal("expected cancellation error")
	}
}

func waitDone(t *testing.T, sess *Session) SessionResult {
	t.Helper()
	select {
	case res := <-sess.Done():
		return res
	case <-time.After(3 * time.Second):
		t.Fatal("session.Done timeout")
		return SessionResult{}
	}
}
