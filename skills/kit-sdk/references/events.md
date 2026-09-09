# Kit SDK: Event System

> Part of the `kit-sdk` skill. Read `SKILL.md` first for the overview and critical constraints.

## Event System

Events are read-only observations of the agent lifecycle. Register before calling Prompt.

### Typed convenience subscribers

```go
// Each returns an unsubscribe function.
unsub := host.OnToolCall(func(e kit.ToolCallEvent) {
    // e.ToolCallID, e.ToolName, e.ToolKind, e.ToolArgs, e.ParsedArgs
})
defer unsub()

host.OnToolCallStart(func(e kit.ToolCallStartEvent) {
    // Fires when the LLM begins generating tool call arguments.
    // e.ToolCallID, e.ToolName, e.ToolKind
    // Use this to show a "running" indicator immediately — before the
    // full argument JSON finishes streaming (eliminates "dead air").
})

host.OnToolCallDelta(func(e kit.ToolCallDeltaEvent) {
    // Fires for each streamed fragment of tool call arguments.
    // e.ToolCallID, e.Delta (JSON fragment)
    // Useful for live-previewing artifact content or progress indicators.
})

host.OnToolCallEnd(func(e kit.ToolCallEndEvent) {
    // Fires when tool argument streaming is complete, before execution.
    // e.ToolCallID
    // Transition UI from "generating args" to "executing".
})

host.OnToolResult(func(e kit.ToolResultEvent) {
    // e.ToolCallID, e.ToolName, e.ToolKind, e.ToolArgs, e.ParsedArgs
    // e.Result, e.IsError, e.Metadata (*ToolResultMetadata)
})

host.OnToolOutput(func(e kit.ToolOutputEvent) {
    // e.ToolCallID, e.ToolName, e.Chunk, e.IsStderr
    // Streaming bash output chunks
})

host.OnMessageUpdate(func(e kit.MessageUpdateEvent) {
    fmt.Print(e.Chunk) // real-time text streaming
})

host.OnResponse(func(e kit.ResponseEvent) {
    // e.Content — final response text
})

host.OnTurnStart(func(e kit.TurnStartEvent) {
    // e.Prompt
})

host.OnTurnEnd(func(e kit.TurnEndEvent) {
    // e.Response, e.Error, e.StopReason
})

host.OnStepStart(func(e kit.StepStartEvent) {
    // e.StepNumber — which LLM call step (1-based)
})

host.OnStepFinish(func(e kit.StepFinishEvent) {
    // e.StepNumber, e.HasToolCalls, e.FinishReason, e.Usage (LLMUsage)
})

host.OnWarnings(func(e kit.WarningsEvent) {
    for _, w := range e.Warnings {
        log.Printf("warning: %s", w)
    }
})

host.OnError(func(e kit.ErrorEvent) {
    log.Printf("agent error: %v", e.Error)
})

host.OnRetry(func(e kit.RetryEvent) {
    log.Printf("retrying (attempt %d): %v", e.Attempt, e.Error)
})

host.OnTextStart(func(e kit.TextStartEvent) {
    // e.ID — content block ID
})

host.OnTextEnd(func(e kit.TextEndEvent) {
    // e.ID — content block ID
})

host.OnReasoningStart(func(e kit.ReasoningStartEvent) {
    // e.ID — reasoning block ID
})

host.OnSource(func(e kit.SourceEvent) {
    // e.SourceType, e.ID, e.URL, e.Title
})

host.OnStreamFinish(func(e kit.StreamFinishEvent) {
    // e.Usage (LLMUsage), e.FinishReason
})

// Additional typed subscribers for previously generic-only events:
host.OnMessageStart(func(e kit.MessageStartEvent) {})
host.OnMessageEnd(func(e kit.MessageEndEvent) { /* e.Content */ })
host.OnReasoningDelta(func(e kit.ReasoningDeltaEvent) { /* e.Delta */ })
host.OnReasoningComplete(func(e kit.ReasoningCompleteEvent) {})
host.OnToolExecutionStart(func(e kit.ToolExecutionStartEvent) { /* e.ToolCallID, e.ToolName, e.ToolKind, e.ToolArgs */ })
host.OnToolExecutionEnd(func(e kit.ToolExecutionEndEvent) { /* e.ToolCallID, e.ToolName, e.ToolKind */ })
host.OnToolCallContent(func(e kit.ToolCallContentEvent) { /* e.Content */ })
host.OnStepUsage(func(e kit.StepUsageEvent) { /* e.InputTokens, e.OutputTokens, e.CacheReadTokens, e.CacheWriteTokens */ })
host.OnCompaction(func(e kit.CompactionEvent) { /* e.Summary, e.OriginalTokens, e.CompactedTokens, ... */ })
host.OnSteerConsumed(func(e kit.SteerConsumedEvent) { /* e.Count */ })
```

