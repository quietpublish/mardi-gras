package gastown

import (
	"fmt"
	"regexp"
	"strings"
)

// issueIDPattern matches beads issue IDs: a prefix (possibly hyphenated)
// followed by one or more separator+segment pairs, where the separator is a
// hyphen or a dot, and a segment is letters, digits and underscores. The dot
// matters — beads gives child issues dotted ids (`infra-h0xb.9`) — and so does
// the character set, which is bd's own (`my_project-hbu`, `MyApp-d0g`).
// Rejecting either makes those issues untouchable. Keep in sync with
// internal/data/validate.go, which explains why neither widens the
// anti-injection guard.
var issueIDPattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_]*([-.][a-zA-Z0-9_]+)+$`)

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
