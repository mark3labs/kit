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
