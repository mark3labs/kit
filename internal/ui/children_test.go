package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/mark3labs/mcphost/internal/app"
)

// ==========================================================================
// InputComponent tests
// ==========================================================================

// --------------------------------------------------------------------------
// Helpers
// --------------------------------------------------------------------------

// newTestInput creates an InputComponent with the given AppController (may be nil).
func newTestInput(ctrl AppController) *InputComponent {
	return NewInputComponent(80, "test input", ctrl)
}

// sendInputMsg calls component.Update with the given message, returns the
// updated component and the cmd.
func sendInputMsg(c *InputComponent, msg tea.Msg) (*InputComponent, tea.Cmd) {
	m, cmd := c.Update(msg)
	return m.(*InputComponent), cmd
}

// pressKey simulates a single key press on the InputComponent.
func pressKey(c *InputComponent, r rune) (*InputComponent, tea.Cmd) {
	return sendInputMsg(c, tea.KeyPressMsg{Code: r})
}

// typeText types a string into the InputComponent character by character.
func typeText(c *InputComponent, text string) *InputComponent {
	for _, ch := range text {
		c.textarea.SetValue(c.textarea.Value() + string(ch))
		c.lastValue = c.textarea.Value()
	}
	return c
}

// runCmd executes a tea.Cmd and returns the resulting tea.Msg.
// Returns nil if cmd is nil.
func runCmd(cmd tea.Cmd) tea.Msg {
	if cmd == nil {
		return nil
	}
	return cmd()
}

// --------------------------------------------------------------------------
// TestInputComponent_SubmitEmitsSubmitMsg verifies that pressing enter on a
// non-empty textarea emits a submitMsg with the typed text.
// --------------------------------------------------------------------------

func TestInputComponent_SubmitEmitsSubmitMsg(t *testing.T) {
	ctrl := &stubAppController{}
	c := newTestInput(ctrl)

	// Type text directly into the textarea (bypassing key events to keep the
	// test simple — we only care about the submit path here).
	c.textarea.SetValue("hello world")
	c.lastValue = "hello world"

	// Press enter via key press (no popup visible).
	_, cmd := sendInputMsg(c, tea.KeyPressMsg{Code: tea.KeyEnter})

	msg := runCmd(cmd)
	if msg == nil {
		t.Fatal("expected a cmd from pressing enter on non-empty input")
	}

	sm, ok := msg.(submitMsg)
	if !ok {
		t.Fatalf("expected submitMsg, got %T", msg)
	}
	if sm.Text != "hello world" {
		t.Fatalf("expected Text='hello world', got %q", sm.Text)
	}
}

// TestInputComponent_CtrlD_SubmitEmitsSubmitMsg verifies that ctrl+d also
// submits the text.
func TestInputComponent_CtrlD_SubmitEmitsSubmitMsg(t *testing.T) {
	ctrl := &stubAppController{}
	c := newTestInput(ctrl)

	c.textarea.SetValue("ctrl+d submit")
	c.lastValue = "ctrl+d submit"

	_, cmd := sendInputMsg(c, tea.KeyPressMsg{Code: 'd', Mod: tea.ModCtrl})

	msg := runCmd(cmd)
	if msg == nil {
		t.Fatal("expected a cmd from ctrl+d on non-empty input")
	}
	sm, ok := msg.(submitMsg)
	if !ok {
		t.Fatalf("expected submitMsg from ctrl+d, got %T", msg)
	}
	if sm.Text != "ctrl+d submit" {
		t.Fatalf("expected Text='ctrl+d submit', got %q", sm.Text)
	}
}

// TestInputComponent_EmptySubmit_NoCmd verifies that submitting an empty or
// whitespace-only string produces no cmd.
func TestInputComponent_EmptySubmit_NoCmd(t *testing.T) {
	ctrl := &stubAppController{}
	c := newTestInput(ctrl)

	// textarea is empty by default
	_, cmd := sendInputMsg(c, tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		t.Fatal("expected nil cmd from submitting empty input")
	}
}

