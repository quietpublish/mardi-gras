package ui

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
	SymProgress      = "█"
	SymProgressEmpty = "░"
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
