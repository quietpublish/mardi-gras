package data

import (
	"testing"
	"time"
)

func TestParentID(t *testing.T) {
	tests := []struct {
		id   string
		want string
	}{
		{"mg-007", ""},
		{"mg-007.1", "mg-007"},
		{"mg-007.2.1", "mg-007.2"},
		{"bd-a3f8.1.1", "bd-a3f8.1"},
		{"simple", ""},
	}
	for _, tc := range tests {
		t.Run(tc.id, func(t *testing.T) {
			iss := Issue{ID: tc.id}
			if got := iss.ParentID(); got != tc.want {
				t.Errorf("ParentID(%q) = %q, want %q", tc.id, got, tc.want)
			}
		})
	}
}

func TestParentRelationshipDepth(t *testing.T) {
	root := &Issue{ID: "root.legacy"}
	child := &Issue{
		ID: "child",
		Dependencies: []Dependency{{
			IssueID:     "child",
			DependsOnID: "root.legacy",
			Type:        "parent-child",
		}},
	}
	grandchild := &Issue{
		ID: "grandchild.with.dots",
		Dependencies: []Dependency{{
			IssueID:     "grandchild.with.dots",
			DependsOnID: "child",
			Type:        "parent-child",
		}},
	}
	historicalID := &Issue{ID: "former.child"}
	missingParent := &Issue{
		ID: "orphan",
		Dependencies: []Dependency{{
			IssueID:     "orphan",
			DependsOnID: "not-loaded",
			Type:        "parent-child",
		}},
	}
	nonParentDependency := &Issue{
		ID: "related",
		Dependencies: []Dependency{{
			IssueID:     "related",
			DependsOnID: root.ID,
			Type:        "related",
		}},
	}
	cycleA := &Issue{ID: "cycle-a", Dependencies: []Dependency{{IssueID: "cycle-a", DependsOnID: "cycle-b", Type: "parent-child"}}}
	cycleB := &Issue{ID: "cycle-b", Dependencies: []Dependency{{IssueID: "cycle-b", DependsOnID: "cycle-a", Type: "parent-child"}}}
	issueMap := map[string]*Issue{
		root.ID:                root,
		child.ID:               child,
		grandchild.ID:          grandchild,
		historicalID.ID:        historicalID,
		missingParent.ID:       missingParent,
		nonParentDependency.ID: nonParentDependency,
		cycleA.ID:              cycleA,
		cycleB.ID:              cycleB,
	}

	tests := []struct {
		name  string
		issue *Issue
		want  int
	}{
		{"top level dotted ID", root, 0},
		{"actual child", child, 1},
		{"actual grandchild", grandchild, 2},
		{"historical ID without relationship", historicalID, 0},
		{"parent not loaded", missingParent, 0},
		{"non-parent dependency", nonParentDependency, 0},
		{"cycle stops after loaded parent", cycleA, 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.issue.ParentRelationshipDepth(issueMap); got != tc.want {
				t.Errorf("ParentRelationshipDepth() = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestIsOverdue(t *testing.T) {
	past := time.Now().Add(-48 * time.Hour)
	future := time.Now().Add(48 * time.Hour)

	tests := []struct {
		name   string
		dueAt  *time.Time
		status Status
		want   bool
	}{
		{"nil due", nil, StatusOpen, false},
		{"past due, open", &past, StatusOpen, true},
		{"past due, closed", &past, StatusClosed, false},
		{"future due, open", &future, StatusOpen, false},
		{"past due, in_progress", &past, StatusInProgress, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			iss := Issue{DueAt: tc.dueAt, Status: tc.status}
			if got := iss.IsOverdue(); got != tc.want {
				t.Errorf("IsOverdue() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestIsDeferred(t *testing.T) {
	past := time.Now().Add(-24 * time.Hour)
	future := time.Now().Add(5 * 24 * time.Hour)

	tests := []struct {
		name       string
		deferUntil *time.Time
		want       bool
	}{
		{"nil", nil, false},
		{"past", &past, false},
		{"future", &future, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			iss := Issue{DeferUntil: tc.deferUntil}
			if got := iss.IsDeferred(); got != tc.want {
				t.Errorf("IsDeferred() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestDueLabel(t *testing.T) {
	tests := []struct {
		name   string
		offset time.Duration
		want   string
	}{
		{"3 days overdue", -3 * 24 * time.Hour, "3d overdue"},
		{"due today (slightly past)", -2 * time.Hour, "due today"},
		{"due today (slightly future)", 6 * time.Hour, "due today"},
		{"1 day left", 36 * time.Hour, "1d left"},
		{"5 days left", 5*24*time.Hour + 12*time.Hour, "5d left"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			due := time.Now().Add(tc.offset)
			iss := Issue{DueAt: &due}
			got := iss.DueLabel()
			if got != tc.want {
				t.Errorf("DueLabel() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestDueLabelNil(t *testing.T) {
	iss := Issue{}
	if got := iss.DueLabel(); got != "" {
		t.Errorf("DueLabel() with nil DueAt = %q, want empty", got)
	}
}

func TestDeferLabel(t *testing.T) {
	future := time.Now().Add(5*24*time.Hour + 12*time.Hour)
	iss := Issue{DeferUntil: &future}
	got := iss.DeferLabel()
	if got != "deferred 5d" {
		t.Errorf("DeferLabel() = %q, want %q", got, "deferred 5d")
	}
}

func TestDeferLabelNil(t *testing.T) {
	iss := Issue{}
	if got := iss.DeferLabel(); got != "" {
		t.Errorf("DeferLabel() with nil DeferUntil = %q, want empty", got)
	}
}

func TestDeferLabelPast(t *testing.T) {
	past := time.Now().Add(-24 * time.Hour)
	iss := Issue{DeferUntil: &past}
	if got := iss.DeferLabel(); got != "" {
		t.Errorf("DeferLabel() with past DeferUntil = %q, want empty", got)
	}
}

func TestAgeLabel(t *testing.T) {
	tests := []struct {
		name   string
		offset time.Duration
		want   string
	}{
		{"just created", -10 * time.Second, "just now"},
		{"30 minutes ago is still 'just now'", -30 * time.Minute, "just now"},
		{"two hours ago", -2*time.Hour - 30*time.Minute, "2h"},
		{"23 hours ago", -23 * time.Hour, "23h"},
		{"exactly 1 day", -25 * time.Hour, "1 day"},
		{"5 days ago", -5*24*time.Hour - time.Hour, "5 days"},
		{"29 days ago is still days", -29*24*time.Hour - time.Hour, "29 days"},
		{"30 days converts to weeks", -30*24*time.Hour - time.Hour, "4 weeks"},
		{"8 weeks ago", -56*24*time.Hour - time.Hour, "8 weeks"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			iss := Issue{CreatedAt: time.Now().Add(tc.offset)}
			got := iss.AgeLabel()
			if got != tc.want {
				t.Errorf("AgeLabel(offset=%v) = %q, want %q", tc.offset, got, tc.want)
			}
		})
	}
}

func TestAgePositive(t *testing.T) {
	iss := Issue{CreatedAt: time.Now().Add(-time.Hour)}
	if age := iss.Age(); age < 50*time.Minute || age > 70*time.Minute {
		t.Errorf("Age() = %v, want ~1h", age)
	}
}

func hierIssue(id, parent string, status Status, pri Priority) Issue {
	iss := Issue{ID: id, Title: id, Status: status, Priority: pri, IssueType: TypeTask, CreatedAt: time.Now()}
	if parent != "" {
		iss.Dependencies = []Dependency{{IssueID: id, DependsOnID: parent, Type: "parent-child"}}
	}
	return iss
}

// TestOrderHierarchicallyPlacesChildrenUnderParents is the core of the #110
// fix: sorting is priority-then-recency, so a child could land far from its
// parent while still being indented — making the indent claim a parent it did
// not have. Children now follow their parent directly.
func TestOrderHierarchicallyPlacesChildrenUnderParents(t *testing.T) {
	// Arrives in priority order, which interleaves two families.
	issues := []Issue{
		hierIssue("mg-100", "", StatusOpen, PriorityHigh),
		hierIssue("mg-200", "", StatusOpen, PriorityHigh),
		hierIssue("mg-201", "mg-200", StatusOpen, PriorityMedium),
		hierIssue("mg-101", "mg-100", StatusOpen, PriorityLow),
	}
	ordered, depth := OrderHierarchically(issues)

	var ids []string
	for _, iss := range ordered {
		ids = append(ids, iss.ID)
	}
	want := []string{"mg-100", "mg-101", "mg-200", "mg-201"}
	if len(ids) != len(want) {
		t.Fatalf("ordered = %v, want %v", ids, want)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("ordered = %v, want %v", ids, want)
		}
	}
	for id, wantDepth := range map[string]int{"mg-100": 0, "mg-101": 1, "mg-200": 0, "mg-201": 1} {
		if depth[id] != wantDepth {
			t.Errorf("depth[%s] = %d, want %d", id, depth[id], wantDepth)
		}
	}
}

// TestOrderHierarchicallyParentOutsideSection is the cross-section case #110
// was filed for: a child whose parent is not on screen here must render flat,
// not indented under whatever unrelated row precedes it.
func TestOrderHierarchicallyParentOutsideSection(t *testing.T) {
	issues := []Issue{
		hierIssue("mg-001", "", StatusInProgress, PriorityHigh),
		hierIssue("mg-101", "mg-100", StatusInProgress, PriorityMedium), // parent lives elsewhere
	}
	_, depth := OrderHierarchically(issues)
	if depth["mg-101"] != 0 {
		t.Errorf("depth[mg-101] = %d, want 0 — its parent is not in this section", depth["mg-101"])
	}
}

// TestOrderHierarchicallyReparentedKeepsDottedID covers #102's original report:
// the dotted ID survives reparenting, so only the edge may decide depth.
func TestOrderHierarchicallyReparentedKeepsDottedID(t *testing.T) {
	issues := []Issue{
		hierIssue("mg-100", "", StatusOpen, PriorityHigh),
		hierIssue("mg-100.20", "", StatusOpen, PriorityMedium), // dotted, but edge removed
	}
	_, depth := OrderHierarchically(issues)
	if depth["mg-100.20"] != 0 {
		t.Errorf("depth[mg-100.20] = %d, want 0 — the parent-child edge was removed", depth["mg-100.20"])
	}
}

// TestOrderHierarchicallyCycleEmitsEveryIssue guards against a malformed graph
// silently dropping rows from the parade.
func TestOrderHierarchicallyCycleEmitsEveryIssue(t *testing.T) {
	issues := []Issue{
		hierIssue("mg-a", "mg-b", StatusOpen, PriorityHigh),
		hierIssue("mg-b", "mg-a", StatusOpen, PriorityHigh),
		hierIssue("mg-c", "", StatusOpen, PriorityHigh),
	}
	ordered, _ := OrderHierarchically(issues)
	if len(ordered) != 3 {
		t.Fatalf("ordered %d issues, want 3 — a cycle must not drop rows", len(ordered))
	}
	seen := map[string]bool{}
	for _, iss := range ordered {
		if seen[iss.ID] {
			t.Fatalf("issue %s emitted twice", iss.ID)
		}
		seen[iss.ID] = true
	}
}

func TestOrderHierarchicallyEmpty(t *testing.T) {
	ordered, depth := OrderHierarchically(nil)
	if len(ordered) != 0 || len(depth) != 0 {
		t.Errorf("OrderHierarchically(nil) = %v, %v", ordered, depth)
	}
}
