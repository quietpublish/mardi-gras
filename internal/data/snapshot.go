package data

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"
)

// An IssueSnapshot is what mg is willing to tell an outside judge (Jev)
// about an issue: enough to answer questions about it, and nothing that
// names people, hosts or the workspace. It lives here, next to DiffIssues,
// so redaction is reviewed as a data concern and so --status mode and tests
// never touch the network package that sends it.
//
// Deliberately absent: Owner, Assignee and CreatedBy (usernames and
// emails), Notes, Design, AcceptanceCriteria and CloseReason (free text
// written by agents, the likeliest place for a pasted secret), Metadata,
// dependency IDs (counts carry the signal) and every timestamp (ages do).
type IssueSnapshot struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description,omitempty"`
	Type        string   `json:"type"`
	Status      string   `json:"status"`
	Priority    int      `json:"priority"`
	Labels      []string `json:"labels,omitempty"`

	// Ages in whole days, so the snapshot (and its hash) changes at most
	// once a day for an untouched issue.
	AgeDays         int  `json:"age_days"`
	SinceUpdateDays int  `json:"since_update_days"`
	DueInDays       *int `json:"due_in_days,omitempty"`       // negative when overdue
	DeferredForDays *int `json:"deferred_for_days,omitempty"` // only while deferred

	// Claimed says who has the issue without naming anyone: "you" for the
	// viewer, "someone else", or empty when unassigned. Without it the
	// judge saw every in-progress issue as someone else's (mg-xge.1).
	Claimed string `json:"claimed,omitempty"`

	Blocking     int `json:"blocking"`               // open issues this one waits on
	MissingDeps  int `json:"missing_deps,omitempty"` // blockers that do not exist
	CommentCount int `json:"comment_count,omitempty"`
}

// SnapshotScope is how much of an issue a snapshot carries.
type SnapshotScope int

const (
	// SnapshotMinimal sends the title and structural fields only.
	SnapshotMinimal SnapshotScope = iota
	// SnapshotStandard adds the start of the description.
	SnapshotStandard
)

// ParseSnapshotScope reads a scope name ("minimal", "standard"); an empty
// string is the default, SnapshotStandard. ok is false for anything else.
func ParseSnapshotScope(s string) (scope SnapshotScope, ok bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "standard":
		return SnapshotStandard, true
	case "minimal":
		return SnapshotMinimal, true
	default:
		return SnapshotStandard, false
	}
}

// snapshotDescriptionRunes bounds the description sent at standard scope.
const snapshotDescriptionRunes = 600

// SnapshotForJudge builds the snapshot of iss at now. eval is the issue's
// dependency evaluation against the current issue set.
func SnapshotForJudge(iss Issue, eval DepEval, scope SnapshotScope, now time.Time) IssueSnapshot {
	s := IssueSnapshot{
		ID:           iss.ID,
		Title:        ScrubSecrets(iss.Title),
		Type:         string(iss.IssueType),
		Status:       string(iss.Status),
		Priority:     int(iss.Priority),
		AgeDays:      wholeDays(now.Sub(iss.CreatedAt)),
		Blocking:     len(eval.BlockingIDs),
		MissingDeps:  len(eval.MissingIDs),
		CommentCount: iss.CommentCount,
	}
	updated := iss.UpdatedAt
	if updated.IsZero() {
		updated = iss.CreatedAt
	}
	s.SinceUpdateDays = wholeDays(now.Sub(updated))
	if iss.DueAt != nil {
		d := signedDays(iss.DueAt.Sub(now))
		s.DueInDays = &d
	}
	if iss.DeferUntil != nil && iss.DeferUntil.After(now) {
		d := signedDays(iss.DeferUntil.Sub(now))
		s.DeferredForDays = &d
	}
	if len(iss.Labels) > 0 {
		s.Labels = slices.Clone(iss.Labels)
		slices.Sort(s.Labels)
	}
	if scope >= SnapshotStandard && iss.Description != "" {
		s.Description = ScrubSecrets(truncateRunes(stripCodeFences(iss.Description), snapshotDescriptionRunes))
	}
	return s
}

