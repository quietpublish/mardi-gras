package gastown

import (
	"encoding/json"
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
