package app

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/matt-wright86/mardi-gras/internal/data"
	"github.com/matt-wright86/mardi-gras/internal/gastown"
)

// newAgentOutputModel returns a sized model in tmux whose second issue has
// a live agent pane. Capture Cmds are never run here: they would call tmux.
func newAgentOutputModel(t *testing.T) Model {
	t.Helper()
	issues := []data.Issue{
		testIssue("open-1", data.StatusOpen),
		testIssue("open-2", data.StatusOpen),
	}
	m := New(issues, data.Source{}, data.DefaultBlockingTypes)
	m.startedAt = time.Now().Add(-time.Second) // bypass startup guard
	m.gtEnv = gastown.Env{}
	m.driver = gastown.NewGTDriver()
	model, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 60})
	got := model.(Model)
	got.inTmux = true
	got.activeAgents = map[string]string{"open-2": "%7"}
	return got
}

func press(t *testing.T, m Model, key rune) (Model, tea.Cmd) {
	t.Helper()
	model, cmd := m.Update(tea.KeyPressMsg{Code: key, Text: string(key)})
	return model.(Model), cmd
}

func TestCapturable(t *testing.T) {
	m := newAgentOutputModel(t)
	if !m.capturable("open-2") {
		t.Error("expected an agent in a tmux pane to be capturable")
	}
	if m.capturable("open-1") {
		t.Error("expected an issue without an agent not to be")
	}
	m.activeAgents["open-1"] = "obsidian" // an orchestrator agent name
	if m.capturable("open-1") {
		t.Error("expected an orchestrator agent, which has no pane mg can read, not to be")
	}
	m.inTmux = false
	if m.capturable("open-2") {
		t.Error("expected nothing to be capturable outside tmux")
	}
}

func TestKeyJCapturesAgentOutput(t *testing.T) {
	m := newAgentOutputModel(t)

	got, cmd := press(t, m, 'j')
	if got.parade.SelectedIssue == nil || got.parade.SelectedIssue.ID != "open-2" {
		t.Fatalf("expected open-2 selected, got %+v", got.parade.SelectedIssue)
	}
	if cmd == nil || !got.captureInFlight {
		t.Fatal("expected moving onto an agent's issue to capture its pane")
	}

	got, _ = press(t, got, 'k')
	if got.captureWanted {
		t.Fatal("expected no capture for an issue without an agent")
	}
}

func TestKeyJCaptureSingleFlight(t *testing.T) {
	m := newAgentOutputModel(t)
	got, _ := press(t, m, 'j') // capture 1 in flight
	got, _ = press(t, got, 'k')
	got, cmd := press(t, got, 'j')
	if !got.captureWanted || cmd != nil {
		t.Fatalf("wanted %v cmd %v: want the second capture queued behind the first", got.captureWanted, cmd != nil)
	}

	// The first capture lands; the queued one goes out.
	model, cmd := got.Update(agentOutputMsg{issueID: "open-2", lines: []string{"old"}})
	got = model.(Model)
	if cmd == nil || !got.captureInFlight || got.captureWanted {
		t.Fatal("expected the queued capture to be issued once the first landed")
	}
}

func TestAgentOutputMsg(t *testing.T) {
	m := newAgentOutputModel(t)
	got, _ := press(t, m, 'j')

	model, _ := got.Update(agentOutputMsg{issueID: "open-2", lines: []string{"agent is working hard"}})
	got = model.(Model)
	if got.captureInFlight {
		t.Fatal("expected the capture to be finished")
	}
	if !strings.Contains(got.detail.View(), "agent is working hard") {
		t.Fatal("expected the captured output to be drawn, not just stored")
	}

	// A refresh of the same selection keeps it on screen until the next capture.
	got.syncSelection()
	if !strings.Contains(got.detail.View(), "agent is working hard") {
		t.Fatal("expected output kept across a refresh of the same selection")
	}
}

