package components

import (
	"testing"

	"github.com/matt-wright86/mardi-gras/internal/data"
)

func TestParsePriority(t *testing.T) {
	tests := []struct {
		input  string
		expect data.Priority
	}{
		{"0", data.PriorityCritical},
		{"1", data.PriorityHigh},
		{"2", data.PriorityMedium},
		{"3", data.PriorityLow},
		{"4", data.PriorityBacklog},
		{"", data.PriorityMedium},
		{"999", data.PriorityMedium},
		{"abc", data.PriorityMedium},
	}

	for _, tc := range tests {
		t.Run("input_"+tc.input, func(t *testing.T) {
			got := ParsePriority(tc.input)
			if got != tc.expect {
				t.Fatalf("ParsePriority(%q) = %d, want %d", tc.input, got, tc.expect)
			}
		})
	}
}
