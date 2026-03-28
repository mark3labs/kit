---
title: Callbacks
description: Monitor tool calls and streaming output with the Kit Go SDK.
---

# Callbacks

## Event-based monitoring

For more granular control, use the event subscription API:

```go
// Subscribe returns an unsubscribe function
unsub := host.OnToolCall(func(event kit.ToolCallEvent) {
    fmt.Printf("Tool: %s, Args: %s\n", event.Name, event.Args)
})
defer unsub()

unsub2 := host.OnToolResult(func(event kit.ToolResultEvent) {
    fmt.Printf("Result: %s (error: %v)\n", event.Name, event.IsError)
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

## Hook system

Hooks allow you to intercept and modify behavior. Unlike events, hooks can modify or cancel operations:

```go
// Intercept tool calls before execution
host.OnBeforeToolCall(0, func(ctx context.Context, name string, args string) (string, error) {
    if name == "bash" {
        log.Println("Bash command:", args)
    }
    return args, nil // return modified args or error to cancel
})

// Process results after tool execution
host.OnAfterToolResult(0, func(ctx context.Context, name string, result string) (string, error) {
    return result, nil
})

// Before/after each agent turn
host.OnBeforeTurn(0, func(ctx context.Context) error {
    return nil
})

host.OnAfterTurn(0, func(ctx context.Context) error {
    return nil
})
```

The first argument is a priority (lower = runs first).

## Subagent event monitoring

Monitor real-time events from LLM-initiated subagents (when the model uses the `spawn_subagent` tool):

```go
host.OnToolCall(func(e kit.ToolCallEvent) {
    if e.ToolName == "spawn_subagent" {
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
