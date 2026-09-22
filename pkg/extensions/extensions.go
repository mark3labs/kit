// Package extensions is the public, extension-facing type surface of Kit.
//
// Kit extensions are plain Go files interpreted at runtime, so they never
// import this package themselves — they use the "kit/ext" import path that the
// interpreter provides. This package exists for the Go code AROUND an
// extension: unit tests, build tooling, and any program that drives the test
// harness in github.com/mark3labs/kit/pkg/extensions/test.
//
// Every declaration here is an alias for the corresponding Kit-internal type,
// so values cross the boundary with no conversion:
//
//	package main
//
//	import (
//	    "testing"
//
//	    ext "github.com/mark3labs/kit/pkg/extensions"
//	    "github.com/mark3labs/kit/pkg/extensions/test"
//	)
//
//	func TestBlocksDangerousTool(t *testing.T) {
//	    h := test.New(t)
//	    h.LoadFile("my-ext.go")
//
//	    result, err := h.Emit(ext.ToolCallEvent{
//	        ToolName: "write",
//	        Input:    `{"path":"main.go"}`,
//	    })
//	    if err != nil {
//	        t.Fatal(err)
//	    }
//	    test.AssertBlocked(t, result, "not allowed")
//	}
package extensions

import (
	internalext "github.com/mark3labs/kit/internal/extensions"
)

// ---------------------------------------------------------------------------
// Aliases from internal/extensions/events.go
// ---------------------------------------------------------------------------

// EventType identifies a point in KIT's lifecycle where extensions can hook in.
type EventType = internalext.EventType

// ToolCall fires before a tool executes. Handlers can block execution.
const ToolCall = internalext.ToolCall

// ToolCallInputStart fires when the LLM begins generating tool call
// arguments. The tool name is known but the full argument JSON is still
// being streamed.
const ToolCallInputStart = internalext.ToolCallInputStart

// ToolCallInputDelta fires for each streamed fragment of tool call
// arguments as they arrive from the LLM.
const ToolCallInputDelta = internalext.ToolCallInputDelta

// ToolCallInputEnd fires when tool argument streaming is complete,
// before the tool call is parsed and execution begins.
const ToolCallInputEnd = internalext.ToolCallInputEnd

// ToolExecutionStart fires when a tool begins executing.
const ToolExecutionStart = internalext.ToolExecutionStart

// ToolExecutionEnd fires when a tool finishes executing.
const ToolExecutionEnd = internalext.ToolExecutionEnd

// ToolOutput fires when a tool produces streaming output chunks.
const ToolOutput = internalext.ToolOutput

// ToolResult fires after a tool executes. Handlers can modify the result.
const ToolResult = internalext.ToolResult

// Input fires when user input is received. Handlers can transform or handle it.
const Input = internalext.Input

// BeforeAgentStart fires before the agent loop begins for a prompt.
const BeforeAgentStart = internalext.BeforeAgentStart

// AgentStart fires when the agent loop begins processing.
const AgentStart = internalext.AgentStart

// AgentEnd fires when the agent finishes responding.
const AgentEnd = internalext.AgentEnd

// MessageStart fires when a new assistant message begins.
const MessageStart = internalext.MessageStart

// MessageUpdate fires for each streaming text chunk.
const MessageUpdate = internalext.MessageUpdate

// MessageEnd fires when the assistant message is complete.
const MessageEnd = internalext.MessageEnd

// SessionStart fires when a session is loaded or created.
const SessionStart = internalext.SessionStart

// SessionShutdown fires when the application is closing.
const SessionShutdown = internalext.SessionShutdown

// ModelChange fires after the active model is changed via ctx.SetModel().
const ModelChange = internalext.ModelChange

// ThinkingLevelChange fires after the extended-thinking effort level is
// changed, whether by the user, an extension, or an automatic downgrade
// when switching to a model that does not support the current level.
const ThinkingLevelChange = internalext.ThinkingLevelChange

// TerminalResize fires when the terminal dimensions change, and once at
// startup with the initial size. Interactive TUI only.
const TerminalResize = internalext.TerminalResize

