package data

import (
	"errors"
	"testing"
	"time"
)

var t0 = time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)

func rec(seq int64, id string) JournalRecord {
	return JournalRecord{Seq: seq, TS: "t", Op: "update", IssueID: id, Issue: &JournalIssue{}}
}

func wisp(seq int64, id string) JournalRecord {
	return JournalRecord{Seq: seq, TS: "t", Op: "create", IssueID: id, Issue: &JournalIssue{Ephemeral: true}}
}

// following returns a follower that has found head 10 at t0 and absorbed its
// baseline reload, ready for reloads to be judged.
func following(t *testing.T) JournalFollower {
	t.Helper()
	f, _ := NewJournalFollower(false)
	f, _ = f.Enabled(true, nil, t0)
	ref := rec(10, "x")
	f, act := f.HeadFound(JournalAnchor{Seq: 10, Ref: &ref}, nil, t0)
	if f.Phase != JournalFollowing || act != JournalRefresh {
		t.Fatalf("setup: phase %v action %v", f.Phase, act)
	}
	return f.Reloaded(nil, t0, t0.Add(time.Second))
}

func TestNewJournalFollower(t *testing.T) {
	f, act := NewJournalFollower(false)
	if f.Phase != JournalDetecting || act != JournalCheckEnabled {
		t.Fatalf("got %v, %v; want detecting and a check", f.Phase, act)
	}
}

func TestNewJournalFollowerOptedOut(t *testing.T) {
	f, act := NewJournalFollower(true)
	if f.Phase != JournalOff || act != JournalNoAction {
		t.Fatalf("got %v, %v; want off", f.Phase, act)
	}
	if f, act = f.Tick(t0.Add(24 * time.Hour)); f.Phase != JournalOff || act != JournalNoAction {
		t.Fatal("an opted-out follower must never re-detect")
	}
}

func TestJournalFollowerEnabled(t *testing.T) {
	f, _ := NewJournalFollower(false)
	if got, act := f.Enabled(true, nil, t0); got.Phase != JournalBaselining || act != JournalFindHead {
		t.Errorf("enabled: got %v, %v; want baselining and a head search", got.Phase, act)
	}
	if got, _ := f.Enabled(false, errors.New("boom"), t0); got.Phase != JournalBackoff || !got.RetryAt.Equal(t0.Add(journalRetryMin)) {
		t.Errorf("error: got %v retry %v; want backoff for %v", got.Phase, got.RetryAt, journalRetryMin)
	}
}

func TestJournalFollowerEnabledDisabled(t *testing.T) {
	f, _ := NewJournalFollower(false)
	f, act := f.Enabled(false, nil, t0)
	if f.Phase != JournalOff || act != JournalNoAction {
		t.Fatalf("got %v, %v; want off", f.Phase, act)
	}
	if _, act := f.Tick(t0.Add(journalRecheckEvery - time.Second)); act != JournalNoAction {
		t.Fatal("expected no re-check before the interval")
	}
	f, act = f.Tick(t0.Add(journalRecheckEvery))
	if f.Phase != JournalDetecting || act != JournalCheckEnabled {
		t.Fatalf("got %v, %v; want a re-check once the interval passes", f.Phase, act)
	}
}

func TestJournalFollowerHeadFound(t *testing.T) {
	f, _ := NewJournalFollower(false)
	f, _ = f.Enabled(true, nil, t0)

	if got, _ := f.HeadFound(JournalAnchor{}, errors.New("boom"), t0); got.Phase != JournalBackoff {
		t.Errorf("error: got %v, want backoff", got.Phase)
	}
	got, act := f.HeadFound(JournalAnchor{}, ErrJournalUnsupported, t0)
	if got.Phase != JournalOff || act != JournalNoAction {
		t.Errorf("unsupported: got %v, %v; want off", got.Phase, act)
	}
	if got, _ = got.Tick(t0.Add(24 * time.Hour)); got.Phase != JournalOff {
		t.Error("a bd without the journal must not be re-detected")
	}
	got, act = f.HeadFound(JournalAnchor{Seq: 7}, nil, t0)
	if !got.Live() || got.Anchor.Seq != 7 || act != JournalRefresh {
		t.Errorf("found: got %v anchor %d, %v; want following from 7 with a reload", got.Phase, got.Anchor.Seq, act)
	}
}

