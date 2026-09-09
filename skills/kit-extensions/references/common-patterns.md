# Kit Extensions: Common Patterns

> Part of the `kit-extensions` skill. Read `SKILL.md` first for the overview and critical constraints.

## Common Patterns

### Pattern: Tool Call Blocking

Block operations by intercepting tool calls. The command-execution tool is named `shell` (it was `bash` in older builds). Fail closed when the input cannot be parsed, and treat a substring match as a convenience guard only — `rm -r -f`, `$(...)`, or a script file all get past it. A real security boundary must parse the command and enforce an allowlist.

```go
api.OnToolCall(func(tc ext.ToolCallEvent, ctx ext.Context) *ext.ToolCallResult {
    if tc.ToolName != "shell" {
        return nil
    }
    var input struct{ Command string `json:"command"` }
    if err := json.Unmarshal([]byte(tc.Input), &input); err != nil {
        // Fail closed: never run a command you could not inspect.
        return &ext.ToolCallResult{Block: true, Reason: "unreadable shell input: " + err.Error()}
    }
    // Illustrative only — not a security control (see note above).
    if strings.Contains(input.Command, "rm -rf") {
        return &ext.ToolCallResult{
            Block:  true,
            Reason: "Dangerous command blocked",
        }
    }
    return nil
})
```

### Pattern: System Prompt Injection

Augment the agent's behavior by injecting instructions:

```go
api.OnBeforeAgentStart(func(_ ext.BeforeAgentStartEvent, ctx ext.Context) *ext.BeforeAgentStartResult {
    prompt := "Always respond with bullet points."
    return &ext.BeforeAgentStartResult{SystemPrompt: &prompt}
})
```

### Pattern: Background Processing with SendMessage

Run work in a goroutine and inject results back:

```go
api.RegisterCommand(ext.CommandDef{
    Name: "run",
    Description: "Run a command in the background",
    Execute: func(args string, ctx ext.Context) (string, error) {
        go func() {
            out, err := exec.Command("sh", "-c", args).CombinedOutput()
            if err != nil {
                ctx.SendMessage(fmt.Sprintf("Command failed: %s\n%s", err, out))
                return
            }
            ctx.SendMessage(fmt.Sprintf("Command output:\n```\n%s\n```", out))
        }()
        return "Running in background...", nil
    },
})
```

### Pattern: Ephemeral Context Injection

Inject information into every LLM turn without persisting in session history:

```go
api.OnContextPrepare(func(e ext.ContextPrepareEvent, ctx ext.Context) *ext.ContextPrepareResult {
    data, err := os.ReadFile(".kit/context.md")
    if err != nil {
        return nil
    }
    injected := ext.ContextMessage{
        Index:   -1,  // -1 = new message, not from session
        Role:    "system",
        Content: string(data),
    }
    msgs := append([]ext.ContextMessage{injected}, e.Messages...)
    return &ext.ContextPrepareResult{Messages: msgs}
})
```

### Pattern: Live Widget Updates

Update a widget periodically from a goroutine. `OnSessionStart` fires for every session that is opened or created and `ext.Context` carries no cancellation signal, so retire the previous ticker with a mutex-guarded generation counter (the same pattern as `examples/extensions/status-footer.go`) or stale goroutines keep calling `ctx.SetWidget`:

```go
var (
    mu        sync.Mutex
    tickerGen int
)

api.OnSessionStart(func(_ ext.SessionStartEvent, ctx ext.Context) {
    mu.Lock()
    tickerGen++
    gen := tickerGen // this goroutine's generation
    mu.Unlock()

    go func() {
        ticker := time.NewTicker(time.Second)
        defer ticker.Stop()
        for range ticker.C {
            mu.Lock()
            stale := gen != tickerGen
            mu.Unlock()
            if stale {
                return // a newer session (or shutdown) superseded this ticker
            }
            ctx.SetWidget(ext.WidgetConfig{
                ID:        "clock",
                Placement: ext.WidgetAbove,
                Content:   ext.WidgetContent{Text: time.Now().Format("15:04:05")},
                Style:     ext.WidgetStyle{BorderColor: "#89b4fa"},
            })
        }
    }()
})

api.OnSessionShutdown(func(_ ext.SessionShutdownEvent, _ ext.Context) {
    mu.Lock()
    tickerGen++ // retire the running ticker
    mu.Unlock()
})
```

### Pattern: Custom Theme with Slash Command

Register a theme and provide a slash command shortcut to activate it:

