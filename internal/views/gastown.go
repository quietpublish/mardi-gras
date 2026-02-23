package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/matt-wright86/mardi-gras/internal/gastown"
	"github.com/matt-wright86/mardi-gras/internal/ui"
)

// GasTown renders the Gas Town control surface panel in place of the detail pane.
type GasTown struct {
	width    int
	height   int
	viewport viewport.Model
	status   *gastown.TownStatus
	env      gastown.Env
}

// NewGasTown creates a Gas Town panel.
func NewGasTown(width, height int) GasTown {
	vp := viewport.New(width-2, height)
	g := GasTown{
		width:    width,
		height:   height,
		viewport: vp,
	}
	g.viewport.SetContent(g.renderContent())
	return g
}

// SetSize updates dimensions.
func (g *GasTown) SetSize(width, height int) {
	g.width = width
	g.height = height
	g.viewport.Width = width - 2
	g.viewport.Height = height
	g.viewport.SetContent(g.renderContent())
}

// SetStatus updates the panel with fresh Gas Town state.
func (g *GasTown) SetStatus(status *gastown.TownStatus, env gastown.Env) {
	g.status = status
	g.env = env
	g.viewport.SetContent(g.renderContent())
}

// Update forwards viewport messages for scrolling.
func (g GasTown) Update(msg tea.Msg) (GasTown, tea.Cmd) {
	var cmd tea.Cmd
	g.viewport, cmd = g.viewport.Update(msg)
	return g, cmd
}

// View renders the Gas Town panel with border.
func (g GasTown) View() string {
	content := g.viewport.View()
	return ui.GasTownBorder.Height(g.height).Render(content)
}

func (g *GasTown) renderContent() string {
	contentWidth := g.width - 4
	if contentWidth < 20 {
		contentWidth = 20
	}

	if g.status == nil {
		return lipgloss.NewStyle().
			Width(contentWidth).
			Foreground(ui.Muted).
			Render(ui.SymTown + " Gas Town not available")
	}

	var sections []string

	sections = append(sections, renderTownHeader(g.env, g.status, contentWidth))
	sections = append(sections, renderAgentRoster(g.status.Agents, contentWidth))

	if len(g.status.Convoys) > 0 {
		sections = append(sections, renderConvoys(g.status.Convoys, contentWidth))
	}

	return strings.Join(sections, "\n")
}

func renderTownHeader(env gastown.Env, status *gastown.TownStatus, width int) string {
	var lines []string

	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(ui.BrightGold).
		Render(ui.SymTown + " GAS TOWN")
	lines = append(lines, title)
	lines = append(lines, "")

	if env.Role != "" {
		lines = append(lines, ui.GasTownLabel.Render("Role: ")+ui.RoleBadge(env.Role))
	}
	if env.Rig != "" {
		lines = append(lines, ui.GasTownLabel.Render("Rig:  ")+ui.GasTownValue.Render(env.Rig))
	}
	if env.Scope != "" {
		lines = append(lines, ui.GasTownLabel.Render("Scope: ")+ui.GasTownValue.Render(env.Scope))
	}

	// Summary counts
	working := status.WorkingCount()
	total := len(status.Agents)
	mail := status.UnreadMail()

	summary := fmt.Sprintf("%d/%d agents working", working, total)
	if mail > 0 {
		summary += fmt.Sprintf("  %s %d unread", ui.SymMail, mail)
	}
	lines = append(lines, "")
	lines = append(lines, ui.GasTownValue.Render(summary))

	return strings.Join(lines, "\n")
}

