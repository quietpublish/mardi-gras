package gastown

import (
	"testing"
	"time"

	"github.com/matt-wright86/mardi-gras/internal/data"
)

func TestVelocityMetrics(t *testing.T) {
	now := time.Date(2026, 2, 23, 14, 0, 0, 0, time.UTC)
	today := time.Date(2026, 2, 23, 10, 0, 0, 0, time.UTC)
	yesterday := time.Date(2026, 2, 22, 10, 0, 0, 0, time.UTC)
	lastWeek := time.Date(2026, 2, 15, 10, 0, 0, 0, time.UTC)

	closedToday := today
	closedYesterday := yesterday

	issues := []data.Issue{
		{ID: "a", Status: data.StatusOpen, CreatedAt: today},
		{ID: "b", Status: data.StatusInProgress, CreatedAt: yesterday},
		{ID: "c", Status: data.StatusClosed, CreatedAt: lastWeek, ClosedAt: &closedToday},
		{ID: "d", Status: data.StatusClosed, CreatedAt: lastWeek, ClosedAt: &closedYesterday},
		{ID: "e", Status: data.StatusOpen, CreatedAt: lastWeek},
	}

	status := &TownStatus{
		Agents: []AgentRuntime{
			{Name: "a1", Running: true, HasWork: true},
			{Name: "a2", Running: true, HasWork: true},
			{Name: "a3", Running: true, HasWork: false},
			{Name: "a4", Running: false},
			{Name: "a5", Running: false},
		},
	}

	costs := &CostsOutput{
		Total:    CostTotal{Cost: 7.84},
		Sessions: 6,
	}

	v := ComputeVelocityAt(issues, status, costs, now)

	// Issue counts
	if v.OpenCount != 3 {
		t.Fatalf("OpenCount = %d, want 3", v.OpenCount)
	}
	if v.CreatedToday != 1 {
		t.Fatalf("CreatedToday = %d, want 1", v.CreatedToday)
	}
	// 2026-02-23 is Monday. Go Weekday()=1, so week start = Feb 22 (Sunday).
	// yesterday (Feb 22) is on the week start boundary, so it counts.
	if v.CreatedWeek != 2 {
		t.Fatalf("CreatedWeek = %d, want 2", v.CreatedWeek)
	}
	if v.ClosedToday != 1 {
		t.Fatalf("ClosedToday = %d, want 1", v.ClosedToday)
	}
	// closedYesterday (Feb 22) is on the week start boundary, so both count.
	if v.ClosedWeek != 2 {
		t.Fatalf("ClosedWeek = %d, want 2", v.ClosedWeek)
	}

	// Agent counts
	if v.TotalAgents != 5 {
		t.Fatalf("TotalAgents = %d, want 5", v.TotalAgents)
	}
	if v.WorkingAgents != 2 {
		t.Fatalf("WorkingAgents = %d, want 2", v.WorkingAgents)
	}

	// Cost
	if v.TodayCost != 7.84 {
		t.Fatalf("TodayCost = %f, want 7.84", v.TodayCost)
	}
	if v.TodaySessions != 6 {
		t.Fatalf("TodaySessions = %d, want 6", v.TodaySessions)
	}
}

func TestVelocityNilInputs(t *testing.T) {
	v := ComputeVelocity(nil, nil, nil)
	if v.OpenCount != 0 {
		t.Fatalf("expected 0 open count, got %d", v.OpenCount)
	}
	if v.TotalAgents != 0 {
		t.Fatalf("expected 0 agents, got %d", v.TotalAgents)
	}
	if v.TodayCost != 0 {
		t.Fatalf("expected 0 cost, got %f", v.TodayCost)
	}
}

func TestVelocityEmptyIssues(t *testing.T) {
	v := ComputeVelocity([]data.Issue{}, nil, nil)
	if v.OpenCount != 0 || v.ClosedToday != 0 || v.CreatedToday != 0 {
		t.Fatalf("expected all zeros, got %+v", v)
	}
}