// TurnStateChange fires when the UI moves between its idle and working
// states. Unlike AgentStart/AgentEnd this also covers work that never
// reaches the agent loop, such as shell commands. Interactive TUI only.
const TurnStateChange = internalext.TurnStateChange

// ContextPrepare fires after context is built from the session tree and
// before the messages are sent to the LLM. Handlers can filter, reorder,
// or inject messages into the context window.
const ContextPrepare = internalext.ContextPrepare

// BeforeFork fires before the session tree is branched to a different
// entry point. Handlers can cancel the fork by returning Cancel=true.
const BeforeFork = internalext.BeforeFork

// BeforeSessionSwitch fires before the session is switched to a new
// branch (e.g. /new command). Handlers can cancel by returning Cancel=true.
const BeforeSessionSwitch = internalext.BeforeSessionSwitch

// BeforeCompact fires before context compaction runs. Handlers can
// cancel compaction by returning Cancel=true.
const BeforeCompact = internalext.BeforeCompact

// SubagentStart fires when a subagent tool call begins executing.
// Carries the tool call ID and the task description.
const SubagentStart = internalext.SubagentStart

// SubagentChunk fires for each real-time event emitted by a running
// subagent: text chunks, tool calls, tool results, etc.
const SubagentChunk = internalext.SubagentChunk

// SubagentEnd fires when a subagent tool call completes (success
// or error). Carries the final response and any error message.
const SubagentEnd = internalext.SubagentEnd

// StepStart fires when a new LLM call begins within a multi-step
// agent turn.
const StepStart = internalext.StepStart

// StepFinish fires when a step completes, providing step number,
// finish reason, and token usage.
const StepFinish = internalext.StepFinish

// ReasoningStart fires when the LLM begins reasoning/thinking.
const ReasoningStart = internalext.ReasoningStart

// Warnings fires when the LLM provider returns warnings.
const Warnings = internalext.Warnings

// Source fires when the LLM references a source (e.g. web search).
const Source = internalext.Source

// Error fires when an agent-level error occurs during streaming.
const Error = internalext.Error

// Retry fires when the LLM provider request is retried after a
// transient error.
const Retry = internalext.Retry

// PrepareStep fires between steps within a multi-step agent turn,
// after steering messages are injected and before messages are sent
// to the LLM. Handlers can replace the context window for this step.
const PrepareStep = internalext.PrepareStep

// LLMUsage fires after each LLM provider call with the token and cost
// deltas for that single call. Extensions use it to attribute usage to
// specific calls/models and to drive budget enforcement between calls.
const LLMUsage = internalext.LLMUsage

// ---------------------------------------------------------------------------
// Aliases from internal/extensions/api.go
// ---------------------------------------------------------------------------

// Event is the interface satisfied by all event types internally.
type Event = internalext.Event

// Result is the interface satisfied by all result types internally.
type Result = internalext.Result

// HandlerFunc is the internal handler signature used by the runner.
type HandlerFunc = internalext.HandlerFunc

// Context provides runtime information to handlers about the current session.
type Context = internalext.Context

// SessionMessage represents a conversation message exposed to extensions.
// This is a simplified, read-only view of the internal message structures.
type SessionMessage = internalext.SessionMessage

// TreeNode represents a node in the session tree for navigation.
// Extensions use this to traverse conversation history and implement
// features like "fresh context" loops and branch summarization.
type TreeNode = internalext.TreeNode

// TreeNavigationResult reports success or failure of tree operations.
type TreeNavigationResult = internalext.TreeNavigationResult

// Skill represents a loaded skill file with parsed YAML frontmatter.
type Skill = internalext.Skill

// SkillLoadResult reports skills loaded from a directory.
type SkillLoadResult = internalext.SkillLoadResult

// PromptTemplate represents a parsed template with variable placeholders.
type PromptTemplate = internalext.PromptTemplate

// ArgumentPattern defines how to parse command arguments.
type ArgumentPattern = internalext.ArgumentPattern

// ParseResult reports argument parsing outcome.
type ParseResult = internalext.ParseResult

