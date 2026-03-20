---
name: kit-extensions
description: Guide for creating Kit extensions. Use when the user asks to build, create, or modify a Kit extension, add a custom tool, slash command, widget, keyboard shortcut, editor interceptor, tool renderer, or hook into any Kit lifecycle event.
---

# Kit Extensions Development Guide

Kit extensions are single-file Go programs interpreted at runtime by Yaegi. They hook into Kit's lifecycle, register custom tools and slash commands, display widgets, intercept editor input, render tool output, register and switch color themes, and more.

Extensions can be distributed via git repositories using `kit install`. Repos can contain single extensions or collections of multiple extensions.

## Extension Structure

Every extension must export a `package main` with an `Init(api ext.API)` function:

```go
//go:build ignore

package main

import "kit/ext"

func Init(api ext.API) {
    // Register event handlers, tools, commands, etc.
}
```

The `//go:build ignore` tag prevents `go build` from compiling the file directly.

## Extension Locations

Extensions are auto-loaded from these directories:

- `~/.config/kit/extensions/*.go` (global, single files)
- `~/.config/kit/extensions/*/main.go` (global, subdirectories)
- `.kit/extensions/*.go` (project-local, single files)
- `.kit/extensions/*/main.go` (project-local, subdirectories)

Or loaded explicitly:

```bash
kit -e path/to/extension.go
kit --extension path/to/extension.go
```

## Import Path

Extensions import the Kit API as `"kit/ext"`. The full standard library is available plus `os/exec` for subprocess spawning.

## API Overview

The `Init` function receives an `ext.API` object for registering handlers, and event handlers receive an `ext.Context` with runtime capabilities.

---

## Lifecycle Events

Kit provides 18 lifecycle events. Each handler receives an event struct and a `Context`.

### Session Events

```go
// Fired when session is loaded/created.
api.OnSessionStart(func(e ext.SessionStartEvent, ctx ext.Context) {
    // e.SessionID string
})

// Fired when Kit is shutting down. Use for cleanup.
api.OnSessionShutdown(func(e ext.SessionShutdownEvent, ctx ext.Context) {
    // No fields.
})
```

### Agent Turn Events

```go
// Before agent starts processing. Can inject system prompt or text.
api.OnBeforeAgentStart(func(e ext.BeforeAgentStartEvent, ctx ext.Context) *ext.BeforeAgentStartResult {
    // e.Prompt string
    // Return nil to pass through.
    // Return &ext.BeforeAgentStartResult{SystemPrompt: &s} to augment system prompt.
    // Return &ext.BeforeAgentStartResult{InjectText: &s} to inject text before prompt.
    return nil
})

// Agent loop has started.
api.OnAgentStart(func(e ext.AgentStartEvent, ctx ext.Context) {
    // e.Prompt string
})

// Agent finished responding.
api.OnAgentEnd(func(e ext.AgentEndEvent, ctx ext.Context) {
    // e.Response string
    // e.StopReason string — "completed", "cancelled", "error"
})
```

### Tool Events

```go
// Before a tool executes. Can block the call.
api.OnToolCall(func(e ext.ToolCallEvent, ctx ext.Context) *ext.ToolCallResult {
    // e.ToolName string
    // e.ToolCallID string
    // e.Input string — JSON-encoded parameters
    // e.Source string — "llm" or "user"
    // Return nil to allow.
    // Return &ext.ToolCallResult{Block: true, Reason: "..."} to block.
    return nil
})

// Tool execution started (informational only).
api.OnToolExecutionStart(func(e ext.ToolExecutionStartEvent, ctx ext.Context) {
    // e.ToolName string
})

// Tool execution ended (informational only).
api.OnToolExecutionEnd(func(e ext.ToolExecutionEndEvent, ctx ext.Context) {
    // e.ToolName string
})

// After a tool returns. Can modify the result.
api.OnToolResult(func(e ext.ToolResultEvent, ctx ext.Context) *ext.ToolResultResult {
    // e.ToolName string
    // e.Input string
    // e.Content string
    // e.IsError bool
    // Return nil to pass through.
    // Return &ext.ToolResultResult{Content: &s} to replace content.
    // Return &ext.ToolResultResult{IsError: &b} to change error status.
    return nil
})
```

### Input Events

```go
// User submitted input. Can handle or transform it.
api.OnInput(func(e ext.InputEvent, ctx ext.Context) *ext.InputResult {
    // e.Text string
    // e.Source string — "interactive", "cli", "script", "queue"
    // Return nil to pass through to agent.
    // Return &ext.InputResult{Action: "handled"} to consume without sending to agent.
    // Return &ext.InputResult{Action: "transform", Text: "new text"} to rewrite.
    return nil
})
```

### Streaming Events

