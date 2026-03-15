package gastown

// Embedded JSON constants for gt command output testing.
// Mirrors the pattern in internal/data/contract_test.go.

import (
	"encoding/json"
	"testing"
)

// gtConvoyListJSON is sample output from `gt convoy list --json`.
const gtConvoyListJSON = `[
	{
		"id": "conv-001",
		"title": "Auth migration",
		"status": "active",
		"owned": true,
		"merge_strategy": "squash",
		"tracked": [
			{"id": "mg-10", "title": "Migrate auth", "status": "in_progress", "worker": "polecat-nux"},
			{"id": "mg-11", "title": "Update tests", "status": "open", "blocked": true}
		],
		"completed": 1,
		"total": 3,
		"progress_pct": 33.3,
		"ready_count": 1,
		"active_count": 1,
		"assignees": ["polecat-nux"]
	}
]`

// gtConvoyStatusJSON is sample output from `gt convoy status <id> --json`.
const gtConvoyStatusJSON = `{
	"id": "conv-001",
	"title": "Auth migration",
	"status": "active",
	"owned": true,
	"tracked": [
		{"id": "mg-10", "title": "Migrate auth", "status": "in_progress", "worker": "polecat-nux"},
		{"id": "mg-11", "title": "Update tests", "status": "open", "blocked": true}
	],
	"completed": 1,
	"total": 3,
	"progress_pct": 33.3
}`

// gtMailInboxJSON is sample output from `gt mail inbox --json`.
const gtMailInboxJSON = `[
	{
		"id": "mail-001",
		"from": "mayor",
		"to": "polecat-nux",
		"subject": "New assignment",
		"body": "Please work on mg-10.",
		"timestamp": "2026-03-15T10:00:00Z",
		"read": false,
		"priority": "normal",
		"type": "assignment",
		"thread_id": "thread-abc"
	},
	{
		"id": "mail-002",
		"from": "witness",
		"subject": "Review complete",
		"timestamp": "2026-03-15T09:00:00Z",
		"read": true
	}
]`

// gtMailReadJSON is sample output from `gt mail read <id> --json`.
const gtMailReadJSON = `{
	"id": "mail-001",
	"from": "mayor",
	"to": "polecat-nux",
	"subject": "New assignment",
	"body": "Please work on mg-10.",
	"timestamp": "2026-03-15T10:00:00Z",
	"read": true,
	"priority": "normal",
	"type": "assignment",
	"thread_id": "thread-abc"
}`

// gtCostsJSON is sample output from `gt costs --json`.
const gtCostsJSON = `{
	"period": "2026-03-15",
	"total": {
		"input_tokens": 150000,
		"output_tokens": 50000,
		"cost": 12.50
	},
	"sessions": 8,
	"by_role": [
		{"role": "polecat", "sessions": 5, "cost": 8.00},
		{"role": "witness", "sessions": 3, "cost": 4.50}
	],
	"by_rig": [
		{"rig": "mardi_gras", "sessions": 8, "cost": 12.50}
	]
}`

// gtFormulaListOutput is sample output from `gt formula list` (plain text, not JSON).
const gtFormulaListOutput = "default\nshiny\nquick-fix\n"

// gtCommentsJSON is sample output from `bd comments <id> --json`.
const gtCommentsJSON = `[
	{
		"id": "cmt-001",
		"author": "alice",
		"body": "Starting work on this.",
		"created_at": "2026-03-15T10:00:00Z"
	},
	{
		"id": "cmt-002",
		"author": "polecat-nux",
		"body": "Implementation complete, ready for review.",
		"created_at": "2026-03-15T14:00:00Z"
	}
]`

func TestContractConvoyList(t *testing.T) {
	var convoys []ConvoyDetail
	if err := json.Unmarshal([]byte(gtConvoyListJSON), &convoys); err != nil {
		t.Fatalf("failed to parse convoy list: %v", err)
	}
	if len(convoys) != 1 {
		t.Fatalf("expected 1 convoy, got %d", len(convoys))
	}
	c := convoys[0]
	if c.ID != "conv-001" {
		t.Errorf("ID = %q, want conv-001", c.ID)
	}
	if c.Title != "Auth migration" {
		t.Errorf("Title = %q, want Auth migration", c.Title)
	}
	if !c.Owned {
		t.Error("Owned should be true")
	}
	if len(c.Tracked) != 2 {
		t.Fatalf("expected 2 tracked issues, got %d", len(c.Tracked))
	}
	if c.Tracked[0].Worker != "polecat-nux" {
		t.Errorf("Tracked[0].Worker = %q, want polecat-nux", c.Tracked[0].Worker)
	}
	if !c.Tracked[1].Blocked {
		t.Error("Tracked[1] should be blocked")
	}
}

