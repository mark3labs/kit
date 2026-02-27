# Plan 05: Additional Prompt Modes

**Priority**: P1
**Effort**: Medium
**Goal**: Add `Steer()`, `FollowUp()`, `PromptWithOptions()` methods; app's `executeStep()` should call SDK methods

## Background

Pi has `session.prompt()`, `session.steer()`, `session.followUp()`, `session.compact()`. Kit only has `Prompt()` and `PromptWithCallbacks()`. The Kit CLI app implements its own agent loop in `internal/app/app.go:executeStep()` which duplicates SDK logic. After this plan, both the app and SDK users call the same methods.

## Prerequisites

- Plan 00 (Create `pkg/kit/`)
- Plan 03 (Event subscriber system)

## Step-by-Step

### Step 1: Extract shared callback helpers

To avoid duplicating callback wiring across `Prompt`, `Steer`, `FollowUp`, etc., extract internal helpers:

**File**: `pkg/kit/kit.go`

```go
func (m *Kit) makeToolCallHandler() agent.ToolCallHandler {
    return func(name, args string) {
        m.events.emit(ToolCallEvent{ToolName: name, ToolArgs: args})
    }
}

func (m *Kit) makeToolExecutionHandler() agent.ToolExecutionHandler {
    return func(name string, isStarting bool) {
        if isStarting {
            m.events.emit(ToolExecutionStartEvent{ToolName: name})
        } else {
            m.events.emit(ToolExecutionEndEvent{ToolName: name})
        }
    }
}

func (m *Kit) makeToolResultHandler() agent.ToolResultHandler {
    return func(name, args, result string, isError bool) {
        m.events.emit(ToolResultEvent{ToolName: name, ToolArgs: args, Result: result, IsError: isError})
    }
}

func (m *Kit) makeResponseHandler() agent.ResponseHandler {
    return func(content string) { m.events.emit(ResponseEvent{Content: content}) }
}

func (m *Kit) makeStreamingHandler() agent.StreamingResponseHandler {
    return func(chunk string) { m.events.emit(MessageUpdateEvent{Chunk: chunk}) }
}

// getMessages retrieves conversation history from the best available source.
func (m *Kit) getMessages() []fantasy.Message {
    if m.treeSession != nil {
        msgs, _, _ := m.treeSession.BuildContext()
        return msgs
    }
    return m.sessionMgr.GetMessages()
}

// updateSession persists generation results.
func (m *Kit) updateSession(userMsg fantasy.Message, result *agent.GenerateWithLoopResult) {
    if m.treeSession != nil {
        m.treeSession.AppendFantasyMessage(userMsg)
        for _, msg := range result.Messages {
            m.treeSession.AppendMessage(msg)
        }
    }
    _ = m.sessionMgr.ReplaceAllMessages(result.ConversationMessages)
}

// generate is the shared generation path for all prompt modes.
func (m *Kit) generate(ctx context.Context, messages []fantasy.Message) (*agent.GenerateWithLoopResult, error) {
    return m.agent.GenerateWithLoopAndStreaming(
        ctx, messages,
        m.makeToolCallHandler(),
        m.makeToolExecutionHandler(),
        m.makeToolResultHandler(),
        m.makeResponseHandler(),
        nil, // onToolCallContent
        m.makeStreamingHandler(),
    )
}
```

### Step 2: Refactor Prompt() to use shared helpers

```go
func (m *Kit) Prompt(ctx context.Context, msg string) (string, error) {
    messages := m.getMessages()
    userMsg := fantasy.NewUserMessage(msg)
    messages = append(messages, userMsg)

    m.events.emit(TurnStartEvent{Prompt: msg})
    m.events.emit(MessageStartEvent{})

    result, err := m.generate(ctx, messages)
    if err != nil {
        m.events.emit(TurnEndEvent{Error: err})
        return "", fmt.Errorf("generation failed: %w", err)
    }

    m.updateSession(userMsg, result)
    response := result.FinalResponse.Content.Text()
    m.events.emit(MessageEndEvent{Content: response})
    m.events.emit(TurnEndEvent{Response: response})
    return response, nil
}
```

### Step 3: Add Steer()

```go
// Steer injects a system message and triggers a new agent turn.
// Use for dynamically adjusting behavior without a visible user message.
func (m *Kit) Steer(ctx context.Context, instruction string) (string, error) {
    messages := m.getMessages()
    sysMsg := fantasy.NewSystemMessage(instruction)
    messages = append(messages, sysMsg)
    userMsg := fantasy.NewUserMessage("Please acknowledge and follow the above instruction.")
    messages = append(messages, userMsg)

    m.events.emit(TurnStartEvent{Prompt: "[steer] " + instruction})
    m.events.emit(MessageStartEvent{})

    result, err := m.generate(ctx, messages)
    if err != nil {
        m.events.emit(TurnEndEvent{Error: err})
        return "", fmt.Errorf("steer failed: %w", err)
    }

    m.updateSession(userMsg, result)
    response := result.FinalResponse.Content.Text()
    m.events.emit(MessageEndEvent{Content: response})
    m.events.emit(TurnEndEvent{Response: response})
    return response, nil
}
```

