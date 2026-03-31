package gastown

import (
	"encoding/json"
	"fmt"
)

// PatrolScanResult represents the output of `gt patrol scan --json`.
type PatrolScanResult struct {
	Rig         string         `json:"rig"`
	Timestamp   string         `json:"timestamp"`
	Zombies     PatrolFinding  `json:"zombies"`
	Stalls      PatrolFinding  `json:"stalls"`
	Completions PatrolFinding  `json:"completions"`
	Details     []PatrolDetail `json:"details,omitempty"`
}

// PatrolFinding is a checked/found pair from patrol scan.
type PatrolFinding struct {
	Checked int `json:"checked"`
	Found   int `json:"found"`
}

// PatrolDetail provides specifics about a patrol finding (agent identity, work info).
// The shape of this struct will be refined once we capture output from a rig with
// active agents — the current implementation handles it being absent.
type PatrolDetail struct {
	Type     string `json:"type"`      // "zombie", "stall", "completion"
	Agent    string `json:"agent"`     // agent name
	Rig      string `json:"rig"`       // rig name
	Role     string `json:"role"`      // agent role
	HookBead string `json:"hook_bead"` // attached work
	Detail   string `json:"detail"`    // human-readable description
}

// FetchPatrolScan runs `gt patrol scan --json` and returns the parsed result.
func FetchPatrolScan() (*PatrolScanResult, error) {
	out, err := runWithTimeout(timeoutMedium, "gt", "patrol", "scan", "--json")
	if err != nil {
		return nil, fmt.Errorf("gt patrol scan: %w", err)
	}
	var result PatrolScanResult
	if err := json.Unmarshal(out, &result); err != nil {
		return nil, fmt.Errorf("gt patrol scan parse: %w", err)
	}
	return &result, nil
}

// PatrolScanProblems converts patrol scan findings into Problem entries.
// Only emits problems when findings are non-zero. Returns nil if scan is nil.
func PatrolScanProblems(scan *PatrolScanResult) []Problem {
	if scan == nil {
		return nil
	}

	var problems []Problem

	if scan.Zombies.Found > 0 {
		detail := fmt.Sprintf("Patrol detected %d zombie(s) across %d checked agents", scan.Zombies.Found, scan.Zombies.Checked)
		problems = append(problems, Problem{
			Type:     "patrol_zombie",
			Detail:   detail,
			Severity: "error",
			RigName:  scan.Rig,
		})
	}

	if scan.Stalls.Found > 0 {
		detail := fmt.Sprintf("Patrol detected %d stall(s) across %d checked agents", scan.Stalls.Found, scan.Stalls.Checked)
		problems = append(problems, Problem{
			Type:     "patrol_stall",
			Detail:   detail,
			Severity: "warn",
			RigName:  scan.Rig,
		})
	}

	// Enrich with individual detail entries if available
	for _, d := range scan.Details {
		prob := Problem{
			Detail:   d.Detail,
			Severity: "warn",
			RigName:  d.Rig,
			Agent: AgentRuntime{
				Name:     d.Agent,
				Role:     d.Role,
				Rig:      d.Rig,
				HookBead: d.HookBead,
			},
		}
		switch d.Type {
		case "zombie":
			prob.Type = "patrol_zombie"
			prob.Severity = "error"
		case "stall":
			prob.Type = "patrol_stall"
		case "completion":
			continue // completions are informational, not problems
		default:
			prob.Type = "patrol_" + d.Type
		}
		problems = append(problems, prob)
	}

	return problems
}
