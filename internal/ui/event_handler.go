package ui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/mark3labs/kit/internal/app"
)

// CLIEventHandler routes app-layer events to CLI display methods for
// non-interactive modes (--prompt and script). It supports two display
// strategies depending on whether streaming is active:
//
// Streaming mode (StreamChunkEvents arrive):
//   - Chunks are printed directly to stdout as they arrive, giving the user
//     real-time feedback identical to the interactive TUI.
//   - At flush boundaries (tool calls, step completion) a trailing newline
//     is printed and the streamed flag prevents double-rendering.
//
// Non-streaming mode (no StreamChunkEvents):
//   - The complete response arrives via ResponseCompleteEvent or
//     StepCompleteEvent and is rendered through the formatted CLI display.
type CLIEventHandler struct {
	cli       *CLI
	modelName string

	spinner       *Spinner
	lastDisplayed string          // tracks content shown (non-streaming)
	streamBuf     strings.Builder // accumulated stream text (for lastDisplayed tracking)
	streaming     bool            // true once the first StreamChunkEvent arrives
}

// NewCLIEventHandler creates a handler that routes app events to the given CLI.
// modelName is shown in assistant message headers.
func NewCLIEventHandler(cli *CLI, modelName string) *CLIEventHandler {
	return &CLIEventHandler{cli: cli, modelName: modelName}
}

// Cleanup ensures any active spinner is stopped. Must be called after the
// agent step finishes (whether successfully or not).
func (h *CLIEventHandler) Cleanup() {
	if h.spinner != nil {
		h.spinner.Stop()
		h.spinner = nil
	}
}

func (h *CLIEventHandler) stopSpinner() {
	if h.spinner != nil {
		h.spinner.Stop()
		h.spinner = nil
	}
}

func (h *CLIEventHandler) startSpinner() {
	h.stopSpinner()
	h.spinner = NewSpinner()
	h.spinner.Start()
}

// endStream finishes a streaming block: prints a trailing newline, records
// what was displayed (for dedup), and resets the streaming state.
func (h *CLIEventHandler) endStream() {
	if !h.streaming {
		return
	}
	fmt.Println() // terminate the streamed line(s)
	h.lastDisplayed = strings.TrimSpace(h.streamBuf.String())
	h.streamBuf.Reset()
	h.streaming = false
}

// Handle processes a single app event and renders it via the CLI. This is
// the callback passed to app.RunOnceWithDisplay.
func (h *CLIEventHandler) Handle(msg tea.Msg) {
	switch e := msg.(type) {
	case app.SpinnerEvent:
		if e.Show {
			h.startSpinner()
		} else {
			h.stopSpinner()
		}

	case app.StreamChunkEvent:
		h.stopSpinner()
		// Print each chunk to stdout immediately so the user sees streaming
		// text in real-time, matching the interactive TUI experience.
		fmt.Print(e.Content)
		h.streamBuf.WriteString(e.Content)
		h.streaming = true

	case app.ToolCallContentEvent:
		// In streaming mode this text was already printed via StreamChunkEvents.
		// Only display when we haven't been streaming (non-streaming path).
		if !h.streaming {
			h.stopSpinner()
			_ = h.cli.DisplayAssistantMessageWithModel(e.Content, h.modelName)
			h.lastDisplayed = e.Content
			h.startSpinner()
		}

	case app.ToolCallStartedEvent:
		h.stopSpinner()
		// End any active stream before tool execution. The tool call itself
		// is NOT displayed here — a unified block (header + result) will be
		// rendered when the ToolResultEvent arrives.
		h.endStream()

	case app.ToolExecutionEvent:
		if e.IsStarting {
			h.startSpinner()
		} else {
			h.stopSpinner()
		}

	case app.ToolResultEvent:
		h.stopSpinner()
		h.cli.DisplayToolMessage(e.ToolName, e.ToolArgs, e.Result, e.IsError)
		h.startSpinner()

	case app.ResponseCompleteEvent:
		h.stopSpinner()
		// Non-streaming fallback: display the complete response.
		// In streaming mode the text was already printed chunk-by-chunk.
		if !h.streaming && e.Content != "" {
			_ = h.cli.DisplayAssistantMessageWithModel(e.Content, h.modelName)
			h.lastDisplayed = e.Content
		}

	case app.ExtensionPrintEvent:
		h.stopSpinner()
		switch e.Level {
		case "info":
			h.cli.DisplayInfo(e.Text)
		case "error":
			h.cli.DisplayError(fmt.Errorf("%s", e.Text))
		case "block":
			h.cli.DisplayExtensionBlock(e.Text, e.BorderColor, e.Subtitle)
		default:
			fmt.Println(e.Text)
		}

	case app.StepCompleteEvent:
		h.stopSpinner()

		// End any active stream.
		h.endStream()

		// Non-streaming fallback: render the full response if not already shown.
		responseText := ""
		if e.Response != nil {
			responseText = e.Response.Content.Text()
		}
		if responseText != "" && responseText != h.lastDisplayed {
			_ = h.cli.DisplayAssistantMessageWithModel(responseText, h.modelName)
		}

		// Display usage. The app layer has already updated the shared
		// UsageTracker before sending this event.
		h.cli.DisplayUsageAfterResponse()

		// Reset for next step in the agentic loop.
		h.lastDisplayed = ""
	}
}