```go
api.OnMessageStart(func(e ext.MessageStartEvent, ctx ext.Context) {})
api.OnMessageUpdate(func(e ext.MessageUpdateEvent, ctx ext.Context) {
    // e.Chunk string — streaming text chunk
})
api.OnMessageEnd(func(e ext.MessageEndEvent, ctx ext.Context) {
    // e.Content string — full message content
})
```

### Model Events

```go
api.OnModelChange(func(e ext.ModelChangeEvent, ctx ext.Context) {
    // e.NewModel string
    // e.PreviousModel string
    // e.Source string — "extension" or "user"
})
```

### Context Filtering

```go
// Before messages are sent to the LLM. Can filter, reorder, or inject messages.
api.OnContextPrepare(func(e ext.ContextPrepareEvent, ctx ext.Context) *ext.ContextPrepareResult {
    // e.Messages []ext.ContextMessage
    // Each ContextMessage has: Index int, Role string, Content string
    // Index -1 means a new injected message (not from session).
    // Return nil to pass through.
    // Return &ext.ContextPrepareResult{Messages: msgs} to replace the context window.
    return nil
})
```

### Session Control Events

```go
// Before forking the session tree. Can cancel.
api.OnBeforeFork(func(e ext.BeforeForkEvent, ctx ext.Context) *ext.BeforeForkResult {
    // e.TargetID string, e.IsUserMessage bool, e.UserText string
    return nil // or &ext.BeforeForkResult{Cancel: true, Reason: "..."}
})

// Before switching/clearing session. Can cancel.
api.OnBeforeSessionSwitch(func(e ext.BeforeSessionSwitchEvent, ctx ext.Context) *ext.BeforeSessionSwitchResult {
    // e.Reason string — "new" or "clear"
    return nil // or &ext.BeforeSessionSwitchResult{Cancel: true, Reason: "..."}
})

// Before context compaction. Can cancel.
api.OnBeforeCompact(func(e ext.BeforeCompactEvent, ctx ext.Context) *ext.BeforeCompactResult {
    // e.EstimatedTokens, e.ContextLimit int
    // e.UsagePercent float64, e.MessageCount int, e.IsAutomatic bool
    return nil // or &ext.BeforeCompactResult{Cancel: true, Reason: "..."}
})
```

### Custom Events

```go
// Subscribe to custom events emitted by other extensions.
api.OnCustomEvent("event-name", func(data string) {
    // data is arbitrary string payload
})

// Emit from Context:
ctx.EmitCustomEvent("event-name", "payload")
```

---

## Registering Tools

Tools are functions the LLM can invoke:

```go
api.RegisterTool(ext.ToolDef{
    Name:        "current_time",
    Description: "Get the current date and time",
    Parameters:  `{"type":"object","properties":{}}`,
    Execute: func(input string) (string, error) {
        return time.Now().Format(time.RFC3339), nil
    },
})
```

For long-running tools with cancellation and progress:

```go
api.RegisterTool(ext.ToolDef{
    Name:        "slow_task",
    Description: "A long-running task with progress reporting",
    Parameters:  `{"type":"object","properties":{"query":{"type":"string"}}}`,
    ExecuteWithContext: func(input string, tc ext.ToolContext) (string, error) {
        for i := 0; i < 10; i++ {
            if tc.IsCancelled() {
                return "cancelled", nil
            }
            tc.OnProgress(fmt.Sprintf("Step %d/10...", i+1))
            time.Sleep(time.Second)
        }
        return "done", nil
    },
})
```

Parameters must be a JSON Schema string. The `input` argument is the JSON-encoded parameters from the LLM.

---

## Registering Slash Commands

Commands are user-facing actions invoked with `/name` in the input:

```go
api.RegisterCommand(ext.CommandDef{
    Name:        "echo",
    Description: "Echo back the provided text",
    Execute: func(args string, ctx ext.Context) (string, error) {
        ctx.PrintInfo("You said: " + args)
        return "", nil
    },
    // Optional tab-completion:
    Complete: func(prefix string, ctx ext.Context) []string {
        return []string{"hello", "world"}
    },
})
```

Slash commands run in a dedicated goroutine (not a `tea.Cmd`), so they can safely block on prompts, I/O, etc.

---

## Registering Keyboard Shortcuts

```go
api.RegisterShortcut(ext.ShortcutDef{
    Key:         "ctrl+alt+p",
    Description: "Toggle plan mode",
}, func(ctx ext.Context) {
    // handler runs when shortcut is pressed
})
```

---

## Registering Options

Options are configurable values resolved from env vars, config, or defaults:

```go
api.RegisterOption(ext.OptionDef{
    Name:        "my-setting",
    Description: "Controls something",
    Default:     "false",
})

// Read at runtime (resolution: env KIT_OPT_MY_SETTING > config options.my-setting > default):
val := ctx.GetOption("my-setting")

// Set at runtime:
ctx.SetOption("my-setting", "true")
```

