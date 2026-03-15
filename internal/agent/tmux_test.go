package agent

import (
	"os"
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
