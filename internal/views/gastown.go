package views

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/matt-wright86/mardi-gras/internal/gastown"
	"github.com/matt-wright86/mardi-gras/internal/ui"
)

// GasTownSection tracks which section of the panel has focus.
type GasTownSection int

const (
	SectionAgents GasTownSection = iota
	SectionConvoys
)

// GasTownActionMsg carries user intent from the Gas Town panel back to app.go.
type GasTownActionMsg struct {
	Type     string // "nudge", "handoff", "decommission", "convoy_land", "convoy_close"
	Agent    gastown.AgentRuntime
	ConvoyID string
}

// GasTown renders the Gas Town control surface panel in place of the detail pane.
type GasTown struct {
	width       int
	height      int
	scrollOff   int // vertical scroll offset (manual, not viewport)
	status      *gastown.TownStatus
	env         gastown.Env
	agentCursor int            // cursor index within the agent list
	section     GasTownSection // which section has focus

	// Convoy state
	convoyCursor   int                      // cursor within convoy list
	convoyDetails  []gastown.ConvoyDetail   // rich convoy data from gt convoy list
	expandedConvoy int                      // index of expanded convoy, -1 = none
}

// NewGasTown creates a Gas Town panel.
func NewGasTown(width, height int) GasTown {
	return GasTown{
		width:          width,
		height:         height,
		expandedConvoy: -1,
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
		g.agentCursor = max(len(status.Agents)-1, 0)
	}
}

// SetConvoyDetails updates the convoy detail list from gt convoy list --json.
func (g *GasTown) SetConvoyDetails(convoys []gastown.ConvoyDetail) {
	g.convoyDetails = convoys
	if g.convoyCursor >= len(convoys) {
		g.convoyCursor = max(len(convoys)-1, 0)
	}
}

// SelectedAgent returns the currently selected agent, or nil if none.
func (g *GasTown) SelectedAgent() *gastown.AgentRuntime {
	if g.section != SectionAgents {
		return nil
	}
	if g.status == nil || len(g.status.Agents) == 0 {
		return nil
	}
	if g.agentCursor >= 0 && g.agentCursor < len(g.status.Agents) {
		return &g.status.Agents[g.agentCursor]
	}
	return nil
}

