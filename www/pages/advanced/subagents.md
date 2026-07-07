---
title: Subagents
description: Multi-agent orchestration with Kit subagents.
---

# Subagents

Kit supports multi-agent orchestration through both subprocess spawning and in-process subagents.

## Subprocess pattern

Spawn Kit as a subprocess for isolated agent execution:

```bash
kit "Analyze codebase" \
    --json \
    --no-session \
    --no-extensions \
    --quiet \
    --model anthropic/claude-haiku-latest
```

Key flags for subprocess usage:

| Flag | Purpose |
|------|---------|
| `--quiet` | Stdout only, no TUI |
| `--no-session` | Ephemeral, no persistence |
| `--no-extensions` | Prevent recursive extension loading |
| `--json` | Machine-readable output |
| `--system-prompt` | Custom system prompt (string or file path) |

Positional arguments are the prompt. `@file` arguments attach file content as context.

## Built-in subagent tool

Kit includes a built-in `subagent` tool that the LLM can use to delegate tasks to independent child agents:

```
subagent(
    task: "Analyze the test files and summarize coverage",
    model: "anthropic/claude-haiku-latest",   // optional
    system_prompt: "You are a test analysis expert.",  // optional
    timeout_seconds: 300                               // optional, max 1800
)
```

Subagents run as separate in-process Kit instances with full tool access (except spawning further subagents, to prevent infinite recursion). They can run in parallel.

## Extension subagents

Extensions can spawn subagents programmatically:

```go
_, result, err := ctx.SpawnSubagent(ext.SubagentConfig{
    Prompt:       "Review this code for security issues",
    Model:        "anthropic/claude-sonnet-latest",
    SystemPrompt: "You are a security auditor.",
    Blocking:     true,
})
```

With `Blocking: false` (the default), the subagent runs in a background goroutine and `SpawnSubagent` returns immediately with a non-nil handle (result is nil); use `OnComplete`/`OnEvent` callbacks or the handle to observe the run:

```go
handle, _, err := ctx.SpawnSubagent(ext.SubagentConfig{
    Prompt: "Write unit tests for UserService",
    OnOutput: func(chunk string) {
        // Live assistant text chunks (e.g. update a widget)
    },
    OnComplete: func(result ext.SubagentResult) {
        ctx.SendMessage("Subagent finished:\n" + result.Response)
    },
})
// handle.Kill()   — cancel the running subagent
// handle.Wait()   — block until completion, returns SubagentResult
// <-handle.Done() — channel that closes on completion
```

Background subagents run in-process (no subprocess): they get their own session, event bus, and agent loop, inherit the parent's active tools minus the `subagent` tool, and do not load extensions. Sessions are persisted by default; set `NoSession: true` for ephemeral runs.

### Monitoring subagents from extensions

When the LLM (not the extension itself) spawns a subagent using the `subagent` tool, extensions can monitor its activity in real-time using three lifecycle event handlers:

```go
// Track active subagents and display their output
var subagentWidgets map[string]*SubagentWidget

func Init(api ext.API) {
    // Subagent started by the main agent
    api.OnSubagentStart(func(e ext.SubagentStartEvent, ctx ext.Context) {
        // e.ToolCallID — unique ID for this subagent invocation
        // e.Task — the task/prompt sent to the subagent
        widget := NewWidget(e.ToolCallID, e.Task)
        subagentWidgets[e.ToolCallID] = widget
        ctx.SetWidget(widget.Config())
    })

    // Real-time streaming from subagent
    api.OnSubagentChunk(func(e ext.SubagentChunkEvent, ctx ext.Context) {
        // e.ToolCallID — matches the start event
        // e.ChunkType — "text", "tool_call", "tool_execution_start", "tool_result"
        // e.Content — text content
        // e.ToolName — tool name (for tool chunks)
        // e.IsError — true if tool result failed
        widget := subagentWidgets[e.ToolCallID]
        if widget != nil {
            widget.AddOutput(e)
            ctx.SetWidget(widget.Config())
        }
    })

    // Subagent completed
    api.OnSubagentEnd(func(e ext.SubagentEndEvent, ctx ext.Context) {
        // e.Response — final response from subagent
        // e.ErrorMsg — error message if subagent failed
        widget := subagentWidgets[e.ToolCallID]
        if widget != nil {
            widget.MarkComplete(e.Response, e.ErrorMsg)
            ctx.SetWidget(widget.Config())
            delete(subagentWidgets, e.ToolCallID)
        }
    })
}
```

**Event structs:**

```go
type SubagentStartEvent struct {
    ToolCallID string  // Unique ID for this subagent invocation
    Task       string  // The task/prompt sent to subagent
}

type SubagentChunkEvent struct {
    ToolCallID string  // Matches SubagentStartEvent.ToolCallID
    Task       string  // Task description
    ChunkType  string  // "text", "tool_call", "tool_execution_start", "tool_result"
    Content    string  // For text chunks
    ToolName   string  // For tool-related chunks
    IsError    bool    // For tool_result chunks
}

type SubagentEndEvent struct {
    ToolCallID string  // Matches start event
    Task       string  // Task description
    Response   string  // Final response from subagent
    ErrorMsg   string  // Error message if failed
}
```

This enables building monitoring widgets that display real-time activity from all subagents spawned by the main agent.

## Go SDK subagents

The SDK provides in-process subagent spawning:

```go
result, err := host.Subagent(ctx, kit.SubagentConfig{
    Task:         "Summarize the changes in this PR",
    Model:        "anthropic/claude-haiku-latest",
    SystemPrompt: "You are a code reviewer.",
    Timeout:      5 * time.Minute,
})
```

### Real-time subagent events

Use `SubscribeSubagent` to receive real-time events from LLM-initiated subagents (i.e., when the model uses the `subagent` tool). Register inside an `OnToolCall` handler using the tool call ID:

```go
host.OnToolCall(func(e kit.ToolCallEvent) {
    if e.ToolName == "subagent" {
        host.SubscribeSubagent(e.ToolCallID, func(event kit.Event) {
            switch ev := event.(type) {
            case kit.MessageUpdateEvent:
                fmt.Print(ev.Chunk) // streaming text from child
            case kit.ToolCallEvent:
                fmt.Printf("Child calling: %s\n", ev.ToolName)
            case kit.ToolResultEvent:
                fmt.Printf("Child result: %s\n", ev.ToolName)
            }
        })
    }
})
```

The listener receives the same event types as `Subscribe()` (`ToolCallEvent`, `MessageUpdateEvent`, `ReasoningDeltaEvent`, etc.) but scoped to the child agent's activity. Listeners are cleaned up automatically when the subagent completes.

If no listeners are registered for a tool call, no event dispatching overhead is incurred.
