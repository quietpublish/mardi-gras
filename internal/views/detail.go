package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
	"github.com/matt-wright86/mardi-gras/internal/data"
	"github.com/matt-wright86/mardi-gras/internal/ui"
)

// Detail renders the right-panel issue details with a scrollable viewport.
type Detail struct {
	Issue         *data.Issue
	AllIssues     []data.Issue
	IssueMap      map[string]*data.Issue
	BlockingTypes map[string]bool
	Viewport      viewport.Model
	Width         int
	Height        int
	Focused       bool
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
	d.Viewport.SetContent(d.renderContent())
	d.Viewport.GotoTop()
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

	// ID
	lines = append(lines, d.row("ID:", ui.DetailValue.Render(issue.ID)))

	// Description
	if issue.Description != "" {
		lines = append(lines, "")
		lines = append(lines, ui.DetailSection.Render("DESCRIPTION"))
		// Word-wrap description to panel width
		wrapped := wordWrap(issue.Description, d.Width-4)
		lines = append(lines, ui.DetailValue.Render(wrapped))
	}

	// Close reason
	if issue.CloseReason != "" {
		lines = append(lines, "")
		lines = append(lines, ui.DetailSection.Render("CLOSE REASON"))
		wrapped := wordWrap(issue.CloseReason, d.Width-4)
		lines = append(lines, ui.DetailValue.Render(wrapped))
	}

	// Notes
	if issue.Notes != "" {
		lines = append(lines, "")
		lines = append(lines, ui.DetailSection.Render("NOTES"))
		wrapped := wordWrap(issue.Notes, d.Width-4)
		lines = append(lines, ui.DetailValue.Render(wrapped))
	}

	// Dependencies — use EvaluateDependencies for full classification
	eval := issue.EvaluateDependencies(d.IssueMap, bt)
	blocks := issue.BlocksIDs(d.AllIssues, bt)
	hasDeps := len(eval.Edges) > 0 || len(blocks) > 0
	if hasDeps {
		lines = append(lines, "")
		lines = append(lines, ui.DetailSection.Render("DEPENDENCIES"))

		// Blocking (unresolved)
		for _, id := range eval.BlockingIDs {
			title := id
			if dep, ok := d.IssueMap[id]; ok {
				title = dep.Title
			}
			lines = append(lines, ui.DepBlocked.Render(
				fmt.Sprintf("  %s waiting on %s %s (%s)", ui.SymStalled, ui.DepArrow, id, truncate(title, 30)),
			))
		}

		// Missing (dangling ref)
		for _, id := range eval.MissingIDs {
			lines = append(lines, ui.DepMissing.Render(
				fmt.Sprintf("  %s missing %s %s (not found)", ui.SymMissing, ui.DepArrow, id),
			))
		}

		// Resolved (closed blockers) — dimmed
		for _, id := range eval.ResolvedIDs {
			title := id
			if dep, ok := d.IssueMap[id]; ok {
				title = dep.Title
			}
			lines = append(lines, ui.DepResolved.Render(
				fmt.Sprintf("  %s resolved %s %s (%s)", ui.SymResolved, ui.DepArrow, id, truncate(title, 30)),
			))
		}

		// Non-blocking (other dep types)
		for _, edge := range eval.NonBlocking {
			title := edge.DependsOnID
			if dep, ok := d.IssueMap[edge.DependsOnID]; ok {
				title = dep.Title
			}
			lines = append(lines, ui.DepNonBlocking.Render(
				fmt.Sprintf("  %s related %s %s (%s) (type: %s)", ui.SymNonBlocking, ui.DepArrow, edge.DependsOnID, truncate(title, 25), edge.Type),
			))
		}

		// Reverse: what this issue blocks
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

	return strings.Join(lines, "\n")
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

func truncate(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	if max <= 3 {
		return string(runes[:max])
	}
	return string(runes[:max-3]) + "..."
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
