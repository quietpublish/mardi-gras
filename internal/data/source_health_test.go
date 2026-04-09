package data

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestSourceHealthDegradation verifies that 3 consecutive failures transition
// a healthy source into the degraded state.
func TestSourceHealthDegradation(t *testing.T) {
	h := SourceHealth{}
	err := errors.New("bd list failed")

	h = h.RecordFailure(err)
	if h.State != HealthHealthy {
		t.Fatalf("after 1 failure: want HealthHealthy, got %s", h.State)
	}
	if h.ConsecFailures != 1 {
		t.Fatalf("after 1 failure: want ConsecFailures=1, got %d", h.ConsecFailures)
	}

	h = h.RecordFailure(err)
	if h.State != HealthHealthy {
		t.Fatalf("after 2 failures: want HealthHealthy, got %s", h.State)
	}

	h = h.RecordFailure(err)
	if h.State != HealthDegraded {
		t.Fatalf("after 3 failures: want HealthDegraded, got %s", h.State)
	}
	if h.ConsecFailures != 3 {
		t.Fatalf("after 3 failures: want ConsecFailures=3, got %d", h.ConsecFailures)
	}
}

// TestSourceHealthRecoveryFromDegraded verifies that a single success while
// in degraded state returns to healthy without requiring two successes.
func TestSourceHealthRecoveryFromDegraded(t *testing.T) {
	h := SourceHealth{State: HealthDegraded, ConsecFailures: 3}

	h = h.RecordSuccess()
	if h.State != HealthHealthy {
		t.Fatalf("recovery from degraded: want HealthHealthy, got %s", h.State)
	}
	if h.ConsecFailures != 0 {
		t.Fatalf("recovery from degraded: want ConsecFailures=0, got %d", h.ConsecFailures)
	}
}

