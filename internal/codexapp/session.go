package codexapp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// SessionOptions controls the thread StartSession opens. Only Prompt is
// required; zero values are omitted so the server uses its defaults.
type SessionOptions struct {
	Prompt         string
	Cwd            string
	Sandbox        string // "read-only" | "workspace-write" | "danger-full-access"
	ApprovalPolicy string // "untrusted" | "on-request" | "never"
	Model          string
	// ApprovalsReviewer is who answers approval requests: "user" routes them
	// to mg. Sent as the thread parameter and as a config override, and when
	// set the server's effective value must match or StartSession fails:
	// a config.toml reviewer such as guardian_subagent used to approve
	// escalations mg never saw (mg-xge.4).
	ApprovalsReviewer string
	// Config overrides individual settings from CODEX_HOME/config.toml for
	// this thread.
	Config map[string]any
}

// ThreadInfo is what thread/start reports the server actually applied.
type ThreadInfo struct {
	ID                string
	Model             string
	ApprovalPolicy    string
	ApprovalsReviewer string
	Cwd               string
}

// Session is one turn on a thread: it streams the turn's events and ends
// with its result. Replies run as new turns (StartReplySession) on the same
// thread and process.
//
// A session ends when:
//   - the server reports turn/completed (completed, failed or interrupted)
//   - the transport closes underneath it
//   - Cancel is called (it interrupts the turn)
type Session struct {
	client   *Client
	threadID string
	turnID   atomic.Value // string

	events chan CodexEvent
	done   chan SessionResult

	lastMu    sync.Mutex
	lastAgent string

	finishOnce sync.Once
	closeOnce  sync.Once

	// evMu guards delivery into events against closeEvents, so readLoop can
	// never send on a channel a finishing session just closed.
	evMu     sync.Mutex
	evClosed bool
}

// SessionResult is the terminal outcome of a turn.
type SessionResult struct {
	ThreadID string
	Content  string
	Err      error
}

// StartSession opens a thread with opts and runs its first turn with
// opts.Prompt. The returned Session's first event is session_configured,
// carrying the settings the server applied.
func (c *Client) StartSession(ctx context.Context, opts SessionOptions) (*Session, error) {
	if opts.Prompt == "" {
		return nil, errors.New("codexapp: SessionOptions.Prompt is required")
	}
	info, err := c.StartThread(ctx, opts)
	if err != nil {
		return nil, err
	}
	if opts.ApprovalsReviewer != "" && info.ApprovalsReviewer != opts.ApprovalsReviewer {
		return nil, fmt.Errorf("codex applied approvals reviewer %q, not %q: refusing to run with approvals mg would not see",
			info.ApprovalsReviewer, opts.ApprovalsReviewer)
	}
	return c.startTurn(ctx, info.ID, opts.Prompt, &info)
}

// StartReplySession runs another turn with prompt on an existing thread. The
// previous turn must have completed.
func (c *Client) StartReplySession(ctx context.Context, threadID, prompt string) (*Session, error) {
	if prompt == "" {
		return nil, errors.New("codexapp: StartReplySession requires a prompt")
	}
	if threadID == "" {
		return nil, errors.New("codexapp: StartReplySession requires a threadID")
	}
	return c.startTurn(ctx, threadID, prompt, nil)
}

// StartThread sends thread/start and reports what the server applied. The
// thread is ephemeral: nothing is written to ~/.codex for mg's sessions.
func (c *Client) StartThread(ctx context.Context, opts SessionOptions) (ThreadInfo, error) {
	params := map[string]any{"ephemeral": true, "serviceName": clientName}
	set := func(k, v string) {
		if v != "" {
			params[k] = v
		}
	}
	set("cwd", opts.Cwd)
	set("sandbox", opts.Sandbox)
	set("approvalPolicy", opts.ApprovalPolicy)
	set("model", opts.Model)
	set("approvalsReviewer", opts.ApprovalsReviewer)
	config := map[string]any{}
	for k, v := range opts.Config {
		config[k] = v
	}
	if opts.ApprovalsReviewer != "" {
		config["approvals_reviewer"] = opts.ApprovalsReviewer
	}
	if len(config) > 0 {
		params["config"] = config
	}

	raw, err := c.Call(ctx, methodThreadStart, params)
	if err != nil {
		return ThreadInfo{}, fmt.Errorf("thread/start: %w", err)
	}
	var res struct {
		Thread struct {
			ID string `json:"id"`
		} `json:"thread"`
		Model             string `json:"model"`
		ApprovalPolicy    any    `json:"approvalPolicy"`
		ApprovalsReviewer string `json:"approvalsReviewer"`
		Cwd               string `json:"cwd"`
	}
	if err := json.Unmarshal(raw, &res); err != nil {
		return ThreadInfo{}, fmt.Errorf("thread/start: decode: %w", err)
	}
	if res.Thread.ID == "" {
		return ThreadInfo{}, errors.New("thread/start: no thread id in response")
	}
	policy, _ := res.ApprovalPolicy.(string) // a granular policy is an object
	return ThreadInfo{ID: res.Thread.ID, Model: res.Model, ApprovalPolicy: policy, ApprovalsReviewer: res.ApprovalsReviewer, Cwd: res.Cwd}, nil
}

