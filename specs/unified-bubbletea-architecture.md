# Unified Bubble Tea Architecture

## Overview

Replace the micro-program pattern (3 interactive `tea.NewProgram` calls + 1 standalone progress) with a single persistent Bubble Tea program using child model composition. Extract a thick app layer from `cmd/root.go` to own agent orchestration, message storage, and event emission. TUI becomes purely reactive.

New capabilities: message queueing during streaming, double-tap ESC cancellation, stacked layout (output above, input pinned below), queue badge with clear support.

## User Story

As an MCPHost user, I want the TUI to remain responsive during agent streaming so I can queue follow-up messages, cancel in-progress work, and see a persistent input area -- instead of waiting for each response to complete before typing.

As a developer, I want the TUI architecture to follow Bubble Tea's idiomatic child-model pattern so components are composable, testable, and extensible without terminal ownership conflicts.

## Requirements

### Architecture
- Single `tea.NewProgram()` call for the entire interactive session
- Parent model manages state transitions and routes messages to child components
- Child components: `InputComponent` (slash commands + autocomplete), `StreamComponent` (streaming display + spinner), `ApprovalComponent` (tool approval)
- Ollama `ProgressModel` remains standalone (different lifecycle, runs during provider init)
- Non-interactive mode bypasses `tea.Program` entirely, uses same app layer without TUI

### App Layer
- New `internal/app` package owns: agent orchestration loop, in-memory message store, message queue, tool approval callback, hook execution, session persistence, usage tracking
- App layer exposes `Run(prompt)`, `RunOnce(ctx, prompt)`, `CancelCurrentStep()`, `ClearQueue()`, `QueueLength()`, `ClearMessages()`
- Events sent to TUI via `program.Send()` -- no pubsub infra
- Message store: mutable `[]fantasy.Message` with wrapper IDs, emits events on change. Bridges to `session.Manager` for persistence on each step completion.
- `ToolApprovalFunc` provided at construction via `Options`. Interactive mode: channel handshake with TUI. Non-interactive: auto-approve. Channel must be `select`-able against app context to avoid goroutine leaks on shutdown.
- All 7 agent callbacks from `GenerateWithLoopAndStreaming` (`agent.go:144-151`) mapped to events sent via `program.Send()`. See Events section.
- Hook executor (`hooks.Executor`) owned by app layer. Fires `UserPromptSubmit`, `PreToolUse`, `PostToolUse`, `Stop` at same points as current `runAgenticStep`.

### App Layer Options

Full options mirroring current `AgenticLoopConfig` (`root.go:753-769`):

```go
type Options struct {
    Agent            *agent.Agent
    ToolApprovalFunc ToolApprovalFunc   // required, set at construction
    HookExecutor     *hooks.Executor    // optional
    SessionManager   *session.Manager   // optional, for persistence
    MCPConfig        *config.Config     // for session continuation
    ModelName        string
    ServerNames      []string           // for slash commands
    ToolNames        []string           // for slash commands
    StreamingEnabled bool
    Quiet            bool
    Debug            bool
    CompactMode      bool
}
```

### Events

Events emitted by app layer (defined in `internal/app/events.go`):

| Event | Source callback | Purpose |
|---|---|---|
| `StreamChunkEvent` | `onStreamingResponse` | Streaming text delta |
| `ToolCallStartedEvent` | `onToolCall` | Tool call initiated (name + args) |
| `ToolExecutionEvent` | `onToolExecution` | Tool execution starting/stopping |
| `ToolResultEvent` | `onToolResult` | Tool result (name, args, result, isError) |
| `ToolCallContentEvent` | `onToolCallContent` | Tool call content display |
| `ResponseCompleteEvent` | `onResponse` | Final response text |
| `StepCompleteEvent` | (after generate returns) | Agent step finished, includes usage data |
| `StepErrorEvent` | (on agent error) | Agent step failed with error |
| `QueueUpdatedEvent` | (on queue change) | Queue length changed |
| `ToolApprovalNeededEvent` | `onToolApproval` | Approval required, includes response channel |
| `SpinnerEvent` | (before first chunk) | Show/hide spinner state |
| `HookBlockedEvent` | (hook returns block) | Hook blocked the action |
| `MessageCreatedEvent` | (on history add) | New message added to store |

