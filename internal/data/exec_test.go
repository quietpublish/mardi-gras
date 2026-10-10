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
	// v65 is the accidental v1.2.0/v1.2.1 migration: the database is the problem.
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

// TestSchemaSkewHintLegitimateMigration pins the case the old single-message
// hint got backwards: bd v1.3.0 migrates v53 → v66 by design, so the database
// is healthy and the BINARY is stale. Telling this user to roll the schema
// cursor back would damage a correct upgrade.
func TestSchemaSkewHintLegitimateMigration(t *testing.T) {
	err := errors.New("bd list --json: schema version mismatch: database is at v66, binary knows up to v53 (13 migrations ahead)")
	got := SchemaSkewHint(err)
	if got == "" {
		t.Fatal("SchemaSkewHint() = empty, want remediation text")
	}
	if !strings.Contains(got, "Upgrade THIS bd") {
		t.Errorf("SchemaSkewHint(v66) = %q, want it to advise upgrading the binary forward", got)
	}
	if !strings.Contains(got, "Do NOT roll the schema cursor back") {
		t.Errorf("SchemaSkewHint(v66) = %q, want it to warn against the rollback recovery", got)
	}
	// The rollback runbook must not be offered as the remedy here.
	if strings.Contains(got, "  2. Roll the schema cursor back per") {
		t.Errorf("SchemaSkewHint(v66) = %q, must not prescribe the v1.2.1 rollback", got)
	}
}

// TestSchemaSkewHintUnknownVersion covers a message shape we cannot parse: keep
// both remedies on screen rather than guessing one and being wrong.
func TestSchemaSkewHintUnknownVersion(t *testing.T) {
	err := errors.New("bd list --json: schema version mismatch (no version reported)")
	got := SchemaSkewHint(err)
	if got == "" {
		t.Fatal("SchemaSkewHint() = empty, want remediation text")
	}
	for _, want := range []string{"v65", "v66", "RECOVERY-1.2.1.md"} {
		if !strings.Contains(got, want) {
			t.Errorf("SchemaSkewHint(unparseable) = %q, want it to mention %q", got, want)
		}
	}
}

// TestSchemaSkewHintMigrateConsent pins the opposite direction from the other
// cases: a bd newer than the database that refuses to migrate it without
// consent (beads main after v1.3.x, schema v69). The first case is the real
// --json stderr payload going through wrapExitError, the path a bd list
// failure takes at startup.
func TestSchemaSkewHintMigrateConsent(t *testing.T) {
	jsonStderr := `{
  "error": "database schema is at v66; this bd requires v69 (3 pending migration(s)), and bd no longer migrates a database without explicit consent",
  "hint": "Operator decision required: this database is at schema v66 and this bd requires v69. Migrating is ONE-WAY.",
  "migrate_consent": {"current_version": 66, "required_version": 69, "pending": 3, "severity": "blocking"},
  "schema_version": 1
}`
	cases := map[string]error{
		"json stderr": wrapExitError("bd list --json", &exec.ExitError{Stderr: []byte(jsonStderr)}),
		"error text": errors.New("bd list --json: database schema is at v66; this bd requires v69 (3 pending migration(s)), " +
			"and bd no longer migrates a database without explicit consent — run `bd migrate schema`, or keep using a bd release that matches schema v66"),
		"operator text": errors.New("bd list --json: This database is at schema v66; this bd release uses schema v69."),
	}
	for name, err := range cases {
		got := SchemaSkewHint(err)
		for _, want := range []string{"bd migrate schema", "schema v66", "v69", "ONE-WAY", "matches its schema"} {
			if !strings.Contains(got, want) {
				t.Errorf("%s: SchemaSkewHint = %q, want it to mention %q", name, got, want)
			}
		}
		// The rollback recovery and the skew stopgap belong to the database-ahead
		// direction; here they would be wrong. bd keeps its consent variable out
		// of the refusal, and so does mg.
		for _, bad := range []string{"RECOVERY-1.2.1", "BD_IGNORE_SCHEMA_SKEW", "BD_ALLOW_MIGRATE"} {
			if strings.Contains(got, bad) {
				t.Errorf("%s: SchemaSkewHint = %q, must not mention %q", name, got, bad)
			}
		}
	}
}

func TestSchemaSkewHintMigrateConsentUnparsedVersions(t *testing.T) {
	got := SchemaSkewHint(errors.New("bd list --json: bd no longer migrates a database without explicit consent"))
	if !strings.Contains(got, "bd migrate schema") || !strings.Contains(got, "an older schema") {
		t.Errorf("SchemaSkewHint(no versions) = %q, want the consent advice without version numbers", got)
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
		{"events tail", []string{"events", "tail", "--since", "0", "--json"}, true},
		{"events prune mutates", []string{"events", "prune", "--before", "10"}, false},
		{"bare events is not read-only", []string{"events"}, false},
		{"config get", []string{"config", "get", "events-journal", "--json"}, true},
		{"config set mutates", []string{"config", "set", "events-journal", "true"}, false},
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

func TestBdChildEnvDisablesMetricsForReads(t *testing.T) {
	// Even a user who forces bd metrics on keeps mg's background polling out
	// of them; mg's writes on the user's behalf keep the user's setting.
	t.Setenv("BD_DISABLE_METRICS", "0")

	for _, args := range [][]string{
		{"list", "--json", "--limit", "0", "--all"},
		{"events", "tail", "--since", "4", "--limit", "501", "--json"},
		{"config", "get", "events-journal", "--json"},
		{"show", "mg-42", "--long", "--json"},
	} {
		env := bdChildEnv("bd", args)
		if !hasEnv(env, "BD_DISABLE_METRICS=1") || hasEnv(env, "BD_DISABLE_METRICS=0") {
			t.Errorf("bd %v: want metrics disabled, got %v", args, env)
		}
	}

	for _, args := range [][]string{
		{"update", "mg-42", "--claim"},
		{"config", "set", "events-journal", "true"},
		{"events", "prune", "--before", "10"},
	} {
		env := bdChildEnv("bd", args)
		if !hasEnv(env, "BD_DISABLE_METRICS=0") || hasEnv(env, "BD_DISABLE_METRICS=1") {
			t.Errorf("bd %v: want the user's own setting kept, got %v", args, env)
		}
	}
}

func hasEnv(env []string, want string) bool {
	return slices.Contains(env, want)
}
