package views

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
	"github.com/matt-wright86/mardi-gras/internal/data"
	"github.com/matt-wright86/mardi-gras/internal/ui"
)

func testIssue(id string, status data.Status) data.Issue {
	now := time.Now()
	return data.Issue{
		ID:        id,
		Title:     id,
		Status:    status,
		Priority:  data.PriorityMedium,
		IssueType: data.TypeTask,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func blockedIssue(id string, status data.Status) data.Issue {
	iss := testIssue(id, status)
	iss.Dependencies = []data.Dependency{
		{IssueID: id, DependsOnID: "missing-dep", Type: "blocks"},
	}
	return iss
}

func TestStatusSymbol(t *testing.T) {
	bt := data.DefaultBlockingTypes
	emptyMap := map[string]*data.Issue{}

	tests := []struct {
		name   string
		issue  data.Issue
		expect string
	}{
		{
			name:   "closed",
			issue:  testIssue("closed-1", data.StatusClosed),
			expect: ui.SymPassed,
		},
		{
			name:   "in_progress not blocked",
			issue:  testIssue("rolling-1", data.StatusInProgress),
			expect: ui.SymRolling,
		},
		{
			name:   "in_progress blocked",
			issue:  blockedIssue("stalled-ip-1", data.StatusInProgress),
			expect: ui.SymStalled,
		},
		{
			name:   "open not blocked",
			issue:  testIssue("open-1", data.StatusOpen),
			expect: ui.SymLinedUp,
		},
		{
			name:   "open blocked",
			issue:  blockedIssue("stalled-open-1", data.StatusOpen),
			expect: ui.SymStalled,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := statusSymbol(&tc.issue, emptyMap, bt)
			if got != tc.expect {
				t.Fatalf("statusSymbol(%s) = %q, want %q", tc.issue.ID, got, tc.expect)
			}
		})
	}
}

func TestStatusColor(t *testing.T) {
	bt := data.DefaultBlockingTypes
	emptyMap := map[string]*data.Issue{}

	tests := []struct {
		name   string
		issue  data.Issue
		expect lipgloss.Color
	}{
		{
			name:   "closed",
			issue:  testIssue("closed-1", data.StatusClosed),
			expect: ui.StatusPassed,
		},
		{
			name:   "in_progress not blocked",
			issue:  testIssue("rolling-1", data.StatusInProgress),
			expect: ui.StatusRolling,
		},
		{
			name:   "in_progress blocked",
			issue:  blockedIssue("stalled-ip-1", data.StatusInProgress),
			expect: ui.StatusStalled,
		},
		{
			name:   "open not blocked",
			issue:  testIssue("open-1", data.StatusOpen),
			expect: ui.StatusLinedUp,
		},
		{
			name:   "open blocked",
			issue:  blockedIssue("stalled-open-1", data.StatusOpen),
			expect: ui.StatusStalled,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := statusColor(&tc.issue, emptyMap, bt)
			if got != tc.expect {
				t.Fatalf("statusColor(%s) = %v, want %v", tc.issue.ID, got, tc.expect)
			}
		})
	}
}

func TestParadeViewEmpty(t *testing.T) {
	p := NewParade(nil, 60, 20, data.DefaultBlockingTypes)
	out := p.View()
	if !strings.Contains(out, "No issues found") {
		t.Fatalf("empty parade should contain 'No issues found', got: %s", out)
	}
}

func TestParadeViewSections(t *testing.T) {
	issues := []data.Issue{
		testIssue("roll-1", data.StatusInProgress),
		testIssue("open-1", data.StatusOpen),
	}
	p := NewParade(issues, 80, 40, data.DefaultBlockingTypes)
	out := p.View()

	if !strings.Contains(out, "Rolling") {
		t.Fatal("parade output should contain 'Rolling' section title")
	}
	if !strings.Contains(out, "Lined Up") {
		t.Fatal("parade output should contain 'Lined Up' section title")
	}
}

func TestParadeViewClosedHidden(t *testing.T) {
	issues := []data.Issue{
		testIssue("open-1", data.StatusOpen),
		testIssue("closed-hidden", data.StatusClosed),
	}
	p := NewParade(issues, 80, 40, data.DefaultBlockingTypes)
	// ShowClosed defaults to false
	out := p.View()

	if strings.Contains(out, "closed-hidden") {
		t.Fatal("closed issue ID should not appear when ShowClosed is false")
	}
}

func TestParadeViewClosedShown(t *testing.T) {
	issues := []data.Issue{
		testIssue("open-1", data.StatusOpen),
		testIssue("closed-shown", data.StatusClosed),
	}
	p := NewParade(issues, 80, 40, data.DefaultBlockingTypes)
	p.ToggleClosed()
	out := p.View()

	if !strings.Contains(out, "closed-shown") {
		t.Fatal("closed issue ID should appear when ShowClosed is true")
	}
}

func TestRenderIssueCursor(t *testing.T) {
	issues := []data.Issue{
		testIssue("cursor-1", data.StatusOpen),
	}
	p := NewParade(issues, 80, 20, data.DefaultBlockingTypes)

	var item ParadeItem
	for _, it := range p.Items {
		if it.Issue != nil {
			item = it
			break
		}
	}
	if item.Issue == nil {
		t.Fatal("no selectable item found")
	}

	out := p.renderIssue(item, true)
	if !strings.Contains(out, ui.Cursor) {
		t.Fatalf("renderIssue with selected=true should contain cursor %q, got: %s", ui.Cursor, out)
	}
}

