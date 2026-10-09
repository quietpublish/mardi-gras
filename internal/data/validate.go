package data

import (
	"fmt"
	"regexp"
	"strings"
)

// issueIDPattern matches beads issue IDs: a prefix (possibly hyphenated)
// followed by one or more separator+segment pairs, where the separator is a
// hyphen or a dot, and a segment is letters, digits and underscores.
//
// The character set is bd's own definition of a valid prefix (bd doctor checks
// `^[a-zA-Z][a-zA-Z0-9_-]*$`). bd derives the default prefix from the directory
// name, so a repo named `my_project` gets `my_project-hbu`, and `--prefix MyApp`
// gives `MyApp-d0g`; a narrower set leaves mg unable to write to any issue in
// such a workspace.
//
// The dot is not decoration: beads gives every child issue a dotted id
// (`infra-h0xb.9` for child 9 of `infra-h0xb`), and those ids are handed
// straight back to bd by the mutators in mutate.go, so rejecting them leaves mg
// unable to touch any child issue at all. Nesting is allowed for the same
// reason (`infra-h0xb.9.1`).
//
// Allowing the dot does not widen this guard, which exists to stop a
// user-supplied id becoming a flag or a second argument: the id must START with
// a letter and a separator is only accepted between segments, so a flag
// (`--delete-all`), a path (`../etc`, `a/../b`) and a trailing separator (`a.`)
// are all still rejected. Neither the underscore nor upper case can form a flag
// or a path, and the id reaches bd as a single argv element with no shell.
// Examples: mg-42, bd-a1b2, my-app-xyz123, infra-h0xb.9, infra-h0xb.9.1,
// my_project-hbu, MyApp-d0g
var issueIDPattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_]*([-.][a-zA-Z0-9_]+)+$`)

const (
	maxIssueIDLen = 64
	maxTextLen    = 10000
)

// ValidateIssueID checks that id matches the expected beads issue ID format.
func ValidateIssueID(id string) error {
	if len(id) > maxIssueIDLen {
		return fmt.Errorf("issue ID too long: %d bytes (max %d)", len(id), maxIssueIDLen)
	}
	if !issueIDPattern.MatchString(id) {
		return fmt.Errorf("invalid issue ID %q", id)
	}
	return nil
}

// sanitizeText strips control characters and enforces a maximum byte length.
// Control characters 0x00-0x1F are removed except newline (0x0A), tab (0x09),
// and carriage return (0x0D).
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

// depTypePattern matches the dependency type names bd accepts: lower-case
// words joined by hyphens, such as "blocks", "related" or "discovered-from".
var depTypePattern = regexp.MustCompile(`^[a-z]+(-[a-z]+)*$`)

// ValidateDepType rejects anything that is not a bd dependency type name,
// so a user-supplied value can never become a flag or an extra argument.
func ValidateDepType(depType string) error {
	if !depTypePattern.MatchString(depType) {
		return fmt.Errorf("invalid dependency type %q", depType)
	}
	return nil
}
