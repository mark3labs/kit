package app

import (
	"context"
	"fmt"
	"sync"

	tea "charm.land/bubbletea/v2"
	"charm.land/fantasy"

	"github.com/mark3labs/kit/internal/extensions"
	"github.com/mark3labs/kit/internal/session"
	kit "github.com/mark3labs/kit/pkg/kit"
)

// App is the application-layer orchestrator. It owns the agentic loop,
// conversation history (via MessageStore), and queue management. It is
// designed to be created once per session and reused across multiple prompts.
//
// In interactive mode the caller creates a tea.Program and registers it via
// SetProgram; App then sends events to it as agent work progresses.
//
// In non-interactive mode the caller uses RunOnce, which writes the response
// directly to an io.Writer.
//
// App satisfies the ui.AppController interface defined in internal/ui/model.go:
//
//	Run(prompt string) int
//	CancelCurrentStep()
//	QueueLength() int
//	ClearQueue()
//	ClearMessages()
type App struct {
	opts Options

	// store holds the conversation history.
	store *MessageStore

	// program is the Bubble Tea program used to send events in interactive mode.
	// Nil in non-interactive mode.
	program *tea.Program

	// cancelStep cancels the current in-flight agent step. It is replaced on
	// each new step and called by CancelCurrentStep().
	cancelStep context.CancelFunc

	// mu protects busy, queue, and cancelStep.
	mu    sync.Mutex
	busy  bool
	queue []string

	// wg tracks in-flight goroutines; Close() waits on it.
	wg sync.WaitGroup

	// closed is set to true after Close() is called; new Run() calls are
	// silently dropped.
	closed bool

	// rootCtx/rootCancel are used to signal shutdown to all goroutines.
	rootCtx    context.Context
	rootCancel context.CancelFunc
}

// New creates a new App with the provided options and pre-loaded messages.
// initialMessages may be nil or empty for a fresh session.
func New(opts Options, initialMessages []fantasy.Message) *App {
	rootCtx, rootCancel := context.WithCancel(context.Background())
	return &App{
		opts:       opts,
		store:      NewMessageStoreWithMessages(initialMessages),
		rootCtx:    rootCtx,
		rootCancel: rootCancel,
		// cancelStep starts as a no-op so CancelCurrentStep() is always safe.
		cancelStep: func() {},
	}
}

// SetProgram registers the Bubble Tea program used to send events in
// interactive mode. Must be called before Run() in interactive mode.
func (a *App) SetProgram(p *tea.Program) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.program = p
}

// --------------------------------------------------------------------------
// AppController interface
// --------------------------------------------------------------------------

// Run queues a prompt for execution. If the app is idle the prompt is
// executed immediately in a background goroutine; otherwise it is appended
// to the queue.
//
// Returns the current queue depth after the operation: 0 means the prompt
// was started immediately (or the app is closed), >0 means it was queued.
// The caller is responsible for updating any UI state (e.g. queue badge)
// based on the returned value — Run does NOT send events to the program,
// because it may be called synchronously from within Bubble Tea's Update
// loop where prog.Send would deadlock.
//
// Satisfies ui.AppController.
func (a *App) Run(prompt string) int {
	a.mu.Lock()

	if a.closed {
		a.mu.Unlock()
		return 0
	}

	if a.busy {
		a.queue = append(a.queue, prompt)
		qLen := len(a.queue)
		a.mu.Unlock()
		return qLen
	}

	a.busy = true
	a.wg.Add(1)
	a.mu.Unlock()
	go a.drainQueue(prompt)
	return 0
}

// CancelCurrentStep cancels the currently executing agent step. Safe to call
// even when no step is running (it is a no-op in that case).
//
// Satisfies ui.AppController.
func (a *App) CancelCurrentStep() {
	a.mu.Lock()
	cancel := a.cancelStep
	a.mu.Unlock()
	cancel()
}

// QueueLength returns the number of prompts currently waiting in the queue.
//
// Satisfies ui.AppController.
func (a *App) QueueLength() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.queue)
}

