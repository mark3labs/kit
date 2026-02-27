# Simplify Streaming & Styling Implementation Plan

## Overview

Kit's `internal/ui/` layer has accumulated complexity that can be reduced without touching the SDK's public API or changing user-visible behavior. This plan targets renderer duplication, an over-engineered block renderer, dead-weight abstractions, and minor SDK API hygiene -- all informed by a comparative analysis with Charm's crush codebase.

## Current State Analysis

### What's Good (Keep)
- **SDK event bus** (`pkg/kit/events.go`): Clean, type-safe, correct separation for "build your own kit" consumers.
- **SDK -> App event translation** (`internal/app/app.go:444-475`): Necessary bridge between public API and internal TUI. Single `Subscribe()` call, clean mapping.
- **Tool-specific renderers** (`internal/ui/tool_renderers.go`): Side-by-side diff, syntax-highlighted code, green-tinted Write blocks, bash output -- all useful.
- **Deferred markdown rendering** in `StreamComponent`: Accumulates chunks in `strings.Builder`, only calls glamour on flush. Correct approach.
- **KITT spinner** (`stream.go`, `spinner.go`): Clean, theme-aware, shared between TUI and stderr paths.
- **Theme system** (`enhanced_styles.go`): Catppuccin-based adaptive dark/light. Solid.

### What's Not Good (Change)

1. **Renderer duplication**: `MessageRenderer` (961 lines) and `CompactRenderer` (512 lines) share near-identical logic:
   - `formatBashOutput()` copy-pasted (~60 lines each, `messages.go:656` and `compact_renderer.go:447`)
   - `formatToolArgs()` duplicated (`messages.go:594` and `compact_renderer.go:387`)
   - Every `print*` method in `model.go` (lines 882-936) has `if m.compactMode { compact } else { standard }` branching
   - Both renderers already share `renderToolBody()` and `formatToolParams()` -- proving the pattern works

2. **Block renderer 3-phase pipeline**: `block_renderer.go:170-233` only triggers for `WithBackground()`, which is used by **one call site**: user messages (`messages.go:214`). 60 lines of complexity for one message type.

3. **`MessageContainer` is dead weight for TUI**: The TUI model (`model.go`) never uses it -- renders directly via `printUserMessage()` -> `tea.Println()`. Only `cli.go` uses it, and immediately clears after each display (`cli.go:261`). A stateful container used statelessly.

4. **Dual markdown paths**: `toMarkdown()` and `toMarkdownWithBg()` (`styles.go:332-344`) exist because user messages pass a background hex. Removing user message backgrounds collapses this to one function.

5. **SDK public API leaks**: `SetupAgent()`, `AgentSetupOptions`, `BuildProviderConfig()` are exported from `pkg/kit/` but only called internally by `New()` (`kit.go:232`). `Options` has 4 CLI-specific fields.

### Key Discoveries
- `WithBackground()` is called exactly once: `messages.go:214` for user messages
- `toMarkdownWithBg()` is called exactly once: `messages.go:199` via `renderMarkdownWithBg()`
- `SetupAgent` is called exactly once: `kit.go:232` inside `New()`
- `BuildProviderConfig` is called exactly once: `setup.go:87` inside `SetupAgent()`
- `GetAgent()` is used in `cmd/root.go:375` to call `CollectAgentMetadata()` which needs `GetTools()`, `GetLoadingMessage()`, `GetLoadedServerNames()`, `GetMCPToolCount()`, `GetExtensionToolCount()`
- `GetExtRunner()` is used in `cmd/root.go:405` to set extension context and emit `SessionStart`
- `GetBufferedLogger()` is used in `cmd/root.go:387` to flush debug messages in non-interactive mode
- `formatBashOutput()` has identical semantics in both renderers (parse `<stdout>`/`<stderr>` tags, style stderr with error color) but slightly different signatures

## Desired End State

After this plan is complete:
- **One renderer interface** with two implementations sharing all common logic via a base struct
- **No `MessageContainer`** -- both CLI and TUI paths render messages directly
- **Single-pass block rendering** -- no 3-phase pipeline
- **Single markdown path** -- no `toMarkdownWithBg()` / `renderMarkdownWithBg()`
- **Tighter SDK API** -- `SetupAgent` and friends are internal; `Options` is split
- **All existing tests pass**, visual output is unchanged (or trivially different for user message background)

