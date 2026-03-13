package views

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/matt-wright86/mardi-gras/internal/gastown"
	"github.com/matt-wright86/mardi-gras/internal/ui"
)

// RecoveryActionMsg is emitted when the user presses R on a dead_rig problem.
type RecoveryActionMsg struct {
	RigName string
	Orphans []gastown.OrphanedIssue
}

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
	keyMsg, ok := msg.(tea.KeyPressMsg)
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
	case "R":
		prob := p.problems[p.cursor]
		if prob.Type != "dead_rig" {
			return p, nil
		}
		return p, func() tea.Msg {
			return RecoveryActionMsg{RigName: prob.RigName, Orphans: prob.Orphans}
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
		hasDeadRig := false
		for _, prob := range p.problems {
			if prob.Type == "dead_rig" {
				hasDeadRig = true
				break
			}
		}
		hint := "  n nudge  h handoff  K decommission"
		if hasDeadRig {
			hint += "  R recover rig"
		}
		lines = append(lines, hintStyle.Render(hint))
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

	prefix := "  "
	if idx == p.cursor {
		prefix = ui.ItemCursor.Render(ui.Cursor) + " "
	}

	// Context label: agent info for agent problems, category for doctor problems
	contextLabel := ""
	switch prob.Type {
	case "doctor":
		if prob.Category != "" {
			contextLabel = prob.Category
		}
	case "dead_rig":
		contextLabel = "rig " + prob.RigName
	case "agent_exited", "agent_permission":
		contextLabel = "tmux agent"
	default:
		contextLabel = fmt.Sprintf("%s %s", prob.Agent.Role, prob.Agent.Name)
	}

	// First line: severity + type + context
	typeSym := ""
	if prob.Type == "dead_rig" {
		typeSym = ui.SymDeadRig + " "
	}
	line1 := fmt.Sprintf("%s%s %s%s  %s",
		prefix,
		sevStyle.Render(sevSym+" "+sevLabel),
		typeSym,
		lipgloss.NewStyle().Foreground(ui.Light).Bold(true).Render(typeLabel),
		lipgloss.NewStyle().Foreground(ui.Muted).Render(contextLabel),
	)
	lines = append(lines, line1)

	// Second line: detail
	detailStyle := lipgloss.NewStyle().Foreground(ui.Light)
	lines = append(lines, "    "+detailStyle.Render(prob.Detail))

	// Orphan list for dead_rig problems
	if prob.Type == "dead_rig" && len(prob.Orphans) > 0 {
		orphanStyle := lipgloss.NewStyle().Foreground(ui.Muted)
		for _, o := range prob.Orphans {
			title := o.Title
			if title == "" {
				title = o.IssueID
			}
			orphanLine := fmt.Sprintf("      %s %s  %s",
				lipgloss.NewStyle().Foreground(ui.StatusStalled).Render(ui.SymIdle),
				orphanStyle.Render(o.IssueID),
				orphanStyle.Render(title))
			if o.AgentName != "" {
				orphanLine += lipgloss.NewStyle().Foreground(ui.Dim).Render(
					fmt.Sprintf(" (was %s)", o.AgentName))
			}
			lines = append(lines, orphanLine)
		}
	}

	// Fix command
	if prob.Fix != "" {
		fixStyle := lipgloss.NewStyle().Foreground(ui.Dim).Italic(true)
		lines = append(lines, "    "+fixStyle.Render("fix: "+prob.Fix))
	}

	return lines
}
