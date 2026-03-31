package data

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
)

// SourceMode indicates how issues are loaded.
type SourceMode int

const (
	SourceJSONL SourceMode = iota // Legacy: read from .beads/issues.jsonl (or --path)
	SourceCLI                     // Preferred: shell out to bd list --json
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

// CheckBdVersion runs `bd --version` and returns a warning if the installed
// version is known to be broken. Returns "" for any safe or unparseable version.
func CheckBdVersion() string {
	return checkBdVersionOutput(nil)
}

// checkBdVersionOutput parses version output. If out is nil, it shells out to bd.
func checkBdVersionOutput(out []byte) string {
	if out == nil {
		var err error
		out, err = runWithTimeout(timeoutShort, "bd", "--version")
		if err != nil {
			return ""
		}
	}
	return parseBdVersionWarning(string(out))
}

// parseBdVersionWarning returns a warning string if the version is known-broken,
// or "" otherwise. Accepts output like "bd version 0.59.0".
func parseBdVersionWarning(output string) string {
	// Expected format: "bd version X.Y.Z" (possibly with trailing newline)
	fields := strings.Fields(strings.TrimSpace(output))
	if len(fields) < 2 {
		return ""
	}
	// Version is the last field (handles "bd version 0.59.0" and "0.59.0")
	ver := fields[len(fields)-1]
	if ver == "0.59.0" {
		return "bd v0.59.0 has a known bug where --json is ignored; upgrade to v0.60.0+"
	}
	return ""
}

// FetchIssuesCLI runs `bd list --json --limit 0 --all` and parses the result.
func FetchIssuesCLI(projectDir string) ([]Issue, error) {
	out, err := runWithTimeout(timeoutMedium, "bd", bdListArgs()...)
	if err != nil {
		return nil, wrapExitError("bd list --json", err)
	}
	return parseIssuesCLIOutput(out, LoadIssuePrefix(projectDir))
}

func bdListArgs() []string {
	return []string{"list", "--json", "--limit", "0", "--all"}
}

func parseIssuesCLIOutput(out []byte, expectedPrefix string) ([]Issue, error) {
	var issues []Issue
	if err := json.Unmarshal(out, &issues); err != nil {
		// Check if bd returned tree-formatted text instead of JSON
		trimmed := strings.TrimSpace(string(out))
		if trimmed != "" && !strings.HasPrefix(trimmed, "[") && !strings.HasPrefix(trimmed, "{") {
			return nil, fmt.Errorf("bd list returned non-JSON output (tree format?) — bd v0.59.0 has a known bug, upgrade to v0.60.0+")
		}
		return nil, fmt.Errorf("bd list parse: %w", err)
	}
	if err := validateIssuePrefixes(issues, expectedPrefix); err != nil {
		return nil, err
	}
	SortIssues(issues)
	return issues, nil
}

// BeadsContext holds workspace identity from `bd context --json`.
type BeadsContext struct {
	BeadsDir     string `json:"beads_dir"`
	RepoRoot     string `json:"repo_root"`
	IsRedirected bool   `json:"is_redirected"`
	Backend      string `json:"backend"`
	DoltMode     string `json:"dolt_mode"`
	Database     string `json:"database"`
	Role         string `json:"role"`
	BdVersion    string `json:"bd_version"`
}

// FetchContext runs `bd context --json` and returns workspace identity info.
func FetchContext() (*BeadsContext, error) {
	out, err := runWithTimeout(timeoutShort, "bd", "context", "--json")
	if err != nil {
		return nil, wrapExitError("bd context", err)
	}
	var ctx BeadsContext
	if err := json.Unmarshal(out, &ctx); err != nil {
		return nil, fmt.Errorf("bd context parse: %w", err)
	}
	return &ctx, nil
}

func validateIssuePrefixes(issues []Issue, expectedPrefix string) error {
	expectedPrefix = strings.TrimSpace(expectedPrefix)
	if expectedPrefix == "" || len(issues) == 0 {
		return nil
	}

	seenExpected := false
	mismatched := make(map[string]bool)
	for _, issue := range issues {
		prefix := issuePrefixFromID(issue.ID)
		if prefix == "" {
			continue
		}
		if prefix == expectedPrefix {
			seenExpected = true
			continue
		}
		if prefix == "hq" {
			continue
		}
		mismatched[prefix] = true
	}

	if seenExpected || len(mismatched) != 1 {
		return nil
	}

	for prefix := range mismatched {
		return fmt.Errorf("bd list returned %q issues, but this workspace expects %q — possible cross-project Dolt routing", prefix, expectedPrefix)
	}
	return nil
}

func issuePrefixFromID(id string) string {
	id = strings.TrimSpace(id)
	lastHyphen := strings.LastIndex(id, "-")
	if lastHyphen <= 0 {
		return ""
	}
	return id[:lastHyphen]
}

// FetchCurrentIssueID runs `bd show --current --json` and returns the active issue ID.
// Returns ("", nil) if no current issue exists (bd exits non-zero).
func FetchCurrentIssueID() (string, error) {
	out, err := runWithTimeout(timeoutShort, "bd", "show", "--current", "--json")
	if err != nil {
		return "", nil // bd exits non-zero when no current issue — not an error
	}
	var result struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		return "", fmt.Errorf("bd show --current parse: %w", err)
	}
	return result.ID, nil
}

