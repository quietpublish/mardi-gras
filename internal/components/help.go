package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/matt-wright86/mardi-gras/internal/ui"
)

// Help renders the global ? help modal.
type Help struct {
	Width  int
	Height int
}

type helpBinding struct {
	key  string
	desc string
}

type helpSection struct {
	title    string
	bindings []helpBinding
}

// NewHelp creates a new help rendering component.
func NewHelp(width, height int) Help {
	return Help{Width: width, Height: height}
}

// View returns the rendered modal block positioned at the center of the terminal.
func (h Help) View() string {
	contentWidth := h.Width - 8
	if contentWidth > 84 {
		contentWidth = 84
	}
	if contentWidth < 44 {
		contentWidth = 44
	}

	sections := []helpSection{
		{
			title: "GLOBAL",
			bindings: []helpBinding{
				{key: "q", desc: "Quit application"},
				{key: "tab", desc: "Switch active pane"},
				{key: "?", desc: "Toggle help"},
				{key: ": / Ctrl+K", desc: "Open command palette"},
			},
		},
		{
			title: "PARADE",
			bindings: []helpBinding{
				{key: "j / k", desc: "Navigate up/down"},
				{key: "g / G", desc: "Jump to top/bottom"},
				{key: "enter", desc: "Focus detail pane"},
				{key: "c", desc: "Toggle closed issues"},
				{key: "/", desc: "Enter filter mode (fuzzy)"},
				{key: "f", desc: "Toggle focus mode (my work + top priority)"},
				{key: "a", desc: "Launch agent (tmux: new window)"},
				{key: "A", desc: "Kill active agent on issue"},
			},
		},
		{
			title: "QUICK ACTIONS",
			bindings: []helpBinding{
				{key: "1", desc: "Set status: in_progress"},
				{key: "2", desc: "Set status: open"},
				{key: "3", desc: "Close issue"},
				{key: "!", desc: "Set priority: P1 (high)"},
				{key: "@", desc: "Set priority: P2 (medium)"},
				{key: "#", desc: "Set priority: P3 (low)"},
				{key: "$", desc: "Set priority: P4 (backlog)"},
				{key: "b", desc: "Copy branch name to clipboard"},
				{key: "B", desc: "Create + checkout git branch"},
				{key: "N", desc: "Create new issue"},
			},
		},
		{
			title: "MULTI-SELECT",
			bindings: []helpBinding{
				{key: "space / x", desc: "Toggle select on cursor issue"},
				{key: "Shift+J/K", desc: "Select and move down/up"},
				{key: "X", desc: "Clear all selections"},
				{key: "1/2/3", desc: "Bulk set status on selected"},
				{key: "a", desc: "Sling all selected issues"},
				{key: "s", desc: "Pick formula and sling all selected"},
			},
		},
		{
			title: "DETAIL",
			bindings: []helpBinding{
				{key: "j / k", desc: "Scroll up/down"},
				{key: "esc", desc: "Back to parade pane"},
				{key: "/", desc: "Enter filter mode"},
				{key: "a", desc: "Launch agent (tmux: new window)"},
				{key: "A", desc: "Kill active agent on issue"},
			},
		},
		{
			title: "FILTER",
			bindings: []helpBinding{
				{key: "esc", desc: "Clear query and exit"},
				{key: "enter", desc: "Apply query and exit"},
				{key: "type:bug", desc: "Match issue type"},
				{key: "p0, p1...", desc: "Match priority level"},
			},
		},
		{
			title: "GAS TOWN (when gt detected)",
			bindings: []helpBinding{
				{key: "ctrl+g", desc: "Toggle Gas Town panel"},
				{key: "a", desc: "Sling issue to polecat (or tmux fallback)"},
				{key: "s", desc: "Pick formula and sling to polecat"},
				{key: "n", desc: "Nudge agent with message"},
				{key: "A", desc: "Unsling/kill agent"},
			},
		},
		{
			title: "GAS TOWN PANEL (ctrl+g)",
			bindings: []helpBinding{
				{key: "j / k", desc: "Navigate agents"},
				{key: "g / G", desc: "Jump to first/last agent"},
				{key: "n", desc: "Nudge selected agent"},
				{key: "h", desc: "Handoff work from agent"},
				{key: "K", desc: "Decommission polecat"},
			},
		},
	}

	header := lipgloss.JoinVertical(
		lipgloss.Left,
		ui.HelpTitle.Width(contentWidth).Render("[ MARDI GRAS HELP ]"),
		ui.HelpSubtitle.Width(contentWidth).Render("Navigation and filter shortcuts"),
	)

	body := h.renderSections(contentWidth, sections)
	footer := ui.HelpHint.Width(contentWidth).Render("Press esc, q, or ? to close")

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		"",
		body,
		"",
		footer,
	)

	box := ui.HelpOverlayBg.Width(contentWidth + 4).Render(content)

	return lipgloss.Place(h.Width, h.Height, lipgloss.Center, lipgloss.Center, box)
}

func (h Help) renderSections(width int, sections []helpSection) string {
	blocks := make([]string, 0, len(sections))
	for i := range sections {
		blocks = append(blocks, h.renderSection(width, sections[i], h.maxKeyWidth(sections)))
	}
	return strings.Join(blocks, "\n\n")
}

func (h Help) renderSection(width int, section helpSection, keyWidth int) string {
	rows := make([]string, 0, len(section.bindings))
	descWidth := width - keyWidth - 3
	if descWidth < 16 {
		descWidth = 16
	}

	for i := range section.bindings {
		b := section.bindings[i]
		key := ui.HelpKey.Width(keyWidth).Render(b.key)
		desc := ansi.Truncate(b.desc, descWidth, "...")
		row := lipgloss.JoinHorizontal(
			lipgloss.Top,
			key,
			" ",
			ui.HelpDesc.Width(descWidth).Render(desc),
		)
		rows = append(rows, row)
	}

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		ui.HelpSection.Render(section.title),
		strings.Join(rows, "\n"),
	)
	return content
}

func (h Help) maxKeyWidth(sections []helpSection) int {
	keyWidth := 10
	for i := range sections {
		for j := range sections[i].bindings {
			l := len(sections[i].bindings[j].key)
			if l > keyWidth {
				keyWidth = l
			}
		}
	}
	if keyWidth > 12 {
		return 12
	}
	return keyWidth
}
