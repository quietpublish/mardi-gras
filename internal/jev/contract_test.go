//go:build jevcontract

package jev

// The contract test runs mg's client against a real System One endpoint. It
// is not part of `make test`: it needs a key, costs a fraction of a cent, and
// cannot run from a sandbox that blocks egress. Run it from a developer
// machine before a release:
//
//	make contract-jev KEY=$TYPESAFE_API_KEY
//	MG_JEV_CONTRACT_URL=http://localhost:8080 make contract-jev KEY=x   # a self-hosted server
//
// It checks the envelope decodes, each question type comes back typed, and
// usage is reported, which keeps the cost meter honest.

import (
	"context"
	"os"
	"slices"
	"testing"
	"time"
)

func TestJevContract(t *testing.T) {
	key := os.Getenv("MG_JEV_CONTRACT_KEY")
	if key == "" {
		t.Skip("MG_JEV_CONTRACT_KEY not set")
	}
	opts := []ClientOption{WithTimeout(10 * time.Second)}
	if u := os.Getenv("MG_JEV_CONTRACT_URL"); u != "" {
		opts = append(opts, WithURL(u))
	}
	c, err := NewClient(key, opts...)
	if err != nil {
		t.Fatal(err)
	}

	state := map[string]any{
		"issue": map[string]any{
			"id": "mg-1", "title": "Login loop on Safari after 2FA", "type": "bug", "priority": 1,
			"description": "Users on Safari 18 are bounced back to the login page after entering a 2FA code.",
		},
	}
	levels := []string{"Park", "Can wait", "Do next", "Do now"}
	res, err := c.Evaluate(context.Background(), state, map[string]Question{
		"user_facing": NewNoul("Does this issue affect end users directly?"),
		"area": NewChoice("Which area of the product does this issue belong to?",
			Option{Name: "auth", Desc: "login, sessions, 2FA"},
			Option{Name: "billing", Desc: "payments and invoices"},
			Option{Name: "docs"}),
		"urgency": NewScore("How urgently should this be started?", "Park", "Can wait", "Do next", "Do now"),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("model %s usage %+v", res.Model, res.Usage)
	for name, a := range res.Answers {
		t.Logf("%s: %+v", name, a)
	}

	if a := res.Answers["user_facing"]; a.Type != Noul || a.Noul < 0 || a.Noul > 1 {
		t.Errorf("user_facing = %+v", a)
	} else if a.Confidence < 0.5 || a.Confidence > 1 {
		// The hosted API sends no confidence for a noul; the client derives
		// one, and every gate in mg reads it.
		t.Errorf("user_facing confidence = %v, want max(p,1-p) in [0.5,1]", a.Confidence)
	}
	if a := res.Answers["area"]; a.Type != Choice || a.Choice == "" {
		t.Errorf("area = %+v", a)
	} else if sum := probSum(a); len(a.Probabilities) > 0 && (sum < 0.95 || sum > 1.05) {
		t.Errorf("area probabilities sum to %v", sum)
	}
	if a := res.Answers["urgency"]; a.Type != Score {
		t.Errorf("urgency = %+v", a)
	} else if len(a.Probabilities) > 0 {
		// Index keys must have been relabelled, and the point score must be
		// the expected index of the distribution it came with.
		var expected float64
		for i, l := range levels {
			expected += float64(i) * a.Probabilities[l]
		}
		for k := range a.Probabilities {
			if !slices.Contains(levels, k) {
				t.Errorf("urgency probabilities keyed by %q, want a level label: %+v", k, a.Probabilities)
			}
		}
		if d := a.Score - expected; d < -0.05 || d > 0.05 {
			t.Errorf("urgency score %v but expected index %v from %+v", a.Score, expected, a.Probabilities)
		}
		if a.Confidence <= 0 {
			t.Errorf("urgency confidence = %v", a.Confidence)
		}
	}
	if res.Usage.InputTokens == 0 {
		t.Error("usage.input_tokens is zero; the cost meter would under-report")
	}
}

func probSum(a Answer) float64 {
	var s float64
	for _, p := range a.Probabilities {
		s += p
	}
	return s
}