// ClearQueue discards all queued prompts. The caller is responsible for
// updating any UI state (e.g. queue badge) — ClearQueue does NOT send
// events to the program, because it may be called synchronously from
// within Bubble Tea's Update loop where prog.Send would deadlock.
//
// Satisfies ui.AppController.
func (a *App) ClearQueue() {
	a.mu.Lock()
	a.queue = a.queue[:0]
	a.mu.Unlock()
}

// ClearMessages empties the conversation history. When a tree session is
// active the leaf pointer is reset to the root, creating an implicit branch.
//
// Satisfies ui.AppController.
func (a *App) ClearMessages() {
	a.store.Clear()
	if a.opts.TreeSession != nil {
		a.opts.TreeSession.ResetLeaf()
	}
}

// GetTreeSession returns the tree session manager, or nil if not configured.
func (a *App) GetTreeSession() *session.TreeManager {
	return a.opts.TreeSession
}

// CompactConversation summarises older messages to free context space. It
// returns an error synchronously if compaction cannot start (agent busy or
// app closed). The actual compaction runs in a background goroutine and
// delivers CompactCompleteEvent or CompactErrorEvent through the registered
// tea.Program. customInstructions is optional text appended to the summary
// prompt (e.g. "Focus on the API design decisions").
//
// Satisfies ui.AppController.
func (a *App) CompactConversation(customInstructions string) error {
	a.mu.Lock()
	if a.closed {
		a.mu.Unlock()
		return fmt.Errorf("app is closed")
	}
	if a.busy {
		a.mu.Unlock()
		return fmt.Errorf("cannot compact while the agent is working")
	}
	if a.opts.Kit == nil {
		a.mu.Unlock()
		return fmt.Errorf("SDK instance not available")
	}
	a.busy = true
	a.wg.Add(1)
	a.mu.Unlock()

	go func() {
		defer a.wg.Done()
		defer func() {
			a.mu.Lock()
			a.busy = false
			a.mu.Unlock()
		}()

		result, err := a.opts.Kit.Compact(a.rootCtx, nil, customInstructions)
		if err != nil {
			a.sendEvent(CompactErrorEvent{Err: err})
			return
		}
		if result == nil {
			a.sendEvent(CompactErrorEvent{Err: fmt.Errorf("nothing to compact")})
			return
		}

		// Sync in-memory store with the compacted session.
		if a.opts.TreeSession != nil {
			a.store.Replace(a.opts.TreeSession.GetFantasyMessages())
		}

		a.sendEvent(CompactCompleteEvent{
			Summary:         result.Summary,
			OriginalTokens:  result.OriginalTokens,
			CompactedTokens: result.CompactedTokens,
			MessagesRemoved: result.MessagesRemoved,
		})
	}()
	return nil
}

// --------------------------------------------------------------------------
// Non-interactive execution
// --------------------------------------------------------------------------

// RunOnce executes a single agent step synchronously and prints the final
// response text to stdout. No intermediate events are emitted. Blocks until
// the step completes or ctx is cancelled.
func (a *App) RunOnce(ctx context.Context, prompt string) error {
	stepCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	a.mu.Lock()
	a.cancelStep = cancel
	a.mu.Unlock()

	result, err := a.executeStep(stepCtx, prompt, nil)
	if err != nil {
		return err
	}

	if result.Response != "" {
		fmt.Println(result.Response)
	}
	return nil
}

// RunOnceResult executes a single agent step synchronously and returns the
// full TurnResult without printing anything. This is used by --json mode to
// capture structured output for serialization.
func (a *App) RunOnceResult(ctx context.Context, prompt string) (*kit.TurnResult, error) {
	stepCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	a.mu.Lock()
	a.cancelStep = cancel
	a.mu.Unlock()

	return a.executeStep(stepCtx, prompt, nil)
}

