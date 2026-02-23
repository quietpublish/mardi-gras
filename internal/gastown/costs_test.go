package gastown

import (
	"encoding/json"
	"testing"
)

func TestCostsOutputParsing(t *testing.T) {
	raw := `{
		"period": "today",
		"total": {"input_tokens": 150000, "output_tokens": 50000, "cost": 47.23},
		"sessions": 20,
		"by_role": [
			{"role": "polecat", "sessions": 12, "cost": 12.30},
			{"role": "witness", "sessions": 3, "cost": 3.20},
			{"role": "refinery", "sessions": 5, "cost": 5.60}
		],
		"by_rig": [
			{"rig": "beads", "sessions": 15, "cost": 35.00},
			{"rig": "other", "sessions": 5, "cost": 12.23}
		]
	}`

	var costs CostsOutput
	if err := json.Unmarshal([]byte(raw), &costs); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if costs.Period != "today" {
		t.Errorf("expected period 'today', got %q", costs.Period)
	}
	if costs.Total.Cost != 47.23 {
		t.Errorf("expected total cost 47.23, got %f", costs.Total.Cost)
	}
	if costs.Total.InputTokens != 150000 {
		t.Errorf("expected 150000 input tokens, got %d", costs.Total.InputTokens)
	}
	if costs.Sessions != 20 {
		t.Errorf("expected 20 sessions, got %d", costs.Sessions)
	}
	if len(costs.ByRole) != 3 {
		t.Fatalf("expected 3 role costs, got %d", len(costs.ByRole))
	}
	if costs.ByRole[0].Role != "polecat" {
		t.Errorf("expected first role 'polecat', got %q", costs.ByRole[0].Role)
	}
	if costs.ByRole[0].Sessions != 12 {
		t.Errorf("expected polecat sessions 12, got %d", costs.ByRole[0].Sessions)
	}
	if len(costs.ByRig) != 2 {
		t.Fatalf("expected 2 rig costs, got %d", len(costs.ByRig))
	}
	if costs.ByRig[0].Rig != "beads" {
		t.Errorf("expected first rig 'beads', got %q", costs.ByRig[0].Rig)
	}
}

func TestCostsOutputEmpty(t *testing.T) {
	raw := `{
		"period": "today",
		"total": {"input_tokens": 0, "output_tokens": 0, "cost": 0},
		"sessions": 0,
		"by_role": [],
		"by_rig": []
	}`

	var costs CostsOutput
	if err := json.Unmarshal([]byte(raw), &costs); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if costs.Sessions != 0 {
		t.Errorf("expected 0 sessions, got %d", costs.Sessions)
	}
	if len(costs.ByRole) != 0 {
		t.Errorf("expected 0 role costs, got %d", len(costs.ByRole))
	}
}
