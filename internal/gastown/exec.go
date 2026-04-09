package gastown

import (
	"context"
	"os/exec"
	"strings"
	"time"
)

// sanitizeOutput truncates command output for error messages to avoid leaking
// internal paths or stack traces. Scrubs absolute paths, takes first line only,
// and caps at 200 chars.
func sanitizeOutput(out []byte) string {
	s := strings.TrimSpace(string(out))
	if idx := strings.IndexByte(s, '\n'); idx >= 0 {
		s = s[:idx]
	}
	s = scrubPaths(s)
	if len(s) > 200 {
		s = s[:200] + "..."
	}
	return s
}

// scrubPaths replaces absolute filesystem paths (/a/b/c) with .../basename
// to avoid leaking directory structure in user-visible error messages.
func scrubPaths(s string) string {
	words := strings.Fields(s)
	for i, w := range words {
		clean := strings.TrimRight(w, ":),;")
		if len(clean) < 3 || clean[0] != '/' || strings.Count(clean, "/") < 2 {
			continue
		}
		if last := strings.LastIndex(clean, "/"); last > 0 {
			words[i] = ".../" + clean[last+1:] + w[len(clean):]
		}
	}
	return strings.Join(words, " ")
}

// Default timeout tiers for external command execution.
const (
	defaultTimeoutLong   = 30 * time.Second // commands known to be slow (gt status --json ~9s)
	defaultTimeoutMedium = 15 * time.Second // moderate data fetches (convoy list, mail inbox, costs)
	defaultTimeoutShort  = 5 * time.Second  // quick mutations (sling, nudge, bd update)
)

// Timeout tiers used at runtime. Defaults match the constants above
// but can be overridden via SetCmdTimeout for slow connections.
var (
	timeoutLong   = defaultTimeoutLong
	timeoutMedium = defaultTimeoutMedium
	timeoutShort  = defaultTimeoutShort
)

// SetCmdTimeout overrides all timeout tiers by scaling them proportionally.
// A value of 60 (seconds) doubles all timeouts (since the default long is 30s).
// Values <= 0 are ignored.
//
// SAFETY: Must be called during program initialization (main, before
// tea.NewProgram.Run) — never after command goroutines have started.
// Go's memory model guarantees that writes in the launching goroutine
// are visible to goroutines it subsequently spawns.
func SetCmdTimeout(seconds int) {
	if seconds <= 0 {
		return
	}
	scale := float64(seconds) / float64(defaultTimeoutLong/time.Second)
	timeoutLong = time.Duration(float64(defaultTimeoutLong) * scale)
	timeoutMedium = time.Duration(float64(defaultTimeoutMedium) * scale)
	timeoutShort = time.Duration(float64(defaultTimeoutShort) * scale)
}

// runWithTimeout executes a command with a context timeout and returns its stdout.
var runWithTimeout = func(timeout time.Duration, name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return exec.CommandContext(ctx, name, args...).Output()
}

// runCombinedWithTimeout executes a command with a context timeout and returns combined stdout+stderr.
var runCombinedWithTimeout = func(timeout time.Duration, name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}

// execWithTimeout executes a command with a context timeout, discarding output.
var execWithTimeout = func(timeout time.Duration, name string, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return exec.CommandContext(ctx, name, args...).Run()
}
