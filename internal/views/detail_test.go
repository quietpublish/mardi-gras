package views

import (
	"strings"
	"testing"
	"time"

	"github.com/matt-wright86/mardi-gras/internal/data"
	"github.com/matt-wright86/mardi-gras/internal/gastown"
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

func TestSetMolecule(t *testing.T) {
	issues := []data.Issue{
		{ID: "mg-001", Title: "Test Issue", Status: data.StatusInProgress, Priority: data.PriorityMedium, IssueType: data.TypeTask, CreatedAt: time.Now()},
	}
	d := NewDetail(80, 30, issues)
	d.SetIssue(&issues[0])

	dag := &gastown.DAGInfo{
		RootID:    "mg-001",
		RootTitle: "Test Issue",
		Nodes: map[string]*gastown.DAGNode{
			"s1": {ID: "s1", Title: "Design", Status: "done", Tier: 0},
			"s2": {ID: "s2", Title: "Implement", Status: "in_progress", Tier: 1},
		},
		TierGroups: [][]string{{"s1"}, {"s2"}},
	}
	progress := &gastown.MoleculeProgress{
		TotalSteps: 3,
		DoneSteps:  1,
		Percent:    33,
	}

	d.SetMolecule("mg-001", dag, progress)

	if d.MoleculeDAG != dag {
		t.Fatal("MoleculeDAG not set")
	}
	if d.MoleculeProgress != progress {
		t.Fatal("MoleculeProgress not set")
	}
	if d.MoleculeIssueID != "mg-001" {
		t.Fatalf("MoleculeIssueID = %q, want %q", d.MoleculeIssueID, "mg-001")
	}
}

func TestSetMoleculeClearsOnIssueChange(t *testing.T) {
	issues := []data.Issue{
		{ID: "mg-001", Title: "Issue 1", Status: data.StatusInProgress, Priority: data.PriorityMedium, IssueType: data.TypeTask, CreatedAt: time.Now()},
		{ID: "mg-002", Title: "Issue 2", Status: data.StatusOpen, Priority: data.PriorityMedium, IssueType: data.TypeTask, CreatedAt: time.Now()},
	}
	d := NewDetail(80, 30, issues)
	d.SetIssue(&issues[0])

	dag := &gastown.DAGInfo{
		RootID: "mg-001",
		Nodes:  map[string]*gastown.DAGNode{"s1": {ID: "s1", Status: "done"}},
	}
	d.SetMolecule("mg-001", dag, nil)

	// Switch to a different issue
	d.SetIssue(&issues[1])

	if d.MoleculeDAG != nil {
		t.Fatal("MoleculeDAG should be cleared when switching issues")
	}
	if d.MoleculeIssueID != "" {
		t.Fatalf("MoleculeIssueID should be empty, got %q", d.MoleculeIssueID)
	}
}

func TestMoleculeRenderingInContent(t *testing.T) {
	issues := []data.Issue{
		{ID: "mg-001", Title: "Test Issue", Status: data.StatusInProgress, Priority: data.PriorityMedium, IssueType: data.TypeTask, CreatedAt: time.Now()},
	}
	d := NewDetail(80, 40, issues)
	d.SetIssue(&issues[0])

	dag := &gastown.DAGInfo{
		RootID:    "mg-001",
		RootTitle: "Build Feature",
		Nodes: map[string]*gastown.DAGNode{
			"s1": {ID: "s1", Title: "Design", Status: "done", Tier: 0},
			"s2": {ID: "s2", Title: "Implement", Status: "in_progress", Tier: 1},
			"s3": {ID: "s3", Title: "Test", Status: "blocked", Tier: 2, Dependencies: []string{"s2"}},
		},
		TierGroups: [][]string{{"s1"}, {"s2"}, {"s3"}},
	}
	progress := &gastown.MoleculeProgress{
		TotalSteps: 3,
		DoneSteps:  1,
		Percent:    33,
	}
	d.SetMolecule("mg-001", dag, progress)

	content := d.renderContent()

	if !strings.Contains(content, "MOLECULE") {
		t.Error("content should contain MOLECULE section")
	}
	if !strings.Contains(content, "Design") {
		t.Error("content should contain step title 'Design'")
	}
	if !strings.Contains(content, "Implement") {
		t.Error("content should contain step title 'Implement'")
	}
	if !strings.Contains(content, "done") {
		t.Error("content should contain 'done' status")
	}
	if !strings.Contains(content, "in_progress") {
		t.Error("content should contain 'in_progress' status")
	}
}

func TestActivityRenderingInContent(t *testing.T) {
	created := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	updated := time.Date(2025, 1, 16, 14, 30, 0, 0, time.UTC)
	issues := []data.Issue{
		{ID: "mg-001", Title: "Test Issue", Status: data.StatusInProgress,
			Priority: data.PriorityMedium, IssueType: data.TypeTask,
			CreatedAt: created, UpdatedAt: updated},
	}
	d := NewDetail(80, 40, issues)
	d.SetIssue(&issues[0])

	content := d.renderContent()

	if !strings.Contains(content, "ACTIVITY") {
		t.Error("content should contain ACTIVITY section")
	}
	if !strings.Contains(content, "Created") {
		t.Error("content should contain 'Created' event")
	}
	if !strings.Contains(content, "Updated") {
		t.Error("content should contain 'Updated' event")
	}
}

func TestActivityWithAgent(t *testing.T) {
	issues := []data.Issue{
		{ID: "mg-001", Title: "Test Issue", Status: data.StatusInProgress,
			Priority: data.PriorityMedium, IssueType: data.TypeTask,
			CreatedAt: time.Now()},
	}
	d := NewDetail(80, 40, issues)
	d.ActiveAgents = map[string]string{"mg-001": "polecat-1"}
	d.TownStatus = &gastown.TownStatus{
		Agents: []gastown.AgentRuntime{
			{Name: "polecat-1", Role: "polecat", State: "working", HookBead: "mg-001"},
		},
	}
	d.SetIssue(&issues[0])

	content := d.renderContent()

	if !strings.Contains(content, "polecat-1") {
		t.Error("content should show agent name in activity")
	}
}

func TestActivityWithClosedIssue(t *testing.T) {
	created := time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC)
	closed := time.Date(2025, 1, 17, 9, 0, 0, 0, time.UTC)
	issues := []data.Issue{
		{ID: "mg-001", Title: "Test Issue", Status: data.StatusClosed,
			Priority: data.PriorityMedium, IssueType: data.TypeTask,
			CreatedAt: created, ClosedAt: &closed},
	}
	d := NewDetail(80, 40, issues)
	d.SetIssue(&issues[0])

	content := d.renderContent()

	if !strings.Contains(content, "Closed") {
		t.Error("content should contain 'Closed' event")
	}
}

