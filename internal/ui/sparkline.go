package ui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/lucasb-eyer/go-colorful"
)

// Block characters for sparkline rendering (8 levels, bottom to top).
var sparkBlocks = []string{"▁", "▂", "▃", "▄", "▅", "▆", "▇", "█"}

// RenderSparkline renders a compact sparkline from integer values.
// Each value maps to one block character (8 height levels).
// Colors follow a green→gold→red gradient based on value intensity.
func RenderSparkline(values []int, width int) string {
	if len(values) == 0 || width <= 0 {
		return strings.Repeat(" ", width)
	}

	// Find max value for scaling
	maxVal := 0
	for _, v := range values {
		if v > maxVal {
			maxVal = v
		}
	}
	if maxVal == 0 {
		return lipgloss.NewStyle().Foreground(Dim).Render(
			strings.Repeat("▁", min(len(values), width)))
	}

	// Gradient: green (low activity) → gold (medium) → red (high)
	cLow := toColorful(DimGreen)
	cMid := toColorful(BrightGold)
	cHigh := toColorful(StateBackoff)

	var b strings.Builder
	n := min(len(values), width)
	for i := 0; i < n; i++ {
		v := values[i]
		// Scale to 0-7 for block selection
		level := 0
		if maxVal > 0 && v > 0 {
			level = min(v*7/maxVal, 7)
		}

		// Color based on intensity (0.0 = green, 1.0 = red)
		t := float64(v) / float64(maxVal)
		var c colorful.Color
		if t < 0.5 {
			c = cLow.BlendLuv(cMid, t*2)
		} else {
			c = cMid.BlendLuv(cHigh, (t-0.5)*2)
		}

		b.WriteString(getCachedChar([]rune(sparkBlocks[level])[0], c.Hex()))
	}

	return b.String()
}

// HeatChar returns a single character with color indicating activity level.
// 0 events = dim dot, low = green, medium = gold, high = red.
func HeatChar(eventCount, maxCount int) string {
	if eventCount == 0 {
		return lipgloss.NewStyle().Foreground(Dim).Render("·")
	}

	cLow := toColorful(BrightGreen)
	cMid := toColorful(BrightGold)
	cHigh := toColorful(StateBackoff)

	t := 0.0
	if maxCount > 0 {
		t = float64(eventCount) / float64(maxCount)
	}

	var c colorful.Color
	if t < 0.5 {
		c = cLow.BlendLuv(cMid, t*2)
	} else {
		c = cMid.BlendLuv(cHigh, (t-0.5)*2)
	}

	sym := "▪"
	if t > 0.7 {
		sym = "▮"
	}

	return getCachedChar([]rune(sym)[0], c.Hex())
}

// brailleTable maps pairs of vertical values (left 0-4, right 0-4) to braille chars.
// Each cell encodes two data points, doubling horizontal resolution.
var brailleTable = [5][5]rune{
	{' ', '⢀', '⢠', '⢰', '⢸'},
	{'⡀', '⣀', '⣠', '⣰', '⣸'},
	{'⡄', '⣄', '⣤', '⣴', '⣼'},
	{'⡆', '⣆', '⣦', '⣶', '⣾'},
	{'⡇', '⣇', '⣧', '⣷', '⣿'},
}

// BrailleSparkline renders a compact sparkline using braille characters.
// Each character cell encodes two data points for double horizontal resolution.
// The style is applied uniformly; use a gradient externally for colored sparklines.
func BrailleSparkline(data []float64, width int, style lipgloss.Style) string {
	if len(data) == 0 || width <= 0 {
		return strings.Repeat(" ", width)
	}

	// Find max for normalization
	maxVal := 0.0
	for _, v := range data {
		if v > maxVal {
			maxVal = v
		}
	}
	if maxVal == 0 {
		return style.Render(strings.Repeat(" ", width))
	}

	// Normalize to 0-4 range
	norm := make([]int, len(data))
	for i, v := range data {
		norm[i] = int(v * 4.0 / maxVal)
		if norm[i] > 4 {
			norm[i] = 4
		}
	}

	// Pair up values into braille chars (each char = 2 data points)
	var b strings.Builder
	chars := 0
	for i := 0; i < len(norm)-1 && chars < width; i += 2 {
		b.WriteRune(brailleTable[norm[i]][norm[i+1]])
		chars++
	}
	// Handle odd trailing value
	if len(norm)%2 == 1 && chars < width {
		b.WriteRune(brailleTable[norm[len(norm)-1]][0])
		chars++
	}
	// Pad remaining width
	for chars < width {
		b.WriteRune(' ')
		chars++
	}

	return style.Render(b.String())
}

