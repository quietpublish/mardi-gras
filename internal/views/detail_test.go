package views

import (
	"strings"
	"testing"

	"github.com/matt-wright86/mardi-gras/internal/data"
)

func TestParadeLabel(t *testing.T) {
	issues := []data.Issue{
		{ID: "mg-001", Title: "Blocker", Status: data.StatusOpen, Priority: data.PriorityHigh, IssueType: data.TypeTask},
		{ID: "mg-002", Title: "Blocked", Status: data.StatusOpen, Priority: data.PriorityMedium, IssueType: data.TypeTask,
			Dependencies: []data.Dependency{{IssueID: "mg-002", DependsOnID: "mg-001", Type: "blocks"}}},
		{ID: "mg-003", Title: "Rolling", Status: data.StatusInProgress, Priority: data.PriorityHigh, IssueType: data.TypeTask},
		{ID: "mg-004", Title: "Closed", Status: data.StatusClosed, Priority: data.PriorityMedium, IssueType: data.TypeTask},
	}
	issueMap := data.BuildIssueMap(issues)
	bt := data.DefaultBlockingTypes

	tests := []struct {
		name   string
		issue  *data.Issue
		expect string
	}{
		{name: "open unblocked", issue: issueMap["mg-001"], expect: "Lined Up"},
		{name: "open blocked", issue: issueMap["mg-002"], expect: "Stalled"},
		{name: "in_progress", issue: issueMap["mg-003"], expect: "Rolling"},
		{name: "closed", issue: issueMap["mg-004"], expect: "Past the Stand"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := paradeLabel(tc.issue, issueMap, bt)
			if got != tc.expect {
				t.Fatalf("paradeLabel(%s) = %q, want %q", tc.issue.ID, got, tc.expect)
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		expect string
	}{
		{name: "short string", input: "hello", maxLen: 10, expect: "hello"},
		{name: "exact fit", input: "hello", maxLen: 5, expect: "hello"},
		{name: "needs truncation", input: "hello world", maxLen: 8, expect: "hello..."},
		{name: "very short max", input: "hello", maxLen: 2, expect: "he"},
		{name: "max 3", input: "hello", maxLen: 3, expect: "hel"},
		{name: "empty string", input: "", maxLen: 5, expect: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := truncate(tc.input, tc.maxLen)
			if got != tc.expect {
				t.Fatalf("truncate(%q, %d) = %q, want %q", tc.input, tc.maxLen, got, tc.expect)
			}
		})
	}
}

func TestWordWrap(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		width  int
		expect string
	}{
		{name: "no wrap needed", input: "short text", width: 20, expect: "short text"},
		{name: "wraps at word boundary", input: "hello world foo bar", width: 11, expect: "hello world\nfoo bar"},
		{name: "single long word", input: "superlongword", width: 5, expect: "superlongword"},
		{name: "empty string", input: "", width: 10, expect: ""},
		{name: "zero width", input: "hello", width: 0, expect: "hello"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := wordWrap(tc.input, tc.width)
			if got != tc.expect {
				t.Fatalf("wordWrap(%q, %d) = %q, want %q", tc.input, tc.width, got, tc.expect)
			}
		})
	}
}

func TestSetIssueUpdatesContent(t *testing.T) {
	issues := []data.Issue{
		{ID: "mg-001", Title: "Test Issue Title", Status: data.StatusOpen, Priority: data.PriorityMedium, IssueType: data.TypeTask},
	}
	d := NewDetail(60, 20, issues)
	d.SetIssue(&issues[0])

	content := d.Viewport.View()
	if !strings.Contains(content, "Test Issue Title") {
		t.Fatalf("viewport content should contain issue title, got: %s", content)
	}
}

func TestSetSizeUpdatesDimensions(t *testing.T) {
	issues := []data.Issue{
		{ID: "mg-001", Title: "Test", Status: data.StatusOpen, Priority: data.PriorityMedium, IssueType: data.TypeTask},
	}
	d := NewDetail(60, 20, issues)

	d.SetSize(100, 30)
	if d.Width != 100 {
		t.Fatalf("Width = %d, want 100", d.Width)
	}
	if d.Height != 30 {
		t.Fatalf("Height = %d, want 30", d.Height)
	}
	if d.Viewport.Width != 98 {
		t.Fatalf("Viewport.Width = %d, want 98 (width-2)", d.Viewport.Width)
	}
	if d.Viewport.Height != 30 {
		t.Fatalf("Viewport.Height = %d, want 30", d.Viewport.Height)
	}
}
