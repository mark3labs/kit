# Plan 10: App-as-SDK-Consumer — Complete Integration

**Priority**: P4
**Effort**: High
**Goal**: Make the CLI app a full consumer of the SDK. `cmd/root.go` creates a `*Kit` via `kit.New()`. The app receives `*Kit`, calls `kit.PromptResult()`, subscribes to SDK events for TUI rendering, and extension observation events route through the SDK EventBus. This closes all deferred work from Plans 03, 05, and 09.

## Background

Plans 00–09 built the SDK surface (`pkg/kit/`) but the CLI app still bypasses it for the critical path:

- `cmd/root.go` calls `SetupAgent()` directly instead of `kit.New()`
- `internal/app/app.go:executeStep()` calls `agent.GenerateWithLoopAndStreaming()` directly with 150+ lines of manual callback wiring, extension event dispatch, and session persistence — all of which the SDK already handles in `runTurn()`
- Extension observation events (AgentStart, AgentEnd, MessageStart, MessageUpdate, MessageEnd) are emitted from `executeStep()`, not from the SDK
- The app receives an `AgentRunner` interface, not a `*Kit`

After this plan, `executeStep()` becomes a thin wrapper around `kit.PromptResult()`, and extension events flow through the SDK's EventBus.

### Deferred Items Resolved

| Source | What | How |
|--------|------|-----|
| Plan 03 Step 6 | App TUI subscribes to SDK events | Step 5 |
| Plan 03 Step 7 | Extension observation events forward to SDK EventBus | Step 4 |
| Plan 05 Step 6 | `executeStep()` delegates to SDK `Prompt()` | Step 6 |
| Plan 09 Phase 2 | App uses only SDK hooks, no direct runner calls | Step 6 |

## Prerequisites

- Plans 00–09 (all complete)

## Step-by-Step

### Step 1: Extend Kit for CLI consumption

**Files**: `pkg/kit/kit.go`, `pkg/kit/setup.go`

The CLI needs fields that the programmatic SDK doesn't: spinner for Ollama loading, buffered debug logger, pre-loaded MCP config. Add these to `Options` and expose results via getters.

**1a. Add CLI fields to `Options`** (`pkg/kit/kit.go:48-71`):

```go
type Options struct {
    // ... existing fields ...

    // CLI-specific fields (ignored by programmatic SDK users)
    MCPConfig         *config.Config    // Pre-loaded MCP config (skips LoadAndValidateConfig if set)
    ShowSpinner       bool              // Show loading spinner for Ollama models
    SpinnerFunc       agent.SpinnerFunc // Spinner implementation (nil = no spinner)
    UseBufferedLogger bool              // Buffer debug messages for later display
    Debug             bool              // Enable debug logging
}
```

**1b. Add fields and getters to `Kit` struct** (`pkg/kit/kit.go:22-36`):

```go
type Kit struct {
    // ... existing fields ...
    extRunner      *extensions.Runner
    bufferedLogger *tools.BufferedDebugLogger
}
```

Getters:

```go
// GetExtRunner returns the extension runner (nil if extensions are disabled).
func (m *Kit) GetExtRunner() *extensions.Runner { return m.extRunner }

// GetBufferedLogger returns the buffered debug logger (nil if not configured).
func (m *Kit) GetBufferedLogger() *tools.BufferedDebugLogger { return m.bufferedLogger }

// GetAgent returns the underlying agent. Callers that need the raw agent
// (e.g. for GetTools(), GetLoadingMessage()) can use this.
func (m *Kit) GetAgent() *agent.Agent { return m.agent }

// GetTreeSession returns the current tree session manager.
// (Already exists as a method — verify it's public.)
```

**1c. Update `New()`** (`pkg/kit/kit.go:111-204`):

- If `opts.MCPConfig != nil`, skip `config.LoadAndValidateConfig()` and use it directly
- If `opts.Debug`, set `viper.Set("debug", true)`
- Pass `ShowSpinner`, `SpinnerFunc`, `UseBufferedLogger` through to `SetupAgent()`
- Store `agentResult.ExtRunner` and `agentResult.BufferedLogger` on the Kit struct

