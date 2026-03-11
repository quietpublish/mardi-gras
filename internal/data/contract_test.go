package data

import (
	"encoding/json"
	"testing"
	"time"
)

// Contract tests validate that our JSON parsing matches the expected bd CLI
// output format. These catch breaking changes in the bd JSON contract before
// they surface as runtime bugs.

// --- Sample bd output constants ---

// bdListBasicIssue represents a fully-populated issue from `bd list --json`.
const bdListBasicIssue = `[{
	"id": "proj-042",
	"title": "Implement rate limiting",
	"description": "Add per-user rate limiting to the API gateway.",
	"status": "in_progress",
	"priority": 1,
	"issue_type": "feature",
	"owner": "alice@example.com",
	"assignee": "bob@example.com",
	"created_at": "2026-03-01T09:00:00-06:00",
	"created_by": "alice",
	"updated_at": "2026-03-05T14:30:00-06:00",
	"labels": ["backend", "security"],
	"due_at": "2026-03-15T17:00:00-06:00",
	"metadata": {
		"effort": 5,
		"component": "api-gateway",
		"reviewed": true
	}
}]`

// bdListClosedIssue represents a closed issue with close_reason and closed_at.
const bdListClosedIssue = `[{
	"id": "proj-010",
	"title": "Fix login redirect loop",
	"status": "closed",
	"priority": 0,
	"issue_type": "bug",
	"owner": "alice@example.com",
	"created_at": "2026-02-20T10:00:00-06:00",
	"created_by": "alice",
	"updated_at": "2026-02-21T16:00:00-06:00",
	"closed_at": "2026-02-21T16:00:00-06:00",
	"close_reason": "Root cause was stale session cookie; cleared on redirect."
}]`

// bdListWithDeps represents an issue with blocking dependencies.
const bdListWithDeps = `[{
	"id": "proj-050",
	"title": "Deploy canary release",
	"status": "open",
	"priority": 2,
	"issue_type": "task",
	"owner": "dev@example.com",
	"created_at": "2026-03-01T09:00:00-06:00",
	"created_by": "dev",
	"updated_at": "2026-03-01T09:00:00-06:00",
	"dependencies": [
		{
			"issue_id": "proj-050",
			"depends_on_id": "proj-042",
			"type": "blocks",
			"created_at": "2026-03-01T09:00:00-06:00",
			"created_by": "dev"
		},
		{
			"issue_id": "proj-050",
			"depends_on_id": "proj-010",
			"type": "related",
			"created_at": "2026-03-01T10:00:00-06:00",
			"created_by": "dev"
		}
	]
}]`

// bdListEpicWithChildren represents an epic and its hierarchical children.
const bdListEpicWithChildren = `[
	{
		"id": "proj-100",
		"title": "Platform migration",
		"description": "Migrate all services to the new platform.",
		"status": "open",
		"priority": 1,
		"issue_type": "epic",
		"owner": "lead@example.com",
		"created_at": "2026-03-01T08:00:00-06:00",
		"created_by": "lead",
		"updated_at": "2026-03-01T08:00:00-06:00"
	},
	{
		"id": "proj-100.1",
		"title": "Migrate auth service",
		"status": "in_progress",
		"priority": 2,
		"issue_type": "task",
		"owner": "dev@example.com",
		"created_at": "2026-03-02T09:00:00-06:00",
		"created_by": "lead",
		"updated_at": "2026-03-04T10:00:00-06:00"
	},
	{
		"id": "proj-100.2",
		"title": "Migrate billing service",
		"status": "open",
		"priority": 2,
		"issue_type": "task",
		"owner": "dev@example.com",
		"created_at": "2026-03-02T09:00:00-06:00",
		"created_by": "lead",
		"updated_at": "2026-03-02T09:00:00-06:00"
	},
	{
		"id": "proj-100.2.1",
		"title": "Update billing database schema",
		"status": "open",
		"priority": 3,
		"issue_type": "task",
		"owner": "dev@example.com",
		"created_at": "2026-03-02T10:00:00-06:00",
		"created_by": "lead",
		"updated_at": "2026-03-02T10:00:00-06:00"
	}
]`

