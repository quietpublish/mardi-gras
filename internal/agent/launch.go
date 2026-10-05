// Package agent handles AI agent runtime detection and launch. It supports
// Claude Code and Cursor, with tmux window dispatch for multi-agent sessions.
package agent

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/matt-wright86/mardi-gras/internal/data"
)

// Runtime identifies which AI agent binary to use.
type Runtime string

const (
	RuntimeClaude Runtime = "claude"
	RuntimeCursor Runtime = "cursor-agent"
	RuntimeCodex  Runtime = "codex"
)

// AgentCommandEnv names an executable to launch INSTEAD of the runtime's own
// binary. The runtime's flags are still appended, so a wrapper sees exactly the
// argv the binary would have received and only has to forward it.
//
// It exists for setups where the agent must not be exec'd directly: a launcher
// that routes through a gateway, a sandbox, or a credential broker has to be
// the process mg starts, and no PATH arrangement can express that — see
// agentCommand for why the tmux path rules out a shim.
//
// The value is a single executable, not a command line: it is never split on
// spaces, so arguments cannot be passed here. A wrapper that needs fixed
// arguments should be a small script that adds them and execs the agent.
const AgentCommandEnv = "MG_AGENT_CMD"

// ResolveAgentCommand resolves an MG_AGENT_CMD value to the absolute path of
// an executable, or fails.
//
// A bare name is looked up on PATH, and a relative path is taken against the
// current directory — either way the result is absolute, because a relative
// argv[0] would be re-resolved later against whatever directory or PATH the
// process that finally execs it has (see agentCommand). A name found only via
// a relative PATH entry (exec.ErrDot) is accepted and made absolute for the
// same reason: the user put that entry there, and absolutizing it is what
// keeps it pointing at the same file.
func ResolveAgentCommand(value string) (string, error) {
	v := strings.TrimSpace(value)
	p, err := exec.LookPath(v)
	if err != nil && !errors.Is(err, exec.ErrDot) {
		return "", fmt.Errorf("%s=%q is not an executable: %w", AgentCommandEnv, v, err)
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", fmt.Errorf("%s=%q: %w", AgentCommandEnv, v, err)
	}
	return abs, nil
}

// wrapperConfigured returns the trimmed MG_AGENT_CMD value and whether it is
// set. A blank or whitespace-only value counts as unset.
func wrapperConfigured() (string, bool) {
	v := strings.TrimSpace(os.Getenv(AgentCommandEnv))
	return v, v != ""
}

// DetectRuntime returns the agent runtime to launch.
//
// If MG_AGENT_RUNTIME is set to "claude", "cursor" (or "cursor-agent"), or
// "codex" and the corresponding binary is on PATH, that runtime wins. Unknown
// values or missing binaries fall through to the default detection order:
// claude, then cursor-agent, then codex.
//
// A configured MG_AGENT_CMD counts as an agent in its own right: it is what
// gets exec'd, so whether a runtime's binary is on PATH no longer decides
// anything. The runtime then only picks the flag set the wrapper receives —
// MG_AGENT_RUNTIME if it names one, else whatever the PATH order finds, else
// claude. This is what lets a sandbox that exposes nothing but the wrapper
// launch agents at all.
func DetectRuntime() Runtime {
	pref := strings.ToLower(strings.TrimSpace(os.Getenv("MG_AGENT_RUNTIME")))
	_, wrapped := wrapperConfigured()
	switch pref {
	case "claude":
		if wrapped || onPath("claude") {
			return RuntimeClaude
		}
	case "cursor", "cursor-agent":
		if wrapped || onPath("cursor-agent") {
			return RuntimeCursor
		}
	case "codex":
		if wrapped || onPath("codex") {
			return RuntimeCodex
		}
	}
	if onPath("claude") {
		return RuntimeClaude
	}
	if onPath("cursor-agent") {
		return RuntimeCursor
	}
	if onPath("codex") {
		return RuntimeCodex
	}
	if wrapped {
		return RuntimeClaude
	}
	return ""
}

