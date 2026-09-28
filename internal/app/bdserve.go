package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/matt-wright86/mardi-gras/internal/components"
	"github.com/matt-wright86/mardi-gras/internal/data"
)

// MG_BD_SERVE=<url> points mg at a running `bd serve` (bd 1.3.0+, preview,
// Dolt server mode only). While its event stream is connected it feeds the
// journal follower in place of the `bd events tail` probes, so changes
// arrive in about a second with no bd process per check. Records go through
// the same path as probed ones (absorbJournal), so relevance, mg's own
// writes, partial detection and resync all work unchanged. Whenever the
// stream is down, CLI probes carry on, and mg reconnects with backoff from
// the follower's current anchor.

// serveStream is the app's side of the bd serve stream.
type serveStream struct {
	events    <-chan data.ServeEvent
	cancel    context.CancelFunc
	running   bool
	connected bool
	retryAt   time.Time
	backoff   time.Duration
}

type serveMsg struct {
	ev data.ServeEvent
	ok bool // false once the stream has closed
}

const (
	serveBackoffMin = time.Second
	serveBackoffMax = 30 * time.Second
	// serveDisabledRetry is how long to wait after bd serve reports the
	// journal off: it reads the setting only at startup.
	serveDisabledRetry = 5 * time.Minute
)

// maybeStartServe opens the stream when one is configured, the journal is
// live, none is running, and any retry delay has passed.
func (m *Model) maybeStartServe() tea.Cmd {
	s := &m.journal.serve
	if m.journal.serveURL == "" || s.running || !m.journal.follower.Live() || time.Now().Before(s.retryAt) {
		return nil
	}
	projectID := ""
	if m.beadsContext != nil {
		projectID = m.beadsContext.ProjectID
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.events = data.WatchServeJournal(ctx, m.journal.serveURL, projectID, m.journal.follower.Anchor)
	s.cancel = cancel
	s.running = true
	return waitServe(s.events)
}

func waitServe(events <-chan data.ServeEvent) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-events
		return serveMsg{ev: ev, ok: ok}
	}
}

func (m Model) handleServe(msg serveMsg) (tea.Model, tea.Cmd) {
	s := &m.journal.serve
	if !msg.ok {
		s.cancel = nil
		s.running = false
		s.connected = false
		if s.retryAt.Before(time.Now()) {
			s.backoff = min(max(2*s.backoff, serveBackoffMin), serveBackoffMax)
			s.retryAt = time.Now().Add(s.backoff)
		}
		return m, nil
	}
	next := waitServe(s.events)
	ev := msg.ev
	switch {
	case ev.Connected:
		s.connected = true
		s.backoff = 0
		return m, next
	case ev.Err == nil:
		cmds := m.absorbJournal(ev.Records, nil, time.Now())
		return m, tea.Batch(append(cmds, next)...)
	}

	s.connected = false
	var trunc *data.JournalTruncatedError
	switch {
	case errors.Is(ev.Err, data.ErrJournalReset), errors.As(ev.Err, &trunc):
		// The anchor is gone: the follower re-finds the head, and the stream
		// restarts from there once it is live again.
		cmds := m.absorbJournal(nil, ev.Err, time.Now())
		return m, tea.Batch(append(cmds, next)...)
	case errors.Is(ev.Err, data.ErrServeJournalDisabled):
		s.retryAt = time.Now().Add(serveDisabledRetry)
	case errors.Is(ev.Err, data.ErrServeRefused):
		m.journal.serveURL = "" // retrying won't help this session
		toast, cmd := components.ShowToast(
			fmt.Sprintf("MG_BD_SERVE: %v; checking the journal through bd instead", ev.Err),
			components.ToastWarn, toastDuration,
		)
		m.toast = toast
		return m, tea.Batch(cmd, next)
	case ev.RetryAfter > 0:
		s.retryAt = time.Now().Add(ev.RetryAfter)
	}
	logAction("bd serve stream: %v", ev.Err)
	return m, next
}

// stopServe closes the stream, if one is open.
func (m *Model) stopServe() {
	s := &m.journal.serve
	if s.cancel != nil {
		s.cancel()
	}
	s.connected = false
}
