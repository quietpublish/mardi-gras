package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/matt-wright86/mardi-gras/internal/data"
)

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustWrite(t *testing.T, path string, content []byte) {
	t.Helper()
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestFindBeadsFileInCurrentDir(t *testing.T) {
	dir := t.TempDir()
	mustMkdir(t, filepath.Join(dir, ".beads"))
	mustWrite(t, filepath.Join(dir, ".beads", "issues.jsonl"), []byte("[]"))

	got := findBeadsFile(dir)
	want := filepath.Join(dir, ".beads", "issues.jsonl")
	if got != want {
		t.Errorf("findBeadsFile(%q) = %q, want %q", dir, got, want)
	}
}

func TestFindBeadsFileWalksUp(t *testing.T) {
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, ".beads"))
	mustWrite(t, filepath.Join(root, ".beads", "issues.jsonl"), []byte("[]"))

	child := filepath.Join(root, "a", "b")
	mustMkdir(t, child)

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

func TestResolveSourceExplicitPathTrailingSlash(t *testing.T) {
	src := resolveSource(t.TempDir(), "/some/path/.beads/issues.jsonl/")
	if src.Path != "/some/path/.beads/issues.jsonl" {
		t.Fatalf("expected trailing slash cleaned, got %q", src.Path)
	}
	if src.ProjectDir != "/some/path" {
		t.Fatalf("expected ProjectDir /some/path, got %q", src.ProjectDir)
	}
}

func TestResolveSourceExplicitRelativePath(t *testing.T) {
	src := resolveSource(t.TempDir(), "./project/.beads/issues.jsonl")
	if !filepath.IsAbs(src.Path) {
		t.Fatalf("expected absolute path from relative --path, got %q", src.Path)
	}
	if !filepath.IsAbs(src.ProjectDir) {
		t.Fatalf("expected absolute ProjectDir from relative --path, got %q", src.ProjectDir)
	}
}

func TestResolveSourceCLIPreferredOverJSONL(t *testing.T) {
	if _, err := exec.LookPath("bd"); err != nil {
		t.Skip("bd not on PATH, skipping CLI preference test")
	}

	dir := t.TempDir()
	mustMkdir(t, filepath.Join(dir, ".beads"))
	mustWrite(t, filepath.Join(dir, ".beads", "issues.jsonl"), []byte("[]"))

	src := resolveSource(dir, "")
	if src.Mode != data.SourceCLI {
		t.Fatalf("expected SourceCLI when bd is on PATH, got %d", src.Mode)
	}
	if src.ProjectDir != dir {
		t.Fatalf("expected ProjectDir %q, got %q", dir, src.ProjectDir)
	}
}

func TestResolveSourceJSONLLegacyFallback(t *testing.T) {
	dir := t.TempDir()
	mustMkdir(t, filepath.Join(dir, ".beads"))
	mustWrite(t, filepath.Join(dir, ".beads", "issues.jsonl"), []byte("[]"))

	// Remove bd from PATH so CLI is not available
	t.Setenv("PATH", dir)

	src := resolveSource(dir, "")
	if src.Mode != data.SourceJSONL {
		t.Fatalf("expected SourceJSONL as legacy fallback, got %d", src.Mode)
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
	mustMkdir(t, filepath.Join(root, ".beads"))
	mustWrite(t, filepath.Join(root, ".beads", "issues.jsonl"), []byte("[]"))

	child := filepath.Join(root, "a", "b")
	mustMkdir(t, child)

	// Remove bd from PATH so we test the JSONL walkup path
	t.Setenv("PATH", root)

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
	mustMkdir(t, filepath.Join(dir, ".beads"))

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
	mustMkdir(t, filepath.Join(dir, ".beads"))

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
	mustMkdir(t, filepath.Join(dir, ".beads"))

	got := findBeadsDir(dir)
	if got != dir {
		t.Errorf("findBeadsDir(%q) = %q, want %q", dir, got, dir)
	}
}

func TestFindBeadsDirWalksUp(t *testing.T) {
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, ".beads"))

	child := filepath.Join(root, "a", "b")
	mustMkdir(t, child)

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