// TestSourceHealthFallbackEntry verifies that ProbeJSONLFallback succeeds for
// a fresh JSONL file and fails for a stale or missing one.
func TestSourceHealthFallbackEntry(t *testing.T) {
	dir := t.TempDir()
	beadsDir := filepath.Join(dir, ".beads")
	if err := os.MkdirAll(beadsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	jsonlPath := filepath.Join(beadsDir, "issues.jsonl")

	// Write fresh file.
	if err := os.WriteFile(jsonlPath, []byte(`{}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	path, mod, ok := ProbeJSONLFallback(dir)
	if !ok {
		t.Fatal("ProbeJSONLFallback: expected ok=true for fresh file")
	}
	if path != jsonlPath {
		t.Fatalf("ProbeJSONLFallback: path mismatch, got %s", path)
	}
	if mod.IsZero() {
		t.Fatal("ProbeJSONLFallback: expected non-zero modTime")
	}
}

// TestSourceHealthFallbackEntryStale verifies ProbeJSONLFallback rejects a file
// whose modification time is older than jsonlMaxAge.
func TestSourceHealthFallbackEntryStale(t *testing.T) {
	dir := t.TempDir()
	beadsDir := filepath.Join(dir, ".beads")
	if err := os.MkdirAll(beadsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	jsonlPath := filepath.Join(beadsDir, "issues.jsonl")
	if err := os.WriteFile(jsonlPath, []byte(`{}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Back-date modification time to 2 hours ago.
	stale := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(jsonlPath, stale, stale); err != nil {
		t.Fatal(err)
	}

	_, _, ok := ProbeJSONLFallback(dir)
	if ok {
		t.Fatal("ProbeJSONLFallback: expected ok=false for stale file")
	}
}

// TestSourceHealthRecoveryFromFallback verifies that 2 consecutive successes
// while in fallback/recovering transitions the source back to healthy.
func TestSourceHealthRecoveryFromFallback(t *testing.T) {
	h := SourceHealth{State: HealthFallback}

	h = h.RecordSuccess()
	if h.State != HealthRecovering {
		t.Fatalf("after 1 success in fallback: want HealthRecovering, got %s", h.State)
	}
	if h.ConsecSuccesses != 1 {
		t.Fatalf("after 1 success in fallback: want ConsecSuccesses=1, got %d", h.ConsecSuccesses)
	}

	h = h.RecordSuccess()
	if h.State != HealthHealthy {
		t.Fatalf("after 2 successes: want HealthHealthy, got %s", h.State)
	}
}

// TestSourceHealthToastSuppression verifies that ShouldShowToast is true only
// on the first failure and suppressed on subsequent failures.
func TestSourceHealthToastSuppression(t *testing.T) {
	h := SourceHealth{}
	err := errors.New("fail")

	h = h.RecordFailure(err)
	if !h.ShouldShowToast() {
		t.Fatal("ShouldShowToast: expected true on first failure")
	}

	h = h.RecordFailure(err)
	if h.ShouldShowToast() {
		t.Fatal("ShouldShowToast: expected false on second failure")
	}

	h = h.RecordFailure(err)
	if h.ShouldShowToast() {
		t.Fatal("ShouldShowToast: expected false on third failure")
	}
}

// TestSourceHealthStalenessLevels verifies that StalenessLevel returns the
// correct tier (0/1/2) based on elapsed time since last success.
func TestSourceHealthStalenessLevels(t *testing.T) {
	// Level 0: no last success.
	h := SourceHealth{}
	if got := h.StalenessLevel(); got != 0 {
		t.Fatalf("no last success: want level 0, got %d", got)
	}

	// Level 0: just refreshed.
	h.LastSuccess = time.Now()
	if got := h.StalenessLevel(); got != 0 {
		t.Fatalf("just refreshed: want level 0, got %d", got)
	}

	// Level 1: 45 seconds ago.
	h.LastSuccess = time.Now().Add(-45 * time.Second)
	if got := h.StalenessLevel(); got != 1 {
		t.Fatalf("45s ago: want level 1, got %d", got)
	}

	// Level 2: 3 minutes ago.
	h.LastSuccess = time.Now().Add(-3 * time.Minute)
	if got := h.StalenessLevel(); got != 2 {
		t.Fatalf("3m ago: want level 2, got %d", got)
	}
}

// TestFindJSONLPath verifies the walk-up logic finds .beads/issues.jsonl.
func TestFindJSONLPath(t *testing.T) {
	// Create: /tmp/root/.beads/issues.jsonl
	root := t.TempDir()
	beadsDir := filepath.Join(root, ".beads")
	if err := os.MkdirAll(beadsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	jsonlPath := filepath.Join(beadsDir, "issues.jsonl")
	if err := os.WriteFile(jsonlPath, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	// Search from a subdirectory.
	sub := filepath.Join(root, "src", "pkg")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	got := FindJSONLPath(sub)
	if got != jsonlPath {
		t.Fatalf("FindJSONLPath from sub: want %s, got %s", jsonlPath, got)
	}

	// Search from a directory with no .beads ancestor.
	isolated := t.TempDir()
	if got := FindJSONLPath(isolated); got != "" {
		t.Fatalf("FindJSONLPath isolated: want empty, got %s", got)
	}
}

// TestProbeJSONLFallback verifies all three result cases: fresh ok, stale rejected,
// missing rejected.
func TestProbeJSONLFallback(t *testing.T) {
	// Missing: no .beads dir at all.
	missing := t.TempDir()
	if _, _, ok := ProbeJSONLFallback(missing); ok {
		t.Fatal("ProbeJSONLFallback missing: expected ok=false")
	}

	// Fresh file.
	freshDir := t.TempDir()
	beadsDir := filepath.Join(freshDir, ".beads")
	if err := os.MkdirAll(beadsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	jsonlPath := filepath.Join(beadsDir, "issues.jsonl")
	if err := os.WriteFile(jsonlPath, []byte(`{}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	path, _, ok := ProbeJSONLFallback(freshDir)
	if !ok {
		t.Fatal("ProbeJSONLFallback fresh: expected ok=true")
	}
	if path != jsonlPath {
		t.Fatalf("ProbeJSONLFallback fresh: path mismatch, got %s", path)
	}

	// Stale file.
	staleDir := t.TempDir()
	staleBD := filepath.Join(staleDir, ".beads")
	if err := os.MkdirAll(staleBD, 0o755); err != nil {
		t.Fatal(err)
	}
	stalePath := filepath.Join(staleBD, "issues.jsonl")
	if err := os.WriteFile(stalePath, []byte(`{}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(stalePath, old, old); err != nil {
		t.Fatal(err)
	}
	if _, _, ok := ProbeJSONLFallback(staleDir); ok {
		t.Fatal("ProbeJSONLFallback stale: expected ok=false")
	}
}
