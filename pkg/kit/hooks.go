package kit

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"charm.land/fantasy"
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
	ToolName string
	ToolArgs string
}

// BeforeToolCallResult controls whether the tool call proceeds.
type BeforeToolCallResult struct {
	Block  bool   // true prevents the tool from running
	Reason string // human-readable reason for blocking
}

// AfterToolResultHook is the input for hooks that fire after a tool executes.
type AfterToolResultHook struct {
	ToolName string
	ToolArgs string
	Result   string
	IsError  bool
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
func (hr *hookRegistry[In, Out]) run(input In) *Out {
	hr.mu.RLock()
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

// ---------------------------------------------------------------------------
// Tool wrapping via hooks
// ---------------------------------------------------------------------------

// hookedTool wraps a fantasy.AgentTool to run BeforeToolCall and
// AfterToolResult hooks around each execution. The registries are referenced
// by pointer so hooks added after agent creation are still invoked.
type hookedTool struct {
	inner           fantasy.AgentTool
	beforeToolCall  *hookRegistry[BeforeToolCallHook, BeforeToolCallResult]
	afterToolResult *hookRegistry[AfterToolResultHook, AfterToolResultResult]
}

func (h *hookedTool) Info() fantasy.ToolInfo                       { return h.inner.Info() }
func (h *hookedTool) ProviderOptions() fantasy.ProviderOptions     { return h.inner.ProviderOptions() }
func (h *hookedTool) SetProviderOptions(o fantasy.ProviderOptions) { h.inner.SetProviderOptions(o) }

func (h *hookedTool) Run(ctx context.Context, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
	toolName := h.inner.Info().Name

	// 1. BeforeToolCall — can block execution.
	if h.beforeToolCall.hasHooks() {
		if result := h.beforeToolCall.run(BeforeToolCallHook{
			ToolName: toolName,
			ToolArgs: call.Input,
		}); result != nil && result.Block {
			reason := result.Reason
			if reason == "" {
				reason = "blocked by hook"
			}
			return fantasy.NewTextErrorResponse(fmt.Sprintf("Error: %s", reason)),
				fmt.Errorf("tool blocked by hook: %s", reason)
		}
	}

	// 2. Execute actual tool.
	resp, err := h.inner.Run(ctx, call)

	// 3. AfterToolResult — can modify output.
	if h.afterToolResult.hasHooks() {
		if result := h.afterToolResult.run(AfterToolResultHook{
			ToolName: toolName,
			ToolArgs: call.Input,
			Result:   resp.Content,
			IsError:  err != nil || resp.IsError,
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
) func([]fantasy.AgentTool) []fantasy.AgentTool {
	return func(tools []fantasy.AgentTool) []fantasy.AgentTool {
		wrapped := make([]fantasy.AgentTool, len(tools))
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
