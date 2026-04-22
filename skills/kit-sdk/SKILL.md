---
name: kit-sdk
description: Guide for building Go applications with the Kit SDK. Use when the user asks to create a program, service, script, or application that uses Kit programmatically as a Go library — e.g. embedding LLM interactions, building agents, creating CLI tools powered by Kit, or integrating Kit into backend services. Do NOT use for Kit extensions (use kit-extensions skill instead).
---

# Kit SDK Development Guide

The Kit SDK (`pkg/kit`) lets you embed Kit's full agent capabilities — LLM interactions, tool execution, session management, streaming, hooks — into any Go application. Unlike extensions (which are interpreted scripts running inside Kit's TUI), SDK programs are standalone compiled Go binaries.

## Installation

```bash
go get github.com/mark3labs/kit
```

Import path (alias recommended):

```go
import kit "github.com/mark3labs/kit/pkg/kit"
```

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"

    kit "github.com/mark3labs/kit/pkg/kit"
)

func main() {
    ctx := context.Background()

    host, err := kit.New(ctx, nil) // nil = load ~/.kit.yml defaults
    if err != nil {
        log.Fatal(err)
    }
    defer func() { _ = host.Close() }()

    response, err := host.Prompt(ctx, "What is 2+2?")
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println(response)
}
```

## Core Lifecycle

1. **Create**: `kit.New(ctx, opts)` — loads config, initializes MCP servers, creates LLM provider, sets up agent
2. **Interact**: `host.Prompt(ctx, msg)` — send messages, agent uses tools as needed
3. **Close**: `host.Close()` — cleans up MCP connections, model resources, session file handle

Always defer `Close()`:

```go
defer func() { _ = host.Close() }()
```

---

## Options Reference

All fields are optional. Zero values use CLI defaults.

```go
host, err := kit.New(ctx, &kit.Options{
    // Model
    Model:        "anthropic/claude-sonnet-4-5-20250929", // "provider/model" format
    SystemPrompt: "You are a helpful assistant",
    ConfigFile:   "/path/to/config.yml",                  // default: ~/.kit.yml

    // Behavior
    MaxSteps:  10,   // 0 = unlimited tool-calling steps
    Streaming: true, // stream LLM output (default from config)
    Quiet:     true, // suppress debug output
    Debug:     true, // enable debug logging

    // Generation parameters — override env/config/per-model defaults.
    // Leaving a field at its zero/nil value lets the precedence chain
    // resolve a value (KIT_* env → .kit.yml → modelSettings/customModels →
    // 8192 floor for MaxTokens, provider defaults for samplers).
    MaxTokens:        16384,             // 0 = auto-resolve; non-zero suppresses right-sizing
    ThinkingLevel:    "medium",          // "off", "none", "minimal", "low", "medium", "high" ("" = default)
    Temperature:      ptrFloat32(0.2),   // pointer so explicit 0.0 != unset
    TopP:             nil,                // nil = leave provider/per-model default
    TopK:             nil,                // nil = leave provider/per-model default
    FrequencyPenalty: nil,
    PresencePenalty:  nil,

    // Provider configuration — override env/config without viper.Set workarounds.
    ProviderAPIKey: "sk-...",                    // "" = use config / provider env var
    ProviderURL:    "https://proxy.internal/v1", // "" = provider default endpoint
    TLSSkipVerify:  false,                       // true only; can't force-disable via Options

    // Session
    SessionDir:  "/path/to/project",  // base dir for session discovery (default: cwd)
    SessionPath: "/path/to/session.jsonl", // open specific session file
    Continue:    true,                // resume most recent session for SessionDir
    NoSession:   true,                // ephemeral in-memory session, no disk persistence
    SessionManager: myCustomSession,  // custom SessionManager implementation (advanced)

    // Tools
    Tools:            []kit.Tool{kit.NewBashTool()}, // REPLACES entire default tool set
    ExtraTools:       []kit.Tool{myTool},            // ADDS alongside core/MCP/extension tools
    DisableCoreTools: true,                        // Use no core tools (0 tools, for chat-only)

    // Configuration
    SkipConfig:   true,                        // Skip .kit.yml files (viper defaults + env vars still apply)

    // Skills
    Skills:    []string{"/path/to/skill.md"}, // explicit skill files (empty = auto-discover)
    SkillsDir: "/path/to/skills",             // override project-local skills dir
    NoSkills:  true,                          // disable skill loading entirely

    // Feature toggles
    NoExtensions:   true,                     // disable Yaegi extension loading entirely
    NoContextFiles: true,                     // disable automatic AGENTS.md loading

    // Compaction
    AutoCompact:       true,                        // auto-compact near context limit
    CompactionOptions: &kit.CompactionOptions{...}, // nil = defaults

    // MCP OAuth — both fields are opt-in. If MCPAuthHandler is nil,
    // remote MCP servers that require OAuth will fail to connect with
    // an authorization-required error instead of silently opening a
    // browser. CLI consumers use NewCLIMCPAuthHandler; other embedders
    // implement MCPAuthHandler or configure DefaultMCPAuthHandler.
    MCPAuthHandler: mcpAuthHandler,             // nil = OAuth disabled
    MCPTokenStoreFactory: func(serverURL string) (kit.MCPTokenStore, error) {
        return myCustomStore(serverURL), nil  // custom OAuth token storage
    },

    // In-Process MCP Servers
    InProcessMCPServers: map[string]*kit.MCPServer{
        "docs": mcpSrv,  // *server.MCPServer from mcp-go — no subprocess needed
    },
})

