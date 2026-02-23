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

func TestRenderIssueDueBadge(t *testing.T) {
	issues := []data.Issue{
		testIssue("due-1", data.StatusOpen),
	}
	// Set due in 2 days — should show SymDueDate badge
	due := time.Now().Add(2 * 24 * time.Hour)
	issues[0].DueAt = &due

	p := NewParade(issues, 100, 20, data.DefaultBlockingTypes)
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
	if !strings.Contains(out, ui.SymDueDate) {
		t.Fatalf("renderIssue with upcoming due should contain %q, got: %s", ui.SymDueDate, out)
	}
}

func TestRenderIssueOverdueBadge(t *testing.T) {
	issues := []data.Issue{
		testIssue("overdue-1", data.StatusOpen),
	}
	past := time.Now().Add(-3 * 24 * time.Hour)
	issues[0].DueAt = &past

	p := NewParade(issues, 100, 20, data.DefaultBlockingTypes)
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
	if !strings.Contains(out, ui.SymOverdue) {
		t.Fatalf("renderIssue with overdue should contain %q, got: %s", ui.SymOverdue, out)
	}
}

func TestRenderIssueDeferredDim(t *testing.T) {
	issues := []data.Issue{
		testIssue("defer-1", data.StatusOpen),
	}
	future := time.Now().Add(5 * 24 * time.Hour)
	issues[0].DeferUntil = &future

	p := NewParade(issues, 100, 20, data.DefaultBlockingTypes)
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
	if !strings.Contains(out, ui.SymDeferred) {
		t.Fatalf("renderIssue with deferred should contain %q, got: %s", ui.SymDeferred, out)
	}
}

func TestRenderIssueHierarchicalIndent(t *testing.T) {
	parent := testIssue("mg-007", data.StatusOpen)
	child := testIssue("mg-007.1", data.StatusOpen)
	grandchild := testIssue("mg-007.1.1", data.StatusOpen)

	issues := []data.Issue{parent, child, grandchild}
	p := NewParade(issues, 100, 30, data.DefaultBlockingTypes)

	// Collect rendered output for child and grandchild
	for _, it := range p.Items {
		if it.Issue == nil {
			continue
		}
		out := p.renderIssue(it, false)
		switch it.Issue.ID {
		case "mg-007":
			// Parent should not have extra indent (no leading spaces before sym)
		case "mg-007.1":
			// Depth 1 → 2 spaces of indent
			if !strings.Contains(out, "  "+ui.SymLinedUp) {
				t.Errorf("child issue should be indented, got: %s", out)
			}
		case "mg-007.1.1":
			// Depth 2 → 4 spaces of indent
			if !strings.Contains(out, "    "+ui.SymLinedUp) {
				t.Errorf("grandchild issue should be double-indented, got: %s", out)
			}
		}
	}
}

func TestDetailRenderContentDueDate(t *testing.T) {
	iss := testIssue("due-detail-1", data.StatusOpen)
	due := time.Now().Add(2 * 24 * time.Hour)
	iss.DueAt = &due

	d := Detail{
		Width:         80,
		Height:        40,
		BlockingTypes: data.DefaultBlockingTypes,
		Issue:         &iss,
		IssueMap:      data.BuildIssueMap([]data.Issue{iss}),
	}
	out := d.renderContent()
	if !strings.Contains(out, "Due:") {
		t.Fatalf("renderContent with DueAt set should contain 'Due:', got: %s", out)
	}
	if !strings.Contains(out, ui.SymDueDate) {
		t.Fatalf("renderContent with upcoming due should contain %q, got: %s", ui.SymDueDate, out)
	}
}

func TestDetailRenderContentRichDeps(t *testing.T) {
	target := testIssue("dep-target", data.StatusOpen)
	iss := testIssue("rich-dep-1", data.StatusOpen)
	iss.Dependencies = []data.Dependency{
		{IssueID: "rich-dep-1", DependsOnID: "dep-target", Type: "related"},
	}

	allIssues := []data.Issue{target, iss}
	issueMap := data.BuildIssueMap(allIssues)

	d := Detail{
		Width:         80,
		Height:        40,
		BlockingTypes: data.DefaultBlockingTypes,
		Issue:         issueMap["rich-dep-1"],
		IssueMap:      issueMap,
		AllIssues:     allIssues,
	}
	out := d.renderContent()
	if !strings.Contains(out, ui.SymRelated) {
		t.Fatalf("renderContent with related dep should contain %q, got: %s", ui.SymRelated, out)
	}
	if !strings.Contains(out, "related to") {
		t.Fatalf("renderContent with related dep should contain 'related to', got: %s", out)
	}
}

