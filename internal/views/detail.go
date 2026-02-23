package views

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/matt-wright86/mardi-gras/internal/data"
	"github.com/matt-wright86/mardi-gras/internal/gastown"
	"github.com/matt-wright86/mardi-gras/internal/ui"
)

// Detail renders the right-panel issue details with a scrollable viewport.
type Detail struct {
	Issue              *data.Issue
	AllIssues          []data.Issue
	IssueMap           map[string]*data.Issue
	BlockingTypes      map[string]bool
	Viewport           viewport.Model
	Width              int
	Height             int
	Focused            bool
	ActiveAgents       map[string]string
	TownStatus         *gastown.TownStatus
	MoleculeDAG        *gastown.DAGInfo
	MoleculeProgress   *gastown.MoleculeProgress
	MoleculeIssueID    string // which issue the molecule data belongs to
	Comments           []gastown.Comment
	CommentsIssueID    string // which issue the comments belong to
}

// NewDetail creates a detail panel.
func NewDetail(width, height int, issues []data.Issue) Detail {
	vp := viewport.New(width-2, height)
	return Detail{
		AllIssues: issues,
		IssueMap:  data.BuildIssueMap(issues),
		Viewport:  vp,
		Width:     width,
		Height:    height,
	}
}

// SetIssue updates the displayed issue and rebuilds content.
func (d *Detail) SetIssue(issue *data.Issue) {
	d.Issue = issue
	// Clear stale molecule data when switching issues
	if issue == nil || issue.ID != d.MoleculeIssueID {
		d.MoleculeDAG = nil
		d.MoleculeProgress = nil
		d.MoleculeIssueID = ""
	}
	// Clear stale comments when switching issues
	if issue == nil || issue.ID != d.CommentsIssueID {
		d.Comments = nil
		d.CommentsIssueID = ""
	}
	d.Viewport.SetContent(d.renderContent())
	d.Viewport.GotoTop()
}

// SetMolecule updates the molecule DAG and progress for the current issue.
func (d *Detail) SetMolecule(issueID string, dag *gastown.DAGInfo, progress *gastown.MoleculeProgress) {
	d.MoleculeDAG = dag
	d.MoleculeProgress = progress
	d.MoleculeIssueID = issueID
	if d.Issue != nil {
		d.Viewport.SetContent(d.renderContent())
	}
}

// SetComments updates the comments for the current issue.
func (d *Detail) SetComments(issueID string, comments []gastown.Comment) {
	d.Comments = comments
	d.CommentsIssueID = issueID
	if d.Issue != nil {
		d.Viewport.SetContent(d.renderContent())
	}
}

// SetSize updates dimensions.
func (d *Detail) SetSize(width, height int) {
	d.Width = width
	d.Height = height
	d.Viewport.Width = width - 2
	d.Viewport.Height = height
	if d.Issue != nil {
		d.Viewport.SetContent(d.renderContent())
	}
}

// View renders the detail panel.
func (d *Detail) View() string {
	if d.Issue == nil {
		empty := lipgloss.NewStyle().
			Width(d.Width).
			Height(d.Height).
			Foreground(ui.Muted).
			Align(lipgloss.Center, lipgloss.Center).
			Render("No issue selected")
		return ui.DetailBorder.Height(d.Height).Render(empty)
	}

	content := d.Viewport.View()
	return ui.DetailBorder.Height(d.Height).Render(content)
}

// renderMarkdown renders markdown text using glamour with dark theme.
func (d *Detail) renderMarkdown(text string) string {
	contentWidth := d.Width - 6
	if contentWidth < 20 {
		contentWidth = 20
	}
	r, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(contentWidth),
	)
	if err != nil {
		return wordWrap(text, d.Width-4)
	}
	rendered, err := r.Render(text)
	if err != nil {
		return wordWrap(text, d.Width-4)
	}
	return strings.TrimRight(rendered, "\n")
}

