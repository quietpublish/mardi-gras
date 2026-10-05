package codexapp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
)

// Transport is the read/write side of a JSON-RPC connection. The default
// transport spawns `codex app-server` (see SubprocessTransport); tests use an
// in-memory pipe pair.
type Transport interface {
	// Reader returns a stream of newline-delimited JSON objects from the server.
	Reader() io.Reader
	// Writer accepts newline-delimited JSON objects to send to the server.
	Writer() io.Writer
	// Close shuts the transport down. After Close, Reader will return io.EOF
	// after draining any in-flight bytes, and Writer will return an error.
	Close() error
}

// Client speaks app-server JSON-RPC over the supplied Transport. One Client
// manages one subprocess (or pipe pair). Construct with Dial.
type Client struct {
	t     Transport
	enc   *json.Encoder
	dec   *json.Decoder
	w     io.Writer
	wMu   sync.Mutex
	rdErr atomic.Value // error

	nextID  atomic.Int64
	pending sync.Map // map[int]chan response

	// eventBuffer sizes each Session's event channel.
	eventBuffer int
	// sessions maps a thread ID to the Session running its current turn, so
	// readLoop routes each notification to the turn it belongs to.
	sessions     sync.Map
	eventsClosed atomic.Bool

	// fileChanges caches each fileChange item's paths by item ID: its
	// approval request does not carry them, but the item arrives first.
	fileChanges sync.Map // map[string][]string

	serverReqCh chan ServerRequest

	closeOnce sync.Once
	closeErr  error
	done      chan struct{}
}

// ClientOption customizes Dial behavior.
type ClientOption func(*clientOptions)

type clientOptions struct {
	clientVersion string
	eventBuffer   int
}

// WithClientVersion overrides the version reported to the server in
// initialize. Defaults to "dev".
func WithClientVersion(v string) ClientOption {
	return func(o *clientOptions) { o.clientVersion = v }
}

// WithEventBuffer sets the buffered channel size for each Session's event
// delivery; the oldest event is dropped when it is full. Defaults to 64.
func WithEventBuffer(n int) ClientOption {
	return func(o *clientOptions) {
		if n > 0 {
			o.eventBuffer = n
		}
	}
}

// Dial constructs a Client around the given Transport and performs the
// initialize handshake. It does not start a thread; see StartSession.
//
// On any handshake failure the transport is closed before returning.
func Dial(ctx context.Context, t Transport, opts ...ClientOption) (*Client, error) {
	o := clientOptions{
		clientVersion: clientVersionFallback,
		eventBuffer:   64,
	}
	for _, opt := range opts {
		opt(&o)
	}

	c := &Client{
		t:           t,
		dec:         json.NewDecoder(bufio.NewReader(t.Reader())),
		w:           t.Writer(),
		eventBuffer: o.eventBuffer,
		serverReqCh: make(chan ServerRequest, 16),
		done:        make(chan struct{}),
	}
	c.enc = json.NewEncoder(c.w)

	go c.readLoop()

	if err := c.initialize(ctx, o.clientVersion); err != nil {
		_ = c.Close()
		return nil, err
	}
	if err := c.notify(methodInitialized, nil); err != nil {
		_ = c.Close()
		return nil, fmt.Errorf("notify initialized: %w", err)
	}
	return c, nil
}

// ServerRequests returns a receive channel of approval requests. The channel
// is closed when the client shuts down. Each request must be answered with
// Respond using its RawID, or the turn that issued it stalls.
func (c *Client) ServerRequests() <-chan ServerRequest {
	return c.serverReqCh
}

// Done returns a channel that is closed when the transport hits EOF or an
// unrecoverable read error.
func (c *Client) Done() <-chan struct{} {
	return c.done
}

// ReadError returns the error that terminated the read loop, if any.
func (c *Client) ReadError() error {
	if v := c.rdErr.Load(); v != nil {
		if e, ok := v.(error); ok {
			return e
		}
	}
	return nil
}

// Close shuts the client and transport down. Safe to call multiple times.
// In-flight Calls unblock via c.done, closed by readLoop at EOF. Pending
// response channels are not closed here: that would race readLoop
// dispatching a response it just took from the pending map.
func (c *Client) Close() error {
	c.closeOnce.Do(func() {
		c.closeErr = c.t.Close()
	})
	return c.closeErr
}

