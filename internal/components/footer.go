package components

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
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
}

// ParadeBindings are the default keybindings for the parade view.
var ParadeBindings = []FooterBinding{
	{Key: "?", Desc: "help"},
	{Key: "/", Desc: "filter"},
	{Key: "j/k", Desc: "navigate"},
	{Key: "tab", Desc: "switch pane"},
	{Key: "c", Desc: "toggle closed"},
	{Key: "q", Desc: "quit"},
}

// DetailBindings are keybindings when the detail pane is focused.
var DetailBindings = []FooterBinding{
	{Key: "?", Desc: "help"},
	{Key: "/", Desc: "filter"},
	{Key: "j/k", Desc: "scroll"},
	{Key: "tab", Desc: "switch pane"},
	{Key: "esc", Desc: "back"},
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
	keybindings := strings.Join(parts, "  ")

	// Build source info (left side)
	sourceInfo := ""
	if f.SourcePath != "" {
		name := filepath.Base(f.SourcePath)
		mode := "(auto)"
		if f.PathExplicit {
			mode = "(--path)"
		}
		age := "?"
		if !f.LastRefresh.IsZero() {
			elapsed := time.Since(f.LastRefresh)
			switch {
			case elapsed < time.Minute:
				age = fmt.Sprintf("%ds ago", int(elapsed.Seconds()))
			case elapsed < time.Hour:
				age = fmt.Sprintf("%dm ago", int(elapsed.Minutes()))
			default:
				age = fmt.Sprintf("%dh ago", int(elapsed.Hours()))
			}
		}
		sourceInfo = ui.FooterSource.Render(fmt.Sprintf("%s %s · %s", name, mode, age))
	}

	if sourceInfo != "" {
		// Lay out: source left, keybindings right
		sourceW := lipgloss.Width(sourceInfo)
		keysW := lipgloss.Width(keybindings)
		gap := f.Width - sourceW - keysW - 2 // 2 for padding
		if gap < 1 {
			gap = 1
		}
		content := sourceInfo + strings.Repeat(" ", gap) + keybindings
		return ui.FooterStyle.Width(f.Width).Render(content)
	}

	return ui.FooterStyle.Width(f.Width).Render(keybindings)
}

// NewFooter creates a footer with the given width and pane focus.
func NewFooter(width int, detailFocused bool) Footer {
	bindings := ParadeBindings
	if detailFocused {
		bindings = DetailBindings
	}
	return Footer{Width: width, Bindings: bindings}
}

// Divider returns a full-width horizontal divider line.
func Divider(width int) string {
	return lipgloss.NewStyle().
		Foreground(ui.DimPurple).
		Width(width).
		Render(strings.Repeat(ui.DividerH, width))
}
