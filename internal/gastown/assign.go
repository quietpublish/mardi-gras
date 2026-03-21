package gastown

import "fmt"

// Assign creates a new bead and hooks it to a crew member via `gt assign`.
// Returns the command output (human-readable, no JSON). Optional parameters
// issueType, priority, and label are passed as flags when non-empty.
// When nudge is true, the agent is woken after hooking.
func Assign(crewMember, title, issueType, priority, label string, nudge bool) (string, error) {
	args := []string{"assign"}

	if issueType != "" {
		args = append(args, "-t", issueType)
	}
	if priority != "" {
		args = append(args, "-p", priority)
	}
	if label != "" {
		args = append(args, "-l", label)
	}
	if nudge {
		args = append(args, "--nudge")
	}

	args = append(args, "--", crewMember, title)

	out, err := runCombinedWithTimeout(timeoutShort, "gt", args...)
	if err != nil {
		return "", fmt.Errorf("gt assign: %w (%s)", err, sanitizeOutput(out))
	}
	return string(out), nil
}
