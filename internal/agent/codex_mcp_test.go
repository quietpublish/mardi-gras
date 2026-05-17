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
