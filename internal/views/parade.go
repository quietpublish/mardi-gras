package views

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/matt-wright86/mardi-gras/internal/data"
	"github.com/matt-wright86/mardi-gras/internal/gastown"
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
	ActiveAgents  map[string]string // issueID -> tmux window name
	TownStatus    *gastown.TownStatus
	ChangedIDs    map[string]bool // recently changed issues (change indicator dot)
	Selected      map[string]bool // multi-selected issue IDs
	HasGasTown    bool            // Gas Town available (shows sling/nudge hints)
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

// ToggleSelect toggles multi-select on the issue at the cursor.
func (p *Parade) ToggleSelect() {
	if p.Cursor < 0 || p.Cursor >= len(p.Items) {
		return
	}
	item := p.Items[p.Cursor]
	if !item.isSelectable() || item.Issue == nil {
		return
	}
	if p.Selected == nil {
		p.Selected = make(map[string]bool)
	}
	id := item.Issue.ID
	if p.Selected[id] {
		delete(p.Selected, id)
	} else {
		p.Selected[id] = true
	}
}

// ClearSelection removes all multi-selections.
func (p *Parade) ClearSelection() {
	p.Selected = nil
}

// SelectedIssues returns the list of multi-selected issues.
func (p *Parade) SelectedIssues() []*data.Issue {
	if len(p.Selected) == 0 {
		return nil
	}
	var result []*data.Issue
	for _, item := range p.Items {
		if item.Issue != nil && p.Selected[item.Issue.ID] {
			result = append(result, item.Issue)
		}
	}
	return result
}

