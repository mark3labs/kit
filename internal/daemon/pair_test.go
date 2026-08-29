package daemon

import (
	"context"
	"testing"
)

// Regression test: an approval answer that lands after the pairing window
// expired must be rejected, even when it was already queued in the input
// channel when the deadline fired.
func TestPromptDecisionAfterWindowExpiry(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if !promptDecision(ctx, "y") {
		t.Fatal("live window + y should approve")
	}
	if !promptDecision(ctx, "yes") {
		t.Fatal("live window + yes should approve")
	}
	cancel()
	if promptDecision(ctx, "y") {
		t.Fatal("expired window must deny a queued y")
	}
	if promptDecision(context.Background(), "n") {
		t.Fatal("n should deny")
	}
	if promptDecision(context.Background(), "") {
		t.Fatal("empty answer should deny")
	}
}
