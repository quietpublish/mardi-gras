package ui

import (
	"strings"
	"testing"
)

func TestHighlightMatchesBasic(t *testing.T) {
	got := HighlightMatches("hello", []int{0, 4}, 0)
	// All five input runes must still be present.
	for _, r := range "hello" {
		if !strings.ContainsRune(got, r) {
			t.Errorf("output missing rune %q: %q", r, got)
		}
	}
	// Some ANSI styling must have been applied (h and o are matched).
	if !strings.Contains(got, "\x1b[") {
		t.Errorf("expected ANSI escape in highlighted output, got %q", got)
	}
}

func TestHighlightMatchesEmptyIndices(t *testing.T) {
	got := HighlightMatches("hello", nil, 0)
	if got != "hello" {
		t.Errorf("no indices should yield raw text, got %q", got)
	}
}

func TestHighlightMatchesTruncatesToMaxLen(t *testing.T) {
	got := HighlightMatches("abcdefghij", []int{0}, 5)
	// Must contain a-e and must NOT contain f or later.
	for _, r := range "abcde" {
		if !strings.ContainsRune(got, r) {
			t.Errorf("expected rune %q in truncated output: %q", r, got)
		}
	}
	for _, r := range "fghij" {
		if strings.ContainsRune(got, r) {
			t.Errorf("rune %q should have been truncated: %q", r, got)
		}
	}
}

func TestHighlightMatchesIndexBeyondMaxLenIgnored(t *testing.T) {
	// Index 8 points past maxLen=5 — must not panic, must not affect output rendering.
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("HighlightMatches panicked on out-of-range index: %v", r)
		}
	}()
	got := HighlightMatches("abcdefghij", []int{8}, 5)
	for _, r := range "abcde" {
		if !strings.ContainsRune(got, r) {
			t.Errorf("expected rune %q, got %q", r, got)
		}
	}
}

func TestHighlightMatchesMultiByte(t *testing.T) {
	// "⚜" is 3 bytes, 1 rune. Highlighting rune index 1 should style only it.
	text := "a⚜b"
	got := HighlightMatches(text, []int{1}, 0)
	if !strings.ContainsRune(got, '⚜') {
		t.Errorf("multi-byte rune missing from output: %q", got)
	}
	if !strings.ContainsRune(got, 'a') || !strings.ContainsRune(got, 'b') {
		t.Errorf("output missing surrounding runes: %q", got)
	}
}

func TestRoleBadgeContainsRole(t *testing.T) {
	got := RoleBadge("polecat")
	if !strings.Contains(got, "polecat") {
		t.Errorf("RoleBadge should embed role name, got %q", got)
	}
	if !strings.Contains(got, "\x1b[") {
		t.Errorf("RoleBadge should apply styling, got %q", got)
	}
}

func TestStateBadgeRendersSymbolAndLabel(t *testing.T) {
	tests := []struct {
		state  string
		symbol string
	}{
		{"working", SymWorking},
		{"spawning", SymSpawning},
		{"backoff", SymBackoff},
		{"stuck", SymStuck},
		{"awaiting-gate", SymGate},
		{"fix_needed", SymFixNeeded},
		{"patrolling", SymPatrolling},
		{"paused", SymPaused},
		{"idle", SymIdle},
	}
	for _, tc := range tests {
		t.Run(tc.state, func(t *testing.T) {
			got := StateBadge(tc.state)
			if !strings.Contains(got, tc.symbol) {
				t.Errorf("StateBadge(%q) missing symbol %q: %q", tc.state, tc.symbol, got)
			}
			if !strings.Contains(got, tc.state) {
				t.Errorf("StateBadge(%q) missing state label: %q", tc.state, got)
			}
		})
	}
}

func TestStateBadgeUnknownFallsBackToIdle(t *testing.T) {
	got := StateBadge("frobnicating")
	if !strings.Contains(got, SymIdle) {
		t.Errorf("unknown state should fall back to SymIdle, got %q", got)
	}
}

func TestOverlayInnerWidthDoesNotRewrap(t *testing.T) {
	// Content rendered at the box width minus 4 was re-wrapped by the box,
	// stranding a "──" under the palette divider (mg-zmz).
	for _, w := range []int{40, 66, 72} {
		line := strings.Repeat("─", OverlayInnerWidth(w))
		box := OverlayBox(line, w)
		if got := strings.Count(box, "\n") + 1; got != 1+4 {
			t.Fatalf("box width %d: %d lines, want 5 (one content line + border and padding)", w, got)
		}
	}
}
