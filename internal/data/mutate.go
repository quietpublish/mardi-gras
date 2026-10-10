package data

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"sync/atomic"
)

// ErrChangedElsewhere marks a write bd refused because the field no longer
// held the value mg showed: someone (an agent, another terminal) changed the
// issue first. Nothing was written. The caller should reload, not retry.
var ErrChangedElsewhere = errors.New("changed elsewhere")

// exitGuardMismatch is bd's exit code for a failed --if-status/--if-assignee
// guard (bd v1.3.0+); every other failure exits 1.
const exitGuardMismatch = 13

// casGuardsUnsupported latches once this bd is known to reject the --if-*
// guards (anything before v1.3.0), so an older binary costs one wasted probe
// per process instead of a doubled call on every write.
var casGuardsUnsupported atomic.Bool

// guardedUpdate runs `bd update <id> <set> <guard>`, where guard is an
// --if-status or --if-assignee compare-and-swap check: bd applies set only if
// the field still holds the value mg last showed, and exits 13 otherwise, which
// becomes ErrChangedElsewhere. On a bd too old for the guards it falls back to
// the unguarded write, which is what mg always did before.
func guardedUpdate(issueID, set, guard string) error {
	if !casGuardsUnsupported.Load() {
		// runWithTimeout, not execWithTimeout: Output() keeps bd's stderr on the
		// ExitError, so the mismatch reason reaches the toast.
		_, err := runWithTimeout(timeoutShort, "bd", "update", issueID, set, guard)
		if err == nil {
			return nil
		}
		wrapped := wrapExitError("bd update", err)
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == exitGuardMismatch {
			return fmt.Errorf("%w: %w", ErrChangedElsewhere, wrapped)
		}
		if !strings.Contains(wrapped.Error(), "unknown flag") {
			// A real failure, not a capability gap — don't retry and double the cost.
			return wrapped
		}
		casGuardsUnsupported.Store(true)
	}
	return execWithTimeout(timeoutShort, "bd", "update", issueID, set)
}

// SetStatus runs `bd update <id> --status=<to>` to change an issue's status.
// from is the status mg showed; when set, bd refuses the write with
// ErrChangedElsewhere if the issue has moved on since. An empty from writes
// unguarded.
func SetStatus(issueID string, from, to Status) error {
	if err := ValidateIssueID(issueID); err != nil {
		return err
	}
	if from == "" {
		return execWithTimeout(timeoutShort, "bd", "update", issueID, "--status="+string(to))
	}
	return guardedUpdate(issueID, "--status="+string(to), "--if-status="+string(from))
}

// ClaimIssue runs `bd update <id> --claim` to atomically set assignee and status to in_progress.
// Fails if the issue is already claimed by another agent, preventing races in multi-agent workflows.
func ClaimIssue(issueID string) error {
	if err := ValidateIssueID(issueID); err != nil {
		return err
	}
	return execWithTimeout(timeoutShort, "bd", "update", issueID, "--claim")
}

// ClaimNextReady runs `bd ready --claim --json` to atomically claim the highest-priority
// ready issue from the queue. Returns nil if nothing is claimable. Available on bd v1.0.4+
// (PR #3578); on older bd it surfaces "unknown flag: --claim" via wrapExitError.
func ClaimNextReady() (*Issue, error) {
	out, err := runWithTimeout(timeoutShort, "bd", "ready", "--claim", "--json")
	if err != nil {
		return nil, wrapExitError("bd ready --claim", err)
	}
	var issues []Issue
	if err := json.Unmarshal(out, &issues); err != nil {
		return nil, fmt.Errorf("bd ready --claim parse: %w", err)
	}
	if len(issues) == 0 {
		return nil, nil
	}
	return &issues[0], nil
}

// CloseIssue runs `bd close <id>` to close an issue.
func CloseIssue(issueID string) error {
	if err := ValidateIssueID(issueID); err != nil {
		return err
	}
	return execWithTimeout(timeoutShort, "bd", "close", issueID)
}

// CloseAndClaimNext runs `bd close --claim-next --json <id>` and returns the
// next claimed issue ID, if any.
// ErrClaimUnreadable means bd closed the issue (it exited 0) but its
// --claim-next output could not be read, so which issue it claimed, if any,
// is unknown. It is a partial success, not a failed close (mg-299).
var ErrClaimUnreadable = errors.New("closed, but which issue bd claimed next could not be read")

