package extensions

import "testing"

func TestRunner_EmitLLMUsage(t *testing.T) {
	var got LLMUsageEvent
	var called bool
	ext := makeHandlerExt("llmusage.go", map[EventType][]HandlerFunc{
		LLMUsage: {
			func(e Event, c Context) Result {
				got = e.(LLMUsageEvent)
				called = true
				return nil
			},
		},
	})

	r := makeRunner(ext)
	_, err := r.Emit(LLMUsageEvent{
		InputTokens:  100,
		OutputTokens: 50,
		Cost:         0.0012,
		Model:        "claude-sonnet-4-5-20250929",
		Provider:     "anthropic",
		StepNumber:   2,
		FinishReason: "tool_calls",
	})
	if err != nil {
		t.Fatalf("emit: %v", err)
	}
	if !called {
		t.Fatal("expected LLMUsage handler to be called")
	}
	if got.InputTokens != 100 || got.OutputTokens != 50 {
		t.Errorf("token fields not propagated: %+v", got)
	}
	if got.Cost != 0.0012 {
		t.Errorf("cost not propagated, got %v", got.Cost)
	}
	if got.Model != "claude-sonnet-4-5-20250929" || got.Provider != "anthropic" {
		t.Errorf("model/provider not propagated: %+v", got)
	}
	if got.StepNumber != 2 || got.FinishReason != "tool_calls" {
		t.Errorf("step/finish reason not propagated: %+v", got)
	}
}

func TestRunner_LLMUsageRegisteredViaTestAPI(t *testing.T) {
	// Verify NewTestAPI wires up onLLMUsage so the extension can call
	// api.OnLLMUsage during Init.
	ext := &LoadedExtension{Handlers: make(map[EventType][]HandlerFunc)}
	api := NewTestAPI(ext)

	var calls int
	api.OnLLMUsage(func(e LLMUsageEvent, c Context) {
		calls++
	})

	if len(ext.Handlers[LLMUsage]) != 1 {
		t.Fatalf("expected 1 LLMUsage handler registered, got %d", len(ext.Handlers[LLMUsage]))
	}

	r := makeRunner(*ext)
	_, _ = r.Emit(LLMUsageEvent{InputTokens: 1})
	if calls != 1 {
		t.Errorf("expected handler called once, got %d", calls)
	}
}

func TestAgentEndEvent_EnrichedFields(t *testing.T) {
	// Verify the enriched event carries through Emit without mangling.
	var got AgentEndEvent
	ext := makeHandlerExt("end.go", map[EventType][]HandlerFunc{
		AgentEnd: {
			func(e Event, c Context) Result {
				got = e.(AgentEndEvent)
				return nil
			},
		},
	})
	r := makeRunner(ext)
	_, err := r.Emit(AgentEndEvent{
		Response:              "done",
		StopReason:            "completed",
		ToolCallCount:         3,
		ToolNames:             []string{"bash", "read", "bash"},
		LLMCallCount:          4,
		InputTokensDelta:      1500,
		OutputTokensDelta:     400,
		CacheReadTokensDelta:  200,
		CacheWriteTokensDelta: 100,
		CostDelta:             0.0123,
		DurationMs:            2500,
	})
	if err != nil {
		t.Fatalf("emit: %v", err)
	}
	if got.ToolCallCount != 3 {
		t.Errorf("ToolCallCount: got %d want 3", got.ToolCallCount)
	}
	if len(got.ToolNames) != 3 || got.ToolNames[0] != "bash" || got.ToolNames[2] != "bash" {
		t.Errorf("ToolNames: %v", got.ToolNames)
	}
	if got.LLMCallCount != 4 {
		t.Errorf("LLMCallCount: got %d want 4", got.LLMCallCount)
	}
	if got.InputTokensDelta != 1500 || got.OutputTokensDelta != 400 {
		t.Errorf("token deltas: %+v", got)
	}
	if got.CacheReadTokensDelta != 200 || got.CacheWriteTokensDelta != 100 {
		t.Errorf("cache deltas: %+v", got)
	}
	if got.CostDelta != 0.0123 {
		t.Errorf("CostDelta: got %v", got.CostDelta)
	}
	if got.DurationMs != 2500 {
		t.Errorf("DurationMs: got %d", got.DurationMs)
	}
}
