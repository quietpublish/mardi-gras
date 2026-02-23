package gastown

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEventParsing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "events.jsonl")
	content := `{"ts":"2026-02-23T01:00:51Z","source":"gt","type":"session_start","actor":"mayor","payload":{"role":"mayor"},"visibility":"feed"}
{"ts":"2026-02-23T01:02:37Z","source":"gt","type":"sling","actor":"mayor","payload":{"target":"mardi_gras/quartz","bead":"bd-c8q"},"visibility":"feed"}
{"ts":"2026-02-23T01:05:00Z","source":"gt","type":"nudge","actor":"mayor","payload":{"target":"mardi_gras/quartz","reason":"Run gt prime"},"visibility":"feed"}
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	events, err := LoadRecentEvents(path, 20)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(events))
	}

	// Newest first
	if events[0].Type != "nudge" {
		t.Fatalf("expected newest event type 'nudge', got %q", events[0].Type)
	}
	if events[2].Type != "session_start" {
		t.Fatalf("expected oldest event type 'session_start', got %q", events[2].Type)
	}

	// Payload extraction
	target := EventPayloadString(events[0], "target")
	if target != "mardi_gras/quartz" {
		t.Fatalf("expected target 'mardi_gras/quartz', got %q", target)
	}
}

func TestEventParsingLimit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "events.jsonl")
	content := `{"ts":"1","source":"gt","type":"a","actor":"x","visibility":"feed"}
{"ts":"2","source":"gt","type":"b","actor":"x","visibility":"feed"}
{"ts":"3","source":"gt","type":"c","actor":"x","visibility":"feed"}
{"ts":"4","source":"gt","type":"d","actor":"x","visibility":"feed"}
{"ts":"5","source":"gt","type":"e","actor":"x","visibility":"feed"}
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	events, err := LoadRecentEvents(path, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(events))
	}
	// Newest first: e, d, c
	if events[0].Type != "e" {
		t.Fatalf("expected 'e', got %q", events[0].Type)
	}
	if events[2].Type != "c" {
		t.Fatalf("expected 'c', got %q", events[2].Type)
	}
}

func TestEventParsingEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "events.jsonl")
	if err := os.WriteFile(path, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	events, err := LoadRecentEvents(path, 20)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if events != nil {
		t.Fatalf("expected nil events for empty file, got %d", len(events))
	}
}

func TestEventParsingMissingFile(t *testing.T) {
	events, err := LoadRecentEvents("/nonexistent/path/events.jsonl", 20)
	if err != nil {
		t.Fatalf("missing file should return nil error, got %v", err)
	}
	if events != nil {
		t.Fatalf("expected nil events for missing file, got %d", len(events))
	}
}

func TestEventsPath(t *testing.T) {
	// With GT_HOME set
	t.Setenv("GT_HOME", "/tmp/mygt")
	path := EventsPath()
	if path != "/tmp/mygt/.events.jsonl" {
		t.Fatalf("expected '/tmp/mygt/.events.jsonl', got %q", path)
	}

	// Without GT_HOME — should use ~/gt/
	t.Setenv("GT_HOME", "")
	path = EventsPath()
	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, "gt", ".events.jsonl")
	if path != expected {
		t.Fatalf("expected %q, got %q", expected, path)
	}
}

func TestEventPayloadStringMissing(t *testing.T) {
	ev := Event{Payload: nil}
	if s := EventPayloadString(ev, "foo"); s != "" {
		t.Fatalf("expected empty string, got %q", s)
	}
}
