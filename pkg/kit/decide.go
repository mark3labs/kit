package kit

import (
	"context"
	"errors"
	"fmt"

	"github.com/mark3labs/kit/internal/decision"
	"github.com/mark3labs/kit/internal/models"
)

// Decision models answer typed questions about a state with probabilities
// instead of generated text. Use them to classify, route, score and gate
// actions. Kit finds them in the model database (models.dev entries of type
// "decision") and selects the wire protocol from the same data, so one
// request shape works for every supported host.

// DecisionQuestionType is the kind of a decision question.
type DecisionQuestionType = decision.QuestionType

// Decision question types.
const (
	// DecisionBoolean questions are answered with P(true).
	DecisionBoolean = decision.Boolean
	// DecisionChoice questions are answered with one option from a set.
	DecisionChoice = decision.Choice
	// DecisionScore questions are answered with a position on an ordered
	// scale.
	DecisionScore = decision.Score
)

// DecisionQuestion is one typed question. Build it with BooleanQuestion,
// ChoiceQuestion or ScoreQuestion.
type DecisionQuestion = decision.Question

// DecisionOption is one answer of a choice question. An empty Description
// lets the name describe itself.
type DecisionOption = decision.Option

// DecisionState is what the questions are about. Set Text, JSON or both,
// and optionally Images.
type DecisionState = decision.State

// DecisionImage is an image attached to a DecisionState. Models whose
// catalog entry lists image input accept PNG and JPEG; Cloudflare Workers AI
// and Ollama also accept WebP.
type DecisionImage = decision.Image

// DecisionAnswer is the normalized answer to one question.
type DecisionAnswer = decision.Answer

// DecisionProbability is the probability of one option or level.
type DecisionProbability = decision.Probability

// DecisionUsage is the token usage of one decision request.
type DecisionUsage = decision.Usage

// DecisionResult is the result of Decide. Read answers with Boolean, Choice
// or Score, which check the question type.
type DecisionResult = decision.Result

// DecisionAPIError is an HTTP error from a decision provider.
type DecisionAPIError = decision.APIError

// Decision errors.
var (
	// ErrNotDecisionModel indicates a model that is not a decision model.
	ErrNotDecisionModel = models.ErrNotDecisionModel
	// ErrDecisionModelNotChat indicates a decision model used as a chat
	// model (for example with SetModel).
	ErrDecisionModelNotChat = models.ErrDecisionModelNotChat
	// ErrUnknownDecisionWire indicates a decision model whose protocol Kit
	// does not know.
	ErrUnknownDecisionWire = models.ErrUnknownDecisionWire
	// ErrInvalidDecisionRequest indicates a request that fails validation.
	// Nothing is sent.
	ErrInvalidDecisionRequest = decision.ErrInvalidRequest
	// ErrInvalidDecisionResponse indicates answers that do not match the
	// questions.
	ErrInvalidDecisionResponse = decision.ErrInvalidResponse
	// ErrDecisionRefused indicates that the model refused a question.
	ErrDecisionRefused = decision.ErrRefused
	// ErrNoDecisionAnswer indicates a question ID with no answer.
	ErrNoDecisionAnswer = decision.ErrNoAnswer
	// ErrWrongDecisionAnswerType indicates an accessor for a different
	// question type.
	ErrWrongDecisionAnswerType = decision.ErrWrongAnswerType
)

// DecisionRequest is one decision request.
type DecisionRequest struct {
	// Model is a decision model in "provider/model" format, for example
	// "openrouter/typesafe/jev-1.14", "vercel/typesafe-ai/jev",
	// "cloudflare-workers-ai/@cf/cloudflare/clef-flash" or
	// "ollama/pplx-decider". See GetDecisionModels.
	Model     string
	State     DecisionState
	Questions []DecisionQuestion
	// ProviderOptions are sent as-is on wires that accept them (the Vercel
	// AI Gateway), for example {"gateway": {"zeroDataRetention": true}}.
	ProviderOptions map[string]any
}

// BooleanQuestion returns a question that is answered with P(true).
func BooleanQuestion(id, instructions string) DecisionQuestion {
	return DecisionQuestion{ID: id, Type: DecisionBoolean, Instructions: instructions}
}

