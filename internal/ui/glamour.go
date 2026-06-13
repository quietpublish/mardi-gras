package ui

import (
	"github.com/charmbracelet/glamour/ansi"
	"github.com/charmbracelet/glamour/styles"
)

func glamourStr(s string) *string { return &s }
func glamourBool(b bool) *bool    { return &b }

// MardiGrasGlamourStyle returns a glamour StyleConfig themed to mg's brand
// palette: a gold-on-purple H1 banner, bright-purple subheadings, gold
// emphasis and links, and green inline code. It clones the built-in dark
// style — so code-block syntax highlighting, spacing, and table rules stay
// sensible — then recolors only the elements mg cares about.
//
// The clone is a value copy of the package var; every override replaces the
// inner pointer rather than mutating through it, so the shared DarkStyleConfig
// is never touched.
//
// Hex literals here mirror the brand constants in theme.go. lipgloss/v2 colors
// are color.Color interface values (not string-convertible), and glamour wants
// hex strings, so the values are repeated here rather than derived.
func MardiGrasGlamourStyle() ansi.StyleConfig {
	c := styles.DarkStyleConfig

	// H1 banner: bright gold on purple — the Mardi Gras look. (BrightGold/Purple)
	c.H1.Color = glamourStr("#FFD700")
	c.H1.BackgroundColor = glamourStr("#7B2D8E")
	c.H1.Bold = glamourBool(true)

	// Subheadings (H2–H5 inherit Heading; H6 sets its own color) in bright purple.
	c.Heading.Color = glamourStr("#9B59B6") // BrightPurple
	c.Heading.Bold = glamourBool(true)
	c.H6.Color = glamourStr("#9B59B6") // BrightPurple
	c.H6.Bold = glamourBool(true)

	// Emphasis and links in gold.
	c.Emph.Color = glamourStr("#F5C518")   // Gold
	c.Strong.Color = glamourStr("#FFD700") // BrightGold
	c.Strong.Bold = glamourBool(true)
	c.Link.Color = glamourStr("#F5C518") // Gold
	c.Link.Underline = glamourBool(true)
	c.LinkText.Color = glamourStr("#F5C518") // Gold

	// Inline code in bright green; list markers in purple.
	c.Code.Color = glamourStr("#2ECC71")        // BrightGreen
	c.Item.Color = glamourStr("#9B59B6")        // BrightPurple
	c.Enumeration.Color = glamourStr("#9B59B6") // BrightPurple

	return c
}
