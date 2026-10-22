
# Callbacks

## Event-based monitoring

Subscribe to events for real-time monitoring. Each method returns an unsubscribe function:

```go
unsub := host.OnToolCall(func(event kit.ToolCallEvent) {
    fmt.Printf("Tool: %s, Args: %s\n", event.ToolName, event.ToolArgs)
})
defer unsub()

unsub2 := host.OnToolResult(func(event kit.ToolResultEvent) {
    fmt.Printf("Result: %s (error: %v)\n", event.ToolName, event.IsError)
})
defer unsub2()

unsub3 := host.OnMessageUpdate(func(event kit.MessageUpdateEvent) {
    fmt.Print(event.Chunk)
})
defer unsub3()

unsub4 := host.OnResponse(func(event kit.ResponseEvent) {
    fmt.Println("Final response received")
})
defer unsub4()

unsub5 := host.OnTurnStart(func(event kit.TurnStartEvent) {
    fmt.Println("Turn started")
})
defer unsub5()

unsub6 := host.OnTurnEnd(func(event kit.TurnEndEvent) {
    fmt.Println("Turn ended")
})
defer unsub6()
```

## Tool call argument streaming

For tools with large arguments (e.g., `write` with a full file body), the `ToolCallEvent` only fires after the full argument JSON finishes streaming — which can take 5-10+ seconds of "dead air." These three events fire during argument generation so UIs can show activity immediately:

```go
host.OnToolCallStart(func(event kit.ToolCallStartEvent) {
    // Fires as soon as the LLM begins generating tool arguments.
    // event.ToolCallID, event.ToolName, event.ToolKind
    fmt.Printf("⏳ %s generating arguments...\n", event.ToolName)
})

host.OnToolCallDelta(func(event kit.ToolCallDeltaEvent) {
    // Each streamed JSON fragment of the tool arguments.
    // event.ToolCallID, event.Delta
    // Useful for live-previewing content or showing byte progress.
})

host.OnToolCallEnd(func(event kit.ToolCallEndEvent) {
    // Tool argument streaming complete — execution about to begin.
    // event.ToolCallID
    fmt.Printf("✓ Arguments ready, executing...\n")
})
```

**Full tool lifecycle**: `ToolCallStartEvent` → `ToolCallDeltaEvent` (repeated) → `ToolCallEndEvent` → `ToolCallEvent` → `ToolExecutionStartEvent` → `ToolOutputEvent` (optional) → `ToolExecutionEndEvent` → `ToolResultEvent`

## Hook system

Hooks can **modify or cancel** operations. Unlike events (read-only), hooks are read-write interceptors.

### BeforeToolCall — block tool execution

```go
host.OnBeforeToolCall(kit.HookPriorityNormal, func(h kit.BeforeToolCallHook) *kit.BeforeToolCallResult {
    // h.ToolCallID, h.ToolName, h.ToolArgs
    // The command-execution tool from kit.NewShellTool reports itself as "shell".
    if h.ToolName == "shell" && strings.Contains(h.ToolArgs, "rm -rf") {
        return &kit.BeforeToolCallResult{Block: true, Reason: "dangerous command"}
    }
    return nil // allow
})
```

A blocked tool does not run. The model receives `Error: <Reason>` as the tool
result and the turn continues, so the model can react (for example, ask the
user or try something else). This is the same as a block from an extension.

