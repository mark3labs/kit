package kit

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

// ---------------------------------------------------------------------------
// Priority
// ---------------------------------------------------------------------------

// HookPriority controls execution order of hooks. Lower values run first.
type HookPriority int

const (
	// HookPriorityHigh runs before normal hooks.
	HookPriorityHigh HookPriority = 0
	// HookPriorityNormal is the default priority.
	HookPriorityNormal HookPriority = 50
	// HookPriorityLow runs after normal hooks.
	HookPriorityLow HookPriority = 100
)

// ---------------------------------------------------------------------------
// Hook input/result types
// ---------------------------------------------------------------------------

// BeforeToolCallHook is the input for hooks that fire before a tool executes.
type BeforeToolCallHook struct {
	ToolCallID string
	ToolName   string
	ToolArgs   string
}

// BeforeToolCallResult controls whether the tool call proceeds.
type BeforeToolCallResult struct {
	Block  bool   // true prevents the tool from running
	Reason string // human-readable reason for blocking
}

// AfterToolResultHook is the input for hooks that fire after a tool executes.
type AfterToolResultHook struct {
	ToolCallID string
	ToolName   string
	ToolArgs   string
	Result     string
	IsError    bool
}

// AfterToolResultResult can modify the tool's output before it reaches the LLM.
type AfterToolResultResult struct {
	Result  *string // non-nil overrides the result text
	IsError *bool   // non-nil overrides the error flag
}

// BeforeTurnHook is the input for hooks that fire before a prompt turn.
type BeforeTurnHook struct {
	Prompt string
}

// BeforeTurnResult can modify the prompt, inject system messages, or add context.
type BeforeTurnResult struct {
	Prompt       *string // override prompt text in the user message
	SystemPrompt *string // prepend a system message
	InjectText   *string // prepend a user context message
}

// AfterTurnHook is the input for hooks that fire after a prompt turn completes.
type AfterTurnHook struct {
	Response string
	Error    error
}

// AfterTurnResult is a placeholder — after-turn hooks are observation-only.
type AfterTurnResult struct{}

// ContextPrepareHook is the input for hooks that fire after the context window
// is assembled from the session tree (including compaction) and before the
// messages are sent to the LLM. Hooks can filter, reorder, or inject messages.
type ContextPrepareHook struct {
	// Messages is the current context as LLM message objects.
	Messages []LLMMessage
}

// ContextPrepareResult can replace the context window.
type ContextPrepareResult struct {
	// Messages replaces the entire context window. If nil, the original
	// messages are used.
	Messages []LLMMessage
}

// BeforeCompactHook is the input for hooks that fire before compaction runs.
type BeforeCompactHook struct {
	// EstimatedTokens is the estimated token count of the conversation.
	EstimatedTokens int
	// ContextLimit is the model's context window size in tokens.
	ContextLimit int
	// UsagePercent is the fraction of context used (0.0–1.0).
	UsagePercent float64
	// MessageCount is the number of messages in the conversation.
	MessageCount int
	// IsAutomatic is true when compaction was triggered automatically.
	IsAutomatic bool
}

// BeforeCompactResult controls whether compaction proceeds. Extensions can
// cancel compaction or provide a custom summary that replaces the default
// LLM-generated one.
type BeforeCompactResult struct {
	// Cancel, when true, prevents compaction from proceeding.
	Cancel bool
	// Reason is a human-readable explanation when Cancel is true.
	Reason string
	// Summary, when non-empty, replaces the default LLM-generated summary.
	// The extension is responsible for generating a useful summary.
	// Ignored when Cancel is true.
	Summary string
}

// PrepareStepHook is the input for hooks that fire between steps within a
// multi-step agent turn, with full message replacement capability. This is
// the most powerful interception point — it fires after the existing steering
// logic (if any) and before the messages are sent to the LLM.
//
// Use cases:
//   - Transforming tool results (e.g. converting image tool results to FilePart
//     user messages for vision models that don't support media in tool results)
//   - Dynamic tool filtering per step
//   - Mid-turn context injection beyond simple steering
//   - Custom stop conditions that inspect message history
type PrepareStepHook struct {
	// StepNumber is the zero-based step index within the current turn.
	StepNumber int
	// Messages is the current context window that will be sent to the LLM.
	// This includes any steering messages already injected in this step.
	Messages []LLMMessage
}

