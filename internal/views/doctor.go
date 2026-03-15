package views

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/matt-wright86/mardi-gras/internal/data"
	"github.com/matt-wright86/mardi-gras/internal/ui"
)

// Doctor renders the bd doctor diagnostics overlay in place of the detail pane.
type Doctor struct {
	width  int
	height int
	result *data.DoctorResult
	cursor int
}

// NewDoctor creates a Doctor panel.
func NewDoctor(width, height int) Doctor {
	return Doctor{width: width, height: height}
}

// SetSize updates dimensions.
func (d *Doctor) SetSize(width, height int) {
	d.width = width
	d.height = height
}

// SetResult updates the diagnostic result.
func (d *Doctor) SetResult(result *data.DoctorResult) {
	d.result = result
	d.cursor = 0
}

// HasResult returns true if a diagnostic result has been loaded.
func (d *Doctor) HasResult() bool {
	return d.result != nil
}

// Update handles key events for the doctor view.
func (d Doctor) Update(msg tea.Msg) (Doctor, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return d, nil
	}
	if d.result == nil || len(d.result.Diagnostics) == 0 {
		return d, nil
	}

	last := len(d.result.Diagnostics) - 1
	switch keyMsg.String() {
	case "j", "down":
		if d.cursor < last {
			d.cursor++
		}
	case "k", "up":
		if d.cursor > 0 {
			d.cursor--
		}
	case "g":
		d.cursor = 0
	case "G":
		d.cursor = last
	}

	return d, nil
}

// View renders the doctor diagnostics panel.
func (d Doctor) View() string {
	var lines []string

	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(ui.BrightGold)

	switch {
	case d.result == nil:
		lines = append(lines, headerStyle.Render("DIAGNOSTICS"))
		lines = append(lines, "")
		loadingStyle := lipgloss.NewStyle().Foreground(ui.Dim)
		lines = append(lines, loadingStyle.Render("  Running bd doctor..."))
	case d.result.OK:
		lines = append(lines, headerStyle.Render("DIAGNOSTICS"))
		lines = append(lines, "")
		okStyle := lipgloss.NewStyle().Foreground(ui.BrightGreen)
		lines = append(lines, okStyle.Render("  "+ui.SymResolved+" "+d.result.Summary))
		lines = append(lines, "")
		// Still show all checks even when OK
		for i, diag := range d.result.Diagnostics {
			lines = append(lines, d.renderDiagnostic(i, diag)...)
			lines = append(lines, "")
		}
	default:
		warnStyle := lipgloss.NewStyle().Foreground(ui.StatusStalled).Bold(true)
		header := fmt.Sprintf("DIAGNOSTICS — %s", d.result.Summary)
		lines = append(lines, warnStyle.Render(header))
		lines = append(lines, "")

		for i, diag := range d.result.Diagnostics {
			lines = append(lines, d.renderDiagnostic(i, diag)...)
			lines = append(lines, "")
		}
	}

	// Hint bar
	hintStyle := lipgloss.NewStyle().Foreground(ui.Dim)
	lines = append(lines, hintStyle.Render("  D close  R refresh"))

	content := strings.Join(lines, "\n")

	return ui.DetailBorder.
		Width(d.width).
		Height(d.height).
		Render(content)
}

func (d Doctor) renderDiagnostic(idx int, diag data.DoctorDiagnostic) []string {
	var lines []string

	// Status symbol and style
	var sym string
	var statusStyle lipgloss.Style
	switch diag.Status {
	case "error":
		sym = ui.SymStalled
		statusStyle = lipgloss.NewStyle().Foreground(ui.StatusStalled).Bold(true)
	case "warning":
		sym = ui.SymOverdue
		statusStyle = lipgloss.NewStyle().Foreground(ui.BrightGold).Bold(true)
	default: // "ok"
		sym = ui.SymResolved
		statusStyle = lipgloss.NewStyle().Foreground(ui.BrightGreen)
	}

	prefix := "  "
	if idx == d.cursor {
		prefix = ui.ItemCursor.Render(ui.Cursor) + " "
	}

	// Name + status + category
	nameStyle := lipgloss.NewStyle().Foreground(ui.Light).Bold(true)
	categoryStyle := lipgloss.NewStyle().Foreground(ui.Muted)

	line1 := fmt.Sprintf("%s%s %s",
		prefix,
		statusStyle.Render(sym+" "+diag.Status),
		nameStyle.Render(diag.Name),
	)
	if diag.Category != "" {
		line1 += "  " + categoryStyle.Render(diag.Category)
	}
	lines = append(lines, line1)

	// Explanation
	if diag.Explanation != "" {
		detailStyle := lipgloss.NewStyle().Foreground(ui.Light)
		lines = append(lines, "    "+detailStyle.Render(diag.Explanation))
	}

	// Severity (for non-ok)
	if diag.Severity != "" && diag.Status != "ok" {
		sevStyle := lipgloss.NewStyle().Foreground(ui.Muted).Italic(true)
		lines = append(lines, "    "+sevStyle.Render("severity: "+diag.Severity))
	}

	// Observed vs Expected
	if diag.Observed != "" {
		obsStyle := lipgloss.NewStyle().Foreground(ui.Muted)
		lines = append(lines, "    "+obsStyle.Render("observed: "+diag.Observed))
	}
	if diag.Expected != "" {
		expStyle := lipgloss.NewStyle().Foreground(ui.Muted)
		lines = append(lines, "    "+expStyle.Render("expected: "+diag.Expected))
	}

	// Fix commands
	if len(diag.Commands) > 0 {
		cmdStyle := lipgloss.NewStyle().Foreground(ui.Dim)
		for _, cmd := range diag.Commands {
			lines = append(lines, "    "+cmdStyle.Render("$ "+cmd))
		}
	}

	return lines
}
