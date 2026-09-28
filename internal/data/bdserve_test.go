package data

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// fakeServe emulates `bd serve`'s journal endpoints, shaped by bytes
// captured from bd 1.3.0: a paged read and an SSE watch stream that opens
// with `retry: 3000`, sends `id:`/`data:` frames and `: heartbeat` comments,
// and ends a pruned-past stream with `event: truncated`. Like the real
// server, only the paged endpoint checks Bd-Project-Id.
type fakeServe struct {
	journal   *fakeJournal // the paged endpoint answers from this
	frames    chan string  // raw SSE text for the watch stream
	status    int          // non-200 makes both endpoints fail with it
	body      string
	retry     string
	serves    string       // project the paged endpoint accepts; "" accepts any
	projectID string       // header seen on the watch stream
	watches   atomic.Int32 // watch connections accepted
}

func newFakeServe(t *testing.T, head int64) (f *fakeServe, url string) {
	t.Helper()
	f = &fakeServe{journal: &fakeJournal{floor: 1, head: head}, frames: make(chan string, 16), status: http.StatusOK}
	mux := http.NewServeMux()
	fail := func(w http.ResponseWriter) bool {
		if f.status == http.StatusOK {
			return false
		}
		if f.retry != "" {
			w.Header().Set("Retry-After", f.retry)
		}
		w.WriteHeader(f.status)
		_, _ = w.Write([]byte(f.body))
		return true
	}
	mux.HandleFunc("/v0/beads/events", func(w http.ResponseWriter, r *http.Request) {
		if fail(w) {
			return
		}
		if id := r.Header.Get("Bd-Project-Id"); f.serves != "" && id != "" && id != f.serves {
			// bd 1.3.0's reply, less the echoed ID.
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"code":"invalid_argument","detail":"the Bd-Project-Id header names a project this server does not serve"}`))
			return
		}
		var since int64
		_, _ = fmt.Sscan(r.URL.Query().Get("since"), &since)
		var records []JournalRecord
		for seq := since + 1; seq <= f.journal.head && len(records) < 1; seq++ {
			records = append(records, f.journal.record(seq))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"head": f.journal.head, "records": records})
	})
	mux.HandleFunc("/v0/beads/events:watch", func(w http.ResponseWriter, r *http.Request) {
		f.projectID = r.Header.Get("Bd-Project-Id")
		if fail(w) {
			return
		}
		f.watches.Add(1)
		w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, "retry: 3000\n\n")
		w.(http.Flusher).Flush()
		for {
			select {
			case frame := <-f.frames:
				_, _ = fmt.Fprint(w, frame)
				w.(http.Flusher).Flush()
			case <-r.Context().Done():
				return
			}
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return f, srv.URL
}

func recordFrame(r JournalRecord) string {
	b, _ := json.Marshal(r)
	return fmt.Sprintf("id: %d\ndata: %s\n\n", r.Seq, b)
}

func nextServe(t *testing.T, events <-chan ServeEvent) ServeEvent {
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
	return ServeEvent{}
}

func TestWatchServeJournal(t *testing.T) {
	f, url := newFakeServe(t, 10)
	f.serves = "proj-1"
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ref := f.journal.record(10)
	events := WatchServeJournal(ctx, url, "proj-1", JournalAnchor{Seq: 10, Ref: &ref})

	if ev := nextServe(t, events); !ev.Connected {
		t.Fatalf("got %+v, want connected", ev)
	}
	f.frames <- ": heartbeat\n\n"
	f.frames <- recordFrame(f.journal.record(11)) + recordFrame(f.journal.record(12))
	var seqs []int64
	for len(seqs) < 2 {
		for _, r := range nextServe(t, events).Records {
			seqs = append(seqs, r.Seq)
		}
	}
	if seqs[0] != 11 || seqs[1] != 12 {
		t.Fatalf("seqs = %v, want [11 12]", seqs)
	}
	if f.projectID != "proj-1" {
		t.Fatalf("Bd-Project-Id = %q, want proj-1", f.projectID)
	}

	cancel()
	for range events { // drains, then closes
	}
}

func TestWatchServeJournalReset(t *testing.T) {
	_, url := newFakeServe(t, 10)
	stale := (&fakeJournal{gen: 1}).record(10) // a different journal's seq 10
	events := WatchServeJournal(testCtx(t), url, "", JournalAnchor{Seq: 10, Ref: &stale})
	if ev := nextServe(t, events); !errors.Is(ev.Err, ErrJournalReset) {
		t.Fatalf("got %+v, want ErrJournalReset before connecting", ev)
	}
}

func TestWatchServeJournalTruncated(t *testing.T) {
	f, url := newFakeServe(t, 10)
	ref := f.journal.record(10)
	events := WatchServeJournal(testCtx(t), url, "", JournalAnchor{Seq: 10, Ref: &ref})
	nextServe(t, events) // connected
	f.frames <- "retry: 60000\n\nevent: truncated\ndata: {\"code\":\"events_journal_truncated\",\"since\":10,\"floor\":15,\"head\":20,\"status\":410}\n\n"
	ev := nextServe(t, events)
	var trunc *JournalTruncatedError
	if !errors.As(ev.Err, &trunc) || trunc.Head != 20 || trunc.Floor != 15 {
		t.Fatalf("got %+v, want truncation with head 20, floor 15", ev)
	}
}

func TestWatchServeJournalStatus(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
		retry  string
		check  func(ServeEvent) bool
	}{
		{"journal off", http.StatusConflict, `{"code":"events_journal_disabled"}`, "",
			func(ev ServeEvent) bool { return errors.Is(ev.Err, ErrServeJournalDisabled) }},
		{"pruned past", http.StatusGone, `{"code":"events_journal_truncated","since":1,"floor":5,"head":9}`, "",
			func(ev ServeEvent) bool { var tr *JournalTruncatedError; return errors.As(ev.Err, &tr) && tr.Head == 9 }},
		{"auth", http.StatusUnauthorized, `unauthorized`, "",
			func(ev ServeEvent) bool { return errors.Is(ev.Err, ErrServeRefused) }},
		{"bad request", http.StatusBadRequest, `{"code":"invalid_argument"}`, "",
			func(ev ServeEvent) bool { return errors.Is(ev.Err, ErrServeRefused) }},
		{"saturated", http.StatusServiceUnavailable, `{"code":"events_watch_saturated"}`, "7",
			func(ev ServeEvent) bool { return ev.Err != nil && ev.RetryAfter == 7*time.Second }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, url := newFakeServe(t, 1)
			f.status, f.body, f.retry = tt.status, tt.body, tt.retry
			ev := nextServe(t, WatchServeJournal(testCtx(t), url, "", JournalAnchor{Seq: 1}))
			if !tt.check(ev) {
				t.Fatalf("got %+v", ev)
			}
		})
	}
}

// The watch stream ignores Bd-Project-Id, so the anchor check's paged read
// is what keeps mg off another workspace's server.
func TestWatchServeJournalWrongProject(t *testing.T) {
	f, url := newFakeServe(t, 3)
	f.serves = "proj-1"
	ref := f.journal.record(3)
	ev := nextServe(t, WatchServeJournal(testCtx(t), url, "proj-2", JournalAnchor{Seq: 3, Ref: &ref}))
	if !errors.Is(ev.Err, ErrServeRefused) {
		t.Fatalf("got %+v, want refused", ev)
	}
	if n := f.watches.Load(); n != 0 {
		t.Fatalf("watch connections = %d, want none for another workspace", n)
	}
}

func TestWatchServeJournalIdle(t *testing.T) {
	old := serveIdle
	serveIdle = 50 * time.Millisecond
	t.Cleanup(func() { serveIdle = old })

	f, url := newFakeServe(t, 3)
	ref := f.journal.record(3)
	events := WatchServeJournal(testCtx(t), url, "", JournalAnchor{Seq: 3, Ref: &ref})
	nextServe(t, events) // connected, then silence
	if ev := nextServe(t, events); ev.Err == nil || errors.Is(ev.Err, ErrServeRefused) {
		t.Fatalf("got %+v, want a retryable drop from the idle watchdog", ev)
	}
}

func TestWatchServeJournalLongRecord(t *testing.T) {
	f, url := newFakeServe(t, 1)
	ref := f.journal.record(1)
	events := WatchServeJournal(testCtx(t), url, "", JournalAnchor{Seq: 1, Ref: &ref})
	nextServe(t, events)
	long := fmt.Sprintf(`id: 2%sdata: {"seq":2,"ts":"t","op":"update","issue_id":"a","issue":{"title":%q}}%s`,
		"\n", strings.Repeat("x", 200_000), "\n\n")
	f.frames <- long
	if ev := nextServe(t, events); len(ev.Records) != 1 || ev.Records[0].Seq != 2 {
		t.Fatalf("got %+v, want the long record", ev)
	}
}

// testCtx is cancelled when the test ends, so a watch never outlives it:
// httptest's Close waits for the stream to finish.
func testCtx(t *testing.T) context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	return ctx
}