// SelectedConvoy returns the currently selected convoy, or nil if none.
func (g *GasTown) SelectedConvoy() *gastown.ConvoyDetail {
	if g.section != SectionConvoys {
		return nil
	}
	if len(g.convoyDetails) == 0 {
		return nil
	}
	if g.convoyCursor >= 0 && g.convoyCursor < len(g.convoyDetails) {
		return &g.convoyDetails[g.convoyCursor]
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

// Section returns the currently focused section.
func (g *GasTown) Section() GasTownSection {
	return g.section
}

// Update handles key messages for the Gas Town panel.
// Returns a tea.Cmd when the panel wants to emit an action back to app.go.
func (g GasTown) Update(msg tea.Msg) (GasTown, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return g, nil
	}

	switch km.String() {
	case "tab":
		// Toggle between sections
		if g.section == SectionAgents && len(g.convoyDetails) > 0 {
			g.section = SectionConvoys
		} else {
			g.section = SectionAgents
		}
		return g, nil

	case "j", "down":
		g.moveCursorDown()
		return g, nil

	case "k", "up":
		g.moveCursorUp()
		return g, nil

	case "g":
		g.jumpTop()
		return g, nil

	case "G":
		g.jumpBottom()
		return g, nil

	case "enter":
		if g.section == SectionConvoys {
			if g.expandedConvoy == g.convoyCursor {
				g.expandedConvoy = -1
			} else {
				g.expandedConvoy = g.convoyCursor
			}
		}
		return g, nil

	case "n":
		if g.section == SectionAgents {
			if a := g.SelectedAgent(); a != nil {
				agent := *a
				return g, func() tea.Msg {
					return GasTownActionMsg{Type: "nudge", Agent: agent}
				}
			}
		}

	case "h":
		if g.section == SectionAgents {
			if a := g.SelectedAgent(); a != nil {
				agent := *a
				return g, func() tea.Msg {
					return GasTownActionMsg{Type: "handoff", Agent: agent}
				}
			}
		}

	case "K":
		if g.section == SectionAgents {
			if a := g.SelectedAgent(); a != nil && a.Role == "polecat" {
				agent := *a
				return g, func() tea.Msg {
					return GasTownActionMsg{Type: "decommission", Agent: agent}
				}
			}
		}

	case "l":
		if g.section == SectionConvoys {
			if c := g.SelectedConvoy(); c != nil {
				convoyID := c.ID
				return g, func() tea.Msg {
					return GasTownActionMsg{Type: "convoy_land", ConvoyID: convoyID}
				}
			}
		}

	case "x":
		if g.section == SectionConvoys {
			if c := g.SelectedConvoy(); c != nil {
				convoyID := c.ID
				return g, func() tea.Msg {
					return GasTownActionMsg{Type: "convoy_close", ConvoyID: convoyID}
				}
			}
		}
	}

	return g, nil
}

func (g *GasTown) moveCursorDown() {
	if g.section == SectionAgents {
		count := g.AgentCount()
		if count > 0 && g.agentCursor < count-1 {
			g.agentCursor++
			g.ensureVisible()
		}
	} else {
		count := len(g.convoyDetails)
		if count > 0 && g.convoyCursor < count-1 {
			g.convoyCursor++
		}
	}
}

func (g *GasTown) moveCursorUp() {
	if g.section == SectionAgents {
		if g.agentCursor > 0 {
			g.agentCursor--
			g.ensureVisible()
		}
	} else {
		if g.convoyCursor > 0 {
			g.convoyCursor--
		}
	}
}

func (g *GasTown) jumpTop() {
	if g.section == SectionAgents {
		g.agentCursor = 0
		g.scrollOff = 0
	} else {
		g.convoyCursor = 0
	}
}

func (g *GasTown) jumpBottom() {
	if g.section == SectionAgents {
		count := g.AgentCount()
		if count > 0 {
			g.agentCursor = count - 1
			g.ensureVisible()
		}
	} else {
		count := len(g.convoyDetails)
		if count > 0 {
			g.convoyCursor = count - 1
		}
	}
}

// ensureVisible adjusts scroll offset so the cursor row is on screen.
func (g *GasTown) ensureVisible() {
	visibleRows := max(g.height-12, 3)

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
	contentWidth := max(g.width-4, 20)

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

	sections = append(sections, renderTownHeader(g.env, g.status))
	sections = append(sections, g.renderAgentRoster(contentWidth))

	if len(g.status.Rigs) > 0 {
		sections = append(sections, renderRigs(g.status.Rigs))
	}

	if len(g.convoyDetails) > 0 {
		sections = append(sections, g.renderConvoyDetails(contentWidth))
	} else if len(g.status.Convoys) > 0 {
		sections = append(sections, renderConvoys(g.status.Convoys, contentWidth))
	}

	// Hint bar at bottom
	sections = append(sections, g.renderHints())

	return strings.Join(sections, "\n")
}

func renderTownHeader(env gastown.Env, status *gastown.TownStatus) string {
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

	// Section title with focus indicator
	titleStr := "AGENTS"
	if g.section == SectionAgents {
		titleStr = ui.Cursor + " AGENTS"
	}
	lines = append(lines, ui.GasTownTitle.Render(titleStr))

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
		isSelected := g.section == SectionAgents && i == g.agentCursor

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
		workWidth := max(width-nameW-roleW-stateW-6, 8)
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

func renderRigs(rigs []gastown.RigStatus) string {
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

// renderConvoyDetails renders the rich convoy section with cursor and expand/collapse.
func (g *GasTown) renderConvoyDetails(width int) string {
	var lines []string

	titleStr := "CONVOYS"
	if g.section == SectionConvoys {
		titleStr = ui.Cursor + " CONVOYS"
	}
	lines = append(lines, ui.GasTownTitle.Render(titleStr))

	for i, c := range g.convoyDetails {
		isSelected := g.section == SectionConvoys && i == g.convoyCursor
		isExpanded := i == g.expandedConvoy

		// Status badge
		statusColor := ui.Muted
		if c.Status == "open" {
			statusColor = ui.BrightGreen
		}
		statusStyle := lipgloss.NewStyle().Foreground(statusColor)

		// Expand/collapse indicator
		expandSym := "+"
		if isExpanded {
			expandSym = "-"
		}

		// Cursor
		prefix := "  "
		if isSelected {
			prefix = ui.ItemCursor.Render(ui.Cursor+" ") + ""
		}

		titleLine := fmt.Sprintf("%s%s %s  %s  %d/%d",
			prefix,
			lipgloss.NewStyle().Foreground(ui.Dim).Render(expandSym),
			ui.GasTownValue.Render(truncateGT(c.Title, width-20)),
			statusStyle.Render("["+c.Status+"]"),
			c.Completed, c.Total)

		if isSelected {
			titleLine = ui.GasTownAgentSelected.Width(width).Render(titleLine)
		}
		lines = append(lines, titleLine)

		// Progress bar
		barWidth := max(width-16, 10)
		bar := progressBar(c.Completed, c.Total, barWidth)
		lines = append(lines, fmt.Sprintf("    %s", bar))

		// Expanded: show tracked issues
		if isExpanded && len(c.Tracked) > 0 {
			for _, t := range c.Tracked {
				sym := ui.SymIdle
				issueColor := ui.Muted
				switch t.Status {
				case "closed":
					sym = ui.SymResolved
					issueColor = ui.BrightGreen
				case "in_progress", "hooked":
					sym = ui.SymWorking
					issueColor = ui.BrightGold
				case "open":
					sym = ui.SymLinedUp
				}
				style := lipgloss.NewStyle().Foreground(issueColor)

				issueLine := fmt.Sprintf("      %s %s",
					style.Render(sym),
					style.Render(truncateGT(t.ID+" "+t.Title, width-10)))

				if t.Worker != "" {
					workerStyle := lipgloss.NewStyle().Foreground(ui.Dim)
					issueLine += workerStyle.Render(fmt.Sprintf(" [%s]", t.Worker))
				}

				lines = append(lines, issueLine)
			}
		}
	}

	return strings.Join(lines, "\n")
}

// renderConvoys renders the basic convoy section (fallback when no detail data).
func renderConvoys(convoys []gastown.ConvoyInfo, width int) string {
	var lines []string

	lines = append(lines, ui.GasTownTitle.Render("CONVOYS"))

	for _, c := range convoys {
		statusStyle := lipgloss.NewStyle().Foreground(ui.Muted)
		if c.Status == "rolling" || c.Status == "active" || c.Status == "open" {
			statusStyle = lipgloss.NewStyle().Foreground(ui.BrightGreen)
		}
		titleLine := fmt.Sprintf("  %s  %s",
			ui.GasTownValue.Render(c.Title),
			statusStyle.Render("["+c.Status+"]"))
		lines = append(lines, titleLine)

		barWidth := max(width-16, 10)
		bar := progressBar(c.Done, c.Total, barWidth)
		label := fmt.Sprintf("%d/%d", c.Done, c.Total)
		lines = append(lines, fmt.Sprintf("  %s %s", bar, ui.GasTownLabel.Render(label)))
	}

	return strings.Join(lines, "\n")
}

func (g *GasTown) renderHints() string {
	hint := "n nudge  h handoff  K decommission  j/k navigate  tab section"
	if g.section == SectionConvoys {
		hint = "enter expand  l land  x close  j/k navigate  tab section"
	}
	return "\n" + ui.GasTownHint.Render(hint)
}

// progressBar renders a unicode block progress bar.
func progressBar(done, total, width int) string {
	if total <= 0 || width <= 0 {
		return strings.Repeat(ui.SymProgressEmpty, width)
	}
	filled := max(min(done*width/total, width), 0)
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