// startTurn registers a Session for threadID and sends turn/start. info,
// when set, is announced first as session_configured.
func (c *Client) startTurn(ctx context.Context, threadID, prompt string, info *ThreadInfo) (*Session, error) {
	s := &Session{
		client:   c,
		threadID: threadID,
		events:   make(chan CodexEvent, c.eventBuffer),
		done:     make(chan SessionResult, 1),
	}
	if info != nil {
		b, _ := json.Marshal(map[string]any{"type": "session_configured", "thread_id": info.ID,
			"model": info.Model, "approval_policy": info.ApprovalPolicy, "cwd": info.Cwd})
		s.deliver(CodexEvent{Meta: EventMeta{ThreadID: threadID}, Msg: b})
	}
	// Register before writing: the server streams the turn's notifications
	// as soon as it reads turn/start, and readLoop drops events for a thread
	// with no live session.
	if prev, loaded := c.sessions.Swap(threadID, s); loaded {
		if p, ok := prev.(*Session); ok {
			p.finish(SessionResult{ThreadID: threadID, Err: errors.New("superseded by a new turn")})
		}
	}

	raw, err := c.Call(ctx, methodTurnStart, map[string]any{
		"threadId": threadID,
		"input":    []map[string]any{{"type": "text", "text": prompt, "text_elements": []any{}}},
	})
	if err != nil {
		c.sessions.CompareAndDelete(threadID, s)
		s.closeEvents()
		return nil, fmt.Errorf("turn/start: %w", err)
	}
	var res struct {
		Turn struct {
			ID string `json:"id"`
		} `json:"turn"`
	}
	if json.Unmarshal(raw, &res) == nil {
		s.setTurnID(res.Turn.ID)
	}
	return s, nil
}

// Events returns the turn's transcript events. The channel is closed when
// the turn ends.
func (s *Session) Events() <-chan CodexEvent {
	return s.events
}

// Done yields the turn's result exactly once.
func (s *Session) Done() <-chan SessionResult {
	return s.done
}

// ThreadID returns the thread this turn runs on.
func (s *Session) ThreadID() string {
	return s.threadID
}

// TurnID returns the turn's id, once the server has reported it.
func (s *Session) TurnID() string {
	if v, ok := s.turnID.Load().(string); ok {
		return v
	}
	return ""
}

func (s *Session) setTurnID(id string) {
	if id != "" {
		s.turnID.Store(id)
	}
}

func (s *Session) lastAgentMessage() string {
	s.lastMu.Lock()
	defer s.lastMu.Unlock()
	return s.lastAgent
}

func (s *Session) setLastAgentMessage(text string) {
	s.lastMu.Lock()
	defer s.lastMu.Unlock()
	s.lastAgent = text
}

// Cancel interrupts the turn. The server answers with turn/completed
// (interrupted), which ends the session; if it does not within a few
// seconds, the session ends anyway.
func (s *Session) Cancel() {
	turn := s.TurnID()
	go func() {
		if turn != "" {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			_, _ = s.client.Call(ctx, methodTurnInterrupt, map[string]any{"threadId": s.threadID, "turnId": turn})
		}
		select {
		case <-time.After(3 * time.Second):
			s.finish(SessionResult{ThreadID: s.threadID, Err: errors.New("turn interrupted")})
		case <-s.client.Done():
		}
	}()
}

// finish ends the turn with res: it unregisters the session, closes Events
// and sends res on Done. Only the first call has an effect.
func (s *Session) finish(res SessionResult) {
	s.finishOnce.Do(func() {
		s.client.sessions.CompareAndDelete(s.threadID, s)
		s.closeEvents()
		s.done <- res
	})
}

// closeEvents closes the events channel. Events already buffered stay
// readable, so a final agent_message emitted just before turn/completed is
// still seen.
func (s *Session) closeEvents() {
	s.evMu.Lock()
	defer s.evMu.Unlock()
	s.closeOnce.Do(func() {
		s.evClosed = true
		close(s.events)
	})
}

// deliver pushes an event, dropping the oldest when the buffer is full.
// Serialized with closeEvents under evMu.
func (s *Session) deliver(ev CodexEvent) {
	s.evMu.Lock()
	defer s.evMu.Unlock()
	if s.evClosed {
		return
	}
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
