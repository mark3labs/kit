// Package extensions re-exports the extension-facing types from Kit's
// internal extension system so external consumers can construct events,
// inspect results, and call the public test harness without importing
// internal packages.
//
// Type aliases (not new types) keep identity with the internal definitions,
// so values remain interchangeable with the runtime and with
// pkg/extensions/test.
package extensions

import internalext "github.com/mark3labs/kit/internal/extensions"

// ==== Core interfaces and runner types ====

// Event is the interface satisfied by every lifecycle event.
type Event = internalext.Event

// Result is the interface satisfied by every event handler result.
type Result = internalext.Result

// EventType identifies a point in Kit's lifecycle where extensions can hook in.
type EventType = internalext.EventType

// LoadedExtension is an extension that has been loaded and initialised.
type LoadedExtension = internalext.LoadedExtension

// Runner dispatches events to loaded extensions.
type Runner = internalext.Runner

// ToolDef describes a tool registered by an extension.
type ToolDef = internalext.ToolDef

// CommandDef describes a slash command registered by an extension.
type CommandDef = internalext.CommandDef

// ==== Event type constants ====

const (
	ToolCall            = internalext.ToolCall
	ToolCallInputStart  = internalext.ToolCallInputStart
	ToolCallInputDelta  = internalext.ToolCallInputDelta
	ToolCallInputEnd    = internalext.ToolCallInputEnd
	ToolExecutionStart  = internalext.ToolExecutionStart
	ToolExecutionEnd    = internalext.ToolExecutionEnd
	ToolOutput          = internalext.ToolOutput
	ToolResult          = internalext.ToolResult
	Input               = internalext.Input
	BeforeAgentStart    = internalext.BeforeAgentStart
	AgentStart          = internalext.AgentStart
	AgentEnd            = internalext.AgentEnd
	MessageStart        = internalext.MessageStart
	MessageUpdate       = internalext.MessageUpdate
	MessageEnd          = internalext.MessageEnd
	SessionStart        = internalext.SessionStart
	SessionShutdown     = internalext.SessionShutdown
	ModelChange         = internalext.ModelChange
	ThinkingLevelChange = internalext.ThinkingLevelChange
	TerminalResize      = internalext.TerminalResize
	TurnStateChange     = internalext.TurnStateChange
	ContextPrepare      = internalext.ContextPrepare
	BeforeFork          = internalext.BeforeFork
	BeforeSessionSwitch = internalext.BeforeSessionSwitch
	BeforeCompact       = internalext.BeforeCompact
	SubagentStart       = internalext.SubagentStart
	SubagentChunk       = internalext.SubagentChunk
	SubagentEnd         = internalext.SubagentEnd
	StepStart           = internalext.StepStart
	StepFinish          = internalext.StepFinish
	ReasoningStart      = internalext.ReasoningStart
	Warnings            = internalext.Warnings
	Source              = internalext.Source
	Error               = internalext.Error
	Retry               = internalext.Retry
	PrepareStep         = internalext.PrepareStep
	LLMUsage            = internalext.LLMUsage
)

// AllEventTypes returns every supported event type.
func AllEventTypes() []EventType {
	return internalext.AllEventTypes()
}

// ==== Lifecycle events ====

// ToolCallEvent is emitted before a tool executes.
type ToolCallEvent = internalext.ToolCallEvent

// ToolCallInputStartEvent is emitted when tool-call argument streaming begins.
type ToolCallInputStartEvent = internalext.ToolCallInputStartEvent

// ToolCallInputDeltaEvent is emitted for each streamed tool-call argument fragment.
type ToolCallInputDeltaEvent = internalext.ToolCallInputDeltaEvent

// ToolCallInputEndEvent is emitted when tool-call argument streaming completes.
type ToolCallInputEndEvent = internalext.ToolCallInputEndEvent

// ToolExecutionStartEvent is emitted when a tool begins executing.
type ToolExecutionStartEvent = internalext.ToolExecutionStartEvent

