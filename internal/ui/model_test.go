package ui

import (
	"errors"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/mark3labs/kit/internal/app"
	"github.com/mark3labs/kit/internal/session"
	kit "github.com/mark3labs/kit/pkg/kit"
)

// --------------------------------------------------------------------------
// Stub AppController
// --------------------------------------------------------------------------

// stubAppController is a minimal implementation of AppController for tests.
// It records which methods were called and allows the test to inspect the results.
type stubAppController struct {
	runCalls         []string
	cancelCalled     int
	clearQueueCalled int
	clearMsgCalled   int
	queueLen         int
}

func (s *stubAppController) Run(prompt string) int {
	s.runCalls = append(s.runCalls, prompt)
	return s.queueLen
}

func (s *stubAppController) CancelCurrentStep() {
	s.cancelCalled++
}

func (s *stubAppController) QueueLength() int {
	return s.queueLen
}

func (s *stubAppController) ClearQueue() {
	s.clearQueueCalled++
	s.queueLen = 0
}

func (s *stubAppController) ClearMessages() {
	s.clearMsgCalled++
}

func (s *stubAppController) ReloadMessagesFromTree() {
	// no-op in tests
}

func (s *stubAppController) CompactConversation(_ string) error {
	return nil
}

func (s *stubAppController) GetTreeSession() *session.TreeManager {
	return nil
}

func (s *stubAppController) SwitchTreeSession(_ *session.TreeManager) {
	// no-op in tests
}

func (s *stubAppController) SendEvent(_ tea.Msg) {
	// no-op in tests
}

func (s *stubAppController) AddContextMessage(_ string) {
	// no-op in tests
}

func (s *stubAppController) RunWithFiles(prompt string, _ []kit.LLMFilePart) int {
	s.runCalls = append(s.runCalls, prompt)
	return s.queueLen
}

func (s *stubAppController) Steer(prompt string) int {
	s.runCalls = append(s.runCalls, prompt)
	return s.queueLen
}

// --------------------------------------------------------------------------
// Stub child components
// --------------------------------------------------------------------------

// stubStreamComponent satisfies streamComponentIface without rendering anything.
type stubStreamComponent struct {
	resetCalled     int
	height          int
	lastMsg         tea.Msg
	renderedContent string // returned by GetRenderedContent
}

func (s *stubStreamComponent) Init() tea.Cmd { return nil }
func (s *stubStreamComponent) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	s.lastMsg = msg
	return s, nil
}
func (s *stubStreamComponent) View() tea.View             { return tea.NewView("") }
func (s *stubStreamComponent) Reset()                     { s.resetCalled++; s.renderedContent = "" }
func (s *stubStreamComponent) SetHeight(h int)            { s.height = h }
func (s *stubStreamComponent) GetRenderedContent() string { return s.renderedContent }
func (s *stubStreamComponent) ConsumeOverflow() string    { return "" }
func (s *stubStreamComponent) SpinnerView() string        { return "" }
func (s *stubStreamComponent) SetThinkingVisible(bool)    {}
func (s *stubStreamComponent) HasReasoning() bool         { return false }
func (s *stubStreamComponent) UpdateTheme()               {}

// stubInputComponent satisfies inputComponentIface without rendering anything.
type stubInputComponent struct {
	lastMsg tea.Msg
}

func (s *stubInputComponent) Init() tea.Cmd { return nil }
func (s *stubInputComponent) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	s.lastMsg = msg
	return s, nil
}
func (s *stubInputComponent) View() tea.View { return tea.NewView("") }

// --------------------------------------------------------------------------
// newTestAppModel creates an AppModel with stub children for unit tests.
// --------------------------------------------------------------------------

