package components

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/matt-wright86/mardi-gras/internal/data"
	"github.com/matt-wright86/mardi-gras/internal/ui"
)

// CreateFormResult is sent when the create form completes.
type CreateFormResult struct {
	Title     string
	Type      string
	Priority  string
	Cancelled bool
}

// CreateForm is a mini-form for creating a new issue.
type CreateForm struct {
	form      *huh.Form
	title     string
	issueType string
	priority  string
	width     int
	height    int
}

// NewCreateForm creates a new issue creation form.
func NewCreateForm(width, height int) CreateForm {
	cf := CreateForm{
		issueType: "task",
		priority:  "2",
		width:     width,
		height:    height,
	}

	theme := huh.ThemeCharm()
	theme.Focused.Title = theme.Focused.Title.Foreground(ui.BrightGold)
	theme.Focused.SelectedOption = theme.Focused.SelectedOption.Foreground(ui.BrightGreen)

	cf.form = huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Title").
				Value(&cf.title).
				Placeholder("Issue title..."),
			huh.NewSelect[string]().
				Title("Type").
				Options(
					huh.NewOption("Task", "task"),
					huh.NewOption("Bug", "bug"),
					huh.NewOption("Feature", "feature"),
					huh.NewOption("Chore", "chore"),
				).
				Value(&cf.issueType),
			huh.NewSelect[string]().
				Title("Priority").
				Options(
					huh.NewOption("P0 Critical", "0"),
					huh.NewOption("P1 High", "1"),
					huh.NewOption("P2 Medium", "2"),
					huh.NewOption("P3 Low", "3"),
					huh.NewOption("P4 Backlog", "4"),
				).
				Value(&cf.priority),
		),
	).WithTheme(theme).WithWidth(width - 8).WithShowHelp(false)

	return cf
}

// Init returns the form's init command.
func (cf CreateForm) Init() tea.Cmd {
	return cf.form.Init()
}

// Update forwards messages to the huh form.
func (cf CreateForm) Update(msg tea.Msg) (CreateForm, tea.Cmd) {
	// Handle escape to cancel
	if km, ok := msg.(tea.KeyMsg); ok {
		if km.String() == "esc" {
			return cf, func() tea.Msg {
				return CreateFormResult{Cancelled: true}
			}
		}
	}

	form, cmd := cf.form.Update(msg)
	if f, ok := form.(*huh.Form); ok {
		cf.form = f
	}

	// Check if form is complete
	if cf.form.State == huh.StateCompleted {
		return cf, func() tea.Msg {
			return CreateFormResult{
				Title:    cf.title,
				Type:     cf.issueType,
				Priority: cf.priority,
			}
		}
	}

	return cf, cmd
}

// View renders the form.
func (cf CreateForm) View() string {
	return cf.form.View()
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