// Call issues a JSON-RPC request and waits for the matching response. It
// blocks until ctx is canceled, the server responds, or the transport closes.
func (c *Client) Call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	id := int(c.nextID.Add(1))
	ch := make(chan response, 1)
	c.pending.Store(id, ch)
	defer c.pending.Delete(id)

	raw, err := marshalParams(params)
	if err != nil {
		return nil, fmt.Errorf("marshal params: %w", err)
	}
	if err := c.writeJSON(request{JSONRPC: jsonRPCVersion, ID: id, Method: method, Params: raw}); err != nil {
		return nil, fmt.Errorf("write request: %w", err)
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-c.done:
		if err := c.ReadError(); err != nil {
			return nil, fmt.Errorf("transport closed: %w", err)
		}
		return nil, errors.New("transport closed")
	case resp := <-ch:
		if resp.Error != nil {
			return nil, fmt.Errorf("rpc error %d: %s", resp.Error.Code, resp.Error.Message)
		}
		return resp.Result, nil
	}
}

// notify sends a JSON-RPC notification (no id, no response expected).
func (c *Client) notify(method string, params any) error {
	raw, err := marshalParams(params)
	if err != nil {
		return err
	}
	return c.writeJSON(notification{JSONRPC: jsonRPCVersion, Method: method, Params: raw})
}

// Respond answers a server request with a result, echoing its RawID.
func (c *Client) Respond(rawID json.RawMessage, result any) error {
	raw, err := marshalParams(result)
	if err != nil {
		return fmt.Errorf("marshal result: %w", err)
	}
	return c.writeJSON(response{JSONRPC: jsonRPCVersion, ID: rawID, Result: raw})
}

// RespondError answers a server request with a JSON-RPC error object.
func (c *Client) RespondError(rawID json.RawMessage, code int, message string) error {
	return c.writeJSON(response{JSONRPC: jsonRPCVersion, ID: rawID, Error: &rpcError{Code: code, Message: message}})
}

func (c *Client) initialize(ctx context.Context, version string) error {
	params := map[string]any{
		"clientInfo": map[string]any{"name": clientName, "title": nil, "version": version},
		"capabilities": map[string]any{
			"experimentalApi":           false,
			"optOutNotificationMethods": optOutNotifications,
		},
	}
	if _, err := c.Call(ctx, methodInitialize, params); err != nil {
		return fmt.Errorf("initialize: %w", err)
	}
	return nil
}

func (c *Client) writeJSON(v any) error {
	c.wMu.Lock()
	defer c.wMu.Unlock()
	return c.enc.Encode(v)
}

func marshalParams(params any) (json.RawMessage, error) {
	if params == nil {
		return nil, nil
	}
	if raw, ok := params.(json.RawMessage); ok {
		return raw, nil
	}
	return json.Marshal(params)
}

// readLoop dispatches inbound messages: server requests (id and method),
// responses to our requests (id only), and notifications (method only).
func (c *Client) readLoop() {
	defer func() {
		if !c.eventsClosed.Swap(true) {
			close(c.serverReqCh)
		}
		// The transport is gone: end every live turn, or a consumer waiting
		// on Done() would wait forever.
		err := c.ReadError()
		if err == nil {
			err = errors.New("codex app-server transport closed")
		}
		c.sessions.Range(func(_, v any) bool {
			if sess, ok := v.(*Session); ok {
				sess.finish(SessionResult{ThreadID: sess.threadID, Err: err})
			}
			return true
		})
		close(c.done)
	}()
	for {
		var msg response
		if err := c.dec.Decode(&msg); err != nil {
			if !errors.Is(err, io.EOF) {
				c.rdErr.Store(err)
			}
			return
		}
		switch {
		case msg.Method != "" && len(msg.ID) > 0:
			c.handleServerRequest(msg)
		case len(msg.ID) > 0:
			id, ok := parseIntID(msg.ID)
			if !ok {
				continue
			}
			if v, ok := c.pending.LoadAndDelete(id); ok {
				if ch, ok := v.(chan response); ok {
					ch <- msg
				}
			}
		case msg.Method != "":
			c.handleNotification(msg.Method, msg.Params)
		}
	}
}

