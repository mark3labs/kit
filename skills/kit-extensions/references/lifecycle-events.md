# Kit Extensions: Lifecycle Events

> Part of the `kit-extensions` skill. Read `SKILL.md` first for the overview and critical constraints.

## Lifecycle Events

Kit provides 30 lifecycle events. Each handler receives an event struct and a `Context`.

### Session Events

```go
// Fired when session is loaded/created.
api.OnSessionStart(func(e ext.SessionStartEvent, ctx ext.Context) {
    // e.SessionID string
})

// Fired when Kit is shutting down. Use for cleanup.
api.OnSessionShutdown(func(e ext.SessionShutdownEvent, ctx ext.Context) {
    // No fields.
})
```

### Agent Turn Events

```go
// Before agent starts processing. Can inject system prompt or text.
api.OnBeforeAgentStart(func(e ext.BeforeAgentStartEvent, ctx ext.Context) *ext.BeforeAgentStartResult {
    // e.Prompt string
    // Return nil to pass through.
    // Return &ext.BeforeAgentStartResult{SystemPrompt: &s} to augment system prompt.
    // Return &ext.BeforeAgentStartResult{InjectText: &s} to inject text before prompt.
    return nil
})

// Agent loop has started.
api.OnAgentStart(func(e ext.AgentStartEvent, ctx ext.Context) {
    // e.Prompt string
})

// Agent finished responding. Carries per-turn aggregates so observer-style
// extensions don't need to maintain parallel bookkeeping.
api.OnAgentEnd(func(e ext.AgentEndEvent, ctx ext.Context) {
    // e.Response string
    // e.StopReason string — "error" (on failure), "completed" (when LLM returns
    //   empty stop reason), or the raw LLM provider value passed through
    //   (e.g. "stop", "length" (max output tokens hit), "tool-calls", "content-filter").
    //   To detect errors, check e.StopReason == "error".
    //   Do NOT compare against "completed" for success — instead check != "error".
    //
    // Per-turn aggregates (computed by Kit's runtime):
    // e.ToolCallCount          int       — total tool invocations this turn
    // e.ToolNames              []string  — tool names in call order (duplicates preserved)
    // e.LLMCallCount           int       — LLM round-trips / tool-loop iterations
    // e.InputTokensDelta       int       — sum of input tokens across LLM calls this turn
    // e.OutputTokensDelta      int
    // e.CacheReadTokensDelta   int
    // e.CacheWriteTokensDelta  int
    // e.CostDelta              float64   — USD cost (zero when pricing unknown / OAuth)
    // e.DurationMs             int64     — wall-clock duration AgentStart→AgentEnd
})

// Per-LLM-call usage — fires after each provider round-trip with token + cost
// deltas attributed to that specific call. A single turn typically produces
// multiple LLMUsageEvents (one per tool-loop iteration). Use this for accurate
// budget enforcement that needs to react between calls instead of waiting
// for the turn to finish.
api.OnLLMUsage(func(e ext.LLMUsageEvent, ctx ext.Context) {
    // e.InputTokens, e.OutputTokens             int
    // e.CacheReadTokens, e.CacheWriteTokens     int
    // e.Cost                                    float64  — USD; zero when pricing unknown / OAuth
    // e.Model, e.Provider                       string   — model used for THIS call
    //                                                      (may differ across calls if SetModel was called)
    // e.StepNumber                              int      — zero-based step index in this turn
    // e.FinishReason                            string   — "stop" / "tool_calls" / "length" / ...
    // e.RequestID                               string   — optional provider correlation id (may be empty)
})
```

### Tool Events

```go
// Before a tool executes. Can block the call.
api.OnToolCall(func(e ext.ToolCallEvent, ctx ext.Context) *ext.ToolCallResult {
    // e.ToolName string
    // e.ToolCallID string
    // e.Input string — JSON-encoded parameters
    // e.Source string — "llm" or "user"
    // Return nil to allow.
    // Return &ext.ToolCallResult{Block: true, Reason: "..."} to block.
    return nil
})

// Tool execution started (informational only).
api.OnToolExecutionStart(func(e ext.ToolExecutionStartEvent, ctx ext.Context) {
    // e.ToolName string
})

// Tool execution ended (informational only).
api.OnToolExecutionEnd(func(e ext.ToolExecutionEndEvent, ctx ext.Context) {
    // e.ToolName string
})

// After a tool returns. Can modify the result.
api.OnToolResult(func(e ext.ToolResultEvent, ctx ext.Context) *ext.ToolResultResult {
    // e.ToolName string
    // e.Input string
    // e.Content string
    // e.IsError bool
    // Return nil to pass through.
    // Return &ext.ToolResultResult{Content: &s} to replace content.
    // Return &ext.ToolResultResult{IsError: &b} to change error status.
    return nil
})
```

### Tool Call Input Streaming Events

These events fire during the LLM's tool argument generation phase, **before** the tool call is fully parsed and before `OnToolCall` fires. They enable UIs to show tool activity immediately rather than waiting for the full argument JSON to finish streaming.

