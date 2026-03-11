package data

import (
	"errors"
	"os/exec"
	"testing"
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
