package jev

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// Environment variables. Jev is strictly opt-in: only EnvAPIKey turns it on.
// The key is never a flag, since flags show in ps.
const (
	// EnvAPIKey holds the bearer token. Set, Jev is enabled; unset, mg never
	// contacts a Jev server.
	EnvAPIKey = "MG_JEV_API_KEY" //nolint:gosec // the variable's name, not a credential
	// EnvURL names an API-compatible server (OpenJev, Von, tensai) instead of
	// the hosted endpoint. A host alone gets /v1/systemone appended.
	EnvURL = "MG_JEV_URL"
	// EnvModel overrides the model route (default DefaultModel).
	EnvModel = "MG_JEV_MODEL"
	// EnvToggle set to "off" disables Jev for this run even with a key, the
	// same idiom as MG_EVENTS=off. The --no-jev flag sets it.
	EnvToggle = "MG_JEV"
	// EnvScope picks how much of an issue is sent: "minimal" (title and
	// structure) or "standard" (plus a truncated description, the default).
	// The app parses it; see data.ParseSnapshotScope.
	EnvScope = "MG_JEV_SCOPE"
)

// defaultTimeoutShort is the per-attempt budget. Jev answers in well under a
// second, so a request past this tier has already failed.
const defaultTimeoutShort = 5 * time.Second

var timeoutShort = defaultTimeoutShort

// SetCmdTimeout scales the request timeout the way data.SetCmdTimeout and
// gastown.SetCmdTimeout scale theirs: seconds is relative to a 30s baseline.
//
// SAFETY: call during program initialization, before any client is built.
func SetCmdTimeout(seconds int) {
	if seconds <= 0 {
		return
	}
	scale := float64(seconds) / 30.0
	timeoutShort = time.Duration(float64(defaultTimeoutShort) * scale)
}

// Timeout returns the current per-attempt request timeout.
func Timeout() time.Duration { return timeoutShort }

// FromEnv builds the client the environment describes, or returns nil, nil
// when Jev is not enabled: no MG_JEV_API_KEY, or MG_JEV=off. An error means
// the operator asked for Jev but misconfigured it (a bad URL); callers should
// say so once and run without it.
func FromEnv() (*Client, error) {
	return fromEnv(os.Getenv)
}

func fromEnv(getenv func(string) string) (*Client, error) {
	if strings.EqualFold(strings.TrimSpace(getenv(EnvToggle)), "off") {
		return nil, nil
	}
	key := strings.TrimSpace(getenv(EnvAPIKey))
	if key == "" {
		return nil, nil
	}
	var opts []ClientOption
	if u := strings.TrimSpace(getenv(EnvURL)); u != "" {
		opts = append(opts, WithURL(u))
	}
	if m := strings.TrimSpace(getenv(EnvModel)); m != "" {
		opts = append(opts, WithModel(m))
	}
	c, err := NewClient(key, opts...)
	if err != nil {
		return nil, fmt.Errorf("%w (from %s)", err, EnvURL)
	}
	return c, nil
}
