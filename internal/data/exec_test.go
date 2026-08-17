package data

import (
	"errors"
	"os/exec"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestParseBdStderrJSON(t *testing.T) {
	stderr := []byte(`{"error":"project_id mismatch","details":[{"field":"project_id","message":"expected abc, got xyz"}]}`)
	got := parseBdStderr(stderr)
	want := "project_id mismatch: expected abc, got xyz"
	if got != want {
		t.Errorf("parseBdStderr() = %q, want %q", got, want)
	}
}

func TestParseBdStderrJSONNoDetails(t *testing.T) {
	stderr := []byte(`{"error":"database locked"}`)
	got := parseBdStderr(stderr)
	if got != "database locked" {
		t.Errorf("parseBdStderr() = %q, want %q", got, "database locked")
	}
}

func TestParseBdStderrRawText(t *testing.T) {
	stderr := []byte("Error: no such issue proj-999\n")
	got := parseBdStderr(stderr)
	if got != "no such issue proj-999" {
		t.Errorf("parseBdStderr() = %q, want %q", got, "no such issue proj-999")
	}
}

func TestParseBdStderrRawMultiline(t *testing.T) {
	stderr := []byte("something went wrong\nmore details\n")
	got := parseBdStderr(stderr)
	if got != "something went wrong" {
		t.Errorf("parseBdStderr() = %q, want %q", got, "something went wrong")
	}
}

func TestParseBdStderrEmpty(t *testing.T) {
	got := parseBdStderr(nil)
	if got != "" {
		t.Errorf("parseBdStderr(nil) = %q, want empty", got)
	}
	got = parseBdStderr([]byte(""))
	if got != "" {
		t.Errorf("parseBdStderr(empty) = %q, want empty", got)
	}
}

func TestSchemaSkewHint(t *testing.T) {
	// The literal message bd emits when the database is ahead of the binary.
	err := errors.New("bd list --json: schema version mismatch: database is at v65, binary knows up to v53 (12 migrations ahead)")
	got := SchemaSkewHint(err)
	if got == "" {
		t.Fatal("SchemaSkewHint() = empty, want remediation text")
	}
	if !strings.Contains(got, "RECOVERY-1.2.1.md") {
		t.Errorf("SchemaSkewHint() = %q, want it to point at the recovery guide", got)
	}
	if !strings.Contains(got, "BD_IGNORE_SCHEMA_SKEW=1") {
		t.Errorf("SchemaSkewHint() = %q, want it to name the stopgap", got)
	}
}

func TestSchemaSkewHintUnrelatedErrors(t *testing.T) {
	// Anything else must fall through to the generic Dolt-server advice.
	cases := []error{
		nil,
		errors.New("bd list --json: connection refused"),
		errors.New("bd: command not found"),
		errors.New(""),
	}
	for _, err := range cases {
		if got := SchemaSkewHint(err); got != "" {
			t.Errorf("SchemaSkewHint(%v) = %q, want empty", err, got)
		}
	}
}

func TestWrapExitErrorWithStderr(t *testing.T) {
	exitErr := &exec.ExitError{
		Stderr: []byte(`{"error":"issue not found"}`),
	}
	got := wrapExitError("bd show", exitErr)
	want := "bd show: issue not found"
	if got.Error() != want {
		t.Errorf("wrapExitError() = %q, want %q", got.Error(), want)
	}
}

func TestWrapExitErrorNonExitError(t *testing.T) {
	orig := errors.New("timeout")
	got := wrapExitError("bd list", orig)
	if got != orig {
		t.Errorf("wrapExitError should return original error for non-ExitError, got %v", got)
	}
}

func TestWrapExitErrorEmptyStderr(t *testing.T) {
	exitErr := &exec.ExitError{
		Stderr: nil,
	}
	got := wrapExitError("bd list", exitErr)
	// Should return original error when no stderr to parse
	if got != exitErr {
		t.Errorf("wrapExitError should return original error for empty stderr, got %v", got)
	}
}

func TestSetCmdTimeoutScalesProportionally(t *testing.T) {
	defer func() {
		timeoutMedium = defaultTimeoutMedium
		timeoutShort = defaultTimeoutShort
	}()

	SetCmdTimeout(60) // double the 30s baseline
	if timeoutMedium != 30*time.Second {
		t.Errorf("timeoutMedium = %v, want 30s", timeoutMedium)
	}
	if timeoutShort != 10*time.Second {
		t.Errorf("timeoutShort = %v, want 10s", timeoutShort)
	}
}

func TestSetCmdTimeoutIgnoresZero(t *testing.T) {
	defer func() {
		timeoutMedium = defaultTimeoutMedium
		timeoutShort = defaultTimeoutShort
	}()

	SetCmdTimeout(0)
	if timeoutMedium != defaultTimeoutMedium {
		t.Errorf("timeoutMedium changed on zero input: %v", timeoutMedium)
	}
}

func TestBdReadOnlyArgs(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want bool
	}{
		{"empty args", []string{}, false},
		{"nil args", nil, false},
		{"list", []string{"list", "--json"}, true},
		{"show", []string{"show", "mg-42"}, true},
		{"context", []string{"context"}, true},
		{"doctor", []string{"doctor", "--json"}, true},
		{"--version", []string{"--version"}, true},
		{"version", []string{"version"}, true},
		{"comments read", []string{"comments", "mg-42"}, true},
		{"comments add is a mutation", []string{"comments", "add", "mg-42", "--", "body"}, false},
		{"ready plain", []string{"ready"}, true},
		{"ready --json", []string{"ready", "--json"}, true},
		{"ready --claim mutates", []string{"ready", "--claim", "--json"}, false},
		{"prune --dry-run", []string{"prune", "--older-than", "30d", "--dry-run"}, true},
		{"prune --force mutates", []string{"prune", "--older-than", "30d", "--force"}, false},
		{"prune with no flag is a mutation", []string{"prune"}, false},
		{"update is a mutation", []string{"update", "mg-42", "--status=closed"}, false},
		{"close is a mutation", []string{"close", "mg-42"}, false},
		{"create is a mutation", []string{"create", "--title=x"}, false},
		{"note is a mutation", []string{"note", "mg-42", "--", "body"}, false},
		{"label add is a mutation", []string{"label", "add", "mg-42", "--", "x"}, false},
		{"dep add is a mutation", []string{"dep", "add", "mg-42", "--", "mg-10"}, false},
		{"unknown subcommand is not read-only", []string{"frobnicate"}, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := bdReadOnlyArgs(tc.args)
			if got != tc.want {
				t.Errorf("bdReadOnlyArgs(%v) = %v, want %v", tc.args, got, tc.want)
			}
		})
	}
}