// ModelConditional represents an <if-model> block for evaluation.
type ModelConditional = internalext.ModelConditional

// ModelCapabilities describes what a model supports.
type ModelCapabilities = internalext.ModelCapabilities

// ModelPricing describes a model's token costs, expressed in US dollars per
// one million tokens (the unit used by the models.dev registry). Extensions
// use it to compute costs or savings that Kit does not report directly —
// for example the value of prompt-cache reads:
//
//	caps, _ := ctx.GetModelCapabilities("")
//	usage := ctx.GetSessionUsage()
//	if caps.Pricing.Known && caps.Pricing.HasCacheRead {
//	    saved := float64(usage.TotalCacheReadTokens) *
//	        (caps.Pricing.Input - caps.Pricing.CacheRead) / 1_000_000
//	}
//
// Costs for a raw token count are therefore tokens * rate / 1_000_000.
type ModelPricing = internalext.ModelPricing

// ModelResolutionResult reports model chain resolution outcome.
type ModelResolutionResult = internalext.ModelResolutionResult

// ExtensionEntry represents persisted extension data stored in the session.
// Extensions use AppendEntry to save custom state and GetEntries to retrieve
// it on session resume.
type ExtensionEntry = internalext.ExtensionEntry

// ContextMessage represents a single message in the LLM context window.
// Used by OnContextPrepare to let extensions inspect and modify the messages
// that will be sent to the LLM.
type ContextMessage = internalext.ContextMessage

// CompleteRequest configures a standalone LLM completion call. Extensions use
// this with ctx.Complete() to make direct LLM calls without the agent tool loop.
type CompleteRequest = internalext.CompleteRequest

// CompleteResponse contains the LLM response and usage metadata from a
// standalone completion call.
type CompleteResponse = internalext.CompleteResponse

// StatusBarEntry represents a keyed entry in the TUI status bar. Extensions
// can set multiple independent entries that render alongside the built-in
// model name and token usage display.
type StatusBarEntry = internalext.StatusBarEntry

// CompactConfig configures a programmatic context compaction request.
type CompactConfig = internalext.CompactConfig

// FilePart describes a file attachment for multimodal messages. Extensions
// use this with SendMultimodalMessage to attach images or documents.
type FilePart = internalext.FilePart

// SessionUsage contains aggregated token usage and cost statistics for
// the current session. Extensions use this with GetSessionUsage() to
// report usage information.
type SessionUsage = internalext.SessionUsage

// PrintBlockOpts configures a custom styled block for PrintBlock.
type PrintBlockOpts = internalext.PrintBlockOpts

// API is passed to each extension's Init function. Extensions use it to
// register typed event handlers, custom tools, and slash commands.
type API = internalext.API

// WidgetPlacement determines where a widget appears in the TUI layout
// relative to the input area.
type WidgetPlacement = internalext.WidgetPlacement

// WidgetAbove places the widget above the input area, between the
// separator and queued messages.
const WidgetAbove = internalext.WidgetAbove

// WidgetBelow places the widget below the input area, between the
// input and the status bar.
const WidgetBelow = internalext.WidgetBelow

// WidgetContent describes what to render in a widget slot.
//
// Content is resolved in priority order: Render (if non-nil), then Text.
// Render gives the extension the terminal width and takes whatever string it
// returns verbatim, so it can emit arbitrary ANSI, box drawing, sparklines,
// or per-frame animation. Text is the simple declarative form.
type WidgetContent = internalext.WidgetContent

// WidgetStyle configures the visual appearance of a widget.
type WidgetStyle = internalext.WidgetStyle

// WidgetConfig fully describes a widget for placement in the TUI.
// Extensions identify widgets by ID; calling SetWidget with the same ID
// replaces the previous widget. IDs should be descriptive to avoid
// collisions across extensions (e.g. "myext:token-counter").
type WidgetConfig = internalext.WidgetConfig

// PromptSelectConfig configures a selection prompt that presents the user
// with a list of options to choose from.
type PromptSelectConfig = internalext.PromptSelectConfig

// PromptSelectResult is the response from a selection prompt.
type PromptSelectResult = internalext.PromptSelectResult

