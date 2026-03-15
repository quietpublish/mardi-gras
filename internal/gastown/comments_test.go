package gastown

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestCommentParsing(t *testing.T) {
	raw := `[
		{"id":"c-1","author":"claude (Toast)","body":"JWT validation needs refresh token handling","created_at":"2025-02-22T10:30:00Z"},
		{"id":"c-2","author":"overseer","body":"Approved, ship it","created_at":"2025-02-22T11:15:00Z"}
	]`

	var comments []Comment
	if err := json.Unmarshal([]byte(raw), &comments); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if len(comments) != 2 {
		t.Fatalf("expected 2 comments, got %d", len(comments))
	}

	if comments[0].Author != "claude (Toast)" {
		t.Errorf("expected author 'claude (Toast)', got %q", comments[0].Author)
	}
	if comments[0].Body != "JWT validation needs refresh token handling" {
		t.Errorf("unexpected body: %q", comments[0].Body)
	}
	if comments[1].ID != "c-2" {
		t.Errorf("expected ID 'c-2', got %q", comments[1].ID)
	}
	if comments[1].Time != "2025-02-22T11:15:00Z" {
		t.Errorf("expected created_at time, got %q", comments[1].Time)
	}
}

func TestCommentParsingEmpty(t *testing.T) {
	raw := `[]`
	var comments []Comment
	if err := json.Unmarshal([]byte(raw), &comments); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(comments) != 0 {
		t.Fatalf("expected 0 comments, got %d", len(comments))
	}
}

func TestFetchCommentsHappy(t *testing.T) {
	defer mockRun([]byte(gtCommentsJSON), nil)()
	comments, err := FetchComments("mg-10")
	if err != nil {
		t.Fatalf("FetchComments() error = %v", err)
	}
	if len(comments) != 2 {
		t.Fatalf("expected 2 comments, got %d", len(comments))
	}
	if comments[0].Author != "alice" {
		t.Errorf("Author = %q, want alice", comments[0].Author)
	}
}

func TestFetchCommentsExecError(t *testing.T) {
	defer mockRun(nil, errors.New("bd not found"))()
	_, err := FetchComments("mg-10")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestFetchCommentsArgs(t *testing.T) {
	calls, restore := mockRunCapture([]byte(`[]`), nil)
	defer restore()
	_, _ = FetchComments("mg-42")
	args := (*calls)[0]
	// Should be: bd comments mg-42 --json
	if len(args) != 4 || args[0] != "bd" || args[1] != "comments" || args[2] != "mg-42" || args[3] != "--json" {
		t.Errorf("args = %v", args)
	}
}
