package codexmcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeServer is a long-lived JSON-RPC peer for tests. A single goroutine reads
// inbound requests and forwards them to the Incoming channel; tests respond
// via Respond / SendEvent which encode onto the writer side.
type fakeServer struct {
	dec *json.Decoder
	enc *json.Encoder

	Incoming chan request
	// Raw mirrors Incoming as the undecoded JSON bytes, so tests can inspect
	// fields request doesn't model (e.g. a response's result, or a string id
	// that won't fit request.ID int). Best-effort, non-blocking.
	Raw chan json.RawMessage

	wMu     sync.Mutex
	stopped bool

	stopOnce sync.Once
	stop     chan struct{}
}

func newFakeServer(t *testing.T) (*Client, *fakeServer) {
	t.Helper()
	transport, sr, sw := newPipePair()
	fs := &fakeServer{
		dec:      json.NewDecoder(bufio.NewReader(sr)),
		enc:      json.NewEncoder(sw),
		Incoming: make(chan request, 16),
		Raw:      make(chan json.RawMessage, 16),
		stop:     make(chan struct{}),
	}

	go fs.readLoop(sr, sw)

	// Pre-arm: the handshake fires the initialize request and the
	// notifications/initialized message. Auto-respond to initialize.
	handshakeDone := make(chan struct{})
	go func() {
		defer close(handshakeDone)
		for req := range fs.Incoming {
			if req.Method == methodInitialize {
				fs.Respond(req.ID, json.RawMessage(`{
					"protocolVersion":"2025-03-26",
					"capabilities":{"tools":{"listChanged":true}},
					"serverInfo":{"name":"fake","version":"0.1"}
				}`))
				return
			}
			// Drain anything else — shouldn't happen pre-init.
		}
	}()

	c, err := Dial(context.Background(), transport)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	select {
	case <-handshakeDone:
	case <-time.After(2 * time.Second):
		t.Fatal("handshake timeout")
	}
	// notifications/initialized is a fire-and-forget; ensure it arrived but
	// don't require a response.
	select {
	case n := <-fs.Incoming:
		if n.Method != methodNotifyInitialized {
			t.Fatalf("expected notifications/initialized, got %q", n.Method)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("notifications/initialized not received")
	}

	t.Cleanup(func() {
		_ = c.Close()
		fs.Stop()
	})
	return c, fs
}

func (f *fakeServer) readLoop(sr, sw interface{ Close() error }) {
	defer func() {
		_ = sr.Close()
		_ = sw.Close()
		close(f.Incoming)
	}()
	for {
		var raw json.RawMessage
		if err := f.dec.Decode(&raw); err != nil {
			return
		}
		// Mirror raw bytes (best-effort) before lossy decode into request.
		select {
		case f.Raw <- raw:
		default:
		}
		var req request
		_ = json.Unmarshal(raw, &req) // ignore errors: string ids / responses decode partially
		select {
		case <-f.stop:
			return
		case f.Incoming <- req:
		}
	}
}

func (f *fakeServer) Stop() {
	f.stopOnce.Do(func() {
		f.wMu.Lock()
		f.stopped = true
		f.wMu.Unlock()
		close(f.stop)
	})
}

func (f *fakeServer) Respond(id int, result json.RawMessage) {
	f.wMu.Lock()
	defer f.wMu.Unlock()
	if f.stopped {
		return
	}
	_ = f.enc.Encode(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"result":  result,
	})
}

func (f *fakeServer) RespondError(id, code int, msg string) {
	f.wMu.Lock()
	defer f.wMu.Unlock()
	if f.stopped {
		return
	}
	_ = f.enc.Encode(map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"error":   map[string]any{"code": code, "message": msg},
	})
}

func (f *fakeServer) SendEvent(reqID int, threadID, msg string) {
	f.wMu.Lock()
	defer f.wMu.Unlock()
	if f.stopped {
		return
	}
	_ = f.enc.Encode(map[string]any{
		"jsonrpc": "2.0",
		"method":  methodCodexEvent,
		"params": map[string]any{
			"_meta": map[string]any{"requestId": reqID, "threadId": threadID},
			"id":    "",
			"msg":   json.RawMessage(msg),
		},
	})
}

func (f *fakeServer) SendRaw(v any) {
	f.wMu.Lock()
	defer f.wMu.Unlock()
	if f.stopped {
		return
	}
	_ = f.enc.Encode(v)
}

