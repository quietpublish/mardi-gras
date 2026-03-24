package views

import (
	"testing"

	"github.com/matt-wright86/mardi-gras/internal/data"
)

func BenchmarkParadeView(b *testing.B) {
	issues := make([]data.Issue, 1000)
	for i := 0; i < 1000; i++ {
		issues[i] = data.Issue{
			ID:     "mg-1000",
			Title:  "Test Issue",
			Status: data.StatusInProgress,
		}
	}
	bt := data.DefaultBlockingTypes
	p := NewParade(issues, 100, 40, bt)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = p.View()
	}
}