// Tiny helper to take the address of a literal for pointer fields.
func ptrFloat32(v float32) *float32 { return &v }
```

**Critical distinction**: `Tools` replaces ALL default tools (core + MCP + extension). `ExtraTools` adds tools alongside the defaults. Use `Tools` to restrict the agent's capabilities; use `ExtraTools` to extend them.

**In-process MCP servers** bypass subprocess spawning entirely. Pass `*server.MCPServer` instances from mcp-go via `InProcessMCPServers` or call `AddInProcessMCPServer()` at runtime.

### Generation & provider Options (cheat sheet)

| Field | Type | Empty/nil means | Notes |
|-------|------|-----------------|-------|
| `MaxTokens` | `int` | Auto-resolve (env → config → per-model → 8192 floor) | Non-zero suppresses `rightSizeMaxTokens` |
| `ThinkingLevel` | `string` | Auto-resolve (→ `"off"`) | Valid: `"off"`, `"none"`, `"minimal"`, `"low"`, `"medium"`, `"high"` |
| `Temperature` | `*float32` | Leave provider/per-model default | Pointer so explicit `0.0` ≠ unset |
| `TopP` | `*float32` | Leave provider/per-model default | |
| `TopK` | `*int32` | Leave provider/per-model default | |
| `FrequencyPenalty` | `*float32` | Leave provider/per-model default | OpenAI-family |
| `PresencePenalty` | `*float32` | Leave provider/per-model default | OpenAI-family |
| `ProviderAPIKey` | `string` | Use config / provider env var | Overrides pre-existing viper state |
| `ProviderURL` | `string` | Use provider default endpoint | Same base URL flag as `--provider-url` |
| `TLSSkipVerify` | `bool` | — | Only effective when `true`; cannot force-disable via Options |

These fields eliminate the old `viper.Set("max-tokens", 16384)` dance many
downstream embedders used to do before calling `kit.New()`. Everything is
now discoverable via godoc on `kit.Options`.

---

## Prompt Methods

### Simple prompt — string in, string out

```go
response, err := host.Prompt(ctx, "Explain this code")
```

### Full result with usage stats

```go
result, err := host.PromptResult(ctx, "Analyze this file")
// result.Response     — assistant's text
// result.StopReason   — "stop", "length", "tool-calls", "error", etc.
// result.SessionID    — session UUID
// result.TotalUsage   — aggregate tokens across all steps (*kit.LLMUsage)
//                        LLMUsage{InputTokens, OutputTokens, TotalTokens,
//                                 ReasoningTokens, CacheCreationTokens, CacheReadTokens}
// result.FinalUsage   — tokens from last API call only (*kit.LLMUsage)
//                        For context window fill, sum: InputTokens + CacheReadTokens +
//                        CacheCreationTokens + OutputTokens (with prompt caching,
//                        InputTokens alone understates the context)
// result.Messages     — full updated conversation ([]kit.LLMMessage)
//                        LLMMessage{Role kit.LLMMessageRole, Content string}
```

### Multimodal with file attachments

```go
files := []kit.LLMFilePart{{
    Filename:  "screenshot.png",
    MediaType: "image/png",
    Data:      imageBytes,
}}
result, err := host.PromptResultWithFiles(ctx, "What's in this image?", files)
```

### Per-call system message injection

```go
response, err := host.PromptWithOptions(ctx, "Review this PR", kit.PromptOptions{
    SystemMessage: "Focus on security vulnerabilities only.",
})
```

### System-level steering (no visible user message)

```go
response, err := host.Steer(ctx, "Switch to a more formal tone")
```

### Continue without new input

```go
response, err := host.FollowUp(ctx, "") // empty = "Continue."
```

### Multiple user messages in one turn

```go
result, err := host.PromptResultWithMessages(ctx, []string{
    "Here is the code:",
    "@file.go", // content from earlier
    "Please review it.",
})
```

---

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

---

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

---

## Tools

### Creating custom tools

Use `kit.NewTool` to create custom tools. The JSON schema is auto-generated from the input struct — no external dependencies required:

```go
type WeatherInput struct {
    City string `json:"city" description:"City name, e.g. 'San Francisco'"`
}