func (d *Detail) renderContent() string {
	issue := d.Issue
	if issue == nil {
		return ""
	}

	bt := d.BlockingTypes
	if bt == nil {
		bt = data.DefaultBlockingTypes
	}

	var lines []string

	// Title
	lines = append(lines, ui.DetailTitle.Render(issue.Title))
	lines = append(lines, "")

	// Status row
	statusSym := statusSymbol(issue, d.IssueMap, bt)
	statusLabel := paradeLabel(issue, d.IssueMap, bt)
	statusStyle := lipgloss.NewStyle().Foreground(statusColor(issue, d.IssueMap, bt))
	lines = append(lines, d.row("Status:", statusStyle.Render(statusSym+" "+statusLabel+" ("+string(issue.Status)+")")))

	// Type
	typeColor := ui.IssueTypeColor(string(issue.IssueType))
	lines = append(lines, d.row("Type:", lipgloss.NewStyle().Foreground(typeColor).Render(string(issue.IssueType))))

	// Priority
	prioColor := ui.PriorityColor(int(issue.Priority))
	prioLabel := fmt.Sprintf("%s (%s)", data.PriorityLabel(issue.Priority), data.PriorityName(issue.Priority))
	lines = append(lines, d.row("Priority:", lipgloss.NewStyle().Foreground(prioColor).Bold(true).Render(prioLabel)))

	// Owner
	if issue.Owner != "" {
		lines = append(lines, d.row("Owner:", ui.DetailValue.Render(issue.Owner)))
	}

	// Assignee
	if issue.Assignee != "" {
		lines = append(lines, d.row("Assignee:", ui.DetailValue.Render(issue.Assignee)))
	}

	// Age
	lines = append(lines, d.row("Age:", ui.DetailValue.Render(issue.AgeLabel())))

	// Due date
	if issue.DueAt != nil {
		dueLabel := issue.DueLabel()
		if issue.IsOverdue() {
			dueLabel = ui.OverdueBadge.Render(ui.SymOverdue + " " + dueLabel)
		} else {
			dueLabel = ui.DueSoonBadge.Render(ui.SymDueDate + " " + dueLabel)
		}
		lines = append(lines, d.row("Due:", dueLabel))
	}

	// Deferred
	if issue.IsDeferred() {
		lines = append(lines, d.row("Deferred:", ui.DeferredStyle.Render(ui.SymDeferred+" "+issue.DeferLabel())))
	}

	// ID
	lines = append(lines, d.row("ID:", ui.DetailValue.Render(issue.ID)))

	// Agent status
	if d.ActiveAgents != nil {
		if _, active := d.ActiveAgents[issue.ID]; active {
			agentStyle := lipgloss.NewStyle().Foreground(ui.StatusAgent).Bold(true)
			if d.TownStatus != nil {
				if a := d.TownStatus.AgentForIssue(issue.ID); a != nil {
					lines = append(lines, d.row("Worker:", agentStyle.Render(
						fmt.Sprintf("%s %s (%s)", ui.SymAgent, a.Name, a.Role),
					)))
					if a.State != "" {
						lines = append(lines, d.row("State:", ui.DetailValue.Render(a.State)))
					}
				} else {
					lines = append(lines, d.row("Agent:", agentStyle.Render(
						fmt.Sprintf("%s active", ui.SymAgent),
					)))
				}
			} else {
				lines = append(lines, d.row("Agent:", agentStyle.Render(
					fmt.Sprintf("%s active", ui.SymAgent),
				)))
			}
		}
	}

	// Description (markdown rendered)
	if issue.Description != "" {
		lines = append(lines, "")
		lines = append(lines, ui.DetailSection.Render("DESCRIPTION"))
		lines = append(lines, d.renderMarkdown(issue.Description))
	}

	// Close reason
	if issue.CloseReason != "" {
		lines = append(lines, "")
		lines = append(lines, ui.DetailSection.Render("CLOSE REASON"))
		lines = append(lines, d.renderMarkdown(issue.CloseReason))
	}

	// Notes (markdown rendered)
	if issue.Notes != "" {
		lines = append(lines, "")
		lines = append(lines, ui.DetailSection.Render("NOTES"))
		lines = append(lines, d.renderMarkdown(issue.Notes))
	}

	// Acceptance Criteria (markdown rendered)
	if issue.AcceptanceCriteria != "" {
		lines = append(lines, "")
		lines = append(lines, ui.DetailSection.Render("ACCEPTANCE CRITERIA"))
		lines = append(lines, d.renderMarkdown(issue.AcceptanceCriteria))
	}

	// Design (markdown rendered)
	if issue.Design != "" {
		lines = append(lines, "")
		lines = append(lines, ui.DetailSection.Render("DESIGN"))
		lines = append(lines, d.renderMarkdown(issue.Design))
	}

	// Dependencies
	eval := issue.EvaluateDependencies(d.IssueMap, bt)
	blocks := issue.BlocksIDs(d.AllIssues, bt)
	hasDeps := len(eval.Edges) > 0 || len(blocks) > 0
	if hasDeps {
		lines = append(lines, "")
		lines = append(lines, ui.DetailSection.Render("DEPENDENCIES"))

		for _, id := range eval.BlockingIDs {
			title := id
			if dep, ok := d.IssueMap[id]; ok {
				title = dep.Title
			}
			lines = append(lines, ui.DepBlocked.Render(
				fmt.Sprintf("  %s waiting on %s %s (%s)", ui.SymStalled, ui.DepArrow, id, truncate(title, 30)),
			))
		}

		for _, id := range eval.MissingIDs {
			lines = append(lines, ui.DepMissing.Render(
				fmt.Sprintf("  %s missing %s %s (not found)", ui.SymMissing, ui.DepArrow, id),
			))
		}

		for _, id := range eval.ResolvedIDs {
			title := id
			if dep, ok := d.IssueMap[id]; ok {
				title = dep.Title
			}
			lines = append(lines, ui.DepResolved.Render(
				fmt.Sprintf("  %s resolved %s %s (%s)", ui.SymResolved, ui.DepArrow, id, truncate(title, 30)),
			))
		}

		for _, edge := range eval.NonBlocking {
			title := edge.DependsOnID
			if dep, ok := d.IssueMap[edge.DependsOnID]; ok {
				title = dep.Title
			}
			sym, verb, style := depTypeDisplay(edge.Type)
			lines = append(lines, style.Render(
				fmt.Sprintf("  %s %s %s %s (%s)", sym, verb, ui.DepArrow, edge.DependsOnID, truncate(title, 25)),
			))
		}

		for _, id := range blocks {
			title := id
			if dep, ok := d.IssueMap[id]; ok {
				title = dep.Title
			}
			lines = append(lines, ui.DepBlocks.Render(
				fmt.Sprintf("  %s blocks %s %s (%s)", ui.SymRolling, ui.DepArrow, id, truncate(title, 30)),
			))
		}
	}

	// Gate status (when agent is awaiting-gate)
	if gateSection := d.renderGateStatus(); gateSection != "" {
		lines = append(lines, "")
		lines = append(lines, gateSection)
	}

	// Molecule DAG section
	if d.MoleculeDAG != nil && d.MoleculeIssueID == issue.ID {
		lines = append(lines, "")
		lines = append(lines, d.renderMolecule())
	}

	// Activity section (timestamps + agent info)
	lines = append(lines, "")
	lines = append(lines, d.renderActivity())

	// Comments section
	if len(d.Comments) > 0 && d.CommentsIssueID == issue.ID {
		lines = append(lines, "")
		lines = append(lines, d.renderComments())
	}

	return strings.Join(lines, "\n")
}

