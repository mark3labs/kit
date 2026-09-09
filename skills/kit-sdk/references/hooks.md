# Kit SDK: Hook System (Interceptors)

> Part of the `kit-sdk` skill. Read `SKILL.md` first for the overview and critical constraints.

## Hook System (Interceptors)

Hooks can **modify or cancel** operations. Events are read-only; hooks are read-write.

### BeforeToolCall — block tool execution

```go
unsub := host.OnBeforeToolCall(kit.HookPriorityNormal, func(h kit.BeforeToolCallHook) *kit.BeforeToolCallResult {
    // h.ToolCallID, h.ToolName, h.ToolArgs
    if h.ToolName == "bash" {
        return &kit.BeforeToolCallResult{Block: true, Reason: "bash disabled"}
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

### PrepareStep — intercept/replace messages before each LLM call

```go
host.OnPrepareStep(kit.HookPriorityNormal, func(h kit.PrepareStepHook) *kit.PrepareStepResult {
    // h.StepNumber  — which step in the current turn (1-based)
    // h.Messages    — []kit.LLMMessage being sent to the LLM
    // Return nil to pass through unchanged, or replace messages:
    modified := filterSensitiveMessages(h.Messages)
    return &kit.PrepareStepResult{Messages: modified}
})
```

`PrepareStep` fires before every LLM API call within a turn (including tool-call loop iterations). Unlike `ContextPrepare` (which operates on the full context window once per turn), `PrepareStep` runs per-step and sees the messages that include the latest tool results.

### ContextPrepare — filter/inject context window

```go
host.OnContextPrepare(kit.HookPriorityNormal, func(h kit.ContextPrepareHook) *kit.ContextPrepareResult {
    // h.Messages — []kit.LLMMessage (the full context being sent to the LLM)
    // Return nil to pass through, or replace entire context:
    return &kit.ContextPrepareResult{Messages: filteredMessages}
})
```

### BeforeCompact — cancel or customize compaction

```go
host.OnBeforeCompact(kit.HookPriorityNormal, func(h kit.BeforeCompactHook) *kit.BeforeCompactResult {
    // h.EstimatedTokens, h.ContextLimit, h.UsagePercent, h.MessageCount, h.IsAutomatic
    if h.IsAutomatic && h.UsagePercent < 0.9 {
        return &kit.BeforeCompactResult{Cancel: true, Reason: "not yet"}
    }
    return nil
})
```

### Hook priorities

```go
kit.HookPriorityHigh   = 0   // runs first
kit.HookPriorityNormal = 50  // default
kit.HookPriorityLow    = 100 // runs last
```

Lower values run first. Within the same priority, registration order applies. First non-nil result wins.

