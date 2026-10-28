
# Decision API

Decision models classify, route, score and provide probabilistic gates. They return typed answers with probabilities, not chat or completion text. They are not security authorization guarantees. Enforce access rules in host code.

## Discover models

`GetDecisionModels` returns sorted `DecisionModelInfo` entries. `Model` is the `provider/model` identifier; `Provider`, `ID` and `Name` identify the catalog entry. `Wire` is `systemone`, `workers-ai`, or `gateway`; an empty wire means Kit does not know how to call that entry. `AcceptsImages` reports catalog image input, `ContextLimit` is tokens (zero means unknown), and `InputCostPerMTok` is USD per million input tokens. Catalog presence does not guarantee credentials, remote availability or a supported wire. Use `kit update-models` to refresh the catalog; do not hard-code a model count or price.

## Standalone or instance

`Decide` (`kit.Decide(ctx, req)`) needs no Kit instance. It resolves stored credentials or provider environment variables with default transport settings, without an instance transport wrapper. `Kit.Decide` uses the instance HTTP transport and TLS setting, and endpoint/API-key overrides when they apply to the requested model. It does not switch the chat model or append a normal agent turn to its session. See [authentication](/sdk/authentication) and [transport tracing](/sdk/callbacks#provider-request-tracing).

When `DecisionRequest.Model` is empty, `Kit.Decide` uses the default decision model: `Options.DecisionModel` / `WithDecisionModel`, the `--decision-model` flag, `KIT_DECISION_MODEL` or `decision-model` in `.kit.yml`. `Kit.GetDecisionModel` returns it, and Kit reads it on each call. When no model is set, `Decide` returns an error that wraps `ErrNoDecisionModel` and `ErrInvalidDecisionRequest`. The standalone `kit.Decide` reads no config, so its request must name a model. At startup Kit writes a warning to stderr when the configured value is not a decision model that it can call.

A decision model cannot serve as an agent chat model. Selecting one with `SetModel` returns `ErrDecisionModelNotChat`.

## Build a request

`DecisionRequest` requires a decision `Model`, nonempty `State`, and ordered `Questions` with unique, nonempty IDs. `ProviderOptions` passes wire-dependent options through where supported (currently the gateway wire); it is not a universal provider configuration contract.

`DecisionState` accepts `Text`, JSON-encodable `JSON`, or both, with optional `Images`. Images can also supply the state alone. `DecisionImage` contains `MediaType`, nonempty `Data` bytes and optional `Filename`. PNG and JPEG are supported on image-capable models; Cloudflare and Ollama also accept WebP. Cloudflare permits at most four images. Jev can reject images smaller than about 300 pixels per side. Check model capability first. Image representation differs by wire: multipart assets, data URLs, inline base64 or gateway state parts; identical wire bodies are not promised.

`DecisionQuestionType` has `DecisionBoolean`, `DecisionChoice` and `DecisionScore`. Build a `DecisionQuestion` with `BooleanQuestion`, `ChoiceQuestion` and `ScoreQuestion`. Boolean questions need instructions or `True`/`False` descriptions. A choice has 2–255 uniquely named `DecisionOption` values, with optional descriptions. Scores have 2–10 nonempty ordered levels, lowest first. Non-gateway wires accept at most 64 questions. Validation occurs before sending.

The following complete example is also compile-checked in `pkg/kit/decide_example_test.go`. Normal tests do not execute it or make provider requests.

```go
package kit_test

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/mark3labs/kit/pkg/kit"
)

// ExampleDecide demonstrates all three question kinds. It needs provider
// credentials and network access, so normal tests compile but do not run it.
func ExampleDecide() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	result, err := kit.Decide(ctx, kit.DecisionRequest{
		Model: "openrouter/typesafe/jev-1.14", // Discover current models with GetDecisionModels.
		State: kit.DecisionState{Text: "My card was charged twice for one order."},
		Questions: []kit.DecisionQuestion{
			kit.BooleanQuestion("refund", "Does the customer request a refund?"),
			kit.ChoiceQuestion("team", "Which team should handle this ticket?",
				kit.DecisionOption{Name: "billing", Description: "Payments and refunds"},
				kit.DecisionOption{Name: "technical"}),
			kit.ScoreQuestion("urgency", "How urgent is this ticket?", "low", "medium", "high"),
		},
	})
	if err != nil {
		if apiErr, ok := (errors.AsType[*kit.DecisionAPIError])(err); ok {
			fmt.Println("Provider HTTP status:", apiErr.StatusCode)
		} else if errors.Is(err, kit.ErrInvalidDecisionRequest) {
			fmt.Println("Correct the request before sending again")
		}
		return
	}
	refund, err := result.Boolean("refund")
	if err != nil {
		fmt.Println("Refund answer unavailable")
		return
	}
	team, err := result.Choice("team")
	if err != nil {
		fmt.Println("Team answer unavailable")
		return
	}
	urgency, err := result.Score("urgency")
	if err != nil {
		fmt.Println("Urgency answer unavailable")
		return
	}
	fmt.Println(refund.Probability, team.Choice, urgency.Score)
	fmt.Println("Input tokens:", result.Usage.InputTokens)
}
```

## Read answers by ID

`DecisionResult` contains ordered `Answers`, serving `Model`, `Usage` and provider-specific `Metadata`. Use question IDs, not positions, to retrieve answers. `Answer(id string) (DecisionAnswer, bool)` reports presence. `Boolean(id string) (DecisionAnswer, error)`, `Choice(id string) (DecisionAnswer, error)` and `Score(id string) (DecisionAnswer, error)` also check type and refusal. Always check the error before using a zero value.

`DecisionAnswer` contains `Probability`, which is P(true) only for boolean answers and remains zero for choice and score answers. Use `Prob(name)` for a named option or level. `Choice` is the selected name. `Score` is the expected zero-based level, possibly fractional; `Level` is the most probable level when a distribution exists (otherwise rounded score). `Distribution` holds `DecisionProbability` values in question order. `Prob(name)` returns `(float64, bool)`; absence is not zero probability. `Confidence`, when present, measures distribution concentration, not probability of correctness. `Refused` marks refusal. `DecisionUsage` contains input and output token counts. Do not assume all providers return distributions, confidence, model names, usage detail or metadata.

## Custom decision providers

An application can supply its own decision model, for example an in-process local runtime, a test double or a proxy. Register a `DecisionProviderFactory` process-wide with `RegisterDecisionProvider`, or for one Kit with `Options.DecisionProviders` (`WithDecisionProvider`). Requests for `name/<model>` then go to the `DecisionModel` that the factory returns, and not to the model database. Instance factories win over process-wide ones. Subagents inherit instance factories. Extension `ctx.Decide` uses them too, because it calls `Kit.Decide`.

```go
host, err := kit.New(ctx, &kit.Options{
    DecisionModel: "local/my-decider",
    DecisionProviders: map[string]kit.DecisionProviderFactory{
        "local": func(ctx context.Context, cfg *kit.ProviderConfig, model string) (kit.DecisionModel, error) {
            return myBackend.DecisionModel(ctx, model) // cache loaded resources
        },
    },
})
```

Kit calls the factory for each request and does not close the model. Kit validates the request before the call (the same question rules as the wires, but no wire limit on the question count or image types). After the call, Kit checks the result: each question needs exactly one answer with its ID and type, and an unknown or duplicate answer is `ErrInvalidDecisionResponse`. Kit puts answers and distributions in question order and sets `Level` of score answers. Use `DecisionModelFunc` to adapt a plain function. Kit does not retry a custom model.

## Errors and cancellation

Use `errors.Is` for wrapped sentinels:

| Sentinel | Action |
| --- | --- |
| `ErrNotDecisionModel` | Select a decision catalog entry, not a chat model. |
| `ErrDecisionModelNotChat` | Use Decide instead of chat/SetModel. |
| `ErrUnknownDecisionWire` | Select an entry with a supported wire. |
| `ErrInvalidDecisionRequest` | Correct validation failures; nothing was sent. |
| `ErrNoDecisionModel` | Set `DecisionRequest.Model` or the `decision-model` config; nothing was sent. |
| `ErrInvalidDecisionResponse` | Treat mismatched, missing or malformed answers as failure. |
| `ErrDecisionRefused` | A typed accessor found a refused answer; use an explicit host fallback. |
| `ErrNoDecisionAnswer` | Check the question ID. |
| `ErrWrongDecisionAnswerType` | Use the accessor matching the question kind. |

For example, `if errors.Is(err, kit.ErrDecisionRefused) { /* host fallback */ }` handles accessor refusal. The example uses `errors.AsType` to inspect the `StatusCode` of `DecisionAPIError`. Its `Message` and raw `Body` can contain sensitive provider data: do not log them or request bodies/credentials by default. Provider errors are also classified through `ClassifyProviderError`.

Supply a context deadline. Each HTTP attempt is bounded to two minutes. There are at most three attempts (initial plus two retries), for network/read failures and HTTP 429, 529, 500, 502, 503 and 504. Backoff is one then two seconds; a positive integer-seconds `Retry-After` replaces it, capped at 30 seconds. Parent cancellation stops requests and waits. Validation and response decoding are not retried. A retry can incur additional provider cost; do not assume exactly-once remote processing.

These limits are independent of agent-loop `RetryPolicy` and step resume. Decision calls do not resume chat steps or execute local tools. See [agent recovery](/sdk/sessions#model-request-retries) for that separate contract, and the [API index](/sdk/api-reference) for the full public surface.
