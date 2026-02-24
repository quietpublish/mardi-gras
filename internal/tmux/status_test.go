package tmux

import (
	"strings"
	"testing"

	"github.com/matt-wright86/mardi-gras/internal/data"
	"github.com/matt-wright86/mardi-gras/internal/ui"
)

func TestStatusLineFormat(t *testing.T) {
	issues, _, err := data.LoadIssues("../../testdata/sample.jsonl")
	if err != nil {
		t.Fatalf("LoadIssues: %v", err)
	}

	groups := data.GroupByParade(issues, data.DefaultBlockingTypes)
	got := StatusLine(groups)

	// Verify tmux markup present
	if !strings.Contains(got, "#[fg=") {
		t.Errorf("expected tmux fg markup, got: %s", got)
	}

	// Verify all symbols present
	for _, sym := range []string{ui.FleurDeLis, ui.SymRolling, ui.SymLinedUp, ui.SymStalled, ui.SymPassed} {
		if !strings.Contains(got, sym) {
			t.Errorf("missing symbol %q in: %s", sym, got)
		}
	}

	// Verify correct counts
	for _, want := range []string{"3" + ui.SymRolling, "12" + ui.SymLinedUp, "3" + ui.SymStalled, "3" + ui.SymPassed} {
		if !strings.Contains(got, want) {
			t.Errorf("missing count %q in: %s", want, got)
		}
	}
}

func TestStatusLineEmptyGroups(t *testing.T) {
	groups := map[data.ParadeStatus][]data.Issue{
		data.ParadeRolling:      {},
		data.ParadeLinedUp:      {},
		data.ParadeStalled:      {},
		data.ParadePastTheStand: {},
	}

	got := StatusLine(groups)

	// All counts should be 0
	for _, want := range []string{"0" + ui.SymRolling, "0" + ui.SymLinedUp, "0" + ui.SymStalled, "0" + ui.SymPassed} {
		if !strings.Contains(got, want) {
			t.Errorf("missing zero count %q in: %s", want, got)
		}
	}
}
