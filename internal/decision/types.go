// Package decision sends typed questions to decision models and normalizes
// their answers. Decision models answer each question with probabilities
// instead of generated text.
//
// The package speaks three wires (see models.DecisionWire): SystemOne (the
// TypeSafe request shape that most hosts speak), Cloudflare Workers AI (the
// SystemOne body in Cloudflare's envelope) and the Vercel AI Gateway
// decision protocol. Callers use one request and one result shape for all
// of them.
package decision

import (
	"errors"
	"fmt"
	"math"
	"strconv"
)

// Errors returned by Decide and the answer accessors.
var (
	// ErrInvalidRequest indicates a request that fails validation. Kit
	// returns it before it sends anything.
	ErrInvalidRequest = errors.New("invalid decision request")

	// ErrInvalidResponse indicates an answer set that does not match the
	// questions: a missing answer, a wrong type or an unknown option.
	ErrInvalidResponse = errors.New("invalid decision response")

	// ErrRefused indicates that the model refused to answer a question.
	ErrRefused = errors.New("decision model refused the question")

	// ErrNoAnswer indicates that a result has no answer with the given ID.
	ErrNoAnswer = errors.New("no answer for question")

	// ErrWrongAnswerType indicates an accessor for a different question type.
	ErrWrongAnswerType = errors.New("wrong answer type")
)

// QuestionType is the kind of a question.
type QuestionType string

const (
	// Boolean questions are answered with P(true).
	Boolean QuestionType = "boolean"
	// Choice questions are answered with one option from a named set.
	Choice QuestionType = "choice"
	// Score questions are answered with a position on an ordered scale.
	Score QuestionType = "score"
)

// Option is one answer of a choice question.
type Option struct {
	// Name is the value returned when the model selects this option.
	Name string `json:"name"`
	// Description tells the model what the option means. Empty lets the
	// name describe itself.
	Description string `json:"description,omitempty"`
}

// Question is one typed question. Use the fields that match Type.
type Question struct {
	// ID identifies the answer. It must be unique in the request.
	ID   string       `json:"id"`
	Type QuestionType `json:"type"`
	// Instructions state the question.
	Instructions string `json:"instructions,omitempty"`
	// Options are the answers of a choice question, in order.
	Options []Option `json:"options,omitempty"`
	// Levels are the steps of a score question, lowest first.
	Levels []string `json:"levels,omitempty"`
	// True and False optionally describe the two cases of a boolean
	// question.
	True  string `json:"true,omitempty"`
	False string `json:"false,omitempty"`
}

// Image is an image attached to the state.
type Image struct {
	// MediaType is the image type, for example "image/png".
	MediaType string `json:"media_type"`
	Data      []byte `json:"data"`
	// Filename is optional.
	Filename string `json:"filename,omitempty"`
}

// State is what the questions are about. Set Text, JSON or both, and
// optionally Images.
type State struct {
	Text string `json:"text,omitempty"`
	// JSON is any value that encodes to JSON: a struct, map or slice.
	JSON   any     `json:"json,omitempty"`
	Images []Image `json:"images,omitempty"`
}

// Request is one decision request.
type Request struct {
	State     State      `json:"state"`
	Questions []Question `json:"questions"`
	// ProviderOptions are sent as-is on wires that accept them (Vercel AI
	// Gateway), for example {"gateway": {"zeroDataRetention": true}}.
	ProviderOptions map[string]any `json:"provider_options,omitempty"`
}

// Probability is the probability of one option or level.
type Probability struct {
	// Index is the position of the option or level in the question.
	Index int `json:"index"`
	// Name is the option name or the level text.
	Name        string  `json:"name"`
	Probability float64 `json:"probability"`
}

// Answer is the normalized answer to one question.
type Answer struct {
	ID   string       `json:"id"`
	Type QuestionType `json:"type"`
	// Refused is true when the model refused the question. The other
	// fields are then zero.
	Refused bool `json:"refused,omitempty"`
	// Probability is P(true) for a boolean question.
	Probability float64 `json:"probability,omitempty"`
	// Choice is the selected option of a choice question.
	Choice string `json:"choice,omitempty"`
	// Score is the expected level of a score question, in [0, levels-1].
	// It can be fractional.
	Score float64 `json:"score,omitempty"`
	// Level is the most probable level of a score question.
	Level int `json:"level,omitempty"`
	// Distribution has the probability of each option or level, in
	// question order. Nil when the model returns none.
	Distribution []Probability `json:"distribution,omitempty"`
	// Confidence shows how strongly the distribution favors one answer. It
	// is not the probability that the answer is correct. Nil when the model
	// returns none.
	Confidence *float64 `json:"confidence,omitempty"`
}