func CloseAndClaimNext(issueID string) (string, error) {
	if err := ValidateIssueID(issueID); err != nil {
		return "", err
	}
	out, err := runWithTimeout(timeoutShort, "bd", "close", "--claim-next", "--json", issueID)
	if err != nil {
		return "", wrapExitError("bd close --claim-next", err)
	}

	var result struct {
		Claimed *Issue `json:"claimed"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		return "", fmt.Errorf("%w: %w", ErrClaimUnreadable, err)
	}
	if result.Claimed == nil {
		return "", nil
	}
	return result.Claimed.ID, nil
}

// SetPriority runs `bd update <id> --priority=<n>` to change priority.
func SetPriority(issueID string, priority Priority) error {
	if err := ValidateIssueID(issueID); err != nil {
		return err
	}
	return execWithTimeout(timeoutShort, "bd", "update", issueID, fmt.Sprintf("--priority=%d", priority))
}

// CreateIssue runs `bd create` with the given parameters and returns the new issue ID.
func CreateIssue(title string, issueType IssueType, priority Priority) (string, error) {
	title = sanitizeText(title, maxTextLen)
	args := []string{
		"create",
		"--title=" + title,
		"--type=" + string(issueType),
		fmt.Sprintf("--priority=%d", priority),
	}
	out, err := runWithTimeout(timeoutShort, "bd", args...)
	if err != nil {
		return "", wrapExitError("bd create", err)
	}
	// bd create prints the new issue ID
	return strings.TrimSpace(string(out)), nil
}

// UpdateTitle runs `bd update <id> --title=<title>` to change an issue's title.
func UpdateTitle(issueID, title string) error {
	if err := ValidateIssueID(issueID); err != nil {
		return err
	}
	title = sanitizeText(title, maxTextLen)
	return execWithTimeout(timeoutShort, "bd", "update", issueID, "--title="+title)
}

// SetType runs `bd update <id> --type=<type>` to change an issue's type.
func SetType(issueID string, issueType IssueType) error {
	if err := ValidateIssueID(issueID); err != nil {
		return err
	}
	return execWithTimeout(timeoutShort, "bd", "update", issueID, "--type="+sanitizeText(string(issueType), maxTextLen))
}

// UpdateDescription runs `bd update <id> --description=<text>` to replace an
// issue's description; an empty one clears it.
func UpdateDescription(issueID, description string) error {
	if err := ValidateIssueID(issueID); err != nil {
		return err
	}
	return execWithTimeout(timeoutShort, "bd", "update", issueID, "--description="+sanitizeText(description, maxTextLen))
}

// AddComment runs `bd comments add <id> -- <body>` to add a comment to an issue.
func AddComment(issueID, body string) error {
	if err := ValidateIssueID(issueID); err != nil {
		return err
	}
	body = sanitizeText(body, maxTextLen)
	_, err := runWithTimeout(timeoutShort, "bd", "comments", "add", issueID, "--", body)
	return wrapExitError("bd comments add", err)
}

// AddNote runs `bd note <id> -- <body>` to add a note to an issue.
func AddNote(issueID, body string) error {
	if err := ValidateIssueID(issueID); err != nil {
		return err
	}
	body = sanitizeText(body, maxTextLen)
	_, err := runWithTimeout(timeoutShort, "bd", "note", issueID, "--", body)
	return wrapExitError("bd note", err)
}

// SetAssignee runs `bd update <id> --assignee=<to>` to assign an issue. from
// is the assignee mg showed, "" for unassigned, and is always checked: bd
// refuses the write with ErrChangedElsewhere if someone else took the issue
// first, rather than silently taking it from them.
func SetAssignee(issueID, from, to string) error {
	if err := ValidateIssueID(issueID); err != nil {
		return err
	}
	to = sanitizeText(to, maxTextLen)
	from = sanitizeText(from, maxTextLen)
	return guardedUpdate(issueID, "--assignee="+to, "--if-assignee="+from)
}

// AddLabel runs `bd label add <id> -- <label>` to add a label to an issue.
func AddLabel(issueID, label string) error {
	if err := ValidateIssueID(issueID); err != nil {
		return err
	}
	label = sanitizeText(label, maxTextLen)
	return execWithTimeout(timeoutShort, "bd", "label", "add", issueID, "--", label)
}

// PrunePreview runs `bd prune --older-than <age> --dry-run` and returns
// the raw output so callers can surface it in a toast. Available on bd v1.1+.
func PrunePreview(olderThan string) (string, error) {
	if olderThan == "" {
		olderThan = "30d"
	}
	out, err := runWithTimeout(timeoutMedium, "bd", "prune", "--older-than", olderThan, "--dry-run")
	if err != nil {
		return "", wrapExitError("bd prune --dry-run", err)
	}
	return firstLine(string(out)), nil
}

// PruneClosed runs `bd prune --older-than <age> --force` to actually delete
// closed non-ephemeral beads. Available on bd v1.1+.
func PruneClosed(olderThan string) (string, error) {
	if olderThan == "" {
		olderThan = "30d"
	}
	out, err := runWithTimeout(timeoutMedium, "bd", "prune", "--older-than", olderThan, "--force")
	if err != nil {
		return "", wrapExitError("bd prune", err)
	}
	return firstLine(string(out)), nil
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	first, _, _ := strings.Cut(s, "\n")
	return strings.TrimSpace(first)
}

// AddDependency runs `bd dep add <id> -- <depends-on-id>` to add a blocking dependency.
func AddDependency(issueID, dependsOnID string) error {
	if err := ValidateIssueID(issueID); err != nil {
		return err
	}
	if err := ValidateIssueID(dependsOnID); err != nil {
		return err
	}
	return execWithTimeout(timeoutShort, "bd", "dep", "add", issueID, "--", dependsOnID)
}

// BranchName generates a git branch name from an issue.
func BranchName(issue Issue) string {
	prefix := "feat"
	switch issue.IssueType {
	case TypeBug:
		prefix = "fix"
	case TypeChore:
		prefix = "chore"
	case TypeTask:
		prefix = "task"
	}
	slug := slugify(issue.Title)
	return fmt.Sprintf("%s/%s-%s", prefix, issue.ID, slug)
}

// slugify converts a title to a URL-safe slug.
func slugify(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	lastDash := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		case r == ' ', r == '-', r == '_', r == '/':
			if !lastDash && b.Len() > 0 {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	result := b.String()
	result = strings.TrimRight(result, "-")
	if len(result) > 50 {
		result = result[:50]
		result = strings.TrimRight(result, "-")
	}
	return result
}
