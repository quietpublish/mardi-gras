package data

import (
	"sort"
	"strings"
	"time"
	"unicode"
)

// duplicateClosedWindow is how recently a closed issue must have closed to
// still count as a possible duplicate of new work.
const duplicateClosedWindow = 14 * 24 * time.Hour

// DuplicateCandidates returns up to limit issues whose titles share enough
// words with title to be worth asking a judge about: open and in-progress
// issues, plus ones closed in the last two weeks. It is a cheap local
// prefilter so a duplicate check never ships the whole backlog; the ranking
// is lexical and deliberately generous, the judge decides.
func DuplicateCandidates(issues []Issue, title string, now time.Time, limit int) []Issue {
	want := tokenSet(title)
	if len(want) == 0 || limit <= 0 {
		return nil
	}
	type scored struct {
		iss   Issue
		score float64
	}
	var found []scored
	for i := range issues {
		iss := &issues[i]
		if iss.Status == StatusClosed {
			if iss.ClosedAt == nil || now.Sub(*iss.ClosedAt) > duplicateClosedWindow {
				continue
			}
		}
		have := tokenSet(iss.Title)
		if len(have) == 0 {
			continue
		}
		shared := 0
		for t := range want {
			if have[t] {
				shared++
			}
		}
		if shared == 0 {
			continue
		}
		smaller := min(len(want), len(have))
		union := len(want) + len(have) - shared
		// Half containment (a short title inside a longer one), half
		// Jaccard (how alike the two are overall).
		score := 0.5*float64(shared)/float64(smaller) + 0.5*float64(shared)/float64(union)
		if score < 0.25 {
			continue
		}
		found = append(found, scored{*iss, score})
	}
	sort.SliceStable(found, func(i, j int) bool {
		if found[i].score != found[j].score {
			return found[i].score > found[j].score
		}
		return found[i].iss.UpdatedAt.After(found[j].iss.UpdatedAt)
	})
	if len(found) > limit {
		found = found[:limit]
	}
	out := make([]Issue, len(found))
	for i, f := range found {
		out[i] = f.iss
	}
	return out
}

// titleStopwords are words that say nothing about what an issue is about.
var titleStopwords = map[string]bool{
	"a": true, "an": true, "the": true, "to": true, "of": true, "in": true, "on": true,
	"for": true, "and": true, "or": true, "is": true, "are": true, "be": true, "with": true,
	"when": true, "after": true, "from": true, "at": true, "by": true, "it": true, "this": true,
	"that": true, "not": true, "no": true, "as": true, "into": true, "via": true,
	"fix": true, "add": true, "update": true, "make": true, "should": true,
}

// tokenSet splits a title into lower-cased words worth comparing: no
// stopwords, nothing shorter than two characters.
func tokenSet(s string) map[string]bool {
	out := make(map[string]bool)
	for _, w := range strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}) {
		if len(w) < 2 || titleStopwords[w] {
			continue
		}
		out[w] = true
	}
	return out
}

// AddDependencyTyped runs `bd dep add --type=<type> <id> -- <depends-on-id>`
// for a non-blocking relationship such as "duplicates" or "related".
func AddDependencyTyped(issueID, dependsOnID, depType string) error {
	if err := ValidateIssueID(issueID); err != nil {
		return err
	}
	if err := ValidateIssueID(dependsOnID); err != nil {
		return err
	}
	if err := ValidateDepType(depType); err != nil {
		return err
	}
	return execWithTimeout(timeoutShort, "bd", "dep", "add", "--type="+depType, issueID, "--", dependsOnID)
}
