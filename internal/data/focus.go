package data

import (
	"os"
	"sort"
	"strings"
)

// FocusFilter returns a filtered, prioritized list for focus mode:
//  1. In-progress issues that are yours: assigned to you (see CurrentActor
//     for who that is), or claimed by nobody, as when an agent marks an issue
//     in progress without claiming it
//  2. Highest priority unblocked issues (open, not blocked)
//  3. Blocked issues with context
//
// With no known identity (me == ""), every in-progress issue counts.
func FocusFilter(issues []Issue, blockingTypes map[string]bool, me string) []Issue {
	myWork, ready, blocked := focusGroups(issues, blockingTypes, me)

	// Sort ready by priority (P0 first)
	sort.Slice(ready, func(i, j int) bool {
		return ready[i].Priority < ready[j].Priority
	})
	return focusAssemble(myWork, ready, blocked)
}

// FocusLevels is the urgency scale a judge rates issues on, lowest first.
var FocusLevels = []string{"Park", "Can wait", "Do next", "Do now"}

// FocusVerdict is a judge's opinion of one open issue for focus mode.
type FocusVerdict struct {
	Urgency              float64 // expected position on FocusLevels, 0 (Park) to 3 (Do now)
	Confidence           float64 // the judge's confidence in Urgency
	Actionable           float64 // p(can be started right now)
	ActionableConfidence float64
}

// Thresholds for acting on a verdict: below the confidence floor an issue
// keeps the slot its priority gives it, and only a confident "cannot start"
// moves an unblocked issue out of the ready list.
const (
	FocusConfidenceFloor   = 0.6
	focusNotActionableProb = 0.4
	focusNotActionableConf = 0.7
)

// FocusLevelLabel names the level nearest to an urgency.
func FocusLevelLabel(urgency float64) string {
	i := int(urgency + 0.5)
	if i < 0 {
		i = 0
	}
	if i >= len(FocusLevels) {
		i = len(FocusLevels) - 1
	}
	return FocusLevels[i]
}

// FocusRank is FocusFilter with the ready list ordered by a judge's
// verdicts: confident urgency first, and an issue the judge is confident
// cannot be started now moves down with the blocked ones. An issue without
// a confident verdict keeps the place its priority alone would give it, so a
// shaky verdict never jumps the queue. Without verdicts it is FocusFilter.
func FocusRank(issues []Issue, blockingTypes map[string]bool, me string, verdicts map[string]FocusVerdict) []Issue {
	if len(verdicts) == 0 {
		return FocusFilter(issues, blockingTypes, me)
	}
	myWork, ready, blocked := focusGroups(issues, blockingTypes, me)

	kept := ready[:0]
	for _, iss := range ready {
		if v, ok := verdicts[iss.ID]; ok && v.Actionable < focusNotActionableProb && v.ActionableConfidence >= focusNotActionableConf {
			blocked = append(blocked, iss)
			continue
		}
		kept = append(kept, iss)
	}
	ready = kept

	sort.SliceStable(ready, func(i, j int) bool {
		si, sj := focusScore(ready[i], verdicts), focusScore(ready[j], verdicts)
		if si != sj {
			return si > sj
		}
		if ready[i].Priority != ready[j].Priority {
			return ready[i].Priority < ready[j].Priority
		}
		return ready[i].UpdatedAt.After(ready[j].UpdatedAt)
	})
	return focusAssemble(myWork, ready, blocked)
}

// focusScore places an issue on the urgency scale: the judge's expected
// level when confident, else the level its priority implies (P0 is "Do
// now", P4 is "Park"), so both kinds of issue sort on one axis.
func focusScore(iss Issue, verdicts map[string]FocusVerdict) float64 {
	if v, ok := verdicts[iss.ID]; ok && v.Confidence >= FocusConfidenceFloor {
		return v.Urgency
	}
	return max(0, 3-0.75*float64(iss.Priority))
}

// focusGroups splits the open issues into the three focus-mode lists.
func focusGroups(issues []Issue, blockingTypes map[string]bool, me string) (myWork, ready, blocked []Issue) {
	issueMap := BuildIssueMap(issues)
	for _, iss := range issues {
		if iss.Status == StatusClosed {
			continue
		}
		isBlocked := iss.EvaluateDependencies(issueMap, blockingTypes).IsBlocked
		isMine := me == "" || iss.Assignee == "" || SameActor(iss.Assignee, me)

		switch {
		case iss.Status == StatusInProgress:
			if isMine {
				myWork = append(myWork, iss)
			}
		case isBlocked:
			blocked = append(blocked, iss)
		default:
			ready = append(ready, iss)
		}
	}
	return myWork, ready, blocked
}

// focusAssemble caps and concatenates the lists: my work, then up to five
// ready issues, then up to three blocked ones for context.
func focusAssemble(myWork, ready, blocked []Issue) []Issue {
	if len(ready) > 5 {
		ready = ready[:5]
	}
	if len(blocked) > 3 {
		blocked = blocked[:3]
	}
	var result []Issue
	result = append(result, myWork...)
	result = append(result, ready...)
	result = append(result, blocked...)
	return result
}

// CurrentActor resolves who "you" are the way bd 1.3.0 does when it records
// a claim: BEADS_ACTOR, BD_ACTOR, the workspace's .beads/config.yaml actor,
// git user.name, then $USER. Claims assign the issue to that name, so focus
// mode must compare assignees with it rather than with the login name.
func CurrentActor(projectDir string) string {
	for _, env := range []string{"BEADS_ACTOR", "BD_ACTOR"} {
		if v := strings.TrimSpace(os.Getenv(env)); v != "" {
			return v
		}
	}
	if actor := LoadConfiguredActor(projectDir); actor != "" {
		return actor
	}
	if out, err := runWithTimeout(timeoutShort, "git", "config", "user.name"); err == nil {
		if name := strings.TrimSpace(string(out)); name != "" {
			return name
		}
	}
	return strings.TrimSpace(os.Getenv("USER"))
}

// SameActor reports whether two actor names are the same identity, by bd's
// rule: a run of '.', '_' or '-' is one separator, except an exact "--",
// which stands for '/'. Unlike bd, it also ignores case.
func SameActor(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	return strings.EqualFold(canonicalActor(a), canonicalActor(b))
}

func canonicalActor(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); {
		if !isActorSeparator(s[i]) {
			b.WriteByte(s[i])
			i++
			continue
		}
		j := i
		for j < len(s) && isActorSeparator(s[j]) {
			j++
		}
		if s[i:j] == "--" {
			b.WriteByte('/')
		} else {
			b.WriteByte('_')
		}
		i = j
	}
	return b.String()
}

func isActorSeparator(c byte) bool {
	return c == '.' || c == '_' || c == '-'
}