// bdListMinimal represents the sparsest valid issue bd can return.
const bdListMinimal = `[{
	"id": "proj-001",
	"title": "Placeholder task",
	"status": "open",
	"priority": 4,
	"issue_type": "task",
	"created_at": "2026-03-01T00:00:00Z",
	"created_by": "system",
	"updated_at": "2026-03-01T00:00:00Z"
}]`

// bdListWithHOP represents an issue with HOP (Hierarchy of Proof) fields.
const bdListWithHOP = `[{
	"id": "proj-060",
	"title": "Validated feature",
	"status": "closed",
	"priority": 2,
	"issue_type": "feature",
	"created_at": "2026-03-01T09:00:00-06:00",
	"created_by": "polecat-nux",
	"updated_at": "2026-03-05T14:00:00-06:00",
	"closed_at": "2026-03-05T14:00:00-06:00",
	"creator": {
		"name": "polecat-nux",
		"platform": "gastown",
		"org": "myteam",
		"uri": "hop://gastown/myteam/polecat-nux"
	},
	"validations": [
		{
			"validator": {
				"name": "alice",
				"platform": "human"
			},
			"outcome": "accepted",
			"quality_score": 0.85,
			"comment": "Good implementation, minor style nits.",
			"timestamp": "2026-03-05T13:00:00-06:00"
		}
	],
	"quality_score": 0.85,
	"crystallizes": true
}]`

// bdShowDetailIssue represents enriched output from `bd show <id> --long --json`.
const bdShowDetailIssue = `[{
	"id": "proj-042",
	"title": "Implement rate limiting",
	"description": "Add per-user rate limiting to the API gateway.",
	"status": "in_progress",
	"priority": 1,
	"issue_type": "feature",
	"owner": "alice@example.com",
	"assignee": "bob@example.com",
	"created_at": "2026-03-01T09:00:00-06:00",
	"created_by": "alice",
	"updated_at": "2026-03-05T14:30:00-06:00",
	"notes": "Using token bucket algorithm. Redis-backed for distributed state.",
	"design": "## Approach\n\nToken bucket per user ID, 100 req/min default.",
	"acceptance_criteria": "- [ ] Rate limit headers in response\n- [ ] 429 status on exceeded\n- [ ] Configurable per-route limits",
	"labels": ["backend", "security"],
	"due_at": "2026-03-15T17:00:00-06:00",
	"defer_until": "2026-03-10T09:00:00-06:00"
}]`

// --- Contract tests ---

func TestContractBasicIssue(t *testing.T) {
	var issues []Issue
	if err := json.Unmarshal([]byte(bdListBasicIssue), &issues); err != nil {
		t.Fatalf("failed to parse basic issue: %v", err)
	}
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(issues))
	}

	iss := issues[0]
	assertEqual(t, "ID", iss.ID, "proj-042")
	assertEqual(t, "Title", iss.Title, "Implement rate limiting")
	assertEqual(t, "Description", iss.Description, "Add per-user rate limiting to the API gateway.")
	assertEqual(t, "Status", string(iss.Status), "in_progress")
	assertIntEqual(t, "Priority", int(iss.Priority), 1)
	assertEqual(t, "IssueType", string(iss.IssueType), "feature")
	assertEqual(t, "Owner", iss.Owner, "alice@example.com")
	assertEqual(t, "Assignee", iss.Assignee, "bob@example.com")
	assertEqual(t, "CreatedBy", iss.CreatedBy, "alice")

	if iss.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}
	if iss.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should not be zero")
	}

	// Labels
	if len(iss.Labels) != 2 {
		t.Fatalf("expected 2 labels, got %d", len(iss.Labels))
	}
	assertEqual(t, "Labels[0]", iss.Labels[0], "backend")
	assertEqual(t, "Labels[1]", iss.Labels[1], "security")

	// DueAt
	if iss.DueAt == nil {
		t.Fatal("DueAt should not be nil")
	}
	if iss.DueAt.Year() != 2026 || iss.DueAt.Month() != 3 || iss.DueAt.Day() != 15 {
		t.Errorf("DueAt = %v, expected 2026-03-15", iss.DueAt)
	}

	// Metadata
	if iss.Metadata == nil {
		t.Fatal("Metadata should not be nil")
	}
	if effort, ok := iss.Metadata["effort"].(float64); !ok || effort != 5 {
		t.Errorf("Metadata[effort] = %v, expected 5", iss.Metadata["effort"])
	}
	if comp, ok := iss.Metadata["component"].(string); !ok || comp != "api-gateway" {
		t.Errorf("Metadata[component] = %v, expected api-gateway", iss.Metadata["component"])
	}
	if rev, ok := iss.Metadata["reviewed"].(bool); !ok || !rev {
		t.Errorf("Metadata[reviewed] = %v, expected true", iss.Metadata["reviewed"])
	}

	// Optional fields should be zero/nil when not present
	if iss.ClosedAt != nil {
		t.Error("ClosedAt should be nil for open issue")
	}
	if iss.CloseReason != "" {
		t.Error("CloseReason should be empty for open issue")
	}
	if len(iss.Dependencies) != 0 {
		t.Error("Dependencies should be empty when not present")
	}
}

