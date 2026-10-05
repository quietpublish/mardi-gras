package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/matt-wright86/mardi-gras/internal/codexapp"
)

// CodexAppHandle owns the lifecycle of one codex app-server subprocess plus the
// most recent session running against it. Callers must call Close when done;
// mg's app closes all handles on quit. Replies rotate the session pointer
// (see Reply); the underlying subprocess is reused across replies.
type CodexAppHandle struct {
	transport *codexapp.SubprocessTransport
	client    *codexapp.Client
	session   *codexapp.Session

	closeOnce sync.Once
	closeErr  error
}

// Session returns the most recent codexapp.Session attached to this handle.
// After Reply rotates the session, Session() returns the new one.
func (h *CodexAppHandle) Session() *codexapp.Session { return h.session }

// ServerRequests returns the client's server-initiated request channel (codex
// approval prompts). The channel is stable across Reply session rotation since it
// belongs to the underlying client/subprocess. Returns nil if the handle is closed.
func (h *CodexAppHandle) ServerRequests() <-chan codexapp.ServerRequest {
	if h.client == nil {
		return nil
	}
	return h.client.ServerRequests()
}

// Respond answers a server-initiated request (e.g. an approval prompt) with a
// result, echoing the request's RawID. See codexapp.Client.Respond.
func (h *CodexAppHandle) Respond(rawID json.RawMessage, result any) error {
	if h.client == nil {
		return errors.New("agent: CodexAppHandle has no client (already closed?)")
	}
	return h.client.Respond(rawID, result)
}

// Reply continues the conversation by invoking codex-reply with the given
// prompt against the threadID captured from the original session. The new
// session becomes h.Session(); the old session has already terminated by the
// time the caller is allowed to call Reply (mg gates the call on the prior
// session's terminal Done, per #47's v0 design).
//
// The ctx parameter is currently unused for the session lifetime — like
// LaunchCodexApp, Reply detaches the session from the caller's ctx so a
// defer-cancel in the dispatch goroutine doesn't kill the session before
// any reply event is rendered (the same trap v0.21.1 fixed on the launch
// path). ctx is reserved for a future setup-only timeout if needed.
func (h *CodexAppHandle) Reply(ctx context.Context, prompt string) (*codexapp.Session, error) {
	_ = ctx // reserved; intentionally not propagated to StartReplySession
	if h.client == nil {
		return nil, errors.New("agent: CodexAppHandle has no client (already closed?)")
	}
	threadID := ""
	if h.session != nil {
		threadID = h.session.ThreadID()
	}
	if threadID == "" {
		return nil, errors.New("agent: cannot Reply — original session has no threadID yet")
	}
	sess, err := h.client.StartReplySession(context.Background(), threadID, prompt)
	if err != nil {
		return nil, fmt.Errorf("start codex-reply session: %w", err)
	}
	h.session = sess
	return sess, nil
}

// Close cancels the session, terminates the subprocess, and releases pipes.
// Safe to call multiple times.
func (h *CodexAppHandle) Close() error {
	h.closeOnce.Do(func() {
		if h.session != nil {
			h.session.Cancel()
		}
		if h.client != nil {
			_ = h.client.Close()
		}
		// transport.Close is called by client.Close via the Transport interface.
	})
	return h.closeErr
}

// StderrTail returns the last stderr lines emitted by the subprocess. Useful
// for diagnostic messages when the session ends with an error.
func (h *CodexAppHandle) StderrTail(n int) []string {
	if h.transport == nil {
		return nil
	}
	return h.transport.StderrLines(n)
}

// LaunchCodexAppOptions controls how an in-app codex session is launched.
type LaunchCodexAppOptions struct {
	// Prompt is the initial user prompt. Required.
	Prompt string
	// ProjectDir is the working directory for the subprocess and the codex
	// session's cwd argument.
	ProjectDir string
	// Sandbox overrides codex's sandbox mode. Defaults to "workspace-write" to
	// match the tmux-launched path in agent.Command.
	Sandbox string
	// ApprovalPolicy overrides codex's approval policy. Defaults to "never"
	// for unattended launches; "on-request" routes approvals to mg's modal
	// and pins approvals_reviewer to the user.
	ApprovalPolicy string
	// Model optionally overrides the codex model.
	Model string
	// ClientVersion is advertised to the server in initialize. Defaults to
	// "dev".
	ClientVersion string
}

