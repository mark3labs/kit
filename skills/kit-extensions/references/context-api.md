# Kit Extensions: Context API Reference

> Part of the `kit-extensions` skill. Read `SKILL.md` first for the overview and critical constraints.

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

**`HandleKey` runs synchronously on the TUI event loop** — unlike a shortcut
handler, blocking here freezes the whole interface. Keep it fast; hand slow work
to a goroutine.

It is also the last hook before the editor, so it never sees keys an earlier
stage consumed: `ctrl+c`, any registered extension shortcut, `esc` during a
running turn, `ctrl+x` and its chord suffix, and scrollback keys like `pgup`.

### Terminal Size

```go
width, height := ctx.GetTerminalSize()  // 0, 0 outside the interactive TUI
```

Footer, header, and widget content is rendered at **full terminal width with no
truncation** — a longer line wraps and silently consumes a row of scrollback.
Measure and truncate before calling `SetFooter`/`SetHeader`/`SetWidget`, and
re-render on `OnTerminalResize`.

This is a **function, not a field**, so it reports the live size. A long-lived
goroutine (a ticking clock in a footer, say) that captured a `Context` still
observes resizes; a struct field would freeze at the value copied when the
handler was invoked.

Note that multi-byte characters occupy more than one column — count display
width, not bytes or runes, when fitting to `width`.

### Thinking Level

```go
level := ctx.GetThinkingLevel()  // "off", "none", "minimal", "low", "medium", "high"
```

Models without reasoning support report `"off"`. To distinguish "reasoning is
switched off" from "this model cannot reason at all", pair it with
`ctx.GetModelCapabilities("").Reasoning`.

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

// Aggregated token usage and cost for the session (interactive TUI only):
usage := ctx.GetSessionUsage()
// usage.TotalInputTokens, .TotalOutputTokens
// usage.TotalCacheReadTokens, .TotalCacheWriteTokens
// usage.TotalCost float64 — USD
// usage.RequestCount int
// usage.IsOAuth bool — true for subscription credentials (Claude Pro/Max).
//   TotalCost is always 0 under OAuth because the user is not billed per
//   token; check this flag to distinguish "not billed" from "$0 spent".

// Append-only log in the session tree (fork-aware, walked on every branch read):
id, err := ctx.AppendEntry("my-type", "data string")
entries := ctx.GetEntries("my-type")  // []ext.ExtensionEntry{ID, EntryType, Data, Timestamp}
```

### Session State (last-write-wins)

Key-value store scoped to the session, persisted to a sidecar file
(`<session>.ext-state.json`) outside the conversation tree. Reads are O(1)
(no branch walk), writes don't grow the JSONL, and the store is not
duplicated on fork. State is invisible to the LLM and survives session
resume. For ephemeral / in-memory sessions, state lives only in memory.

```go
ctx.SetState("myext:budget-cap", "10.00")          // last write wins
val, ok := ctx.GetState("myext:budget-cap")        // (string, bool)
ctx.DeleteState("myext:budget-cap")                // no-op if missing
keys := ctx.ListState()                            // []string, unspecified order
```

**When to use which:**

| Need | Use |
|------|-----|
| Snapshot state ("current value of X") | `SetState` / `GetState` |
| Audit log / event history | `AppendEntry` / `GetEntries` |
| One-shot per-turn signal | enriched `AgentEndEvent` fields |
| Per-LLM-call observation | `OnLLMUsage` event |

Namespace keys with your extension name (e.g. `"myext:budget-cap"`) to avoid
collisions across extensions.

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

