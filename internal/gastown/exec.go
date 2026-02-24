package gastown

import (
	"context"
	"os/exec"
	"time"
)

// Timeout tiers for external command execution.
const (
	// TimeoutLong is for commands known to be slow (gt status --json ~9s).
	TimeoutLong = 30 * time.Second

	// TimeoutMedium is for moderate data fetches (convoy list, mail inbox, bd list, costs, mol dag).
	TimeoutMedium = 15 * time.Second

	// TimeoutShort is for quick mutations (sling, nudge, bd update, bd close).
	TimeoutShort = 5 * time.Second
)

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