// TestInputComponent_SubmitClearsTextarea verifies that after submit the
// textarea is cleared.
func TestInputComponent_SubmitClearsTextarea(t *testing.T) {
	ctrl := &stubAppController{}
	c := newTestInput(ctrl)

	c.textarea.SetValue("some text")
	c.lastValue = "some text"

	c, _ = sendInputMsg(c, tea.KeyPressMsg{Code: tea.KeyEnter})

	if c.textarea.Value() != "" {
		t.Fatalf("expected textarea to be cleared after submit, got %q", c.textarea.Value())
	}
}

// --------------------------------------------------------------------------
// TestInputComponent_QuitReturnsTeaQuit verifies that submitting /quit (and its
// aliases) returns a tea.Quit cmd.
// --------------------------------------------------------------------------

func TestInputComponent_QuitReturnsTeaQuit(t *testing.T) {
	aliases := []string{"/quit", "/q", "/exit"}
	ctrl := &stubAppController{}

	for _, alias := range aliases {
		t.Run(alias, func(t *testing.T) {
			c := newTestInput(ctrl)
			c.textarea.SetValue(alias)
			c.lastValue = alias

			_, cmd := sendInputMsg(c, tea.KeyPressMsg{Code: tea.KeyEnter})
			if cmd == nil {
				t.Fatalf("%s: expected tea.Quit cmd, got nil", alias)
			}
			msg := runCmd(cmd)
			if _, ok := msg.(tea.QuitMsg); !ok {
				t.Fatalf("%s: expected QuitMsg, got %T", alias, msg)
			}
		})
	}
}

// --------------------------------------------------------------------------
// TestInputComponent_ClearCallsClearMessages verifies that /clear (and its
// aliases) calls appCtrl.ClearMessages() and returns no submitMsg.
// --------------------------------------------------------------------------

func TestInputComponent_ClearCallsClearMessages(t *testing.T) {
	aliases := []string{"/clear", "/c", "/cls"}
	for _, alias := range aliases {
		t.Run(alias, func(t *testing.T) {
			ctrl := &stubAppController{}
			c := newTestInput(ctrl)
			c.textarea.SetValue(alias)
			c.lastValue = alias

			_, cmd := sendInputMsg(c, tea.KeyPressMsg{Code: tea.KeyEnter})

			if ctrl.clearMsgCalled != 1 {
				t.Fatalf("%s: expected ClearMessages() called once, got %d", alias, ctrl.clearMsgCalled)
			}
			// No cmd should be returned (no submitMsg forwarded to parent).
			if cmd != nil {
				msg := runCmd(cmd)
				if _, ok := msg.(submitMsg); ok {
					t.Fatalf("%s: /clear should not emit submitMsg, got submitMsg", alias)
				}
			}
		})
	}
}

// TestInputComponent_ClearNilCtrl_NoPanic verifies that /clear with a nil
// appCtrl does not panic.
func TestInputComponent_ClearNilCtrl_NoPanic(t *testing.T) {
	c := newTestInput(nil)
	c.textarea.SetValue("/clear")
	c.lastValue = "/clear"

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("unexpected panic on /clear with nil controller: %v", r)
		}
	}()

	_, _ = sendInputMsg(c, tea.KeyPressMsg{Code: tea.KeyEnter})
}

// --------------------------------------------------------------------------
// TestInputComponent_ClearQueueCallsClearQueue verifies that /clear-queue (and
// its alias /cq) calls appCtrl.ClearQueue() and returns no submitMsg.
// --------------------------------------------------------------------------

func TestInputComponent_ClearQueueCallsClearQueue(t *testing.T) {
	aliases := []string{"/clear-queue", "/cq"}
	for _, alias := range aliases {
		t.Run(alias, func(t *testing.T) {
			ctrl := &stubAppController{}
			c := newTestInput(ctrl)
			c.textarea.SetValue(alias)
			c.lastValue = alias

			_, cmd := sendInputMsg(c, tea.KeyPressMsg{Code: tea.KeyEnter})

			if ctrl.clearQueueCalled != 1 {
				t.Fatalf("%s: expected ClearQueue() called once, got %d", alias, ctrl.clearQueueCalled)
			}
			if cmd != nil {
				msg := runCmd(cmd)
				if _, ok := msg.(submitMsg); ok {
					t.Fatalf("%s: /clear-queue should not emit submitMsg, got submitMsg", alias)
				}
			}
		})
	}
}

