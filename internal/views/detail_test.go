package views

import (
	"strings"
	"testing"
	"time"

	"github.com/matt-wright86/mardi-gras/internal/data"
	"github.com/matt-wright86/mardi-gras/internal/gastown"
	"github.com/matt-wright86/mardi-gras/internal/ui"
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
	// DAG flow connectors between tiers
	if !strings.Contains(content, ui.SymDAGFlow) {
		t.Error("content should contain DAG flow connector between tiers")
	}
	// Step symbols
	if !strings.Contains(content, ui.SymStepDone) {
		t.Error("content should contain done step symbol")
	}
	if !strings.Contains(content, ui.SymStepActive) {
		t.Error("content should contain active step symbol")
	}
}

func TestMoleculeDAGParallelBranching(t *testing.T) {
	issues := []data.Issue{
		{ID: "mg-001", Title: "Test Issue", Status: data.StatusInProgress, Priority: data.PriorityMedium, IssueType: data.TypeTask, CreatedAt: time.Now()},
	}
	d := NewDetail(80, 40, issues)
	d.SetIssue(&issues[0])

	dag := &gastown.DAGInfo{
		RootID:    "mg-001",
		RootTitle: "Shiny Workflow",
		Nodes: map[string]*gastown.DAGNode{
			"s1": {ID: "s1", Title: "Design", Status: "done", Tier: 0},
			"s2": {ID: "s2", Title: "Implement A", Status: "in_progress", Tier: 1, Parallel: true},
			"s3": {ID: "s3", Title: "Implement B", Status: "in_progress", Tier: 1, Parallel: true},
			"s4": {ID: "s4", Title: "Test", Status: "blocked", Tier: 2},
			"s5": {ID: "s5", Title: "Submit", Status: "blocked", Tier: 3},
		},
		TierGroups: [][]string{{"s1"}, {"s2", "s3"}, {"s4"}, {"s5"}},
	}
	d.SetMolecule("mg-001", dag, nil)
	content := d.renderContent()

	// Parallel branch connectors
	if !strings.Contains(content, ui.SymDAGBranch) {
		t.Error("content should contain branch start connector for parallel nodes")
	}
	if !strings.Contains(content, ui.SymDAGJoin) {
		t.Error("content should contain branch end connector for parallel nodes")
	}
	if !strings.Contains(content, "Implement A") {
		t.Error("content should contain parallel step 'Implement A'")
	}
	if !strings.Contains(content, "Implement B") {
		t.Error("content should contain parallel step 'Implement B'")
	}
}

func TestMoleculeDAGFiveWayParallel(t *testing.T) {
	issues := []data.Issue{
		{ID: "mg-001", Title: "Test Issue", Status: data.StatusInProgress, Priority: data.PriorityMedium, IssueType: data.TypeTask, CreatedAt: time.Now()},
	}
	d := NewDetail(80, 40, issues)
	d.SetIssue(&issues[0])

	dag := &gastown.DAGInfo{
		RootID:    "mg-001",
		RootTitle: "Rule of Five",
		Nodes: map[string]*gastown.DAGNode{
			"s1": {ID: "s1", Title: "Implement", Status: "done", Tier: 0},
			"r1": {ID: "r1", Title: "Correctness", Status: "in_progress", Tier: 1, Parallel: true},
			"r2": {ID: "r2", Title: "Security", Status: "ready", Tier: 1, Parallel: true},
			"r3": {ID: "r3", Title: "Performance", Status: "ready", Tier: 1, Parallel: true},
			"r4": {ID: "r4", Title: "Maintainability", Status: "ready", Tier: 1, Parallel: true},
			"r5": {ID: "r5", Title: "Testing", Status: "ready", Tier: 1, Parallel: true},
			"s2": {ID: "s2", Title: "Submit", Status: "blocked", Tier: 2},
		},
		TierGroups: [][]string{{"s1"}, {"r1", "r2", "r3", "r4", "r5"}, {"s2"}},
	}
	d.SetMolecule("mg-001", dag, nil)
	content := d.renderContent()

	// Should have branch start, middle forks, and join
	if !strings.Contains(content, ui.SymDAGBranch) {
		t.Error("content should contain branch start for 5-way parallel")
	}
	if !strings.Contains(content, ui.SymDAGFork) {
		t.Error("content should contain fork connectors for middle parallel nodes")
	}
	if !strings.Contains(content, ui.SymDAGJoin) {
		t.Error("content should contain branch end for 5-way parallel")
	}
	// All five review aspects present
	for _, title := range []string{"Correctness", "Security", "Performance", "Maintainability", "Testing"} {
		if !strings.Contains(content, title) {
			t.Errorf("content should contain parallel step %q", title)
		}
	}
}

