package data

import (
	"sort"
	"strings"

	"github.com/sahilm/fuzzy"
)

// ExcludeByType filters out issues whose type is in excludeTypes.
// Keys in excludeTypes must be lowercase (parseTypeSet ensures this at input).
func ExcludeByType(issues []Issue, excludeTypes map[string]bool) []Issue {
	if len(excludeTypes) == 0 {
		return issues
	}

	filtered := make([]Issue, 0, len(issues))
	for _, issue := range issues {
		if !excludeTypes[string(issue.IssueType)] {
			filtered = append(filtered, issue)
		}
	}
	return filtered
}

// ExcludeByLabel filters out issues that carry any label in excludeLabels.
// Keys in excludeLabels must be lowercase (parseTypeSet ensures this at input).
// Issues with no labels are always kept.
func ExcludeByLabel(issues []Issue, excludeLabels map[string]bool) []Issue {
	if len(excludeLabels) == 0 {
		return issues
	}

	filtered := make([]Issue, 0, len(issues))
	for _, issue := range issues {
		skip := false
		for _, label := range issue.Labels {
			if excludeLabels[strings.ToLower(label)] {
				skip = true
				break
			}
		}
		if !skip {
			filtered = append(filtered, issue)
		}
	}
	return filtered
}

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

	// Second pass: free text (see matchFreeText)
	if len(freeTokens) > 0 {
		candidates, _ = matchFreeText(candidates, freeTokens)
	}

	return candidates
}

// isStructuredToken returns true for tokens with explicit prefixes or priority shorthands.
func isStructuredToken(token string) bool {
	if strings.HasPrefix(token, "type:") || strings.HasPrefix(token, "priority:") || strings.HasPrefix(token, "label:") {
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

		case strings.HasPrefix(token, "label:"):
			val := strings.TrimPrefix(token, "label:")
			for _, label := range issue.Labels {
				if strings.ToLower(label) == val {
					matched = true
					break
				}
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

// matchFreeText keeps the issues in which every token appears, as a
// case-insensitive substring, somewhere in the ID, title, description,
// people, notes or labels. Results keep their input order, which is
// priority order from SortIssues, so parade sections stay sorted. highlights
// maps an issue ID to the rune indices of the tokens found in its title.
//
// A subsequence match over all of that text matched nearly everything
// ("auth" hit "Evaluate new caching layer"; mg-9ik), so fuzzy matching now
// runs only when no issue contains the query, and only over ID and title:
// it forgives typos and abbreviations ("lgn tkn") without matching noise.
func matchFreeText(issues []Issue, tokens []string) (result []Issue, highlights map[string][]int) {
	if len(issues) == 0 || len(tokens) == 0 {
		return issues, nil
	}
	src := issueSearchSource{issues: issues}
	highlights = make(map[string][]int)
	for i, issue := range issues {
		text := strings.ToLower(src.String(i))
		all := true
		for _, tok := range tokens {
			if !strings.Contains(text, tok) {
				all = false
				break
			}
		}
		if !all {
			continue
		}
		result = append(result, issue)
		if idx := titleTokenIndices(issue.Title, tokens); len(idx) > 0 {
			highlights[issue.ID] = idx
		}
	}
	if len(result) > 0 {
		return result, highlights
	}
	return fuzzyTitleMatch(issues, strings.Join(tokens, " "))
}

// titleTokenIndices returns the rune indices covered by each token's
// occurrences in title, for highlighting.
func titleTokenIndices(title string, tokens []string) []int {
	lower := []rune(strings.ToLower(title))
	n := len([]rune(title))
	seen := make(map[int]bool)
	var idx []int
	for _, tok := range tokens {
		t := []rune(tok)
		for start := 0; start+len(t) <= len(lower); start++ {
			if string(lower[start:start+len(t)]) != tok {
				continue
			}
			for k := start; k < start+len(t) && k < n; k++ {
				if !seen[k] {
					seen[k] = true
					idx = append(idx, k)
				}
			}
		}
	}
	sort.Ints(idx)
	return idx
}

// titleSearchSource is "ID Title" only: the fuzzy fallback's haystack.
type titleSearchSource []Issue

func (s titleSearchSource) String(i int) string { return s[i].ID + " " + s[i].Title }
func (s titleSearchSource) Len() int            { return len(s) }

// fuzzyTitleMatch is the typo fallback: a subsequence match over ID and
// title, returned in input order with title highlights.
func fuzzyTitleMatch(issues []Issue, query string) (result []Issue, highlights map[string][]int) {
	matches := fuzzy.FindFrom(query, titleSearchSource(issues))
	sort.Slice(matches, func(i, j int) bool { return matches[i].Index < matches[j].Index })
	result = make([]Issue, 0, len(matches))
	highlights = make(map[string][]int)
	for _, match := range matches {
		issue := issues[match.Index]
		result = append(result, issue)
		prefix := len([]rune(issue.ID)) + 1 // "ID "
		titleLen := len([]rune(issue.Title))
		var idx []int
		for _, i := range match.MatchedIndexes {
			if t := i - prefix; t >= 0 && t < titleLen {
				idx = append(idx, t)
			}
		}
		if len(idx) > 0 {
			highlights[issue.ID] = idx
		}
	}
	return result, highlights
}

// FilterIssuesWithHighlights returns filtered issues plus a map of issue ID →
// matched rune indices in the title. Used for rendering highlights.
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

	if len(candidates) == 0 {
		return nil, nil
	}
	return matchFreeText(candidates, freeTokens)
}