func TestJournalFollowerProbed(t *testing.T) {
	f := following(t)

	got, act := f.Probed(nil, nil, t0, t0)
	if act != JournalNoAction || got.Anchor.Seq != 10 {
		t.Errorf("nothing new: got %v, anchor %d", act, got.Anchor.Seq)
	}
	got, act = f.Probed([]JournalRecord{rec(11, "a"), rec(12, "b")}, nil, t0, t0)
	if act != JournalRefresh || got.Anchor.Seq != 12 || got.Anchor.Ref == nil || got.Anchor.Ref.IssueID != "b" {
		t.Errorf("new records: got %v, anchor %+v; want a reload and the anchor on 12", act, got.Anchor)
	}
	got, act = f.Probed([]JournalRecord{wisp(11, "w")}, nil, t0, t0)
	if act != JournalNoAction || got.Anchor.Seq != 11 {
		t.Errorf("wisps only: got %v, anchor %d; want no reload but the anchor moved", act, got.Anchor.Seq)
	}
}

func TestJournalFollowerProbedFullPage(t *testing.T) {
	f := following(t)
	page := make([]JournalRecord, JournalProbeLimit)
	for i := range page {
		page[i] = rec(int64(11+i), "a")
	}
	// A backlog: re-anchor at the head, whose reload covers it, rather than
	// page through it.
	got, act := f.Probed(page, nil, t0, t0)
	if got.Phase != JournalBaselining || act != JournalFindHead {
		t.Fatalf("got %v, %v; want a head search", got.Phase, act)
	}
	if got, act = got.HeadFound(JournalAnchor{Seq: 900}, nil, t0); act != JournalRefresh {
		t.Fatalf("head found: got %v, want the reload that covers the backlog", act)
	}
}

func TestJournalFollowerProbedResync(t *testing.T) {
	for name, err := range map[string]error{
		"reset":     ErrJournalReset,
		"truncated": &JournalTruncatedError{Since: 9, Floor: 20, Head: 30},
	} {
		t.Run(name, func(t *testing.T) {
			got, act := following(t).Probed(nil, err, t0, t0)
			if got.Phase != JournalBaselining || act != JournalFindHead {
				t.Fatalf("got %v, %v; want a new head search", got.Phase, act)
			}
		})
	}
}

func TestJournalFollowerProbedUnsupported(t *testing.T) {
	got, _ := following(t).Probed(nil, ErrJournalUnsupported, t0, t0)
	if got.Phase != JournalOff {
		t.Fatalf("got %v, want off", got.Phase)
	}
}

func TestJournalFollowerProbedFailures(t *testing.T) {
	f := following(t)
	boom := errors.New("timeout")
	for i := 1; i < journalMaxFailures; i++ {
		if f, _ = f.Probed(nil, boom, t0, t0); !f.Live() {
			t.Fatalf("backed off after %d failures, want %d", i, journalMaxFailures)
		}
	}
	if f, _ = f.Probed(nil, boom, t0, t0); f.Phase != JournalBackoff {
		t.Fatalf("got %v after %d failures, want backoff", f.Phase, journalMaxFailures)
	}
	f, act := f.Tick(f.RetryAt)
	if f.Phase != JournalDetecting || act != JournalCheckEnabled {
		t.Fatalf("got %v, %v; want detection at RetryAt", f.Phase, act)
	}
}

