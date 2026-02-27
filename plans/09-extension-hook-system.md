# Plan 09: Extension Hook System

**Priority**: P3
**Effort**: High
**Goal**: Expose Go-native interception hooks in the SDK. The Kit CLI app registers its own extension handlers as SDK hooks, proving the API is complete.

## Background

Pi has 20+ lifecycle hooks. Kit already has an internal extension system (`internal/extensions/`) with 13 event types, a `Runner` for dispatch, and tool wrapping. But none of this is accessible through the SDK.

This plan exposes hooks in the SDK and migrates the app's extension dispatch to use them — making the CLI the proof that the hook API is production-ready.

## Prerequisites

- Plan 00 (Create `pkg/kit/`)
- Plan 01 (Export tools — for custom tool registration)
- Plan 02 (Richer type exports)
- Plan 03 (Event subscriber system — observation layer)

## Design: Events vs Hooks

| | Events (Plan 03) | Hooks (This Plan) |
|--|------------------|-------------------|
| Purpose | **Observe** | **Intercept** |
| Can block? | No | Yes (BeforeToolCall) |
| Can modify? | No | Yes (AfterToolResult) |
| Pattern | `Subscribe(func(Event))` | `OnBeforeToolCall(func(Hook) *Result)` |
| Priority | N/A | High/Normal/Low ordering |

Both coexist — events fire regardless; hooks run before/after and can alter execution.

## Step-by-Step

### Step 1: Define hook input/result types

**File**: `pkg/kit/hooks.go` (new)

```go
package kit

type HookPriority int

const (
    HookPriorityHigh   HookPriority = 0
    HookPriorityNormal HookPriority = 50
    HookPriorityLow    HookPriority = 100
)

// BeforeToolCall — can block tool execution
type BeforeToolCallHook struct {
    ToolName string
    ToolArgs string
}
type BeforeToolCallResult struct {
    Block  bool
    Reason string
}

// AfterToolResult — can modify tool output
type AfterToolResultHook struct {
    ToolName string
    ToolArgs string
    Result   string
    IsError  bool
}
type AfterToolResultResult struct {
    Result  *string // non-nil overrides
    IsError *bool   // non-nil overrides
}

// BeforeTurn — can modify prompt, inject context
type BeforeTurnHook struct {
    Prompt string
}
type BeforeTurnResult struct {
    Prompt       *string // override prompt
    SystemPrompt *string // prepend system message
    InjectText   *string // prepend user message (context)
}

// AfterTurn — observe completion
type AfterTurnHook struct {
    Response string
    Error    error
}
```

### Step 2: Implement generic hook registry with priority ordering

```go
type hookRegistry[In any, Out any] struct {
    mu    sync.RWMutex
    hooks []hookEntry[In, Out]
    next  int
}

type hookEntry[In any, Out any] struct {
    id       int
    priority HookPriority
    handler  func(In) *Out
}

func (hr *hookRegistry[In, Out]) register(p HookPriority, h func(In) *Out) func() { ... }
func (hr *hookRegistry[In, Out]) run(input In) *Out { ... } // first non-nil result wins
```

### Step 3: Add registries to Kit struct and expose registration methods

```go
type Kit struct {
    // ... existing fields ...
    beforeToolCall  *hookRegistry[BeforeToolCallHook, BeforeToolCallResult]
    afterToolResult *hookRegistry[AfterToolResultHook, AfterToolResultResult]
    beforeTurn      *hookRegistry[BeforeTurnHook, BeforeTurnResult]
    afterTurn       *hookRegistry[AfterTurnHook, struct{}]
}

func (m *Kit) OnBeforeToolCall(p HookPriority, h func(BeforeToolCallHook) *BeforeToolCallResult) func() { ... }
func (m *Kit) OnAfterToolResult(p HookPriority, h func(AfterToolResultHook) *AfterToolResultResult) func() { ... }
func (m *Kit) OnBeforeTurn(p HookPriority, h func(BeforeTurnHook) *BeforeTurnResult) func() { ... }
func (m *Kit) OnAfterTurn(p HookPriority, h func(AfterTurnHook)) func() { ... }
```

### Step 4: Wire hooks into Prompt flow

In `Prompt()`:
1. Run `beforeTurn` hooks — can modify prompt, inject system/context messages
2. Wrap tools with `hookedTool` that runs `beforeToolCall` (can block) and `afterToolResult` (can modify)
3. Run `afterTurn` hooks after generation

### Step 5: Tool wrapping via hooks

```go
type hookedTool struct {
    inner fantasy.AgentTool
    kit   *Kit
}

func (h *hookedTool) Run(ctx context.Context, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
    // 1. BeforeToolCall hook — can block
    result := h.kit.beforeToolCall.run(BeforeToolCallHook{...})
    if result != nil && result.Block { return error }

    // 2. Execute actual tool
    resp, err := h.inner.Run(ctx, call)

    // 3. AfterToolResult hook — can modify
    after := h.kit.afterToolResult.run(AfterToolResultHook{...})
    if after != nil { /* apply overrides */ }

    return resp, err
}
```