func TestMoleculeProgressBar(t *testing.T) {
	bar := moleculeProgressBar(3, 10, 20)
	if bar == "" {
		t.Fatal("progress bar should not be empty")
	}
	if len([]rune(bar)) == 0 {
		t.Fatal("progress bar should have characters")
	}

	// Edge cases
	emptyBar := moleculeProgressBar(0, 0, 10)
	if emptyBar == "" {
		t.Fatal("zero-total bar should not be empty")
	}
}

func TestFormatTime(t *testing.T) {
	ts := time.Date(2025, 2, 15, 14, 30, 0, 0, time.UTC)
	got := formatTime(ts)
	if !strings.Contains(got, "Feb 15") {
		t.Errorf("formatTime should contain date, got %q", got)
	}

	// Zero time
	zero := formatTime(time.Time{})
	if strings.TrimSpace(zero) != "" {
		t.Errorf("zero time should be blank, got %q", zero)
	}
}

func TestGateStatusRendering(t *testing.T) {
	issues := []data.Issue{
		{ID: "mg-001", Title: "Gated Issue", Status: data.StatusInProgress,
			Priority: data.PriorityMedium, IssueType: data.TypeTask,
			CreatedAt: time.Now()},
	}
	d := NewDetail(80, 40, issues)
	d.TownStatus = &gastown.TownStatus{
		Agents: []gastown.AgentRuntime{
			{Name: "Toast", Role: "polecat", State: "awaiting-gate", HookBead: "mg-001", Running: true},
		},
	}
	d.ActiveAgents = map[string]string{"mg-001": "Toast"}
	d.SetIssue(&issues[0])

	content := d.renderContent()

	if !strings.Contains(content, "GATE") {
		t.Error("content should contain GATE section when agent is awaiting-gate")
	}
	if !strings.Contains(content, "Waiting on gate") {
		t.Error("content should show 'Waiting on gate' indicator")
	}
	if !strings.Contains(content, "Toast") {
		t.Error("content should show agent name in gate section")
	}
}