TUI-internal messages (defined in `internal/ui/events.go`, NOT in app layer):

| Message | Purpose |
|---|---|
| `submitMsg` | Input component submitted text |
| `approvalResultMsg` | Approval component returned decision |
| `cancelTimerExpiredMsg` | 2s ESC timer expired |

### TUI Behavior
- Stacked layout: latest response output above, input textarea pinned below
- Output area shows latest response only. Completed responses emitted above the BT-managed region via `tea.Println()` before the model resets for the next interaction. This works with BT v2 inline mode (no alt screen).
- Input textarea keeps current sizing behavior from `SlashCommandInput`
- Slash command autocomplete fully self-contained in input component. Component holds `*app.App` reference for executing commands that affect app state (`/clear` calls `app.ClearMessages()`, `/quit` returns `tea.Quit` to parent, `/clear-queue` calls `app.ClearQueue()`). Parent receives either a `submitMsg` (text prompt) or a `tea.Cmd` (slash command side effect).
- Message queueing: user can submit while agent streams. Queue badge shows "N queued" near input. `/clear-queue` slash command flushes queue.
- Double-tap ESC: first press shows "press again to cancel", second press calls `App.CancelCurrentStep()`. Timer expires after 2s, resets state.
- Tool approval: agent blocks on `ToolApprovalFunc` callback. Callback sends `ToolApprovalNeededEvent` (containing a `chan<- bool` response channel) to program, then blocks on that channel via `select` with `ctx.Done()`. TUI transitions to approval state, user decides, parent sends result on channel. If ctx cancelled, callback returns `false, ctx.Err()`.
- Keyboard during streaming: input textarea remains focused and editable. All keystrokes go to the input component normally. ESC is intercepted by parent for cancel flow. Enter/submit queues the message via `app.Run()`.
- Spinner: `StreamComponent` renders a spinner animation (replacing the current standalone goroutine-based `ui.Spinner`) when the agent is processing but hasn't sent any chunks yet. First `StreamChunkEvent` transitions from spinner to streaming display. No more goroutine writing to stderr.

### Compact Mode

Current code uses two renderers (`MessageRenderer` and `CompactRenderer`) toggled by `cli.compactMode`. Both renderers retained. The `CompactMode` flag propagated through `App.Options` → parent model → child components. Each component checks the flag and delegates to the appropriate renderer for message formatting.

### Usage Tracking

`UsageTracker` moves to the app layer. Created during `App.New()` using model info from `Options`. App layer calls `UpdateUsageFromResponse()` after each step. Emits usage data in `StepCompleteEvent`. TUI renders usage via retained `UsageTracker.RenderUsageInfo()` method. Non-interactive mode reads usage from app layer directly.

### Non-Interactive Mode
- Same app layer, no TUI. `ToolApprovalFunc` auto-approves (provided at construction). Output prints directly to stdout.
- Current `runNonInteractiveMode` refactored to use `app.RunOnce()`.
- **Behavior change**: current non-interactive non-quiet mode creates a BT streaming display program. New behavior: `RunOnce()` accepts an optional `StreamingWriter io.Writer` for real-time output. Non-interactive passes `os.Stdout`. No BT program created.

### Session Persistence

`session.Manager` owned by app layer (passed via `Options.SessionManager`). App layer calls `session.Manager.AddMessages()` after each step completion and on queue drain. `--load-session` flag handled in `cmd/root.go` before app construction -- loaded messages passed to `App.New()` as initial history. `MessageStore.Clear()` also calls `session.Manager.ReplaceAllMessages()`.

### Error Handling

