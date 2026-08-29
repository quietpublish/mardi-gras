package codexmcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
)

// SessionOptions controls how StartSession invokes `tools/call codex`.
//
// Only Prompt is required. Cwd, Sandbox, ApprovalPolicy, and Model are passed
// through to the codex tool when set. Anything left at its zero value is
// omitted from the arguments object so the server uses its defaults.
type SessionOptions struct {
	Prompt         string
	Cwd            string
	Sandbox        string // "read-only" | "workspace-write" | "danger-full-access"
	ApprovalPolicy string // "untrusted" | "on-failure" | "on-request" | "never"
	Model          string
}

// Session represents one in-flight codex tool call. It holds a reference to
// the Client and surfaces events scoped to its requestId / threadId via
// Events().
//
// A session ends when:
//   - tools/call returns (Result or Err is set on the SessionDone channel)
//   - the transport closes underneath it
//   - Cancel is called
//
// Multiple sessions cannot share one Client today: codex's mcp-server appears
// to handle one tool invocation at a time (the second blocks behind the
// first). mg should pair one Client to one Session for now.
type Session struct {
	client   *Client
	threadID atomic.Value // string
	reqID    int

	events     chan CodexEvent
	done       chan SessionResult
	cancelFunc context.CancelFunc

	stop      chan struct{}
	stopOnce  sync.Once
	closeOnce sync.Once

	// evMu guards delivery into events against closeEvents, so the client's
	// readLoop can never send on a channel a terminating session just closed.
	evMu     sync.Mutex
	evClosed bool
}

// SessionResult is the terminal outcome of a session.
type SessionResult struct {
	ThreadID string
	Content  string
	Err      error
}

// StartSession invokes the codex tool with the given options and returns a
// Session whose Events() channel streams codex/event notifications correlated
// to this request. The call is non-blocking; tools/call runs in a goroutine
// and its result lands on Done().
//
// The provided ctx governs the tools/call only — events continue to drain
// until the session terminates. Use Cancel to stop early.
//
// The Client supports *sequential* tool calls (one Session at a time); see
// StartReplySession for continuing a conversation under the same threadId.
func (c *Client) StartSession(ctx context.Context, opts SessionOptions) (*Session, error) {
	if opts.Prompt == "" {
		return nil, errors.New("codexmcp: SessionOptions.Prompt is required")
	}
	return c.startToolSession(ctx, codexToolName, buildCodexArgs(opts))
}

// StartReplySession continues an existing codex conversation by calling the
// codex-reply tool with the given threadID and prompt. Returns a new Session
// whose Events()/Done() streams correspond to the *new* tools/call; the
// original Session has already terminated. The underlying codex subprocess
// (and its conversation history at threadID) is reused, so replies are
// near-instant compared to the cold-start handshake.
//
// Callers must ensure the prior Session has reached Done() before calling
// this — codex serializes tool calls on its end.
func (c *Client) StartReplySession(ctx context.Context, threadID, prompt string) (*Session, error) {
	if prompt == "" {
		return nil, errors.New("codexmcp: StartReplySession requires a prompt")
	}
	if threadID == "" {
		return nil, errors.New("codexmcp: StartReplySession requires a threadID")
	}
	return c.startToolSession(ctx, codexReplyToolName, map[string]any{
		"threadId": threadID,
		"prompt":   prompt,
	})
}

// startToolSession is the shared core of StartSession and StartReplySession:
// it marshals a tools/call request for the named tool, registers a pending
// response channel, writes the request, and spawns the demux + await
// goroutines.
func (c *Client) startToolSession(ctx context.Context, toolName string, args map[string]any) (*Session, error) {
	params := toolsCallParams{Name: toolName, Arguments: args}
	callCtx, cancel := context.WithCancel(ctx)

	s := &Session{
		client:     c,
		events:     make(chan CodexEvent, c.eventBuffer),
		done:       make(chan SessionResult, 1),
		cancelFunc: cancel,
		stop:       make(chan struct{}),
	}

	rawParams, err := json.Marshal(params)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("marshal tools/call params: %w", err)
	}

	id := int(c.nextID.Add(1))
	s.reqID = id

	respCh := make(chan response, 1)
	c.pending.Store(id, respCh)

	req := request{
		JSONRPC: jsonRPCVersion,
		ID:      id,
		Method:  methodToolsCall,
		Params:  rawParams,
	}
	// Register BEFORE writing: the server can emit a codex/event for this
	// request as soon as it reads it, and readLoop drops any event with no
	// live session. Registering afterwards loses events to a fast server.
	c.sessions.Store(id, s)

	if err := c.writeJSON(req); err != nil {
		c.sessions.Delete(id)
		c.pending.Delete(id)
		close(s.events) // safe — nothing was ever routed here
		cancel()
		return nil, fmt.Errorf("write tools/call: %w", err)
	}

	go s.awaitResponse(callCtx, respCh)
	return s, nil
}

// Events returns events whose `_meta.requestId` matches this session.
// The channel is closed when the session terminates.
func (s *Session) Events() <-chan CodexEvent {
	return s.events
}

// Done returns a channel that yields the terminal result exactly once.
func (s *Session) Done() <-chan SessionResult {
	return s.done
}