// renderMolecule renders the molecule DAG step chain.
func (d *Detail) renderMolecule() string {
	dag := d.MoleculeDAG
	if dag == nil {
		return ""
	}

	var lines []string

	// Section header with progress
	header := "MOLECULE"
	if d.MoleculeProgress != nil {
		p := d.MoleculeProgress
		header = fmt.Sprintf("MOLECULE: %s (%d/%d, %d%%)",
			truncate(dag.RootTitle, 20), p.DoneSteps, p.TotalSteps, p.Percent)
	}
	lines = append(lines, ui.DetailSection.Render(header))

	// Progress bar
	if d.MoleculeProgress != nil {
		p := d.MoleculeProgress
		barWidth := max(d.Width-16, 10)
		bar := moleculeProgressBar(p.DoneSteps, p.TotalSteps, barWidth)
		lines = append(lines, fmt.Sprintf("  %s", bar))
	}

	// Render steps organized by tier
	if len(dag.TierGroups) > 0 {
		for tierIdx, tier := range dag.TierGroups {
			if len(tier) == 0 {
				continue
			}
			// Tier label
			tierLabel := fmt.Sprintf("tier %d", tierIdx)
			lines = append(lines, ui.MolTierLabel.Render(fmt.Sprintf("  %s %s", ui.SymTierLine, tierLabel)))

			for _, nodeID := range tier {
				node, ok := dag.Nodes[nodeID]
				if !ok {
					continue
				}
				lines = append(lines, d.renderMoleculeStep(node))
			}
		}
	} else {
		// Fallback: render all nodes without tier grouping
		for _, node := range dag.Nodes {
			lines = append(lines, d.renderMoleculeStep(node))
		}
	}

	// Critical path hint
	if len(dag.CriticalPath) > 1 {
		pathStr := truncate(strings.Join(dag.CriticalPath, " > "), d.Width-18)
		lines = append(lines, ui.MolTierLabel.Render(fmt.Sprintf("  critical: %s", pathStr)))
	}

	return strings.Join(lines, "\n")
}

