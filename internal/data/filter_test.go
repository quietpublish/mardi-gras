package data

import (
	"testing"
)

func TestExcludeByType(t *testing.T) {
	issues := []Issue{
		{ID: "vv-001", Title: "Fix login bug", IssueType: TypeBug},
		{ID: "vv-002", Title: "Parent epic", IssueType: TypeEpic},
		{ID: "vv-003", Title: "Chore cleanup", IssueType: TypeChore},
	}

	result := ExcludeByType(issues, map[string]bool{"epic": true, "chore": true})
	if len(result) != 1 {
		t.Fatalf("expected 1 issue after exclude, got %d", len(result))
	}
	if result[0].ID != "vv-001" {
		t.Fatalf("expected vv-001 to remain, got %q", result[0].ID)
	}
}

func TestFilterIssues(t *testing.T) {
	issues := []Issue{
		{ID: "vv-001", Title: "Fix login bug", IssueType: TypeBug, Priority: PriorityCritical},
		{ID: "vv-002", Title: "Add search feature", IssueType: TypeFeature, Priority: PriorityHigh},
		{ID: "vv-003", Title: "Update documentation", IssueType: TypeChore, Priority: PriorityLow},
		{ID: "vv-004", Title: "Refactor auth flow", IssueType: TypeTask, Priority: PriorityMedium},
	}

	tests := []struct {
		name     string
		query    string
		expected []string // expected issue IDs
	}{
		{
			name:     "Empty query",
			query:    "",
			expected: []string{"vv-001", "vv-002", "vv-003", "vv-004"},
		},
		{
			name:     "Free text partial word",
			query:    "login",
			expected: []string{"vv-001"},
		},
		{
			name:     "Free text case insensitive",
			query:    "search",
			expected: []string{"vv-002"},
		},
		{
			name:     "Free text matches ID",
			query:    "vv-003",
			expected: []string{"vv-003"},
		},
		{
			name:     "Type filter exact",
			query:    "type:bug",
			expected: []string{"vv-001"},
		},
		{
			name:     "Type filter and free text",
			query:    "type:feature search",
			expected: []string{"vv-002"},
		},
		{
			name:     "Type filter no match",
			query:    "type:epic",
			expected: []string{},
		},
		{
			name:     "Priority short label (p0)",
			query:    "p0",
			expected: []string{"vv-001"},
		},
		{
			name:     "Priority short label (p1)",
			query:    "P1",
			expected: []string{"vv-002"},
		},
		{
			name:     "Priority explicit number",
			query:    "priority:3",
			expected: []string{"vv-003"},
		},
		{
			name:     "Priority explicit name",
			query:    "priority:medium",
			expected: []string{"vv-004"},
		},
		{
			name:     "Multiple structured tokens combined",
			query:    "type:feature p1",
			expected: []string{"vv-002"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FilterIssues(issues, tt.query)
			if len(result) != len(tt.expected) {
				t.Fatalf("expected %d results, got %d", len(tt.expected), len(result))
			}

			// Verify IDs
			resMap := make(map[string]bool)
			for _, r := range result {
				resMap[r.ID] = true
			}
			for _, exp := range tt.expected {
				if !resMap[exp] {
					t.Errorf("expected issue %s to be in result, but it was not", exp)
				}
			}
		})
	}
}

func TestFilterIssuesFuzzy(t *testing.T) {
	issues := []Issue{
		{ID: "vv-001", Title: "Login token expiry bug", IssueType: TypeBug, Priority: PriorityCritical},
		{ID: "vv-002", Title: "Add search feature", IssueType: TypeFeature, Priority: PriorityHigh},
		{ID: "vv-003", Title: "Update documentation", IssueType: TypeChore, Priority: PriorityLow},
	}

	// Fuzzy matching: "lgn tkn" should match "Login token expiry bug"
	result := FilterIssues(issues, "lgn tkn")
	found := false
	for _, r := range result {
		if r.ID == "vv-001" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("fuzzy query 'lgn tkn' should match 'Login token expiry bug', got %d results", len(result))
	}
}

