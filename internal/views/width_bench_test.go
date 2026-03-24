package views

import (
	"testing"
	"charm.land/lipgloss/v2"
)

func BenchmarkLipglossWidthWithAnsi(b *testing.B) {
	s := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF00FF")).Render("Some text with ANSI")
	s += lipgloss.NewStyle().Foreground(lipgloss.Color("#00FFFF")).Render(" and more ANSI")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = lipgloss.Width(s)
	}
}