func newTestAppModel(ctrl AppController) (*AppModel, *stubStreamComponent, *stubInputComponent) {
	stream := &stubStreamComponent{}
	input := &stubInputComponent{}
	m := &AppModel{
		state:                 stateInput,
		appCtrl:               ctrl,
		stream:                stream,
		input:                 input,
		renderer:              newMessageRenderer(80, false),
		modelName:             "test-model",
		width:                 80,
		height:                24,
		streamingBashMaxLines: 50, // Initialize buffer cap like NewAppModel does
	}
	return m, stream, input
}

// --------------------------------------------------------------------------
// Helpers
// --------------------------------------------------------------------------

// sendMsg calls m.Update once with the given message and returns the updated model.
func sendMsg(m *AppModel, msg tea.Msg) *AppModel {
	updated, _ := m.Update(msg)
	result := updated.(*AppModel)
	// Simulate BubbleTea's frame cycle: View() is called after every Update().
	// This flushes any pending layoutDirty work (e.g. distributeHeight).
	_ = result.View()
	return result
}

// --------------------------------------------------------------------------
// State transitions
// --------------------------------------------------------------------------

// TestStateTransition_InputToWorking verifies that a submitMsg while in
// stateInput transitions the model to stateWorking.
func TestStateTransition_InputToWorking(t *testing.T) {
	ctrl := &stubAppController{}
	m, _, _ := newTestAppModel(ctrl)

	if m.state != stateInput {
		t.Fatalf("expected stateInput, got %v", m.state)
	}

	m = sendMsg(m, submitMsg{Text: "hello"})

	if m.state != stateWorking {
		t.Fatalf("expected stateWorking after submitMsg, got %v", m.state)
	}
	if len(ctrl.runCalls) != 1 || ctrl.runCalls[0] != "hello" {
		t.Fatalf("expected Run called with 'hello', got %v", ctrl.runCalls)
	}
}

// TestStateTransition_WorkingToInput_StepComplete verifies that StepCompleteEvent
// transitions from stateWorking back to stateInput and keeps stream content
// visible (deferred flush — no Reset until next SpinnerEvent{Show: true}).
func TestStateTransition_WorkingToInput_StepComplete(t *testing.T) {
	ctrl := &stubAppController{}
	m, stream, _ := newTestAppModel(ctrl)
	m.state = stateWorking

	m = sendMsg(m, app.StepCompleteEvent{ResponseText: "all done"})

	if m.state != stateInput {
		t.Fatalf("expected stateInput after StepCompleteEvent, got %v", m.state)
	}
	if stream.resetCalled != 0 {
		t.Fatalf("expected stream NOT reset on StepCompleteEvent (deferred flush), got %d resets", stream.resetCalled)
	}
}

// TestStateTransition_WorkingToInput_StepError verifies that StepErrorEvent
// transitions from stateWorking back to stateInput and keeps stream content
// visible (deferred flush — no Reset until next SpinnerEvent{Show: true}).
func TestStateTransition_WorkingToInput_StepError(t *testing.T) {
	ctrl := &stubAppController{}
	m, stream, _ := newTestAppModel(ctrl)
	m.state = stateWorking

	m = sendMsg(m, app.StepErrorEvent{Err: errors.New("something broke")})

	if m.state != stateInput {
		t.Fatalf("expected stateInput after StepErrorEvent, got %v", m.state)
	}
	if stream.resetCalled != 0 {
		t.Fatalf("expected stream NOT reset on StepErrorEvent (deferred flush), got %d resets", stream.resetCalled)
	}
}

// TestStepError_nilErr verifies that a StepErrorEvent with a nil error does not
// panic and still transitions to stateInput.
func TestStepError_nilErr(t *testing.T) {
	ctrl := &stubAppController{}
	m, _, _ := newTestAppModel(ctrl)
	m.state = stateWorking

	m = sendMsg(m, app.StepErrorEvent{Err: nil})

	if m.state != stateInput {
		t.Fatalf("expected stateInput for nil-error StepErrorEvent, got %v", m.state)
	}
}

// --------------------------------------------------------------------------
// StepCancelledEvent
// --------------------------------------------------------------------------