func TestMoleculeDAGCriticalPathTitles(t *testing.T) {
	issues := []data.Issue{
		{ID: "mg-001", Title: "Test Issue", Status: data.StatusInProgress, Priority: data.PriorityMedium, IssueType: data.TypeTask, CreatedAt: time.Now()},
	}
	d := NewDetail(80, 40, issues)
	d.SetIssue(&issues[0])

	dag := &gastown.DAGInfo{
		RootID:    "mg-001",
		RootTitle: "Feature",
		Nodes: map[string]*gastown.DAGNode{
			"s1": {ID: "s1", Title: "Design", Status: "done", Tier: 0},
			"s2": {ID: "s2", Title: "Implement", Status: "in_progress", Tier: 1},
			"s3": {ID: "s3", Title: "Submit", Status: "blocked", Tier: 2},
		},
		TierGroups:   [][]string{{"s1"}, {"s2"}, {"s3"}},
		CriticalPath: []string{"s1", "s2", "s3"},
	}
	d.SetMolecule("mg-001", dag, nil)
	content := d.renderContent()

	// Critical path shows titles, not IDs
	if !strings.Contains(content, "critical:") {
		t.Error("content should contain critical path line")
	}
	if !strings.Contains(content, "Design") {
		t.Error("critical path should use title 'Design' not ID 's1'")
	}
	if strings.Contains(content, "s1 ") {
		t.Error("critical path should not show raw IDs")
	}
	// Arrow separator
	if !strings.Contains(content, "→") {
		t.Error("critical path should use → separator")
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
			{Name: "Toast", Role: "polecat", State: "awaiting-gate", HookBead: "mg-001"},
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
			{Name: "Toast", Role: "polecat", State: "working", HookBead: "mg-001"},
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

func TestQualitySectionRendered(t *testing.T) {
	score := float32(0.85)
	cryst := true
	issues := []data.Issue{
		{ID: "mg-001", Title: "HOP Issue", Status: data.StatusClosed,
			Priority:     data.PriorityMedium,
			IssueType:    data.TypeTask,
			CreatedAt:    time.Now(),
			QualityScore: &score,
			Crystallizes: &cryst,
			Creator: &data.EntityRef{
				Name:     "polecat-alpha",
				Platform: "gastown",
				URI:      "hop://gastown/mardi_gras/polecat-alpha",
			},
			Validations: []data.Validation{
				{
					Validator:    data.EntityRef{Name: "witness", Platform: "gastown"},
					Outcome:      data.OutcomeAccepted,
					QualityScore: 0.9,
				},
				{
					Validator:    data.EntityRef{Name: "refinery", Platform: "gastown"},
					Outcome:      data.OutcomeAccepted,
					QualityScore: 0.8,
					Comment:      "Clean implementation",
				},
			},
		},
	}
	d := NewDetail(80, 40, issues)
	d.SetIssue(&issues[0])

	content := d.renderContent()

	if !strings.Contains(content, "QUALITY") {
		t.Error("content should contain QUALITY section")
	}
	if !strings.Contains(content, "0.85") {
		t.Error("content should contain quality score")
	}
	if !strings.Contains(content, "good") {
		t.Error("content should contain quality label 'good'")
	}
	if !strings.Contains(content, "polecat-alpha") {
		t.Error("content should contain creator name")
	}
	if !strings.Contains(content, "witness") {
		t.Error("content should contain validator name")
	}
	if !strings.Contains(content, "refinery") {
		t.Error("content should contain second validator name")
	}
	if !strings.Contains(content, "crystallizes") {
		t.Error("content should contain crystallization indicator")
	}
}

func TestQualitySectionNotRenderedWithoutScore(t *testing.T) {
	issues := []data.Issue{
		{ID: "mg-001", Title: "No HOP", Status: data.StatusOpen,
			Priority:  data.PriorityMedium,
			IssueType: data.TypeTask,
			CreatedAt: time.Now()},
	}
	d := NewDetail(80, 40, issues)
	d.SetIssue(&issues[0])

	content := d.renderContent()

	if strings.Contains(content, "QUALITY") {
		t.Error("content should not contain QUALITY section when no quality score")
	}
}

func TestFormulaRecommendationRendered(t *testing.T) {
	issues := []data.Issue{
		{ID: "bd-001", Title: "Add authentication middleware", Status: data.StatusOpen,
			Priority: data.PriorityHigh, IssueType: data.TypeFeature, CreatedAt: time.Now()},
	}
	d := NewDetail(80, 40, issues)
	d.SetIssue(&issues[0])

	content := d.renderContent()

	if !strings.Contains(content, "FORMULA") {
		t.Error("content should contain FORMULA section for open issue")
	}
	if !strings.Contains(content, "security-audit") {
		t.Error("content should contain security-audit recommendation for auth issue")
	}
}

func TestCrossRigDepsRendered(t *testing.T) {
	issues := []data.Issue{
		{ID: "bd-001", Title: "Fix token validation", Status: data.StatusOpen,
			Priority: data.PriorityMedium, IssueType: data.TypeBug, CreatedAt: time.Now(),
			Dependencies: []data.Dependency{
				{IssueID: "bd-001", DependsOnID: "external:gastown:gt-c3f2", Type: "blocks"},
				{IssueID: "bd-001", DependsOnID: "external:wyvern:wy-e5f6", Type: "related"},
			},
		},
	}
	d := NewDetail(80, 40, issues)
	d.SetIssue(&issues[0])

	content := d.renderContent()

	if !strings.Contains(content, "CROSS-RIG") {
		t.Error("content should contain CROSS-RIG section")
	}
	if !strings.Contains(content, "gastown") {
		t.Error("content should contain rig name 'gastown'")
	}
	if !strings.Contains(content, "wyvern") {
		t.Error("content should contain rig name 'wyvern'")
	}
	if !strings.Contains(content, "gt-c3f2") {
		t.Error("content should contain external issue ID")
	}
}

func TestCrossRigDepsNotRenderedForLocalDeps(t *testing.T) {
	issues := []data.Issue{
		{ID: "bd-001", Title: "Local issue", Status: data.StatusOpen,
			Priority: data.PriorityMedium, IssueType: data.TypeTask, CreatedAt: time.Now(),
			Dependencies: []data.Dependency{
				{IssueID: "bd-001", DependsOnID: "bd-002", Type: "blocks"},
			},
		},
		{ID: "bd-002", Title: "Another local", Status: data.StatusOpen,
			Priority: data.PriorityMedium, IssueType: data.TypeTask, CreatedAt: time.Now()},
	}
	d := NewDetail(80, 40, issues)
	d.SetIssue(&issues[0])

	content := d.renderContent()

	if strings.Contains(content, "CROSS-RIG") {
		t.Error("content should not contain CROSS-RIG section for local-only deps")
	}
}

func TestFormulaRecommendationNotRenderedForClosed(t *testing.T) {
	issues := []data.Issue{
		{ID: "bd-001", Title: "Add authentication middleware", Status: data.StatusClosed,
			Priority: data.PriorityHigh, IssueType: data.TypeFeature, CreatedAt: time.Now()},
	}
	d := NewDetail(80, 40, issues)
	d.SetIssue(&issues[0])

	content := d.renderContent()

	if strings.Contains(content, "FORMULA") {
		t.Error("content should not contain FORMULA section for closed issue")
	}
}

func TestQualityRejectedValidation(t *testing.T) {
	score := float32(0.3)
	issues := []data.Issue{
		{ID: "mg-001", Title: "Rejected Issue", Status: data.StatusInProgress,
			Priority:     data.PriorityMedium,
			IssueType:    data.TypeTask,
			CreatedAt:    time.Now(),
			QualityScore: &score,
			Validations: []data.Validation{
				{
					Validator:    data.EntityRef{Name: "witness"},
					Outcome:      data.OutcomeRejected,
					QualityScore: 0.2,
				},
			},
		},
	}
	d := NewDetail(80, 40, issues)
	d.SetIssue(&issues[0])

	content := d.renderContent()

	if !strings.Contains(content, "QUALITY") {
		t.Error("content should contain QUALITY section")
	}
	if !strings.Contains(content, "poor") {
		t.Error("content should contain quality label 'poor'")
	}
	if !strings.Contains(content, "rejected") {
		t.Error("content should contain 'rejected' outcome")
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

func TestMetadataSchemaRendered(t *testing.T) {
	issues := []data.Issue{
		{ID: "mg-001", Title: "Test Issue", Status: data.StatusOpen,
			Priority: data.PriorityMedium, IssueType: data.TypeTask,
			CreatedAt: time.Now()},
	}
	d := NewDetail(80, 40, issues)
	min0 := 0.0
	max100 := 100.0
	d.MetadataSchema = &data.MetadataSchema{
		Mode: "warn",
		Fields: map[string]data.MetadataFieldSchema{
			"team": {
				Type:     data.MetaEnum,
				Required: true,
				Values:   []string{"platform", "frontend", "backend"},
			},
			"priority_score": {
				Type: data.MetaInt,
				Min:  &min0,
				Max:  &max100,
			},
		},
	}
	d.SetIssue(&issues[0])

	content := d.renderContent()

	if !strings.Contains(content, "METADATA") {
		t.Error("content should contain METADATA section")
	}
	if !strings.Contains(content, "warn") {
		t.Error("content should contain mode 'warn'")
	}
	if !strings.Contains(content, "team") {
		t.Error("content should contain field name 'team'")
	}
	if !strings.Contains(content, "enum") {
		t.Error("content should contain field type 'enum'")
	}
	if !strings.Contains(content, "priority_score") {
		t.Error("content should contain field name 'priority_score'")
	}
}

func TestMetadataNotRenderedWithoutSchema(t *testing.T) {
	issues := []data.Issue{
		{ID: "mg-001", Title: "No Metadata", Status: data.StatusOpen,
			Priority: data.PriorityMedium, IssueType: data.TypeTask,
			CreatedAt: time.Now()},
	}
	d := NewDetail(80, 40, issues)
	d.SetIssue(&issues[0])

	content := d.renderContent()

	if strings.Contains(content, "METADATA") {
		t.Error("content should not contain METADATA section without schema")
	}
}

func TestMetadataWithIssueValues(t *testing.T) {
	issues := []data.Issue{
		{ID: "mg-001", Title: "With Metadata", Status: data.StatusOpen,
			Priority: data.PriorityMedium, IssueType: data.TypeTask,
			CreatedAt: time.Now(),
			Metadata: map[string]interface{}{
				"team":   "frontend",
				"urgent": true,
			},
		},
	}
	d := NewDetail(80, 40, issues)
	d.MetadataSchema = &data.MetadataSchema{
		Mode: "warn",
		Fields: map[string]data.MetadataFieldSchema{
			"team": {
				Type:     data.MetaEnum,
				Required: true,
				Values:   []string{"platform", "frontend", "backend"},
			},
			"urgent": {
				Type: data.MetaBool,
			},
		},
	}
	d.SetIssue(&issues[0])

	content := d.renderContent()

	if !strings.Contains(content, "METADATA") {
		t.Error("content should contain METADATA section")
	}
	if !strings.Contains(content, "frontend") {
		t.Error("content should contain metadata value 'frontend'")
	}
	if !strings.Contains(content, "true") {
		t.Error("content should contain metadata value 'true'")
	}
}

func TestMetadataRawValuesWithoutSchema(t *testing.T) {
	issues := []data.Issue{
		{ID: "mg-001", Title: "Raw Metadata", Status: data.StatusOpen,
			Priority: data.PriorityMedium, IssueType: data.TypeTask,
			CreatedAt: time.Now(),
			Metadata: map[string]interface{}{
				"custom_field": "value123",
			},
		},
	}
	d := NewDetail(80, 40, issues)
	d.SetIssue(&issues[0])

	content := d.renderContent()

	if !strings.Contains(content, "METADATA") {
		t.Error("content should contain METADATA section for raw metadata")
	}
	if !strings.Contains(content, "custom_field") {
		t.Error("content should contain raw metadata key")
	}
	if !strings.Contains(content, "value123") {
		t.Error("content should contain raw metadata value")
	}
}
