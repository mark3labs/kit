package extensions

import (
	"context"
	"encoding/json"

	"github.com/mark3labs/kit/internal/hooks"
)

// HooksAsExtension wraps an existing hooks.HookConfig as a LoadedExtension
// so that legacy .kit/hooks.yml configurations continue to work alongside
// the new Yaegi extension system. The adapter translates the old event names
// and shell-command execution model into extension HandlerFunc handlers.
func HooksAsExtension(config *hooks.HookConfig) *LoadedExtension {
	if config == nil || len(config.Hooks) == 0 {
		return nil
	}

	ext := &LoadedExtension{
		Path:     "hooks.yml (compat)",
		Handlers: make(map[EventType][]HandlerFunc),
	}

	executor := hooks.NewExecutor(config, "", "")

	// Map PreToolUse → ToolCall
	if matchers, ok := config.Hooks[hooks.PreToolUse]; ok && len(matchers) > 0 {
		ext.Handlers[ToolCall] = []HandlerFunc{
			func(event Event, _ Context) Result {
				tc, ok := event.(ToolCallEvent)
				if !ok {
					return nil
				}
				input := &hooks.PreToolUseInput{
					ToolName:  tc.ToolName,
					ToolInput: json.RawMessage(tc.Input),
				}
				output, err := executor.ExecuteHooks(context.Background(), hooks.PreToolUse, input)
				if err != nil || output == nil {
					return nil
				}
				if output.Decision == "block" {
					return ToolCallResult{Block: true, Reason: output.Reason}
				}
				return nil
			},
		}
	}

	// Map PostToolUse → ToolResult
	if matchers, ok := config.Hooks[hooks.PostToolUse]; ok && len(matchers) > 0 {
		ext.Handlers[ToolResult] = []HandlerFunc{
			func(event Event, _ Context) Result {
				tr, ok := event.(ToolResultEvent)
				if !ok {
					return nil
				}
				input := &hooks.PostToolUseInput{
					ToolName:     tr.ToolName,
					ToolInput:    json.RawMessage(tr.Input),
					ToolResponse: json.RawMessage(tr.Content),
				}
				_, _ = executor.ExecuteHooks(context.Background(), hooks.PostToolUse, input)
				return nil // legacy hooks don't modify results
			},
		}
	}

	// Map UserPromptSubmit → Input
	if matchers, ok := config.Hooks[hooks.UserPromptSubmit]; ok && len(matchers) > 0 {
		ext.Handlers[Input] = []HandlerFunc{
			func(event Event, _ Context) Result {
				ie, ok := event.(InputEvent)
				if !ok {
					return nil
				}
				input := &hooks.UserPromptSubmitInput{
					Prompt: ie.Text,
				}
				output, err := executor.ExecuteHooks(context.Background(), hooks.UserPromptSubmit, input)
				if err != nil || output == nil {
					return nil
				}
				if output.Decision == "block" {
					return InputResult{Action: "handled"}
				}
				return nil
			},
		}
	}

	// Map Stop → AgentEnd
	if matchers, ok := config.Hooks[hooks.Stop]; ok && len(matchers) > 0 {
		ext.Handlers[AgentEnd] = []HandlerFunc{
			func(event Event, _ Context) Result {
				ae, ok := event.(AgentEndEvent)
				if !ok {
					return nil
				}
				input := &hooks.StopInput{
					Response:   ae.Response,
					StopReason: ae.StopReason,
				}
				_, _ = executor.ExecuteHooks(context.Background(), hooks.Stop, input)
				return nil
			},
		}
	}

	return ext
}