func renderAgentRoster(agents []gastown.AgentRuntime, width int) string {
	var lines []string

	lines = append(lines, ui.GasTownTitle.Render("AGENTS"))

	if len(agents) == 0 {
		lines = append(lines, ui.GasTownLabel.Render("  No agents registered"))
		return strings.Join(lines, "\n")
	}

	// Column widths
	nameW := 14
	roleW := 10
	stateW := 12

	// Header row
	headerStyle := lipgloss.NewStyle().Foreground(ui.Dim).Bold(true)
	header := fmt.Sprintf("  %-*s %-*s %-*s %s",
		nameW, "Name", roleW, "Role", stateW, "State", "Work")
	lines = append(lines, headerStyle.Render(header))
	lines = append(lines, lipgloss.NewStyle().Foreground(ui.Dim).Render(
		"  "+strings.Repeat("─", width-4)))

	for _, a := range agents {
		// State symbol + color
		stateSym := ui.SymIdle
		switch a.State {
		case "working":
			stateSym = ui.SymWorking
		case "backoff":
			stateSym = ui.SymBackoff
		}
		stateStyle := lipgloss.NewStyle().Foreground(ui.AgentStateColor(a.State))
		stateStr := stateStyle.Render(fmt.Sprintf("%-*s", stateW, stateSym+" "+a.State))

		// Role with color
		roleStyle := lipgloss.NewStyle().Foreground(ui.RoleColor(a.Role))
		roleStr := roleStyle.Render(fmt.Sprintf("%-*s", roleW, a.Role))

		// Name
		nameStyle := lipgloss.NewStyle().Foreground(ui.Light)
		name := a.Name
		if len(name) > nameW {
			name = name[:nameW-1] + "…"
		}
		nameStr := nameStyle.Render(fmt.Sprintf("%-*s", nameW, name))

		// Work title (truncated)
		workWidth := width - nameW - roleW - stateW - 6
		if workWidth < 8 {
			workWidth = 8
		}
		work := a.WorkTitle
		if work == "" && a.HookBead != "" {
			work = a.HookBead
		}
		if work == "" {
			work = "-"
		}
		work = truncateGT(work, workWidth)
		workStyle := lipgloss.NewStyle().Foreground(ui.Muted)

		// Mail indicator
		mailStr := ""
		if a.Mail > 0 {
			mailStr = lipgloss.NewStyle().Foreground(ui.StatusMail).Render(
				fmt.Sprintf(" %s%d", ui.SymMail, a.Mail))
		}

		row := fmt.Sprintf("  %s %s %s %s%s",
			nameStr, roleStr, stateStr, workStyle.Render(work), mailStr)
		lines = append(lines, row)
	}

	return strings.Join(lines, "\n")
}

func renderConvoys(convoys []gastown.ConvoyInfo, width int) string {
	var lines []string

	lines = append(lines, ui.GasTownTitle.Render("CONVOYS"))

	for _, c := range convoys {
		// Title + status badge
		statusStyle := lipgloss.NewStyle().Foreground(ui.Muted)
		if c.Status == "rolling" || c.Status == "active" {
			statusStyle = lipgloss.NewStyle().Foreground(ui.BrightGreen)
		}
		titleLine := fmt.Sprintf("  %s  %s",
			ui.GasTownValue.Render(c.Title),
			statusStyle.Render("["+c.Status+"]"))
		lines = append(lines, titleLine)

		// Progress bar
		barWidth := width - 16
		if barWidth < 10 {
			barWidth = 10
		}
		bar := progressBar(c.Done, c.Total, barWidth)
		label := fmt.Sprintf("%d/%d", c.Done, c.Total)
		lines = append(lines, fmt.Sprintf("  %s %s", bar, ui.GasTownLabel.Render(label)))
	}

	return strings.Join(lines, "\n")
}

// progressBar renders a unicode block progress bar.
func progressBar(done, total, width int) string {
	if total <= 0 || width <= 0 {
		return strings.Repeat(ui.SymProgressEmpty, width)
	}
	filled := done * width / total
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}
	empty := width - filled

	filledStyle := lipgloss.NewStyle().Foreground(ui.BrightGreen)
	emptyStyle := lipgloss.NewStyle().Foreground(ui.Dim)

	return filledStyle.Render(strings.Repeat(ui.SymProgress, filled)) +
		emptyStyle.Render(strings.Repeat(ui.SymProgressEmpty, empty))
}

// truncateGT truncates a string for the Gas Town panel.
func truncateGT(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return string(runes[:maxLen])
	}
	return string(runes[:maxLen-3]) + "..."
}