// TestStateTransition_WorkingToInput_StepCancelled verifies that StepCancelledEvent
// transitions from stateWorking back to stateInput and keeps stream content
// visible (deferred flush — no Reset until next SpinnerEvent{Show: true}).
func TestStateTransition_WorkingToInput_StepCancelled(t *testing.T) {
	ctrl := &stubAppController{}
	m, stream, _ := newTestAppModel(ctrl)
	m.state = stateWorking

	m = sendMsg(m, app.StepCancelledEvent{})

	if m.state != stateInput {
		t.Fatalf("expected stateInput after StepCancelledEvent, got %v", m.state)
	}
	if stream.resetCalled != 0 {
		t.Fatalf("expected stream NOT reset on StepCancelledEvent (deferred flush), got %d resets", stream.resetCalled)
	}
}

// TestStepCancelled_clearsCanceling verifies that StepCancelledEvent clears
// the canceling flag.
func TestStepCancelled_clearsCanceling(t *testing.T) {
	ctrl := &stubAppController{}
	m, _, _ := newTestAppModel(ctrl)
	m.state = stateWorking
	m.canceling = true

	m = sendMsg(m, app.StepCancelledEvent{})

	if m.canceling {
		t.Fatal("expected canceling=false after StepCancelledEvent")
	}
}

// TestStepCancelled_preservesStreamContent verifies that StepCancelledEvent
// does NOT flush stream content — it stays visible for deferred flush.
func TestStepCancelled_preservesStreamContent(t *testing.T) {
	ctrl := &stubAppController{}
	m, stream, _ := newTestAppModel(ctrl)
	m.state = stateWorking
	stream.renderedContent = "partial assistant response"

	_ = sendMsg(m, app.StepCancelledEvent{})

	if stream.renderedContent != "partial assistant response" {
		t.Fatal("expected stream content preserved after StepCancelledEvent")
	}
}

// TestStepCancelled_noStreamContent_noCmd verifies that StepCancelledEvent with
// no accumulated stream content produces a nil cmd (nothing to flush).
func TestStepCancelled_noStreamContent_noCmd(t *testing.T) {
	ctrl := &stubAppController{}
	m, _, _ := newTestAppModel(ctrl)
	m.state = stateWorking

	_, cmd := m.Update(app.StepCancelledEvent{})

	if cmd != nil {
		t.Fatal("expected nil cmd on StepCancelledEvent with no stream content")
	}
}

// TestStepCancelled_noErrorPrinted verifies that StepCancelledEvent does NOT
// produce an error message (unlike StepErrorEvent).
func TestStepCancelled_noErrorPrinted(t *testing.T) {
	ctrl := &stubAppController{}
	m, _, _ := newTestAppModel(ctrl)
	m.state = stateWorking

	// With no stream content, cmd should be nil (no flush, and no error print).
	_, cmd := m.Update(app.StepCancelledEvent{})

	if cmd != nil {
		t.Fatal("expected nil cmd for StepCancelledEvent with no stream content — should not print error")
	}
}

// --------------------------------------------------------------------------
// ESC cancel flow
// --------------------------------------------------------------------------

// TestESCCancel_singleTap verifies that the first ESC press during stateWorking
// sets canceling=true (and does not call CancelCurrentStep).
func TestESCCancel_singleTap(t *testing.T) {
	ctrl := &stubAppController{}
	m, _, _ := newTestAppModel(ctrl)
	m.state = stateWorking

	m = sendMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})

	if !m.canceling {
		t.Fatal("expected canceling=true after first ESC in stateWorking")
	}
	if ctrl.cancelCalled != 0 {
		t.Fatalf("expected no CancelCurrentStep call after first ESC, got %d", ctrl.cancelCalled)
	}
}

