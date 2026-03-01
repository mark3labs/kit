# Kit vs Pi Extension System: Comprehensive Gap Analysis

> Generated: 2026-03-01
> Source: [pi-mono extensions](https://github.com/badlogic/pi-mono/tree/main/packages/coding-agent/examples/extensions)

---

## Executive Summary

Pi's extension ecosystem contains **57+ example extensions** spanning safety guards, git integration, custom providers, session management, resource discovery, and advanced UI patterns. Kit has **10 example extensions** and a solid foundation with 13 lifecycle events and a rich widget/overlay/editor system, but lacks several critical API surfaces that Pi exposes. The gaps fall into three tiers:

- **Critical (17 gaps)**: Missing API capabilities that block entire categories of extensions
- **Moderate (7 gaps)**: Capabilities that exist but lack depth compared to Pi
- **Covered (14 areas)**: Capabilities where Kit has parity or near-parity

---

## Pi Extension Inventory (57 extensions)

### Safety & Lifecycle (5)
| Extension | Description |
|---|---|
| `permission-gate.ts` | Confirms dangerous bash commands (rm -rf, sudo) |
| `protected-paths.ts` | Blocks writes to protected paths (.env, .git/) |
| `confirm-destructive.ts` | Confirms destructive session actions (clear, fork) |
| `dirty-repo-guard.ts` | Prevents session changes with uncommitted git |
| `sandbox/` | OS-level sandboxing via `@anthropic-ai/sandbox-runtime` |

### Git Integration (2)
| Extension | Description |
|---|---|
| `git-checkpoint.ts` | Git stash checkpoints per turn, restore on fork |
| `auto-commit-on-exit.ts` | Auto-commits using assistant message as commit msg |

### Custom Tools (11)
| Extension | Description |
|---|---|
| `hello.ts` | Minimal custom tool |
| `question.ts` | Agent-initiated user questions with custom UI |
| `questionnaire.ts` | Multi-question tabbed tool |
| `tool-override.ts` | Override built-in tools |
| `built-in-tool-renderer.ts` | Custom compact rendering for built-in tools |
| `minimal-mode.ts` | Override all tool rendering for minimal display |
| `truncated-tool.ts` | Ripgrep with output truncation |
| `antigravity-image-gen.ts` | Image generation with external API |
| `ssh.ts` | Delegate tools to remote via SSH |
| `subagent/` | Delegate to specialized subagents |
| `todo.ts` | Todo list tool with state persistence |

### Commands & UI (25)
| Extension | Description |
|---|---|
| `preset.ts` | Named presets (model, thinking, tools, instructions) |
| `plan-mode/` | Read-only exploration with /plan command |
| `tools.ts` | Interactive /tools to enable/disable tools |
| `handoff.ts` | Transfer context to new session |
| `qna.ts` | Extract questions from response into editor |
| `commands.ts` | /commands with introspection and tab completion |
| `model-status.ts` | Model change notifications in status bar |
| `send-user-message.ts` | Programmatic message injection (3 modes) |
| `timed-confirm.ts` | Auto-dismissing dialogs with AbortSignal |
| `rpc-demo.ts` | Full UI method catalog |
| `modal-editor.ts` | Vim-like modal editor (full editor replacement) |
| `rainbow-editor.ts` | Animated rainbow editor |
| `notify.ts` | Desktop OS notifications |
| `titlebar-spinner.ts` | Terminal title animations |
| `summarize.ts` | Conversation summarization |
| `custom-footer.ts` | Custom footer with git branch + token stats |
| `custom-header.ts` | Custom ASCII art header |
| `overlay-test.ts` | Custom overlay with Focusable components |
| `overlay-qa-tests.ts` | Comprehensive overlay QA tests |
| `doom-overlay/` | DOOM game running as overlay |
| `shutdown-command.ts` | /quit command via ctx.shutdown() |
| `reload-runtime.ts` | Hot reload extensions at runtime |
| `interactive-shell.ts` | Full terminal takeover for vim/htop |
| `inline-bash.ts` | !{command} expansion in prompts |
| `snake.ts` | Snake game with custom UI |

### System Prompt & Compaction (4)
| Extension | Description |
|---|---|
| `pirate.ts` | System prompt append |
| `claude-rules.ts` | Project rules loader from .claude/rules/ |
| `custom-compaction.ts` | Custom compaction with cross-model summarization |
| `trigger-compact.ts` | Auto-trigger compaction at token threshold |

### System Integration (1)
| Extension | Description |
|---|---|
| `mac-system-theme.ts` | Syncs theme with macOS dark/light mode |

### Resources (1)
| Extension | Description |
|---|---|
| `dynamic-resources/` | Dynamic skill/prompt/theme loading |

### Messages & Communication (2)
| Extension | Description |
|---|---|
| `message-renderer.ts` | Custom message rendering with expandable details |
| `event-bus.ts` | Inter-extension pub/sub communication |

### Session Metadata (2)
| Extension | Description |
|---|---|
| `session-name.ts` | Name sessions for selector |
| `bookmark.ts` | Bookmark entries with labels for /tree |

### Custom Providers (3)
| Extension | Description |
|---|---|
| `custom-provider-anthropic/` | Custom Anthropic provider with OAuth |
| `custom-provider-gitlab-duo/` | GitLab Duo provider |
| `custom-provider-qwen-cli/` | Qwen CLI provider |

### External Dependencies (1)
| Extension | Description |
|---|---|
| `with-deps/` | Extension with own package.json |

---

## Kit Extension Inventory (10 extensions)

| Extension | Description |
|---|---|
| `minimal.go` | UI visibility, footer, context stats polling |
| `widget-status.go` | Persistent widgets, OnToolResult, input commands |
| `tool-logger.go` | Tool event logging, PrintBlock with styling |
| `header-footer-demo.go` | Custom header/footer with slash commands |
| `prompt-demo.go` | All 3 prompt types + chained workflow |
| `overlay-demo.go` | Modal overlay dialogs (info, actions, markdown, scroll) |
| `custom-editor-demo.go` | Vim-like editor interceptor |
| `tool-renderer-demo.go` | Custom tool rendering for read/bash |
| `subagent-widget.go` | Background subprocess agents with live widgets |
| `kit-kit.go` | Meta-agent with parallel experts, grid widget |

---

## Gap Analysis: Critical Gaps (Missing API Capabilities)

### Gap 1: Session Management API
**Pi has:** `ctx.sessionManager` with full conversation access
- `getEntries()` / `getBranch()` -- Read conversation history
- `getLeafEntry()` -- Current leaf entry
- `getLabel(entryId)` / `pi.setLabel()` -- Entry metadata/labeling
- `getSessionFile()` -- Session file path

**Kit has:** Nothing. Extensions cannot read conversation history.

**Impact:** Blocks auto-commit (needs last assistant message), git checkpoints (needs entry IDs), handoff (needs full conversation), QnA extraction, state restoration on session resume, bookmark labeling.

**Implementation approach:**
- Add `GetMessages func() []MessageEntry` to `Context` returning conversation messages
- Add `GetCurrentEntryID func() string` for session tree position
- Add `SetLabel func(entryId, label string)` / `GetLabel func(entryId string) string`
- Wire in `cmd/root.go` via closures reading from the session store

---

### Gap 2: Model Management API
**Pi has:**
- `ctx.modelRegistry.find(provider, model)` -- Look up models
- `ctx.modelRegistry.getApiKey(model)` -- Get API keys
- `pi.setModel(model)` -- Change active model at runtime
- `pi.setThinkingLevel(level)` -- Set reasoning budget

**Kit has:** `ctx.Model string` (read-only model name)

**Impact:** Blocks preset system (model switching), custom compaction (cross-model calls), QnA extraction (direct LLM calls), any extension needing to invoke a different model.

**Implementation approach:**
- Add `SetModel func(provider, model string) error` to `Context`
- Add `GetAvailableModels func() []ModelInfo` returning provider/model/context info
- Add `GetAPIKey func(provider string) string` for credential access
- Add `SetThinkingLevel func(level string)` for reasoning budget control
- Wire through to the existing `llm.Provider` interface

---

### Gap 3: Tool Management API
**Pi has:**
- `pi.getAllTools()` -- List all registered tools
- `pi.setActiveTools(names)` -- Enable/disable specific tools

**Kit has:** Nothing for tool introspection or filtering.

**Impact:** Blocks plan-mode (restricts tools to read-only set), preset system (tool filtering), /tools interactive toggle, any policy-based tool restriction.

**Implementation approach:**
- Add `GetAllTools func() []ToolInfo` to `Context` with name, description, enabled status
- Add `SetActiveTools func(names []string)` to filter which tools the LLM can use
- Add `IsToolEnabled func(name string) bool` for individual checks
- Integrate with the existing tool wrapper pipeline in `wrapper.go`

---

### Gap 4: Session Lifecycle Events (Before-hooks with Cancel)
**Pi has:**
- `session_before_switch` -- Can cancel session switching
- `session_before_fork` -- Can cancel forking
- `session_switch` -- React to session changes

**Kit has:** `OnSessionStart` and `OnSessionShutdown` only. No before-hooks, no cancel capability, no fork/branch events.

**Impact:** Blocks dirty-repo-guard, confirm-destructive, git-checkpoint (restore on fork), any defensive workflow that needs to gate session operations.

**Implementation approach:**
- Add `OnSessionBeforeSwitch` event with `SessionBeforeSwitchResult{Cancel bool, Reason string}`
- Add `OnSessionBeforeFork` event with similar cancel capability
- Add `OnSessionSwitch` event for post-switch notifications
- Emit these from session management code before performing operations

---

### Gap 5: Compaction Events
**Pi has:** `session_before_compact` allowing custom compaction strategies (e.g., summarize entire conversation with a cheaper model instead of truncating).

**Kit has:** Nothing.

**Impact:** Blocks custom-compaction and trigger-compact patterns. Users cannot customize how context compaction works.

**Implementation approach:**
- Add `OnBeforeCompact` event with `BeforeCompactEvent{EstimatedTokens, ContextLimit int}`
- Result type: `BeforeCompactResult{Summary *string, FirstKeptEntryID *string}`
- If extension returns a summary, use it instead of default compaction
- Add `TriggerCompact func()` to `Context` for manual compaction triggers

---

### Gap 6: Custom Provider Registration
**Pi has:** `pi.registerProvider()` allowing extensions to register complete LLM providers with streaming, OAuth, and model definitions.

**Kit has:** No extension-facing provider registration. Providers are compiled-in via the `llm.Provider` interface.

**Impact:** Blocks custom-provider-anthropic, custom-provider-gitlab-duo, custom-provider-qwen-cli patterns. Users cannot add new LLM backends via extensions.

**Implementation approach:**
- Add `RegisterProvider(ProviderDef)` to `API` with:
  - `Name string`, `Models []ModelDef`
  - `Stream func(model, messages, options) StreamResult`
- This is a large undertaking. The `ProviderDef` would need to bridge to the compiled `llm.Provider` interface.
- **Yaegi limitation:** Complex streaming interfaces may hit Yaegi's interface generation bugs. May need concrete struct wrappers.
- **Priority:** Lower -- this is architecturally complex and has narrow use cases.

---

### Gap 7: CLI Flag Registration
**Pi has:** `pi.registerFlag("preset", {description, type})` and `pi.getFlag("preset")` allowing extensions to add CLI flags.

**Kit has:** Nothing. Extensions cannot influence CLI argument parsing.

**Impact:** Blocks preset system (--preset flag), plan-mode (--plan flag), sandbox (--no-sandbox flag).

**Implementation approach:**
- Add `RegisterFlag(FlagDef)` to `API` with `Name, Description, Type string, Default any`
- Add `GetFlag func(name string) any` to `Context`
- Parse extension flags after loading extensions but before `Init()`
- Store in a `map[string]any` on the Runner

---

### Gap 8: Keyboard Shortcut Registration
**Pi has:** `pi.registerShortcut(Key.ctrlShift("u"), {description, handler})` for global keyboard shortcuts.

**Kit has:** Nothing. Only editor interceptors can handle keys, and only when the editor has focus.

**Impact:** Blocks global shortcuts like Ctrl+Alt+P for plan mode toggle, Ctrl+Shift+U for preset switching.

**Implementation approach:**
- Add `RegisterShortcut(ShortcutDef)` to `API` with `Key string, Description string, Handler func(Context)`
- Bridge to BubbleTea's key handling in `model.go` Update() method
- Query registered shortcuts from Runner in the key dispatch path

---

### Gap 9: Custom Message Rendering
**Pi has:** `pi.registerMessageRenderer(customType, renderFn)` for custom visual rendering of specific message types (not just tool results).

**Kit has:** `RegisterToolRenderer` for tool-specific rendering only. No general message renderer.

**Impact:** Blocks status-update messages, extension-branded messages, and any custom message type that needs bespoke visual treatment.

**Implementation approach:**
- Add `RegisterMessageRenderer(MessageRenderConfig)` to `API` with:
  - `CustomType string` -- message type to match
  - `Render func(content, details string, expanded bool, width int) string`
- Integrate with the stream component's message rendering pipeline

---

### Gap 10: Programmatic Editor Control
**Pi has:**
- `ctx.ui.setEditorText(text)` -- Pre-fill the input editor
- `ctx.ui.setEditorComponent(factory)` -- Replace the entire editor

**Kit has:** `SetEditor(EditorConfig)` which is an interceptor (HandleKey/Render) but does NOT allow setting editor text or full replacement.

**Impact:** Blocks QnA (pre-fill editor with extracted questions), handoff (pre-fill with handoff prompt), and full editor replacement patterns.

**Implementation approach:**
- Add `SetEditorText func(text string)` to `Context` -- inserts text into the active editor
- Optionally add `SetEditorComponent func(EditorComponentConfig)` for full replacement (complex due to BubbleTea integration)

---

### Gap 11: Turn-Level Events
**Pi has:**
- `turn_start` -- Fires when a new LLM turn begins
- `turn_end` -- Fires when a turn completes

**Kit has:** `OnAgentStart`/`OnAgentEnd` which fire at the agent loop level (may span multiple turns), and `OnMessageStart`/`OnMessageEnd` for streaming. No dedicated turn boundary events.

**Impact:** Blocks git-checkpoint (create stash per turn), plan-mode (track done markers per turn), preset (persist state per turn), progress tracking.

**Implementation approach:**
- Add `OnTurnStart(func(TurnStartEvent, Context))` and `OnTurnEnd(func(TurnEndEvent, Context))`
- `TurnStartEvent{TurnNumber int, Prompt string}`
- `TurnEndEvent{TurnNumber int, Response string, StopReason string}`
- Emit from the agent loop between turns

---

### Gap 12: Context Filtering Event
**Pi has:** `pi.on("context", ...)` -- Lets extensions filter/modify messages before sending to the LLM. Returns `{messages: [...]}` to replace the context window.

**Kit has:** Nothing. Extensions cannot influence what messages the LLM sees.

**Impact:** Blocks plan-mode (filter stale messages), any extension needing to manage context window content, RAG-style context injection.

**Implementation approach:**
- Add `OnContextPrepare(func(ContextPrepareEvent, Context) *ContextPrepareResult)`
- `ContextPrepareEvent{Messages []MessageEntry}`
- `ContextPrepareResult{Messages []MessageEntry}` -- return filtered/modified set
- Emit just before sending messages to the LLM provider

---

### Gap 13: Inter-Extension Event Bus
**Pi has:** `pi.events.on(name, handler)` / `pi.events.emit(name, data)` for decoupled inter-extension communication.

**Kit has:** Nothing. Extensions are isolated; they cannot communicate with each other.

**Impact:** Blocks coordinated multi-extension workflows (e.g., theme extension reacting to mode changes from another extension).

**Implementation approach:**
- Add `OnCustomEvent func(name string, handler func(data string))` to `API`
- Add `EmitCustomEvent func(name, data string)` to `Context`
- Store handlers in Runner's event map, dispatch via `Emit`

---

### Gap 14: Session Persistence for Extensions
**Pi has:** `pi.appendEntry(customType, data)` -- Persists extension-specific data in the session journal. Survives across session resume.

**Kit has:** Nothing. Extension state is ephemeral (package-level vars lost on restart).

**Impact:** Blocks preset state restoration, plan-mode progress persistence, todo list persistence across sessions, any extension needing durable state.

**Implementation approach:**
- Add `AppendEntry func(entryType string, data string)` to `Context`
- Add `GetEntries func(entryType string) []string` to `Context` for retrieval
- Store in session file as custom entry types
- Emit entries during `OnSessionStart` for restoration

---

### Gap 15: Resource Discovery System
**Pi has:** `resources_discover` event where extensions can dynamically register skills, prompts, and themes by returning file paths.

**Kit has:** Nothing. No concept of dynamic resource loading.

**Impact:** Blocks dynamic-resources pattern. Extensions cannot contribute prompts, skills, or themes at runtime.

**Implementation approach:**
- Add `OnResourceDiscovery(func(ResourceDiscoveryEvent, Context) *ResourceDiscoveryResult)`
- `ResourceDiscoveryResult{SkillPaths, PromptPaths, ThemePaths []string}`
- Integrate with any future resource/skill loading system

---

### Gap 16: Programmatic Shutdown and Reload
**Pi has:**
- `ctx.shutdown()` -- Programmatically quit the application
- `ctx.reload()` -- Hot-reload all extensions at runtime

**Kit has:** Neither capability.

**Impact:** Blocks shutdown-command, reload-runtime patterns. Extensions cannot control app lifecycle.

**Implementation approach:**
- Add `Shutdown func()` to `Context` -- triggers graceful shutdown
- Add `Reload func() error` to `Context` -- reloads all extensions
- Wire via BubbleTea Quit msg and loader re-initialization

---

### Gap 17: Direct LLM Completion from Extensions
**Pi has:** Extensions can call `complete()` from `@mariozechner/pi-ai` to make LLM calls outside the main agent loop (e.g., summarization, question extraction, handoff generation).

**Kit has:** No way for extensions to invoke LLM completions directly. Extensions can only spawn Kit subprocesses.

**Impact:** Blocks in-process LLM calls for summarization, QnA extraction, context transfer. The subprocess pattern works but is heavier.

**Implementation approach:**
- Add `Complete func(CompleteRequest) (string, error)` to `Context`
- `CompleteRequest{Model, SystemPrompt string, Messages []SimpleMessage}`
- Wire through to existing `llm.Provider.Complete()` method
- Consider rate limiting and cost awareness

---

## Gap Analysis: Moderate Gaps (Partial Coverage)

### Gap M1: Tool Registration Depth
**Pi has:** `renderCall(args, theme)`, `renderResult(result, {expanded, isPartial}, theme)` directly on tool definition. Also `onUpdate` streaming callback, `AbortSignal`, and TypeBox schemas.

**Kit has:** Separate `RegisterToolRenderer()` and simpler `RegisterTool()` with JSON schema string and basic execute handler.

**Implementation approach:** Enhance `ToolDef` with optional `RenderHeader`/`RenderBody` fields. Add `onUpdate func(string)` to execute handler for streaming tool progress. Add abort/cancel context.

---

### Gap M2: Command Tab Completion
**Pi has:** `getArgumentCompletions(prefix)` on command registration for tab-completing command arguments.

**Kit has:** `RegisterCommand()` without completion support.

**Implementation approach:** Add optional `Complete func(prefix string) []string` to `CommandDef`.

---

### Gap M3: Keyed Status Bar Entries
**Pi has:** `ctx.ui.setStatus(key, text)` for multiple independent status bar indicators.

**Kit has:** `SetFooter(HeaderFooterConfig)` as a single custom footer, not keyed status entries.

**Implementation approach:** Add `SetStatus func(key, text string)` / `RemoveStatus func(key string)` to `Context`. Render all keyed entries in the status bar region.

---

### Gap M4: Full Custom TUI Components
**Pi has:** `ctx.ui.custom<T>(factory)` where factory receives `(tui, theme, keybindings, done)` and returns a `Focusable` component. Supports overlays and full TUI takeover (including `tui.stop()`/`tui.start()` for subprocess terminal sharing).

**Kit has:** `ShowOverlay(OverlayConfig)` with text content and action buttons. No way to render completely custom interactive components or suspend the TUI.

**Implementation approach:**
- This is architecturally complex with Yaegi. A simpler approach: add `SuspendTUI func(callback func())` to `Context` that stops BubbleTea, runs the callback (allowing raw terminal use), then restarts.
- For custom overlays: enhance `OverlayConfig` with a `RenderFunc` option for custom content rendering.

---

### Gap M5: SendMessage Delivery Modes
**Pi has:** Three modes:
- `pi.sendUserMessage(text)` -- Normal (triggers turn)
- `pi.sendUserMessage(text, {deliverAs: "steer"})` -- Interrupts current stream
- `pi.sendUserMessage(text, {deliverAs: "followUp"})` -- Queues after current stream

**Kit has:** `ctx.SendMessage(string)` which queues if agent is busy (similar to followUp), but no steering/interrupt mode and no structured content.

**Implementation approach:**
- Add `SendMessageOpts{DeliverAs string}` parameter to `SendMessage`
- Support `"steer"` (cancel current + send) and `"followUp"` (queue) modes
- Add `SendStructuredMessage func(content []ContentBlock, opts SendMessageOpts)` for multi-part messages

---

### Gap M6: Model Change Event
**Pi has:** `model_select` event with `event.model`, `event.previousModel`, `event.source`.

**Kit has:** No model change notification.

**Implementation approach:** Add `OnModelChange(func(ModelChangeEvent, Context))` with `NewModel, PreviousModel, Source string`.

---

### Gap M7: User Bash Hook
**Pi has:** `user_bash` event for intercepting user-initiated `!command` invocations, separate from tool-initiated bash. Can return custom `result` to override execution.

**Kit has:** No distinction between user-initiated and tool-initiated bash. `OnToolCall` catches both.

**Implementation approach:** Add `OnUserBash(func(UserBashEvent, Context) *UserBashResult)` or tag `ToolCallEvent` with a `Source` field (`"user"` vs `"tool"`).

---

## Capabilities with Parity (Covered)

| Capability | Kit | Pi | Status |
|---|---|---|---|
| Session start/shutdown events | `OnSessionStart`, `OnSessionShutdown` | `session_start`, `session_shutdown` | Parity |
| Before agent start (system prompt injection) | `OnBeforeAgentStart` returns `InjectText`, `SystemPrompt` | `before_agent_start` returns `systemPrompt`, `message` | Parity |
| Agent lifecycle events | `OnAgentStart`, `OnAgentEnd` | `agent_start`, `agent_end` | Parity |
| Message streaming events | `OnMessageStart`, `OnMessageUpdate`, `OnMessageEnd` | N/A (Pi uses `turn_start`/`turn_end` instead) | Kit advantage |
| Tool call interception (blocking) | `OnToolCall` returns `Block`, `Reason` | `tool_call` returns `block`, `reason` | Parity |
| Tool result modification | `OnToolResult` returns modified `Content`, `IsError` | `tool_result` returns modified content | Parity |
| Tool execution timing | `OnToolExecutionStart`, `OnToolExecutionEnd` | N/A | Kit advantage |
| Input interception/transform | `OnInput` returns `Action` (continue/transform/handled) | `input` returns `action` (continue/transform/handled) | Parity |
| Custom tool registration | `RegisterTool(ToolDef)` | `pi.registerTool({...})` | Parity (Pi richer) |
| Custom command registration | `RegisterCommand(CommandDef)` | `pi.registerCommand(name, {...})` | Parity |
| Widget system | `SetWidget`/`RemoveWidget` with placement, priority | `setWidget(key, lines)` | Parity |
| Header/Footer | `SetHeader`/`SetFooter` with content/style | `setHeader`/`setFooter` with factory | Parity (different models) |
| Overlay dialogs | `ShowOverlay` with actions, scrolling, markdown | `ctx.ui.custom({overlay: true})` | Pi richer |
| Interactive prompts | `PromptSelect`, `PromptConfirm`, `PromptInput` | `ctx.ui.select`, `ctx.ui.confirm`, `ctx.ui.input` | Parity |
| Editor interceptor | `SetEditor(EditorConfig)` with HandleKey/Render | `setEditorComponent()` for full replacement | Pi richer |
| Tool renderer customization | `RegisterToolRenderer(ToolRenderConfig)` | `renderCall`/`renderResult` on tool def | Parity |
| UI visibility control | `SetUIVisibility(UIVisibility)` | N/A (Pi uses direct component replacement) | Kit advantage |
| Context stats | `GetContextStats()` returns tokens, limit, usage% | Token data via `sessionManager.getBranch()` | Kit advantage (dedicated API) |
| Print functions | `Print`, `PrintInfo`, `PrintError`, `PrintBlock` | `ctx.ui.notify(msg, level)` | Different models, both adequate |
| Subprocess spawning | `os/exec` via Yaegi stdlib access | `pi.exec()` abstracted API | Parity (different approach) |

---

## Priority-Ordered Implementation Roadmap

### Phase 1: High-Impact, Lower Complexity
These gaps block the most important extension patterns and are relatively straightforward to implement.

1. **Session Management API** (Gap 1) -- Enables git integration, state restoration, bookmarks
2. **Turn-Level Events** (Gap 11) -- Enables per-turn checkpoints and progress tracking
3. **Session Persistence** (Gap 14) -- Enables durable extension state across restarts
4. **Programmatic Editor Control** (Gap 10) -- Enables QnA and handoff patterns
5. **Keyed Status Bar** (Gap M3) -- Enables richer status display

### Phase 2: Medium Impact, Medium Complexity
6. **Tool Management API** (Gap 3) -- Enables plan-mode and tool filtering
7. **Model Management API** (Gap 2) -- Enables presets and model switching
8. **CLI Flag Registration** (Gap 7) -- Enables --preset, --plan flags
9. **Inter-Extension Event Bus** (Gap 13) -- Enables cross-extension coordination
10. **SendMessage Delivery Modes** (Gap M5) -- Enables steering and follow-up patterns

### Phase 3: High Impact, High Complexity
11. **Session Lifecycle Before-Hooks** (Gap 4) -- Enables safety guards with cancel
12. **Context Filtering Event** (Gap 12) -- Enables context management
13. **Compaction Events** (Gap 5) -- Enables custom compaction strategies
14. **Direct LLM Completion** (Gap 17) -- Enables in-process sub-agent calls
15. **Full Custom TUI Components** (Gap M4) -- Enables interactive-shell, games

### Phase 4: Specialized / Lower Priority
16. **Keyboard Shortcut Registration** (Gap 8) -- Nice-to-have for power users
17. **Custom Message Rendering** (Gap 9) -- Nice-to-have for branded messages
18. **Custom Provider Registration** (Gap 6) -- Architecturally complex, narrow use cases
19. **Resource Discovery** (Gap 15) -- Depends on future skill/resource system
20. **Programmatic Shutdown/Reload** (Gap 16) -- Nice-to-have lifecycle control
21. **Model Change Event** (Gap M6) -- Nice-to-have notification
22. **User Bash Hook** (Gap M7) -- Nice-to-have distinction
23. **Command Tab Completion** (Gap M2) -- Nice-to-have UX improvement
24. **Tool Registration Depth** (Gap M1) -- Incremental improvement

---

## Extension Ecosystem Gap: Example Extensions We Should Build

Beyond API gaps, Pi simply has more example extensions demonstrating real-world patterns. Extensions we should create (once APIs exist):

| Extension | Pi Equivalent | Required API Additions |
|---|---|---|
| Permission gate (dangerous command confirmation) | `permission-gate.ts` | None (works today with OnToolCall) |
| Protected paths (block writes to .env, .git/) | `protected-paths.ts` | None (works today with OnToolCall) |
| Auto-commit on exit | `auto-commit-on-exit.ts` | Gap 1 (session messages) |
| Git checkpoints per turn | `git-checkpoint.ts` | Gaps 1, 4, 11 |
| Desktop notifications | `notify.ts` | None (works today with OnAgentEnd + os/exec) |
| Inline bash expansion (!{cmd}) | `inline-bash.ts` | None (works today with OnInput transform) |
| Plan mode (read-only exploration) | `plan-mode/` | Gaps 3, 7, 11, 12, 14 |
| Preset system | `preset.ts` | Gaps 2, 3, 7, 8, 14 |
| Dirty repo guard | `dirty-repo-guard.ts` | Gap 4 |
| QnA extraction | `qna.ts` | Gaps 1, 10, 17 |
| Handoff to new session | `handoff.ts` | Gaps 1, 10, 17, 22 (newSession) |
| Custom compaction | `custom-compaction.ts` | Gaps 2, 5 |
| Interactive shell (vim/htop) | `interactive-shell.ts` | Gap M4 (TUI suspend) |
| Event bus | `event-bus.ts` | Gap 13 |

### Extensions Buildable Today (No API Changes Needed)
These can be built right now with Kit's existing extension API:

1. **Permission gate** -- Use `OnToolCall` to intercept bash with `rm -rf`, return `Block: true`
2. **Protected paths** -- Use `OnToolCall` to check write/edit tool paths against deny-list
3. **Desktop notifications** -- Use `OnAgentEnd` + `os/exec` for OSC 777 or `notify-send`
4. **Inline bash expansion** -- Use `OnInput` with `Action: "transform"` to expand `!{cmd}`
5. **Pirate mode** -- Use `OnBeforeAgentStart` to append to system prompt
6. **Project rules loader** -- Use `OnSessionStart` to scan, `OnBeforeAgentStart` to inject
7. **Titlebar spinner** -- Use `OnAgentStart`/`OnAgentEnd` + `os.Stdout` for OSC sequences
8. **File trigger** -- Use `OnSessionStart` to set up `fsnotify` watcher, `SendMessage` to inject

---

## Summary Statistics

| Metric | Pi | Kit | Gap |
|---|---|---|---|
| Example extensions | 57+ | 10 | -47 |
| Lifecycle events | 16+ | 13 | -3+ |
| API methods on context | 35+ | 22 | -13+ |
| Custom providers | 3 | 0 | -3 |
| Session management APIs | 6 | 0 | -6 |
| Model management APIs | 4 | 1 (read-only) | -3 |
| Tool management APIs | 2 | 0 | -2 |
| Critical API gaps | -- | -- | 17 |
| Moderate API gaps | -- | -- | 7 |
| Extensions buildable today | -- | -- | 8 |
