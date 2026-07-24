package ui

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// Pre-built styles for the Mardi Gras theme. Assigned by rebuildStyles (via
// SetTheme) so a theme switch rebakes them from the active palette.
var (
	// Header
	HeaderStyle  lipgloss.Style
	HeaderCounts lipgloss.Style

	// Bead string decorations
	BeadStylePurple lipgloss.Style
	BeadStyleGold   lipgloss.Style
	BeadStyleGreen  lipgloss.Style

	// Section headers in parade list (used for title text color within borders)
	SectionRolling lipgloss.Style
	SectionLinedUp lipgloss.Style
	SectionStalled lipgloss.Style
	SectionPassed  lipgloss.Style

	// Pre-rendered status indicators
	StatusRollingStr string
	StatusLinedUpStr string
	StatusStalledStr string
	StatusPassedStr  string

	// Issue items in the list
	ItemNormal   lipgloss.Style
	ItemSelected lipgloss.Style
	ItemCursor   lipgloss.Style

	// Detail panel (right side)
	DetailBorder  lipgloss.Style
	DetailTitle   lipgloss.Style
	DetailLabel   lipgloss.Style
	DetailValue   lipgloss.Style
	DetailSection lipgloss.Style

	// Priority badge
	BadgePriority lipgloss.Style

	// Pre-rendered priority badges
	BadgeP0 string
	BadgeP1 string
	BadgeP2 string
	BadgeP3 string
	BadgeP4 string

	// Type badge
	BadgeType lipgloss.Style

	// Footer
	FooterStyle lipgloss.Style
	FooterKey   lipgloss.Style
	FooterDesc  lipgloss.Style

	// Dependency display
	DepBlocked     lipgloss.Style
	DepBlocks      lipgloss.Style
	DepMissing     lipgloss.Style
	DepResolved    lipgloss.Style
	DepNonBlocking lipgloss.Style

	// Due date badges
	OverdueBadge  lipgloss.Style
	DueSoonBadge  lipgloss.Style
	DeferredStyle lipgloss.Style

	// Rich dependency styles
	DepRelated    lipgloss.Style
	DepDuplicates lipgloss.Style
	DepSupersedes lipgloss.Style

	AgentBadge  lipgloss.Style
	ConvoyBadge lipgloss.Style
	GasTownTag  lipgloss.Style

	// Gas Town panel
	GasTownBorder lipgloss.Style
	GasTownTitle  lipgloss.Style
	GasTownLabel  lipgloss.Style
	GasTownValue  lipgloss.Style
	GasTownHint   lipgloss.Style

	FooterSource lipgloss.Style

	// Molecule step styles
	MolStepDone    lipgloss.Style
	MolStepActive  lipgloss.Style
	MolStepReady   lipgloss.Style
	MolStepBlocked lipgloss.Style
	MolTierLabel   lipgloss.Style
	MolDAGFlow     lipgloss.Style
	MolCritical    lipgloss.Style

	// Metadata fields
	MetaFieldName    lipgloss.Style
	MetaFieldNameDim lipgloss.Style
	MetaFieldType    lipgloss.Style
	MetaFieldValue   lipgloss.Style
	MetaRequired     lipgloss.Style

	// Filter Input
	InputPrompt lipgloss.Style
	InputText   lipgloss.Style
	InputCursor lipgloss.Style

	// Help Overlay
	HelpOverlayBg lipgloss.Style
	HelpTitle     lipgloss.Style
	HelpSubtitle  lipgloss.Style
	HelpSection   lipgloss.Style
	HelpKey       lipgloss.Style
	HelpDesc      lipgloss.Style
	HelpHint      lipgloss.Style

	// Toast notifications
	ToastInfo    lipgloss.Style
	ToastSuccess lipgloss.Style
	ToastWarn    lipgloss.Style
	ToastError   lipgloss.Style

	matchStyle lipgloss.Style
)

// bgSequence returns the raw SGR that sets bg as the background color.
func bgSequence(bg color.Color) string {
	r, g, b, _ := bg.RGBA()
	return fmt.Sprintf("\x1b[48;2;%d;%d;%dm", r>>8, g>>8, b>>8)
}