The hook wrapper composes with the existing extension wrapper:
```go
// Extension wrapper runs first (inner), SDK hooks run outside (outer)
tools = extensionWrapper(tools)  // extensions wrap
tools = m.wrapToolsWithHooks(tools) // SDK hooks wrap on top
```

### Step 6: App-as-Consumer — Extension system registers as SDK hooks

This is the payoff step. The app's extension `Runner` currently dispatches events directly in `internal/app/app.go:executeStep()`. After this plan, extensions register as SDK hooks during initialization:

**File**: `pkg/kit/setup.go` or a new `pkg/kit/extensions_bridge.go`

```go
// bridgeExtensions registers extension handlers as SDK hooks.
// This makes the extension system a consumer of the SDK hook API.
func (m *Kit) bridgeExtensions(runner *extensions.Runner) {
    // Extension BeforeAgentStart → SDK BeforeTurn hook
    if runner.HasHandlers(extensions.BeforeAgentStart) {
        m.OnBeforeTurn(HookPriorityNormal, func(h BeforeTurnHook) *BeforeTurnResult {
            result, _ := runner.Emit(extensions.BeforeAgentStartEvent{Prompt: h.Prompt})
            if r, ok := result.(extensions.BeforeAgentStartResult); ok {
                return &BeforeTurnResult{
                    SystemPrompt: r.SystemPrompt,
                    InjectText:   r.InjectText,
                }
            }
            return nil
        })
    }

    // Extension Input → SDK BeforeTurn hook (higher priority, runs first)
    if runner.HasHandlers(extensions.Input) {
        m.OnBeforeTurn(HookPriorityHigh, func(h BeforeTurnHook) *BeforeTurnResult {
            result, _ := runner.Emit(extensions.InputEvent{Text: h.Prompt})
            if r, ok := result.(extensions.InputResult); ok {
                if r.Action == "transform" {
                    return &BeforeTurnResult{Prompt: &r.Text}
                }
            }
            return nil
        })
    }

    // Extension ToolCall → SDK BeforeToolCall hook
    // (Already handled by extensions.WrapToolsWithExtensions, but could also
    //  be bridged here for SDK-only consumers)
}
```

Called during `Kit.New()`:
```go
if setupResult.ExtRunner != nil {
    k.bridgeExtensions(setupResult.ExtRunner)
}
```

**Migration path**:
1. **Phase 1** (this plan): Bridge existing extensions as SDK hooks
2. **Phase 2** (future): `executeStep()` in app.go uses only SDK hooks, removes direct runner calls
3. **Phase 3** (future): Extension runner emits SDK events/hooks natively instead of its own types

### Step 7: Custom tool registration via Options

```go
type Options struct {
    // ... existing fields ...
    ExtraTools []Tool // Additional tools for the agent
}
```

### Step 8: Write tests and verify

```bash
go build -o output/kit ./cmd/kit
go test -race ./...
```

## Files Changed Summary

| Action | File | Change |
|--------|------|--------|
| CREATE | `pkg/kit/hooks.go` | Hook types, registry, registration methods |
| EDIT | `pkg/kit/kit.go` | Hook registries, tool wrapper, Prompt hook invocation |
| CREATE | `pkg/kit/extensions_bridge.go` | Bridge extension events to SDK hooks |
| EDIT | `internal/app/app.go` | Gradual migration to use SDK hooks |

## API Surface After This Plan

```go
// Block dangerous tool calls
k.OnBeforeToolCall(kit.HookPriorityHigh, func(h kit.BeforeToolCallHook) *kit.BeforeToolCallResult {
    if h.ToolName == "bash" && isDangerous(h.ToolArgs) {
        return &kit.BeforeToolCallResult{Block: true, Reason: "dangerous"}
    }
    return nil
})

// Modify tool results
k.OnAfterToolResult(kit.HookPriorityNormal, func(h kit.AfterToolResultHook) *kit.AfterToolResultResult {
    sanitized := redact(h.Result)
    return &kit.AfterToolResultResult{Result: &sanitized}
})

// Inject context before each turn
k.OnBeforeTurn(kit.HookPriorityNormal, func(h kit.BeforeTurnHook) *kit.BeforeTurnResult {
    ctx := loadProjectContext()
    return &kit.BeforeTurnResult{InjectText: &ctx}
})
```

## Verification Checklist

- [ ] BeforeToolCall hooks can block tool calls
- [ ] AfterToolResult hooks can modify results
- [ ] BeforeTurn hooks can modify prompts and inject context
- [ ] Priority ordering works correctly
- [ ] Unregister removes hooks
- [ ] Extension system bridges to SDK hooks
- [ ] Hooks compose with existing extension wrapper
- [ ] Thread-safe under concurrent access
