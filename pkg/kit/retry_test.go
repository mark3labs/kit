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

func TestStreamCollectorDiscardStep(t *testing.T) {
	c := &streamCollector{}
	c.markStep()
	c.add(StreamEvent{Kind: StreamEventTextDelta, Text: "step 0"})
	c.markStep()
	c.add(StreamEvent{Kind: StreamEventTextDelta, Text: "partial"})
	c.add(StreamEvent{Kind: StreamEventReasoningDelta, Reasoning: "r"})
	c.discardStep()
	c.markStep()
	c.add(StreamEvent{Kind: StreamEventTextDelta, Text: "full"})
	got := c.drain()
	if len(got) != 2 || got[0].Text != "step 0" || got[1].Text != "full" {
		t.Fatalf("events = %+v", got)
	}
	var nilCollector *streamCollector
	nilCollector.markStep()
	nilCollector.discardStep()
}

func TestStepResumeEventType(t *testing.T) {
	if (StepResumeEvent{}).EventType() != EventStepResume {
		t.Fatal("wrong event type")
	}
	if got := DefaultRetryPolicy(); got.MaxResumes != 5 || got.ResumeInitialDelay != 5*time.Second || got.ResumeMaxDelay != 80*time.Second {
		t.Fatalf("resume defaults = %+v", got)
	}
}
