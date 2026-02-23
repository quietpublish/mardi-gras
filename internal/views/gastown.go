package views

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/matt-wright86/mardi-gras/internal/gastown"
	"github.com/matt-wright86/mardi-gras/internal/ui"
)

// GasTownActionMsg carries user intent from the Gas Town panel back to app.go.
type GasTownActionMsg struct {
	Type  string // "nudge", "handoff", "decommission"
	Agent gastown.AgentRuntime
}

// GasTown renders the Gas Town control surface panel in place of the detail pane.
type GasTown struct {
	width       int
	height      int
	scrollOff   int // vertical scroll offset (manual, not viewport)
	status      *gastown.TownStatus
	env         gastown.Env
	agentCursor int // cursor index within the agent list
}

// NewGasTown creates a Gas Town panel.
func NewGasTown(width, height int) GasTown {
	return GasTown{
		width:  width,
		height: height,
	}
}

// SetSize updates dimensions.
func (g *GasTown) SetSize(width, height int) {
	g.width = width
	g.height = height
}

// SetStatus updates the panel with fresh Gas Town state.
func (g *GasTown) SetStatus(status *gastown.TownStatus, env gastown.Env) {
	g.status = status
	g.env = env
	// Clamp cursor to valid range when agent list changes
	if status != nil && g.agentCursor >= len(status.Agents) {
		g.agentCursor = len(status.Agents) - 1
		if g.agentCursor < 0 {
			g.agentCursor = 0
		}
	}
}

// SelectedAgent returns the currently selected agent, or nil if none.
func (g *GasTown) SelectedAgent() *gastown.AgentRuntime {
	if g.status == nil || len(g.status.Agents) == 0 {
		return nil
	}
	if g.agentCursor >= 0 && g.agentCursor < len(g.status.Agents) {
		return &g.status.Agents[g.agentCursor]
	}
	return nil
}

// AgentCount returns the number of agents in the current status.
func (g *GasTown) AgentCount() int {
	if g.status == nil {
		return 0
	}
	return len(g.status.Agents)
}

// Update handles key messages for the Gas Town panel.
// Returns a tea.Cmd when the panel wants to emit an action back to app.go.
func (g GasTown) Update(msg tea.Msg) (GasTown, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return g, nil
	}

	agentCount := g.AgentCount()

	switch km.String() {
	case "j", "down":
		if agentCount > 0 && g.agentCursor < agentCount-1 {
			g.agentCursor++
			g.ensureVisible()
		}
		return g, nil

	case "k", "up":
		if g.agentCursor > 0 {
			g.agentCursor--
			g.ensureVisible()
		}
		return g, nil

	case "g":
		g.agentCursor = 0
		g.scrollOff = 0
		return g, nil

	case "G":
		if agentCount > 0 {
			g.agentCursor = agentCount - 1
			g.ensureVisible()
		}
		return g, nil

	case "n":
		if a := g.SelectedAgent(); a != nil {
			agent := *a
			return g, func() tea.Msg {
				return GasTownActionMsg{Type: "nudge", Agent: agent}
			}
		}

	case "h":
		if a := g.SelectedAgent(); a != nil {
			agent := *a
			return g, func() tea.Msg {
				return GasTownActionMsg{Type: "handoff", Agent: agent}
			}
		}

	case "K":
		if a := g.SelectedAgent(); a != nil && a.Role == "polecat" {
			agent := *a
			return g, func() tea.Msg {
				return GasTownActionMsg{Type: "decommission", Agent: agent}
			}
		}
	}

	return g, nil
}

// ensureVisible adjusts scroll offset so the cursor row is on screen.
func (g *GasTown) ensureVisible() {
	// Each agent is 1 line. Header section takes ~8 lines, agent header takes 2 lines.
	// Available height for agent rows = g.height - headerLines
	visibleRows := g.height - 12
	if visibleRows < 3 {
		visibleRows = 3
	}

	if g.agentCursor < g.scrollOff {
		g.scrollOff = g.agentCursor
	}
	if g.agentCursor >= g.scrollOff+visibleRows {
		g.scrollOff = g.agentCursor - visibleRows + 1
	}
}

