package agent

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInTmux(t *testing.T) {
	// Save and restore original value.
	orig := os.Getenv("TMUX")
	defer os.Setenv("TMUX", orig)

	os.Setenv("TMUX", "/tmp/tmux-1000/default,12345,0")
	if !InTmux() {
		t.Error("expected InTmux()=true when TMUX is set")
	}

	os.Unsetenv("TMUX")
	if InTmux() {
		t.Error("expected InTmux()=false when TMUX is unset")
	}
}

func TestWindowName(t *testing.T) {
	tests := []struct {
		issueID string
		want    string
	}{
		{"bd-a1b2", "mg-bd-a1b2"},
		{"mg-001", "mg-mg-001"},
		{"xyz", "mg-xyz"},
	}
	for _, tt := range tests {
		got := WindowName(tt.issueID)
		if got != tt.want {
			t.Errorf("WindowName(%q) = %q, want %q", tt.issueID, got, tt.want)
		}
	}
}

func TestParseAgentPanes(t *testing.T) {
	output := "mg-bd-a1b2\t%5\n\t%0\nmg-bd-c3d4\t%8\n\t%1\n"
	agents := parseAgentPanes(output)

	if len(agents) != 2 {
		t.Fatalf("expected 2 agent panes, got %d: %v", len(agents), agents)
	}

	if agents["bd-a1b2"] != "%5" {
		t.Errorf("missing or wrong entry for bd-a1b2: %v", agents)
	}
	if agents["bd-c3d4"] != "%8" {
		t.Errorf("missing or wrong entry for bd-c3d4: %v", agents)
	}
}

func TestParseAgentPanesEmpty(t *testing.T) {
	agents := parseAgentPanes("")
	if len(agents) != 0 {
		t.Errorf("expected 0 agent panes from empty input, got %d", len(agents))
	}
}

func TestParseAgentPanesNoAgents(t *testing.T) {
	output := "\t%0\n\t%1\n\t%2\n"
	agents := parseAgentPanes(output)
	if len(agents) != 0 {
		t.Errorf("expected 0 agent panes, got %d: %v", len(agents), agents)
	}
}

func TestParseAgentPanesDuplicateIssueID(t *testing.T) {
	// Two panes tagged with the same issue ID — last-write-wins is the
	// documented behavior of the map-based parser. This pins it.
	output := "mg-bd-x9z\t%5\nmg-bd-x9z\t%9\n"
	agents := parseAgentPanes(output)
	if len(agents) != 1 {
		t.Fatalf("expected 1 entry after dedup, got %d: %v", len(agents), agents)
	}
	if agents["bd-x9z"] != "%9" {
		t.Errorf("expected last-write-wins (%%9), got %q", agents["bd-x9z"])
	}
}

func TestParseAgentPanesGarbledLines(t *testing.T) {
	// Lines without a tab, lines with empty paneID, and lines with
	// non-mg- tags must all be silently dropped without panicking.
	output := "no-tab-here\nmg-bd-aaa\t\nmg-bd-bbb\t%3\nbogus-tag\t%4\n\n"
	agents := parseAgentPanes(output)
	if len(agents) != 1 {
		t.Fatalf("expected 1 valid agent pane, got %d: %v", len(agents), agents)
	}
	if agents["bd-bbb"] != "%3" {
		t.Errorf("expected bd-bbb=%%3, got %v", agents)
	}
}

func TestSanitizeCaptureOutput(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxLines int
		wantLen  int // number of non-empty output lines
	}{
		{
			name:     "trims trailing blanks",
			input:    "line1\nline2\n\n\n\n",
			maxLines: 10,
			wantLen:  2,
		},
		{
			name:     "respects maxLines",
			input:    "a\nb\nc\nd\ne\nf\n",
			maxLines: 3,
			wantLen:  3,
		},
		{
			name:     "empty input",
			input:    "",
			maxLines: 10,
			wantLen:  0,
		},
		{
			name:     "all blank lines",
			input:    "\n\n\n\n",
			maxLines: 10,
			wantLen:  0,
		},
		{
			name:     "strips ANSI escape codes",
			input:    "\x1b[32mgreen text\x1b[0m\nnormal\n",
			maxLines: 10,
			wantLen:  2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines := sanitizeCaptureOutput(tt.input, tt.maxLines)
			if len(lines) != tt.wantLen {
				t.Errorf("got %d lines, want %d: %v", len(lines), tt.wantLen, lines)
			}
		})
	}
}