// DoctorDiagnostic represents a single finding from `bd doctor --agent --json`.
type DoctorDiagnostic struct {
	Name        string   `json:"name"`
	Status      string   `json:"status"`       // "error", "warning", "ok"
	Severity    string   `json:"severity"`     // "blocking", "degraded", etc.
	Category    string   `json:"category"`     // "Core System", "Git Integration", etc.
	Explanation string   `json:"explanation"`  // Human-readable detail
	Observed    string   `json:"observed"`     // What was found
	Expected    string   `json:"expected"`     // What was expected
	Commands    []string `json:"commands"`     // Suggested fix commands
	SourceFiles []string `json:"source_files"` // Upstream source references
}

// DoctorResult holds the full output of `bd doctor --agent --json`.
type DoctorResult struct {
	OK          bool               `json:"overall_ok"`
	Summary     string             `json:"summary"`
	Diagnostics []DoctorDiagnostic `json:"diagnostics"`
}

// FetchDoctorDiagnostics runs `bd doctor --agent --json` and returns findings.
// Only returns error/warning diagnostics (not passed checks).
func FetchDoctorDiagnostics() (*DoctorResult, error) {
	out, err := runWithTimeout(timeoutMedium, "bd", "doctor", "--agent", "--json")
	if err != nil {
		// bd doctor exits non-zero when problems found — still has valid JSON on stdout
		if out == nil {
			return nil, wrapExitError("bd doctor", err)
		}
	}
	var result DoctorResult
	if err := json.Unmarshal(out, &result); err != nil {
		return nil, fmt.Errorf("bd doctor parse: %w", err)
	}
	return &result, nil
}

// FetchIssueDetail runs `bd show <id> --long --json` and returns the enriched issue.
// Returns fields not available from bd list: notes, design, acceptance_criteria.
// The --long flag requests extended metadata (agent identity, gate fields, etc.).
func FetchIssueDetail(issueID string) (*Issue, error) {
	out, err := runWithTimeout(timeoutShort, "bd", "show", issueID, "--long", "--json")
	if err != nil {
		return nil, wrapExitError("bd show", err)
	}
	var issues []Issue
	if err := json.Unmarshal(out, &issues); err != nil {
		return nil, fmt.Errorf("bd show parse: %w", err)
	}
	if len(issues) == 0 {
		return nil, fmt.Errorf("bd show: no issue returned")
	}
	return &issues[0], nil
}

// FetchIssuesNow returns a tea.Cmd that fetches issues via bd CLI immediately
// (no timer delay). Emits FileChangedMsg on success, FileWatchErrorMsg on failure.
func FetchIssuesNow(projectDir string) tea.Cmd {
	return func() tea.Msg {
		issues, err := FetchIssuesCLI(projectDir)
		if err != nil {
			return FileWatchErrorMsg{Err: err}
		}
		return FileChangedMsg{Issues: issues, LastMod: time.Now()}
	}
}
