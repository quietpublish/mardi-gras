package jev

import (
	"bytes"
	"encoding/json"
	"strconv"
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

// Answer is one typed verdict, normalized. Noul is p(yes) for a noul; Choice
// names the chosen option; Score is the expected position on the level
// scale (0 is the first level). Probabilities is keyed by option or level
// label and Confidence is the probability of the answer given: the top
// option's or level's, or max(p, 1-p) for a noul.
//
// The hosted API (observed against jev-1.13.0) keys a score's probabilities
// by level index with a separate legend, and sends a noul with neither
// probabilities nor confidence. normalize folds both into this shape so no
// caller has to know, and a server that already answers in labels and
// confidences passes through unchanged.
type Answer struct {
	Type          Kind
	Noul          float64
	Choice        string
	Score         float64
	Probabilities map[string]float64
	Confidence    float64
}

// wireAnswer is an answer as the server sends it. Confidence is a pointer so
// an omitted field is told apart from a reported zero, and Legend maps
// index keys in Probabilities to labels when the server keys by index.
type wireAnswer struct {
	Type          Kind               `json:"type"`
	Noul          float64            `json:"noul"`
	Choice        string             `json:"choice"`
	Score         float64            `json:"score"`
	Legend        map[string]string  `json:"legend,omitempty"`
	Probabilities map[string]float64 `json:"probabilities,omitempty"`
	Confidence    *float64           `json:"confidence,omitempty"`
}

// normalize converts a wire answer into an Answer, using the question it
// answers to label index-keyed probabilities when the server sent no legend.
func (w wireAnswer) normalize(q Question) Answer {
	a := Answer{Type: w.Type, Noul: w.Noul, Choice: w.Choice, Score: w.Score}
	if len(w.Probabilities) > 0 {
		a.Probabilities = relabel(w.Probabilities, w.Legend, scoreLevels(q))
	}
	switch {
	case w.Confidence != nil:
		a.Confidence = *w.Confidence
	case a.Type == Noul:
		a.Confidence = max(a.Noul, 1-a.Noul)
	default:
		for _, p := range a.Probabilities {
			a.Confidence = max(a.Confidence, p)
		}
	}
	return a
}

// relabel maps probability keys through legend, then through levels for
// keys that are level indices. Keys neither covers are kept as they are.
func relabel(probs map[string]float64, legend map[string]string, levels []string) map[string]float64 {
	out := make(map[string]float64, len(probs))
	for k, p := range probs {
		label := k
		if l, ok := legend[k]; ok && l != "" {
			label = l
		} else if i, err := strconv.Atoi(k); err == nil && i >= 0 && i < len(levels) {
			label = levels[i]
		}
		out[label] += p
	}
	return out
}

// scoreLevels returns a score question's levels, or nil.
func scoreLevels(q Question) []string {
	if q.Type != Score {
		return nil
	}
	levels, _ := q.Criteria.([]string)
	return levels
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
	Model   string                `json:"model"`
	Answers map[string]wireAnswer `json:"answers"`
	Usage   Usage                 `json:"usage"`
}
