package jev

import (
	"errors"
	"fmt"
	"time"
)

// HealthState is where the Jev circuit stands.
type HealthState int

const (
	// HealthHealthy: calls go through.
	HealthHealthy HealthState = iota
	// HealthDegraded: a few consecutive failures; calls still go through.
	HealthDegraded
	// HealthOpen: too many failures; calls are skipped until RetryAt.
	HealthOpen
	// HealthHalfOpen: the backoff elapsed; one probe call may go through.
	HealthHalfOpen
	// HealthDisabled: a permanent failure (rejected key, wrong URL) stopped
	// Jev for the session.
	HealthDisabled
)

// Thresholds and backoff for the circuit. The counts mirror
// data.SourceHealth; the backoff mirrors the bead stream's reconnect.
const (
	degradeThreshold = 3
	openThreshold    = 5
	openBackoffMin   = 30 * time.Second
	openBackoffMax   = 5 * time.Minute
)

// Health is the circuit breaker in front of a Jev client. Methods take value
// receivers and return the new Health, so it composes with BubbleTea's
// immutable model, and they take the time so tests can drive it.
type Health struct {
	State          HealthState
	ConsecFailures int
	LastError      error
	LastSuccess    time.Time
	RetryAt        time.Time     // when an Open circuit may probe again
	backoff        time.Duration // the current Open period
	// Notice is a one-line message for the user, set when a failure first
	// appears, when the circuit opens and when Jev is disabled. The reader
	// clears it after showing it; successes never set one.
	Notice string
}

// Allow reports whether a call may be made now. An Open circuit whose
// backoff has elapsed moves to HalfOpen and allows one probe.
func (h Health) Allow(now time.Time) (Health, bool) {
	switch h.State {
	case HealthDisabled:
		return h, false
	case HealthOpen:
		if now.Before(h.RetryAt) {
			return h, false
		}
		h.State = HealthHalfOpen
		return h, true
	case HealthHalfOpen:
		// The probe is out; no second one until it reports.
		return h, false
	default:
		return h, true
	}
}

// RecordSuccess closes the circuit.
func (h Health) RecordSuccess(now time.Time) Health {
	if h.State == HealthDisabled {
		return h
	}
	h.State = HealthHealthy
	h.ConsecFailures = 0
	h.LastError = nil
	h.LastSuccess = now
	h.RetryAt = time.Time{}
	h.backoff = 0
	return h
}

// RecordFailure counts a failed call and moves the circuit along. A
// *StatusError that is Permanent disables Jev for the session outright.
func (h Health) RecordFailure(err error, now time.Time) Health {
	if h.State == HealthDisabled {
		return h
	}
	h.LastError = err
	h.ConsecFailures++

	var se *StatusError
	if errors.As(err, &se) && se.Permanent() {
		h.State = HealthDisabled
		h.Notice = fmt.Sprintf("Jev disabled for this session: HTTP %d", se.Code)
		return h
	}

	switch h.State {
	case HealthHealthy, HealthDegraded:
		if h.ConsecFailures == 1 {
			h.Notice = "Jev request failed: " + short(err)
		}
		if h.ConsecFailures >= openThreshold {
			h.open(now)
		} else if h.ConsecFailures >= degradeThreshold {
			h.State = HealthDegraded
		}
	case HealthHalfOpen:
		// The probe failed: back off longer.
		h.open(now)
	}
	return h
}

func (h *Health) open(now time.Time) {
	if h.backoff == 0 {
		h.backoff = openBackoffMin
	} else {
		h.backoff = min(2*h.backoff, openBackoffMax)
	}
	h.State = HealthOpen
	h.RetryAt = now.Add(h.backoff)
	h.Notice = fmt.Sprintf("Jev paused after %d failures; retrying in %s", h.ConsecFailures, h.backoff)
}

// Label is the footer chip text for the state: "" while healthy.
func (h Health) Label() string {
	switch h.State {
	case HealthDegraded:
		return "degraded"
	case HealthOpen, HealthHalfOpen:
		return "paused"
	case HealthDisabled:
		return "off"
	default:
		return ""
	}
}

// Level is the chip's severity: 0 normal, 1 amber, 2 red.
func (h Health) Level() int {
	switch h.State {
	case HealthDegraded:
		return 1
	case HealthOpen, HealthHalfOpen, HealthDisabled:
		return 2
	default:
		return 0
	}
}

// String implements fmt.Stringer.
func (s HealthState) String() string {
	switch s {
	case HealthHealthy:
		return "healthy"
	case HealthDegraded:
		return "degraded"
	case HealthOpen:
		return "open"
	case HealthHalfOpen:
		return "half-open"
	case HealthDisabled:
		return "disabled"
	default:
		return "unknown"
	}
}

// short keeps an error to one readable line for a toast.
func short(err error) string {
	if err == nil {
		return ""
	}
	s := err.Error()
	if len(s) > 80 {
		s = s[:80] + "…"
	}
	return s
}
