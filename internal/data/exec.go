package data

import (
	"context"
	"os/exec"
	"time"
)

// Timeout tiers for external command execution.
const (
	// timeoutMedium is for data fetches (bd list --json).
	timeoutMedium = 15 * time.Second

	// timeoutShort is for quick mutations (bd update, bd close, bd create, git config).
	timeoutShort = 5 * time.Second
)

// runWithTimeout executes a command with a context timeout and returns its stdout.
func runWithTimeout(timeout time.Duration, name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return exec.CommandContext(ctx, name, args...).Output()
}

// execWithTimeout executes a command with a context timeout, discarding output.
func execWithTimeout(timeout time.Duration, name string, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return exec.CommandContext(ctx, name, args...).Run()
}