---

## Context API Reference

The `ext.Context` struct provides runtime capabilities via function fields.

### Output

```go
ctx.Print("plain text")                    // plain output
ctx.PrintInfo("styled info block")         // bordered info block
ctx.PrintError("styled error block")       // red error block
ctx.PrintBlock(ext.PrintBlockOpts{         // custom styled block
    Text:        "content",
    BorderColor: "#a6e3a1",
    Subtitle:    "my-ext",
})
ctx.RenderMessage("renderer-name", "content")  // use a registered message renderer
```

### Message Injection

```go
ctx.SendMessage("prompt text")     // inject message and trigger agent turn (queued)
ctx.CancelAndSend("new prompt")   // cancel current turn, clear queue, send new message
```

### Widgets

Persistent UI elements displayed above or below the input area:

```go
ctx.SetWidget(ext.WidgetConfig{
    ID:        "my-widget",
    Placement: ext.WidgetAbove,  // or ext.WidgetBelow
    Content:   ext.WidgetContent{
        Text:     "Status: Active",
        Markdown: false,  // set true for markdown rendering
    },
    Style: ext.WidgetStyle{
        BorderColor: "#a6e3a1",  // hex color
        NoBorder:    false,
    },
    Priority: 0,  // lower values render first
})

ctx.RemoveWidget("my-widget")
```

### Header and Footer

```go
ctx.SetHeader(ext.HeaderFooterConfig{
    Content: ext.WidgetContent{Text: "My Header"},
    Style:   ext.WidgetStyle{BorderColor: "#89b4fa"},
})
ctx.RemoveHeader()

ctx.SetFooter(ext.HeaderFooterConfig{
    Content: ext.WidgetContent{Text: "My Footer"},
    Style:   ext.WidgetStyle{BorderColor: "#585b70"},
})
ctx.RemoveFooter()
```

### Status Bar

```go
ctx.SetStatus("key", "PLAN MODE", 10)  // key, text, priority (lower = further left)
ctx.RemoveStatus("key")
```

### Interactive Prompts

These block until the user responds (safe in slash commands and goroutines):

```go
// Selection list
result := ctx.PromptSelect(ext.PromptSelectConfig{
    Message: "Pick one:",
    Options: []string{"Option A", "Option B", "Option C"},
})
if !result.Cancelled {
    // result.Value string, result.Index int
}

// Yes/No confirmation
result := ctx.PromptConfirm(ext.PromptConfirmConfig{
    Message:      "Are you sure?",
    DefaultValue: false,
})
if !result.Cancelled {
    // result.Value bool
}

// Text input
result := ctx.PromptInput(ext.PromptInputConfig{
    Message:     "Enter name:",
    Placeholder: "my-project",
    Default:     "",
})
if !result.Cancelled {
    // result.Value string
}

// Multi-select (toggle with spacebar, confirm with enter)
result := ctx.PromptMultiSelect(ext.PromptMultiSelectConfig{
    Message: "Select extensions to install:",
    Options: []string{"git", "todo", "weather"},
    DefaultSelected: []int{0, 1, 2},  // pre-selected indices; nil = all selected
})
if !result.Cancelled {
    // result.Values []string — selected option texts
    // result.Indices []int — selected option indices
}
```

### Overlay Dialogs

Modal dialogs with optional action buttons:

```go
result := ctx.ShowOverlay(ext.OverlayConfig{
    Title:   "Confirmation",
    Content: ext.WidgetContent{Text: "Are you sure you want to proceed?", Markdown: true},
    Style:   ext.OverlayStyle{BorderColor: "#f38ba8"},
    Width:   60,          // 0 = 60% of terminal width
    MaxHeight: 20,        // 0 = 80% of terminal height
    Anchor:  ext.OverlayCenter,  // or ext.OverlayTopCenter, ext.OverlayBottomCenter
    Actions: []string{"Confirm", "Cancel"},
})
if !result.Cancelled {
    // result.Action string, result.Index int
}
```

### Editor Interceptor

Wrap the built-in text input with custom key handling and rendering:

```go
ctx.SetEditor(ext.EditorConfig{
    HandleKey: func(key string, currentText string) ext.EditorKeyAction {
        if key == "ctrl+s" {
            return ext.EditorKeyAction{Type: ext.EditorKeySubmit, SubmitText: currentText}
        }
        return ext.EditorKeyAction{Type: ext.EditorKeyPassthrough}
    },
    Render: func(width int, defaultContent string) string {
        return "[custom] " + defaultContent
    },
})

ctx.ResetEditor()                  // remove interceptor
ctx.SetEditorText("prefilled")     // set editor text content
```