func TestAgentOutputMsgStale(t *testing.T) {
	m := newAgentOutputModel(t)

	// Selection is open-1; a capture for open-2 lands late.
	model, _ := m.Update(agentOutputMsg{issueID: "open-2", lines: []string{"late"}})
	if got := model.(Model); got.detail.AgentOutputID != "" {
		t.Fatal("expected a capture for a no-longer-selected issue to be dropped")
	}

	// A capture that lands after the agent was killed.
	got, _ := press(t, m, 'j')
	model, _ = got.Update(agentStatusMsg{activeAgents: map[string]string{}})
	model, _ = model.(Model).Update(agentOutputMsg{issueID: "open-2", lines: []string{"ghost"}})
	gone := model.(Model)
	if strings.Contains(gone.detail.View(), "ghost") {
		t.Fatal("expected a capture for an agent that is gone to be dropped")
	}
}

func TestAgentStatusMsgAgentGone(t *testing.T) {
	m := newAgentOutputModel(t)
	got, _ := press(t, m, 'j')
	model, _ := got.Update(agentOutputMsg{issueID: "open-2", lines: []string{"last words"}})
	got = model.(Model)

	model, _ = got.Update(agentStatusMsg{activeAgents: map[string]string{}})
	got = model.(Model)
	if got.detail.AgentOutput != nil || strings.Contains(got.detail.View(), "last words") {
		t.Fatal("expected a dead agent's output to leave the screen")
	}
}

func TestAgentStatusMsgTailsLiveAgent(t *testing.T) {
	m := newAgentOutputModel(t)
	got, _ := press(t, m, 'j')
	model, _ := got.Update(agentOutputMsg{issueID: "open-2", lines: []string{"tick"}})
	got = model.(Model)

	// Each agent poll re-captures the live pane, so the output tails it.
	model, cmd := got.Update(agentStatusMsg{activeAgents: map[string]string{"open-2": "%7"}})
	if cmd == nil || !model.(Model).captureInFlight {
		t.Fatal("expected an agent poll to re-capture the selected live pane")
	}
}

func TestAgentLaunchedMsgCaptures(t *testing.T) {
	m := newAgentOutputModel(t) // open-1 selected, no agent yet
	model, cmd := m.Update(agentLaunchedMsg{issueID: "open-1", windowName: "%9"})
	got := model.(Model)
	if cmd == nil || !got.captureInFlight {
		t.Fatal("expected a freshly launched agent's pane to be captured")
	}
	if got.activeAgents["open-1"] != "%9" || got.activeAgents["open-2"] != "%7" {
		t.Fatalf("agents = %v, want the launch added to the existing map", got.activeAgents)
	}
}

func TestCurrentIssueMsgCapturesAgentOutput(t *testing.T) {
	// A selection change outside parade navigation still captures.
	m := newAgentOutputModel(t)
	model, cmd := m.Update(currentIssueMsg{issueID: "open-2"})
	if cmd == nil || !model.(Model).captureInFlight {
		t.Fatal("expected restoring the selection onto an agent's issue to capture")
	}
}

func TestTownStatusMsgNoPaneNoCapture(t *testing.T) {
	m := newAgentOutputModel(t)
	m.gtEnv = gastown.Env{Available: true}
	m.gtPollInFlight = true
	status := &gastown.TownStatus{Agents: []gastown.AgentRuntime{
		{Name: "obsidian", HookBead: "open-1", State: "working"},
	}}
	model, cmd := m.Update(townStatusMsg{status: status})
	got := model.(Model)
	if got.captureInFlight || got.captureWanted {
		t.Fatalf("cmd %v: an orchestrator agent has no pane mg can capture", cmd != nil)
	}
}

func TestAgentOutputMsgWhileModalOpen(t *testing.T) {
	m := newAgentOutputModel(t)
	got, _ := press(t, m, 'j') // capture in flight
	got.showPalette = true

	model, _ := got.Update(agentOutputMsg{issueID: "open-2", lines: []string{"behind the palette"}})
	got = model.(Model)
	if got.captureInFlight {
		t.Fatal("a capture swallowed by the palette would block every later capture")
	}
	if !strings.Contains(got.detail.View(), "behind the palette") {
		t.Fatal("expected the capture applied behind the palette")
	}
}
