package codexapp

import (
	"bufio"
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"
)

// peer is the server side of a pipe pair: tests read what the client sent
// and write responses, notifications and server requests by hand.
type peer struct {
	t   *testing.T
	dec *json.Decoder
	enc *json.Encoder
	wMu sync.Mutex
	in  chan map[string]any
}

// dial connects a Client to a scripted peer, answering the handshake.
func dial(t *testing.T) (*Client, *peer) {
	t.Helper()
	tp, sr, sw := newPipePair()
	p := &peer{t: t, dec: json.NewDecoder(bufio.NewReader(sr)), enc: json.NewEncoder(sw), in: make(chan map[string]any, 64)}
	go func() {
		defer close(p.in)
		for {
			var m map[string]any
			if err := p.dec.Decode(&m); err != nil {
				return
			}
			p.in <- m
		}
	}()
	type dialed struct {
		c   *Client
		err error
	}
	ch := make(chan dialed, 1)
	go func() {
		c, err := Dial(context.Background(), tp)
		ch <- dialed{c, err}
	}()
	init := p.expect("initialize")
	params, _ := init["params"].(map[string]any)
	caps, _ := params["capabilities"].(map[string]any)
	if _, ok := caps["optOutNotificationMethods"]; !ok {
		t.Error("initialize should opt out of chatty notifications")
	}
	p.respond(init["id"], map[string]any{"userAgent": "fake"})
	p.expect("initialized")
	d := <-ch
	if d.err != nil {
		t.Fatalf("Dial: %v", d.err)
	}
	t.Cleanup(func() { _ = d.c.Close() })
	return d.c, p
}

func (p *peer) expect(method string) map[string]any {
	p.t.Helper()
	select {
	case m, ok := <-p.in:
		if !ok {
			p.t.Fatalf("connection closed waiting for %s", method)
		}
		if m["method"] != method {
			p.t.Fatalf("got %v, want %s", m["method"], method)
		}
		return m
	case <-time.After(3 * time.Second):
		p.t.Fatalf("timeout waiting for %s", method)
	}
	return nil
}

func (p *peer) send(v any) {
	p.wMu.Lock()
	defer p.wMu.Unlock()
	_ = p.enc.Encode(v)
}

func (p *peer) respond(id, result any) { p.send(map[string]any{"id": id, "result": result}) }
func (p *peer) notify(method string, params any) {
	p.send(map[string]any{"method": method, "params": params})
}

// startSession opens thread-1 and its first turn, answering both requests.
func startSession(t *testing.T, c *Client, p *peer, opts SessionOptions) *Session {
	t.Helper()
	type started struct {
		s   *Session
		err error
	}
	ch := make(chan started, 1)
	go func() {
		s, err := c.StartSession(context.Background(), opts)
		ch <- started{s, err}
	}()
	ts := p.expect("thread/start")
	params, _ := ts["params"].(map[string]any)
	p.respond(ts["id"], map[string]any{"thread": map[string]any{"id": "thread-1"}, "model": "gpt-test",
		"approvalPolicy": params["approvalPolicy"], "approvalsReviewer": params["approvalsReviewer"], "cwd": params["cwd"]})
	turn := p.expect("turn/start")
	p.respond(turn["id"], map[string]any{"turn": map[string]any{"id": "turn-1"}})
	st := <-ch
	if st.err != nil {
		t.Fatalf("StartSession: %v", st.err)
	}
	return st.s
}

func next(t *testing.T, s *Session) CodexEvent {
	t.Helper()
	select {
	case ev, ok := <-s.Events():
		if !ok {
			t.Fatal("events closed early")
		}
		return ev
	case <-time.After(3 * time.Second):
		t.Fatal("event timeout")
	}
	return CodexEvent{}
}

func done(t *testing.T, s *Session) SessionResult {
	t.Helper()
	select {
	case r := <-s.Done():
		return r
	case <-time.After(3 * time.Second):
		t.Fatal("Done timeout")
	}
	return SessionResult{}
}