// PromptConfirmConfig configures a yes/no confirmation prompt.
type PromptConfirmConfig = internalext.PromptConfirmConfig

// PromptConfirmResult is the response from a confirmation prompt.
type PromptConfirmResult = internalext.PromptConfirmResult

// PromptInputConfig configures a free-form text input prompt.
type PromptInputConfig = internalext.PromptInputConfig

// PromptInputResult is the response from a text input prompt.
type PromptInputResult = internalext.PromptInputResult

// PromptMultiSelectConfig configures a multi-selection prompt that allows
// the user to toggle multiple options and confirm their selection.
type PromptMultiSelectConfig = internalext.PromptMultiSelectConfig

// PromptMultiSelectResult is the response from a multi-selection prompt.
type PromptMultiSelectResult = internalext.PromptMultiSelectResult

// HeaderFooterConfig describes a custom header or footer region that replaces
// or augments the default TUI chrome. Extensions use ctx.SetHeader/SetFooter
// to place one; only one header and one footer can be active at a time (the
// latest call wins). Reuses WidgetContent and WidgetStyle for consistency.
type HeaderFooterConfig = internalext.HeaderFooterConfig

// UIVisibility controls which built-in TUI chrome elements are visible.
// The zero value shows everything (backward compatible). Extensions call
// ctx.SetUIVisibility to customise the layout — for example, a "minimal"
// theme can hide the startup banner, status bar, and input hint and replace
// them with a single custom footer.
type UIVisibility = internalext.UIVisibility

// ContextStats contains current context-window usage information.
// Extensions can poll this via ctx.GetContextStats() to build usage
// meters, auto-compaction triggers, etc.
type ContextStats = internalext.ContextStats

// OverlayAnchor determines the vertical position of an overlay dialog
// within the TUI view.
type OverlayAnchor = internalext.OverlayAnchor

// OverlayCenter positions the dialog in the vertical center.
const OverlayCenter = internalext.OverlayCenter

// OverlayTopCenter positions the dialog near the top of the view.
const OverlayTopCenter = internalext.OverlayTopCenter

// OverlayBottomCenter positions the dialog near the bottom of the view.
const OverlayBottomCenter = internalext.OverlayBottomCenter

// OverlayStyle configures the visual appearance of an overlay dialog.
type OverlayStyle = internalext.OverlayStyle

// OverlayConfig fully describes a modal overlay dialog. Extensions call
// ctx.ShowOverlay(config) to display the dialog and block until the user
// dismisses it or selects an action. The dialog renders as a bordered box
// positioned within the TUI, with optional scrollable content and action
// buttons.
//
// Example:
//
//	result := ctx.ShowOverlay(ext.OverlayConfig{
//	    Title:   "Build Results",
//	    Content: ext.WidgetContent{Text: "All 42 tests passed."},
//	    Style:   ext.OverlayStyle{BorderColor: "#a6e3a1"},
//	    Width:   60,
//	    Actions: []string{"Continue", "Show Details"},
//	})
type OverlayConfig = internalext.OverlayConfig

// OverlayResult is the response from a ShowOverlay call.
type OverlayResult = internalext.OverlayResult

// ModelInfoEntry represents a known model from the registry. Used by
// GetAvailableModels to let extensions discover which models are available.
type ModelInfoEntry = internalext.ModelInfoEntry

// ToolInfo provides read-only information about a tool available to the agent.
// Used by GetAllTools to let extensions inspect and filter the tool set.
type ToolInfo = internalext.ToolInfo

// ToolContext provides runtime context to a tool's ExecuteWithContext handler.
// It allows tools to check for cancellation and report progress while running.
type ToolContext = internalext.ToolContext

// ToolDef describes a custom tool registered by an extension.
type ToolDef = internalext.ToolDef

// CommandDef describes a slash command registered by an extension.
type CommandDef = internalext.CommandDef

// ShortcutDef describes a global keyboard shortcut registered by an extension.
// Shortcuts fire across all app states except modal prompts/overlays.
// Use modifier combinations (e.g., "ctrl+p", "alt+t", "f1") — avoid bare
// characters like "a" or "x" which conflict with text input.
type ShortcutDef = internalext.ShortcutDef

