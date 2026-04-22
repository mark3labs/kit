package kit

import (
	"context"
	"fmt"
	"sync"
	"testing"
)

// ---------------------------------------------------------------------------
// Hook registry tests
// ---------------------------------------------------------------------------

func TestHookRegistry_RegisterAndRun(t *testing.T) {
	hr := newHookRegistry[string, string]()

	hr.register(HookPriorityNormal, func(input string) *string {
		result := "handled: " + input
		return &result
	})

	got := hr.run("hello")
	if got == nil {
		t.Fatal("expected non-nil result")
		return
	}
	if *got != "handled: hello" {
		t.Errorf("expected 'handled: hello', got %q", *got)
	}
}

func TestHookRegistry_FirstNonNilWins(t *testing.T) {
	hr := newHookRegistry[string, string]()

	// First hook returns nil.
	hr.register(HookPriorityNormal, func(_ string) *string {
		return nil
	})
	// Second hook returns a result.
	hr.register(HookPriorityNormal, func(input string) *string {
		result := "second: " + input
		return &result
	})
	// Third hook would also return, but should never be reached.
	hr.register(HookPriorityNormal, func(input string) *string {
		result := "third: " + input
		return &result
	})

	got := hr.run("test")
	if got == nil {
		t.Fatal("expected non-nil result")
		return
	}
	if *got != "second: test" {
		t.Errorf("expected 'second: test', got %q", *got)
	}
}

func TestHookRegistry_PriorityOrdering(t *testing.T) {
	hr := newHookRegistry[string, string]()

	// Register in reverse priority order.
	hr.register(HookPriorityLow, func(_ string) *string {
		result := "low"
		return &result
	})
	hr.register(HookPriorityHigh, func(_ string) *string {
		result := "high"
		return &result
	})
	hr.register(HookPriorityNormal, func(_ string) *string {
		result := "normal"
		return &result
	})

	got := hr.run("x")
	if got == nil {
		t.Fatal("expected non-nil result")
		return
	}
	if *got != "high" {
		t.Errorf("expected 'high' (priority 0 runs first), got %q", *got)
	}
}

func TestHookRegistry_SamePriorityPreservesOrder(t *testing.T) {
	hr := newHookRegistry[int, string]()

	hr.register(HookPriorityNormal, func(n int) *string {
		result := "first"
		return &result
	})
	hr.register(HookPriorityNormal, func(n int) *string {
		result := "second"
		return &result
	})

	got := hr.run(0)
	if got == nil || *got != "first" {
		t.Errorf("expected 'first' (insertion order), got %v", got)
	}
}

func TestHookRegistry_Unregister(t *testing.T) {
	hr := newHookRegistry[string, string]()

	// Verify initial state (merged from TestHookRegistry_HasHooks).
	if hr.hasHooks() {
		t.Error("expected hasHooks to be false initially")
	}

	unregister := hr.register(HookPriorityNormal, func(input string) *string {
		result := "should be gone"
		return &result
	})

	if !hr.hasHooks() {
		t.Fatal("expected hasHooks to be true after registration")
	}

	unregister()

	if hr.hasHooks() {
		t.Fatal("expected hasHooks to be false after unregister")
	}

	got := hr.run("test")
	if got != nil {
		t.Errorf("expected nil after unregister, got %v", *got)
	}
}

func TestHookRegistry_NoHooksReturnsNil(t *testing.T) {
	hr := newHookRegistry[string, string]()

	got := hr.run("test")
	if got != nil {
		t.Errorf("expected nil when no hooks registered, got %v", *got)
	}
}

func TestHookRegistry_ConcurrentAccess(t *testing.T) {
	hr := newHookRegistry[int, int]()

	var wg sync.WaitGroup
	const n = 100

	// Concurrent registrations.
	for range n {
		wg.Go(func() {
			unsub := hr.register(HookPriorityNormal, func(x int) *int {
				result := x * 2
				return &result
			})
			// Immediately unregister half the time.
			unsub()
		})
	}

	// Concurrent runs while registrations are happening.
	for range n {
		wg.Go(func() {
			hr.run(42)
		})
	}

	wg.Wait()
}