// PrepareStepResult can replace the context window between steps.
type PrepareStepResult struct {
	// Messages replaces the entire context window for this step. If nil,
	// the original messages (including any steering) are used unchanged.
	Messages []LLMMessage
}

// ---------------------------------------------------------------------------
// Generic hook registry with priority ordering
// ---------------------------------------------------------------------------

type hookEntry[In any, Out any] struct {
	id       int
	priority HookPriority
	handler  func(In) *Out
}

type hookRegistry[In any, Out any] struct {
	mu    sync.RWMutex
	hooks []hookEntry[In, Out]
	next  int
}

func newHookRegistry[In any, Out any]() *hookRegistry[In, Out] {
	return &hookRegistry[In, Out]{}
}

// register adds a hook with the given priority and returns an unregister
// function. Within the same priority, hooks run in registration order.
func (hr *hookRegistry[In, Out]) register(p HookPriority, h func(In) *Out) func() {
	hr.mu.Lock()
	id := hr.next
	hr.next++
	hr.hooks = append(hr.hooks, hookEntry[In, Out]{id: id, priority: p, handler: h})
	// Stable sort preserves insertion order within the same priority.
	sort.SliceStable(hr.hooks, func(i, j int) bool {
		return hr.hooks[i].priority < hr.hooks[j].priority
	})
	hr.mu.Unlock()

	return func() {
		hr.mu.Lock()
		defer hr.mu.Unlock()
		for i, entry := range hr.hooks {
			if entry.id == id {
				hr.hooks = append(hr.hooks[:i], hr.hooks[i+1:]...)
				return
			}
		}
	}
}

// run executes all hooks in priority order. The first non-nil result wins.
// Returns nil immediately if no hooks are registered.
func (hr *hookRegistry[In, Out]) run(input In) *Out {
	hr.mu.RLock()
	if len(hr.hooks) == 0 {
		hr.mu.RUnlock()
		return nil
	}
	snapshot := make([]hookEntry[In, Out], len(hr.hooks))
	copy(snapshot, hr.hooks)
	hr.mu.RUnlock()

	for _, entry := range snapshot {
		if result := entry.handler(input); result != nil {
			return result
		}
	}
	return nil
}

// hasHooks returns true if any hooks are registered.
func (hr *hookRegistry[In, Out]) hasHooks() bool {
	hr.mu.RLock()
	defer hr.mu.RUnlock()
	return len(hr.hooks) > 0
}

// ---------------------------------------------------------------------------
// Hook registration methods on Kit
// ---------------------------------------------------------------------------

// OnBeforeToolCall registers a hook that fires before each tool execution.
// Return a non-nil BeforeToolCallResult with Block=true to prevent the tool
// from running. Hooks execute in priority order; the first non-nil result wins.
// Returns an unregister function.
func (m *Kit) OnBeforeToolCall(p HookPriority, h func(BeforeToolCallHook) *BeforeToolCallResult) func() {
	return m.beforeToolCall.register(p, h)
}

// OnAfterToolResult registers a hook that fires after each tool execution.
// Return a non-nil AfterToolResultResult to modify the tool's output before
// it reaches the LLM. Hooks execute in priority order; the first non-nil
// result wins. Returns an unregister function.
func (m *Kit) OnAfterToolResult(p HookPriority, h func(AfterToolResultHook) *AfterToolResultResult) func() {
	return m.afterToolResult.register(p, h)
}

// OnBeforeTurn registers a hook that fires before each prompt turn. Return
// a non-nil BeforeTurnResult to modify the prompt, inject a system message,
// or prepend context. Hooks execute in priority order; the first non-nil
// result wins. Returns an unregister function.
func (m *Kit) OnBeforeTurn(p HookPriority, h func(BeforeTurnHook) *BeforeTurnResult) func() {
	return m.beforeTurn.register(p, h)
}

// OnAfterTurn registers a hook that fires after each prompt turn completes.
// This is observation-only — the handler cannot modify the response. Hooks
// execute in priority order. Returns an unregister function.
func (m *Kit) OnAfterTurn(p HookPriority, h func(AfterTurnHook)) func() {
	return m.afterTurn.register(p, func(input AfterTurnHook) *AfterTurnResult {
		h(input)
		return nil
	})
}

