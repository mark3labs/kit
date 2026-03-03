<p align="center">
  <img src="logo.jpg" alt="KIT" width="400">
</p>

<p align="center">
  <a href="https://github.com/mark3labs/kit/actions/workflows/ci.yml"><img src="https://github.com/mark3labs/kit/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="https://github.com/mark3labs/kit/releases/latest"><img src="https://img.shields.io/github/v/release/mark3labs/kit?style=flat&color=blue" alt="Release"></a>
  <a href="https://www.npmjs.com/package/@mark3labs/kit"><img src="https://img.shields.io/npm/v/@mark3labs/kit?style=flat&color=cb3837" alt="npm"></a>
  <a href="https://pkg.go.dev/github.com/mark3labs/kit"><img src="https://pkg.go.dev/badge/github.com/mark3labs/kit.svg" alt="Go Reference"></a>
  <a href="https://github.com/mark3labs/kit/blob/master/LICENSE"><img src="https://img.shields.io/github/license/mark3labs/kit?style=flat" alt="License"></a>
  <a href="https://discord.gg/RqSS2NQVsY"><img src="https://img.shields.io/badge/Discord-community-5865F2?style=flat&logo=discord&logoColor=white" alt="Discord"></a>
</p>

# KIT (Knowledge Inference Tool)

A powerful, extensible AI coding agent CLI with multi-provider support, built-in tools, and a rich extension system.

## Features

- **Multi-Provider LLM Support**: Anthropic, OpenAI, Google Gemini, Ollama, Azure OpenAI, AWS Bedrock, OpenRouter, and more
- **Built-in Core Tools**: bash, read, write, edit, grep, find, ls - no MCP overhead
- **MCP Integration**: Connect external MCP servers for expanded capabilities
- **Extension System**: Write custom tools, commands, widgets, and UI modifications in Go
- **Interactive TUI**: Rich terminal interface powered by Bubble Tea with streaming, syntax highlighting, and custom rendering
- **Session Management**: Tree-based conversation history with branching support
- **Non-Interactive Mode**: Script-friendly `--prompt` mode with JSON output
- **Go SDK**: Embed Kit in your own applications

## Installation

### Using npm (recommended)

```bash
npm install -g @mark3labs/kit
```

### Using Go

```bash
go install github.com/mark3labs/kit/cmd/kit@latest
```

### Building from source

```bash
git clone https://github.com/mark3labs/kit.git
cd kit
go build -o kit ./cmd/kit
```

## Quick Start

### Basic Usage

```bash
# Start interactive session
kit

# Run a one-off prompt
kit --prompt "List files in src/"

# Continue the most recent session
kit --continue

# Use specific model
kit --model anthropic/claude-sonnet-4-5-20250929
```

### Non-Interactive Mode

```bash
# Get JSON output for scripting
kit --prompt "Explain main.go" --json

# Quiet mode (final response only)
kit --quiet --prompt "Run tests"

# Ephemeral mode (no session file)
kit --prompt "Quick question" --no-session
```

## Configuration

Kit looks for configuration in the following locations (in order of priority):

1. CLI flags
2. Environment variables (with `KIT_` prefix)
3. `./.kit.yml` (project-local)
4. `~/.kit.yml` (global)

### Basic Configuration

Create `~/.kit.yml`:

```yaml
model: anthropic/claude-sonnet-4-5-20250929
max-tokens: 4096
temperature: 0.7
stream: true
```

### Environment Variables

```bash
export ANTHROPIC_API_KEY="sk-..."
export OPENAI_API_KEY="sk-..."
export KIT_MODEL="openai/gpt-4o"
```

### MCP Server Configuration

Add external MCP servers to `.kit.yml`:

```yaml
mcpServers:
  filesystem:
    type: local
    command: ["npx", "-y", "@modelcontextprotocol/server-filesystem", "/path/to/allowed"]
    environment:
      LOG_LEVEL: "info"
    allowedTools: ["read_file", "write_file"]
  
  search:
    type: remote
    url: "https://mcp.example.com/search"
```

## CLI Reference

### Global Flags