// FillBackground paints bg beneath already-styled content. Inner spans end
// with an SGR reset that would otherwise punch terminal-default holes in the
// fill — invisible on matching backgrounds, visible artifacts everywhere else
// — so the background is applied up front and re-asserted after every reset.
func FillBackground(content string, bg color.Color) string {
	seq := bgSequence(bg)
	for _, reset := range []string{"\x1b[0m", "\x1b[m"} {
		content = strings.ReplaceAll(content, reset, reset+seq)
	}
	return seq + content + "\x1b[m"
}

// SelectedRow truncates and pads a styled row to width, then paints the
// selection background beneath the whole row. Use for cursor rows in lists —
// applying a Background style to a row with inner spans leaves holes (and
// Width-wrapping overflows), which this avoids.
func SelectedRow(row string, width int) string {
	row = ansi.Truncate(row, width, "")
	if pad := width - lipgloss.Width(row); pad > 0 {
		row += strings.Repeat(" ", pad)
	}
	return FillBackground(row, DimPurple)
}

// OverlayBox renders content inside the shared overlay box (help, command
// palette, dialogs), with the box background re-asserted across inner spans.
func OverlayBox(content string, width int) string {
	return HelpOverlayBg.Width(width).Render(FillBackground(content, HelpBg))
}

// rebuildStyles bakes the active palette into the exported styles. Called by
// SetTheme after the palette vars are assigned.
func rebuildStyles() {
	// Header
	HeaderStyle = lipgloss.NewStyle().
		Bold(true).
		Background(DimPurple).
		Padding(0, 1)

	HeaderCounts = lipgloss.NewStyle().
		Foreground(Light)

	// Bead string decorations
	BeadStylePurple = lipgloss.NewStyle().Foreground(Purple)
	BeadStyleGold = lipgloss.NewStyle().Foreground(Gold)
	BeadStyleGreen = lipgloss.NewStyle().Foreground(Green)

	// Section headers in parade list
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

	// Pre-rendered status indicators
	StatusRollingStr = lipgloss.NewStyle().Foreground(StatusRolling).Render(SymRolling)
	StatusLinedUpStr = lipgloss.NewStyle().Foreground(StatusLinedUp).Render(SymLinedUp)
	StatusStalledStr = lipgloss.NewStyle().Foreground(StatusStalled).Render(SymStalled)
	StatusPassedStr = lipgloss.NewStyle().Foreground(StatusPassed).Render(SymPassed)

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

	// Pre-rendered priority badges
	BadgeP0 = BadgePriority.Foreground(PrioP0).Render("P0")
	BadgeP1 = BadgePriority.Foreground(PrioP1).Render("P1")
	BadgeP2 = BadgePriority.Foreground(PrioP2).Render("P2")
	BadgeP3 = BadgePriority.Foreground(PrioP3).Render("P3")
	BadgeP4 = BadgePriority.Foreground(PrioP4).Render("P4")

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

	AgentBadge = lipgloss.NewStyle().Foreground(StatusAgent).Bold(true)
	ConvoyBadge = lipgloss.NewStyle().Foreground(StatusConvoy).Bold(true)
	GasTownTag = lipgloss.NewStyle().Foreground(BrightPurple).Italic(true)

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

	GasTownHint = lipgloss.NewStyle().
		Foreground(Dim).
		MarginTop(1)

	FooterSource = lipgloss.NewStyle().
		Foreground(Muted)

	// Molecule step styles
	MolStepDone = lipgloss.NewStyle().
		Foreground(BrightGreen)

	MolStepActive = lipgloss.NewStyle().
		Foreground(BrightGold).
		Bold(true)

	MolStepReady = lipgloss.NewStyle().
		Foreground(Light)

	MolStepBlocked = lipgloss.NewStyle().
		Foreground(StatusStalled)

	MolTierLabel = lipgloss.NewStyle().
		Foreground(Dim).
		Italic(true)

	MolDAGFlow = lipgloss.NewStyle().
		Foreground(Dim)

	MolCritical = lipgloss.NewStyle().
		Foreground(BrightGold).
		Bold(true)

	// Metadata fields
	MetaFieldName = lipgloss.NewStyle().
		Foreground(Light)

	MetaFieldNameDim = lipgloss.NewStyle().
		Foreground(Muted)

	MetaFieldType = lipgloss.NewStyle().
		Foreground(Muted)

	MetaFieldValue = lipgloss.NewStyle().
		Foreground(BrightGreen)

	MetaRequired = lipgloss.NewStyle().
		Foreground(StatusStalled)

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
		Background(HelpBg).
		Padding(1, 2)

	HelpTitle = lipgloss.NewStyle().
		Bold(true).
		Foreground(BrightGold).
		Align(lipgloss.Center)

	HelpSubtitle = lipgloss.NewStyle().
		Foreground(HelpSubtitleFg).
		Align(lipgloss.Center)

	HelpSection = lipgloss.NewStyle().
		Bold(true).
		Foreground(BrightGreen).
		Underline(true)

	HelpKey = lipgloss.NewStyle().
		Bold(true).
		Foreground(Gold)

	HelpDesc = lipgloss.NewStyle().
		Foreground(HelpDescFg)

	HelpHint = lipgloss.NewStyle().
		Foreground(HelpHintFg).
		Align(lipgloss.Center)

	// Toast notifications
	ToastInfo = lipgloss.NewStyle().
		Foreground(Light).
		Background(DimPurple).
		Padding(0, 1)

	ToastSuccess = lipgloss.NewStyle().
		Foreground(ToastAccentFg).
		Background(BrightGreen).
		Bold(true).
		Padding(0, 1)

	ToastWarn = lipgloss.NewStyle().
		Foreground(ToastAccentFg).
		Background(BrightGold).
		Bold(true).
		Padding(0, 1)

	ToastError = lipgloss.NewStyle().
		Foreground(ToastErrorFg).
		Background(StatusStalled).
		Bold(true).
		Padding(0, 1)

	matchStyle = lipgloss.NewStyle().Foreground(BrightGold).Bold(true).Underline(true)
}

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
	case "spawning":
		sym = SymSpawning
	case "backoff", "degraded":
		sym = SymBackoff
	case "stuck":
		sym = SymStuck
	case "awaiting-gate":
		sym = SymGate
	case "fix_needed":
		sym = SymFixNeeded
	case "patrolling":
		sym = SymPatrolling
	case "paused", "muted":
		sym = SymPaused
	}
	return lipgloss.NewStyle().
		Foreground(AgentStateColor(state)).
		Render(sym + " " + state)
}

