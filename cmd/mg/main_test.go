package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/matt-wright86/mardi-gras/internal/data"
)

func TestFindBeadsFileInCurrentDir(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".beads"), 0o755)
	os.WriteFile(filepath.Join(dir, ".beads", "issues.jsonl"), []byte("[]"), 0o644)

	got := findBeadsFile(dir)
	want := filepath.Join(dir, ".beads", "issues.jsonl")
	if got != want {
		t.Errorf("findBeadsFile(%q) = %q, want %q", dir, got, want)
	}
}

func TestFindBeadsFileWalksUp(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, ".beads"), 0o755)
	os.WriteFile(filepath.Join(root, ".beads", "issues.jsonl"), []byte("[]"), 0o644)

	child := filepath.Join(root, "a", "b")
	os.MkdirAll(child, 0o755)

	got := findBeadsFile(child)
	want := filepath.Join(root, ".beads", "issues.jsonl")
	if got != want {
		t.Errorf("findBeadsFile(%q) = %q, want %q", child, got, want)
	}
}

func TestFindBeadsFileNotFound(t *testing.T) {
	dir := t.TempDir()

	got := findBeadsFile(dir)
	if got != "" {
		t.Errorf("findBeadsFile(%q) = %q, want empty string", dir, got)
	}
}

func TestParseBlockingTypesFlag(t *testing.T) {
	got := parseBlockingTypes("blocks,depends")
	want := map[string]bool{"blocks": true, "depends": true}

	if len(got) != len(want) {
		t.Fatalf("parseBlockingTypes returned %d entries, want %d", len(got), len(want))
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("parseBlockingTypes missing key %q or wrong value", k)
		}
	}
}

func TestParseBlockingTypesEnvFallback(t *testing.T) {
	t.Setenv("MG_BLOCK_TYPES", "blocks,depends")

	got := parseBlockingTypes("")
	want := map[string]bool{"blocks": true, "depends": true}

	if len(got) != len(want) {
		t.Fatalf("parseBlockingTypes returned %d entries, want %d", len(got), len(want))
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("parseBlockingTypes missing key %q or wrong value", k)
		}
	}
}

func TestParseBlockingTypesDefault(t *testing.T) {
	t.Setenv("MG_BLOCK_TYPES", "")

	got := parseBlockingTypes("")
	want := data.DefaultBlockingTypes

	if len(got) != len(want) {
		t.Fatalf("parseBlockingTypes returned %d entries, want %d", len(got), len(want))
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("parseBlockingTypes missing key %q or wrong value", k)
		}
	}
}

func TestParseBlockingTypesTrimsWhitespace(t *testing.T) {
	got := parseBlockingTypes(" blocks , depends ")
	want := map[string]bool{"blocks": true, "depends": true}

	if len(got) != len(want) {
		t.Fatalf("parseBlockingTypes returned %d entries, want %d", len(got), len(want))
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("parseBlockingTypes key %q: got %v, want %v", k, got[k], v)
		}
	}
}

func TestParseBlockingTypesEmptyCommas(t *testing.T) {
	got := parseBlockingTypes(",,")
	want := data.DefaultBlockingTypes

	if len(got) != len(want) {
		t.Fatalf("parseBlockingTypes(%q) returned %d entries, want %d (DefaultBlockingTypes)", ",,", len(got), len(want))
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("parseBlockingTypes(%q) missing key %q or wrong value", ",,", k)
		}
	}
}
