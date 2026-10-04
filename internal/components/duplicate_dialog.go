package components

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/matt-wright86/mardi-gras/internal/ui"
)

// DuplicateAction is what the user chose in the duplicate dialog.
type DuplicateAction int

const (
	// DuplicateCancel abandons the new issue.
	DuplicateCancel DuplicateAction = iota
	// DuplicateJump abandons the new issue and selects the existing one.
	DuplicateJump
	// DuplicateCreate creates the new issue anyway.
	DuplicateCreate
	// DuplicateLink creates the new issue and marks it a duplicate of the
	// selected existing one.
	DuplicateLink
)

// DuplicateDialogResult is sent when the duplicate dialog completes.
// TargetID is the selected existing issue for Jump and Link.
type DuplicateDialogResult struct {
	Action   DuplicateAction
	TargetID string
}

// DuplicateCandidate is one existing issue the judge thinks the new one
// may duplicate, with its probability.
type DuplicateCandidate struct {
	ID     string
	Title  string
	Status string
	Prob   float64
}

// DuplicateDialog asks, before creating an issue, whether it already exists.
type DuplicateDialog struct {
	title      string
	candidates []DuplicateCandidate // best first
	cursor     int
	width      int
}

// NewDuplicateDialog builds the dialog for a new issue titled title.
func NewDuplicateDialog(title string, candidates []DuplicateCandidate, width int) DuplicateDialog {
	return DuplicateDialog{title: title, candidates: candidates, width: width}
}

// Selected returns the candidate under the cursor.
func (d DuplicateDialog) Selected() DuplicateCandidate {
	if len(d.candidates) == 0 {
		return DuplicateCandidate{}
	}
	return d.candidates[d.cursor]
}

// Update handles key events.
func (d DuplicateDialog) Update(msg tea.Msg) (DuplicateDialog, tea.Cmd) {
	km, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return d, nil
	}
	result := func(action DuplicateAction) tea.Cmd {
		target := d.Selected().ID
		return func() tea.Msg { return DuplicateDialogResult{Action: action, TargetID: target} }
	}
	switch km.String() {
	case "esc", "q":
		return d, result(DuplicateCancel)
	case "j", "down":
		if d.cursor < len(d.candidates)-1 {
			d.cursor++
		}
	case "k", "up":
		if d.cursor > 0 {
			d.cursor--
		}
	case "enter":
		return d, result(DuplicateJump)
	case "c":
		return d, result(DuplicateCreate)
	case "l":
		return d, result(DuplicateLink)
	}
	return d, nil
}

// View renders the dialog body; the caller frames it.
func (d DuplicateDialog) View() string {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(ui.BrightGold)
	dimStyle := lipgloss.NewStyle().Foreground(ui.Dim)
	normalStyle := lipgloss.NewStyle().Foreground(ui.Light)
	selectedStyle := lipgloss.NewStyle().Foreground(ui.BrightGreen)
	warnStyle := lipgloss.NewStyle().Foreground(ui.StateStuck)

	var lines []string
	lines = append(lines,
		titleStyle.Render(fmt.Sprintf("  %s This may already exist", ui.SymDuplicates)),
		"",
		dimStyle.Render("  New: ")+normalStyle.Render(truncate(d.title, d.width-12)),
		"",
	)
	for i, c := range d.candidates {
		cursor := "    "
		idStyle, titleStyle := dimStyle, normalStyle
		if i == d.cursor {
			cursor = selectedStyle.Render("  > ")
			idStyle, titleStyle = selectedStyle, selectedStyle
		}
		pct := warnStyle.Render(fmt.Sprintf("%3.0f%%", c.Prob*100))
		status := ""
		if c.Status != "" {
			status = dimStyle.Render(" (" + c.Status + ")")
		}
		avail := d.width - 12 - len(c.ID) - len(c.Status) - 8
		lines = append(lines, fmt.Sprintf("%s%s %s  %s%s", cursor, pct, idStyle.Render(c.ID), titleStyle.Render(truncate(c.Title, avail)), status))
	}
	return strings.Join(lines, "\n")
}

// truncate shortens s to at most n runes with an ellipsis.
func truncate(s string, n int) string {
	if n < 4 {
		n = 4
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}
