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

// CreateFormResult is sent when the create form completes.
type CreateFormResult struct {
	Title      string
	Type       string
	Priority   string
	CrewMember string // non-empty when assigning to a Gas Town crew member
	Cancelled  bool
}

// selectOption is a single option in a select field.
type selectOption struct {
	Label string
	Value string
}

var typeOptions = []selectOption{
	{Label: "Task", Value: "task"},
	{Label: "Bug", Value: "bug"},
	{Label: "Feature", Value: "feature"},
	{Label: "Chore", Value: "chore"},
	{Label: "Epic", Value: "epic"},
	{Label: "Spike", Value: "spike"},
	{Label: "Story", Value: "story"},
	{Label: "Milestone", Value: "milestone"},
}

var priorityOptions = []selectOption{
	{Label: "P0 Critical", Value: "0"},
	{Label: "P1 High", Value: "1"},
	{Label: "P2 Medium", Value: "2"},
	{Label: "P3 Low", Value: "3"},
	{Label: "P4 Backlog", Value: "4"},
}

// CreateForm is a mini-form for creating a new issue.
type CreateForm struct {
	titleInput  textinput.Model
	crewInput   textinput.Model
	typeIdx     int // selected index in typeOptions
	prioIdx     int // selected index in priorityOptions
	activeField int // 0=title, 1=type, 2=priority, 3=crew (when gtAvailable)
	fieldCount  int // 3 without GT, 4 with GT
	width       int
	height      int
}

// NewCreateForm creates a new issue creation form (without Gas Town crew field).
func NewCreateForm(width, height int) CreateForm {
	return newCreateForm(width, height, false)
}

// NewCreateFormWithGT creates a new issue creation form with a crew member field.
func NewCreateFormWithGT(width, height int) CreateForm {
	return newCreateForm(width, height, true)
}

func newCreateForm(width, height int, gtAvailable bool) CreateForm {
	ti := textinput.New()
	ti.Prompt = ""
	ti.Placeholder = "Issue title..."
	ti.SetWidth(width - 16)
	ti.Focus()

	ci := textinput.New()
	ci.Prompt = ""
	ci.Placeholder = "Crew member name (optional)..."
	ci.SetWidth(width - 16)

	fieldCount := 3
	if gtAvailable {
		fieldCount = 4
	}

	return CreateForm{
		titleInput:  ti,
		crewInput:   ci,
		typeIdx:     0, // default: task
		prioIdx:     2, // default: P2 Medium
		activeField: 0,
		fieldCount:  fieldCount,
		width:       width,
		height:      height,
	}
}

// Init returns the blink command for the text input cursor.
func (cf CreateForm) Init() tea.Cmd {
	return textinput.Blink
}

// Update handles messages for the form.
func (cf CreateForm) Update(msg tea.Msg) (CreateForm, tea.Cmd) {
	km, ok := msg.(tea.KeyPressMsg)
	if !ok {
		// Forward non-key messages to text input
		var cmd tea.Cmd
		cf.titleInput, cmd = cf.titleInput.Update(msg)
		return cf, cmd
	}

	lastField := cf.fieldCount - 1

	switch km.String() {
	case "esc":
		return cf, func() tea.Msg {
			return CreateFormResult{Cancelled: true}
		}

	case "tab":
		cf.activeField = (cf.activeField + 1) % cf.fieldCount
		cf.focusActiveInput()
		return cf, nil

	case "shift+tab":
		cf.activeField = (cf.activeField + cf.fieldCount - 1) % cf.fieldCount
		cf.focusActiveInput()
		return cf, nil

	case "enter":
		if cf.activeField == lastField {
			// Submit on last field
			title := cf.titleInput.Value()
			if title == "" {
				return cf, nil
			}
			return cf, func() tea.Msg {
				return CreateFormResult{
					Title:      title,
					Type:       typeOptions[cf.typeIdx].Value,
					Priority:   priorityOptions[cf.prioIdx].Value,
					CrewMember: cf.crewInput.Value(),
				}
			}
		}
		// On other fields, advance
		cf.activeField++
		cf.focusActiveInput()
		return cf, nil

	case "j", "down":
		if cf.activeField == 1 {
			if cf.typeIdx < len(typeOptions)-1 {
				cf.typeIdx++
			}
			return cf, nil
		}
		if cf.activeField == 2 {
			if cf.prioIdx < len(priorityOptions)-1 {
				cf.prioIdx++
			}
			return cf, nil
		}

	case "k", "up":
		if cf.activeField == 1 {
			if cf.typeIdx > 0 {
				cf.typeIdx--
			}
			return cf, nil
		}
		if cf.activeField == 2 {
			if cf.prioIdx > 0 {
				cf.prioIdx--
			}
			return cf, nil
		}
	}

	// Forward to text input when on a text field
	if cf.activeField == 0 {
		var cmd tea.Cmd
		cf.titleInput, cmd = cf.titleInput.Update(msg)
		return cf, cmd
	}
	if cf.activeField == 3 {
		var cmd tea.Cmd
		cf.crewInput, cmd = cf.crewInput.Update(msg)
		return cf, cmd
	}

	return cf, nil
}

