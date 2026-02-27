package kit

import "github.com/mark3labs/kit/internal/extensions"

// bridgeExtensions registers extension event handlers as SDK hooks. This makes
// the existing extension system a consumer of the SDK hook API, proving the
// hook surface is production-ready.
//
// Phase 1 (this plan): bridge BeforeAgentStart and Input as BeforeTurn hooks.
// Tool-level events (ToolCall, ToolResult) are already handled by the extension
// tool wrapper (internal/extensions/wrapper.go) which composes underneath the
// SDK hook wrapper.
//
// Phase 2 (future): app.executeStep() migrates to SDK hooks exclusively.
// Phase 3 (future): extension runner emits SDK events/hooks natively.
func (m *Kit) bridgeExtensions(runner *extensions.Runner) {
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
}
