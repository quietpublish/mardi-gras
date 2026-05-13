package data

import (
	"context"
	"encoding/json"
	"errors"
	"os"
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
//
// SAFETY: Must be called during program initialization (main, before
// tea.NewProgram.Run) — never after command goroutines have started.
// Go's memory model guarantees that writes in the launching goroutine
// are visible to goroutines it subsequently spawns.
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
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = bdChildEnv(name, args)
	return cmd.Output()
}

// execWithTimeout executes a command with a context timeout, discarding output.
var execWithTimeout = func(timeout time.Duration, name string, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = bdChildEnv(name, args)
	return cmd.Run()
}

// bdChildEnv returns the child process environment. For `bd`, it pins
// `BD_JSON_ENVELOPE=0` so a user's shell setting cannot flip bd's --json
// output into the envelope form that mg does not yet parse, and pins
// `BD_DOLT_AUTO_COMMIT=off` for read-only subcommands so each query does
// not fire a no-op `dolt_commit()` that costs a fresh connection per call
// (mirrors gt's `bdReadOnlyEnv` pattern from GH#3596). For other commands
// it returns nil (inherits parent env).
//
// Beads v2.0 will default to envelope mode; that's the migration window for
// mg to handle both shapes. Until then, pin legacy.
func bdChildEnv(name string, args []string) []string {
	if name != "bd" {
		return nil
	}
	readOnly := bdReadOnlyArgs(args)
	env := os.Environ()
	// Drop any inherited copies of the keys we manage, then re-pin them.
	filtered := env[:0]
	for _, kv := range env {
		if strings.HasPrefix(kv, "BD_JSON_ENVELOPE=") {
			continue
		}
		if readOnly && strings.HasPrefix(kv, "BD_DOLT_AUTO_COMMIT=") {
			continue
		}
		filtered = append(filtered, kv)
	}
	filtered = append(filtered, "BD_JSON_ENVELOPE=0")
	if readOnly {
		filtered = append(filtered, "BD_DOLT_AUTO_COMMIT=off")
	}
	return filtered
}

// bdReadOnlyArgs reports whether the given `bd` arg list invokes a
// read-only subcommand (list, show, context, doctor, --version, plain
// `ready`, or `prune --dry-run`). Used to opt into BD_DOLT_AUTO_COMMIT=off
// without affecting mutating commands.
func bdReadOnlyArgs(args []string) bool {
	if len(args) == 0 {
		return false
	}
	switch args[0] {
	case "list", "show", "context", "doctor", "comments", "--version", "version":
		// `comments` is the read form; `comments add` is a mutation (handled
		// by separate AddComment path, which calls runWithTimeout with the
		// `add` subcommand and is therefore not classified here as read-only).
		if args[0] == "comments" && len(args) > 1 && args[1] == "add" {
			return false
		}
		return true
	case "ready":
		// `bd ready` is read-only; `bd ready --claim` mutates.
		for _, a := range args {
			if a == "--claim" {
				return false
			}
		}
		return true
	case "prune":
		// `bd prune --dry-run` is read-only; `bd prune --force` mutates.
		for _, a := range args {
			if a == "--dry-run" {
				return true
			}
		}
		return false
	}
	return false
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
		var b strings.Builder
		b.WriteString(bdErr.Error)
		for _, d := range bdErr.Details {
			if d.Message != "" {
				b.WriteString(": ")
				b.WriteString(d.Message)
			}
		}
		return sanitizeErrMsg(b.String())
	}

	// Fall back to raw text (strip common prefixes)
	msg := trimmed
	msg = strings.TrimPrefix(msg, "Error: ")
	msg = strings.TrimPrefix(msg, "error: ")
	// Take first line only for toast display
	if idx := strings.IndexByte(msg, '\n'); idx >= 0 {
		msg = msg[:idx]
	}
	return sanitizeErrMsg(msg)
}

// sanitizeErrMsg scrubs absolute paths and truncates an error message
// for safe display in toast notifications.
func sanitizeErrMsg(msg string) string {
	msg = scrubPaths(msg)
	if len(msg) > 200 {
		msg = msg[:200] + "..."
	}
	return msg
}

// scrubPaths replaces absolute filesystem paths (/a/b/c) with .../basename
// to avoid leaking directory structure in user-visible error messages.
func scrubPaths(s string) string {
	words := strings.Fields(s)
	for i, w := range words {
		// Preserve trailing punctuation attached to the path
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