Agent errors (API failures, rate limits, MCP crashes) emitted as `StepErrorEvent`. Parent model receives the event, passes error to `StreamComponent` for inline display (matching current behavior), then transitions to `stateInput`. No automatic retry -- user can retry by submitting again.

### Graceful Shutdown

Shutdown sequence when user quits (Ctrl+C or `/quit`):

1. Parent model returns `tea.Quit`
2. `tea.Program.Run()` returns in `cmd/root.go`
3. If agent goroutine running: `app.CancelCurrentStep()` called (deferred)
4. `app.Close()` called (deferred) -- cancels app context, waits for agent goroutine to exit
5. `mcpAgent.Close()` called (deferred, existing) -- closes MCP connections and provider

`App` holds a top-level `context.Context` (created with `context.WithCancel` in `New()`). All agent goroutines use this context. `App.Close()` cancels it and calls `sync.WaitGroup.Wait()` to ensure clean exit.

### Parent Model State Machine

```
stateInput ──submit──→ stateWorking ──StepComplete──→ stateInput
                            │                              ↑
                            ├──ToolApproval──→ stateApproval──approve/deny──┘
                            │                              │
                            ├──StepError────→ stateInput   │
                            │                              │
                            └──Cancel────────→ stateInput  │
                                                           │
                          (queue non-empty: auto-drain) ───┘
```

States:
- `stateInput` -- input focused, waiting for user
- `stateWorking` -- agent running (spinner → streaming → tool calls → streaming → ...)
- `stateApproval` -- tool approval dialog active (sub-state of working)

### Testing
- Unit tests for each child component (send messages, assert state transitions)
- Unit tests for parent model (state routing, child delegation, cancel flow, error handling)
- Unit tests for app layer (message store, queue, cancel, session save ordering, ToolApprovalFunc channel + ctx cancellation)

## Technical Implementation

### Package Structure

```
internal/
  app/
    app.go              # App struct, New(), Run(), RunOnce(), CancelCurrentStep(), Close()
    app_test.go         # App tests (queue, cancel, drain, session save)
    messages.go         # MessageStore (in-memory, wraps []fantasy.Message, bridges session.Manager)
    messages_test.go    # MessageStore tests
    events.go           # All event types sent to TUI via program.Send()
    options.go          # Options struct, ToolApprovalFunc type
  ui/
    model.go            # Parent tea.Model (AppModel), state machine, message routing
    model_test.go       # Parent model tests
    input.go            # InputComponent (refactored slash_command_input.go)
    input_test.go       # Input tests
    stream.go           # StreamComponent (refactored streaming_display.go + spinner)
    stream_test.go      # Stream tests
    approval.go         # ApprovalComponent (refactored tool_approval_input.go)
    approval_test.go    # Approval tests
    events.go           # TUI-internal message types (submitMsg, approvalResultMsg, cancelTimerExpiredMsg)
    cli.go              # Retained: SetupCLI factory (creates App + AppModel), non-TUI helpers
    messages.go         # Retained: message rendering (used by StreamComponent)
    styles.go           # Retained
    enhanced_styles.go  # Retained
    compact_renderer.go # Retained
    block_renderer.go   # Retained
    commands.go         # Retained + /clear-queue added
    fuzzy.go            # Retained
    usage_tracker.go    # Retained (used by app layer)
    debug_logger.go     # Retained (used by app layer)
  ui/progress/
    ollama.go           # Retained standalone (not part of refactor)
```

### Parent Model

```go
type appState int
const (
    stateInput    appState = iota  // Input focused, waiting for user
    stateWorking                    // Agent running, streaming output
    stateApproval                   // Tool approval dialog active
)

type AppModel struct {
    state        appState
    app          *app.App            // Thick app layer reference
    input        InputComponent      // Child: user input + autocomplete
    stream       StreamComponent     // Child: streaming display + spinner
    approval     ApprovalComponent   // Child: tool approval
    renderer     *MessageRenderer    // For tea.Println of completed responses
    compactRdr   *CompactRenderer    // Compact mode renderer
    compactMode  bool                // Which renderer to use
    queueCount   int                 // Cached from QueueUpdatedEvent
    canceling    bool                // Double-tap ESC state
    approvalChan chan<- bool         // Response channel for current approval
    width        int
    height       int
}
```