### Step 4: Add FollowUp()

```go
// FollowUp continues the conversation without new user input.
func (m *Kit) FollowUp(ctx context.Context) (string, error) {
    messages := m.getMessages()
    if len(messages) == 0 {
        return "", fmt.Errorf("cannot follow up: no previous messages")
    }
    userMsg := fantasy.NewUserMessage("Continue.")
    messages = append(messages, userMsg)

    m.events.emit(TurnStartEvent{Prompt: "[follow-up]"})
    m.events.emit(MessageStartEvent{})

    result, err := m.generate(ctx, messages)
    if err != nil {
        m.events.emit(TurnEndEvent{Error: err})
        return "", fmt.Errorf("follow-up failed: %w", err)
    }

    m.updateSession(userMsg, result)
    response := result.FinalResponse.Content.Text()
    m.events.emit(MessageEndEvent{Content: response})
    m.events.emit(TurnEndEvent{Response: response})
    return response, nil
}
```

### Step 5: Add PromptWithOptions()

```go
type PromptOptions struct {
    SystemMessage string // Injected before the prompt
    MaxSteps      int    // Override max steps for this call (0 = default)
}

func (m *Kit) PromptWithOptions(ctx context.Context, msg string, opts PromptOptions) (string, error) {
    messages := m.getMessages()
    if opts.SystemMessage != "" {
        messages = append(messages, fantasy.NewSystemMessage(opts.SystemMessage))
    }
    userMsg := fantasy.NewUserMessage(msg)
    messages = append(messages, userMsg)

    m.events.emit(TurnStartEvent{Prompt: msg})
    m.events.emit(MessageStartEvent{})

    result, err := m.generate(ctx, messages)
    if err != nil {
        m.events.emit(TurnEndEvent{Error: err})
        return "", fmt.Errorf("generation failed: %w", err)
    }

    m.updateSession(userMsg, result)
    response := result.FinalResponse.Content.Text()
    m.events.emit(MessageEndEvent{Content: response})
    m.events.emit(TurnEndEvent{Response: response})
    return response, nil
}
```

### Step 6: App-as-Consumer — Refactor `executeStep()` to use SDK

Currently `internal/app/app.go:executeStep()` (lines 364-520) contains a full agent loop with extension events, message building, and session persistence. It should be replaced by SDK method calls.

**File**: `internal/app/app.go` (migration)

```go
// Before: 150+ lines of agent loop logic in executeStep()

// After: executeStep delegates to the Kit SDK
func (a *App) executeStep(ctx context.Context, prompt string, sendFn func(tea.Msg)) (*agent.GenerateWithLoopResult, error) {
    // Extension Input hook (stays in app — it's a pre-SDK concern)
    if a.opts.Extensions != nil && a.opts.Extensions.HasHandlers(extensions.Input) {
        result, _ := a.opts.Extensions.Emit(extensions.InputEvent{Text: prompt})
        if r, ok := result.(extensions.InputResult); ok && r.Action == "handled" {
            return nil, nil
        }
        if r, ok := result.(extensions.InputResult); ok && r.Text != "" {
            prompt = r.Text
        }
    }

    sendFn(SpinnerEvent{Show: true})

    // Use SDK prompt — events handled by subscriber bridge (Plan 03)
    response, err := a.kit.Prompt(ctx, prompt)
    if err != nil {
        sendFn(StepErrorEvent{Err: err})
        return nil, err
    }

    sendFn(SpinnerEvent{Show: false})
    sendFn(StepCompleteEvent{})
    _ = response

    return nil, nil // Result comes through events
}
```

**Note**: This is a simplification. The real migration needs to handle:
- Extension `BeforeAgentStart` events (map to Plan 09 hooks)
- Spinner show/hide
- The fact that `executeStep` returns `*GenerateWithLoopResult` for further processing

The migration is gradual:
1. **Phase 1**: App calls `kit.Prompt()` for simple cases
2. **Phase 2**: Extension events bridge through SDK hooks (Plan 09)
3. **Phase 3**: `executeStep()` becomes a thin adapter

### Step 7: Verify

```bash
go build -o output/kit ./cmd/kit
go test -race ./...
go vet ./...
```

## Files Changed Summary

| Action | File | Change |
|--------|------|--------|
| EDIT | `pkg/kit/kit.go` | Steer(), FollowUp(), PromptWithOptions(), shared helpers |
| EDIT | `internal/app/app.go` | Gradual migration of executeStep to use SDK |

## Verification Checklist

- [ ] `Steer()` injects system message and triggers response
- [ ] `FollowUp()` continues without user message
- [ ] `PromptWithOptions()` accepts per-call system message
- [ ] All methods emit events via EventBus
- [ ] Shared helpers eliminate callback duplication
- [ ] App's `executeStep()` uses SDK (at least for simple paths)