func TestStartSessionSendsThreadSettings(t *testing.T) {
	c, p := dial(t)
	type started struct {
		s   *Session
		err error
	}
	ch := make(chan started, 1)
	go func() {
		s, err := c.StartSession(context.Background(), SessionOptions{Prompt: "hi", Cwd: "/w", Sandbox: "workspace-write",
			ApprovalPolicy: "on-request", ApprovalsReviewer: "user"})
		ch <- started{s, err}
	}()
	ts := p.expect("thread/start")
	params, _ := ts["params"].(map[string]any)
	cfg, _ := params["config"].(map[string]any)
	if params["approvalsReviewer"] != "user" || cfg["approvals_reviewer"] != "user" || params["sandbox"] != "workspace-write" || params["ephemeral"] != true {
		t.Fatalf("thread/start params = %v", params)
	}
	p.respond(ts["id"], map[string]any{"thread": map[string]any{"id": "thread-1"}, "approvalPolicy": "on-request", "approvalsReviewer": "user"})
	turn := p.expect("turn/start")
	tp, _ := turn["params"].(map[string]any)
	input, _ := tp["input"].([]any)
	if tp["threadId"] != "thread-1" || len(input) != 1 || input[0].(map[string]any)["text"] != "hi" {
		t.Fatalf("turn/start params = %v", tp)
	}
	p.respond(turn["id"], map[string]any{"turn": map[string]any{"id": "turn-1"}})
	st := <-ch
	if st.err != nil {
		t.Fatalf("StartSession: %v", st.err)
	}
	if ev := next(t, st.s); ev.EventType() != "session_configured" {
		t.Fatalf("first event = %q, want session_configured", ev.EventType())
	}
}

func TestStartSessionEdgeCaseReviewerMismatchFailsClosed(t *testing.T) {
	c, p := dial(t)
	errCh := make(chan error, 1)
	go func() {
		_, err := c.StartSession(context.Background(), SessionOptions{Prompt: "hi", ApprovalsReviewer: "user"})
		errCh <- err
	}()
	ts := p.expect("thread/start")
	p.respond(ts["id"], map[string]any{"thread": map[string]any{"id": "thread-1"}, "approvalsReviewer": "guardian_subagent"})
	if err := <-errCh; err == nil || !strings.Contains(err.Error(), "guardian_subagent") {
		t.Fatalf("err = %v, want a refusal", err)
	}
}

func TestTurnEventsTranslateAndComplete(t *testing.T) {
	c, p := dial(t)
	s := startSession(t, c, p, SessionOptions{Prompt: "go"})
	next(t, s) // session_configured

	item := func(method string, it map[string]any) {
		p.notify(method, map[string]any{"threadId": "thread-1", "turnId": "turn-1", "item": it})
	}
	p.notify("turn/started", map[string]any{"threadId": "thread-1", "turn": map[string]any{"id": "turn-1"}})
	item("item/completed", map[string]any{"type": "userMessage", "id": "u", "content": []any{map[string]any{"type": "text", "text": "go"}}})
	item("item/started", map[string]any{"type": "commandExecution", "id": "c1", "command": "/bin/zsh -lc 'curl -sI https://example.com'", "cwd": "/w"})
	item("item/completed", map[string]any{"type": "commandExecution", "id": "c1", "status": "declined", "exitCode": nil})
	item("item/started", map[string]any{"type": "fileChange", "id": "f1", "changes": []any{map[string]any{"path": "STATUS.md"}}})
	item("item/completed", map[string]any{"type": "agentMessage", "id": "a1", "text": "done"})
	p.notify("error", map[string]any{"threadId": "thread-1", "error": map[string]any{"message": "transient"}, "willRetry": true})
	p.notify("turn/completed", map[string]any{"threadId": "thread-1", "turn": map[string]any{"id": "turn-1", "status": "completed"}})

	want := []string{"task_started", "user_message", "exec_command_begin", "exec_command_end", "patch", "agent_message", "task_complete"}
	for i, w := range want {
		ev := next(t, s)
		if ev.EventType() != w {
			t.Fatalf("event %d = %q, want %q", i, ev.EventType(), w)
		}
		switch w {
		case "exec_command_begin":
			var b ExecCommandBeginEvent
			_ = json.Unmarshal(ev.Msg, &b)
			if strings.Join(b.Command, "|") != "/bin/zsh|-lc|curl -sI https://example.com" {
				t.Fatalf("argv = %q", b.Command)
			}
		case "exec_command_end":
			var e ExecCommandEndEvent
			_ = json.Unmarshal(ev.Msg, &e)
			if e.ExitCode != nil || e.Status != "declined" {
				t.Fatalf("end = %+v, want declined with no exit code", e)
			}
		}
	}
	res := done(t, s)
	if res.Err != nil || res.Content != "done" || res.ThreadID != "thread-1" {
		t.Fatalf("result = %+v", res)
	}
}