**EditorKeyAction types:**
- `ext.EditorKeyPassthrough` — let the default editor handle the key
- `ext.EditorKeyConsumed` — swallow the key, do nothing
- `ext.EditorKeyRemap` — remap to a different key: `EditorKeyAction{Type: ext.EditorKeyRemap, RemappedKey: "up"}`
- `ext.EditorKeySubmit` — submit text: `EditorKeyAction{Type: ext.EditorKeySubmit, SubmitText: "text"}`

### UI Visibility

```go
ctx.SetUIVisibility(ext.UIVisibility{
    HideStartupMessage: true,
    HideStatusBar:      true,
    HideSeparator:      true,
    HideInputHint:      true,
})
```

### Session Data

```go
stats := ctx.GetContextStats()     // .EstimatedTokens, .ContextLimit, .UsagePercent, .MessageCount
msgs := ctx.GetMessages()          // []ext.SessionMessage on current branch
path := ctx.GetSessionPath()       // file path of session JSONL

// Persist custom data in the session tree:
id, err := ctx.AppendEntry("my-type", "data string")
entries := ctx.GetEntries("my-type")  // []ext.ExtensionEntry{ID, EntryType, Data, Timestamp}
```

### Model Management

```go
err := ctx.SetModel("anthropic/claude-sonnet-4-20250514")
models := ctx.GetAvailableModels()  // []ext.ModelInfoEntry
```

### Tool Management

```go
tools := ctx.GetAllTools()              // []ext.ToolInfo{Name, Description, Source, Enabled}
ctx.SetActiveTools([]string{"read", "grep"})  // restrict to these tools only
ctx.SetActiveTools(nil)                 // re-enable all tools
```

### LLM Completions

Make standalone LLM calls (bypasses the agent tool loop):

```go
resp, err := ctx.Complete(ext.CompleteRequest{
    Model:    "",             // empty = current model
    System:   "You are ...",  // optional system prompt
    Prompt:   "Summarize...", // the prompt
    MaxTokens: 1000,          // 0 = provider default
    OnChunk:  func(chunk string) { /* streaming */ },
})
// resp.Text, resp.InputTokens, resp.OutputTokens, resp.Model
```

### TUI Suspension

Temporarily release the terminal for interactive subprocesses:

```go
ctx.SuspendTUI(func() {
    cmd := exec.Command("vim", "file.go")
    cmd.Stdin = os.Stdin
    cmd.Stdout = os.Stdout
    cmd.Stderr = os.Stderr
    cmd.Run()
})
```

### Themes

Register, switch, and list color themes at runtime:

```go
// Register a custom theme (empty fields inherit from default).
ctx.RegisterTheme("neon", ext.ThemeColorConfig{
    Primary:    ext.ThemeColor{Light: "#CC00FF", Dark: "#FF00FF"},
    Secondary:  ext.ThemeColor{Light: "#0088CC", Dark: "#00FFFF"},
    Success:    ext.ThemeColor{Light: "#00CC44", Dark: "#00FF66"},
    Warning:    ext.ThemeColor{Light: "#CCAA00", Dark: "#FFFF00"},
    Error:      ext.ThemeColor{Light: "#CC0033", Dark: "#FF0055"},
    Info:       ext.ThemeColor{Light: "#0088CC", Dark: "#00CCFF"},
    Text:       ext.ThemeColor{Light: "#111111", Dark: "#F0F0F0"},
    Background: ext.ThemeColor{Light: "#F0F0F0", Dark: "#0A0A14"},
    MdKeyword:  ext.ThemeColor{Light: "#CC00FF", Dark: "#FF00FF"},
    MdString:   ext.ThemeColor{Light: "#00CC44", Dark: "#00FF66"},
    MdComment:  ext.ThemeColor{Light: "#888888", Dark: "#555555"},
})

// Switch to a theme by name (built-in, file-based, or extension-registered).
err := ctx.SetTheme("neon")

// List all available theme names.
names := ctx.ListThemes()  // []string
```

**ThemeColorConfig fields:**

| Field | Description |
|-------|-------------|
| `Primary` | Main brand/accent color |
| `Secondary` | Secondary accent |
| `Success` | Success states |
| `Warning` | Warning states |
| `Error` | Error/critical states |
| `Info` | Informational states |
| `Text` | Primary text |
| `Muted` | Dimmed text |
| `VeryMuted` | Very dimmed text |
| `Background` | Base background |
| `Border` | Panel borders |
| `MutedBorder` | Subtle dividers |
| `System` | System messages |
| `Tool` | Tool-related elements |
| `Accent` | Secondary highlight |
| `Highlight` | Highlighted regions |
| `MdHeading` | Markdown headings |
| `MdLink` | Markdown links |
| `MdKeyword` | Syntax: keywords |
| `MdString` | Syntax: strings |
| `MdNumber` | Syntax: numbers |
| `MdComment` | Syntax: comments |

