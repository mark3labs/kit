# Plan 03: Event/Subscriber System

**Priority**: P1
**Effort**: High
**Goal**: Create a unified event system in the SDK that replaces the three parallel event systems currently in the codebase

## Background

Kit currently has **three separate event systems** that overlap:

1. **SDK callbacks** (`sdk/kit.go`) — 3 function pointers on `PromptWithCallbacks`
2. **Extension events** (`internal/extensions/events.go`) — 13 typed events dispatched via `Runner.Emit()`
3. **App/TUI events** (`internal/app/events.go`) — 13 `tea.Msg` structs for BubbleTea UI updates

Pi uses a single `session.subscribe(listener)` pattern. This plan creates a unified event system in `pkg/kit/` that:
- Replaces SDK callbacks
- Becomes the canonical event layer that extensions and the app emit through
- The TUI adapts SDK events into `tea.Msg` for rendering (TUI-specific concern stays in `internal/ui/`)

## Prerequisites

- Plan 00 (Create `pkg/kit/`)
- Plan 02 (Richer type exports)

## Design Decisions

1. **Single source of truth** — events are defined in `pkg/kit/`, not scattered across packages
2. **Multiple subscribers** supported with unsubscribe
3. **Thread-safe** emission
4. **App subscribes to SDK events** — the TUI layer adapts them to `tea.Msg`
5. **Extensions emit through SDK** — the extension runner emits SDK events, not its own types

## Step-by-Step

### Step 1: Define public event types

**File**: `pkg/kit/events.go` (new)

```go
package kit

import "sync"

// EventType identifies the kind of event.
type EventType string

const (
    EventTurnStart          EventType = "turn_start"
    EventTurnEnd            EventType = "turn_end"
    EventMessageStart       EventType = "message_start"
    EventMessageUpdate      EventType = "message_update"
    EventMessageEnd         EventType = "message_end"
    EventToolCall           EventType = "tool_call"
    EventToolExecutionStart EventType = "tool_execution_start"
    EventToolExecutionEnd   EventType = "tool_execution_end"
    EventToolResult         EventType = "tool_result"
    EventToolCallContent    EventType = "tool_call_content"
    EventResponse           EventType = "response"
    EventSessionStart       EventType = "session_start"
    EventSessionShutdown    EventType = "session_shutdown"
)

// Event is the interface for all event types.
type Event interface {
    EventType() EventType
}
```

### Step 2: Define concrete event structs

These cover the union of all three current event systems:

```go
type TurnStartEvent struct{ Prompt string }
func (e TurnStartEvent) EventType() EventType { return EventTurnStart }

type TurnEndEvent struct{ Response string; Error error }
func (e TurnEndEvent) EventType() EventType { return EventTurnEnd }

type MessageStartEvent struct{}
func (e MessageStartEvent) EventType() EventType { return EventMessageStart }

type MessageUpdateEvent struct{ Chunk string }
func (e MessageUpdateEvent) EventType() EventType { return EventMessageUpdate }

type MessageEndEvent struct{ Content string }
func (e MessageEndEvent) EventType() EventType { return EventMessageEnd }

type ToolCallEvent struct{ ToolName string; ToolArgs string }
func (e ToolCallEvent) EventType() EventType { return EventToolCall }

type ToolExecutionStartEvent struct{ ToolName string }
func (e ToolExecutionStartEvent) EventType() EventType { return EventToolExecutionStart }

type ToolExecutionEndEvent struct{ ToolName string }
func (e ToolExecutionEndEvent) EventType() EventType { return EventToolExecutionEnd }

type ToolResultEvent struct{ ToolName, ToolArgs, Result string; IsError bool }
func (e ToolResultEvent) EventType() EventType { return EventToolResult }

type ToolCallContentEvent struct{ Content string }
func (e ToolCallContentEvent) EventType() EventType { return EventToolCallContent }

type ResponseEvent struct{ Content string }
func (e ResponseEvent) EventType() EventType { return EventResponse }
```

### Step 3: Implement EventBus

