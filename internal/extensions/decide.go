package extensions

import "github.com/mark3labs/kit/internal/decision"

// ---------------------------------------------------------------------------
// Decision types (exposed to Yaegi — concrete structs)
// ---------------------------------------------------------------------------

// DecisionRequest configures a ctx.Decide call. Decision models answer typed
// questions about a state with probabilities instead of generated text.
type DecisionRequest struct {
	// Model is a decision model in "provider/model" format. Empty uses the
	// configured default (see ctx.GetDecisionModel). When both are empty,
	// Decide returns ErrNoDecisionModel.
	Model string

	// State is what the questions are about. Set Text, JSON or both.
	State DecisionState

	// Questions are the typed questions. Build them with BooleanQuestion,
	// ChoiceQuestion or ScoreQuestion.
	Questions []DecisionQuestion

	// ProviderOptions are sent as-is on wires that accept them (the Vercel
	// AI Gateway).
	ProviderOptions map[string]any
}

// DecisionQuestion is one typed question.
type DecisionQuestion = decision.Question

// DecisionQuestionType is the kind of a decision question.
type DecisionQuestionType = decision.QuestionType

// DecisionOption is one answer of a choice question.
type DecisionOption = decision.Option

// DecisionState is what the questions are about.
type DecisionState = decision.State

// DecisionImage is an image attached to a DecisionState.
type DecisionImage = decision.Image

// DecisionResult is the result of ctx.Decide. Read answers with Boolean,
// Choice or Score, which check the question type.
type DecisionResult = decision.Result

// DecisionAnswer is the normalized answer to one question.
type DecisionAnswer = decision.Answer

// DecisionProbability is the probability of one option or level.
type DecisionProbability = decision.Probability

// DecisionUsage is the token usage of one decision request.
type DecisionUsage = decision.Usage

// DecisionModelInfo describes one decision model of the model database.
type DecisionModelInfo struct {
	// Model is the "provider/model" string to pass in DecisionRequest.Model.
	Model    string
	Provider string
	ID       string
	Name     string
	// Wire is the protocol Kit uses. Empty when Kit cannot call the model.
	Wire string
	// AcceptsImages reports whether the catalog lists image input.
	AcceptsImages bool
	// ContextLimit is the context window in tokens. 0 when unknown.
	ContextLimit int
	// InputCostPerMTok is the input price in USD per million tokens.
	InputCostPerMTok float64
}

// Decision question types.
const (
	DecisionBoolean = decision.Boolean
	DecisionChoice  = decision.Choice
	DecisionScore   = decision.Score
)

// Decision errors. Extensions detect them with errors.Is.
var (
	// ErrNoDecisionModel indicates a request with no model and no
	// configured default. Tell the user to set decision-model.
	ErrNoDecisionModel = decision.ErrNoModel
	// ErrInvalidDecisionRequest indicates a request that fails validation.
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

// BooleanQuestion returns a question that is answered with P(true).
func BooleanQuestion(id, instructions string) DecisionQuestion {
	return DecisionQuestion{ID: id, Type: DecisionBoolean, Instructions: instructions}
}

// ChoiceQuestion returns a question that is answered with one of options.
// The options keep their order.
func ChoiceQuestion(id, instructions string, options ...DecisionOption) DecisionQuestion {
	return DecisionQuestion{ID: id, Type: DecisionChoice, Instructions: instructions, Options: options}
}

// ScoreQuestion returns a question that is answered with a position on the
// scale of levels, lowest first.
func ScoreQuestion(id, instructions string, levels ...string) DecisionQuestion {
	return DecisionQuestion{ID: id, Type: DecisionScore, Instructions: instructions, Levels: levels}
}
