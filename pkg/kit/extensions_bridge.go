package kit

import "github.com/mark3labs/kit/internal/extensions"

// bridgeExtensions registers extension event handlers as SDK hooks and
// subscribes to SDK observation events to forward them to the extension runner.
//
// Interception hooks (Input, BeforeAgentStart) were bridged in Plan 09.
// Observation events (AgentStart/End, MessageStart/Update/End) are bridged here
// so extensions see them regardless of whether the app layer or SDK drives
// the generation loop.
//
// Tool-level events (ToolCall, ToolResult) are handled by the extension tool
// wrapper (internal/extensions/wrapper.go) which composes underneath the SDK
// hook wrapper.
func (m *Kit) bridgeExtensions(runner *extensions.Runner) {
	// --- Interception hooks ---

	// Extension Input → BeforeTurn hook (high priority, runs first).
	// An Input handler with Action="transform" replaces the prompt text.
	if runner.HasHandlers(extensions.Input) {
		m.OnBeforeTurn(HookPriorityHigh, func(h BeforeTurnHook) *BeforeTurnResult {
			result, _ := runner.Emit(extensions.InputEvent{Text: h.Prompt})
			if r, ok := result.(extensions.InputResult); ok {
				if r.Action == "transform" {
					return &BeforeTurnResult{Prompt: &r.Text}
				}
			}
			return nil
		})
	}

	// Extension BeforeAgentStart → BeforeTurn hook (normal priority).
	// Can inject a system prompt prefix and/or context text.
	if runner.HasHandlers(extensions.BeforeAgentStart) {
		m.OnBeforeTurn(HookPriorityNormal, func(h BeforeTurnHook) *BeforeTurnResult {
			result, _ := runner.Emit(extensions.BeforeAgentStartEvent{Prompt: h.Prompt})
			if r, ok := result.(extensions.BeforeAgentStartResult); ok {
				return &BeforeTurnResult{
					SystemPrompt: r.SystemPrompt,
					InjectText:   r.InjectText,
				}
			}
			return nil
		})
	}

	// --- Observation event forwarding ---
	// Subscribe to SDK events and forward to extension runner so extensions
	// see lifecycle events from the SDK's runTurn()/generate() path.

	if runner.HasHandlers(extensions.AgentStart) {
		m.Subscribe(func(e Event) {
			if ev, ok := e.(TurnStartEvent); ok {
				_, _ = runner.Emit(extensions.AgentStartEvent{Prompt: ev.Prompt})
			}
		})
	}

	if runner.HasHandlers(extensions.MessageStart) {
		m.Subscribe(func(e Event) {
			if _, ok := e.(MessageStartEvent); ok {
				_, _ = runner.Emit(extensions.MessageStartEvent{})
			}
		})
	}

	if runner.HasHandlers(extensions.MessageUpdate) {
		m.Subscribe(func(e Event) {
			if ev, ok := e.(MessageUpdateEvent); ok {
				_, _ = runner.Emit(extensions.MessageUpdateEvent{Chunk: ev.Chunk})
			}
		})
	}

	if runner.HasHandlers(extensions.MessageEnd) {
		m.Subscribe(func(e Event) {
			if ev, ok := e.(MessageEndEvent); ok {
				_, _ = runner.Emit(extensions.MessageEndEvent{Content: ev.Content})
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
				_, _ = runner.Emit(extensions.AgentEndEvent{
					Response:   response,
					StopReason: stopReason,
				})
			}
		})
	}
}
