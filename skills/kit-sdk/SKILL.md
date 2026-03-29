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

    // Session
    SessionDir:  "/path/to/project",  // base dir for session discovery (default: cwd)
    SessionPath: "/path/to/session.jsonl", // open specific session file
    Continue:    true,                // resume most recent session for SessionDir
    NoSession:   true,                // ephemeral in-memory session, no disk persistence

    // Tools
    Tools:      []kit.Tool{kit.NewBashTool()}, // REPLACES entire default tool set
    ExtraTools: []kit.Tool{myTool},            // ADDS alongside core/MCP/extension tools

    // Skills
    Skills:    []string{"/path/to/skill.md"}, // explicit skill files (empty = auto-discover)
    SkillsDir: "/path/to/skills",             // override project-local skills dir

    // Compaction
    AutoCompact:       true,                        // auto-compact near context limit
    CompactionOptions: &kit.CompactionOptions{...}, // nil = defaults
})
```

**Critical distinction**: `Tools` replaces ALL default tools (core + MCP + extension). `ExtraTools` adds tools alongside the defaults. Use `Tools` to restrict the agent's capabilities; use `ExtraTools` to extend them.

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
// result.FinalUsage   — tokens from last API call only
// result.Messages     — full updated conversation ([]kit.LLMMessage)
```

### Multimodal with file attachments

```go
import "charm.land/fantasy"

files := []fantasy.FilePart{{
    Name:      "screenshot.png",
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

host.OnToolResult(func(e kit.ToolResultEvent) {
    // e.ToolCallID, e.ToolName, e.ToolKind, e.ToolArgs, e.ParsedArgs
    // e.Result, e.IsError, e.Metadata (*ToolResultMetadata)
})

host.OnToolOutput(func(e kit.ToolOutputEvent) {
    // e.ToolCallID, e.ToolName, e.Chunk, e.IsStderr
    // Streaming bash output chunks
})

host.OnStreaming(func(e kit.MessageUpdateEvent) {
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
```

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

### ContextPrepare — filter/inject context window

```go
host.OnContextPrepare(kit.HookPriorityNormal, func(h kit.ContextPrepareHook) *kit.ContextPrepareResult {
    // h.Messages — []fantasy.Message (the full context being sent to the LLM)
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
providers := kit.GetFantasyProviders()   // providers usable with fantasy
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

---

## Context & Compaction

```go
tokens := host.EstimateContextTokens()  // heuristic token count
shouldCompact := host.ShouldCompact()    // true if near context limit

stats := host.GetContextStats()
// stats.EstimatedTokens — uses API-reported count when available (more accurate)
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

// LLM types (from charm.land/fantasy)
kit.LLMMessage, kit.LLMUsage, kit.LLMResponse

// Compaction types
kit.CompactionResult, kit.CompactionOptions

// Conversion helpers
msgs := kit.ConvertToFantasyMessages(&msg)   // SDK message → fantasy messages
msg := kit.ConvertFromFantasyMessage(fMsg)    // fantasy message → SDK message
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
host.OnStreaming(func(e kit.MessageUpdateEvent) {
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
