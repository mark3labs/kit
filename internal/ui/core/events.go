package core

// ImageAttachment holds a clipboard image that will be sent alongside the
// user's text prompt to the LLM. The data is raw image bytes; MediaType is
// a MIME type like "image/png".
type ImageAttachment struct {
	// Data is the raw image bytes (PNG, JPEG, etc.).
	Data []byte
	// MediaType is the MIME type (e.g. "image/png", "image/jpeg").
	MediaType string
}

// SubmitMsg is sent by the InputComponent when the user submits a text prompt.
// The parent model receives this and calls app.Run(Text) to start agent processing.
type SubmitMsg struct {
	// Text is the user's input text to send to the agent.
	Text string
	// Images holds clipboard image attachments to send alongside the text.
	// Empty when no images are attached.
	Images []ImageAttachment
}

// CancelTimerExpiredMsg is sent by the tea.Tick command that starts when the user
// presses ESC once during stateWorking. If this message arrives before the user
// presses ESC a second time, the canceling state is reset to false.
type CancelTimerExpiredMsg struct{}

// --- Tree session events ---

// TreeNodeSelectedMsg is sent when the user selects a node in the tree selector.
type TreeNodeSelectedMsg struct {
	// ID is the entry ID of the selected node.
	ID string
	// Entry is the underlying entry object.
	Entry any
	// IsUser is true if the selected entry is a user message.
	IsUser bool
	// UserText is the user message text (only set when IsUser is true).
	UserText string
}

// TreeCancelledMsg is sent when the user cancels the tree selector (ESC).
type TreeCancelledMsg struct{}

// ShellCommandMsg is sent by the InputComponent when the user submits a
// ! or !! prefixed command. The parent model intercepts this to execute
// the shell command directly instead of forwarding to the LLM.
//
// Matching pi's behavior:
//   - !cmd  → run shell command, output INCLUDED in LLM context
//   - !!cmd → run shell command, output EXCLUDED from LLM context
type ShellCommandMsg struct {
	// Command is the shell command to execute (prefix stripped).
	Command string
	// ExcludeFromContext is true for !! (output excluded from LLM context),
	// false for ! (output included in LLM context).
	ExcludeFromContext bool
}

// ShellCommandResultMsg carries the result of a shell command execution
// back to the parent model for display.
type ShellCommandResultMsg struct {
	// Command is the original shell command that was executed.
	Command string
	// Output is the combined stdout/stderr output.
	Output string
	// ExitCode is the process exit code (0 = success).
	ExitCode int
	// Err is non-nil if the command failed to start or timed out.
	Err error
	// ExcludeFromContext mirrors the flag from ShellCommandMsg.
	ExcludeFromContext bool
}
