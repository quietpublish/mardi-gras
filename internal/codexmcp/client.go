package codexmcp

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
// transport spawns `codex mcp-server` as a subprocess (see SubprocessTransport);
// tests use an in-memory pipe pair.
type Transport interface {
	// Reader returns a stream of newline-delimited JSON objects from the server.
	Reader() io.Reader
	// Writer accepts newline-delimited JSON objects to send to the server.
	Writer() io.Writer
	// Close shuts the transport down. After Close, Reader will return io.EOF
	// after draining any in-flight bytes, and Writer will return an error.
	Close() error
}

// Client speaks JSON-RPC to a Codex MCP server over the supplied Transport.
// One Client manages one subprocess (or pipe pair). The zero value is not
// usable — construct with Dial.
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
	// sessions maps a tools/call requestID to the Session that owns it, so
	// readLoop can route each codex/event directly to its session instead of
	// having every live session race for one shared channel.
	sessions     sync.Map
	eventsClosed atomic.Bool

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

// WithClientVersion overrides the version reported to the server in initialize.
// Defaults to "dev".
func WithClientVersion(v string) ClientOption {
	return func(o *clientOptions) { o.clientVersion = v }
}

// WithEventBuffer sets the buffered channel size for each Session's event
// delivery. Events are routed per-session by requestID, so this bounds how
// many events one session may hold before the oldest is dropped. Defaults
// to 64. A larger buffer reduces backpressure on the reader goroutine when the
// consumer (e.g. BubbleTea) is slow to drain.
func WithEventBuffer(n int) ClientOption {
	return func(o *clientOptions) {
		if n > 0 {
			o.eventBuffer = n
		}
	}
}

// Dial constructs a Client around the given Transport and performs the MCP
// initialize handshake. It does not start a Codex session — call StartSession
// for that.
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
	if err := c.notify(methodNotifyInitialized, nil); err != nil {
		_ = c.Close()
		return nil, fmt.Errorf("notify initialized: %w", err)
	}
	return c, nil
}

// ServerRequests returns a receive channel of server-initiated JSON-RPC requests
// (e.g. codex's `elicitation/create` approval prompts). The channel is closed when
// the client shuts down. Each request must be answered with Respond/RespondError
// using its RawID, or the agent loop that issued it will stall.
func (c *Client) ServerRequests() <-chan ServerRequest {
	return c.serverReqCh
}

// Done returns a channel that is closed when the underlying transport hits
// EOF or an unrecoverable read error. Reading from it is non-blocking only
// after shutdown.
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
// In-flight Call goroutines unblock via the c.done channel, which is closed
// by readLoop when the transport reports EOF (which Close triggers by
// closing stdin). We deliberately do NOT close pending response channels —
// doing so would race with readLoop dispatching a response that was just
// LoadAndDelete'd from the pending map, causing a send-on-closed-channel
// panic.
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
	req := request{
		JSONRPC: jsonRPCVersion,
		ID:      id,
		Method:  method,
		Params:  raw,
	}
	if err := c.writeJSON(req); err != nil {
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
	case resp, ok := <-ch:
		if !ok {
			return nil, errors.New("client closed")
		}
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
	n := notification{
		JSONRPC: jsonRPCVersion,
		Method:  method,
		Params:  raw,
	}
	return c.writeJSON(n)
}

// Respond answers a server-initiated request (ServerRequest) with a result. The
// rawID must be the ServerRequest.RawID, echoed back verbatim so the server can
// match the reply to its outstanding request.
func (c *Client) Respond(rawID json.RawMessage, result any) error {
	raw, err := marshalParams(result)
	if err != nil {
		return fmt.Errorf("marshal result: %w", err)
	}
	return c.writeJSON(response{
		JSONRPC: jsonRPCVersion,
		ID:      rawID,
		Result:  raw,
	})
}

// RespondError answers a server-initiated request with a JSON-RPC error object.
func (c *Client) RespondError(rawID json.RawMessage, code int, message string) error {
	return c.writeJSON(response{
		JSONRPC: jsonRPCVersion,
		ID:      rawID,
		Error:   &rpcError{Code: code, Message: message},
	})
}

func (c *Client) initialize(ctx context.Context, version string) error {
	params := initializeParams{
		ProtocolVersion: mcpProtocolVersion,
		// Advertise the elicitation capability so the server is free to send
		// `elicitation/create` approval requests (MCP spec). Codex ships these
		// regardless today, but this keeps mg spec-conformant and forward-compatible.
		Capabilities: map[string]any{"elicitation": map[string]any{}},
		ClientInfo: clientInfo{
			Name:    clientName,
			Version: version,
		},
	}
	_, err := c.Call(ctx, methodInitialize, params)
	if err != nil {
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
	b, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// readLoop dispatches inbound JSON-RPC messages:
//   - request (id + method, e.g. elicitation/create) → server-initiated request,
//     forwarded on serverReqCh.
//   - response (id, no method) → routed to the matching pending channel.
//   - notification named codex/event → decoded and routed to the owning
//     Session by requestID (see routeEvent).
//
// Unknown notifications are dropped.
func (c *Client) readLoop() {
	defer func() {
		if !c.eventsClosed.Swap(true) {
			close(c.serverReqCh)
		}
		// The transport is gone, so no further events can arrive. Close every
		// live session's channel or a consumer ranging over Events() would
		// block forever waiting on a stream that has ended.
		c.sessions.Range(func(_, v any) bool {
			if sess, ok := v.(*Session); ok {
				sess.closeEvents()
			}
			return true
		})
		close(c.done)
	}()
	for {
		var msg response
		if err := c.dec.Decode(&msg); err != nil {
			if errors.Is(err, io.EOF) {
				return
			}
			c.rdErr.Store(err)
			return
		}
		switch {
		case msg.Method != "" && len(msg.ID) > 0:
			// Server-initiated request. Best-effort, non-blocking send: approval
			// prompts don't pile up, and we must never block the read loop (which
			// also feeds in-flight Call responses).
			select {
			case c.serverReqCh <- ServerRequest{RawID: msg.ID, Method: msg.Method, Params: msg.Params}:
			default:
			}
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
		case msg.Method == methodCodexEvent:
			var ev CodexEvent
			if err := json.Unmarshal(msg.Params, &ev); err == nil {
				c.routeEvent(ev)
			}
		default:
			// Unknown notification — drop.
		}
	}
}

// routeEvent delivers a codex/event to the session that issued the matching
// tools/call. An event with no live session — one arriving after its session
// terminated, or carrying an unknown requestID — is dropped.
//
// Routing by requestID here is what makes overlapping sessions safe. Sessions
// used to each run a demuxer over one shared channel and discard events whose
// requestID did not match, so a session that had not finished winding down
// could swallow the next session's events.
func (c *Client) routeEvent(ev CodexEvent) {
	v, ok := c.sessions.Load(ev.Meta.RequestID)
	if !ok {
		return
	}
	if sess, ok := v.(*Session); ok {
		sess.deliver(ev)
	}
}
