package app

import (
	"log"
	"os"
	"sync"

	tea "charm.land/bubbletea/v2"
)

var (
	debugLog  *log.Logger
	debugOnce sync.Once
)

func initDebugLog() {
	debugOnce.Do(func() {
		if os.Getenv("MG_DEBUG") == "" {
			return
		}
		f, err := os.OpenFile("mg-debug.log", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
		if err != nil {
			return
		}
		debugLog = log.New(f, "", log.Ltime|log.Lmicroseconds)
		debugLog.Println("=== mg debug log started ===")
	})
}

func dbg(format string, args ...any) {
	initDebugLog()
	if debugLog != nil {
		debugLog.Printf(format, args...)
	}
}

// logMsg logs an incoming message to the debug log with useful detail.
func logMsg(msg tea.Msg) {
	initDebugLog()
	if debugLog == nil {
		return
	}
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		k := msg.Key()
		debugLog.Printf("KeyPress   String=%-16q Keystroke=%-16q Code=%U Mod=%d Text=%q",
			msg.String(), msg.Keystroke(), k.Code, k.Mod, k.Text)
	case tea.KeyReleaseMsg:
		k := msg.Key()
		debugLog.Printf("KeyRelease String=%-16q Code=%U Mod=%d Text=%q",
			msg.String(), k.Code, k.Mod, k.Text)
	case tea.MouseMsg:
		debugLog.Printf("Mouse      %T", msg)
	case tea.WindowSizeMsg:
		debugLog.Printf("WindowSize %dx%d", msg.Width, msg.Height)
	case tea.KeyboardEnhancementsMsg:
		debugLog.Printf("KbdEnhance %+v", msg)

	// App messages — log type only to avoid noise
	case headerShimmerMsg:
		// too frequent, skip
	case gasTownTickMsg:
		// too frequent, skip
	default:
		debugLog.Printf("Msg        %T  %+v", msg, msg)
	}
}

// logRoute logs which handler a key was routed to.
func logRoute(route string) {
	initDebugLog()
	if debugLog != nil {
		debugLog.Printf("  -> route: %s", route)
	}
}

// logAction logs a specific action taken in response to a key.
func logAction(format string, args ...any) {
	initDebugLog()
	if debugLog != nil {
		debugLog.Printf("  -> action: "+format, args...)
	}
}

// logState logs key model state for debugging.
func logState(m Model) {
	initDebugLog()
	if debugLog != nil {
		pane := "parade"
		if m.activPane == PaneDetail {
			pane = "detail"
		}
		debugLog.Printf("  state: pane=%s help=%v filter=%v palette=%v create=%v gastown=%v problems=%v cursor=%d",
			pane, m.showHelp, m.filtering, m.showPalette, m.creating, m.showGasTown, m.showProblems, m.parade.Cursor)
	}
}
