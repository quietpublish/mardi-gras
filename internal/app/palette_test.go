package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/matt-wright86/mardi-gras/internal/components"
	"github.com/matt-wright86/mardi-gras/internal/data"
)

func initModel(t *testing.T) Model {
	t.Helper()
	issues := []data.Issue{
		testIssue("open-1", data.StatusOpen),
		testIssue("open-2", data.StatusOpen),
		testIssue("closed-1", data.StatusClosed),
	}
	m := New(issues, data.Source{}, data.DefaultBlockingTypes)
	model, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	return model.(Model)
}

func TestColonOpensPalette(t *testing.T) {
	got := initModel(t)

	model, _ := got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{':'}})
	got = model.(Model)

	if !got.showPalette {
		t.Fatal("expected showPalette to be true after pressing :")
	}
}

func TestCtrlKOpensPalette(t *testing.T) {
	got := initModel(t)

	model, _ := got.Update(tea.KeyMsg{Type: tea.KeyCtrlK})
	got = model.(Model)

	if !got.showPalette {
		t.Fatal("expected showPalette to be true after pressing ctrl+k")
	}
}

func TestPaletteForwardsKeys(t *testing.T) {
	got := initModel(t)

	// Open palette
	model, _ := got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{':'}})
	got = model.(Model)

	// Press 'j' — should NOT move parade cursor (should go to palette input)
	oldCursor := got.parade.Cursor
	model, _ = got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	got = model.(Model)

	if got.parade.Cursor != oldCursor {
		t.Fatalf("expected parade cursor unchanged at %d, got %d", oldCursor, got.parade.Cursor)
	}
	if !got.showPalette {
		t.Fatal("expected palette to remain open")
	}
}

func TestPaletteResultCancelledClosesPalette(t *testing.T) {
	got := initModel(t)

	// Open palette
	model, _ := got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{':'}})
	got = model.(Model)
	if !got.showPalette {
		t.Fatal("expected palette to be open")
	}

	// Send cancelled result
	model, _ = got.Update(components.PaletteResult{Cancelled: true})
	got = model.(Model)

	if got.showPalette {
		t.Fatal("expected showPalette to be false after cancelled result")
	}
}

func TestPaletteResultExecutesAction(t *testing.T) {
	got := initModel(t)

	// Open palette
	model, _ := got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{':'}})
	got = model.(Model)

	// Send toggle closed action
	model, _ = got.Update(components.PaletteResult{Action: components.ActionToggleClosed})
	got = model.(Model)

	if got.showPalette {
		t.Fatal("expected palette to close after executing action")
	}
	if !got.parade.ShowClosed {
		t.Fatal("expected ShowClosed to be true after ActionToggleClosed")
	}
}

func TestPaletteCtrlCQuits(t *testing.T) {
	got := initModel(t)

	// Open palette
	model, _ := got.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{':'}})
	got = model.(Model)

	// Press ctrl+c
	_, cmd := got.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("expected quit command from ctrl+c during palette")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Fatalf("expected tea.QuitMsg, got %T", msg)
	}
}

func TestBuildPaletteCommandsBase(t *testing.T) {
	got := initModel(t)
	cmds := got.buildPaletteCommands()

	// Check that essential actions are present
	required := map[components.PaletteAction]bool{
		components.ActionSetInProgress:   false,
		components.ActionSetOpen:         false,
		components.ActionCloseIssue:      false,
		components.ActionSetPriorityHigh: false,
		components.ActionCopyBranch:      false,
		components.ActionToggleFocus:     false,
		components.ActionFilter:          false,
		components.ActionHelp:            false,
		components.ActionQuit:            false,
	}

	for _, cmd := range cmds {
		if _, want := required[cmd.Action]; want {
			required[cmd.Action] = true
		}
	}

	for action, found := range required {
		if !found {
			t.Errorf("expected action %d to be present in base commands", action)
		}
	}
}

func TestBuildPaletteCommandsConditional(t *testing.T) {
	got := initModel(t)

	t.Run("no agent commands when unavailable", func(t *testing.T) {
		got.claudeAvail = false
		cmds := got.buildPaletteCommands()
		for _, cmd := range cmds {
			if cmd.Action == components.ActionLaunchAgent || cmd.Action == components.ActionKillAgent {
				t.Errorf("unexpected agent action %d when claudeAvail=false", cmd.Action)
			}
		}
	})

	t.Run("agent commands when available", func(t *testing.T) {
		got.claudeAvail = true
		cmds := got.buildPaletteCommands()
		foundLaunch := false
		foundKill := false
		for _, cmd := range cmds {
			if cmd.Action == components.ActionLaunchAgent {
				foundLaunch = true
			}
			if cmd.Action == components.ActionKillAgent {
				foundKill = true
			}
		}
		if !foundLaunch {
			t.Error("expected ActionLaunchAgent when claudeAvail=true")
		}
		if !foundKill {
			t.Error("expected ActionKillAgent when claudeAvail=true")
		}
	})
}