```go
// In New(), replace lines 152-176:
mcpConfig := opts.MCPConfig
if mcpConfig == nil {
    var err error
    mcpConfig, err = config.LoadAndValidateConfig()
    if err != nil {
        return nil, fmt.Errorf("failed to load MCP config: %w", err)
    }
}

agentResult, err := SetupAgent(ctx, AgentSetupOptions{
    MCPConfig:         mcpConfig,
    Quiet:             opts.Quiet,
    ShowSpinner:       opts.ShowSpinner,
    SpinnerFunc:       opts.SpinnerFunc,
    UseBufferedLogger: opts.UseBufferedLogger,
    CoreTools:         opts.Tools,
    ExtraTools:        opts.ExtraTools,
    ToolWrapper:       hookToolWrapper(beforeToolCall, afterToolResult),
})

// Store on Kit struct:
k := &Kit{
    // ... existing fields ...
    extRunner:      agentResult.ExtRunner,
    bufferedLogger: agentResult.BufferedLogger,
}
```

**Verification**:
```bash
go build -o output/kit ./cmd/kit
go test -race ./...
golangci-lint run ./...
```

Existing behavior is unchanged — the new fields default to zero values.

---

### Step 2: Add TurnResult and PromptResult method

**File**: `pkg/kit/kit.go`

The current `Prompt()` returns `(string, error)`, which is fine for simple SDK usage but the app needs usage stats and conversation messages. Add a richer return path.

**2a. Define TurnResult** (new, in `pkg/kit/kit.go`):

```go
// TurnResult contains the full result of a prompt turn, including usage
// statistics and the updated conversation. Use PromptResult() instead of
// Prompt() when you need access to this data.
type TurnResult struct {
    // Response is the assistant's final text response.
    Response string

    // TotalUsage is the aggregate token usage across all steps in the turn
    // (includes tool-calling loop iterations). Nil if the provider didn't
    // report usage.
    TotalUsage *FantasyUsage

    // FinalUsage is the token usage from the last API call only. Use this
    // for context window fill estimation (InputTokens + OutputTokens ≈
    // current context size). Nil if unavailable.
    FinalUsage *FantasyUsage

    // Messages is the full updated conversation after the turn, including
    // any tool call/result messages added during the agent loop.
    Messages []FantasyMessage
}
```

**2b. Modify `runTurn()` to return `*TurnResult`** (`pkg/kit/kit.go:319`):

Change signature from:
```go
func (m *Kit) runTurn(ctx context.Context, promptLabel string, prompt string, preMessages []fantasy.Message) (string, error)
```
To:
```go
func (m *Kit) runTurn(ctx context.Context, promptLabel string, prompt string, preMessages []fantasy.Message) (*TurnResult, error)
```

Build and return `TurnResult` from the `agent.GenerateWithLoopResult`:

```go
responseText := result.FinalResponse.Content.Text()

turnResult := &TurnResult{
    Response: responseText,
    Messages: result.ConversationMessages,
}
if result.TotalUsage != nil {
    turnResult.TotalUsage = result.TotalUsage
}
if result.FinalResponse != nil {
    turnResult.FinalUsage = &result.FinalResponse.Usage
}

// ... existing event emission and persistence ...

return turnResult, nil
```

On the error path, return `nil, err` (as before, but with `*TurnResult` instead of `""`).

**2c. Update all prompt methods** to extract the string from `TurnResult`:

```go
func (m *Kit) Prompt(ctx context.Context, message string) (string, error) {
    result, err := m.runTurn(ctx, message, message, []fantasy.Message{
        fantasy.NewUserMessage(message),
    })
    if err != nil {
        return "", err
    }
    return result.Response, nil
}
```

Same pattern for `Steer()`, `FollowUp()`, `PromptWithOptions()`, `PromptWithCallbacks()`.

**2d. Add `PromptResult()` method**:

```go
// PromptResult sends a message and returns the full turn result including
// usage statistics and conversation messages. Use this instead of Prompt()
// when you need more than just the response text.
func (m *Kit) PromptResult(ctx context.Context, message string) (*TurnResult, error) {
    return m.runTurn(ctx, message, message, []fantasy.Message{
        fantasy.NewUserMessage(message),
    })
}
```

**Verification**:
```bash
go build -o output/kit ./cmd/kit
go test -race ./...
golangci-lint run ./...
```

Existing `Prompt()` callers (examples, tests) are unaffected.

---