// Prob returns the probability of the named option or level. ok is false
// when the distribution does not include it.
func (a Answer) Prob(name string) (p float64, ok bool) {
	for _, d := range a.Distribution {
		if d.Name == name {
			return d.Probability, true
		}
	}
	return 0, false
}

// Usage is the token usage of one request.
type Usage struct {
	InputTokens  int64 `json:"input_tokens"`
	OutputTokens int64 `json:"output_tokens"`
}

// Result is the normalized result of one request.
type Result struct {
	// Answers are in question order.
	Answers []Answer `json:"answers"`
	// Model is the model that answered, as reported by the provider (for
	// example a resolved version such as "jev-1.14.0").
	Model string `json:"model,omitempty"`
	Usage Usage  `json:"usage"`
	// Metadata holds provider-specific data, such as gateway routing and
	// cost.
	Metadata map[string]any `json:"metadata,omitempty"`
}

// Answer returns the answer with the given ID.
func (r *Result) Answer(id string) (Answer, bool) {
	for _, a := range r.Answers {
		if a.ID == id {
			return a, true
		}
	}
	return Answer{}, false
}

// Boolean returns the answer to a boolean question.
func (r *Result) Boolean(id string) (Answer, error) { return r.typed(id, Boolean) }

// Choice returns the answer to a choice question.
func (r *Result) Choice(id string) (Answer, error) { return r.typed(id, Choice) }

// Score returns the answer to a score question.
func (r *Result) Score(id string) (Answer, error) { return r.typed(id, Score) }

func (r *Result) typed(id string, t QuestionType) (Answer, error) {
	a, ok := r.Answer(id)
	if !ok {
		return Answer{}, fmt.Errorf("%w %q", ErrNoAnswer, id)
	}
	if a.Type != t {
		return Answer{}, fmt.Errorf("%w: question %q is %s, not %s", ErrWrongAnswerType, id, a.Type, t)
	}
	if a.Refused {
		return a, fmt.Errorf("%w %q", ErrRefused, id)
	}
	return a, nil
}

// buildAnswer normalizes one wire answer against its question. probs is
// keyed by option name (choice) or level index (score).
func buildAnswer(q Question, choice string, score, probability float64, probs map[string]float64, confidence *float64) (Answer, error) {
	a := Answer{ID: q.ID, Type: q.Type, Confidence: confidence}
	switch q.Type {
	case Boolean:
		if probability < 0 || probability > 1 || math.IsNaN(probability) {
			return Answer{}, fmt.Errorf("%w: question %q: probability %v is out of range", ErrInvalidResponse, q.ID, probability)
		}
		a.Probability = probability
	case Choice:
		found := false
		for i, o := range q.Options {
			if o.Name == choice {
				found = true
			}
			if p, ok := probs[o.Name]; ok {
				a.Distribution = append(a.Distribution, Probability{Index: i, Name: o.Name, Probability: p})
			}
		}
		if !found {
			return Answer{}, fmt.Errorf("%w: question %q: unknown choice %q", ErrInvalidResponse, q.ID, choice)
		}
		a.Choice = choice
	case Score:
		maxLevel := float64(len(q.Levels) - 1)
		if score < 0 || score > maxLevel || math.IsNaN(score) {
			return Answer{}, fmt.Errorf("%w: question %q: score %v is out of range [0, %v]", ErrInvalidResponse, q.ID, score, maxLevel)
		}
		a.Score = score
		a.Level = int(math.Round(score))
		best := -1.0
		for i, level := range q.Levels {
			p, ok := probs[strconv.Itoa(i)]
			if !ok {
				continue
			}
			a.Distribution = append(a.Distribution, Probability{Index: i, Name: level, Probability: p})
			if p > best {
				best, a.Level = p, i
			}
		}
	}
	return a, nil
}