// TestESCCancel_doubleTap verifies that a second ESC press while canceling=true
// calls CancelCurrentStep() and resets canceling.
func TestESCCancel_doubleTap(t *testing.T) {
	ctrl := &stubAppController{}
	m, _, _ := newTestAppModel(ctrl)
	m.state = stateWorking
	m.canceling = true // simulate first ESC already pressed

	m = sendMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})

	if m.canceling {
		t.Fatal("expected canceling=false after second ESC")
	}
	if ctrl.cancelCalled != 1 {
		t.Fatalf("expected CancelCurrentStep called once after double ESC, got %d", ctrl.cancelCalled)
	}
}

// TestESCCancel_timerExpiry verifies that cancelTimerExpiredMsg resets canceling
// to false (timer fires before second ESC).
func TestESCCancel_timerExpiry(t *testing.T) {
	ctrl := &stubAppController{}
	m, _, _ := newTestAppModel(ctrl)
	m.state = stateWorking
	m.canceling = true

	m = sendMsg(m, cancelTimerExpiredMsg{})

	if m.canceling {
		t.Fatal("expected canceling=false after timer expiry")
	}
	if ctrl.cancelCalled != 0 {
		t.Fatalf("expected no CancelCurrentStep after timer expiry, got %d", ctrl.cancelCalled)
	}
}

// TestESCCancel_noEffectInStateInput verifies that ESC in stateInput does not set
// canceling (it's passed to child components instead).
func TestESCCancel_noEffectInStateInput(t *testing.T) {
	ctrl := &stubAppController{}
	m, _, _ := newTestAppModel(ctrl)
	// state is stateInput by default

	m = sendMsg(m, tea.KeyPressMsg{Code: tea.KeyEscape})

	if m.canceling {
		t.Fatal("expected canceling=false when ESC pressed in stateInput")
	}
	if ctrl.cancelCalled != 0 {
		t.Fatalf("expected no CancelCurrentStep in stateInput, got %d", ctrl.cancelCalled)
	}
}

// TestESCCancel_clearedOnStepComplete verifies that completing a step clears
// any pending canceling state.
func TestESCCancel_clearedOnStepComplete(t *testing.T) {
	ctrl := &stubAppController{}
	m, _, _ := newTestAppModel(ctrl)
	m.state = stateWorking
	m.canceling = true

	m = sendMsg(m, app.StepCompleteEvent{ResponseText: "done"})

	if m.canceling {
		t.Fatal("expected canceling=false after StepCompleteEvent")
	}
}

// --------------------------------------------------------------------------
// Queued messages
// --------------------------------------------------------------------------

// TestQueuedMessages_storedOnQueuedSubmit verifies that submitting a prompt
// while the agent is busy stores the text in queuedMessages (not scrollback).
func TestQueuedMessages_storedOnQueuedSubmit(t *testing.T) {
	ctrl := &stubAppController{queueLen: 1} // simulate busy (will return 1)
	m, _, _ := newTestAppModel(ctrl)
	m.state = stateWorking

	_, cmd := m.Update(submitMsg{Text: "queued prompt"})

	if len(m.queuedMessages) != 1 {
		t.Fatalf("expected 1 queued message, got %d", len(m.queuedMessages))
	}
	if m.queuedMessages[0] != "queued prompt" {
		t.Fatalf("expected queued message text 'queued prompt', got %q", m.queuedMessages[0])
	}
	// Should NOT produce a tea.Println cmd (message is anchored, not in scrollback).
	if cmd != nil {
		t.Fatal("expected nil cmd for queued submit (message should not print to scrollback)")
	}
}