func TestContractConvoyStatus(t *testing.T) {
	var detail ConvoyDetail
	if err := json.Unmarshal([]byte(gtConvoyStatusJSON), &detail); err != nil {
		t.Fatalf("failed to parse convoy status: %v", err)
	}
	if detail.ID != "conv-001" {
		t.Errorf("ID = %q, want conv-001", detail.ID)
	}
	if detail.Completed != 1 || detail.Total != 3 {
		t.Errorf("progress = %d/%d, want 1/3", detail.Completed, detail.Total)
	}
}

func TestContractMailInbox(t *testing.T) {
	var msgs []MailMessage
	if err := json.Unmarshal([]byte(gtMailInboxJSON), &msgs); err != nil {
		t.Fatalf("failed to parse mail inbox: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].From != "mayor" {
		t.Errorf("msgs[0].From = %q, want mayor", msgs[0].From)
	}
	if msgs[0].Read {
		t.Error("msgs[0] should be unread")
	}
	if !msgs[1].Read {
		t.Error("msgs[1] should be read")
	}
}

func TestContractMailRead(t *testing.T) {
	var msg MailMessage
	if err := json.Unmarshal([]byte(gtMailReadJSON), &msg); err != nil {
		t.Fatalf("failed to parse mail read: %v", err)
	}
	if msg.ID != "mail-001" {
		t.Errorf("ID = %q, want mail-001", msg.ID)
	}
	if msg.Body != "Please work on mg-10." {
		t.Errorf("Body = %q", msg.Body)
	}
}

func TestContractCosts(t *testing.T) {
	var costs CostsOutput
	if err := json.Unmarshal([]byte(gtCostsJSON), &costs); err != nil {
		t.Fatalf("failed to parse costs: %v", err)
	}
	if costs.Sessions != 8 {
		t.Errorf("Sessions = %d, want 8", costs.Sessions)
	}
	if costs.Total.Cost != 12.50 {
		t.Errorf("Total.Cost = %f, want 12.50", costs.Total.Cost)
	}
	if len(costs.ByRole) != 2 {
		t.Fatalf("expected 2 role costs, got %d", len(costs.ByRole))
	}
}

func TestContractComments(t *testing.T) {
	var comments []Comment
	if err := json.Unmarshal([]byte(gtCommentsJSON), &comments); err != nil {
		t.Fatalf("failed to parse comments: %v", err)
	}
	if len(comments) != 2 {
		t.Fatalf("expected 2 comments, got %d", len(comments))
	}
	if comments[0].Author != "alice" {
		t.Errorf("comments[0].Author = %q, want alice", comments[0].Author)
	}
}

func TestContractForwardCompatConvoy(t *testing.T) {
	futureJSON := `[{
		"id": "conv-999",
		"title": "Future convoy",
		"status": "active",
		"tracked": [],
		"completed": 0,
		"total": 0,
		"new_future_field": "hello",
		"analytics": {"velocity": 2.5}
	}]`
	var convoys []ConvoyDetail
	if err := json.Unmarshal([]byte(futureJSON), &convoys); err != nil {
		t.Fatalf("forward compat broken for convoy: %v", err)
	}
	if convoys[0].ID != "conv-999" {
		t.Errorf("ID = %q, want conv-999", convoys[0].ID)
	}
}

func TestContractForwardCompatMail(t *testing.T) {
	futureJSON := `[{
		"id": "mail-999",
		"from": "system",
		"subject": "Future mail",
		"read": false,
		"new_field": true,
		"attachments": [{"name": "file.txt"}]
	}]`
	var msgs []MailMessage
	if err := json.Unmarshal([]byte(futureJSON), &msgs); err != nil {
		t.Fatalf("forward compat broken for mail: %v", err)
	}
	if msgs[0].ID != "mail-999" {
		t.Errorf("ID = %q, want mail-999", msgs[0].ID)
	}
}
