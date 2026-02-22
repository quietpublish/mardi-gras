package data

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sort"
)

// LoadIssues reads and parses a Beads JSONL file.
func LoadIssues(path string) ([]Issue, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open issues file: %w", err)
	}
	defer f.Close()

	var issues []Issue
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var issue Issue
		if err := json.Unmarshal(line, &issue); err != nil {
			return nil, fmt.Errorf("parse issue at line %d: %w", lineNum, err)
		}
		issues = append(issues, issue)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan issues file: %w", err)
	}

	SortIssues(issues)
	return issues, nil
}

// SortIssues sorts by: active first, then priority (ascending), then recency.
func SortIssues(issues []Issue) {
	sort.Slice(issues, func(i, j int) bool {
		a, b := issues[i], issues[j]
		aActive := a.Status != StatusClosed
		bActive := b.Status != StatusClosed
		if aActive != bActive {
			return aActive
		}
		if a.Priority != b.Priority {
			return a.Priority < b.Priority
		}
		return a.UpdatedAt.After(b.UpdatedAt)
	})
}

// GroupByParade groups issues into parade sections.
func GroupByParade(issues []Issue, blockingTypes map[string]bool) map[ParadeStatus][]Issue {
	issueMap := BuildIssueMap(issues)
	groups := map[ParadeStatus][]Issue{
		ParadeRolling:      {},
		ParadeLinedUp:      {},
		ParadeStalled:      {},
		ParadePastTheStand: {},
	}
	for _, issue := range issues {
		group := issue.ParadeGroup(issueMap, blockingTypes)
		groups[group] = append(groups[group], issue)
	}
	return groups
}