weatherTool := kit.NewTool("get_weather", "Get current weather for a city",
    func(ctx context.Context, input WeatherInput) (kit.ToolOutput, error) {
        // Your logic here (API calls, database lookups, etc.)
        return kit.TextResult("72°F, sunny in " + input.City), nil
    },
)

host, _ := kit.New(ctx, &kit.Options{
    ExtraTools: []kit.Tool{weatherTool},
})
```

**Struct tags** control the generated schema:

| Tag | Purpose | Example |
|-----|---------|---------|
| `json:"name"` | Parameter name | `json:"city"` |
| `description:"..."` | Description shown to the LLM | `description:"City name"` |
| `enum:"a,b,c"` | Restrict valid values | `enum:"json,text,csv"` |
| `omitempty` | Marks parameter as optional | `json:"limit,omitempty"` |

**Return helpers:**

| Function | Description |
|----------|-------------|
| `kit.TextResult(content)` | Successful text result |
| `kit.ErrorResult(content)` | Error result (LLM sees it as a tool error) |
| `kit.ImageResult(content, data, mediaType)` | Image result with binary data (e.g. `"image/png"`) |
| `kit.MediaResult(content, data, mediaType)` | Non-image media result (e.g. `"audio/mpeg"`) |

**ToolOutput fields** (for advanced use):

```go
kit.ToolOutput{
    Content:   "result text",     // text returned to the LLM
    IsError:   false,             // true = LLM sees this as an error
    Data:      pngBytes,          // optional binary data (images, audio)
    MediaType: "image/png",       // MIME type for binary Data
    Metadata:  map[string]any{},  // opaque metadata for hooks/UI (not sent to LLM)
}
```

**Parallel tools** — mark as safe for concurrent execution:

```go
searchTool := kit.NewParallelTool("search", "Search the web",
    func(ctx context.Context, input SearchInput) (kit.ToolOutput, error) {
        return kit.TextResult("results..."), nil
    },
)
```

**Tool call ID** — available in context for logging/tracing:

```go
tool := kit.NewTool("my_tool", "...",
    func(ctx context.Context, input MyInput) (kit.ToolOutput, error) {
        callID := kit.ToolCallIDFromContext(ctx) // correlation ID from the LLM
        log.Printf("[%s] my_tool called", callID)
        return kit.TextResult("ok"), nil
    },
)
```

### Built-in tool constructors

```go
kit.NewReadTool(opts...)  // file reading
kit.NewWriteTool(opts...) // file writing
kit.NewEditTool(opts...)  // surgical text editing
kit.NewBashTool(opts...)  // bash command execution
kit.NewGrepTool(opts...) // content search (uses ripgrep when available)
kit.NewFindTool(opts...) // file search (uses fd when available)
kit.NewLsTool(opts...)   // directory listing
```

### Tool bundles

```go
kit.AllTools(opts...)       // all 7 core tools
kit.CodingTools(opts...)    // bash, read, write, edit
kit.ReadOnlyTools(opts...)  // read, grep, find, ls
kit.SubagentTools(opts...)  // all except subagent (prevents recursion)
```

### Tool options

```go
kit.WithWorkDir("/path/to/dir") // override working directory for file-based tools
```

### Using tools in Options

```go
// Restricted: agent can ONLY run bash
host, _ := kit.New(ctx, &kit.Options{
    Tools: []kit.Tool{kit.NewBashTool()},
})

// Extended: all defaults PLUS a custom tool
host, _ := kit.New(ctx, &kit.Options{
    ExtraTools: []kit.Tool{myCustomTool},
})
```

### Querying tools at runtime

```go
names := host.GetToolNames()       // []string of all tool names
tools := host.GetTools()           // []kit.Tool (full tool objects)
mcpCount := host.GetMCPToolCount() // tools from MCP servers
extCount := host.GetExtensionToolCount() // tools from extensions
ready := host.MCPToolsReady()      // true when async MCP tool loading is complete
```

---

## Session Management

Sessions automatically persist as JSONL tree files. No explicit save needed.

### Session modes (via Options)

| Mode | Options | Behavior |
|------|---------|----------|
| Default | `{}` | New session file for cwd |
| Specific file | `{SessionPath: "path.jsonl"}` | Open existing session |
| Continue | `{Continue: true}` | Resume most recent session for cwd |
| Ephemeral | `{NoSession: true}` | In-memory only, no disk persistence |
| Custom dir | `{SessionDir: "/path"}` | Base directory for session discovery |

### Instance methods

```go
host.GetSessionPath() // file path of active session
host.GetSessionID()   // UUID of active session
host.ClearSession()   // reset to fresh branch (doesn't delete file)
host.Branch("entry-id") // branch from a specific entry
host.SetSessionName("my session") // set display name

