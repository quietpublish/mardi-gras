package data

import (
	"os"
	"time"

	tea "charm.land/bubbletea/v2"
)

// FileChangedMsg signals that the issues file was modified on disk.
// Used by the app model to trigger a full parade rebuild.
// This is emitted by the polling watcher when a newer file modtime is detected.
type FileChangedMsg struct {
	Issues  []Issue
	LastMod time.Time
	Skipped int // Count of malformed JSONL lines skipped during load
}

// FileUnchangedMsg signals a completed watch poll without changes.
type FileUnchangedMsg struct {
	LastMod time.Time
}

// FileWatchErrorMsg signals a poll error (stat/load). The app should keep polling.
type FileWatchErrorMsg struct {
	Err error
}

// Refresh intervals. The app owns the timer (one per session, see
// internal/app/refresh.go); these only say how long it waits between fetches.
const (
	WatchInterval   = 1200 * time.Millisecond // re-stat a JSONL issues file
	CLIPollInterval = 5 * time.Second         // re-run bd list
)

// CheckFile stats a JSONL issues file once and emits a single message:
// changed (with the reloaded issues), unchanged, or error. It returns nil when
// path is empty.
func CheckFile(path string, lastMod time.Time) tea.Cmd {
	if path == "" {
		return nil
	}
	return func() tea.Msg {
		info, err := os.Stat(path)
		if err != nil {
			return FileWatchErrorMsg{Err: err}
		}

		modTime := info.ModTime()
		if !modTime.After(lastMod) {
			return FileUnchangedMsg{LastMod: lastMod}
		}

		issues, skipped, err := LoadIssues(path)
		if err != nil {
			return FileWatchErrorMsg{Err: err}
		}
		return FileChangedMsg{Issues: issues, LastMod: modTime, Skipped: skipped}
	}
}

// FileModTime returns the file's modification time.
func FileModTime(path string) (time.Time, error) {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}, err
	}
	return info.ModTime(), nil
}

// CLIHealthCheckMsg reports a CLI health probe result. It is distinct from the
// primary FileChangedMsg/FileWatchErrorMsg cycle so the app can route recovery
// probes separately from normal data polls.
type CLIHealthCheckMsg struct {
	Issues []Issue
	Err    error
}

const cliHealthCheckInterval = 15 * time.Second

// CLIHealthCheck polls bd list on a longer interval to detect CLI recovery
// while the app is operating in JSONL fallback mode.
func CLIHealthCheck(projectDir string) tea.Cmd {
	return tea.Tick(cliHealthCheckInterval, func(time.Time) tea.Msg {
		issues, err := FetchIssuesCLI(projectDir)
		if err != nil {
			return CLIHealthCheckMsg{Err: err}
		}
		return CLIHealthCheckMsg{Issues: issues}
	})
}