```go
type EventListener func(event Event)

type eventBus struct {
    mu        sync.RWMutex
    listeners map[int]EventListener
    nextID    int
}

func newEventBus() *eventBus {
    return &eventBus{listeners: make(map[int]EventListener)}
}

func (eb *eventBus) subscribe(listener EventListener) func() {
    eb.mu.Lock()
    id := eb.nextID
    eb.nextID++
    eb.listeners[id] = listener
    eb.mu.Unlock()
    return func() {
        eb.mu.Lock()
        delete(eb.listeners, id)
        eb.mu.Unlock()
    }
}

func (eb *eventBus) emit(event Event) {
    eb.mu.RLock()
    snapshot := make([]EventListener, 0, len(eb.listeners))
    for _, l := range eb.listeners {
        snapshot = append(snapshot, l)
    }
    eb.mu.RUnlock()
    for _, l := range snapshot {
        l(event)
    }
}
```

### Step 4: Wire EventBus into Kit struct

**File**: `pkg/kit/kit.go`

```go
type Kit struct {
    agent       *agent.Agent
    sessionMgr  *session.Manager
    modelString string
    events      *eventBus
}

func (m *Kit) Subscribe(listener EventListener) func() {
    return m.events.subscribe(listener)
}
```

### Step 5: Wire all agent callbacks to emit events

Update `Prompt()` and `PromptWithCallbacks()` to emit events at every stage of the agent generation flow. Events fire at these points (matching the lifecycle in `internal/app/app.go:364-520`):

1. Before generation: `TurnStartEvent`, `MessageStartEvent`
2. During streaming: `MessageUpdateEvent` per chunk
3. On tool call: `ToolCallEvent`, `ToolExecutionStartEvent`
4. On tool result: `ToolExecutionEndEvent`, `ToolResultEvent`
5. On response: `ResponseEvent`
6. After generation: `MessageEndEvent`, `TurnEndEvent`

Extract shared callback helpers to avoid duplication:

```go
func (m *Kit) makeToolCallHandler() agent.ToolCallHandler {
    return func(name, args string) {
        m.events.emit(ToolCallEvent{ToolName: name, ToolArgs: args})
    }
}
// ... similar for all callback types
```

### Step 6: App-as-Consumer — TUI subscribes to SDK events

This is the critical refactor. Currently `internal/app/app.go:executeStep()` emits TUI events directly via `sendFn(StreamChunkEvent{...})`. After this change:

1. The SDK's `Prompt()` emits SDK events
2. The app subscribes to SDK events and converts them to `tea.Msg`

**File**: `internal/app/app.go` (migration pattern)

```go
// In App initialization, subscribe to SDK events and bridge to TUI
func (a *App) setupEventBridge() {
    a.kit.Subscribe(func(e kit.Event) {
        switch ev := e.(type) {
        case kit.MessageUpdateEvent:
            a.sendToTUI(StreamChunkEvent{Content: ev.Chunk})
        case kit.ToolCallEvent:
            a.sendToTUI(ToolCallStartedEvent{ToolName: ev.ToolName, ToolArgs: ev.ToolArgs})
        case kit.ToolResultEvent:
            a.sendToTUI(ToolResultEvent{
                ToolName: ev.ToolName, ToolArgs: ev.ToolArgs,
                Result: ev.Result, IsError: ev.IsError,
            })
        case kit.ResponseEvent:
            a.sendToTUI(ResponseCompleteEvent{Content: ev.Content})
        // ... etc
        }
    })
}
```

**Migration steps**:
1. First: app subscribes to SDK events AND keeps its own emission (dual-emit phase)
2. Then: remove direct emission from `executeStep()`, rely solely on SDK events
3. Finally: remove `internal/app/events.go` types that are now redundant

### Step 7: Extension events bridge to SDK events

The extension `Runner` should emit through the SDK event bus rather than its own parallel system. This can be bridged:

```go
// In Kit initialization, bridge extension events to SDK events
func (m *Kit) bridgeExtensionEvents(runner *extensions.Runner) {
    // When extensions emit events, forward them as SDK events
    // This is done by having the Runner call back into the SDK
    runner.SetEventForwarder(func(event extensions.Event) {
        switch e := event.(type) {
        case extensions.ToolCallEvent:
            m.events.emit(ToolCallEvent{ToolName: e.ToolName, ToolArgs: e.Input})
        // ... etc
        }
    })
}
```

**Note**: This is a gradual migration. The extension Runner keeps its typed events for Yaegi compatibility, but forwards them to the SDK bus. Eventually the extension system could be refactored to emit SDK events natively.

### Step 8: Typed convenience subscribers