func TestFilterIssuesSearchDescription(t *testing.T) {
	issues := []Issue{
		{ID: "vv-001", Title: "Fix login bug", Description: "Token expiry causes redirect loop"},
		{ID: "vv-002", Title: "Add search feature", Description: "Full-text search across issues"},
	}

	result := FilterIssues(issues, "redirect loop")
	if len(result) != 1 || result[0].ID != "vv-001" {
		ids := make([]string, len(result))
		for i, r := range result {
			ids[i] = r.ID
		}
		t.Fatalf("expected [vv-001], got %v", ids)
	}
}

func TestFilterIssuesSearchLabels(t *testing.T) {
	issues := []Issue{
		{ID: "vv-001", Title: "Fix login bug", Labels: []string{"backend", "security"}},
		{ID: "vv-002", Title: "Add search feature", Labels: []string{"frontend"}},
	}

	result := FilterIssues(issues, "security")
	if len(result) != 1 || result[0].ID != "vv-001" {
		ids := make([]string, len(result))
		for i, r := range result {
			ids[i] = r.ID
		}
		t.Fatalf("expected [vv-001], got %v", ids)
	}
}

func TestFilterIssuesSearchAssignee(t *testing.T) {
	issues := []Issue{
		{ID: "vv-001", Title: "Fix login bug", Assignee: "alice"},
		{ID: "vv-002", Title: "Add search feature", Assignee: "bob"},
	}

	result := FilterIssues(issues, "alice")
	if len(result) != 1 || result[0].ID != "vv-001" {
		ids := make([]string, len(result))
		for i, r := range result {
			ids[i] = r.ID
		}
		t.Fatalf("expected [vv-001], got %v", ids)
	}
}

func TestFilterIssuesSearchOwner(t *testing.T) {
	issues := []Issue{
		{ID: "vv-001", Title: "Fix login bug", Owner: "teamlead"},
		{ID: "vv-002", Title: "Add search feature", Owner: "pm"},
	}

	result := FilterIssues(issues, "teamlead")
	if len(result) != 1 || result[0].ID != "vv-001" {
		ids := make([]string, len(result))
		for i, r := range result {
			ids[i] = r.ID
		}
		t.Fatalf("expected [vv-001], got %v", ids)
	}
}

func TestFilterIssuesSearchNotes(t *testing.T) {
	issues := []Issue{
		{ID: "vv-001", Title: "Fix login bug", Notes: "Using token bucket algorithm"},
		{ID: "vv-002", Title: "Add search feature"},
	}

	result := FilterIssues(issues, "token bucket")
	if len(result) != 1 || result[0].ID != "vv-001" {
		ids := make([]string, len(result))
		for i, r := range result {
			ids[i] = r.ID
		}
		t.Fatalf("expected [vv-001], got %v", ids)
	}
}

func TestFilterIssuesSearchCombinedFields(t *testing.T) {
	// Query matches description but not title — should still find it
	issues := []Issue{
		{ID: "vv-001", Title: "Implement rate limiting", Description: "Redis-backed token bucket", Assignee: "alice", Labels: []string{"backend"}},
		{ID: "vv-002", Title: "Fix typo", Description: "README spelling"},
	}

	// "redis" only appears in description
	result := FilterIssues(issues, "redis")
	if len(result) != 1 || result[0].ID != "vv-001" {
		ids := make([]string, len(result))
		for i, r := range result {
			ids[i] = r.ID
		}
		t.Fatalf("expected [vv-001], got %v", ids)
	}
}

func TestIsStructuredToken(t *testing.T) {
	tests := []struct {
		token    string
		expected bool
	}{
		{"type:bug", true},
		{"priority:high", true},
		{"p0", true},
		{"p4", true},
		{"p5", false},
		{"login", false},
		{"type", false},
	}

	for _, tt := range tests {
		t.Run(tt.token, func(t *testing.T) {
			if got := isStructuredToken(tt.token); got != tt.expected {
				t.Errorf("isStructuredToken(%q) = %v, want %v", tt.token, got, tt.expected)
			}
		})
	}
}
