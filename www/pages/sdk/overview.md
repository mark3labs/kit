---
title: Go SDK
description: Embed Kit in your Go applications.
---

# Go SDK

The `pkg/kit` package lets you embed Kit as a library in your Go applications.

## Installation

```bash
go get github.com/mark3labs/kit/pkg/kit
```

## Basic usage

```go
package main

import (
    "context"
    "log"

    kit "github.com/mark3labs/kit/pkg/kit"
)

func main() {
    ctx := context.Background()

    // Create Kit instance with default configuration
    host, err := kit.New(ctx, nil)
    if err != nil {
        log.Fatal(err)
    }
    defer host.Close()

    // Send a prompt
    response, err := host.Prompt(ctx, "What is 2+2?")
    if err != nil {
        log.Fatal(err)
    }

    println(response)
}
```

## Multi-turn conversations

Conversations retain context automatically across calls:

```go
host.Prompt(ctx, "My name is Alice")
response, _ := host.Prompt(ctx, "What's my name?")
// response: "Your name is Alice"
```

## Additional prompt methods

The SDK provides several prompt variants:

| Method | Description |
|--------|-------------|
| `Prompt(ctx, message)` | Simple prompt, returns response string |
| `PromptWithOptions(ctx, message, opts)` | With per-call options |
| `PromptResult(ctx, message)` | Returns full `TurnResult` with usage stats |
| `PromptResultWithFiles(ctx, message, files)` | Multimodal with file attachments |
| `Steer(ctx, instruction)` | System-level steering without user message |
| `FollowUp(ctx, text)` | Continue without new user input |

## Custom tools

Create custom tools with `kit.NewTool`. The JSON schema is auto-generated from the input struct — no external dependencies required:

```go
type WeatherInput struct {
    City string `json:"city" description:"City name"`
}

weatherTool := kit.NewTool("get_weather", "Get current weather for a city",
    func(ctx context.Context, input WeatherInput) (kit.ToolOutput, error) {
        return kit.TextResult("72°F, sunny in " + input.City), nil
    },
)

host, _ := kit.New(ctx, &kit.Options{
    ExtraTools: []kit.Tool{weatherTool},
})
```

Struct tags control the schema:

- `json:"name"` — parameter name
- `description:"..."` — description shown to the LLM
- `enum:"a,b,c"` — restrict valid values
- `omitempty` — marks the parameter as optional

Return values:

| Helper | Description |
|--------|-------------|
| `kit.TextResult(s)` | Successful text result |
| `kit.ErrorResult(s)` | Error result (LLM sees it as a tool error) |
| `kit.ImageResult(s, data, mediaType)` | Image result with binary data (e.g. `"image/png"`) |
| `kit.MediaResult(s, data, mediaType)` | Non-image media result (e.g. `"audio/mpeg"`) |

Binary data (images, audio, etc.) in `ToolOutput.Data` is automatically forwarded to the LLM when `MediaType` is set. For advanced use, return a `kit.ToolOutput` struct directly with `Data`, `MediaType`, and `Metadata` fields.

Use `kit.NewParallelTool` for tools that are safe to run concurrently. Use `kit.ToolCallIDFromContext(ctx)` to retrieve the LLM-assigned call ID for logging or tracing.

## Generation & provider overrides

SDK consumers can configure generation parameters and provider endpoints
entirely in-code via `Options`, without touching `.kit.yml` or `viper.Set()`:

```go
host, _ := kit.New(ctx, &kit.Options{
    Model:          "anthropic/claude-sonnet-4-5-20250929",
    MaxTokens:      16384,             // 0 = auto-resolve (env → config → per-model → floor)
    ThinkingLevel:  "high",            // "off" | "none" | "minimal" | "low" | "medium" | "high"
    Temperature:    ptrFloat32(0.2),   // nil = provider/per-model default
    ProviderAPIKey: os.Getenv("MY_SECRET"), // overrides pre-existing viper state
    ProviderURL:    "https://proxy.internal/v1",
})

func ptrFloat32(v float32) *float32 { return &v }
```

