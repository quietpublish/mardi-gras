package data

import (
	"errors"
	"time"
)

// JournalPhase is where mg is in using the events journal as its change
// signal.
type JournalPhase int

const (
	JournalOff        JournalPhase = iota // not in use; reloads poll as before
	JournalDetecting                      // asking bd whether the journal is enabled
	JournalBaselining                     // finding the newest record to follow from
	JournalFollowing                      // probing for records newer than Anchor
	JournalBackoff                        // bd calls failed; polling as before until RetryAt
)

// JournalAction is what the app should do after feeding the follower an
// event.
type JournalAction int

const (
	JournalNoAction     JournalAction = iota
	JournalCheckEnabled               // run JournalEnabled and feed Enabled
	JournalFindHead                   // run FindJournalHead and feed HeadFound
	JournalRefresh                    // reload issues (bd list)
	JournalProbeNow                   // probe again without waiting: a full page came back
)

// Tuning for the journal follower.
const (
	JournalProbeInterval = 2 * time.Second // between probes while following
	JournalProbeLimit    = 500             // records per probe

	journalMaxFailures  = 3               // consecutive probe failures before backing off
	journalRetryMin     = time.Minute     // first backoff delay, doubling
	journalRetryMax     = 5 * time.Minute // backoff ceiling
	journalRecheckEvery = 5 * time.Minute // re-ask bd whether the journal is (still) enabled
	journalSeenWindow   = 2 * time.Minute // how long a probed record explains a reload's change
	journalPartialDecay = 5 * time.Minute // quiet time before Partial clears
)

// JournalFollower tracks mg's use of the bd events journal. It is a pure
// value type, like SourceHealth: the app owns the timers and the bd calls,
// feeds their results in, and carries out the returned JournalAction.
//
// The journal is only ever a trigger. A probe that finds records asks for a
// reload; issue state always comes from bd list.
type JournalFollower struct {
	Phase  JournalPhase
	Anchor JournalAnchor // Following: the newest record accounted for

	// Partial is set when a reload shows a change no journal record
	// explains: a Go-library write, a dolt pull, a writer with the journal
	// off. The app then keeps its backstop reload at the legacy rate. It
	// clears after journalPartialDecay without another unexplained change.
	Partial bool

	// RetryAt is when to leave Backoff, or when to re-ask bd whether a
	// disabled journal has been turned on.
	RetryAt time.Time

	permanent  bool      // Off for good: opted out, or bd has no journal
	recheckAt  time.Time // Following: when to re-ask bd whether the journal is still on
	baselineAt time.Time // when the current anchor came from a head search
	// baselined is set once a reload started after baselineAt has landed.
	// Only reloads after that one are diffed against a snapshot that is
	// itself newer than the anchor, so only they can be judged.
	baselined  bool
	failures   int
	retryDelay time.Duration

	// seen maps issue IDs named by probed records to when they were probed.
	// suspects holds IDs a reload changed that nothing in seen explained;
	// the next probe is their last chance.
	seen            map[string]time.Time
	suspects        map[string]bool
	lastUnexplained time.Time
}

// NewJournalFollower starts detection, or stays off for the session when the
// user opted out.
func NewJournalFollower(optedOut bool) (JournalFollower, JournalAction) {
	if optedOut {
		return JournalFollower{Phase: JournalOff, permanent: true}, JournalNoAction
	}
	return JournalFollower{Phase: JournalDetecting}, JournalCheckEnabled
}

// Live reports whether reloads are currently driven by the journal.
func (f JournalFollower) Live() bool {
	return f.Phase == JournalFollowing
}

// Enabled feeds the result of JournalEnabled.
func (f JournalFollower) Enabled(on bool, err error, now time.Time) (JournalFollower, JournalAction) {
	switch f.Phase {
	case JournalDetecting:
		if err != nil {
			return f.backoff(now)
		}
		if !on {
			return f.off(now), JournalNoAction
		}
		f.Phase = JournalBaselining
		return f, JournalFindHead
	case JournalFollowing:
		// The periodic re-check. A failed check proves nothing; keep going.
		f.recheckAt = now.Add(journalRecheckEvery)
		if err == nil && !on {
			return f.off(now), JournalNoAction
		}
	}
	return f, JournalNoAction
}

// HeadFound feeds the result of FindJournalHead. Following starts with a
// reload so that everything at or below the anchor is on screen; every
// record after it will be probed.
func (f JournalFollower) HeadFound(anchor JournalAnchor, err error, now time.Time) (JournalFollower, JournalAction) {
	if f.Phase != JournalBaselining {
		return f, JournalNoAction
	}
	if errors.Is(err, ErrJournalUnsupported) {
		return JournalFollower{Phase: JournalOff, permanent: true}, JournalNoAction
	}
	if err != nil {
		return f.backoff(now)
	}
	f.Phase = JournalFollowing
	f.Anchor = anchor
	f.baselineAt = now
	f.baselined = false
	f.recheckAt = now.Add(journalRecheckEvery)
	f.failures = 0
	f.retryDelay = 0
	f.seen = nil
	f.suspects = nil
	return f, JournalRefresh
}