// --------------------------------------------------------------------------
// TestInputComponent_UnknownSlashCommand_ForwardsAsSubmit verifies that a
// slash command not in the registry is forwarded as a submitMsg.
// --------------------------------------------------------------------------

func TestInputComponent_UnknownSlashCommand_ForwardsAsSubmit(t *testing.T) {
	ctrl := &stubAppController{}
	c := newTestInput(ctrl)
	c.textarea.SetValue("/unknown-command")
	c.lastValue = "/unknown-command"

	_, cmd := sendInputMsg(c, tea.KeyPressMsg{Code: tea.KeyEnter})

	msg := runCmd(cmd)
	if msg == nil {
		t.Fatal("expected submitMsg for unknown slash command")
	}
	sm, ok := msg.(submitMsg)
	if !ok {
		t.Fatalf("expected submitMsg for unknown slash command, got %T", msg)
	}
	if sm.Text != "/unknown-command" {
		t.Fatalf("expected Text='/unknown-command', got %q", sm.Text)
	}
}

// ==========================================================================
// StreamComponent tests
// ==========================================================================

// --------------------------------------------------------------------------
// Helpers
// --------------------------------------------------------------------------

// newTestStream creates a StreamComponent with a fixed width and model name,
// in non-compact mode.
func newTestStream() *StreamComponent {
	return NewStreamComponent(false, 80, "test-model")
}

// sendStreamMsg calls component.Update and returns the updated component.
func sendStreamMsg(c *StreamComponent, msg tea.Msg) *StreamComponent {
	m, _ := c.Update(msg)
	return m.(*StreamComponent)
}

// --------------------------------------------------------------------------
// TestStreamComponent_Init_ReturnsNil verifies Init() returns nil (no startup cmd).
// --------------------------------------------------------------------------

func TestStreamComponent_Init_ReturnsNil(t *testing.T) {
	c := newTestStream()
	cmd := c.Init()
	if cmd != nil {
		t.Fatal("expected Init() to return nil cmd")
	}
}

// --------------------------------------------------------------------------
// TestStreamComponent_SpinnerTransition verifies that SpinnerEvent{Show:true}
// transitions phase from idle → spinner and starts the tick loop.
// --------------------------------------------------------------------------

func TestStreamComponent_SpinnerTransition(t *testing.T) {
	c := newTestStream()

	if c.phase != streamPhaseIdle {
		t.Fatalf("expected streamPhaseIdle initially, got %v", c.phase)
	}

	_, cmd := c.Update(app.SpinnerEvent{Show: true})

	if c.phase != streamPhaseSpinner {
		t.Fatalf("expected streamPhaseSpinner after SpinnerEvent{Show:true}, got %v", c.phase)
	}
	// A tick cmd should have been returned to start the animation loop.
	if cmd == nil {
		t.Fatal("expected tick cmd from SpinnerEvent{Show:true}")
	}
}

// TestStreamComponent_SpinnerShowFalse_NoTransitionFromIdle verifies that
// SpinnerEvent{Show:false} when idle has no effect.
func TestStreamComponent_SpinnerShowFalse_NoTransitionFromIdle(t *testing.T) {
	c := newTestStream()

	c = sendStreamMsg(c, app.SpinnerEvent{Show: false})

	if c.phase != streamPhaseIdle {
		t.Fatalf("expected streamPhaseIdle after SpinnerEvent{Show:false}, got %v", c.phase)
	}
}

// --------------------------------------------------------------------------
// TestStreamComponent_SpinnerToStreaming_OnFirstChunk verifies that receiving
// a StreamChunkEvent while in spinner phase transitions to streaming phase.
// --------------------------------------------------------------------------

