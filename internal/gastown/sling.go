package gastown

import (
	"fmt"
	"os/exec"
	"strings"
)

// Sling dispatches work via `gt sling`. This is Gas Town's core primitive --
// it auto-spawns polecats, auto-creates convoys, and starts the work lifecycle.
func Sling(issueID string) error {
	return exec.Command("gt", "sling", issueID).Run()
}

// SlingWithFormula dispatches work using a named formula.
// e.g., SlingWithFormula("bd-a1b2", "shiny") runs the full
// design->implement->review->test->submit workflow.
func SlingWithFormula(issueID, formula string) error {
	return exec.Command("gt", "sling", "--formula", formula, issueID).Run()
}

// ListFormulas returns available formula names by parsing `gt formula list`.
func ListFormulas() ([]string, error) {
	out, err := exec.Command("gt", "formula", "list").Output()
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
	return exec.Command("gt", "unsling", issueID).Run()
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
	args := []string{"nudge", target}
	if message != "" {
		args = append(args, message)
	}
	return exec.Command("gt", args...).Run()
}
