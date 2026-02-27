package app

import (
	"context"
	"fmt"
	"sync"

	tea "charm.land/bubbletea/v2"
	"charm.land/fantasy"

	"github.com/mark3labs/kit/internal/agent"
	"github.com/mark3labs/kit/internal/extensions"
	"github.com/mark3labs/kit/internal/session"
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

	// Record token usage for the completed step.
	a.updateUsage(result, prompt)

	responseText := ""
	if result.FinalResponse != nil {
		responseText = result.FinalResponse.Content.Text()
	}
	if responseText != "" {
		fmt.Println(responseText)
	}
	return nil
}

// RunOnceWithDisplay executes a single agent step synchronously, sending
// intermediate display events (spinner, tool calls, streaming chunks, etc.)
// to eventFn. This is the non-TUI equivalent of the interactive Run() path —
// used by script mode and non-interactive --prompt mode when output is needed.
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

	// Record token usage for the completed step.
	a.updateUsage(result, prompt)

	// Send step complete so the display handler can render the final response.
	if eventFn != nil && result.FinalResponse != nil {
		eventFn(StepCompleteEvent{
			Response: result.FinalResponse,
			Usage:    result.TotalUsage,
		})
	}

	return nil
}

// --------------------------------------------------------------------------
// Close
// --------------------------------------------------------------------------

// Close signals all background goroutines to stop and waits for them to finish.
// After Close() returns it is safe to call agent.Close().
func (a *App) Close() {
	a.mu.Lock()
	if a.closed {
		a.mu.Unlock()
		return
	}
	a.closed = true
	cancel := a.cancelStep
	a.mu.Unlock()

	// --- Extension: SessionShutdown ---
	if a.opts.Extensions != nil && a.opts.Extensions.HasHandlers(extensions.SessionShutdown) {
		_, _ = a.opts.Extensions.Emit(extensions.SessionShutdownEvent{})
	}

	// Cancel any in-flight step and the root context.
	cancel()
	a.rootCancel()

	// Wait for background goroutines.
	a.wg.Wait()

	// Close tree session file handle.
	if a.opts.TreeSession != nil {
		_ = a.opts.TreeSession.Close()
	}
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

	// Record token usage for the completed step.
	a.updateUsage(result, prompt)

	a.sendEvent(StepCompleteEvent{
		Response: result.FinalResponse,
		Usage:    result.TotalUsage,
	})
}

// --------------------------------------------------------------------------
// Internal: single agent step
// --------------------------------------------------------------------------

