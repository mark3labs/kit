# KIT SDK

The KIT SDK (`pkg/kit`) lets you embed Kit's full agent capabilities — LLM interactions, tool execution, session management, streaming, hooks — into any Go application.

## Installation

```bash
go get github.com/mark3labs/kit
```

## Basic Usage

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

    // Create Kit instance with default configuration
    host, err := kit.New(ctx, nil)
    if err != nil {
        log.Fatal(err)
    }
    defer func() { _ = host.Close() }()

    // Send a prompt
    response, err := host.Prompt(ctx, "What is 2+2?")
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println(response)
}
```

## Configuration

The SDK behaves identically to the CLI:
- Loads configuration from `~/.kit.yml` by default
- Creates default configuration if none exists
- Respects all environment variables (`KIT_*`)
- Uses the same defaults as the CLI

### Options

You can override specific settings:

```go
host, err := kit.New(ctx, &kit.Options{
    Model:        "ollama/llama3",            // Override model
    SystemPrompt: "You are a helpful bot",    // Override system prompt
    ConfigFile:   "/path/to/config.yml",      // Use specific config file
    MaxSteps:     10,                         // Override max steps
    Streaming:    true,                       // Enable streaming
    Quiet:        true,                       // Suppress debug output

    // Session options
    SessionPath:  "./session.jsonl",          // Open specific session
    Continue:     true,                       // Resume most recent session
    NoSession:    true,                       // Ephemeral mode

    // Tool options
    Tools:            []kit.Tool{kit.NewBashTool()}, // Replace default tool set
    ExtraTools:       []kit.Tool{myTool},            // Add alongside defaults
    DisableCoreTools: true,                        // Use no core tools (0 tools)

    // Configuration
    SkipConfig:   true,                        // Skip .kit.yml files (viper defaults + env vars still apply)

    // Compaction
    AutoCompact:  true,                       // Auto-compact near context limit

    // In-process MCP servers (map name → *kit.MCPServer)
    InProcessMCPServers: map[string]*kit.MCPServer{
        "docs": mcpSrv,
    },
})
```

## Advanced Usage

### With Tool Callbacks

Monitor tool execution in real-time:

```go
unsub := host.OnToolCall(func(e kit.ToolCallEvent) {
    fmt.Printf("Calling tool: %s\n", e.ToolName)
})
defer unsub()

unsub2 := host.OnToolResult(func(e kit.ToolResultEvent) {
    if e.IsError {
        fmt.Printf("Tool %s failed: %s\n", e.ToolName, e.Result)
    } else {
        fmt.Printf("Tool %s succeeded\n", e.ToolName)
    }
})
defer unsub2()

unsub3 := host.OnMessageUpdate(func(e kit.MessageUpdateEvent) {
    fmt.Print(e.Chunk)
})
defer unsub3()

response, err := host.Prompt(
    ctx,
    "List files in the current directory",
)
```

### Dynamic MCP Server Management

Add, remove, and list MCP servers at runtime:

```go
// Add an MCP server at runtime
n, err := host.AddMCPServer(ctx, "github", kit.MCPServerConfig{
    Command: "npx",
    Args:    []string{"-y", "@modelcontextprotocol/server-github"},
})
fmt.Printf("Loaded %d tools from MCP server\n", n)

// List connected MCP servers
for _, s := range host.ListMCPServers() {
    fmt.Printf("%s: %d tools\n", s.Name, s.ToolCount)
}

// Disconnect a server and remove its tools
host.RemoveMCPServer("github")
```

### In-Process MCP Servers

Register mcp-go servers that run in the same process — no subprocess spawning,
no network I/O. This is ideal for custom tool servers implemented in Go:

```go
import (
    "github.com/mark3labs/mcp-go/mcp"
    "github.com/mark3labs/mcp-go/server"
)

// Create an mcp-go server with tools
mcpSrv := server.NewMCPServer("my-tools", "1.0.0",
    server.WithToolCapabilities(true),
)
mcpSrv.AddTool(mcp.NewTool("search_docs",
    mcp.WithDescription("Search documentation"),
    mcp.WithString("query", mcp.Required()),
), searchHandler)

// Option 1: At init time via Options
host, _ := kit.New(ctx, &kit.Options{
    InProcessMCPServers: map[string]*kit.MCPServer{
        "docs": mcpSrv,
    },
})

// Option 2: At runtime
n, err := host.AddInProcessMCPServer(ctx, "docs", mcpSrv)
fmt.Printf("Loaded %d tools from in-process server\n", n)
```

Kit does not take ownership of the server's lifecycle — the caller is responsible for any cleanup. In-process server tools are prefixed the same way as external MCP servers (e.g. `"docs__search_docs"`).

### MCP Prompts

MCP servers can expose prompt templates via the MCP prompts capability.
Kit exposes these through the SDK:

```go
// List prompts from all connected MCP servers
prompts := host.ListMCPPrompts()
for _, p := range prompts {
    fmt.Printf("%s/%s: %s\n", p.Server, p.Name, p.Description)
}

