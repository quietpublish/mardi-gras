package views

import (
	"fmt"
	"strings"
	"time"

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
	SectionMail
)

// GasTownActionMsg carries user intent from the Gas Town panel back to app.go.
type GasTownActionMsg struct {
	Type     string // "nudge", "handoff", "decommission", "convoy_land", "convoy_close", "mail_reply", "mail_archive", "mail_read", "mail_compose"
	Agent    gastown.AgentRuntime
	ConvoyID string
	Mail     gastown.MailMessage
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
	convoyCursor   int                    // cursor within convoy list
	convoyDetails  []gastown.ConvoyDetail // rich convoy data from gt convoy list
	expandedConvoy int                    // index of expanded convoy, -1 = none

	// Mail state
	mailCursor   int                    // cursor within mail list
	mailMessages []gastown.MailMessage  // messages from gt mail inbox
	expandedMail int                    // index of expanded message, -1 = none

	// Costs data
	costs *gastown.CostsOutput

	// Activity feed
	events []gastown.Event

	// Velocity metrics
	velocity *gastown.VelocityMetrics

	// Agent scorecards (HOP quality aggregates)
	scorecards []gastown.AgentScorecard

	// Convoy predictions
	predictions []gastown.ConvoyPrediction

	// Liveness tick state
	tickCount      int                  // incremented every tick for animations
	workStartTimes map[string]time.Time // agent name -> when they started working

	// Activity sparkline data (computed from events)
	agentHistograms map[string][]int // agent name -> bucketed event counts
	agentEventCount map[string]int   // agent name -> total event count
	maxEventCount   int              // max event count across all agents (for heat scaling)
}

// NewGasTown creates a Gas Town panel.
func NewGasTown(width, height int) GasTown {
	return GasTown{
		width:          width,
		height:         height,
		expandedConvoy: -1,
		expandedMail:   -1,
		workStartTimes: make(map[string]time.Time),
	}
}

// SetSize updates dimensions.
func (g *GasTown) SetSize(width, height int) {
	g.width = width
	g.height = height
}

// SetStatus updates the panel with fresh Gas Town state.
func (g *GasTown) SetStatus(status *gastown.TownStatus, env gastown.Env) {
	// Track work start times: record when agents transition to "working"
	if status != nil {
		currentWorking := make(map[string]bool)
		for _, a := range status.Agents {
			if a.State == "working" {
				currentWorking[a.Name] = true
				if _, tracked := g.workStartTimes[a.Name]; !tracked {
					g.workStartTimes[a.Name] = time.Now()
				}
			}
		}
		// Clean up agents that stopped working
		for name := range g.workStartTimes {
			if !currentWorking[name] {
				delete(g.workStartTimes, name)
			}
		}
	}

	g.status = status
	g.env = env
	// Clamp cursor to valid range when agent list changes
	if status != nil && g.agentCursor >= len(status.Agents) {
		g.agentCursor = max(len(status.Agents)-1, 0)
	}
}

// Tick advances the liveness animation state.
func (g *GasTown) Tick() {
	g.tickCount++
}

