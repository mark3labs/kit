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
- **Built-in Core Tools**: bash (with interactive sudo password prompt), read, write, edit, grep, find, ls, subagent - no MCP overhead
- **Smart @ Attachments**: Binary files auto-detected via MIME type, MCP resources via `@mcp:server:uri`
- **MCP Integration**: Connect external MCP servers for expanded capabilities
- **Extension System**: Write custom tools, commands, widgets, and UI modifications in Go
- **Theming**: 22 built-in color themes (KITT, Catppuccin, Dracula, Nord, etc.) with runtime switching, persistence, and custom theme files
- **Model Persistence**: Model and thinking level selections are automatically saved and restored across sessions
- **Prompt Templates**: Create reusable prompt templates with shell-style argument substitution
- **Interactive TUI**: Rich terminal interface powered by Bubble Tea with streaming, syntax highlighting, and custom rendering
- **Session Management**: Tree-based conversation history with branching support
- **Non-Interactive Mode**: Script-friendly positional args with JSON output
- **ACP Server**: Run Kit as an [Agent Client Protocol](https://agentclientprotocol.com) agent over stdio
- **Go SDK**: Embed Kit in your own applications

## Installation

### Using npm / bun / pnpm

```bash
npm install -g @mark3labs/kit
# or
bun install -g @mark3labs/kit
# or
pnpm install -g @mark3labs/kit
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
kit "List files in src/"

# Attach files as context
kit @main.go @test.go "Review these files"

# Continue the most recent session
kit --continue

# Model and thinking level selections are automatically persisted
# across sessions and restored on next launch

# Use specific model
kit --model anthropic/claude-sonnet-latest
```

### Non-Interactive Mode

```bash
# Get JSON output for scripting
kit "Explain main.go" --json

# Quiet mode (final response only)
kit "Run tests" --quiet

# Ephemeral mode (no session file)
kit "Quick question" --no-session
```

### ACP Server Mode

Kit can run as an [ACP (Agent Client Protocol)](https://agentclientprotocol.com) agent server, enabling ACP-compatible clients (such as [OpenCode](https://github.com/sst/opencode)) to drive Kit as a remote coding agent over stdio.

```bash
# Start Kit as an ACP server (communicates via JSON-RPC 2.0 on stdin/stdout)
kit acp

# With debug logging to stderr
kit acp --debug
```

The ACP server exposes Kit's full capabilities — LLM execution, tool calls (bash, read, write, edit, grep, etc.), and session persistence — over the standard ACP protocol. Sessions are persisted to Kit's normal JSONL session files, so they can be resumed later.

## Configuration

Kit looks for configuration in the following locations (in order of priority):

1. CLI flags
2. Environment variables (with `KIT_` prefix)
3. `./.kit.yml` / `./.kit.yaml` / `./.kit.json` (project-local)
4. `~/.kit.yml` / `~/.kit.yaml` / `~/.kit.json` (global)

### Basic Configuration

Create `~/.kit.yml`:

```yaml
model: anthropic/claude-sonnet-latest
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

# Behavior (non-interactive: pass prompt as positional arg)
--quiet                  Suppress all output (non-interactive only)
--json                   Output response as JSON (non-interactive only)
--no-exit                Enter interactive mode after prompt completes
--max-steps              Maximum agent steps (0 for unlimited)
--stream                 Enable streaming output (default: true)
--compact                Enable compact output mode
--auto-compact           Auto-compact conversation near context limit

# Extensions
--extension, -e          Load additional extension file(s) (repeatable)
--no-extensions          Disable all extensions
--prompt-template        Load a specific prompt template by name
--no-prompt-templates    Disable prompt template loading

# Generation parameters
--max-tokens             Maximum tokens in response (default: 4096)
--temperature            Randomness 0.0-1.0 (default: 0.7)
--top-p                  Nucleus sampling 0.0-1.0 (default: 0.95)
--top-k                  Limit top K tokens (default: 40)
--stop-sequences         Custom stop sequences (comma-separated)
--frequency-penalty      Penalize frequent tokens 0.0-2.0 (default: 0.0)
--presence-penalty       Penalize present tokens 0.0-2.0 (default: 0.0)
--thinking-level         Extended thinking level: off, minimal, low, medium, high (default: off)

# System
--config                 Config file path (default: ~/.kit.yml)
--system-prompt          System prompt text or file path
--debug                  Enable debug logging
```

### Commands

```bash
# Authentication (for OAuth-enabled providers)
kit auth login [provider]    # Start OAuth flow (e.g., anthropic)
kit auth logout [provider]   # Remove credentials for provider
kit auth status              # Check authentication status

# Model database
kit models [provider]        # List available models (optionally filter by provider)
kit models --all             # Show all providers (not just LLM-compatible)
kit update-models [source]   # Update model database (from models.dev, URL, file, or 'embedded')

# Extension management
kit extensions list          # List discovered extensions
kit extensions validate      # Validate extension files
kit extensions init          # Generate example extension template
kit install <git-url>        # Install extensions from git repositories
kit install -l <git-url>     # Install to project-local .kit/git/ directory
kit install -u <git-url>     # Update an already-installed package
kit install --uninstall <pkg> # Remove an installed package

# Skills
kit skill                    # Install the Kit extensions skill via skills.sh

# ACP server
kit acp                      # Start as ACP agent (stdio JSON-RPC)
kit acp --debug              # With debug logging to stderr
```

## Themes

Kit ships with 22 built-in color themes that control all UI elements. Switch at runtime:

```
/theme dracula
/theme catppuccin
/theme tokyonight
```

Theme selections are automatically saved and restored on next launch (stored in `~/.config/kit/preferences.yml`). This persistence also applies to **model** and **thinking level** selections — all are saved together and restored on startup.

### Custom themes

Drop a `.yml` file in `~/.config/kit/themes/` (user) or `.kit/themes/` (project):

```yaml
# ~/.config/kit/themes/my-theme.yml
primary:
  light: "#8839ef"
  dark: "#cba6f7"
success:
  light: "#40a02b"
  dark: "#a6e3a1"
```

Built-in themes: `kitt`, `catppuccin`, `dracula`, `tokyonight`, `nord`, `gruvbox`, `monokai`, `solarized`, `github`, `one-dark`, `rose-pine`, `ayu`, `material`, `everforest`, `kanagawa`, `amoled`, `synthwave`, `vesper`, `flexoki`, `matrix`, `vercel`, `zenburn`

## Extension System

Extensions are Go source files that run via Yaegi interpreter. They can add custom tools, slash commands, widgets, keyboard shortcuts, themes, and intercept lifecycle events.

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

**Lifecycle Events**: OnSessionStart, OnSessionShutdown, OnBeforeAgentStart, OnAgentStart, OnAgentEnd, OnToolCall, OnToolExecutionStart, OnToolOutput, OnToolExecutionEnd, OnToolResult, OnInput, OnMessageStart, OnMessageUpdate, OnMessageEnd, OnModelChange, OnContextPrepare, OnBeforeFork, OnBeforeSessionSwitch, OnBeforeCompact, OnCustomEvent, OnSubagentStart, OnSubagentChunk, OnSubagentEnd

**Custom Components**:
- **Tools**: Add new tools the LLM can invoke
- **Commands**: Register slash commands (e.g., `/mycommand`)
- **Options**: Register configurable extension options
- **Widgets**: Persistent status displays above/below input
- **Headers/Footers**: Persistent content above/below the conversation
- **Status Bar**: Custom status bar entries
- **Shortcuts**: Global keyboard shortcuts
- **Overlays**: Modal dialogs with markdown content
- **Tool Renderers**: Customize how tool calls display
- **Message Renderers**: Custom rendering for assistant messages
- **Editor Interceptors**: Handle key events and wrap rendering
- **Interactive Prompts**: Select, confirm, input, and multi-select dialogs
- **Subagents**: Spawn in-process child Kit instances
- **LLM Completion**: Direct model calls via `Complete()`
- **Themes**: Register and switch color themes via `RegisterTheme`, `SetTheme`, `ListThemes`
- **Custom Events**: Inter-extension communication via `EmitCustomEvent`

**Bridged SDK APIs** (NEW): Extensions can now access internal SDK capabilities:
- **Tree Navigation**: Navigate conversation history (`GetTreeNode`, `GetCurrentBranch`, `NavigateTo`), summarize branches (`SummarizeBranch`), and implement fresh context loops (`CollapseBranch`)
- **Skill Loading**: Dynamically load and inject skills at runtime (`LoadSkill`, `DiscoverSkills`, `InjectSkillAsContext`)
- **Template Parsing**: Parse and render templates with `{{variables}}` (`ParseTemplate`, `RenderTemplate`), parse CLI-style arguments (`ParseArguments`, `SimpleParseArguments`), and evaluate model conditionals (`EvaluateModelConditional`, `RenderWithModelConditionals`)
- **Model Resolution**: Resolve model fallback chains (`ResolveModelChain`), query model capabilities (`GetModelCapabilities`, `CheckModelAvailable`), and extract provider/model ID (`GetCurrentProvider`, `GetCurrentModelID`)

### Extension Examples

See the `examples/extensions/` directory:

- [`minimal.go`](examples/extensions/minimal.go) - Clean UI with custom footer
- [`auto-commit.go`](examples/extensions/auto-commit.go) - Auto-commit on shutdown
- [`bookmark.go`](examples/extensions/bookmark.go) - Bookmark conversations
- [`branded-output.go`](examples/extensions/branded-output.go) - Branded output rendering
- [`bridge-demo.go`](examples/extensions/bridge_demo.go) - Bridged SDK API demo (tree navigation, skills, templates, model resolution)
- [`compact-notify.go`](examples/extensions/compact-notify.go) - Notification on compaction
- [`confirm-destructive.go`](examples/extensions/confirm-destructive.go) - Confirm destructive operations
- [`context-inject.go`](examples/extensions/context-inject.go) - Inject context into conversations
- [`conversation-manager.go`](examples/extensions/conversation-manager.go) - **NEW** Tree navigation, branch summarization, and fresh context loops
- [`custom-editor-demo.go`](examples/extensions/custom-editor-demo.go) - Vim-like modal editor
- [`dev-reload.go`](examples/extensions/dev-reload.go) - Development live-reload
- [`header-footer-demo.go`](examples/extensions/header-footer-demo.go) - Custom headers and footers
- [`inline-bash.go`](examples/extensions/inline-bash.go) - Inline bash execution
- [`interactive-shell.go`](examples/extensions/interactive-shell.go) - Interactive shell integration
- [`kit-kit.go`](examples/extensions/kit-kit.go) - Kit-in-Kit (sub-agent spawning)
- [`lsp-diagnostics.go`](examples/extensions/lsp-diagnostics.go) - LSP diagnostic integration
- [`notify.go`](examples/extensions/notify.go) - Desktop notifications
- [`overlay-demo.go`](examples/extensions/overlay-demo.go) - Modal dialogs
- [`permission-gate.go`](examples/extensions/permission-gate.go) - Permission gating for tools
- [`pirate.go`](examples/extensions/pirate.go) - Pirate-themed personality
- [`plan-mode.go`](examples/extensions/plan-mode.go) - Read-only planning mode
- [`project-rules.go`](examples/extensions/project-rules.go) - Project-specific rules
- [`prompt-demo.go`](examples/extensions/prompt-demo.go) - Interactive prompts (select/confirm/input)
- [`prompt-templates.go`](examples/extensions/prompt-templates.go) - **NEW** Frontmatter-driven templates with model switching and skill injection
- [`protected-paths.go`](examples/extensions/protected-paths.go) - Path protection for sensitive files
- [`subagent-widget.go`](examples/extensions/subagent-widget.go) - Multi-agent orchestration with status widget
- [`subagent-test.go`](examples/extensions/subagent-test.go) - Subagent testing utilities
- [`summarize.go`](examples/extensions/summarize.go) - Conversation summarization
- [`tool-logger.go`](examples/extensions/tool-logger.go) - Log all tool calls
- [`neon-theme.go`](examples/extensions/neon-theme.go) - Custom theme registration and switching
- [`tool-renderer-demo.go`](examples/extensions/tool-renderer-demo.go) - Custom tool call rendering
- [`widget-status.go`](examples/extensions/widget-status.go) - Persistent status widgets

Also see [`.kit/extensions/go-edit-lint.go`](.kit/extensions/go-edit-lint.go) (in this repo) for a project-local extension example that runs gopls and golangci-lint on Go file edits.

### Loading Extensions

**Auto-discovery** (loads automatically):
- `~/.config/kit/extensions/*.go` (global single files)
- `~/.config/kit/extensions/*/main.go` (global subdirectory extensions)
- `.kit/extensions/*.go` (project-local single files)
- `.kit/extensions/*/main.go` (project-local subdirectory extensions)
- `~/.local/share/kit/git/` (global git-installed packages)
- `.kit/git/` (project-local git-installed packages)

**Explicit loading**:
```bash
kit -e path/to/extension.go
kit -e ext1.go -e ext2.go  # Multiple extensions
```

**Disable auto-load**:
```bash
kit --no-extensions
```

### Testing Extensions

Kit provides a testing package to help you write unit tests for your extensions:

```go
package main

import (
    "testing"
    "github.com/mark3labs/kit/pkg/extensions/test"
    "github.com/mark3labs/kit/internal/extensions"
)

func TestMyExtension(t *testing.T) {
    harness := test.New(t)
    harness.LoadFile("my-ext.go")
    
    // Emit events and verify behavior
    _, err := harness.Emit(extensions.SessionStartEvent{SessionID: "test"})
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    
    // Verify the extension printed something
    test.AssertPrinted(t, harness, "session started")
}
```

**Available assertions:**
- `AssertBlocked()`, `AssertNotBlocked()` — Verify tool blocking
- `AssertWidgetSet()`, `AssertWidgetText()` — Verify widget content
- `AssertPrinted()`, `AssertPrintedContains()` — Verify output
- `AssertToolRegistered()`, `AssertCommandRegistered()` — Verify registration

See [`examples/extensions/tool-logger_test.go`](examples/extensions/tool-logger_test.go) for a complete example with 14 test cases covering tool calls, input handling, and session lifecycle.

### Prompt Templates

Create reusable prompt templates with shell-style argument substitution. Templates are loaded from `~/.kit/prompts/*.md` and `.kit/prompts/*.md`.

**Example template** (`~/.kit/prompts/review.md`):
```markdown
---
description: Review code for issues
---
Review the following code for bugs and security issues.
Focus on $1 specifically.
```

**Usage:**
```
/review error handling
```

**Argument placeholders:**
- `$1`, `$2`, etc. — Individual arguments
- `$@` or `$ARGUMENTS` — All arguments (zero or more)
- `$+` — All arguments (one or more required; error if none given)
- `${@:2}` — Arguments from position 2 onwards
- `${@:1:3}` — 3 arguments starting at position 1

Placeholders inside fenced code blocks (```) and inline code spans are ignored.

Disable templates with `--no-prompt-templates` or load a specific template with `--prompt-template <name>`.

## Session Management

Kit uses a tree-based session model that supports branching and forking conversations.

### Session Locations

- Default: `~/.kit/sessions/<cwd-path>/<timestamp>_<id>.jsonl`
- Path separators in the working directory are replaced with `--` (e.g., `/home/user/project` becomes `home--user--project`)
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

### Interactive Session Commands

During an interactive session, use these slash commands:

| Command | Description |
|---------|-------------|
| `/name [name]` | Set or display the session's display name |
| `/session` | Show session info (path, ID, message count) |
| `/resume` | Open the session picker to switch sessions |
| `/export [path]` | Export session as JSONL (auto-generates path if omitted) |
| `/import <path>` | Import and switch to a session from a JSONL file |
| `/share` | Upload session to GitHub Gist and get a shareable viewer URL |
| `/tree` | Navigate the session tree |
| `/fork` | Fork to new session from an earlier message |
| `/new` | Start a fresh session |

### Keyboard Shortcuts

| Shortcut | Description |
|----------|-------------|
| `Ctrl+X e` | Open `$VISUAL`/`$EDITOR` to compose or edit your prompt |
| `Ctrl+X s` | Steer — inject a system-level instruction mid-turn |
| `ESC ESC` | Cancel the current operation (tool call or streaming) |
| `↑` / `↓` | Navigate prompt history |

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
    defer func() { _ = host.Close() }()
    
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

    // Session options
    SessionPath:  "./session.jsonl",  // Open specific session
    Continue:     true,                // Resume most recent session
    NoSession:    true,                // Ephemeral mode

    // Tool options
    Tools:            []kit.Tool{...},     // Replace default tool set entirely
    ExtraTools:       []kit.Tool{...},     // Add tools alongside defaults
    DisableCoreTools: true,                // Use no core tools (0 tools, for chat-only)

    // Configuration
    SkipConfig:   true,                   // Skip .kit.yml files (viper defaults + env vars still apply)

    // Compaction
    AutoCompact:  true,                // Auto-compact near context limit

    Debug:        true,                // Debug logging
})
```

### Custom Tools

Create custom tools with automatic schema generation — no external dependencies needed:

```go
type SearchInput struct {
    Query string `json:"query" description:"Search query"`
}

searchTool := kit.NewTool("search", "Search the codebase",
    func(ctx context.Context, input SearchInput) (kit.ToolOutput, error) {
        return kit.TextResult("Found: ..."), nil
    },
)

host, _ := kit.New(ctx, &kit.Options{
    ExtraTools: []kit.Tool{searchTool}, // adds alongside built-in tools
})
```

Use `kit.NewParallelTool` for tools safe to run concurrently. See the [SDK docs](/sdk/overview) for full details on struct tags, `ToolOutput` fields, and `ToolCallIDFromContext`.

### With Callbacks

```go
unsub := host.OnToolCall(func(e kit.ToolCallEvent) {
    println("Calling tool:", e.ToolName)
})
defer unsub()

unsub2 := host.OnToolResult(func(e kit.ToolResultEvent) {
    if e.IsError {
        println("Tool failed:", e.ToolName)
    }
})
defer unsub2()

unsub3 := host.OnStreaming(func(e kit.MessageUpdateEvent) {
    print(e.Chunk)
})
defer unsub3()

response, err := host.Prompt(
    ctx,
    "List files in current directory",
)
```

### Session Management

```go
// Multi-turn conversations retain context automatically
host.Prompt(ctx, "My name is Alice")
response, _ := host.Prompt(ctx, "What's my name?")

// Sessions are persisted automatically to JSONL files.
// Access session info:
path := host.GetSessionPath()
id := host.GetSessionID()

// Clear conversation history
host.ClearSession()
```

Session persistence is configured via `Options`:

```go
host, _ := kit.New(ctx, &kit.Options{
    SessionPath: "./my-session.jsonl",  // Open specific session
    Continue:    true,                   // Resume most recent session
    NoSession:   true,                   // Ephemeral mode
})
```

## Advanced Usage

### Subagent Pattern

Spawn Kit as a subprocess for multi-agent orchestration:

```bash
kit "Analyze codebase" \
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
  "stop_reason": "end_turn",
  "session_id": "a1b2c3d4e5f6",
  "usage": {
    "input_tokens": 1024,
    "output_tokens": 512,
    "total_tokens": 1536,
    "cache_read_tokens": 0,
    "cache_creation_tokens": 0
  },
  "messages": [
    {
      "role": "assistant",
      "parts": [
        {"type": "text", "data": "..."},
        {"type": "tool_call", "data": {"name": "...", "args": "..."}},
        {"type": "tool_result", "data": {"name": "...", "result": "..."}}
      ]
    }
  ]
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
cmd/kit/             - CLI entry point (main.go)
cmd/                 - CLI command implementations (root, auth, models, etc.)
pkg/kit/             - Go SDK for embedding Kit
internal/app/        - Application orchestrator (agent loop, message store, queue)
internal/agent/      - Agent execution and tool dispatch
internal/auth/       - OAuth authentication and credential storage
internal/acpserver/  - ACP (Agent Client Protocol) server
internal/clipboard/  - Cross-platform clipboard operations
internal/compaction/ - Conversation compaction and summarization
internal/config/     - Configuration management
internal/core/       - Built-in tools (bash, read, write, edit, grep, find, ls)
internal/extensions/ - Yaegi extension system
internal/kitsetup/   - Initial setup wizard
internal/message/    - Message content types and structured content blocks
internal/models/     - Provider and model management
internal/session/    - Session persistence (tree-based JSONL)
internal/skills/     - Skill loading and system prompt composition
internal/tools/      - MCP tool integration
internal/ui/         - Bubble Tea TUI components
examples/extensions/ - Example extension files
npm/                 - NPM package wrapper for distribution
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
- **Custom** - Any OpenAI-compatible endpoint via `--provider-url`
- **Auto-routed** - Any provider from models.dev database

### Custom Provider

Use `custom/custom` when pointing Kit at any OpenAI-compatible endpoint with `--provider-url`:

```bash
kit --provider-url "http://localhost:8080/v1" "Hello"
```

This automatically defaults to `custom/custom` without needing to specify a model. The custom provider routes through the `openaicompat` provider and supports:

- Zero cost tracking (input/output = 0)
- 262K context window, 65K output limit
- Reasoning and temperature support
- Optional `CUSTOM_API_KEY` environment variable or `--provider-api-key` flag

### Model String Format

```bash
provider/model            # Standard format
anthropic/claude-sonnet-latest
openai/gpt-4o
ollama/llama3
google/gemini-2.0-flash-exp
```

### Model Aliases

```bash
# Anthropic Claude
claude-opus-latest        → claude-opus-4-6
claude-sonnet-latest      → claude-sonnet-4-6
claude-haiku-latest       → claude-haiku-4-5
claude-4-opus-latest      → claude-opus-4-6
claude-4-sonnet-latest    → claude-sonnet-4-6
claude-4-haiku-latest     → claude-haiku-4-5
claude-3-7-sonnet-latest  → claude-3-7-sonnet-20250219
claude-3-5-sonnet-latest   → claude-3-5-sonnet-20241022
claude-3-5-haiku-latest   → claude-3-5-haiku-20241022
claude-3-opus-latest      → claude-3-opus-20240229

# OpenAI GPT
o1-latest                 → o1
o3-latest                 → o3
o4-latest                 → o4-mini
gpt-5-latest              → gpt-5.4
gpt-5-chat-latest         → gpt-5.4
gpt-4-latest              → gpt-4o
gpt-4                     → gpt-4o
gpt-3.5-latest            → gpt-3.5-turbo
gpt-3.5                   → gpt-3.5-turbo
codex-latest              → codex-mini-latest

# Google Gemini
gemini-pro-latest         → gemini-2.5-pro
gemini-flash-latest       → gemini-2.5-flash
gemini-flash              → gemini-2.5-flash
gemini-pro                → gemini-2.5-pro
```

## Contributing

Contributions are welcome! Please see the [contribution guide](contribute/contribute.md) for guidelines.

## License

[MIT](LICENSE)

## Community

- [Discord](https://discord.gg/RqSS2NQVsY)
- [GitHub Issues](https://github.com/mark3labs/kit/issues)
- [Documentation](https://github.com/mark3labs/kit/wiki)