func TestTurnEdgeCaseFailedAndInterrupted(t *testing.T) {
	c, p := dial(t)
	s := startSession(t, c, p, SessionOptions{Prompt: "go"})
	p.notify("turn/completed", map[string]any{"threadId": "thread-1", "turn": map[string]any{"status": "failed", "error": map[string]any{"message": "quota exceeded"}}})
	if res := done(t, s); res.Err == nil || res.Err.Error() != "quota exceeded" {
		t.Fatalf("failed turn result = %+v", res)
	}

	// Cancel interrupts the active turn; the server ends it as interrupted.
	type started struct {
		s   *Session
		err error
	}
	ch := make(chan started, 1)
	go func() {
		s, err := c.StartReplySession(context.Background(), "thread-1", "again")
		ch <- started{s, err}
	}()
	turn := p.expect("turn/start")
	p.respond(turn["id"], map[string]any{"turn": map[string]any{"id": "turn-2"}})
	st := <-ch
	st.s.Cancel()
	in := p.expect("turn/interrupt")
	ip, _ := in["params"].(map[string]any)
	if ip["turnId"] != "turn-2" || ip["threadId"] != "thread-1" {
		t.Fatalf("interrupt params = %v", ip)
	}
	p.respond(in["id"], map[string]any{})
	p.notify("turn/completed", map[string]any{"threadId": "thread-1", "turn": map[string]any{"status": "interrupted"}})
	if res := done(t, st.s); res.Err == nil || !strings.Contains(res.Err.Error(), "interrupted") {
		t.Fatalf("interrupted result = %+v", res)
	}
}

func TestTransportCloseEndsTurn(t *testing.T) {
	c, p := dial(t)
	s := startSession(t, c, p, SessionOptions{Prompt: "go"})
	_ = c.Close()
	if res := done(t, s); res.Err == nil {
		t.Fatal("a closed transport should end the turn with an error")
	}
}

func TestApprovalsForwardedParsed(t *testing.T) {
	c, p := dial(t)
	s := startSession(t, c, p, SessionOptions{Prompt: "go"})
	_ = s

	p.send(map[string]any{"id": 7, "method": "item/commandExecution/requestApproval", "params": map[string]any{
		"threadId": "thread-1", "itemId": "c1", "command": "/bin/zsh -lc 'rm -rf build'", "cwd": "/w", "reason": "clean up"}})
	req := <-c.ServerRequests()
	if req.Approval.Kind != "exec" || strings.Join(req.Approval.Command, "|") != "/bin/zsh|-lc|rm -rf build" || req.Approval.Cwd != "/w" || req.Approval.Reason != "clean up" {
		t.Fatalf("approval = %+v", req.Approval)
	}
	if !SameRequestID(req.RawID, json.RawMessage(`7`)) {
		t.Fatalf("raw id = %s", req.RawID)
	}

	// A file change's paths come from the item that precedes its approval.
	p.notify("item/started", map[string]any{"threadId": "thread-1", "item": map[string]any{"type": "fileChange", "id": "f1",
		"changes": []any{map[string]any{"path": "a.go"}, map[string]any{"path": "b.go"}}}})
	p.send(map[string]any{"id": "x8", "method": "item/fileChange/requestApproval", "params": map[string]any{"threadId": "thread-1", "itemId": "f1"}})
	req = <-c.ServerRequests()
	if req.Approval.Kind != "patch" || len(req.Approval.Changes) != 2 {
		t.Fatalf("patch approval = %+v", req.Approval)
	}
	if _, ok := req.Approval.Changes["a.go"]; !ok {
		t.Fatalf("changes = %v", req.Approval.Changes)
	}
}

