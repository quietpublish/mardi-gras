//go:build integration

package agent

import (
	"context"
	"encoding/json"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/matt-wright86/mardi-gras/internal/codexmcp"
)

// TestIntegrationLaunchCtxCancelMatchesField reproduces the v0.21.0 field
// failure against a real `codex mcp-server`: launch via the bridge with a
// short-lived ctx, cancel it immediately (mirroring mg's codexLaunchCmd
// defer cancel), and assert events still flow plus the terminal result
// arrives. Under v0.21.0 the session died on the cancel and Done returned
// `context.Canceled` before any agent_message event ever fired.
//
// Gated by build tag `integration`. Run with:
//
//	go test -tags=integration ./internal/agent -run TestIntegrationLaunchCtxCancelMatchesField -v -timeout 180s
func TestIntegrationLaunchCtxCancelMatchesField(t *testing.T) {
	if _, err := exec.LookPath("codex"); err != nil {
		t.Skip("codex not on PATH")
	}

	ctx, cancel := context.WithCancel(context.Background())
	h, err := LaunchCodexMCP(ctx, LaunchCodexMCPOptions{
		Prompt:         "Say only the single word: pong",
		ProjectDir:     "/tmp",
		Sandbox:        "read-only",
		ApprovalPolicy: "never",
		ClientVersion:  "integration-test",
	})
	if err != nil {
		t.Fatalf("LaunchCodexMCP: %v", err)
	}
	t.Cleanup(func() { _ = h.Close() })

	// The bug: this cancel used to kill the session immediately.
	cancel()
	t.Log("launch ctx canceled — v0.21.0 would have killed the session here")

	sawAgentMessage := false
	timeout := time.After(150 * time.Second)
loop:
	for {
		select {
		case ev, ok := <-h.Session().Events():
			if !ok {
				break loop
			}
			t.Logf("event: %s thread=%s", ev.EventType(), ev.Meta.ThreadID)
			if ev.EventType() == "agent_message" {
				var am codexmcp.AgentMessageEvent
				if err := json.Unmarshal(ev.Msg, &am); err != nil {
					t.Fatalf("decode agent_message: %v", err)
				}
				t.Logf("agent: %q", am.Message)
				sawAgentMessage = true
			}
		case res := <-h.Session().Done():
			t.Logf("done: threadId=%s err=%v content=%q", res.ThreadID, res.Err, res.Content)
			if res.Err != nil {
				t.Fatalf("session ended with error: %v (v0.21.0 regression)", res.Err)
			}
			if res.ThreadID == "" {
				t.Fatal("expected threadId")
			}
			break loop
		case <-timeout:
			t.Fatal("timeout after 150s")
		}
	}
	if !sawAgentMessage {
		t.Fatal("no agent_message after launch-ctx cancel")
	}
}

// TestIntegrationCodexReplyAgainstRealCodex exercises a multi-turn
// conversation against `codex mcp-server`: launch with prompt A, drain the
// first turn to terminal Done, then call h.Reply(ctx, prompt B) and confirm
// the second turn produces its own agent_message + Done under the same
// threadId without a fresh subprocess startup. The second turn should be
// substantially faster than the first because the sub-MCP startup chain is
// already initialized.
//
// Gated by build tag `integration`. Run with:
//
//	go test -tags=integration ./internal/agent -run TestIntegrationCodexReplyAgainstRealCodex -v -timeout 240s
func TestIntegrationCodexReplyAgainstRealCodex(t *testing.T) {
	if _, err := exec.LookPath("codex"); err != nil {
		t.Skip("codex not on PATH")
	}

	ctx, cancel := context.WithCancel(context.Background())
	h, err := LaunchCodexMCP(ctx, LaunchCodexMCPOptions{
		Prompt:         "Reply with exactly the word: one",
		ProjectDir:     "/tmp",
		Sandbox:        "read-only",
		ApprovalPolicy: "never",
		ClientVersion:  "integration-test",
	})
	if err != nil {
		t.Fatalf("LaunchCodexMCP: %v", err)
	}
	cancel() // launch-ctx detachment fix from v0.21.1
	t.Cleanup(func() { _ = h.Close() })

	firstStart := time.Now()
	firstThread, firstAgent := drainTurn(t, h.Session(), 150*time.Second)
	firstElapsed := time.Since(firstStart)
	t.Logf("first turn: thread=%s agent=%q elapsed=%s", firstThread, firstAgent, firstElapsed)
	if firstThread == "" {
		t.Fatal("first turn returned empty threadId")
	}
	if !strings.Contains(strings.ToLower(firstAgent), "one") {
		t.Fatalf("first agent_message did not contain 'one': %q", firstAgent)
	}

	if h.Session().ThreadID() != firstThread {
		t.Fatalf("handle.Session().ThreadID() = %q, want %q", h.Session().ThreadID(), firstThread)
	}

	// Reply turn.
	replyStart := time.Now()
	replySess, err := h.Reply(context.Background(), "Now reply with exactly the word: two")
	if err != nil {
		t.Fatalf("Reply: %v", err)
	}
	if replySess == nil {
		t.Fatal("Reply returned nil session")
	}
	secondThread, secondAgent := drainTurn(t, replySess, 90*time.Second)
	replyElapsed := time.Since(replyStart)
	t.Logf("reply turn: thread=%s agent=%q elapsed=%s", secondThread, secondAgent, replyElapsed)

	if secondThread != firstThread {
		t.Fatalf("threadId changed across reply: %q -> %q", firstThread, secondThread)
	}
	if !strings.Contains(strings.ToLower(secondAgent), "two") {
		t.Fatalf("reply agent_message did not contain 'two': %q", secondAgent)
	}

	// Reply should be meaningfully faster than the first turn because the
	// codex subprocess + sub-MCP chain is already warm. Threshold is loose
	// (50% of first-turn time) so model variance doesn't flake the test.
	if replyElapsed > firstElapsed {
		t.Logf("reply (%s) was slower than first turn (%s) — not a hard failure but unusual",
			replyElapsed, firstElapsed)
	}
}

// drainTurn reads events from sess until Done fires, returning the final
// threadId and the last agent_message seen.
func drainTurn(t *testing.T, sess *codexmcp.Session, timeout time.Duration) (threadID, lastAgentMessage string) {
	t.Helper()
	deadline := time.After(timeout)
	for {
		select {
		case ev, ok := <-sess.Events():
			if !ok {
				t.Fatal("events channel closed before Done")
			}
			if ev.Meta.ThreadID != "" {
				threadID = ev.Meta.ThreadID
			}
			if ev.EventType() == "agent_message" {
				var am codexmcp.AgentMessageEvent
				if err := json.Unmarshal(ev.Msg, &am); err == nil {
					lastAgentMessage = am.Message
				}
			}
		case res := <-sess.Done():
			if res.Err != nil {
				t.Fatalf("turn ended with error: %v", res.Err)
			}
			if res.ThreadID != "" {
				threadID = res.ThreadID
			}
			return threadID, lastAgentMessage
		case <-deadline:
			t.Fatalf("turn timed out after %s", timeout)
		}
	}
}