// MiniSparkline renders a compact 3-character activity indicator using block elements.
// Values should be recent activity counts (e.g. last 3 time periods).
// Returns empty string if all values are zero.
func MiniSparkline(values [3]int) string {
	maxVal := 0
	allZero := true
	for _, v := range values {
		if v > maxVal {
			maxVal = v
		}
		if v > 0 {
			allZero = false
		}
	}
	if allZero {
		return ""
	}

	cLow := toColorful(DimGreen)
	cHigh := toColorful(BrightGold)

	var b strings.Builder
	for _, v := range values {
		level := 0
		if maxVal > 0 && v > 0 {
			level = min(v*7/maxVal, 7)
		}
		t := float64(v) / float64(maxVal)
		c := cLow.BlendLuv(cHigh, t)
		style := lipgloss.NewStyle().Foreground(lipgloss.Color(c.Hex()))
		b.WriteString(style.Render(sparkBlocks[level]))
	}
	return b.String()
}

// DualSparkline renders two datasets stacked vertically in the same row.
// Top data uses upper blocks (▀), bottom uses lower blocks (▄), overlap uses (█).
// This doubles the information density — e.g. cost rate above, velocity below.
func DualSparkline(top, bottom []float64, width int, topStyle, bottomStyle lipgloss.Style) string {
	if width <= 0 {
		return ""
	}

	normalize := func(data []float64) []int {
		maxVal := 0.0
		for _, v := range data {
			if v > maxVal {
				maxVal = v
			}
		}
		result := make([]int, len(data))
		if maxVal == 0 {
			return result
		}
		for i, v := range data {
			result[i] = int(v * 2.0 / maxVal) // 0, 1, or 2
			if result[i] > 2 {
				result[i] = 2
			}
		}
		return result
	}

	normTop := normalize(top)
	normBot := normalize(bottom)

	var b strings.Builder
	for i := 0; i < width; i++ {
		t, bt := 0, 0
		if i < len(normTop) {
			t = normTop[i]
		}
		if i < len(normBot) {
			bt = normBot[i]
		}
		switch {
		case t > 0 && bt > 0:
			b.WriteString(topStyle.Render("█"))
		case t > 0:
			b.WriteString(topStyle.Render("▀"))
		case bt > 0:
			b.WriteString(bottomStyle.Render("▄"))
		default:
			b.WriteRune(' ')
		}
	}
	return b.String()
}

// ConvoyPipeline renders a compact convoy progress pipeline: ●─●─◐─○─○
// Each position represents a tracked issue: ● closed, ◐ in_progress, ○ open.
func ConvoyPipeline(statuses []string, maxWidth int) string {
	if len(statuses) == 0 {
		return ""
	}

	// If too many issues, show truncated with count
	n := len(statuses)
	if n > maxWidth/2 { // each node is 1 char + 1 connector
		n = maxWidth / 2
	}

	doneStyle := lipgloss.NewStyle().Foreground(BrightGreen)
	activeStyle := lipgloss.NewStyle().Foreground(BrightGold)
	openStyle := lipgloss.NewStyle().Foreground(Dim)
	connDone := lipgloss.NewStyle().Foreground(DimGreen).Render("─")
	connOpen := lipgloss.NewStyle().Foreground(Dim).Render("─")

	var b strings.Builder
	for i := 0; i < n; i++ {
		if i > 0 {
			// Connector color based on left node
			switch statuses[i-1] {
			case "closed":
				b.WriteString(connDone)
			default:
				b.WriteString(connOpen)
			}
		}
		switch statuses[i] {
		case "closed":
			b.WriteString(doneStyle.Render("●"))
		case "in_progress", "hooked":
			b.WriteString(activeStyle.Render("◐"))
		default:
			b.WriteString(openStyle.Render("○"))
		}
	}

	if n < len(statuses) {
		b.WriteString(lipgloss.NewStyle().Foreground(Muted).Render(
			fmt.Sprintf(" +%d", len(statuses)-n)))
	}

	return b.String()
}