// executeStep runs a single agentic step using the agent in opts.
// It adds the user prompt to the MessageStore before calling the agent, and
// replaces the store with the full updated conversation on success.
// eventFn receives intermediate display events (tool calls, streaming chunks,
// etc.); it may be nil when no display is needed (e.g. quiet RunOnce).
func (a *App) executeStep(ctx context.Context, prompt string, eventFn func(tea.Msg)) (*agent.GenerateWithLoopResult, error) {
	sendFn := func(msg tea.Msg) {
		if eventFn != nil {
			eventFn(msg)
		}
	}

	// --- Extension: Input event (can transform or handle the prompt) ---
	if a.opts.Extensions != nil && a.opts.Extensions.HasHandlers(extensions.Input) {
		result, _ := a.opts.Extensions.Emit(extensions.InputEvent{
			Text:   prompt,
			Source: a.inputSource(),
		})
		if r, ok := result.(extensions.InputResult); ok {
			switch r.Action {
			case "transform":
				prompt = r.Text
			case "handled":
				// Extension handled the input; skip the agent entirely.
				return &agent.GenerateWithLoopResult{}, nil
			}
		}
	}

	// Add user message to the store immediately so history is consistent
	// even if the step is later cancelled.
	userMsg := fantasy.NewUserMessage(prompt)
	a.store.Add(userMsg)

	// Persist user message to tree session if configured.
	if a.opts.TreeSession != nil {
		_, _ = a.opts.TreeSession.AppendFantasyMessage(userMsg)
	}

	// Build the full message slice for the agent call.
	// When a tree session is active, build context from the tree (walks
	// leaf-to-root) so that only the current branch is sent to the LLM.
	var msgs []fantasy.Message
	if a.opts.TreeSession != nil {
		msgs = a.opts.TreeSession.GetFantasyMessages()
	} else {
		msgs = a.store.GetAll()
	}

	// Track message count before agent runs so we can diff new messages.
	sentCount := len(msgs)

	// --- Extension: BeforeAgentStart ---
	// Extensions can inject a system message or prepend context text into the
	// conversation before the agent runs.
	if a.opts.Extensions != nil && a.opts.Extensions.HasHandlers(extensions.BeforeAgentStart) {
		result, _ := a.opts.Extensions.Emit(extensions.BeforeAgentStartEvent{Prompt: prompt})
		if r, ok := result.(extensions.BeforeAgentStartResult); ok {
			if r.SystemPrompt != nil && *r.SystemPrompt != "" {
				// Prepend a system message so the LLM sees extension-provided
				// instructions. This supplements (not replaces) the agent's
				// configured system prompt.
				msgs = append([]fantasy.Message{fantasy.NewSystemMessage(*r.SystemPrompt)}, msgs...)
			}
			if r.InjectText != nil && *r.InjectText != "" {
				// Prepend a user message with the injected context so it
				// appears early in the conversation window.
				msgs = append([]fantasy.Message{fantasy.NewUserMessage(*r.InjectText)}, msgs...)
			}
		}
	}

	// --- Extension: AgentStart ---
	if a.opts.Extensions != nil && a.opts.Extensions.HasHandlers(extensions.AgentStart) {
		_, _ = a.opts.Extensions.Emit(extensions.AgentStartEvent{Prompt: prompt})
	}

	// Signal spinner start.
	sendFn(SpinnerEvent{Show: true})

	// --- Extension: MessageStart ---
	if a.opts.Extensions != nil && a.opts.Extensions.HasHandlers(extensions.MessageStart) {
		_, _ = a.opts.Extensions.Emit(extensions.MessageStartEvent{})
	}

	result, err := a.opts.Agent.GenerateWithLoopAndStreaming(ctx, msgs,
		// onToolCall
		func(toolName, toolArgs string) {
			sendFn(ToolCallStartedEvent{ToolName: toolName, ToolArgs: toolArgs})
		},
		// onToolExecution
		func(toolName string, isStarting bool) {
			sendFn(ToolExecutionEvent{ToolName: toolName, IsStarting: isStarting})
		},
		// onToolResult
		func(toolName, toolArgs, result string, isError bool) {
			sendFn(ToolResultEvent{
				ToolName: toolName,
				ToolArgs: toolArgs,
				Result:   result,
				IsError:  isError,
			})
		},
		// onResponse (final non-streaming response)
		func(content string) {
			sendFn(ResponseCompleteEvent{Content: content})
		},
		// onToolCallContent
		func(content string) {
			sendFn(ToolCallContentEvent{Content: content})
		},
		// onStreamingResponse — spinner keeps running alongside streaming text
		func(chunk string) {
			// Extension: MessageUpdate (observe streaming chunks)
			if a.opts.Extensions != nil && a.opts.Extensions.HasHandlers(extensions.MessageUpdate) {
				_, _ = a.opts.Extensions.Emit(extensions.MessageUpdateEvent{Chunk: chunk})
			}
			sendFn(StreamChunkEvent{Content: chunk})
		},
	)

	if err != nil {
		// --- Extension: AgentEnd with error ---
		if a.opts.Extensions != nil && a.opts.Extensions.HasHandlers(extensions.AgentEnd) {
			_, _ = a.opts.Extensions.Emit(extensions.AgentEndEvent{
				Response:   "",
				StopReason: "error",
			})
		}
		return nil, err
	}

	// --- Extension: MessageEnd ---
	responseText := ""
	if result.FinalResponse != nil {
		responseText = result.FinalResponse.Content.Text()
	}
	if a.opts.Extensions != nil && a.opts.Extensions.HasHandlers(extensions.MessageEnd) {
		_, _ = a.opts.Extensions.Emit(extensions.MessageEndEvent{Content: responseText})
	}

	// --- Extension: AgentEnd with success ---
	if a.opts.Extensions != nil && a.opts.Extensions.HasHandlers(extensions.AgentEnd) {
		_, _ = a.opts.Extensions.Emit(extensions.AgentEndEvent{
			Response:   responseText,
			StopReason: "completed",
		})
	}

	// Replace the store with the full updated conversation returned by the agent
	// (includes tool call/result messages added during the step).
	a.store.Replace(result.ConversationMessages)

	// Persist new messages (tool calls, tool results, assistant response)
	// to the tree session. Only append messages beyond what we sent.
	if a.opts.TreeSession != nil && len(result.ConversationMessages) > sentCount {
		for _, msg := range result.ConversationMessages[sentCount:] {
			_, _ = a.opts.TreeSession.AppendFantasyMessage(msg)
		}
	}

	return result, nil
}

// inputSource returns a string identifying how the current session receives
// input — used by the Input extension event.
func (a *App) inputSource() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.program != nil {
		return "interactive"
	}
	return "cli"
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

// updateUsage records token usage from a completed agent step into the configured
// UsageTracker (if any). It uses the actual token counts from the agent result's
// TotalUsage field when available; otherwise it falls back to text-based estimation.
//
// TotalUsage is the sum across all tool-calling steps in a single agent run and
// is used for session cost tracking. For context window utilization we use the
// final response's per-call usage (FinalResponse.Usage) which reflects the actual
// context size at the last API call.
func (a *App) updateUsage(result *agent.GenerateWithLoopResult, userPrompt string) {
	if a.opts.UsageTracker == nil || result == nil {
		return
	}

	usage := result.TotalUsage
	inputTokens := int(usage.InputTokens)
	outputTokens := int(usage.OutputTokens)
	if inputTokens > 0 && outputTokens > 0 {
		cacheReadTokens := int(usage.CacheReadTokens)
		cacheWriteTokens := int(usage.CacheCreationTokens)
		a.opts.UsageTracker.UpdateUsage(inputTokens, outputTokens, cacheReadTokens, cacheWriteTokens)
	} else {
		// Fall back to text-based estimation when the provider omits token counts.
		responseText := ""
		if result.FinalResponse != nil {
			responseText = result.FinalResponse.Content.Text()
		}
		a.opts.UsageTracker.EstimateAndUpdateUsage(userPrompt, responseText)
		return // EstimateAndUpdateUsage already sets context tokens internally
	}

	// Set context window utilization from the final API call's per-step usage.
	// FinalResponse.Usage represents the last step only (not the aggregate),
	// so input+output there reflects the actual context fill level.
	if result.FinalResponse != nil {
		fu := result.FinalResponse.Usage
		if ct := int(fu.InputTokens) + int(fu.OutputTokens); ct > 0 {
			a.opts.UsageTracker.SetContextTokens(ct)
		}
	}
}