`h.ToolArgs` is the raw JSON the model produced. A substring match over it is a
convenience guard, not a security boundary — `rm -r -f`, a shell expansion, or a
script file all get past it. Hooks can restrict exposed tools and arguments, but
allowing an arbitrary shell, interpreter, or script still permits indirect
actions. [Execution environment restrictions](/quick-start#tool-execution-and-trust)
remain necessary when input is untrusted.

### AfterToolResult — modify tool output

```go
host.OnAfterToolResult(kit.HookPriorityNormal, func(h kit.AfterToolResultHook) *kit.AfterToolResultResult {
    // h.ToolCallID, h.ToolName, h.ToolArgs, h.Result, h.IsError
    if h.ToolName == "read" {
        filtered := redactSecrets(h.Result)
        return &kit.AfterToolResultResult{Result: &filtered}
    }
    return nil
})
```

### BeforeTurn — modify prompt, inject messages

```go
host.OnBeforeTurn(kit.HookPriorityNormal, func(h kit.BeforeTurnHook) *kit.BeforeTurnResult {
    // h.Prompt
    newPrompt := h.Prompt + "\nAlways respond in JSON."
    return &kit.BeforeTurnResult{Prompt: &newPrompt}
    // Also available: SystemPrompt *string, InjectText *string
})
```

### AfterTurn — observation only

```go
host.OnAfterTurn(kit.HookPriorityNormal, func(h kit.AfterTurnHook) {
    // h.Response, h.Error
    log.Printf("Turn completed: %d chars", len(h.Response))
})
```

### PrepareStep — intercept messages between steps

The most powerful hook — fires between steps within a multi-step agent turn, after any steering messages are injected and before messages are sent to the LLM. Can replace the entire context window.

```go
host.OnPrepareStep(kit.HookPriorityNormal, func(h kit.PrepareStepHook) *kit.PrepareStepResult {
    // h.StepNumber — zero-based step index within the turn
    // h.Messages   — current context window (includes any steering)

    // Example: transform tool results with images into user messages
    modified := transformImageToolResults(h.Messages)
    return &kit.PrepareStepResult{Messages: modified}
    // Return nil to pass through unchanged
})
```

Use cases: transforming tool results (e.g., image data for vision models), dynamic tool filtering per step, mid-turn context injection, custom stop conditions.

Hooks run synchronously. Use `h.Context` to bound preparation I/O. A non-nil
`PrepareStepResult.Error` aborts before the provider request and takes precedence
over Messages, Tools, and ToolChoice. `Prompt` and `PromptResult` preserve
`errors.Is` identity without automatic retry, resume, or media recovery.
Completed tool/result pairs remain in partial results and persistence. Kit
checks cancellation before and after hooks, but does not start a worker to
interrupt an uncooperative hook.

Skill changes completed inside preparation affect this request's managed
prompt and activation schema. Explicit overrides still win. Nil Tools uses the
live set; an empty non-nil Tools slice offers no tools. The override lasts one
step. Hosts must finish any durable write before returning: Kit does not
provide a transaction over external storage.

### Hook priorities

```go
kit.HookPriorityHigh   = 0   // runs first
kit.HookPriorityNormal = 50  // default
kit.HookPriorityLow    = 100 // runs last
```

Lower values run first. Interceptor hooks use the first non-nil result. `OnStepContext` is different: all decorators run and each receives the previous context.

## All event types

| Event | Typed Subscriber | Description |
|-------|-----------------|-------------|
| `TurnStartEvent` | `OnTurnStart` | Agent turn started |
| `TurnEndEvent` | `OnTurnEnd` | Agent turn completed |
| `MessageStartEvent` | `OnMessageStart` | New assistant message begins |
| `MessageUpdateEvent` | `OnMessageUpdate` | Streaming text chunk from LLM |
| `MessageEndEvent` | `OnMessageEnd` | Assistant message complete |
| `ToolCallStartEvent` | `OnToolCallStart` | LLM began generating tool call arguments |
| `ToolCallDeltaEvent` | `OnToolCallDelta` | Streamed JSON fragment of tool call arguments |
| `ToolCallEndEvent` | `OnToolCallEnd` | Tool argument streaming complete |
| `ToolCallEvent` | `OnToolCall` | Tool call fully parsed, about to execute |
| `ToolExecutionStartEvent` | `OnToolExecutionStart` | Tool begins executing |
| `ToolExecutionEndEvent` | `OnToolExecutionEnd` | Tool finishes executing |
| `ToolResultEvent` | `OnToolResult` | Tool execution completed with result |
| `ToolCallContentEvent` | `OnToolCallContent` | Text content alongside tool calls |
| `ToolOutputEvent` | `OnToolOutput` | Streaming output chunk from tool (e.g., bash) |
| `ResponseEvent` | `OnResponse` | Final response received |
| `ReasoningStartEvent` | `OnReasoningStart` | LLM begins reasoning/thinking |
| `ReasoningDeltaEvent` | `OnReasoningDelta` | Streaming reasoning/thinking chunk |
| `ReasoningCompleteEvent` | `OnReasoningComplete` | Reasoning/thinking finished |
| `StepStartEvent` | `OnStepStart` | New LLM call begins within a turn |
| `StepFinishEvent` | `OnStepFinish` | Step completes (with usage, finish reason, tool call info) |
| `StepUsageEvent` | `OnStepUsage` | Per-step token usage |
| `StreamFinishEvent` | `OnStreamFinish` | Per-step stream completes (with usage + finish reason) |
| `TextStartEvent` | `OnTextStart` | LLM begins text content generation |
| `TextEndEvent` | `OnTextEnd` | LLM finishes text content generation |
| `WarningsEvent` | `OnWarnings` | LLM provider returned warnings |
| `SourceEvent` | `OnSource` | LLM referenced a source (e.g., web search) |
| `ErrorEvent` | `OnError` | Agent-level error during streaming |
| `StepResumeEvent` | `OnStepResume` | Failed streaming step will restart; discard failed-step deltas |
| `RetryEvent` | `OnRetry` | LLM request retried after transient error |
| `CompactionEvent` | `OnCompaction` | Conversation compacted (fires on success **and** failure — check `Err`) |
| `SteerConsumedEvent` | `OnSteerConsumed` | Steering messages injected into turn |
| `PasswordPromptEvent` | `Subscribe` (generic only) | Sudo command needs password (respond via `ResponseCh`) |

### Compaction telemetry

`CompactionEvent` fires after every compaction attempt. On success `Err` is
`nil` and the summary/token/file fields are populated; on failure `Err` is
non-nil and the rest are zero-valued. This lets you wire symmetric
start/end lifecycle telemetry without hand-rolling the failure path:

```go
host.OnCompaction(func(e kit.CompactionEvent) {
    if e.Err != nil {
        log.Printf("compaction failed: %v", e.Err)
        return
    }
    log.Printf("compacted %d → %d tokens (%d messages removed)",
        e.OriginalTokens, e.CompactedTokens, e.MessagesRemoved)
})
```

## Subagent event monitoring

Monitor real-time events from LLM-initiated subagents (when the model uses the `subagent` tool):

```go
host.OnToolCall(func(e kit.ToolCallEvent) {
    if e.ToolName == "subagent" {
        host.SubscribeSubagent(e.ToolCallID, func(event kit.Event) {
            // Receives the same event types as Subscribe(), scoped to the child agent
            switch ev := event.(type) {
            case kit.MessageUpdateEvent:
                fmt.Print(ev.Chunk)
            case kit.ToolCallEvent:
                fmt.Printf("Subagent calling: %s\n", ev.ToolName)
            }
        })
    }
})
```

For retained history rather than live callbacks, use `SubagentRuns` or `GetSubagentRun`. To follow every run of the host as it changes, use `SubscribeSubagentRuns`. These snapshots include active and finished runs started by the LLM, extensions, or direct SDK calls. See [Retained run snapshots](/advanced/subagents#retained-run-snapshots) for limits and an API comparison. Existing live listeners are unchanged.

`SubscribeSubagent` returns an unsubscribe function. Listeners are also cleaned up automatically when the subagent completes. See [Subagents](/advanced/subagents) for more details.

## Provider request tracing

Use `WithHTTPTransport` (or `Options.HTTPTransport`) to wrap the base transport
of built-in providers. The wrapper receives requests after Kit applies its
headers. It must return a non-nil `http.RoundTripper`. It passes to model changes,
temporary completions, and subagents. Custom provider factories own their HTTP
clients; this option does not wrap them, MCP, OAuth, or credential requests.

Use `OnStepContext` to pass a step span or other context value to the provider
request and that step's tools:

The fragment assumes `host` and an application-owned comparable `stepKey`.

```go
host.OnStepContext(kit.HookPriorityNormal, func(h kit.StepContextHook) context.Context {
    return context.WithValue(h.Context, stepKey, h.StepNumber)
})
```

Every context hook runs in priority order. A nil result keeps the current
context. Derive results from `h.Context` so turn cancellation still applies.
Each step starts from the turn context, not the previous step context. A resumed
step runs the hooks again with the same step number. `OnPrepareStep` keeps its
existing first-non-nil-result behavior. Context hooks run after preparation and
before `StepStartEvent`. Both `StepStartEvent.Model` and `StepFinishEvent.Model`
report the `provider/model` that served the step, even if the configured model
changes during the turn. Kit does not create spans or depend on a tracing SDK.
