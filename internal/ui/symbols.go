package ui

import (
	"fmt"
	"strings"
)

// Unicode symbols for the Mardi Gras theme.
const (
	FleurDeLis = "⚜"

	// Status indicators
	SymRolling = "●"
	SymLinedUp = "♪"
	SymStalled = "⊘"
	SymPassed  = "✓"

	// Bead string
	BeadRound   = "●"
	BeadDiamond = "◆"
	BeadDash    = "─"

	// Navigation
	Cursor    = ">"
	Expanded  = "▼"
	Collapsed = "▶"

	// Dependencies
	DepArrow       = "→"
	DepTree        = "└─"
	SymMissing     = "!"
	SymResolved    = "✓" // alias of SymPassed
	SymNonBlocking = "·"
	SymNextArrow   = "next →"
	SymAgent       = "⚡"
	SymConvoy      = "◐"
	SymMail        = "✉"
	SymSling       = "➤"
	SymChanged     = "◈"
	SymSelected    = "◉"
	SymUnselected  = "○"

	// Due dates
	SymOverdue  = "▲"
	SymDeferred = "⏸"
	SymDueDate  = "◷"

	// Rich dependency types
	SymRelated    = "↔"
	SymDuplicates = "⊜"
	SymSupersedes = "⇢"

	// Section borders (rounded)
	BoxTopLeft     = "╭"
	BoxTopRight    = "╮"
	BoxBottomLeft  = "╰"
	BoxBottomRight = "╯"
	BoxHorizontal  = "─"
	BoxVertical    = "│"

	// Separators
	DividerH = "━"
	DividerV = "│"
	CornerTL = "┯"
	CornerBL = "┷"

	// Gas Town panel
	SymIdle          = "○"
	SymWorking       = "●"
	SymBackoff       = "◌"
	SymStuck         = "⚠" // alias of SymWarning — agent requesting help
	SymSpawning      = "◐" // half-filled — session starting
	SymGate          = "◷" // alias of SymDueDate — waiting on gate
	SymPaused        = "⏸" // alias of SymDeferred — intentionally suspended
	SymFixNeeded     = "🔧" // review feedback — needs rework
	SymProgress      = "█"
	SymProgressEmpty = "░"
	SymDog           = "🐕"
	SymTown          = "⛽"

	// Problems
	SymWarning = "⚠"

	// Molecule steps
	SymStepDone    = "✓"
	SymStepActive  = "●"
	SymStepReady   = "○"
	SymStepBlocked = "⊘"
	SymStepSkipped = "─"
	SymTierLine    = "│"

	// DAG flow connectors
	SymDAGFlow   = "│"
	SymDAGBranch = "┌"
	SymDAGFork   = "├"
	SymDAGJoin   = "└"
	SymDAGArrow  = "↓"

	// HOP quality indicators
	SymStar      = "★"
	SymStarEmpty = "☆"
	SymCrystal   = "◆"
	SymEphemeral = "◇"
	SymValidator = "⚖"
)

// superscriptDigits maps 0-9 to their Unicode superscript equivalents.
var superscriptDigits = [10]string{"⁰", "¹", "²", "³", "⁴", "⁵", "⁶", "⁷", "⁸", "⁹"}

// Superscript converts a non-negative integer to superscript Unicode digits.
func Superscript(n int) string {
	if n < 0 {
		n = 0
	}
	if n < 10 {
		return superscriptDigits[n]
	}
	var b strings.Builder
	for _, d := range fmt.Appendf(nil, "%d", n) {
		b.WriteString(superscriptDigits[d-'0'])
	}
	return b.String()
}
