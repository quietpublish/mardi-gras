// Package ui defines the Mardi Gras design system: color palette, role and
// state colors, lipgloss styles, unicode symbols, gradients, and sparkline
// renderers. It contains no business logic.
package ui

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

// Theme selects the palette variant. Dark is the default; light targets
// light-background terminals (issue #86).
type Theme int

const (
	ThemeDark Theme = iota
	ThemeLight
)

var currentTheme Theme

// isLight reports whether the light palette is active.
func isLight() bool { return currentTheme == ThemeLight }

// SetTheme switches the active palette and rebakes every derived style and
// gradient. Call it before the BubbleTea program starts: pre-rendered strings
// or styles captured by other packages at init time will not update after
// first render — never capture ui palette vars, styles, or pre-rendered
// strings in package-level vars outside internal/ui.
func SetTheme(t Theme) {
	currentTheme = t
	if t == ThemeLight {
		applyLightPalette()
	} else {
		applyDarkPalette()
	}
	applyDerived()
	rebuildStyles()
	rebuildGradients()
}

func init() { SetTheme(ThemeDark) }

// Mardi Gras palette. Values are assigned by SetTheme — every var has a dark
// and a light variant in the apply funcs below. The neutral names describe the
// dark theme (White = strongest ink); read them as semantic text roles.
var (
	// Core parade colors
	Purple color.Color
	Gold   color.Color
	Green  color.Color

	// Brighter variants for emphasis
	BrightPurple color.Color
	BrightGold   color.Color
	BrightGreen  color.Color

	// Dimmed variants for backgrounds/borders
	DimPurple color.Color
	DimGold   color.Color
	DimGreen  color.Color

	// Neutrals (White = strongest text ink → Darkest = deepest surface)
	White   color.Color
	Light   color.Color
	Muted   color.Color
	Dim     color.Color
	Dark    color.Color
	Darkest color.Color

	// Semantic: parade status
	StatusRolling color.Color
	StatusLinedUp color.Color
	StatusStalled color.Color
	StatusPassed  color.Color
	StatusAgent   color.Color
	StatusConvoy  color.Color
	StatusMail    color.Color

	// Priority colors (P0=critical red → P4=backlog gray)
	PrioP0 color.Color
	PrioP1 color.Color
	PrioP2 color.Color
	PrioP3 color.Color
	PrioP4 color.Color

	// Issue type colors
	ColorBug       color.Color
	ColorFeature   color.Color
	ColorTask      color.Color
	ColorChore     color.Color
	ColorEpic      color.Color
	ColorSpike     color.Color
	ColorStory     color.Color
	ColorMilestone color.Color

	// Neutrals (extra)
	Silver color.Color

	// Accent primitive shared by the witness role and the heat gradient
	Orange color.Color

	// Gas Town role colors
	RoleMayor    color.Color
	RoleDeacon   color.Color // Blue — town health monitor
	RolePolecat  color.Color
	RoleCrew     color.Color
	RoleWitness  color.Color // Orange — rig reviewer
	RoleRefinery color.Color // Teal — merge processor
	RoleDog      color.Color // Deep purple — infrastructure worker
	RoleDefault  color.Color

	// Gas Town agent state colors
	StateWorking    color.Color
	StateIdle       color.Color
	StateBackoff    color.Color
	StateStuck      color.Color // Amber — agent requesting help
	StateSpawn      color.Color // Cyan — session starting
	StateGate       color.Color // Waiting on external trigger
	StateFixNeeded  color.Color // Pink — review feedback, needs rework
	StatePropelled  color.Color // Dark turquoise — ACP propulsion, output suppressed
	StatePatrolling color.Color // Sky blue — witness/deacon scanning rounds

	// Overlay/toast ink colors (no brand-name equivalent; theme-tuned)
	HelpBg         color.Color // overlay background (help, palette, dialogs)
	HelpSubtitleFg color.Color
	HelpDescFg     color.Color
	HelpHintFg     color.Color
	ToastAccentFg  color.Color // text on success/warn toast backgrounds
	ToastErrorFg   color.Color // text on the error toast's red background
)

// applyDerived assigns the theme-invariant semantic aliases — colors defined
// in terms of other palette entries the same way in both themes. Runs after
// the per-theme palette so the aliases pick up the active values.
func applyDerived() {
	StatusRolling = BrightGreen
	StatusLinedUp = BrightGold
	StatusPassed = Muted
	StatusAgent = BrightPurple
	StatusConvoy = BrightGold
	StatusMail = BrightGreen

	PrioP2 = BrightGold
	PrioP3 = BrightGreen
	PrioP4 = Muted

	ColorBug = StatusStalled
	ColorFeature = BrightPurple
	ColorTask = BrightGold
	ColorChore = Muted

	RoleMayor = BrightGold
	RolePolecat = BrightGreen
	RoleCrew = BrightPurple
	RoleWitness = Orange
	RoleDefault = Silver

	StateWorking = BrightGreen
	StateIdle = Silver
	StateBackoff = StatusStalled
	StateGate = BrightGold
}

