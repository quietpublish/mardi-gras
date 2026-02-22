package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/matt-wright86/mardi-gras/internal/data"
	"github.com/matt-wright86/mardi-gras/internal/ui"
)

// Header renders the top title bar with bead string and counts.
type Header struct {
	Width  int
	Groups map[data.ParadeStatus][]data.Issue
}

// View renders the header.
func (h Header) View() string {
	rolling := len(h.Groups[data.ParadeRolling])
	linedUp := len(h.Groups[data.ParadeLinedUp])
	stalled := len(h.Groups[data.ParadeStalled])
	total := rolling + linedUp + stalled + len(h.Groups[data.ParadePastTheStand])

	titleStr := fmt.Sprintf("%s MARDI GRAS %s", ui.FleurDeLis, ui.FleurDeLis)
	title := ui.HeaderStyle.Render(ui.ApplyMardiGrasGradient(titleStr))

	counts := ui.HeaderCounts.Render(fmt.Sprintf(
		" %d ⊘  %d ♪  %d ●  %d ✓ ",
		stalled, linedUp, rolling, len(h.Groups[data.ParadePastTheStand]),
	))

	bar := h.renderProgressBar(total, len(h.Groups[data.ParadePastTheStand]), 20)

	titleLine := lipgloss.JoinHorizontal(
		lipgloss.Center,
		title,
		counts,
		"  ",
		bar,
	)

	// Pad to full width
	titleLine = lipgloss.NewStyle().Width(h.Width).Render(titleLine)

	beadStr := h.renderBeadString()

	return lipgloss.JoinVertical(lipgloss.Left, titleLine, beadStr)
}

// renderBeadString creates the decorative bead string separator.
func (h Header) renderBeadString() string {
	beads := []string{ui.BeadRound, ui.BeadDiamond}

	var parts []string
	visibleWidth := 0
	ci := 0
	for visibleWidth < h.Width-2 {
		bead := beads[ci%2]
		parts = append(parts, bead)
		visibleWidth++
		if visibleWidth < h.Width-2 {
			parts = append(parts, ui.BeadDash)
			visibleWidth++
		}
		ci++
	}

	rawString := strings.Join(parts, "")
	gradientString := ui.ApplyMardiGrasGradient(rawString)
	return lipgloss.NewStyle().Width(h.Width).Render(gradientString)
}

func (h Header) renderProgressBar(total, done int, length int) string {
	if total == 0 {
		return ""
	}
	filledLen := int((float64(done) / float64(total)) * float64(length))
	emptyLen := length - filledLen

	filled := strings.Repeat("█", filledLen)
	empty := strings.Repeat("█", emptyLen) // Or "━"

	percent := int((float64(done) / float64(total)) * 100)

	styledFilled := ui.ApplyPartialMardiGrasGradient(filled, length)
	styledEmpty := lipgloss.NewStyle().Foreground(ui.DimPurple).Render(empty)

	textRight := ui.HeaderCounts.Render(fmt.Sprintf(" %d%%", percent))

	return styledFilled + styledEmpty + textRight
}
