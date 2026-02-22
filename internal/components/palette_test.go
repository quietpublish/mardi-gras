package components

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func testCommands() []PaletteCommand {
	return []PaletteCommand{
		{Name: "Set status: in_progress", Desc: "Mark rolling", Key: "1", Action: ActionSetInProgress},
		{Name: "Set status: open", Desc: "Mark lined up", Key: "2", Action: ActionSetOpen},
		{Name: "Close issue", Desc: "Mark closed", Key: "3", Action: ActionCloseIssue},
		{Name: "Set priority: P1 high", Desc: "Urgent", Key: "!", Action: ActionSetPriorityHigh},
		{Name: "Copy branch name", Desc: "Clipboard", Key: "b", Action: ActionCopyBranch},
		{Name: "Toggle focus mode", Desc: "My work", Key: "f", Action: ActionToggleFocus},
		{Name: "Filter", Desc: "Fuzzy filter", Key: "/", Action: ActionFilter},
		{Name: "Help", Desc: "Show help", Key: "?", Action: ActionHelp},
		{Name: "Quit", Desc: "Exit", Key: "q", Action: ActionQuit},
	}
}

func TestNewPalette(t *testing.T) {
	cmds := testCommands()
	p := NewPalette(80, 24, cmds)

	if len(p.filtered) != len(cmds) {
		t.Fatalf("expected %d filtered commands, got %d", len(cmds), len(p.filtered))
	}
	if p.cursor != 0 {
		t.Fatalf("expected cursor at 0, got %d", p.cursor)
	}
	if p.width != 80 || p.height != 24 {
		t.Fatalf("expected dimensions 80x24, got %dx%d", p.width, p.height)
	}
}

func TestPaletteEscCancels(t *testing.T) {
	p := NewPalette(80, 24, testCommands())
	_, cmd := p.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("expected command from esc key")
	}
	msg := cmd()
	result, ok := msg.(PaletteResult)
	if !ok {
		t.Fatalf("expected PaletteResult, got %T", msg)
	}
	if !result.Cancelled {
		t.Fatal("expected Cancelled to be true")
	}
}

func TestPaletteEnterSelectsAction(t *testing.T) {
	cmds := testCommands()
	p := NewPalette(80, 24, cmds)

	// Move cursor down to second item
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyDown})

	_, cmd := p.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected command from enter key")
	}
	msg := cmd()
	result, ok := msg.(PaletteResult)
	if !ok {
		t.Fatalf("expected PaletteResult, got %T", msg)
	}
	if result.Cancelled {
		t.Fatal("expected Cancelled to be false")
	}
	if result.Action != cmds[1].Action {
		t.Fatalf("expected action %d, got %d", cmds[1].Action, result.Action)
	}
}

func TestPaletteEnterEmptyFilterCancels(t *testing.T) {
	p := NewPalette(80, 24, testCommands())

	// Type something that matches nothing
	for _, r := range "zzzzzzz" {
		p, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	if len(p.filtered) != 0 {
		t.Fatalf("expected 0 filtered commands, got %d", len(p.filtered))
	}

	_, cmd := p.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected command from enter on empty list")
	}
	msg := cmd()
	result, ok := msg.(PaletteResult)
	if !ok {
		t.Fatalf("expected PaletteResult, got %T", msg)
	}
	if !result.Cancelled {
		t.Fatal("expected Cancelled when no matches")
	}
}

func TestPaletteNavigation(t *testing.T) {
	cmds := testCommands()
	p := NewPalette(80, 24, cmds)

	t.Run("down moves cursor", func(t *testing.T) {
		p2, _ := p.Update(tea.KeyMsg{Type: tea.KeyDown})
		if p2.cursor != 1 {
			t.Fatalf("expected cursor at 1, got %d", p2.cursor)
		}
	})

	t.Run("up at top stays at 0", func(t *testing.T) {
		p2, _ := p.Update(tea.KeyMsg{Type: tea.KeyUp})
		if p2.cursor != 0 {
			t.Fatalf("expected cursor at 0, got %d", p2.cursor)
		}
	})

	t.Run("ctrl+n moves down", func(t *testing.T) {
		p2, _ := p.Update(tea.KeyMsg{Type: tea.KeyCtrlN})
		if p2.cursor != 1 {
			t.Fatalf("expected cursor at 1, got %d", p2.cursor)
		}
	})

	t.Run("ctrl+p at top stays at 0", func(t *testing.T) {
		p2, _ := p.Update(tea.KeyMsg{Type: tea.KeyCtrlP})
		if p2.cursor != 0 {
			t.Fatalf("expected cursor at 0, got %d", p2.cursor)
		}
	})

	t.Run("down clamps at last item", func(t *testing.T) {
		p2 := p
		for i := 0; i < len(cmds)+5; i++ {
			p2, _ = p2.Update(tea.KeyMsg{Type: tea.KeyDown})
		}
		if p2.cursor != len(cmds)-1 {
			t.Fatalf("expected cursor clamped at %d, got %d", len(cmds)-1, p2.cursor)
		}
	})
}