func TestStreamComponent_SpinnerToStreaming_OnFirstChunk(t *testing.T) {
	c := newTestStream()

	// Enter spinner phase.
	c = sendStreamMsg(c, app.SpinnerEvent{Show: true})
	if c.phase != streamPhaseSpinner {
		t.Fatalf("precondition: expected streamPhaseSpinner, got %v", c.phase)
	}

	// Receive first chunk.
	c = sendStreamMsg(c, app.StreamChunkEvent{Content: "hello"})

	if c.phase != streamPhaseStreaming {
		t.Fatalf("expected streamPhaseStreaming after first chunk, got %v", c.phase)
	}
	if c.streamContent.String() != "hello" {
		t.Fatalf("expected streamContent='hello', got %q", c.streamContent.String())
	}
}

// --------------------------------------------------------------------------
// TestStreamComponent_ChunkAccumulation verifies that multiple StreamChunkEvents
// accumulate in order.
// --------------------------------------------------------------------------

func TestStreamComponent_ChunkAccumulation(t *testing.T) {
	c := newTestStream()

	chunks := []string{"Hello", ", ", "world", "!"}
	for _, chunk := range chunks {
		c = sendStreamMsg(c, app.StreamChunkEvent{Content: chunk})
	}

	got := c.streamContent.String()
	want := "Hello, world!"
	if got != want {
		t.Fatalf("expected accumulated content %q, got %q", want, got)
	}
	if c.phase != streamPhaseStreaming {
		t.Fatalf("expected streamPhaseStreaming, got %v", c.phase)
	}
}

// --------------------------------------------------------------------------
// TestStreamComponent_ToolExecution_IsStarting shows spinner during execution.
// --------------------------------------------------------------------------

func TestStreamComponent_ToolExecution_IsStarting_ShowsSpinner(t *testing.T) {
	c := newTestStream()

	_, cmd := c.Update(app.ToolExecutionEvent{
		ToolName:   "exec_tool",
		IsStarting: true,
	})

	if c.phase != streamPhaseSpinner {
		t.Fatalf("expected streamPhaseSpinner during tool execution, got %v", c.phase)
	}
	if !strings.Contains(c.spinnerMsg, "exec_tool") {
		t.Fatalf("expected spinnerMsg to contain tool name, got %q", c.spinnerMsg)
	}
	if cmd == nil {
		t.Fatal("expected tick cmd from ToolExecutionEvent{IsStarting:true}")
	}
}

// TestStreamComponent_ToolExecution_NotStarting goes idle after execution.
func TestStreamComponent_ToolExecution_NotStarting_GoesIdle(t *testing.T) {
	c := newTestStream()
	c.phase = streamPhaseSpinner // simulating execution in progress

	c = sendStreamMsg(c, app.ToolExecutionEvent{
		ToolName:   "some_tool",
		IsStarting: false,
	})

	if c.phase != streamPhaseIdle {
		t.Fatalf("expected streamPhaseIdle after tool execution finished, got %v", c.phase)
	}
}

// --------------------------------------------------------------------------
// TestStreamComponent_GetRenderedContent verifies the method returns rendered
// text when content is accumulated, and empty string when not.
// --------------------------------------------------------------------------

func TestStreamComponent_GetRenderedContent_Empty(t *testing.T) {
	c := newTestStream()
	if got := c.GetRenderedContent(); got != "" {
		t.Fatalf("expected empty GetRenderedContent on idle component, got %q", got)
	}
}

func TestStreamComponent_GetRenderedContent_WithText(t *testing.T) {
	c := newTestStream()
	c = sendStreamMsg(c, app.StreamChunkEvent{Content: "hello world"})
	got := c.GetRenderedContent()
	if got == "" {
		t.Fatal("expected non-empty GetRenderedContent after chunks")
	}
	// The rendered output contains ANSI escape codes from the message renderer,
	// so check for the text fragments rather than an exact substring.
	if !strings.Contains(got, "hello") {
		t.Fatalf("expected rendered content to contain 'hello', got %q", got)
	}
}