// Get conversation messages
msgs := host.GetSessionMessages()       // []extensions.SessionMessage (flattened text)
msgs := host.GetStructuredMessages()     // []kit.StructuredMessage (typed content parts)
```

### Package-level session operations (no Kit instance needed)

```go
sessions, _ := kit.ListSessions("/path/to/project") // sessions for a directory
sessions, _ := kit.ListAllSessions()                  // all sessions everywhere
kit.DeleteSession("/path/to/session.jsonl")
tm, _ := kit.OpenTreeSession("/path/to/session.jsonl") // open for direct access
```

### Custom Session Manager (Advanced)

You can provide a custom session manager to store conversation history in your own backend (database, cloud storage, etc.) instead of the default JSONL files.

```go
// Implement the SessionManager interface
type MyDatabaseSessionManager struct {
    db *sql.DB
    // ... other fields
}

func (s *MyDatabaseSessionManager) AppendMessage(msg kit.LLMMessage) (string, error) {
    // Store message in your database
}

func (s *MyDatabaseSessionManager) GetMessages() []kit.LLMMessage {
    // Retrieve messages from your database
}

// ... implement all other SessionManager methods

// Use with Kit
host, _ := kit.New(ctx, &kit.Options{
    SessionManager: myCustomSession,  // Your custom implementation
    Model: "anthropic/claude-sonnet-latest",
})
```

**SessionManager Interface:**

```go
type SessionManager interface {
    AppendMessage(msg kit.LLMMessage) (entryID string, err error)
    GetMessages() []kit.LLMMessage
    BuildContext() (messages []kit.LLMMessage, provider string, modelID string)
    Branch(entryID string) error
    GetCurrentBranch() []kit.BranchEntry
    GetChildren(parentID string) []string
    GetEntry(entryID string) *kit.BranchEntry
    GetSessionID() string
    GetSessionName() string
    SetSessionName(name string) error
    GetCreatedAt() time.Time
    IsPersisted() bool
    AppendCompaction(summary string, firstKeptEntryID string,
        tokensBefore, tokensAfter int, messagesRemoved int, readFiles, modifiedFiles []string) (string, error)
    GetLastCompaction() *kit.CompactionEntry
    AppendExtensionData(extType, data string) (string, error)
    GetExtensionData(extType string) []kit.ExtensionDataEntry
    AppendModelChange(provider, modelID string) (string, error)
    GetContextEntryIDs() []string
    Close() error
}
```

**Use Cases:**
- **PocketBase integration**: Store sessions as PocketBase records
- **Cloud storage**: Persist sessions to S3, GCS, or Azure Blob
- **Multi-user apps**: Store sessions per user in a database
- **Custom retention**: Implement your own session cleanup policies

**Note:** When using a custom SessionManager, the following Options are ignored:
- `SessionPath` - your manager handles its own storage
- `Continue` - your manager handles session selection
- `NoSession` - use an in-memory implementation instead

---

## Model Management

### At creation time

```go
host, _ := kit.New(ctx, &kit.Options{
    Model: "openai/gpt-4o",
})
```

### At runtime

```go
err := host.SetModel(ctx, "anthropic/claude-sonnet-4-5-20250929")
modelStr := host.GetModelString()   // "provider/model"
info := host.GetModelInfo()          // *kit.ModelInfo (capabilities, limits, pricing) or nil
isReasoning := host.IsReasoningModel()
level := host.GetThinkingLevel()
err = host.SetThinkingLevel(ctx, "medium") // recreates agent with new thinking budget
```

### Model registry

```go
models := host.GetAvailableModels()      // []extensions.ModelInfoEntry
providers := kit.GetSupportedProviders() // []string
providers := kit.GetLLMProviders()       // providers with LLM support
models, _ := kit.GetModelsForProvider("anthropic") // map[string]kit.ModelInfo
info := kit.LookupModel("anthropic", "claude-sonnet-4-5-20250929") // *kit.ModelInfo
info := kit.GetProviderInfo("openai")    // *kit.ProviderInfo (env vars, API URL)
err := kit.ValidateEnvironment("anthropic", "") // check API keys
suggestions := kit.SuggestModels("anthropic", "claudee") // fuzzy match
kit.RefreshModelRegistry() // reload model database
```

### Model string format

Always `"provider/model"`: `"anthropic/claude-sonnet-4-5-20250929"`, `"openai/gpt-4o"`, `"ollama/qwen3:8b"`.

```go
provider, modelID, err := kit.ParseModelString("anthropic/claude-sonnet-4-5-20250929")
```

### Per-model system prompts

Models can have per-model system prompts configured via `modelSettings` or `customModels` in `.kit.yml`. When the user hasn't explicitly set a system prompt (via `--system-prompt`, config, or `Options.SystemPrompt`), the per-model prompt is used as the base and composed with AGENTS.md context and skills.

On `SetModel()`, if the new model has a per-model system prompt and no custom global prompt was set, the per-model prompt automatically replaces the previous one.

### Per-model generation parameters

Models can define default generation parameters (`temperature`, `top_p`, `top_k`, `frequency_penalty`, `presence_penalty`) via `modelSettings` or `customModels` `params` in `.kit.yml`. These defaults apply when the user hasn't explicitly set the parameter. Explicit CLI flags or config values always take priority.

---

## Dynamic MCP Server Management

Add, remove, and inspect MCP servers at runtime without restarting Kit:

```go
// Add a new MCP server — tools become available immediately
n, err := host.AddMCPServer(ctx, "github", kit.MCPServerConfig{
    Command:     []string{"npx", "-y", "@modelcontextprotocol/server-github"},
    Environment: map[string]string{"GITHUB_TOKEN": os.Getenv("GITHUB_TOKEN")},
})
fmt.Printf("Loaded %d tools from github server\n", n)