### Verification
- `go build -o output/kit ./cmd/kit` succeeds
- `go test -race ./...` passes
- `go vet ./...` clean
- Manual: interactive TUI renders messages identically (user messages lose background fill but keep border)
- Manual: `--prompt` non-interactive mode output is identical
- Manual: compact mode output is identical

## What We're NOT Doing

- Changing the SDK event bus or event types
- Changing the app layer (`internal/app/`) event translation
- Modifying the streaming pipeline (chunk accumulation, deferred flush)
- Altering tool-specific renderers (diff, code, write, bash)
- Touching the KITT spinner implementation
- Changing the Catppuccin theme or color palette
- Using alternate screen buffer

## Implementation Approach

Bottom-up: start with the shared rendering infrastructure, then remove dead code, then clean up the SDK surface. Each phase is independently shippable.

---

## Phase 1: Unify Renderers

### Overview
Extract duplicated logic into shared functions and introduce a `Renderer` interface so the `if compact { ... } else { ... }` branching throughout `model.go` collapses to a single call.

### Changes Required

#### 1. Extract shared formatting functions
**File**: `internal/ui/format.go` (new)
**Changes**: Move duplicated functions out of both renderers into package-level helpers.

```go
package ui

// parseBashOutput parses <stdout>/<stderr> tagged output and returns
// styled text. Shared by both MessageRenderer and CompactRenderer.
func parseBashOutput(result string, theme Theme) (stdout string, stderr string) {
    // ... extracted from messages.go:657-725 and compact_renderer.go:447-511
}

// formatToolArgsCompact formats tool arguments for compact single-line display.
func formatToolArgsCompact(args string, maxWidth int) string {
    // ... extracted from compact_renderer.go:387-409
}
```

Functions to extract:
- `formatBashOutput()` -- merge the two implementations (identical logic, different return types)
- `formatToolArgsCompact()` from `CompactRenderer`
- `formatToolArgs()` from `MessageRenderer` (the JSON-stripping version)

#### 2. Define Renderer interface
**File**: `internal/ui/messages.go`
**Changes**: Add interface at top of file.

```go
// Renderer is the interface satisfied by both MessageRenderer and
// CompactRenderer. It allows model.go to call rendering methods without
// branching on compact mode.
type Renderer interface {
    RenderUserMessage(content string, timestamp time.Time) UIMessage
    RenderAssistantMessage(content string, timestamp time.Time, modelName string) UIMessage
    RenderToolMessage(toolName, toolArgs, toolResult string, isError bool) UIMessage
    RenderSystemMessage(content string, timestamp time.Time) UIMessage
    RenderErrorMessage(errorMsg string, timestamp time.Time) UIMessage
    RenderDebugMessage(message string, timestamp time.Time) UIMessage
    RenderDebugConfigMessage(config map[string]any, timestamp time.Time) UIMessage
    SetWidth(width int)
}
```

Both `MessageRenderer` and `CompactRenderer` already satisfy this (they have identical method signatures). No code changes needed on either struct.

#### 3. Collapse branching in model.go
**File**: `internal/ui/model.go`
**Changes**: Replace the dual `renderer` + `compactRdr` fields with a single `renderer Renderer` field.

Replace fields at `model.go:160-166`:
```go
// Before:
renderer   *MessageRenderer
compactRdr *CompactRenderer
compactMode bool

// After:
renderer    Renderer
compactMode bool   // retained for StreamComponent selection
```

Update constructor at `model.go:268-269`:
```go
// Before:
renderer:   NewMessageRenderer(width, false),
compactRdr: NewCompactRenderer(width, false),

// After:
renderer: func() Renderer {
    if opts.CompactMode {
        return NewCompactRenderer(width, false)
    }
    return NewMessageRenderer(width, false)
}(),
```