// --------------------------------------------------------------------------
// TestStreamComponent_Reset clears all accumulated state.
// --------------------------------------------------------------------------

func TestStreamComponent_Reset(t *testing.T) {
	c := newTestStream()

	// Accumulate some state.
	c = sendStreamMsg(c, app.SpinnerEvent{Show: true})
	c = sendStreamMsg(c, app.StreamChunkEvent{Content: "some text"})
	c.spinnerFrame = 5

	c.Reset()

	if c.phase != streamPhaseIdle {
		t.Fatalf("expected streamPhaseIdle after Reset(), got %v", c.phase)
	}
	if c.spinnerFrame != 0 {
		t.Fatalf("expected spinnerFrame=0 after Reset(), got %d", c.spinnerFrame)
	}
	if c.streamContent.String() != "" {
		t.Fatalf("expected empty streamContent after Reset(), got %q", c.streamContent.String())
	}
	if !c.timestamp.IsZero() {
		t.Fatal("expected zero timestamp after Reset()")
	}
	if c.spinnerMsg != "Thinking…" {
		t.Fatalf("expected spinnerMsg reset to default, got %q", c.spinnerMsg)
	}
}

// --------------------------------------------------------------------------
// TestStreamComponent_SetHeight propagates to height field.
// --------------------------------------------------------------------------

func TestStreamComponent_SetHeight(t *testing.T) {
	c := newTestStream()
	c.SetHeight(20)
	if c.height != 20 {
		t.Fatalf("expected height=20, got %d", c.height)
	}
}

// TestStreamComponent_SetHeight_Negative_ClampsToZero verifies negative values
// are clamped to 0.
func TestStreamComponent_SetHeight_Negative_ClampsToZero(t *testing.T) {
	c := newTestStream()
	c.SetHeight(-5)
	if c.height != 0 {
		t.Fatalf("expected height=0 for negative input, got %d", c.height)
	}
}

// --------------------------------------------------------------------------
// TestStreamComponent_SpinnerTick advances the frame counter.
// --------------------------------------------------------------------------

func TestStreamComponent_SpinnerTick_AdvancesFrame(t *testing.T) {
	c := newTestStream()

	// Enter spinner phase first.
	c = sendStreamMsg(c, app.SpinnerEvent{Show: true})
	initialFrame := c.spinnerFrame

	// Send a tick.
	_, cmd := c.Update(streamSpinnerTickMsg{})

	if c.spinnerFrame != initialFrame+1 {
		t.Fatalf("expected spinnerFrame=%d, got %d", initialFrame+1, c.spinnerFrame)
	}
	// The tick should re-schedule itself.
	if cmd == nil {
		t.Fatal("expected tick cmd to be re-scheduled in spinner phase")
	}
}

// TestStreamComponent_SpinnerTick_NoReschedule_WhenNotSpinner verifies that a
// tick in non-spinner phase does not re-schedule.
func TestStreamComponent_SpinnerTick_NoReschedule_WhenNotSpinner(t *testing.T) {
	c := newTestStream()
	// phase is idle — tick should be ignored.
	_, cmd := c.Update(streamSpinnerTickMsg{})
	if cmd != nil {
		t.Fatal("expected no tick reschedule when not in spinner phase")
	}
}

// ==========================================================================
// ApprovalComponent tests
// ==========================================================================

// --------------------------------------------------------------------------
// Helpers
// --------------------------------------------------------------------------

// newTestApproval creates a new ApprovalComponent with canned tool info.
func newTestApproval() *ApprovalComponent {
	return NewApprovalComponent("test_tool", `{"arg":"val"}`, 80)
}

// sendApprovalMsg calls component.Update and returns the updated component and cmd.
func sendApprovalMsg(a *ApprovalComponent, msg tea.Msg) (*ApprovalComponent, tea.Cmd) {
	m, cmd := a.Update(msg)
	return m.(*ApprovalComponent), cmd
}

