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
    Tools:        []kit.Tool{kit.NewBashTool()}, // Replace default tool set
    ExtraTools:   []kit.Tool{myTool},            // Add alongside defaults

    // Compaction
    AutoCompact:  true,                       // Auto-compact near context limit
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

unsub3 := host.OnStreaming(func(e kit.MessageUpdateEvent) {
    fmt.Print(e.Chunk)
})
defer unsub3()

response, err := host.Prompt(
    ctx,
    "List files in the current directory",
)
```

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

## Environment Variables

All CLI environment variables work with the SDK:

- `KIT_MODEL` - Override model
- `ANTHROPIC_API_KEY` - Anthropic API key
- `OPENAI_API_KEY` - OpenAI API key
- `GEMINI_API_KEY` - Google API key
- etc.

## License

Same as KIT CLI
