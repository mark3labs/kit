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