// TestQueuedMessages_poppedOnQueueUpdated verifies that QueueUpdatedEvent pops
// consumed messages from queuedMessages and moves them to pendingUserPrints.
// The actual printing is deferred to SpinnerEvent{Show: true} to preserve
// chronological order with the preceding assistant response.
func TestQueuedMessages_poppedOnQueueUpdated(t *testing.T) {
	ctrl := &stubAppController{}
	m, _, _ := newTestAppModel(ctrl)
	m.queuedMessages = []string{"first", "second", "third"}

	// Simulate drainQueue popping one item (length goes from 3 to 2).
	m = sendMsg(m, app.QueueUpdatedEvent{Length: 2})

	if len(m.queuedMessages) != 2 {
		t.Fatalf("expected 2 queued messages after pop, got %d", len(m.queuedMessages))
	}
	if m.queuedMessages[0] != "second" {
		t.Fatalf("expected first remaining message 'second', got %q", m.queuedMessages[0])
	}
	// Popped message should be deferred to pendingUserPrints.
	if len(m.pendingUserPrints) != 1 {
		t.Fatalf("expected 1 pending user print, got %d", len(m.pendingUserPrints))
	}
	if m.pendingUserPrints[0] != "first" {
		t.Fatalf("expected pending message 'first', got %q", m.pendingUserPrints[0])
	}
}

// TestQueuedMessages_allPoppedOnDrain verifies that QueueUpdatedEvent with
// Length=0 pops all remaining queued messages into pendingUserPrints.
func TestQueuedMessages_allPoppedOnDrain(t *testing.T) {
	ctrl := &stubAppController{}
	m, _, _ := newTestAppModel(ctrl)
	m.queuedMessages = []string{"alpha", "beta"}

	m = sendMsg(m, app.QueueUpdatedEvent{Length: 0})

	if len(m.queuedMessages) != 0 {
		t.Fatalf("expected 0 queued messages after drain, got %d", len(m.queuedMessages))
	}
	if len(m.pendingUserPrints) != 2 {
		t.Fatalf("expected 2 pending user prints, got %d", len(m.pendingUserPrints))
	}
}

// --------------------------------------------------------------------------
// Window resize
// --------------------------------------------------------------------------

// TestWindowResize_updatesWidthHeight verifies that tea.WindowSizeMsg updates
// m.width and m.height.
func TestWindowResize_updatesWidthHeight(t *testing.T) {
	ctrl := &stubAppController{}
	m, _, _ := newTestAppModel(ctrl)

	m = sendMsg(m, tea.WindowSizeMsg{Width: 120, Height: 40})

	if m.width != 120 {
		t.Fatalf("expected width=120, got %d", m.width)
	}
	if m.height != 40 {
		t.Fatalf("expected height=40, got %d", m.height)
	}
}

// TestWindowResize_propagatesToStream verifies that tea.WindowSizeMsg is forwarded
// to the stream component.
func TestWindowResize_propagatesToStream(t *testing.T) {
	ctrl := &stubAppController{}
	m, stream, _ := newTestAppModel(ctrl)

	_ = sendMsg(m, tea.WindowSizeMsg{Width: 100, Height: 30})

	if stream.lastMsg == nil {
		t.Fatal("expected stream component to receive WindowSizeMsg")
	}
	if _, ok := stream.lastMsg.(tea.WindowSizeMsg); !ok {
		t.Fatalf("expected stream.lastMsg to be WindowSizeMsg, got %T", stream.lastMsg)
	}
}

// TestWindowResize_distributeHeight verifies that distributeHeight correctly
// sets the stream height after a resize.
func TestWindowResize_distributeHeight(t *testing.T) {
	ctrl := &stubAppController{}
	m, stream, _ := newTestAppModel(ctrl)

	// With height=30, stream height = 30 - 1 (separator) - 9 (input) - 1 (statusBar) = 19
	m = sendMsg(m, tea.WindowSizeMsg{Width: 80, Height: 30})
	_ = m

	if stream.height != 19 {
		t.Fatalf("expected stream height=19, got %d", stream.height)
	}
}

// --------------------------------------------------------------------------
// tea.Println on step complete
// --------------------------------------------------------------------------

// TestStepComplete_preservesStreamContent verifies that StepCompleteEvent
// does NOT flush stream content — it stays visible for deferred flush.
func TestStepComplete_preservesStreamContent(t *testing.T) {
	ctrl := &stubAppController{}
	m, stream, _ := newTestAppModel(ctrl)
	m.state = stateWorking
	// Simulate accumulated streaming text.
	stream.renderedContent = "rendered assistant text"

	_ = sendMsg(m, app.StepCompleteEvent{ResponseText: "final answer"})

	if stream.renderedContent != "rendered assistant text" {
		t.Fatal("expected stream content preserved after StepCompleteEvent")
	}
}

