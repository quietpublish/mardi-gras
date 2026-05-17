package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/matt-wright86/mardi-gras/internal/codexmcp"
)

func TestLaunchCodexMCPRequiresPrompt(t *testing.T) {
	_, err := LaunchCodexMCP(context.Background(), LaunchCodexMCPOptions{})
	if err == nil {
		t.Fatal("expected error for empty prompt")
	}
}

// pipeTransport is a Transport backed by io.Pipe pairs for bridge tests.
type pipeTransport struct {
	clientRead  *io.PipeReader
	clientWrite *io.PipeWriter
}

func (p *pipeTransport) Reader() io.Reader { return p.clientRead }
func (p *pipeTransport) Writer() io.Writer { return p.clientWrite }
func (p *pipeTransport) Close() error {
	_ = p.clientWrite.Close()
	_ = p.clientRead.Close()
	return nil
}

// fakeMCPServer is a minimal MCP peer driven by a goroutine. It auto-responds
// to initialize, observes notifications/initialized, then expects exactly one
// tools/call and replies with a canned result.
type fakeMCPServer struct {
	dec    *json.Decoder
	enc    *json.Encoder
	wMu    sync.Mutex
	closed chan struct{}
}

func newFakePipe(t *testing.T) (codexmcp.Transport, *codexmcp.SubprocessTransport, *fakeMCPServer) {
	t.Helper()
	cr, sw := io.Pipe()
	sr, cw := io.Pipe()
	transport := &pipeTransport{clientRead: cr, clientWrite: cw}
	fs := &fakeMCPServer{
		dec:    json.NewDecoder(bufio.NewReader(sr)),
		enc:    json.NewEncoder(sw),
		closed: make(chan struct{}),
	}
	go fs.run(t, sr, sw)
	return transport, nil, fs
}

// runHandshake processes the MCP initialize request and the
// notifications/initialized notification, leaving the server ready to
// accept tools/call requests. Returns false if the handshake stream is
// interrupted (closed pipe). Used by both the canned bridge harness
// (`run`) and tests that need to drive their own custom tools/call flow
// after the handshake.
func (f *fakeMCPServer) runHandshake() bool {
	var init map[string]any
	if err := f.dec.Decode(&init); err != nil {
		return false
	}
	id, _ := init["id"].(float64)
	f.respond(int(id), map[string]any{
		"protocolVersion": "2025-03-26",
		"capabilities":    map[string]any{"tools": map[string]any{"listChanged": true}},
		"serverInfo":      map[string]any{"name": "fake", "version": "0.1"},
	})
	var n map[string]any
	return f.dec.Decode(&n) == nil
}

func (f *fakeMCPServer) run(t *testing.T, sr, sw io.Closer) {
	t.Helper()
	defer func() {
		_ = sr.Close()
		_ = sw.Close()
		close(f.closed)
	}()
	if !f.runHandshake() {
		return
	}
	// tools/call
	var call map[string]any
	if err := f.dec.Decode(&call); err != nil {
		return
	}
	callID, _ := call["id"].(float64)
	// Send a single agent_message event then resolve.
	f.notify("codex/event", map[string]any{
		"_meta": map[string]any{"requestId": int(callID), "threadId": "thread-bridge"},
		"id":    "1",
		"msg":   map[string]any{"type": "agent_message", "message": "from-bridge"},
	})
	f.respond(int(callID), map[string]any{
		"structuredContent": map[string]any{"threadId": "thread-bridge", "content": "OK"},
	})
	// Keep loop alive draining further inbound until close.
	for {
		var m map[string]any
		if err := f.dec.Decode(&m); err != nil {
			return
		}
	}
}

func (f *fakeMCPServer) respond(id int, result any) {
	f.wMu.Lock()
	defer f.wMu.Unlock()
	_ = f.enc.Encode(map[string]any{"jsonrpc": "2.0", "id": id, "result": result})
}

func (f *fakeMCPServer) notify(method string, params any) {
	f.wMu.Lock()
	defer f.wMu.Unlock()
	_ = f.enc.Encode(map[string]any{"jsonrpc": "2.0", "method": method, "params": params})
}

