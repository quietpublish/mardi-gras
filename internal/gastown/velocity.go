package gastown

import (
	"time"

	"github.com/matt-wright86/mardi-gras/internal/data"
)

// VelocityMetrics aggregates workflow throughput from issues, agents, and costs.
type VelocityMetrics struct {
	// Issue flow
	OpenCount    int
	ClosedToday  int
	ClosedWeek   int
	CreatedToday int
	CreatedWeek  int

	// Agent utilization (from current status snapshot)
	TotalAgents   int
	WorkingAgents int

	// Cost context (from CostsOutput, if available)
	TodayCost     float64
	TodaySessions int
}

// ComputeVelocity derives velocity metrics from existing data sources.
// All parameters are optional (nil-safe).
func ComputeVelocity(issues []data.Issue, status *TownStatus, costs *CostsOutput) *VelocityMetrics {
	return ComputeVelocityAt(issues, status, costs, time.Now())
}

// ComputeVelocityAt is like ComputeVelocity but accepts a reference time for testing.
func ComputeVelocityAt(issues []data.Issue, status *TownStatus, costs *CostsOutput, now time.Time) *VelocityMetrics {
	v := &VelocityMetrics{}

	todayStart := startOfDay(now)
	weekStart := todayStart.AddDate(0, 0, -int(now.Weekday()))

	for _, iss := range issues {
		switch iss.Status {
		case data.StatusOpen, data.StatusInProgress:
			v.OpenCount++
		}

		if iss.CreatedAt.After(todayStart) || iss.CreatedAt.Equal(todayStart) {
			v.CreatedToday++
		}
		if iss.CreatedAt.After(weekStart) || iss.CreatedAt.Equal(weekStart) {
			v.CreatedWeek++
		}

		if iss.Status == data.StatusClosed && iss.ClosedAt != nil {
			if iss.ClosedAt.After(todayStart) || iss.ClosedAt.Equal(todayStart) {
				v.ClosedToday++
			}
			if iss.ClosedAt.After(weekStart) || iss.ClosedAt.Equal(weekStart) {
				v.ClosedWeek++
			}
		}
	}

	if status != nil {
		v.TotalAgents = len(status.Agents)
		v.WorkingAgents = status.WorkingCount()
	}

	if costs != nil {
		v.TodayCost = costs.Total.Cost
		v.TodaySessions = costs.Sessions
	}

	return v
}

// startOfDay returns midnight (00:00:00) of the given time's date in local timezone.
func startOfDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}
