package kit_test

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/kit/pkg/kit"
)

// ExampleRegisterDecisionProvider plugs an in-process decision model into
// Kit. A real backend (for example a local runtime) replaces the keyword
// rule.
func ExampleRegisterDecisionProvider() {
	rule := kit.DecisionModelFunc(func(_ context.Context, req kit.DecisionRequest) (*kit.DecisionResult, error) {
		p := 0.1
		if strings.Contains(req.State.Text, "refund") {
			p = 0.9
		}
		return &kit.DecisionResult{Answers: []kit.DecisionAnswer{
			{ID: "refund", Type: kit.DecisionBoolean, Probability: p},
		}}, nil
	})
	_ = kit.RegisterDecisionProvider("local", func(context.Context, *kit.ProviderConfig, string) (kit.DecisionModel, error) {
		return rule, nil
	})
	defer kit.UnregisterDecisionProvider("local")

	result, err := kit.Decide(context.Background(), kit.DecisionRequest{
		Model:     "local/keywords",
		State:     kit.DecisionState{Text: "I want a refund."},
		Questions: []kit.DecisionQuestion{kit.BooleanQuestion("refund", "Does the customer request a refund?")},
	})
	if err != nil {
		fmt.Println(err)
		return
	}
	refund, _ := result.Boolean("refund")
	fmt.Println(refund.Probability)
	// Output: 0.9
}