func TestGateStatusNotShownWhenWorking(t *testing.T) {
	issues := []data.Issue{
		{ID: "mg-001", Title: "Working Issue", Status: data.StatusInProgress,
			Priority: data.PriorityMedium, IssueType: data.TypeTask,
			CreatedAt: time.Now()},
	}
	d := NewDetail(80, 40, issues)
	d.TownStatus = &gastown.TownStatus{
		Agents: []gastown.AgentRuntime{
			{Name: "Toast", Role: "polecat", State: "working", HookBead: "mg-001", Running: true},
		},
	}
	d.SetIssue(&issues[0])

	gate := d.renderGateStatus()
	if gate != "" {
		t.Error("gate section should not render when agent state is 'working'")
	}
}

func TestGateStatusNotShownWithoutTownStatus(t *testing.T) {
	issues := []data.Issue{
		{ID: "mg-001", Title: "No GT", Status: data.StatusInProgress,
			Priority: data.PriorityMedium, IssueType: data.TypeTask,
			CreatedAt: time.Now()},
	}
	d := NewDetail(80, 40, issues)
	d.SetIssue(&issues[0])

	gate := d.renderGateStatus()
	if gate != "" {
		t.Error("gate section should not render without TownStatus")
	}
}

func TestCommentsRendering(t *testing.T) {
	issues := []data.Issue{
		{ID: "mg-001", Title: "Commented Issue", Status: data.StatusInProgress,
			Priority: data.PriorityMedium, IssueType: data.TypeTask,
			CreatedAt: time.Now()},
	}
	d := NewDetail(80, 40, issues)
	d.SetIssue(&issues[0])

	comments := []gastown.Comment{
		{ID: "c-1", Author: "claude (Toast)", Body: "JWT validation needs refresh", Time: "2025-02-22T10:30:00Z"},
		{ID: "c-2", Author: "overseer", Body: "Approved, ship it", Time: "2025-02-22T11:15:00Z"},
	}
	d.SetComments("mg-001", comments)

	content := d.renderContent()

	if !strings.Contains(content, "COMMENTS (2)") {
		t.Error("content should contain 'COMMENTS (2)' section header")
	}
	if !strings.Contains(content, "claude (Toast)") {
		t.Error("content should contain comment author")
	}
	if !strings.Contains(content, "JWT validation") {
		t.Error("content should contain comment body")
	}
	if !strings.Contains(content, "overseer") {
		t.Error("content should contain second comment author")
	}
}

func TestCommentsNotShownWhenEmpty(t *testing.T) {
	issues := []data.Issue{
		{ID: "mg-001", Title: "No Comments", Status: data.StatusOpen,
			Priority: data.PriorityMedium, IssueType: data.TypeTask,
			CreatedAt: time.Now()},
	}
	d := NewDetail(80, 40, issues)
	d.SetIssue(&issues[0])

	content := d.renderContent()

	if strings.Contains(content, "COMMENTS") {
		t.Error("content should not contain COMMENTS section when no comments")
	}
}

func TestCommentsClearedOnIssueSwitch(t *testing.T) {
	issues := []data.Issue{
		{ID: "mg-001", Title: "Issue 1", Status: data.StatusInProgress,
			Priority: data.PriorityMedium, IssueType: data.TypeTask, CreatedAt: time.Now()},
		{ID: "mg-002", Title: "Issue 2", Status: data.StatusOpen,
			Priority: data.PriorityMedium, IssueType: data.TypeTask, CreatedAt: time.Now()},
	}
	d := NewDetail(80, 40, issues)
	d.SetIssue(&issues[0])

	comments := []gastown.Comment{
		{ID: "c-1", Author: "test", Body: "Hello"},
	}
	d.SetComments("mg-001", comments)

	if len(d.Comments) != 1 {
		t.Fatal("comments should be set")
	}

	// Switch to different issue — comments should clear
	d.SetIssue(&issues[1])

	if d.Comments != nil {
		t.Error("comments should be cleared when switching issues")
	}
	if d.CommentsIssueID != "" {
		t.Errorf("CommentsIssueID should be empty, got %q", d.CommentsIssueID)
	}
}

func TestSetCommentsUpdatesContent(t *testing.T) {
	issues := []data.Issue{
		{ID: "mg-001", Title: "Test", Status: data.StatusInProgress,
			Priority: data.PriorityMedium, IssueType: data.TypeTask, CreatedAt: time.Now()},
	}
	d := NewDetail(80, 40, issues)
	d.SetIssue(&issues[0])

	comments := []gastown.Comment{
		{ID: "c-1", Author: "reviewer", Body: "Looks good"},
	}
	d.SetComments("mg-001", comments)

	if d.CommentsIssueID != "mg-001" {
		t.Fatalf("CommentsIssueID = %q, want %q", d.CommentsIssueID, "mg-001")
	}
	if len(d.Comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(d.Comments))
	}
}
