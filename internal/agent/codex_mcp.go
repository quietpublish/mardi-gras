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
// session running against it. Callers must call Close when done; mg's app
// closes all handles on quit.
type CodexMCPHandle struct {
	transport *codexmcp.SubprocessTransport
	client    *codexmcp.Client
	session   *codexmcp.Session

	closeOnce sync.Once
	closeErr  error
}

// Session returns the underlying codexmcp.Session for event/done channels.
func (h *CodexMCPHandle) Session() *codexmcp.Session { return h.session }

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
