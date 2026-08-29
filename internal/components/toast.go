package components

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/matt-wright86/mardi-gras/internal/ui"
)

// ToastLevel controls the style of a toast message.
type ToastLevel int

const (
	ToastInfo ToastLevel = iota
	ToastSuccess
	ToastWarn
	ToastError
)

// Toast is a transient notification message that auto-dismisses.
type Toast struct {
	Message   string
	Level     ToastLevel
	ExpiresAt time.Time
}

// ToastDismissMsg signals that the current toast should be cleared.
type ToastDismissMsg struct{}

// ShowToast creates a new toast and returns a dismiss command.
func ShowToast(message string, level ToastLevel, duration time.Duration) (Toast, tea.Cmd) {
	t := Toast{
		Message:   message,
		Level:     level,
		ExpiresAt: time.Now().Add(duration),
	}
	cmd := tea.Tick(duration, func(time.Time) tea.Msg {
		return ToastDismissMsg{}
	})
	return t, cmd
}

// View renders the toast bar.
//
// The toast occupies exactly one row — the divider line between the parade and
// the footer — so the message is collapsed to a single line and truncated to
// fit. lipgloss's Width() sets a minimum and WRAPS anything longer, which is
// how a long message used to render as two or three rows and push the layout
// past the bottom of the terminal. Truncating here fixes every caller at once
// rather than asking each one to keep its strings short: bd errors reach this
// via sanitizeErrMsg's 200-character cap, which is far wider than any real
// terminal's toast row.
func (t Toast) View(width int) string {
	if t.Message == "" {
		return ""
	}

	var style lipgloss.Style
	switch t.Level {
	case ToastSuccess:
		style = ui.ToastSuccess
	case ToastWarn:
		style = ui.ToastWarn
	case ToastError:
		style = ui.ToastError
	default:
		style = ui.ToastInfo
	}

	return style.Width(width).Render(fitToastLine(t.Message, width-style.GetHorizontalFrameSize()))
}

// fitToastLine collapses msg to one line and truncates it to avail display
// cells, appending an ellipsis when it had to cut. Width is measured in cells,
// not bytes, so emoji and box-drawing characters in a message are counted the
// way the terminal renders them.
func fitToastLine(msg string, avail int) string {
	msg = strings.Join(strings.Fields(msg), " ")
	if avail <= 0 {
		return ""
	}
	if lipgloss.Width(msg) <= avail {
		return msg
	}
	if avail == 1 {
		return "…"
	}
	return ansi.Truncate(msg, avail, "…")
}

// Active returns true if the toast has a message and hasn't expired.
func (t Toast) Active() bool {
	return t.Message != "" && time.Now().Before(t.ExpiresAt)
}