```go
func (m *Kit) OnToolCall(handler func(ToolCallEvent)) func() {
    return m.Subscribe(func(e Event) {
        if tc, ok := e.(ToolCallEvent); ok { handler(tc) }
    })
}

func (m *Kit) OnToolResult(handler func(ToolResultEvent)) func() {
    return m.Subscribe(func(e Event) {
        if tr, ok := e.(ToolResultEvent); ok { handler(tr) }
    })
}

func (m *Kit) OnStreaming(handler func(MessageUpdateEvent)) func() {
    return m.Subscribe(func(e Event) {
        if mu, ok := e.(MessageUpdateEvent); ok { handler(mu) }
    })
}
```

### Step 9: Write tests and verify

```bash
go build -o output/kit ./cmd/kit
go test -race ./...
go vet ./...
```

## Files Changed Summary

| Action | File | Change |
|--------|------|--------|
| CREATE | `pkg/kit/events.go` | Event types, EventBus, Subscribe() |
| EDIT | `pkg/kit/kit.go` | Add eventBus field, Subscribe(), callback helpers |
| EDIT | `internal/app/app.go` | Subscribe to SDK events (gradual migration) |
| EDIT | `internal/extensions/runner.go` | Optional: event forwarding to SDK bus |

## Event Flow After This Plan

```
Agent.GenerateWithLoopAndStreaming()
    ↓ fantasy callbacks
pkg/kit/kit.go  (SDK Prompt method)
    ↓ emits SDK events
EventBus
    ↓ dispatches to all subscribers
    ├── External SDK user's listener
    ├── App TUI bridge → tea.Msg → BubbleTea Update()
    └── Extension bridge → Runner.Emit() → Yaegi handlers
```

**Single source of truth**: The SDK EventBus is the only event dispatcher.

## Verification Checklist

- [x] `go build -o output/kit ./cmd/kit` succeeds
- [x] `go test -race ./...` passes
- [x] Events fire in correct order: TurnStart → MessageStart → updates → ToolCall → ToolResult → MessageEnd → TurnEnd
- [x] Multiple subscribers receive all events
- [x] Unsubscribe removes listener
- [ ] App TUI still renders correctly via event bridge (deferred — see below)
- [x] Thread-safe under concurrent calls

## Implemented (Steps 1-5, 8-9)

Core event system is complete:
- Event types, concrete structs, EventBus in `pkg/kit/events.go`
- `Kit.Subscribe()` + typed helpers (`OnToolCall`, `OnToolResult`, `OnStreaming`, `OnResponse`, `OnTurnStart`, `OnTurnEnd`)
- `Prompt()` and `PromptWithCallbacks()` emit full lifecycle events
- 10 tests covering subscribe/unsubscribe, ordering, concurrency, self-unsubscribe
- Example updated to use `Subscribe` API; `PromptWithCallbacks` marked deprecated

## Deferred (Steps 6-7)

### Step 6: App TUI bridge — app subscribes to SDK events

The app (`internal/app/app.go`) currently owns an `AgentRunner` interface (not a `*Kit`),
and emits `tea.Msg` events directly from `executeStep()` callbacks. To bridge through the
SDK EventBus:

1. The app needs a `*Kit` reference (or at minimum an `*eventBus` / `Subscribe` func)
2. `executeStep()` would stop emitting `tea.Msg` directly and instead let the SDK emit
   SDK events, with a subscriber in the app that converts them to `tea.Msg`
3. Dual-emit phase first (both old and new), then remove direct emission

**Why deferred**: The app doesn't have a `Kit` reference — it receives an `AgentRunner`.
Changing this requires restructuring `internal/app/options.go` and `cmd/root.go` where
the app is created. This is better done as part of the gradual "app consumes SDK" migration
(tracked in the README architecture diagram).

### Step 7: Extension events bridge — Runner emits through SDK EventBus

The extension `Runner` (`internal/extensions/runner.go`) has its own typed events. To
forward them through the SDK bus:

1. Add `SetEventForwarder(func(extensions.Event))` to Runner
2. In Kit initialization, bridge extension events to SDK events
3. Extensions keep their typed events for Yaegi compatibility but forward to SDK bus

**Why deferred**: Same dependency as Step 6 — requires the Kit instance to be wired
into the extension runner initialization path. Plan 09 (Extension hook system) is the
natural place to complete this bridge.
