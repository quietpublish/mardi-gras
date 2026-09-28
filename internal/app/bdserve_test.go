package app

import (
	"fmt"
	"testing"
	"time"

	"github.com/matt-wright86/mardi-gras/internal/data"
)

// serveModel is a live journal with a bd serve stream the test feeds.
func serveModel(t *testing.T) Model {
	t.Helper()
	m := liveModel(t, []data.Issue{testIssue("a", data.StatusOpen)}) // anchor at seq 10
	m.journal.serveURL = "http://127.0.0.1:1"
	m.journal.serve = serveStream{events: make(chan data.ServeEvent), running: true}
	return m
}

func serve(t *testing.T, m Model, ev data.ServeEvent) Model {
	t.Helper()
	model, _ := m.Update(serveMsg{ev: ev, ok: true})
	return model.(Model)
}

func TestServeConnectedPausesProbes(t *testing.T) {
	m := serve(t, serveModel(t), data.ServeEvent{Connected: true})
	if !m.journal.serve.connected {
		t.Fatal("expected the stream marked connected")
	}
	model, _ := m.Update(journalTickMsg{gen: m.journal.gen})
	if model.(Model).journal.busy {
		t.Fatal("expected no CLI probe while the stream feeds the follower")
	}
}

func TestServeRecordsAbsorbed(t *testing.T) {
	m := serve(t, serveModel(t), data.ServeEvent{Connected: true})
	m.refresh.landedAt = time.Now().Add(-time.Minute)
	m = serve(t, m, data.ServeEvent{Records: []data.JournalRecord{probeRecord(11, "a"), probeRecord(12, "a")}})
	if m.journal.follower.Anchor.Seq != 12 {
		t.Fatalf("anchor %d, want 12", m.journal.follower.Anchor.Seq)
	}
	if !m.refresh.inFlight {
		t.Fatal("expected streamed records for an issue on screen to reload")
	}
	if len(m.journal.recent) != 2 {
		t.Fatalf("feed has %d records, want 2", len(m.journal.recent))
	}
}

func TestServeStaleRecordsIgnored(t *testing.T) {
	m := serveModel(t)
	m = serve(t, m, data.ServeEvent{Records: []data.JournalRecord{probeRecord(8, "a"), probeRecord(10, "a")}})
	if m.journal.follower.Anchor.Seq != 10 || m.refresh.inFlight {
		t.Fatalf("anchor %d inFlight %v: records at or below the anchor must change nothing", m.journal.follower.Anchor.Seq, m.refresh.inFlight)
	}
}

func TestServeResetRebaselines(t *testing.T) {
	m := serve(t, serveModel(t), data.ServeEvent{Err: data.ErrJournalReset})
	if m.journal.follower.Phase != data.JournalBaselining || !m.journal.busy {
		t.Fatalf("phase %v busy %v, want a head search", m.journal.follower.Phase, m.journal.busy)
	}
}

func TestServeRefusedDisables(t *testing.T) {
	m := serve(t, serveModel(t), data.ServeEvent{Err: fmt.Errorf("%w: HTTP 401", data.ErrServeRefused)})
	if m.journal.serveURL != "" || !m.toast.Active() {
		t.Fatalf("serveURL %q toast %v: want the stream given up with a toast", m.journal.serveURL, m.toast.Active())
	}
}

func TestServeDisabledRetriesLater(t *testing.T) {
	m := serve(t, serveModel(t), data.ServeEvent{Err: data.ErrServeJournalDisabled})
	if wait := time.Until(m.journal.serve.retryAt); wait < serveDisabledRetry-time.Minute {
		t.Fatalf("retry in %v, want about %v", wait, serveDisabledRetry)
	}
}

func TestServeClosedBacksOff(t *testing.T) {
	m := serve(t, serveModel(t), data.ServeEvent{Connected: true})
	model, _ := m.Update(serveMsg{ok: false})
	got := model.(Model)
	if got.journal.serve.running || got.journal.serve.connected {
		t.Fatal("expected a closed stream to be marked down")
	}
	if wait := time.Until(got.journal.serve.retryAt); wait <= 0 || wait > serveBackoffMin {
		t.Fatalf("retry in %v, want within %v", wait, serveBackoffMin)
	}
	if got.maybeStartServe() != nil {
		t.Fatal("expected no restart before the backoff is up")
	}
}

func TestServeWhileModalOpen(t *testing.T) {
	m := serveModel(t)
	m.showPalette = true
	m = serve(t, m, data.ServeEvent{Connected: true})
	if !m.journal.serve.connected {
		t.Fatal("a stream event swallowed by the palette would strand the stream")
	}
}

func TestMaybeStartServe(t *testing.T) {
	m := liveModel(t, nil)
	if m.maybeStartServe() != nil {
		t.Fatal("expected no stream without MG_BD_SERVE")
	}
	m.journal.serveURL = "http://127.0.0.1:1" // nothing listens: the stream just ends
	if m.maybeStartServe() == nil || !m.journal.serve.running {
		t.Fatal("expected a stream once configured and live")
	}
	if m.maybeStartServe() != nil {
		t.Fatal("expected one stream at a time")
	}
	m.stopServe()

	off := newCLIModel(t, nil)
	off.journal.serveURL = "http://127.0.0.1:1"
	if off.maybeStartServe() != nil {
		t.Fatal("expected no stream before the journal is live")
	}
}

func TestNewJournalLoopServeURL(t *testing.T) {
	t.Setenv("MG_BD_SERVE", " http://127.0.0.1:28080 ")
	if j := newJournalLoop(data.SourceCLI, false); j.serveURL != "http://127.0.0.1:28080" {
		t.Fatalf("serveURL = %q", j.serveURL)
	}
	if j := newJournalLoop(data.SourceCLI, true); j.serveURL != "" {
		t.Fatal("expected no stream when the journal is opted out")
	}
}
