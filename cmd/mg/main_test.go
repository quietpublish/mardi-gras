package main

import (
	"os"
	"os/exec"
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

// ---------------------------------------------------------------------------
// resolveSource tests
// ---------------------------------------------------------------------------

func TestResolveSourceExplicitPath(t *testing.T) {
	src := resolveSource(t.TempDir(), "/some/path/.beads/issues.jsonl")
	if src.Mode != data.SourceJSONL {
		t.Fatalf("expected SourceJSONL, got %d", src.Mode)
	}
	if src.Path != "/some/path/.beads/issues.jsonl" {
		t.Fatalf("expected explicit path, got %q", src.Path)
	}
	if !src.Explicit {
		t.Fatal("expected Explicit to be true with --path flag")
	}
	if src.ProjectDir != "/some/path" {
		t.Fatalf("expected ProjectDir /some/path, got %q", src.ProjectDir)
	}
}

func TestResolveSourceJSONLExists(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".beads"), 0o755)
	os.WriteFile(filepath.Join(dir, ".beads", "issues.jsonl"), []byte("[]"), 0o644)

	src := resolveSource(dir, "")
	if src.Mode != data.SourceJSONL {
		t.Fatalf("expected SourceJSONL, got %d", src.Mode)
	}
	if src.Path == "" {
		t.Fatal("expected non-empty Path")
	}
	if src.Explicit {
		t.Fatal("expected Explicit to be false for auto-detected JSONL")
	}
}

func TestResolveSourceJSONLWalksUp(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, ".beads"), 0o755)
	os.WriteFile(filepath.Join(root, ".beads", "issues.jsonl"), []byte("[]"), 0o644)

	child := filepath.Join(root, "a", "b")
	os.MkdirAll(child, 0o755)

	src := resolveSource(child, "")
	if src.Mode != data.SourceJSONL {
		t.Fatalf("expected SourceJSONL, got %d", src.Mode)
	}
	want := filepath.Join(root, ".beads", "issues.jsonl")
	if src.Path != want {
		t.Fatalf("expected path %q, got %q", want, src.Path)
	}
}

func TestResolveSourceCLIFallback(t *testing.T) {
	// Only test if bd is on PATH
	if _, err := exec.LookPath("bd"); err != nil {
		t.Skip("bd not on PATH, skipping CLI fallback test")
	}

	dir := t.TempDir()
	// Create .beads/ dir but no issues.jsonl
	os.MkdirAll(filepath.Join(dir, ".beads"), 0o755)

	src := resolveSource(dir, "")
	if src.Mode != data.SourceCLI {
		t.Fatalf("expected SourceCLI, got %d", src.Mode)
	}
	if src.ProjectDir != dir {
		t.Fatalf("expected ProjectDir %q, got %q", dir, src.ProjectDir)
	}
}

func TestResolveSourceNoBdNoCLI(t *testing.T) {
	dir := t.TempDir()
	// Create .beads/ dir but no issues.jsonl
	os.MkdirAll(filepath.Join(dir, ".beads"), 0o755)

	// Override PATH to exclude bd
	t.Setenv("PATH", dir) // temp dir won't have bd

	src := resolveSource(dir, "")
	// Without bd on PATH and no JSONL, should return empty source
	if src.Mode != data.SourceJSONL {
		t.Fatalf("expected default SourceJSONL mode, got %d", src.Mode)
	}
	if src.Path != "" {
		t.Fatalf("expected empty Path, got %q", src.Path)
	}
}

func TestResolveSourceNoBeadsDir(t *testing.T) {
	dir := t.TempDir()
	// No .beads/ at all

	src := resolveSource(dir, "")
	if src.Path != "" {
		t.Fatalf("expected empty Path with no .beads dir, got %q", src.Path)
	}
}

// ---------------------------------------------------------------------------
// findBeadsDir tests
// ---------------------------------------------------------------------------

func TestFindBeadsDirInCurrentDir(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".beads"), 0o755)

	got := findBeadsDir(dir)
	if got != dir {
		t.Errorf("findBeadsDir(%q) = %q, want %q", dir, got, dir)
	}
}

func TestFindBeadsDirWalksUp(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, ".beads"), 0o755)

	child := filepath.Join(root, "a", "b")
	os.MkdirAll(child, 0o755)

	got := findBeadsDir(child)
	if got != root {
		t.Errorf("findBeadsDir(%q) = %q, want %q", child, got, root)
	}
}

func TestFindBeadsDirNotFound(t *testing.T) {
	dir := t.TempDir()
	got := findBeadsDir(dir)
	if got != "" {
		t.Errorf("findBeadsDir(%q) = %q, want empty string", dir, got)
	}
}
