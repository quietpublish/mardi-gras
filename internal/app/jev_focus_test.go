package app

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/matt-wright86/mardi-gras/internal/data"
	"github.com/matt-wright86/mardi-gras/internal/jev"
)

// focusJudge answers the focus sweep's questions from a per-issue table.
// Keys on the wire are "<id>/urgency" and "<id>/actionable".
func focusJudge(verdicts map[string]data.FocusVerdict) *fakeJudge {
	return &fakeJudge{answer: func(key string, q jev.Question) (jev.Answer, bool) {
		id, name, ok := strings.Cut(key, "/")
		v, known := verdicts[id]
		if !ok || !known {
			return jev.Answer{}, false
		}
		switch name {
		case focusQUrgency:
			// Put all the mass on the nearest level so the expected level
			// round-trips through the probabilities.
			probs := map[string]float64{}
			for _, l := range data.FocusLevels {
				probs[l] = 0
			}
			probs[data.FocusLevelLabel(v.Urgency)] = 1
			return jev.Answer{Type: jev.Score, Probabilities: probs, Confidence: v.Confidence}, true
		case focusQActionable:
			return jev.Answer{Type: jev.Noul, Noul: v.Actionable, Confidence: v.ActionableConfidence}, true
		}
		return jev.Answer{}, false
	}}
}

func focusIssues() []data.Issue {
	return []data.Issue{
		testIssue("p0", data.StatusOpen),
		testIssue("p2a", data.StatusOpen),
		testIssue("p2b", data.StatusOpen),
		testIssue("done", data.StatusClosed),
	}
}

func newFocusModel(t *testing.T, judge jev.Evaluator) Model {
	t.Helper()
	issues := focusIssues()
	issues[0].Priority = data.PriorityCritical
	m := New(issues, data.Source{}, data.DefaultBlockingTypes)
	model, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = model.(Model)
	m.jev.client = judge
	m.jev.questions = focusQuestions
	m.jev.now = func() time.Time { return jevTestNow }
	return m
}

func paradeOrder(m Model) []string {
	var ids []string
	for _, item := range m.parade.Items {
		if item.Issue != nil {
			ids = append(ids, item.Issue.ID)
		}
	}
	return ids
}

func TestFocusQuestionsRegisteredWhenEnabled(t *testing.T) {
	t.Setenv(jev.EnvAPIKey, "k")
	t.Setenv(jev.EnvToggle, "")
	m := New(nil, data.Source{}, data.DefaultBlockingTypes)
	if m.jev.questions == nil {
		t.Fatal("Jev on should register the focus question set on the sweep")
	}
	qs := m.jev.questions(data.IssueSnapshot{})
	if qs[focusQUrgency].Type != jev.Score || qs[focusQActionable].Type != jev.Noul {
		t.Fatalf("questions = %+v", qs)
	}
	t.Setenv(jev.EnvAPIKey, "")
	if New(nil, data.Source{}, data.DefaultBlockingTypes).jev.questions != nil {
		t.Fatal("Jev off: nothing registered, nothing swept")
	}
}