// TestStepComplete_noStreamContent_noCmd verifies that StepCompleteEvent with
// no accumulated stream content produces a nil cmd (nothing to flush).
func TestStepComplete_noStreamContent_noCmd(t *testing.T) {
	ctrl := &stubAppController{}
	m, _, _ := newTestAppModel(ctrl)
	m.state = stateWorking

	_, cmd := m.Update(app.StepCompleteEvent{})

	if cmd != nil {
		t.Fatal("expected nil cmd on StepCompleteEvent with no stream content")
	}
}

// TestSubmitMsg_printsUserMessage verifies that submitMsg produces a tea.Println
// cmd for the user message.
func TestSubmitMsg_printsUserMessage(t *testing.T) {
	ctrl := &stubAppController{}
	m, _, _ := newTestAppModel(ctrl)

	_, cmd := m.Update(submitMsg{Text: "user query"})

	if cmd == nil {
		t.Fatal("expected non-nil cmd (tea.Println) for user message on submitMsg")
	}
}

// TestToolCallStarted_flushesOnly verifies that ToolCallStartedEvent flushes
// accumulated stream content but does NOT print a tool call block (the unified
// block is printed later on ToolResultEvent).
func TestToolCallStarted_flushesOnly(t *testing.T) {
	ctrl := &stubAppController{}
	m, stream, _ := newTestAppModel(ctrl)
	m.state = stateWorking

	// With no stream content, flush returns nil → cmd should be nil.
	_, cmd := m.Update(app.ToolCallStartedEvent{
		ToolName: "bash",
		ToolArgs: `{"cmd":"ls"}`,
	})

	if cmd != nil {
		t.Fatal("expected nil cmd on ToolCallStartedEvent with no stream content")
	}

	// With stream content, flush returns tea.Println → cmd should be non-nil.
	stream.renderedContent = "partial text"
	_, cmd = m.Update(app.ToolCallStartedEvent{
		ToolName: "bash",
		ToolArgs: `{"cmd":"ls"}`,
	})

	if cmd == nil {
		t.Fatal("expected non-nil cmd on ToolCallStartedEvent with stream content to flush")
	}
}

// TestToolResult_printsAndStartsSpinner verifies that ToolResultEvent produces
// a non-nil cmd and the stream receives a SpinnerEvent.
func TestToolResult_printsAndStartsSpinner(t *testing.T) {
	ctrl := &stubAppController{}
	m, stream, _ := newTestAppModel(ctrl)
	m.state = stateWorking

	_, cmd := m.Update(app.ToolResultEvent{
		ToolName: "bash",
		ToolArgs: "{}",
		Result:   "output",
		IsError:  false,
	})

	if cmd == nil {
		t.Fatal("expected non-nil cmd on ToolResultEvent")
	}
	// Stream should have received a SpinnerEvent to start spinner for next LLM call.
	if stream.lastMsg == nil {
		t.Fatal("expected stream to receive SpinnerEvent after ToolResultEvent")
	}
	if se, ok := stream.lastMsg.(app.SpinnerEvent); !ok || !se.Show {
		t.Fatalf("expected SpinnerEvent{Show:true}, got %T", stream.lastMsg)
	}
}

