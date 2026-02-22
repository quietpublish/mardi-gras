package data

import (
	"fmt"
	"time"
)

// Status represents the state of a Beads issue.
type Status string

const (
	StatusOpen       Status = "open"
	StatusInProgress Status = "in_progress"
	StatusClosed     Status = "closed"
)

// IssueType represents the category of a Beads issue.
type IssueType string

const (
	TypeTask    IssueType = "task"
	TypeBug     IssueType = "bug"
	TypeFeature IssueType = "feature"
	TypeChore   IssueType = "chore"
	TypeEpic    IssueType = "epic"
)

// Priority ranges from 0 (critical) to 4 (backlog).
type Priority int

const (
	PriorityCritical Priority = 0
	PriorityHigh     Priority = 1
	PriorityMedium   Priority = 2
	PriorityLow      Priority = 3
	PriorityBacklog  Priority = 4
)

// ParadeStatus maps issues to their parade float group.
type ParadeStatus int

const (
	ParadeRolling    ParadeStatus = iota // in_progress
	ParadeLinedUp                        // open, not blocked
	ParadeStalled                        // open, blocked
	ParadePastTheStand                   // closed
)

// Dependency represents a relationship between two issues.
type Dependency struct {
	IssueID     string `json:"issue_id"`
	DependsOnID string `json:"depends_on_id"`
	Type        string `json:"type"`
	CreatedAt   string `json:"created_at"`
	CreatedBy   string `json:"created_by"`
}

// DefaultBlockingTypes is the set of dependency types that count as blockers.
var DefaultBlockingTypes = map[string]bool{"blocks": true}

// DepStatus classifies a single dependency edge after evaluation.
type DepStatus int

const (
	DepBlocking    DepStatus = iota // unresolved blocker exists
	DepResolved                     // blocker exists but is closed
	DepMissing                      // depends_on_id not found in map
	DepNonBlocking                  // dep type not in blockingTypes set
)

// DepEdge is a single evaluated dependency relationship.
type DepEdge struct {
	Type        string
	DependsOnID string
	Status      DepStatus
}

// DepEval is the result of evaluating all dependencies for an issue.
type DepEval struct {
	Edges         []DepEdge
	BlockingIDs   []string  // exist + not closed
	ResolvedIDs   []string  // exist + closed
	MissingIDs    []string  // not found in issueMap
	NonBlocking   []DepEdge // dep type not in blockingTypes
	IsBlocked     bool
	NextBlockerID string // first of BlockingIDs, else first of MissingIDs
}

// Issue represents a single Beads issue.
type Issue struct {
	ID                 string       `json:"id"`
	Title              string       `json:"title"`
	Description        string       `json:"description,omitempty"`
	Status             Status       `json:"status"`
	Priority           Priority     `json:"priority"`
	IssueType          IssueType    `json:"issue_type"`
	Owner              string       `json:"owner,omitempty"`
	Assignee           string       `json:"assignee,omitempty"`
	CreatedAt          time.Time    `json:"created_at"`
	CreatedBy          string       `json:"created_by,omitempty"`
	UpdatedAt          time.Time    `json:"updated_at"`
	ClosedAt           *time.Time   `json:"closed_at,omitempty"`
	CloseReason        string       `json:"close_reason,omitempty"`
	Dependencies       []Dependency `json:"dependencies,omitempty"`
	Notes              string       `json:"notes,omitempty"`
	Design             string       `json:"design,omitempty"`
	AcceptanceCriteria string       `json:"acceptance_criteria,omitempty"`
}