// ToolExecutionEndEvent is emitted when a tool finishes executing.
type ToolExecutionEndEvent = internalext.ToolExecutionEndEvent

// ToolOutputEvent is emitted when a tool produces streaming output.
type ToolOutputEvent = internalext.ToolOutputEvent

// ToolResultEvent is emitted after a tool executes.
type ToolResultEvent = internalext.ToolResultEvent

// InputEvent is emitted when user input is received.
type InputEvent = internalext.InputEvent

// BeforeAgentStartEvent is emitted before the agent loop begins for a prompt.
type BeforeAgentStartEvent = internalext.BeforeAgentStartEvent

// AgentStartEvent is emitted when the agent loop begins processing.
type AgentStartEvent = internalext.AgentStartEvent

// AgentEndEvent is emitted when the agent finishes responding.
type AgentEndEvent = internalext.AgentEndEvent

// MessageStartEvent is emitted when a new assistant message begins.
type MessageStartEvent = internalext.MessageStartEvent

// MessageUpdateEvent is emitted for each streaming text chunk.
type MessageUpdateEvent = internalext.MessageUpdateEvent

// MessageEndEvent is emitted when the assistant message is complete.
type MessageEndEvent = internalext.MessageEndEvent

// SessionStartEvent is emitted when a session is loaded or created.
type SessionStartEvent = internalext.SessionStartEvent

// SessionShutdownEvent is emitted when the application is closing.
type SessionShutdownEvent = internalext.SessionShutdownEvent

// ModelChangeEvent is emitted after the active model changes.
type ModelChangeEvent = internalext.ModelChangeEvent

// ThinkingLevelChangeEvent is emitted after the thinking effort level changes.
type ThinkingLevelChangeEvent = internalext.ThinkingLevelChangeEvent

// TerminalResizeEvent is emitted when the terminal dimensions change.
type TerminalResizeEvent = internalext.TerminalResizeEvent

// TurnStateChangeEvent is emitted when the UI moves between idle and working.
type TurnStateChangeEvent = internalext.TurnStateChangeEvent

// ContextPrepareEvent is emitted before messages are sent to the LLM.
type ContextPrepareEvent = internalext.ContextPrepareEvent

// BeforeForkEvent is emitted before the session tree is branched.
type BeforeForkEvent = internalext.BeforeForkEvent

// BeforeSessionSwitchEvent is emitted before the session is switched.
type BeforeSessionSwitchEvent = internalext.BeforeSessionSwitchEvent

// BeforeCompactEvent is emitted before context compaction runs.
type BeforeCompactEvent = internalext.BeforeCompactEvent

// SubagentStartEvent is emitted when a subagent tool call begins.
type SubagentStartEvent = internalext.SubagentStartEvent

// SubagentChunkEvent is emitted for each real-time subagent event.
type SubagentChunkEvent = internalext.SubagentChunkEvent

// SubagentEndEvent is emitted when a subagent tool call completes.
type SubagentEndEvent = internalext.SubagentEndEvent

// StepStartEvent is emitted when a new LLM call begins within a turn.
type StepStartEvent = internalext.StepStartEvent

// StepFinishEvent is emitted when a step completes.
type StepFinishEvent = internalext.StepFinishEvent

// ReasoningStartEvent is emitted when the LLM begins reasoning.
type ReasoningStartEvent = internalext.ReasoningStartEvent

// WarningsEvent is emitted when the LLM provider returns warnings.
type WarningsEvent = internalext.WarningsEvent

// SourceEvent is emitted when the LLM references a source.
type SourceEvent = internalext.SourceEvent

// ErrorEvent is emitted when an agent-level error occurs during streaming.
type ErrorEvent = internalext.ErrorEvent

// RetryEvent is emitted when an LLM provider request is retried.
type RetryEvent = internalext.RetryEvent