// Probed feeds the result of ProbeJournal from the current anchor.
func (f JournalFollower) Probed(records []JournalRecord, err error, now time.Time) (JournalFollower, JournalAction) {
	if f.Phase != JournalFollowing {
		return f, JournalNoAction
	}
	var trunc *JournalTruncatedError
	switch {
	case errors.Is(err, ErrJournalUnsupported):
		// bd was downgraded under a running mg.
		return JournalFollower{Phase: JournalOff, permanent: true}, JournalNoAction
	case errors.Is(err, ErrJournalReset), errors.As(err, &trunc):
		// The sequence mg followed is gone or pruned past: start over.
		f.Phase = JournalBaselining
		return f, JournalFindHead
	case err != nil:
		f.failures++
		if f.failures >= journalMaxFailures {
			return f.backoff(now)
		}
		return f, JournalNoAction
	}

	f.failures = 0
	f.Anchor = f.Anchor.AnchorAt(records)
	relevant := false
	for _, r := range records {
		if r.Ephemeral() {
			continue // wisps never reach bd list
		}
		relevant = true
		if f.seen == nil {
			f.seen = make(map[string]time.Time)
		}
		f.seen[r.IssueID] = now
		delete(f.suspects, r.IssueID)
	}
	if len(f.suspects) > 0 {
		f.Partial = true
		f.lastUnexplained = now
	} else if f.Partial && now.Sub(f.lastUnexplained) >= journalPartialDecay {
		f.Partial = false
	}
	f.suspects = nil

	switch {
	case len(records) >= JournalProbeLimit:
		return f, JournalProbeNow
	case relevant:
		return f, JournalRefresh
	}
	return f, JournalNoAction
}

// Reloaded feeds the issues a completed reload changed (added, edited or
// removed) and when that reload started. Changes no probed record explains
// become suspects for the next probe.
func (f JournalFollower) Reloaded(changed []string, startedAt, now time.Time) JournalFollower {
	if f.Phase != JournalFollowing || startedAt.Before(f.baselineAt) {
		// A reload that started before the head search can show changes
		// recorded at or below the anchor, which no probe will return.
		return f
	}
	if !f.baselined {
		// The first reload after the head search is diffed against a
		// snapshot from before the anchor, so the same holds; it becomes
		// the baseline the next reload is judged against.
		f.baselined = true
		return f
	}
	for id, at := range f.seen {
		if now.Sub(at) > journalSeenWindow {
			delete(f.seen, id)
		}
	}
	for _, id := range changed {
		if _, ok := f.seen[id]; ok {
			continue
		}
		if f.suspects == nil {
			f.suspects = make(map[string]bool)
		}
		f.suspects[id] = true
	}
	return f
}

// Tick lets the follower act on the clock: leave Backoff, re-ask bd about a
// disabled journal, or re-check that a followed journal is still enabled.
func (f JournalFollower) Tick(now time.Time) (JournalFollower, JournalAction) {
	switch f.Phase {
	case JournalBackoff:
		if !now.Before(f.RetryAt) {
			f.Phase = JournalDetecting
			return f, JournalCheckEnabled
		}
	case JournalOff:
		if !f.permanent && !f.RetryAt.IsZero() && !now.Before(f.RetryAt) {
			f.Phase = JournalDetecting
			return f, JournalCheckEnabled
		}
	case JournalFollowing:
		if !now.Before(f.recheckAt) {
			f.recheckAt = now.Add(journalRecheckEvery)
			return f, JournalCheckEnabled
		}
	}
	return f, JournalNoAction
}

// off parks the follower until the journal might have been turned on.
func (f JournalFollower) off(now time.Time) JournalFollower {
	return JournalFollower{Phase: JournalOff, RetryAt: now.Add(journalRecheckEvery)}
}

// backoff parks the follower after failures, doubling the delay each time.
func (f JournalFollower) backoff(now time.Time) (JournalFollower, JournalAction) {
	switch {
	case f.retryDelay == 0:
		f.retryDelay = journalRetryMin
	case f.retryDelay < journalRetryMax:
		f.retryDelay = min(2*f.retryDelay, journalRetryMax)
	}
	f.Phase = JournalBackoff
	f.RetryAt = now.Add(f.retryDelay)
	f.failures = 0
	f.Partial = false
	return f, JournalNoAction
}