// handleServerRequest forwards approvals and answers every other server
// request itself: the server waits on each one, so none may go unanswered.
func (c *Client) handleServerRequest(msg response) {
	switch msg.Method {
	case methodCommandApproval:
		var p struct {
			Command string `json:"command"`
			Cwd     string `json:"cwd"`
			Reason  string `json:"reason"`
		}
		_ = json.Unmarshal(msg.Params, &p)
		argv, ok := splitShell(p.Command)
		if !ok || len(argv) == 0 {
			argv = []string{p.Command} // unparseable: the deny-list scans it whole
		}
		c.forwardApproval(msg, Approval{Kind: "exec", Command: argv, Cwd: p.Cwd, Reason: p.Reason, Message: p.Reason})

	case methodFileChangeApproval:
		var p struct {
			ItemID    string `json:"itemId"`
			Reason    string `json:"reason"`
			GrantRoot string `json:"grantRoot"`
		}
		_ = json.Unmarshal(msg.Params, &p)
		v, ok := c.fileChanges.Load(p.ItemID)
		if !ok && p.GrantRoot == "" {
			// No file list to show or check: decline rather than ask a
			// human to approve changes nobody can see (fail closed).
			_ = c.Respond(msg.ID, map[string]any{"decision": "decline"})
			return
		}
		changes := map[string]json.RawMessage{}
		if paths, ok := v.([]string); ok {
			for _, path := range paths {
				changes[path] = nil
			}
		}
		c.forwardApproval(msg, Approval{Kind: "patch", Changes: changes, Root: p.GrantRoot, Reason: p.Reason, Message: p.Reason})

	case "item/permissions/requestApproval":
		_ = c.Respond(msg.ID, map[string]any{"permissions": map[string]any{}, "scope": "turn"})

	case "mcpServer/elicitation/request":
		_ = c.Respond(msg.ID, map[string]any{"action": "decline", "content": nil, "_meta": nil})

	default:
		_ = c.RespondError(msg.ID, rpcMethodNotFound, "mardi-gras does not handle "+msg.Method)
	}
}

// forwardApproval hands an approval to the app. If the queue is full the
// request is declined rather than dropped: a dropped request stalls the turn.
func (c *Client) forwardApproval(msg response, a Approval) {
	if c.eventsClosed.Load() {
		return
	}
	select {
	case c.serverReqCh <- ServerRequest{RawID: msg.ID, Method: msg.Method, Params: msg.Params, Approval: a}:
	default:
		_ = c.Respond(msg.ID, map[string]any{"decision": "decline"})
	}
}

// handleNotification translates a notification into transcript events for
// the thread's current turn, and ends the turn on turn/completed.
func (c *Client) handleNotification(method string, params json.RawMessage) {
	var head struct {
		ThreadID string `json:"threadId"`
	}
	_ = json.Unmarshal(params, &head)

	// Cache file changes even before a turn is registered: the approval
	// that needs them can follow at once.
	if method == "item/started" || method == "item/completed" {
		var p struct {
			Item threadItem `json:"item"`
		}
		if json.Unmarshal(params, &p) == nil && p.Item.Type == "fileChange" {
			c.fileChanges.Store(p.Item.ID, p.Item.paths())
		}
	}

	v, ok := c.sessions.Load(head.ThreadID)
	if !ok {
		return
	}
	sess, ok := v.(*Session)
	if !ok {
		return
	}
	for _, msg := range translate(method, params, sess) {
		sess.deliver(CodexEvent{Meta: EventMeta{ThreadID: head.ThreadID}, Msg: msg})
	}
	if method == "turn/completed" {
		sess.finish(turnResult(params, sess))
	}
}

// threadItem is the subset of a v2 ThreadItem mg renders.
type threadItem struct {
	Type     string `json:"type"`
	ID       string `json:"id"`
	Text     string `json:"text"`
	Command  string `json:"command"`
	Cwd      string `json:"cwd"`
	Status   string `json:"status"`
	ExitCode *int   `json:"exitCode"`
	Server   string `json:"server"`
	Tool     string `json:"tool"`
	Content  []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Changes []struct {
		Path string `json:"path"`
	} `json:"changes"`
}

func (it threadItem) paths() []string {
	out := make([]string, 0, len(it.Changes))
	for _, ch := range it.Changes {
		out = append(out, ch.Path)
	}
	return out
}