Collapse all `print*` helpers (`model.go:882-1001`). Example for `printUserMessage`:
```go
// Before (model.go:882-892):
func (m *AppModel) printUserMessage(text string) tea.Cmd {
    var rendered string
    if m.compactMode {
        msg := m.compactRdr.RenderUserMessage(text, time.Now())
        rendered = msg.Content
    } else {
        msg := m.renderer.RenderUserMessage(text, time.Now())
        rendered = msg.Content
    }
    return tea.Println(rendered)
}

// After:
func (m *AppModel) printUserMessage(text string) tea.Cmd {
    return tea.Println(m.renderer.RenderUserMessage(text, time.Now()).Content)
}
```

Apply the same pattern to: `printAssistantMessage`, `printToolResult`, `printErrorResponse`, `printSystemMessage`, `PrintStartupInfo`.

#### 4. Update cli.go to use Renderer interface
**File**: `internal/ui/cli.go`
**Changes**: Replace dual renderer fields with single `Renderer`.

```go
// Before (cli.go:18-19):
messageRenderer  *MessageRenderer
compactRenderer  *CompactRenderer

// After:
renderer Renderer
```

Collapse all `Display*` methods the same way as model.go.

#### 5. Collapse formatBashOutput duplication
**File**: `internal/ui/messages.go` and `internal/ui/compact_renderer.go`
**Changes**: Both `formatBashOutput` methods call the new shared `parseBashOutput()` from `format.go`, then apply their own styling (MessageRenderer applies width, CompactRenderer doesn't).

### Success Criteria

#### Automated Verification:
- [ ] Build succeeds: `go build -o output/kit ./cmd/kit`
- [ ] All tests pass: `go test -race ./...`
- [ ] No lint errors: `go vet ./...`
- [ ] Format clean: `go fmt ./...`

#### Manual Verification:
- [ ] Interactive TUI renders all message types correctly (user, assistant, tool, system, error)
- [ ] Compact mode renders correctly
- [ ] `--prompt` non-interactive mode renders correctly
- [ ] Tool-specific renderers (diff, code, write, bash) display unchanged

**Implementation Note**: After completing this phase and all automated verification passes, pause here for manual confirmation from the human that the manual testing was successful before proceeding to the next phase.

---

## Phase 2: Simplify Block Renderer

### Overview
Remove the 3-phase background rendering pipeline. User messages switch from background fill to border-only (matching assistant messages). This eliminates `WithBackground()`, `toMarkdownWithBg()`, `renderMarkdownWithBg()`, and the `generateMarkdownStyleConfig(bgHex)` background propagation path.

### Changes Required

#### 1. Remove user message background
**File**: `internal/ui/messages.go`
**Changes**: In `RenderUserMessage()`, remove `WithBackground(theme.Highlight)` and the `renderMarkdownWithBg` call.

```go
// Before (messages.go:196-216):
bgHex := colorHex(theme.Highlight)
messageContent := r.renderMarkdownWithBg(content, r.width-8, bgHex)
...
rendered := renderContentBlock(
    fullContent, r.width,
    WithAlign(lipgloss.Left),
    WithBorderColor(theme.Primary),
    WithBackground(theme.Highlight),
    WithMarginBottom(1),
)

// After:
messageContent := r.renderMarkdown(content, r.width-8)
...
rendered := renderContentBlock(
    fullContent, r.width,
    WithAlign(lipgloss.Left),
    WithBorderColor(theme.Primary),
    WithMarginBottom(1),
)
```

User messages will now have a left border (like system messages) but no background fill. The `theme.Primary` border color (Catppuccin Mauve) still visually distinguishes them.

#### 2. Remove 3-phase pipeline from block_renderer.go
**File**: `internal/ui/block_renderer.go`
**Changes**: Remove the `WithBackground()` option, the `bgColor` field, and the entire `if hasBg { ... }` branch (lines 170-233). Only the simple single-pass path remains.

The `blockRenderer` struct drops the `bgColor` field. `renderContentBlock()` shrinks from ~145 lines to ~60.

#### 3. Remove markdown background helpers
**File**: `internal/ui/styles.go`
**Changes**: Remove `toMarkdownWithBg()` (lines 338-344). Remove the `bgHex` parameter from `GetMarkdownRenderer()` and `generateMarkdownStyleConfig()`. Remove all `BackgroundColor: docBg` fields and `IndentToken` background styling from the glamour config.

**File**: `internal/ui/messages.go`
**Changes**: Remove `renderMarkdownWithBg()` method (lines 733-738).

#### 4. Remove colorHex helper if unused
**File**: `internal/ui/enhanced_styles.go`
**Changes**: Check if `colorHex()` is still used elsewhere. If not, remove it.

### Success Criteria

#### Automated Verification:
- [ ] Build succeeds: `go build -o output/kit ./cmd/kit`
- [ ] All tests pass: `go test -race ./...`
- [ ] No lint errors: `go vet ./...`

#### Manual Verification:
- [ ] User messages display with left border (no background fill) -- visually acceptable
- [ ] Markdown inside user messages renders correctly without background
- [ ] All other message types unchanged
- [ ] Code blocks in user messages (if any) still render correctly

**Implementation Note**: After completing this phase and all automated verification passes, pause here for manual confirmation from the human that the manual testing was successful before proceeding to the next phase.

---

## Phase 3: Remove MessageContainer

### Overview
`MessageContainer` (`messages.go:740-961`) is not used by the TUI. The CLI path uses it as a one-shot printer (add message, render, clear). Replace CLI usage with direct renderer calls.

### Changes Required

#### 1. Simplify CLI to use Renderer directly
**File**: `internal/ui/cli.go`
**Changes**: Remove the `messageContainer` field. Each `Display*` method calls the renderer and prints directly:

```go
// Before (cli.go:96-105):
func (c *CLI) DisplayUserMessage(message string) {
    var msg UIMessage
    if c.compactMode {
        msg = c.compactRenderer.RenderUserMessage(message, time.Now())
    } else {
        msg = c.messageRenderer.RenderUserMessage(message, time.Now())
    }
    c.messageContainer.AddMessage(msg)
    c.displayContainer()
}

// After (with Phase 1's Renderer interface):
func (c *CLI) DisplayUserMessage(message string) {
    fmt.Println(c.renderer.RenderUserMessage(message, time.Now()).Content)
}
```

Apply the same to: `DisplayAssistantMessageWithModel`, `DisplayToolMessage`, `DisplayError`, `DisplayInfo`, `DisplayCancellation`, `DisplayDebugMessage`, `DisplayDebugConfig`.

Remove `displayContainer()` method (`cli.go:254-262`).

#### 2. Delete MessageContainer
**File**: `internal/ui/messages.go`
**Changes**: Remove `MessageContainer` struct and all its methods (lines 740-961): `NewMessageContainer`, `AddMessage`, `SetModelName`, `UpdateLastMessage`, `Clear`, `SetSize`, `Render`, `renderEmptyState`, `renderCompactMessages`, `renderCompactEmptyState`.

This removes ~220 lines including the welcome screen rendering. The welcome screen (`renderEmptyState`) is not shown in the TUI path (startup info is printed via `PrintStartupInfo()`). For the CLI path, the startup info is displayed via `SetupCLI()` in `factory.go:116-133`.

#### 3. Remove unused CLI fields
**File**: `internal/ui/cli.go`
**Changes**: Remove `height` field (only used by `MessageContainer`). Remove `modelName` field (pass directly to renderer calls -- already available on the `CLI` struct from `SetModelName`). Simplify `updateSize()` to not propagate to a container.

### Success Criteria

#### Automated Verification:
- [ ] Build succeeds: `go build -o output/kit ./cmd/kit`
- [ ] All tests pass: `go test -race ./...`
- [ ] No lint errors: `go vet ./...`

#### Manual Verification:
- [ ] `--prompt` mode displays all message types correctly
- [ ] `--prompt --compact` mode works
- [ ] Debug messages display in debug mode
- [ ] No welcome screen regression (startup info still shown via `PrintStartupInfo` / `SetupCLI`)

**Implementation Note**: After completing this phase and all automated verification passes, pause here for manual confirmation from the human that the manual testing was successful before proceeding to the next phase.

---

## Phase 4: SDK Public API Tightening

### Overview
Move internal-only exports out of `pkg/kit/` and split `Options` to separate SDK concerns from CLI concerns. Add narrower accessor methods to replace `GetAgent()`.

### Changes Required

#### 1. Move SetupAgent to internal
**File**: `pkg/kit/setup.go` -> `internal/kitsetup/setup.go` (new package)
**Changes**: Move `SetupAgent()`, `AgentSetupOptions`, `AgentSetupResult`, `BuildProviderConfig()`, `extensionCreationOpts`, and `loadExtensions()` to a new `internal/kitsetup` package.

Update `pkg/kit/kit.go:232` to call `kitsetup.SetupAgent()` instead.

Note: `AgentSetupResult` references `*agent.Agent`, `*tools.BufferedDebugLogger`, and `*extensions.Runner` -- all internal types. This is fine since the new package is also internal.

#### 2. Split Options
**File**: `pkg/kit/kit.go`
**Changes**: Move CLI-specific fields out of `Options` into a separate struct consumed only by the CLI.

```go
// Options configures Kit creation for SDK users.
type Options struct {
    Model        string
    SystemPrompt string
    ConfigFile   string
    MaxSteps     int
    Streaming    bool
    Quiet        bool
    Tools        []Tool
    ExtraTools   []Tool

    // Session configuration
    SessionDir  string
    SessionPath string
    Continue    bool
    NoSession   bool

    // Skills
    Skills    []string
    SkillsDir string

    // Compaction
    AutoCompact       bool
    CompactionOptions *CompactionOptions

    // Debug enables debug logging for the SDK.
    Debug bool
}

// CLIOptions holds fields only relevant to the CLI binary.
// SDK users should not need these.
type CLIOptions struct {
    MCPConfig         *config.Config
    ShowSpinner       bool
    SpinnerFunc       SpinnerFunc
    UseBufferedLogger bool
}
```

Update `New()` to accept `CLIOptions` as an optional second parameter or embed it in a `NewWithCLIOptions()` variant. The simplest approach: add `CLIOptions *CLIOptions` as an optional field on `Options` itself:

```go
type Options struct {
    // ... SDK fields ...

    // CLI is optional CLI-specific configuration. SDK users leave this nil.
    CLI *CLIOptions
}
```

This keeps `New()` signature unchanged while making it clear which fields are SDK vs CLI.

#### 3. Add narrower accessor methods
**File**: `pkg/kit/kit.go`
**Changes**: Add methods that expose only what `cmd/root.go` actually needs, without leaking internal types.

```go
// GetToolNames returns the names of all tools available to the agent.
func (m *Kit) GetToolNames() []string {
    tools := m.agent.GetTools()
    names := make([]string, len(tools))
    for i, t := range tools {
        names[i] = t.Info().Name
    }
    return names
}

// GetLoadingMessage returns the agent's startup info message (e.g. GPU
// fallback info), or empty string if none.
func (m *Kit) GetLoadingMessage() string {
    return m.agent.GetLoadingMessage()
}

// GetLoadedServerNames returns the names of successfully loaded MCP servers.
func (m *Kit) GetLoadedServerNames() []string {
    return m.agent.GetLoadedServerNames()
}

// GetMCPToolCount returns the number of tools loaded from external MCP servers.
func (m *Kit) GetMCPToolCount() int {
    return m.agent.GetMCPToolCount()
}

// GetExtensionToolCount returns the number of tools registered by extensions.
func (m *Kit) GetExtensionToolCount() int {
    return m.agent.GetExtensionToolCount()
}
```

#### 4. Update cmd/root.go to use new accessors
**File**: `cmd/root.go` and `cmd/setup.go`
**Changes**: Replace `kitInstance.GetAgent()` calls with the new narrower methods. `CollectAgentMetadata()` takes `*kit.Kit` instead of `*agent.Agent`.

```go
// Before (cmd/setup.go:17):
func CollectAgentMetadata(mcpAgent *agent.Agent, mcpConfig *config.Config) (...)

// After:
func CollectAgentMetadata(k *kit.Kit, mcpConfig *config.Config) (...)
```

For `GetExtRunner()`: The only usage is in `cmd/root.go:405-441` to set extension context. This can be handled by adding a `SetExtensionContext()` method on Kit that wraps the runner interaction:

```go
// SetExtensionContext configures the extension runner with the given
// context functions. No-op if extensions are disabled.
func (m *Kit) SetExtensionContext(ctx ExtensionContext) { ... }

// EmitSessionStart fires the SessionStart event for extensions.
func (m *Kit) EmitSessionStart() { ... }
```

For `GetBufferedLogger()`: The only usage is `cmd/root.go:387-391` to flush debug messages. Add:

```go
// GetBufferedDebugMessages returns any debug messages that were buffered
// during initialization, then clears the buffer. Returns nil if no
// messages were buffered.
func (m *Kit) GetBufferedDebugMessages() []string { ... }
```

#### 5. Deprecate old accessors
**File**: `pkg/kit/kit.go`
**Changes**: Mark `GetAgent()`, `GetExtRunner()`, `GetBufferedLogger()` as deprecated but keep them for one release cycle:

```go
// Deprecated: Use GetToolNames(), GetLoadingMessage(), etc. instead.
func (m *Kit) GetAgent() *agent.Agent { return m.agent }
```

### Success Criteria

#### Automated Verification:
- [ ] Build succeeds: `go build -o output/kit ./cmd/kit`
- [ ] All tests pass: `go test -race ./...`
- [ ] No lint errors: `go vet ./...`
- [ ] `go doc pkg/kit` shows clean public API without `SetupAgent`

#### Manual Verification:
- [ ] Interactive mode works end-to-end
- [ ] `--prompt` mode works end-to-end
- [ ] Extensions load and dispatch correctly
- [ ] Debug mode shows buffered messages in non-interactive mode

**Implementation Note**: After completing this phase and all automated verification passes, pause here for manual confirmation from the human that the manual testing was successful before proceeding to the next phase.

---

## Testing Strategy

### Unit Tests
- Verify `Renderer` interface is satisfied by both implementations (compile-time check via `var _ Renderer = (*MessageRenderer)(nil)`)
- Verify `parseBashOutput()` handles all tag combinations (stdout only, stderr only, mixed, no tags)
- Verify `formatToolParams()` truncation behavior (existing test coverage)
- Verify `SetupAgent` is no longer importable from external packages

### Integration Tests
- Existing `model_test.go` and `children_test.go` continue to pass
- Existing `usage_tracker_test.go` and `usage_tracker_render_test.go` continue to pass

### Manual Testing Steps
1. Start interactive TUI, send a message, verify user/assistant/tool rendering
2. Send a message that triggers tool calls (bash, read, edit), verify tool blocks
3. Use `/compact` command, verify compaction summary renders
4. Use `--prompt "list files"` in non-interactive mode, verify output
5. Use `--prompt --compact "list files"`, verify compact output
6. Use `--prompt --quiet "list files"`, verify only response text appears
7. Start with `--debug`, verify debug messages display
8. Verify `/usage`, `/tools`, `/servers`, `/help` commands render correctly

## Performance Considerations

- Removing `MessageContainer` eliminates one allocation + clear cycle per message in CLI mode
- Removing the 3-phase block render eliminates two extra `lipgloss.Place()` / `lipgloss.Width()` calls per user message
- Collapsing the renderer branching eliminates one virtual dispatch vs two field lookups -- negligible
- No performance regressions expected; this is purely code simplification

## Migration Notes

- `SetupAgent`, `AgentSetupOptions`, `AgentSetupResult`, `BuildProviderConfig` move from public to internal. Any external SDK consumer using these (unlikely -- they're undocumented escape hatches) would need to use `kit.New()` instead.
- User messages lose their subtle background tint. This is a minor visual change. The border color (`theme.Primary`) still distinguishes them clearly from assistant messages (no border).
- `GetAgent()`, `GetExtRunner()`, `GetBufferedLogger()` are deprecated, not removed. External consumers have a migration path to the narrower methods.

## References
- Crush streaming architecture: `internal/agent/hyper/provider.go`, `internal/app/app.go` (non-interactive path)
- Crush styling: `internal/ui/styles/styles.go`, `internal/ui/chat/assistant.go` (render caching)
- Kit block renderer: `internal/ui/block_renderer.go:170-233` (3-phase pipeline)
- Kit renderer duplication: `internal/ui/messages.go:656` vs `internal/ui/compact_renderer.go:447`
- Kit SDK setup: `pkg/kit/setup.go` (only called from `pkg/kit/kit.go:232`)