// SelectionCount returns the number of multi-selected issues.
func (p *Parade) SelectionCount() int {
	return len(p.Selected)
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

// sectionHints returns the keyboard shortcut hints for a parade section.
func (p *Parade) sectionHints(sec paradeSection) string {
	type hint struct {
		key  string
		desc string
	}
	var hints []hint

	switch sec.Status {
	case data.ParadeRolling:
		if p.HasGasTown {
			hints = append(hints, hint{"n", "nudge"}, hint{"A", "unsling"})
		} else {
			hints = append(hints, hint{"a", "agent"}, hint{"3", "close"})
		}
	case data.ParadeLinedUp:
		if p.HasGasTown {
			hints = append(hints, hint{"a", "sling"}, hint{"s", "formula"})
		} else {
			hints = append(hints, hint{"1", "start"}, hint{"a", "agent"})
		}
	case data.ParadeStalled:
		hints = append(hints, hint{"enter", "details"})
	case data.ParadePastTheStand:
		// "press c" hint handled inline in title text
	}

	if len(hints) == 0 {
		return ""
	}

	var parts []string
	for _, h := range hints {
		parts = append(parts, ui.SectionHintKey.Render(h.key)+ui.SectionHintDesc.Render("·"+h.desc))
	}
	return strings.Join(parts, " ")
}

// renderBorderTop builds a top border line: ╭─ ● Rolling (2) ──── a·sling ─╮
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

	// Build shortcut hints for this section
	hintStr := p.sectionHints(sec)
	hintWidth := lipgloss.Width(hintStr)

	// ╭─ <title> ─────────────╮
	prefix := borderStyle.Render(ui.BoxTopLeft + ui.BoxHorizontal + " ")
	suffix := borderStyle.Render(" " + ui.BoxTopRight)

	prefixW := lipgloss.Width(prefix)
	suffixW := lipgloss.Width(suffix)

	// Truncate title text if it exceeds available space
	availableForTitle := p.Width - prefixW - suffixW - 1 // -1 for space after title
	if titleWidth > availableForTitle && availableForTitle > 0 {
		titleText = truncate(titleText, availableForTitle)
		coloredTitle = sec.Style.Render(titleText)
		titleWidth = lipgloss.Width(coloredTitle)
	}

	// Calculate fill, reserving space for hints if they fit
	totalUsed := prefixW + titleWidth + 1 + suffixW // +1 for space after title
	remaining := p.Width - totalUsed

	if hintWidth > 0 && remaining >= hintWidth+4 {
		// Enough room: ── <fill> ── hint ──
		leftFill := remaining - hintWidth - 3 // 3 = spaces around hint + trailing dash
		if leftFill < 1 {
			leftFill = 1
		}
		fill := borderStyle.Render(" " + strings.Repeat(ui.BoxHorizontal, leftFill) + " ") +
			hintStr +
			borderStyle.Render(" " + ui.BoxHorizontal)
		return prefix + coloredTitle + fill + suffix
	}

	// No room for hints — plain fill
	fillLen := remaining
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

	// Multi-select checkbox
	selectPrefix := ""
	selectWidth := 0
	if len(p.Selected) > 0 {
		if p.Selected[issue.ID] {
			selectPrefix = lipgloss.NewStyle().Foreground(ui.BrightGold).Bold(true).Render(ui.SymSelected) + " "
		} else {
			selectPrefix = lipgloss.NewStyle().Foreground(ui.Dim).Render(ui.SymUnselected) + " "
		}
		selectWidth = 2
	}

	// Change indicator dot
	changePrefix := ""
	changeWidth := 0
	if p.ChangedIDs != nil && p.ChangedIDs[issue.ID] {
		changePrefix = lipgloss.NewStyle().Foreground(ui.BrightGold).Render(ui.SymChanged) + " "
		changeWidth = 2
	}

	// Agent badge prefix
	agentPrefix := ""
	agentWidth := 0
	if p.ActiveAgents != nil {
		if _, active := p.ActiveAgents[issue.ID]; active {
			if p.TownStatus != nil {
				// Gas Town: show named agent
				if a := p.TownStatus.AgentForIssue(issue.ID); a != nil {
					label := fmt.Sprintf("%s %s", ui.SymAgent, a.Name)
					agentPrefix = ui.AgentBadge.Render(label) + " "
					agentWidth = len(a.Name) + 2 // symbol + space + name + space
				} else {
					agentPrefix = ui.AgentBadge.Render(ui.SymAgent) + " "
					agentWidth = 2
				}
			} else {
				// Tmux-only: generic badge
				agentPrefix = ui.AgentBadge.Render(ui.SymAgent) + " "
				agentWidth = 2
			}
		}
	}

	// Hierarchical indent based on dot-separated ID depth
	depth := issue.NestingDepth()
	indent := strings.Repeat("  ", depth)
	indentWidth := depth * 2

	// Due date badge
	dueBadge := ""
	dueWidth := 0
	if issue.IsOverdue() {
		label := fmt.Sprintf("%s %s", ui.SymOverdue, issue.DueLabel())
		dueBadge = " " + ui.OverdueBadge.Render(label)
		dueWidth = lipgloss.Width(dueBadge)
	} else if issue.DueAt != nil && issue.Status != data.StatusClosed {
		days := int(issue.DueAt.Sub(time.Now()).Hours() / 24)
		if days <= 3 {
			label := fmt.Sprintf("%s %s", ui.SymDueDate, issue.DueLabel())
			dueBadge = " " + ui.DueSoonBadge.Render(label)
			dueWidth = lipgloss.Width(dueBadge)
		}
	}

	// Deferred badge
	deferBadge := ""
	deferWidth := 0
	if issue.IsDeferred() {
		deferBadge = " " + ui.DeferredStyle.Render(ui.SymDeferred)
		deferWidth = 2
	}

	// Build the "next blocker" hint for stalled issues
	var rawHint string
	hintStyle := lipgloss.NewStyle().Foreground(ui.Muted)
	eval := issue.EvaluateDependencies(p.issueMap, p.blockingTypes)
	if eval.IsBlocked && eval.NextBlockerID != "" {
		if target, ok := p.issueMap[eval.NextBlockerID]; ok {
			rawHint = fmt.Sprintf(" %s %s %s", ui.SymNextArrow, eval.NextBlockerID, target.Title)
		} else {
			rawHint = fmt.Sprintf(" %s missing %s", ui.SymNextArrow, eval.NextBlockerID)
		}
	}

	// Inner width (between │ borders, with 1 char padding each side)
	innerWidth := p.Width - 4 // │ + space + content + space + │

	// First, constrain the hint length if the terminal is very narrow
	maxHint := innerWidth - 16 - agentWidth - indentWidth - dueWidth - deferWidth
	if maxHint < 0 {
		maxHint = 0
	}

	if lipgloss.Width(rawHint) > maxHint && maxHint > 0 {
		rawHint = truncate(rawHint, maxHint)
	} else if maxHint == 0 {
		rawHint = ""
	}

	hint := ""
	if rawHint != "" {
		hint = hintStyle.Render(rawHint)
	}

	hintLen := lipgloss.Width(hint)
	maxTitle := innerWidth - 16 - hintLen - agentWidth - changeWidth - selectWidth - indentWidth - dueWidth - deferWidth
	if maxTitle < 0 {
		maxTitle = 0
	}
	title := truncate(issue.Title, maxTitle)

	// Apply dim styling to deferred issue titles
	titleStyle := lipgloss.NewStyle()
	if issue.IsDeferred() {
		titleStyle = ui.DeferredStyle
	}

	line := fmt.Sprintf("%s%s %s%s%s%s %s %s",
		indent,
		symStyle.Render(sym),
		selectPrefix,
		changePrefix,
		agentPrefix,
		issue.ID,
		titleStyle.Render(title),
		prioStyle.Render(prio),
	)
	line += dueBadge + deferBadge + hint

	leftBorder := borderStyle.Render(ui.BoxVertical)
	rightBorder := borderStyle.Render(ui.BoxVertical)

	if selected {
		cursor := ui.ItemCursor.Render(ui.Cursor + " ")
		row := cursor + line
		// Pad to fill inner width, then apply highlight
		rowWidth := lipgloss.Width(row)
		if padLen := innerWidth - rowWidth; padLen > 0 {
			row += strings.Repeat(" ", padLen)
		}
		content := ui.ItemSelectedBg.Render(ansi.Truncate(row, innerWidth, ""))
		return leftBorder + " " + content + " " + rightBorder
	}

	// Non-selected: pad with leading space for alignment (matching cursor indent)
	row := "  " + line
	if padLen := innerWidth - lipgloss.Width(row); padLen > 0 {
		row += strings.Repeat(" ", padLen)
	}
	content := ansi.Truncate(row, innerWidth, "")
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
