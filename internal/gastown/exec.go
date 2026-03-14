package gastown

import (
	"context"
	"os/exec"
	"time"
)

// Default timeout tiers for external command execution.
const (
	defaultTimeoutLong   = 30 * time.Second // commands known to be slow (gt status --json ~9s)
	defaultTimeoutMedium = 15 * time.Second // moderate data fetches (convoy list, mail inbox, costs)
	defaultTimeoutShort  = 5 * time.Second  // quick mutations (sling, nudge, bd update)
)

// Timeout tiers used at runtime. Defaults match the constants above
// but can be overridden via SetCmdTimeout for slow connections.
var (
	TimeoutLong   = defaultTimeoutLong
	TimeoutMedium = defaultTimeoutMedium
	TimeoutShort  = defaultTimeoutShort
)

// SetCmdTimeout overrides all timeout tiers by scaling them proportionally.
// A value of 60 (seconds) doubles all timeouts (since the default long is 30s).
// Values <= 0 are ignored.
func SetCmdTimeout(seconds int) {
	if seconds <= 0 {
		return
	}
	scale := float64(seconds) / float64(defaultTimeoutLong/time.Second)
	TimeoutLong = time.Duration(float64(defaultTimeoutLong) * scale)
	TimeoutMedium = time.Duration(float64(defaultTimeoutMedium) * scale)
	TimeoutShort = time.Duration(float64(defaultTimeoutShort) * scale)
}

// runWithTimeout executes a command with a context timeout and returns its stdout.
func runWithTimeout(timeout time.Duration, name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return exec.CommandContext(ctx, name, args...).Output()
}

// runCombinedWithTimeout executes a command with a context timeout and returns combined stdout+stderr.
func runCombinedWithTimeout(timeout time.Duration, name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}

// execWithTimeout executes a command with a context timeout, discarding output.
func execWithTimeout(timeout time.Duration, name string, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return exec.CommandContext(ctx, name, args...).Run()
}
