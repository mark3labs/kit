package kit

import (
	"testing"
	"time"
)

func TestWithRetryPolicyCopiesValue(t *testing.T) {
	policy := RetryPolicy{MaxAttempts: 2, InitialDelay: time.Millisecond, MaxDelay: time.Second, MaxElapsed: time.Minute}
	option := WithRetryPolicy(policy)
	first, second := &Options{}, &Options{}
	option(first)
	option(second)
	if first.RetryPolicy == nil || *first.RetryPolicy != policy {
		t.Fatalf("policy = %v", first.RetryPolicy)
	}
	first.RetryPolicy.MaxAttempts = 1
	if second.RetryPolicy.MaxAttempts != 2 {
		t.Fatal("options share mutable policy")
	}
	if got := DefaultRetryPolicy(); got.MaxAttempts != 4 || got.InitialDelay != time.Second || got.MaxDelay != 30*time.Second || got.MaxElapsed != 2*time.Minute {
		t.Fatalf("defaults = %v", got)
	}
}

func TestRetryEventCarriesDelay(t *testing.T) {
	bus := newEventBus()
	var got RetryEvent
	bus.subscribe(func(e Event) {
		if r, ok := e.(RetryEvent); ok {
			got = r
		}
	})
	bus.emit(RetryEvent{Attempt: 2, Delay: 3 * time.Second})
	if got.Attempt != 2 || got.Delay != 3*time.Second {
		t.Fatalf("event = %v", got)
	}
}
