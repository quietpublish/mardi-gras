package agent

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
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

// LaunchInTmux opens a new tmux pane running claude to the right of the current pane.
func LaunchInTmux(prompt, projectDir, issueID string) (string, error) {
	paneName := WindowName(issueID)
	cmd := exec.Command("tmux", "split-window",
		"-h",        // vertical split (pane to the right)
		"-l", "60%", // agent gets 60% of width
		"-d", // don't switch focus
		"-c", projectDir,
		"-P", "-F", "#{pane_id}", // print the new pane ID
		"--", "claude", "--teammate-mode", "tmux", prompt,
	)
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