func TestBdChildEnvPinsReadOnlyOnly(t *testing.T) {
	// Pre-seed BD_DOLT_AUTO_COMMIT in parent env to verify it is stripped on
	// read-only and pinned to off, but preserved (inherited) on mutations.
	t.Setenv("BD_DOLT_AUTO_COMMIT", "on")
	t.Setenv("BD_JSON_ENVELOPE", "1")

	readOnly := bdChildEnv("bd", []string{"list", "--json"})
	if !hasEnv(readOnly, "BD_JSON_ENVELOPE=0") {
		t.Errorf("read-only env missing BD_JSON_ENVELOPE=0: %v", readOnly)
	}
	if !hasEnv(readOnly, "BD_DOLT_AUTO_COMMIT=off") {
		t.Errorf("read-only env missing BD_DOLT_AUTO_COMMIT=off: %v", readOnly)
	}
	if hasEnv(readOnly, "BD_DOLT_AUTO_COMMIT=on") {
		t.Errorf("read-only env should strip inherited BD_DOLT_AUTO_COMMIT=on: %v", readOnly)
	}

	mutate := bdChildEnv("bd", []string{"update", "mg-42", "--status=closed"})
	if !hasEnv(mutate, "BD_JSON_ENVELOPE=0") {
		t.Errorf("mutate env missing BD_JSON_ENVELOPE=0: %v", mutate)
	}
	if hasEnv(mutate, "BD_DOLT_AUTO_COMMIT=off") {
		t.Errorf("mutate env should not pin BD_DOLT_AUTO_COMMIT=off (let bd auto-commit): %v", mutate)
	}

	if env := bdChildEnv("gt", []string{"status", "--json"}); env != nil {
		t.Errorf("bdChildEnv(gt, ...) should return nil (inherit parent), got %v", env)
	}
}

func hasEnv(env []string, want string) bool {
	return slices.Contains(env, want)
}
