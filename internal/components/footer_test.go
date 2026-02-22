package components

import (
	"strings"
	"testing"
)

func TestNewFooterParadeBindings(t *testing.T) {
	f := NewFooter(80, false, false)
	if len(f.Bindings) != len(ParadeBindings) {
		t.Fatalf("expected %d bindings, got %d", len(ParadeBindings), len(f.Bindings))
	}
	for i, b := range f.Bindings {
		if b.Key != ParadeBindings[i].Key || b.Desc != ParadeBindings[i].Desc {
			t.Fatalf("binding %d: got {%s,%s}, want {%s,%s}", i, b.Key, b.Desc, ParadeBindings[i].Key, ParadeBindings[i].Desc)
		}
	}
}

func TestNewFooterDetailBindings(t *testing.T) {
	f := NewFooter(80, true, false)
	if len(f.Bindings) != len(DetailBindings) {
		t.Fatalf("expected %d bindings, got %d", len(DetailBindings), len(f.Bindings))
	}
	for i, b := range f.Bindings {
		if b.Key != DetailBindings[i].Key || b.Desc != DetailBindings[i].Desc {
			t.Fatalf("binding %d: got {%s,%s}, want {%s,%s}", i, b.Key, b.Desc, DetailBindings[i].Key, DetailBindings[i].Desc)
		}
	}
}

func TestNewFooterGasTownAddsBindings(t *testing.T) {
	f := NewFooter(80, false, true)

	// Should have ParadeBindings + 2 Gas Town bindings (sling, nudge)
	expected := len(ParadeBindings) + 2
	if len(f.Bindings) != expected {
		t.Fatalf("expected %d bindings with Gas Town, got %d", expected, len(f.Bindings))
	}

	// Find sling and nudge before quit
	foundSling := false
	foundNudge := false
	quitIdx := -1
	for i, b := range f.Bindings {
		switch b.Key {
		case "s":
			foundSling = true
			if quitIdx >= 0 {
				t.Fatal("sling binding appears after quit")
			}
		case "n":
			foundNudge = true
			if quitIdx >= 0 {
				t.Fatal("nudge binding appears after quit")
			}
		case "q":
			quitIdx = i
		}
	}
	if !foundSling {
		t.Fatal("missing sling binding")
	}
	if !foundNudge {
		t.Fatal("missing nudge binding")
	}
}

func TestInsertBefore(t *testing.T) {
	bindings := []FooterBinding{
		{Key: "a", Desc: "alpha"},
		{Key: "b", Desc: "beta"},
		{Key: "c", Desc: "gamma"},
	}
	extra := FooterBinding{Key: "x", Desc: "extra"}

	result := insertBefore(bindings, "b", extra)
	if len(result) != 4 {
		t.Fatalf("expected 4 bindings, got %d", len(result))
	}
	if result[1].Key != "x" {
		t.Fatalf("expected extra at index 1, got %s", result[1].Key)
	}
	if result[2].Key != "b" {
		t.Fatalf("expected b at index 2, got %s", result[2].Key)
	}
}

func TestInsertBeforeMissingKey(t *testing.T) {
	bindings := []FooterBinding{
		{Key: "a", Desc: "alpha"},
		{Key: "b", Desc: "beta"},
	}
	extra := FooterBinding{Key: "x", Desc: "extra"}

	result := insertBefore(bindings, "z", extra)
	if len(result) != 3 {
		t.Fatalf("expected 3 bindings, got %d", len(result))
	}
	if result[2].Key != "x" {
		t.Fatalf("expected extra appended at end, got %s at index 2", result[2].Key)
	}
}

func TestBulkFooterContainsCount(t *testing.T) {
	output := BulkFooter(80, 5)
	if !strings.Contains(output, "5") {
		t.Fatal("BulkFooter output should contain the selection count")
	}
}
