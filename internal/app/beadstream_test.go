package app

import (
	"testing"
	"time"

	"github.com/matt-wright86/mardi-gras/internal/data"
	"github.com/matt-wright86/mardi-gras/internal/gastown"
)

// streamModel is a bd-backed model with a live bead stream whose channel the
// test owns.
func streamModel(t *testing.T) (m Model, events chan gastown.BeadEvent) {
	t.Helper()
	m = newCLIModel(t, []data.Issue{testIssue("a", data.StatusOpen)})
	events = make(chan gastown.BeadEvent, 1)
	model, cmd := m.Update(beadStreamStartedMsg{events: events, cancel: func() {}})
	if cmd == nil {
		t.Fatal("expected the stream to start being read")
	}
	return model.(Model), events
}

func TestBeadStreamChangeReloads(t *testing.T) {
	m, _ := streamModel(t)
	m.refresh.landedAt = time.Now().Add(-time.Minute)

	model, cmd := m.Update(beadStreamMsg{ev: gastown.BeadEvent{Kind: gastown.BeadChanged, BeadID: "a", IssueType: "task"}, ok: true})
	if !model.(Model).refresh.inFlight || cmd == nil {
		t.Fatal("expected a bead change to reload now and keep reading")
	}
}

func TestBeadStreamChangeSpaced(t *testing.T) {
	m, _ := streamModel(t)
	m.refresh.landedAt = time.Now() // a reload just landed

	model, _ := m.Update(beadStreamMsg{ev: gastown.BeadEvent{Kind: gastown.BeadChanged, BeadID: "a"}, ok: true})
	got := model.(Model)
	if got.refresh.inFlight {
		t.Fatal("expected the reload spaced, not immediate")
	}
	if due := time.Until(got.refresh.dueAt); due > beadStreamGap || due <= 0 {
		t.Fatalf("next reload due in %v, want within %v (sooner than the 5s poll)", due, beadStreamGap)
	}
}

func TestBeadStreamSkipsHiddenTypes(t *testing.T) {
	m, _ := streamModel(t)
	m.refresh.landedAt = time.Now().Add(-time.Minute)
	for _, ev := range []gastown.BeadEvent{
		{Kind: gastown.BeadChanged, BeadID: "s-1", IssueType: "session"},
		{Kind: gastown.BeadChanged, BeadID: "m-1", IssueType: "message"},
		{Kind: gastown.BeadChanged, BeadID: "w-1", Ephemeral: true},
	} {
		model, cmd := m.Update(beadStreamMsg{ev: ev, ok: true})
		if model.(Model).refresh.inFlight || cmd == nil {
			t.Errorf("%+v: want no reload, and the stream still read", ev)
		}
	}
}

func TestBeadStreamResumedReloads(t *testing.T) {
	m, _ := streamModel(t)
	m.refresh.landedAt = time.Now() // even right after a reload
	model, _ := m.Update(beadStreamMsg{ev: gastown.BeadEvent{Kind: gastown.BeadStreamResumed}, ok: true})
	if !model.(Model).refresh.inFlight {
		t.Fatal("expected a resumed stream to reload at once: it missed the gap")
	}
}

func TestBeadStreamClosed(t *testing.T) {
	m, _ := streamModel(t)
	model, cmd := m.Update(beadStreamMsg{ok: false})
	if cmd != nil || model.(Model).beadStream.events != nil {
		t.Fatal("expected a closed stream to stop being read")
	}
}

func TestBeadStreamWhileModalOpen(t *testing.T) {
	m, _ := streamModel(t)
	m.refresh.landedAt = time.Now().Add(-time.Minute)
	m.showPalette = true
	model, cmd := m.Update(beadStreamMsg{ev: gastown.BeadEvent{Kind: gastown.BeadChanged, BeadID: "a"}, ok: true})
	if cmd == nil || !model.(Model).refresh.inFlight {
		t.Fatal("a stream event swallowed by the palette would stop the stream being read")
	}
}

func TestStartBeadStream(t *testing.T) {
	m := newCLIModel(t, nil) // Gas Town driver: no stream
	if m.startBeadStream() != nil {
		t.Error("expected no stream without FeatureSSE")
	}
	gc, err := gastown.NewGCDriver("http://127.0.0.1:1", "bourbon")
	if err != nil {
		t.Fatal(err)
	}
	m.driver = gc
	if m.startBeadStream() == nil {
		t.Error("expected a stream from a Gas City driver reading through bd")
	}
	m.sourceMode = data.SourceJSONL
	if m.startBeadStream() != nil {
		t.Error("expected no stream when reading JSONL")
	}
}

func TestRefreshSpacedByKeepsSmallestGap(t *testing.T) {
	m := newCLIModel(t, nil)
	m.requestRefresh() // a fetch is in flight
	m.refreshSpaced()  // the journal: 5s
	m.refreshSpacedBy(beadStreamGap)
	m.refreshSpaced()
	if m.refresh.spacedGap != beadStreamGap {
		t.Fatalf("spacedGap = %v, want the smallest request (%v)", m.refresh.spacedGap, beadStreamGap)
	}
}
