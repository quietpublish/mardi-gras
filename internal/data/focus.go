package data

import (
	"os"
	"sort"
	"strings"
)

// FocusFilter returns a filtered, prioritized list for focus mode:
// 1. Issues assigned to the current user that are in_progress
// 2. Highest priority unblocked issues (open, not blocked)
// 3. Blocked issues with context
func FocusFilter(issues []Issue, blockingTypes map[string]bool) []Issue {
	user := currentUser()
	issueMap := BuildIssueMap(issues)

	var myWork []Issue  // in_progress, assigned to me
	var ready []Issue   // open, not blocked
	var blocked []Issue // open, blocked (context)

	for _, iss := range issues {
		if iss.Status == StatusClosed {
			continue
		}
		isBlocked := iss.EvaluateDependencies(issueMap, blockingTypes).IsBlocked
		isMine := user != "" && (strings.EqualFold(iss.Assignee, user) || strings.EqualFold(iss.Owner, user))

		switch {
		case iss.Status == StatusInProgress:
			if isMine || user == "" {
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

// currentUser tries to determine the current user from environment or git config.
func currentUser() string {
	if user := os.Getenv("USER"); user != "" {
		return user
	}
	out, err := runWithTimeout(timeoutShort, "git", "config", "user.name")
	if err == nil {
		return strings.TrimSpace(string(out))
	}
	return ""
}