// renderMoleculeStep renders a single molecule step with status symbol.
func (d *Detail) renderMoleculeStep(node *gastown.DAGNode) string {
	var sym string
	var style lipgloss.Style

	switch node.Status {
	case "done", "closed":
		sym = ui.SymStepDone
		style = ui.MolStepDone
	case "in_progress":
		sym = ui.SymStepActive
		style = ui.MolStepActive
	case "ready":
		sym = ui.SymStepReady
		style = ui.MolStepReady
	case "blocked":
		sym = ui.SymStepBlocked
		style = ui.MolStepBlocked
	default:
		sym = ui.SymStepReady
		style = lipgloss.NewStyle().Foreground(ui.Muted)
	}

	title := truncate(node.Title, d.Width-18)
	statusLabel := node.Status

	// Show dependency info for blocked steps
	suffix := ""
	if node.Status == "blocked" && len(node.Dependencies) > 0 {
		suffix = fmt.Sprintf(" (needs %s)", truncate(strings.Join(node.Dependencies, ", "), 20))
	}
	if node.Parallel {
		suffix += " ||"
	}

	return style.Render(fmt.Sprintf("    %s %-12s %s%s", sym, statusLabel, title, suffix))
}

// renderActivity renders the activity timeline from issue timestamps.
func (d *Detail) renderActivity() string {
	issue := d.Issue
	if issue == nil {
		return ""
	}

	var lines []string
	lines = append(lines, ui.DetailSection.Render("ACTIVITY"))

	timeStyle := lipgloss.NewStyle().Foreground(ui.Muted)
	eventStyle := lipgloss.NewStyle().Foreground(ui.Light)

	// Created
	lines = append(lines, fmt.Sprintf("  %s  %s",
		timeStyle.Render(formatTime(issue.CreatedAt)),
		eventStyle.Render("Created")))

	// Due date
	if issue.DueAt != nil {
		dueLabel := "Due"
		if issue.IsOverdue() {
			dueLabel = ui.OverdueBadge.Render("Overdue")
		}
		lines = append(lines, fmt.Sprintf("  %s  %s",
			timeStyle.Render(issue.DueAt.Format("Jan 02 15:04")),
			dueLabel))
	}

	// Agent assignment
	if d.TownStatus != nil && issue.ID != "" {
		if a := d.TownStatus.AgentForIssue(issue.ID); a != nil {
			agentStyle := lipgloss.NewStyle().Foreground(ui.StatusAgent)
			lines = append(lines, fmt.Sprintf("  %s  %s",
				timeStyle.Render("  now"),
				agentStyle.Render(fmt.Sprintf("%s %s (%s) working", ui.SymAgent, a.Name, a.Role))))
		}
	}

	// Molecule progress
	if d.MoleculeProgress != nil && d.MoleculeIssueID == issue.ID {
		p := d.MoleculeProgress
		molStyle := lipgloss.NewStyle().Foreground(ui.BrightGold)
		label := fmt.Sprintf("Molecule %d%% (%d/%d steps)", p.Percent, p.DoneSteps, p.TotalSteps)
		if p.Complete {
			label = "Molecule complete"
			molStyle = lipgloss.NewStyle().Foreground(ui.BrightGreen)
		}
		lines = append(lines, fmt.Sprintf("  %s  %s",
			timeStyle.Render("  now"),
			molStyle.Render(label)))
	}

	// Updated (if different from created)
	if !issue.UpdatedAt.IsZero() && issue.UpdatedAt.After(issue.CreatedAt.Add(time.Minute)) {
		lines = append(lines, fmt.Sprintf("  %s  %s",
			timeStyle.Render(formatTime(issue.UpdatedAt)),
			eventStyle.Render("Updated")))
	}

	// Closed
	if issue.ClosedAt != nil {
		lines = append(lines, fmt.Sprintf("  %s  %s",
			timeStyle.Render(formatTime(*issue.ClosedAt)),
			ui.MolStepDone.Render("Closed")))
	}

	return strings.Join(lines, "\n")
}

