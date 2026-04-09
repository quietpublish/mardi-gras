package gastown

import "fmt"

// OrphanedIssue represents an issue left without an agent after rig death.
type OrphanedIssue struct {
	IssueID   string
	Title     string
	AgentName string // the dead agent that was working on it
	AgentRole string // polecat, crew, etc.
}

// RecoveryMode controls what happens after orphaned issues are released.
type RecoveryMode int

const (
	RecoveryResling      RecoveryMode = iota // release + re-sling to same rig
	RecoveryReleaseOnly                      // release only, manual re-dispatch
	RecoverySlingFormula                     // release + re-sling with formula
)

// RecoveryResult reports per-issue outcomes from a recovery operation.
type RecoveryResult struct {
	Released    []string // issue IDs successfully released
	Reslung     []string // issue IDs successfully re-dispatched
	ReleaseFail map[string]error
	SlingFail   map[string]error
}

// FindOrphans identifies orphaned issues from the current town status.
// An orphan is an agent with hooked work whose session is not running.
func FindOrphans(status *TownStatus, rigName string) []OrphanedIssue {
	if status == nil {
		return nil
	}

	var orphans []OrphanedIssue
	for _, a := range status.Agents {
		if a.Rig != rigName {
			continue
		}
		if a.HookBead != "" && !a.Running {
			orphans = append(orphans, OrphanedIssue{
				IssueID:   a.HookBead,
				Title:     a.WorkTitle,
				AgentName: a.Name,
				AgentRole: a.Role,
			})
		}
	}

	return orphans
}

// FindDeadRigs returns rig names that have zero polecats but orphaned work.
func FindDeadRigs(status *TownStatus) []string {
	if status == nil {
		return nil
	}

	// Build set of rigs with zero polecats.
	deadRigs := make(map[string]bool)
	for _, r := range status.Rigs {
		if r.PolecatCount == 0 {
			deadRigs[r.Name] = true
		}
	}

	if len(deadRigs) == 0 {
		return nil
	}

	// Only include rigs that also have orphaned work.
	rigsWithOrphans := make(map[string]bool)
	for _, a := range status.Agents {
		if deadRigs[a.Rig] && a.HookBead != "" && !a.Running {
			rigsWithOrphans[a.Rig] = true
		}
	}

	var result []string
	for rig := range rigsWithOrphans {
		result = append(result, rig)
	}

	return result
}

// ReleaseIssue releases a single orphaned issue back to open status.
func ReleaseIssue(issueID, reason string) error {
	if err := validateIssueID(issueID); err != nil {
		return err
	}
	args := []string{"release", issueID}
	if reason != "" {
		reason = sanitizeText(reason, maxTextLen)
		args = append(args, "--reason", reason)
	}
	return execWithTimeout(timeoutShort, "gt", args...)
}

// RecoverRig releases orphaned issues and optionally re-dispatches them.
func RecoverRig(orphans []OrphanedIssue, rigName string, mode RecoveryMode, formula string) RecoveryResult {
	result := RecoveryResult{
		ReleaseFail: make(map[string]error),
		SlingFail:   make(map[string]error),
	}

	// Phase 1: Release all orphaned issues.
	var toSling []string
	for _, o := range orphans {
		reason := fmt.Sprintf("rig recovery: agent %s died", o.AgentName)
		if err := ReleaseIssue(o.IssueID, reason); err != nil {
			result.ReleaseFail[o.IssueID] = err
			continue
		}
		result.Released = append(result.Released, o.IssueID)
		toSling = append(toSling, o.IssueID)
	}

	if mode == RecoveryReleaseOnly {
		return result
	}

	// Phase 2: Re-sling released issues.
	for _, id := range toSling {
		var err error
		switch mode {
		case RecoverySlingFormula:
			err = SlingWithFormula(id, formula)
		default:
			err = Sling(id)
		}
		if err != nil {
			result.SlingFail[id] = err
		} else {
			result.Reslung = append(result.Reslung, id)
		}
	}

	return result
}