Each field is an `ext.ThemeColor` with `Light` and `Dark` hex strings. Kit ships 22 built-in themes: `kitt`, `catppuccin`, `dracula`, `tokyonight`, `nord`, `gruvbox`, `monokai`, `solarized`, `github`, `one-dark`, `rose-pine`, `ayu`, `material`, `everforest`, `kanagawa`, `amoled`, `synthwave`, `vesper`, `flexoki`, `matrix`, `vercel`, `zenburn`.

Users can also drop `.yml`/`.yaml`/`.json` theme files in `~/.config/kit/themes/` (global) or `.kit/themes/` (project-local). Extension-registered themes take highest precedence.

### Application Control

```go
ctx.Exit()                        // graceful shutdown
err := ctx.ReloadExtensions()     // hot-reload all extensions from disk
```

### Context Fields

```go
ctx.SessionID    // string
ctx.CWD          // string — current working directory
ctx.Model        // string — active model name
ctx.Interactive  // bool — true if running in TUI mode
```

---

## Tool Renderers

Customize how tool calls are displayed in the TUI:

```go
api.RegisterToolRenderer(ext.ToolRenderConfig{
    ToolName:     "bash",
    DisplayName:  "Shell",           // replaces auto-capitalized name
    BorderColor:  "#89b4fa",
    Background:   "",
    BodyMarkdown: true,              // render body through markdown
    RenderHeader: func(toolArgs string, width int) string {
        var args struct{ Command string `json:"command"` }
        json.Unmarshal([]byte(toolArgs), &args)
        return "$ " + args.Command
    },
    RenderBody: func(toolResult string, isError bool, width int) string {
        if isError {
            return "ERROR: " + toolResult
        }
        return toolResult
    },
})
```

## Message Renderers

Define named output styles for `ctx.RenderMessage()`:

```go
api.RegisterMessageRenderer(ext.MessageRendererConfig{
    Name: "success",
    Render: func(content string, width int) string {
        return "  " + content  // green checkmark prefix
    },
})

// Usage in handlers:
ctx.RenderMessage("success", "All tests passed")
```

---

## Critical Yaegi Constraints

### No Named Function References in Struct Fields

Yaegi has a bug where named function references assigned to struct fields return zero values across the interpreter boundary. Always use anonymous closure literals:

```go
// WRONG - will silently return zero values:
func myHandler(key, text string) ext.EditorKeyAction {
    return ext.EditorKeyAction{Type: ext.EditorKeyPassthrough}
}
ctx.SetEditor(ext.EditorConfig{HandleKey: myHandler})

// CORRECT - use anonymous closure:
ctx.SetEditor(ext.EditorConfig{
    HandleKey: func(key, text string) ext.EditorKeyAction {
        return ext.EditorKeyAction{Type: ext.EditorKeyPassthrough}
    },
})
```

This applies to ALL struct fields that take function values: `ToolDef.Execute`, `CommandDef.Execute`, `EditorConfig.HandleKey`, `EditorConfig.Render`, `ToolRenderConfig.RenderHeader`, `ToolRenderConfig.RenderBody`, etc.

### No Interfaces Across the Boundary

All extension-facing API types are concrete structs, never interfaces. Yaegi crashes on interface wrapper generation.

### Package-Level Variables for State

Yaegi supports package-level variables captured in closures. This is the standard way to maintain state across event callbacks:

```go
package main

import "kit/ext"

var callCount int
var lastTool string

func Init(api ext.API) {
    api.OnToolResult(func(e ext.ToolResultEvent, ctx ext.Context) *ext.ToolResultResult {
        callCount++
        lastTool = e.ToolName
        return nil
    })
}
```

---

## Common Patterns

### Pattern: Tool Call Blocking

Block dangerous operations by intercepting tool calls:

```go
api.OnToolCall(func(tc ext.ToolCallEvent, ctx ext.Context) *ext.ToolCallResult {
    if tc.ToolName == "bash" {
        var input struct{ Command string `json:"command"` }
        json.Unmarshal([]byte(tc.Input), &input)
        if strings.Contains(input.Command, "rm -rf") {
            return &ext.ToolCallResult{
                Block:  true,
                Reason: "Dangerous command blocked",
            }
        }
    }
    return nil
})
```

### Pattern: System Prompt Injection

Augment the agent's behavior by injecting instructions:

```go
api.OnBeforeAgentStart(func(_ ext.BeforeAgentStartEvent, ctx ext.Context) *ext.BeforeAgentStartResult {
    prompt := "Always respond with bullet points."
    return &ext.BeforeAgentStartResult{SystemPrompt: &prompt}
})
```

### Pattern: Background Processing with SendMessage

Run work in a goroutine and inject results back:

```go
api.RegisterCommand(ext.CommandDef{
    Name: "run",
    Description: "Run a command in the background",
    Execute: func(args string, ctx ext.Context) (string, error) {
        go func() {
            out, err := exec.Command("sh", "-c", args).CombinedOutput()
            if err != nil {
                ctx.SendMessage(fmt.Sprintf("Command failed: %s\n%s", err, out))
                return
            }
            ctx.SendMessage(fmt.Sprintf("Command output:\n```\n%s\n```", out))
        }()
        return "Running in background...", nil
    },
})
```

### Pattern: Ephemeral Context Injection

Inject information into every LLM turn without persisting in session history:

```go
api.OnContextPrepare(func(e ext.ContextPrepareEvent, ctx ext.Context) *ext.ContextPrepareResult {
    data, err := os.ReadFile(".kit/context.md")
    if err != nil {
        return nil
    }
    injected := ext.ContextMessage{
        Index:   -1,  // -1 = new message, not from session
        Role:    "system",
        Content: string(data),
    }
    msgs := append([]ext.ContextMessage{injected}, e.Messages...)
    return &ext.ContextPrepareResult{Messages: msgs}
})
```

### Pattern: Live Widget Updates

Update a widget periodically from a goroutine:

```go
api.OnSessionStart(func(_ ext.SessionStartEvent, ctx ext.Context) {
    go func() {
        ticker := time.NewTicker(time.Second)
        defer ticker.Stop()
        for range ticker.C {
            ctx.SetWidget(ext.WidgetConfig{
                ID:        "clock",
                Placement: ext.WidgetAbove,
                Content:   ext.WidgetContent{Text: time.Now().Format("15:04:05")},
                Style:     ext.WidgetStyle{BorderColor: "#89b4fa"},
            })
        }
    }()
})
```

### Pattern: Custom Theme with Slash Command

Register a theme and provide a slash command shortcut to activate it:

```go
api.OnSessionStart(func(_ ext.SessionStartEvent, ctx ext.Context) {
    ctx.RegisterTheme("neon", ext.ThemeColorConfig{
        Primary:    ext.ThemeColor{Light: "#CC00FF", Dark: "#FF00FF"},
        Secondary:  ext.ThemeColor{Light: "#0088CC", Dark: "#00FFFF"},
        Success:    ext.ThemeColor{Light: "#00CC44", Dark: "#00FF66"},
        Warning:    ext.ThemeColor{Light: "#CCAA00", Dark: "#FFFF00"},
        Error:      ext.ThemeColor{Light: "#CC0033", Dark: "#FF0055"},
        Info:       ext.ThemeColor{Light: "#0088CC", Dark: "#00CCFF"},
        Text:       ext.ThemeColor{Light: "#111111", Dark: "#F0F0F0"},
        Background: ext.ThemeColor{Light: "#F0F0F0", Dark: "#0A0A14"},
    })
})

api.RegisterCommand(ext.CommandDef{
    Name:        "neon",
    Description: "Switch to the neon cyberpunk theme",
    Execute: func(args string, ctx ext.Context) (string, error) {
        if err := ctx.SetTheme("neon"); err != nil {
            return "", err
        }
        return "Neon theme activated!", nil
    },
})
```

### Pattern: Spawning Kit as a Sub-Agent

Use `ctx.SpawnSubagent` to spawn an isolated child Kit instance. The subagent runs as a subprocess with `--json --no-extensions` flags, ensuring isolation.

**Blocking mode** — waits for completion:

```go
_, result, err := ctx.SpawnSubagent(ext.SubagentConfig{
    Prompt:       "Analyze the test files and summarize coverage",
    Model:        "anthropic/claude-haiku-3-5-20241022",  // empty = parent's model
    SystemPrompt: "You are a test analysis expert.",
    Timeout:      2 * time.Minute,  // 0 = 5 minute default
    Blocking:     true,
})
if err != nil {
    ctx.PrintError("spawn failed: " + err.Error())
    return
}
if result.Error != nil {
    ctx.PrintError("subagent failed: " + result.Error.Error())
    return
}
ctx.PrintInfo("Result:\n" + result.Response)
// result.Elapsed, result.ExitCode, result.SessionID
// result.Usage.InputTokens, result.Usage.OutputTokens (if available)
```

**Background mode** — returns immediately with a handle:

```go
handle, _, err := ctx.SpawnSubagent(ext.SubagentConfig{
    Prompt: "Write unit tests for UserService",
    OnOutput: func(chunk string) {
        // Live stderr streaming (progress, tool calls, etc.)
    },
    OnEvent: func(event ext.SubagentEvent) {
        // Real-time events: "text", "reasoning", "tool_call",
        // "tool_result", "tool_execution_start", "tool_execution_end",
        // "turn_start", "turn_end"
        // event.Type, event.Content, event.ToolName, event.ToolArgs, etc.
    },
    OnComplete: func(result ext.SubagentResult) {
        ctx.SendMessage("Subagent finished:\n" + result.Response)
    },
})
// handle.Kill()        — terminate the subagent
// handle.Wait()        — block until completion, returns SubagentResult
// <-handle.Done()      — channel that closes on completion
```