// TestToolOutputEvent_accumulatesBashOutput verifies that ToolOutputEvent
// accumulates stdout and stderr lines into the streaming bash output buffers.
func TestToolOutputEvent_accumulatesBashOutput(t *testing.T) {
	ctrl := &stubAppController{}
	m, _, _ := newTestAppModel(ctrl)
	m.state = stateWorking

	// Send stdout chunk.
	m = sendMsg(m, app.ToolOutputEvent{
		ToolCallID: "call-1",
		ToolName:   "bash",
		Chunk:      "line one\n",
		IsStderr:   false,
	})

	if len(m.streamingBashOutput) != 1 || m.streamingBashOutput[0] != "line one\n" {
		t.Fatalf("expected streamingBashOutput=['line one\\n'], got %v", m.streamingBashOutput)
	}
	if len(m.streamingBashStderr) != 0 {
		t.Fatalf("expected empty streamingBashStderr, got %v", m.streamingBashStderr)
	}

	// Send another stdout chunk.
	m = sendMsg(m, app.ToolOutputEvent{
		ToolCallID: "call-1",
		ToolName:   "bash",
		Chunk:      "line two\n",
		IsStderr:   false,
	})

	if len(m.streamingBashOutput) != 2 {
		t.Fatalf("expected 2 stdout lines, got %d", len(m.streamingBashOutput))
	}

	// Send stderr chunk.
	m = sendMsg(m, app.ToolOutputEvent{
		ToolCallID: "call-1",
		ToolName:   "bash",
		Chunk:      "error: something failed\n",
		IsStderr:   true,
	})

	if len(m.streamingBashStderr) != 1 {
		t.Fatalf("expected 1 stderr line, got %d", len(m.streamingBashStderr))
	}
	if m.streamingBashStderr[0] != "error: something failed\n" {
		t.Fatalf("expected stderr 'error: something failed\\n', got %q", m.streamingBashStderr[0])
	}
}

// TestToolResult_clearsStreamingBashOutput verifies that ToolResultEvent clears
// the streaming bash output buffers since the final result will be printed.
func TestToolResult_clearsStreamingBashOutput(t *testing.T) {
	ctrl := &stubAppController{}
	m, _, _ := newTestAppModel(ctrl)
	m.state = stateWorking

	// Accumulate some bash output.
	m.streamingBashOutput = []string{"output line"}
	m.streamingBashStderr = []string{"error line"}

	_, _ = m.Update(app.ToolResultEvent{
		ToolName: "bash",
		ToolArgs: `{"cmd":"ls"}`,
		Result:   "output line\nerror line\n",
		IsError:  false,
	})

	if len(m.streamingBashOutput) != 0 {
		t.Fatalf("expected streamingBashOutput cleared, got %v", m.streamingBashOutput)
	}
	if len(m.streamingBashStderr) != 0 {
		t.Fatalf("expected streamingBashStderr cleared, got %v", m.streamingBashStderr)
	}
}

// TestToolCallStarted_extractsBashCommand verifies that ToolCallStartedEvent
// extracts the bash command from ToolArgs and stores it for the streaming output header.
func TestToolCallStarted_extractsBashCommand(t *testing.T) {
	ctrl := &stubAppController{}
	m, _, _ := newTestAppModel(ctrl)
	m.state = stateWorking

	// Send ToolCallStartedEvent with bash command.
	m = sendMsg(m, app.ToolCallStartedEvent{
		ToolCallID: "call-1",
		ToolName:   "bash",
		ToolArgs:   `{"command":"ls -la /home"}`,
	})

	if m.streamingBashCommand != "ls -la /home" {
		t.Fatalf("expected streamingBashCommand='ls -la /home', got %q", m.streamingBashCommand)
	}

	// ToolResultEvent should clear the command.
	m = sendMsg(m, app.ToolResultEvent{
		ToolCallID: "call-1",
		ToolName:   "bash",
		ToolArgs:   `{"command":"ls -la /home"}`,
		Result:     "output",
		IsError:    false,
	})

	if m.streamingBashCommand != "" {
		t.Fatalf("expected streamingBashCommand cleared, got %q", m.streamingBashCommand)
	}
}