// ---------------------------------------------------------------------------
// hookedTool tests
// ---------------------------------------------------------------------------

// mockAgentTool implements the AgentTool interface for testing.
type mockAgentTool struct {
	name  string
	runFn func(ctx context.Context, call LLMToolCall) (LLMToolResponse, error)
	popts LLMProviderOptions
}

func (m *mockAgentTool) Info() LLMToolInfo {
	return LLMToolInfo{Name: m.name, Description: "mock tool"}
}
func (m *mockAgentTool) ProviderOptions() LLMProviderOptions     { return m.popts }
func (m *mockAgentTool) SetProviderOptions(o LLMProviderOptions) { m.popts = o }
func (m *mockAgentTool) Run(ctx context.Context, call LLMToolCall) (LLMToolResponse, error) {
	if m.runFn != nil {
		return m.runFn(ctx, call)
	}
	return newLLMTextResponse("default output"), nil
}

// newEmptyHookedTool creates a hookedTool with empty hook registries and the given mock tool.
func newEmptyHookedTool(mock *mockAgentTool) *hookedTool {
	before := newHookRegistry[BeforeToolCallHook, BeforeToolCallResult]()
	after := newHookRegistry[AfterToolResultHook, AfterToolResultResult]()
	return &hookedTool{inner: mock, beforeToolCall: before, afterToolResult: after}
}

