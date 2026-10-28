package decision

import (
	"fmt"
	"slices"

	"github.com/mark3labs/kit/internal/models"
)

// Limits shared by the SystemOne hosts (TypeSafe, Cloudflare, Ollama,
// OpenRouter and others).
const (
	maxQuestions = 64
	minOptions   = 2
	maxOptions   = 255
	minLevels    = 2
	maxLevels    = 10
)

// validate checks a request against the question rules and the endpoint's
// image support. It returns an error that wraps ErrInvalidRequest.
func validate(ep *models.DecisionEndpoint, req Request) error {
	if err := validateQuestions(req); err != nil {
		return err
	}
	if ep.Wire != models.DecisionWireGateway && len(req.Questions) > maxQuestions {
		return invalid("%d questions; the limit is %d", len(req.Questions), maxQuestions)
	}

	if n := len(req.State.Images); n > 0 {
		if !ep.AcceptsImages {
			return invalid("model %s/%s does not accept images", ep.Provider, ep.Model)
		}
		if ep.MaxImages > 0 && n > ep.MaxImages {
			return invalid("%d images; %s accepts at most %d", n, ep.Provider, ep.MaxImages)
		}
		for i, img := range req.State.Images {
			if len(img.Data) == 0 {
				return invalid("image %d is empty", i)
			}
			if !slices.Contains(ep.ImageTypes, img.MediaType) {
				return invalid("image %d has media type %q; %s accepts %v", i, img.MediaType, ep.Provider, ep.ImageTypes)
			}
		}
	}
	return nil
}

// invalid returns an error that wraps ErrInvalidRequest.
func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidRequest, fmt.Sprintf(format, args...))
}

// validateQuestions checks the state and the question rules that apply to
// every model. It does not check limits that depend on the wire.
func validateQuestions(req Request) error {
	if req.State.Text == "" && req.State.JSON == nil && len(req.State.Images) == 0 {
		return invalid("state is empty")
	}
	if len(req.Questions) == 0 {
		return invalid("no questions")
	}

	seen := make(map[string]bool, len(req.Questions))
	for _, q := range req.Questions {
		if q.ID == "" {
			return invalid("a question has no ID")
		}
		if seen[q.ID] {
			return invalid("duplicate question ID %q", q.ID)
		}
		seen[q.ID] = true

		switch q.Type {
		case Boolean:
			if q.Instructions == "" && q.True == "" && q.False == "" {
				return invalid("boolean question %q needs instructions or a true/false description", q.ID)
			}
		case Choice:
			if len(q.Options) < minOptions || len(q.Options) > maxOptions {
				return invalid("choice question %q has %d options; it needs %d to %d", q.ID, len(q.Options), minOptions, maxOptions)
			}
			names := make(map[string]bool, len(q.Options))
			for _, o := range q.Options {
				if o.Name == "" {
					return invalid("choice question %q has an option without a name", q.ID)
				}
				if names[o.Name] {
					return invalid("choice question %q has duplicate option %q", q.ID, o.Name)
				}
				names[o.Name] = true
			}
		case Score:
			if len(q.Levels) < minLevels || len(q.Levels) > maxLevels {
				return invalid("score question %q has %d levels; it needs %d to %d", q.ID, len(q.Levels), minLevels, maxLevels)
			}
			for i, l := range q.Levels {
				if l == "" {
					return invalid("score question %q: level %d is empty", q.ID, i)
				}
			}
		default:
			return invalid("question %q has unknown type %q", q.ID, q.Type)
		}
	}
	return nil
}
