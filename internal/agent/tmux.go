package agent

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/x/ansi"
)

// InTmux returns true if the current process is running inside a tmux session.
func InTmux() bool {
	return os.Getenv("TMUX") != ""
}

// TmuxAvailable returns true if the tmux binary is on PATH.
func TmuxAvailable() bool {
	_, err := exec.LookPath("tmux")
	return err == nil
}

// WindowName returns the tmux window name for a given issue ID.
func WindowName(issueID string) string {
	return "mg-" + issueID
}

// LaunchInTmux opens a new tmux pane running the agent to the right of the current pane.
func LaunchInTmux(prompt, projectDir, issueID string) (string, error) {
	paneName := WindowName(issueID)
	// Build agent command based on detected runtime. The binary comes from
	// agentCommand so MG_AGENT_CMD can interpose a wrapper, and it is absolute
	// for the reason documented there: tmux resolves the pane command against
	// the tmux SERVER's PATH, not this process's.
	rt := DetectRuntime()
	bin := agentCommand(rt)
	var agentArgs []string
	switch rt {
	case RuntimeCursor:
		agentArgs = []string{bin, "-f", "-p", prompt}
	case RuntimeCodex:
		// --no-alt-screen preserves tmux scrollback inside the split pane.
		agentArgs = []string{bin,
			"--no-alt-screen",
			"--sandbox", "workspace-write",
			"-a", "on-request",
			"-C", projectDir,
			prompt}
	default: // Claude Code
		agentArgs = []string{bin, "--teammate-mode", "tmux", prompt}
	}

	tmuxArgs := []string{"split-window",
		"-h",        // vertical split (pane to the right)
		"-l", "60%", // agent gets 60% of width
		"-d", // don't switch focus
		"-c", projectDir,
		"-P", "-F", "#{pane_id}", // print the new pane ID
		"--",
	}
	tmuxArgs = append(tmuxArgs, agentArgs...)
	cmd := exec.Command("tmux", tmuxArgs...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("tmux split-window: %w", err)
	}
	paneID := strings.TrimSpace(string(out))

	// Tag the pane with our naming convention so we can find it later.
	// tmux doesn't name panes, but we can set an environment variable.
	_ = exec.Command("tmux", "set-option", "-p", "-t", paneID,
		"@mg_agent", paneName).Run()

	return paneID, nil
}

// ListAgentWindows returns a map of issueID -> paneID for all tmux panes
// tagged with the @mg_agent option.
func ListAgentWindows() (map[string]string, error) {
	// List all panes with their @mg_agent value and pane_id.
	out, err := exec.Command("tmux", "list-panes", "-a",
		"-F", "#{@mg_agent}\t#{pane_id}").Output()
	if err != nil {
		return nil, fmt.Errorf("tmux list-panes: %w", err)
	}
	return parseAgentPanes(string(out)), nil
}

// parseAgentPanes extracts agent panes from tmux list-panes output.
// Each line is "mg-<issueID>\t%<paneNum>" for tagged panes, or "\t%<paneNum>" for untagged.
func parseAgentPanes(output string) map[string]string {
	agents := make(map[string]string)
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			continue
		}
		tag := strings.TrimSpace(parts[0])
		paneID := strings.TrimSpace(parts[1])
		if strings.HasPrefix(tag, "mg-") && paneID != "" {
			issueID := strings.TrimPrefix(tag, "mg-")
			agents[issueID] = paneID
		}
	}
	return agents
}

// KillAgentWindow closes the tmux pane for the given issue.
func KillAgentWindow(issueID string) error {
	// Find the pane ID first.
	agents, err := ListAgentWindows()
	if err != nil {
		return err
	}
	paneID, ok := agents[issueID]
	if !ok {
		return fmt.Errorf("no agent pane for %s", issueID)
	}
	return exec.Command("tmux", "kill-pane", "-t", paneID).Run()
}

// captureTimeout bounds one capture-pane call, so a wedged tmux server
// cannot leave captures blocked forever.
const captureTimeout = 2 * time.Second

// IsPaneID reports whether s is a tmux pane ID ("%12"). mg's agent map holds
// pane IDs for agents it launched in tmux, and agent names under an
// orchestrator, which have no pane mg can read.
func IsPaneID(s string) bool {
	return len(s) > 1 && s[0] == '%'
}

// CapturePane captures the last maxLines of output from a tmux pane.
// Returns sanitized lines (ANSI stripped, trailing blanks trimmed), or nil
// if the pane is gone or the capture fails or times out.
func CapturePane(paneID string, maxLines int) []string {
	ctx, cancel := context.WithTimeout(context.Background(), captureTimeout)
	defer cancel()
	// capture-pane -p prints to stdout, -S -N starts N lines from the end
	out, err := exec.CommandContext(ctx, "tmux", "capture-pane",
		"-t", paneID, "-p", "-S", fmt.Sprintf("-%d", maxLines+20)).Output()
	if err != nil {
		return nil
	}
	return sanitizeCaptureOutput(string(out), maxLines)
}

// sanitizeCaptureOutput strips ANSI codes, trims trailing blanks,
// and returns the last maxLines of non-empty content.
func sanitizeCaptureOutput(raw string, maxLines int) []string {
	// Strip ANSI escape sequences
	clean := stripANSI(raw)

	// Split into lines and trim trailing blanks
	allLines := strings.Split(clean, "\n")
	// Remove trailing empty lines
	for len(allLines) > 0 && strings.TrimSpace(allLines[len(allLines)-1]) == "" {
		allLines = allLines[:len(allLines)-1]
	}

	if len(allLines) == 0 {
		return nil
	}

	// Take last maxLines
	if len(allLines) > maxLines {
		allLines = allLines[len(allLines)-maxLines:]
	}

	return allLines
}

// stripANSI removes ANSI escape sequences and stray control characters from a string.
// Uses charmbracelet/x/ansi which handles all sequence types (CSI, OSC, DCS, etc.),
// then strips remaining C0/C1 control bytes that aren't part of escape sequences.
func stripANSI(s string) string {
	s = ansi.Strip(s)
	// Remove control characters (0x00-0x1F except \t, \n, \r) and DEL (0x7F).
	// These can leak from captured tmux output and should not reach the TUI.
	return strings.Map(func(r rune) rune {
		if r == '\t' || r == '\n' || r == '\r' {
			return r
		}
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, s)
}

// SelectAgentWindow switches focus to the tmux pane for the given issue.
func SelectAgentWindow(issueID string) error {
	agents, err := ListAgentWindows()
	if err != nil {
		return err
	}
	paneID, ok := agents[issueID]
	if !ok {
		return fmt.Errorf("no agent pane for %s", issueID)
	}
	return exec.Command("tmux", "select-pane", "-t", paneID).Run()
}