func TestPaletteScrollOffset(t *testing.T) {
	// Create more commands than paletteMaxVisible (12)
	cmds := make([]PaletteCommand, 20)
	for i := range cmds {
		cmds[i] = PaletteCommand{
			Name:   strings.Repeat("x", i+1),
			Desc:   "desc",
			Key:    "k",
			Action: ActionHelp,
		}
	}
	p := NewPalette(80, 40, cmds)

	// Navigate down past visible window
	for i := 0; i < paletteMaxVisible+2; i++ {
		p, _ = p.Update(tea.KeyMsg{Type: tea.KeyDown})
	}

	if p.scrollOffset == 0 {
		t.Fatal("expected scrollOffset to advance past 0")
	}
	if p.cursor < paletteMaxVisible {
		t.Fatalf("expected cursor past %d, got %d", paletteMaxVisible, p.cursor)
	}

	// Navigate back up
	for i := 0; i < paletteMaxVisible+2; i++ {
		p, _ = p.Update(tea.KeyMsg{Type: tea.KeyUp})
	}
	if p.scrollOffset != 0 {
		t.Fatalf("expected scrollOffset back at 0, got %d", p.scrollOffset)
	}
}

func TestPaletteFuzzyFilter(t *testing.T) {
	p := NewPalette(80, 24, testCommands())

	// Type "focus" — should match "Toggle focus mode"
	for _, r := range "focus" {
		p, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}

	if len(p.filtered) == 0 {
		t.Fatal("expected at least one match for 'focus'")
	}
	found := false
	for _, cmd := range p.filtered {
		if cmd.Action == ActionToggleFocus {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected 'Toggle focus mode' in filtered results")
	}
	if p.cursor != 0 {
		t.Fatalf("expected cursor reset to 0, got %d", p.cursor)
	}
}

func TestPaletteFuzzyFilterClearRestores(t *testing.T) {
	cmds := testCommands()
	p := NewPalette(80, 24, cmds)

	// Type to filter
	for _, r := range "quit" {
		p, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	if len(p.filtered) >= len(cmds) {
		t.Fatal("expected filtered list to be shorter than full list")
	}

	// Clear by backspacing
	for i := 0; i < 4; i++ {
		p, _ = p.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	}

	if len(p.filtered) != len(cmds) {
		t.Fatalf("expected all %d commands restored, got %d", len(cmds), len(p.filtered))
	}
}

func TestPaletteSelectedNameDefault(t *testing.T) {
	cmds := testCommands()
	p := NewPalette(80, 24, cmds)
	name := p.SelectedName()
	if name != cmds[0].Name {
		t.Fatalf("expected %q, got %q", cmds[0].Name, name)
	}
}

func TestPaletteSelectedNameAfterNavigation(t *testing.T) {
	cmds := testCommands()
	p := NewPalette(80, 24, cmds)
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyDown})
	p, _ = p.Update(tea.KeyMsg{Type: tea.KeyDown})
	name := p.SelectedName()
	if name != cmds[2].Name {
		t.Fatalf("expected %q at cursor 2, got %q", cmds[2].Name, name)
	}
}

func TestPaletteSelectedNameEmpty(t *testing.T) {
	p := NewPalette(80, 24, nil)
	name := p.SelectedName()
	if name != "" {
		t.Fatalf("expected empty string for nil commands, got %q", name)
	}
}

func TestPaletteViewNotEmpty(t *testing.T) {
	p := NewPalette(80, 24, testCommands())
	view := p.View()
	if view == "" {
		t.Fatal("expected non-empty view")
	}
	if !strings.Contains(view, "COMMAND PALETTE") {
		t.Fatal("expected view to contain 'COMMAND PALETTE'")
	}
}
