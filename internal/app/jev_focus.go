package app

import (
	"github.com/matt-wright86/mardi-gras/internal/data"
	"github.com/matt-wright86/mardi-gras/internal/jev"
)

// Jev-ranked focus mode: the first sweep feature. Focus mode (`f`) shows
// your in-progress work, then the five ready issues it thinks you should
// take next, then a few blocked ones for context. Today "next" means
// highest priority, so five P2 issues created the same day are a coin flip.
// With Jev on, the sweep loop asks two questions about every open issue
// (how soon it should be started, and whether it can be started now), and
// focus mode orders the ready list by the answers: confident urgency first,
// an issue the judge is sure cannot start moved down with the blocked ones,
// and anything without a confident verdict left where its priority puts it.
// The parade marks ranked rows with a heat-coloured › and the detail panel
// shows the verdict as a "Next up:" row. Without verdicts, focus mode is
// exactly FocusFilter.

// Question names, which are also the keys of the cached answers.
const (
	focusQUrgency    = "urgency"
	focusQActionable = "actionable"
)

// focusQuestions is the sweep's question set: asked about every open issue
// whose snapshot changed, cached by the loop.
func focusQuestions(data.IssueSnapshot) map[string]jev.Question {
	return map[string]jev.Question{
		focusQUrgency: jev.NewScore(
			"How soon should this issue be started, relative to the other issues in this list? "+
				"Being overdue or due within a few days raises it; blocking other open work raises it; "+
				"P0/P1 raises it; being stale, deferred, or vaguely titled lowers it; "+
				"an issue someone else is already working on is Park.",
			data.FocusLevels...),
		focusQActionable: jev.NewNoul(
			"Could a developer start this issue right now without first asking a question or waiting on someone? " +
				"No open blockers, enough of a description to begin, and not assigned to someone else."),
	}
}

// focusVerdicts turns the loop's cached answers into data's verdict type.
// Issues with no urgency answer are left out, so they sort by priority.
func (m Model) focusVerdicts() map[string]data.FocusVerdict {
	if !m.jev.enabled() || len(m.jev.cache) == 0 {
		return nil
	}
	out := make(map[string]data.FocusVerdict, len(m.jev.cache))
	for id, e := range m.jev.cache {
		u, ok := e.answers[focusQUrgency]
		if !ok {
			continue
		}
		v := data.FocusVerdict{Urgency: expectedLevel(u, data.FocusLevels), Confidence: u.Confidence}
		if a, ok := e.answers[focusQActionable]; ok {
			v.Actionable, v.ActionableConfidence = a.Noul, a.Confidence
		}
		out[id] = v
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// expectedLevel is the mean position on the level scale: the sum of each
// level's index weighted by its probability. A server that reports only a
// point score is taken at its word.
func expectedLevel(a jev.Answer, levels []string) float64 {
	if len(a.Probabilities) == 0 {
		return a.Score
	}
	var sum, total float64
	for i, l := range levels {
		p := a.Probabilities[l]
		sum += float64(i) * p
		total += p
	}
	if total == 0 {
		return a.Score
	}
	return sum / total
}

// focusVerdictFor returns the verdict for one issue, or nil.
func (m Model) focusVerdictFor(issueID string) *data.FocusVerdict {
	e, ok := m.jev.cache[issueID]
	if !ok {
		return nil
	}
	u, ok := e.answers[focusQUrgency]
	if !ok {
		return nil
	}
	v := data.FocusVerdict{Urgency: expectedLevel(u, data.FocusLevels), Confidence: u.Confidence}
	if a, ok := e.answers[focusQActionable]; ok {
		v.Actionable, v.ActionableConfidence = a.Noul, a.Confidence
	}
	return &v
}

// focusRanks is what the parade draws: urgency per ranked issue, only in
// focus mode and only for confident verdicts.
func (m Model) focusRanks(verdicts map[string]data.FocusVerdict) map[string]float64 {
	if !m.focusMode || len(verdicts) == 0 {
		return nil
	}
	ranks := make(map[string]float64, len(verdicts))
	for id, v := range verdicts {
		if v.Confidence >= data.FocusConfidenceFloor {
			ranks[id] = v.Urgency
		}
	}
	if len(ranks) == 0 {
		return nil
	}
	return ranks
}

// applyFocusVerdicts refreshes the views that show verdicts after a sweep
// lands: the detail row for the selected issue, and in focus mode the
// parade's order and badges.
func (m *Model) applyFocusVerdicts() {
	if sel := m.parade.SelectedIssue; sel != nil {
		m.detail.SetFocusVerdict(sel.ID, m.focusVerdictFor(sel.ID))
	}
	if m.focusMode {
		m.rebuildParade()
	}
}