See [Options](/sdk/options#generation-parameters) for the full field reference,
including `TopP`, `TopK`, `FrequencyPenalty`, `PresencePenalty`, and `TLSSkipVerify`.

## Event system

Subscribe to events for monitoring:

```go
unsubscribe := host.OnToolCall(func(event kit.ToolCallEvent) {
    fmt.Println("Tool called:", event.Name)
})
defer unsubscribe()

host.OnToolResult(func(event kit.ToolResultEvent) {
    fmt.Println("Tool result:", event.Name)
})

host.OnMessageUpdate(func(event kit.MessageUpdateEvent) {
    fmt.Print(event.Chunk)
})
```

## Model management

Switch models at runtime:

```go
host.SetModel(ctx, "openai/gpt-4o")
info := host.GetModelInfo()
models := host.GetAvailableModels()
```

## Dynamic MCP servers

Add and remove MCP servers at runtime:

```go
n, err := host.AddMCPServer(ctx, "github", kit.MCPServerConfig{
    Command: []string{"npx", "-y", "@modelcontextprotocol/server-github"},
})
fmt.Printf("Loaded %d tools\n", n)

err = host.RemoveMCPServer("github")
servers := host.ListMCPServers() // []kit.MCPServerStatus
```

### In-process MCP servers

Register mcp-go servers running in the same process — zero subprocess overhead:

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
n, _ := host.AddInProcessMCPServer(ctx, "docs", mcpSrv)
```

## Runtime skills and context files

Kit auto-discovers skills and `AGENTS.md`-style context files during `New()`,
but multi-tenant hosts (chatbots, web services, per-user agents) often need
to swap these **after** construction. The runtime mutators below recompose
the system prompt and apply it to the agent so the next turn picks up the
updated instructions — no restart, no file shuffling.

```go
// Add a programmatic skill — no file on disk required.
host.AddSkill(&kit.Skill{
    Name:        "polite-french",
    Description: "Respond in French and always greet the user.",
    Content:     "Always reply in French. Open every response with 'Bonjour'.",
})

// Or load one from disk.
host.LoadAndAddSkill("/var/skills/refund-policy.md")

// Project context (AGENTS.md equivalents): inline content from a DB...
host.AddContextFileContent(
    fmt.Sprintf("session://%s/AGENTS.md", userID),
    rulesFromDB,
)
// ...or load from disk.
host.LoadAndAddContextFile("/etc/agents/tenant-acme.md")

// Remove individually when a session ends.
host.RemoveSkill("polite-french")
host.RemoveContextFile(fmt.Sprintf("session://%s/AGENTS.md", userID))

// Or replace the whole set in one call.
host.SetSkills(activeSkillsForUser)
host.SetContextFiles(activeContextForUser)

// Inspect current state (snapshot copies — safe to mutate).
skills := host.GetSkills()
ctxFiles := host.GetContextFiles()
```

Key points:

- **Auto-refresh.** Every `Add*` / `Remove*` / `Set*` call recomposes the system
  prompt against the captured base prompt (preserving per-model overrides and
  `--system-prompt` resolution) and pushes the result onto the agent. Call
  `host.RefreshSystemPrompt()` only if you mutate state through a different
  path and need to force a re-render.
- **Dedup keys.** Skills dedupe by `Name`; context files dedupe by `Path`.
  Re-adding the same key replaces the entry instead of appending a duplicate.
- **Path is opaque.** `ContextFile.Path` does not have to point at a real file
  — it's only used for dedup and for the `Instructions from: <Path>` header
  injected into the prompt. URIs like `session://user-123/AGENTS.md` work fine.
- **Thread safety.** All readers and mutators are safe to call concurrently
  from multiple goroutines; the underlying state is guarded by an internal
  `RWMutex`.
- **Init-time options still apply.** `Options.Skills`, `Options.SkillsDir`,
  `Options.NoSkills`, and `Options.NoContextFiles` continue to control the
  startup set; the runtime API mutates from whatever state `New()` produced.
  See [SDK options](/sdk/options#skills--configuration).

## MCP prompts and resources

Query prompts and resources exposed by connected MCP servers:

```go
// List and expand prompts
prompts := host.ListMCPPrompts()
result, _ := host.GetMCPPrompt(ctx, "server", "prompt-name", map[string]string{"key": "value"})

// List and read resources
resources := host.ListMCPResources()
content, _ := host.ReadMCPResource(ctx, "server", "file:///path")
```

## MCP tasks (long-running tools)

Kit advertises [MCP task support](https://modelcontextprotocol.io/specification/2025-11-25/basic/utilities/tasks)
during `initialize`, so cooperating servers can return a `taskId` immediately
and let Kit poll `tasks/get` / `tasks/result` until the operation completes.
This avoids HTTP/SSE proxy timeouts on long tools and gives you clean
cancellation via context.

```go
host, _ := kit.New(ctx, &kit.Options{
    MCPTaskMode: map[string]kit.MCPTaskMode{
        "build-server": kit.MCPTaskModeAlways,
    },
    MCPTaskProgress: func(p kit.MCPTaskProgress) {
        log.Printf("%s: %s", p.TaskID, p.Status)
    },
})

// Inspect / cancel in-flight tasks
tasks, _ := host.ListMCPTasks(ctx, "build-server")
_, _    = host.CancelMCPTask(ctx, "build-server", tasks[0].TaskID)
```

Defaults to `MCPTaskModeAuto` per server, so any existing MCP server keeps
its previous synchronous behaviour. See [SDK options → MCP Tasks](/sdk/options#mcp-tasks)
for the full surface.

## Context and compaction

Monitor and manage context usage:

```go
tokens := host.EstimateContextTokens()
stats := host.GetContextStats()

if host.ShouldCompact() {
    result, err := host.Compact(ctx, nil, "")
}
```

## In-process subagents

Spawn child Kit instances without subprocess overhead:

```go
result, err := host.Subagent(ctx, kit.SubagentConfig{
    Prompt:    "Analyze the test files",
    Model:     "anthropic/claude-haiku-3-5-20241022",
    NoSession: true,
    Timeout:   2 * time.Minute,
})
```

See [Options](/sdk/options), [Callbacks](/sdk/callbacks), and [Sessions](/sdk/sessions) for more details.