**SubagentConfig fields:**

| Field | Type | Description |
|-------|------|-------------|
| `Prompt` | string | Task instruction (required) |
| `Model` | string | Override model ("provider/model"), empty = parent's |
| `SystemPrompt` | string | Custom system prompt, empty = default |
| `Timeout` | time.Duration | Execution limit, 0 = 5 minutes |
| `Blocking` | bool | Wait for completion vs return handle |
| `NoSession` | bool | Don't persist subagent session file |
| `ParentSessionID` | string | Link to parent session (optional) |
| `OnOutput` | func(string) | Stderr streaming callback |
| `OnEvent` | func(SubagentEvent) | Real-time event callback |
| `OnComplete` | func(SubagentResult) | Completion callback |

**SubagentResult fields:**

| Field | Type | Description |
|-------|------|-------------|
| `Response` | string | Final text response |
| `Error` | error | Non-nil on failure |
| `ExitCode` | int | Process exit code (0 = success) |
| `Elapsed` | time.Duration | Total execution time |
| `Usage` | *SubagentUsage | Token usage (InputTokens, OutputTokens) |
| `SessionID` | string | Subagent's session ID (if persisted) |

You can also spawn Kit as a raw subprocess for simpler cases:

```bash
kit --quiet --no-session --no-extensions --system-prompt "You are a reviewer" --model anthropic/claude-sonnet-4-20250514 "Review this code"
```

---

## Testing Extensions

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
    
    // Test event handlers
    _, err := harness.Emit(extensions.SessionStartEvent{SessionID: "test"})
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    
    // Verify behavior with assertions
    test.AssertPrinted(t, harness, "session started")
    test.AssertWidgetSet(t, harness, "my-widget")
    
    // Test tool blocking
    result, _ := harness.Emit(extensions.ToolCallEvent{ToolName: "dangerous"})
    test.AssertBlocked(t, result, "not allowed")
}
```

**Key testing patterns:**
- Load extensions with `LoadFile()` or `LoadString()` for inline code
- Emit events with `Emit()` to trigger handlers
- Verify with 25+ assertion helpers: `AssertWidgetSet()`, `AssertToolRegistered()`, `AssertPrintInfo()`, etc.
- Mock prompts by setting results on `harness.Context().SetPromptSelectResult()`
- Test multiple scenarios per extension with isolated harness instances

See `examples/extensions/tool-logger_test.go` for a complete example with 14 test cases.

### CLI Testing Commands

```bash
# Validate syntax of all discovered extensions
kit extensions validate

# List loaded extensions
kit extensions list

# Run with a specific extension
kit -e path/to/extension.go

# Run with multiple extensions
kit -e ext1.go -e ext2.go

# Disable all extensions
kit --no-extensions

# Generate an example extension scaffold
kit extensions init
```

---

## Distributing Extensions via Git Repositories

Extensions can be distributed and installed from git repositories using `kit install`. This enables sharing extensions with others and maintaining versioned collections.

### Repository Structure

Extensions support two organization patterns within a repo:

**Single-file extensions** (simple, standalone):
```
my-extension-repo/
├── weather.go           # Single extension file
├── todo.go              # Another extension
└── README.md            # Installation and usage docs
```

**Multi-file extensions** (with `main.go` entry point):
```
my-extension-repo/
├── git-tools/
│   ├── main.go          # Entry point
│   ├── helpers.go       # Supporting code
│   └── config.go        # Configuration
├── todo/
│   ├── main.go          # Entry point
│   └── storage.go       # Storage logic
└── README.md
```

**Hybrid approach** (single files + subdirectories with main.go):
```
my-extensions/
├── weather.go           # Single file extension
├── calculator.go        # Single file extension
├── git-tools/
│   ├── main.go          # Multi-file extension
│   └── utils.go
└── README.md
```

### Installing from Git

Users install extensions using the `kit install` command:

```bash
# Install from GitHub (latest)
kit install github.com/user/repo

# Pin to a specific version/tag
kit install github.com/user/repo@v1.0.0
kit install github.com/user/repo@main
kit install github.com/user/repo@abc1234

# Install locally in project (./.kit/git/)
kit install github.com/user/repo --local

# Interactive selection for repos with multiple extensions
kit install github.com/user/collection --select
```

Supported URL formats:
- `github.com/user/repo` — Shorthand (defaults to HTTPS)
- `git:github.com/user/repo` — Git prefix format
- `https://github.com/user/repo` — HTTPS URL
- `ssh://git@github.com/user/repo` — SSH URL
- `git@github.com:user/repo` — SSH shorthand

### Managing Installed Extensions