// Remove an MCP server — its tools are no longer available
err = host.RemoveMCPServer("github")

// List all currently loaded MCP servers
servers := host.ListMCPServers()
for _, s := range servers {
    fmt.Printf("Server %s: %d tools\n", s.Name, s.ToolCount)
}
```

`AddMCPServer` is safe to call while the agent is idle. If a turn is in progress, new tools are visible starting from the next LLM step. Tool names are prefixed with the server name (e.g. `"github__create_issue"`).

### In-Process MCP Servers

Register mcp-go servers that run in the same process — no subprocess spawning,
no network I/O:

```go
import (
    "github.com/mark3labs/mcp-go/mcp"
    "github.com/mark3labs/mcp-go/server"
)

mcpSrv := server.NewMCPServer("my-tools", "1.0.0",
    server.WithToolCapabilities(true),
)
mcpSrv.AddTool(mcp.NewTool("search_docs",
    mcp.WithDescription("Search documentation"),
    mcp.WithString("query", mcp.Required()),
), searchHandler)

// At init time
host, _ := kit.New(ctx, &kit.Options{
    InProcessMCPServers: map[string]*kit.MCPServer{
        "docs": mcpSrv,
    },
})

// Or at runtime
n, err := host.AddInProcessMCPServer(ctx, "docs", mcpSrv)
```

Kit does not own the server lifecycle — the caller handles cleanup. Tools are prefixed as usual (e.g. `"docs__search_docs"`).

### MCP Prompts

Query and expand prompts defined by connected MCP servers:

```go
// List all prompts from all connected MCP servers
prompts := host.ListMCPPrompts()
for _, p := range prompts {
    fmt.Printf("%s/%s: %s\n", p.ServerName, p.Name, p.Description)
    for _, arg := range p.Arguments {
        fmt.Printf("  arg: %s (required: %v)\n", arg.Name, arg.Required)
    }
}

// Expand a specific prompt with arguments
result, err := host.GetMCPPrompt(ctx, "myserver", "code-review", map[string]string{
    "language": "go",
    "style":    "thorough",
})
// result.Description — optional server description
// result.Messages — []MCPPromptMessage with Role, Content, and FileParts
for _, msg := range result.Messages {
    fmt.Printf("[%s] %s\n", msg.Role, msg.Content)
    // msg.FileParts contains binary attachments (images, embedded resources)
}
```

### MCP Resources

Read and subscribe to resources exposed by MCP servers:

```go
// List all resources from connected servers
resources := host.ListMCPResources()
for _, r := range resources {
    fmt.Printf("%s: %s (%s)\n", r.URI, r.Name, r.MIMEType)
}

// Read a specific resource
content, err := host.ReadMCPResource(ctx, "myserver", "file:///path/to/file")
if content.IsBlob {
    // Binary content in content.BlobData
} else {
    // Text content in content.Text
}

// Subscribe to resource change notifications
err = host.SubscribeMCPResource(ctx, "myserver", "file:///path/to/file")
// Unsubscribe later
err = host.UnsubscribeMCPResource(ctx, "myserver", "file:///path/to/file")
```

### MCP OAuth Authorization

When a remote MCP server requires OAuth, Kit runs the full authorization flow
(dynamic client registration → PKCE → user consent → token exchange → token
persistence) but delegates the **user-facing step** — displaying the
authorization URL and receiving the callback — to an `MCPAuthHandler`.

The SDK ships three building blocks:

| Building block | When to use |
|---|---|
| **No handler** (`Options.MCPAuthHandler = nil`) | Default. OAuth is disabled; 401s from remote MCP servers surface as errors. Correct for library, daemon, and web-app embedders that don't want side effects. |
| **`kit.NewCLIMCPAuthHandler()`** | CLI/TUI apps. Opens the system browser, prints status to stderr (or via `NotifyFunc`), runs a localhost callback server. This is what the `kit` binary uses. |
| **`kit.NewDefaultMCPAuthHandler()` + `OnAuthURL`** | Custom UX. Get the transport mechanics (port reservation + callback server) from the SDK; wire your own presentation in the `OnAuthURL(serverName, authURL)` closure. |
| **Implement `kit.MCPAuthHandler` directly** | Full control. No localhost binding — e.g. return the URL from an HTTP endpoint and have the consumer POST the callback URL back. |

**CLI-style embedder (browser + stderr):**

```go
authHandler, err := kit.NewCLIMCPAuthHandler()
if err != nil {
    log.Fatal(err)
}
defer authHandler.Close() // release the reserved port