// MessageRendererConfig provides a named rendering function that extensions
// can invoke via ctx.RenderMessage(name, content). Unlike tool renderers
// (which hook into the automatic tool result display), message renderers are
// invoked explicitly by extension code for branded status updates, progress
// reports, or any custom visual output.
//
// Example:
//
//	api.RegisterMessageRenderer(ext.MessageRendererConfig{
//	    Name: "build-status",
//	    Render: func(content string, width int) string {
//	        border := strings.Repeat("─", width-4)
//	        return "╭" + border + "╮\n│ " + content + "\n╰" + border + "╯"
//	    },
//	})
type MessageRendererConfig = internalext.MessageRendererConfig

// OptionDef describes a configuration option that an extension can register.
// Options are resolved from env vars, config file, or default value.
type OptionDef = internalext.OptionDef

// ToolRenderConfig provides custom rendering functions for a tool's display
// in the TUI. Extensions register tool renderers via API.RegisterToolRenderer()
// during Init. Both render functions are optional — if nil or if they return
// an empty string, the builtin renderer (or default) is used as a fallback.
//
// Example:
//
//	api.RegisterToolRenderer(ext.ToolRenderConfig{
//	    ToolName: "my-tool",
//	    RenderHeader: func(toolArgs string, width int) string {
//	        // Parse args and return a compact summary for the header
//	        return "my-tool: doing something"
//	    },
//	    RenderBody: func(toolResult string, isError bool, width int) string {
//	        // Return custom formatted result body
//	        if isError {
//	            return "ERROR: " + toolResult
//	        }
//	        return "Result: " + toolResult
//	    },
//	})
type ToolRenderConfig = internalext.ToolRenderConfig

// EditorKeyActionType defines the outcome of an editor key interception.
type EditorKeyActionType = internalext.EditorKeyActionType

// EditorKeyPassthrough lets the built-in editor handle the key normally.
const EditorKeyPassthrough = internalext.EditorKeyPassthrough

// EditorKeyConsumed means the extension handled the key. The editor
// should re-render but not process the key further.
const EditorKeyConsumed = internalext.EditorKeyConsumed

// EditorKeyRemap transforms the key into a different key before passing
// it to the built-in editor. Use RemappedKey to specify the target
// (e.g., "left", "right", "up", "down", "backspace", "delete", "enter",
// "tab", "home", "end", or a single character like "a").
const EditorKeyRemap = internalext.EditorKeyRemap

// EditorKeySubmit forces immediate text submission. The SubmitText field
// specifies the text to submit (empty = use editor's current text).
const EditorKeySubmit = internalext.EditorKeySubmit

// EditorKeyAction is returned by an editor interceptor's HandleKey function
// to indicate how a key press should be handled.
type EditorKeyAction = internalext.EditorKeyAction

// EditorConfig defines an editor interceptor/decorator that wraps the built-in
// input editor. Extensions can intercept key events (remap, consume, or force
// submit) and/or modify the rendered output (add mode indicators, apply visual
// effects).
//
// Uses concrete function fields instead of interfaces for Yaegi safety.
//
// IMPORTANT (Yaegi limitation — forward references lose their bodies):
// If you reference a function BY NAME from inside a function literal, and
// that function is declared LATER in the file, Yaegi builds the wrapper
// before the function is compiled. The field then holds a callable that does
// nothing and returns zero values — no panic, no error. This affects every
// Kit API taking a function, and also direct arguments such as
// api.OnToolCall(fn) and api.RegisterShortcut(def, fn).
//
//	// BROKEN — myHandler is declared below this closure:
//	api.OnAgentStart(func(e ext.AgentStartEvent, ctx ext.Context) {
//	    ctx.SetEditor(ext.EditorConfig{HandleKey: myHandler})
//	})
//	func myHandler(k, t string) ext.EditorKeyAction { ... }
//
// Any one of these fixes it:
//
//  1. Declare helpers ABOVE the code that references them (simplest).
//  2. Wrap in an anonymous closure:
//     HandleKey: func(k string, t string) ext.EditorKeyAction { return myHandler(k, t) }
//  3. Assign to a local first: f := myHandler; ...{HandleKey: f}
//
// Verified against yaegi v0.12.0-v0.16.1; not fixed upstream.
type EditorConfig = internalext.EditorConfig

