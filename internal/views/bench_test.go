package views

import (
	"github.com/matt-wright86/mardi-gras/internal/data"
	"testing"
)

func BenchmarkParadeRenderIssue(b *testing.B) {
	issue := data.Issue{
		ID:       "mg-1000",
		Title:    "Test Issue",
		Status:   data.StatusInProgress,
		Priority: data.PriorityHigh,
	}
	bt := data.DefaultBlockingTypes
	emptyMap := map[string]*data.Issue{}
	eval := issue.EvaluateDependencies(emptyMap, bt)
	p := NewParade([]data.Issue{issue}, 100, 40, bt)
	item := ParadeItem{Issue: &issue, Section: sections[0], Eval: &eval}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.renderIssue(item, false, 0)
	}
}

func BenchmarkParadeRenderIssueSelected(b *testing.B) {
	issue := data.Issue{
		ID:       "mg-1000",
		Title:    "Test Issue",
		Status:   data.StatusInProgress,
		Priority: data.PriorityHigh,
	}
	bt := data.DefaultBlockingTypes
	emptyMap := map[string]*data.Issue{}
	eval := issue.EvaluateDependencies(emptyMap, bt)
	p := NewParade([]data.Issue{issue}, 100, 40, bt)
	item := ParadeItem{Issue: &issue, Section: sections[0], Eval: &eval}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.renderIssue(item, true, 0)
	}
}
