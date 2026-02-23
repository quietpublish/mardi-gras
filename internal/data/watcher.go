package data

import (
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// FileChangedMsg signals that the issues file was modified on disk.
// Used by the app model to trigger a full parade rebuild.
// This is emitted by the polling watcher when a newer file modtime is detected.
type FileChangedMsg struct {
	Issues  []Issue
	LastMod time.Time
}

// FileUnchangedMsg signals a completed watch poll without changes.
type FileUnchangedMsg struct {
	LastMod time.Time
}

// FileWatchErrorMsg signals a poll error (stat/load). The app should keep polling.
type FileWatchErrorMsg struct {
	Err error
}

const watchInterval = 1200 * time.Millisecond
const cliPollInterval = 5 * time.Second

// WatchFile polls a JSONL file and emits a single message (changed, unchanged, or error).
// Callers should schedule it again after handling the returned message.
func WatchFile(path string, lastMod time.Time) tea.Cmd {
	if path == "" {
		return nil
	}
	return tea.Tick(watchInterval, func(time.Time) tea.Msg {
		info, err := os.Stat(path)
		if err != nil {
			return FileWatchErrorMsg{Err: err}
		}

		modTime := info.ModTime()
		if !modTime.After(lastMod) {
			return FileUnchangedMsg{LastMod: lastMod}
		}

		issues, err := LoadIssues(path)
		if err != nil {
			return FileWatchErrorMsg{Err: err}
		}
		return FileChangedMsg{Issues: issues, LastMod: modTime}
	})
}

// PollCLI polls bd list --json on a timer and emits FileChangedMsg or FileWatchErrorMsg.
// The app's diffIssues() handles no-op detection when nothing changed.
func PollCLI() tea.Cmd {
	return tea.Tick(cliPollInterval, func(time.Time) tea.Msg {
		issues, err := FetchIssuesCLI()
		if err != nil {
			return FileWatchErrorMsg{Err: err}
		}
		return FileChangedMsg{Issues: issues, LastMod: time.Now()}
	})
}

// FileModTime returns the file's modification time.
func FileModTime(path string) (time.Time, error) {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}, err
	}
	return info.ModTime(), nil
}