// renderGateStatus renders the gate waiting section when an agent is awaiting-gate.
func (d *Detail) renderGateStatus() string {
	if d.Issue == nil || d.TownStatus == nil {
		return ""
	}
	a := d.TownStatus.AgentForIssue(d.Issue.ID)
	if a == nil || a.State != "awaiting-gate" {
		return ""
	}

	var lines []string
	lines = append(lines, ui.DetailSection.Render("GATE"))

	gateStyle := lipgloss.NewStyle().Foreground(ui.BrightGold)
	agentLabel := lipgloss.NewStyle().Foreground(ui.Muted).Render(fmt.Sprintf("[agent: %s]", a.Name))
	lines = append(lines, fmt.Sprintf("  %s %s",
		gateStyle.Render(ui.SymDeferred+" Waiting on gate"),
		agentLabel))

	return strings.Join(lines, "\n")
}

// renderComments renders the comments section.
func (d *Detail) renderComments() string {
	var lines []string
	lines = append(lines, ui.DetailSection.Render(fmt.Sprintf("COMMENTS (%d)", len(d.Comments))))

	timeStyle := lipgloss.NewStyle().Foreground(ui.Muted)
	authorStyle := lipgloss.NewStyle().Foreground(ui.Light).Bold(true)
	bodyStyle := lipgloss.NewStyle().Foreground(ui.Light)
	bodyWidth := max(d.Width-12, 20)

	for _, c := range d.Comments {
		// Author + time
		timeLabel := c.Time
		if timeLabel == "" {
			timeLabel = ""
		}
		header := fmt.Sprintf("  %s  %s",
			authorStyle.Render(truncate(c.Author, 20)),
			timeStyle.Render(timeLabel))
		lines = append(lines, header)

		// Body (wrapped)
		if c.Body != "" {
			wrapped := wordWrap(c.Body, bodyWidth)
			for _, bline := range strings.Split(wrapped, "\n") {
				lines = append(lines, "    "+bodyStyle.Render(bline))
			}
		}
		lines = append(lines, "") // blank line between comments
	}

	return strings.Join(lines, "\n")
}

// moleculeProgressBar renders a progress bar for molecule steps.
func moleculeProgressBar(done, total, width int) string {
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

// formatTime renders a time as a compact label.
func formatTime(t time.Time) string {
	if t.IsZero() {
		return "          "
	}
	return t.Format("Jan 02 15:04")
}

func (d *Detail) row(label, value string) string {
	return ui.DetailLabel.Render(label) + " " + value
}

func paradeLabel(issue *data.Issue, issueMap map[string]*data.Issue, blockingTypes map[string]bool) string {
	switch issue.Status {
	case data.StatusClosed:
		return "Past the Stand"
	case data.StatusInProgress:
		if issue.EvaluateDependencies(issueMap, blockingTypes).IsBlocked {
			return "Stalled"
		}
		return "Rolling"
	default:
		if issue.EvaluateDependencies(issueMap, blockingTypes).IsBlocked {
			return "Stalled"
		}
		return "Lined Up"
	}
}

func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return string(runes[:maxLen])
	}
	return string(runes[:maxLen-3]) + "..."
}

// depTypeDisplay returns a symbol, verb, and style for a non-blocking dependency type.
func depTypeDisplay(depType string) (symbol string, verb string, style lipgloss.Style) {
	switch depType {
	case "related":
		return ui.SymRelated, "related to", ui.DepRelated
	case "duplicates":
		return ui.SymDuplicates, "duplicates", ui.DepDuplicates
	case "supersedes":
		return ui.SymSupersedes, "supersedes", ui.DepSupersedes
	case "discovered-from":
		return ui.SymNonBlocking, "discovered from", ui.DepNonBlocking
	case "waits-for":
		return ui.SymStalled, "waits for", ui.DepBlocked
	case "parent-child":
		return ui.DepTree, "child of", ui.DepNonBlocking
	case "replies-to":
		return ui.SymNonBlocking, "replies to", ui.DepNonBlocking
	default:
		return ui.SymNonBlocking, depType, ui.DepNonBlocking
	}
}

func wordWrap(s string, width int) string {
	if width <= 0 {
		return s
	}
	words := strings.Fields(s)
	if len(words) == 0 {
		return ""
	}

	var lines []string
	current := words[0]
	for _, word := range words[1:] {
		if len(current)+1+len(word) > width {
			lines = append(lines, current)
			current = word
		} else {
			current += " " + word
		}
	}
	lines = append(lines, current)
	return strings.Join(lines, "\n")
}
