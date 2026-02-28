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
type StepCompleteEvent struct {
	// ResponseText is the final assistant response text.
	ResponseText string
}

// StepErrorEvent is sent when an agent step fails with an error.
// The TUI should display the error inline and transition back to input state.
type StepErrorEvent struct {
	// Err is the error that caused the step to fail.
	Err error
}

// StepCancelledEvent is sent when an agent step is cancelled by the user
// (e.g. via double-ESC). The TUI should flush any partially streamed content,
// cut off the agent message where it was, and return to input state without
// displaying an error.
type StepCancelledEvent struct{}

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

// CompactCompleteEvent is sent when a /compact operation finishes successfully.
// It carries the summary text and before/after statistics.
type CompactCompleteEvent struct {
	// Summary is the LLM-generated structured summary of the compacted messages.
	Summary string
	// OriginalTokens is the estimated token count before compaction.
	OriginalTokens int
	// CompactedTokens is the estimated token count after compaction.
	CompactedTokens int
	// MessagesRemoved is the number of messages that were summarised away.
	MessagesRemoved int
}

// CompactErrorEvent is sent when a /compact operation fails.
type CompactErrorEvent struct {
	// Err is the error that caused compaction to fail.
	Err error
}

// WidgetUpdateEvent is sent when an extension adds, updates, or removes a
// widget via ctx.SetWidget or ctx.RemoveWidget. The TUI re-reads widget state
// from its WidgetProvider on the next render cycle.
type WidgetUpdateEvent struct{}

// ExtensionPrintEvent is sent when an extension calls ctx.Print, ctx.PrintInfo,
// ctx.PrintError, or ctx.PrintBlock. The TUI renders it via the appropriate
// renderer and tea.Println (scrollback); the CLI handler uses
// DisplayInfo/DisplayError or plain fmt.Println. This exists because BubbleTea
// captures stdout, so plain fmt.Println inside extensions would be swallowed.
type ExtensionPrintEvent struct {
	// Text is the content the extension wants to display to the user.
	Text string
	// Level controls the rendering style:
	//   ""      — plain text (no styling)
	//   "info"  — system message block (bordered, themed)
	//   "error" — error block (red border, bold text)
	//   "block" — custom block with BorderColor and Subtitle
	Level string
	// BorderColor is a hex color (e.g. "#a6e3a1") for Level="block".
	BorderColor string
	// Subtitle is optional muted text below the content for Level="block".
	Subtitle string
}

// PromptResponse carries the user's answer to an interactive prompt. The TUI
// sends exactly one PromptResponse through the channel embedded in
// PromptRequestEvent when the user completes or cancels the prompt.
type PromptResponse struct {
	// Value is the response text — the selected option (select), or the
	// entered text (input). Unused for confirm prompts.
	Value string
	// Index is the zero-based index of the selected option (select only).
	Index int
	// Confirmed is the boolean answer for confirm prompts.
	Confirmed bool
	// Cancelled is true if the user dismissed the prompt (ESC) or the
	// prompt could not be shown (e.g. app shutting down).
	Cancelled bool
}

// PromptRequestEvent is sent when an extension requests an interactive
// prompt from the user (select, confirm, or text input). The TUI enters a
// modal prompt state, renders the prompt, and sends a single PromptResponse
// through ResponseCh when the user completes or cancels.
//
// The extension goroutine blocks on the read side of ResponseCh until the
// TUI sends a response. The channel must have buffer size >= 1.
type PromptRequestEvent struct {
	// PromptType is "select", "confirm", or "input".
	PromptType string
	// Message is the question displayed to the user.
	Message string
	// Options lists the choices for select prompts.
	Options []string
	// Default is the pre-filled value: "true"/"false" for confirm prompts,
	// or the initial text for input prompts.
	Default string
	// Placeholder is the ghost text for input prompts.
	Placeholder string
	// ResponseCh receives the user's answer. The TUI must send exactly one
	// value. The channel must be buffered (cap >= 1) so sending never
	// blocks inside Update().
	ResponseCh chan<- PromptResponse
}

// OverlayResponse carries the user's answer to a modal overlay dialog. The
// TUI sends exactly one OverlayResponse through the channel embedded in
// OverlayRequestEvent when the user completes or cancels the overlay.
type OverlayResponse struct {
	// Action is the text of the selected action button, or "" if no actions
	// were configured or the dialog was dismissed without selection.
	Action string
	// Index is the zero-based index of the selected action, or -1 if no
	// action was selected.
	Index int
	// Cancelled is true if the user dismissed the overlay (ESC) or the
	// overlay could not be shown (e.g. non-interactive mode).
	Cancelled bool
}

// OverlayRequestEvent is sent when an extension requests a modal overlay
// dialog. The TUI enters an overlay state, renders the dialog, and sends a
// single OverlayResponse through ResponseCh when the user dismisses or
// selects an action.
//
// The extension goroutine blocks on the read side of ResponseCh until the
// TUI sends a response. The channel must have buffer size >= 1.
type OverlayRequestEvent struct {
	// Title is displayed at the top of the dialog. Empty means no title.
	Title string
	// Content is the text to render inside the dialog body.
	Content string
	// Markdown, when true, renders Content as styled markdown.
	Markdown bool
	// BorderColor is a hex color for the dialog border. Empty uses default.
	BorderColor string
	// Background is a hex color for the dialog background. Empty = none.
	Background string
	// Width is the dialog width in columns. 0 = auto (60% of terminal).
	Width int
	// MaxHeight limits dialog height. 0 = auto (80% of terminal).
	MaxHeight int
	// Anchor is the vertical positioning: "center", "top-center", "bottom-center".
	Anchor string
	// Actions lists the action button labels. Empty = simple dismiss dialog.
	Actions []string
	// ResponseCh receives the user's response. Must have buffer size >= 1.
	ResponseCh chan<- OverlayResponse
}