// ClaimedBy is the snapshot's Claimed value for an assignee as seen by me.
// An unknown viewer cannot claim anything, so every assignee is someone else.
func ClaimedBy(assignee, me string) string {
	switch {
	case assignee == "":
		return ""
	case me != "" && SameActor(assignee, me):
		return "you"
	default:
		return "someone else"
	}
}

// Hash identifies the snapshot's content: two snapshots with the same hash
// would be sent identically, so a cached verdict for one answers the other.
// Because ages are whole days, an untouched issue keeps its hash for a day.
func (s IssueSnapshot) Hash() string {
	b, _ := json.Marshal(s) // the struct only holds marshallable fields
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:16])
}

func wholeDays(d time.Duration) int {
	if d < 0 {
		return 0
	}
	return int(d.Hours() / 24)
}

// signedDays rounds toward zero, so "due in 36 hours" is 1 and "overdue by
// 36 hours" is -1.
func signedDays(d time.Duration) int {
	return int(d.Hours() / 24)
}

func truncateRunes(s string, n int) string {
	if n <= 0 {
		return ""
	}
	count := 0
	for i := range s {
		if count == n {
			return strings.TrimRightFunc(s[:i], unicode.IsSpace) + "…"
		}
		count++
	}
	return s
}

var codeFence = regexp.MustCompile("(?s)```.*?(```|$)")

// stripCodeFences replaces fenced code blocks with a marker. Code is where
// agents paste logs and config, and a judge of the issue's shape does not
// need it.
func stripCodeFences(s string) string {
	return codeFence.ReplaceAllString(s, "[code]")
}

// Patterns of things that should never leave the machine even when they
// appear in a title or description. Each match becomes "[redacted]".
var secretPatterns = []*regexp.Regexp{
	// Well-known key prefixes: OpenAI/Anthropic-style sk-, GitHub gh?_,
	// GitLab glpat-, Slack xox?-, AWS AKIA, npm_.
	regexp.MustCompile(`\b(?:sk-[A-Za-z0-9_-]{8,}|gh[pousr]_[A-Za-z0-9]{20,}|glpat-[A-Za-z0-9_-]{16,}|xox[abprs]-[A-Za-z0-9-]{10,}|AKIA[0-9A-Z]{16}|npm_[A-Za-z0-9]{30,})`),
	// Bearer tokens and long hex strings (hashes, hex keys).
	regexp.MustCompile(`(?i)\bbearer\s+[A-Za-z0-9._~+/=-]{16,}`),
	regexp.MustCompile(`\b[A-Fa-f0-9]{32,}\b`),
}

// secretAssignment catches "api_key=..." / "password: ..." style pairs,
// keeping the name and dropping the value.
var secretAssignment = regexp.MustCompile(`(?i)\b(api[_-]?key|secret|token|passw(?:or)?d|authorization)(\s*[:=]\s*)["']?\S+`)

// longOpaque is a run long enough to be a credential; it is redacted only
// when it mixes letters and digits, so long words and paths survive.
var longOpaque = regexp.MustCompile(`[A-Za-z0-9+/=_-]{40,}`)

// ScrubSecrets replaces token-shaped substrings with "[redacted]". It is
// deliberately eager: a false positive costs a judge a little context, a
// false negative sends a credential to a third party.
func ScrubSecrets(s string) string {
	if s == "" {
		return s
	}
	// Token shapes first, so "Authorization: Bearer x" collapses to one
	// marker when the assignment pattern then drops the value.
	for _, re := range secretPatterns {
		s = re.ReplaceAllString(s, "[redacted]")
	}
	s = secretAssignment.ReplaceAllString(s, "$1$2[redacted]")
	return longOpaque.ReplaceAllStringFunc(s, func(m string) string {
		if looksOpaque(m) {
			return "[redacted]"
		}
		return m
	})
}

func looksOpaque(s string) bool {
	var letters, digits bool
	for _, r := range s {
		switch {
		case unicode.IsLetter(r):
			letters = true
		case unicode.IsDigit(r):
			digits = true
		}
		if letters && digits {
			return true
		}
	}
	return false
}
