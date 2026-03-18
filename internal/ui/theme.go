// Package ui defines the Mardi Gras design system: color palette, role and
// state colors, lipgloss styles, unicode symbols, gradients, and sparkline
// renderers. It contains no business logic.
package ui

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

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
	RoleMayor    = BrightGold
	RoleDeacon   = lipgloss.Color("#3498DB") // Blue — town health monitor
	RolePolecat  = BrightGreen
	RoleCrew     = BrightPurple
	RoleWitness  = lipgloss.Color("#E67E22") // Orange — rig reviewer
	RoleRefinery = lipgloss.Color("#1ABC9C") // Teal — merge processor
	RoleDog      = lipgloss.Color("#8E44AD") // Deep purple — infrastructure worker
	RoleDefault  = Silver

	// Gas Town agent state colors
	StateWorking   = BrightGreen
	StateIdle      = Silver
	StateBackoff   = lipgloss.Color("#E74C3C")
	StateStuck     = lipgloss.Color("#FF8C00") // Amber — agent requesting help
	StateSpawn     = lipgloss.Color("#3498DB") // Cyan — session starting
	StateGate      = BrightGold                // Waiting on external trigger
	StateFixNeeded = lipgloss.Color("#E056A0") // Pink — review feedback, needs rework
	StatePropelled = lipgloss.Color("#00CED1") // Dark turquoise — ACP propulsion, output suppressed

	// HOP quality colors
	QualityExcellent = BrightGold                // 0.9+
	QualityGood      = BrightGreen               // 0.7+
	QualityFair      = Silver                    // 0.5+
	QualityPoor      = lipgloss.Color("#E74C3C") // 0.3+
	QualityLow       = Dim                       // below 0.3
	CrystalColor     = BrightPurple              // crystallizing work
	EphemeralColor   = Dim                       // ephemeral work
)

// PriorityColor returns the theme color for a priority level.
func PriorityColor(p int) color.Color {
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
func RoleColor(role string) color.Color {
	switch role {
	case "mayor", "coordinator":
		return RoleMayor
	case "deacon", "health-check":
		return RoleDeacon
	case "polecat":
		return RolePolecat
	case "crew":
		return RoleCrew
	case "witness":
		return RoleWitness
	case "refinery":
		return RoleRefinery
	case "dog":
		return RoleDog
	default:
		return RoleDefault
	}
}

// AgentStateColor returns the theme color for a Gas Town agent state.
func AgentStateColor(state string) color.Color {
	switch state {
	case "working":
		return StateWorking
	case "spawning":
		return StateSpawn
	case "backoff", "degraded":
		return StateBackoff
	case "stuck":
		return StateStuck
	case "awaiting-gate":
		return StateGate
	case "fix_needed":
		return StateFixNeeded
	case "propelled":
		return StatePropelled
	case "paused", "muted":
		return Dim
	default:
		return StateIdle
	}
}

// QualityColor returns the theme color for a quality score (0.0-1.0).
func QualityColor(score float32) color.Color {
	switch {
	case score >= 0.9:
		return QualityExcellent
	case score >= 0.7:
		return QualityGood
	case score >= 0.5:
		return QualityFair
	case score >= 0.3:
		return QualityPoor
	default:
		return QualityLow
	}
}

func IssueTypeColor(t string) color.Color {
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