### Event Flow

```
User types → InputComponent.Update() → submit
  ↓
Parent receives submitMsg → calls app.Run(prompt) in tea.Cmd goroutine
  ↓
Parent transitions to stateWorking → StreamComponent active (spinner mode)
  ↓
App layer goroutine: agent processes
  → program.Send(SpinnerEvent{Show: true})
  → program.Send(ToolCallStartedEvent{...})
  → program.Send(StreamChunkEvent{...})  (first chunk hides spinner)
  → program.Send(ToolResultEvent{...})
  → program.Send(ToolCallContentEvent{...})
  ↓
Parent routes events to StreamComponent.Update()
  ↓
Agent needs tool approval → ToolApprovalFunc called
  → creates chan bool, sends ToolApprovalNeededEvent{ResponseChan: ch}
  → blocks: select { case result := <-ch; case <-ctx.Done() }
  ↓
Parent stores channel in approvalChan, transitions to stateApproval
  ↓
User approves → Parent sends on approvalChan → Agent continues
  ↓
Agent completes → app sends StepCompleteEvent{Usage: ...}
  ↓
Parent: tea.Println() completed response, transitions to stateInput
  ↓
If queue non-empty: App auto-drains next message, stays in stateWorking
```

### Cancel Flow

```
User presses ESC during stateWorking
  ↓
Parent sets canceling=true, returns cancelTimerCmd (2s tea.Tick)
  ↓
User presses ESC again within 2s → Parent calls app.CancelCurrentStep()
  ↓
App cancels step context → agent goroutine exits
  → ToolApprovalFunc unblocks via ctx.Done() if waiting
  → StepErrorEvent or StepCompleteEvent emitted
  ↓
Parent transitions to stateInput
  ↓
cancelTimerExpiredMsg arrives (if no second ESC) → resets canceling=false
```

### cmd/root.go Changes

```go
// runNormalMode becomes:
appInstance, err := app.New(app.Options{
    Agent:            mcpAgent,
    ToolApprovalFunc: toolApprovalFunc,  // set per mode, see below
    HookExecutor:     hookExecutor,
    SessionManager:   sessionManager,
    MCPConfig:        mcpConfig,
    ModelName:        modelString,
    ServerNames:      serverNames,
    ToolNames:        toolNames,
    StreamingEnabled: viper.GetBool("stream"),
    Quiet:            quietFlag,
    Debug:            viper.GetBool("debug"),
    CompactMode:      viper.GetBool("compact"),
}, initialMessages)  // loaded from session if --load-session
defer appInstance.Close()

// Interactive mode:
toolApprovalFunc = app.NewInteractiveApprovalFunc(appInstance)
model := ui.NewAppModel(appInstance, uiOpts)
program := tea.NewProgram(model)
appInstance.SetProgram(program)  // Safe: app.Run() not called until Init()
_, err := program.Run()

// Non-interactive mode:
toolApprovalFunc = app.AutoApproveFunc
result, err := appInstance.RunOnce(ctx, prompt, os.Stdout) // stdout for streaming
printResult(result)
```

**SetProgram timing**: Safe because `app.Run()` is only called from `tea.Cmd` functions after the program starts its event loop. `AppModel.Init()` returns no command that calls `app.Run()` -- the first `Run()` call happens when the user submits input or when `Init()` dispatches an initial prompt (non-interactive continuation via `--no-exit`).

## Tasks

