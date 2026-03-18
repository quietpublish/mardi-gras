// Package tmux provides a status bar widget that renders parade issue counts
// in tmux-compatible color format.
package tmux

import (
	"fmt"

	"github.com/matt-wright86/mardi-gras/internal/data"
	"github.com/matt-wright86/mardi-gras/internal/ui"
)

// tmux 256-color equivalents for the parade theme.
const (
	colourRolling = "colour42"  // BrightGreen #2ECC71
	colourLinedUp = "colour220" // BrightGold  #FFD700
	colourStalled = "colour196" // Red         #E74C3C
	colourPassed  = "colour244" // Muted       #888888
	colourFleur   = "colour134" // Purple      #7B2D8E
)

// StatusLine returns a tmux-formatted status string showing parade counts.
// Output uses tmux #[fg=colourN] markup — no lipgloss dependency.
func StatusLine(groups map[data.ParadeStatus][]data.Issue) string {
	rolling := len(groups[data.ParadeRolling])
	linedUp := len(groups[data.ParadeLinedUp])
	stalled := len(groups[data.ParadeStalled])
	passed := len(groups[data.ParadePastTheStand])

	return fmt.Sprintf(
		"#[fg=%s]%s #[fg=%s]%d%s #[fg=%s]%d%s #[fg=%s]%d%s #[fg=%s]%d%s",
		colourFleur, ui.FleurDeLis,
		colourRolling, rolling, ui.SymRolling,
		colourLinedUp, linedUp, ui.SymLinedUp,
		colourStalled, stalled, ui.SymStalled,
		colourPassed, passed, ui.SymPassed,
	)
}
