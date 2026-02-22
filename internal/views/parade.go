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
	Color  lipgloss.Color
	Status data.ParadeStatus
}

var sections = []paradeSection{
	{Title: "Rolling", Symbol: ui.SymRolling, Style: ui.SectionRolling, Color: ui.StatusRolling, Status: data.ParadeRolling},
	{Title: "Lined Up", Symbol: ui.SymLinedUp, Style: ui.SectionLinedUp, Color: ui.StatusLinedUp, Status: data.ParadeLinedUp},
	{Title: "Stalled", Symbol: ui.SymStalled, Style: ui.SectionStalled, Color: ui.StatusStalled, Status: data.ParadeStalled},
	{Title: "Past the Stand", Symbol: ui.SymPassed, Style: ui.SectionPassed, Color: ui.StatusPassed, Status: data.ParadePastTheStand},
}

// ParadeItem is a renderable entry — a section header, footer, or issue.
type ParadeItem struct {
	IsHeader bool
	IsFooter bool
	Section  paradeSection
	Issue    *data.Issue
}

// isSelectable returns true if this item can receive the cursor.
func (item ParadeItem) isSelectable() bool {
	return !item.IsHeader && !item.IsFooter
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
		// Move cursor to first selectable item
		for i, item := range p.Items {
			if item.isSelectable() {
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

		// Header (top border)
		p.Items = append(p.Items, ParadeItem{IsHeader: true, Section: sec})

		// Closed section: show collapsed count or expanded list
		if sec.Status == data.ParadePastTheStand {
			if p.ShowClosed {
				for i := range issues {
					p.Items = append(p.Items, ParadeItem{Issue: &issues[i], Section: sec})
				}
			}
		} else {
			for i := range issues {
				p.Items = append(p.Items, ParadeItem{Issue: &issues[i], Section: sec})
			}
		}

		// Footer (bottom border)
		p.Items = append(p.Items, ParadeItem{IsFooter: true, Section: sec})
	}
}

// MoveUp moves the cursor up, skipping headers and footers.
func (p *Parade) MoveUp() {
	for i := p.Cursor - 1; i >= 0; i-- {
		if p.Items[i].isSelectable() {
			p.Cursor = i
			p.SelectedIssue = p.Items[i].Issue
			p.ensureVisible()
			return
		}
	}
}

// MoveDown moves the cursor down, skipping headers and footers.
func (p *Parade) MoveDown() {
	for i := p.Cursor + 1; i < len(p.Items); i++ {
		if p.Items[i].isSelectable() {
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
		if item.isSelectable() && item.Issue.ID == selectedID {
			p.Cursor = i
			p.SelectedIssue = item.Issue
			p.ensureVisible()
			return
		}
	}
	// Fallback to first selectable item
	for i, item := range p.Items {
		if item.isSelectable() {
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
		switch {
		case item.IsHeader:
			lines = append(lines, p.renderBorderTop(item.Section))
		case item.IsFooter:
			lines = append(lines, p.renderBorderBottom(item.Section))
		default:
			lines = append(lines, p.renderIssue(item, globalIdx == p.Cursor))
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

// renderBorderTop builds a top border line: ╭─ ● Rolling (2) ────────╮
func (p *Parade) renderBorderTop(sec paradeSection) string {
	count := len(p.Groups[sec.Status])
	borderStyle := lipgloss.NewStyle().Foreground(sec.Color)

	// Build the title content
	var titleText string
	if sec.Status == data.ParadePastTheStand {
		toggle := ui.Collapsed
		if p.ShowClosed {
			toggle = ui.Expanded
		}
		titleText = fmt.Sprintf("%s %s %s (%d)", toggle, sec.Symbol, sec.Title, count)
		if !p.ShowClosed {
			titleText += " press c"
		}
	} else {
		titleText = fmt.Sprintf("%s %s (%d)", sec.Symbol, sec.Title, count)
	}

	coloredTitle := sec.Style.Render(titleText)
	titleWidth := lipgloss.Width(coloredTitle)

	// ╭─ <title> ─────────────╮
	prefix := borderStyle.Render(ui.BoxTopLeft + ui.BoxHorizontal + " ")
	suffix := borderStyle.Render(" " + ui.BoxTopRight)

	// Fill remaining width with ─ (account for space after title)
	prefixW := lipgloss.Width(prefix)
	suffixW := lipgloss.Width(suffix)
	fillLen := p.Width - prefixW - titleWidth - 1 - suffixW // -1 for space before fill
	if fillLen < 1 {
		fillLen = 1
	}
	fill := borderStyle.Render(" " + strings.Repeat(ui.BoxHorizontal, fillLen))

	return prefix + coloredTitle + fill + suffix
}

// renderBorderBottom builds a bottom border line: ╰────────────────────╯
func (p *Parade) renderBorderBottom(sec paradeSection) string {
	borderStyle := lipgloss.NewStyle().Foreground(sec.Color)

	// ╰─...─╯
	cornerL := borderStyle.Render(ui.BoxBottomLeft)
	cornerR := borderStyle.Render(ui.BoxBottomRight)
	cornersW := lipgloss.Width(cornerL) + lipgloss.Width(cornerR)

	fillLen := p.Width - cornersW
	if fillLen < 1 {
		fillLen = 1
	}
	fill := borderStyle.Render(strings.Repeat(ui.BoxHorizontal, fillLen))

	return cornerL + fill + cornerR
}

// renderIssue renders an issue row wrapped in │ section borders.
func (p *Parade) renderIssue(item ParadeItem, selected bool) string {
	issue := item.Issue
	sec := item.Section
	borderStyle := lipgloss.NewStyle().Foreground(sec.Color)

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

	// Inner width (between │ borders, with 1 char padding each side)
	innerWidth := p.Width - 4 // │ + space + content + space + │

	// Truncate title to fit
	hintLen := lipgloss.Width(hint)
	maxTitle := innerWidth - 16 - hintLen
	if maxTitle < 0 {
		maxTitle = 0
	}
	title := truncate(issue.Title, maxTitle)

	line := fmt.Sprintf("%s %s %s %s",
		symStyle.Render(sym),
		issue.ID,
		title,
		prioStyle.Render(prio),
	)
	line += hint

	leftBorder := borderStyle.Render(ui.BoxVertical)
	rightBorder := borderStyle.Render(ui.BoxVertical)

	if selected {
		cursor := ui.ItemCursor.Render(ui.Cursor + " ")
		row := cursor + line
		// Pad content to fill inner width, then highlight
		content := ui.ItemSelectedBg.Width(innerWidth).MaxWidth(innerWidth).Render(row)
		return leftBorder + " " + content + " " + rightBorder
	}

	// Non-selected: pad with leading space for alignment (matching cursor indent)
	row := "  " + line
	content := lipgloss.NewStyle().Width(innerWidth).MaxWidth(innerWidth).Render(row)
	return leftBorder + " " + content + " " + rightBorder
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
