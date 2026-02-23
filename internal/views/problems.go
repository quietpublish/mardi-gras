package views

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/matt-wright86/mardi-gras/internal/gastown"
	"github.com/matt-wright86/mardi-gras/internal/ui"
)

// Problems renders the problems detection view in place of the detail pane.
type Problems struct {
	width    int
	height   int
	problems []gastown.Problem
	cursor   int
}

// NewProblems creates a Problems panel.
func NewProblems(width, height int) Problems {
	return Problems{width: width, height: height}
}

// SetSize updates dimensions.
func (p *Problems) SetSize(width, height int) {
	p.width = width
	p.height = height
}

// SetProblems updates the problem list.
func (p *Problems) SetProblems(problems []gastown.Problem) {
	p.problems = problems
	if p.cursor >= len(problems) {
		p.cursor = max(len(problems)-1, 0)
	}
}

// Count returns the number of detected problems.
func (p *Problems) Count() int {
	return len(p.problems)
}

// Update handles key events for the problems view.
func (p Problems) Update(msg tea.Msg) (Problems, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return p, nil
	}
	if len(p.problems) == 0 {
		return p, nil
	}

	switch keyMsg.String() {
	case "j", "down":
		if p.cursor < len(p.problems)-1 {
			p.cursor++
		}
	case "k", "up":
		if p.cursor > 0 {
			p.cursor--
		}
	case "g":
		p.cursor = 0
	case "G":
		p.cursor = len(p.problems) - 1

	// Actions on selected problem's agent
	case "n":
		a := p.problems[p.cursor].Agent
		return p, func() tea.Msg {
			return GasTownActionMsg{Type: "nudge", Agent: a}
		}
	case "h":
		a := p.problems[p.cursor].Agent
		return p, func() tea.Msg {
			return GasTownActionMsg{Type: "handoff", Agent: a}
		}
	case "K":
		a := p.problems[p.cursor].Agent
		if a.Role != "polecat" {
			return p, nil
		}
		return p, func() tea.Msg {
			return GasTownActionMsg{Type: "decommission", Agent: a}
		}
	}

	return p, nil
}

// View renders the problems panel.
func (p Problems) View() string {
	var lines []string

	// Header
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(ui.BrightGold)
	if len(p.problems) == 0 {
		lines = append(lines, headerStyle.Render("PROBLEMS"))
		lines = append(lines, "")
		okStyle := lipgloss.NewStyle().Foreground(ui.BrightGreen)
		lines = append(lines, okStyle.Render("  "+ui.SymResolved+" No problems detected"))
	} else {
		warnStyle := lipgloss.NewStyle().Foreground(ui.StatusStalled).Bold(true)
		header := fmt.Sprintf("PROBLEMS (%d detected)", len(p.problems))
		lines = append(lines, warnStyle.Render(header))
		lines = append(lines, "")

		for i, prob := range p.problems {
			lines = append(lines, p.renderProblem(i, prob)...)
			lines = append(lines, "") // spacer between problems
		}

		// Hint bar
		hintStyle := lipgloss.NewStyle().Foreground(ui.Dim)
		lines = append(lines, hintStyle.Render("  n nudge  h handoff  K decommission"))
	}

	content := strings.Join(lines, "\n")

	return ui.DetailBorder.
		Width(p.width).
		Height(p.height).
		Render(content)
}

func (p Problems) renderProblem(idx int, prob gastown.Problem) []string {
	var lines []string

	// Severity + type badge
	var sevSym, sevLabel string
	var sevStyle lipgloss.Style
	switch prob.Severity {
	case "error":
		sevSym = ui.SymStalled
		sevLabel = "ERROR"
		sevStyle = lipgloss.NewStyle().Foreground(ui.StatusStalled).Bold(true)
	default:
		sevSym = ui.SymOverdue
		sevLabel = "WARN"
		sevStyle = lipgloss.NewStyle().Foreground(ui.BrightGold).Bold(true)
	}

	typeLabel := strings.ToUpper(prob.Type)
	agentLabel := fmt.Sprintf("%s %s", prob.Agent.Role, prob.Agent.Name)

	prefix := "  "
	if idx == p.cursor {
		prefix = ui.ItemCursor.Render(ui.Cursor) + " "
	}

	// First line: severity + type + agent
	line1 := fmt.Sprintf("%s%s %s  %s",
		prefix,
		sevStyle.Render(sevSym+" "+sevLabel),
		lipgloss.NewStyle().Foreground(ui.Light).Bold(true).Render(typeLabel),
		lipgloss.NewStyle().Foreground(ui.Muted).Render(agentLabel),
	)
	lines = append(lines, line1)

	// Second line: detail
	detailStyle := lipgloss.NewStyle().Foreground(ui.Light)
	lines = append(lines, "    "+detailStyle.Render(prob.Detail))

	return lines
}