func TestRenderIssueNoCursor(t *testing.T) {
	issues := []data.Issue{
		testIssue("nocursor-1", data.StatusOpen),
	}
	p := NewParade(issues, 80, 20, data.DefaultBlockingTypes)

	var item ParadeItem
	for _, it := range p.Items {
		if it.Issue != nil {
			item = it
			break
		}
	}
	if item.Issue == nil {
		t.Fatal("no selectable item found")
	}

	out := p.renderIssue(item, false)
	if strings.Contains(out, ui.Cursor) {
		t.Fatalf("renderIssue with selected=false should not contain cursor %q, got: %s", ui.Cursor, out)
	}
}

func TestRenderIssueMultiSelect(t *testing.T) {
	issues := []data.Issue{
		testIssue("sel-1", data.StatusOpen),
	}
	p := NewParade(issues, 80, 20, data.DefaultBlockingTypes)
	p.Selected = map[string]bool{"sel-1": true}

	var item ParadeItem
	for _, it := range p.Items {
		if it.Issue != nil {
			item = it
			break
		}
	}
	if item.Issue == nil {
		t.Fatal("no selectable item found")
	}

	out := p.renderIssue(item, false)
	if !strings.Contains(out, ui.SymSelected) {
		t.Fatalf("renderIssue with multi-select should contain %q, got: %s", ui.SymSelected, out)
	}
}

func TestRenderIssueChangedDot(t *testing.T) {
	issues := []data.Issue{
		testIssue("chg-1", data.StatusOpen),
	}
	p := NewParade(issues, 80, 20, data.DefaultBlockingTypes)
	p.ChangedIDs = map[string]bool{"chg-1": true}

	var item ParadeItem
	for _, it := range p.Items {
		if it.Issue != nil {
			item = it
			break
		}
	}
	if item.Issue == nil {
		t.Fatal("no selectable item found")
	}

	out := p.renderIssue(item, false)
	if !strings.Contains(out, ui.SymChanged) {
		t.Fatalf("renderIssue with ChangedIDs should contain %q, got: %s", ui.SymChanged, out)
	}
}

func TestDetailViewNilIssue(t *testing.T) {
	d := Detail{Width: 60, Height: 20}
	d.Viewport = viewport.New(58, 20)
	out := d.View()

	if !strings.Contains(out, "No issue selected") {
		t.Fatalf("detail with nil issue should contain 'No issue selected', got: %s", out)
	}
}

func TestDetailRenderContentSections(t *testing.T) {
	d := Detail{Width: 80, Height: 40, BlockingTypes: data.DefaultBlockingTypes}
	iss := testIssue("test-1", data.StatusOpen)
	iss.Description = "desc text"
	iss.Notes = "notes text"
	iss.AcceptanceCriteria = "ac text"
	iss.Design = "design text"
	iss.CloseReason = "reason text"
	d.Issue = &iss
	d.IssueMap = data.BuildIssueMap([]data.Issue{iss})
	out := d.renderContent()

	for _, section := range []string{"DESCRIPTION", "NOTES", "ACCEPTANCE CRITERIA", "DESIGN", "CLOSE REASON"} {
		if !strings.Contains(out, section) {
			t.Errorf("renderContent should contain section %q, got: %s", section, out)
		}
	}
}

func TestDetailRenderContentDependencies(t *testing.T) {
	blocker := testIssue("dep-1", data.StatusOpen)
	blocked := testIssue("blocked-1", data.StatusOpen)
	blocked.Dependencies = []data.Dependency{
		{IssueID: "blocked-1", DependsOnID: "dep-1", Type: "blocks"},
	}

	allIssues := []data.Issue{blocker, blocked}
	issueMap := data.BuildIssueMap(allIssues)

	d := Detail{
		Width:         80,
		Height:        40,
		BlockingTypes: data.DefaultBlockingTypes,
		Issue:         issueMap["blocked-1"],
		IssueMap:      issueMap,
		AllIssues:     allIssues,
	}
	out := d.renderContent()

	if !strings.Contains(out, "DEPENDENCIES") {
		t.Fatalf("renderContent for blocked issue should contain 'DEPENDENCIES', got: %s", out)
	}
	if !strings.Contains(out, "waiting on") {
		t.Fatalf("renderContent for blocked issue should contain 'waiting on', got: %s", out)
	}
}

func TestDetailRenderContentOwnerAssignee(t *testing.T) {
	iss := testIssue("owner-1", data.StatusOpen)
	iss.Owner = "alice"
	iss.Assignee = "bob"

	d := Detail{
		Width:         80,
		Height:        40,
		BlockingTypes: data.DefaultBlockingTypes,
		Issue:         &iss,
		IssueMap:      data.BuildIssueMap([]data.Issue{iss}),
	}
	out := d.renderContent()

	if !strings.Contains(out, "Owner:") {
		t.Fatalf("renderContent should contain 'Owner:', got: %s", out)
	}
	if !strings.Contains(out, "Assignee:") {
		t.Fatalf("renderContent should contain 'Assignee:', got: %s", out)
	}
}