func TestDepTypeDisplay(t *testing.T) {
	tests := []struct {
		depType    string
		wantSym    string
		wantVerb   string
	}{
		{"related", ui.SymRelated, "related to"},
		{"duplicates", ui.SymDuplicates, "duplicates"},
		{"supersedes", ui.SymSupersedes, "supersedes"},
		{"discovered-from", ui.SymNonBlocking, "discovered from"},
		{"waits-for", ui.SymStalled, "waits for"},
		{"parent-child", ui.DepTree, "child of"},
		{"replies-to", ui.SymNonBlocking, "replies to"},
		{"unknown-type", ui.SymNonBlocking, "unknown-type"},
	}
	for _, tc := range tests {
		t.Run(tc.depType, func(t *testing.T) {
			sym, verb, _ := depTypeDisplay(tc.depType)
			if sym != tc.wantSym {
				t.Errorf("depTypeDisplay(%q) sym = %q, want %q", tc.depType, sym, tc.wantSym)
			}
			if verb != tc.wantVerb {
				t.Errorf("depTypeDisplay(%q) verb = %q, want %q", tc.depType, verb, tc.wantVerb)
			}
		})
	}
}

func TestSectionHintsNoGasTown(t *testing.T) {
	p := newTestParade()
	p.HasGasTown = false

	rolling := paradeSection{Status: data.ParadeRolling}
	linedUp := paradeSection{Status: data.ParadeLinedUp}
	stalled := paradeSection{Status: data.ParadeStalled}
	passed := paradeSection{Status: data.ParadePastTheStand}

	// Rolling without Gas Town shows agent + close
	h := p.sectionHints(rolling)
	if !strings.Contains(h, "agent") {
		t.Errorf("Rolling hints without GT should contain 'agent', got: %s", h)
	}
	if !strings.Contains(h, "close") {
		t.Errorf("Rolling hints without GT should contain 'close', got: %s", h)
	}

	// Lined Up without Gas Town shows start + agent
	h = p.sectionHints(linedUp)
	if !strings.Contains(h, "start") {
		t.Errorf("Lined Up hints without GT should contain 'start', got: %s", h)
	}

	// Stalled shows details
	h = p.sectionHints(stalled)
	if !strings.Contains(h, "details") {
		t.Errorf("Stalled hints should contain 'details', got: %s", h)
	}

	// Past the Stand has no hints (handled by title text)
	h = p.sectionHints(passed)
	if h != "" {
		t.Errorf("Past the Stand hints should be empty, got: %s", h)
	}
}

func TestSectionHintsGasTown(t *testing.T) {
	p := newTestParade()
	p.HasGasTown = true

	rolling := paradeSection{Status: data.ParadeRolling}
	linedUp := paradeSection{Status: data.ParadeLinedUp}

	// Rolling with Gas Town shows nudge + unsling
	h := p.sectionHints(rolling)
	if !strings.Contains(h, "nudge") {
		t.Errorf("Rolling hints with GT should contain 'nudge', got: %s", h)
	}
	if !strings.Contains(h, "unsling") {
		t.Errorf("Rolling hints with GT should contain 'unsling', got: %s", h)
	}

	// Lined Up with Gas Town shows sling + formula
	h = p.sectionHints(linedUp)
	if !strings.Contains(h, "sling") {
		t.Errorf("Lined Up hints with GT should contain 'sling', got: %s", h)
	}
	if !strings.Contains(h, "formula") {
		t.Errorf("Lined Up hints with GT should contain 'formula', got: %s", h)
	}
}

func TestRenderBorderTopShowsHints(t *testing.T) {
	issues := []data.Issue{
		testIssue("roll-1", data.StatusInProgress),
	}
	p := NewParade(issues, 80, 20, data.DefaultBlockingTypes)
	p.HasGasTown = true

	sec := sections[0] // Rolling
	out := p.renderBorderTop(sec)
	if !strings.Contains(out, "nudge") {
		t.Errorf("renderBorderTop for Rolling with GT should contain 'nudge' hint, got: %s", out)
	}
}

func TestRenderBorderTopHiddenWhenNarrow(t *testing.T) {
	issues := []data.Issue{
		testIssue("roll-1", data.StatusInProgress),
	}
	// Very narrow width — hints shouldn't fit
	p := NewParade(issues, 30, 20, data.DefaultBlockingTypes)
	p.HasGasTown = true

	sec := sections[0] // Rolling
	out := p.renderBorderTop(sec)
	// Should still render without crashing, just no hints
	if !strings.Contains(out, "Rolling") {
		t.Errorf("renderBorderTop should still contain title even when narrow, got: %s", out)
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