// codexTransportFactory is the function used to spawn the codex app-server transport.
// Tests override this to inject a pipe-based transport without a real codex
// binary.
var codexTransportFactory = func(opts LaunchCodexAppOptions) (codexapp.Transport, *codexapp.SubprocessTransport, error) {
	bin, err := codexCommand()
	if err != nil {
		return nil, nil, err
	}
	t, err := codexapp.SpawnSubprocess(codexapp.WithBinary(bin), codexapp.WithDir(opts.ProjectDir))
	if err != nil {
		return nil, nil, fmt.Errorf("spawn codex app-server: %w", err)
	}
	return t, t, nil
}

// LaunchCodexApp spawns `codex app-server`, performs the handshake, opens a
// thread and runs its first turn. It returns a handle the caller uses to
// consume events and to clean up. `codex mcp-server`, used before, was
// removed in codex 0.154.0 (mg-xge.2).
//
// LaunchCodexApp requires `codex` on PATH, or MG_AGENT_CMD standing in for
// codex (see codexCommand). If neither is there the call returns
// ErrCodexUnavailable so callers can fall back to the tmux path.
func LaunchCodexApp(ctx context.Context, opts LaunchCodexAppOptions) (*CodexAppHandle, error) {
	if strings.TrimSpace(opts.Prompt) == "" {
		return nil, errors.New("agent: LaunchCodexApp requires a prompt")
	}

	transport, subproc, err := codexTransportFactory(opts)
	if err != nil {
		return nil, err
	}

	clientVersion := opts.ClientVersion
	if clientVersion == "" {
		clientVersion = "dev"
	}
	client, err := codexapp.Dial(ctx, transport, codexapp.WithClientVersion(clientVersion))
	if err != nil {
		var stderr string
		if subproc != nil {
			stderr = strings.Join(subproc.StderrLines(10), "\n")
		}
		if stderr != "" {
			return nil, fmt.Errorf("codex app-server handshake: %w (stderr: %s)", err, stderr)
		}
		return nil, fmt.Errorf("codex app-server handshake: %w", err)
	}

	sandbox := opts.Sandbox
	if sandbox == "" {
		sandbox = "workspace-write"
	}
	approval := opts.ApprovalPolicy
	if approval == "" {
		approval = "never"
	}

	// Detach the session from the caller's ctx. mg's launch path defer-cancels
	// the launch ctx once LaunchCodexApp returns, which would kill the
	// session before any event flows. Cancellation is via CodexAppHandle.Close.
	session, err := client.StartSession(context.Background(), codexapp.SessionOptions{
		Prompt:         opts.Prompt,
		Cwd:            opts.ProjectDir,
		Sandbox:        sandbox,
		ApprovalPolicy: approval,
		Model:          opts.Model,
		// StartSession fails closed when the server applies another reviewer.
		ApprovalsReviewer: approvalsReviewer(approval),
	})
	if err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("start codex session: %w", err)
	}

	return &CodexAppHandle{
		transport: subproc,
		client:    client,
		session:   session,
	}, nil
}

// approvalsReviewer is who answers a session's approval requests. on-request
// means a human is watching mg's approval modal, but a user's config.toml can
// route approvals to codex's own reviewer (approvals_reviewer =
// "guardian_subagent" or "auto_review"), which approves escalations without
// ever asking mg: the modal, the deny-list banner and the Jev reading never
// see them (mg-xge.4). Pin the reviewer to the user for those sessions.
func approvalsReviewer(approval string) string {
	if approval != "on-request" {
		return ""
	}
	return "user"
}

// ErrCodexUnavailable indicates that the codex binary is not on PATH and no
// MG_AGENT_CMD wrapper stands in for it.
var ErrCodexUnavailable = errors.New("agent: codex binary not on PATH")

// CodexLaunchable reports why a codex session cannot start, or nil if it
// can: the same resolution the launch uses (codex on PATH, or MG_AGENT_CMD
// standing in for codex), so the UI never offers a start that would fail.
func CodexLaunchable() error {
	_, err := codexCommand()
	return err
}
