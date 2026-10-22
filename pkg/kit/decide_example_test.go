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