func TestContractClosedIssue(t *testing.T) {
	var issues []Issue
	if err := json.Unmarshal([]byte(bdListClosedIssue), &issues); err != nil {
		t.Fatalf("failed to parse closed issue: %v", err)
	}

	iss := issues[0]
	assertEqual(t, "Status", string(iss.Status), "closed")
	assertEqual(t, "CloseReason", iss.CloseReason, "Root cause was stale session cookie; cleared on redirect.")

	if iss.ClosedAt == nil {
		t.Fatal("ClosedAt should not be nil for closed issue")
	}
	if iss.ClosedAt.Year() != 2026 || iss.ClosedAt.Month() != 2 || iss.ClosedAt.Day() != 21 {
		t.Errorf("ClosedAt = %v, expected 2026-02-21", iss.ClosedAt)
	}
}

func TestContractDependencies(t *testing.T) {
	var issues []Issue
	if err := json.Unmarshal([]byte(bdListWithDeps), &issues); err != nil {
		t.Fatalf("failed to parse issue with deps: %v", err)
	}

	iss := issues[0]
	if len(iss.Dependencies) != 2 {
		t.Fatalf("expected 2 dependencies, got %d", len(iss.Dependencies))
	}

	// First dep: blocking
	dep0 := iss.Dependencies[0]
	assertEqual(t, "dep0.IssueID", dep0.IssueID, "proj-050")
	assertEqual(t, "dep0.DependsOnID", dep0.DependsOnID, "proj-042")
	assertEqual(t, "dep0.Type", dep0.Type, "blocks")
	assertEqual(t, "dep0.CreatedBy", dep0.CreatedBy, "dev")
	if dep0.CreatedAt == "" {
		t.Error("dep0.CreatedAt should not be empty")
	}

	// Second dep: non-blocking
	dep1 := iss.Dependencies[1]
	assertEqual(t, "dep1.Type", dep1.Type, "related")
	assertEqual(t, "dep1.DependsOnID", dep1.DependsOnID, "proj-010")
}

