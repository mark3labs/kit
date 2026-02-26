package ui

import (
	"errors"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/fantasy"
	"github.com/mark3labs/kit/internal/app"
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
		state:       stateInput,
		appCtrl:     ctrl,
		stream:      stream,
		input:       input,
		renderer:    NewMessageRenderer(80, false),
		compactRdr:  NewCompactRenderer(80, false),
		compactMode: false,
		modelName:   "test-model",
		width:       80,
		height:      24,
	}
	return m, stream, input
}

// --------------------------------------------------------------------------
// Helpers
// --------------------------------------------------------------------------

// sendMsg calls m.Update once with the given message and returns the updated model.
func sendMsg(m *AppModel, msg tea.Msg) *AppModel {
	updated, _ := m.Update(msg)
	return updated.(*AppModel)
}

// makeTestResponse constructs a fantasy.Response with the given text content.
// Uses fantasy.TextContent (the type that ResponseContent.Text() recognises) rather
// than TextPart (which is a request-side type).
func makeTestResponse(text string) *fantasy.Response {
	return &fantasy.Response{
		Content: fantasy.ResponseContent{fantasy.TextContent{Text: text}},
	}
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
// transitions from stateWorking back to stateInput and resets the stream component.
func TestStateTransition_WorkingToInput_StepComplete(t *testing.T) {
	ctrl := &stubAppController{}
	m, stream, _ := newTestAppModel(ctrl)
	m.state = stateWorking

	m = sendMsg(m, app.StepCompleteEvent{
		Response: makeTestResponse("all done"),
		Usage:    fantasy.Usage{},
	})

	if m.state != stateInput {
		t.Fatalf("expected stateInput after StepCompleteEvent, got %v", m.state)
	}
	if stream.resetCalled != 1 {
		t.Fatalf("expected stream.Reset() called once, got %d", stream.resetCalled)
	}
}

// TestStateTransition_WorkingToInput_StepError verifies that StepErrorEvent
// transitions from stateWorking back to stateInput and resets the stream component.
func TestStateTransition_WorkingToInput_StepError(t *testing.T) {
	ctrl := &stubAppController{}
	m, stream, _ := newTestAppModel(ctrl)
	m.state = stateWorking

	m = sendMsg(m, app.StepErrorEvent{Err: errors.New("something broke")})

	if m.state != stateInput {
		t.Fatalf("expected stateInput after StepErrorEvent, got %v", m.state)
	}
	if stream.resetCalled != 1 {
		t.Fatalf("expected stream.Reset() called once, got %d", stream.resetCalled)
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
// transitions from stateWorking back to stateInput and resets the stream component.
func TestStateTransition_WorkingToInput_StepCancelled(t *testing.T) {
	ctrl := &stubAppController{}
	m, stream, _ := newTestAppModel(ctrl)
	m.state = stateWorking

	m = sendMsg(m, app.StepCancelledEvent{})

	if m.state != stateInput {
		t.Fatalf("expected stateInput after StepCancelledEvent, got %v", m.state)
	}
	if stream.resetCalled != 1 {
		t.Fatalf("expected stream.Reset() called once, got %d", stream.resetCalled)
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

// TestStepCancelled_flushesStreamContent verifies that StepCancelledEvent
// flushes accumulated stream content via tea.Println (non-nil cmd).
func TestStepCancelled_flushesStreamContent(t *testing.T) {
	ctrl := &stubAppController{}
	m, stream, _ := newTestAppModel(ctrl)
	m.state = stateWorking
	stream.renderedContent = "partial assistant response"

	_, cmd := m.Update(app.StepCancelledEvent{})

	if cmd == nil {
		t.Fatal("expected non-nil cmd (tea.Println) on StepCancelledEvent with stream content")
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

	m = sendMsg(m, app.StepCompleteEvent{
		Response: makeTestResponse("done"),
	})

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
// consumed messages from queuedMessages and prints them to scrollback.
func TestQueuedMessages_poppedOnQueueUpdated(t *testing.T) {
	ctrl := &stubAppController{}
	m, _, _ := newTestAppModel(ctrl)
	m.queuedMessages = []string{"first", "second", "third"}

	// Simulate drainQueue popping one item (length goes from 3 to 2).
	_, cmd := m.Update(app.QueueUpdatedEvent{Length: 2})

	if len(m.queuedMessages) != 2 {
		t.Fatalf("expected 2 queued messages after pop, got %d", len(m.queuedMessages))
	}
	if m.queuedMessages[0] != "second" {
		t.Fatalf("expected first remaining message 'second', got %q", m.queuedMessages[0])
	}
	// Should produce a cmd (tea.Println for the popped user message).
	if cmd == nil {
		t.Fatal("expected non-nil cmd (tea.Println) for popped message")
	}
}

// TestQueuedMessages_allPoppedOnDrain verifies that QueueUpdatedEvent with
// Length=0 pops all remaining queued messages.
func TestQueuedMessages_allPoppedOnDrain(t *testing.T) {
	ctrl := &stubAppController{}
	m, _, _ := newTestAppModel(ctrl)
	m.queuedMessages = []string{"alpha", "beta"}

	m = sendMsg(m, app.QueueUpdatedEvent{Length: 0})

	if len(m.queuedMessages) != 0 {
		t.Fatalf("expected 0 queued messages after drain, got %d", len(m.queuedMessages))
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

	// With height=30, stream height = 30 - 1 (separator) - 5 (input) = 24
	m = sendMsg(m, tea.WindowSizeMsg{Width: 80, Height: 30})
	_ = m

	if stream.height != 24 {
		t.Fatalf("expected stream height=24, got %d", stream.height)
	}
}

// --------------------------------------------------------------------------
// tea.Println on step complete
// --------------------------------------------------------------------------

// TestStepComplete_flushesStreamContent verifies that StepCompleteEvent
// flushes accumulated stream content via tea.Println (non-nil cmd).
func TestStepComplete_flushesStreamContent(t *testing.T) {
	ctrl := &stubAppController{}
	m, stream, _ := newTestAppModel(ctrl)
	m.state = stateWorking
	// Simulate accumulated streaming text.
	stream.renderedContent = "rendered assistant text"

	_, cmd := m.Update(app.StepCompleteEvent{
		Response: makeTestResponse("final answer"),
		Usage:    fantasy.Usage{},
	})

	// A non-nil cmd means flushStreamContent returned tea.Println(...)
	if cmd == nil {
		t.Fatal("expected non-nil cmd (tea.Println) on StepCompleteEvent with stream content")
	}
}

// TestStepComplete_noStreamContent_noCmd verifies that StepCompleteEvent with
// no accumulated stream content produces a nil cmd (nothing to flush).
func TestStepComplete_noStreamContent_noCmd(t *testing.T) {
	ctrl := &stubAppController{}
	m, _, _ := newTestAppModel(ctrl)
	m.state = stateWorking

	_, cmd := m.Update(app.StepCompleteEvent{Response: nil})

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

// TestToolCallStarted_flushesAndPrints verifies that ToolCallStartedEvent
// produces a non-nil cmd (flush + tool call print).
func TestToolCallStarted_flushesAndPrints(t *testing.T) {
	ctrl := &stubAppController{}
	m, _, _ := newTestAppModel(ctrl)
	m.state = stateWorking

	_, cmd := m.Update(app.ToolCallStartedEvent{
		ToolName: "bash",
		ToolArgs: `{"cmd":"ls"}`,
	})

	if cmd == nil {
		t.Fatal("expected non-nil cmd on ToolCallStartedEvent")
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