func TestServerRequestsEdgeCasesAnsweredByClient(t *testing.T) {
	c, p := dial(t)
	_ = c
	// A file-change approval with no file list we saw: decline, fail closed.
	p.send(map[string]any{"id": 1, "method": "item/fileChange/requestApproval", "params": map[string]any{"threadId": "t", "itemId": "unknown"}})
	// Permissions: deny. MCP elicitation: decline. Anything else: error.
	p.send(map[string]any{"id": 2, "method": "item/permissions/requestApproval", "params": map[string]any{}})
	p.send(map[string]any{"id": 3, "method": "mcpServer/elicitation/request", "params": map[string]any{}})
	p.send(map[string]any{"id": 4, "method": "item/tool/call", "params": map[string]any{}})

	got := map[float64]map[string]any{}
	for len(got) < 4 {
		select {
		case m := <-p.in:
			if id, ok := m["id"].(float64); ok {
				got[id] = m
			}
		case <-time.After(3 * time.Second):
			t.Fatalf("answers so far: %v", got)
		}
	}
	if r, _ := got[1]["result"].(map[string]any); r["decision"] != "decline" {
		t.Fatalf("unseen file change = %v, want decline", got[1])
	}
	if r, _ := got[2]["result"].(map[string]any); r["scope"] != "turn" {
		t.Fatalf("permissions = %v", got[2])
	}
	if r, _ := got[3]["result"].(map[string]any); r["action"] != "decline" {
		t.Fatalf("elicitation = %v", got[3])
	}
	if e, _ := got[4]["error"].(map[string]any); e["code"] != float64(rpcMethodNotFound) {
		t.Fatalf("unknown = %v, want a method-not-found error", got[4])
	}
	select {
	case req := <-c.ServerRequests():
		t.Fatalf("nothing should reach the app, got %+v", req)
	default:
	}
}

func TestResolvedRequestBecomesEvent(t *testing.T) {
	c, p := dial(t)
	s := startSession(t, c, p, SessionOptions{Prompt: "go"})
	next(t, s) // session_configured
	p.notify("serverRequest/resolved", map[string]any{"threadId": "thread-1", "requestId": 7})
	ev := next(t, s)
	var r RequestResolvedEvent
	_ = json.Unmarshal(ev.Msg, &r)
	if ev.EventType() != "server_request_resolved" || !SameRequestID(r.RequestID, json.RawMessage(`7`)) {
		t.Fatalf("event = %s", ev.Msg)
	}
}

func TestSplitShell(t *testing.T) {
	for in, want := range map[string]string{
		`/bin/zsh -lc 'curl -sI https://example.com'`: `/bin/zsh|-lc|curl -sI https://example.com`,
		`echo "a \"b\" c" d\ e`:                       `echo|a "b" c|d e`,
		`git commit -m 'it'"'"'s'`:                    `git|commit|-m|it's`,
		`  spaced   out  `:                            `spaced|out`,
	} {
		argv, ok := splitShell(in)
		if !ok || strings.Join(argv, "|") != want {
			t.Errorf("splitShell(%q) = %q %v, want %q", in, argv, ok, want)
		}
	}
	if _, ok := splitShell(`echo 'unterminated`); ok {
		t.Error("an unterminated quote should not parse")
	}
}

func TestSameRequestID(t *testing.T) {
	if !SameRequestID(json.RawMessage(`7`), json.RawMessage(` 7 `)) || SameRequestID(json.RawMessage(`7`), json.RawMessage(`"7"`)) {
		t.Fatal("ids compare by decoded value and type")
	}
}