func TestContractEpicWithChildren(t *testing.T) {
	var issues []Issue
	if err := json.Unmarshal([]byte(bdListEpicWithChildren), &issues); err != nil {
		t.Fatalf("failed to parse epic with children: %v", err)
	}

	if len(issues) != 4 {
		t.Fatalf("expected 4 issues, got %d", len(issues))
	}

	// Verify parent-child hierarchy via dot notation
	epic := issues[0]
	assertEqual(t, "epic.ID", epic.ID, "proj-100")
	assertEqual(t, "epic.IssueType", string(epic.IssueType), "epic")
	if epic.ParentID() != "" {
		t.Error("epic should have no parent")
	}
	assertIntEqual(t, "epic.NestingDepth", epic.NestingDepth(), 0)

	child1 := issues[1]
	assertEqual(t, "child1.ID", child1.ID, "proj-100.1")
	assertEqual(t, "child1.ParentID", child1.ParentID(), "proj-100")
	assertIntEqual(t, "child1.NestingDepth", child1.NestingDepth(), 1)

	child2 := issues[2]
	assertEqual(t, "child2.ID", child2.ID, "proj-100.2")
	assertEqual(t, "child2.ParentID", child2.ParentID(), "proj-100")

	grandchild := issues[3]
	assertEqual(t, "grandchild.ID", grandchild.ID, "proj-100.2.1")
	assertEqual(t, "grandchild.ParentID", grandchild.ParentID(), "proj-100.2")
	assertIntEqual(t, "grandchild.NestingDepth", grandchild.NestingDepth(), 2)
}

func TestContractMinimalIssue(t *testing.T) {
	var issues []Issue
	if err := json.Unmarshal([]byte(bdListMinimal), &issues); err != nil {
		t.Fatalf("failed to parse minimal issue: %v", err)
	}

	iss := issues[0]
	assertEqual(t, "ID", iss.ID, "proj-001")
	assertEqual(t, "Title", iss.Title, "Placeholder task")
	assertEqual(t, "Status", string(iss.Status), "open")
	assertIntEqual(t, "Priority", int(iss.Priority), 4)
	assertEqual(t, "IssueType", string(iss.IssueType), "task")

	// All optional fields should be zero-value
	if iss.Description != "" {
		t.Errorf("Description should be empty, got %q", iss.Description)
	}
	if iss.Owner != "" {
		t.Errorf("Owner should be empty, got %q", iss.Owner)
	}
	if iss.Assignee != "" {
		t.Errorf("Assignee should be empty, got %q", iss.Assignee)
	}
	if iss.ClosedAt != nil {
		t.Error("ClosedAt should be nil")
	}
	if iss.CloseReason != "" {
		t.Error("CloseReason should be empty")
	}
	if len(iss.Dependencies) != 0 {
		t.Error("Dependencies should be empty")
	}
	if len(iss.Labels) != 0 {
		t.Error("Labels should be empty")
	}
	if iss.DueAt != nil {
		t.Error("DueAt should be nil")
	}
	if iss.DeferUntil != nil {
		t.Error("DeferUntil should be nil")
	}
	if iss.Metadata != nil {
		t.Error("Metadata should be nil")
	}
	if iss.Notes != "" {
		t.Error("Notes should be empty")
	}
	if iss.Design != "" {
		t.Error("Design should be empty")
	}
	if iss.AcceptanceCriteria != "" {
		t.Error("AcceptanceCriteria should be empty")
	}
	if iss.Creator != nil {
		t.Error("Creator should be nil")
	}
	if len(iss.Validations) != 0 {
		t.Error("Validations should be empty")
	}
	if iss.QualityScore != nil {
		t.Error("QualityScore should be nil")
	}
	if iss.Crystallizes != nil {
		t.Error("Crystallizes should be nil")
	}
}

