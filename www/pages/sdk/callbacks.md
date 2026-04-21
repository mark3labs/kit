---
title: Callbacks
description: Monitor tool calls and streaming output with the Kit Go SDK.
---

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

unsub3 := host.OnStreaming(func(event kit.MessageUpdateEvent) {
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
    if h.ToolName == "bash" && strings.Contains(h.ToolArgs, "rm -rf") {
        return &kit.BeforeToolCallResult{Block: true, Reason: "dangerous command"}
    }
    return nil // allow
})
```

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

### Hook priorities

```go
kit.HookPriorityHigh   = 0   // runs first
kit.HookPriorityNormal = 50  // default
kit.HookPriorityLow    = 100 // runs last
```

Lower values run first. First non-nil result wins.

## All event types

| Event | Description |
|-------|-------------|
| `ToolCallStartEvent` | LLM began generating tool call arguments (tool name known, args streaming) |
| `ToolCallDeltaEvent` | Streamed JSON fragment of tool call arguments |
| `ToolCallEndEvent` | Tool argument streaming complete, before execution begins |
| `ToolCallEvent` | Tool call fully parsed and about to execute |
| `ToolResultEvent` | Tool execution completed with result |
| `ToolOutputEvent` | Streaming output chunk from tool (e.g., bash stdout/stderr) |
| `MessageUpdateEvent` | Streaming text chunk from LLM |
| `ResponseEvent` | Final response received |
| `TurnStartEvent` | Agent turn started |
| `TurnEndEvent` | Agent turn completed |
| `PasswordPromptEvent` | Sudo command needs password (respond via `ResponseCh`) |

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

`SubscribeSubagent` returns an unsubscribe function. Listeners are also cleaned up automatically when the subagent completes. See [Subagents](/advanced/subagents) for more details.
