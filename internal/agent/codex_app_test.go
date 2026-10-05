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

	"github.com/matt-wright86/mardi-gras/internal/codexapp"
)

func TestLaunchCodexAppRequiresPrompt(t *testing.T) {
	_, err := LaunchCodexApp(context.Background(), LaunchCodexAppOptions{})
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

// fakeAppServer is a minimal `codex app-server` peer. It answers initialize,
// thread/start and turn/start, and runs each turn as turn/started, one
// completed agentMessage ("reply: <prompt>") and turn/completed. reviewer is
// the approvalsReviewer it claims to apply; "" echoes the request's.
type fakeAppServer struct {
	dec      *json.Decoder
	enc      *json.Encoder
	wMu      sync.Mutex
	reviewer string

	mu    sync.Mutex
	turns int
}

func newFakeAppServer(t *testing.T, reviewer string) (*pipeTransport, *fakeAppServer) {
	t.Helper()
	cr, sw := io.Pipe()
	sr, cw := io.Pipe()
	fs := &fakeAppServer{
		dec:      json.NewDecoder(bufio.NewReader(sr)),
		enc:      json.NewEncoder(sw),
		reviewer: reviewer,
	}
	go fs.run(sr, sw)
	return &pipeTransport{clientRead: cr, clientWrite: cw}, fs
}

func (f *fakeAppServer) run(sr, sw io.Closer) {
	defer func() {
		_ = sr.Close()
		_ = sw.Close()
	}()
	for {
		var m map[string]any
		if err := f.dec.Decode(&m); err != nil {
			return
		}
		id, hasID := m["id"]
		params, _ := m["params"].(map[string]any)
		switch m["method"] {
		case "initialize":
			f.respond(id, map[string]any{"userAgent": "fake/0.160.0"})
		case "thread/start":
			reviewer := f.reviewer
			if reviewer == "" {
				reviewer, _ = params["approvalsReviewer"].(string)
			}
			f.respond(id, map[string]any{
				"thread":            map[string]any{"id": "thread-1"},
				"model":             "gpt-test",
				"approvalPolicy":    params["approvalPolicy"],
				"approvalsReviewer": reviewer,
				"cwd":               params["cwd"],
			})
		case "turn/start":
			f.mu.Lock()
			f.turns++
			turn := "turn-" + string(rune('0'+f.turns))
			f.mu.Unlock()
			prompt := ""
			if input, ok := params["input"].([]any); ok && len(input) > 0 {
				if first, ok := input[0].(map[string]any); ok {
					prompt, _ = first["text"].(string)
				}
			}
			f.respond(id, map[string]any{"turn": map[string]any{"id": turn, "status": "inProgress"}})
			f.notify("turn/started", map[string]any{"threadId": "thread-1", "turn": map[string]any{"id": turn}})
			f.notify("item/completed", map[string]any{"threadId": "thread-1", "turnId": turn,
				"item": map[string]any{"type": "agentMessage", "id": "m-" + turn, "text": "reply: " + prompt}})
			f.notify("turn/completed", map[string]any{"threadId": "thread-1",
				"turn": map[string]any{"id": turn, "status": "completed"}})
		default:
			if hasID {
				f.respond(id, map[string]any{})
			}
		}
	}
}

func (f *fakeAppServer) respond(id, result any) {
	f.wMu.Lock()
	defer f.wMu.Unlock()
	_ = f.enc.Encode(map[string]any{"id": id, "result": result})
}

func (f *fakeAppServer) notify(method string, params any) {
	f.wMu.Lock()
	defer f.wMu.Unlock()
	_ = f.enc.Encode(map[string]any{"method": method, "params": params})
}

// withFakeCodexTransport swaps codexTransportFactory for a fake app-server
// that applies reviewer ("" echoes the request) and restores it on cleanup.
func withFakeCodexTransport(t *testing.T, reviewer string) {
	t.Helper()
	prev := codexTransportFactory
	codexTransportFactory = func(LaunchCodexAppOptions) (codexapp.Transport, *codexapp.SubprocessTransport, error) {
		tp, _ := newFakeAppServer(t, reviewer)
		return tp, nil, nil
	}
	t.Cleanup(func() { codexTransportFactory = prev })
}

// nextEvent reads one event or fails after a timeout.
func nextEvent(t *testing.T, sess *codexapp.Session) codexapp.CodexEvent {
	t.Helper()
	select {
	case ev, ok := <-sess.Events():
		if !ok {
			t.Fatal("events closed early")
		}
		return ev
	case <-time.After(3 * time.Second):
		t.Fatal("event timeout")
	}
	return codexapp.CodexEvent{}
}

func waitDone(t *testing.T, sess *codexapp.Session) codexapp.SessionResult {
	t.Helper()
	select {
	case res := <-sess.Done():
		return res
	case <-time.After(3 * time.Second):
		t.Fatal("Done timeout")
	}
	return codexapp.SessionResult{}
}

func TestLaunchCodexAppBridgesEventsAndDone(t *testing.T) {
	withFakeCodexTransport(t, "")
	h, err := LaunchCodexApp(context.Background(), LaunchCodexAppOptions{Prompt: "do thing", ProjectDir: "/tmp"})
	if err != nil {
		t.Fatalf("LaunchCodexApp: %v", err)
	}
	t.Cleanup(func() { _ = h.Close() })

	var types []string
	for _, want := range []string{"session_configured", "task_started", "agent_message", "task_complete"} {
		ev := nextEvent(t, h.Session())
		types = append(types, ev.EventType())
		if ev.EventType() != want {
			t.Fatalf("events %v, want %s next", types, want)
		}
		if want == "agent_message" {
			var am codexapp.AgentMessageEvent
			_ = json.Unmarshal(ev.Msg, &am)
			if am.Message != "reply: do thing" {
				t.Fatalf("message = %q", am.Message)
			}
		}
	}
	res := waitDone(t, h.Session())
	if res.Err != nil || res.ThreadID != "thread-1" || res.Content != "reply: do thing" {
		t.Fatalf("result = %+v", res)
	}
}

func TestLaunchCodexAppReportsTransportError(t *testing.T) {
	prev := codexTransportFactory
	codexTransportFactory = func(opts LaunchCodexAppOptions) (codexapp.Transport, *codexapp.SubprocessTransport, error) {
		return nil, nil, ErrCodexUnavailable
	}
	t.Cleanup(func() { codexTransportFactory = prev })

	_, err := LaunchCodexApp(context.Background(), LaunchCodexAppOptions{Prompt: "x"})
	if !errors.Is(err, ErrCodexUnavailable) {
		t.Fatalf("expected ErrCodexUnavailable, got %v", err)
	}
}

// The real factory, not a stub: these never get as far as spawning anything.

func TestCodexTransportFactoryWithoutCodex(t *testing.T) {
	withFakePath(t /* no fakes */)
	t.Setenv("MG_AGENT_RUNTIME", "")

	_, _, err := codexTransportFactory(LaunchCodexAppOptions{Prompt: "x", ProjectDir: t.TempDir()})
	if !errors.Is(err, ErrCodexUnavailable) {
		t.Fatalf("expected ErrCodexUnavailable, got %v", err)
	}
}

func TestCodexTransportFactoryUnresolvableWrapperFailsClosed(t *testing.T) {
	withFakePath(t, "codex") // must not be spawned behind the wrapper's back
	t.Setenv(AgentCommandEnv, "definitely-not-a-binary-xyz")
	t.Setenv("MG_AGENT_RUNTIME", "codex")

	_, _, err := codexTransportFactory(LaunchCodexAppOptions{Prompt: "x", ProjectDir: t.TempDir()})
	if err == nil || !strings.Contains(err.Error(), AgentCommandEnv) {
		t.Fatalf("expected an error naming %s, got %v", AgentCommandEnv, err)
	}
}

// TestLaunchCtxDoesNotKillSession: the session must outlive the launch
// context, which mg's codexLaunchCmd cancels as soon as LaunchCodexApp returns.
func TestLaunchCtxDoesNotKillSession(t *testing.T) {
	withFakeCodexTransport(t, "")
	ctx, cancel := context.WithCancel(context.Background())
	h, err := LaunchCodexApp(ctx, LaunchCodexAppOptions{Prompt: "do thing", ProjectDir: "/tmp"})
	if err != nil {
		t.Fatalf("LaunchCodexApp: %v", err)
	}
	t.Cleanup(func() { _ = h.Close() })
	cancel()

	if res := waitDone(t, h.Session()); res.Err != nil {
		t.Fatalf("session ended with error after the launch ctx was canceled: %v", res.Err)
	}
}

func TestReplyRunsANewTurnOnTheSameThread(t *testing.T) {
	withFakeCodexTransport(t, "")
	h, err := LaunchCodexApp(context.Background(), LaunchCodexAppOptions{Prompt: "first", ProjectDir: "/tmp"})
	if err != nil {
		t.Fatalf("LaunchCodexApp: %v", err)
	}
	t.Cleanup(func() { _ = h.Close() })
	first := h.Session()
	waitDone(t, first)

	ctx, cancel := context.WithCancel(context.Background())
	sess, err := h.Reply(ctx, "second")
	cancel() // like codexReplyCmd's defer: must not kill the turn
	if err != nil {
		t.Fatalf("Reply: %v", err)
	}
	if sess == first || h.Session() != sess {
		t.Fatal("Reply should rotate the handle onto the new turn")
	}
	res := waitDone(t, sess)
	if res.Err != nil || res.ThreadID != "thread-1" || res.Content != "reply: second" {
		t.Fatalf("reply result = %+v", res)
	}
}

func TestCloseIsIdempotent(t *testing.T) {
	withFakeCodexTransport(t, "")
	h, err := LaunchCodexApp(context.Background(), LaunchCodexAppOptions{Prompt: "x"})
	if err != nil {
		t.Fatalf("LaunchCodexApp: %v", err)
	}
	if err := h.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if err := h.Close(); err != nil {
		t.Fatalf("double close: %v", err)
	}
}

func TestApprovalsReviewerPinnedWhenHumanPresent(t *testing.T) {
	// A config.toml with approvals_reviewer = "guardian_subagent" had codex
	// approve a network escalation itself; mg's modal never saw it (mg-xge.4).
	if got := approvalsReviewer("on-request"); got != "user" {
		t.Fatalf("on-request reviewer = %q, want user", got)
	}
	if got := approvalsReviewer("never"); got != "" {
		t.Fatalf("unattended reviewer = %q, want no override", got)
	}
}

func TestLaunchCodexAppEdgeCaseOtherReviewerFailsClosed(t *testing.T) {
	// If the server applies a reviewer other than the user anyway, approvals
	// would bypass mg: refuse to run the session.
	withFakeCodexTransport(t, "guardian_subagent")
	_, err := LaunchCodexApp(context.Background(), LaunchCodexAppOptions{Prompt: "x", ApprovalPolicy: "on-request"})
	if err == nil || !strings.Contains(err.Error(), "guardian_subagent") {
		t.Fatalf("err = %v, want a refusal naming the applied reviewer", err)
	}
}