func TestContractHOPFields(t *testing.T) {
	var issues []Issue
	if err := json.Unmarshal([]byte(bdListWithHOP), &issues); err != nil {
		t.Fatalf("failed to parse HOP issue: %v", err)
	}

	iss := issues[0]

	// Creator (EntityRef)
	if iss.Creator == nil {
		t.Fatal("Creator should not be nil")
	}
	assertEqual(t, "Creator.Name", iss.Creator.Name, "polecat-nux")
	assertEqual(t, "Creator.Platform", iss.Creator.Platform, "gastown")
	assertEqual(t, "Creator.Org", iss.Creator.Org, "myteam")
	assertEqual(t, "Creator.URI", iss.Creator.URI, "hop://gastown/myteam/polecat-nux")

	// Validations
	if len(iss.Validations) != 1 {
		t.Fatalf("expected 1 validation, got %d", len(iss.Validations))
	}
	v := iss.Validations[0]
	assertEqual(t, "Validator.Name", v.Validator.Name, "alice")
	assertEqual(t, "Validator.Platform", v.Validator.Platform, "human")
	assertEqual(t, "Outcome", string(v.Outcome), "accepted")
	if v.QualityScore < 0.84 || v.QualityScore > 0.86 {
		t.Errorf("QualityScore = %f, expected ~0.85", v.QualityScore)
	}
	assertEqual(t, "Comment", v.Comment, "Good implementation, minor style nits.")
	if v.Timestamp.IsZero() {
		t.Error("Validation.Timestamp should not be zero")
	}

	// Issue-level quality fields
	if iss.QualityScore == nil {
		t.Fatal("QualityScore should not be nil")
	}
	if *iss.QualityScore < 0.84 || *iss.QualityScore > 0.86 {
		t.Errorf("Issue.QualityScore = %f, expected ~0.85", *iss.QualityScore)
	}
	if iss.Crystallizes == nil {
		t.Fatal("Crystallizes should not be nil")
	}
	if !*iss.Crystallizes {
		t.Error("Crystallizes should be true")
	}
}

func TestContractDetailFields(t *testing.T) {
	var issues []Issue
	if err := json.Unmarshal([]byte(bdShowDetailIssue), &issues); err != nil {
		t.Fatalf("failed to parse detail issue: %v", err)
	}

	iss := issues[0]
	assertEqual(t, "Notes", iss.Notes, "Using token bucket algorithm. Redis-backed for distributed state.")
	if iss.Design == "" {
		t.Error("Design should not be empty")
	}
	if iss.AcceptanceCriteria == "" {
		t.Error("AcceptanceCriteria should not be empty")
	}
	if iss.DeferUntil == nil {
		t.Fatal("DeferUntil should not be nil")
	}
	if iss.DeferUntil.Year() != 2026 || iss.DeferUntil.Month() != 3 || iss.DeferUntil.Day() != 10 {
		t.Errorf("DeferUntil = %v, expected 2026-03-10", iss.DeferUntil)
	}
}

func TestContractForwardCompatibility(t *testing.T) {
	// bd may add new fields in future versions. Verify our parser doesn't break.
	futureJSON := `[{
		"id": "proj-999",
		"title": "Future issue",
		"status": "open",
		"priority": 2,
		"issue_type": "task",
		"created_at": "2026-03-01T00:00:00Z",
		"created_by": "system",
		"updated_at": "2026-03-01T00:00:00Z",
		"new_field_string": "hello",
		"new_field_number": 42,
		"new_field_object": {"nested": true},
		"new_field_array": [1, 2, 3]
	}]`

	var issues []Issue
	if err := json.Unmarshal([]byte(futureJSON), &issues); err != nil {
		t.Fatalf("forward compatibility broken — unknown fields cause parse error: %v", err)
	}

	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(issues))
	}
	assertEqual(t, "ID", issues[0].ID, "proj-999")
	assertEqual(t, "Title", issues[0].Title, "Future issue")
}

func TestContractForwardCompatibilityDependency(t *testing.T) {
	// Dependencies may gain new fields too.
	futureDepJSON := `[{
		"id": "proj-888",
		"title": "Issue with future dep fields",
		"status": "open",
		"priority": 2,
		"issue_type": "task",
		"created_at": "2026-03-01T00:00:00Z",
		"created_by": "system",
		"updated_at": "2026-03-01T00:00:00Z",
		"dependencies": [{
			"issue_id": "proj-888",
			"depends_on_id": "proj-001",
			"type": "blocks",
			"created_at": "2026-03-01T00:00:00Z",
			"created_by": "system",
			"strength": "hard",
			"reason": "API contract dependency"
		}]
	}]`

	var issues []Issue
	if err := json.Unmarshal([]byte(futureDepJSON), &issues); err != nil {
		t.Fatalf("forward compatibility broken on dependencies: %v", err)
	}

	if len(issues[0].Dependencies) != 1 {
		t.Fatalf("expected 1 dependency, got %d", len(issues[0].Dependencies))
	}
	assertEqual(t, "dep.Type", issues[0].Dependencies[0].Type, "blocks")
}

