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
)

// DetectRuntime returns the agent runtime to launch.
//
// If MG_AGENT_RUNTIME is set to "claude" or "cursor" (or "cursor-agent") and
// the corresponding binary is on PATH, that runtime wins. Unknown values or
// missing binaries fall through to the default detection order: claude first,
// then cursor-agent.
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
		}
	}
	if _, err := exec.LookPath("claude"); err == nil {
		return RuntimeClaude
	}
	if _, err := exec.LookPath("cursor-agent"); err == nil {
		return RuntimeCursor
	}
	return ""
}

// Available returns true if any supported agent CLI is on PATH.
func Available() bool {
	return DetectRuntime() != ""
}

// RuntimeLabel returns a display name for the runtime.
func (r Runtime) RuntimeLabel() string {
	switch r {
	case RuntimeClaude:
		return "Claude Code"
	case RuntimeCursor:
		return "Cursor"
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
func Command(prompt, projectDir string) *exec.Cmd {
	rt := DetectRuntime()
	var c *exec.Cmd
	switch rt {
	case RuntimeCursor:
		c = exec.Command("cursor-agent", "-f", "-p", prompt)
	default: // Claude Code
		c = exec.Command("claude", prompt)
	}
	c.Dir = projectDir
	return c
}