// extractApprovalResult runs the cmd and returns the approvalResultMsg, or fails.
func extractApprovalResult(t *testing.T, cmd tea.Cmd) approvalResultMsg {
	t.Helper()
	if cmd == nil {
		t.Fatal("expected non-nil cmd from approval key press")
	}
	msg := cmd()
	result, ok := msg.(approvalResultMsg)
	if !ok {
		t.Fatalf("expected approvalResultMsg, got %T", msg)
	}
	return result
}

// --------------------------------------------------------------------------
// TestApprovalComponent_DefaultSelection is "yes".
// --------------------------------------------------------------------------

func TestApprovalComponent_DefaultSelection_IsYes(t *testing.T) {
	a := newTestApproval()
	if !a.selected {
		t.Fatal("expected default selection to be 'yes' (selected=true)")
	}
}

// --------------------------------------------------------------------------
// TestApprovalComponent_Y_ApproveEmitsTrue verifies that pressing 'y' emits
// approvalResultMsg{Approved: true}.
// --------------------------------------------------------------------------

func TestApprovalComponent_Y_ApproveEmitsTrue(t *testing.T) {
	a := newTestApproval()
	_, cmd := sendApprovalMsg(a, tea.KeyPressMsg{Code: 'y'})
	result := extractApprovalResult(t, cmd)
	if !result.Approved {
		t.Fatal("expected Approved=true for 'y' key press")
	}
}

// TestApprovalComponent_YUpper_ApproveEmitsTrue verifies that pressing 'Y'
// also emits approvalResultMsg{Approved: true}.
func TestApprovalComponent_YUpper_ApproveEmitsTrue(t *testing.T) {
	a := newTestApproval()
	_, cmd := sendApprovalMsg(a, tea.KeyPressMsg{Code: 'Y'})
	result := extractApprovalResult(t, cmd)
	if !result.Approved {
		t.Fatal("expected Approved=true for 'Y' key press")
	}
}

// --------------------------------------------------------------------------
// TestApprovalComponent_N_DenyEmitsFalse verifies that pressing 'n' emits
// approvalResultMsg{Approved: false}.
// --------------------------------------------------------------------------

func TestApprovalComponent_N_DenyEmitsFalse(t *testing.T) {
	a := newTestApproval()
	_, cmd := sendApprovalMsg(a, tea.KeyPressMsg{Code: 'n'})
	result := extractApprovalResult(t, cmd)
	if result.Approved {
		t.Fatal("expected Approved=false for 'n' key press")
	}
}

// TestApprovalComponent_NUpper_DenyEmitsFalse verifies that pressing 'N'
// also emits approvalResultMsg{Approved: false}.
func TestApprovalComponent_NUpper_DenyEmitsFalse(t *testing.T) {
	a := newTestApproval()
	_, cmd := sendApprovalMsg(a, tea.KeyPressMsg{Code: 'N'})
	result := extractApprovalResult(t, cmd)
	if result.Approved {
		t.Fatal("expected Approved=false for 'N' key press")
	}
}

// --------------------------------------------------------------------------
// TestApprovalComponent_Enter_ConfirmsSelection verifies that pressing enter
// confirms the currently selected option.
// --------------------------------------------------------------------------

func TestApprovalComponent_Enter_ConfirmsYesSelection(t *testing.T) {
	a := newTestApproval()
	a.selected = true // "yes" selected

	_, cmd := sendApprovalMsg(a, tea.KeyPressMsg{Code: tea.KeyEnter})
	result := extractApprovalResult(t, cmd)
	if !result.Approved {
		t.Fatal("expected Approved=true when 'yes' is selected and enter pressed")
	}
}

func TestApprovalComponent_Enter_ConfirmsNoSelection(t *testing.T) {
	a := newTestApproval()
	a.selected = false // "no" selected

	_, cmd := sendApprovalMsg(a, tea.KeyPressMsg{Code: tea.KeyEnter})
	result := extractApprovalResult(t, cmd)
	if result.Approved {
		t.Fatal("expected Approved=false when 'no' is selected and enter pressed")
	}
}