func TestSanitizeCaptureOutputContent(t *testing.T) {
	input := "\x1b[1;34mBold blue\x1b[0m\nplain text\n"
	lines := sanitizeCaptureOutput(input, 10)
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	if lines[0] != "Bold blue" {
		t.Errorf("line 0 = %q, want 'Bold blue'", lines[0])
	}
	if lines[1] != "plain text" {
		t.Errorf("line 1 = %q, want 'plain text'", lines[1])
	}
}

func TestStripANSI_OSC52(t *testing.T) {
	// OSC 52 clipboard-write: ESC ] 52 ; c ; <base64> ST
	// This is the attack vector — a compromised agent could write to the
	// user's clipboard via captured tmux pane output.
	input := "before\x1b]52;c;SGVsbG8=\x1b\\after"
	got := stripANSI(input)
	if got != "beforeafter" {
		t.Errorf("stripANSI did not remove OSC 52 sequence: got %q", got)
	}
}

func TestStripANSI_OSCWindowTitle(t *testing.T) {
	// OSC 0 (set window title): ESC ] 0 ; <title> BEL
	input := "text\x1b]0;evil-title\x07more"
	got := stripANSI(input)
	if got != "textmore" {
		t.Errorf("stripANSI did not remove OSC 0 sequence: got %q", got)
	}
}

func TestStripANSI_ControlBytes(t *testing.T) {
	// Control bytes 0x00-0x1F should be stripped, except \n (0x0A),
	// \t (0x09), and \r (0x0D) which are legitimate whitespace.
	input := "hello\x00\x01\x02\x03world\x7f"
	got := stripANSI(input)
	for _, b := range got {
		if b < 0x20 && b != '\n' && b != '\t' && b != '\r' {
			t.Errorf("control byte 0x%02x not stripped from result: %q", b, got)
		}
		if b == 0x7f {
			t.Errorf("DEL (0x7F) not stripped from result: %q", got)
		}
	}
	if got != "helloworld" {
		t.Errorf("stripANSI(%q) = %q, want %q", input, got, "helloworld")
	}
}

func TestSanitizeCaptureOutputTakesLastLines(t *testing.T) {
	input := "old1\nold2\nold3\nnew1\nnew2\n"
	lines := sanitizeCaptureOutput(input, 2)
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	if lines[0] != "new1" {
		t.Errorf("line 0 = %q, want 'new1'", lines[0])
	}
	if lines[1] != "new2" {
		t.Errorf("line 1 = %q, want 'new2'", lines[1])
	}
}

// withFakeTmux makes PATH a temp dir holding only a fake tmux, which logs every
// invocation and answers the split-window format query with %7, so the launch
// functions run to completion with no server. It clears MG_AGENT_CMD and
// MG_AGENT_RUNTIME so the ambient seat cannot decide the result. Returns the
// dir and the log path.
func withFakeTmux(t *testing.T) (dir, log string) {
	t.Helper()
	dir = t.TempDir()
	log = filepath.Join(dir, "calls.log")
	script := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> " + log + "\n" +
		"if [ \"$1\" = split-window ]; then printf '%%7\\n'; fi\n"
	if err := os.WriteFile(filepath.Join(dir, "tmux"), []byte(script), 0o755); err != nil {
		t.Fatalf("write fake tmux: %v", err)
	}
	t.Setenv("PATH", dir)
	t.Setenv(AgentCommandEnv, "")
	t.Setenv("MG_AGENT_RUNTIME", "")
	return dir, log
}

// splitWindowCall returns the last split-window invocation the fake tmux
// logged, or "" if there was none.
func splitWindowCall(t *testing.T, log string) string {
	t.Helper()
	raw, err := os.ReadFile(log)
	if os.IsNotExist(err) {
		return ""
	}
	if err != nil {
		t.Fatalf("read fake tmux log: %v", err)
	}
	var split string
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.HasPrefix(line, "split-window ") {
			split = line
		}
	}
	return split
}

