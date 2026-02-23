package ui

import "github.com/charmbracelet/lipgloss"

// Mardi Gras palette.
var (
	// Core parade colors
	Purple = lipgloss.Color("#7B2D8E")
	Gold   = lipgloss.Color("#F5C518")
	Green  = lipgloss.Color("#1D8348")

	// Brighter variants for emphasis
	BrightPurple = lipgloss.Color("#9B59B6")
	BrightGold   = lipgloss.Color("#FFD700")
	BrightGreen  = lipgloss.Color("#2ECC71")

	// Dimmed variants for backgrounds/borders
	DimPurple = lipgloss.Color("#4A1259")
	DimGold   = lipgloss.Color("#8B7D00")
	DimGreen  = lipgloss.Color("#145A32")

	// Neutrals
	White   = lipgloss.Color("#FAFAFA")
	Light   = lipgloss.Color("#CCCCCC")
	Muted   = lipgloss.Color("#888888")
	Dim     = lipgloss.Color("#555555")
	Dark    = lipgloss.Color("#333333")
	Darkest = lipgloss.Color("#1A1A1A")

	// Semantic: parade status
	StatusRolling = BrightGreen
	StatusLinedUp = BrightGold
	StatusStalled = lipgloss.Color("#E74C3C")
	StatusPassed  = Muted
	StatusAgent   = BrightPurple
	StatusConvoy  = BrightGold
	StatusMail    = BrightGreen

	// Priority colors (P0=critical red → P4=backlog gray)
	PrioP0 = lipgloss.Color("#FF3333")
	PrioP1 = lipgloss.Color("#FF8C00")
	PrioP2 = BrightGold
	PrioP3 = BrightGreen
	PrioP4 = Muted

	// Issue type colors
	ColorBug     = lipgloss.Color("#E74C3C")
	ColorFeature = BrightPurple
	ColorTask    = BrightGold
	ColorChore   = Muted
	ColorEpic    = lipgloss.Color("#3498DB")

	// Neutrals (extra)
	Silver = lipgloss.Color("#AAAAAA")

	// Gas Town role colors
	RoleMayor   = BrightGold
	RolePolecat = BrightGreen
	RoleCrew    = BrightPurple
	RoleDefault = Silver

	// Gas Town agent state colors
	StateWorking = BrightGreen
	StateIdle    = Silver
	StateBackoff = lipgloss.Color("#E74C3C")
)

// PriorityColor returns the theme color for a priority level.
func PriorityColor(p int) lipgloss.Color {
	switch p {
	case 0:
		return PrioP0
	case 1:
		return PrioP1
	case 2:
		return PrioP2
	case 3:
		return PrioP3
	case 4:
		return PrioP4
	default:
		return Muted
	}
}

// IssueTypeColor returns the theme color for an issue type.
// RoleColor returns the theme color for a Gas Town agent role.
func RoleColor(role string) lipgloss.Color {
	switch role {
	case "mayor":
		return RoleMayor
	case "polecat":
		return RolePolecat
	case "crew":
		return RoleCrew
	default:
		return RoleDefault
	}
}

// AgentStateColor returns the theme color for a Gas Town agent state.
func AgentStateColor(state string) lipgloss.Color {
	switch state {
	case "working":
		return StateWorking
	case "backoff":
		return StateBackoff
	default:
		return StateIdle
	}
}

func IssueTypeColor(t string) lipgloss.Color {
	switch t {
	case "bug":
		return ColorBug
	case "feature":
		return ColorFeature
	case "task":
		return ColorTask
	case "chore":
		return ColorChore
	case "epic":
		return ColorEpic
	default:
		return Muted
	}
}
