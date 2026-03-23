package gastown

import (
	"sort"

	"github.com/matt-wright86/mardi-gras/internal/data"
)

// AgentScorecard aggregates metrics for a single agent.
type AgentScorecard struct {
	Name         string
	IssuesClosed int
}

// ComputeScorecards derives per-agent scorecards from closed issues.
// It maps issues to agents via the Assignee field.
func ComputeScorecards(issues []data.Issue) []AgentScorecard {
	agents := make(map[string]*AgentScorecard)

	for _, iss := range issues {
		if iss.Status != data.StatusClosed {
			continue
		}

		name := iss.Assignee
		if name == "" {
			continue
		}

		sc, ok := agents[name]
		if !ok {
			sc = &AgentScorecard{Name: name}
			agents[name] = sc
		}

		sc.IssuesClosed++
	}

	result := make([]AgentScorecard, 0, len(agents))
	for _, sc := range agents {
		result = append(result, *sc)
	}

	// Sort by issues closed descending
	sort.Slice(result, func(i, j int) bool {
		return result[i].IssuesClosed > result[j].IssuesClosed
	})

	return result
}
