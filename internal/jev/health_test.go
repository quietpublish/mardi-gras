package jev

import (
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"
)

var t0 = time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)

func failN(h Health, n int, err error, now time.Time) Health {
	for range n {
		h = h.RecordFailure(err, now)
	}
	return h
}

func TestHealthDegradesThenOpens(t *testing.T) {
	var h Health
	transient := errors.New("jev: dial tcp: connection refused")

	h = failN(h, 1, transient, t0)
	if h.State != HealthHealthy || !strings.Contains(h.Notice, "failed") {
		t.Fatalf("after 1 failure: %s, notice %q", h.State, h.Notice)
	}
	h.Notice = ""

	h = failN(h, 2, transient, t0)
	if h.State != HealthDegraded || h.Notice != "" {
		t.Fatalf("after 3 failures: %s, notice %q (only the first failure toasts)", h.State, h.Notice)
	}
	if _, ok := h.Allow(t0); !ok {
		t.Fatal("degraded must still allow calls")
	}

	h = failN(h, 2, transient, t0)
	if h.State != HealthOpen || !strings.Contains(h.Notice, "paused") {
		t.Fatalf("after 5 failures: %s, notice %q", h.State, h.Notice)
	}
	if h.Label() != "paused" || h.Level() != 2 {
		t.Fatalf("label %q level %d", h.Label(), h.Level())
	}
	if _, ok := h.Allow(t0.Add(openBackoffMin - time.Second)); ok {
		t.Fatal("open circuit allowed a call before RetryAt")
	}

	h, ok := h.Allow(t0.Add(openBackoffMin))
	if !ok || h.State != HealthHalfOpen {
		t.Fatalf("at RetryAt: allowed %v, state %s", ok, h.State)
	}
	if _, ok := h.Allow(t0.Add(openBackoffMin)); ok {
		t.Fatal("half-open must allow exactly one probe")
	}

	// The probe fails: open again, backoff doubled.
	h = h.RecordFailure(transient, t0.Add(openBackoffMin))
	if h.State != HealthOpen || h.backoff != 2*openBackoffMin {
		t.Fatalf("after failed probe: %s, backoff %v", h.State, h.backoff)
	}

	// The next probe succeeds: fully closed.
	h, _ = h.Allow(h.RetryAt)
	h = h.RecordSuccess(h.RetryAt)
	if h.State != HealthHealthy || h.ConsecFailures != 0 || h.backoff != 0 || h.Label() != "" {
		t.Fatalf("after success: %+v", h)
	}
}

func TestHealthBackoffCaps(t *testing.T) {
	var h Health
	err := errors.New("x")
	h = failN(h, openThreshold, err, t0)
	for range 10 {
		h, _ = h.Allow(h.RetryAt)
		h = h.RecordFailure(err, h.RetryAt)
	}
	if h.backoff != openBackoffMax {
		t.Fatalf("backoff %v, want capped at %v", h.backoff, openBackoffMax)
	}
}

func TestHealthEdgeCaseSuccessResets(t *testing.T) {
	var h Health
	h = failN(h, degradeThreshold, errors.New("x"), t0)
	h = h.RecordSuccess(t0)
	if h.State != HealthHealthy || h.ConsecFailures != 0 || h.LastSuccess != t0 {
		t.Fatalf("%+v", h)
	}
	// A fresh first failure toasts again.
	h.Notice = ""
	h = h.RecordFailure(errors.New("y"), t0)
	if h.Notice == "" {
		t.Fatal("first failure after recovery should notice")
	}
}

func TestHealthEdgeCasePermanentDisables(t *testing.T) {
	var h Health
	h = h.RecordFailure(&StatusError{Code: http.StatusUnauthorized}, t0)
	if h.State != HealthDisabled || !strings.Contains(h.Notice, "401") {
		t.Fatalf("%s, notice %q", h.State, h.Notice)
	}
	if _, ok := h.Allow(t0.Add(time.Hour)); ok {
		t.Fatal("disabled must never allow")
	}
	if h.RecordSuccess(t0).State != HealthDisabled {
		t.Fatal("disabled is for the session")
	}
	if h.Label() != "off" || h.Level() != 2 {
		t.Fatalf("label %q level %d", h.Label(), h.Level())
	}
}

func TestHealthEdgeCaseTransientStatusIsNotPermanent(t *testing.T) {
	var h Health
	for _, code := range []int{http.StatusBadRequest, http.StatusUnprocessableEntity, http.StatusTooManyRequests, 529, http.StatusInternalServerError} {
		h = Health{}.RecordFailure(&StatusError{Code: code}, t0)
		if h.State == HealthDisabled {
			t.Errorf("HTTP %d disabled Jev; only 401/403/404 should", code)
		}
	}
}
