package components

import (
	"strings"

	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/matt-wright86/mardi-gras/internal/data"
	"github.com/matt-wright86/mardi-gras/internal/ui"
)

// EditFormResult is sent when the edit form completes. Changed lists the
// fields the user changed ("title", "type", "priority", "status",
// "description"), so the app writes only those.
type EditFormResult struct {
	IssueID     string
	Title       string
	Type        string
	Priority    string
	Status      string
	Description string
	Changed     []string
	Cancelled   bool
}

// Has reports whether field was changed.
func (r EditFormResult) Has(field string) bool {
	for _, f := range r.Changed {
		if f == field {
			return true
		}
	}
	return false
}

// statusOptions are the statuses the edit form offers.
var statusOptions = []selectOption{
	{Label: "Open", Value: string(data.StatusOpen)},
	{Label: "In progress", Value: string(data.StatusInProgress)},
	{Label: "Closed", Value: string(data.StatusClosed)},
}

// Edit form fields, in tab order.
const (
	editFieldTitle = iota
	editFieldType
	editFieldPriority
	editFieldStatus
	editFieldDescription
	editFieldCount
)

// EditForm edits an existing issue's title, type, priority, status and
// description. It covered only title and priority, so the rest needed the
// bd CLI (mg-nd2). Type, priority and status are one-line selectors so the
// description fits on a 24-row terminal.
type EditForm struct {
	issueID     string
	titleInput  textinput.Model
	descInput   textarea.Model
	types       []selectOption // typeOptions, plus the issue's own type if custom
	typeIdx     int
	prioIdx     int
	statusIdx   int
	activeField int
	err         string // shown under the title after a rejected save
	width       int
	height      int

	orig EditFormResult // the issue as opened, to work out what changed
}

// NewEditForm creates an edit form pre-populated from an existing issue.
func NewEditForm(width, height int, issue *data.Issue) EditForm {
	ti := textinput.New()
	ti.Prompt = ""
	ti.Placeholder = "Issue title..."
	ti.SetWidth(inputWidth(width))
	ti.SetValue(issue.Title)
	ti.Focus()

	ta := textarea.New()
	ta.Placeholder = "Description (markdown)..."
	ta.ShowLineNumbers = false
	ta.Prompt = ""
	ta.SetWidth(inputWidth(width))
	ta.SetHeight(5)
	ta.SetValue(issue.Description)
	ta.Blur()

	types := typeOptions
	typeIdx := indexOfOption(types, string(issue.IssueType))
	if typeIdx < 0 {
		// A custom type stays selectable instead of silently becoming Task.
		types = append(append([]selectOption{}, typeOptions...), selectOption{Label: string(issue.IssueType), Value: string(issue.IssueType)})
		typeIdx = len(types) - 1
	}
	statusIdx := max(indexOfOption(statusOptions, string(issue.Status)), 0)
	prioIdx := min(max(int(issue.Priority), 0), len(priorityOptions)-1)

	ef := EditForm{
		issueID:     issue.ID,
		titleInput:  ti,
		descInput:   ta,
		types:       types,
		typeIdx:     typeIdx,
		prioIdx:     prioIdx,
		statusIdx:   statusIdx,
		activeField: editFieldTitle,
		width:       width,
		height:      height,
	}
	ef.orig = ef.values()
	return ef
}

func indexOfOption(opts []selectOption, value string) int {
	for i, o := range opts {
		if o.Value == value {
			return i
		}
	}
	return -1
}

// values is the form's current content as a result, without Changed.
func (ef EditForm) values() EditFormResult {
	return EditFormResult{
		IssueID:     ef.issueID,
		Title:       strings.TrimSpace(ef.titleInput.Value()),
		Type:        ef.types[ef.typeIdx].Value,
		Priority:    priorityOptions[ef.prioIdx].Value,
		Status:      statusOptions[ef.statusIdx].Value,
		Description: ef.descInput.Value(),
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
		// Non-key messages (a paste, the blink) go to the focused input.
		var cmd tea.Cmd
		if ef.activeField == editFieldDescription {
			ef.descInput, cmd = ef.descInput.Update(msg)
		} else {
			ef.titleInput, cmd = ef.titleInput.Update(msg)
		}
		return ef, cmd
	}

	switch km.String() {
	case "esc":
		return ef, func() tea.Msg {
			return EditFormResult{Cancelled: true}
		}

	case "tab":
		ef.focus((ef.activeField + 1) % editFieldCount)
		return ef, nil

	case "shift+tab":
		ef.focus((ef.activeField + editFieldCount - 1) % editFieldCount)
		return ef, nil

	case "ctrl+s":
		return ef.save()

	case "enter":
		// Enter saves from any field but the description, where it starts
		// a new line; ctrl+s saves from there (mg-vtc, mg-nd2).
		if ef.activeField != editFieldDescription {
			return ef.save()
		}

	case "j", "down", "l", "right":
		if ef.cycle(+1) {
			return ef, nil
		}

	case "k", "up", "h", "left":
		if ef.cycle(-1) {
			return ef, nil
		}
	}

	var cmd tea.Cmd
	switch ef.activeField {
	case editFieldTitle:
		ef.titleInput, cmd = ef.titleInput.Update(msg)
		if strings.TrimSpace(ef.titleInput.Value()) != "" {
			ef.err = ""
		}
	case editFieldDescription:
		ef.descInput, cmd = ef.descInput.Update(msg)
	}
	return ef, cmd
}

