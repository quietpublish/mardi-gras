package jev

import (
	"bytes"
	"encoding/json"
)

// Kind is a question type.
type Kind string

const (
	// Noul asks a yes/no question; the answer is p(yes).
	Noul Kind = "noul"
	// Choice asks for one of a set of named options.
	Choice Kind = "choice"
	// Score asks for a point on an ordered scale of labelled levels.
	Score Kind = "score"
)

// Question is one typed question. Criteria is nil for a noul, the options
// for a choice and the ordered level labels for a score. Build questions
// with NewNoul, NewChoice and NewScore so the criteria match the type.
type Question struct {
	Type         Kind   `json:"type"`
	Instructions string `json:"instructions"`
	Criteria     any    `json:"criteria,omitempty"`
}

// Option is one choice option. Desc may be empty.
type Option struct {
	Name string
	Desc string
}

// options marshals as a JSON object that keeps the options in order, which
// is the form the API documents for described options.
type options []Option

func (o options) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, opt := range o {
		if i > 0 {
			b.WriteByte(',')
		}
		k, err := json.Marshal(opt.Name)
		if err != nil {
			return nil, err
		}
		var v []byte
		if opt.Desc == "" {
			v = []byte("null")
		} else {
			v, err = json.Marshal(opt.Desc)
			if err != nil {
				return nil, err
			}
		}
		b.Write(k)
		b.WriteByte(':')
		b.Write(v)
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}

// NewNoul builds a yes/no question.
func NewNoul(instructions string) Question {
	return Question{Type: Noul, Instructions: instructions}
}

// NewChoice builds a pick-one question over opts, in the order given.
func NewChoice(instructions string, opts ...Option) Question {
	return Question{Type: Choice, Instructions: instructions, Criteria: options(opts)}
}

// NewScore builds an ordinal question over levels, from lowest to highest.
func NewScore(instructions string, levels ...string) Question {
	return Question{Type: Score, Instructions: instructions, Criteria: levels}
}

// Answer is one typed verdict as the server returns it. Noul is p(yes) for a
// noul; Choice names the chosen option; Score is the position on the level
// scale. Probabilities is keyed by option or level label, and Confidence is
// the model's calibrated confidence in the answer.
type Answer struct {
	Type          Kind               `json:"type"`
	Noul          float64            `json:"noul"`
	Choice        string             `json:"choice"`
	Score         float64            `json:"score"`
	Probabilities map[string]float64 `json:"probabilities,omitempty"`
	Confidence    float64            `json:"confidence"`
}

// Yes reports a noul answer at or above threshold, the caller's gate.
func (a Answer) Yes(threshold float64) bool {
	return a.Type == Noul && a.Noul >= threshold
}

// Usage is what a request cost. Output tokens are free but reported.
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// Result bundles the answers to one Evaluate call with what they cost, so
// the app can meter spend.
type Result struct {
	Model   string
	Answers map[string]Answer
	Usage   Usage
}

// Wire envelope.
type request struct {
	State     any                 `json:"state"`
	Model     string              `json:"model"`
	Questions map[string]Question `json:"questions"`
}

type response struct {
	Model   string            `json:"model"`
	Answers map[string]Answer `json:"answers"`
	Usage   Usage             `json:"usage"`
}
