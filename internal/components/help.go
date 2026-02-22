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
			},
		},
		{
			title: "PARADE",
			bindings: []helpBinding{
				{key: "j / k", desc: "Navigate up/down"},
				{key: "g / G", desc: "Jump to top/bottom"},
				{key: "enter", desc: "Focus detail pane"},
				{key: "c", desc: "Toggle closed issues"},
				{key: "/", desc: "Enter filter mode"},
			},
		},
		{
			title: "DETAIL",
			bindings: []helpBinding{
				{key: "j / k", desc: "Scroll up/down"},
				{key: "esc", desc: "Back to parade pane"},
				{key: "/", desc: "Enter filter mode"},
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