// RunOnceWithDisplay executes a single agent step synchronously, sending
// intermediate display events (spinner, tool calls, streaming chunks, etc.)
// to eventFn. This is the non-TUI equivalent of the interactive Run() path —
// used by non-interactive --prompt mode when output is needed.
//
// The eventFn receives the same event types as the Bubble Tea TUI
// (SpinnerEvent, ToolCallStartedEvent, StreamChunkEvent, StepCompleteEvent,
// etc.) and is responsible for rendering them.
//
// Blocks until the step completes or ctx is cancelled.
func (a *App) RunOnceWithDisplay(ctx context.Context, prompt string, eventFn func(tea.Msg)) error {
	stepCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	a.mu.Lock()
	a.cancelStep = cancel
	a.mu.Unlock()

	result, err := a.executeStep(stepCtx, prompt, eventFn)
	if err != nil {
		return err
	}

	// Send step complete so the display handler can render the final response.
	if eventFn != nil {
		eventFn(StepCompleteEvent{ResponseText: result.Response})
	}

	return nil
}

// --------------------------------------------------------------------------
// Close
// --------------------------------------------------------------------------

// Close signals all background goroutines to stop and waits for them to finish.
// After Close() returns it is safe to call Kit.Close() / agent.Close().
func (a *App) Close() {
	a.mu.Lock()
	if a.closed {
		a.mu.Unlock()
		return
	}
	a.closed = true
	cancel := a.cancelStep
	a.mu.Unlock()

	// Cancel any in-flight step and the root context.
	cancel()
	a.rootCancel()

	// Wait for background goroutines.
	a.wg.Wait()
}

// --------------------------------------------------------------------------
// Internal: queue drain loop
// --------------------------------------------------------------------------

// drainQueue runs in a goroutine. It executes the given prompt and then
// continues draining the queue until it is empty.
// Must be called with a.busy == true and a.wg incremented.
func (a *App) drainQueue(firstPrompt string) {
	defer a.wg.Done()

	prompt := firstPrompt
	for {
		a.runPrompt(prompt)

		a.mu.Lock()
		// Stop draining if the app is shutting down.
		if a.closed || a.rootCtx.Err() != nil {
			a.busy = false
			a.queue = a.queue[:0]
			a.mu.Unlock()
			return
		}
		if len(a.queue) == 0 {
			a.busy = false
			a.mu.Unlock()
			return
		}
		prompt = a.queue[0]
		a.queue = a.queue[1:]
		qLen := len(a.queue)
		a.mu.Unlock()
		// sendEvent must be called without a.mu held (see sendEvent comment).
		a.sendEvent(QueueUpdatedEvent{Length: qLen})
	}
}

// runPrompt executes a single prompt: adds the user message to the store,
// runs the agent step, and sends the appropriate event to the program.
func (a *App) runPrompt(prompt string) {
	// Create a per-step cancellable context.
	stepCtx, cancel := context.WithCancel(a.rootCtx)
	a.mu.Lock()
	a.cancelStep = cancel
	a.mu.Unlock()
	defer cancel()

	// Build event function that sends to the registered tea.Program (if any).
	a.mu.Lock()
	prog := a.program
	a.mu.Unlock()

	eventFn := func(msg tea.Msg) {
		if prog != nil {
			prog.Send(msg)
		}
	}

	result, err := a.executeStep(stepCtx, prompt, eventFn)
	if err != nil {
		if stepCtx.Err() != nil {
			// Step was cancelled by the user (e.g. double-ESC). Send a
			// cancellation event so the TUI can cut off the response
			// cleanly without printing an error.
			a.sendEvent(StepCancelledEvent{})
			return
		}
		a.sendEvent(StepErrorEvent{Err: err})
		return
	}

	a.sendEvent(StepCompleteEvent{ResponseText: result.Response})
}

// --------------------------------------------------------------------------
// Internal: single agent step
// --------------------------------------------------------------------------