// ToolCallEvent fires before a tool executes.
type ToolCallEvent = internalext.ToolCallEvent

// ToolCallResult controls whether the tool call proceeds.
type ToolCallResult = internalext.ToolCallResult

// ToolCallInputStartEvent fires when the LLM begins generating tool call
// arguments. The tool name is known but the full argument JSON is still
// being streamed.
type ToolCallInputStartEvent = internalext.ToolCallInputStartEvent

// ToolCallInputDeltaEvent fires for each streamed fragment of tool call
// arguments as they arrive from the LLM.
type ToolCallInputDeltaEvent = internalext.ToolCallInputDeltaEvent

// ToolCallInputEndEvent fires when tool argument streaming is complete,
// before the tool call is parsed and execution begins.
type ToolCallInputEndEvent = internalext.ToolCallInputEndEvent

// ToolExecutionStartEvent fires when a tool begins executing.
type ToolExecutionStartEvent = internalext.ToolExecutionStartEvent

// ToolExecutionEndEvent fires when a tool finishes executing.
type ToolExecutionEndEvent = internalext.ToolExecutionEndEvent

// ToolOutputEvent fires when a tool produces streaming output chunks.
// This is primarily used for long-running tools like the shell tool to show output
// in real-time as it arrives, before the tool completes.
type ToolOutputEvent = internalext.ToolOutputEvent

// ToolResultEvent fires after tool execution with the output.
type ToolResultEvent = internalext.ToolResultEvent

// ToolResultResult can modify the tool's output before it reaches the LLM.
type ToolResultResult = internalext.ToolResultResult

// InputEvent fires when user input is received.
type InputEvent = internalext.InputEvent

// InputResult controls what happens with user input.
//
//	Action: "continue" (default), "transform", "handled"
type InputResult = internalext.InputResult

// BeforeAgentStartEvent fires before the agent loop begins.
type BeforeAgentStartEvent = internalext.BeforeAgentStartEvent

// BeforeAgentStartResult can inject context before the agent runs.
type BeforeAgentStartResult = internalext.BeforeAgentStartResult

// AgentStartEvent fires when the agent loop begins.
type AgentStartEvent = internalext.AgentStartEvent

// AgentEndEvent fires when the agent finishes responding. In addition to the
// final response and stop reason, the event carries per-turn aggregates so
// observer-style extensions don't have to maintain parallel bookkeeping in
// OnToolResult / OnStepFinish handlers.
type AgentEndEvent = internalext.AgentEndEvent

// MessageStartEvent fires when a new assistant message begins.
type MessageStartEvent = internalext.MessageStartEvent

// MessageUpdateEvent fires for each streaming text chunk.
type MessageUpdateEvent = internalext.MessageUpdateEvent

// MessageEndEvent fires when the assistant message is complete.
type MessageEndEvent = internalext.MessageEndEvent

// SessionStartEvent fires when a session is loaded or created.
type SessionStartEvent = internalext.SessionStartEvent

// SessionShutdownEvent fires when the application is closing.
type SessionShutdownEvent = internalext.SessionShutdownEvent

// ModelChangeEvent fires after the active model is changed via ctx.SetModel().
type ModelChangeEvent = internalext.ModelChangeEvent

// ThinkingLevelChangeEvent fires after the extended-thinking effort level
// changes. Handlers that display the level should re-read it here rather than
// caching a value from session start.
type ThinkingLevelChangeEvent = internalext.ThinkingLevelChangeEvent

// TerminalResizeEvent fires when the terminal is resized, and once at startup
// with the initial dimensions so a handler can lay out without waiting for the
// user to resize. Interactive TUI only.
//
// The same values are available through Context.GetTerminalSize();
// this event exists so extensions can re-render chrome immediately instead of
// polling.
type TerminalResizeEvent = internalext.TerminalResizeEvent

