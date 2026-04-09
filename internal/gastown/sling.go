package gastown

import (
	"fmt"
	"os/exec"
	"strings"
)

// Sling dispatches work via `gt sling`. This is Gas Town's core primitive --
// it auto-spawns polecats, auto-creates convoys, and starts the work lifecycle.
func Sling(issueID string) error {
	if err := validateIssueID(issueID); err != nil {
		return err
	}
	return execWithTimeout(timeoutShort, "gt", "sling", issueID)
}

// SlingWithFormula dispatches work using a named formula.
// e.g., SlingWithFormula("bd-a1b2", "shiny") runs the full
// design->implement->review->test->submit workflow.
func SlingWithFormula(issueID, formula string) error {
	if err := validateIssueID(issueID); err != nil {
		return err
	}
	formula = sanitizeText(formula, maxTextLen)
	return execWithTimeout(timeoutShort, "gt", "sling", formula, "--on", issueID)
}

// ListFormulas returns available formula names by parsing `gt formula list`.
func ListFormulas() ([]string, error) {
	out, err := runWithTimeout(timeoutShort, "gt", "formula", "list")
	if err != nil {
		return nil, fmt.Errorf("gt formula list: %w", err)
	}
	var names []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			names = append(names, line)
		}
	}
	return names, nil
}

// Unsling stops work dispatched via `gt sling`.
func Unsling(issueID string) error {
	if err := validateIssueID(issueID); err != nil {
		return err
	}
	return execWithTimeout(timeoutShort, "gt", "unsling", issueID)
}

// SlingMultiple dispatches multiple issues sequentially.
func SlingMultiple(issueIDs []string) error {
	for _, id := range issueIDs {
		if err := Sling(id); err != nil {
			return fmt.Errorf("sling %s: %w", id, err)
		}
	}
	return nil
}

// SlingMultipleWithFormula dispatches multiple issues with a named formula.
func SlingMultipleWithFormula(issueIDs []string, formula string) error {
	for _, id := range issueIDs {
		if err := SlingWithFormula(id, formula); err != nil {
			return fmt.Errorf("sling %s with %s: %w", id, formula, err)
		}
	}
	return nil
}

// Nudge sends a wake-up message to the agent working on the given issue.
func Nudge(target, message string) error {
	target = sanitizeText(target, maxTextLen)
	args := []string{"nudge", target}
	if message != "" {
		message = sanitizeText(message, maxTextLen)
		args = append(args, "--", message)
	}
	return execWithTimeout(timeoutShort, "gt", args...)
}

// HandoffInTmux launches `gt handoff <target>` in a new tmux pane.
// Handoff is interactive (starts a new agent session), so it can't run inline.
func HandoffInTmux(target, projectDir string) (string, error) {
	cmd := exec.Command("tmux", "split-window",
		"-h",
		"-l", "60%",
		"-d",
		"-c", projectDir,
		"-P", "-F", "#{pane_id}",
		"--", "gt", "handoff", target,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("tmux split-window for handoff: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// Decommission kills a polecat process by its address.
func Decommission(address string) error {
	return execWithTimeout(timeoutShort, "gt", "polecat", "kill", address)
}

// CascadeClose closes an issue and all its children via `gt close --cascade`.
// Requires Gas Town v0.11.0+.
func CascadeClose(issueID string) error {
	if err := validateIssueID(issueID); err != nil {
		return err
	}
	return execWithTimeout(timeoutShort, "gt", "close", "--cascade", issueID)
}
