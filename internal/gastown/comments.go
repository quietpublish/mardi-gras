package gastown

import (
	"encoding/json"
	"fmt"
)

// Comment represents a single comment on a beads issue.
type Comment struct {
	ID     string `json:"id"`
	Author string `json:"author"`
	Body   string `json:"body"`
	Time   string `json:"created_at"`
}

// FetchComments runs `bd comments <issueID> --json` and parses the output.
func FetchComments(issueID string) ([]Comment, error) {
	out, err := runWithTimeout(TimeoutMedium, "bd", "comments", issueID, "--json")
	if err != nil {
		return nil, fmt.Errorf("bd comments: %w", err)
	}
	var comments []Comment
	if err := json.Unmarshal(out, &comments); err != nil {
		return nil, fmt.Errorf("bd comments parse: %w", err)
	}
	return comments, nil
}