func TestContractTimestampFormats(t *testing.T) {
	// bd may emit timestamps in different valid RFC3339 formats.
	cases := []struct {
		name      string
		timestamp string
	}{
		{"UTC Z suffix", `"2026-03-01T09:00:00Z"`},
		{"negative offset", `"2026-03-01T09:00:00-06:00"`},
		{"positive offset", `"2026-03-01T09:00:00+05:30"`},
		{"zero offset", `"2026-03-01T09:00:00+00:00"`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			jsonStr := `[{"id":"ts-1","title":"test","status":"open","priority":0,` +
				`"issue_type":"task","created_at":` + tc.timestamp +
				`,"created_by":"x","updated_at":` + tc.timestamp + `}]`

			var issues []Issue
			if err := json.Unmarshal([]byte(jsonStr), &issues); err != nil {
				t.Fatalf("failed to parse timestamp %s: %v", tc.timestamp, err)
			}
			if issues[0].CreatedAt.IsZero() {
				t.Error("CreatedAt should not be zero")
			}
		})
	}
}

func TestContractDoctorDiagnostic(t *testing.T) {
	// Validate bd doctor --agent --json contract.
	doctorJSON := `{
		"overall_ok": false,
		"summary": "2 issues found",
		"diagnostics": [
			{
				"name": "dolt_server",
				"status": "error",
				"severity": "blocking",
				"category": "Core System",
				"explanation": "Dolt server is not running",
				"observed": "no process on port 3307",
				"expected": "dolt sql-server running on port 3307",
				"commands": ["dolt sql-server --port 3307"],
				"source_files": ["internal/db/server.go"]
			},
			{
				"name": "git_clean",
				"status": "warning",
				"severity": "degraded",
				"category": "Git Integration",
				"explanation": "Working tree has uncommitted changes",
				"observed": "3 modified files",
				"expected": "clean working tree"
			}
		]
	}`

	var result DoctorResult
	if err := json.Unmarshal([]byte(doctorJSON), &result); err != nil {
		t.Fatalf("failed to parse doctor result: %v", err)
	}

	if result.OK {
		t.Error("OK should be false")
	}
	assertEqual(t, "Summary", result.Summary, "2 issues found")

	if len(result.Diagnostics) != 2 {
		t.Fatalf("expected 2 diagnostics, got %d", len(result.Diagnostics))
	}

	d0 := result.Diagnostics[0]
	assertEqual(t, "d0.Name", d0.Name, "dolt_server")
	assertEqual(t, "d0.Status", d0.Status, "error")
	assertEqual(t, "d0.Severity", d0.Severity, "blocking")
	assertEqual(t, "d0.Category", d0.Category, "Core System")
	if len(d0.Commands) != 1 {
		t.Fatalf("expected 1 command, got %d", len(d0.Commands))
	}
	if len(d0.SourceFiles) != 1 {
		t.Fatalf("expected 1 source file, got %d", len(d0.SourceFiles))
	}

	// Second diagnostic has no commands/source_files — should be nil/empty
	d1 := result.Diagnostics[1]
	assertEqual(t, "d1.Name", d1.Name, "git_clean")
	if len(d1.Commands) != 0 {
		t.Errorf("expected no commands, got %d", len(d1.Commands))
	}
	if len(d1.SourceFiles) != 0 {
		t.Errorf("expected no source files, got %d", len(d1.SourceFiles))
	}
}

func TestContractEmptyList(t *testing.T) {
	// bd list --json returns [] when no issues exist.
	var issues []Issue
	if err := json.Unmarshal([]byte(`[]`), &issues); err != nil {
		t.Fatalf("failed to parse empty list: %v", err)
	}
	if len(issues) != 0 {
		t.Errorf("expected 0 issues, got %d", len(issues))
	}
}