// View renders the Gas Town panel with border.
func (g GasTown) View() string {
	content := g.renderContent()
	// Manual scrolling: split into lines and take the visible window
	lines := strings.Split(content, "\n")
	if g.scrollOff > 0 && g.scrollOff < len(lines) {
		lines = lines[g.scrollOff:]
	}
	visible := strings.Join(lines, "\n")
	return ui.GasTownBorder.Height(g.height).Render(visible)
}

func (g *GasTown) renderContent() string {
	contentWidth := g.width - 4
	if contentWidth < 20 {
		contentWidth = 20
	}

	if g.status == nil {
		msg := ui.SymTown + " Gas Town not available"
		if g.env.Available {
			msg = ui.SymTown + " Loading Gas Town status..."
		}
		return lipgloss.NewStyle().
			Width(contentWidth).
			Foreground(ui.Muted).
			Render(msg)
	}

	var sections []string

	sections = append(sections, renderTownHeader(g.env, g.status, contentWidth))
	sections = append(sections, g.renderAgentRoster(contentWidth))

	if len(g.status.Rigs) > 0 {
		sections = append(sections, renderRigs(g.status.Rigs, contentWidth))
	}

	if len(g.status.Convoys) > 0 {
		sections = append(sections, renderConvoys(g.status.Convoys, contentWidth))
	}

	// Hint bar at bottom
	sections = append(sections, g.renderHints(contentWidth))

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

// renderAgentRoster renders the agent list with a selectable cursor.
func (g *GasTown) renderAgentRoster(width int) string {
	agents := g.status.Agents
	var lines []string

	lines = append(lines, ui.GasTownTitle.Render("AGENTS"))

	if len(agents) == 0 {
		lines = append(lines, ui.GasTownLabel.Render("  No agents registered"))
		return strings.Join(lines, "\n")
	}

	// Column widths
	nameW := 14
	roleW := 14
	stateW := 12

	// Header row
	headerStyle := lipgloss.NewStyle().Foreground(ui.Dim).Bold(true)
	header := fmt.Sprintf("  %-*s %-*s %-*s %s",
		nameW, "Name", roleW, "Role", stateW, "State", "Work")
	lines = append(lines, headerStyle.Render(header))
	lines = append(lines, lipgloss.NewStyle().Foreground(ui.Dim).Render(
		"  "+strings.Repeat("─", width-4)))

	for i, a := range agents {
		isSelected := i == g.agentCursor

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
		if isSelected {
			nameStyle = nameStyle.Bold(true).Foreground(ui.White)
		}
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

		// Cursor indicator
		prefix := "  "
		if isSelected {
			prefix = ui.ItemCursor.Render(ui.Cursor+" ") + ""
		}

		row := fmt.Sprintf("%s%s %s %s %s%s",
			prefix, nameStr, roleStr, stateStr, workStyle.Render(work), mailStr)

		if isSelected {
			row = ui.GasTownAgentSelected.Width(width).Render(row)
		}

		lines = append(lines, row)
	}

	return strings.Join(lines, "\n")
}

func renderRigs(rigs []gastown.RigStatus, width int) string {
	var lines []string

	lines = append(lines, ui.GasTownTitle.Render("RIGS"))

	for _, r := range rigs {
		var badges []string
		badges = append(badges, fmt.Sprintf("%d polecats", r.PolecatCount))
		if r.CrewCount > 0 {
			badges = append(badges, fmt.Sprintf("%d crew", r.CrewCount))
		}
		if r.HasWitness {
			badges = append(badges, "witness")
		}
		if r.HasRefinery {
			badges = append(badges, "refinery")
		}

		nameStyle := lipgloss.NewStyle().Foreground(ui.Light).Bold(true)
		infoStyle := lipgloss.NewStyle().Foreground(ui.Muted)

		line := fmt.Sprintf("  %s  %s",
			nameStyle.Render(r.Name),
			infoStyle.Render(strings.Join(badges, " | ")))
		lines = append(lines, line)
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

func (g *GasTown) renderHints(width int) string {
	hint := ui.GasTownHint.Render("n nudge  h handoff  K decommission  j/k navigate")
	return "\n" + hint
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
