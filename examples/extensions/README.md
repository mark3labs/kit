# Kit Extension Examples

A collection of example extensions demonstrating various Kit capabilities. These can be installed individually or as a complete collection.

## Installation

### Install all examples
```bash
kit install github.com/mark3labs/kit/examples/extensions
```

### Install with interactive selection
```bash
kit install github.com/mark3labs/kit/examples/extensions --select
```

### Install locally in your project
```bash
kit install github.com/mark3labs/kit/examples/extensions --local
```

## Extension Index

### Core Concepts

| Extension | Description | Key API |
|-----------|-------------|---------|
| `minimal.go` | Minimal viable extension | Basic `Init()` function |
| `plan-mode.go` | Restrict agent to read-only tools | `OnBeforeAgentStart`, `SetActiveTools` |
| `tool-logger.go` | Log all tool calls to file | `OnToolCall`, `OnToolResult` |
| `notify.go` | Display notifications | `PrintInfo`, `PrintBlock` |

### UI & Widgets

| Extension | Description | Key API |
|-----------|-------------|---------|
| `widget-status.go` | Persistent status widget | `SetWidget`, `RemoveWidget` |
| `header-footer-demo.go` | Custom header/footer | `SetHeader`, `SetFooter` |
| `overlay-demo.go` | Modal overlay dialogs | `ShowOverlay` |
| `compact-notify.go` | Compact mode notifications | `PrintBlock` |
| `branded-output.go` | Custom styled output | `PrintBlock` with colors |

### Input & Editor

| Extension | Description | Key API |
|-----------|-------------|---------|
| `custom-editor-demo.go` | Custom key handling | `SetEditor`, `EditorKeyAction` |
| `pirate.go` | Transform user input | `OnInput`, `InputResult` |
| `interactive-shell.go` | Custom command input | Slash commands with prompts |
| `inline-bash.go` | Execute bash inline | Input handling, `exec` |

### Session & Context

| Extension | Description | Key API |
|-----------|-------------|---------|
| `context-inject.go` | Inject context into prompts | `OnContextPrepare` |
| `bookmark.go` | Bookmark messages | `AppendEntry`, `GetEntries` |
| `project-rules.go` | Project-specific rules | Session data, file reading |
| `protected-paths.go` | Block dangerous operations | `OnToolCall` with blocking |
| `permission-gate.go` | Confirm destructive actions | `OnToolCall` with confirmation |

### Tools & Commands

| Extension | Description | Key API |
|-----------|-------------|---------|
| `auto-commit.go` | Auto-commit changes | Custom tool, git operations |
| `summarize.go` | Summarize conversation | Custom tool with parameters |
| `confirm-destructive.go` | Confirm destructive commands | `OnToolCall` blocking |
| `lsp-diagnostics.go` | LSP integration | Complex extension, external process |

### Subagents & Background Tasks

| Extension | Description | Key API |
|-----------|-------------|---------|
| `kit-kit.go` | Spawn Kit as subagent | Subagent spawning |
| `subagent-test.go` | Test subagent functionality | `SpawnSubagent` |
| `subagent-widget.go` | Widget with subagent updates | Goroutines + widgets |
| `dev-reload.go` | Hot reload extensions | `ReloadExtensions` |

### Rendering

| Extension | Description | Key API |
|-----------|-------------|---------|
| `tool-renderer-demo.go` | Custom tool output styling | `RegisterToolRenderer` |
| `prompt-demo.go` | Interactive prompts | `PromptSelect`, `PromptConfirm` |

## Extension Details

### minimal.go
The bare minimum extension showing the required structure:
- Package `main`
- Import `kit/ext`
- Export `Init(api ext.API)` function

### plan-mode.go
A complete example demonstrating:
- Slash command (`/plan`)
- Keyboard shortcut (`ctrl+alt+p`)
- Option registration
- Status bar indicators
- System prompt injection
- Tool filtering

### widget-status.go
Shows how to create persistent UI elements:
- Create widgets with `SetWidget`
- Update content dynamically
- Remove when done
- Handle session lifecycle

### context-inject.go
Advanced context manipulation:
- Read project files
- Inject into LLM context
- Filter messages
- Use negative indices for ephemeral content

### lsp-diagnostics.go
Complex real-world example:
- Multi-file extension
- External process management (LSP server)
- File watching
- Diagnostics aggregation

## Multi-File Extension Example

The `kit-kit-agents/` directory demonstrates the multi-file pattern:

```
kit-kit-agents/
├── main.go          # Entry point with Init()
├── agent.go         # Agent configuration
├── manager.go       # Agent lifecycle management
└── README.md        # Documentation
```

When the repo is installed, all files in subdirectories with `main.go` are loaded as separate extensions.

## Testing & Validation

After installing, test the extensions:

```bash
# List all loaded extensions
kit extensions list

# Validate all extensions
kit extensions validate

# Run with a specific extension
kit -e ~/.local/share/kit/git/github.com/mark3labs/kit/examples/extensions/plan-mode.go
```

## Creating Your Own

1. Copy `minimal.go` as a starting point
2. Modify the `Init()` function to register your handlers
3. Use the other examples for reference on specific APIs
4. Test with `kit -e your-extension.go`
5. Share by pushing to a git repository!

## Update

To get the latest examples:

```bash
kit install github.com/mark3labs/kit/examples/extensions --update
```

## See Also

- [Kit Extensions Guide](https://github.com/mark3labs/kit/blob/main/.agents/skills/kit-extensions/SKILL.md)
- [API Reference](https://github.com/mark3labs/kit/blob/main/internal/extensions/api.go)
- [Example Extensions Source](https://github.com/mark3labs/kit/tree/main/examples/extensions)
