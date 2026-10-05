package app

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/matt-wright86/mardi-gras/internal/components"
	"github.com/matt-wright86/mardi-gras/internal/data"
	"github.com/matt-wright86/mardi-gras/internal/gastown"
)

func initModel(t *testing.T) Model {
	t.Helper()
	issues := []data.Issue{
		testIssue("open-1", data.StatusOpen),
		testIssue("open-2", data.StatusOpen),
		testIssue("closed-1", data.StatusClosed),
	}
	m := New(issues, data.Source{}, data.DefaultBlockingTypes)
	m.startedAt = time.Now().Add(-time.Second) // bypass startup guard
	// Pin the Gas Town driver. New() selects one from the ambient environment,
	// so on a host with gc installed and no gt these tests would otherwise run
	// against a Gas City driver and lose every gt-gated command. Tests that
	// want Gas City behaviour should set m.driver themselves.
	m.driver = gastown.NewGTDriver()
	model, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	return model.(Model)
}

func TestColonOpensPalette(t *testing.T) {
	got := initModel(t)

	model, _ := got.Update(tea.KeyPressMsg{Code: ':', Text: ":"})
	got = model.(Model)

	if !got.showPalette {
		t.Fatal("expected showPalette to be true after pressing :")
	}
}

func TestCtrlKOpensPalette(t *testing.T) {
	got := initModel(t)

	model, _ := got.Update(tea.KeyPressMsg{Code: 'k', Mod: tea.ModCtrl})
	got = model.(Model)

	if !got.showPalette {
		t.Fatal("expected showPalette to be true after pressing ctrl+k")
	}
}

func TestPaletteForwardsKeys(t *testing.T) {
	got := initModel(t)

	// Open palette
	model, _ := got.Update(tea.KeyPressMsg{Code: ':', Text: ":"})
	got = model.(Model)

	// Press 'j' — should NOT move parade cursor (should go to palette input)
	oldCursor := got.parade.Cursor
	model, _ = got.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
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
	model, _ := got.Update(tea.KeyPressMsg{Code: ':', Text: ":"})
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
	model, _ := got.Update(tea.KeyPressMsg{Code: ':', Text: ":"})
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
	model, _ := got.Update(tea.KeyPressMsg{Code: ':', Text: ":"})
	got = model.(Model)

	// Press ctrl+c
	_, cmd := got.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
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
		components.ActionAddNote:         false,
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
		got.agentAvail = false
		cmds := got.buildPaletteCommands()
		for _, cmd := range cmds {
			if cmd.Action == components.ActionLaunchAgent || cmd.Action == components.ActionKillAgent {
				t.Errorf("unexpected agent action %d when agentAvail=false", cmd.Action)
			}
		}
	})

	t.Run("no recovery command without dead rigs", func(t *testing.T) {
		got.gtEnv.Available = true
		got.townStatus = &gastown.TownStatus{}
		cmds := got.buildPaletteCommands()
		for _, cmd := range cmds {
			if cmd.Action == components.ActionRecoverRigs {
				t.Error("unexpected ActionRecoverRigs when no dead rigs")
			}
		}
	})

	t.Run("recovery command with dead rigs", func(t *testing.T) {
		got.gtEnv.Available = true
		got.townStatus = &gastown.TownStatus{
			Rigs: []gastown.RigStatus{{Name: "dead_rig", PolecatCount: 0}},
			Agents: []gastown.AgentRuntime{
				{Name: "ghost", Rig: "dead_rig", HookBead: "mg-001", Running: false},
			},
		}
		cmds := got.buildPaletteCommands()
		found := false
		for _, cmd := range cmds {
			if cmd.Action == components.ActionRecoverRigs {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected ActionRecoverRigs when dead rigs exist")
		}
	})

	t.Run("agent commands when available", func(t *testing.T) {
		got.agentAvail = true
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
			t.Error("expected ActionLaunchAgent when agentAvail=true")
		}
		if !foundKill {
			t.Error("expected ActionKillAgent when agentAvail=true")
		}
	})
}

func TestPaletteEditingCommandsMatchTheirKeys(t *testing.T) {
	// The palette replays each command's key, so both paths open the same
	// thing (mg-hl4).
	for _, tc := range []struct {
		action components.PaletteAction
		check  func(Model) bool
		want   string
	}{
		{components.ActionEditIssue, func(m Model) bool { return m.editing }, "edit form"},
		{components.ActionComment, func(m Model) bool { return m.qaMode == "comment" }, "comment prompt"},
		{components.ActionAssignIssue, func(m Model) bool { return m.qaMode == "assign" }, "assign prompt"},
		{components.ActionAddLabel, func(m Model) bool { return m.qaMode == "label" }, "label prompt"},
		{components.ActionAddDependency, func(m Model) bool { return m.qaMode == "link" }, "link prompt"},
		{components.ActionToggleDoctor, func(m Model) bool { return m.showDoctor }, "doctor overlay"},
		{components.ActionToggleChanges, func(m Model) bool { return m.showChanges }, "recent changes"},
	} {
		model, _ := initModel(t).executePaletteAction(tc.action)
		if !tc.check(model.(Model)) {
			t.Errorf("action %d did not open the %s", tc.action, tc.want)
		}
	}
}

func TestPaletteListsEditingCommands(t *testing.T) {
	keys := map[string]bool{}
	for _, c := range initModel(t).buildPaletteCommands() {
		keys[c.Key] = true
	}
	for _, k := range []string{"e", "r", "y", "t", "l", "D", "E"} {
		if !keys[k] {
			t.Errorf("palette has no command for %q", k)
		}
	}
}