func TestJournalFollowerBackoffDoubles(t *testing.T) {
	f, _ := NewJournalFollower(false)
	now := t0
	var delays []time.Duration
	for range 5 {
		f, _ = f.Enabled(false, errors.New("boom"), now)
		delays = append(delays, f.RetryAt.Sub(now))
		now = f.RetryAt
		f, _ = f.Tick(now)
	}
	want := []time.Duration{time.Minute, 2 * time.Minute, 4 * time.Minute, 5 * time.Minute, 5 * time.Minute}
	for i := range want {
		if delays[i] != want[i] {
			t.Fatalf("delays = %v, want %v", delays, want)
		}
	}
	// A successful baseline resets the backoff.
	f, _ = f.Enabled(true, nil, now)
	f, _ = f.HeadFound(JournalAnchor{}, nil, now)
	f, _ = f.Probed(nil, errors.New("a"), now, now)
	f, _ = f.Probed(nil, errors.New("b"), now, now)
	f, _ = f.Probed(nil, errors.New("c"), now, now)
	if got := f.RetryAt.Sub(now); got != journalRetryMin {
		t.Fatalf("delay after recovery = %v, want %v", got, journalRetryMin)
	}
}

func TestJournalFollowerPartial(t *testing.T) {
	f := following(t)
	now := t0.Add(time.Minute)

	// A change whose record was probed is explained.
	f, _ = f.Probed([]JournalRecord{rec(11, "a")}, nil, now, now)
	f = f.Reloaded([]string{"a"}, now, now)
	f, _ = f.Probed(nil, nil, now, now)
	if f.Partial {
		t.Fatal("a journaled change must not mark the journal partial")
	}

	// A change whose record lands in the next probe is explained too.
	f = f.Reloaded([]string{"b"}, now, now)
	f, _ = f.Probed([]JournalRecord{rec(12, "b")}, nil, now, now)
	if f.Partial {
		t.Fatal("a change explained by the next probe must not mark the journal partial")
	}

	// A change no record ever explains marks it partial.
	f = f.Reloaded([]string{"c"}, now, now)
	f, _ = f.Probed(nil, nil, now, now)
	if !f.Partial {
		t.Fatal("expected an unexplained change to mark the journal partial")
	}

	// Partial decays after a quiet spell.
	f, _ = f.Probed(nil, nil, now.Add(journalPartialDecay-time.Second), now.Add(journalPartialDecay-time.Second))
	if !f.Partial {
		t.Fatal("partial cleared too early")
	}
	f, _ = f.Probed(nil, nil, now.Add(journalPartialDecay), now.Add(journalPartialDecay))
	if f.Partial {
		t.Fatal("expected partial to clear after a quiet spell")
	}
}

func TestJournalFollowerPartialSeenWindow(t *testing.T) {
	f := following(t)
	f, _ = f.Probed([]JournalRecord{rec(11, "a")}, nil, t0, t0)

	// A record probed long ago does not explain a fresh change to that issue.
	later := t0.Add(journalSeenWindow + time.Second)
	f = f.Reloaded([]string{"a"}, later, later)
	f, _ = f.Probed(nil, nil, later, later)
	if !f.Partial {
		t.Fatal("a stale record must not explain a new change")
	}
}

func TestJournalFollowerReloadedBaseline(t *testing.T) {
	f, _ := NewJournalFollower(false)
	f, _ = f.Enabled(true, nil, t0)
	f, _ = f.HeadFound(JournalAnchor{Seq: 10}, nil, t0)

	// A reload that started before the head search is not judged, and does
	// not count as the baseline.
	f = f.Reloaded([]string{"early"}, t0.Add(-time.Second), t0)
	// The first reload after it shows changes recorded below the anchor.
	f = f.Reloaded([]string{"below-anchor"}, t0, t0.Add(time.Second))
	f, _ = f.Probed(nil, nil, t0.Add(2*time.Second), t0.Add(2*time.Second))
	if f.Partial {
		t.Fatal("the baseline reload must not be judged")
	}
	// Reloads after the baseline are.
	f = f.Reloaded([]string{"unjournaled"}, t0.Add(3*time.Second), t0.Add(3*time.Second))
	f, _ = f.Probed(nil, nil, t0.Add(4*time.Second), t0.Add(4*time.Second))
	if !f.Partial {
		t.Fatal("expected reloads after the baseline to be judged")
	}
}

