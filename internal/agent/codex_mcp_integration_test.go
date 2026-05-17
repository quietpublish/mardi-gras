//go:build integration

package agent

import (
	"context"
	"encoding/json"
	"os/exec"
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
