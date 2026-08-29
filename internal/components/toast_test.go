package components

import (
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
)

func TestToastActiveBeforeExpiry(t *testing.T) {
	toast, _ := ShowToast("hello", ToastInfo, 5*time.Second)
	if !toast.Active() {
		t.Fatal("toast should be active before expiry")
	}
}

func TestToastInactiveAfterExpiry(t *testing.T) {
	toast := Toast{
		Message:   "expired",
		Level:     ToastInfo,
		ExpiresAt: time.Now().Add(-1 * time.Second),
	}
	if toast.Active() {
		t.Fatal("toast should be inactive after expiry")
	}
}

func TestToastInactiveWhenEmpty(t *testing.T) {
	toast := Toast{
		Message:   "",
		ExpiresAt: time.Now().Add(5 * time.Second),
	}
	if toast.Active() {
		t.Fatal("toast with empty message should not be active")
	}
}

func TestShowToastSetsExpiry(t *testing.T) {
	before := time.Now()
	toast, _ := ShowToast("test", ToastSuccess, 3*time.Second)
	after := time.Now()

	expectedMin := before.Add(3 * time.Second)
	expectedMax := after.Add(3 * time.Second)

	if toast.ExpiresAt.Before(expectedMin) || toast.ExpiresAt.After(expectedMax) {
		t.Fatalf("ExpiresAt %v not in expected range [%v, %v]", toast.ExpiresAt, expectedMin, expectedMax)
	}
}

func TestToastViewEmptyWhenInactive(t *testing.T) {
	toast := Toast{
		Message:   "",
		ExpiresAt: time.Now().Add(-1 * time.Second),
	}
	if got := toast.View(80); got != "" {
		t.Fatalf("inactive toast View() = %q, want empty string", got)
	}
}

func TestToastViewIncludesMessageForEachLevel(t *testing.T) {
	tests := []struct {
		name  string
		level ToastLevel
	}{
		{"info", ToastInfo},
		{"success", ToastSuccess},
		{"warn", ToastWarn},
		{"error", ToastError},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			toast := Toast{Message: "hello " + tc.name, Level: tc.level}
			got := toast.View(80)
			if !strings.Contains(got, "hello "+tc.name) {
				t.Errorf("View() = %q, want to contain message", got)
			}
			if !strings.Contains(got, "\x1b[") {
				t.Errorf("View() missing ANSI styling for level %v: %q", tc.level, got)
			}
		})
	}
}

func TestToastViewDistinctStylePerLevel(t *testing.T) {
	// Each level applies a distinct background; the rendered string must
	// differ between Info/Success/Warn/Error for the same message+width.
	msg := "alert"
	info := Toast{Message: msg, Level: ToastInfo}.View(40)
	success := Toast{Message: msg, Level: ToastSuccess}.View(40)
	warn := Toast{Message: msg, Level: ToastWarn}.View(40)
	errToast := Toast{Message: msg, Level: ToastError}.View(40)

	pairs := [][2]string{{info, success}, {info, warn}, {info, errToast}, {success, errToast}, {warn, errToast}}
	for i, p := range pairs {
		if p[0] == p[1] {
			t.Errorf("pair %d: levels rendered identically (style switch broken)", i)
		}
	}
}

func TestToastViewZeroWidthDoesNotPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Toast.View(0) panicked: %v", r)
		}
	}()
	_ = Toast{Message: "x", Level: ToastInfo}.View(0)
}

// TestToastViewFitsOneRow is the regression that matters: the toast occupies a
// single divider row, and lipgloss's Width() wraps rather than truncates, so a
// long message used to render two or three rows and push the layout past the
// bottom of the terminal. Every level and every realistic width must stay at
// exactly one line.
func TestToastViewFitsOneRow(t *testing.T) {
	// The real bd error that surfaced this: 362 characters, reachable by any
	// user whose Beads store has pending migrations against dirty tables.
	bdErr := "bd close: failed to open database: failed to initialize schema: " +
		"schema migration: pending schema migrations alter pre-existing dirty tables: " +
		"child_counters, comments, compaction_snapshots, dependencies, events, " +
		"issue_snapshots, issues, labels; run 'bd dolt commit' to commit the working " +
		"set at the current schema, then re-run the migration"

	messages := map[string]string{
		"long bd error":  bdErr,
		"emoji + dashes": "⛔ town is frozen for migration (by someone) — 💬 see the docs for the full recovery procedure",
		"short":          "closed mg-1",
		"multi-line":     "first line\nsecond line\nthird line",
	}
	levels := []ToastLevel{ToastInfo, ToastSuccess, ToastWarn, ToastError}

	for name, msg := range messages {
		for _, lvl := range levels {
			for _, width := range []int{40, 60, 80, 120, 200} {
				toast := Toast{Message: msg, Level: lvl, ExpiresAt: time.Now().Add(time.Minute)}
				out := toast.View(width)
				if got := lipgloss.Height(out); got != 1 {
					t.Errorf("%s at level %d width %d: rendered %d rows, want 1", name, lvl, width, got)
				}
				if got := lipgloss.Width(out); got > width {
					t.Errorf("%s at level %d width %d: rendered %d cells, want <= %d", name, lvl, width, got, width)
				}
			}
		}
	}
}

// TestFitToastLine covers the truncation helper directly, including the
// degenerate widths a very narrow terminal can produce.
func TestFitToastLine(t *testing.T) {
	tests := []struct {
		name, msg string
		avail     int
		want      string
	}{
		{"fits untouched", "hello", 10, "hello"},
		{"exact fit", "hello", 5, "hello"},
		{"truncated with ellipsis", "hello world", 8, "hello w…"},
		{"newlines collapse to spaces", "a\nb\nc", 10, "a b c"},
		{"zero width", "hello", 0, ""},
		{"negative width", "hello", -3, ""},
		{"single cell", "hello", 1, "…"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := fitToastLine(tc.msg, tc.avail); got != tc.want {
				t.Errorf("fitToastLine(%q, %d) = %q, want %q", tc.msg, tc.avail, got, tc.want)
			}
		})
	}
}

// TestFitToastLineCountsCellsNotBytes guards the emoji case: a naive len()
// truncation would cut mid-rune and under-fill the row.
func TestFitToastLineCountsCellsNotBytes(t *testing.T) {
	msg := "💬💬💬💬💬" // 5 double-width glyphs = 10 cells, 20 bytes
	got := fitToastLine(msg, 6)
	if w := lipgloss.Width(got); w > 6 {
		t.Errorf("fitToastLine(emoji, 6) = %q (%d cells), want <= 6", got, w)
	}
	if !strings.Contains(got, "…") {
		t.Errorf("fitToastLine(emoji, 6) = %q, want an ellipsis marking the cut", got)
	}
}
