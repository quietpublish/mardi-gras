// Package agent handles AI agent runtime detection and launch. It supports
// Claude Code and Cursor, with tmux window dispatch for multi-agent sessions.
package agent

import (
	"fmt"
	"os"
	"os/exec"
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
const AgentCommandEnv = "MG_AGENT_CMD"

// DetectRuntime returns the agent runtime to launch.
//
// If MG_AGENT_RUNTIME is set to "claude", "cursor" (or "cursor-agent"), or
// "codex" and the corresponding binary is on PATH, that runtime wins. Unknown
// values or missing binaries fall through to the default detection order:
// claude, then cursor-agent, then codex.
func DetectRuntime() Runtime {
	if pref := strings.ToLower(strings.TrimSpace(os.Getenv("MG_AGENT_RUNTIME"))); pref != "" {
		switch pref {
		case "claude":
			if _, err := exec.LookPath("claude"); err == nil {
				return RuntimeClaude
			}
		case "cursor", "cursor-agent":
			if _, err := exec.LookPath("cursor-agent"); err == nil {
				return RuntimeCursor
			}
		case "codex":
			if _, err := exec.LookPath("codex"); err == nil {
				return RuntimeCodex
			}
		}
	}
	if _, err := exec.LookPath("claude"); err == nil {
		return RuntimeClaude
	}
	if _, err := exec.LookPath("cursor-agent"); err == nil {
		return RuntimeCursor
	}
	if _, err := exec.LookPath("codex"); err == nil {
		return RuntimeCodex
	}
	return ""
}

// Available returns true if any supported agent CLI is on PATH.
func Available() bool {
	return DetectRuntime() != ""
}

// agentCommand returns the executable to launch for rt.
//
// With MG_AGENT_CMD unset it returns rt's own binary name — or claude when no
// runtime was detected, which is what the hardcoded command did — so the
// default behaviour is unchanged.
//
// When MG_AGENT_CMD IS set its value is resolved with exec.LookPath and the
// ABSOLUTE path is returned. That is what makes the tmux dispatch path work:
// LaunchInTmux hands the name to `tmux split-window`, and the tmux SERVER
// resolves the pane command against the server's own PATH, not this process's.
// A bare name could therefore run a different binary than the one resolved
// here, or none at all, leaving a pane that exits instantly with no error mg
// can see.
//
// A wrapper that is configured but not executable is reported on stderr and
// the runtime's binary is launched instead, so a typo degrades to the previous
// behaviour rather than breaking agent launch outright.
func agentCommand(rt Runtime) string {
	bin := string(rt)
	if bin == "" {
		// Nothing detected on PATH. Name claude anyway: that is what the
		// hardcoded command did, and an empty argv[0] would make tmux's
		// split-window fail outright.
		bin = string(RuntimeClaude)
	}
	if cmd := strings.TrimSpace(os.Getenv(AgentCommandEnv)); cmd != "" {
		if abs, err := exec.LookPath(cmd); err == nil {
			return abs
		}
		fmt.Fprintf(os.Stderr, "mg: %s=%q is not executable; launching %q directly\n",
			AgentCommandEnv, cmd, bin)
	}
	return bin
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
// The binary is agentCommand(rt): the runtime's own name unless MG_AGENT_CMD
// interposes a wrapper.
func Command(prompt, projectDir string) *exec.Cmd {
	rt := DetectRuntime()
	bin := agentCommand(rt)
	var c *exec.Cmd
	switch rt {
	case RuntimeCursor:
		c = exec.Command(bin, "-f", "-p", prompt)
	case RuntimeCodex:
		c = exec.Command(bin,
			"--sandbox", "workspace-write",
			"-a", "on-request",
			"-C", projectDir,
			prompt)
	default: // Claude Code
		c = exec.Command(bin, prompt)
	}
	c.Dir = projectDir
	return c
}
