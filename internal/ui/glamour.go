package ui

import (
	"fmt"
	"image/color"
	"reflect"

	"github.com/charmbracelet/glamour/ansi"
	"github.com/charmbracelet/glamour/styles"
	"github.com/muesli/termenv"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"
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

// NewMarkdownRenderer builds the markdown→ANSI pipeline for issue bodies,
// themed with MardiGrasGlamourStyle and wrapped at wordWrap columns.
//
// It mirrors glamour.NewTermRenderer with one deliberate difference: goldmark's
// raw-HTML parsers are left out. Issue bodies are plain text, so a tag-shaped
// span like "<your-name>" has to survive verbatim — but goldmark parses any
// "<" that looks like a tag as raw HTML, and glamour's renderer sanitizes those
// nodes down to nothing, silently deleting the span (and, for an unclosed tag,
// every line up to the next ">"). Without the parsers no HTML node is ever
// produced, so the brackets stay ordinary text.
//
// Removing the parsers is what makes this correct rather than escaping the
// brackets around the render: any stand-in character is still a foreign
// character to the stages downstream of the parser, which read it as itself
// rather than as "<". Chroma flags it as a lexer error and paints it on a red
// background inside code fences, and its display width is not guaranteed to
// match, which shifts word-wrap arithmetic. Here the author's real bytes reach
// chroma and the wrapper.
//
// glamour v1.0.0 offers no option for this — every TermRendererOption reaches
// only ansi.Options, and its goldmark instance is unexported — so the pipeline
// is assembled here instead.
func NewMarkdownRenderer(wordWrap int) goldmark.Markdown {
	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM, extension.DefinitionList),
		goldmark.WithParser(parser.NewParser(
			parser.WithBlockParsers(withoutHTML(parser.DefaultBlockParsers(), parser.NewHTMLBlockParser())...),
			parser.WithInlineParsers(withoutHTML(parser.DefaultInlineParsers(), parser.NewRawHTMLParser())...),
			parser.WithParagraphTransformers(parser.DefaultParagraphTransformers()...),
			parser.WithAutoHeadingID(),
		)),
	)
	// SetRenderer (rather than a renderer option) matches glamour: it replaces
	// the renderer wholesale after the extensions have registered their parsers.
	md.SetRenderer(renderer.NewRenderer(renderer.WithNodeRenderers(
		util.Prioritized(ansi.NewRenderer(ansi.Options{
			WordWrap:     wordWrap,
			ColorProfile: termenv.TrueColor,
			Styles:       MardiGrasGlamourStyle(),
		}), glamourRendererPriority),
	)))
	return md
}

// glamourRendererPriority mirrors glamour's unexported highPriority, the
// priority it gives its ANSI renderer so it outranks every extension renderer.
const glamourRendererPriority = 1000

// withoutHTML returns ps minus any entry of the same concrete type as exclude.
// Matching on type rather than priority keeps this working if goldmark renumbers
// its defaults, and keeps any parser goldmark adds later.
func withoutHTML(ps []util.PrioritizedValue, exclude any) []util.PrioritizedValue {
	excludeType := reflect.TypeOf(exclude)
	kept := make([]util.PrioritizedValue, 0, len(ps))
	for _, p := range ps {
		if reflect.TypeOf(p.Value) != excludeType {
			kept = append(kept, p)
		}
	}
	return kept
}
