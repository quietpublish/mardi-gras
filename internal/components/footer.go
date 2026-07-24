package components

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/matt-wright86/mardi-gras/internal/data"
	"github.com/matt-wright86/mardi-gras/internal/ui"
)

// FooterBinding is a key-description pair.
type FooterBinding struct {
	Key  string
	Desc string
}

// Footer renders the keybinding help bar.
type Footer struct {
	Width        int
	Bindings     []FooterBinding
	SourcePath   string
	LastRefresh  time.Time
	PathExplicit bool
	SourceMode   data.SourceMode
	BeadsContext *data.BeadsContext
	SourceHealth *data.SourceHealth
	Focus        bool // focus mode active — show a persistent badge (audit #12)
}

// FooterModeChip renders a small gold mode indicator for bottom-bar overlays.
func FooterModeChip(label string) string {
	return ui.FooterKey.Render(label)
}

// ParadeBindings are the default keybindings for the parade view.
var ParadeBindings = []FooterBinding{
	{Key: "?", Desc: "help"},
	{Key: ":", Desc: "palette"},
	{Key: "/", Desc: "filter"},
	{Key: "j/k", Desc: "navigate"},
	{Key: "1/2/3", Desc: "status"},
	{Key: "b", Desc: "branch"},
	{Key: "N", Desc: "new"},
	{Key: "a", Desc: "agent"},
	{Key: "q", Desc: "quit"},
}

// DetailBindings are keybindings when the detail pane is focused.
var DetailBindings = []FooterBinding{
	{Key: "?", Desc: "help"},
	{Key: "/", Desc: "filter"},
	{Key: "j/k", Desc: "scroll"},
	{Key: "tab", Desc: "switch pane"},
	{Key: "esc", Desc: "back"},
	{Key: "a", Desc: "agent"},
	{Key: "A", Desc: "kill agent"},
	{Key: "q", Desc: "quit"},
}

// View renders the footer.
func (f Footer) View() string {
	// Build keybindings section (right side)
	var parts []string
	for _, b := range f.Bindings {
		key := ui.FooterKey.Render(b.Key)
		desc := ui.FooterDesc.Render(b.Desc)
		parts = append(parts, key+" "+desc)
	}

	// Build source info (left side)
	sourceInfo := ""
	if f.SourceMode == data.SourceCLI || f.SourcePath != "" {
		name := "bd list"
		mode := "(cli)"
		if f.SourceMode != data.SourceCLI {
			name = filepath.Base(f.SourcePath)
			mode = "(legacy)"
			if f.PathExplicit {
				mode = "(--path)"
			}
		}
		age := "?"
		if !f.LastRefresh.IsZero() {
			age = data.RelativeAge(time.Since(f.LastRefresh))
		}
		contextInfo := ""
		if f.BeadsContext != nil && f.BeadsContext.Database != "" {
			contextInfo = f.BeadsContext.Database
			if f.BeadsContext.Backend != "" {
				contextInfo += "/" + f.BeadsContext.Backend
			}
			if f.BeadsContext.BdVersion != "" {
				contextInfo += " v" + f.BeadsContext.BdVersion
			}
			contextInfo = " · " + contextInfo
		}

		// Override rendering when source is in a degraded or fallback state.
		if f.SourceHealth != nil && f.SourceHealth.IsDegraded() {
			sourceInfo = f.renderHealthState(age)
		} else {
			sourceInfo = ui.FooterSource.Render(fmt.Sprintf("%s %s · %s%s", name, mode, age, contextInfo))
		}
	}

	// Persistent focus-mode badge: without it the only signal is a transient
	// toast (audit #12).
	if f.Focus {
		badge := ui.FooterKey.Render(ui.FleurDeLis + " FOCUS")
		if sourceInfo != "" {
			sourceInfo = badge + ui.FooterSource.Render(" · ") + sourceInfo
		} else {
			sourceInfo = badge
		}
	}

	if sourceInfo != "" {
		// Lay out: source left, keybindings right. Drop whole trailing hint
		// chips when width is short — clipping one mid-keyword reads as a
		// rendering bug (audit #11).
		sourceW := lipgloss.Width(sourceInfo)
		keybindings := fitBindings(parts, f.Width-sourceW-3)
		keysW := lipgloss.Width(keybindings)
		gap := max(f.Width-sourceW-keysW-2, 1) // 2 for padding
		content := sourceInfo + strings.Repeat(" ", gap) + keybindings
		return ui.FooterStyle.Width(f.Width).Render(content)
	}

	return ui.FooterStyle.Width(f.Width).Render(fitBindings(parts, f.Width-2))
}