// ChoiceQuestion returns a question that is answered with one of options.
// The options keep their order on the wire.
func ChoiceQuestion(id, instructions string, options ...DecisionOption) DecisionQuestion {
	return DecisionQuestion{ID: id, Type: DecisionChoice, Instructions: instructions, Options: options}
}

// ScoreQuestion returns a question that is answered with a position on the
// scale of levels, lowest first.
func ScoreQuestion(id, instructions string, levels ...string) DecisionQuestion {
	return DecisionQuestion{ID: id, Type: DecisionScore, Instructions: instructions, Levels: levels}
}

// DecisionModelInfo describes one decision model of the model database.
type DecisionModelInfo struct {
	// Model is the "provider/model" string to pass to Decide.
	Model    string
	Provider string
	ID       string
	Name     string
	// Wire is the protocol Kit uses: "systemone", "workers-ai" or
	// "gateway". Empty when Kit cannot call the model.
	Wire string
	// AcceptsImages reports whether the catalog lists image input.
	AcceptsImages bool
	// ContextLimit is the context window in tokens. 0 when unknown.
	ContextLimit int
	// InputCostPerMTok is the input price in USD per million tokens.
	// Decision models usually do not charge for output.
	InputCostPerMTok float64
}

// GetDecisionModels lists the decision models of the model database,
// sorted by provider and model ID. Run 'kit update-models' to refresh the
// database.
func GetDecisionModels() []DecisionModelInfo {
	registry := models.GetGlobalRegistry()
	entries := registry.DecisionModels()
	out := make([]DecisionModelInfo, 0, len(entries))
	for _, e := range entries {
		wire, _ := registry.DecisionWireFor(e.Provider, e.ID)
		images, _ := e.Info.SupportsImageInput()
		out = append(out, DecisionModelInfo{
			Model:            e.Provider + "/" + e.ID,
			Provider:         e.Provider,
			ID:               e.ID,
			Name:             e.Info.Name,
			Wire:             string(wire),
			AcceptsImages:    images,
			ContextLimit:     e.Info.Limit.Context,
			InputCostPerMTok: e.Info.Cost.Input,
		})
	}
	return out
}

// Decide sends req to a decision model with the default settings: API keys
// from stored credentials or the environment, and no transport wrapper. Use
// Kit.Decide to apply a Kit's HTTP transport and endpoint overrides.
func Decide(ctx context.Context, req DecisionRequest) (*DecisionResult, error) {
	return decide(ctx, &models.ProviderConfig{ModelString: req.Model}, req)
}

// Decide sends req to a decision model. It uses the Kit's HTTP transport
// (WithHTTPTransport), TLS setting and, when they apply to req.Model, its
// provider URL and API key overrides. The Kit's chat model does not change.
//
// Decide validates the request before it sends it, and retries rate limits
// and transient provider failures twice. Errors from the provider are
// classified like other provider errors (see ClassifyProviderError).
func (m *Kit) Decide(ctx context.Context, req DecisionRequest) (*DecisionResult, error) {
	cfg := &models.ProviderConfig{
		ModelString:   req.Model,
		TLSSkipVerify: m.v.GetBool("tls-skip-verify"),
		ConfigStore:   m.v,
		HTTPTransport: m.httpTransport,
	}
	m.applyEffectiveProviderSettings(cfg, req.Model)
	return decide(ctx, cfg, req)
}

func decide(ctx context.Context, cfg *models.ProviderConfig, req DecisionRequest) (*DecisionResult, error) {
	if req.Model == "" {
		return nil, fmt.Errorf("%w: no model", ErrInvalidDecisionRequest)
	}
	ep, err := models.ResolveDecisionEndpoint(cfg)
	if err != nil {
		return nil, err
	}
	res, err := decision.Decide(ctx, ep, decision.Request{
		State:           req.State,
		Questions:       req.Questions,
		ProviderOptions: req.ProviderOptions,
	})
	if err != nil {
		err = fmt.Errorf("decide with %s: %w", req.Model, err)
		if errors.Is(err, ErrInvalidDecisionRequest) || errors.Is(err, ErrInvalidDecisionResponse) {
			return nil, err
		}
		return nil, ClassifyProviderError(err)
	}
	return res, nil
}
