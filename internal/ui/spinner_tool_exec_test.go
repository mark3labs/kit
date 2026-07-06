package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/mark3labs/kit/internal/app"
)

// newTestAppModelWithRealStream builds an AppModel wired to a REAL
// StreamComponent (not the stub) so the spinner rendering and tick-loop
// forwarding through the AppModel's Update default-case can be exercised
// end-to-end.
func newTestAppModelWithRealStream(ctrl AppController) (*AppModel, *StreamComponent) {
	stream := NewStreamComponent(80, "test-model")
	input := &stubInputComponent{}
	m := &AppModel{
		state:                 stateInput,
		appCtrl:               ctrl,
		stream:                stream,
		input:                 input,
		renderer:              newMessageRenderer(80, false),
		modelName:             "test-model",
		providerName:          "testprov",
		width:                 80,
		height:                24,
		streamingBashMaxLines: 50,
		scrollList:            NewScrollList(80, 20),
		messages:              []MessageItem{},
	}
	return m, stream
}

// sendMsgExec is like sendMsg but also returns the tea.Cmd from Update so the
// caller can drive the cmd chain (e.g. spinner ticks) manually.
func sendMsgExec(m *AppModel, msg tea.Msg) (*AppModel, tea.Cmd) {
	updated, cmd := m.Update(msg)
	result := updated.(*AppModel)
	_ = result.View()
	return result, cmd
}

// execCmds runs a tea.Cmd (which may be a batch) and returns the messages it
// produces. tea.Cmd is func() tea.Msg; tea.Batch returns a func that yields the
// first non-nil msg from its sub-cmds — so we unwrap batches by re-invoking.
func execCmds(t *testing.T, cmd tea.Cmd) []tea.Msg {
	t.Helper()
	if cmd == nil {
		return nil
	}
	// A single (non-batch) cmd yields one msg.
	msg := cmd()
	if msg == nil {
		// Could be a batch that returns nil from its wrapper; nothing to do.
		return nil
	}
	return []tea.Msg{msg}
}

// TestSpinnerShowsToolName_TextThenTool covers the scenario where the model
// streams text BEFORE making a tool call. In this case ToolCallStartedEvent
// triggers flushStreamContent() which calls stream.Reset() (clearing
// activeTools + stopping the spinner). The immediately-following
// ToolExecutionEvent must re-add the tool and restart the spinner so the
// status bar still shows the tool name during execution.
func TestSpinnerShowsToolName_TextThenTool(t *testing.T) {
	ctrl := &stubAppController{}
	m, stream := newTestAppModelWithRealStream(ctrl)

	// Step start.
	var cmd tea.Cmd
	m, cmd = sendMsgExec(m, app.SpinnerEvent{Show: true})
	_ = cmd

	// Model streams some assistant text first.
	m, _ = sendMsgExec(m, app.StreamChunkEvent{Content: "Let me check the files."})

	// Tool call parsed: ToolCallStartedEvent flushes (and Resets) the stream.
	m, _ = sendMsgExec(m, app.ToolCallStartedEvent{
		ToolCallID: "call-1", ToolName: "bash", ToolArgs: `{"command":"sleep 2; echo done"}`,
	})

	// Immediately followed by ToolExecutionEvent{IsStarting:true}.
	// The returned cmd (a spinner tick) is intentionally discarded — the
	// long-running-bash simulation below synthesizes its own ticks at the
	// current generation so the test doesn't depend on cmd-chain plumbing.
	m, _ = sendMsgExec(m, app.ToolExecutionEvent{
		ToolCallID: "call-1", ToolName: "bash", ToolArgs: `{"command":"sleep 2; echo done"}`, IsStarting: true,
	})

	if !stream.spinning {
		t.Fatalf("expected spinner restarted after flushStreamContent Reset + ToolExecutionEvent")
	}
	statusBar := m.renderStatusBar()
	if !strings.Contains(statusBar, "bash") {
		t.Fatalf("expected status bar to show 'bash' after text-then-tool flow, got: %q", statusBar)
	}

	// Simulate a LONG-running bash: drive several spinner ticks and confirm
	// the tool label persists and the frame keeps advancing (the tick loop
	// survives via the AppModel default-case forwarding).
	gen := stream.spinnerGeneration
	for i := range 5 {
		frameBefore := stream.spinnerFrame
		m, _ = sendMsgExec(m, streamSpinnerTickMsg{generation: gen})
		if stream.spinnerFrame != frameBefore+1 {
			t.Fatalf("tick %d: expected frame advance, got %d -> %d", i, frameBefore, stream.spinnerFrame)
		}
		if sb := m.renderStatusBar(); !strings.Contains(sb, "bash") {
			t.Fatalf("tick %d: status bar lost 'bash' label mid-execution, got: %q", i, sb)
		}
	}
}

