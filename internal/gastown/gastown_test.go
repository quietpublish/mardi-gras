package gastown

import (
	"encoding/json"
	"os"
	"testing"
)

func TestDetectNoEnvVars(t *testing.T) {
	// Clear GT env vars for this test
	os.Unsetenv("GT_ROLE")
	os.Unsetenv("GT_RIG")
	os.Unsetenv("GT_SCOPE")
	os.Unsetenv("GT_POLECAT")
	os.Unsetenv("GT_CREW")
	env := Detect()
	if env.Active {
		t.Error("expected Active=false with no GT_ env vars")
	}
	if env.Role != "" {
		t.Errorf("expected empty Role, got %q", env.Role)
	}
}

func TestDetectWithEnvVars(t *testing.T) {
	t.Setenv("GT_ROLE", "polecat")
	t.Setenv("GT_RIG", "beads")
	t.Setenv("GT_POLECAT", "Toast")
	env := Detect()
	if !env.Active {
		t.Error("expected Active=true")
	}
	if env.Role != "polecat" {
		t.Errorf("expected Role=polecat, got %q", env.Role)
	}
	if env.Worker != "Toast" {
		t.Errorf("expected Worker=Toast, got %q", env.Worker)
	}
}

func TestDetectCrewWorker(t *testing.T) {
	t.Setenv("GT_ROLE", "crew")
	t.Setenv("GT_RIG", "beads")
	t.Setenv("GT_CREW", "Muffin")
	env := Detect()
	if env.Worker != "Muffin" {
		t.Errorf("expected Worker=Muffin, got %q", env.Worker)
	}
}

func TestTownStatusAgentForIssue(t *testing.T) {
	status := &TownStatus{
		Agents: []AgentRuntime{
			{Name: "Toast", Role: "polecat", HookBead: "bd-a1b2", Running: true, HasWork: true},
			{Name: "Muffin", Role: "polecat", HookBead: "bd-c3d4", Running: true, HasWork: true},
			{Name: "Whiskers", Role: "polecat", HookBead: "", Running: true, HasWork: false},
		},
	}
	agent := status.AgentForIssue("bd-a1b2")
	if agent == nil || agent.Name != "Toast" {
		t.Errorf("expected Toast for bd-a1b2, got %v", agent)
	}
	if status.AgentForIssue("bd-nope") != nil {
		t.Error("expected nil for unknown issue")
	}
}

func TestTownStatusAgentForIssueNil(t *testing.T) {
	var status *TownStatus
	if status.AgentForIssue("bd-a1b2") != nil {
		t.Error("expected nil from nil TownStatus")
	}
}

func TestTownStatusActiveAgentMap(t *testing.T) {
	status := &TownStatus{
		Agents: []AgentRuntime{
			{Name: "Toast", HookBead: "bd-a1b2", Running: true},
			{Name: "Muffin", HookBead: "", Running: true},        // no hook
			{Name: "Stale", HookBead: "bd-e5f6", Running: false}, // not running
		},
	}
	m := status.ActiveAgentMap()
	if len(m) != 1 {
		t.Errorf("expected 1 active agent, got %d", len(m))
	}
	if m["bd-a1b2"] != "Toast" {
		t.Errorf("expected Toast for bd-a1b2, got %q", m["bd-a1b2"])
	}
}

func TestTownStatusActiveAgentMapNil(t *testing.T) {
	var status *TownStatus
	m := status.ActiveAgentMap()
	if len(m) != 0 {
		t.Errorf("expected empty map from nil TownStatus, got %d", len(m))
	}
}

func TestTownStatusWorkingCount(t *testing.T) {
	status := &TownStatus{
		Agents: []AgentRuntime{
			{Name: "Toast", HasWork: true, Running: true},
			{Name: "Muffin", HasWork: false, Running: true},
			{Name: "Stale", HasWork: true, Running: false},
		},
	}
	if got := status.WorkingCount(); got != 1 {
		t.Errorf("expected 1 working, got %d", got)
	}

	var nilStatus *TownStatus
	if got := nilStatus.WorkingCount(); got != 0 {
		t.Errorf("expected 0 from nil, got %d", got)
	}
}

func TestTownStatusUnreadMail(t *testing.T) {
	status := &TownStatus{
		Agents: []AgentRuntime{
			{Name: "Toast", Mail: 3},
			{Name: "Muffin", Mail: 0},
			{Name: "Whiskers", Mail: 2},
		},
	}
	if got := status.UnreadMail(); got != 5 {
		t.Errorf("expected 5 unread, got %d", got)
	}

	var nilStatus *TownStatus
	if got := nilStatus.UnreadMail(); got != 0 {
		t.Errorf("expected 0 from nil, got %d", got)
	}
}

