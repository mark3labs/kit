package app

import "charm.land/fantasy"

// StreamChunkEvent is sent by the app layer when a streaming text delta arrives
// from the LLM. Each chunk contains an incremental portion of the response.
type StreamChunkEvent struct {
	// Content is the incremental text delta from the streaming response.
	Content string
}

// ToolCallStartedEvent is sent when a tool call has been parsed and is about to execute.
// It carries the tool name and its arguments for display purposes.
type ToolCallStartedEvent struct {
	// ToolName is the name of the tool being called.
	ToolName string
	// ToolArgs is the JSON-encoded arguments for the tool call.
	ToolArgs string
}

// ToolExecutionEvent is sent when a tool starts or finishes executing.
// The IsStarting flag distinguishes between the start and end of execution.
type ToolExecutionEvent struct {
	// ToolName is the name of the tool being executed.
	ToolName string
	// IsStarting is true when execution is beginning, false when it is complete.
	IsStarting bool
}

// ToolResultEvent is sent after a tool execution completes with its result.
type ToolResultEvent struct {
	// ToolName is the name of the tool that was executed.
	ToolName string
	// ToolArgs is the JSON-encoded arguments that were passed to the tool.
	ToolArgs string
	// Result is the text output from the tool execution.
	Result string
	// IsError indicates whether the tool returned an error result.
	IsError bool
}

// ToolCallContentEvent is sent when a step includes text content alongside tool calls.
// This allows the TUI to display assistant commentary that accompanies tool usage.
type ToolCallContentEvent struct {
	// Content is the assistant text that accompanies one or more tool calls.
	Content string
}

// ResponseCompleteEvent is sent when the LLM produces a final (non-streaming) response.
// In streaming mode, this may be empty if all content was delivered via StreamChunkEvents.
type ResponseCompleteEvent struct {
	// Content is the complete final response text.
	Content string
}

// StepCompleteEvent is sent when an agent step finishes successfully.
// It includes the final response and aggregated usage statistics for the step.
type StepCompleteEvent struct {
	// Response is the final fantasy response from the completed step.
	Response *fantasy.Response
	// Usage contains aggregated token usage data for the step.
	Usage fantasy.Usage
}

// StepErrorEvent is sent when an agent step fails with an error.
// The TUI should display the error inline and transition back to input state.
type StepErrorEvent struct {
	// Err is the error that caused the step to fail.
	Err error
}

// QueueUpdatedEvent is sent whenever the message queue length changes.
// The TUI uses this to update the queue badge display.
type QueueUpdatedEvent struct {
	// Length is the current number of messages waiting in the queue.
	Length int
}

// SpinnerEvent is sent to show or hide the spinner animation in the stream component.
// The spinner is shown before the first streaming chunk arrives and hidden once
// content begins flowing or the step completes.
type SpinnerEvent struct {
	// Show is true to display the spinner and false to hide it.
	Show bool
}

// MessageCreatedEvent is sent when a new message is added to the message store.
// This allows the TUI to stay in sync with the conversation history.
type MessageCreatedEvent struct {
	// Message is the fantasy message that was added to the store.
	Message fantasy.Message
}