func (f *fakeServer) Expect(t *testing.T, method string, timeout time.Duration) request {
	t.Helper()
	select {
	case req := <-f.Incoming:
		if req.Method != method {
			t.Fatalf("expected method %q, got %q", method, req.Method)
		}
		return req
	case <-time.After(timeout):
		t.Fatalf("timed out waiting for %s", method)
		return request{}
	}
}

func TestDialPerformsInitialize(t *testing.T) {
	c, _ := newFakeServer(t)
	if c == nil {
		t.Fatal("expected client")
	}
}

func TestCallSuccess(t *testing.T) {
	c, fs := newFakeServer(t)
	go func() {
		req := fs.Expect(t, "ping", 2*time.Second)
		fs.Respond(req.ID, json.RawMessage(`{"ok":true}`))
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	res, err := c.Call(ctx, "ping", nil)
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if !strings.Contains(string(res), "\"ok\":true") {
		t.Fatalf("unexpected result %s", res)
	}
}

func TestCallError(t *testing.T) {
	c, fs := newFakeServer(t)
	go func() {
		req := fs.Expect(t, "bogus", 2*time.Second)
		fs.RespondError(req.ID, -32601, "method not found")
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := c.Call(ctx, "bogus", nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "method not found") {
		t.Fatalf("error message lost: %v", err)
	}
}

func TestCallContextCancel(t *testing.T) {
	c, fs := newFakeServer(t)
	// Drain the request so writes don't backpressure.
	go func() { _ = fs.Expect(t, "hang", 2*time.Second) }()

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()
	_, err := c.Call(ctx, "hang", nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected Canceled, got %v", err)
	}
}

func TestEventsRoutedToOwningSession(t *testing.T) {
	c, fs := newFakeServer(t)
	// StartSession takes requestID 1 from the shared counter, so an event
	// stamped with requestId 1 belongs to it.
	sess, err := c.StartSession(context.Background(), SessionOptions{Prompt: "hi"})
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}
	go fs.SendEvent(sess.reqID, "thread-A", `{"type":"agent_message","message":"hello"}`)
	select {
	case ev := <-sess.Events():
		if ev.Meta.RequestID != sess.reqID || ev.Meta.ThreadID != "thread-A" {
			t.Fatalf("bad meta: %+v", ev.Meta)
		}
		if ev.EventType() != "agent_message" {
			t.Fatalf("bad event type: %q", ev.EventType())
		}
		var am AgentMessageEvent
		if err := json.Unmarshal(ev.Msg, &am); err != nil {
			t.Fatalf("decode agent_message: %v", err)
		}
		if am.Message != "hello" {
			t.Fatalf("bad message: %q", am.Message)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for event")
	}
}

// TestEventsNotDeliveredToForeignSession pins the routing guarantee that fixes
// the TestReplyRotatesSession flake: an event addressed to one session must
// never be consumed by another. Sessions previously each demuxed the same
// shared channel and silently discarded what did not match, so a session still
// winding down could swallow the next session's events.
func TestEventsNotDeliveredToForeignSession(t *testing.T) {
	c, fs := newFakeServer(t)
	first, err := c.StartSession(context.Background(), SessionOptions{Prompt: "one"})
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}
	second, err := c.StartSession(context.Background(), SessionOptions{Prompt: "two"})
	if err != nil {
		t.Fatalf("StartSession 2: %v", err)
	}
	// Address the event to the SECOND session while the first is still live.
	go fs.SendEvent(second.reqID, "thread-B", `{"type":"agent_message","message":"for-second"}`)

	select {
	case ev := <-second.Events():
		if ev.Meta.RequestID != second.reqID {
			t.Fatalf("second session got requestID %d, want %d", ev.Meta.RequestID, second.reqID)
		}
	case ev := <-first.Events():
		t.Fatalf("first session swallowed an event addressed to the second: %+v", ev.Meta)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout — event reached neither session")
	}
}

func TestUnknownNotificationIsDropped(t *testing.T) {
	c, fs := newFakeServer(t)
	sess, err := c.StartSession(context.Background(), SessionOptions{Prompt: "hi"})
	if err != nil {
		t.Fatalf("StartSession: %v", err)
	}
	fs.SendRaw(map[string]any{
		"jsonrpc": "2.0",
		"method":  "notifications/cancelled",
		"params":  map[string]any{"requestId": sess.reqID},
	})
	fs.SendEvent(sess.reqID, "t", `{"type":"task_started"}`)
	select {
	case ev := <-sess.Events():
		if ev.EventType() != "task_started" {
			t.Fatalf("got %q — was the unknown notification leaked?", ev.EventType())
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout")
	}
}

func TestCloseIsIdempotentAndUnblocksCallers(t *testing.T) {
	c, _ := newFakeServer(t)
	go func() {
		time.Sleep(20 * time.Millisecond)
		_ = c.Close()
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	_, err := c.Call(ctx, "hang", nil)
	if err == nil {
		t.Fatal("expected error after Close")
	}
	if err := c.Close(); err != nil {
		t.Fatalf("double close: %v", err)
	}
}

func TestServerInitiatedRequestRoutedToChannel(t *testing.T) {
	c, fs := newFakeServer(t)

	fs.SendRaw(map[string]any{
		"jsonrpc": "2.0",
		"id":      17,
		"method":  "elicitation/create",
		"params": map[string]any{
			"message":           "Allow Codex to run `ls -la`?",
			"codex_elicitation": "exec-approval",
			"codex_command":     []string{"ls", "-la"},
			"codex_cwd":         "/work/proj",
		},
	})

	select {
	case sr := <-c.ServerRequests():
		if sr.Method != "elicitation/create" {
			t.Fatalf("method = %q, want elicitation/create", sr.Method)
		}
		var id int
		if err := json.Unmarshal(sr.RawID, &id); err != nil || id != 17 {
			t.Fatalf("RawID = %s (err %v), want 17", sr.RawID, err)
		}
		a, ok := ParseElicitApproval(sr.Params)
		if !ok || a.Kind != "exec" {
			t.Fatalf("ParseElicitApproval ok=%v kind=%q, want true/exec", ok, a.Kind)
		}
		if strings.Join(a.Command, " ") != "ls -la" {
			t.Fatalf("command = %v, want [ls -la]", a.Command)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("server request not routed to ServerRequests()")
	}
}

// TestServerRequestStringIDSurvivesAndEchoes guards the id-hardening: a string id
// (allowed by the MCP spec) must not kill readLoop, and Respond must echo it back
// verbatim rather than coercing to an int.
func TestServerRequestStringIDSurvivesAndEchoes(t *testing.T) {
	c, fs := newFakeServer(t)

	fs.SendRaw(map[string]any{
		"jsonrpc": "2.0",
		"id":      "abc-1",
		"method":  "elicitation/create",
		"params": map[string]any{
			"message":           "Allow Codex to apply a patch?",
			"codex_elicitation": "patch-approval",
			"codex_changes": map[string]any{
				"internal/foo/bar.go": map[string]any{"type": "add", "content": "x"},
			},
		},
	})

	var sr ServerRequest
	select {
	case sr = <-c.ServerRequests():
	case <-time.After(2 * time.Second):
		t.Fatal("string-id server request not routed")
	}
	if string(sr.RawID) != `"abc-1"` {
		t.Fatalf("RawID = %s, want \"abc-1\"", sr.RawID)
	}
	a, ok := ParseElicitApproval(sr.Params)
	if !ok || a.Kind != "patch" {
		t.Fatalf("ParseElicitApproval ok=%v kind=%q, want true/patch", ok, a.Kind)
	}
	if _, has := a.Changes["internal/foo/bar.go"]; !has {
		t.Fatalf("changes missing expected path: %v", a.Changes)
	}

	// Drain handshake bytes (initialize, notifications/initialized) buffered on
	// Raw so the next read is mg's response.
	for draining := true; draining; {
		select {
		case <-fs.Raw:
		default:
			draining = false
		}
	}

	// Respond and confirm the server sees the string id echoed verbatim.
	if err := c.Respond(sr.RawID, map[string]any{"decision": "denied"}); err != nil {
		t.Fatalf("Respond: %v", err)
	}
	select {
	case raw := <-fs.Raw:
		var got struct {
			ID     json.RawMessage `json:"id"`
			Result struct {
				Decision string `json:"decision"`
			} `json:"result"`
		}
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatalf("decode response: %v (raw %s)", err, raw)
		}
		if string(got.ID) != `"abc-1"` {
			t.Fatalf("echoed id = %s, want \"abc-1\"", got.ID)
		}
		if got.Result.Decision != "denied" {
			t.Fatalf("decision = %q, want denied", got.Result.Decision)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Respond produced no output")
	}

	if err := c.ReadError(); err != nil {
		t.Fatalf("readLoop died on string id: %v", err)
	}
}
