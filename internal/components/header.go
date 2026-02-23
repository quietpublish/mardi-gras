package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/matt-wright86/mardi-gras/internal/data"
	"github.com/matt-wright86/mardi-gras/internal/gastown"
	"github.com/matt-wright86/mardi-gras/internal/ui"
)

// Header renders the top title bar with bead string and counts.
type Header struct {
	Width            int
	Groups           map[data.ParadeStatus][]data.Issue
	AgentCount       int
	TownStatus       *gastown.TownStatus
	GasTownAvailable bool
	ProblemCount     int
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

	agentInfo := ""
	if h.AgentCount > 0 {
		agentStyle := lipgloss.NewStyle().Foreground(ui.StatusAgent).Bold(true)
		agentInfo = agentStyle.Render(fmt.Sprintf(" %s%d", ui.SymAgent, h.AgentCount))
	}

	gasTownInfo := ""
	if h.GasTownAvailable && h.TownStatus != nil {
		working := h.TownStatus.WorkingCount()
		totalAgents := len(h.TownStatus.Agents)
		gtStyle := lipgloss.NewStyle().Foreground(ui.BrightPurple).Italic(true)

		parts := []string{fmt.Sprintf("gt:%d/%d", working, totalAgents)}

		if mail := h.TownStatus.UnreadMail(); mail > 0 {
			parts = append(parts, fmt.Sprintf("%s%d", ui.SymMail, mail))
		}

		activeConvoys := 0
		for _, c := range h.TownStatus.Convoys {
			if c.Status == "open" {
				activeConvoys++
			}
		}
		if activeConvoys > 0 {
			parts = append(parts, fmt.Sprintf("%s%d", ui.SymConvoy, activeConvoys))
		}

		if mq := h.TownStatus.MQStatus(); mq != nil && (mq.Pending > 0 || mq.InFlight > 0) {
			mqLabel := fmt.Sprintf("MQ:%d", mq.Pending+mq.InFlight)
			if mq.Health == "stale" || mq.State == "blocked" {
				mqLabel = lipgloss.NewStyle().Foreground(ui.StatusStalled).Bold(true).Render(mqLabel)
			} else {
				mqLabel = gtStyle.Render(mqLabel)
			}
			parts = append(parts, mqLabel)
		}

		gasTownInfo = gtStyle.Render(" " + strings.Join(parts, " "))
	}

	problemInfo := ""
	if h.ProblemCount > 0 {
		warnStyle := lipgloss.NewStyle().Foreground(ui.StatusStalled).Bold(true)
		problemInfo = warnStyle.Render(fmt.Sprintf(" %s%d", ui.SymWarning, h.ProblemCount))
	}

	bar := h.renderProgressBar(total, len(h.Groups[data.ParadePastTheStand]), 20)

	titleLine := lipgloss.JoinHorizontal(
		lipgloss.Center,
		title,
		counts,
		agentInfo,
		gasTownInfo,
		problemInfo,
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

func (h Header) renderProgressBar(total, done, length int) string {
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
