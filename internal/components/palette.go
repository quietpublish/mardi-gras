package components

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/matt-wright86/mardi-gras/internal/ui"
	"github.com/sahilm/fuzzy"
)

// PaletteAction identifies an executable command from the palette.
type PaletteAction int

const (
	ActionSetInProgress PaletteAction = iota
	ActionSetOpen
	ActionCloseIssue
	ActionSetPriorityHigh
	ActionSetPriorityMedium
	ActionSetPriorityLow
	ActionSetPriorityBacklog
	ActionCopyBranch
	ActionCreateBranch
	ActionNewIssue
	ActionToggleFocus
	ActionToggleClosed
	ActionFilter
	ActionLaunchAgent
	ActionKillAgent
	ActionSlingFormula
	ActionNudgeAgent
	ActionHelp
	ActionQuit
)

// PaletteCommand is a single entry in the command palette.
type PaletteCommand struct {
	Name   string
	Desc   string
	Key    string
	Action PaletteAction
}

// PaletteResult is the message sent when the palette closes.
type PaletteResult struct {
	Action    PaletteAction
	Cancelled bool
}

// commandSource implements fuzzy.Source for palette commands.
type commandSource struct {
	commands []PaletteCommand
}

func (s commandSource) String(i int) string {
	return s.commands[i].Name + " " + s.commands[i].Desc
}

func (s commandSource) Len() int {
	return len(s.commands)
}

// Palette is the fuzzy command picker overlay.
type Palette struct {
	input        textinput.Model
	commands     []PaletteCommand
	filtered     []PaletteCommand
	cursor       int
	scrollOffset int
	width        int
	height       int
}

const paletteMaxVisible = 12

// NewPalette creates a new command palette.
func NewPalette(width, height int, commands []PaletteCommand) Palette {
	ti := textinput.New()
	ti.Prompt = ui.InputPrompt.Render(ui.FleurDeLis + " ")
	ti.Placeholder = "Type a command..."
	ti.TextStyle = ui.InputText
	ti.Cursor.Style = ui.InputCursor
	ti.Focus()

	contentWidth := width - 8
	if contentWidth > 60 {
		contentWidth = 60
	}
	ti.Width = contentWidth - 6

	return Palette{
		input:    ti,
		commands: commands,
		filtered: commands,
		width:    width,
		height:   height,
	}
}

// Init returns the blink command for the text input cursor.
func (p Palette) Init() tea.Cmd {
	return textinput.Blink
}

// Update handles messages for the palette.
func (p Palette) Update(msg tea.Msg) (Palette, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return p, func() tea.Msg {
				return PaletteResult{Cancelled: true}
			}
		case "enter":
			if len(p.filtered) > 0 {
				action := p.filtered[p.cursor].Action
				return p, func() tea.Msg {
					return PaletteResult{Action: action}
				}
			}
			return p, func() tea.Msg {
				return PaletteResult{Cancelled: true}
			}
		case "up", "ctrl+p":
			p.moveUp()
			return p, nil
		case "down", "ctrl+n":
			p.moveDown()
			return p, nil
		}
	}

	// Forward to text input
	oldVal := p.input.Value()
	var cmd tea.Cmd
	p.input, cmd = p.input.Update(msg)
	if p.input.Value() != oldVal {
		p.refilter()
	}
	return p, cmd
}

func (p *Palette) moveUp() {
	if p.cursor > 0 {
		p.cursor--
	}
	if p.cursor < p.scrollOffset {
		p.scrollOffset = p.cursor
	}
}

func (p *Palette) moveDown() {
	if p.cursor < len(p.filtered)-1 {
		p.cursor++
	}
	if p.cursor >= p.scrollOffset+paletteMaxVisible {
		p.scrollOffset = p.cursor - paletteMaxVisible + 1
	}
}

func (p *Palette) refilter() {
	query := strings.TrimSpace(p.input.Value())
	if query == "" {
		p.filtered = p.commands
		p.cursor = 0
		p.scrollOffset = 0
		return
	}

	src := commandSource{commands: p.commands}
	matches := fuzzy.FindFrom(query, src)

	result := make([]PaletteCommand, 0, len(matches))
	for _, match := range matches {
		result = append(result, p.commands[match.Index])
	}
	p.filtered = result
	p.cursor = 0
	p.scrollOffset = 0
}

// View renders the command palette overlay.
func (p Palette) View() string {
	contentWidth := p.width - 8
	if contentWidth > 60 {
		contentWidth = 60
	}
	if contentWidth < 40 {
		contentWidth = 40
	}

	// Title
	title := ui.HelpTitle.Width(contentWidth).Render(ui.FleurDeLis + " COMMAND PALETTE")

	// Input field
	inputLine := p.input.View()

	// Separator
	sep := lipgloss.NewStyle().Foreground(ui.DimPurple).Render(strings.Repeat("─", contentWidth))

	// Command list
	visible := p.filtered
	end := p.scrollOffset + paletteMaxVisible
	if end > len(visible) {
		end = len(visible)
	}
	if p.scrollOffset < len(visible) {
		visible = visible[p.scrollOffset:end]
	} else {
		visible = nil
	}

	keyWidth := 5
	nameWidth := contentWidth - keyWidth - 4
	if nameWidth < 20 {
		nameWidth = 20
	}

	rows := make([]string, 0, len(visible))
	for i, cmd := range visible {
		idx := p.scrollOffset + i
		cursor := "  "
		nameStyle := ui.HelpDesc
		if idx == p.cursor {
			cursor = ui.ItemCursor.Render(ui.Cursor + " ")
			nameStyle = lipgloss.NewStyle().Foreground(ui.White).Bold(true)
		}

		name := ansi.Truncate(cmd.Name, nameWidth, "...")
		key := ui.HelpKey.Width(keyWidth).Align(lipgloss.Right).Render(cmd.Key)
		row := cursor + nameStyle.Render(name)

		// Pad to fill width, then append key
		rowWidth := lipgloss.Width(row)
		gap := contentWidth - rowWidth - keyWidth
		if gap < 1 {
			gap = 1
		}
		row = row + strings.Repeat(" ", gap) + key

		if idx == p.cursor {
			row = ui.ItemSelectedBg.Width(contentWidth).Render(row)
		}

		rows = append(rows, row)
	}

	list := strings.Join(rows, "\n")
	if len(p.filtered) == 0 {
		list = ui.HelpHint.Width(contentWidth).Render("No matching commands")
	}

	// Hint
	hint := ui.HelpHint.Width(contentWidth).Render("esc to close")

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		inputLine,
		sep,
		list,
		"",
		hint,
	)

	box := ui.HelpOverlayBg.Width(contentWidth + 4).Render(content)

	return lipgloss.Place(p.width, p.height, lipgloss.Center, lipgloss.Center, box)
}
