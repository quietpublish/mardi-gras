package data

import (
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"strings"
	"time"
)

// Default timeout tiers for external command execution.
const (
	defaultTimeoutMedium = 15 * time.Second // data fetches (bd list --json)
	defaultTimeoutShort  = 5 * time.Second  // quick mutations (bd update, bd close, bd create)
)

// Runtime timeout tiers, overridable via SetCmdTimeout.
var (
	timeoutMedium = defaultTimeoutMedium
	timeoutShort  = defaultTimeoutShort
)

// SetCmdTimeout overrides data-layer timeout tiers by scaling proportionally.
// The seconds value is relative to a 30s baseline (matching gastown.SetCmdTimeout).
func SetCmdTimeout(seconds int) {
	if seconds <= 0 {
		return
	}
	scale := float64(seconds) / 30.0
	timeoutMedium = time.Duration(float64(defaultTimeoutMedium) * scale)
	timeoutShort = time.Duration(float64(defaultTimeoutShort) * scale)
}

// runWithTimeout executes a command with a context timeout and returns its stdout.
var runWithTimeout = func(timeout time.Duration, name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return exec.CommandContext(ctx, name, args...).Output()
}

// execWithTimeout executes a command with a context timeout, discarding output.
var execWithTimeout = func(timeout time.Duration, name string, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return exec.CommandContext(ctx, name, args...).Run()
}

// bdStderrError represents a structured JSON error from bd's stderr.
type bdStderrError struct {
	Error   string `json:"error"`
	Details []struct {
		Field   string `json:"field,omitempty"`
		Message string `json:"message,omitempty"`
	} `json:"details,omitempty"`
}

// parseBdStderr extracts a human-readable error message from bd's stderr output.
// bd v0.59.1+ emits structured JSON on stderr when --json is active.
// Falls back to raw text for older versions or non-JSON errors.
func parseBdStderr(stderr []byte) string {
	trimmed := strings.TrimSpace(string(stderr))
	if trimmed == "" {
		return ""
	}

	// Try structured JSON parse first
	var bdErr bdStderrError
	if json.Unmarshal(stderr, &bdErr) == nil && bdErr.Error != "" {
		msg := bdErr.Error
		for _, d := range bdErr.Details {
			if d.Message != "" {
				msg += ": " + d.Message
			}
		}
		return msg
	}

	// Fall back to raw text (strip common prefixes)
	msg := trimmed
	msg = strings.TrimPrefix(msg, "Error: ")
	msg = strings.TrimPrefix(msg, "error: ")
	// Take first line only for toast display
	if idx := strings.IndexByte(msg, '\n'); idx >= 0 {
		msg = msg[:idx]
	}
	return msg
}

// wrapExitError extracts a readable error from an exec.ExitError's stderr,
// using parseBdStderr for structured JSON when available. Returns the original
// error unchanged if it's not an ExitError or has no stderr.
func wrapExitError(prefix string, err error) error {
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return err
	}
	msg := parseBdStderr(exitErr.Stderr)
	if msg != "" {
		return errors.New(prefix + ": " + msg)
	}
	return err
}