func TestFocusSweepRanksReadyIssues(t *testing.T) {
	judge := focusJudge(map[string]data.FocusVerdict{
		"p0":  {Urgency: 0, Confidence: 0.9, Actionable: 0.9, ActionableConfidence: 0.9}, // park the P0
		"p2a": {Urgency: 3, Confidence: 0.9, Actionable: 0.9, ActionableConfidence: 0.9}, // do now
		"p2b": {Urgency: 1, Confidence: 0.9, Actionable: 0.9, ActionableConfidence: 0.9},
	})
	m := newFocusModel(t, judge)

	// Focus mode before any verdict: priority order, no badges, plain badge.
	// (Set directly: printable keys go through the deferred-key buffer.)
	m.focusMode = true
	m.rebuildParade()
	if !m.focusMode || paradeOrder(m)[0] != "p0" || m.parade.Ranks != nil {
		t.Fatalf("before verdicts: focus %v order %v ranks %v", m.focusMode, paradeOrder(m), m.parade.Ranks)
	}

	m = sweep(t, m, m.scheduleJev())

	if judge.count() != 1 {
		t.Fatalf("calls = %d", judge.count())
	}
	call := judge.calls[0]
	if _, ok := call.questions["p2a/urgency"]; !ok || len(call.questions) != 6 {
		t.Fatalf("expected urgency and actionable for each of 3 open issues, got %d: %v", len(call.questions), call.questions)
	}
	if _, asked := call.questions["done/urgency"]; asked {
		t.Fatal("closed issues are not swept")
	}

	order := paradeOrder(m)
	if strings.Join(order, ",") != "p2a,p2b,p0" {
		t.Fatalf("focus order after verdicts = %v", order)
	}
	if m.parade.Ranks["p2a"] != 3 || len(m.parade.Ranks) != 3 {
		t.Fatalf("ranks = %v", m.parade.Ranks)
	}
	if v := m.focusVerdictFor("p2a"); v == nil || v.Urgency != 3 || v.Actionable != 0.9 {
		t.Fatalf("verdict for p2a = %+v", v)
	}
	if !strings.Contains(m.View().Content, "FOCUS · jev") {
		t.Fatal("footer badge should say the list is Jev-ordered")
	}

	// Leaving focus mode drops the badges and the ordering.
	m.focusMode = false
	m.rebuildParade()
	if m.parade.Ranks != nil || paradeOrder(m)[0] != "p0" {
		t.Fatalf("after leaving focus: order %v ranks %v", paradeOrder(m), m.parade.Ranks)
	}
}

func TestFocusSweepEdgeCaseLowConfidenceKeepsPriorityOrder(t *testing.T) {
	judge := focusJudge(map[string]data.FocusVerdict{
		"p0":  {Urgency: 0, Confidence: 0.2, Actionable: 0.9, ActionableConfidence: 0.9},
		"p2a": {Urgency: 3, Confidence: 0.2, Actionable: 0.9, ActionableConfidence: 0.9},
		"p2b": {Urgency: 1, Confidence: 0.2, Actionable: 0.9, ActionableConfidence: 0.9},
	})
	m := newFocusModel(t, judge)
	m.focusMode = true
	m = sweep(t, m, m.scheduleJev())
	if paradeOrder(m)[0] != "p0" {
		t.Fatalf("shaky verdicts must not reorder: %v", paradeOrder(m))
	}
	if m.parade.Ranks != nil {
		t.Fatalf("no badges for shaky verdicts: %v", m.parade.Ranks)
	}
	if strings.Contains(m.View().Content, "FOCUS · jev") {
		t.Fatal("badge should not claim Jev ordering when nothing was confident")
	}
}

func TestFocusVerdictShownOnSelection(t *testing.T) {
	judge := focusJudge(map[string]data.FocusVerdict{
		"p0": {Urgency: 2.9, Confidence: 0.9, Actionable: 0.8, ActionableConfidence: 0.9},
	})
	m := newFocusModel(t, judge)
	m = sweep(t, m, m.scheduleJev()) // not in focus mode: verdicts still cached
	if m.focusMode || m.parade.Ranks != nil {
		t.Fatal("outside focus mode the parade is untouched")
	}
	m.restoreParadeSelection("p0")
	m.syncSelection()
	if m.detail.FocusIssueID != "p0" || m.detail.Focus == nil || m.detail.Focus.Urgency != 3 {
		t.Fatalf("detail verdict = %+v for %q", m.detail.Focus, m.detail.FocusIssueID)
	}
	// The closed issue was never swept, so it has no verdict.
	if m.focusVerdictFor("done") != nil {
		t.Fatal("an issue the judge never saw has no verdict")
	}
}

func TestExpectedLevel(t *testing.T) {
	a := jev.Answer{Type: jev.Score, Probabilities: map[string]float64{"Park": 0.1, "Can wait": 0.2, "Do next": 0.3, "Do now": 0.4}}
	if got := expectedLevel(a, data.FocusLevels); got < 1.99 || got > 2.01 {
		t.Fatalf("expected level = %v, want 2.0", got)
	}
	if got := expectedLevel(jev.Answer{Type: jev.Score, Score: 1.5}, data.FocusLevels); got != 1.5 {
		t.Fatalf("a point score should pass through, got %v", got)
	}
}