```go
// Fires when the LLM begins generating tool call arguments.
// The tool name is known but the full argument JSON is still streaming.
api.OnToolCallInputStart(func(e ext.ToolCallInputStartEvent, ctx ext.Context) {
    // e.ToolCallID string — stable ID for correlating tool lifecycle events
    // e.ToolName string — name of the tool being called
    // e.ToolKind string — "execute", "edit", "read", "search", "agent"
    ctx.PrintInfo("Tool starting: " + e.ToolName)
})

// Fires for each streamed fragment of tool call arguments.
// Useful for live-previewing artifact content or showing a progress indicator.
api.OnToolCallInputDelta(func(e ext.ToolCallInputDeltaEvent, ctx ext.Context) {
    // e.ToolCallID string
    // e.Delta string — JSON fragment of tool arguments
})

// Fires when tool argument streaming is complete, before the tool call
// is parsed and execution begins. Transition UI from "generating args"
// to "executing".
api.OnToolCallInputEnd(func(e ext.ToolCallInputEndEvent, ctx ext.Context) {
    // e.ToolCallID string
})
```

**Full tool lifecycle order**: `OnToolCallInputStart` → `OnToolCallInputDelta` (repeated) → `OnToolCallInputEnd` → `OnToolCall` → `OnToolExecutionStart` → `OnToolOutput` (optional, repeated) → `OnToolExecutionEnd` → `OnToolResult`

### Input Events

```go
// User submitted input. Can handle or transform it.
api.OnInput(func(e ext.InputEvent, ctx ext.Context) *ext.InputResult {
    // e.Text string
    // e.Source string — "interactive", "cli", "script", "queue"
    // Return nil to pass through to agent.
    // Return &ext.InputResult{Action: "handled"} to consume without sending to agent.
    // Return &ext.InputResult{Action: "transform", Text: "new text"} to rewrite.
    return nil
})
```

### Streaming Events

```go
api.OnMessageStart(func(e ext.MessageStartEvent, ctx ext.Context) {})
api.OnMessageUpdate(func(e ext.MessageUpdateEvent, ctx ext.Context) {
    // e.Chunk string — streaming text chunk
})
api.OnMessageEnd(func(e ext.MessageEndEvent, ctx ext.Context) {
    // e.Content string — full message content
})
```

### Model Events

```go
api.OnModelChange(func(e ext.ModelChangeEvent, ctx ext.Context) {
    // e.NewModel string
    // e.PreviousModel string
    // e.Source string — "extension" or "user"
})

// Extended-thinking effort level changed.
api.OnThinkingLevelChange(func(e ext.ThinkingLevelChangeEvent, ctx ext.Context) {
    // e.NewLevel, e.PreviousLevel string — off, none, minimal, low, medium, high
    // e.Source string — "user" (/thinking or shift+tab) or "model_fallback"
    //   ("model_fallback" = automatic downgrade because the newly selected
    //    model does not support the previous level)
})
```

### UI Events

Interactive TUI only; these do not fire in headless, ACP, or script mode.

```go
// Terminal resized. Also fires once at startup with the initial size.
api.OnTerminalResize(func(e ext.TerminalResizeEvent, ctx ext.Context) {
    // e.Width, e.Height int
})

// UI entered or left the working state.
api.OnTurnStateChange(func(e ext.TurnStateChangeEvent, ctx ext.Context) {
    // e.State, e.Previous string — "working" or "idle"
})
```

`OnTurnStateChange` is a **superset of `OnAgentStart`/`OnAgentEnd`**: it also
covers work that never reaches the agent loop (shell commands run with `!`) and
fires on every path back to idle, including cancellation and error. Use it to
drive a spinner or turn timer; use `OnAgentStart`/`OnAgentEnd` when you
specifically care about agent turns and their token usage.

### Context Filtering

```go
// Before messages are sent to the LLM. Can filter, reorder, or inject messages.
api.OnContextPrepare(func(e ext.ContextPrepareEvent, ctx ext.Context) *ext.ContextPrepareResult {
    // e.Messages []ext.ContextMessage
    // Each ContextMessage has: Index int, Role string, Content string
    // Index -1 means a new injected message (not from session).
    // Return nil to pass through.
    // Return &ext.ContextPrepareResult{Messages: msgs} to replace the context window.
    return nil
})
```

### Session Control Events

```go
// Before forking the session tree. Can cancel.
api.OnBeforeFork(func(e ext.BeforeForkEvent, ctx ext.Context) *ext.BeforeForkResult {
    // e.TargetID string, e.IsUserMessage bool, e.UserText string
    return nil // or &ext.BeforeForkResult{Cancel: true, Reason: "..."}
})

// Before switching/clearing session. Can cancel.
api.OnBeforeSessionSwitch(func(e ext.BeforeSessionSwitchEvent, ctx ext.Context) *ext.BeforeSessionSwitchResult {
    // e.Reason string — "new" or "clear"
    return nil // or &ext.BeforeSessionSwitchResult{Cancel: true, Reason: "..."}
})

// Before context compaction. Can cancel.
api.OnBeforeCompact(func(e ext.BeforeCompactEvent, ctx ext.Context) *ext.BeforeCompactResult {
    // e.EstimatedTokens, e.ContextLimit int
    // e.UsagePercent float64, e.MessageCount int, e.IsAutomatic bool
    return nil // or &ext.BeforeCompactResult{Cancel: true, Reason: "..."}
})
```

### Custom Events

```go
// Subscribe to custom events emitted by other extensions.
api.OnCustomEvent("event-name", func(data string) {
    // data is arbitrary string payload
})

// Emit from Context:
ctx.EmitCustomEvent("event-name", "payload")
```