```bash
# Update an installed extension (skips pinned versions)
kit install github.com/user/repo --update

# Remove an installed extension
kit install github.com/user/repo --uninstall

# List all loaded extensions
kit extensions list

# Validate all extensions
kit extensions validate
```

### Extension Selection

For repos containing multiple extensions, users can select which to install:

```bash
# Interactive selection
kit install github.com/user/collection --select
```

This prompts the user to choose which extensions to install. Selected extensions are recorded in the manifest, and only those are loaded at runtime (others in the repo are ignored).

### README Template for Extension Repos

Include this in your extension repo's README.md:

```markdown
# My Kit Extensions

A collection of extensions for [Kit](https://github.com/mark3labs/kit).

## Installation

### Install all extensions
\`\`\`bash
kit install github.com/username/repo
\`\`\`

### Install specific extensions
\`\`\`bash
kit install github.com/username/repo --select
\`\`\`

### Install locally in a project
\`\`\`bash
kit install github.com/username/repo --local
\`\`\`

## Extensions

### Extension Name
Description of what it does.

- **Path**: `./ext-name/main.go` or `./ext-name.go`
- **Commands**: `/command-name`
- **Tools**: `tool_name`

## Requirements

- Kit vX.Y.Z+
- Any other dependencies

## Update

\`\`\`bash
kit install github.com/username/repo --update
\`\`\`
```

### Storage Locations

Installed extensions are stored at:

- **Global**: `~/.local/share/kit/git/<host>/<owner>/<repo>/`
- **Project-local**: `./.kit/git/<host>/<owner>/<repo>/`
- **Manifest**: `packages.json` in respective directories

---

## Complete Example: Plan Mode

A full extension that restricts the agent to read-only tools, with a slash command, keyboard shortcut, option, status bar indicator, and system prompt injection:

```go
//go:build ignore

package main

import (
    "strings"
    "kit/ext"
)

func Init(api ext.API) {
    readOnlyTools := []string{"read", "grep", "find", "ls"}
    var planActive bool

    api.RegisterOption(ext.OptionDef{
        Name:        "plan",
        Description: "Start in plan mode (read-only tools)",
        Default:     "false",
    })

    api.RegisterShortcut(ext.ShortcutDef{
        Key:         "ctrl+alt+p",
        Description: "Toggle plan/explore mode",
    }, func(ctx ext.Context) {
        planActive = !planActive
        applyMode(ctx, planActive, readOnlyTools)
    })

    api.RegisterCommand(ext.CommandDef{
        Name:        "plan",
        Description: "Toggle plan/explore mode",
        Execute: func(args string, ctx ext.Context) (string, error) {
            planActive = !planActive
            applyMode(ctx, planActive, readOnlyTools)
            return "", nil
        },
    })

    api.OnSessionStart(func(_ ext.SessionStartEvent, ctx ext.Context) {
        if strings.ToLower(ctx.GetOption("plan")) == "true" {
            planActive = true
            applyMode(ctx, true, readOnlyTools)
        }
    })

    api.OnBeforeAgentStart(func(_ ext.BeforeAgentStartEvent, ctx ext.Context) *ext.BeforeAgentStartResult {
        if !planActive {
            return nil
        }
        prompt := `You are in PLAN MODE (read-only). You can ONLY read and search.
Focus on understanding, analysis, and generating plans.`
        return &ext.BeforeAgentStartResult{SystemPrompt: &prompt}
    })
}

func applyMode(ctx ext.Context, active bool, tools []string) {
    if active {
        ctx.SetActiveTools(tools)
        ctx.SetStatus("plan-mode", "PLAN MODE (read-only)", 10)
        ctx.PrintInfo("Plan mode ON")
    } else {
        ctx.SetActiveTools(nil)
        ctx.RemoveStatus("plan-mode")
        ctx.PrintInfo("Plan mode OFF")
    }
}
```

## Key Files for Reference

- [`internal/extensions/api.go`](https://github.com/mark3labs/kit/blob/main/internal/extensions/api.go) — Complete API type definitions
- [`internal/extensions/runner.go`](https://github.com/mark3labs/kit/blob/main/internal/extensions/runner.go) — Event dispatch and state management
- [`internal/extensions/loader.go`](https://github.com/mark3labs/kit/blob/main/internal/extensions/loader.go) — Yaegi interpreter setup
- [`internal/extensions/symbols.go`](https://github.com/mark3labs/kit/blob/main/internal/extensions/symbols.go) — All types exported to extensions
- [`pkg/extensions/test/`](https://github.com/mark3labs/kit/tree/main/pkg/extensions/test) — Testing package with harness, mocks, and assertions
- [`examples/extensions/tool-logger_test.go`](https://github.com/mark3labs/kit/blob/main/examples/extensions/tool-logger_test.go) — Complete test example
- [`examples/extensions/`](https://github.com/mark3labs/kit/tree/main/examples/extensions) — 25+ working example extensions