// TickCount returns the current tick for external use.
func (g *GasTown) TickCount() int {
	return g.tickCount
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

// SetMailMessages updates the mail list from gt mail inbox --json.
func (g *GasTown) SetMailMessages(msgs []gastown.MailMessage) {
	g.mailMessages = msgs
	if g.mailCursor >= len(msgs) {
		g.mailCursor = max(len(msgs)-1, 0)
	}
}

// SetCosts updates the cost data from gt costs --json.
func (g *GasTown) SetCosts(costs *gastown.CostsOutput) {
	g.costs = costs
}

// GetCosts returns the current cost data for velocity computation.
func (g *GasTown) GetCosts() *gastown.CostsOutput {
	return g.costs
}

// GetConvoys returns the current convoy details for predictions.
func (g *GasTown) GetConvoys() []gastown.ConvoyDetail {
	return g.convoyDetails
}

// SetEvents updates the activity event feed and recomputes sparkline data.
func (g *GasTown) SetEvents(events []gastown.Event) {
	g.events = events

	// Compute per-agent histograms (8 buckets over last 24h)
	g.agentHistograms = gastown.AgentActivityHistogram(events, 8, 24*time.Hour)
	g.agentEventCount = gastown.AgentEventCount(events)
	g.maxEventCount = 0
	for _, c := range g.agentEventCount {
		if c > g.maxEventCount {
			g.maxEventCount = c
		}
	}
}

// SetVelocity updates the velocity metrics.
func (g *GasTown) SetVelocity(v *gastown.VelocityMetrics) {
	g.velocity = v
}

// SetScorecards updates the agent quality scorecards.
func (g *GasTown) SetScorecards(cards []gastown.AgentScorecard) {
	g.scorecards = cards
}

// SetPredictions updates the convoy completion predictions.
func (g *GasTown) SetPredictions(preds []gastown.ConvoyPrediction) {
	g.predictions = preds
}

// SelectedMail returns the currently selected mail message, or nil if none.
func (g *GasTown) SelectedMail() *gastown.MailMessage {
	if g.section != SectionMail {
		return nil
	}
	if len(g.mailMessages) == 0 {
		return nil
	}
	if g.mailCursor >= 0 && g.mailCursor < len(g.mailMessages) {
		return &g.mailMessages[g.mailCursor]
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
		// Cycle through sections: Agents → Convoys → Mail → Agents
		switch g.section {
		case SectionAgents:
			if len(g.convoyDetails) > 0 {
				g.section = SectionConvoys
			} else if len(g.mailMessages) > 0 {
				g.section = SectionMail
			}
		case SectionConvoys:
			if len(g.mailMessages) > 0 {
				g.section = SectionMail
			} else {
				g.section = SectionAgents
			}
		case SectionMail:
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
		} else if g.section == SectionMail {
			if g.expandedMail == g.mailCursor {
				g.expandedMail = -1
			} else {
				g.expandedMail = g.mailCursor
				// Emit mark-read action when expanding
				if m := g.SelectedMail(); m != nil && !m.Read {
					mail := *m
					return g, func() tea.Msg {
						return GasTownActionMsg{Type: "mail_read", Mail: mail}
					}
				}
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

	case "r":
		if g.section == SectionMail {
			if m := g.SelectedMail(); m != nil {
				mail := *m
				return g, func() tea.Msg {
					return GasTownActionMsg{Type: "mail_reply", Mail: mail}
				}
			}
		}

	case "d":
		if g.section == SectionMail {
			if m := g.SelectedMail(); m != nil {
				mail := *m
				return g, func() tea.Msg {
					return GasTownActionMsg{Type: "mail_archive", Mail: mail}
				}
			}
		}

	case "w":
		if g.section == SectionAgents {
			if a := g.SelectedAgent(); a != nil {
				agent := *a
				return g, func() tea.Msg {
					return GasTownActionMsg{Type: "mail_compose", Agent: agent}
				}
			}
		} else if g.section == SectionMail {
			if m := g.SelectedMail(); m != nil {
				// Compose a new message to the sender of the selected mail
				agent := gastown.AgentRuntime{
					Name:    m.From,
					Address: m.From,
				}
				return g, func() tea.Msg {
					return GasTownActionMsg{Type: "mail_compose", Agent: agent}
				}
			}
		}
	}

	return g, nil
}

func (g *GasTown) moveCursorDown() {
	switch g.section {
	case SectionAgents:
		count := g.AgentCount()
		if count > 0 && g.agentCursor < count-1 {
			g.agentCursor++
			g.ensureVisible()
		}
	case SectionConvoys:
		count := len(g.convoyDetails)
		if count > 0 && g.convoyCursor < count-1 {
			g.convoyCursor++
		}
	case SectionMail:
		count := len(g.mailMessages)
		if count > 0 && g.mailCursor < count-1 {
			g.mailCursor++
		}
	}
}

func (g *GasTown) moveCursorUp() {
	switch g.section {
	case SectionAgents:
		if g.agentCursor > 0 {
			g.agentCursor--
			g.ensureVisible()
		}
	case SectionConvoys:
		if g.convoyCursor > 0 {
			g.convoyCursor--
		}
	case SectionMail:
		if g.mailCursor > 0 {
			g.mailCursor--
		}
	}
}

func (g *GasTown) jumpTop() {
	switch g.section {
	case SectionAgents:
		g.agentCursor = 0
		g.scrollOff = 0
	case SectionConvoys:
		g.convoyCursor = 0
	case SectionMail:
		g.mailCursor = 0
	}
}

func (g *GasTown) jumpBottom() {
	switch g.section {
	case SectionAgents:
		count := g.AgentCount()
		if count > 0 {
			g.agentCursor = count - 1
			g.ensureVisible()
		}
	case SectionConvoys:
		count := len(g.convoyDetails)
		if count > 0 {
			g.convoyCursor = count - 1
		}
	case SectionMail:
		count := len(g.mailMessages)
		if count > 0 {
			g.mailCursor = count - 1
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
	contentWidth := max(g.width-3, 20) // border (1) + padding (1) + right margin (1)

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
		sections = append(sections, renderRigs(g.status.Rigs, contentWidth))
	}

	if len(g.convoyDetails) > 0 {
		sections = append(sections, g.renderConvoyDetails(contentWidth))
	} else if len(g.status.Convoys) > 0 {
		sections = append(sections, renderConvoys(g.status.Convoys, contentWidth))
	}

	if len(g.mailMessages) > 0 {
		sections = append(sections, g.renderMail(contentWidth))
	}

	if g.costs != nil {
		sections = append(sections, g.renderCosts(contentWidth))
	}

	if len(g.events) > 0 {
		sections = append(sections, g.renderActivity(contentWidth))
	}

	if g.velocity != nil {
		sections = append(sections, g.renderVelocity(contentWidth))
	}

	if len(g.scorecards) > 0 {
		sections = append(sections, g.renderScorecards(contentWidth))
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

	lines = append(lines, ui.SectionDivider("AGENTS", width, g.section == SectionAgents))

	if len(agents) == 0 {
		lines = append(lines, ui.GasTownLabel.Render("  No agents registered"))
		return strings.Join(lines, "\n")
	}

	// Column widths
	nameW := 14
	roleW := 14
	stateW := 16 // extra room for "● working 15m"

	// Header row (extra space for heat indicator column)
	headerStyle := lipgloss.NewStyle().Foreground(ui.Dim).Bold(true)
	header := fmt.Sprintf("   %-*s %-*s %-*s %s",
		nameW, "Name", roleW, "Role", stateW, "State", "Work")
	lines = append(lines, headerStyle.Render(header))
	lines = append(lines, lipgloss.NewStyle().Foreground(ui.Dim).Render(
		"  "+strings.Repeat("─", width-4)))

	for i, a := range agents {
		isSelected := g.section == SectionAgents && i == g.agentCursor

		// State symbol + color (with breathing dot for working agents)
		stateSym := ui.SymIdle
		switch a.State {
		case "working":
			stateSym = ui.SymWorking
		case "spawning":
			stateSym = ui.SymSpawning
		case "backoff", "degraded":
			stateSym = ui.SymBackoff
		case "stuck":
			stateSym = ui.SymStuck
		case "awaiting-gate":
			stateSym = ui.SymGate
		case "paused", "muted":
			stateSym = ui.SymPaused
		}

		stateColor := ui.AgentStateColor(a.State)
		// Breathing effect: working agents alternate bright/dim on tick
		if a.State == "working" && g.tickCount%2 == 1 {
			stateColor = ui.DimGreen
		}
		stateStyle := lipgloss.NewStyle().Foreground(stateColor)

		// Append work duration for working agents
		stateLabel := stateSym + " " + a.State
		if a.State == "working" {
			if started, ok := g.workStartTimes[a.Name]; ok {
				stateLabel += " " + formatDuration(time.Since(started))
			}
		}
		stateStr := stateStyle.Render(fmt.Sprintf("%-*s", stateW, stateLabel))

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

		// Heat indicator (single char showing activity level)
		heat := ui.HeatChar(g.agentEventCount[a.Name], g.maxEventCount)

		// Work title (truncated) — leave room for sparkline
		sparkW := 8
		workWidth := max(width-nameW-roleW-stateW-sparkW-9, 4) // 9 = spaces + heat + mail padding
		work := a.WorkTitle
		if work == "" && a.HookBead != "" {
			work = a.HookBead
		}
		if work == "" {
			work = "-"
		}
		work = truncateGT(work, workWidth)
		workStyle := lipgloss.NewStyle().Foreground(ui.Muted)

		// Activity sparkline (8-char mini graph)
		sparkline := ""
		if hist, ok := g.agentHistograms[a.Name]; ok {
			sparkline = " " + ui.RenderSparkline(hist, sparkW)
		}

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

		row := fmt.Sprintf("%s%s%s %s %s %s%s%s",
			prefix, heat, nameStr, roleStr, stateStr, workStyle.Render(work), sparkline, mailStr)

		if isSelected {
			row = ui.GasTownAgentSelected.Width(width).Render(row)
		}

		lines = append(lines, row)
	}

	return strings.Join(lines, "\n")
}

func renderRigs(rigs []gastown.RigStatus, width int) string {
	var lines []string

	lines = append(lines, ui.SectionDivider("RIGS", width, false))

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

	lines = append(lines, ui.SectionDivider("CONVOYS", width, g.section == SectionConvoys))

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

		// Progress bar + ETA prediction
		barWidth := max(width-16, 10)
		bar := progressBar(c.Completed, c.Total, barWidth)
		barLine := fmt.Sprintf("    %s", bar)

		// Append ETA if prediction exists for this convoy
		for _, pred := range g.predictions {
			if pred.ConvoyID == c.ID && pred.ETALabel != "" && pred.ETALabel != "unknown" {
				etaStyle := lipgloss.NewStyle().Foreground(ui.Dim)
				barLine += etaStyle.Render(fmt.Sprintf("  ETA ~%s", pred.ETALabel))
				break
			}
		}
		lines = append(lines, barLine)

		// Compact pipeline visualization (when collapsed)
		if !isExpanded && len(c.Tracked) > 0 {
			statuses := make([]string, len(c.Tracked))
			for j, t := range c.Tracked {
				statuses[j] = t.Status
			}
			pipeWidth := max(width-8, 10)
			lines = append(lines, "    "+ui.ConvoyPipeline(statuses, pipeWidth))
		}

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

// renderMail renders the mail section with cursor and expand/collapse.
func (g *GasTown) renderMail(width int) string {
	var lines []string

	// Count unread
	unread := 0
	for _, m := range g.mailMessages {
		if !m.Read {
			unread++
		}
	}

	titleStr := "MAIL"
	if unread > 0 {
		titleStr = fmt.Sprintf("MAIL (%d unread)", unread)
	}
	lines = append(lines, ui.SectionDivider(titleStr, width, g.section == SectionMail))

	for i, m := range g.mailMessages {
		isSelected := g.section == SectionMail && i == g.mailCursor
		isExpanded := i == g.expandedMail

		// Unread indicator
		unreadSym := " "
		if !m.Read {
			unreadSym = lipgloss.NewStyle().Foreground(ui.StatusMail).Render(ui.SymMail)
		}

		// Priority indicator
		prioStyle := lipgloss.NewStyle().Foreground(ui.Muted)
		switch m.Priority {
		case "urgent":
			prioStyle = lipgloss.NewStyle().Foreground(ui.PrioP0)
		case "high":
			prioStyle = lipgloss.NewStyle().Foreground(ui.PrioP1)
		}

		// From (truncated)
		from := m.From
		if len(from) > 18 {
			from = from[:15] + "..."
		}
		fromStyle := lipgloss.NewStyle().Foreground(ui.Light)
		if !m.Read {
			fromStyle = fromStyle.Bold(true)
		}

		// Subject (truncated)
		subjectWidth := max(width-24, 10)
		subject := truncateGT(m.Subject, subjectWidth)
		subjectStyle := lipgloss.NewStyle().Foreground(ui.Muted)
		if !m.Read {
			subjectStyle = lipgloss.NewStyle().Foreground(ui.White)
		}

		// Cursor
		prefix := "  "
		if isSelected {
			prefix = ui.ItemCursor.Render(ui.Cursor+" ") + ""
		}

		// Type badge
		typeBadge := ""
		if m.Type == "task" {
			typeBadge = prioStyle.Render("[task] ")
		} else if m.Type == "scavenge" {
			typeBadge = prioStyle.Render("[scav] ")
		}

		row := fmt.Sprintf("%s%s %s %s%s",
			prefix, unreadSym, fromStyle.Render(from), typeBadge, subjectStyle.Render(subject))

		if isSelected {
			row = ui.GasTownAgentSelected.Width(width).Render(row)
		}
		lines = append(lines, row)

		// Expanded: show full message body
		if isExpanded {
			bodyWidth := max(width-8, 20)
			body := m.Body
			if body == "" {
				body = "(no body)"
			}
			// Wrap body lines
			bodyStyle := lipgloss.NewStyle().Foreground(ui.Light).Width(bodyWidth)
			lines = append(lines, "")
			lines = append(lines, "      "+lipgloss.NewStyle().Foreground(ui.Dim).Render("From: ")+fromStyle.Render(m.From))
			if m.Time != "" {
				lines = append(lines, "      "+lipgloss.NewStyle().Foreground(ui.Dim).Render("Time: ")+lipgloss.NewStyle().Foreground(ui.Muted).Render(m.Time))
			}
			lines = append(lines, "")
			for _, bline := range strings.Split(bodyStyle.Render(body), "\n") {
				lines = append(lines, "      "+bline)
			}
			lines = append(lines, "")
		}
	}

	return strings.Join(lines, "\n")
}

// renderConvoys renders the basic convoy section (fallback when no detail data).
func renderConvoys(convoys []gastown.ConvoyInfo, width int) string {
	var lines []string

	lines = append(lines, ui.SectionDivider("CONVOYS", width, false))

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

// renderCosts renders the costs section with per-role breakdown.
func (g *GasTown) renderCosts(width int) string {
	c := g.costs
	var lines []string

	totalLabel := fmt.Sprintf("$%.2f", c.Total.Cost)
	if c.Period != "" {
		totalLabel = fmt.Sprintf("%s: $%.2f", c.Period, c.Total.Cost)
	}
	lines = append(lines, ui.SectionDivider("COSTS", width, false))
	lines = append(lines, "  "+lipgloss.NewStyle().Foreground(ui.Muted).Render(totalLabel))

	if len(c.ByRole) > 0 {
		for _, rc := range c.ByRole {
			roleStyle := lipgloss.NewStyle().Foreground(ui.RoleColor(rc.Role))
			costStyle := lipgloss.NewStyle().Foreground(ui.Muted)
			line := fmt.Sprintf("  %-12s %d sessions  %s",
				roleStyle.Render(rc.Role),
				rc.Sessions,
				costStyle.Render(fmt.Sprintf("$%.2f", rc.Cost)))
			lines = append(lines, line)
		}
	}

	if c.Total.InputTokens > 0 || c.Total.OutputTokens > 0 {
		tokenStyle := lipgloss.NewStyle().Foreground(ui.Dim)
		lines = append(lines, tokenStyle.Render(
			fmt.Sprintf("  tokens: %dk in / %dk out",
				c.Total.InputTokens/1000, c.Total.OutputTokens/1000)))
	}

	return strings.Join(lines, "\n")
}

// renderVelocity renders the workflow velocity metrics section.
func (g *GasTown) renderVelocity(width int) string {
	v := g.velocity
	var lines []string

	lines = append(lines, ui.SectionDivider("VELOCITY", width, false))

	// Issues line
	labelStyle := lipgloss.NewStyle().Foreground(ui.Dim)
	valStyle := lipgloss.NewStyle().Foreground(ui.Light)
	greenStyle := lipgloss.NewStyle().Foreground(ui.BrightGreen)

	issuesLine := fmt.Sprintf("  %s +%d today (+%d week)   %s %d today (%d week)   %s %d open",
		labelStyle.Render("Issues"),
		v.CreatedToday, v.CreatedWeek,
		greenStyle.Render("closed"),
		v.ClosedToday, v.ClosedWeek,
		valStyle.Render(""),
		v.OpenCount)
	lines = append(lines, issuesLine)

	// Agents line
	if v.TotalAgents > 0 {
		pct := 0
		if v.TotalAgents > 0 {
			pct = v.WorkingAgents * 100 / v.TotalAgents
		}
		agentsLine := fmt.Sprintf("  %s %d/%d working (%d%%)",
			labelStyle.Render("Agents"),
			v.WorkingAgents, v.TotalAgents, pct)
		lines = append(lines, agentsLine)
	}

	// Cost line
	if v.TodayCost > 0 || v.TodaySessions > 0 {
		costLine := fmt.Sprintf("  %s $%.2f today   %d sessions",
			labelStyle.Render("Cost  "),
			v.TodayCost, v.TodaySessions)
		lines = append(lines, costLine)
	}

	return strings.Join(lines, "\n")
}

// renderActivity renders the activity feed section.
func (g *GasTown) renderActivity(width int) string {
	var lines []string

	lines = append(lines, ui.SectionDivider("ACTIVITY", width, false))
	lines = append(lines, "  "+lipgloss.NewStyle().Foreground(ui.Muted).Render(
		fmt.Sprintf("last %d events", len(g.events))))

	for _, ev := range g.events {
		ts := formatEventTime(ev.Timestamp)
		label := eventLabel(ev)
		detail := eventDetail(ev, width-24)

		tsStyle := lipgloss.NewStyle().Foreground(ui.Dim)
		labelStyle := lipgloss.NewStyle().Foreground(eventColor(ev))
		detailStyle := lipgloss.NewStyle().Foreground(ui.Light)

		line := fmt.Sprintf("  %s  %s  %s",
			tsStyle.Render(fmt.Sprintf("%-8s", ts)),
			labelStyle.Render(fmt.Sprintf("%-10s", label)),
			detailStyle.Render(truncateGT(detail, width-24)))
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

// eventLabel returns a short label for an event type.
func eventLabel(ev gastown.Event) string {
	switch ev.Type {
	case "session_start":
		return "session"
	case "session_death":
		return "death"
	case "sling":
		return "sling"
	case "nudge":
		return "nudge"
	case "handoff":
		return "handoff"
	case "spawn":
		return "spawn"
	case "patrol_started":
		return "patrol"
	default:
		return ev.Type
	}
}

// eventColor returns a theme color for an event type.
func eventColor(ev gastown.Event) lipgloss.Color {
	switch ev.Type {
	case "sling":
		return ui.BrightGreen // dispatch = forward motion
	case "session_death":
		return ui.StateBackoff // death = red
	case "nudge":
		return ui.BrightGold // nudge = attention
	case "handoff":
		return ui.BrightPurple // handoff = transition
	case "session_start", "spawn":
		return ui.StateSpawn // start = cyan
	case "patrol_started":
		return ui.RoleDeacon // patrol = blue (deacon activity)
	default:
		return ui.Muted
	}
}

// eventDetail extracts a human-readable detail from the event payload.
func eventDetail(ev gastown.Event, _ int) string {
	actor := shortenActor(ev.Actor)
	switch ev.Type {
	case "sling":
		target := gastown.EventPayloadString(ev, "target")
		bead := gastown.EventPayloadString(ev, "bead")
		if target != "" {
			detail := actor + " -> " + shortenActor(target)
			if bead != "" {
				detail += "  " + bead
			}
			return detail
		}
	case "nudge":
		target := gastown.EventPayloadString(ev, "target")
		reason := gastown.EventPayloadString(ev, "reason")
		if target != "" {
			detail := actor + " -> " + shortenActor(target)
			if reason != "" {
				if len(reason) > 30 {
					reason = reason[:27] + "..."
				}
				detail += "  \"" + reason + "\""
			}
			return detail
		}
	case "session_start":
		topic := gastown.EventPayloadString(ev, "topic")
		if topic != "" {
			return actor + " started (" + topic + ")"
		}
		return actor + " started"
	case "session_death":
		reason := gastown.EventPayloadString(ev, "reason")
		if reason != "" {
			return actor + " (" + reason + ")"
		}
		return actor
	case "handoff":
		subject := gastown.EventPayloadString(ev, "subject")
		if subject != "" {
			return actor + ": " + subject
		}
		return actor
	case "spawn":
		polecat := gastown.EventPayloadString(ev, "polecat")
		if polecat != "" {
			return actor + " -> " + polecat
		}
		return actor
	}
	return actor
}

// shortenActor strips common prefixes for compact display.
func shortenActor(s string) string {
	// Already short
	if len(s) <= 16 {
		return s
	}
	return s
}

// formatDuration renders a duration as a compact human-readable label.
func formatDuration(d time.Duration) string {
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	default:
		return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
	}
}

// formatEventTime formats an RFC3339 timestamp as a relative time string.
func formatEventTime(ts string) string {
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return ts
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}

// renderScorecards renders the HOP agent quality scorecards section.
func (g *GasTown) renderScorecards(width int) string {
	var lines []string

	lines = append(lines, ui.SectionDivider("SCORECARDS", width, false))
	lines = append(lines, "  "+lipgloss.NewStyle().Foreground(ui.Muted).Render(
		fmt.Sprintf("%d agents", len(g.scorecards))))

	labelStyle := lipgloss.NewStyle().Foreground(ui.Dim)
	nameStyle := lipgloss.NewStyle().Foreground(ui.Light)

	for _, sc := range g.scorecards {
		stars := ""
		if sc.TotalScored > 0 {
			stars = ui.RenderStarsCompact(sc.AvgQuality) + " "
		}

		crystLabel := ""
		if sc.Crystallizing > 0 || sc.Ephemeral > 0 {
			crystLabel = fmt.Sprintf("  %s%d%s%d",
				lipgloss.NewStyle().Foreground(ui.CrystalColor).Render(ui.SymCrystal),
				sc.Crystallizing,
				lipgloss.NewStyle().Foreground(ui.EphemeralColor).Render(ui.SymEphemeral),
				sc.Ephemeral)
		}

		line := fmt.Sprintf("  %s%-12s %s%s closed %s",
			stars,
			nameStyle.Render(sc.Name),
			labelStyle.Render(fmt.Sprintf("%d", sc.IssuesClosed)),
			labelStyle.Render(" closed"),
			crystLabel)
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

func (g *GasTown) renderHints() string {
	var hint string
	switch g.section {
	case SectionAgents:
		hint = "n nudge  w mail  h handoff  K decommission  j/k navigate  tab section"
	case SectionConvoys:
		hint = "enter expand  l land  x close  j/k navigate  tab section"
	case SectionMail:
		hint = "enter read  r reply  w compose  d archive  j/k navigate  tab section"
	}
	return "\n" + ui.GasTownHint.Render(hint)
}

// progressBar renders a unicode block progress bar with Mardi Gras gradient.
func progressBar(done, total, width int) string {
	if total <= 0 || width <= 0 {
		return strings.Repeat(ui.SymProgressEmpty, width)
	}
	filled := max(min(done*width/total, width), 0)
	empty := width - filled

	emptyStyle := lipgloss.NewStyle().Foreground(ui.Dim)

	filledStr := strings.Repeat(ui.SymProgress, filled)
	return ui.ApplyPartialMardiGrasGradient(filledStr, width) +
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
