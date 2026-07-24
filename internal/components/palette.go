package components

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
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
	ActionAddNote
	ActionToggleFocus
	ActionToggleClosed
	ActionFilter
	ActionLaunchAgent
	ActionKillAgent
	ActionSlingFormula
	ActionNudgeAgent
	ActionHelp
	ActionQuit
	ActionFormulaSelect
	ActionToggleGasTown
	ActionAssign
	ActionCreateConvoy
	ActionCascadeClose
	ActionCycleLayout
	ActionRecoverRigs
	ActionPrunePreview
	ActionPruneClosed
	ActionClaimNextReady
	ActionCodexResume
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
	matched      [][]int // fuzzy match indices per filtered entry (nil when unfiltered)
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
	ti.Focus()

	contentWidth := width - 8
	if contentWidth > 60 {
		contentWidth = 60
	}
	ti.SetWidth(contentWidth - 6)

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
	if msg, ok := msg.(tea.KeyPressMsg); ok {
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
		p.matched = nil
		p.cursor = 0
		p.scrollOffset = 0
		return
	}

	src := commandSource{commands: p.commands}
	matches := fuzzy.FindFrom(query, src)

	result := make([]PaletteCommand, 0, len(matches))
	matched := make([][]int, 0, len(matches))
	for _, match := range matches {
		result = append(result, p.commands[match.Index])
		matched = append(matched, match.MatchedIndexes)
	}
	p.filtered = result
	p.matched = matched
	p.cursor = 0
	p.scrollOffset = 0
}

// SelectedName returns the Name of the currently highlighted command.
func (p Palette) SelectedName() string {
	if p.cursor >= 0 && p.cursor < len(p.filtered) {
		return p.filtered[p.cursor].Name
	}
	return ""
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
		selected := idx == p.cursor
		cursor := "  "
		if selected {
			cursor = ui.ItemCursor.Render(ui.Cursor + " ")
		}

		// Leading fixed-width hotkey chip: keeps key↔command pairing obvious
		// and the row safely inside the box (audit #7).
		key := ui.HelpKey.Width(keyWidth).Render(ansi.Truncate(cmd.Key, keyWidth-1, ""))

		name := ansi.Truncate(cmd.Name, nameWidth, "...")
		var renderedName string
		switch {
		case p.matched != nil && idx < len(p.matched) && len(p.matched[idx]) > 0:
			renderedName = ui.HighlightMatches(name, p.matched[idx], nameWidth)
		case selected:
			renderedName = lipgloss.NewStyle().Foreground(ui.White).Bold(true).Render(name)
		default:
			renderedName = ui.HelpDesc.Render(name)
		}

		row := cursor + key + renderedName
		if selected {
			row = ui.SelectedRow(row, contentWidth)
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

	box := ui.OverlayBox(content, contentWidth+4)

	return lipgloss.Place(p.width, p.height, lipgloss.Center, lipgloss.Center, box)
}
