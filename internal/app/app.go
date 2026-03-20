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

// queueItem holds a prompt and optional image attachments for the execution queue.
type queueItem struct {
	Prompt string
	Files  []fantasy.FilePart
}

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
	queue []queueItem

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
	return a.RunWithFiles(prompt, nil)
}

// RunWithFiles queues a multimodal prompt (text + image files) for execution.
// If the app is idle the prompt executes immediately; otherwise it is queued.
// Returns the current queue depth (0 = started immediately, >0 = queued).
//
// Satisfies ui.AppController (via RunWithImages which converts ImageAttachment
// to fantasy.FilePart).
func (a *App) RunWithFiles(prompt string, files []fantasy.FilePart) int {
	a.mu.Lock()

	if a.closed {
		a.mu.Unlock()
		return 0
	}

	item := queueItem{Prompt: prompt, Files: files}

	if a.busy {
		a.queue = append(a.queue, item)
		qLen := len(a.queue)
		a.mu.Unlock()
		return qLen
	}

	a.busy = true
	a.wg.Add(1)
	a.mu.Unlock()
	go a.drainQueue(item)
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

// Steer cancels the current agent step (if running), clears the queue, and
// sends a new message that will execute as soon as the current step finishes
// cancelling. If the agent is idle, the message executes immediately.
// This is the "steer" delivery mode for SendMessage.
func (a *App) Steer(prompt string) {
	a.mu.Lock()

	if a.closed {
		a.mu.Unlock()
		return
	}

	item := queueItem{Prompt: prompt}

	if !a.busy {
		// Not busy — start immediately, same as Run().
		a.busy = true
		a.wg.Add(1)
		a.mu.Unlock()
		go a.drainQueue(item)
		return
	}

	// Agent is busy: clear queue, insert steer message, then cancel.
	a.queue = []queueItem{item}
	cancel := a.cancelStep
	a.mu.Unlock()
	cancel()
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

// AddContextMessage adds a user-role message to the conversation history
// without triggering an LLM response. Used by the ! shell command prefix
// to inject command output into context so the LLM can reference it in
// subsequent turns.
//
// Satisfies ui.AppController.
func (a *App) AddContextMessage(text string) {
	msg := fantasy.NewUserMessage(text)
	a.store.Add(msg)

	// Persist to tree session if active.
	if ts := a.opts.TreeSession; ts != nil {
		_, _ = ts.AppendFantasyMessage(msg)
	}
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

	result, err := a.executeStep(stepCtx, prompt, nil, nil)
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

	return a.executeStep(stepCtx, prompt, nil, nil)
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

	result, err := a.executeStep(stepCtx, prompt, eventFn, nil)
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

// drainQueue runs in a goroutine. It collects all queued items (including the
// first one) and submits them together as a single batch. This ensures that
// when multiple messages are queued while the agent is working, they are all
// submitted together in one turn rather than sequentially.
// Must be called with a.busy == true and a.wg incremented.
func (a *App) drainQueue(first queueItem) {
	defer a.wg.Done()

	// Collect all items to process in this batch
	var items []queueItem
	items = append(items, first)

	// Process batches until no more items are queued
	for {
		// Drain the queue to collect any pending items
		a.mu.Lock()
		items = append(items, a.queue...)
		a.queue = a.queue[:0] // Clear the queue
		queueLen := len(a.queue)
		a.mu.Unlock()

		// Send queue updated event (queue is now empty)
		a.sendEvent(QueueUpdatedEvent{Length: queueLen})

		// Process all collected items as a single batch
		a.runQueueBatch(items)

		// Check if more items were queued while we were processing
		a.mu.Lock()
		hasMore := len(a.queue) > 0
		if hasMore {
			// Start a new batch with the newly queued items
			items = a.queue
			a.queue = a.queue[:0]
		}
		a.mu.Unlock()

		if !hasMore {
			// No more items, we're done
			break
		}
		// Process the new batch
	}

	// Mark as no longer busy
	a.mu.Lock()
	a.busy = false
	a.mu.Unlock()
}

// runQueueBatch executes multiple queue items as a single agent turn.
// All items are submitted together, and the agent responds once to the combined context.
func (a *App) runQueueBatch(items []queueItem) {
	if len(items) == 0 {
		return
	}

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

	// Execute the batch
	result, err := a.executeBatch(stepCtx, items, eventFn)
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

// runQueueItem executes a single queue item: adds the user message to the store,
// runs the agent step, and sends the appropriate event to the program.
// Deprecated: Use runQueueBatch which handles both single and multiple items.
func (a *App) runQueueItem(item queueItem) {
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

	result, err := a.executeStep(stepCtx, item.Prompt, eventFn, item.Files)
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
// PromptResult() (or PromptResultWithFiles for multimodal), which handles
// session persistence, hooks, extension events, and the generation loop.
func (a *App) executeStep(ctx context.Context, prompt string, eventFn func(tea.Msg), files []fantasy.FilePart) (*kit.TurnResult, error) {
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

	var result *kit.TurnResult
	var err error
	if len(files) > 0 {
		result, err = a.opts.Kit.PromptResultWithFiles(ctx, prompt, files)
	} else {
		result, err = a.opts.Kit.PromptResult(ctx, prompt)
	}
	if err != nil {
		return nil, err
	}

	// Sync in-memory store with the SDK's authoritative conversation.
	a.store.Replace(result.Messages)

	// Update usage tracker.
	a.updateUsageFromTurnResult(result, prompt)

	return result, nil
}

// executeBatch runs a batch of queue items as a single agent step by delegating
// to the SDK's PromptResultWithMessages(), which handles session persistence,
// hooks, extension events, and the generation loop.
func (a *App) executeBatch(ctx context.Context, items []queueItem, eventFn func(tea.Msg)) (*kit.TurnResult, error) {
	// Test hook: bypass SDK entirely (single item only for test compatibility).
	if a.opts.PromptFunc != nil {
		if len(items) == 1 {
			return a.opts.PromptFunc(ctx, items[0].Prompt)
		}
		// For batch mode with PromptFunc, just use the first item
		return a.opts.PromptFunc(ctx, items[0].Prompt)
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

	// Check if any items have file attachments
	hasFiles := false
	for _, item := range items {
		if len(item.Files) > 0 {
			hasFiles = true
			break
		}
	}

	var result *kit.TurnResult
	var err error

	if len(items) == 1 {
		// Single item: use the original path for compatibility
		item := items[0]
		if len(item.Files) > 0 || hasFiles {
			result, err = a.opts.Kit.PromptResultWithFiles(ctx, item.Prompt, item.Files)
		} else {
			result, err = a.opts.Kit.PromptResult(ctx, item.Prompt)
		}
	} else {
		// Multiple items: batch them together
		var messages []string
		for _, item := range items {
			messages = append(messages, item.Prompt)
		}

		// TODO: Handle file attachments in batch mode
		// For now, files are ignored in batch mode (rare edge case)
		if hasFiles {
			// If files exist, fall back to processing just the first item with files
			for _, item := range items {
				if len(item.Files) > 0 {
					result, err = a.opts.Kit.PromptResultWithFiles(ctx, item.Prompt, item.Files)
					break
				}
			}
		} else {
			result, err = a.opts.Kit.PromptResultWithMessages(ctx, messages)
		}
	}

	if err != nil {
		return nil, err
	}

	// Sync in-memory store with the SDK's authoritative conversation.
	a.store.Replace(result.Messages)

	// Update usage tracker (using last item's prompt for tracking).
	a.updateUsageFromTurnResult(result, items[len(items)-1].Prompt)

	return result, nil
}

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
			sendFn(ToolCallStartedEvent{ToolCallID: ev.ToolCallID, ToolName: ev.ToolName, ToolArgs: ev.ToolArgs})
		case kit.ToolExecutionStartEvent:
			sendFn(ToolExecutionEvent{ToolCallID: ev.ToolCallID, ToolName: ev.ToolName, ToolArgs: ev.ToolArgs, IsStarting: true})
		case kit.ToolExecutionEndEvent:
			sendFn(ToolExecutionEvent{ToolCallID: ev.ToolCallID, ToolName: ev.ToolName, IsStarting: false})
		case kit.ToolResultEvent:
			sendFn(ToolResultEvent{
				ToolCallID: ev.ToolCallID, ToolName: ev.ToolName, ToolArgs: ev.ToolArgs,
				Result: ev.Result, IsError: ev.IsError,
			})
		case kit.ToolCallContentEvent:
			sendFn(ToolCallContentEvent{Content: ev.Content})
		case kit.ResponseEvent:
			sendFn(ResponseCompleteEvent{Content: ev.Content})
		case kit.MessageUpdateEvent:
			sendFn(StreamChunkEvent{Content: ev.Chunk})
		case kit.ReasoningDeltaEvent:
			sendFn(ReasoningChunkEvent{Delta: ev.Delta})
		}
	}))

	return func() {
		for _, unsub := range unsubs {
			unsub()
		}
	}
}

// QuitFromExtension triggers a graceful shutdown. In interactive mode it
// sends a tea.QuitMsg to the program so the TUI exits cleanly. In
// non-interactive mode it cancels the root context, stopping any in-flight
// step. Safe to call from any goroutine; idempotent.
func (a *App) QuitFromExtension() {
	a.mu.Lock()
	prog := a.program
	a.mu.Unlock()
	if prog != nil {
		prog.Send(tea.QuitMsg{})
		return
	}
	// Non-interactive: cancel the root context.
	a.rootCancel()
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

// NotifyModelChanged sends a ModelChangedEvent to the TUI so it updates
// the model name in the status bar and message attribution.
func (a *App) NotifyModelChanged(provider, model string) {
	a.mu.Lock()
	prog := a.program
	a.mu.Unlock()
	if prog != nil {
		prog.Send(ModelChangedEvent{ProviderName: provider, ModelName: model})
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

// SuspendTUI temporarily releases the terminal from the TUI, runs the
// callback (which may spawn interactive subprocesses), and then restores
// the TUI. In non-interactive mode (no program registered) the callback
// runs directly with no terminal state changes.
//
// Safe to call from any goroutine (extension command handlers run in
// goroutines). Blocks until the callback returns.
func (a *App) SuspendTUI(callback func()) error {
	a.mu.Lock()
	prog := a.program
	a.mu.Unlock()
	if prog == nil {
		// Non-interactive: just run the callback directly.
		callback()
		return nil
	}
	if err := prog.ReleaseTerminal(); err != nil {
		return fmt.Errorf("release terminal: %w", err)
	}
	callback()
	if err := prog.RestoreTerminal(); err != nil {
		return fmt.Errorf("restore terminal: %w", err)
	}
	return nil
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
