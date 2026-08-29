package data

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"sync/atomic"
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

// BdVersionWarning returns a warning if the given bd version is known to be
// broken or dangerous, or "" for any safe or unrecognized version. The argument
// is a bare version string as reported by `bd context --json` ("1.2.1"), so
// callers can reuse the context fetch instead of spawning `bd --version`.
func BdVersionWarning(version string) string {
	switch strings.TrimPrefix(strings.TrimSpace(version), "v") {
	case "0.59.0":
		return "bd v0.59.0 has a known bug where --json is ignored; upgrade to v0.60.0+"
	case "1.2.0", "1.2.1":
		// Published by accident on 2026-08-11 without release testing. Running
		// one of these even once migrates the local schema v53→v65, after which
		// every other bd version halts with "schema version mismatch".
		// Kept short deliberately: the toast that renders this occupies a
		// single divider row, and a longer string wraps at 80 columns.
		return "bd v" + strings.TrimPrefix(strings.TrimSpace(version), "v") +
			" migrates your Beads schema — untested, upgrade to v1.2.2+"
	}
	return ""
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
	return BdVersionWarning(fields[len(fields)-1])
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

// briefDepsUnsupported latches once this bd is known to reject --brief-deps, so
// an older binary costs exactly one wasted probe per process rather than a
// doubled subprocess call on every detail-panel open.
var briefDepsUnsupported atomic.Bool

// FetchIssueDetail runs `bd show <id> --long --json` and returns the enriched issue.
// Returns fields not available from bd list: notes, design, acceptance_criteria.
// The --long flag requests extended metadata (agent identity, gate fields, etc.).
//
// It also passes --brief-deps (bd v1.3.0+) when the binary accepts it. That flag
// shrinks the inlined `dependencies` array, which matters here because mg parses
// that array and then throws it away: `bd show` and `bd list` return DIFFERENT
// dependency shapes — list returns the edge (`issue_id`/`depends_on_id`/`type`,
// which is what data.Dependency models), while show inlines each dependency as a
// whole issue plus `dependency_type`. None of show's keys match, so the decoded
// Dependencies are empty structs, and SetRichDetail deliberately merges only
// Notes/Design/AcceptanceCriteria. Upstream measured that array at 193 KB of a
// 214 KB response, so this is a large saving on bytes mg never reads.
//
// Do NOT start merging rich.Dependencies without fixing the shape mismatch
// first — it would overwrite real edges from bd list with blank ones.
func FetchIssueDetail(issueID string) (*Issue, error) {
	args := []string{"show", issueID, "--long", "--json"}
	if !briefDepsUnsupported.Load() {
		out, err := runWithTimeout(timeoutShort, "bd", "show", issueID, "--long", "--brief-deps", "--json")
		if err == nil {
			return parseIssueDetail(out)
		}
		wrapped := wrapExitError("bd show", err)
		if !strings.Contains(wrapped.Error(), "unknown flag") {
			// A real failure, not a capability gap — don't retry and double the cost.
			return nil, wrapped
		}
		briefDepsUnsupported.Store(true)
	}
	out, err := runWithTimeout(timeoutShort, "bd", args...)
	if err != nil {
		return nil, wrapExitError("bd show", err)
	}
	return parseIssueDetail(out)
}

// parseIssueDetail decodes the single-issue array `bd show --json` returns.
func parseIssueDetail(out []byte) (*Issue, error) {
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