// executeStep runs a single agentic step by delegating to the SDK's
// PromptResult(), which handles session persistence, hooks, extension
// events, and the generation loop.
func (a *App) executeStep(ctx context.Context, prompt string, eventFn func(tea.Msg)) (*kit.TurnResult, error) {
	// Test hook: bypass SDK entirely.
	if a.opts.PromptFunc != nil {
		return a.opts.PromptFunc(ctx, prompt)
	}

	sendFn := func(msg tea.Msg) {
		if eventFn != nil {
			eventFn(msg)
		}
	}

	// Subscribe to SDK events for TUI rendering. The subscription is
	// temporary — it lives only for the duration of this step.
	unsub := a.subscribeSDKEvents(sendFn)
	defer unsub()

	// Show spinner while the agent works.
	sendFn(SpinnerEvent{Show: true})

	result, err := a.opts.Kit.PromptResult(ctx, prompt)
	if err != nil {
		return nil, err
	}

	// Sync in-memory store with the SDK's authoritative conversation.
	a.store.Replace(result.Messages)

	// Update usage tracker.
	a.updateUsageFromTurnResult(result, prompt)

	return result, nil
}

// --------------------------------------------------------------------------
// Internal: event helpers
// --------------------------------------------------------------------------

// sendEvent sends a tea.Msg to the registered program if one is set.
// Must NOT be called with a.mu held (to avoid deadlock with the program).
func (a *App) sendEvent(msg tea.Msg) {
	a.mu.Lock()
	prog := a.program
	a.mu.Unlock()
	if prog != nil {
		prog.Send(msg)
	}
}

// subscribeSDKEvents registers temporary SDK event subscribers that convert
// SDK events to tea.Msg events and dispatch them via sendFn. Returns an
// unsubscribe function that removes all listeners.
func (a *App) subscribeSDKEvents(sendFn func(tea.Msg)) func() {
	k := a.opts.Kit
	var unsubs []func()

	unsubs = append(unsubs, k.Subscribe(func(e kit.Event) {
		switch ev := e.(type) {
		case kit.ToolCallEvent:
			sendFn(ToolCallStartedEvent{ToolName: ev.ToolName, ToolArgs: ev.ToolArgs})
		case kit.ToolExecutionStartEvent:
			sendFn(ToolExecutionEvent{ToolName: ev.ToolName, IsStarting: true})
		case kit.ToolExecutionEndEvent:
			sendFn(ToolExecutionEvent{ToolName: ev.ToolName, IsStarting: false})
		case kit.ToolResultEvent:
			sendFn(ToolResultEvent{
				ToolName: ev.ToolName, ToolArgs: ev.ToolArgs,
				Result: ev.Result, IsError: ev.IsError,
			})
		case kit.ToolCallContentEvent:
			sendFn(ToolCallContentEvent{Content: ev.Content})
		case kit.ResponseEvent:
			sendFn(ResponseCompleteEvent{Content: ev.Content})
		case kit.MessageUpdateEvent:
			sendFn(StreamChunkEvent{Content: ev.Chunk})
		}
	}))

	return func() {
		for _, unsub := range unsubs {
			unsub()
		}
	}
}

// PrintFromExtension outputs text from an extension to the user. The level
// controls styling: "" for plain text, "info" for a system message block,
// "error" for an error block. In interactive mode it sends an
// ExtensionPrintEvent through the program so the TUI can render it with the
// appropriate renderer. In non-interactive mode it falls back to stdout.
func (a *App) PrintFromExtension(level, text string) {
	a.mu.Lock()
	prog := a.program
	a.mu.Unlock()
	if prog != nil {
		prog.Send(ExtensionPrintEvent{Text: text, Level: level})
		return
	}
	// Non-interactive fallback: write directly to stdout.
	fmt.Println(text)
}

// SetEditorTextFromExtension sends an EditorTextSetEvent to the TUI to
// pre-fill the input editor. In non-interactive mode this is a no-op.
func (a *App) SetEditorTextFromExtension(text string) {
	a.mu.Lock()
	prog := a.program
	a.mu.Unlock()
	if prog != nil {
		prog.Send(EditorTextSetEvent{Text: text})
	}
}