// SectionDivider renders a btop-style section divider: ── ⚜ TITLE ──────────
// When focused, the fleur-de-lis glows bright gold. The divider carries no
// cursor glyph — rows do — so only one ">" is ever visible (audit #14).
func SectionDivider(title string, width int, focused bool) string {
	usedWidth := 5 + len([]rune(title)) + 1
	trailWidth := max(width-usedWidth, 3)
	trail := strings.Repeat(BoxHorizontal, trailWidth)

	ruleStyle := lipgloss.NewStyle().Foreground(Dim)
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(BrightGold)

	fleurColor := DimGold
	if focused {
		fleurColor = BrightGold
	}
	fleurStyle := lipgloss.NewStyle().Foreground(fleurColor)

	return "\n" +
		ruleStyle.Render(BoxHorizontal+BoxHorizontal+" ") +
		fleurStyle.Render(FleurDeLis) + " " +
		titleStyle.Render(title) + " " +
		ruleStyle.Render(trail)
}

// HighlightMatches renders a string with matched character positions highlighted.
// Matched characters are rendered in bright gold bold; others use default style.
func HighlightMatches(text string, indices []int, maxLen int) string {
	runes := []rune(text)
	if maxLen > 0 && len(runes) > maxLen {
		runes = runes[:maxLen]
	}

	matchSet := make(map[int]bool, len(indices))
	for _, idx := range indices {
		matchSet[idx] = true
	}

	var b strings.Builder
	for i, r := range runes {
		if matchSet[i] {
			b.WriteString(matchStyle.Render(string(r)))
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}