```bash
# Model and provider
--model, -m              Model to use (provider/model format)
--provider-api-key       API key for the provider
--provider-url           Base URL for provider API
--tls-skip-verify        Skip TLS certificate verification

# Session management
--session, -s            Open specific JSONL session file
--continue, -c           Resume most recent session for current directory
--resume, -r             Interactive session picker
--no-session             Ephemeral mode, no persistence

# Behavior
--prompt, -p             Run in non-interactive mode with given prompt
--quiet                  Suppress all output (only with --prompt)
--json                   Output response as JSON (only with --prompt)
--no-exit                Continue to interactive mode after --prompt
--max-steps              Maximum agent steps (0 for unlimited)
--stream                 Enable streaming output (default: true)
--compact                Enable compact output mode
--auto-compact           Auto-compact conversation near context limit

# Extensions
--extension, -e          Load additional extension file(s) (repeatable)
--no-extensions          Disable all extensions

# Generation parameters
--max-tokens             Maximum tokens in response (default: 4096)
--temperature            Randomness 0.0-1.0 (default: 0.7)
--top-p                  Nucleus sampling 0.0-1.0 (default: 0.95)
--top-k                  Limit top K tokens (default: 40)
--stop-sequences         Custom stop sequences (comma-separated)

# System
--config                 Config file path (default: ~/.kit.yml)
--system-prompt          System prompt text or file path
--debug                  Enable debug logging
```

### Commands

```bash
# Authentication (for OAuth-enabled providers)
kit auth login           # Start OAuth flow
kit auth logout          # Remove credentials
kit auth status          # Check authentication status

# Model database
kit models               # List available models
kit models --all         # Show all providers (not just Fantasy-compatible)
kit update-models        # Update local model database from models.dev

# Extension management
kit extensions list      # List discovered extensions
kit extensions validate  # Validate extension files
kit extensions init      # Generate example extension template
```

## Extension System

Extensions are Go source files that run via Yaegi interpreter. They can add custom tools, slash commands, widgets, keyboard shortcuts, and intercept lifecycle events.

### Minimal Extension

```go
//go:build ignore

package main

import "kit/ext"

func Init(api ext.API) {
    api.OnSessionStart(func(_ ext.SessionStartEvent, ctx ext.Context) {
        ctx.SetFooter(ext.HeaderFooterConfig{
            Content: ext.WidgetContent{Text: "Custom Footer"},
        })
    })
}
```

**Usage:**

```bash
kit -e examples/extensions/minimal.go
```

### Extension Capabilities

**Lifecycle Events**: OnSessionStart, OnSessionShutdown, OnAgentStart, OnAgentEnd, OnToolCall, OnToolResult, OnInput, OnMessageStart, OnMessageUpdate, OnMessageEnd, OnModelChange, OnContextPrepare, OnBeforeFork, OnBeforeSessionSwitch, OnBeforeCompact

**Custom Components**:
- **Tools**: Add new tools the LLM can invoke
- **Commands**: Register slash commands (e.g., `/mycommand`)
- **Widgets**: Persistent status displays above/below input
- **Shortcuts**: Global keyboard shortcuts
- **Overlays**: Modal dialogs with markdown content
- **Tool Renderers**: Customize how tool calls display
- **Editor Interceptors**: Handle key events and wrap rendering

### Extension Examples

See the `examples/extensions/` directory:

- `minimal.go` - Clean UI with custom footer
- `notify.go` - Desktop notifications
- `widget-status.go` - Persistent status widgets
- `custom-editor-demo.go` - Vim-like modal editor
- `prompt-demo.go` - Interactive prompts (select/confirm/input)
- `tool-logger.go` - Log all tool calls
- `overlay-demo.go` - Modal dialogs
- `plan-mode.go` - Read-only planning mode
- `subagent-widget.go` - Multi-agent orchestration
- `auto-commit.go` - Auto-commit on shutdown

### Loading Extensions

**Auto-discovery** (loads automatically):
- `./.kit/extensions/*.go` (project-local)
- `~/.config/kit/extensions/*.go` (global)

**Explicit loading**:
```bash
kit -e path/to/extension.go
kit -e ext1.go -e ext2.go  # Multiple extensions
```

**Disable auto-load**:
```bash
kit --no-extensions
```

## Session Management

Kit uses a tree-based session model that supports branching and forking conversations.

### Session Locations

- Default: `~/.local/share/kit/sessions/<cwd-hash>/<uuid>.jsonl`
- Each line is a session entry (messages, tool calls, extension data)
- Supports branching from any message to explore alternate paths

