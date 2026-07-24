package ui

import (
	"fmt"
	"image/color"

	"github.com/charmbracelet/glamour/ansi"
	"github.com/charmbracelet/glamour/styles"
)

func glamourStr(s string) *string { return &s }
func glamourBool(b bool) *bool    { return &b }

// glamourHex converts a palette color to the hex-string pointer glamour wants.
// Every palette var is hex-constructed, so the round-trip is exact.
func glamourHex(c color.Color) *string {
	r, g, b, _ := c.RGBA()
	s := fmt.Sprintf("#%02X%02X%02X", r>>8, g>>8, b>>8)
	return &s
}

// MardiGrasGlamourStyle returns a glamour StyleConfig themed to mg's brand
// palette: a gold-on-purple H1 banner, bright-purple subheadings, gold
// emphasis and links, and green inline code. It clones the built-in style
// matching the active theme — so code-block syntax highlighting, spacing, and
// table rules stay sensible — then recolors only the elements mg cares about,
// deriving each override from the active palette vars.
//
// The clone is a value copy of the package var; every override replaces the
// inner pointer rather than mutating through it, so the shared style config
// is never touched.
func MardiGrasGlamourStyle() ansi.StyleConfig {
	c := styles.DarkStyleConfig
	if isLight() {
		c = styles.LightStyleConfig
	}

	// H1 banner: bright gold on purple — the Mardi Gras look. Brand-fixed in
	// both themes: the banner carries its own purple background.
	c.H1.Color = glamourStr("#FFD700")
	c.H1.BackgroundColor = glamourStr("#7B2D8E")
	c.H1.Bold = glamourBool(true)

	// Subheadings (H2–H5 inherit Heading; H6 sets its own color) in bright
	// purple. Literal "##" prefixes are replaced with a slim bar so heading
	// levels read as hierarchy, not raw markdown (audit #15).
	c.Heading.Color = glamourHex(BrightPurple)
	c.Heading.Bold = glamourBool(true)
	c.H2.Prefix = "▍ "
	c.H3.Prefix = "▎ "
	c.H4.Prefix = "▏ "
	c.H6.Color = glamourHex(BrightPurple)
	c.H6.Bold = glamourBool(true)

	// Emphasis and links in gold.
	c.Emph.Color = glamourHex(Gold)
	c.Strong.Color = glamourHex(BrightGold)
	c.Strong.Bold = glamourBool(true)
	c.Link.Color = glamourHex(Gold)
	c.Link.Underline = glamourBool(true)
	c.LinkText.Color = glamourHex(Gold)

	// Inline code in bright green; list markers in purple.
	c.Code.Color = glamourHex(BrightGreen)
	c.Item.Color = glamourHex(BrightPurple)
	c.Enumeration.Color = glamourHex(BrightPurple)

	return c
}
