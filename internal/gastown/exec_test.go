package gastown

import (
	"slices"
	"strings"
	"testing"
	"time"
)

func TestSetCmdTimeoutDoubles(t *testing.T) {
	// Reset after test
	defer func() {
		timeoutLong = defaultTimeoutLong
		timeoutMedium = defaultTimeoutMedium
		timeoutShort = defaultTimeoutShort
	}()

	SetCmdTimeout(60) // double the 30s baseline
	if timeoutLong != 60*time.Second {
		t.Errorf("timeoutLong = %v, want 60s", timeoutLong)
	}
	if timeoutMedium != 30*time.Second {
		t.Errorf("timeoutMedium = %v, want 30s", timeoutMedium)
	}
	if timeoutShort != 10*time.Second {
		t.Errorf("timeoutShort = %v, want 10s", timeoutShort)
	}
}

func TestSetCmdTimeoutIgnoresZero(t *testing.T) {
	defer func() {
		timeoutLong = defaultTimeoutLong
		timeoutMedium = defaultTimeoutMedium
		timeoutShort = defaultTimeoutShort
	}()

	SetCmdTimeout(0)
	if timeoutLong != defaultTimeoutLong {
		t.Errorf("timeoutLong changed on zero input: %v", timeoutLong)
	}
}

func TestSetCmdTimeoutIgnoresNegative(t *testing.T) {
	defer func() {
		timeoutLong = defaultTimeoutLong
		timeoutMedium = defaultTimeoutMedium
		timeoutShort = defaultTimeoutShort
	}()

	SetCmdTimeout(-5)
	if timeoutLong != defaultTimeoutLong {
		t.Errorf("timeoutLong changed on negative input: %v", timeoutLong)
	}
}

func TestBdChildEnvPinsEnvelopeForBd(t *testing.T) {
	// User had BD_JSON_ENVELOPE=1 in shell — gastown must override it to 0
	// so bd's --json output stays in legacy (non-envelope) form.
	t.Setenv("BD_JSON_ENVELOPE", "1")
	env := bdChildEnv("bd", nil)
	var sawPin, sawInherited bool
	for _, kv := range env {
		switch kv {
		case "BD_JSON_ENVELOPE=0":
			sawPin = true
		case "BD_JSON_ENVELOPE=1":
			sawInherited = true
		}
	}
	if !sawPin {
		t.Errorf("bd env missing BD_JSON_ENVELOPE=0 pin")
	}
	if sawInherited {
		t.Errorf("bd env should have stripped inherited BD_JSON_ENVELOPE=1")
	}
}

func TestBdChildEnvDoesNotPinAutoCommit(t *testing.T) {
	// gastown's only direct bd call is the comment read; the auto-commit
	// polite-citizen pattern lives in internal/data, not here. Verify the
	// asymmetry: gastown.bdChildEnv must NOT set BD_DOLT_AUTO_COMMIT=off.
	env := bdChildEnv("bd", nil)
	for _, kv := range env {
		if strings.HasPrefix(kv, "BD_DOLT_AUTO_COMMIT=") {
			t.Errorf("gastown bdChildEnv should not pin BD_DOLT_AUTO_COMMIT, got %q", kv)
		}
	}
}

func TestBdChildEnvReturnsNilForOtherBinaries(t *testing.T) {
	// For non-bd commands (gt, claude, tmux) the function must return nil so
	// the child inherits the parent env without modification.
	cases := []string{"gt", "claude", "tmux", ""}
	for _, name := range cases {
		if env := bdChildEnv(name, nil); env != nil {
			t.Errorf("bdChildEnv(%q) = %v, want nil", name, env)
		}
	}
}

func TestBdChildEnvDisablesMetricsForCommentRead(t *testing.T) {
	t.Setenv("BD_DISABLE_METRICS", "0")
	read := bdChildEnv("bd", []string{"comments", "mg-42", "--json"})
	if !slices.Contains(read, "BD_DISABLE_METRICS=1") || slices.Contains(read, "BD_DISABLE_METRICS=0") {
		t.Errorf("comment read: want metrics disabled, got %v", read)
	}
	write := bdChildEnv("bd", []string{"comments", "add", "mg-42", "--", "hi"})
	if !slices.Contains(write, "BD_DISABLE_METRICS=0") || slices.Contains(write, "BD_DISABLE_METRICS=1") {
		t.Errorf("comment write: want the user's own setting kept, got %v", write)
	}
}

func TestExecWithTimeoutCarriesStderr(t *testing.T) {
	err := execWithTimeout(5*time.Second, "sh", "-c", "echo progress; echo 'gt: polecat obsidian is busy' >&2; exit 1")
	if err == nil {
		t.Fatal("expected an error")
	}
	if got := err.Error(); !strings.Contains(got, "exit status 1") || !strings.Contains(got, "polecat obsidian is busy") {
		t.Fatalf("error = %q, want the exit status and gt's reason", got)
	}
	if strings.Contains(err.Error(), "progress") {
		t.Fatalf("stdout leaked into the error: %q", err)
	}
}

func TestExecWithTimeoutEdgeCaseSilentFailure(t *testing.T) {
	err := execWithTimeout(5*time.Second, "sh", "-c", "exit 3")
	if err == nil || err.Error() != "exit status 3" {
		t.Fatalf("error = %v, want plain exit status when stderr is empty", err)
	}
}
