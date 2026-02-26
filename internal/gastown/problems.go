package gastown

// Problem represents a detected issue with a Gas Town agent.
type Problem struct {
	Type     string       // "stalled", "backoff", "zombie"
	Agent    AgentRuntime // the affected agent
	Detail   string       // human-readable description
	Severity string       // "warn", "error"
}

// DetectProblems analyzes TownStatus and returns any detected problems.
// All heuristics are stateless — derived from a single status snapshot.
func DetectProblems(status *TownStatus) []Problem {
	if status == nil {
		return nil
	}

	var problems []Problem
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

		// Zombie: agent not running but has hooked work
		if !a.Running && a.HookBead != "" {
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
