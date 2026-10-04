package gastown

import (
	"testing"

	"github.com/matt-wright86/mardi-gras/internal/data"
)

func TestRecommendFormulasSecurityLabel(t *testing.T) {
	issue := data.Issue{
		ID:        "bd-001",
		Title:     "Fix token validation",
		IssueType: data.TypeBug,
		Labels:    []string{"security"},
	}
	recs := RecommendFormulas(issue)
	if len(recs) == 0 {
		t.Fatal("expected recommendations")
	}
	if recs[0].Formula != "security-audit" {
		t.Fatalf("expected security-audit first, got %q", recs[0].Formula)
	}
}

func TestRecommendFormulasSecurityTitle(t *testing.T) {
	issue := data.Issue{
		ID:        "bd-002",
		Title:     "Add authentication middleware",
		IssueType: data.TypeFeature,
	}
	recs := RecommendFormulas(issue)

	hasSecurityAudit := false
	for _, r := range recs {
		if r.Formula == "security-audit" {
			hasSecurityAudit = true
			break
		}
	}
	if !hasSecurityAudit {
		t.Fatal("expected security-audit recommendation for auth-related issue")
	}
}

func TestRecommendFormulasFeature(t *testing.T) {
	issue := data.Issue{
		ID:        "bd-003",
		Title:     "New dashboard widget",
		IssueType: data.TypeFeature,
		Priority:  data.PriorityMedium,
	}
	recs := RecommendFormulas(issue)

	hasShiny := false
	for _, r := range recs {
		if r.Formula == "shiny" {
			hasShiny = true
			break
		}
	}
	if !hasShiny {
		t.Fatal("expected shiny recommendation for feature type")
	}
}

func TestRecommendFormulasHighPriority(t *testing.T) {
	issue := data.Issue{
		ID:        "bd-004",
		Title:     "Critical production outage",
		IssueType: data.TypeBug,
		Priority:  data.PriorityCritical,
	}
	recs := RecommendFormulas(issue)

	hasRuleOfFive := false
	for _, r := range recs {
		if r.Formula == "rule-of-five" {
			hasRuleOfFive = true
			break
		}
	}
	if !hasRuleOfFive {
		t.Fatal("expected rule-of-five for high-priority issue")
	}
}

func TestRecommendFormulasDefaultFallback(t *testing.T) {
	issue := data.Issue{
		ID:        "bd-005",
		Title:     "Update docs",
		IssueType: data.TypeChore,
		Priority:  data.PriorityBacklog,
	}
	recs := RecommendFormulas(issue)
	if len(recs) == 0 {
		t.Fatal("expected at least one recommendation")
	}

	// Should always have mol-polecat-work somewhere
	hasDefault := false
	for _, r := range recs {
		if r.Formula == "mol-polecat-work" {
			hasDefault = true
			break
		}
	}
	if !hasDefault {
		t.Fatal("expected mol-polecat-work as fallback")
	}
}

func TestRecommendFormulasSortedByScore(t *testing.T) {
	// Security feature with high priority → multiple recommendations
	issue := data.Issue{
		ID:        "bd-006",
		Title:     "Implement security review for auth module",
		IssueType: data.TypeFeature,
		Priority:  data.PriorityHigh,
		Labels:    []string{"security", "review"},
	}
	recs := RecommendFormulas(issue)
	if len(recs) < 2 {
		t.Fatalf("expected multiple recommendations, got %d", len(recs))
	}

	// Verify descending score order
	for i := 1; i < len(recs); i++ {
		if recs[i].Score > recs[i-1].Score {
			t.Fatalf("recommendations not sorted: %d (score %d) after %d (score %d)",
				i, recs[i].Score, i-1, recs[i-1].Score)
		}
	}
}

func TestRecommendFormulasReviewLabel(t *testing.T) {
	issue := data.Issue{
		ID:        "bd-007",
		Title:     "PR needs code review",
		IssueType: data.TypeTask,
	}
	recs := RecommendFormulas(issue)

	hasCodeReview := false
	for _, r := range recs {
		if r.Formula == "code-review" {
			hasCodeReview = true
			break
		}
	}
	if !hasCodeReview {
		t.Fatal("expected code-review for issue with 'review' in title")
	}
}

func TestRankFormulas(t *testing.T) {
	installed := []string{"mol-polecat-work", "shiny", "security-audit", "custom-flow"}
	probs := map[string]float64{"shiny": 0.62, "mol-polecat-work": 0.25, "security-audit": 0.08, "custom-flow": 0.05}
	recs := RankFormulas(installed, probs, 0.4)
	if len(recs) != 4 || recs[0].Formula != "shiny" || recs[0].P != 0.62 || recs[1].Formula != "mol-polecat-work" {
		t.Fatalf("recs = %+v", recs)
	}
	if recs[0].Reason != FormulaDescriptions["shiny"] {
		t.Fatalf("known formula should carry its description, got %q", recs[0].Reason)
	}
	if recs[3].Formula != "custom-flow" || recs[3].Reason != "installed formula" {
		t.Fatalf("unknown formula should still be offered: %+v", recs[3])
	}
}

func TestRankFormulasEdgeCaseBelowThreshold(t *testing.T) {
	if recs := RankFormulas([]string{"a", "b"}, map[string]float64{"a": 0.35, "b": 0.3}, 0.4); recs != nil {
		t.Fatalf("a weak best should yield nil so the heuristic stays, got %+v", recs)
	}
}

func TestRankFormulasEdgeCaseUnscoredAndEmpty(t *testing.T) {
	recs := RankFormulas([]string{"a", "b"}, map[string]float64{"a": 0.9}, 0.4)
	if len(recs) != 1 || recs[0].Formula != "a" {
		t.Fatalf("unscored formulas should be left out, got %+v", recs)
	}
	if RankFormulas(nil, nil, 0.4) != nil || RankFormulas([]string{"a"}, nil, 0.4) != nil {
		t.Fatal("nothing to rank should be nil")
	}
}
