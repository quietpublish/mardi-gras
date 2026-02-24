package gastown

import (
	"encoding/json"
	"fmt"
)

// TownStatus is the normalized view of `gt status --json`.
// The Agents slice is flattened from both top-level and per-rig agents.
type TownStatus struct {
	Agents  []AgentRuntime `json:"agents"`
	Rigs    []RigStatus    `json:"rigs"`
	Convoys []ConvoyInfo   `json:"convoys"`
}

// AgentRuntime represents a single Gas Town agent.
type AgentRuntime struct {
	Name    string `json:"name"`
	Role    string `json:"role"`
	Rig     string `json:"rig"`
	HasWork bool   `json:"has_work"`
	WorkTitle string `json:"work_title"`
	HookBead  string `json:"hook_bead"`
	State     string `json:"state"`
	Mail      int    `json:"unread_mail"`
	Address   string `json:"address"`
	Session   string `json:"session"`
}

// RigStatus represents a Gas Town rig (project).
type RigStatus struct {
	Name         string     `json:"name"`
	PolecatCount int        `json:"polecat_count"`
	CrewCount    int        `json:"crew_count"`
	HasWitness   bool       `json:"has_witness"`
	HasRefinery  bool       `json:"has_refinery"`
	MQ           *MQSummary `json:"mq,omitempty"`
}

// MQSummary represents the merge queue status for a rig.
type MQSummary struct {
	Pending  int    `json:"pending"`   // Open MRs ready to merge
	InFlight int    `json:"in_flight"` // MRs currently being processed
	Blocked  int    `json:"blocked"`   // MRs waiting on dependencies
	State    string `json:"state"`     // idle, processing, blocked
	Health   string `json:"health"`    // healthy, stale, empty
}

// ConvoyInfo represents a Gas Town convoy (delivery batch).
type ConvoyInfo struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"`
	Done   int    `json:"done"`
	Total  int    `json:"total"`
}

// rawTownStatus matches the actual `gt status --json` output shape.
type rawTownStatus struct {
	Name    string          `json:"name"`
	Agents  []AgentRuntime  `json:"agents"`
	Rigs    []rawRigStatus  `json:"rigs"`
	Convoys []ConvoyInfo    `json:"convoys"`
}

type rawRigStatus struct {
	Name         string         `json:"name"`
	PolecatCount int            `json:"polecat_count"`
	CrewCount    int            `json:"crew_count"`
	HasWitness   bool           `json:"has_witness"`
	HasRefinery  bool           `json:"has_refinery"`
	Agents       []AgentRuntime `json:"agents"`
	Hooks        []rawHook      `json:"hooks"`
	MQ           *MQSummary     `json:"mq,omitempty"`
}

type rawHook struct {
	Agent    string `json:"agent"`
	Role     string `json:"role"`
	HasWork  bool   `json:"has_work"`
	Molecule string `json:"molecule,omitempty"`
	Title    string `json:"title,omitempty"`
}

// FetchStatus runs `gt status --json` and parses the output.
// Returns nil TownStatus (not error) if gt is not available.
func FetchStatus() (*TownStatus, error) {
	out, err := runWithTimeout(TimeoutLong, "gt", "status", "--json")
	if err != nil {
		return nil, fmt.Errorf("gt status: %w", err)
	}
	var raw rawTownStatus
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("gt status parse: %w", err)
	}
	return normalizeStatus(&raw), nil
}

// normalizeStatus flattens the raw gt output into a unified TownStatus.
func normalizeStatus(raw *rawTownStatus) *TownStatus {
	status := &TownStatus{
		Agents:  raw.Agents,
		Convoys: raw.Convoys,
	}

	for _, rig := range raw.Rigs {
		status.Rigs = append(status.Rigs, RigStatus{
			Name:         rig.Name,
			PolecatCount: rig.PolecatCount,
			CrewCount:    rig.CrewCount,
			HasWitness:   rig.HasWitness,
			HasRefinery:  rig.HasRefinery,
			MQ:           rig.MQ,
		})

		// Build hook lookup: agent address -> hook info
		hookMap := make(map[string]rawHook, len(rig.Hooks))
		for _, h := range rig.Hooks {
			hookMap[h.Agent] = h
		}

		// Merge rig agents, enriching with hook data and rig name
		for _, a := range rig.Agents {
			a.Rig = rig.Name
			if a.State == "" {
				a.State = "idle"
			}
			// Enrich from hook data
			if h, ok := hookMap[a.Address]; ok {
				a.HasWork = h.HasWork
				if h.Molecule != "" {
					a.HookBead = h.Molecule
				}
				if h.Title != "" {
					a.WorkTitle = h.Title
				}
			}
			status.Agents = append(status.Agents, a)
		}
	}

	return status
}

// AgentForIssue returns the agent working on a given issue, if any.
func (s *TownStatus) AgentForIssue(issueID string) *AgentRuntime {
	if s == nil {
		return nil
	}
	for i := range s.Agents {
		if s.Agents[i].HookBead == issueID {
			return &s.Agents[i]
		}
	}
	return nil
}

// ActiveAgentMap returns issueID -> agent name for all agents with hooked work.
// This bridges Gas Town status to the existing ActiveAgents map[string]string.
func (s *TownStatus) ActiveAgentMap() map[string]string {
	m := make(map[string]string)
	if s == nil {
		return m
	}
	for _, a := range s.Agents {
		if a.HookBead != "" && a.State != "idle" && a.State != "" {
			m[a.HookBead] = a.Name
		}
	}
	return m
}

// WorkingCount returns the number of agents currently doing work.
func (s *TownStatus) WorkingCount() int {
	n := 0
	if s == nil {
		return n
	}
	for _, a := range s.Agents {
		if a.State == "working" || a.State == "spawning" {
			n++
		}
	}
	return n
}

// MQStatus returns the first non-nil MQ summary from any rig.
func (s *TownStatus) MQStatus() *MQSummary {
	if s == nil {
		return nil
	}
	for _, r := range s.Rigs {
		if r.MQ != nil {
			return r.MQ
		}
	}
	return nil
}

// UnreadMail returns total unread mail across all agents.
func (s *TownStatus) UnreadMail() int {
	n := 0
	if s == nil {
		return n
	}
	for _, a := range s.Agents {
		n += a.Mail
	}
	return n
}
