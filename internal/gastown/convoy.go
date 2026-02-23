package gastown

import (
	"encoding/json"
	"fmt"
	"os/exec"
)

// ConvoyDetail is the rich convoy info from `gt convoy list --json` or `gt convoy status --json`.
type ConvoyDetail struct {
	ID        string             `json:"id"`
	Title     string             `json:"title"`
	Status    string             `json:"status"`
	Owned     bool               `json:"owned,omitempty"`
	Merge     string             `json:"merge_strategy,omitempty"`
	Tracked   []TrackedIssueInfo `json:"tracked"`
	Completed int                `json:"completed"`
	Total     int                `json:"total"`
}

// TrackedIssueInfo represents an issue tracked by a convoy.
type TrackedIssueInfo struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Status    string `json:"status"`
	IssueType string `json:"issue_type,omitempty"`
	Blocked   bool   `json:"blocked,omitempty"`
	Worker    string `json:"worker,omitempty"`
	WorkerAge string `json:"worker_age,omitempty"`
}

// ConvoyList fetches all convoys via `gt convoy list --json`.
func ConvoyList() ([]ConvoyDetail, error) {
	out, err := exec.Command("gt", "convoy", "list", "--json").Output()
	if err != nil {
		return nil, fmt.Errorf("gt convoy list: %w", err)
	}
	var convoys []ConvoyDetail
	if err := json.Unmarshal(out, &convoys); err != nil {
		return nil, fmt.Errorf("gt convoy list parse: %w", err)
	}
	return convoys, nil
}

// ConvoyStatus fetches detailed status for a single convoy.
func ConvoyStatus(convoyID string) (*ConvoyDetail, error) {
	out, err := exec.Command("gt", "convoy", "status", convoyID, "--json").Output()
	if err != nil {
		return nil, fmt.Errorf("gt convoy status: %w", err)
	}
	var detail ConvoyDetail
	if err := json.Unmarshal(out, &detail); err != nil {
		return nil, fmt.Errorf("gt convoy status parse: %w", err)
	}
	return &detail, nil
}

// ConvoyCreate creates a new convoy tracking the given issues.
// Returns the new convoy ID.
func ConvoyCreate(name string, issueIDs []string) (string, error) {
	args := []string{"convoy", "create", name}
	args = append(args, issueIDs...)
	out, err := exec.Command("gt", args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("gt convoy create: %w (%s)", err, string(out))
	}
	return string(out), nil
}

// ConvoyAdd adds issues to an existing convoy.
func ConvoyAdd(convoyID string, issueIDs []string) error {
	args := []string{"convoy", "add", convoyID}
	args = append(args, issueIDs...)
	out, err := exec.Command("gt", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("gt convoy add: %w (%s)", err, string(out))
	}
	return nil
}

// ConvoyClose closes a convoy.
func ConvoyClose(convoyID string) error {
	out, err := exec.Command("gt", "convoy", "close", convoyID).CombinedOutput()
	if err != nil {
		return fmt.Errorf("gt convoy close: %w (%s)", err, string(out))
	}
	return nil
}

// ConvoyLand lands an owned convoy (cleanup worktrees + close).
func ConvoyLand(convoyID string) error {
	out, err := exec.Command("gt", "convoy", "land", convoyID).CombinedOutput()
	if err != nil {
		return fmt.Errorf("gt convoy land: %w (%s)", err, string(out))
	}
	return nil
}