host, _ := kit.New(ctx, &kit.Options{
    MCPAuthHandler: authHandler,
})
```

**Custom UX embedder (TUI modal, QR code, web redirect, etc.):**

```go
authHandler, _ := kit.NewDefaultMCPAuthHandler()
authHandler.OnAuthURL = func(serverName, authURL string) {
    // Render the URL however you like — no browser or terminal assumptions.
    myUI.ShowAuthPrompt(serverName, authURL)
}
defer authHandler.Close()

host, _ := kit.New(ctx, &kit.Options{
    MCPAuthHandler: authHandler,
})
```

**Important:** `DefaultMCPAuthHandler` with no `OnAuthURL` set will silently
drop the authorization URL and block until the 2-minute callback timeout
fires. Always set `OnAuthURL`, or use a higher-level wrapper like
`CLIMCPAuthHandler`.

### MCP OAuth Token Storage

Once authorization succeeds, the resulting access/refresh tokens are persisted
by an `MCPTokenStore`. By default tokens are written to
`$XDG_CONFIG_HOME/.kit/mcp_tokens.json` (fallback `~/.config/.kit/mcp_tokens.json`),
keyed by server URL, with `0600` file permissions.

Provide a custom store for encrypted storage, database persistence, or
in-memory-only flows:

```go
host, _ := kit.New(ctx, &kit.Options{
    MCPTokenStoreFactory: func(serverURL string) (kit.MCPTokenStore, error) {
        return &MyDatabaseTokenStore{serverURL: serverURL}, nil
    },
})
```

The `MCPTokenStore` interface requires `GetToken`/`SetToken`/`DeleteToken` methods. Return `kit.ErrMCPNoToken` from `GetToken` when no token is stored.

---

## Context & Compaction

```go
tokens := host.EstimateContextTokens()  // heuristic token count
shouldCompact := host.ShouldCompact()    // true if near context limit
// ShouldCompact() uses API-reported token counts (including cache tokens)
// when available, falling back to text-based heuristic before the first turn.

stats := host.GetContextStats()
// stats.EstimatedTokens — uses API-reported count when available (more accurate;
//                          includes system prompts, tool definitions, cache tokens)
// stats.ContextLimit    — model's context window size
// stats.UsagePercent    — fraction used (0.0–1.0)
// stats.MessageCount    — number of messages

// Manual compaction
result, err := host.Compact(ctx, nil, "") // nil opts = defaults, "" = default prompt
// result.Summary, result.OriginalTokens, result.CompactedTokens, result.MessagesRemoved

// Auto-compaction via Options
host, _ := kit.New(ctx, &kit.Options{
    AutoCompact: true,
    CompactionOptions: &kit.CompactionOptions{
        ReserveTokens:   16384,
        KeepRecentTokens: 4096,
        ContextWindow:   200000,
    },
})
```

---

## In-Process Subagents

Spawn child Kit instances without subprocess overhead:

```go
result, err := host.Subagent(ctx, kit.SubagentConfig{
    Prompt:       "Analyze the test files and summarize coverage",
    Model:        "anthropic/claude-haiku-3-5-20241022", // empty = parent's model
    SystemPrompt: "You are a test analysis expert.",
    Tools:        nil,           // nil = SubagentTools() (all except subagent)
    NoSession:    true,          // ephemeral
    Timeout:      2 * time.Minute, // 0 = 5 minute default
    OnEvent: func(e kit.Event) {
        // Real-time events from the child agent
        if chunk, ok := e.(kit.MessageUpdateEvent); ok {
            fmt.Print(chunk.Chunk)
        }
    },
})
// result.Response, result.Error, result.SessionID, result.StopReason
// result.Usage (*kit.LLMUsage), result.Elapsed (time.Duration)
```

### Subscribing to subagent events from parent

```go
host.OnToolCall(func(e kit.ToolCallEvent) {
    if e.ToolName == "subagent" {
        host.SubscribeSubagent(e.ToolCallID, func(child kit.Event) {
            // Real-time events scoped to this subagent
        })
    }
})
```

---

## Extension API

The `Extensions()` method returns an `ExtensionAPI` interface that groups all extension-related functionality. This is the primary way to interact with extension state from the SDK.

```go
extAPI := host.Extensions()

