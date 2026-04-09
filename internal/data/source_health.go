package data

import (
	"os"
	"path/filepath"
	"time"
)

// HealthState is the current resilience state of the data source.
type HealthState int

const (
	HealthHealthy    HealthState = iota // All good; primary source is working.
	HealthDegraded                      // Consecutive failures; approaching fallback.
	HealthFallback                      // Switched to JSONL fallback; probing for CLI recovery.
	HealthRecovering                    // CLI responding again; counting successes before declaring healthy.
)

// Thresholds for state machine transitions.
const (
	degradeThreshold  = 3         // Consecutive failures to enter degraded.
	recoverThreshold  = 2         // Consecutive CLI successes to recover from fallback.
	jsonlMaxAge       = time.Hour // Refuse JSONL fallback if file is older than this.
	stalenessAmberAge = 30 * time.Second
	stalenessRedAge   = 2 * time.Minute

	// DegradeThreshold is the number of consecutive failures before the source
	// transitions to degraded state. Exported for use by the app package.
	DegradeThreshold = degradeThreshold
)

// SourceHealth tracks the resilience state of the issue data source.
// All state-mutating methods use value receivers and return a new SourceHealth,
// making the state machine safe to use with BubbleTea's immutable model pattern.
type SourceHealth struct {
	State           HealthState
	ConsecFailures  int
	ConsecSuccesses int
	FirstFailure    time.Time
	LastSuccess     time.Time
	LastError       error
}

// RecordFailure records a source fetch failure and transitions state as needed.
func (h SourceHealth) RecordFailure(err error) SourceHealth {
	h.LastError = err
	h.ConsecSuccesses = 0
	if h.ConsecFailures == 0 {
		h.FirstFailure = time.Now()
	}
	h.ConsecFailures++
	if h.State == HealthHealthy && h.ConsecFailures >= degradeThreshold {
		h.State = HealthDegraded
	}
	return h
}

// RecordSuccess records a successful source fetch and transitions state as needed.
func (h SourceHealth) RecordSuccess() SourceHealth {
	h.ConsecFailures = 0
	h.LastSuccess = time.Now()
	switch h.State {
	case HealthDegraded:
		// CLI is working again before we reached fallback.
		h.State = HealthHealthy
		h.ConsecSuccesses = 0
	case HealthFallback:
		h.State = HealthRecovering
		h.ConsecSuccesses = 1
	case HealthRecovering:
		h.ConsecSuccesses++
		if h.ConsecSuccesses >= recoverThreshold {
			h.State = HealthHealthy
			h.ConsecSuccesses = 0
		}
	default:
		// Already healthy — reset counters.
		h.ConsecSuccesses = 0
	}
	return h
}

// IsDegraded returns true when the source is not in a fully healthy state.
func (h SourceHealth) IsDegraded() bool {
	return h.State >= HealthDegraded
}

// InFallback returns true when the source is using JSONL as a fallback.
func (h SourceHealth) InFallback() bool {
	return h.State == HealthFallback || h.State == HealthRecovering
}

// ShouldShowToast returns true only on the very first failure, suppressing
// subsequent noise while the source is already known-degraded.
func (h SourceHealth) ShouldShowToast() bool {
	return h.ConsecFailures == 1
}

// StalenessAge returns how long ago the last successful fetch occurred.
// Returns zero if LastSuccess has never been set.
func (h SourceHealth) StalenessAge() time.Duration {
	if h.LastSuccess.IsZero() {
		return 0
	}
	return time.Since(h.LastSuccess)
}

// StalenessLevel returns a staleness tier:
//
//	0 — normal (< 30s or no last-success recorded)
//	1 — amber  (>= 30s)
//	2 — red    (>= 2m)
func (h SourceHealth) StalenessLevel() int {
	age := h.StalenessAge()
	switch {
	case age >= stalenessRedAge:
		return 2
	case age >= stalenessAmberAge:
		return 1
	default:
		return 0
	}
}

// String implements fmt.Stringer for HealthState.
func (s HealthState) String() string {
	switch s {
	case HealthHealthy:
		return "healthy"
	case HealthDegraded:
		return "degraded"
	case HealthFallback:
		return "fallback"
	case HealthRecovering:
		return "recovering"
	default:
		return "unknown"
	}
}

// FindJSONLPath walks up from dir looking for .beads/issues.jsonl.
// Returns the first path found, or "" if not found.
func FindJSONLPath(dir string) string {
	for {
		candidate := filepath.Join(dir, ".beads", "issues.jsonl")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// ProbeJSONLFallback locates a JSONL file and validates it is fresh enough to
// use as a fallback source. Returns ok=true only when the file exists and its
// modification time is within jsonlMaxAge (1 hour).
func ProbeJSONLFallback(dir string) (path string, modTime time.Time, ok bool) {
	path = FindJSONLPath(dir)
	if path == "" {
		return "", time.Time{}, false
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", time.Time{}, false
	}
	mod := info.ModTime()
	if time.Since(mod) >= jsonlMaxAge {
		return "", time.Time{}, false
	}
	return path, mod, true
}
