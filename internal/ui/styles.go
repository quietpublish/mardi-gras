package ui

import "github.com/charmbracelet/lipgloss"

// Pre-built styles for the Mardi Gras theme.
var (
	// Header
	HeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Background(DimPurple).
			Padding(0, 1)

	HeaderCounts = lipgloss.NewStyle().
			Foreground(Light)

	// Bead string decorations
	BeadStylePurple = lipgloss.NewStyle().Foreground(Purple)
	BeadStyleGold   = lipgloss.NewStyle().Foreground(Gold)
	BeadStyleGreen  = lipgloss.NewStyle().Foreground(Green)

	// Section headers in parade list (used for title text color within borders)
	SectionRolling = lipgloss.NewStyle().
			Bold(true).
			Foreground(StatusRolling)

	SectionLinedUp = lipgloss.NewStyle().
			Bold(true).
			Foreground(StatusLinedUp)

	SectionStalled = lipgloss.NewStyle().
			Bold(true).
			Foreground(StatusStalled)

	SectionPassed = lipgloss.NewStyle().
			Bold(true).
			Foreground(StatusPassed)

	// Issue items in the list
	ItemNormal = lipgloss.NewStyle().
			PaddingLeft(3)

	ItemSelected = lipgloss.NewStyle().
			PaddingLeft(1).
			Bold(true).
			Foreground(White)

	ItemCursor = lipgloss.NewStyle().
			Foreground(BrightGold).
			Bold(true)

	ItemSelectedBg = lipgloss.NewStyle().
			Background(DimPurple)

	// Detail panel (right side)
	DetailBorder = lipgloss.NewStyle().
			BorderLeft(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(DimPurple).
			PaddingLeft(1)

	DetailTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(White)

	DetailLabel = lipgloss.NewStyle().
			Foreground(Muted).
			Width(12)

	DetailValue = lipgloss.NewStyle().
			Foreground(Light)

	DetailSection = lipgloss.NewStyle().
			Bold(true).
			Foreground(BrightGold).
			MarginTop(1)

	// Priority badge
	BadgePriority = lipgloss.NewStyle().
			Bold(true)

	// Type badge
	BadgeType = lipgloss.NewStyle().
			Italic(true)

	// Footer
	FooterStyle = lipgloss.NewStyle().
			Foreground(Light).
			Background(DimPurple).
			Padding(0, 1)

	FooterKey = lipgloss.NewStyle().
			Bold(true).
			Foreground(BrightGold)

	FooterDesc = lipgloss.NewStyle().
			Foreground(Light)

	// Dependency display
	DepBlocked = lipgloss.NewStyle().
			Foreground(StatusStalled)

	DepBlocks = lipgloss.NewStyle().
			Foreground(StatusLinedUp)

	DepMissing = lipgloss.NewStyle().
			Foreground(StatusStalled).
			Bold(true)

	DepResolved = lipgloss.NewStyle().
			Foreground(StatusPassed)

	DepNonBlocking = lipgloss.NewStyle().
			Foreground(Muted)

	// Due date badges
	OverdueBadge = lipgloss.NewStyle().
			Foreground(StatusStalled).
			Bold(true)

	DueSoonBadge = lipgloss.NewStyle().
			Foreground(PrioP1) // orange

	DeferredStyle = lipgloss.NewStyle().
			Foreground(Dim)

	// Rich dependency styles
	DepRelated = lipgloss.NewStyle().
			Foreground(BrightPurple)

	DepDuplicates = lipgloss.NewStyle().
			Foreground(Muted).
			Italic(true)

	DepSupersedes = lipgloss.NewStyle().
			Foreground(BrightGold)

	AgentBadge  = lipgloss.NewStyle().Foreground(StatusAgent).Bold(true)
	ConvoyBadge = lipgloss.NewStyle().Foreground(StatusConvoy).Bold(true)
	GasTownTag  = lipgloss.NewStyle().Foreground(BrightPurple).Italic(true)

	// Gas Town panel
	GasTownBorder = lipgloss.NewStyle().
			BorderLeft(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(BrightGold).
			PaddingLeft(1)

	GasTownTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(BrightGold).
			MarginTop(1)

	GasTownLabel = lipgloss.NewStyle().
			Foreground(Muted)

	GasTownValue = lipgloss.NewStyle().
			Foreground(Light)

	FooterSource = lipgloss.NewStyle().
			Foreground(Muted)

	// Filter Input
	InputPrompt = lipgloss.NewStyle().
			Foreground(BrightGold).
			Bold(true).
			PaddingLeft(1)

	InputText = lipgloss.NewStyle().
			Foreground(White)

	InputCursor = lipgloss.NewStyle().
			Foreground(Purple)

	// Help Overlay
	HelpOverlayBg = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(BrightPurple).
			Background(lipgloss.Color("#121521")).
			Padding(1, 2)

	HelpTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(BrightGold).
			Align(lipgloss.Center)

	HelpSubtitle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#A9AFBF")).
			Align(lipgloss.Center)

	HelpSection = lipgloss.NewStyle().
			Bold(true).
			Foreground(BrightGreen).
			Underline(true)

	HelpKey = lipgloss.NewStyle().
		Bold(true).
		Foreground(Gold)

	HelpDesc = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#D6D8DF"))

	HelpHint = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#8E94A6")).
			Align(lipgloss.Center)

	// Toast notifications
	ToastInfo = lipgloss.NewStyle().
			Foreground(Light).
			Background(DimPurple).
			Padding(0, 1)

	ToastSuccess = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#1A1A1A")).
			Background(BrightGreen).
			Bold(true).
			Padding(0, 1)

	ToastWarn = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#1A1A1A")).
			Background(BrightGold).
			Bold(true).
			Padding(0, 1)

	ToastError = lipgloss.NewStyle().
			Foreground(White).
			Background(lipgloss.Color("#E74C3C")).
			Bold(true).
			Padding(0, 1)
)

// RoleBadge returns a styled badge for a Gas Town role.
func RoleBadge(role string) string {
	return lipgloss.NewStyle().
		Foreground(RoleColor(role)).
		Bold(true).
		Render(role)
}

// StateBadge returns a styled badge for an agent state.
func StateBadge(state string) string {
	sym := SymIdle
	switch state {
	case "working":
		sym = SymWorking
	case "backoff":
		sym = SymBackoff
	}
	return lipgloss.NewStyle().
		Foreground(AgentStateColor(state)).
		Render(sym + " " + state)
}