func applyDarkPalette() {
	Purple = lipgloss.Color("#7B2D8E")
	Gold = lipgloss.Color("#F5C518")
	Green = lipgloss.Color("#1D8348")

	BrightPurple = lipgloss.Color("#9B59B6")
	BrightGold = lipgloss.Color("#FFD700")
	BrightGreen = lipgloss.Color("#2ECC71")

	DimPurple = lipgloss.Color("#4A1259")
	DimGold = lipgloss.Color("#8B7D00")
	DimGreen = lipgloss.Color("#145A32")

	White = lipgloss.Color("#FAFAFA")
	Light = lipgloss.Color("#CCCCCC")
	Muted = lipgloss.Color("#888888")
	Dim = lipgloss.Color("#555555")
	Dark = lipgloss.Color("#333333")
	Darkest = lipgloss.Color("#1A1A1A")

	Silver = lipgloss.Color("#AAAAAA")
	Orange = lipgloss.Color("#E67E22")

	StatusStalled = lipgloss.Color("#E74C3C")

	PrioP0 = lipgloss.Color("#FF3333")
	PrioP1 = lipgloss.Color("#FF8C00")

	ColorEpic = lipgloss.Color("#3498DB")
	ColorSpike = lipgloss.Color("#F39C12")
	ColorStory = lipgloss.Color("#A569BD")
	ColorMilestone = lipgloss.Color("#00BCD4")

	RoleDeacon = lipgloss.Color("#3498DB")
	RoleRefinery = lipgloss.Color("#1ABC9C")
	RoleDog = lipgloss.Color("#8E44AD")

	StateStuck = lipgloss.Color("#FF8C00")
	StateSpawn = lipgloss.Color("#3498DB")
	StateFixNeeded = lipgloss.Color("#E056A0")
	StatePropelled = lipgloss.Color("#00CED1")
	StatePatrolling = lipgloss.Color("#5DADE2")

	HelpBg = lipgloss.Color("#121521")
	HelpSubtitleFg = lipgloss.Color("#A9AFBF")
	HelpDescFg = lipgloss.Color("#D6D8DF")
	HelpHintFg = lipgloss.Color("#8E94A6")
	ToastAccentFg = Darkest
	ToastErrorFg = White
}

func applyLightPalette() {
	Purple = lipgloss.Color("#7B2D8E")
	Gold = lipgloss.Color("#9A7D0A")
	Green = lipgloss.Color("#1D8348")

	BrightPurple = lipgloss.Color("#7D3C98")
	BrightGold = lipgloss.Color("#A67C00")
	BrightGreen = lipgloss.Color("#1E8449")

	DimPurple = lipgloss.Color("#D2B4DE")
	DimGold = lipgloss.Color("#D5BF6E")
	DimGreen = lipgloss.Color("#A9DFBF")

	White = lipgloss.Color("#1A1A1A")
	Light = lipgloss.Color("#3D3D3D")
	Muted = lipgloss.Color("#6B6B6B")
	Dim = lipgloss.Color("#9A9A9A")
	Dark = lipgloss.Color("#C8C8C8")
	Darkest = lipgloss.Color("#EAEAEA")

	Silver = lipgloss.Color("#7A7A7A")
	Orange = lipgloss.Color("#AF601A")

	StatusStalled = lipgloss.Color("#C0392B")

	PrioP0 = lipgloss.Color("#C62828")
	PrioP1 = lipgloss.Color("#C05F00")

	ColorEpic = lipgloss.Color("#21618C")
	ColorSpike = lipgloss.Color("#B9770E")
	ColorStory = lipgloss.Color("#76448A")
	ColorMilestone = lipgloss.Color("#00838F")

	RoleDeacon = lipgloss.Color("#21618C")
	RoleRefinery = lipgloss.Color("#117864")
	RoleDog = lipgloss.Color("#6C3483")

	StateStuck = lipgloss.Color("#B9770E")
	StateSpawn = lipgloss.Color("#21618C")
	StateFixNeeded = lipgloss.Color("#AD1457")
	StatePropelled = lipgloss.Color("#00838F")
	StatePatrolling = lipgloss.Color("#2874A6")

	HelpBg = lipgloss.Color("#F3EEF7")
	HelpSubtitleFg = lipgloss.Color("#5D6470")
	HelpDescFg = lipgloss.Color("#2F3339")
	HelpHintFg = lipgloss.Color("#6E7480")
	ToastAccentFg = lipgloss.Color("#FFFFFF")
	ToastErrorFg = lipgloss.Color("#FFFFFF")
}

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
	case "patrolling":
		return StatePatrolling
	case "paused", "muted":
		return Dim
	default:
		return StateIdle
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
	case "spike":
		return ColorSpike
	case "story":
		return ColorStory
	case "milestone":
		return ColorMilestone
	default:
		return Muted
	}
}