// --------------------------------------------------------------------------
// TestApprovalComponent_ESC_DeniesApproval verifies that pressing ESC emits
// approvalResultMsg{Approved: false} (same as "no").
// --------------------------------------------------------------------------

func TestApprovalComponent_ESC_DeniesApproval(t *testing.T) {
	a := newTestApproval()
	_, cmd := sendApprovalMsg(a, tea.KeyPressMsg{Code: tea.KeyEscape})
	result := extractApprovalResult(t, cmd)
	if result.Approved {
		t.Fatal("expected Approved=false for ESC")
	}
}

// --------------------------------------------------------------------------
// TestApprovalComponent_CtrlC_DeniesApproval verifies that ctrl+c emits
// approvalResultMsg{Approved: false}.
// --------------------------------------------------------------------------

func TestApprovalComponent_CtrlC_DeniesApproval(t *testing.T) {
	a := newTestApproval()
	_, cmd := sendApprovalMsg(a, tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	result := extractApprovalResult(t, cmd)
	if result.Approved {
		t.Fatal("expected Approved=false for ctrl+c")
	}
}

// --------------------------------------------------------------------------
// TestApprovalComponent_Left_SelectsYes verifies that pressing left navigates
// to "yes".
// --------------------------------------------------------------------------

func TestApprovalComponent_Left_SelectsYes(t *testing.T) {
	a := newTestApproval()
	a.selected = false // start on "no"

	a, cmd := sendApprovalMsg(a, tea.KeyPressMsg{Code: tea.KeyLeft})

	if !a.selected {
		t.Fatal("expected selected=true (yes) after pressing left")
	}
	if cmd != nil {
		t.Fatal("expected nil cmd from left arrow (just navigation)")
	}
}

// --------------------------------------------------------------------------
// TestApprovalComponent_Right_SelectsNo verifies that pressing right navigates
// to "no".
// --------------------------------------------------------------------------

func TestApprovalComponent_Right_SelectsNo(t *testing.T) {
	a := newTestApproval()
	a.selected = true // start on "yes"

	a, cmd := sendApprovalMsg(a, tea.KeyPressMsg{Code: tea.KeyRight})

	if a.selected {
		t.Fatal("expected selected=false (no) after pressing right")
	}
	if cmd != nil {
		t.Fatal("expected nil cmd from right arrow (just navigation)")
	}
}

// --------------------------------------------------------------------------
// TestApprovalComponent_WindowSizeMsg updates the width field.
// --------------------------------------------------------------------------

func TestApprovalComponent_WindowSizeMsg_UpdatesWidth(t *testing.T) {
	a := newTestApproval()

	a, _ = sendApprovalMsg(a, tea.WindowSizeMsg{Width: 120, Height: 40})

	if a.width != 120 {
		t.Fatalf("expected width=120, got %d", a.width)
	}
}

// --------------------------------------------------------------------------
// TestApprovalComponent_NoQuit verifies that the component never returns a
// tea.Quit cmd (parent manages lifecycle).
// --------------------------------------------------------------------------

func TestApprovalComponent_NoQuitFromYKey(t *testing.T) {
	a := newTestApproval()
	_, cmd := sendApprovalMsg(a, tea.KeyPressMsg{Code: 'y'})

	if cmd != nil {
		msg := runCmd(cmd)
		if _, ok := msg.(tea.QuitMsg); ok {
			t.Fatal("ApprovalComponent must not return tea.Quit (parent manages lifecycle)")
		}
	}
}

// --------------------------------------------------------------------------
// TestApprovalComponent_Init_ReturnsNil verifies Init() returns nil.
// --------------------------------------------------------------------------

func TestApprovalComponent_Init_ReturnsNil(t *testing.T) {
	a := newTestApproval()
	if cmd := a.Init(); cmd != nil {
		t.Fatal("expected Init() to return nil cmd")
	}
}
