package ui

import (
	"math"
	"strings"

	"image/color"

	"charm.land/lipgloss/v2"
	"github.com/lucasb-eyer/go-colorful"
)

// toColorful converts a color.Color to a go-colorful Color for gradient blending.
func toColorful(c color.Color) colorful.Color {
	cf, _ := colorful.MakeColor(c)
	return cf
}

type charKey struct {
	r   rune
	hex string
}

var (
	styleCache = make(map[string]lipgloss.Style)
	charCache  = make(map[charKey]string)
)

func getCachedChar(r rune, hex string) string {
	key := charKey{r, hex}
	if s, ok := charCache[key]; ok {
		return s
	}
	style, ok := styleCache[hex]
	if !ok {
		style = lipgloss.NewStyle().Foreground(lipgloss.Color(hex))
		styleCache[hex] = style
	}
	res := style.Render(string(r))
	charCache[key] = res
	return res
}

// ApplyMardiGrasGradient applies a smooth Purple -> Gold -> Green gradient to the text.
func ApplyMardiGrasGradient(text string) string {
	runes := []rune(text)
	width := len(runes)
	if width == 0 {
		return ""
	}

	c1 := toColorful(Purple)
	c2 := toColorful(Gold)
	c3 := toColorful(Green)

	var b strings.Builder
	for i, r := range runes {
		t := 0.0
		if width > 1 {
			t = float64(i) / float64(width-1)
		}
		var c colorful.Color
		if t < 0.5 {
			c = c1.BlendLuv(c2, t*2)
		} else {
			c = c2.BlendLuv(c3, (t-0.5)*2)
		}

		b.WriteString(getCachedChar(r, c.Hex()))
	}
	return b.String()
}

// ApplyShimmerGradient applies the Mardi Gras gradient with a phase offset that shifts over time.
func ApplyShimmerGradient(text string, offset float64) string {
	runes := []rune(text)
	width := len(runes)
	if width == 0 {
		return ""
	}

	c1 := toColorful(Purple)
	c2 := toColorful(Gold)
	c3 := toColorful(Green)

	var b strings.Builder
	for i, r := range runes {
		t := 0.0
		if width > 1 {
			t = float64(i)/float64(width-1) + offset
		}
		t -= math.Floor(t)

		var c colorful.Color
		switch {
		case t < 1.0/3:
			c = c1.BlendLuv(c2, t*3)
		case t < 2.0/3:
			c = c2.BlendLuv(c3, (t-1.0/3)*3)
		default:
			c = c3.BlendLuv(c1, (t-2.0/3)*3)
		}

		sparkle := 0.8 + 0.2*math.Sin(float64(i)*0.7+offset*math.Pi*6)
		h, s, l := c.Hsl()
		c = colorful.Hsl(h, s, l*sparkle)

		b.WriteString(getCachedChar(r, c.Hex()))
	}
	return b.String()
}

// Gradient is a pre-computed array of 101 styles (0-100%) for smooth color transitions.
type Gradient [101]lipgloss.Style

// NewGradient creates a 3-point gradient: start (0%) → mid (50%) → end (100%).
// Uses Luv color space blending for perceptually uniform transitions.
func NewGradient(start, mid, end color.Color) Gradient {
	s := toColorful(start)
	m := toColorful(mid)
	e := toColorful(end)
	var g Gradient
	for i := range 101 {
		var c colorful.Color
		if i <= 50 {
			c = s.BlendLuv(m, float64(i)/50.0)
		} else {
			c = m.BlendLuv(e, float64(i-50)/50.0)
		}
		g[i] = lipgloss.NewStyle().Foreground(lipgloss.Color(c.Hex()))
	}
	return g
}

// NewGradient2 creates a 2-point gradient: start (0%) → end (100%).
func NewGradient2(start, end color.Color) Gradient {
	s := toColorful(start)
	e := toColorful(end)
	var g Gradient
	for i := range 101 {
		c := s.BlendLuv(e, float64(i)/100.0)
		g[i] = lipgloss.NewStyle().Foreground(lipgloss.Color(c.Hex()))
	}
	return g
}

// At returns the style at the given percentage (clamped to 0-100).
func (g Gradient) At(pct int) lipgloss.Style {
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	return g[pct]
}

// GradientBar renders a progress bar where each filled block is colored
// by its position along the gradient. Unfilled portion uses dim blocks.
func GradientBar(pct float64, width int, g Gradient) string {
	if width <= 0 {
		return ""
	}
	filled := int(pct * float64(width) / 100.0)
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}

	dimStyle := lipgloss.NewStyle().Foreground(Dim)
	var b strings.Builder
	for i := range filled {
		step := i * 100 / width
		b.WriteString(g.At(step).Render("█"))
	}
	for range width - filled {
		b.WriteString(dimStyle.Render("░"))
	}
	return b.String()
}

// Pre-built gradients for common use cases.
var (
	// GradientProgress: green → gold → red (for progress bars, budgets).
	GradientProgress = NewGradient(BrightGreen, BrightGold, lipgloss.Color("#E74C3C"))

	// GradientHeat: green → orange → red (for age/staleness).
	GradientHeat = NewGradient(BrightGreen, lipgloss.Color("#E67E22"), lipgloss.Color("#E74C3C"))

	// GradientPurpleGold: purple → gold (Mardi Gras themed, for selection proximity).
	GradientPurpleGold = NewGradient2(DimPurple, BrightGold)

	// GradientFade: bright → dim (for list item positional fading).
	GradientFade = NewGradient2(White, Dim)
)

// ApplyPartialMardiGrasGradient applies the gradient as if the text was `totalLength` characters long,
// ensuring a partial progress bar maps to the correct segment of the full color spectrum.
func ApplyPartialMardiGrasGradient(text string, totalLength int) string {
	runes := []rune(text)
	if totalLength == 0 {
		return ""
	}

	c1 := toColorful(Purple)
	c2 := toColorful(Gold)
	c3 := toColorful(Green)

	var b strings.Builder
	for i, r := range runes {
		t := 0.0
		if totalLength > 1 {
			t = float64(i) / float64(totalLength-1)
		}
		var c colorful.Color
		if t < 0.5 {
			c = c1.BlendLuv(c2, t*2)
		} else {
			c = c2.BlendLuv(c3, (t-0.5)*2)
		}

		s := lipgloss.NewStyle().Foreground(lipgloss.Color(c.Hex()))
		b.WriteString(s.Render(string(r)))
	}
	return b.String()
}