func TestTownStatusParsing(t *testing.T) {
	// Verify parsing against real gt status --json shape
	raw := `{
		"name": "test-hq",
		"location": "/tmp/gt",
		"agents": [
			{"name":"mayor","address":"mayor/","session":"hq-mayor",
			 "role":"coordinator","running":true,"has_work":false,"unread_mail":0}
		],
		"rigs": [{
			"name":"beads",
			"polecat_count":2,
			"crew_count":1,
			"has_witness":true,
			"has_refinery":true,
			"hooks": [
				{"agent":"beads/toast","role":"polecat","has_work":true,"title":"Fix login"}
			],
			"agents": [
				{"name":"toast","address":"beads/toast","session":"mg-toast",
				 "role":"polecat","running":true,"has_work":true,
				 "work_title":"Fix login","hook_bead":"bd-a1b2",
				 "state":"working","unread_mail":0},
				{"name":"muffin","address":"beads/muffin","session":"mg-muffin",
				 "role":"polecat","running":false,"has_work":false,"unread_mail":2}
			]
		}],
		"summary": {"rig_count":1,"polecat_count":2,"crew_count":1,
			"witness_count":1,"refinery_count":1,"active_hooks":1}
	}`
	var rawStatus rawTownStatus
	if err := json.Unmarshal([]byte(raw), &rawStatus); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	status := normalizeStatus(&rawStatus)

	// Top-level agents (mayor) + rig agents (toast, muffin) = 3
	if len(status.Agents) != 3 {
		t.Errorf("expected 3 agents, got %d", len(status.Agents))
	}

	// Find toast in flattened agents
	var toast *AgentRuntime
	for i := range status.Agents {
		if status.Agents[i].Name == "toast" {
			toast = &status.Agents[i]
			break
		}
	}
	if toast == nil {
		t.Fatal("toast not found in flattened agents")
	}
	if toast.Rig != "beads" {
		t.Errorf("expected toast.Rig=beads, got %q", toast.Rig)
	}
	if toast.State != "working" {
		t.Errorf("expected toast.State=working, got %q", toast.State)
	}
	if toast.HookBead != "bd-a1b2" {
		t.Errorf("expected toast.HookBead=bd-a1b2, got %q", toast.HookBead)
	}

	// Muffin should get idle state inferred
	var muffin *AgentRuntime
	for i := range status.Agents {
		if status.Agents[i].Name == "muffin" {
			muffin = &status.Agents[i]
			break
		}
	}
	if muffin == nil {
		t.Fatal("muffin not found in flattened agents")
	}
	if muffin.State != "idle" {
		t.Errorf("expected muffin.State=idle, got %q", muffin.State)
	}

	if len(status.Rigs) != 1 || status.Rigs[0].Name != "beads" {
		t.Errorf("unexpected rigs: %+v", status.Rigs)
	}
	if status.WorkingCount() != 1 {
		t.Errorf("expected 1 working, got %d", status.WorkingCount())
	}
}

func TestTownStatusMQParsing(t *testing.T) {
	raw := `{
		"name": "test-hq",
		"agents": [],
		"rigs": [{
			"name":"beads",
			"polecat_count":2,
			"crew_count":0,
			"has_witness":false,
			"has_refinery":true,
			"hooks": [],
			"agents": [],
			"mq": {
				"pending": 3,
				"in_flight": 1,
				"blocked": 0,
				"state": "processing",
				"health": "healthy"
			}
		}]
	}`
	var rawStatus rawTownStatus
	if err := json.Unmarshal([]byte(raw), &rawStatus); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	status := normalizeStatus(&rawStatus)

	if len(status.Rigs) != 1 {
		t.Fatalf("expected 1 rig, got %d", len(status.Rigs))
	}
	rig := status.Rigs[0]
	if rig.MQ == nil {
		t.Fatal("expected MQ to be parsed")
	}
	if rig.MQ.Pending != 3 {
		t.Errorf("expected MQ.Pending=3, got %d", rig.MQ.Pending)
	}
	if rig.MQ.InFlight != 1 {
		t.Errorf("expected MQ.InFlight=1, got %d", rig.MQ.InFlight)
	}
	if rig.MQ.State != "processing" {
		t.Errorf("expected MQ.State=processing, got %q", rig.MQ.State)
	}
	if rig.MQ.Health != "healthy" {
		t.Errorf("expected MQ.Health=healthy, got %q", rig.MQ.Health)
	}
}

func TestTownStatusMQStatusHelper(t *testing.T) {
	// No MQ
	status := &TownStatus{
		Rigs: []RigStatus{{Name: "empty"}},
	}
	if status.MQStatus() != nil {
		t.Error("expected nil MQ from rig without MQ")
	}

	// With MQ
	status = &TownStatus{
		Rigs: []RigStatus{
			{Name: "no-mq"},
			{Name: "with-mq", MQ: &MQSummary{Pending: 5, State: "idle", Health: "healthy"}},
		},
	}
	mq := status.MQStatus()
	if mq == nil {
		t.Fatal("expected non-nil MQ")
	}
	if mq.Pending != 5 {
		t.Errorf("expected Pending=5, got %d", mq.Pending)
	}

	// Nil status
	var nilStatus *TownStatus
	if nilStatus.MQStatus() != nil {
		t.Error("expected nil from nil TownStatus")
	}
}

func TestTownStatusMQNoRefinery(t *testing.T) {
	raw := `{
		"name": "test-hq",
		"agents": [],
		"rigs": [{
			"name":"beads",
			"polecat_count":1,
			"crew_count":0,
			"has_witness":false,
			"has_refinery":false,
			"hooks": [],
			"agents": []
		}]
	}`
	var rawStatus rawTownStatus
	if err := json.Unmarshal([]byte(raw), &rawStatus); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	status := normalizeStatus(&rawStatus)

	if status.Rigs[0].MQ != nil {
		t.Error("expected nil MQ when no refinery")
	}
	if status.MQStatus() != nil {
		t.Error("expected nil from MQStatus when no MQ data")
	}
}
