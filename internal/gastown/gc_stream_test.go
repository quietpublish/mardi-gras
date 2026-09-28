package gastown

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// streamServer is a fake supervisor events stream. Each connection sends
// whatever the test puts on frames, one SSE frame per string; closeAfter > 0
// closes the first connection after that many frames.
type streamServer struct {
	frames     chan string
	conns      atomic.Int32
	closeAfter int
	status     int
}

func newStreamServer(t *testing.T) (*streamServer, *GCDriver) {
	t.Helper()
	s := &streamServer{frames: make(chan string, 16), status: http.StatusOK}
	mux := http.NewServeMux()
	mux.HandleFunc("/v0/city/bourbon/events/stream", func(w http.ResponseWriter, r *http.Request) {
		n := s.conns.Add(1)
		if s.status != http.StatusOK {
			w.Header().Set("Content-Type", "application/problem+json")
			w.WriteHeader(s.status)
			_, _ = w.Write([]byte(`{"title":"Service Unavailable","detail":"events not enabled"}`))
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		sent := 0
		for {
			select {
			case f := <-s.frames:
				_, _ = fmt.Fprint(w, f)
				w.(http.Flusher).Flush()
				sent++
				if n == 1 && s.closeAfter > 0 && sent >= s.closeAfter {
					return
				}
			case <-r.Context().Done():
				return
			}
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	d, err := NewGCDriver(srv.URL, "bourbon")
	if err != nil {
		t.Fatal(err)
	}
	return s, d
}

func beadFrame(seq int, typ, id, issueType string) string {
	return fmt.Sprintf("id: %d\nevent: event\ndata: {\"seq\":%d,\"type\":%q,\"actor\":\"cache-reconcile\",\"subject\":%q,\"payload\":{\"bead\":{\"id\":%q,\"title\":\"t\",\"status\":\"open\",\"issue_type\":%q,\"created_at\":\"2026-09-28T00:00:00Z\"}}}\n\n",
		seq, seq, typ, id, id, issueType)
}

const heartbeatFrame = "event: heartbeat\ndata: {\"timestamp\":\"2026-09-28T00:00:00Z\"}\n\n"

func fastStream(t *testing.T) {
	t.Helper()
	idle, lo, hi := gcStreamIdle, gcStreamBackoffMin, gcStreamBackoffMax
	gcStreamBackoffMin, gcStreamBackoffMax = 10*time.Millisecond, 20*time.Millisecond
	t.Cleanup(func() { gcStreamIdle, gcStreamBackoffMin, gcStreamBackoffMax = idle, lo, hi })
}

func next(t *testing.T, events <-chan BeadEvent) BeadEvent {
	t.Helper()
	select {
	case ev, ok := <-events:
		if !ok {
			t.Fatal("stream closed")
		}
		return ev
	case <-time.After(5 * time.Second):
		t.Fatal("no event")
	}
	return BeadEvent{}
}

func TestGCDriverWatchBeadEvents(t *testing.T) {
	fastStream(t)
	s, d := newStreamServer(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events, err := d.WatchBeadEvents(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// Heartbeats and non-bead events are not triggers; the bead change is.
	s.frames <- heartbeatFrame
	s.frames <- "id: 1\nevent: event\ndata: {\"seq\":1,\"type\":\"session.woke\",\"subject\":\"s-1\"}\n\n"
	s.frames <- beadFrame(2, "bead.updated", "mg-q12", "task")
	if ev := next(t, events); ev.Kind != BeadChanged || ev.BeadID != "mg-q12" || ev.IssueType != "task" {
		t.Fatalf("got %+v, want mg-q12 changed", ev)
	}

	// Types bd list hides still arrive, typed, for the caller to skip.
	s.frames <- beadFrame(3, "bead.created", "s-9", "session")
	if ev := next(t, events); ev.IssueType != "session" {
		t.Fatalf("got %+v, want the session bead's type", ev)
	}

	// Cancelling ends the stream.
	cancel()
	select {
	case _, ok := <-events:
		for ok {
			_, ok = <-events
		}
	case <-time.After(5 * time.Second):
		t.Fatal("stream did not close after cancel")
	}
}

func TestGCDriverWatchBeadEventsResumes(t *testing.T) {
	fastStream(t)
	s, d := newStreamServer(t)
	s.closeAfter = 1
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events, _ := d.WatchBeadEvents(ctx)

	s.frames <- beadFrame(1, "bead.closed", "mg-1", "task")
	if ev := next(t, events); ev.Kind != BeadChanged {
		t.Fatalf("got %+v, want a change", ev)
	}
	// The server dropped the connection: the driver reconnects and asks for
	// a reload, since it missed whatever changed in between.
	if ev := next(t, events); ev.Kind != BeadStreamResumed {
		t.Fatalf("got %+v, want resumed", ev)
	}
	if s.conns.Load() < 2 {
		t.Fatalf("connections = %d, want a reconnect", s.conns.Load())
	}
}

func TestGCDriverWatchBeadEventsIdle(t *testing.T) {
	fastStream(t)
	gcStreamIdle = 50 * time.Millisecond
	s, d := newStreamServer(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events, _ := d.WatchBeadEvents(ctx)

	// No heartbeats: the watchdog drops the silent connection and reconnects.
	if ev := next(t, events); ev.Kind != BeadStreamResumed {
		t.Fatalf("got %+v, want resumed after a silent connection", ev)
	}
	if s.conns.Load() < 2 {
		t.Fatalf("connections = %d, want a reconnect", s.conns.Load())
	}
}

func TestGCDriverWatchBeadEventsRefused(t *testing.T) {
	fastStream(t)
	s, d := newStreamServer(t)
	s.status = http.StatusServiceUnavailable
	events, _ := d.WatchBeadEvents(context.Background())

	ev := next(t, events)
	if ev.Kind != BeadStreamStopped || !errors.Is(ev.Err, errStreamRefused) {
		t.Fatalf("got %+v, want stopped: refused", ev)
	}
	if _, ok := <-events; ok {
		t.Fatal("expected the stream to close after stopping")
	}
	if s.conns.Load() != 1 {
		t.Fatalf("connections = %d, want no retry after a refusal", s.conns.Load())
	}
}

func TestParseGCBeadEvent(t *testing.T) {
	long := strings.Repeat("x", 200_000)
	tests := []struct {
		name, event, data string
		want              bool
		id                string
	}{
		{"heartbeat", "heartbeat", `{"timestamp":"t"}`, false, ""},
		{"not json", "event", `{`, false, ""},
		{"other type", "event", `{"type":"session.woke","subject":"s"}`, false, ""},
		{"claim rejected", "event", `{"type":"bead.claim_rejected","subject":"b"}`, false, ""},
		{"deleted", "event", `{"type":"bead.deleted","subject":"b-1"}`, true, "b-1"},
		{"id from payload", "event", `{"type":"bead.updated","payload":{"bead":{"id":"b-2"}}}`, true, "b-2"},
		{"long description", "event", `{"type":"bead.updated","subject":"b-3","payload":{"bead":{"description":"` + long + `"}}}`, true, "b-3"},
	}
	for _, tt := range tests {
		ev, ok := parseGCBeadEvent(tt.event, tt.data)
		if ok != tt.want || (ok && ev.BeadID != tt.id) {
			t.Errorf("%s: got %+v, %v; want %v %q", tt.name, ev, ok, tt.want, tt.id)
		}
	}
}

func TestWatchBeadEventsSupport(t *testing.T) {
	if _, err := (GTDriver{}).WatchBeadEvents(context.Background()); !errors.Is(err, ErrUnsupported) {
		t.Errorf("GTDriver: err = %v, want ErrUnsupported", err)
	}
	if (GTDriver{}).Supports(FeatureSSE) {
		t.Error("GTDriver should not claim FeatureSSE")
	}
	gc, _ := NewGCDriver("http://127.0.0.1:1", "")
	if !gc.Supports(FeatureSSE) || gc.Supports(FeatureVitals) {
		t.Error("GCDriver should support FeatureSSE and nothing else")
	}
}