// translate turns one notification into zero or more transcript events.
func translate(method string, params json.RawMessage, sess *Session) []json.RawMessage {
	event := func(v map[string]any) json.RawMessage {
		b, _ := json.Marshal(v)
		return b
	}
	switch method {
	case "turn/started":
		var p struct {
			Turn struct {
				ID string `json:"id"`
			} `json:"turn"`
		}
		_ = json.Unmarshal(params, &p)
		sess.setTurnID(p.Turn.ID)
		return []json.RawMessage{event(map[string]any{"type": "task_started", "turn_id": p.Turn.ID})}

	case "turn/completed":
		var p struct {
			Turn struct {
				Status string `json:"status"`
			} `json:"turn"`
		}
		_ = json.Unmarshal(params, &p)
		return []json.RawMessage{event(map[string]any{"type": "task_complete", "status": p.Turn.Status, "last_agent_message": sess.lastAgentMessage()})}

	case "error":
		var p struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
			WillRetry bool `json:"willRetry"`
		}
		_ = json.Unmarshal(params, &p)
		if p.WillRetry {
			return nil // the server retries; the final outcome comes later
		}
		return []json.RawMessage{event(map[string]any{"type": "error", "message": p.Error.Message})}

	case "serverRequest/resolved":
		var p struct {
			RequestID json.RawMessage `json:"requestId"`
		}
		_ = json.Unmarshal(params, &p)
		return []json.RawMessage{event(map[string]any{"type": "server_request_resolved", "request_id": p.RequestID})}

	case "item/started", "item/completed":
		var p struct {
			Item threadItem `json:"item"`
		}
		if json.Unmarshal(params, &p) != nil {
			return nil
		}
		return translateItem(method == "item/completed", p.Item, sess, event)
	}
	return nil
}

func translateItem(completed bool, it threadItem, sess *Session, event func(map[string]any) json.RawMessage) []json.RawMessage {
	switch it.Type {
	case "agentMessage":
		if !completed {
			return nil
		}
		sess.setLastAgentMessage(it.Text)
		return []json.RawMessage{event(map[string]any{"type": "agent_message", "message": it.Text})}
	case "userMessage":
		if !completed {
			return nil
		}
		var text []string
		for _, c := range it.Content {
			if c.Text != "" {
				text = append(text, c.Text)
			}
		}
		if len(text) == 0 {
			return nil
		}
		return []json.RawMessage{event(map[string]any{"type": "user_message", "message": joinLines(text)})}
	case "commandExecution":
		if !completed {
			argv, ok := splitShell(it.Command)
			if !ok || len(argv) == 0 {
				argv = []string{it.Command}
			}
			return []json.RawMessage{event(map[string]any{"type": "exec_command_begin", "call_id": it.ID, "command": argv, "cwd": it.Cwd})}
		}
		return []json.RawMessage{event(map[string]any{"type": "exec_command_end", "call_id": it.ID, "exit_code": it.ExitCode, "status": it.Status})}
	case "mcpToolCall":
		if !completed {
			return []json.RawMessage{event(map[string]any{"type": "mcp_tool_call_begin", "call_id": it.ID,
				"invocation": map[string]any{"server": it.Server, "tool": it.Tool}})}
		}
		return []json.RawMessage{event(map[string]any{"type": "mcp_tool_call_end", "call_id": it.ID, "is_error": it.Status == "failed"})}
	case "fileChange":
		status := it.Status
		if !completed {
			status = "proposed"
		}
		return []json.RawMessage{event(map[string]any{"type": "patch", "call_id": it.ID, "paths": it.paths(), "status": status})}
	}
	return nil
}

// turnResult is a completed turn's terminal SessionResult.
func turnResult(params json.RawMessage, sess *Session) SessionResult {
	var p struct {
		Turn struct {
			Status string `json:"status"`
			Error  *struct {
				Message string `json:"message"`
			} `json:"error"`
		} `json:"turn"`
	}
	_ = json.Unmarshal(params, &p)
	res := SessionResult{ThreadID: sess.threadID, Content: sess.lastAgentMessage()}
	switch p.Turn.Status {
	case "failed":
		msg := "turn failed"
		if p.Turn.Error != nil && p.Turn.Error.Message != "" {
			msg = p.Turn.Error.Message
		}
		res.Err = errors.New(msg)
	case "interrupted":
		res.Err = errors.New("turn interrupted")
	}
	return res
}

func joinLines(lines []string) string {
	out := lines[0]
	for _, l := range lines[1:] {
		out += "\n" + l
	}
	return out
}
