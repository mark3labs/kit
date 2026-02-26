package ui

// submitMsg is sent by the InputComponent when the user submits a text prompt.
// The parent model receives this and calls app.Run(Text) to start agent processing.
type submitMsg struct {
	// Text is the user's input text to send to the agent.
	Text string
}

// cancelTimerExpiredMsg is sent by the tea.Tick command that starts when the user
// presses ESC once during stateWorking. If this message arrives before the user
// presses ESC a second time, the canceling state is reset to false.
type cancelTimerExpiredMsg struct{}

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
