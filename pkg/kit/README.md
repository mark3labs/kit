# KIT SDK

The KIT SDK allows you to use KIT programmatically from Go applications without spawning OS processes.

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
    defer host.Close()
    
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
    SystemPrompt: "You are a helpful bot",   // Override system prompt
    ConfigFile:   "/path/to/config.yml",     // Use specific config file
    MaxSteps:     10,                        // Override max steps
    Streaming:    true,                      // Enable streaming
    Quiet:        true,                      // Suppress debug output
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

// Save session
host.SaveSession("./session.json")

// Load session later
host.LoadSession("./session.json")

// Clear session
host.ClearSession()
```

## API Reference

### Types

- `Kit` - Main SDK type
- `Options` - Configuration options
- `Message` - Conversation message
- `ToolCall` - Tool invocation details

### Methods

- `New(ctx, opts)` - Create new Kit instance
- `Prompt(ctx, message)` - Send message and get response
- `LoadSession(path)` - Load session from file
- `SaveSession(path)` - Save session to file
- `ClearSession()` - Clear conversation history
- `GetSessionManager()` - Get session manager for advanced usage
- `GetModelString()` - Get current model string
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