// Check if extensions are loaded
if extAPI.HasExtensions() {
    // Context management
    extAPI.SetContext(extensions.Context{...})
    ctx := extAPI.GetContext()
    extAPI.UpdateContextModel("anthropic/claude-sonnet-4-5-20250929")

    // Widgets, headers, footers
    extAPI.SetWidget(extensions.WidgetConfig{...})
    extAPI.RemoveWidget("widget-id")
    extAPI.SetHeader(extensions.HeaderFooterConfig{...})
    extAPI.SetFooter(extensions.HeaderFooterConfig{...})

    // Status bar
    extAPI.SetStatus(extensions.StatusBarEntry{...})
    extAPI.RemoveStatus("key")

    // Options
    extAPI.SetOption("name", "value")
    val := extAPI.GetOption("name")

    // Tools
    tools := extAPI.GetToolInfos()
    extAPI.SetActiveTools([]string{"bash", "read"})

    // Events
    extAPI.EmitSessionStart()
    extAPI.EmitModelChange("new/model", "old/model", "extension")
    extAPI.EmitCustomEvent("my-event", "data")

    // Commands and lifecycle
    cmds := extAPI.Commands()
    err := extAPI.Reload()
}
```

All methods are no-ops when extensions are disabled (nil runner), so callers don't need nil checks.

---

## Authentication

```go
cm, _ := kit.NewCredentialManager()
hasKey := kit.HasAnthropicCredentials()
apiKey := kit.GetAnthropicAPIKey() // stored creds → ANTHROPIC_API_KEY env var
```

---

## Skills

```go
// Load a single skill file
skill, _ := kit.LoadSkill("/path/to/SKILL.md")
// skill.Name, skill.Description, skill.Content, skill.Path

// Load from directory
skills, _ := kit.LoadSkillsFromDir("/path/to/skills")

// Auto-discover (global + project-local)
skills, _ := kit.LoadSkills("/path/to/project")

// Prompt building with skills
pb := kit.NewPromptBuilder("You are an assistant")
pb.WithSkills(skills)
pb.WithSection("", "Extra context here")
systemPrompt := pb.Build()
```

---

## Re-exported Types

The SDK re-exports internal types so you don't need direct internal imports:

```go
// Message types
kit.Message, kit.MessageRole, kit.ContentPart
kit.TextContent, kit.ReasoningContent, kit.ToolCall, kit.ToolResult, kit.Finish
kit.RoleUser, kit.RoleAssistant, kit.RoleTool, kit.RoleSystem

// Session types
kit.SessionInfo, kit.TreeManager, kit.SessionHeader, kit.MessageEntry

// Config types
kit.Config, kit.MCPServerConfig

// Provider types
kit.ProviderConfig, kit.ProviderResult, kit.ModelInfo, kit.ModelCost, kit.ModelLimit

// LLM types — clean aliases (no external library dependency in consumer code)
kit.LLMMessage      // {Role LLMMessageRole, Content string}
kit.LLMMessagePart  // interface for message content parts
kit.LLMMessageRole  // "user" | "assistant" | "system" | "tool"
kit.LLMUsage        // {InputTokens, OutputTokens, TotalTokens, ReasoningTokens,
                     //  CacheCreationTokens, CacheReadTokens}
kit.LLMResponse     // {Content, FinishReason, Usage}
kit.LLMFilePart     // {Filename, Data []byte, MediaType}
kit.LLMTextPart     // plain-text content part
kit.LLMReasoningPart // reasoning/chain-of-thought content part
kit.LLMToolCall     // {ID, Name, Input string} — execution-layer tool call (for Tool.Run)
kit.LLMToolResponse // {Type, Content, Data, MediaType, IsError, ...} — raw tool response
kit.LLMToolCallPart    // LLM-initiated tool invocation within a message
kit.LLMToolResultPart  // tool result within a message
kit.LLMToolResultOutputContent      // interface for tool result output
kit.LLMToolResultOutputContentText  // text tool result
kit.LLMToolResultOutputContentError // error tool result
kit.LLMToolResultOutputContentMedia // media tool result {Data, MediaType, Text}
kit.LLMToolResultContentType        // "text" | "error" | "media"
kit.LLMToolInfo          // {Name, Description, Parameters, Required, Parallel}
kit.LLMProviderOptions   // provider-specific option maps (keyed by provider name)
kit.LLMProviderMetadata  // provider-specific response metadata
kit.LLMPrompt            // []LLMMessage — ordered prompt sequence
kit.LLMFinishReason      // "stop" | "length" | "tool-calls" | ...

// Compaction types
kit.CompactionResult, kit.CompactionOptions