// TurnStateChangeEvent fires when the UI enters or leaves its working state.
//
// This is a superset of AgentStart/AgentEnd: it also covers work that never
// reaches the agent loop (shell commands run with "!"), and it fires on every
// path back to idle, including cancellation and error. Use it for UI that must
// track whether Kit is busy — a spinner or a turn timer — and use
// AgentStart/AgentEnd when you specifically care about agent turns and their
// token usage.
//
// Interactive TUI only.
type TurnStateChangeEvent = internalext.TurnStateChangeEvent

// ContextPrepareEvent fires after the context window is built from the session
// tree and before the messages are sent to the LLM. Handlers can inspect the
// messages and return a modified set to filter, reorder, or inject context.
type ContextPrepareEvent = internalext.ContextPrepareEvent

// ContextPrepareResult allows extensions to replace the context window.
// Return nil to leave the context unchanged.
type ContextPrepareResult = internalext.ContextPrepareResult

// BeforeForkEvent fires before the session tree is branched to a different
// entry point (via the tree selector or /fork command).
type BeforeForkEvent = internalext.BeforeForkEvent

// BeforeForkResult controls whether the fork proceeds. Return Cancel=true
// with an optional Reason to block the fork.
type BeforeForkResult = internalext.BeforeForkResult

// BeforeSessionSwitchEvent fires before the session is switched to a new
// branch (e.g. /new or /clear commands).
type BeforeSessionSwitchEvent = internalext.BeforeSessionSwitchEvent

// BeforeSessionSwitchResult controls whether the session switch proceeds.
// Return Cancel=true with an optional Reason to block the switch.
type BeforeSessionSwitchResult = internalext.BeforeSessionSwitchResult

// BeforeCompactEvent fires before context compaction runs. Provides
// information about the current context state to help extensions decide
// whether to allow or block compaction.
type BeforeCompactEvent = internalext.BeforeCompactEvent

// BeforeCompactResult controls whether compaction proceeds. Return
// Cancel=true with an optional Reason to block compaction, or provide
// a custom Summary to replace the default LLM-generated one.
type BeforeCompactResult = internalext.BeforeCompactResult

// SubagentStartEvent fires when a subagent tool call begins executing.
type SubagentStartEvent = internalext.SubagentStartEvent

// SubagentChunkEvent fires for each real-time event from a running subagent.
// Type field indicates the kind of event; read the relevant fields accordingly.
type SubagentChunkEvent = internalext.SubagentChunkEvent

// SubagentEndEvent fires when a subagent tool call completes.
type SubagentEndEvent = internalext.SubagentEndEvent

// StepStartEvent fires when a new LLM call begins within a multi-step agent turn.
type StepStartEvent = internalext.StepStartEvent

// StepFinishEvent fires when a step completes, providing step metadata and
// token usage. Usage fields are plain int64 (not LLMUsage) because Yaegi
// cannot handle fantasy types across the interpreter boundary.
type StepFinishEvent = internalext.StepFinishEvent

// ReasoningStartEvent fires when the LLM begins reasoning/thinking.
type ReasoningStartEvent = internalext.ReasoningStartEvent

// WarningsEvent fires when the LLM provider returns warnings about the request.
type WarningsEvent = internalext.WarningsEvent

// SourceEvent fires when the LLM references a source (e.g. from web search).
type SourceEvent = internalext.SourceEvent

// ErrorEvent fires when an agent-level error occurs during streaming.
// Uses string instead of error because Yaegi cannot handle the error
// interface reliably across the interpreter boundary.
type ErrorEvent = internalext.ErrorEvent

// RetryEvent fires when the LLM provider request is retried after a
// transient error.
type RetryEvent = internalext.RetryEvent

// PrepareStepEvent fires between steps within a multi-step agent turn,
// after steering messages are injected and before messages are sent to
// the LLM. Handlers can inspect and replace the context window.
type PrepareStepEvent = internalext.PrepareStepEvent

// PrepareStepResult allows extensions to replace the context window between
// steps. Return nil Messages to leave the context unchanged.
type PrepareStepResult = internalext.PrepareStepResult

