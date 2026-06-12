package gastown

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// gcDefaultBaseURL is a last-resort fallback. The machine-wide supervisor
// actually binds a *dynamically assigned* TCP port (not a fixed 8080) and logs
// it as "Supervisor API listening on http://host:port"; gcDiscoverBaseURL reads
// that. This default is only used when MG_GC_API requests discovery but the log
// can't be found.
const gcDefaultBaseURL = "http://127.0.0.1:8080"

// Environment variables that opt into and configure the Gas City driver.
const (
	// EnvGCAPI selects the Gas City HTTP driver instead of the Gas Town CLI.
	// Set it to a base URL (scheme://host:port, no /v0 suffix) to target a
	// specific supervisor, or to "auto" (any non-URL value) to discover the
	// live API port from ~/.gc/supervisor.log.
	EnvGCAPI = "MG_GC_API"
	// EnvGCCity optionally pins the city name; otherwise the driver picks the
	// first running city reported by GET /v0/cities.
	EnvGCCity = "MG_GC_CITY"
)

// GCEnabled reports whether the user has opted into the Gas City driver.
//
// Gas City is opt-in via MG_GC_API: mg defaults to the Gas Town CLI driver and
// only speaks the Supervisor HTTP API when explicitly told to, so a box with
// both `gt` and `gc` installed keeps its existing gt workflow untouched.
func GCEnabled() bool {
	return strings.TrimSpace(os.Getenv(EnvGCAPI)) != ""
}

// GCBaseURL resolves the Gas City supervisor base URL. If MG_GC_API is an
// explicit http(s) URL it is used verbatim; otherwise (e.g. MG_GC_API=auto) the
// live port is discovered from the supervisor log, falling back to the default.
func GCBaseURL() string {
	v := strings.TrimSpace(os.Getenv(EnvGCAPI))
	if strings.HasPrefix(v, "http://") || strings.HasPrefix(v, "https://") {
		return strings.TrimRight(v, "/")
	}
	if u := gcDiscoverBaseURL(); u != "" {
		return u
	}
	return gcDefaultBaseURL
}

// gcDiscoverBaseURL reads the machine-wide supervisor's log for the URL it
// reports binding. The API port is assigned dynamically at startup, so the log
// is the authoritative source. The control socket ~/.gc/supervisor.sock is a
// separate protocol and does NOT serve the /v0 HTTP API, so it can't be used
// as the base URL.
func gcDiscoverBaseURL() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(home, ".gc", "supervisor.log"))
	if err != nil {
		return ""
	}
	return gcParseSupervisorLog(string(data))
}

var gcListenRe = regexp.MustCompile(`(?i)listening on (https?://\S+)`)

// gcParseSupervisorLog returns the most recent "listening on <url>" address in
// the supervisor log, or "" if none is present.
func gcParseSupervisorLog(log string) string {
	matches := gcListenRe.FindAllStringSubmatch(log, -1)
	if len(matches) == 0 {
		return ""
	}
	return strings.TrimRight(matches[len(matches)-1][1], "/")
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