func TestContractAllStatusValues(t *testing.T) {
	statuses := []string{"open", "in_progress", "closed"}
	for _, s := range statuses {
		t.Run(s, func(t *testing.T) {
			jsonStr := `[{"id":"s-1","title":"test","status":"` + s + `",` +
				`"priority":0,"issue_type":"task",` +
				`"created_at":"2026-03-01T00:00:00Z","created_by":"x",` +
				`"updated_at":"2026-03-01T00:00:00Z"}]`

			var issues []Issue
			if err := json.Unmarshal([]byte(jsonStr), &issues); err != nil {
				t.Fatalf("failed to parse status %q: %v", s, err)
			}
			assertEqual(t, "Status", string(issues[0].Status), s)
		})
	}
}

func TestContractAllIssueTypes(t *testing.T) {
	types := []string{"task", "bug", "feature", "chore", "epic"}
	for _, tp := range types {
		t.Run(tp, func(t *testing.T) {
			jsonStr := `[{"id":"t-1","title":"test","status":"open",` +
				`"priority":0,"issue_type":"` + tp + `",` +
				`"created_at":"2026-03-01T00:00:00Z","created_by":"x",` +
				`"updated_at":"2026-03-01T00:00:00Z"}]`

			var issues []Issue
			if err := json.Unmarshal([]byte(jsonStr), &issues); err != nil {
				t.Fatalf("failed to parse type %q: %v", tp, err)
			}
			assertEqual(t, "IssueType", string(issues[0].IssueType), tp)
		})
	}
}

func TestContractAllPriorityValues(t *testing.T) {
	for p := 0; p <= 4; p++ {
		t.Run(PriorityLabel(Priority(p)), func(t *testing.T) {
			jsonStr := `[{"id":"p-1","title":"test","status":"open",` +
				`"priority":` + string(rune('0'+p)) + `,"issue_type":"task",` +
				`"created_at":"2026-03-01T00:00:00Z","created_by":"x",` +
				`"updated_at":"2026-03-01T00:00:00Z"}]`

			var issues []Issue
			if err := json.Unmarshal([]byte(jsonStr), &issues); err != nil {
				t.Fatalf("failed to parse priority %d: %v", p, err)
			}
			assertIntEqual(t, "Priority", int(issues[0].Priority), p)
		})
	}
}

func TestContractSortAfterParse(t *testing.T) {
	// Verify SortIssues (called by FetchIssuesCLI) produces expected ordering:
	// active first, then by priority, then by recency.
	jsonStr := `[
		{"id":"c-1","title":"Closed","status":"closed","priority":0,"issue_type":"task",
		 "created_at":"2026-03-01T00:00:00Z","created_by":"x","updated_at":"2026-03-05T00:00:00Z",
		 "closed_at":"2026-03-05T00:00:00Z"},
		{"id":"a-2","title":"Low pri","status":"open","priority":3,"issue_type":"task",
		 "created_at":"2026-03-01T00:00:00Z","created_by":"x","updated_at":"2026-03-04T00:00:00Z"},
		{"id":"a-1","title":"High pri","status":"in_progress","priority":0,"issue_type":"bug",
		 "created_at":"2026-03-01T00:00:00Z","created_by":"x","updated_at":"2026-03-03T00:00:00Z"}
	]`

	var issues []Issue
	if err := json.Unmarshal([]byte(jsonStr), &issues); err != nil {
		t.Fatalf("failed to parse: %v", err)
	}
	SortIssues(issues)

	// Active issues first, then by priority ascending
	assertEqual(t, "sorted[0]", issues[0].ID, "a-1") // P0, active
	assertEqual(t, "sorted[1]", issues[1].ID, "a-2") // P3, active
	assertEqual(t, "sorted[2]", issues[2].ID, "c-1") // closed
}

func TestContractBdShowCurrentJSON(t *testing.T) {
	// bd show --current --json returns {"id": "..."}.
	showCurrentJSON := `{"id": "proj-042"}`

	var result struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal([]byte(showCurrentJSON), &result); err != nil {
		t.Fatalf("failed to parse bd show --current: %v", err)
	}
	assertEqual(t, "ID", result.ID, "proj-042")
}

