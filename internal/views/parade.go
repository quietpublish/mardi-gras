package views

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/matt-wright86/mardi-gras/internal/data"
	"github.com/matt-wright86/mardi-gras/internal/ui"
)

// paradeSection defines how each parade group renders.
type paradeSection struct {
	Title  string
	Symbol string
	Style  lipgloss.Style
	Status data.ParadeStatus
}

var sections = []paradeSection{
	{Title: "ROLLING", Symbol: ui.SymRolling, Style: ui.SectionRolling, Status: data.ParadeRolling},
	{Title: "LINED UP", Symbol: ui.SymLinedUp, Style: ui.SectionLinedUp, Status: data.ParadeLinedUp},
	{Title: "STALLED", Symbol: ui.SymStalled, Style: ui.SectionStalled, Status: data.ParadeStalled},
	{Title: "PAST THE STAND", Symbol: ui.SymPassed, Style: ui.SectionPassed, Status: data.ParadePastTheStand},
}

// ParadeItem is a renderable entry — either a section header or an issue.
type ParadeItem struct {
	IsHeader bool
	Section  paradeSection
	Issue    *data.Issue
}

// Parade is the grouped issue list view.
type Parade struct {
	Items         []ParadeItem
	Cursor        int
	ShowClosed    bool
	Width         int
	Height        int
	ScrollOffset  int
	AllIssues     []data.Issue
	Groups        map[data.ParadeStatus][]data.Issue
	issueMap      map[string]*data.Issue
	blockingTypes map[string]bool
	SelectedIssue *data.Issue
}

// NewParade creates a parade view from a set of issues.
func NewParade(issues []data.Issue, width, height int, blockingTypes map[string]bool) Parade {
	groups := data.GroupByParade(issues, blockingTypes)
	issueMap := data.BuildIssueMap(issues)
	p := Parade{
		ShowClosed:    false,
		Width:         width,
		Height:        height,
		AllIssues:     issues,
		Groups:        groups,
		issueMap:      issueMap,
		blockingTypes: blockingTypes,
	}
	p.rebuildItems()
	if len(p.Items) > 0 {
		// Move cursor to first non-header item
		for i, item := range p.Items {
			if !item.IsHeader {
				p.Cursor = i
				p.SelectedIssue = item.Issue
				break
			}
		}
	}
	return p
}

// rebuildItems flattens groups into the renderable item list.
func (p *Parade) rebuildItems() {
	p.Items = nil
	for _, sec := range sections {
		issues := p.Groups[sec.Status]
		if len(issues) == 0 {
			continue
		}

		// Closed section: show collapsed count or expanded list
		if sec.Status == data.ParadePastTheStand {
			p.Items = append(p.Items, ParadeItem{IsHeader: true, Section: sec})
			if p.ShowClosed {
				for i := range issues {
					p.Items = append(p.Items, ParadeItem{Issue: &issues[i]})
				}
			}
			continue
		}

		p.Items = append(p.Items, ParadeItem{IsHeader: true, Section: sec})
		for i := range issues {
			p.Items = append(p.Items, ParadeItem{Issue: &issues[i]})
		}
	}
}

// MoveUp moves the cursor up, skipping headers.
func (p *Parade) MoveUp() {
	for i := p.Cursor - 1; i >= 0; i-- {
		if !p.Items[i].IsHeader {
			p.Cursor = i
			p.SelectedIssue = p.Items[i].Issue
			p.ensureVisible()
			return
		}
	}
}

// MoveDown moves the cursor down, skipping headers.
func (p *Parade) MoveDown() {
	for i := p.Cursor + 1; i < len(p.Items); i++ {
		if !p.Items[i].IsHeader {
			p.Cursor = i
			p.SelectedIssue = p.Items[i].Issue
			p.ensureVisible()
			return
		}
	}
}

// ToggleClosed shows or hides closed issues.
func (p *Parade) ToggleClosed() {
	p.ShowClosed = !p.ShowClosed
	selectedID := ""
	if p.SelectedIssue != nil {
		selectedID = p.SelectedIssue.ID
	}
	p.rebuildItems()
	p.clampScroll()
	// Restore cursor to the same issue if possible
	for i, item := range p.Items {
		if !item.IsHeader && item.Issue.ID == selectedID {
			p.Cursor = i
			p.SelectedIssue = item.Issue
			p.ensureVisible()
			return
		}
	}
	// Fallback to first selectable item
	for i, item := range p.Items {
		if !item.IsHeader {
			p.Cursor = i
			p.SelectedIssue = item.Issue
			p.ensureVisible()
			return
		}
	}
	// No selectable items at all
	p.Cursor = 0
	p.ScrollOffset = 0
	p.SelectedIssue = nil
}

// clampScroll ensures ScrollOffset is within valid bounds for the current Items slice.
func (p *Parade) clampScroll() {
	maxOffset := len(p.Items) - p.Height
	if maxOffset < 0 {
		maxOffset = 0
	}
	if p.ScrollOffset > maxOffset {
		p.ScrollOffset = maxOffset
	}
	if p.ScrollOffset < 0 {
		p.ScrollOffset = 0
	}
}