// LLMUsageEvent fires after each LLM provider call with the per-call token
// and cost deltas. Use this for accurate budget tracking, cost dashboards,
// and any logic that needs to react between LLM calls within a single agent
// turn (rather than only at turn boundaries).
//
// A single agent turn typically produces multiple LLMUsageEvents (one per
// tool-loop iteration). The Model and Provider fields reflect the model used
// for that specific call, which may differ from earlier calls if the
// extension switched models mid-turn via ctx.SetModel().
type LLMUsageEvent = internalext.LLMUsageEvent

// ThemeColors holds the active theme's colors as "#rrggbb" hex strings, with
// the light/dark variants already resolved for the terminal's appearance.
// Returned by ctx.GetTheme().
//
// It is the read counterpart to ThemeColorConfig: that type describes a theme
// being registered (light and dark for each slot), this one reports the colors
// actually in effect right now.
type ThemeColors = internalext.ThemeColors

// ThemeColor is an adaptive color pair with light and dark hex values.
// Either field may be empty to inherit from the default theme.
type ThemeColor = internalext.ThemeColor

// ThemeColorConfig defines a complete color theme that extensions can register
// programmatically via ctx.RegisterTheme(). Uses plain hex strings (not
// color.Color) so the type is safe to pass across the Yaegi boundary.
type ThemeColorConfig = internalext.ThemeColorConfig

// ---------------------------------------------------------------------------
// Aliases from internal/extensions/runner.go
// ---------------------------------------------------------------------------

// Runner manages loaded extensions and dispatches events to their handlers
// sequentially. Handlers execute in extension
// load order; for cancellable events the first blocking result wins.
//
// Each extension has a dedicated reentrant mutex so that handlers for the
// same extension are serialized (preventing data races on shared package-level
// state), while handlers for different extensions may execute concurrently.
type Runner = internalext.Runner

// LoadedExtension represents a single extension that has been discovered,
// loaded, and initialised. It holds the registered handlers and any custom
// tools, commands, or tool renderers the extension provided.
type LoadedExtension = internalext.LoadedExtension

// ---------------------------------------------------------------------------
// Aliases from internal/extensions/subagent.go
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// Subagent types
// ---------------------------------------------------------------------------
// SubagentConfig configures a subagent spawn.
type SubagentConfig = internalext.SubagentConfig

// SubagentEvent carries a real-time event from a running subagent. Extensions
// use the Type field to determine what happened and read the relevant fields.
// This is a concrete struct (not an interface) for Yaegi compatibility.
type SubagentEvent = internalext.SubagentEvent

// SubagentResult contains the outcome of a subagent execution.
type SubagentResult = internalext.SubagentResult

// SubagentUsage contains token usage from the subagent's run.
type SubagentUsage = internalext.SubagentUsage

// SubagentHandle provides control over a background (non-blocking)
// subagent. The subagent runs in-process in a goroutine; the handle
// exposes cancellation and completion signalling.
type SubagentHandle = internalext.SubagentHandle

// ---------------------------------------------------------------------------
// Helper functions
// ---------------------------------------------------------------------------

// AllEventTypes returns every supported event type.
func AllEventTypes() []EventType {
	return internalext.AllEventTypes()
}

// NormalizeShortcutKey converts a shortcut binding into the canonical spelling
// Kit matches against. Modifier aliases are folded, modifiers are reordered
// into canonical order, duplicates are dropped, and known key names are
// normalized. Bindings containing an unrecognized modifier are returned
// trimmed but otherwise untouched, so an unusual-but-valid terminal key name
// still has a chance to match.
func NormalizeShortcutKey(key string) string {
	return internalext.NormalizeShortcutKey(key)
}

// ValidateShortcutKey normalizes a binding and reports whether Kit can deliver
// it. A non-nil error means the shortcut must be rejected: the handler could
// never fire. A non-empty warning means the shortcut will fire but shadows
// built-in behaviour.
func ValidateShortcutKey(key string) (normalized string, warning string, err error) {
	return internalext.ValidateShortcutKey(key)
}
