package gastown

import (
	"fmt"
	"regexp"
	"strings"
)

// issueIDPattern matches beads issue IDs: lowercase prefix (possibly hyphenated) + hyphen + alphanumeric hash.
var issueIDPattern = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)+$`)

const (
	maxIssueIDLen = 64
	maxTextLen    = 10000
)

// validateIssueID checks that id matches the expected beads issue ID format.
func validateIssueID(id string) error {
	if len(id) > maxIssueIDLen {
		return fmt.Errorf("issue ID too long: %d bytes (max %d)", len(id), maxIssueIDLen)
	}
	if !issueIDPattern.MatchString(id) {
		return fmt.Errorf("invalid issue ID %q", id)
	}
	return nil
}

// sanitizeText strips control characters and enforces a maximum byte length.
func sanitizeText(s string, maxLen int) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r < 0x20 && r != '\n' && r != '\t' && r != '\r' {
			continue
		}
		b.WriteRune(r)
	}
	result := b.String()
	if len(result) > maxLen {
		result = result[:maxLen]
	}
	return result
}