### 1. Create app layer skeleton
- [ ] [P0] Create `internal/app/events.go` with all event types: `StreamChunkEvent`, `ToolCallStartedEvent`, `ToolExecutionEvent`, `ToolResultEvent`, `ToolCallContentEvent`, `ResponseCompleteEvent`, `StepCompleteEvent` (with usage data), `StepErrorEvent`, `QueueUpdatedEvent`, `ToolApprovalNeededEvent` (with `ResponseChan chan<- bool`), `SpinnerEvent`, `HookBlockedEvent`, `MessageCreatedEvent`
- [ ] [P0] Create `internal/app/options.go` with `Options` struct (all fields from App Layer Options section), `ToolApprovalFunc` type (`func(ctx context.Context, toolName, toolArgs string) (bool, error)`), `AutoApproveFunc` var, `NewInteractiveApprovalFunc` constructor
- [ ] [P0] Create `internal/app/messages.go` with `MessageStore` wrapping `[]fantasy.Message`. Methods: `Add(fantasy.Message)`, `Replace([]fantasy.Message)`, `GetAll() []fantasy.Message`, `Clear()`. Bridges to `session.Manager` (if non-nil) on every mutation for persistence.
- [ ] [P0] Create `internal/app/app.go` with `App` struct, `New(opts, initialMessages)`, `SetProgram(*tea.Program)`, `Run(prompt)`, `RunOnce(ctx, prompt, io.Writer)`, `CancelCurrentStep()`, `QueueLength()`, `ClearQueue()`, `ClearMessages()`, `Close()`. Internal: `context.WithCancel`, `sync.WaitGroup`, `sync.Mutex` for busy/queue state.

### 2. Migrate agent orchestration into app layer
- [ ] [P1] Move `runAgenticStep` logic from `cmd/root.go:873-1191` into `App.executeStep()`. Map all 7 agent callbacks to `program.Send()` events. Wire `ToolApprovalFunc` for `onToolApproval`. Emit `SpinnerEvent{Show:true}` before calling agent, `SpinnerEvent{Show:false}` on first stream chunk.
- [ ] [P1] Move hook execution from `cmd/root.go:810-828,943-969,1002-1019,1186-1223` into `App.executeStep()`. Fire `UserPromptSubmit` in `Run()` before `executeStep()`. Fire `PreToolUse`/`PostToolUse`/`Stop` at same points. Emit `HookBlockedEvent` if hook blocks.
- [ ] [P1] Move conversation history management into `MessageStore`. `App.executeStep()` calls `store.Add()` for user message before agent call, `store.Replace()` with updated history after agent returns. Store bridges to `session.Manager`.
- [ ] [P1] Move usage tracking into app layer. Create `UsageTracker` in `App.New()` from model info. Call `UpdateUsageFromResponse()` after each step. Include usage data in `StepCompleteEvent`.
- [ ] [P1] Implement queue drain: after step completes (success or error), if queue non-empty, dequeue next message and call `executeStep()` in same goroutine (no new goroutine spawn).

### 3. Create parent TUI model
- [ ] [P1] Create `internal/ui/model.go` with `AppModel` struct (see Parent Model section), `NewAppModel()`, `Init()`, `Update()`, `View()`. State machine routes events to children based on `appState`. Handle `tea.WindowSizeMsg` to distribute height. Store `approvalChan` for tool approval response.
- [ ] [P1] Implement double-tap ESC cancel in parent `Update()`: intercept `tea.KeyPressMsg` for ESC during `stateWorking`. Track `canceling` bool, return `tea.Tick(2*time.Second, ...)` as timer cmd, call `app.CancelCurrentStep()` on second press within window.
- [ ] [P1] Implement `tea.Println()` for completed responses: on `StepCompleteEvent`, render the completed response using message renderer (respecting compact mode), emit via `tea.Println()`, then reset `StreamComponent` state.
- [ ] [P1] Implement `StepErrorEvent` handling: render error inline in stream area, transition to `stateInput`.
- [ ] [P1] Implement graceful quit: Ctrl+C and `/quit` return `tea.Quit`. Deferred `app.Close()` in `cmd/root.go` handles cleanup.