// cycle moves the active selector by delta, clamped. It reports whether a
// selector was active (the key was consumed).
func (ef *EditForm) cycle(delta int) bool {
	step := func(i, n int) int { return min(max(i+delta, 0), n-1) }
	switch ef.activeField {
	case editFieldType:
		ef.typeIdx = step(ef.typeIdx, len(ef.types))
	case editFieldPriority:
		ef.prioIdx = step(ef.prioIdx, len(priorityOptions))
	case editFieldStatus:
		ef.statusIdx = step(ef.statusIdx, len(statusOptions))
	default:
		return false
	}
	return true
}

// focus moves to field, focusing or blurring the text inputs.
func (ef *EditForm) focus(field int) {
	ef.activeField = field
	ef.titleInput.Blur()
	ef.descInput.Blur()
	switch field {
	case editFieldTitle:
		ef.titleInput.Focus()
	case editFieldDescription:
		ef.descInput.Focus()
	}
}

// save validates the title and reports what changed.
func (ef EditForm) save() (EditForm, tea.Cmd) {
	res := ef.values()
	if res.Title == "" {
		ef.err = "Title is required"
		ef.focus(editFieldTitle)
		return ef, nil
	}
	for _, f := range []struct {
		name      string
		now, orig string
	}{
		{"title", res.Title, ef.orig.Title},
		{"type", res.Type, ef.orig.Type},
		{"priority", res.Priority, ef.orig.Priority},
		{"status", res.Status, ef.orig.Status},
		{"description", res.Description, ef.orig.Description},
	} {
		if f.now != f.orig {
			res.Changed = append(res.Changed, f.name)
		}
	}
	return ef, func() tea.Msg { return res }
}

// View renders the edit form.
func (ef EditForm) View() string {
	labelStyle := func(field int, name string) string {
		if ef.activeField == field {
			return lipgloss.NewStyle().Foreground(ui.BrightGold).Bold(true).Render("> " + name)
		}
		return lipgloss.NewStyle().Foreground(ui.Dim).Render("  " + name)
	}

	var lines []string

	// Header with issue ID
	lines = append(lines, lipgloss.NewStyle().Foreground(ui.Muted).Render("EDIT "+ef.issueID))
	lines = append(lines, "")

	// Title field
	lines = append(lines, labelStyle(editFieldTitle, "Title"))
	lines = append(lines, "  "+ef.titleInput.View())
	if ef.err != "" {
		lines = append(lines, "  "+lipgloss.NewStyle().Foreground(ui.StatusStalled).Render(ef.err))
	}
	lines = append(lines, "")

	// One-line selectors: "> Type      ‹ Bug ›"
	selector := func(field int, name, label string, style lipgloss.Style) string {
		value := style.Render(label)
		if ef.activeField == field {
			arrows := lipgloss.NewStyle().Foreground(ui.BrightGreen)
			value = arrows.Render("‹ ") + style.Bold(true).Render(label) + arrows.Render(" ›")
		} else {
			value = "  " + value
		}
		return lipgloss.NewStyle().Width(14).Render(labelStyle(field, name)) + " " + value
	}
	typ := ef.types[ef.typeIdx]
	lines = append(lines, selector(editFieldType, "Type", typ.Label, lipgloss.NewStyle().Foreground(ui.IssueTypeColor(typ.Value))))
	lines = append(lines, selector(editFieldPriority, "Priority", priorityOptions[ef.prioIdx].Label, lipgloss.NewStyle().Foreground(ui.PriorityColor(ef.prioIdx))))
	lines = append(lines, selector(editFieldStatus, "Status", statusOptions[ef.statusIdx].Label, lipgloss.NewStyle().Foreground(ui.Light)))
	lines = append(lines, "")

	// Description
	lines = append(lines, labelStyle(editFieldDescription, "Description"))
	for _, l := range strings.Split(ef.descInput.View(), "\n") {
		lines = append(lines, "  "+l)
	}

	return strings.Join(lines, "\n")
}