// MCP OAuth types
kit.MCPAuthHandler         // interface: RedirectURI() + HandleAuth(ctx, server, authURL) for OAuth UX
kit.DefaultMCPAuthHandler  // SDK-provided transport mechanics (port + callback server); set OnAuthURL hook
kit.CLIMCPAuthHandler      // CLI wrapper around DefaultMCPAuthHandler: opens browser, prints status
kit.NewDefaultMCPAuthHandler()         // random port, no UX side effects
kit.NewDefaultMCPAuthHandlerWithPort() // fixed port (useful when registering a stable redirect URI)
kit.NewCLIMCPAuthHandler()             // CLI handler: browser + stderr + localhost callback
kit.MCPTokenStore        // interface for custom OAuth token storage
kit.MCPToken             // OAuth token struct (access, refresh, expiry)
kit.MCPTokenStoreFactory // func(serverURL string) (MCPTokenStore, error)
kit.ErrMCPNoToken        // sentinel error for "no token stored"
kit.MCPServer            // *server.MCPServer for in-process MCP transport
kit.MCPServerStatus      // {Name string, ToolCount int}
kit.MCPPrompt            // {Name, Description, Arguments []MCPPromptArgument, ServerName}
kit.MCPPromptArgument    // {Name, Description string, Required bool}
kit.MCPPromptMessage     // {Role, Content string, FileParts []LLMFilePart}
kit.MCPPromptResult      // {Description string, Messages []MCPPromptMessage}
kit.MCPResource          // {URI, Name, Description, MIMEType, ServerName}
kit.MCPResourceContent   // {URI, MIMEType, Text string, BlobData []byte, IsBlob bool}

// Conversion helpers
msgs := kit.ConvertToLLMMessages(&msg)   // SDK Message  → []LLMMessage
msg  := kit.ConvertFromLLMMessage(lMsg)  // LLMMessage   → SDK Message
```

---

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
    Tools:        []kit.Tool{kit.NewBashTool()},
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
// Block dangerous commands
host.OnBeforeToolCall(kit.HookPriorityHigh, func(h kit.BeforeToolCallHook) *kit.BeforeToolCallResult {
    if h.ToolName == "bash" && strings.Contains(h.ToolArgs, "rm -rf") {
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

---

## Configuration

The SDK loads config identically to the CLI:

1. Explicit `ConfigFile` in Options (highest priority)
2. `.kit.yml` in current directory
3. `~/.kit.yml` in home directory
4. Environment variables with `KIT_` prefix (`KIT_MODEL`, etc.)
5. Provider-specific env vars (`ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, etc.)

Config files support `${ENV_VAR}` expansion.

```go
// Initialize config manually (usually not needed — kit.New handles this)
kit.InitConfig("/path/to/config.yml", false)
kit.LoadConfigWithEnvSubstitution("/path/to/config.yml")
```

---

## Key Files for Reference

- [`pkg/kit/kit.go`](https://github.com/mark3labs/kit/blob/main/pkg/kit/kit.go) — Kit struct, New(), Prompt methods, Subagent, Close
- [`pkg/kit/extension_api.go`](https://github.com/mark3labs/kit/blob/main/pkg/kit/extension_api.go) — ExtensionAPI interface, kit.Extensions() accessor
- [`pkg/kit/types.go`](https://github.com/mark3labs/kit/blob/main/pkg/kit/types.go) — Re-exported types from internal packages
- [`pkg/kit/tools.go`](https://github.com/mark3labs/kit/blob/main/pkg/kit/tools.go) — Tool constructors and bundles
- [`pkg/kit/events.go`](https://github.com/mark3labs/kit/blob/main/pkg/kit/events.go) — Event types, EventBus, typed subscribers
- [`pkg/kit/hooks.go`](https://github.com/mark3labs/kit/blob/main/pkg/kit/hooks.go) — Hook system (BeforeToolCall, AfterToolResult, etc.)
- [`pkg/kit/sessions.go`](https://github.com/mark3labs/kit/blob/main/pkg/kit/sessions.go) — Session management
- [`pkg/kit/compaction.go`](https://github.com/mark3labs/kit/blob/main/pkg/kit/compaction.go) — Context compaction
- [`pkg/kit/models.go`](https://github.com/mark3labs/kit/blob/main/pkg/kit/models.go) — Model registry lookups
- [`pkg/kit/config.go`](https://github.com/mark3labs/kit/blob/main/pkg/kit/config.go) — Config initialization and defaults
- [`pkg/kit/skills.go`](https://github.com/mark3labs/kit/blob/main/pkg/kit/skills.go) — Skills loading and prompt building
- [`pkg/kit/auth.go`](https://github.com/mark3labs/kit/blob/main/pkg/kit/auth.go) — Credential management
- [`examples/sdk/`](https://github.com/mark3labs/kit/tree/main/examples/sdk) — Working example programs