// OnContextPrepare registers a hook that fires after the context window is
// built from the session tree and before messages are sent to the LLM. Return
// a non-nil ContextPrepareResult with Messages to replace the entire context.
// Hooks execute in priority order; the first non-nil result wins.
// Returns an unregister function.
func (m *Kit) OnContextPrepare(p HookPriority, h func(ContextPrepareHook) *ContextPrepareResult) func() {
	return m.contextPrepare.register(p, h)
}

// OnBeforeCompact registers a hook that fires before context compaction runs.
// Return a non-nil BeforeCompactResult with Cancel=true to prevent compaction.
// Hooks execute in priority order; the first non-nil result wins.
// Returns an unregister function.
func (m *Kit) OnBeforeCompact(p HookPriority, h func(BeforeCompactHook) *BeforeCompactResult) func() {
	return m.beforeCompact.register(p, h)
}

// OnPrepareStep registers a hook that fires between steps within a multi-step
// agent turn, after steering messages are injected and before the messages are
// sent to the LLM. Return a non-nil PrepareStepResult with Messages to replace
// the entire context window for this step. Hooks execute in priority order;
// the first non-nil result wins. Returns an unregister function.
//
// This is the most powerful interception point in the agent lifecycle. It
// enables patterns like transforming tool results, dynamic tool filtering,
// and mid-turn context injection.
func (m *Kit) OnPrepareStep(p HookPriority, h func(PrepareStepHook) *PrepareStepResult) func() {
	return m.prepareStep.register(p, h)
}

// ---------------------------------------------------------------------------
// Tool wrapping via hooks
// ---------------------------------------------------------------------------

// hookedTool wraps an AgentTool to run BeforeToolCall and
// AfterToolResult hooks around each execution. The registries are referenced
// by pointer so hooks added after agent creation are still invoked.
type hookedTool struct {
	inner           Tool
	beforeToolCall  *hookRegistry[BeforeToolCallHook, BeforeToolCallResult]
	afterToolResult *hookRegistry[AfterToolResultHook, AfterToolResultResult]
}

func (h *hookedTool) Info() LLMToolInfo                       { return h.inner.Info() }
func (h *hookedTool) ProviderOptions() LLMProviderOptions     { return h.inner.ProviderOptions() }
func (h *hookedTool) SetProviderOptions(o LLMProviderOptions) { h.inner.SetProviderOptions(o) }

func (h *hookedTool) Run(ctx context.Context, call LLMToolCall) (LLMToolResponse, error) {
	toolName := h.inner.Info().Name

	// 1. BeforeToolCall — can block execution.
	if h.beforeToolCall.hasHooks() {
		if result := h.beforeToolCall.run(BeforeToolCallHook{
			ToolCallID: call.ID,
			ToolName:   toolName,
			ToolArgs:   call.Input,
		}); result != nil && result.Block {
			reason := result.Reason
			if reason == "" {
				reason = "blocked by hook"
			}
			return newLLMTextErrorResponse(fmt.Sprintf("Error: %s", reason)),
				fmt.Errorf("tool blocked by hook: %s", reason)
		}
	}

	// 2. Execute actual tool.
	resp, err := h.inner.Run(ctx, call)

	// 3. AfterToolResult — can modify output.
	if h.afterToolResult.hasHooks() {
		if result := h.afterToolResult.run(AfterToolResultHook{
			ToolCallID: call.ID,
			ToolName:   toolName,
			ToolArgs:   call.Input,
			Result:     resp.Content,
			IsError:    err != nil || resp.IsError,
		}); result != nil {
			if result.Result != nil {
				resp.Content = *result.Result
			}
			if result.IsError != nil {
				resp.IsError = *result.IsError
			}
		}
	}

	return resp, err
}

// hookToolWrapper creates a tool wrapper function that applies hook-based
// tool interception. The wrapper references the hook registries directly,
// so hooks registered after agent creation are still called at execution time.
func hookToolWrapper(
	beforeToolCall *hookRegistry[BeforeToolCallHook, BeforeToolCallResult],
	afterToolResult *hookRegistry[AfterToolResultHook, AfterToolResultResult],
) func([]Tool) []Tool {
	return func(tools []Tool) []Tool {
		wrapped := make([]Tool, len(tools))
		for i, tool := range tools {
			wrapped[i] = &hookedTool{
				inner:           tool,
				beforeToolCall:  beforeToolCall,
				afterToolResult: afterToolResult,
			}
		}
		return wrapped
	}
}
