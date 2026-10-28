package decision

import (
	"fmt"
	"strconv"
)

// CheckRequest validates req for a decision model that runs in process
// (one that an application supplies). It applies the question rules of
// every wire, but no wire limits: the model decides how many questions and
// which image types it accepts. It returns an error that wraps
// ErrInvalidRequest.
func CheckRequest(req Request) error {
	if err := validateQuestions(req); err != nil {
		return err
	}
	for i, img := range req.State.Images {
		if len(img.Data) == 0 {
			return invalid("image %d is empty", i)
		}
		if img.MediaType == "" {
			return invalid("image %d has no media type", i)
		}
	}
	return nil
}

// CheckResult normalizes the result of an in-process decision model in the
// same way as a wire response. Each question must have exactly one answer
// of its type, and no answer may have an unknown ID. CheckResult puts the
// answers in question order, puts the distributions in option or level
// order and sets Level from the distribution (else from Score). It returns
// an error that wraps ErrInvalidResponse. res is not changed.
func CheckResult(req Request, res *Result) (*Result, error) {
	if res == nil {
		return nil, fmt.Errorf("%w: no result", ErrInvalidResponse)
	}
	byID := make(map[string]Answer, len(res.Answers))
	for _, a := range res.Answers {
		if _, dup := byID[a.ID]; dup {
			return nil, fmt.Errorf("%w: duplicate answer %q", ErrInvalidResponse, a.ID)
		}
		byID[a.ID] = a
	}

	out := *res
	out.Answers = make([]Answer, 0, len(req.Questions))
	for _, q := range req.Questions {
		a, ok := byID[q.ID]
		if !ok {
			return nil, fmt.Errorf("%w: no answer for question %q", ErrInvalidResponse, q.ID)
		}
		delete(byID, q.ID)
		if a.Type != q.Type {
			return nil, fmt.Errorf("%w: question %q is %s, answer is %q", ErrInvalidResponse, q.ID, q.Type, a.Type)
		}
		if a.Refused {
			out.Answers = append(out.Answers, Answer{ID: q.ID, Type: q.Type, Refused: true})
			continue
		}
		norm, err := buildAnswer(q, a.Choice, a.Score, a.Probability, distributionProbs(q, a.Distribution), a.Confidence)
		if err != nil {
			return nil, err
		}
		out.Answers = append(out.Answers, norm)
	}
	for id := range byID {
		return nil, fmt.Errorf("%w: answer %q matches no question", ErrInvalidResponse, id)
	}
	return &out, nil
}

// distributionProbs converts a distribution to the probs map of
// buildAnswer: keyed by option name for a choice question and by level
// index for a score question. A score entry is found by its level name, or
// by its index when the name is empty.
func distributionProbs(q Question, dist []Probability) map[string]float64 {
	if len(dist) == 0 {
		return nil
	}
	probs := make(map[string]float64, len(dist))
	for _, d := range dist {
		switch q.Type {
		case Choice:
			probs[d.Name] = d.Probability
		case Score:
			idx := d.Index
			if d.Name != "" {
				idx = -1
				for i, l := range q.Levels {
					if l == d.Name {
						idx = i
						break
					}
				}
			}
			if idx >= 0 && idx < len(q.Levels) {
				probs[strconv.Itoa(idx)] = d.Probability
			}
		}
	}
	return probs
}