```go
api.OnSessionStart(func(_ ext.SessionStartEvent, ctx ext.Context) {
    ctx.RegisterTheme("neon", ext.ThemeColorConfig{
        Primary:    ext.ThemeColor{Light: "#CC00FF", Dark: "#FF00FF"},
        Secondary:  ext.ThemeColor{Light: "#0088CC", Dark: "#00FFFF"},
        Success:    ext.ThemeColor{Light: "#00CC44", Dark: "#00FF66"},
        Warning:    ext.ThemeColor{Light: "#CCAA00", Dark: "#FFFF00"},
        Error:      ext.ThemeColor{Light: "#CC0033", Dark: "#FF0055"},
        Info:       ext.ThemeColor{Light: "#0088CC", Dark: "#00CCFF"},
        Text:       ext.ThemeColor{Light: "#111111", Dark: "#F0F0F0"},
        Background: ext.ThemeColor{Light: "#F0F0F0", Dark: "#0A0A14"},
    })
})

api.RegisterCommand(ext.CommandDef{
    Name:        "neon",
    Description: "Switch to the neon cyberpunk theme",
    Execute: func(args string, ctx ext.Context) (string, error) {
        if err := ctx.SetTheme("neon"); err != nil {
            return "", err
        }
        return "Neon theme activated!", nil
    },
})
```

### Pattern: Spawning Kit as a Sub-Agent

Use `ctx.SpawnSubagent` to spawn an in-process child Kit instance. The subagent gets its own session, event bus, and agent loop, inherits the parent's active tools minus the `subagent` tool (no recursion), and does not load extensions. Its session is persisted by default (set `NoSession: true` for ephemeral runs).

**Blocking mode** — waits for completion:

```go
_, result, err := ctx.SpawnSubagent(ext.SubagentConfig{
    Prompt:       "Analyze the test files and summarize coverage",
    Model:        "anthropic/claude-haiku-3-5-20241022",  // empty = parent's model
    SystemPrompt: "You are a test analysis expert.",
    Timeout:      2 * time.Minute,  // 0 = 5 minute default
    Blocking:     true,
})
if err != nil {
    ctx.PrintError("spawn failed: " + err.Error())
    return
}
if result.Error != nil {
    ctx.PrintError("subagent failed: " + result.Error.Error())
    return
}
ctx.PrintInfo("Result:\n" + result.Response)
// result.Elapsed, result.ExitCode, result.SessionID
// result.Usage.InputTokens, result.Usage.OutputTokens (if available)
```

**Background mode** — returns immediately with a handle:

```go
handle, _, err := ctx.SpawnSubagent(ext.SubagentConfig{
    Prompt: "Write unit tests for UserService",
    OnOutput: func(chunk string) {
        // Live assistant text chunks
    },
    OnEvent: func(event ext.SubagentEvent) {
        // Real-time events: "text", "reasoning", "tool_call",
        // "tool_result", "tool_execution_start", "tool_execution_end",
        // "turn_start", "turn_end"
        // event.Type, event.Content, event.ToolName, event.ToolArgs, etc.
    },
    OnComplete: func(result ext.SubagentResult) {
        ctx.SendMessage("Subagent finished:\n" + result.Response)
    },
})
// handle.Kill()        — terminate the subagent
// handle.Wait()        — block until completion, returns SubagentResult
// <-handle.Done()      — channel that closes on completion
```

**SubagentConfig fields:**

| Field | Type | Description |
|-------|------|-------------|
| `Prompt` | string | Task instruction (required) |
| `Model` | string | Override model ("provider/model"), empty = parent's |
| `SystemPrompt` | string | Custom system prompt, empty = default |
| `Timeout` | time.Duration | Execution limit, 0 = 5 minutes |
| `Blocking` | bool | true = wait and return result; false (default) = background goroutine + handle |
| `NoSession` | bool | Don't persist subagent session file |
| `SessionID` | string | Resume an existing subagent session (from a previous `SubagentResult.SessionID`) for multi-turn follow-ups |
| `ParentSessionID` | string | Override the parent session link (optional; defaults to the host's active persisted session) |
| `OnOutput` | func(string) | Live assistant text chunk callback |
| `OnEvent` | func(SubagentEvent) | Real-time event callback |
| `OnComplete` | func(SubagentResult) | Completion callback |

**SubagentResult fields:**

| Field | Type | Description |
|-------|------|-------------|
| `Response` | string | Final text response |
| `Error` | error | Non-nil on failure |
| `ExitCode` | int | 0 on success, 1 on failure (runs in-process, mirrors Error) |
| `Elapsed` | time.Duration | Total execution time |
| `Usage` | *SubagentUsage | Token usage (InputTokens, OutputTokens) |
| `SessionID` | string | Subagent's session ID (if persisted) |

You can also spawn Kit as a raw subprocess for simpler cases:

```bash
kit --quiet --no-session --no-extensions --system-prompt "You are a reviewer" --model anthropic/claude-sonnet-4-20250514 "Review this code"
```

