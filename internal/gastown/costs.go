package gastown

import (
	"encoding/json"
	"fmt"
)

// CostsOutput represents the parsed output of `gt costs --json`.
type CostsOutput struct {
	Period   string     `json:"period"`
	Total    CostTotal  `json:"total"`
	Sessions int        `json:"sessions"`
	ByRole   []RoleCost `json:"by_role"`
	ByRig    []RigCost  `json:"by_rig"`
}

// CostTotal holds aggregate token/cost totals.
type CostTotal struct {
	InputTokens  int     `json:"input_tokens"`
	OutputTokens int     `json:"output_tokens"`
	Cost         float64 `json:"cost"`
}

// RoleCost is cost breakdown by agent role.
type RoleCost struct {
	Role     string  `json:"role"`
	Sessions int     `json:"sessions"`
	Cost     float64 `json:"cost"`
}

// RigCost is cost breakdown by rig.
type RigCost struct {
	Rig      string  `json:"rig"`
	Sessions int     `json:"sessions"`
	Cost     float64 `json:"cost"`
}

// FetchCosts runs `gt costs --json` and parses the output.
func FetchCosts() (*CostsOutput, error) {
	out, err := runWithTimeout(TimeoutMedium, "gt", "costs", "--json")
	if err != nil {
		return nil, fmt.Errorf("gt costs: %w", err)
	}
	var costs CostsOutput
	if err := json.Unmarshal(out, &costs); err != nil {
		return nil, fmt.Errorf("gt costs parse: %w", err)
	}
	return &costs, nil
}