> **Rename note:** `OnStreaming` has been renamed to `OnMessageUpdate`. The old `OnStreaming` name is kept as a deprecated alias for one release cycle.

### Generic subscriber (receives all events)

```go
unsub := host.Subscribe(func(e kit.Event) {
    switch ev := e.(type) {
    case kit.ToolCallEvent:
        // ...
    case kit.MessageUpdateEvent:
        // ...
    case kit.CompactionEvent:
        // ev.Summary, ev.OriginalTokens, ev.CompactedTokens
    }
})
```

### All event types

| Event Type | Struct | Key Fields |
|------------|--------|------------|
| `turn_start` | `TurnStartEvent` | `Prompt` |
| `turn_end` | `TurnEndEvent` | `Response`, `Error`, `StopReason` |
| `message_start` | `MessageStartEvent` | *(none)* |
| `message_update` | `MessageUpdateEvent` | `Chunk` |
| `message_end` | `MessageEndEvent` | `Content` |
| `tool_call_start` | `ToolCallStartEvent` | `ToolCallID`, `ToolName`, `ToolKind` |
| `tool_call_delta` | `ToolCallDeltaEvent` | `ToolCallID`, `Delta` |
| `tool_call_end` | `ToolCallEndEvent` | `ToolCallID` |
| `tool_call` | `ToolCallEvent` | `ToolCallID`, `ToolName`, `ToolKind`, `ToolArgs`, `ParsedArgs` |
| `tool_execution_start` | `ToolExecutionStartEvent` | `ToolCallID`, `ToolName`, `ToolKind`, `ToolArgs` |
| `tool_execution_end` | `ToolExecutionEndEvent` | `ToolCallID`, `ToolName`, `ToolKind` |
| `tool_result` | `ToolResultEvent` | `ToolCallID`, `ToolName`, `ToolKind`, `ToolArgs`, `ParsedArgs`, `Result`, `IsError`, `Metadata` |
| `tool_call_content` | `ToolCallContentEvent` | `Content` |
| `tool_output` | `ToolOutputEvent` | `ToolCallID`, `ToolName`, `Chunk`, `IsStderr` |
| `response` | `ResponseEvent` | `Content` |
| `compaction` | `CompactionEvent` | `Summary`, `OriginalTokens`, `CompactedTokens`, `MessagesRemoved`, `ReadFiles`, `ModifiedFiles` |
| `reasoning_delta` | `ReasoningDeltaEvent` | `Delta` |
| `step_usage` | `StepUsageEvent` | `InputTokens`, `OutputTokens`, `CacheReadTokens`, `CacheWriteTokens` |
| `steer_consumed` | `SteerConsumedEvent` | `Count` |
| `step_start` | `StepStartEvent` | `StepNumber` |
| `step_finish` | `StepFinishEvent` | `StepNumber`, `HasToolCalls`, `FinishReason`, `Usage` |
| `text_start` | `TextStartEvent` | `ID` |
| `text_end` | `TextEndEvent` | `ID` |
| `reasoning_start` | `ReasoningStartEvent` | `ID` |
| `warnings` | `WarningsEvent` | `Warnings` |
| `source` | `SourceEvent` | `SourceType`, `ID`, `URL`, `Title` |
| `stream_finish` | `StreamFinishEvent` | `Usage`, `FinishReason` |
| `error` | `ErrorEvent` | `Error` |
| `retry` | `RetryEvent` | `Attempt`, `Error` |
| `password_prompt` | `PasswordPromptEvent` | `Prompt`, `ResponseCh` |

**Tool call streaming lifecycle**: `ToolCallStartEvent` → `ToolCallDeltaEvent` (repeated) → `ToolCallEndEvent` → `ToolCallEvent` → `ToolExecutionStartEvent` → `ToolOutputEvent` (optional, repeated) → `ToolExecutionEndEvent` → `ToolResultEvent`

**PasswordPromptEvent** (for sudo password handling):
```go
// PasswordPromptEvent fires when a sudo command needs a password.
// The TUI should display a password prompt and send the result back via ResponseCh.
type PasswordPromptEvent struct {
    // Prompt is the message to display to the user.
    Prompt string
    // ResponseCh receives the password from the TUI.
    // The TUI must send exactly one value: (password, false) for submit
    // or ("", true) for cancel.
    ResponseCh chan<- PasswordPromptResponse
}

// PasswordPromptResponse carries the password prompt result.
type PasswordPromptResponse struct {
    Password  string
    Cancelled bool
}
```

### Tool kind constants

Tools are classified by kind for UI rendering:

- `ToolKindExecute` = `"execute"` — bash
- `ToolKindEdit` = `"edit"` — edit, write
- `ToolKindRead` = `"read"` — read, ls
- `ToolKindSearch` = `"search"` — grep, find
- `ToolKindSubagent` = `"agent"` — subagent

