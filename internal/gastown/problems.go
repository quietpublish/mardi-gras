package gastown

import "fmt"

// Problem represents a detected issue with a Gas Town agent or beads infrastructure.
type Problem struct {
	Type     string          // "stalled", "backoff", "zombie", "dead_rig", "doctor"
	Agent    AgentRuntime    // the affected agent (zero value for rig-level/doctor problems)
	Detail   string          // human-readable description
	Severity string          // "warn", "error"
	Category string          // doctor category (e.g. "Core System", "Git Integration")
	Fix      string          // suggested fix command, if any
	RigName  string          // rig name for rig-level problems
	Orphans  []OrphanedIssue // orphaned issues for dead_rig problems
}

// DetectProblems analyzes TownStatus and returns any detected problems.
// All heuristics are stateless — derived from a single status snapshot.
//
// Dead-rig detection groups orphaned agents under a single "dead_rig"
// problem instead of emitting individual "zombie" entries, reducing
// alarm fatigue when an entire rig is down.
func DetectProblems(status *TownStatus) []Problem {
	if status == nil {
		return nil
	}

	// Identify dead rigs (0 polecats + orphaned work) so we can
	// suppress individual zombie problems for agents on those rigs.
	deadRigs := make(map[string]bool)
	for _, rigName := range FindDeadRigs(status) {
		deadRigs[rigName] = true
	}

	var problems []Problem

	// Emit dead_rig problems first (highest severity).
	for rigName := range deadRigs {
		orphans := FindOrphans(status, rigName)
		detail := fmt.Sprintf("Rig has 0 polecats — %d issues left without an agent", len(orphans))
		problems = append(problems, Problem{
			Type:     "dead_rig",
			Detail:   detail,
			Severity: "error",
			RigName:  rigName,
			Orphans:  orphans,
			Fix:      "gt sling <issue> " + rigName,
		})
	}

	for _, a := range status.Agents {
		// Stalled: agent is running, has work, but idle (should be working)
		if a.HasWork && a.State == "idle" {
			problems = append(problems, Problem{
				Type:     "stalled",
				Agent:    a,
				Detail:   "Has work but idle — may need nudge",
				Severity: "warn",
			})
		}

		// Stuck: agent explicitly requesting help
		if a.State == "stuck" {
			problems = append(problems, Problem{
				Type:     "stuck",
				Agent:    a,
				Detail:   "Agent is stuck and requesting help",
				Severity: "error",
			})
		}

		// Backoff spiral: agent is in backoff state
		if a.State == "backoff" {
			problems = append(problems, Problem{
				Type:     "backoff",
				Agent:    a,
				Detail:   "In backoff state — may be stuck in retry loop",
				Severity: "warn",
			})
		}

		// Zombie: agent not running but has hooked work.
		// Skip agents on dead rigs — they are already covered by dead_rig.
		if !a.Running && a.HookBead != "" && !deadRigs[a.Rig] {
			problems = append(problems, Problem{
				Type:     "zombie",
				Agent:    a,
				Detail:   "Not running but has hooked work (" + a.HookBead + ")",
				Severity: "error",
			})
		}
	}

	return problems
}

// DoctorProblems converts bd doctor diagnostics into Problem entries.
// Only error and warning diagnostics are included (not passed checks).
func DoctorProblems(diagnostics []DoctorDiagnostic) []Problem {
	var problems []Problem
	for _, d := range diagnostics {
		if d.Status == "ok" {
			continue
		}
		sev := "warn"
		if d.Status == "error" {
			sev = "error"
		}
		fix := ""
		if len(d.Commands) > 0 {
			fix = d.Commands[0]
		}
		problems = append(problems, Problem{
			Type:     "doctor",
			Detail:   d.Name + ": " + d.Explanation,
			Severity: sev,
			Category: d.Category,
			Fix:      fix,
		})
	}
	return problems
}

// DoctorDiagnostic mirrors data.DoctorDiagnostic for use within gastown package.
type DoctorDiagnostic struct {
	Name        string   `json:"name"`
	Status      string   `json:"status"`
	Category    string   `json:"category"`
	Explanation string   `json:"explanation"`
	Commands    []string `json:"commands"`
}

// IsStandstill returns whether an agent is in a standstill state that
// requires human intervention, and the reason string for the state.
// Standstill states: stuck, awaiting-gate, fix_needed, and the stalled
// heuristic (has work but idle). Backoff is excluded — it self-resolves.
//
// This is intentionally a superset of DetectProblems: awaiting-gate and
// fix_needed are user-actionable but not system problems, so they are
// surfaced via the parade standstill indicator only (not the Problems panel).
func IsStandstill(agent AgentRuntime) (bool, string) {
	switch agent.State {
	case "stuck":
		return true, "stuck"
	case "awaiting-gate":
		return true, "awaiting-gate"
	case "fix_needed":
		return true, "fix_needed"
	case "idle":
		if agent.HasWork {
			return true, "stalled"
		}
	}
	return false, ""
}

// BuildStandstillIDs returns a map of issue ID → standstill reason for all
// agents that are in standstill and have a hooked issue.
func BuildStandstillIDs(status *TownStatus) map[string]string {
	if status == nil {
		return nil
	}
	ids := make(map[string]string)
	for _, a := range status.Agents {
		if a.HookBead == "" {
			continue
		}
		if ok, reason := IsStandstill(a); ok {
			ids[a.HookBead] = reason
		}
	}
	if len(ids) == 0 {
		return nil
	}
	return ids
}