// Get a specific prompt with arguments
msg, err := host.GetMCPPrompt(ctx, "server-name", "prompt-name", map[string]string{
    "topic": "concurrency",
})
```

### MCP Tasks (long-running tools)

Kit advertises [MCP task support](https://modelcontextprotocol.io/specification/2025-11-25/basic/utilities/tasks)
during `initialize`. Cooperating servers can respond to `tools/call` with a
`taskId` immediately; Kit then polls `tasks/get` / `tasks/result` until the
task reaches a terminal state, and best-effort `tasks/cancel`s on context
cancellation. Servers that don't advertise the capability keep their previous
synchronous behaviour.

```go
host, _ := kit.New(ctx, &kit.Options{
    // Per-server mode: auto (default), never, or always.
    MCPTaskMode: map[string]kit.MCPTaskMode{
        "build-server": kit.MCPTaskModeAlways,
    },
    MCPTaskTimeout:  15 * time.Minute, // total wall-clock cap
    MCPTaskProgress: func(p kit.MCPTaskProgress) {
        log.Printf("%s/%s: %s", p.Server, p.TaskID, p.Status)
    },
})

// Inspect / cancel in-flight tasks
tasks, _ := host.ListMCPTasks(ctx, "build-server")
t, _    := host.GetMCPTask(ctx, "build-server", tasks[0].TaskID)
if !t.Status.IsTerminal() {
    _, _ = host.CancelMCPTask(ctx, "build-server", t.TaskID)
}
```

The progress handler fires once when a task is accepted and again on every
observed status transition; the final invocation always carries a terminal
status (`MCPTaskStatusCompleted`, `MCPTaskStatusFailed`, or
`MCPTaskStatusCancelled`). Don't block in the handler — dispatch long work on
a goroutine.

### Session Management

Maintain conversation context:

```go
// First message
host.Prompt(ctx, "My name is Alice")

// Second message (remembers context)
response, _ := host.Prompt(ctx, "What's my name?")
// Response: "Your name is Alice"

// Clear conversation history
host.ClearSession()
```

## Re-exported Types

The SDK re-exports types so you don't need direct internal imports:

```go
// Message types
kit.Message, kit.MessageRole, kit.ContentPart
kit.TextContent, kit.ReasoningContent, kit.ToolCall, kit.ToolResult, kit.Finish
kit.RoleUser, kit.RoleAssistant, kit.RoleTool, kit.RoleSystem

// LLM types — concrete Kit-owned structs, no external library dependency
kit.LLMMessage      // {Role LLMMessageRole, Content string}
kit.LLMMessageRole  // "user" | "assistant" | "system" | "tool"
kit.LLMUsage        // {InputTokens, OutputTokens, TotalTokens, ...}
kit.LLMResponse     // {Content, FinishReason, Usage}
kit.LLMFilePart     // {Filename, Data []byte, MediaType}

// MCP OAuth types
kit.MCPServer            // *server.MCPServer for in-process MCP transport
kit.MCPServerConfig      // Configuration for an MCP server (stdio, SSE, or in-process)
kit.MCPAuthHandler       // Interface: handles user-facing OAuth authorization
kit.DefaultMCPAuthHandler // Port + callback-server mechanics; set OnAuthURL for presentation
kit.CLIMCPAuthHandler    // CLI wrapper: opens browser, prints status
kit.MCPTokenStore        // Persists OAuth tokens for a single MCP server
kit.MCPToken             // OAuth token (access token, refresh token, expiry)
kit.MCPTokenStoreFactory // Creates an MCPTokenStore for a given server URL

// Conversion helpers
msgs := kit.ConvertToLLMMessages(&msg)   // SDK Message → []LLMMessage
msg  := kit.ConvertFromLLMMessage(lMsg)  // LLMMessage  → SDK Message
```

## API Reference

### Types

- `Kit` - Main SDK type
- `Options` - Configuration options
- `Message` - Conversation message with typed content parts
- `Tool` - Agent tool interface
- `TurnResult` - Full result from a prompt including usage stats

### Key Methods

- `New(ctx, opts)` - Create new Kit instance
- `Prompt(ctx, message)` - Send message and get response string
- `PromptResult(ctx, message)` - Send message and get full TurnResult
- `PromptWithOptions(ctx, message, opts)` - Prompt with per-call options
- `Steer(ctx, instruction)` - System-level steering
- `FollowUp(ctx, text)` - Continue without new user input
- `SetModel(ctx, model)` - Switch model at runtime
- `GetModelString()` - Get current model string
- `GetModelInfo()` - Get model capabilities and limits
- `ClearSession()` - Clear conversation history
- `GetSessionPath()` - Get session file path
- `GetSessionID()` - Get session UUID
- `Close()` - Clean up resources

### Options

Key `Options` fields for SDK usage:

| Field | Description |
|-------|-------------|
| `Model` | Override model (e.g., "anthropic/claude-sonnet-4-5-20250929") |
| `SystemPrompt` | Override system prompt |
| `ConfigFile` | Load specific config file (empty = search defaults) |
| `SkipConfig` | Skip `.kit.yml` loading (defaults + env vars still apply) |
| `Tools` | Replace core tools with custom set |
| `ExtraTools` | Add tools alongside defaults |
| `DisableCoreTools` | Use no core tools (0 tools, for chat-only) |
| `NoSession` | Ephemeral mode (no session persistence) |
| `SessionPath` | Open specific session file |
| `Continue` | Resume most recent session |
| `InProcessMCPServers` | Map of name → `*kit.MCPServer` for in-process MCP servers |
| `Debug` | Enable debug logging |

## Environment Variables

All CLI environment variables work with the SDK:

- `KIT_MODEL` - Override model
- `ANTHROPIC_API_KEY` - Anthropic API key
- `OPENAI_API_KEY` - OpenAI API key
- `GEMINI_API_KEY` - Google API key
- etc.

## License

Same as KIT CLI