// NotifyWidgetUpdate sends a WidgetUpdateEvent to the TUI so it re-renders
// extension widgets. Called from the extension context's SetWidget/RemoveWidget
// closures. In non-interactive mode this is a no-op (widgets are TUI-only).
func (a *App) NotifyWidgetUpdate() {
	a.mu.Lock()
	prog := a.program
	a.mu.Unlock()
	if prog != nil {
		prog.Send(WidgetUpdateEvent{})
	}
}

// SendEvent sends a tea.Msg to the registered program. Safe to call from
// any goroutine. No-op when no program is registered.
//
// Satisfies ui.AppController.
func (a *App) SendEvent(msg tea.Msg) {
	a.sendEvent(msg)
}

// SendPromptRequest sends a PromptRequestEvent to the TUI so the user can
// respond interactively. In non-interactive mode (no program registered) it
// immediately responds with a cancelled result via the channel, ensuring the
// calling extension goroutine never blocks indefinitely.
func (a *App) SendPromptRequest(evt PromptRequestEvent) {
	a.mu.Lock()
	prog := a.program
	a.mu.Unlock()
	if prog != nil {
		prog.Send(evt)
		return
	}
	// Non-interactive fallback: immediately cancel.
	if evt.ResponseCh != nil {
		evt.ResponseCh <- PromptResponse{Cancelled: true}
	}
}

// SendOverlayRequest sends an OverlayRequestEvent to the TUI so the user
// can interact with a modal overlay dialog. In non-interactive mode (no
// program registered) it immediately responds with a cancelled result via the
// channel, ensuring the calling extension goroutine never blocks indefinitely.
func (a *App) SendOverlayRequest(evt OverlayRequestEvent) {
	a.mu.Lock()
	prog := a.program
	a.mu.Unlock()
	if prog != nil {
		prog.Send(evt)
		return
	}
	// Non-interactive fallback: immediately cancel.
	if evt.ResponseCh != nil {
		evt.ResponseCh <- OverlayResponse{Cancelled: true}
	}
}

// PrintBlockFromExtension outputs a custom styled block from an extension.
func (a *App) PrintBlockFromExtension(opts extensions.PrintBlockOpts) {
	a.mu.Lock()
	prog := a.program
	a.mu.Unlock()
	if prog != nil {
		prog.Send(ExtensionPrintEvent{
			Text:        opts.Text,
			Level:       "block",
			BorderColor: opts.BorderColor,
			Subtitle:    opts.Subtitle,
		})
		return
	}
	// Non-interactive fallback.
	if opts.Subtitle != "" {
		fmt.Printf("%s\n  — %s\n", opts.Text, opts.Subtitle)
	} else {
		fmt.Println(opts.Text)
	}
}

// updateUsageFromTurnResult records token usage from an SDK TurnResult into the
// configured UsageTracker. This is the SDK-path equivalent of updateUsage.
func (a *App) updateUsageFromTurnResult(result *kit.TurnResult, userPrompt string) {
	if a.opts.UsageTracker == nil || result == nil {
		return
	}

	if result.TotalUsage != nil {
		inputTokens := int(result.TotalUsage.InputTokens)
		outputTokens := int(result.TotalUsage.OutputTokens)
		if inputTokens > 0 && outputTokens > 0 {
			cacheReadTokens := int(result.TotalUsage.CacheReadTokens)
			cacheWriteTokens := int(result.TotalUsage.CacheCreationTokens)
			a.opts.UsageTracker.UpdateUsage(inputTokens, outputTokens, cacheReadTokens, cacheWriteTokens)
		} else {
			a.opts.UsageTracker.EstimateAndUpdateUsage(userPrompt, result.Response)
			return
		}
	}

	if result.FinalUsage != nil {
		if ct := int(result.FinalUsage.InputTokens) + int(result.FinalUsage.OutputTokens); ct > 0 {
			a.opts.UsageTracker.SetContextTokens(ct)
		}
	}
}