func TestHookedTool_Passthrough(t *testing.T) {
	mock := &mockAgentTool{
		name: "test_tool",
		runFn: func(_ context.Context, _ LLMToolCall) (LLMToolResponse, error) {
			return newLLMTextResponse("hello world"), nil
		},
	}

	ht := newEmptyHookedTool(mock)

	resp, err := ht.Run(context.Background(), LLMToolCall{Input: "{}"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "hello world" {
		t.Errorf("expected 'hello world', got %q", resp.Content)
	}
}

func TestHookedTool_BeforeToolCallBlock(t *testing.T) {
	before := newHookRegistry[BeforeToolCallHook, BeforeToolCallResult]()
	after := newHookRegistry[AfterToolResultHook, AfterToolResultResult]()

	toolRan := false
	mock := &mockAgentTool{
		name: "dangerous_tool",
		runFn: func(_ context.Context, _ LLMToolCall) (LLMToolResponse, error) {
			toolRan = true
			return newLLMTextResponse("should not run"), nil
		},
	}

	before.register(HookPriorityHigh, func(h BeforeToolCallHook) *BeforeToolCallResult {
		if h.ToolName == "dangerous_tool" {
			return &BeforeToolCallResult{Block: true, Reason: "too dangerous"}
		}
		return nil
	})

	ht := &hookedTool{inner: mock, beforeToolCall: before, afterToolResult: after}

	resp, err := ht.Run(context.Background(), LLMToolCall{Input: "{}"})
	if err == nil {
		t.Fatal("expected error from blocked tool")
	}
	if toolRan {
		t.Error("tool should not have run when blocked")
	}
	if resp.Content != "Error: too dangerous" {
		t.Errorf("expected block error message, got %q", resp.Content)
	}
}

func TestHookedTool_BeforeToolCallBlockDefaultReason(t *testing.T) {
	before := newHookRegistry[BeforeToolCallHook, BeforeToolCallResult]()
	after := newHookRegistry[AfterToolResultHook, AfterToolResultResult]()

	mock := &mockAgentTool{name: "tool"}
	before.register(HookPriorityNormal, func(_ BeforeToolCallHook) *BeforeToolCallResult {
		return &BeforeToolCallResult{Block: true}
	})

	ht := &hookedTool{inner: mock, beforeToolCall: before, afterToolResult: after}
	resp, _ := ht.Run(context.Background(), LLMToolCall{})
	if resp.Content != "Error: blocked by hook" {
		t.Errorf("expected default block reason, got %q", resp.Content)
	}
}

func TestHookedTool_AfterToolResultModify(t *testing.T) {
	before := newHookRegistry[BeforeToolCallHook, BeforeToolCallResult]()
	after := newHookRegistry[AfterToolResultHook, AfterToolResultResult]()

	mock := &mockAgentTool{
		name: "tool",
		runFn: func(_ context.Context, _ LLMToolCall) (LLMToolResponse, error) {
			return newLLMTextResponse("secret data"), nil
		},
	}

	after.register(HookPriorityNormal, func(h AfterToolResultHook) *AfterToolResultResult {
		redacted := "[REDACTED]"
		return &AfterToolResultResult{Result: &redacted}
	})

	ht := &hookedTool{inner: mock, beforeToolCall: before, afterToolResult: after}
	resp, err := ht.Run(context.Background(), LLMToolCall{Input: "{}"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "[REDACTED]" {
		t.Errorf("expected '[REDACTED]', got %q", resp.Content)
	}
}

func TestHookedTool_AfterToolResultModifyIsError(t *testing.T) {
	before := newHookRegistry[BeforeToolCallHook, BeforeToolCallResult]()
	after := newHookRegistry[AfterToolResultHook, AfterToolResultResult]()

	mock := &mockAgentTool{
		name: "tool",
		runFn: func(_ context.Context, _ LLMToolCall) (LLMToolResponse, error) {
			return newLLMTextResponse("ok"), nil
		},
	}

	isErr := true
	after.register(HookPriorityNormal, func(h AfterToolResultHook) *AfterToolResultResult {
		return &AfterToolResultResult{IsError: &isErr}
	})

	ht := &hookedTool{inner: mock, beforeToolCall: before, afterToolResult: after}
	resp, err := ht.Run(context.Background(), LLMToolCall{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.IsError {
		t.Error("expected IsError to be overridden to true")
	}
}

func TestHookedTool_HookReceivesToolInfo(t *testing.T) {
	before := newHookRegistry[BeforeToolCallHook, BeforeToolCallResult]()
	after := newHookRegistry[AfterToolResultHook, AfterToolResultResult]()

	mock := &mockAgentTool{
		name: "my_tool",
		runFn: func(_ context.Context, _ LLMToolCall) (LLMToolResponse, error) {
			return newLLMTextResponse("result"), nil
		},
	}

	var capturedBefore BeforeToolCallHook
	var capturedAfter AfterToolResultHook

	before.register(HookPriorityNormal, func(h BeforeToolCallHook) *BeforeToolCallResult {
		capturedBefore = h
		return nil // don't block
	})
	after.register(HookPriorityNormal, func(h AfterToolResultHook) *AfterToolResultResult {
		capturedAfter = h
		return nil // don't modify
	})

	ht := &hookedTool{inner: mock, beforeToolCall: before, afterToolResult: after}
	_, _ = ht.Run(context.Background(), LLMToolCall{Input: `{"key":"value"}`})

	if capturedBefore.ToolName != "my_tool" {
		t.Errorf("BeforeToolCall: expected tool name 'my_tool', got %q", capturedBefore.ToolName)
	}
	if capturedBefore.ToolArgs != `{"key":"value"}` {
		t.Errorf("BeforeToolCall: expected args, got %q", capturedBefore.ToolArgs)
	}
	if capturedAfter.ToolName != "my_tool" {
		t.Errorf("AfterToolResult: expected tool name 'my_tool', got %q", capturedAfter.ToolName)
	}
	if capturedAfter.Result != "result" {
		t.Errorf("AfterToolResult: expected result 'result', got %q", capturedAfter.Result)
	}
}

func TestHookedTool_InfoDelegates(t *testing.T) {
	mock := &mockAgentTool{name: "delegate_test"}
	ht := newEmptyHookedTool(mock)

	if ht.Info().Name != "delegate_test" {
		t.Errorf("expected Info() to delegate to inner tool")
	}
}

// ---------------------------------------------------------------------------
// hookToolWrapper tests
// ---------------------------------------------------------------------------

func TestHookToolWrapper(t *testing.T) {
	before := newHookRegistry[BeforeToolCallHook, BeforeToolCallResult]()
	after := newHookRegistry[AfterToolResultHook, AfterToolResultResult]()

	wrapper := hookToolWrapper(before, after)

	tools := []Tool{
		&mockAgentTool{name: "tool_a"},
		&mockAgentTool{name: "tool_b"},
	}

	wrapped := wrapper(tools)
	if len(wrapped) != 2 {
		t.Fatalf("expected 2 wrapped tools, got %d", len(wrapped))
	}

	// Verify tools are wrapped (different pointer than original).
	for i, wt := range wrapped {
		if _, ok := wt.(*hookedTool); !ok {
			t.Errorf("tool %d: expected *hookedTool, got %T", i, wt)
		}
		if wt.Info().Name != tools[i].Info().Name {
			t.Errorf("tool %d: expected name %q, got %q", i, tools[i].Info().Name, wt.Info().Name)
		}
	}

	// Hooks registered after wrapping should still work.
	var blocked bool
	before.register(HookPriorityNormal, func(h BeforeToolCallHook) *BeforeToolCallResult {
		blocked = true
		return &BeforeToolCallResult{Block: true, Reason: "late hook"}
	})

	_, err := wrapped[0].Run(context.Background(), LLMToolCall{})
	if err == nil {
		t.Error("expected error from late-registered blocking hook")
	}
	if !blocked {
		t.Error("late-registered hook should have been called")
	}
}

// ---------------------------------------------------------------------------
// Hook type tests (BeforeTurn, AfterTurn)
// ---------------------------------------------------------------------------

func TestBeforeTurnHook_PromptOverride(t *testing.T) {
	hr := newHookRegistry[BeforeTurnHook, BeforeTurnResult]()

	override := "modified prompt"
	hr.register(HookPriorityNormal, func(h BeforeTurnHook) *BeforeTurnResult {
		return &BeforeTurnResult{Prompt: &override}
	})

	result := hr.run(BeforeTurnHook{Prompt: "original"})
	if result == nil {
		t.Fatal("expected non-nil result")
		return
	}
	if result.Prompt == nil || *result.Prompt != "modified prompt" {
		t.Errorf("expected prompt override, got %v", result.Prompt)
	}
}

func TestBeforeTurnHook_InjectSystemAndContext(t *testing.T) {
	hr := newHookRegistry[BeforeTurnHook, BeforeTurnResult]()

	sysPr := "be concise"
	ctx := "project context here"
	hr.register(HookPriorityNormal, func(h BeforeTurnHook) *BeforeTurnResult {
		return &BeforeTurnResult{
			SystemPrompt: &sysPr,
			InjectText:   &ctx,
		}
	})

	result := hr.run(BeforeTurnHook{Prompt: "hello"})
	if result == nil {
		t.Fatal("expected non-nil result")
		return
	}
	if result.SystemPrompt == nil || *result.SystemPrompt != "be concise" {
		t.Errorf("expected system prompt injection")
	}
	if result.InjectText == nil || *result.InjectText != "project context here" {
		t.Errorf("expected context injection")
	}
}

func TestAfterTurnHook_ObservationOnly(t *testing.T) {
	hr := newHookRegistry[AfterTurnHook, AfterTurnResult]()

	var captured AfterTurnHook
	hr.register(HookPriorityNormal, func(h AfterTurnHook) *AfterTurnResult {
		captured = h
		return nil // observation only
	})

	hr.run(AfterTurnHook{Response: "agent replied"})
	if captured.Response != "agent replied" {
		t.Errorf("expected captured response, got %q", captured.Response)
	}
}

func TestAfterTurnHook_WithError(t *testing.T) {
	hr := newHookRegistry[AfterTurnHook, AfterTurnResult]()

	var captured AfterTurnHook
	hr.register(HookPriorityNormal, func(h AfterTurnHook) *AfterTurnResult {
		captured = h
		return nil
	})

	testErr := fmt.Errorf("generation failed")
	hr.run(AfterTurnHook{Error: testErr})
	if captured.Error != testErr {
		t.Errorf("expected captured error, got %v", captured.Error)
	}
}

// ---------------------------------------------------------------------------
// Priority constants sanity check
// ---------------------------------------------------------------------------

func TestHookPriorityOrdering(t *testing.T) {
	if HookPriorityHigh >= HookPriorityNormal {
		t.Error("HookPriorityHigh should be less than HookPriorityNormal")
	}
	if HookPriorityNormal >= HookPriorityLow {
		t.Error("HookPriorityNormal should be less than HookPriorityLow")
	}
}

// ---------------------------------------------------------------------------
// Kit method compilation tests (verify API surface exists)
// ---------------------------------------------------------------------------

func TestKit_HookMethodsExist(t *testing.T) {
	k := &Kit{
		events:          newEventBus(),
		beforeToolCall:  newHookRegistry[BeforeToolCallHook, BeforeToolCallResult](),
		afterToolResult: newHookRegistry[AfterToolResultHook, AfterToolResultResult](),
		beforeTurn:      newHookRegistry[BeforeTurnHook, BeforeTurnResult](),
		afterTurn:       newHookRegistry[AfterTurnHook, AfterTurnResult](),
	}

	// Verify all hook registration methods return unsubscribe functions.
	u1 := k.OnBeforeToolCall(HookPriorityNormal, func(_ BeforeToolCallHook) *BeforeToolCallResult {
		return nil
	})
	u2 := k.OnAfterToolResult(HookPriorityNormal, func(_ AfterToolResultHook) *AfterToolResultResult {
		return nil
	})
	u3 := k.OnBeforeTurn(HookPriorityNormal, func(_ BeforeTurnHook) *BeforeTurnResult {
		return nil
	})
	u4 := k.OnAfterTurn(HookPriorityNormal, func(_ AfterTurnHook) {})

	// All should be callable.
	u1()
	u2()
	u3()
	u4()
}

// TestPrepareStepHookRegistry verifies registration and execution of PrepareStep hooks.
func TestPrepareStepHookRegistry(t *testing.T) {
	hr := newHookRegistry[PrepareStepHook, PrepareStepResult]()

	// Register a hook that appends a message.
	hr.register(HookPriorityNormal, func(h PrepareStepHook) *PrepareStepResult {
		if h.StepNumber == 0 {
			// On step 0, prepend a system message.
			newMsgs := make([]LLMMessage, 0, len(h.Messages)+1)
			newMsgs = append(newMsgs, NewLLMSystemMessage("injected"))
			newMsgs = append(newMsgs, h.Messages...)
			return &PrepareStepResult{Messages: newMsgs}
		}
		return nil // No modification for other steps.
	})

	// Test step 0 — should modify messages.
	input := PrepareStepHook{
		StepNumber: 0,
		Messages:   []LLMMessage{NewLLMUserMessage("hello")},
	}
	result := hr.run(input)
	if result == nil {
		t.Fatal("expected non-nil result for step 0")
	}
	if len(result.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(result.Messages))
	}
	if result.Messages[0].Role != LLMRoleSystem {
		t.Errorf("expected system message first, got role %q", result.Messages[0].Role)
	}

	// Test step 1 — should return nil (no modification).
	input.StepNumber = 1
	result = hr.run(input)
	if result != nil {
		t.Errorf("expected nil result for step 1, got %+v", result)
	}
}

// TestPrepareStepHookPriority verifies that PrepareStep hooks respect priority ordering.
func TestPrepareStepHookPriority(t *testing.T) {
	hr := newHookRegistry[PrepareStepHook, PrepareStepResult]()

	var order []string

	// Low priority — should run second.
	hr.register(HookPriorityLow, func(_ PrepareStepHook) *PrepareStepResult {
		order = append(order, "low")
		return nil
	})

	// High priority — should run first and win.
	hr.register(HookPriorityHigh, func(h PrepareStepHook) *PrepareStepResult {
		order = append(order, "high")
		return &PrepareStepResult{Messages: h.Messages}
	})

	input := PrepareStepHook{
		StepNumber: 0,
		Messages:   []LLMMessage{NewLLMUserMessage("test")},
	}
	result := hr.run(input)

	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if len(order) != 1 || order[0] != "high" {
		t.Errorf("expected [high] (first non-nil wins), got %v", order)
	}
}