// ThreadID returns the codex thread id, set after the first session_configured
// event is observed. Empty until then.
func (s *Session) ThreadID() string {
	if v := s.threadID.Load(); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func (s *Session) setThreadID(id string) {
	if id == "" {
		return
	}
	s.threadID.CompareAndSwap(nil, id)
}

// Cancel ends the session early. The transport-level interruption that codex
// supports requires a separate notification (`interrupt`); for now this just
// cancels the pending tools/call and stops demuxing. The subprocess will
// finish its current turn and may still emit events until the parent Client
// is closed.
func (s *Session) Cancel() {
	if s.cancelFunc != nil {
		s.cancelFunc()
	}
	s.signalStop()
}

// signalStop closes the stop channel exactly once. Once closed, demuxEvents
// will return and stop sending on s.events.
func (s *Session) signalStop() {
	s.stopOnce.Do(func() { close(s.stop) })
}

// closeEvents closes the consumer-facing events channel. Buffered events
// already delivered stay readable afterwards, which is what lets a consumer
// still see a final agent_message emitted just before the tool response.
func (s *Session) closeEvents() {
	s.evMu.Lock()
	defer s.evMu.Unlock()
	s.closeOnce.Do(func() {
		s.evClosed = true
		close(s.events)
	})
}

// deliver pushes an event to this session's consumer channel with drop-oldest
// behavior on buffer pressure. It is called from the client's readLoop, which
// has already routed the event here by requestID.
//
// Delivery is serialized with closeEvents under evMu so a session terminating
// concurrently can never cause a send on a closed channel. Because readLoop is
// a single goroutine processing messages in wire order, an event emitted just
// before the tool response (codex's normal `agent_message` then result
// sequence) is always buffered here before awaitResponse can observe the
// response — so it survives the close and is still readable afterwards.
func (s *Session) deliver(ev CodexEvent) {
	s.evMu.Lock()
	defer s.evMu.Unlock()
	if s.evClosed {
		return
	}
	s.setThreadID(ev.Meta.ThreadID)
	select {
	case s.events <- ev:
	default:
		select {
		case <-s.events:
		default:
		}
		select {
		case s.events <- ev:
		default:
		}
	}
}

func (s *Session) awaitResponse(ctx context.Context, respCh chan response) {
	defer func() {
		// Unregister before closing so readLoop stops routing here, then close
		// so consumers ranging over Events() terminate.
		s.client.sessions.Delete(s.reqID)
		s.closeEvents()
		s.signalStop()
	}()
	select {
	case <-ctx.Done():
		s.client.pending.Delete(s.reqID)
		s.done <- SessionResult{ThreadID: s.ThreadID(), Err: ctx.Err()}
	case <-s.client.Done():
		// Transport closed (subprocess exited) before any response — surface
		// the underlying read error if there was one. Without this branch,
		// awaitResponse would block on respCh forever because the client no
		// longer closes pending channels on Close (that closed a race window
		// with readLoop's dispatch).
		s.client.pending.Delete(s.reqID)
		err := s.client.ReadError()
		if err == nil {
			err = errors.New("codex mcp transport closed")
		}
		s.done <- SessionResult{ThreadID: s.ThreadID(), Err: err}
	case resp, ok := <-respCh:
		if !ok {
			s.done <- SessionResult{ThreadID: s.ThreadID(), Err: errors.New("client closed")}
			return
		}
		if resp.Error != nil {
			s.done <- SessionResult{
				ThreadID: s.ThreadID(),
				Err:      fmt.Errorf("codex tool error %d: %s", resp.Error.Code, resp.Error.Message),
			}
			return
		}
		content, thread := parseCodexToolResult(resp.Result)
		s.setThreadID(thread)
		s.done <- SessionResult{ThreadID: s.ThreadID(), Content: content}
	}
}

func buildCodexArgs(opts SessionOptions) map[string]any {
	args := map[string]any{"prompt": opts.Prompt}
	if opts.Cwd != "" {
		args["cwd"] = opts.Cwd
	}
	if opts.Sandbox != "" {
		args["sandbox"] = opts.Sandbox
	}
	if opts.ApprovalPolicy != "" {
		args["approval-policy"] = opts.ApprovalPolicy
	}
	if opts.Model != "" {
		args["model"] = opts.Model
	}
	return args
}

// parseCodexToolResult extracts {threadId, content} from the structured tool
// result. The server returns:
//
//	{
//	  "content": [{"type":"text","text":"..."}],
//	  "structuredContent": {"threadId":"...","content":"..."},
//	  "isError": false
//	}
//
// We prefer structuredContent when present.
func parseCodexToolResult(raw json.RawMessage) (content, threadID string) {
	if len(raw) == 0 {
		return "", ""
	}
	var probe struct {
		StructuredContent struct {
			ThreadID string `json:"threadId"`
			Content  string `json:"content"`
		} `json:"structuredContent"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return "", ""
	}
	if probe.StructuredContent.Content != "" || probe.StructuredContent.ThreadID != "" {
		return probe.StructuredContent.Content, probe.StructuredContent.ThreadID
	}
	for _, c := range probe.Content {
		if c.Type == "text" {
			return c.Text, ""
		}
	}
	return "", ""
}