func TestContractNullOptionalFields(t *testing.T) {
	// bd may explicitly send null for optional fields.
	jsonStr := `[{
		"id": "null-1",
		"title": "Nulls everywhere",
		"status": "open",
		"priority": 2,
		"issue_type": "task",
		"created_at": "2026-03-01T00:00:00Z",
		"created_by": "system",
		"updated_at": "2026-03-01T00:00:00Z",
		"description": null,
		"closed_at": null,
		"due_at": null,
		"defer_until": null,
		"labels": null,
		"dependencies": null,
		"metadata": null,
		"creator": null,
		"validations": null,
		"quality_score": null,
		"crystallizes": null
	}]`

	var issues []Issue
	if err := json.Unmarshal([]byte(jsonStr), &issues); err != nil {
		t.Fatalf("explicit nulls should not break parsing: %v", err)
	}

	iss := issues[0]
	if iss.Description != "" {
		t.Error("null description should be empty string")
	}
	if iss.ClosedAt != nil {
		t.Error("null closed_at should be nil")
	}
	if iss.DueAt != nil {
		t.Error("null due_at should be nil")
	}
	if iss.DeferUntil != nil {
		t.Error("null defer_until should be nil")
	}
	if iss.Labels != nil {
		t.Error("null labels should be nil")
	}
	if iss.Dependencies != nil {
		t.Error("null dependencies should be nil")
	}
	if iss.Metadata != nil {
		t.Error("null metadata should be nil")
	}
	if iss.Creator != nil {
		t.Error("null creator should be nil")
	}
	if iss.Validations != nil {
		t.Error("null validations should be nil")
	}
	if iss.QualityScore != nil {
		t.Error("null quality_score should be nil")
	}
	if iss.Crystallizes != nil {
		t.Error("null crystallizes should be nil")
	}
}

func TestContractMultipleDepTypes(t *testing.T) {
	// Verify all known dependency types parse correctly.
	depTypes := []string{"blocks", "conditional-blocks", "related", "discovered-from", "duplicates"}
	for _, dt := range depTypes {
		t.Run(dt, func(t *testing.T) {
			jsonStr := `[{"id":"dt-1","title":"test","status":"open","priority":0,"issue_type":"task",
				"created_at":"2026-03-01T00:00:00Z","created_by":"x","updated_at":"2026-03-01T00:00:00Z",
				"dependencies":[{"issue_id":"dt-1","depends_on_id":"dt-0","type":"` + dt + `",
				"created_at":"2026-03-01T00:00:00Z","created_by":"x"}]}]`

			var issues []Issue
			if err := json.Unmarshal([]byte(jsonStr), &issues); err != nil {
				t.Fatalf("failed to parse dep type %q: %v", dt, err)
			}
			assertEqual(t, "dep.Type", issues[0].Dependencies[0].Type, dt)
		})
	}
}

func TestContractRoundTripTime(t *testing.T) {
	// Verify time.Time serialization round-trips correctly through JSON.
	now := time.Now().Truncate(time.Second)
	iss := Issue{
		ID:        "rt-1",
		Title:     "roundtrip",
		Status:    StatusOpen,
		IssueType: TypeTask,
		CreatedAt: now,
		UpdatedAt: now,
		CreatedBy: "test",
	}

	data, err := json.Marshal([]Issue{iss})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var parsed []Issue
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if !parsed[0].CreatedAt.Equal(now) {
		t.Errorf("CreatedAt round-trip mismatch: %v vs %v", parsed[0].CreatedAt, now)
	}
}

// --- Helpers ---

func assertEqual(t *testing.T, field, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("%s = %q, want %q", field, got, want)
	}
}

func assertIntEqual(t *testing.T, field string, got, want int) {
	t.Helper()
	if got != want {
		t.Errorf("%s = %d, want %d", field, got, want)
	}
}