// EvaluateDependencies is the canonical function for classifying all dependency
// edges on an issue. It de-duplicates by type|dependsOnID, classifies each edge,
// and computes aggregate blocked state.
func (i *Issue) EvaluateDependencies(issueMap map[string]*Issue, blockingTypes map[string]bool) DepEval {
	var eval DepEval
	seen := make(map[string]bool)

	for _, dep := range i.Dependencies {
		key := dep.Type + "|" + dep.DependsOnID
		if seen[key] {
			continue
		}
		seen[key] = true

		edge := DepEdge{Type: dep.Type, DependsOnID: dep.DependsOnID}

		if !blockingTypes[dep.Type] {
			edge.Status = DepNonBlocking
			eval.NonBlocking = append(eval.NonBlocking, edge)
			eval.Edges = append(eval.Edges, edge)
			continue
		}

		target, exists := issueMap[dep.DependsOnID]
		if !exists {
			edge.Status = DepMissing
			eval.MissingIDs = append(eval.MissingIDs, dep.DependsOnID)
		} else if target.Status == StatusClosed {
			edge.Status = DepResolved
			eval.ResolvedIDs = append(eval.ResolvedIDs, dep.DependsOnID)
		} else {
			edge.Status = DepBlocking
			eval.BlockingIDs = append(eval.BlockingIDs, dep.DependsOnID)
		}
		eval.Edges = append(eval.Edges, edge)
	}

	eval.IsBlocked = len(eval.BlockingIDs) > 0 || len(eval.MissingIDs) > 0
	if len(eval.BlockingIDs) > 0 {
		eval.NextBlockerID = eval.BlockingIDs[0]
	} else if len(eval.MissingIDs) > 0 {
		eval.NextBlockerID = eval.MissingIDs[0]
	}
	return eval
}

// IsBlocked returns true if this issue depends on an unclosed blocker.
// Delegates to EvaluateDependencies with DefaultBlockingTypes.
func (i *Issue) IsBlocked(issueMap map[string]*Issue) bool {
	return i.EvaluateDependencies(issueMap, DefaultBlockingTypes).IsBlocked
}

// BlockedByIDs returns the IDs of open issues blocking this one.
// Delegates to EvaluateDependencies with DefaultBlockingTypes.
func (i *Issue) BlockedByIDs(issueMap map[string]*Issue) []string {
	eval := i.EvaluateDependencies(issueMap, DefaultBlockingTypes)
	return eval.BlockingIDs
}

// BlocksIDs returns the IDs of issues that this issue blocks.
func (i *Issue) BlocksIDs(allIssues []Issue, blockingTypes map[string]bool) []string {
	var blocked []string
	for _, other := range allIssues {
		for _, dep := range other.Dependencies {
			if blockingTypes[dep.Type] && dep.DependsOnID == i.ID {
				blocked = append(blocked, other.ID)
				break
			}
		}
	}
	return blocked
}

// Age returns the duration since the issue was created.
func (i *Issue) Age() time.Duration {
	return time.Since(i.CreatedAt)
}

// AgeLabel returns a human-readable age string.
func (i *Issue) AgeLabel() string {
	days := int(i.Age().Hours() / 24)
	switch {
	case days == 0:
		hours := int(i.Age().Hours())
		if hours == 0 {
			return "just now"
		}
		return fmt.Sprintf("%dh", hours)
	case days == 1:
		return "1 day"
	case days < 30:
		return fmt.Sprintf("%d days", days)
	default:
		return fmt.Sprintf("%d weeks", days/7)
	}
}

// ParadeGroup determines which parade section this issue belongs to.
// Stalled wins over Rolling: an in_progress issue with unresolved blockers is Stalled.
func (i *Issue) ParadeGroup(issueMap map[string]*Issue, blockingTypes map[string]bool) ParadeStatus {
	switch i.Status {
	case StatusClosed:
		return ParadePastTheStand
	case StatusInProgress:
		if i.EvaluateDependencies(issueMap, blockingTypes).IsBlocked {
			return ParadeStalled
		}
		return ParadeRolling
	case StatusOpen:
		if i.EvaluateDependencies(issueMap, blockingTypes).IsBlocked {
			return ParadeStalled
		}
		return ParadeLinedUp
	default:
		return ParadeLinedUp
	}
}

// PriorityLabel returns "P0" through "P4".
func PriorityLabel(p Priority) string {
	return fmt.Sprintf("P%d", p)
}

// PriorityName returns the full name for a priority level.
func PriorityName(p Priority) string {
	switch p {
	case PriorityCritical:
		return "Critical"
	case PriorityHigh:
		return "High"
	case PriorityMedium:
		return "Medium"
	case PriorityLow:
		return "Low"
	case PriorityBacklog:
		return "Backlog"
	default:
		return "Unknown"
	}
}

// BuildIssueMap creates a lookup map from a slice of issues.
func BuildIssueMap(issues []Issue) map[string]*Issue {
	m := make(map[string]*Issue, len(issues))
	for idx := range issues {
		m[issues[idx].ID] = &issues[idx]
	}
	return m
}
