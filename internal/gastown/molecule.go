package gastown

import (
	"encoding/json"
	"fmt"
)

// DAGNode represents a single step in a molecule DAG.
type DAGNode struct {
	ID           string   `json:"id"`
	Title        string   `json:"title"`
	Status       string   `json:"status"` // "done", "in_progress", "ready", "blocked", "open", "closed"
	Parallel     bool     `json:"parallel,omitempty"`
	Dependencies []string `json:"dependencies,omitempty"`
	Dependents   []string `json:"dependents,omitempty"`
	Tier         int      `json:"tier"`
}

// DAGInfo represents the full molecule dependency graph.
type DAGInfo struct {
	RootID       string              `json:"root_id"`
	RootTitle    string              `json:"root_title"`
	TotalNodes   int                 `json:"total_nodes"`
	Tiers        int                 `json:"tiers"`
	CriticalPath []string            `json:"critical_path,omitempty"`
	Nodes        map[string]*DAGNode `json:"nodes"`
	TierGroups   [][]string          `json:"tier_groups"`
}

// MoleculeProgress represents molecule completion stats.
type MoleculeProgress struct {
	RootID       string   `json:"root_id"`
	RootTitle    string   `json:"root_title"`
	MoleculeID   string   `json:"molecule_id,omitempty"`
	TotalSteps   int      `json:"total_steps"`
	DoneSteps    int      `json:"done_steps"`
	InProgress   int      `json:"in_progress_steps"`
	ReadySteps   []string `json:"ready_steps"`
	BlockedSteps []string `json:"blocked_steps"`
	Percent      int      `json:"percent_complete"`
	Complete     bool     `json:"complete"`
}

// StepDoneResult represents the result of marking a step as done.
type StepDoneResult struct {
	StepID        string   `json:"step_id"`
	MoleculeID    string   `json:"molecule_id"`
	StepClosed    bool     `json:"step_closed"`
	NextStepID    string   `json:"next_step_id,omitempty"`
	NextStepTitle string   `json:"next_step_title,omitempty"`
	ParallelSteps []string `json:"parallel_steps,omitempty"`
	Complete      bool     `json:"complete"`
	Action        string   `json:"action"` // "continue", "parallel", "done", "no_more_ready"
}

// ActiveStepID returns the ID of the first in_progress step, or the first ready step.
// Returns empty string if no actionable step is found.
func (d *DAGInfo) ActiveStepID() string {
	if d == nil || len(d.Nodes) == 0 {
		return ""
	}
	// Prefer in_progress steps
	for _, node := range d.Nodes {
		if node.Status == "in_progress" {
			return node.ID
		}
	}
	// Fall back to first ready step (by tier order)
	for _, tier := range d.TierGroups {
		for _, id := range tier {
			if n, ok := d.Nodes[id]; ok && n.Status == "ready" {
				return n.ID
			}
		}
	}
	return ""
}

// MoleculeDAG fetches the molecule DAG for a root issue via `gt mol dag <id> --json`.
func MoleculeDAG(rootID string) (*DAGInfo, error) {
	out, err := runWithTimeout(TimeoutMedium, "gt", "mol", "dag", rootID, "--json")
	if err != nil {
		return nil, fmt.Errorf("gt mol dag: %w", err)
	}
	var dag DAGInfo
	if err := json.Unmarshal(out, &dag); err != nil {
		return nil, fmt.Errorf("gt mol dag parse: %w", err)
	}
	return &dag, nil
}

// MoleculeProgressFetch fetches molecule progress via `gt mol progress <id> --json`.
func MoleculeProgressFetch(rootID string) (*MoleculeProgress, error) {
	out, err := runWithTimeout(TimeoutMedium, "gt", "mol", "progress", rootID, "--json")
	if err != nil {
		return nil, fmt.Errorf("gt mol progress: %w", err)
	}
	var prog MoleculeProgress
	if err := json.Unmarshal(out, &prog); err != nil {
		return nil, fmt.Errorf("gt mol progress parse: %w", err)
	}
	return &prog, nil
}

// MoleculeStepDone marks a step as done via `gt mol step done <id> --json`.
func MoleculeStepDone(stepID string) (*StepDoneResult, error) {
	out, err := runWithTimeout(TimeoutShort, "gt", "mol", "step", "done", stepID, "--json")
	if err != nil {
		return nil, fmt.Errorf("gt mol step done: %w", err)
	}
	var result StepDoneResult
	if err := json.Unmarshal(out, &result); err != nil {
		return nil, fmt.Errorf("gt mol step done parse: %w", err)
	}
	return &result, nil
}
