package components

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/matt-wright86/mardi-gras/internal/data"
	"github.com/matt-wright86/mardi-gras/internal/ui"
)

// EditFormResult is sent when the edit form completes.
type EditFormResult struct {
	IssueID   string
	Title     string
	Priority  string
	Cancelled bool
}

// EditForm is a mini-form for editing an existing issue's title and priority.
type EditForm struct {
	issueID     string
	titleInput  textinput.Model
	prioIdx     int // selected index in priorityOptions
	activeField int // 0=title, 1=priority
	width       int
	height      int
}

// NewEditForm creates an edit form pre-populated from an existing issue.
func NewEditForm(width, height int, issue *data.Issue) EditForm {
	ti := textinput.New()
	ti.Prompt = ""
	ti.Placeholder = "Issue title..."
	ti.SetWidth(width - 16)
	ti.SetValue(issue.Title)
	ti.Focus()

	return EditForm{
		issueID:     issue.ID,
		titleInput:  ti,
		prioIdx:     int(issue.Priority),
		activeField: 0,
		width:       width,
		height:      height,
	}
}

// Init returns the blink command for the text input cursor.
func (ef EditForm) Init() tea.Cmd {
	return textinput.Blink
}

// Update handles messages for the edit form.
func (ef EditForm) Update(msg tea.Msg) (EditForm, tea.Cmd) {
	km, ok := msg.(tea.KeyPressMsg)
	if !ok {
		var cmd tea.Cmd
		ef.titleInput, cmd = ef.titleInput.Update(msg)
		return ef, cmd
	}

	switch km.String() {
	case "esc":
		return ef, func() tea.Msg {
			return EditFormResult{Cancelled: true}
		}

	case "tab":
		ef.activeField = (ef.activeField + 1) % 2
		if ef.activeField == 0 {
			ef.titleInput.Focus()
		} else {
			ef.titleInput.Blur()
		}
		return ef, nil

	case "shift+tab":
		ef.activeField = (ef.activeField + 1) % 2 // +1 == -1 mod 2
		if ef.activeField == 0 {
			ef.titleInput.Focus()
		} else {
			ef.titleInput.Blur()
		}
		return ef, nil

	case "enter":
		if ef.activeField == 1 {
			title := ef.titleInput.Value()
			if title == "" {
				return ef, nil
			}
			return ef, func() tea.Msg {
				return EditFormResult{
					IssueID:  ef.issueID,
					Title:    title,
					Priority: priorityOptions[ef.prioIdx].Value,
				}
			}
		}
		// On title field, advance to priority
		ef.activeField = 1
		ef.titleInput.Blur()
		return ef, nil

	case "j", "down":
		if ef.activeField == 1 {
			if ef.prioIdx < len(priorityOptions)-1 {
				ef.prioIdx++
			}
			return ef, nil
		}

	case "k", "up":
		if ef.activeField == 1 {
			if ef.prioIdx > 0 {
				ef.prioIdx--
			}
			return ef, nil
		}
	}

	if ef.activeField == 0 {
		var cmd tea.Cmd
		ef.titleInput, cmd = ef.titleInput.Update(msg)
		return ef, cmd
	}

	return ef, nil
}

// View renders the edit form.
func (ef EditForm) View() string {
	titleStyle := lipgloss.NewStyle().Foreground(ui.BrightGold).Bold(true)
	selectedStyle := lipgloss.NewStyle().Foreground(ui.BrightGreen)
	normalStyle := lipgloss.NewStyle().Foreground(ui.Light)
	dimStyle := lipgloss.NewStyle().Foreground(ui.Dim)

	var lines []string

	// Header with issue ID
	headerStyle := lipgloss.NewStyle().Foreground(ui.Muted)
	lines = append(lines, headerStyle.Render("EDIT "+ef.issueID))
	lines = append(lines, "")

	// Title field
	var label string
	if ef.activeField == 0 {
		label = titleStyle.Render("> Title")
	} else {
		label = dimStyle.Render("  Title")
	}
	lines = append(lines, label)
	lines = append(lines, "  "+ef.titleInput.View())
	lines = append(lines, "")

	// Priority field
	if ef.activeField == 1 {
		label = titleStyle.Render("> Priority")
	} else {
		label = dimStyle.Render("  Priority")
	}
	lines = append(lines, label)
	for i, opt := range priorityOptions {
		cursor := "  "
		style := normalStyle
		if i == ef.prioIdx {
			cursor = selectedStyle.Render("> ")
			style = selectedStyle
		}
		lines = append(lines, fmt.Sprintf("  %s%s", cursor, style.Render(opt.Label)))
	}

	return strings.Join(lines, "\n")
}
