# Kit SDK: Common Patterns

> Part of the `kit-sdk` skill. Read `SKILL.md` first for the overview and critical constraints.

## Common Patterns

### Pattern: Scripting / CLI pipe

Minimal program for automation — stdout-only output:

```go
host, _ := kit.New(ctx, &kit.Options{Quiet: true})
defer func() { _ = host.Close() }()

response, _ := host.Prompt(ctx, os.Args[1])
fmt.Println(response)
```

### Pattern: Long-running autonomous agent

Daemon that performs repeated independent tasks:

```go
host, _ := kit.New(ctx, &kit.Options{
    SystemPrompt: taskPrompt,
    Tools:        []kit.Tool{kit.NewShellTool()},
    NoSession:    true,
    Quiet:        true,
})
defer func() { _ = host.Close() }()

ticker := time.NewTicker(30 * time.Minute)
for {
    select {
    case <-ticker.C:
        host.ClearSession() // fresh context each iteration
        host.Prompt(ctx, "Perform the monitoring task")
    case <-ctx.Done():
        return
    }
}
```

### Pattern: Streaming output to terminal

```go
host.OnMessageUpdate(func(e kit.MessageUpdateEvent) {
    fmt.Print(e.Chunk)
})
response, _ := host.Prompt(ctx, "Write a poem")
```

### Pattern: Multi-turn conversation with memory

```go
host.Prompt(ctx, "My name is Alice")
response, _ := host.Prompt(ctx, "What's my name?")
// Session automatically maintains context across calls
fmt.Printf("Session: %s\n", host.GetSessionPath())
```

### Pattern: Tool execution monitoring

```go
host.OnToolCall(func(e kit.ToolCallEvent) {
    fmt.Printf("[%s] %s(%s)\n", e.ToolKind, e.ToolName, e.ToolArgs)
})
host.OnToolResult(func(e kit.ToolResultEvent) {
    status := "✓"
    if e.IsError { status = "✗" }
    fmt.Printf("[%s] %s %s\n", e.ToolKind, status, e.ToolName)
})
```

### Pattern: Guard rails with hooks

```go
// Block dangerous commands. The command-execution tool is named "shell"
// (kit.NewShellTool / the deprecated kit.NewBashTool both register it under
// that name). Parse ToolArgs instead of grepping the raw JSON, and fail closed
// on a parse error. A substring match is a convenience guard, not a security
// boundary: "rm -r -f" or a script file slip past it. Enforce an allowlist if
// you need a real control.
host.OnBeforeToolCall(kit.HookPriorityHigh, func(h kit.BeforeToolCallHook) *kit.BeforeToolCallResult {
    if h.ToolName != "shell" {
        return nil
    }
    var args struct{ Command string `json:"command"` }
    if err := json.Unmarshal([]byte(h.ToolArgs), &args); err != nil {
        return &kit.BeforeToolCallResult{Block: true, Reason: "unreadable shell input"}
    }
    if strings.Contains(args.Command, "rm -rf") { // illustrative only
        return &kit.BeforeToolCallResult{Block: true, Reason: "dangerous command"}
    }
    return nil
})

// Inject context before every turn
host.OnBeforeTurn(kit.HookPriorityNormal, func(h kit.BeforeTurnHook) *kit.BeforeTurnResult {
    context := "Current user: admin\nEnvironment: production"
    return &kit.BeforeTurnResult{InjectText: &context}
})
```

### Pattern: Parallel subagents

```go
var wg sync.WaitGroup
results := make([]*kit.SubagentResult, 3)

tasks := []string{"Analyze auth module", "Analyze database layer", "Analyze API routes"}
for i, task := range tasks {
    wg.Add(1)
    go func(idx int, t string) {
        defer wg.Done()
        results[idx], _ = host.Subagent(ctx, kit.SubagentConfig{
            Prompt:    t,
            NoSession: true,
            Timeout:   3 * time.Minute,
        })
    }(i, task)
}
wg.Wait()
```

### Pattern: Read-only analysis agent

```go
host, _ := kit.New(ctx, &kit.Options{
    SystemPrompt: "You are a code reviewer. Only read and analyze, never modify files.",
    Tools:        kit.ReadOnlyTools(),
})
```

