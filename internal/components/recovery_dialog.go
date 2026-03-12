package components

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/matt-wright86/mardi-gras/internal/gastown"
	"github.com/matt-wright86/mardi-gras/internal/ui"
)

// RecoveryDialogResult is sent when the recovery dialog completes.
type RecoveryDialogResult struct {
	RigName   string
	Orphans   []gastown.OrphanedIssue
	Mode      gastown.RecoveryMode
	Cancelled bool
}

// RecoveryDialog shows a confirmation before executing rig recovery.
type RecoveryDialog struct {
	rigName string
	orphans []gastown.OrphanedIssue
	modeIdx int // 0=resling, 1=release-only
	width   int
	height  int
}

var recoveryModes = []struct {
	Label string
	Desc  string
	Mode  gastown.RecoveryMode
}{
	{"Release + Re-sling", "Free orphans and dispatch to new polecats", gastown.RecoveryResling},
	{"Release only", "Free orphans for manual re-dispatch", gastown.RecoveryReleaseOnly},
}

// NewRecoveryDialog creates a recovery confirmation dialog.
func NewRecoveryDialog(rigName string, orphans []gastown.OrphanedIssue, width, height int) RecoveryDialog {
	return RecoveryDialog{
		rigName: rigName,
		orphans: orphans,
		width:   width,
		height:  height,
	}
}

// Update handles key events for the recovery dialog.
func (rd RecoveryDialog) Update(msg tea.Msg) (RecoveryDialog, tea.Cmd) {
	km, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return rd, nil
	}

	switch km.String() {
	case "esc", "q":
		return rd, func() tea.Msg {
			return RecoveryDialogResult{Cancelled: true}
		}

	case "j", "down":
		if rd.modeIdx < len(recoveryModes)-1 {
			rd.modeIdx++
		}

	case "k", "up":
		if rd.modeIdx > 0 {
			rd.modeIdx--
		}

	case "enter":
		selected := recoveryModes[rd.modeIdx]
		return rd, func() tea.Msg {
			return RecoveryDialogResult{
				RigName: rd.rigName,
				Orphans: rd.orphans,
				Mode:    selected.Mode,
			}
		}
	}

	return rd, nil
}

// View renders the recovery confirmation dialog.
func (rd RecoveryDialog) View() string {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(ui.BrightGold)
	warnStyle := lipgloss.NewStyle().Bold(true).Foreground(ui.StatusStalled)
	dimStyle := lipgloss.NewStyle().Foreground(ui.Dim)
	normalStyle := lipgloss.NewStyle().Foreground(ui.Light)
	selectedStyle := lipgloss.NewStyle().Foreground(ui.BrightGreen)

	var lines []string

	// Title
	lines = append(lines, titleStyle.Render(fmt.Sprintf("  %s RECOVER RIG: %s", ui.SymDeadRig, rd.rigName)))
	lines = append(lines, "")

	// Warning
	lines = append(lines, warnStyle.Render(fmt.Sprintf("  %d orphaned issue(s) will be recovered:", len(rd.orphans))))
	lines = append(lines, "")

	// Orphan list
	for _, o := range rd.orphans {
		title := o.Title
		if title == "" {
			title = o.IssueID
		}
		agentInfo := ""
		if o.AgentName != "" {
			agentInfo = dimStyle.Render(fmt.Sprintf(" (was %s)", o.AgentName))
		}
		lines = append(lines, fmt.Sprintf("    %s %s  %s%s",
			lipgloss.NewStyle().Foreground(ui.StatusStalled).Render(ui.SymIdle),
			dimStyle.Render(o.IssueID),
			normalStyle.Render(title),
			agentInfo,
		))
	}
	lines = append(lines, "")

	// Mode selection
	lines = append(lines, titleStyle.Render("  Recovery mode:"))
	lines = append(lines, "")
	for i, mode := range recoveryModes {
		cursor := "    "
		labelStyle := normalStyle
		descStyle := dimStyle
		if i == rd.modeIdx {
			cursor = selectedStyle.Render("  > ")
			labelStyle = selectedStyle
			descStyle = lipgloss.NewStyle().Foreground(ui.Muted)
		}
		lines = append(lines, fmt.Sprintf("%s%s", cursor, labelStyle.Render(mode.Label)))
		lines = append(lines, fmt.Sprintf("      %s", descStyle.Render(mode.Desc)))
	}

	return strings.Join(lines, "\n")
}