// fitBindings joins hint chips, dropping whole chips from the end (with a "…"
// marker) until the row fits the available width.
func fitBindings(parts []string, avail int) string {
	joined := strings.Join(parts, "  ")
	if lipgloss.Width(joined) <= avail || len(parts) == 0 {
		return joined
	}
	ellipsis := ui.FooterSource.Render("…")
	for len(parts) > 1 {
		parts = parts[:len(parts)-1]
		joined = strings.Join(parts, "  ") + " " + ellipsis
		if lipgloss.Width(joined) <= avail {
			return joined
		}
	}
	return ellipsis
}

// renderHealthState builds the source info string for degraded/fallback states.
// It applies amber or red coloring based on staleness level.
func (f Footer) renderHealthState(age string) string {
	h := f.SourceHealth
	staleness := h.StalenessAge()
	ageStr := age
	if staleness != 0 {
		ageStr = data.RelativeAge(staleness)
	}

	var label string
	switch {
	case h.InFallback():
		label = fmt.Sprintf("issues.jsonl (fallback, bd down) · %s", ageStr)
	default:
		label = fmt.Sprintf("bd list (degraded, last success %s)", ageStr)
	}

	style := ui.FooterSource
	switch h.StalenessLevel() {
	case 1:
		style = lipgloss.NewStyle().Foreground(ui.StateStuck) // amber
	case 2:
		style = lipgloss.NewStyle().Foreground(ui.StatusStalled) // red
	}
	return style.Render(label)
}

// NewFooter creates a footer with the given width and pane focus.
func NewFooter(width int, detailFocused, hasGasTown bool) Footer {
	bindings := ParadeBindings
	if detailFocused {
		bindings = DetailBindings
	}
	if hasGasTown {
		gtBindings := []FooterBinding{
			{Key: "^g", Desc: "gas town"},
			{Key: "p", Desc: "problems"},
			{Key: "s", Desc: "sling"},
			{Key: "n", Desc: "nudge"},
		}
		bindings = insertBefore(bindings, "q", gtBindings...)
	}
	return Footer{Width: width, Bindings: bindings}
}

// insertBefore inserts extra bindings before the binding with the given key.
func insertBefore(bindings []FooterBinding, key string, extra ...FooterBinding) []FooterBinding {
	for i, b := range bindings {
		if b.Key != key {
			continue
		}
		result := make([]FooterBinding, 0, len(bindings)+len(extra))
		result = append(result, bindings[:i]...)
		result = append(result, extra...)
		result = append(result, bindings[i:]...)
		return result
	}
	return append(bindings, extra...)
}

// BulkFooter renders the footer bar shown during multi-select.
func BulkFooter(width, count int, hasGasTown bool) string {
	label := ui.FooterKey.Render(fmt.Sprintf(" %d selected: ", count))
	bindings := []FooterBinding{
		{Key: "1", Desc: "in_progress"},
		{Key: "2", Desc: "open"},
		{Key: "3", Desc: "close"},
	}
	if hasGasTown {
		bindings = append(bindings,
			FooterBinding{Key: "a", Desc: "sling"},
			FooterBinding{Key: "s", Desc: "sling+formula"},
		)
	}
	bindings = append(bindings, FooterBinding{Key: "X", Desc: "clear"})
	var parts []string
	for _, b := range bindings {
		key := ui.FooterKey.Render(b.Key)
		desc := ui.FooterDesc.Render(b.Desc)
		parts = append(parts, key+" "+desc)
	}
	content := label + strings.Join(parts, "  ")
	return ui.FooterStyle.Width(width).Render(content)
}

// Divider returns a full-width horizontal divider line.
func Divider(width int) string {
	return lipgloss.NewStyle().
		Foreground(ui.DimPurple).
		Width(width).
		Render(strings.Repeat(ui.DividerH, width))
}