func TestJournalFollowerRecheck(t *testing.T) {
	f := following(t)
	if _, act := f.Tick(t0.Add(journalRecheckEvery - time.Second)); act != JournalNoAction {
		t.Fatal("expected no re-check before the interval")
	}
	f, act := f.Tick(t0.Add(journalRecheckEvery))
	if act != JournalCheckEnabled || !f.Live() {
		t.Fatalf("got %v, %v; want a re-check while still following", f.Phase, act)
	}
	if got, _ := f.Enabled(false, errors.New("boom"), t0); !got.Live() {
		t.Error("a failed re-check must not stop following")
	}
	if got, _ := f.Enabled(true, nil, t0); !got.Live() {
		t.Error("a passing re-check must keep following")
	}
	if got, _ := f.Enabled(false, nil, t0); got.Phase != JournalOff {
		t.Errorf("journal turned off: got %v, want off", got.Phase)
	}
}

func TestJournalFollowerStaleResults(t *testing.T) {
	f := following(t)
	if got, act := f.HeadFound(JournalAnchor{Seq: 99}, nil, t0); act != JournalNoAction || got.Anchor.Seq != 10 {
		t.Error("a head search result outside baselining must be ignored")
	}
	off, _ := NewJournalFollower(true)
	if got, act := off.Probed([]JournalRecord{rec(1, "a")}, nil, t0, t0); act != JournalNoAction || got.Phase != JournalOff {
		t.Error("a probe result outside following must be ignored")
	}
}

func TestJournalFollowerSuspectsWaitForLaterProbe(t *testing.T) {
	f := following(t)
	reloadDone := t0.Add(time.Minute)
	f = f.Reloaded([]string{"x"}, reloadDone.Add(-time.Second), reloadDone)

	// A probe that started before the reload landed may predate x's record.
	f, _ = f.Probed(nil, nil, reloadDone.Add(-500*time.Millisecond), reloadDone.Add(time.Second))
	if f.Partial {
		t.Fatal("a probe that started before the reload landed must not judge it")
	}
	// The next probe, started after, returns x's record: explained.
	f, _ = f.Probed([]JournalRecord{rec(11, "x")}, nil, reloadDone.Add(2*time.Second), reloadDone.Add(2*time.Second))
	if f.Partial {
		t.Fatal("expected x to be explained by the later probe")
	}
}

func TestJournalFollowerProbeInterval(t *testing.T) {
	f := following(t) // head found at t0
	if got := f.ProbeInterval(t0.Add(time.Second)); got != journalActiveProbe {
		t.Fatalf("just after the head search: %v, want %v", got, journalActiveProbe)
	}
	quiet := t0.Add(journalActiveWindow)
	if got := f.ProbeInterval(quiet); got != journalIdleProbe {
		t.Fatalf("after a quiet minute: %v, want %v", got, journalIdleProbe)
	}

	// A record brings the fast cadence back.
	f, _ = f.Probed([]JournalRecord{rec(11, "a")}, nil, quiet, quiet)
	if got := f.ProbeInterval(quiet.Add(time.Second)); got != journalActiveProbe {
		t.Fatalf("after activity: %v, want %v", got, journalActiveProbe)
	}
	// An empty probe does not.
	later := quiet.Add(journalActiveWindow)
	f, _ = f.Probed(nil, nil, later, later)
	if got := f.ProbeInterval(later); got != journalIdleProbe {
		t.Fatalf("empty probes keep it idle: %v, want %v", got, journalIdleProbe)
	}
}
