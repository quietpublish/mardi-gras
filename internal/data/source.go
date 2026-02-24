package data

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// SourceMode indicates how issues are loaded.
type SourceMode int

const (
	SourceJSONL SourceMode = iota // Read from .beads/issues.jsonl
	SourceCLI                     // Shell out to bd list --json
)

// Source describes how mg loads its issue data.
type Source struct {
	Mode       SourceMode
	Path       string // JSONL file path (SourceJSONL) or empty (SourceCLI)
	ProjectDir string // Project root directory
	Explicit   bool   // True if --path was used
}

// Label returns a display string for the footer.
func (s Source) Label() string {
	if s.Mode == SourceCLI {
		return "bd list"
	}
	if s.Path != "" {
		return filepath.Base(s.Path)
	}
	return "issues.jsonl"
}

// FetchIssuesCLI runs `bd list --json --limit 0 --all` and parses the result.
func FetchIssuesCLI() ([]Issue, error) {
	out, err := runWithTimeout(timeoutMedium, "bd", "list", "--json", "--limit", "0", "--all")
	if err != nil {
		return nil, fmt.Errorf("bd list --json: %w", err)
	}
	var issues []Issue
	if err := json.Unmarshal(out, &issues); err != nil {
		return nil, fmt.Errorf("bd list parse: %w", err)
	}
	SortIssues(issues)
	return issues, nil
}

// FetchIssuesNow returns a tea.Cmd that fetches issues via bd CLI immediately
// (no timer delay). Emits FileChangedMsg on success, FileWatchErrorMsg on failure.
func FetchIssuesNow() tea.Cmd {
	return func() tea.Msg {
		issues, err := FetchIssuesCLI()
		if err != nil {
			return FileWatchErrorMsg{Err: err}
		}
		return FileChangedMsg{Issues: issues, LastMod: time.Now()}
	}
}
