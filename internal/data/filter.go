package data

import (
	"strings"

	"github.com/sahilm/fuzzy"
)

// FilterIssues returns a new slice of issues that match the search query.
// It supports explicit tokens (type:bug, p1, priority:high) and fuzzy free-text
// search on ID and Title. All tokens in the query must match (AND logic).
func FilterIssues(issues []Issue, query string) []Issue {
	query = strings.TrimSpace(query)
	if query == "" {
		return issues
	}

	rawTokens := strings.Fields(query)
	var structuredTokens []string
	var freeTokens []string

	for _, t := range rawTokens {
		lower := strings.ToLower(t)
		if isStructuredToken(lower) {
			structuredTokens = append(structuredTokens, lower)
		} else {
			freeTokens = append(freeTokens, lower)
		}
	}

	// First pass: structured token filtering (exact match)
	candidates := issues
	if len(structuredTokens) > 0 {
		var filtered []Issue
		for _, issue := range candidates {
			if matchesStructuredTokens(issue, structuredTokens) {
				filtered = append(filtered, issue)
			}
		}
		candidates = filtered
	}

	// Second pass: fuzzy match on combined free tokens
	if len(freeTokens) > 0 {
		freeQuery := strings.Join(freeTokens, " ")
		candidates = fuzzyFilter(candidates, freeQuery)
	}

	return candidates
}

// isStructuredToken returns true for tokens with explicit prefixes or priority shorthands.
func isStructuredToken(token string) bool {
	if strings.HasPrefix(token, "type:") || strings.HasPrefix(token, "priority:") {
		return true
	}
	// Priority shorthands: p0, p1, p2, p3, p4
	if len(token) == 2 && token[0] == 'p' && token[1] >= '0' && token[1] <= '4' {
		return true
	}
	return false
}

func matchesStructuredTokens(issue Issue, tokens []string) bool {
	issueType := strings.ToLower(string(issue.IssueType))
	issuePriorityLevel := issue.Priority
	issuePriorityName := strings.ToLower(PriorityName(issue.Priority))
	issuePriorityLabel := strings.ToLower(PriorityLabel(issue.Priority))

	for _, token := range tokens {
		matched := false

		switch {
		case strings.HasPrefix(token, "type:"):
			val := strings.TrimPrefix(token, "type:")
			matched = issueType == val

		case strings.HasPrefix(token, "priority:"):
			val := strings.TrimPrefix(token, "priority:")
			switch {
			case val == issuePriorityName:
				matched = true
			case val == "0" && issuePriorityLevel == PriorityCritical:
				matched = true
			case val == "1" && issuePriorityLevel == PriorityHigh:
				matched = true
			case val == "2" && issuePriorityLevel == PriorityMedium:
				matched = true
			case val == "3" && issuePriorityLevel == PriorityLow:
				matched = true
			case val == "4" && issuePriorityLevel == PriorityBacklog:
				matched = true
			}

		case token == issuePriorityLabel:
			matched = true
		}

		if !matched {
			return false
		}
	}

	return true
}

// issueSearchSource implements fuzzy.Source for issue searching.
type issueSearchSource struct {
	issues []Issue
}

func (s issueSearchSource) String(i int) string {
	issue := s.issues[i]
	var b strings.Builder
	b.WriteString(issue.ID)
	b.WriteByte(' ')
	b.WriteString(issue.Title)
	if issue.Description != "" {
		b.WriteByte(' ')
		b.WriteString(issue.Description)
	}
	if issue.Assignee != "" {
		b.WriteByte(' ')
		b.WriteString(issue.Assignee)
	}
	if issue.Owner != "" {
		b.WriteByte(' ')
		b.WriteString(issue.Owner)
	}
	if issue.Notes != "" {
		b.WriteByte(' ')
		b.WriteString(issue.Notes)
	}
	for _, label := range issue.Labels {
		b.WriteByte(' ')
		b.WriteString(label)
	}
	return b.String()
}

func (s issueSearchSource) Len() int {
	return len(s.issues)
}

// fuzzyFilter applies fuzzy matching on issue ID + Title.
func fuzzyFilter(issues []Issue, query string) []Issue {
	if len(issues) == 0 {
		return nil
	}

	src := issueSearchSource{issues: issues}
	matches := fuzzy.FindFrom(query, src)

	result := make([]Issue, 0, len(matches))
	for _, match := range matches {
		result = append(result, issues[match.Index])
	}
	return result
}

// FilterIssuesWithHighlights returns filtered issues plus a map of issue ID → matched
// character indices in the "ID + Title" search string. Used for rendering highlights.
func FilterIssuesWithHighlights(issues []Issue, query string) (result []Issue, matchMap map[string][]int) {
	query = strings.TrimSpace(query)
	if query == "" {
		return issues, nil
	}

	rawTokens := strings.Fields(query)
	var structuredTokens []string
	var freeTokens []string

	for _, t := range rawTokens {
		lower := strings.ToLower(t)
		if isStructuredToken(lower) {
			structuredTokens = append(structuredTokens, lower)
		} else {
			freeTokens = append(freeTokens, lower)
		}
	}

	candidates := issues
	if len(structuredTokens) > 0 {
		var filtered []Issue
		for _, issue := range candidates {
			if matchesStructuredTokens(issue, structuredTokens) {
				filtered = append(filtered, issue)
			}
		}
		candidates = filtered
	}

	if len(freeTokens) == 0 {
		return candidates, nil
	}

	freeQuery := strings.Join(freeTokens, " ")
	if len(candidates) == 0 {
		return nil, nil
	}

	src := issueSearchSource{issues: candidates}
	matches := fuzzy.FindFrom(freeQuery, src)

	result = make([]Issue, 0, len(matches))
	matchMap = make(map[string][]int)
	for _, match := range matches {
		issue := candidates[match.Index]
		result = append(result, issue)
		if len(match.MatchedIndexes) > 0 {
			// Convert from "ID Title" string indices to title-only indices
			idPrefixLen := len(issue.ID) + 1 // "ID " prefix
			var titleIndices []int
			for _, idx := range match.MatchedIndexes {
				titleIdx := idx - idPrefixLen
				if titleIdx >= 0 && titleIdx < len([]rune(issue.Title)) {
					titleIndices = append(titleIndices, titleIdx)
				}
			}
			if len(titleIndices) > 0 {
				matchMap[issue.ID] = titleIndices
			}
		}
	}
	return result, matchMap
}