### Session Commands

```bash
# Resume most recent session for current directory
kit --continue
kit -c

# Interactive session picker
kit --resume
kit -r

# Open specific session file
kit --session path/to/session.jsonl
kit -s path/to/session.jsonl

# Ephemeral mode (no file persistence)
kit --no-session
```

## Go SDK

Embed Kit in your Go applications:

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

### With Options

```go
host, err := kit.New(ctx, &kit.Options{
    Model:        "ollama/llama3",
    SystemPrompt: "You are a helpful bot",
    ConfigFile:   "/path/to/config.yml",
    MaxSteps:     10,
    Streaming:    true,
    Quiet:        true,
})
```

### With Callbacks

```go
response, err := host.PromptWithCallbacks(
    ctx,
    "List files in current directory",
    func(name, args string) {
        // Tool call started
        println("Calling tool:", name)
    },
    func(name, args, result string, isError bool) {
        // Tool call completed
        if isError {
            println("Tool failed:", name)
        }
    },
    func(chunk string) {
        // Streaming text chunk
        print(chunk)
    },
)
```

### Session Management

```go
host.Prompt(ctx, "My name is Alice")
response, _ := host.Prompt(ctx, "What's my name?")

host.SaveSession("./session.json")
host.LoadSession("./session.json")
host.ClearSession()
```

## Advanced Usage

### Subagent Pattern

Spawn Kit as a subprocess for multi-agent orchestration:

```bash
kit --prompt "Analyze codebase" \
    --json \
    --no-session \
    --no-extensions \
    --quiet \
    --model anthropic/claude-haiku-3-5-20241022
```

Parse the JSON output:

```json
{
  "response": "Final assistant response text",
  "model": "anthropic/claude-haiku-3-5-20241022",
  "usage": {
    "input_tokens": 1024,
    "output_tokens": 512,
    "total_tokens": 1536
  },
  "messages": [...]
}
```

### Testing with tmux

Test the TUI non-interactively:

```bash
# Start Kit in detached tmux session
tmux new-session -d -s kittest -x 120 -y 40 \
  "kit -e ext.go --no-session 2>kit.log"

# Wait for startup
sleep 3

# Capture screen
tmux capture-pane -t kittest -p

# Send input
tmux send-keys -t kittest '/command' Enter

# Cleanup
tmux kill-session -t kittest
```

## Development

### Build and Test

```bash
# Build
go build -o output/kit ./cmd/kit

# Run tests
go test -race ./...

# Run specific test
go test -race ./cmd -run TestScriptExecution

# Lint
go vet ./...

# Format
go fmt ./...
```

### Project Structure

```
cmd/kit/           - CLI entry point
cmd/               - CLI command implementations
pkg/kit/           - Go SDK
internal/agent/    - Agent loop and tool execution
internal/ui/       - Bubble Tea TUI components
internal/extensions/ - Yaegi extension system
internal/core/     - Built-in tools
internal/tools/    - MCP tool integration
internal/config/   - Configuration management
internal/session/  - Session persistence
internal/models/   - Provider and model management
examples/extensions/ - Example extension files
```

## Supported Providers

- **Anthropic** - Claude models (native, prompt caching, OAuth)
- **OpenAI** - GPT models
- **Google** - Gemini models
- **Ollama** - Local models
- **Azure OpenAI** - Azure-hosted OpenAI
- **AWS Bedrock** - Bedrock models
- **Google Vertex** - Claude on Vertex AI
- **OpenRouter** - Multi-provider router
- **Vercel AI** - Vercel AI SDK models
- **Auto-routed** - Any provider from models.dev database

### Model String Format

```bash
provider/model            # Standard format
anthropic/claude-sonnet-4-5-20250929
openai/gpt-4o
ollama/llama3
google/gemini-2.0-flash-exp
```

### Model Aliases

```bash
claude-opus-latest      → claude-opus-4-20250514
claude-sonnet-latest    → claude-sonnet-4-5-20250929
claude-3-5-haiku-latest → claude-3-5-haiku-20241022
```

## Contributing

Contributions are welcome! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

[Apache 2.0](LICENSE)

## Community

- [Discord](https://discord.gg/RqSS2NQVsY)
- [GitHub Issues](https://github.com/mark3labs/kit/issues)
- [Documentation](https://github.com/mark3labs/kit/wiki)
