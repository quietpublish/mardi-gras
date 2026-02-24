package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
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
	cLow, _ := colorful.Hex(string(DimGreen))
	cMid, _ := colorful.Hex(string(BrightGold))
	cHigh, _ := colorful.Hex(string(StateBackoff))

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

		style := lipgloss.NewStyle().Foreground(lipgloss.Color(c.Hex()))
		b.WriteString(style.Render(sparkBlocks[level]))
	}

	return b.String()
}

// HeatChar returns a single character with color indicating activity level.
// 0 events = dim dot, low = green, medium = gold, high = red.
func HeatChar(eventCount, maxCount int) string {
	if eventCount == 0 {
		return lipgloss.NewStyle().Foreground(Dim).Render("·")
	}

	cLow, _ := colorful.Hex(string(BrightGreen))
	cMid, _ := colorful.Hex(string(BrightGold))
	cHigh, _ := colorful.Hex(string(StateBackoff))

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

	return lipgloss.NewStyle().Foreground(lipgloss.Color(c.Hex())).Render(sym)
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
