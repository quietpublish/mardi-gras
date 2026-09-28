package data

import (
	"os"
	"sort"
	"strings"
)

// FocusFilter returns a filtered, prioritized list for focus mode:
//  1. In-progress issues that are yours: assigned to you (see CurrentActor
//     for who that is), or claimed by nobody, as when an agent marks an issue
//     in progress without claiming it
//  2. Highest priority unblocked issues (open, not blocked)
//  3. Blocked issues with context
//
// With no known identity (me == ""), every in-progress issue counts.
func FocusFilter(issues []Issue, blockingTypes map[string]bool, me string) []Issue {
	issueMap := BuildIssueMap(issues)

	var myWork []Issue  // in_progress, assigned to me
	var ready []Issue   // open, not blocked
	var blocked []Issue // open, blocked (context)

	for _, iss := range issues {
		if iss.Status == StatusClosed {
			continue
		}
		isBlocked := iss.EvaluateDependencies(issueMap, blockingTypes).IsBlocked
		isMine := me == "" || iss.Assignee == "" || SameActor(iss.Assignee, me)

		switch {
		case iss.Status == StatusInProgress:
			if isMine {
				myWork = append(myWork, iss)
			}
		case isBlocked:
			blocked = append(blocked, iss)
		default:
			ready = append(ready, iss)
		}
	}

	// Sort ready by priority (P0 first)
	sort.Slice(ready, func(i, j int) bool {
		return ready[i].Priority < ready[j].Priority
	})

	// Limit ready to top 5
	if len(ready) > 5 {
		ready = ready[:5]
	}

	// Limit blocked to top 3
	if len(blocked) > 3 {
		blocked = blocked[:3]
	}

	var result []Issue
	result = append(result, myWork...)
	result = append(result, ready...)
	result = append(result, blocked...)
	return result
}

// CurrentActor resolves who "you" are the way bd 1.3.0 does when it records
// a claim: BEADS_ACTOR, BD_ACTOR, the workspace's .beads/config.yaml actor,
// git user.name, then $USER. Claims assign the issue to that name, so focus
// mode must compare assignees with it rather than with the login name.
func CurrentActor(projectDir string) string {
	for _, env := range []string{"BEADS_ACTOR", "BD_ACTOR"} {
		if v := strings.TrimSpace(os.Getenv(env)); v != "" {
			return v
		}
	}
	if actor := LoadConfiguredActor(projectDir); actor != "" {
		return actor
	}
	if out, err := runWithTimeout(timeoutShort, "git", "config", "user.name"); err == nil {
		if name := strings.TrimSpace(string(out)); name != "" {
			return name
		}
	}
	return strings.TrimSpace(os.Getenv("USER"))
}

// SameActor reports whether two actor names are the same identity, by bd's
// rule: a run of '.', '_' or '-' is one separator, except an exact "--",
// which stands for '/'. Unlike bd, it also ignores case.
func SameActor(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	return strings.EqualFold(canonicalActor(a), canonicalActor(b))
}

func canonicalActor(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); {
		if !isActorSeparator(s[i]) {
			b.WriteByte(s[i])
			i++
			continue
		}
		j := i
		for j < len(s) && isActorSeparator(s[j]) {
			j++
		}
		if s[i:j] == "--" {
			b.WriteByte('/')
		} else {
			b.WriteByte('_')
		}
		i = j
	}
	return b.String()
}

func isActorSeparator(c byte) bool {
	return c == '.' || c == '_' || c == '-'
}
