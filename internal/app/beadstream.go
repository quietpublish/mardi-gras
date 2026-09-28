package app

import (
	"context"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/matt-wright86/mardi-gras/internal/data"
	"github.com/matt-wright86/mardi-gras/internal/gastown"
)

// The orchestrator's bead event stream (Gas City: GCDriver.WatchBeadEvents)
// as a reload trigger. Under an orchestrator mg keeps its 5s bd list poll,
// since agents' bd writes reach the stream late or not at all. The stream
// makes controller and API writes (sling, convoys, closes) show at once
// instead of on the next poll.

// beadStream is the app's side of the stream.
type beadStream struct {
	events <-chan gastown.BeadEvent
	cancel context.CancelFunc
}

type beadStreamStartedMsg struct {
	events <-chan gastown.BeadEvent
	cancel context.CancelFunc
}

type beadStreamMsg struct {
	ev gastown.BeadEvent
	ok bool // false once the stream has closed
}

// beadStreamGap is the least time between the last reload and one a stream
// event asks for. The regular poll already runs 5s after each reload, so
// the journal's 5s spacing would make the stream pointless here; 2s lets a
// controller write show promptly while a burst still costs at most one
// reload per 2s.
const beadStreamGap = 2 * time.Second

// beadTypesBdListHides are orchestrator bead types that never reach the
// parade, so their events don't warrant a reload.
var beadTypesBdListHides = map[string]bool{"session": true, "message": true}

// startBeadStream opens the stream when the driver has one and issues come
// from bd. It runs from Init, as a Cmd, so building a model (as tests do)
// never opens a connection.
func (m Model) startBeadStream() tea.Cmd {
	if !m.driver.Supports(gastown.FeatureSSE) || m.sourceMode != data.SourceCLI {
		return nil
	}
	driver := m.driver
	return func() tea.Msg {
		ctx, cancel := context.WithCancel(context.Background())
		events, err := driver.WatchBeadEvents(ctx)
		if err != nil {
			cancel()
			return nil
		}
		return beadStreamStartedMsg{events: events, cancel: cancel}
	}
}

func waitBeadEvent(events <-chan gastown.BeadEvent) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-events
		return beadStreamMsg{ev: ev, ok: ok}
	}
}

func (m Model) handleBeadStreamStarted(msg beadStreamStartedMsg) (tea.Model, tea.Cmd) {
	m.beadStream = beadStream(msg)
	return m, waitBeadEvent(msg.events)
}

func (m Model) handleBeadStream(msg beadStreamMsg) (tea.Model, tea.Cmd) {
	if !msg.ok {
		m.beadStream = beadStream{}
		return m, nil
	}
	next := waitBeadEvent(m.beadStream.events)
	var refresh tea.Cmd
	switch msg.ev.Kind {
	case gastown.BeadChanged:
		if msg.ev.Ephemeral || beadTypesBdListHides[msg.ev.IssueType] {
			return m, next
		}
		refresh = m.refreshSpacedBy(beadStreamGap)
	case gastown.BeadStreamResumed:
		refresh = m.requestRefresh()
	case gastown.BeadStreamStopped:
		logAction("bead stream stopped: %v", msg.ev.Err)
	}
	return m, tea.Batch(refresh, next)
}

// stopBeadStream closes the stream's connection.
func (m *Model) stopBeadStream() {
	if m.beadStream.cancel != nil {
		m.beadStream.cancel()
	}
}
