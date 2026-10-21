package ui

import (
	"time"

	kit "github.com/mark3labs/kit/pkg/kit"
)

// conversationPane is the transcript and the live activity of one agent:
// the scrollback, the streaming component that animates the activity row,
// the buffered stream chunks and the live shell output.
//
// The parent agent has one pane and each subagent run has one. AppModel
// embeds a pointer to the pane on screen, so every transcript operation
// (printToolResult, appendStreamingChunk, renderActivityRow, ...) works on
// whichever agent is shown, and a subagent is displayed by the same code as
// the parent. Events for a pane that is not shown are applied with
// AppModel.withPane, which shows the pane for the duration of the call.
type conversationPane struct {
	// paneID is "" for the parent agent and the run ID for a subagent.
	paneID string

	// child holds the subagent run state. It is nil for the parent pane.
	child *childPaneState

	// stream is the child streaming display component (spinner + streaming text).
	stream streamComponentIface

	// renderer renders completed messages for ScrollList display.
	renderer Renderer

	// scrollList manages the in-memory message history with viewport scrolling.
	scrollList *ScrollList

	// messages holds all completed messages in the conversation history.
	// The scrollList renders from this slice based on its viewport offset.
	messages []MessageItem

	// pendingStreamChunks buffers streaming text/reasoning chunks between
	// coalescing flush ticks. Applying every chunk immediately forces a full
	// markdown re-render of the accumulated message per chunk (O(n²) over a
	// long response); instead chunks are batched and applied at most once
	// per coalescing window (16ms, widened for long messages — see
	// adaptiveStreamFlushInterval) or before any other event that
	// depends on the scrollback state. Consecutive same-role chunks are
	// concatenated in place.
	pendingStreamChunks []pendingStreamChunk

	// stepMessageMark is len(m.messages) when the current agent step
	// started. A step resume removes the streaming items that the failed
	// step added after this index.
	stepMessageMark int

	// streamFlushPending is true while a coalescing flush tick is scheduled,
	// preventing duplicate tick commands.
	streamFlushPending bool

	// turnStartedAt records when the current agent turn began, so the
	// activity row can show elapsed time. Zero when idle.
	turnStartedAt time.Time

	// turnToolCount counts tool calls completed during the current turn, for
	// the turn-completion receipt. Reset when a turn begins.
	turnToolCount int

	// streamingBashOutput holds the current streaming bash output lines.
	// Lines are accumulated as they arrive and displayed in the stream region.
	streamingBashOutput []string

	// streamingBashStderr holds stderr lines separately (rendered differently).
	streamingBashStderr []string

	// streamingBashMaxLines caps how many lines to accumulate to prevent memory issues.
	streamingBashMaxLines int

	// streaming bash fields are only mutated/read from the Bubble Tea event loop
	// (Update/View), so no mutex is required here.
	// streamingBashCommand holds the command being executed for display as a header.
	streamingBashCommand string
}

// childPaneState is the part of a pane that only a subagent run has.
type childPaneState struct {
	// run holds the metadata of the run (its Events are not kept).
	run kit.SubagentRun
	// seq is the Seq of the last run update applied (see kit.SubagentRun.Seq).
	seq uint64
	// working is true from the start of the run until it ends.
	working bool
}

// withPane runs fn with p shown in place of the current pane, then shows
// the previous pane again. A pane that fn changes while it is not on screen
// defers its scroll measurement until it is shown (see ScrollList.SetOffscreen).
func (m *AppModel) withPane(p *conversationPane, fn func()) {
	saved := m.conversationPane
	if p == saved {
		fn()
		return
	}
	m.conversationPane = p
	defer func() { m.conversationPane = saved }()
	fn()
}

// paneWorking reports whether the agent of the shown pane is working: the
// parent agent for the parent pane, the subagent run for a child pane.
func (m *AppModel) paneWorking() bool {
	if m.child != nil {
		return m.child.working
	}
	return m.agentWorking()
}

// paneModelName returns the model name to show on the assistant messages of
// the shown pane.
func (m *AppModel) paneModelName() string {
	if m.child != nil && m.child.run.Model != "" {
		return m.child.run.Model
	}
	return m.modelName
}