func TestLaunchInTmuxRunsTheConfiguredWrapper(t *testing.T) {
	dir, log := withFakeTmux(t)
	shim := writeShim(t, dir, "agent-shim")
	t.Setenv(AgentCommandEnv, shim)

	paneID, err := LaunchInTmux("do the thing", "/tmp/project", "bd-1")
	if err != nil {
		t.Fatalf("LaunchInTmux: %v", err)
	}
	if paneID != "%7" {
		t.Errorf("paneID = %q, want the split-window output %%7", paneID)
	}

	split := splitWindowCall(t, log)
	if split == "" {
		t.Fatal("fake tmux saw no split-window call")
	}

	// The pane command must be the ABSOLUTE wrapper: tmux resolves it against
	// the server's PATH, so anything less can silently launch the bare agent.
	for _, want := range []string{shim, "-c /tmp/project", "--teammate-mode tmux", "do the thing"} {
		if !strings.Contains(split, want) {
			t.Errorf("split-window argv missing %q\n  got: %s", want, split)
		}
	}
	if strings.Contains(split, " claude ") {
		t.Errorf("split-window still launches the bare claude\n  got: %s", split)
	}
}

func TestLaunchInTmuxUnresolvableWrapperOpensNoPane(t *testing.T) {
	dir, log := withFakeTmux(t)
	writeShim(t, dir, "claude") // available, and must not be launched instead
	t.Setenv(AgentCommandEnv, "definitely-not-a-binary-xyz")

	if _, err := LaunchInTmux("do the thing", "/tmp/project", "bd-1"); err == nil {
		t.Fatal("LaunchInTmux with an unresolvable MG_AGENT_CMD must fail")
	}
	if split := splitWindowCall(t, log); split != "" {
		t.Errorf("no pane may open when the wrapper cannot be resolved\n  got: %s", split)
	}
}

func TestLaunchCodexResumeInTmuxRunsTheWrapperForCodex(t *testing.T) {
	dir, log := withFakeTmux(t) // no codex binary anywhere
	shim := writeShim(t, dir, "agent-shim")
	t.Setenv(AgentCommandEnv, shim)
	t.Setenv("MG_AGENT_RUNTIME", "codex")

	if _, err := LaunchCodexResumeInTmux("/tmp/project"); err != nil {
		t.Fatalf("LaunchCodexResumeInTmux: %v", err)
	}
	split := splitWindowCall(t, log)
	if want := "-- " + shim + " resume --last --no-alt-screen -C /tmp/project"; !strings.HasSuffix(split, want) {
		t.Errorf("resume should run through the wrapper\n  got:  %s\n  want: ...%s", split, want)
	}
}

func TestLaunchCodexResumeInTmuxRefusesAWrapperForAnotherRuntime(t *testing.T) {
	dir, log := withFakeTmux(t)
	writeShim(t, dir, "codex") // on PATH, and must not bypass the wrapper
	t.Setenv(AgentCommandEnv, writeShim(t, dir, "agent-shim"))
	t.Setenv("MG_AGENT_RUNTIME", "claude")

	_, err := LaunchCodexResumeInTmux("/tmp/project")
	if err == nil || !strings.Contains(err.Error(), AgentCommandEnv) {
		t.Errorf("err = %v, want a refusal naming %s", err, AgentCommandEnv)
	}
	if split := splitWindowCall(t, log); split != "" {
		t.Errorf("no pane may open around the wrapper\n  got: %s", split)
	}
}

func TestLaunchCodexResumeInTmuxRunsAbsoluteCodex(t *testing.T) {
	dir, log := withFakeTmux(t)
	codex := writeShim(t, dir, "codex")

	if _, err := LaunchCodexResumeInTmux("/tmp/project"); err != nil {
		t.Fatalf("LaunchCodexResumeInTmux: %v", err)
	}
	if split := splitWindowCall(t, log); !strings.Contains(split, "-- "+codex+" resume --last") {
		t.Errorf("resume should run codex by absolute path %q\n  got: %s", codex, split)
	}
}

func TestLaunchCodexResumeInTmuxWithoutCodex(t *testing.T) {
	_, log := withFakeTmux(t)

	if _, err := LaunchCodexResumeInTmux("/tmp/project"); !errors.Is(err, ErrCodexUnavailable) {
		t.Errorf("err = %v, want ErrCodexUnavailable", err)
	}
	if split := splitWindowCall(t, log); split != "" {
		t.Errorf("no pane may open without codex\n  got: %s", split)
	}
}

func TestIsPaneID(t *testing.T) {
	for s, want := range map[string]bool{
		"%0":       true,
		"%12":      true,
		"%":        false,
		"":         false,
		"obsidian": false, // an orchestrator agent name
		"mg-abc":   false,
	} {
		if got := IsPaneID(s); got != want {
			t.Errorf("IsPaneID(%q) = %v, want %v", s, got, want)
		}
	}
}
