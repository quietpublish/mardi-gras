package gastown

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Gas City's supervisor publishes every city event on
// GET /v0/city/{city}/events/stream as server-sent events:
//
//	id: <seq>
//	event: event
//	data: {"seq":..,"type":"bead.updated","subject":"<bead id>","payload":{"bead":{..}},..}
//
// with an `event: heartbeat` every 15s. mg uses the bead events as a reload
// trigger. They arrive at once for controller and API writes (sling, convoys,
// session and molecule beads), which bd's own events journal misses in the
// supervisor's server topology. Agents' bd writes only reach the stream when
// the supervisor rescans, 30-120s later, so the stream speeds updates up but
// cannot replace polling.
//
// The generated gcclient is not used: its StreamEvents wrapper reads the
// whole body, which for a stream means blocking until it ends, and the
// endpoint's models would add thousands of generated lines for one URL.

// gcBeadEventTypes are the stream event types that change what bd list
// returns. Claim rejections and worktree reaping don't, so they are left out.
//
// The last three each move an assignee, and all carry the bead ID as subject:
//   - bead.claim_released (Gas City v1.5.0+): gc hook --claim gave back a claim
//     it won. The release is a write through the agent's own bd context, which
//     reaches the stream as bead.updated only when the supervisor rescans, so
//     this is the only prompt signal for it.
//   - hook.claim.reclaimed_stale: gc hook --claim took over a lease-expired
//     claim, through the same bd context.
//   - bead.dead_assignee_reopened: the reconciler reopened a bead whose
//     assignee's session is gone. The controller's store usually announces the
//     write as bead.updated as well; a duplicate trigger costs nothing, because
//     a waiting trigger absorbs the next one (send).
var gcBeadEventTypes = map[string]bool{
	"bead.created":                true,
	"bead.updated":                true,
	"bead.closed":                 true,
	"bead.deleted":                true,
	"bead.claim_released":         true,
	"hook.claim.reclaimed_stale":  true,
	"bead.dead_assignee_reopened": true,
}

// Stream tuning. Variables so tests can shorten them; each watch copies them
// when it starts (gcStreamTuning), so its goroutines never read the globals.
var (
	gcStreamIdle       = 45 * time.Second // three missed 15s heartbeats
	gcStreamBackoffMin = time.Second
	gcStreamBackoffMax = 30 * time.Second
)

type gcStreamTuning struct {
	idle, backoffMin, backoffMax time.Duration
}

// errStreamRefused marks a response that retrying won't fix.
var errStreamRefused = errors.New("gas city refused the event stream")

// WatchBeadEvents streams bead change events from the supervisor, starting
// from now, reconnecting with backoff after a drop.
func (d *GCDriver) WatchBeadEvents(ctx context.Context) (<-chan BeadEvent, error) {
	out := make(chan BeadEvent, 1)
	tune := gcStreamTuning{idle: gcStreamIdle, backoffMin: gcStreamBackoffMin, backoffMax: gcStreamBackoffMax}
	go d.watchBeadEvents(ctx, out, tune)
	return out, nil
}

func (d *GCDriver) watchBeadEvents(ctx context.Context, out chan<- BeadEvent, tune gcStreamTuning) {
	defer close(out)
	// No Timeout: a client timeout covers reading the body and would cut the
	// stream. ctx and the idle watchdog end it instead.
	client := &http.Client{}
	backoff := tune.backoffMin
	connectedBefore := false
	for ctx.Err() == nil {
		opened, err := d.streamOnce(ctx, client, out, connectedBefore, tune.idle)
		if ctx.Err() != nil {
			return
		}
		if errors.Is(err, errStreamRefused) {
			select {
			case out <- BeadEvent{Kind: BeadStreamStopped, Err: err}:
			case <-ctx.Done():
			}
			return
		}
		if opened {
			connectedBefore = true
			backoff = tune.backoffMin
		}
		select {
		case <-time.After(backoff):
		case <-ctx.Done():
			return
		}
		backoff = min(2*backoff, tune.backoffMax)
	}
}

// streamOnce holds one connection until it drops. opened reports whether the
// supervisor accepted it.
func (d *GCDriver) streamOnce(ctx context.Context, client *http.Client, out chan<- BeadEvent, resumed bool, idle time.Duration) (opened bool, err error) {
	city, err := d.resolveCity(ctx)
	if err != nil {
		return false, err
	}
	connCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	req, err := http.NewRequestWithContext(connCtx, http.MethodGet,
		strings.TrimRight(d.baseURL, "/")+"/v0/city/"+url.PathEscape(city)+"/events/stream", http.NoBody)
	if err != nil {
		return false, err
	}
	req.Header.Set("Accept", "text/event-stream")
	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusServiceUnavailable:
		// No such city, events not enabled, or a read gate mg can't pass.
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return false, fmt.Errorf("%w: %s", errStreamRefused, gcRespErr(resp.StatusCode, body))
	default:
		return false, fmt.Errorf("gc events stream: HTTP %d", resp.StatusCode)
	}

	if resumed {
		// Changes made while disconnected were not seen: ask for a reload.
		send(ctx, out, BeadEvent{Kind: BeadStreamResumed})
	}

	// Idle watchdog: a stream that goes quiet past a few heartbeats is dead
	// even if the socket isn't.
	lines := make(chan struct{}, 1)
	go func() {
		timer := time.NewTimer(idle)
		defer timer.Stop()
		for {
			select {
			case <-lines:
				// Go 1.23+ timers: Reset alone discards any pending fire.
				timer.Reset(idle)
			case <-timer.C:
				cancel()
				return
			case <-connCtx.Done():
				return
			}
		}
	}()

	r := bufio.NewReader(resp.Body)
	var event, data string
	for {
		// ReadString, not a Scanner: a bead with a long description makes a
		// data line longer than a Scanner's default buffer.
		line, err := r.ReadString('\n')
		if err != nil {
			return true, err
		}
		select {
		case lines <- struct{}{}:
		default:
		}
		line = strings.TrimRight(line, "\r\n")
		switch {
		case line == "":
			if ev, ok := parseGCBeadEvent(event, data); ok {
				send(ctx, out, ev)
			}
			event, data = "", ""
		case strings.HasPrefix(line, "event:"):
			event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		case strings.HasPrefix(line, "data:"):
			data += strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " ")
		}
	}
}

// parseGCBeadEvent turns one SSE message into a BeadEvent, if it is a bead
// change.
func parseGCBeadEvent(event, data string) (BeadEvent, bool) {
	if event == "heartbeat" || data == "" {
		return BeadEvent{}, false
	}
	var env struct {
		Type    string `json:"type"`
		Subject string `json:"subject"`
		Payload struct {
			Bead struct {
				ID        string `json:"id"`
				IssueType string `json:"issue_type"`
				Ephemeral bool   `json:"ephemeral"`
			} `json:"bead"`
		} `json:"payload"`
	}
	if json.Unmarshal([]byte(data), &env) != nil || !gcBeadEventTypes[env.Type] {
		return BeadEvent{}, false
	}
	id := env.Subject
	if id == "" {
		id = env.Payload.Bead.ID
	}
	return BeadEvent{
		Kind:      BeadChanged,
		BeadID:    id,
		IssueType: env.Payload.Bead.IssueType,
		Ephemeral: env.Payload.Bead.Ephemeral,
	}, true
}

// send delivers a trigger without ever blocking the stream. A trigger
// already waiting means a reload is already coming, so a second one can be
// dropped.
func send(ctx context.Context, out chan<- BeadEvent, ev BeadEvent) {
	select {
	case out <- ev:
	case <-ctx.Done():
	default:
	}
}
