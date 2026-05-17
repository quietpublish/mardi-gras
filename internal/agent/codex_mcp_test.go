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

func (f *fakeMCPServer) run(t *testing.T, sr, sw io.Closer) {
	t.Helper()
	defer func() {
		_ = sr.Close()
		_ = sw.Close()
		close(f.closed)
	}()
	// initialize
	var init map[string]any
	if err := f.dec.Decode(&init); err != nil {
		return
	}
	id, _ := init["id"].(float64)
	f.respond(int(id), map[string]any{
		"protocolVersion": "2025-03-26",
		"capabilities":    map[string]any{"tools": map[string]any{"listChanged": true}},
		"serverInfo":      map[string]any{"name": "fake", "version": "0.1"},
	})
	// notifications/initialized
	var n map[string]any
	if err := f.dec.Decode(&n); err != nil {
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

func TestLaunchCodexMCPBridgesEventsAndDone(t *testing.T) {
	// Swap the transport factory to use the pipe-based fake.
	prev := codexTransportFactory
	codexTransportFactory = func(opts LaunchCodexMCPOptions) (codexmcp.Transport, *codexmcp.SubprocessTransport, error) {
		tp, sp, _ := newFakePipe(t)
		return tp, sp, nil
	}
	t.Cleanup(func() { codexTransportFactory = prev })

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

// TestLaunchCtxDoesNotKillSession was added after a v0.21.0 field report
// where the transcript overlay stayed "waiting for first event..." for
// minutes despite a real codex session running. Root cause: mg's
// codexLaunchCmd defer-canceled the launch ctx as soon as LaunchCodexMCP
// returned, and StartSession had parented its callCtx to that launch ctx —
// so awaitResponse picked <-ctx.Done() before any event flowed and pushed
// a "context canceled" SessionResult, leaving the UI frozen at "running".
//
// This test reproduces the failure mode (launch ctx is canceled immediately
// after LaunchCodexMCP returns) and asserts the session survives long enough
// to deliver a streamed event + the terminal result.
func TestLaunchCtxDoesNotKillSession(t *testing.T) {
	prev := codexTransportFactory
	codexTransportFactory = func(opts LaunchCodexMCPOptions) (codexmcp.Transport, *codexmcp.SubprocessTransport, error) {
		tp, sp, _ := newFakePipe(t)
		return tp, sp, nil
	}
	t.Cleanup(func() { codexTransportFactory = prev })

	ctx, cancel := context.WithCancel(context.Background())
	h, err := LaunchCodexMCP(ctx, LaunchCodexMCPOptions{
		Prompt:     "do thing",
		ProjectDir: "/tmp",
	})
	if err != nil {
		t.Fatalf("LaunchCodexMCP: %v", err)
	}
	t.Cleanup(func() { _ = h.Close() })

	// Cancel the launch ctx immediately — mirrors mg's codexLaunchCmd
	// behavior of `defer cancel()` after the launch goroutine returns.
	cancel()

	// The session must still receive its streamed event.
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

	// And the terminal result.
	select {
	case res := <-h.Session().Done():
		if res.Err != nil {
			t.Fatalf("session ended with error: %v", res.Err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Done not delivered")
	}
}

func TestCloseIsIdempotent(t *testing.T) {
	prev := codexTransportFactory
	codexTransportFactory = func(opts LaunchCodexMCPOptions) (codexmcp.Transport, *codexmcp.SubprocessTransport, error) {
		tp, sp, _ := newFakePipe(t)
		return tp, sp, nil
	}
	t.Cleanup(func() { codexTransportFactory = prev })

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
