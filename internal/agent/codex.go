package agent

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// CodexHasPriorSession returns true if Codex has at least one rollout file
// persisted under sessionsDir (typically ~/.codex/sessions/). Codex's layout
// is YYYY/MM/DD/*.jsonl, so the search bails on the first match to avoid
// walking the entire history.
//
// Gating `codex resume --last` on this check protects against a documented
// failure mode where Codex writes a session_id before the rollout JSONL is
// flushed; without files present, `resume --last` exits immediately and would
// surface as a confusing empty tmux pane (see openai/codex agent-deck #756).
func CodexHasPriorSession(sessionsDir string) bool {
	if _, err := os.Stat(sessionsDir); err != nil {
		return false
	}
	found := false
	_ = filepath.WalkDir(sessionsDir, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".jsonl") {
			found = true
			return filepath.SkipAll
		}
		return nil
	})
	return found
}

// LaunchCodexResumeInTmux opens a new tmux pane running `codex resume --last`
// rooted at projectDir. Returns the pane ID. Caller is responsible for
// preflighting CodexHasPriorSession to avoid empty-pane surprises.
func LaunchCodexResumeInTmux(projectDir string) (string, error) {
	agentArgs := []string{
		"codex", "resume", "--last",
		"--no-alt-screen",
		"-C", projectDir,
	}
	tmuxArgs := []string{
		"split-window",
		"-h",
		"-l", "60%",
		"-d",
		"-c", projectDir,
		"-P", "-F", "#{pane_id}",
		"--",
	}
	tmuxArgs = append(tmuxArgs, agentArgs...)
	out, err := exec.Command("tmux", tmuxArgs...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("tmux split-window: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}
