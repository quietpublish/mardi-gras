package agent

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"sync"

	"github.com/matt-wright86/mardi-gras/internal/codexmcp"
)

// CodexMCPHandle owns the lifecycle of one codex MCP subprocess plus the
// most recent session running against it. Callers must call Close when done;
// mg's app closes all handles on quit. Replies rotate the session pointer
// (see Reply); the underlying subprocess is reused across replies.
type CodexMCPHandle struct {
	transport *codexmcp.SubprocessTransport
	client    *codexmcp.Client
	session   *codexmcp.Session

	closeOnce sync.Once
	closeErr  error
}

// Session returns the most recent codexmcp.Session attached to this handle.
// After Reply rotates the session, Session() returns the new one.
func (h *CodexMCPHandle) Session() *codexmcp.Session { return h.session }

// Reply continues the conversation by invoking codex-reply with the given
// prompt against the threadID captured from the original session. The new
// session becomes h.Session(); the old session has already terminated by the
// time the caller is allowed to call Reply (mg gates the call on the prior
// session's terminal Done, per #47's v0 design).
//
// The ctx parameter is currently unused for the session lifetime — like
// LaunchCodexMCP, Reply detaches the session from the caller's ctx so a
// defer-cancel in the dispatch goroutine doesn't kill the session before
// any reply event is rendered (the same trap v0.21.1 fixed on the launch
// path). ctx is reserved for a future setup-only timeout if needed.
func (h *CodexMCPHandle) Reply(ctx context.Context, prompt string) (*codexmcp.Session, error) {
	_ = ctx // reserved; intentionally not propagated to StartReplySession
	if h.client == nil {
		return nil, errors.New("agent: CodexMCPHandle has no client (already closed?)")
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
func (h *CodexMCPHandle) Close() error {
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
func (h *CodexMCPHandle) StderrTail(n int) []string {
	if h.transport == nil {
		return nil
	}
	return h.transport.StderrLines(n)
}

// LaunchCodexMCPOptions controls how an MCP-backed codex session is launched.
type LaunchCodexMCPOptions struct {
	// Prompt is the initial user prompt. Required.
	Prompt string
	// ProjectDir is the working directory for the subprocess and the codex
	// session's cwd argument.
	ProjectDir string
	// Sandbox overrides codex's sandbox mode. Defaults to "workspace-write" to
	// match the tmux-launched path in agent.Command.
	Sandbox string
	// ApprovalPolicy overrides codex's approval policy. Defaults to "never"
	// because mg can't currently route exec_approval_request events to a
	// user-visible prompt.
	ApprovalPolicy string
	// Model optionally overrides the codex model.
	Model string
	// ClientVersion is advertised to the server in initialize. Defaults to
	// "dev".
	ClientVersion string
}

// codexTransportFactory is the function used to spawn the codex MCP transport.
// Tests override this to inject a pipe-based transport without a real codex
// binary.
var codexTransportFactory = func(opts LaunchCodexMCPOptions) (codexmcp.Transport, *codexmcp.SubprocessTransport, error) {
	if _, err := exec.LookPath("codex"); err != nil {
		return nil, nil, ErrCodexUnavailable
	}
	t, err := codexmcp.SpawnSubprocess(codexmcp.WithDir(opts.ProjectDir))
	if err != nil {
		return nil, nil, fmt.Errorf("spawn codex mcp-server: %w", err)
	}
	return t, t, nil
}

// LaunchCodexMCP spawns `codex mcp-server`, performs the MCP handshake, and
// starts a session against the codex tool. It returns a handle the caller
// uses to consume events and to clean up.
//
// LaunchCodexMCP requires `codex` on PATH. If the binary is missing the call
// returns ErrCodexUnavailable so callers can fall back to the tmux path.
func LaunchCodexMCP(ctx context.Context, opts LaunchCodexMCPOptions) (*CodexMCPHandle, error) {
	if strings.TrimSpace(opts.Prompt) == "" {
		return nil, errors.New("agent: LaunchCodexMCP requires a prompt")
	}

	transport, subproc, err := codexTransportFactory(opts)
	if err != nil {
		return nil, err
	}

	clientVersion := opts.ClientVersion
	if clientVersion == "" {
		clientVersion = "dev"
	}
	client, err := codexmcp.Dial(ctx, transport, codexmcp.WithClientVersion(clientVersion))
	if err != nil {
		var stderr string
		if subproc != nil {
			stderr = strings.Join(subproc.StderrLines(10), "\n")
		}
		if stderr != "" {
			return nil, fmt.Errorf("codex mcp handshake: %w (stderr: %s)", err, stderr)
		}
		return nil, fmt.Errorf("codex mcp handshake: %w", err)
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
	// the launch ctx once LaunchCodexMCP returns, which would kill the
	// session before any event flows. Cancellation is via CodexMCPHandle.Close.
	session, err := client.StartSession(context.Background(), codexmcp.SessionOptions{
		Prompt:         opts.Prompt,
		Cwd:            opts.ProjectDir,
		Sandbox:        sandbox,
		ApprovalPolicy: approval,
		Model:          opts.Model,
	})
	if err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("start codex session: %w", err)
	}

	return &CodexMCPHandle{
		transport: subproc,
		client:    client,
		session:   session,
	}, nil
}

// ErrCodexUnavailable indicates that the codex binary is not on PATH.
var ErrCodexUnavailable = errors.New("agent: codex binary not on PATH")