### Step 3: Migrate cmd/root.go to use kit.New()

**Files**: `cmd/root.go`, `cmd/setup.go`

Replace the manual `SetupAgent()` → `InitTreeSession()` → `BuildAppOptions()` chain with a single `kit.New()` call.

**3a. Replace agent creation** in `runNormalMode()` (`cmd/root.go:336-362`):

Before:
```go
agentResult, err := SetupAgent(ctx, AgentSetupOptions{...})
mcpAgent := agentResult.Agent
defer func() { _ = mcpAgent.Close() }()
provider, modelName, serverNames, toolNames := CollectAgentMetadata(mcpAgent, mcpConfig)
```

After:
```go
// Build Kit options from CLI flags.
kitOpts := &kit.Options{
    MCPConfig:         mcpConfig,
    ShowSpinner:       true,
    SpinnerFunc:       spinnerFunc,
    UseBufferedLogger: true,
    Quiet:             quietFlag,
    Debug:             debugMode,
    NoSession:         noSessionFlag,
    Continue:          continueFlag,
    SessionPath:       sessionPath,
    AutoCompact:       autoCompactFlag,
}
if resumeFlag {
    sessions, _ := kit.ListSessions("")
    if len(sessions) > 0 {
        kitOpts.SessionPath = sessions[0].Path
    }
}

kitInstance, err := kit.New(ctx, kitOpts)
if err != nil {
    return err
}
defer kitInstance.Close()
```

**3b. Extract metadata from Kit instead of raw agent**:

```go
mcpAgent := kitInstance.GetAgent()
provider, modelName, serverNames, toolNames := CollectAgentMetadata(mcpAgent, mcpConfig)
```

**3c. Get buffered logger and tree session from Kit**:

```go
bufferedLogger := kitInstance.GetBufferedLogger()
// ... display buffered debug messages ...

treeSession := kitInstance.GetTreeSession()
var messages []fantasy.Message
if treeSession != nil {
    messages = treeSession.GetFantasyMessages()
}
```

**3d. Build app options using Kit**:

```go
appOpts := BuildAppOptions(mcpAgent, mcpConfig, modelName, serverNames, toolNames, kitInstance.GetExtRunner())
appOpts.TreeSession = treeSession
appOpts.Kit = kitInstance  // NEW — added in Step 5
```

**3e. Extension context setup** — use Kit's extension runner:

```go
extRunner := kitInstance.GetExtRunner()
if extRunner != nil {
    extRunner.SetContext(extensions.Context{...})
    // Emit SessionStart
}
```

**3f. Remove the separate `kit.InitTreeSession()` call** — Kit.New() handles session creation.

**3g. Remove the `defer func() { _ = mcpAgent.Close() }()`** — `kitInstance.Close()` handles cleanup.

**Verification**:
```bash
go build -o output/kit ./cmd/kit
go test -race ./...
golangci-lint run ./...
# Manual: run `kit -p "hello"` to verify non-interactive mode
# Manual: run `kit` to verify interactive mode
```

The app still uses its own `executeStep()` at this point — that migrates in Step 6.

---

### Step 4: Bridge extension observation events through SDK EventBus

**File**: `pkg/kit/extensions_bridge.go`

Currently `bridgeExtensions()` only bridges `Input` and `BeforeAgentStart` (hook events). The observation events (AgentStart, AgentEnd, MessageStart, MessageUpdate, MessageEnd) are emitted from `app.executeStep()` directly to the extension runner. After this step, the SDK emits them from `runTurn()`/`generate()` and the bridge forwards to extensions.

**4a. Subscribe to SDK events and forward to extension runner**:

Add to `bridgeExtensions()` (`pkg/kit/extensions_bridge.go:16`):