// focusActiveInput ensures the correct text input has focus.
func (cf *CreateForm) focusActiveInput() {
	cf.titleInput.Blur()
	cf.crewInput.Blur()
	switch cf.activeField {
	case 0:
		cf.titleInput.Focus()
	case 3:
		cf.crewInput.Focus()
	}
}

// View renders the form.
func (cf CreateForm) View() string {
	titleStyle := lipgloss.NewStyle().Foreground(ui.BrightGold).Bold(true)
	selectedStyle := lipgloss.NewStyle().Foreground(ui.BrightGreen)
	normalStyle := lipgloss.NewStyle().Foreground(ui.Light)
	dimStyle := lipgloss.NewStyle().Foreground(ui.Dim)

	var lines []string

	// Title field
	var label string
	if cf.activeField == 0 {
		label = titleStyle.Render("> Title")
	} else {
		label = dimStyle.Render("  Title")
	}
	lines = append(lines, label)
	lines = append(lines, "  "+cf.titleInput.View())
	lines = append(lines, "")

	// Type field
	if cf.activeField == 1 {
		label = titleStyle.Render("> Type")
	} else {
		label = dimStyle.Render("  Type")
	}
	lines = append(lines, label)
	for i, opt := range typeOptions {
		cursor := "  "
		style := normalStyle
		if i == cf.typeIdx {
			cursor = selectedStyle.Render("> ")
			style = selectedStyle
		}
		lines = append(lines, fmt.Sprintf("  %s%s", cursor, style.Render(opt.Label)))
	}
	lines = append(lines, "")

	// Priority field
	if cf.activeField == 2 {
		label = titleStyle.Render("> Priority")
	} else {
		label = dimStyle.Render("  Priority")
	}
	lines = append(lines, label)
	for i, opt := range priorityOptions {
		cursor := "  "
		style := normalStyle
		if i == cf.prioIdx {
			cursor = selectedStyle.Render("> ")
			style = selectedStyle
		}
		lines = append(lines, fmt.Sprintf("  %s%s", cursor, style.Render(opt.Label)))
	}

	// Crew member field (only when Gas Town is available)
	if cf.fieldCount > 3 {
		lines = append(lines, "")
		if cf.activeField == 3 {
			label = titleStyle.Render("> Crew")
		} else {
			label = dimStyle.Render("  Crew")
		}
		lines = append(lines, label)
		lines = append(lines, "  "+cf.crewInput.View())
	}

	return strings.Join(lines, "\n")
}

// ParsePriority converts the form's priority string to data.Priority.
func ParsePriority(s string) data.Priority {
	switch s {
	case "0":
		return data.PriorityCritical
	case "1":
		return data.PriorityHigh
	case "3":
		return data.PriorityLow
	case "4":
		return data.PriorityBacklog
	default:
		return data.PriorityMedium
	}
}
