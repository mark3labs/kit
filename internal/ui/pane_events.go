package ui

import (
	"encoding/json"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/mark3labs/kit/internal/app"
	"github.com/mark3labs/kit/internal/core"
)

// applyPaneEvent applies one agent event to the shown pane: the scrollback,
// the streaming component and the activity row. The parent agent and each
// subagent run use this function, so their transcripts are made by the same
// code. It does not change the app state (working, canceling, the queue);
// the caller does that.
func (m *AppModel) applyPaneEvent(msg app.Event) tea.Cmd {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case app.SpinnerEvent:
		if m.stream != nil {
			updated, cmd := m.stream.Update(msg)
			m.stream, _ = updated.(streamComponentIface)
			cmds = append(cmds, cmd)
		}

	case app.ReasoningChunkEvent:
		// Forward to stream component for display rendering
		if m.stream != nil {
			updated, cmd := m.stream.Update(msg)
			m.stream, _ = updated.(streamComponentIface)
			cmds = append(cmds, cmd)
		}

		// Buffer for the coalesced ScrollList update (live display flushes
		// on the next streamAppendFlushMsg tick, ~60 fps).
		if cmd := m.bufferStreamChunk("reasoning", msg.Delta); cmd != nil {
			cmds = append(cmds, cmd)
		}

	case app.ReasoningCompleteEvent:
		// Forward to stream component to freeze reasoning duration
		if m.stream != nil {
			updated, cmd := m.stream.Update(msg)
			m.stream, _ = updated.(streamComponentIface)
			cmds = append(cmds, cmd)
		}

		// Mark the reasoning StreamingMessageItem as complete to freeze its counter
		if len(m.messages) > 0 {
			if streamMsg, ok := m.messages[len(m.messages)-1].(*StreamingMessageItem); ok && streamMsg.role == "reasoning" {
				streamMsg.MarkComplete()
			}
		}

	case app.StreamChunkEvent:
		// Forward to stream component for display rendering
		if m.stream != nil {
			updated, cmd := m.stream.Update(msg)
			m.stream, _ = updated.(streamComponentIface)
			cmds = append(cmds, cmd)
		}

		// Buffer for the coalesced ScrollList update (live display flushes
		// on the next streamAppendFlushMsg tick, ~60 fps).
		if cmd := m.bufferStreamChunk("assistant", msg.Content); cmd != nil {
			cmds = append(cmds, cmd)
		}

	case app.ToolCallStartedEvent:
		// Flush any accumulated streaming text to the ScrollList first (streaming
		// always completes before tool calls fire). The tool call itself is
		// NOT printed here — a unified block (header + result) will be
		// rendered when the ToolResultEvent arrives.
		m.flushStreamContent()

		// For shell commands, extract and store the command for the streaming
		// output header. The earlier tool name is accepted too, so a replayed
		// session keeps its header.
		if msg.ToolName == core.ShellToolName || msg.ToolName == core.LegacyShellToolName {
			var args struct {
				Command string `json:"command"`
			}
			if err := json.Unmarshal([]byte(msg.ToolArgs), &args); err == nil && args.Command != "" {
				m.streamingBashCommand = args.Command
			}
		}
		// Code mode streams its nested call log through the same item;
		// give it a header so the log is not anonymous.
		if msg.ToolName == core.CodeModeToolName {
			m.streamingBashCommand = "Running script"
		}

	case app.ToolExecutionEvent:
		// Pass to stream component for execution spinner display.
		if m.stream != nil {
			updated, cmd := m.stream.Update(msg)
			m.stream, _ = updated.(streamComponentIface)
			cmds = append(cmds, cmd)
		}

	case app.ToolResultEvent:
		// Remove streaming bash output item (if present) before adding the final tool result.
		// The tool result will contain the truncated output.
		if len(m.messages) > 0 {
			if _, ok := m.messages[len(m.messages)-1].(*StreamingBashOutputItem); ok {
				// Remove the streaming bash item
				m.messages = m.messages[:len(m.messages)-1]
			}
		}

		// Add the final tool result with truncated output.
		m.printToolResult(msg)

		// Clear legacy bash output state
		m.streamingBashOutput = nil
		m.streamingBashStderr = nil
		m.streamingBashCommand = ""

		// Start spinner again while waiting for the next LLM response.
		if m.stream != nil {
			updated, cmd := m.stream.Update(app.SpinnerEvent{Show: true})
			m.stream, _ = updated.(streamComponentIface)
			cmds = append(cmds, cmd)
		}

	case app.ToolOutputEvent:
		// Append bash output to streaming bash item in ScrollList.
		// Find or create the streaming bash output item.
		var bashItem *StreamingBashOutputItem
		if len(m.messages) > 0 {
			if item, ok := m.messages[len(m.messages)-1].(*StreamingBashOutputItem); ok {
				bashItem = item
			}
		}

		// Create new bash output item if needed
		if bashItem == nil {
			bashItem = NewStreamingBashOutputItem(generateMessageID(), m.streamingBashCommand)
			m.messages = append(m.messages, bashItem)
		}

		// Append the chunk
		if msg.IsStderr {
			bashItem.AppendStderr(msg.Chunk)
		} else {
			bashItem.AppendStdout(msg.Chunk)
		}
		// Invalidate cached height after mutation.
		if m.scrollList != nil {
			m.scrollList.InvalidateItemHeight(bashItem.ID())
		}

		// Check height and cap if needed - we don't want streaming output to grow forever
		const maxStreamingBashHeight = 20 // Max lines to show during streaming
		if bashItem.Height() > maxStreamingBashHeight {
			// Stop showing new output once we hit the limit
			// The final tool result will show truncated output
			return nil
		}

		// Refresh ScrollList (handles autoscroll internally)
		m.refreshContent()

	case app.RetryEvent:
		// Only pre-output retries reach the UI. Keep completed steps and
		// their messages; reset only the transient stream component.
		if m.stream != nil {
			m.stream.Reset()
			updated, cmd := m.stream.Update(app.SpinnerEvent{Show: true})
			m.stream, _ = updated.(streamComponentIface)
			cmds = append(cmds, cmd)
		}
		m.layoutDirty = true

	case app.StepStartedEvent:
		// Buffered chunks of the previous step were applied before this
		// handler ran, so items from this index on belong to this step.
		m.stepMessageMark = len(m.messages)

	case app.StepResumeEvent:
		// The failed step's output is discarded by the agent and is not
		// saved. Remove it from the display, so the restarted step does
		// not show the same text twice. Completed steps stay.
		m.discardStepStreamingItems()
		m.pendingStreamChunks = m.pendingStreamChunks[:0]
		if m.stream != nil {
			m.stream.Reset()
			updated, cmd := m.stream.Update(app.SpinnerEvent{Show: true})
			m.stream, _ = updated.(streamComponentIface)
			cmds = append(cmds, cmd)
		}
		m.printSystemMessage(stepResumeNotice(msg))
		m.layoutDirty = true

	case app.ResponseCompleteEvent:
		// This event fires for both streaming and non-streaming paths.
		// In streaming mode, mark the StreamingMessageItem as complete.
		// In non-streaming mode (no stream content accumulated), print the text.

		// Check if we have an active StreamingMessageItem
		hasStreamingItem := false
		if len(m.messages) > 0 {
			if streamMsg, ok := m.messages[len(m.messages)-1].(*StreamingMessageItem); ok {
				streamMsg.MarkComplete()
				hasStreamingItem = true
			}
		}

		// Reset stream component
		if m.stream != nil {
			m.stream.Reset()
		}

		// If no streaming item exists and we have content, print it as a regular message
		if !hasStreamingItem && strings.TrimSpace(msg.Content) != "" {
			m.printAssistantMessage(msg.Content)
		}

	case app.StepCompleteEvent:
		// Stop the spinner. The response is already
		// mirrored in the ScrollList as a StreamingMessageItem, so
		// finalizeStreamTurn() freezes it and drops the stream component's
		// redundant copy (see its doc comment).
		if m.stream != nil {
			updated, cmd := m.stream.Update(app.SpinnerEvent{Show: false})
			m.stream, _ = updated.(streamComponentIface)
			cmds = append(cmds, cmd)
		}
		m.finalizeStreamTurn()
		m.printTurnReceipt(turnDone)

	case app.StepCancelledEvent:
		// User cancelled the step (double-ESC). The partial response stays
		// visible via the ScrollList copy; the stream component's copy is
		// dropped so the next turn does not re-print it after the receipt.
		if m.stream != nil {
			updated, cmd := m.stream.Update(app.SpinnerEvent{Show: false})
			m.stream, _ = updated.(streamComponentIface)
			cmds = append(cmds, cmd)
		}
		if len(m.messages) > 0 {
			if item, ok := m.messages[len(m.messages)-1].(*StreamingMessageItem); ok && item.streaming {
				item.MarkIncomplete()
			}
		}
		m.finalizeStreamTurn()
		m.printTurnReceipt(turnCancelled)

	case app.StepErrorEvent:
		// The partial response stays visible via the ScrollList copy; the
		// error block is printed after it, in chronological order.
		if m.stream != nil {
			updated, cmd := m.stream.Update(app.SpinnerEvent{Show: false})
			m.stream, _ = updated.(streamComponentIface)
			cmds = append(cmds, cmd)
		}
		if len(m.messages) > 0 {
			if item, ok := m.messages[len(m.messages)-1].(*StreamingMessageItem); ok && item.streaming {
				item.MarkIncomplete()
			}
		}
		m.finalizeStreamTurn()
		if msg.Err != nil {
			m.printErrorResponse(msg)
		}
		// Close the turn with a failure receipt, consistent with the Done and
		// Interrupted receipts on the other terminal events. The error block
		// above says what went wrong; the receipt records the turn's shape
		// (tool count, elapsed) that the error itself omits.
		m.printTurnReceipt(turnFailed)

	}

	return tea.Batch(cmds...)
}