### 4. Refactor child components
- [ ] [P1] Refactor `slash_command_input.go` → `internal/ui/input.go` as `InputComponent`. Remove `tea.Quit` on submit -- return `submitMsg` as a `tea.Cmd`. Keep autocomplete + popup self-contained. Hold `*app.App` reference for slash command execution: `/clear` → `app.ClearMessages()`, `/clear-queue` → `app.ClearQueue()`, `/quit` → return `tea.Quit` cmd. Remove `os.Exit(0)` from `/quit`.
- [ ] [P1] Refactor `streaming_display.go` → `internal/ui/stream.go` as `StreamComponent`. Add spinner state: render KITT-style animation (from current `spinner.go`) when `SpinnerEvent{Show:true}` received, switch to streaming text on first `StreamChunkEvent`. Accept all display events (`ToolCallStartedEvent`, `ToolResultEvent`, etc.) and render via retained `MessageRenderer`/`CompactRenderer`. Remove `streamDoneMsg`/`tea.Quit` -- parent manages lifecycle. Add `Reset()` to clear state between steps.
- [ ] [P1] Refactor `tool_approval_input.go` → `internal/ui/approval.go` as `ApprovalComponent`. Remove `tea.Quit` -- return `approvalResultMsg{approved: bool}` as a `tea.Cmd`. Parent handles sending result on `approvalChan`.

### 5. Wire TUI to app layer in cmd/root.go
- [ ] [P1] Refactor `runNormalMode()`: create `app.App` with full `Options` (all fields). Wire `ToolApprovalFunc` per mode. Load session messages before construction. Defer `appInstance.Close()`.
- [ ] [P1] Interactive path: create `ui.NewAppModel()` + single `tea.NewProgram(model)` + `appInstance.SetProgram(program)` + `program.Run()`. Remove `SetupCLI()` flow for interactive mode.
- [ ] [P1] Non-interactive path: call `appInstance.RunOnce(ctx, prompt, os.Stdout)`. Handle `--no-exit` by switching to interactive mode after. No `tea.Program` created. Remove old streaming display usage for non-interactive.
- [ ] [P1] Retain `SetupCLI()` as alternative factory for non-interactive quiet mode (just prints final text, no renderers needed). Or inline the quiet-mode logic.

### 6. Implement message queueing UX
- [ ] [P2] Add queue badge rendering in parent `View()` -- show "N queued" right-aligned on separator line when `queueCount > 0`. Update count on `QueueUpdatedEvent`.
- [ ] [P2] Register `/clear-queue` slash command in `internal/ui/commands.go`.
- [ ] [P2] Handle `submitMsg` during `stateWorking`: parent calls `app.Run()` (which queues internally), does NOT transition state. Input component stays active and clears text.

### 7. Stacked layout
- [ ] [P2] Implement stacked `View()` in parent: stream output region (variable height) + separator line + input region (current textarea height). Use `lipgloss.JoinVertical`. Separator shows queue badge if applicable.
- [ ] [P2] Handle `tea.WindowSizeMsg` propagation: calculate input height (fixed, from textarea), separator (1 line), remaining goes to stream. Propagate dimensions to children.

