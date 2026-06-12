package gastown

import (
	"os"
	"strings"
)

// gcDefaultBaseURL is the machine-wide Gas City supervisor default
// (supervisor.toml: port = 8080, bind = "127.0.0.1").
const gcDefaultBaseURL = "http://127.0.0.1:8080"

// Environment variables that opt into and configure the Gas City driver.
const (
	// EnvGCAPI, when set to a non-empty base URL (scheme://host:port, no /v0
	// suffix), selects the Gas City HTTP driver instead of the Gas Town CLI.
	EnvGCAPI = "MG_GC_API"
	// EnvGCCity optionally pins the city name; otherwise the driver picks the
	// first running city reported by GET /v0/cities.
	EnvGCCity = "MG_GC_CITY"
)

// GCEnabled reports whether the user has opted into the Gas City driver.
//
// Gas City is opt-in via MG_GC_API for now: auto-detecting a reachable
// supervisor (and resolving the port from supervisor.toml / city.toml) needs
// validation against a live `gc` install, so until then mg defaults to the
// Gas Town CLI driver and only speaks HTTP when explicitly told to.
func GCEnabled() bool {
	return strings.TrimSpace(os.Getenv(EnvGCAPI)) != ""
}

// GCBaseURL returns the configured Gas City supervisor base URL, or the
// machine-wide default when MG_GC_API is unset/blank.
func GCBaseURL() string {
	if b := strings.TrimSpace(os.Getenv(EnvGCAPI)); b != "" {
		return strings.TrimRight(b, "/")
	}
	return gcDefaultBaseURL
}

// SelectDriver returns the orchestrator driver mg should use. It defaults to
// the Gas Town CLI driver and returns a Gas City HTTP driver only when
// GCEnabled() is true. If the Gas City client cannot be constructed (bad base
// URL), it falls back to the Gas Town driver so mg still runs.
func SelectDriver() Driver {
	if !GCEnabled() {
		return NewGTDriver()
	}
	d, err := NewGCDriver(GCBaseURL(), strings.TrimSpace(os.Getenv(EnvGCCity)))
	if err != nil {
		return NewGTDriver()
	}
	return d
}
