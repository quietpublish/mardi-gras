package data

import (
	"encoding/json"
	"fmt"
	"strings"
)

// SetStatus runs `bd update <id> --status=<status>` to change an issue's status.
func SetStatus(issueID string, status Status) error {
	if err := ValidateIssueID(issueID); err != nil {
		return err
	}
	return execWithTimeout(timeoutShort, "bd", "update", issueID, "--status="+string(status))
}

// ClaimIssue runs `bd update <id> --claim` to atomically set assignee and status to in_progress.
// Fails if the issue is already claimed by another agent, preventing races in multi-agent workflows.
func ClaimIssue(issueID string) error {
	if err := ValidateIssueID(issueID); err != nil {
		return err
	}
	return execWithTimeout(timeoutShort, "bd", "update", issueID, "--claim")
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
		return "", fmt.Errorf("bd close --claim-next parse: %w", err)
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

// SetAssignee runs `bd update <id> --assignee=<name>` to assign an issue.
func SetAssignee(issueID, assignee string) error {
	if err := ValidateIssueID(issueID); err != nil {
		return err
	}
	assignee = sanitizeText(assignee, maxTextLen)
	return execWithTimeout(timeoutShort, "bd", "update", issueID, "--assignee="+assignee)
}

// AddLabel runs `bd label add <id> -- <label>` to add a label to an issue.
func AddLabel(issueID, label string) error {
	if err := ValidateIssueID(issueID); err != nil {
		return err
	}
	label = sanitizeText(label, maxTextLen)
	return execWithTimeout(timeoutShort, "bd", "label", "add", issueID, "--", label)
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