// ensureVisible adjusts scroll offset so cursor is visible.
func (p *Parade) ensureVisible() {
	if p.Cursor < p.ScrollOffset {
		p.ScrollOffset = p.Cursor
	}
	if p.Cursor >= p.ScrollOffset+p.Height {
		p.ScrollOffset = p.Cursor - p.Height + 1
	}
	p.clampScroll()
}

// SetSize updates the available dimensions.
func (p *Parade) SetSize(width, height int) {
	p.Width = width
	p.Height = height
}

// View renders the parade list.
func (p *Parade) View() string {
	if len(p.Items) == 0 {
		content := "No issues found"
		return lipgloss.NewStyle().Width(p.Width).Height(p.Height).Render(content)
	}

	p.clampScroll()

	var lines []string

	end := p.ScrollOffset + p.Height
	if end > len(p.Items) {
		end = len(p.Items)
	}

	visible := p.Items[p.ScrollOffset:end]

	for idx, item := range visible {
		globalIdx := p.ScrollOffset + idx
		if item.IsHeader {
			lines = append(lines, p.renderSectionHeader(item.Section))
		} else {
			lines = append(lines, p.renderIssue(item.Issue, globalIdx == p.Cursor))
		}
	}

	content := strings.Join(lines, "\n")

	// Pad to fill height
	rendered := strings.Count(content, "\n") + 1
	for rendered < p.Height {
		content += "\n"
		rendered++
	}

	return lipgloss.NewStyle().Width(p.Width).Render(content)
}

func (p *Parade) renderSectionHeader(sec paradeSection) string {
	count := len(p.Groups[sec.Status])
	label := fmt.Sprintf("%s %s", sec.Title, sec.Symbol)

	if sec.Status == data.ParadePastTheStand {
		toggle := ui.Collapsed
		if p.ShowClosed {
			toggle = ui.Expanded
		}
		label = fmt.Sprintf("%s %s (%d issues)", toggle, sec.Title, count)
		if !p.ShowClosed {
			label += "  [press c to expand]"
		}
	}

	return sec.Style.Width(p.Width).Render(label)
}

func (p *Parade) renderIssue(issue *data.Issue, selected bool) string {
	sym := statusSymbol(issue, p.issueMap, p.blockingTypes)
	prio := data.PriorityLabel(issue.Priority)

	prioStyle := ui.BadgePriority.Foreground(ui.PriorityColor(int(issue.Priority)))
	symStyle := lipgloss.NewStyle().Foreground(statusColor(issue, p.issueMap, p.blockingTypes))

	// Build the "next blocker" hint for stalled issues
	hint := ""
	eval := issue.EvaluateDependencies(p.issueMap, p.blockingTypes)
	if eval.IsBlocked && eval.NextBlockerID != "" {
		hintStyle := lipgloss.NewStyle().Foreground(ui.Muted)
		if target, ok := p.issueMap[eval.NextBlockerID]; ok {
			hint = hintStyle.Render(fmt.Sprintf(" %s %s %s", ui.SymNextArrow, eval.NextBlockerID, truncate(target.Title, 20)))
		} else {
			hint = hintStyle.Render(fmt.Sprintf(" %s missing %s", ui.SymNextArrow, eval.NextBlockerID))
		}
	}

	// Truncate title to fit, accounting for hint
	hintLen := lipgloss.Width(hint)
	maxTitle := p.Width - 18 - hintLen
	title := issue.Title
	if len(title) > maxTitle && maxTitle > 3 {
		title = title[:maxTitle-3] + "..."
	}

	line := fmt.Sprintf("%s %s %s %s",
		symStyle.Render(sym),
		issue.ID,
		title,
		prioStyle.Render(prio),
	)
	line += hint

	if selected {
		cursor := ui.ItemCursor.Render(ui.Cursor + " ")
		row := cursor + line
		// Full-width highlight for selected row
		return ui.ItemSelectedBg.Width(p.Width).Render(row)
	}

	return ui.ItemNormal.Render(line)
}

func statusSymbol(issue *data.Issue, issueMap map[string]*data.Issue, blockingTypes map[string]bool) string {
	switch issue.Status {
	case data.StatusClosed:
		return ui.SymPassed
	case data.StatusInProgress:
		if issue.EvaluateDependencies(issueMap, blockingTypes).IsBlocked {
			return ui.SymStalled
		}
		return ui.SymRolling
	default:
		if issue.EvaluateDependencies(issueMap, blockingTypes).IsBlocked {
			return ui.SymStalled
		}
		return ui.SymLinedUp
	}
}

func statusColor(issue *data.Issue, issueMap map[string]*data.Issue, blockingTypes map[string]bool) lipgloss.Color {
	switch issue.Status {
	case data.StatusClosed:
		return ui.StatusPassed
	case data.StatusInProgress:
		if issue.EvaluateDependencies(issueMap, blockingTypes).IsBlocked {
			return ui.StatusStalled
		}
		return ui.StatusRolling
	default:
		if issue.EvaluateDependencies(issueMap, blockingTypes).IsBlocked {
			return ui.StatusStalled
		}
		return ui.StatusLinedUp
	}
}