```go
func (m *Kit) bridgeExtensions(runner *extensions.Runner) {
    // ... existing Input and BeforeAgentStart hooks ...

    // Forward SDK observation events to extension runner.
    // These events are emitted by runTurn()/generate() and forwarded here
    // so extensions see them without the app having to emit them manually.

    if runner.HasHandlers(extensions.AgentStart) {
        m.Subscribe(func(e Event) {
            if ev, ok := e.(TurnStartEvent); ok {
                runner.Emit(extensions.AgentStartEvent{Prompt: ev.Prompt})
            }
        })
    }

    if runner.HasHandlers(extensions.MessageStart) {
        m.Subscribe(func(e Event) {
            if _, ok := e.(MessageStartEvent); ok {
                runner.Emit(extensions.MessageStartEvent{})
            }
        })
    }

    if runner.HasHandlers(extensions.MessageUpdate) {
        m.Subscribe(func(e Event) {
            if ev, ok := e.(MessageUpdateEvent); ok {
                runner.Emit(extensions.MessageUpdateEvent{Chunk: ev.Chunk})
            }
        })
    }

    if runner.HasHandlers(extensions.MessageEnd) {
        m.Subscribe(func(e Event) {
            if ev, ok := e.(MessageEndEvent); ok {
                runner.Emit(extensions.MessageEndEvent{Content: ev.Content})
            }
        })
    }

    if runner.HasHandlers(extensions.AgentEnd) {
        m.Subscribe(func(e Event) {
            if ev, ok := e.(TurnEndEvent); ok {
                stopReason := "completed"
                response := ev.Response
                if ev.Error != nil {
                    stopReason = "error"
                    response = ""
                }
                runner.Emit(extensions.AgentEndEvent{
                    Response:   response,
                    StopReason: stopReason,
                })
            }
        })
    }
}
```

**4b. Add SessionShutdown to Kit.Close()**:

In `pkg/kit/kit.go:Close()`:

```go
func (m *Kit) Close() error {
    // Emit SessionShutdown for extensions.
    if m.extRunner != nil && m.extRunner.HasHandlers(extensions.SessionShutdown) {
        m.extRunner.Emit(extensions.SessionShutdownEvent{})
    }
    if m.treeSession != nil {
        _ = m.treeSession.Close()
    }
    return m.agent.Close()
}
```

**Verification**:
```bash
go build -o output/kit ./cmd/kit
go test -race ./...
golangci-lint run ./...
```

At this point, extension observation events will fire from BOTH `executeStep()` (app) and the SDK bridge. This is intentional for the transition — Step 6 removes the app-side emission.

---

### Step 5: Wire app to Kit — add Kit field and SDK event → tea.Msg bridge

**Files**: `internal/app/options.go`, `internal/app/app.go`

Give the app a `*Kit` reference so it can call SDK prompt methods and subscribe to events.

**5a. Add Kit field to `app.Options`** (`internal/app/options.go:50`):

```go
import kit "github.com/mark3labs/kit/pkg/kit"

type Options struct {
    // Kit is the SDK instance. When set, executeStep() delegates to
    // kit.PromptResult() and events flow through SDK subscriptions.
    Kit *kit.Kit

    // Agent is the agent used to run the agentic loop. Required when Kit
    // is nil. When Kit is set, this field is ignored (Kit owns the agent).
    Agent AgentRunner

    // ... rest unchanged ...
}
```

**5b. Create SDK event → tea.Msg bridge function** (`internal/app/app.go`):

```go
// subscribeSDKEvents registers temporary SDK event subscribers that convert
// SDK events to tea.Msg events and dispatch them via sendFn. Returns an
// unsubscribe function that removes all listeners.
func (a *App) subscribeSDKEvents(sendFn func(tea.Msg)) func() {
    k := a.opts.Kit
    var unsubs []func()

    unsubs = append(unsubs, k.Subscribe(func(e kit.Event) {
        switch ev := e.(type) {
        case kit.ToolCallEvent:
            sendFn(ToolCallStartedEvent{ToolName: ev.ToolName, ToolArgs: ev.ToolArgs})
        case kit.ToolExecutionStartEvent:
            sendFn(ToolExecutionEvent{ToolName: ev.ToolName, IsStarting: true})
        case kit.ToolExecutionEndEvent:
            sendFn(ToolExecutionEvent{ToolName: ev.ToolName, IsStarting: false})
        case kit.ToolResultEvent:
            sendFn(ToolResultEvent{
                ToolName: ev.ToolName, ToolArgs: ev.ToolArgs,
                Result: ev.Result, IsError: ev.IsError,
            })
        case kit.ToolCallContentEvent:
            sendFn(ToolCallContentEvent{Content: ev.Content})
        case kit.ResponseEvent:
            sendFn(ResponseCompleteEvent{Content: ev.Content})
        case kit.MessageUpdateEvent:
            sendFn(StreamChunkEvent{Content: ev.Chunk})
        }
    }))

    return func() {
        for _, unsub := range unsubs {
            unsub()
        }
    }
}
```