### 8. Cleanup
- [ ] [P2] Delete standalone `tea.NewProgram` calls from `cli.go` (`GetPrompt`, `StartStreamingMessage`, `GetToolApproval`). Remove `streamProgram`/`streamDone` fields.
- [ ] [P2] Delete `runAgenticStep`, `runAgenticLoop`, `runInteractiveLoop`, `addMessagesToHistory`, `replaceMessagesHistory`, `AgenticLoopConfig` from `cmd/root.go`.
- [ ] [P2] Delete old `spinner.go` (replaced by StreamComponent's inline spinner).
- [ ] [P3] Trim `CLI` struct to only non-TUI helpers needed by non-interactive quiet mode. Remove `GetPrompt`, `StartStreamingMessage`, `UpdateStreamingMessage`, `GetToolApproval`, `finishStreaming`, `HandleSlashCommand`. Retain `DisplayError`, `DisplayInfo` for non-interactive error output if needed, or remove entirely if `RunOnce` handles its own output.

### 9. Tests
- [ ] [P2] Unit tests for `MessageStore`: add, replace, getAll, clear, session.Manager bridge (mock manager, verify calls)
- [ ] [P2] Unit tests for `App`: run (single), run (queued), cancel during step, cancel during approval (verify ToolApprovalFunc unblocks via ctx), queue drain ordering, ClearQueue, Close (verify goroutine cleanup via WaitGroup)
- [ ] [P2] Unit tests for `AppModel`: state transitions (input→working→approval→input), StepError→input, ESC cancel flow (single tap resets, double tap cancels), queue badge update, window resize, tea.Println on step complete
- [ ] [P2] Unit tests for child components: `InputComponent` (submit emits submitMsg, slash commands execute, /quit returns tea.Quit), `StreamComponent` (spinner→streaming transition, chunk accumulation, tool call rendering, reset), `ApprovalComponent` (approve/deny emits approvalResultMsg)

## UI Mockup

### Processing (stateWorking, spinner)

```
  ◇◇◇◆◇◇◇ Thinking...


───────────────────────────────────
> █
```

### During Streaming (stateWorking)

```
  assistant (claude-sonnet-4-20250514)
  Here is the implementation of the requested
  feature. First, I'll create the new file...
  █ (streaming cursor)

─────────────────────────────────── 2 queued
> write tests for that too█
```

### Tool Call in Stream (stateWorking)

```
  assistant (claude-sonnet-4-20250514)
  Let me check the build first.

  ⚙ bash: go build -o output/mcphost
  ◇◇◇◆◇◇◇ Executing...

─────────────────────────────────── 2 queued
> write tests for that too█
```

### During Tool Approval (stateApproval)

```
  assistant (claude-sonnet-4-20250514)
  I need to run a command to check the build.

  ┌─ Tool Approval ──────────────────────┐
  │ bash: go build -o output/mcphost     │
  │                                      │
  │  [Yes]   No                          │
  └──────────────────────────────────────┘

─────────────────────────────────── 2 queued
> █
```

### Cancel in Progress (stateWorking, canceling)

```
  assistant (claude-sonnet-4-20250514)
  Analyzing the codebase structure to find
  relevant files...

  ⚠ Press ESC again to cancel

─────────────────────────────────── 1 queued
> also check the tests█
```

### Error (stateWorking → stateInput)

```
  ✗ Error: API rate limit exceeded. Try again.

───────────────────────────────────
> █
```

## Out of Scope

- Scrollable viewport / chat history browsing (latest response only for v1)
- Ollama `ProgressModel` unification (stays standalone)
- Persistent message storage / database (session JSON files retained as-is)
- Multi-session support
- Split-pane or tabbed layouts
- Mouse interaction
- Changing tool call display format (keep current rendering via retained renderers)
- Prompt history persistence across sessions
- Any visual/theme changes beyond new layout
- Refactoring the `agent.Agent` or `fantasy` interfaces
- Changing hook execution semantics

## Open Questions

- Should queue drain be immediate (next message starts as soon as current step completes) or should there be a brief pause to let the user read the response?
- If the user cancels mid-stream and there are queued messages, should the queue also be flushed or should the next queued message execute?
- Should the input component retain focus (cursor visible, editable) during `stateApproval`, or should focus fully transfer to the approval dialog?
- Should `tea.Println()` of completed responses include tool call/result details, or just the final assistant text? Current behavior shows everything inline.
- How should debug logging work during the TUI lifecycle? Currently `BufferedDebugLogger` accumulates messages shown after agent creation. In the new architecture, should debug messages be events rendered in the stream component?
- For `--no-exit` (non-interactive then interactive): should `RunOnce` return and then `cmd/root.go` creates the TUI program for the interactive continuation, or should the TUI program be created upfront and the initial prompt dispatched via `Init()`?