// TestToolCallStarted_nonBashTool_doesNotSetCommand verifies that non-bash tools
// do not set the streamingBashCommand field.
func TestToolCallStarted_nonBashTool_doesNotSetCommand(t *testing.T) {
	ctrl := &stubAppController{}
	m, _, _ := newTestAppModel(ctrl)
	m.state = stateWorking

	// Send ToolCallStartedEvent with a non-bash tool.
	m = sendMsg(m, app.ToolCallStartedEvent{
		ToolCallID: "call-1",
		ToolName:   "read",
		ToolArgs:   `{"file":"/etc/passwd"}`,
	})

	if m.streamingBashCommand != "" {
		t.Fatalf("expected streamingBashCommand to remain empty for non-bash tools, got %q", m.streamingBashCommand)
	}
}

// TestStepError_printCmd verifies that StepErrorEvent with a non-nil error
// produces a non-nil cmd (the tea.Println call for the error message).
func TestStepError_printCmd(t *testing.T) {
	ctrl := &stubAppController{}
	m, _, _ := newTestAppModel(ctrl)
	m.state = stateWorking

	_, cmd := m.Update(app.StepErrorEvent{Err: errors.New("agent failed")})

	if cmd == nil {
		t.Fatal("expected non-nil cmd (tea.Println) on StepErrorEvent with error")
	}
}

// --------------------------------------------------------------------------
// SpinnerEvent: stateWorking on Show=true
// --------------------------------------------------------------------------

// TestSpinnerEvent_showTransitionsToWorking verifies that SpinnerEvent{Show:true}
// transitions the model to stateWorking (important for queued step drain path).
func TestSpinnerEvent_showTransitionsToWorking(t *testing.T) {
	ctrl := &stubAppController{}
	m, _, _ := newTestAppModel(ctrl)
	// After a step completes, model is back in stateInput. The next queued step
	// starts and fires SpinnerEvent{Show: true}.
	m.state = stateInput

	m = sendMsg(m, app.SpinnerEvent{Show: true})

	if m.state != stateWorking {
		t.Fatalf("expected stateWorking after SpinnerEvent{Show:true} in stateInput, got %v", m.state)
	}
}

// TestSpinnerEvent_hidDoesNotTransitionState verifies that SpinnerEvent{Show:false}
// does not change the state.
func TestSpinnerEvent_hideDoesNotTransitionState(t *testing.T) {
	ctrl := &stubAppController{}
	m, _, _ := newTestAppModel(ctrl)
	m.state = stateWorking

	m = sendMsg(m, app.SpinnerEvent{Show: false})

	if m.state != stateWorking {
		t.Fatalf("expected state to remain stateWorking, got %v", m.state)
	}
}

// --------------------------------------------------------------------------
// ctrl+c produces tea.Quit
// --------------------------------------------------------------------------

// TestCtrlC_producesQuit verifies that ctrl+c always returns a tea.Quit cmd.
func TestCtrlC_producesQuit(t *testing.T) {
	ctrl := &stubAppController{}
	m, _, _ := newTestAppModel(ctrl)

	_, cmd := m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})

	if cmd == nil {
		t.Fatal("expected tea.Quit cmd on ctrl+c, got nil")
	}
	// We verify it's a quit command by running it and checking the message type.
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Fatalf("expected QuitMsg from ctrl+c cmd, got %T", msg)
	}
}

// --------------------------------------------------------------------------
// submitMsg during stateWorking (queue path)
// --------------------------------------------------------------------------

// TestSubmit_duringWorking_stays verifies that submitting a new prompt while in
// stateWorking keeps the model in stateWorking (queued via app.Run).
func TestSubmit_duringWorking_stays(t *testing.T) {
	ctrl := &stubAppController{}
	m, _, _ := newTestAppModel(ctrl)
	m.state = stateWorking

	m = sendMsg(m, submitMsg{Text: "queued prompt"})

	if m.state != stateWorking {
		t.Fatalf("expected stateWorking to persist after submitMsg during working, got %v", m.state)
	}
	if len(ctrl.runCalls) != 1 || ctrl.runCalls[0] != "queued prompt" {
		t.Fatalf("expected Run('queued prompt') called, got %v", ctrl.runCalls)
	}
}