// withFakeCodexTransport swaps the package-level codexTransportFactory for a
// pipe-backed fake and restores it on cleanup. Shared by every bridge test.
func withFakeCodexTransport(t *testing.T) {
	t.Helper()
	prev := codexTransportFactory
	codexTransportFactory = func(LaunchCodexMCPOptions) (codexmcp.Transport, *codexmcp.SubprocessTransport, error) {
		tp, sp, _ := newFakePipe(t)
		return tp, sp, nil
	}
	t.Cleanup(func() { codexTransportFactory = prev })
}

func TestLaunchCodexMCPBridgesEventsAndDone(t *testing.T) {
	withFakeCodexTransport(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	h, err := LaunchCodexMCP(ctx, LaunchCodexMCPOptions{
		Prompt:     "do thing",
		ProjectDir: "/tmp",
	})
	if err != nil {
		t.Fatalf("LaunchCodexMCP: %v", err)
	}
	t.Cleanup(func() { _ = h.Close() })

	select {
	case ev, ok := <-h.Session().Events():
		if !ok {
			t.Fatal("events channel closed before agent_message arrived")
		}
		if ev.EventType() != "agent_message" {
			t.Fatalf("unexpected event type %q", ev.EventType())
		}
		var am codexmcp.AgentMessageEvent
		if err := json.Unmarshal(ev.Msg, &am); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if am.Message != "from-bridge" {
			t.Fatalf("message = %q", am.Message)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("event channel timeout")
	}

	select {
	case res := <-h.Session().Done():
		if res.Err != nil {
			t.Fatalf("Done.Err: %v", res.Err)
		}
		if res.ThreadID != "thread-bridge" {
			t.Fatalf("ThreadID = %q", res.ThreadID)
		}
		if res.Content != "OK" {
			t.Fatalf("Content = %q", res.Content)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Done timeout")
	}
}

func TestLaunchCodexMCPReportsTransportError(t *testing.T) {
	prev := codexTransportFactory
	codexTransportFactory = func(opts LaunchCodexMCPOptions) (codexmcp.Transport, *codexmcp.SubprocessTransport, error) {
		return nil, nil, ErrCodexUnavailable
	}
	t.Cleanup(func() { codexTransportFactory = prev })

	_, err := LaunchCodexMCP(context.Background(), LaunchCodexMCPOptions{Prompt: "x"})
	if !errors.Is(err, ErrCodexUnavailable) {
		t.Fatalf("expected ErrCodexUnavailable, got %v", err)
	}
}

// TestLaunchCtxDoesNotKillSession asserts the session outlives the launch
// context. Without this guarantee, mg's codexLaunchCmd defer-cancel would
// race awaitResponse's <-ctx.Done() arm and push a context.Canceled result
// onto Done before any event is rendered.
func TestLaunchCtxDoesNotKillSession(t *testing.T) {
	withFakeCodexTransport(t)

	ctx, cancel := context.WithCancel(context.Background())
	h, err := LaunchCodexMCP(ctx, LaunchCodexMCPOptions{
		Prompt:     "do thing",
		ProjectDir: "/tmp",
	})
	if err != nil {
		t.Fatalf("LaunchCodexMCP: %v", err)
	}
	t.Cleanup(func() { _ = h.Close() })

	cancel()

	select {
	case ev, ok := <-h.Session().Events():
		if !ok {
			t.Fatal("events channel closed before event arrived — launch ctx still killing session")
		}
		if ev.EventType() != "agent_message" {
			t.Fatalf("unexpected event type %q", ev.EventType())
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no event after 2s — launch ctx still killing session")
	}

	select {
	case res := <-h.Session().Done():
		if res.Err != nil {
			t.Fatalf("session ended with error: %v", res.Err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Done not delivered")
	}
}

func TestReplyRotatesSession(t *testing.T) {
	// Build a fake-MCP transport that:
	//   1. Auto-responds to initialize
	//   2. Handles the first tools/call (codex) with a session_configured
	//      event so the session captures a threadID, then resolves it.
	//   3. Handles the second tools/call (codex-reply) by echoing the
	//      threadId arg back as an agent_message and resolving.
	// Drives Reply and asserts the rotated session yields the expected
	// agent_message.
	cr, sw := io.Pipe()
	sr, cw := io.Pipe()
	transport := &pipeTransport{clientRead: cr, clientWrite: cw}

	fs := &fakeMCPServer{
		dec:    json.NewDecoder(bufio.NewReader(sr)),
		enc:    json.NewEncoder(sw),
		closed: make(chan struct{}),
	}

	go func() {
		defer func() {
			_ = sr.Close()
			_ = sw.Close()
			close(fs.closed)
		}()
		if !fs.runHandshake() {
			return
		}
		// First tools/call (codex)
		var call1 map[string]any
		if err := fs.dec.Decode(&call1); err != nil {
			return
		}
		id1, _ := call1["id"].(float64)
		// Emit session_configured so the session captures the threadId.
		fs.notify("codex/event", map[string]any{
			"_meta": map[string]any{"requestId": int(id1), "threadId": "thr-rot"},
			"id":    "1",
			"msg":   map[string]any{"type": "session_configured", "thread_id": "thr-rot", "model": "gpt-5-mini"},
		})
		fs.respond(int(id1), map[string]any{
			"structuredContent": map[string]any{"threadId": "thr-rot", "content": "initial"},
		})
		// Second tools/call (codex-reply)
		var call2 map[string]any
		if err := fs.dec.Decode(&call2); err != nil {
			return
		}
		id2, _ := call2["id"].(float64)
		params := call2["params"].(map[string]any)
		args := params["arguments"].(map[string]any)
		thread := args["threadId"].(string)
		fs.notify("codex/event", map[string]any{
			"_meta": map[string]any{"requestId": int(id2), "threadId": thread},
			"id":    "2",
			"msg":   map[string]any{"type": "agent_message", "message": "reply for " + thread},
		})
		fs.respond(int(id2), map[string]any{
			"structuredContent": map[string]any{"threadId": thread, "content": "reply-done"},
		})
		// Keep draining.
		for {
			var m map[string]any
			if err := fs.dec.Decode(&m); err != nil {
				return
			}
		}
	}()

	prev := codexTransportFactory
	codexTransportFactory = func(opts LaunchCodexMCPOptions) (codexmcp.Transport, *codexmcp.SubprocessTransport, error) {
		return transport, nil, nil
	}
	t.Cleanup(func() { codexTransportFactory = prev })

	h, err := LaunchCodexMCP(context.Background(), LaunchCodexMCPOptions{Prompt: "initial"})
	if err != nil {
		t.Fatalf("LaunchCodexMCP: %v", err)
	}
	t.Cleanup(func() { _ = h.Close() })

	// Drain the first session to terminal Done so ThreadID is populated.
	origSession := h.Session()
	for {
		select {
		case ev, ok := <-origSession.Events():
			if !ok {
				goto firstDone
			}
			_ = ev
		case res := <-origSession.Done():
			if res.Err != nil {
				t.Fatalf("first session.Err: %v", res.Err)
			}
			goto firstDone
		case <-time.After(2 * time.Second):
			t.Fatal("first session never finished")
		}
	}
firstDone:
	if got := h.Session().ThreadID(); got != "thr-rot" {
		t.Fatalf("threadID = %q after first session", got)
	}

	replySess, err := h.Reply(context.Background(), "follow up")
	if err != nil {
		t.Fatalf("Reply: %v", err)
	}
	if h.Session() != replySess {
		t.Fatal("Reply did not rotate handle.session")
	}
	if h.Session() == origSession {
		t.Fatal("Reply returned the same session as the original")
	}

	// The new session should yield the agent_message event echoed back.
	select {
	case ev, ok := <-replySess.Events():
		if !ok {
			t.Fatal("reply events closed")
		}
		if ev.EventType() != "agent_message" {
			t.Fatalf("event type = %q", ev.EventType())
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no reply event")
	}
	res := <-replySess.Done()
	if res.Err != nil {
		t.Fatalf("reply Done.Err: %v", res.Err)
	}
}

// TestReplyCtxDoesNotKillSession asserts the reply session outlives the
// caller's ctx — the same trap that v0.21.1 fixed on the launch path.
// Without this guarantee, mg's codexReplyCmd defer-cancel would race
// awaitResponse's <-ctx.Done() arm and push a context.Canceled result onto
// Done before any reply event is rendered.
func TestReplyCtxDoesNotKillSession(t *testing.T) {
	cr, sw := io.Pipe()
	sr, cw := io.Pipe()
	transport := &pipeTransport{clientRead: cr, clientWrite: cw}

	fs := &fakeMCPServer{
		dec:    json.NewDecoder(bufio.NewReader(sr)),
		enc:    json.NewEncoder(sw),
		closed: make(chan struct{}),
	}

	go func() {
		defer func() {
			_ = sr.Close()
			_ = sw.Close()
			close(fs.closed)
		}()
		fs.runHandshake()
		// First tools/call: emit a session_configured to seed threadID,
		// then resolve.
		var call1 map[string]any
		if err := fs.dec.Decode(&call1); err != nil {
			return
		}
		id1, _ := call1["id"].(float64)
		fs.notify("codex/event", map[string]any{
			"_meta": map[string]any{"requestId": int(id1), "threadId": "thr-rctx"},
			"id":    "1",
			"msg":   map[string]any{"type": "session_configured", "thread_id": "thr-rctx"},
		})
		fs.respond(int(id1), map[string]any{
			"structuredContent": map[string]any{"threadId": "thr-rctx", "content": "ok"},
		})
		// Second tools/call (codex-reply): emit one event AFTER the
		// expected cancel point, then resolve.
		var call2 map[string]any
		if err := fs.dec.Decode(&call2); err != nil {
			return
		}
		id2, _ := call2["id"].(float64)
		time.Sleep(50 * time.Millisecond) // give the test goroutine time to defer-cancel its ctx
		fs.notify("codex/event", map[string]any{
			"_meta": map[string]any{"requestId": int(id2), "threadId": "thr-rctx"},
			"id":    "2",
			"msg":   map[string]any{"type": "agent_message", "message": "reply survived"},
		})
		fs.respond(int(id2), map[string]any{
			"structuredContent": map[string]any{"threadId": "thr-rctx", "content": "done"},
		})
		for {
			var m map[string]any
			if err := fs.dec.Decode(&m); err != nil {
				return
			}
		}
	}()

	prev := codexTransportFactory
	codexTransportFactory = func(opts LaunchCodexMCPOptions) (codexmcp.Transport, *codexmcp.SubprocessTransport, error) {
		return transport, nil, nil
	}
	t.Cleanup(func() { codexTransportFactory = prev })

	h, err := LaunchCodexMCP(context.Background(), LaunchCodexMCPOptions{Prompt: "initial"})
	if err != nil {
		t.Fatalf("LaunchCodexMCP: %v", err)
	}
	t.Cleanup(func() { _ = h.Close() })

	// Drain the first session to terminal Done so ThreadID is populated.
	orig := h.Session()
	for {
		select {
		case _, ok := <-orig.Events():
			if !ok {
				goto firstDone
			}
		case res := <-orig.Done():
			if res.Err != nil {
				t.Fatalf("first session.Err: %v", res.Err)
			}
			goto firstDone
		case <-time.After(2 * time.Second):
			t.Fatal("first session never finished")
		}
	}
firstDone:
	if h.Session().ThreadID() != "thr-rctx" {
		t.Fatal("threadID not populated")
	}

	// Call Reply with a short-lived ctx; cancel immediately to simulate
	// the codexReplyCmd defer-cancel that would kill the session before
	// the fix.
	replyCtx, cancel := context.WithCancel(context.Background())
	replySess, err := h.Reply(replyCtx, "follow up")
	if err != nil {
		t.Fatalf("Reply: %v", err)
	}
	cancel()

	select {
	case ev, ok := <-replySess.Events():
		if !ok {
			t.Fatal("reply events closed before event arrived — Reply still wired ctx to session")
		}
		if ev.EventType() != "agent_message" {
			t.Fatalf("unexpected event type %q", ev.EventType())
		}
	case <-time.After(3 * time.Second):
		t.Fatal("no reply event within 3s — Reply ctx leaked into session")
	}

	res := <-replySess.Done()
	if res.Err != nil {
		t.Fatalf("reply Done.Err: %v", res.Err)
	}
}

func TestCloseIsIdempotent(t *testing.T) {
	withFakeCodexTransport(t)

	h, err := LaunchCodexMCP(context.Background(), LaunchCodexMCPOptions{Prompt: "x"})
	if err != nil {
		t.Fatalf("LaunchCodexMCP: %v", err)
	}
	if err := h.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if err := h.Close(); err != nil {
		t.Fatalf("double close: %v", err)
	}
	if tail := h.StderrTail(5); len(tail) != 0 {
		// We supplied no subprocess; tail should be empty.
		if !strings.HasPrefix(tail[0], "") {
			t.Fatalf("tail = %v", tail)
		}
	}
}