**5c. Pass Kit in `cmd/root.go`**:

In the `BuildAppOptions` call or directly after:

```go
appOpts.Kit = kitInstance
```

**Verification**:
```bash
go build -o output/kit ./cmd/kit
go test -race ./...
golangci-lint run ./...
```

The bridge function exists but is not called yet. Step 6 wires it in.

---

### Step 6: Migrate executeStep() to use kit.PromptResult()

**File**: `internal/app/app.go`

Replace the 150+ line `executeStep()` with a thin wrapper around `kit.PromptResult()`.

**6a. Rewrite executeStep()**:

The new `executeStep()` when `opts.Kit` is set:

```go
func (a *App) executeStep(ctx context.Context, prompt string, eventFn func(tea.Msg)) (*agent.GenerateWithLoopResult, error) {
    if a.opts.Kit == nil {
        return a.executeStepLegacy(ctx, prompt, eventFn)
    }

    sendFn := func(msg tea.Msg) {
        if eventFn != nil {
            eventFn(msg)
        }
    }

    // Subscribe to SDK events for TUI rendering. The subscription is
    // temporary — it lives only for the duration of this step.
    unsub := a.subscribeSDKEvents(sendFn)
    defer unsub()

    // Show spinner while the agent works.
    sendFn(SpinnerEvent{Show: true})

    result, err := a.opts.Kit.PromptResult(ctx, prompt)
    if err != nil {
        return nil, err
    }

    // Sync in-memory store with the SDK's authoritative conversation.
    a.store.Replace(result.Messages)

    // Update usage tracker.
    a.updateUsageFromTurnResult(result, prompt)

    return &agent.GenerateWithLoopResult{
        ConversationMessages: result.Messages,
    }, nil
}
```

**6b. Rename existing executeStep to executeStepLegacy**:

Keep the old implementation as `executeStepLegacy()` so the transition is safe. It remains as a fallback when `opts.Kit == nil` (e.g. in tests that supply a stub `AgentRunner`).

**6c. Add `updateUsageFromTurnResult` helper**:

```go
func (a *App) updateUsageFromTurnResult(result *kit.TurnResult, userPrompt string) {
    if a.opts.UsageTracker == nil || result == nil {
        return
    }

    if result.TotalUsage != nil {
        inputTokens := int(result.TotalUsage.InputTokens)
        outputTokens := int(result.TotalUsage.OutputTokens)
        if inputTokens > 0 && outputTokens > 0 {
            cacheReadTokens := int(result.TotalUsage.CacheReadTokens)
            cacheWriteTokens := int(result.TotalUsage.CacheCreationTokens)
            a.opts.UsageTracker.UpdateUsage(inputTokens, outputTokens, cacheReadTokens, cacheWriteTokens)
        } else {
            a.opts.UsageTracker.EstimateAndUpdateUsage(userPrompt, result.Response)
            return
        }
    }

    if result.FinalUsage != nil {
        if ct := int(result.FinalUsage.InputTokens) + int(result.FinalUsage.OutputTokens); ct > 0 {
            a.opts.UsageTracker.SetContextTokens(ct)
        }
    }
}
```

**6d. Remove extension event emission from `executeStepLegacy()`**:

Since the SDK bridge (Step 4) now forwards extension observation events, remove these direct calls from `executeStepLegacy()`:
- `extensions.AgentStart` emission (line 432-434)
- `extensions.MessageStart` emission (line 440-442)
- `extensions.MessageUpdate` emission (line 473-475)
- `extensions.MessageEnd` emission (line 496-498)
- `extensions.AgentEnd` emission (lines 482-487, 501-506)

The `Input` and `BeforeAgentStart` extensions are already handled by the SDK hooks (bridged in Plan 09). Remove those too from `executeStepLegacy()`:
- `extensions.Input` emission (lines 372-387)
- `extensions.BeforeAgentStart` emission (lines 414-429)

What remains in `executeStepLegacy()` is just the core generation call — which is now essentially the same as calling `kit.PromptResult()`.

**6e. Remove SessionShutdown from `app.Close()`**:

Since `Kit.Close()` now handles SessionShutdown (Step 4b), remove:

```go
// In app.Close() — remove:
if a.opts.Extensions != nil && a.opts.Extensions.HasHandlers(extensions.SessionShutdown) {
    _, _ = a.opts.Extensions.Emit(extensions.SessionShutdownEvent{})
}
```

**Verification**:
```bash
go build -o output/kit ./cmd/kit
go test -race ./...
golangci-lint run ./...
# Manual: run `kit -p "list files in the current directory"` — verify tool calls render
# Manual: run `kit` in interactive mode — verify streaming, tool results, spinner
# Manual: create a .kit/extensions/ extension with AgentStart handler — verify it fires
```

---

### Step 7: Clean up dead code

**Files**: `internal/app/app.go`, `internal/app/options.go`, `internal/app/events.go`, `cmd/setup.go`

**7a. Remove `executeStepLegacy()`**:

Once confident the SDK path works, delete `executeStepLegacy()` entirely. Update `executeStep()` to remove the `if a.opts.Kit == nil` guard.

**7b. Remove `AgentRunner` interface**:

`internal/app/options.go:17-28` — delete `AgentRunner`. The `Agent AgentRunner` field is no longer used when `Kit` is set. Remove the `Agent` field from `Options`.

**7c. Remove `Extensions` field from `app.Options`**:

`internal/app/options.go:94-98` — the app no longer calls `a.opts.Extensions.Emit()` directly. Extension dispatch goes through SDK hooks/events. Remove the field and all `a.opts.Extensions` references in `app.go`.

**7d. Simplify `BuildAppOptions()` in `cmd/setup.go`**:

Remove the `mcpAgent` and `extRunner` parameters since the app gets these from `Kit`:

```go
func BuildAppOptions(kitInstance *kit.Kit, mcpConfig *config.Config,
    modelName string, serverNames, toolNames []string) app.Options {
    return app.Options{
        Kit:              kitInstance,
        MCPConfig:        mcpConfig,
        ModelName:        modelName,
        ServerNames:      serverNames,
        ToolNames:        toolNames,
        StreamingEnabled: viper.GetBool("stream"),
        Quiet:            quietFlag,
        Debug:            viper.GetBool("debug"),
        CompactMode:      viper.GetBool("compact"),
    }
}
```

**7e. Remove `updateUsage()` from `app.go`** (`app.go:596-627`):

Replaced by `updateUsageFromTurnResult()` which works with `TurnResult` instead of raw `GenerateWithLoopResult`.

**7f. Simplify `SessionStart` emission**:

Move SessionStart from `cmd/root.go:448` into `Kit.New()` or a new `Kit.EmitSessionStart()` method called by the CLI after extension context is configured.

**7g. Remove `inputSource()` helper** (`app.go:524-532`):

Only used by the now-removed Input extension emission.

**7h. Run final verification**:

```bash
go build -o output/kit ./cmd/kit
go test -race ./...
golangci-lint run ./...
```

Confirm no references to removed types/functions. Confirm no unused imports.

---

## Verification Checklist

- [ ] `go build -o output/kit ./cmd/kit` succeeds
- [ ] `go test -race ./...` passes
- [ ] `golangci-lint run ./...` — 0 issues
- [ ] `kit.New()` creates agent, session, extensions in one call
- [ ] `cmd/root.go` no longer calls `SetupAgent()` directly
- [ ] `executeStep()` delegates to `kit.PromptResult()`
- [ ] SDK events drive TUI rendering (tool calls, streaming, results)
- [ ] Extension observation events (AgentStart/End, MessageStart/Update/End) fire via SDK bridge
- [ ] Extension interception events (Input, BeforeAgentStart, ToolCall, ToolResult) still work
- [ ] Usage tracker receives correct token counts
- [ ] Session persistence works (tree session)
- [ ] `--continue` / `--no-session` / `--session` flags work
- [ ] Spinner shows/hides correctly
- [ ] Interactive mode (BubbleTea) works
- [ ] Non-interactive mode (`-p "..."`) works
- [ ] Extension SessionShutdown fires on close
- [ ] No remaining direct `extensions.Emit()` calls in `app.go`
- [ ] `AgentRunner` interface removed
- [ ] `app.Options.Extensions` field removed
