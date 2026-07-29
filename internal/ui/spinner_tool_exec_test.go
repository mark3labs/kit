package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/mark3labs/kit/internal/app"
)

// newTestAppModelWithRealStream builds an AppModel wired to a REAL
// StreamComponent (not the stub) so the spinner rendering and its advance on
// the shared frame clock can be exercised end-to-end.
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

// advanceFrames drives n beats of the AppModel's shared animation clock.
//
// Ticks are synthesized at the clock's current generation rather than driven
// off the real timer, so a test that needs a second of animation does not take
// a second to run.
func advanceFrames(m *AppModel, n int) *AppModel {
	for range n {
		m, _ = sendMsgExec(m, frameTickMsg{
			generation: m.frames.generation,
			frame:      m.frames.frame + 1,
		})
	}
	return m
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
	// Live activity now renders in the activity row above the composer, not
	// in the ambient status bar.
	m.state = stateWorking
	activity := m.renderActivityRow()
	if !strings.Contains(activity, "Running") {
		t.Fatalf("expected activity row to show the bash command after text-then-tool flow, got: %q", activity)
	}

	// Simulate a LONG-running bash: drive several spinner ticks and confirm
	// the tool label persists and the frame keeps advancing (the shared clock
	// keeps beating for as long as the stream reports it is spinning).
	for i := range 5 {
		frameBefore := stream.spinnerFrame
		m = advanceFrames(m, frameEverySpinner)
		if stream.spinnerFrame != frameBefore+1 {
			t.Fatalf("tick %d: expected frame advance, got %d -> %d", i, frameBefore, stream.spinnerFrame)
		}
		if row := m.renderActivityRow(); !strings.Contains(row, "Running") {
			t.Fatalf("tick %d: activity row lost the bash phrase mid-execution, got: %q", i, row)
		}
	}
}

// TestSpinnerShowsToolNameDuringExecution drives the real StreamComponent
// through the exact event sequence the AppModel receives when the agent runs a
// bash tool, and asserts that the status bar visibly indicates the tool is
// running (spinner frame + tool name), and that the spinner keeps animating on
// the shared frame clock.
func TestSpinnerShowsToolNameDuringExecution(t *testing.T) {
	ctrl := &stubAppController{}
	m, stream := newTestAppModelWithRealStream(ctrl)

	// 1. Step starts: SpinnerEvent{Show: true} (app.go emits this).
	var cmd tea.Cmd
	m, cmd = sendMsgExec(m, app.SpinnerEvent{Show: true})
	if !stream.spinning {
		t.Fatalf("expected stream.spinning=true after SpinnerEvent{Show:true}")
	}

	// The activity row should already show the spinner frame.
	m.state = stateWorking
	activity := m.renderActivityRow()
	if !strings.Contains(activity, "▪") {
		t.Fatalf("expected spinner frame in activity row after step start, got: %q", activity)
	}

	// 2. Starting the spinner must have woken the shared clock: the component
	//    schedules nothing itself, so if the AppModel wrapper did not start the
	//    clock the dot would be frozen for the whole turn.
	tickMsgs := execCmds(t, cmd)
	if len(tickMsgs) == 0 {
		t.Fatalf("expected SpinnerEvent to wake the frame clock, got no cmd")
	}
	if _, ok := tickMsgs[0].(frameTickMsg); !ok {
		t.Fatalf("expected a frame clock beat, got %T", tickMsgs[0])
	}
	frameBefore := stream.spinnerFrame
	m = advanceFrames(m, frameEverySpinner)
	frameAfter := stream.spinnerFrame
	if frameAfter != frameBefore+1 {
		t.Fatalf("expected spinner frame to advance %d -> %d+1 on the shared clock, got %d",
			frameBefore, frameBefore, frameAfter)
	}

	// 3. Tool call parsed/starting: ToolCallStartedEvent then ToolExecutionEvent.
	//    (agent.go emits ToolCallEvent then ToolExecutionStartEvent back-to-back
	//    from fantasy's OnToolCall callback.)
	m, _ = sendMsgExec(m, app.ToolCallStartedEvent{
		ToolCallID: "call-1", ToolName: "bash", ToolArgs: `{"command":"ls"}`,
	})
	m, _ = sendMsgExec(m, app.ToolExecutionEvent{
		ToolCallID: "call-1", ToolName: "bash", ToolArgs: `{"command":"ls"}`, IsStarting: true,
	})

	// THE CORE ASSERTION: while bash is "running", the activity row must
	// visibly describe the work so the user knows the tool is executing.
	activity = m.renderActivityRow()
	if !strings.Contains(activity, "Running ls") {
		t.Fatalf("expected activity row to show the running command, got: %q", activity)
	}

	// 4. Simulate the clock beating mid-execution (bash takes time). The beat
	//    keeps the animation alive AND must preserve the tool label.
	m = advanceFrames(m, frameEverySpinner)
	m.state = stateWorking
	activity = m.renderActivityRow()
	if !strings.Contains(activity, "Running ls") {
		t.Fatalf("expected activity row to STILL show the command after a mid-execution tick, got: %q", activity)
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