// TestSpinnerShowsToolNameDuringExecution drives the real StreamComponent
// through the exact event sequence the AppModel receives when the agent runs a
// bash tool, and asserts that the status bar visibly indicates the tool is
// running (spinner frame + tool name), and that the spinner keeps animating via
// tick messages forwarded through the AppModel default case.
func TestSpinnerShowsToolNameDuringExecution(t *testing.T) {
	ctrl := &stubAppController{}
	m, stream := newTestAppModelWithRealStream(ctrl)

	// 1. Step starts: SpinnerEvent{Show: true} (app.go emits this).
	var cmd tea.Cmd
	m, cmd = sendMsgExec(m, app.SpinnerEvent{Show: true})
	if !stream.spinning {
		t.Fatalf("expected stream.spinning=true after SpinnerEvent{Show:true}")
	}

	// The status bar should already show the spinner frame.
	statusBar := m.renderStatusBar()
	if !strings.Contains(statusBar, "▪") {
		t.Fatalf("expected spinner frame in status bar after step start, got: %q", statusBar)
	}

	// 2. Drive one spinner tick to confirm the tick loop is forwarded through
	//    the AppModel default case (this is the critical path: streamSpinnerTickMsg
	//    has no explicit case in AppModel.Update, so it MUST reach `default:`).
	tickMsgs := execCmds(t, cmd)
	if len(tickMsgs) == 0 {
		t.Fatalf("expected SpinnerEvent to return a tick cmd, got nil")
	}
	frameBefore := stream.spinnerFrame
	m, _ = sendMsgExec(m, tickMsgs[0])
	frameAfter := stream.spinnerFrame
	if frameAfter != frameBefore+1 {
		t.Fatalf("expected spinner frame to advance %d -> %d+1 via AppModel default-case forwarding, got %d",
			frameBefore, frameBefore, frameAfter)
	}

	// 3. Tool call parsed/starting: ToolCallStartedEvent then ToolExecutionEvent.
	//    (agent.go emits ToolCallEvent then ToolExecutionStartEvent back-to-back
	//    from fantasy's OnToolCall callback.)
	m, _ = sendMsgExec(m, app.ToolCallStartedEvent{
		ToolCallID: "call-1", ToolName: "bash", ToolArgs: `{"command":"ls"}`,
	})
	// Capture the tick cmd here so step 4 can drive a mid-execution tick.
	_, tickCmd := sendMsgExec(m, app.ToolExecutionEvent{
		ToolCallID: "call-1", ToolName: "bash", ToolArgs: `{"command":"ls"}`, IsStarting: true,
	})

	// THE CORE ASSERTION: while bash is "running", the status bar must visibly
	// show the tool name so the user has an indication the tool is executing.
	statusBar = m.renderStatusBar()
	if !strings.Contains(statusBar, "bash") {
		t.Fatalf("expected status bar to show 'bash' while tool is executing, got: %q", statusBar)
	}

	// 4. Simulate the spinner tick firing mid-execution (bash takes time).
	//    The tick keeps the animation alive AND must preserve the tool label.
	var midTick tea.Msg
	if tickCmd != nil {
		if msgs := execCmds(t, tickCmd); len(msgs) > 0 {
			midTick = msgs[0]
		}
	}
	if midTick == nil {
		// If ToolExecutionEvent returned no new tick (spinner already
		// running), synthesize a tick at the current generation to prove
		// forwarding works mid-execution.
		midTick = streamSpinnerTickMsg{generation: stream.spinnerGeneration}
	}
	m, _ = sendMsgExec(m, midTick)
	statusBar = m.renderStatusBar()
	if !strings.Contains(statusBar, "bash") {
		t.Fatalf("expected status bar to STILL show 'bash' after a mid-execution tick, got: %q", statusBar)
	}
	if !stream.spinning {
		t.Fatalf("expected spinner to still be spinning during tool execution")
	}

	// 5. Tool finishes: ToolExecutionEvent{IsStarting:false} removes the label.
	//    The returned model is irrelevant past this assertion, so discard it.
	_, _ = sendMsgExec(m, app.ToolExecutionEvent{
		ToolCallID: "call-1", ToolName: "bash", IsStarting: false,
	})
	if _, stillRunning := stream.activeTools["call-1"]; stillRunning {
		t.Fatalf("expected bash to be removed from activeTools after execution end")
	}
}
