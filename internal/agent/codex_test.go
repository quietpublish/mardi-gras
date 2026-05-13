package agent

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCodexHasPriorSessionEmpty(t *testing.T) {
	dir := t.TempDir()
	if got := CodexHasPriorSession(dir); got {
		t.Errorf("empty sessions dir should report no prior session, got true")
	}
}

func TestCodexHasPriorSessionMissingDir(t *testing.T) {
	if got := CodexHasPriorSession(filepath.Join(t.TempDir(), "does-not-exist")); got {
		t.Errorf("missing sessions dir should report no prior session, got true")
	}
}

func TestCodexHasPriorSessionFindsJSONL(t *testing.T) {
	// Mirror codex's real layout: <sessionsDir>/YYYY/MM/DD/<uuid>.jsonl
	root := t.TempDir()
	nested := filepath.Join(root, "2026", "05", "13")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(nested, "abc-123.jsonl"), []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("write jsonl: %v", err)
	}
	if got := CodexHasPriorSession(root); !got {
		t.Errorf("expected prior session detection to find nested .jsonl, got false")
	}
}

func TestCodexHasPriorSessionIgnoresNonJSONL(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "2026", "05", "13")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Only non-.jsonl files present — should still report no prior session.
	if err := os.WriteFile(filepath.Join(nested, "stale.tmp"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write tmp: %v", err)
	}
	if got := CodexHasPriorSession(root); got {
		t.Errorf("non-.jsonl files should not count as prior sessions, got true")
	}
}
