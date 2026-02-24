package data

import (
	"fmt"
	"strings"
)

// SetStatus runs `bd update <id> --status=<status>` to change an issue's status.
func SetStatus(issueID string, status Status) error {
	return execWithTimeout(timeoutShort, "bd", "update", issueID, "--status="+string(status))
}

// CloseIssue runs `bd close <id>` to close an issue.
func CloseIssue(issueID string) error {
	return execWithTimeout(timeoutShort, "bd", "close", issueID)
}

// SetPriority runs `bd update <id> --priority=<n>` to change priority.
func SetPriority(issueID string, priority Priority) error {
	return execWithTimeout(timeoutShort, "bd", "update", issueID, fmt.Sprintf("--priority=%d", priority))
}

// CreateIssue runs `bd create` with the given parameters and returns the new issue ID.
func CreateIssue(title string, issueType IssueType, priority Priority) (string, error) {
	args := []string{
		"create",
		"--title=" + title,
		"--type=" + string(issueType),
		fmt.Sprintf("--priority=%d", priority),
	}
	out, err := runWithTimeout(timeoutShort, "bd", args...)
	if err != nil {
		return "", fmt.Errorf("bd create: %w", err)
	}
	// bd create prints the new issue ID
	return strings.TrimSpace(string(out)), nil
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