func onPath(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// Available returns true if any supported agent CLI is on PATH, or
// MG_AGENT_CMD names a wrapper to launch instead.
func Available() bool {
	return DetectRuntime() != ""
}

// agentCommand returns the absolute path of the executable to launch for rt:
// MG_AGENT_CMD when set, else rt's own binary (claude when rt is empty).
//
// The path is ABSOLUTE in both cases. That is what makes the tmux dispatch
// path work: LaunchInTmux hands argv to `tmux split-window`, and the tmux
// SERVER resolves the pane command against the server's own PATH, not this
// process's. A bare name could therefore run a different binary than the one
// resolved here, or none at all, leaving a pane that exits instantly with no
// error mg can see.
//
// It fails closed. A wrapper that is configured but not executable is an
// error, never a silent fallback to the runtime's binary: the wrapper may be
// the only thing standing between the agent and credentials or the network,
// and a warning on stderr would be invisible behind the TUI's alt screen. A
// missing runtime binary is an error for the same reason it was always fatal
// in practice — there is nothing to launch — but now it says so.
func agentCommand(rt Runtime) (string, error) {
	if v, ok := wrapperConfigured(); ok {
		return ResolveAgentCommand(v)
	}
	bin := string(rt)
	if bin == "" {
		bin = string(RuntimeClaude)
	}
	p, err := exec.LookPath(bin)
	if err != nil {
		return "", fmt.Errorf("%s not found on PATH: %w", bin, err)
	}
	return filepath.Abs(p)
}

// agentArgv returns the full argv that launches rt as bin with prompt in
// projectDir. inTmux adds the flags that only make sense inside a tmux pane.
// Command and LaunchInTmux both build their argv here, so the two launch paths
// cannot drift apart.
func agentArgv(rt Runtime, bin, prompt, projectDir string, inTmux bool) []string {
	switch rt {
	case RuntimeCursor:
		return []string{bin, "-f", "-p", prompt}
	case RuntimeCodex:
		argv := []string{bin}
		if inTmux {
			// --no-alt-screen preserves tmux scrollback inside the split pane.
			argv = append(argv, "--no-alt-screen")
		}
		return append(argv,
			"--sandbox", "workspace-write",
			"-a", "on-request",
			"-C", projectDir,
			prompt)
	default: // Claude Code
		argv := []string{bin}
		if inTmux {
			argv = append(argv, "--teammate-mode", "tmux")
		}
		return append(argv, prompt)
	}
}

// codexCommand returns the absolute path to exec for the codex-only launches,
// `codex resume` and the `codex app-server` behind M.
//
// Those take codex's own subcommands, so MG_AGENT_CMD is used only when it
// stands in for codex, i.e. the runtime is codex. A wrapper configured for
// another runtime is refused rather than bypassed: launching codex directly
// would skip whatever the wrapper exists to enforce, and handing it codex
// subcommands would be wrong for a wrapper built around a different agent.
// With no wrapper, codex must be on PATH, or ErrCodexUnavailable.
func codexCommand() (string, error) {
	if _, ok := wrapperConfigured(); ok {
		if rt := DetectRuntime(); rt != RuntimeCodex {
			return "", fmt.Errorf("%s stands in for %s, not codex; set --agent codex to route codex through it",
				AgentCommandEnv, rt)
		}
		return agentCommand(RuntimeCodex)
	}
	p, err := exec.LookPath("codex")
	if err != nil {
		return "", ErrCodexUnavailable
	}
	return filepath.Abs(p)
}

// RuntimeLabel returns a display name for the runtime.
func (r Runtime) RuntimeLabel() string {
	switch r {
	case RuntimeClaude:
		return "Claude Code"
	case RuntimeCursor:
		return "Cursor"
	case RuntimeCodex:
		return "Codex"
	default:
		return "unknown"
	}
}

// BuildPrompt composes the initial prompt for a Claude Code session
// given a selected issue and its evaluated dependencies.
func BuildPrompt(issue data.Issue, deps data.DepEval, issueMap map[string]*data.Issue) string {
	var b strings.Builder

	b.WriteString("Work on this Beads issue:\n\n")
	fmt.Fprintf(&b, "## %s: %s\n\n", issue.ID, issue.Title)

	fmt.Fprintf(&b, "Status: %s | Type: %s | Priority: %s\n",
		issue.Status, issue.IssueType, data.PriorityLabel(issue.Priority))
	if issue.Owner != "" {
		fmt.Fprintf(&b, "Owner: %s\n", issue.Owner)
	}
	if issue.Assignee != "" {
		fmt.Fprintf(&b, "Assignee: %s\n", issue.Assignee)
	}

	if issue.Description != "" {
		fmt.Fprintf(&b, "\n%s\n", issue.Description)
	}

	if issue.Notes != "" {
		fmt.Fprintf(&b, "\n### Notes\n%s\n", issue.Notes)
	}

	if issue.AcceptanceCriteria != "" {
		fmt.Fprintf(&b, "\n### Acceptance Criteria\n%s\n", issue.AcceptanceCriteria)
	}

	if len(deps.Edges) > 0 {
		b.WriteString("\n### Dependencies\n")
		for _, edge := range deps.Edges {
			switch edge.Status {
			case data.DepBlocking:
				if dep, ok := issueMap[edge.DependsOnID]; ok {
					fmt.Fprintf(&b, "- Blocked by: %s (%s) -- %s\n",
						edge.DependsOnID, dep.Title, dep.Status)
				}
			case data.DepMissing:
				fmt.Fprintf(&b, "- Missing: %s (not found)\n", edge.DependsOnID)
			case data.DepResolved:
				if dep, ok := issueMap[edge.DependsOnID]; ok {
					fmt.Fprintf(&b, "- Resolved: %s (%s) -- closed\n",
						edge.DependsOnID, dep.Title)
				}
			case data.DepNonBlocking:
				if dep, ok := issueMap[edge.DependsOnID]; ok {
					fmt.Fprintf(&b, "- Related: %s (%s) -- %s\n",
						edge.DependsOnID, dep.Title, edge.Type)
				}
			}
		}
	}

	fmt.Fprintf(&b, "\n---\nWhen you begin work, run: bd update %s --status=in_progress\n", issue.ID)
	fmt.Fprintf(&b, "When finished, run: bd close %s\n", issue.ID)
	b.WriteString("\nIf this task is complex enough to benefit from parallel work, consider using agent teams to spawn teammates for independent subtasks.")

	return b.String()
}

// Command returns an *exec.Cmd that launches the detected agent runtime
// with the given prompt, working directory set to projectDir.
//
// Codex defaults to sandboxed execution with interactive approval prompts,
// which would block unattended agent sessions. We pass --sandbox workspace-write
// and -a on-request to match the zero-friction posture Claude and Cursor have
// out of the box. Power users can override via codex profiles or
// MG_AGENT_RUNTIME=codex combined with a custom shell alias.
//
// The binary is agentCommand(rt): the runtime's own, unless MG_AGENT_CMD
// interposes a wrapper. An error means there is nothing safe to launch.
func Command(prompt, projectDir string) (*exec.Cmd, error) {
	rt := DetectRuntime()
	bin, err := agentCommand(rt)
	if err != nil {
		return nil, err
	}
	argv := agentArgv(rt, bin, prompt, projectDir, false)
	c := exec.Command(argv[0], argv[1:]...) //nolint:gosec // argv[0] is the resolved agent or the user's own MG_AGENT_CMD
	c.Dir = projectDir
	return c, nil
}