// PrepareStepEvent is emitted between steps within a multi-step turn.
type PrepareStepEvent = internalext.PrepareStepEvent

// LLMUsageEvent is emitted after each LLM provider call with usage deltas.
type LLMUsageEvent = internalext.LLMUsageEvent

// ==== Handler results ====

// ToolCallResult is returned by tool-call handlers to block or allow execution.
type ToolCallResult = internalext.ToolCallResult

// ToolResultResult is returned by tool-result handlers to modify the result.
type ToolResultResult = internalext.ToolResultResult

// InputResult is returned by input handlers to handle or transform input.
type InputResult = internalext.InputResult

// BeforeAgentStartResult is returned by before-agent-start handlers.
type BeforeAgentStartResult = internalext.BeforeAgentStartResult

// ContextPrepareResult is returned by context-prepare handlers.
type ContextPrepareResult = internalext.ContextPrepareResult

// BeforeForkResult is returned by before-fork handlers.
type BeforeForkResult = internalext.BeforeForkResult

// BeforeSessionSwitchResult is returned by before-session-switch handlers.
type BeforeSessionSwitchResult = internalext.BeforeSessionSwitchResult

// BeforeCompactResult is returned by before-compact handlers.
type BeforeCompactResult = internalext.BeforeCompactResult

// PrepareStepResult is returned by prepare-step handlers.
type PrepareStepResult = internalext.PrepareStepResult

// ==== Context and mock-facing types ====

// Context provides runtime information and callbacks to extension handlers.
type Context = internalext.Context

// PrintBlockOpts configures a styled print block.
type PrintBlockOpts = internalext.PrintBlockOpts

// WidgetConfig describes a widget registered by an extension.
type WidgetConfig = internalext.WidgetConfig

// HeaderFooterConfig describes a header or footer registered by an extension.
type HeaderFooterConfig = internalext.HeaderFooterConfig

// UIVisibility controls which UI elements are visible.
type UIVisibility = internalext.UIVisibility

// StatusBarEntry describes a status bar entry.
type StatusBarEntry = internalext.StatusBarEntry

// EditorConfig configures editor behaviour overrides.
type EditorConfig = internalext.EditorConfig

// PromptSelectConfig configures a single-select prompt.
type PromptSelectConfig = internalext.PromptSelectConfig

// PromptSelectResult is the result of a single-select prompt.
type PromptSelectResult = internalext.PromptSelectResult

// PromptConfirmConfig configures a confirm prompt.
type PromptConfirmConfig = internalext.PromptConfirmConfig

// PromptConfirmResult is the result of a confirm prompt.
type PromptConfirmResult = internalext.PromptConfirmResult

// PromptInputConfig configures a text input prompt.
type PromptInputConfig = internalext.PromptInputConfig

// PromptInputResult is the result of a text input prompt.
type PromptInputResult = internalext.PromptInputResult

// PromptMultiSelectConfig configures a multi-select prompt.
type PromptMultiSelectConfig = internalext.PromptMultiSelectConfig

// PromptMultiSelectResult is the result of a multi-select prompt.
type PromptMultiSelectResult = internalext.PromptMultiSelectResult

// OverlayConfig describes an overlay shown by an extension.
type OverlayConfig = internalext.OverlayConfig

// OverlayResult is the result of showing an overlay.
type OverlayResult = internalext.OverlayResult

// ContextStats holds context window statistics.
type ContextStats = internalext.ContextStats

// SessionMessage is a single message in the session history.
type SessionMessage = internalext.SessionMessage

// ExtensionEntry is a custom data entry stored by an extension.
type ExtensionEntry = internalext.ExtensionEntry

// ToolInfo describes a tool available to the agent.
type ToolInfo = internalext.ToolInfo

// CompleteRequest is a request for an LLM completion from an extension.
type CompleteRequest = internalext.CompleteRequest

// CompleteResponse is the result of an LLM completion from an extension.
type CompleteResponse = internalext.CompleteResponse
