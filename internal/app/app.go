package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"sync"
	"sync/atomic"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/fantasy"

	"github.com/mark3labs/kit/internal/extensions"
	"github.com/mark3labs/kit/internal/message"
	"github.com/mark3labs/kit/internal/session"
	kit "github.com/mark3labs/kit/pkg/kit"
)

// queueItem holds a prompt and optional image attachments for the execution queue.
type queueItem struct {
	Prompt string
	Files  []kit.LLMFilePart
}

// ErrAgentBusy is returned when an operation cannot proceed because the agent
// is still processing a turn (including any post-turn extension hooks) and did
// not become idle before the operation's deadline.
//
// This is an alias for extensions.ErrAgentBusy so the extension API and the
// app layer share a single sentinel value — callers can detect the condition
// with errors.Is(err, app.ErrAgentBusy) without substring-matching the error
// message.
var ErrAgentBusy = extensions.ErrAgentBusy

// DefaultNewSessionIdleWait bounds how long RequestNewSessionFromExtension
// will block waiting for the agent to settle. It needs to be generous enough
// to cover real-world post-turn tooling (project formatters, on-save linters,
// hidden tool calls) which routinely hold the busy flag for seconds and
// occasionally minutes — yet still short enough to surface a wedged agent.
//
// Issue #63 reported workloads where the busy window regularly exceeded
// 6 seconds; ten minutes is the same bound the workaround in that issue used.
const DefaultNewSessionIdleWait = 10 * time.Minute

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

	// mu protects busy, queue, cancelStep, and idleCh.
	mu    sync.Mutex
	busy  bool
	queue []queueItem

	// idleCh is closed when the agent transitions from busy back to idle.
	// While the agent is idle the channel is already closed (recv returns
	// immediately). When busy transitions to true a fresh open channel is
	// allocated so callers blocked on the previous one are released. All
	// transitions are funnelled through setBusyLocked to keep the channel
	// pointer in sync with the busy flag.
	//
	// This is the underlying primitive WaitForIdle and
	// RequestNewSessionFromExtension wait on to fix the AgentEnd→NewSession
	// race described in issue #63: AgentEnd is emitted from inside the agent
	// loop, before drainQueue clears busy, so any extension hook that calls
	// ctx.NewSession synchronously would otherwise observe busy==true.
	idleCh chan struct{}

	// wg tracks in-flight goroutines; Close() waits on it.
	wg sync.WaitGroup

	// closed is set to true after Close() is called; new Run() calls are
	// silently dropped.
	closed bool

	// rootCtx/rootCancel are used to signal shutdown to all goroutines.
	rootCtx    context.Context
	rootCancel context.CancelFunc

	// widgetUpdatePending is set to true while a WidgetUpdateEvent burst is
	// being coalesced. The leading edge fires immediately; subsequent calls
	// within the debounce window set widgetUpdateTrailing so a final event
	// is delivered with the latest runner state at the end of the window.
	// Without the trailing send, a rapid SetWidget→RemoveWidget pair (e.g.
	// SubagentEnd pushing a final frame then removing the widget) would let
	// the second call get silently dropped, leaving the TUI's layout stuck
	// on the pre-removal widget height — visible as empty rows below the
	// status bar after the widget disappears.
	widgetUpdatePending  atomic.Bool
	widgetUpdateTrailing atomic.Bool

	// steerDrainFn is the test seam used by releaseBusyAfterCompact to pull
	// any steer messages that arrived during compaction. In production it is
	// nil and the helper falls back to a.opts.Kit.DrainSteer(); tests that
	// need to exercise the steer-drain path without standing up a full
	// *kit.Kit can set this field directly to inject fake items.
	steerDrainFn func() []queueItem
}

// New creates a new App with the provided options and pre-loaded messages.
// initialMessages may be nil or empty for a fresh session.
func New(opts Options, initialMessages []kit.LLMMessage) *App {
	rootCtx, rootCancel := context.WithCancel(context.Background())
	// idleCh starts already closed: the freshly constructed App is idle, so
	// any caller blocking on it via WaitForIdle should be released immediately.
	idleCh := make(chan struct{})
	close(idleCh)
	return &App{
		opts:       opts,
		store:      NewMessageStoreWithMessages(initialMessages),
		rootCtx:    rootCtx,
		rootCancel: rootCancel,
		// cancelStep starts as a no-op so CancelCurrentStep() is always safe.
		cancelStep: func() {},
		idleCh:     idleCh,
	}
}

// setBusyLocked is the single chokepoint for mutating a.busy. It keeps the
// idleCh signalling channel in sync with the busy flag:
//
//   - false → true: allocate a fresh open channel so future WaitForIdle
//     callers block until the next idle transition.
//   - true  → false: close the current channel so any waiters wake up.
//
// No-op when the requested state already matches. The caller must hold a.mu.
func (a *App) setBusyLocked(busy bool) {
	if a.busy == busy {
		return
	}
	a.busy = busy
	if busy {
		a.idleCh = make(chan struct{})
	} else {
		close(a.idleCh)
	}
}

// idleSnapshot returns the current busy state and the channel that will be
// closed on the next idle transition. The snapshot is taken under a.mu so the
// pair is consistent (busy==true ⇒ ch is the open channel for *this* busy
// cycle, not a stale one).
func (a *App) idleSnapshot() (busy bool, ch chan struct{}) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.busy, a.idleCh
}

// WaitForIdle blocks until the agent is idle, the given timeout elapses, or
// the app shuts down. Returns nil on idle, ErrAgentBusy on timeout, or the
// rootCtx error if the app is closing.
//
// A non-positive timeout disables the deadline and waits indefinitely (until
// idle or app shutdown). Safe to call from any goroutine, but never from
// inside the Bubble Tea Update() loop — it blocks.
//
// Idiomatic use from extensions:
//
//	if err := app.WaitForIdle(0); err != nil { /* shutdown */ }
//
// The loop guards against the agent re-arming itself between wakeups: if
// another prompt is queued (or a steer message lands) while we're waiting,
// setBusyLocked allocates a fresh idleCh and we wait again.
func (a *App) WaitForIdle(timeout time.Duration) error {
	var deadline time.Time
	if timeout > 0 {
		deadline = time.Now().Add(timeout)
	}
	for {
		busy, ch := a.idleSnapshot()
		if !busy {
			return nil
		}
		var timer *time.Timer
		var timerCh <-chan time.Time
		if timeout > 0 {
			remaining := time.Until(deadline)
			if remaining <= 0 {
				return ErrAgentBusy
			}
			timer = time.NewTimer(remaining)
			timerCh = timer.C
		}
		select {
		case <-ch:
			// Idle transition observed — loop and re-check under the
			// mutex in case a new busy cycle started immediately after.
		case <-timerCh:
			return ErrAgentBusy
		case <-a.rootCtx.Done():
			if timer != nil {
				timer.Stop()
			}
			return a.rootCtx.Err()
		}
		if timer != nil {
			timer.Stop()
		}
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
// Satisfies ui.AppController.
func (a *App) RunWithFiles(prompt string, files []kit.LLMFilePart) int {
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

	a.setBusyLocked(true)
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

// IsBusy returns true when the agent is currently processing a turn.
func (a *App) IsBusy() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.busy
}

// Abort cancels the current agent step (if running) and clears the queue.
// Unlike InterruptAndSend, no new message is injected — the agent simply
// stops. Safe to call when idle (no-op).
func (a *App) Abort() {
	a.mu.Lock()
	a.queue = a.queue[:0]
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

// Steer injects a steering message into the currently running agent turn.
// If the agent is in a multi-step tool loop, the message is delivered after
// the current tool execution finishes but before the next LLM call (graceful
// mid-turn injection via Fantasy's PrepareStep). If the agent is streaming
// a text-only response (no pending tool calls), the message waits until the
// response completes and then executes as the next turn.
//
// If the agent is idle, the message starts executing immediately (same as Run).
//
// Returns the number of pending steer/queue items (0 = started immediately,
// >0 = injected/queued). The caller must update UI state based on the return
// value — Steer does NOT send events to the program to avoid deadlocking
// when called from within Update().
//
// Satisfies ui.AppController.
func (a *App) Steer(prompt string) int {
	return a.SteerWithFiles(prompt, nil)
}

// SteerWithFiles injects a steering message with optional file attachments
// (e.g. pasted images) into the currently running agent turn. Behaves like
// Steer but includes file parts alongside the text.
//
// Satisfies ui.AppController.
func (a *App) SteerWithFiles(prompt string, files []kit.LLMFilePart) int {
	a.mu.Lock()

	if a.closed {
		a.mu.Unlock()
		return 0
	}

	if !a.busy {
		// Not busy — start immediately, same as RunWithFiles().
		item := queueItem{Prompt: prompt, Files: files}
		a.setBusyLocked(true)
		a.wg.Add(1)
		a.mu.Unlock()
		go a.drainQueue(item)
		return 0
	}

	a.mu.Unlock()

	// Agent is busy — inject via the SDK's steer channel. The message
	// will be picked up by PrepareStep between agent steps (after tool
	// execution, before next LLM call). If PrepareStep doesn't fire
	// (text-only response), drainQueue will pick it up after the turn.
	if a.opts.Kit != nil {
		a.opts.Kit.InjectSteerWithFiles(prompt, files)
	}
	return 1
}

// InterruptAndSend cancels the current agent step (if running), clears the
// queue, and sends a new message that will execute as soon as the current
// step finishes cancelling. If the agent is idle, the message executes
// immediately. This is the hard-cancel delivery mode used by extensions'
// CancelAndSend.
func (a *App) InterruptAndSend(prompt string) {
	a.mu.Lock()

	if a.closed {
		a.mu.Unlock()
		return
	}

	item := queueItem{Prompt: prompt}

	if !a.busy {
		// Not busy — start immediately, same as Run().
		a.setBusyLocked(true)
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

// ReloadMessagesFromTree clears the in-memory message store and reloads it
// from the tree session's current branch. Unlike ClearMessages, this does NOT
// reset the tree session's leaf pointer. Used after Branch() to sync the
// store with the new branch position.
func (a *App) ReloadMessagesFromTree() {
	a.store.Clear()
	if a.opts.TreeSession != nil {
		a.store.Replace(a.opts.TreeSession.GetLLMMessages())
	}
}

// GetTreeSession returns the tree session manager, or nil if not configured.
func (a *App) GetTreeSession() *session.TreeManager {
	return a.opts.TreeSession
}

// SwitchTreeSession replaces the active tree session with a new one and
// reloads the in-memory message store from the new session's messages.
// The old tree session is closed. Used by /resume to switch sessions.
func (a *App) SwitchTreeSession(ts *session.TreeManager) {
	// Close old session.
	if old := a.opts.TreeSession; old != nil {
		_ = old.Close()
	}
	a.opts.TreeSession = ts
	// Also update the kit SDK's tree session so messages are persisted correctly.
	if a.opts.Kit != nil {
		a.opts.Kit.SetTreeSession(ts)
	}
	// Reload messages from new session.
	a.store.Clear()
	if ts != nil {
		a.store.Replace(ts.GetLLMMessages())
	}
}

// PopLastUserMessage truncates the tree session back to the parent of the
// most recent user message on the current branch, syncs the in-memory
// message store, and returns the user prompt text plus any image file
// parts so the caller can resubmit via Run/RunWithFiles.
//
// This is the building block for /retry: the user message and any orphaned
// assistant/tool entries produced by a failed turn become unreachable on
// the current branch (they remain in the session file under a different
// leaf) and are excluded from the next LLM context.
//
// Returns an error when:
//   - the agent is currently working (busy)
//   - the app has been closed
//   - no tree session is active (sessions disabled via --no-session)
//   - no user message exists on the current branch
//
// Satisfies ui.AppController.
func (a *App) PopLastUserMessage() (string, []kit.LLMFilePart, error) {
	a.mu.Lock()
	if a.closed {
		a.mu.Unlock()
		return "", nil, fmt.Errorf("app is closed")
	}
	if a.busy {
		a.mu.Unlock()
		return "", nil, fmt.Errorf("cannot retry while the agent is working")
	}
	a.mu.Unlock()

	ts := a.opts.TreeSession
	if ts == nil {
		return "", nil, fmt.Errorf("no tree session active; /retry requires a session")
	}

	// Walk the current branch backwards to find the most recent user message.
	branch := ts.GetBranch("")
	var target *session.MessageEntry
	for i := len(branch) - 1; i >= 0; i-- {
		me, ok := branch[i].(*session.MessageEntry)
		if !ok {
			continue
		}
		if me.Role == string(message.RoleUser) {
			target = me
			break
		}
	}
	if target == nil {
		return "", nil, fmt.Errorf("no user message to retry")
	}

	// Extract the prompt text and any image parts from the target entry.
	msg, err := target.ToMessage()
	if err != nil {
		return "", nil, fmt.Errorf("decode user message: %w", err)
	}
	prompt := msg.Content()
	var files []kit.LLMFilePart
	for _, part := range msg.Parts {
		if ic, ok := part.(message.ImageContent); ok {
			files = append(files, kit.LLMFilePart{
				Data:      ic.Data,
				MediaType: ic.MediaType,
			})
		}
	}

	// Move the leaf to the parent of the user message. The failed turn's
	// entries (user message + any partial assistant/tool entries) are still
	// in the tree file but no longer on the active branch, so they will not
	// be re-sent to the LLM. runTurn() will append a fresh user message on
	// the next call.
	if err := ts.Branch(target.ParentID); err != nil {
		return "", nil, fmt.Errorf("branch to parent: %w", err)
	}

	// Sync the in-memory store with the new branch position so subsequent
	// reads (and ReloadMessagesFromTree() consumers) see the truncated view.
	a.store.Clear()
	a.store.Replace(ts.GetLLMMessages())

	return prompt, files, nil
}

// AddContextMessage adds a user-role message to the conversation history
// without triggering an LLM response. Used by the ! shell command prefix
// to inject command output into context so the LLM can reference it in
// subsequent turns.
//
// Satisfies ui.AppController.
func (a *App) AddContextMessage(text string) {
	kitMsg := fantasy.NewUserMessage(text)
	a.store.Add(kitMsg)

	// Persist to tree session if active.
	if ts := a.opts.TreeSession; ts != nil {
		_, _ = ts.AppendLLMMessage(fantasy.NewUserMessage(text))
	}
}

// CompactConversation summarises older messages to free context space. It
// returns an error synchronously if compaction cannot start (agent busy or
// app closed). The actual compaction runs in a background goroutine and
// delivers CompactCompleteEvent or CompactErrorEvent through the registered
// tea.Program. customInstructions is optional text appended to the summary
// prompt (e.g. "Focus on the API design decisions").
//
// Any prompts queued via Run/RunWithFiles or steering messages injected via
// Steer/SteerWithFiles while compaction is running are flushed automatically
// once compaction completes (see releaseBusyAfterCompact).
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
	a.setBusyLocked(true)
	a.wg.Add(1)
	a.mu.Unlock()

	go func() {
		defer a.wg.Done()
		defer a.releaseBusyAfterCompact()

		// Subscribe to SDK events for streaming compaction summary to the TUI.
		sendFn := func(msg tea.Msg) {
			if a.program != nil {
				a.program.Send(msg)
			}
		}
		unsub := a.subscribeSDKEvents(sendFn, nil)
		defer unsub()

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
			a.store.Replace(a.opts.TreeSession.GetLLMMessages())
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

// CompactAsync is like CompactConversation but calls onComplete/onError
// callbacks instead of sending TUI events. Used by the extension API's
// ctx.Compact() which needs callback-based notification.
//
// Like CompactConversation, any prompts/steer messages received during
// compaction are flushed automatically once compaction finishes.
func (a *App) CompactAsync(customInstructions string, onComplete func(), onError func(string)) error {
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
	a.setBusyLocked(true)
	a.wg.Add(1)
	a.mu.Unlock()

	go func() {
		defer a.wg.Done()
		defer a.releaseBusyAfterCompact()

		// Subscribe to SDK events for streaming compaction summary to the TUI.
		sendFn := func(msg tea.Msg) {
			if a.program != nil {
				a.program.Send(msg)
			}
		}
		unsub := a.subscribeSDKEvents(sendFn, nil)
		defer unsub()

		result, err := a.opts.Kit.Compact(a.rootCtx, nil, customInstructions)
		if err != nil {
			a.sendEvent(CompactErrorEvent{Err: err})
			if onError != nil {
				onError(err.Error())
			}
			return
		}
		if result == nil {
			a.sendEvent(CompactErrorEvent{Err: fmt.Errorf("nothing to compact")})
			if onError != nil {
				onError("nothing to compact")
			}
			return
		}

		// Sync in-memory store with the compacted session.
		if a.opts.TreeSession != nil {
			a.store.Replace(a.opts.TreeSession.GetLLMMessages())
		}

		a.sendEvent(CompactCompleteEvent{
			Summary:         result.Summary,
			OriginalTokens:  result.OriginalTokens,
			CompactedTokens: result.CompactedTokens,
			MessagesRemoved: result.MessagesRemoved,
		})
		if onComplete != nil {
			onComplete()
		}
	}()
	return nil
}

// releaseBusyAfterCompact is the deferred tail that runs at the end of every
// compaction goroutine (success, error, or panic-after-recover paths). It
// flips a.busy back to false, but before doing so it checks whether any
// prompts piled up while compaction was running:
//
//   - Run/RunWithFiles append to a.queue when a.busy is set.
//   - Steer/SteerWithFiles deposit messages into the SDK steer channel via
//     Kit.InjectSteerWithFiles when a.busy is set.
//
// Without this hand-off the queue would sit idle until the user submits
// another prompt — see issue #27. If we find anything pending we keep busy
// set, splice the steer messages to the front of the queue, and start a
// fresh drainQueue goroutine to deliver them as a single batched turn.
func (a *App) releaseBusyAfterCompact() {
	// Pull steer messages outside the app mutex; DrainSteer takes its own
	// internal lock and we don't want to nest the two. The test seam
	// (a.steerDrainFn) takes precedence so unit tests can inject fake
	// steer items without a real *kit.Kit.
	var steerItems []queueItem
	switch {
	case a.steerDrainFn != nil:
		steerItems = a.steerDrainFn()
	case a.opts.Kit != nil:
		if leftover := a.opts.Kit.DrainSteer(); len(leftover) > 0 {
			steerItems = make([]queueItem, len(leftover))
			for i, sm := range leftover {
				steerItems[i] = queueItem{Prompt: sm.Text, Files: sm.Files}
			}
		}
	}

	a.mu.Lock()
	// If the app was closed while compaction was running, drop everything
	// and just clear busy. Run/Steer would have rejected new items already
	// after Close(), but this guards against in-flight items that slipped
	// in just before closed was set.
	if a.closed {
		a.queue = a.queue[:0]
		a.setBusyLocked(false)
		a.mu.Unlock()
		return
	}

	// Combine steer-channel items (front) with the in-memory queue (back).
	// Steer messages are placed first so they retain their "act now"
	// semantics relative to ordinary queued prompts that arrived later.
	pending := append(steerItems, a.queue...)
	a.queue = a.queue[:0]

	if len(pending) == 0 {
		a.setBusyLocked(false)
		a.mu.Unlock()
		return
	}

	// Hand off to drainQueue: it will pick up the first item directly and
	// scoop the rest from a.queue on its first iteration.
	first := pending[0]
	if len(pending) > 1 {
		a.queue = append(a.queue, pending[1:]...)
	}
	// Stay busy across the goroutine swap.
	a.wg.Add(1)
	a.mu.Unlock()

	// Notify the UI that steer-channel messages were consumed so the
	// steering badge can clear; ordinary queued prompts will be reflected
	// by the QueueUpdatedEvent that drainQueue emits as it picks them up.
	if len(steerItems) > 0 {
		a.sendEvent(SteerConsumedEvent{})
	}

	go a.drainQueue(first)
}

// --------------------------------------------------------------------------
// Non-interactive execution
// --------------------------------------------------------------------------

// RunOnce executes a single agent step synchronously and prints the final
// response text to stdout. No intermediate events are emitted. Blocks until
// the step completes or ctx is cancelled.
func (a *App) RunOnce(ctx context.Context, prompt string) error {
	return a.RunOnceWithFiles(ctx, prompt, nil)
}

// RunOnceWithFiles executes a single agent step synchronously with optional
// multimodal file attachments. Prints the response to stdout and returns.
func (a *App) RunOnceWithFiles(ctx context.Context, prompt string, files []kit.LLMFilePart) error {
	stepCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	a.mu.Lock()
	a.cancelStep = cancel
	a.mu.Unlock()

	result, err := a.executeStep(stepCtx, prompt, nil, files)
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
	return a.RunOnceResultWithFiles(ctx, prompt, nil)
}

// RunOnceResultWithFiles executes a single agent step synchronously with
// optional multimodal file attachments and returns the full TurnResult.
func (a *App) RunOnceResultWithFiles(ctx context.Context, prompt string, files []kit.LLMFilePart) (*kit.TurnResult, error) {
	stepCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	a.mu.Lock()
	a.cancelStep = cancel
	a.mu.Unlock()

	return a.executeStep(stepCtx, prompt, nil, files)
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
	return a.RunOnceWithDisplayAndFiles(ctx, prompt, eventFn, nil)
}

// RunOnceWithDisplayAndFiles executes a single agent step synchronously with
// optional multimodal file attachments, sending intermediate display events.
func (a *App) RunOnceWithDisplayAndFiles(ctx context.Context, prompt string, eventFn func(tea.Msg), files []kit.LLMFilePart) error {
	stepCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	a.mu.Lock()
	a.cancelStep = cancel
	a.mu.Unlock()

	result, err := a.executeStep(stepCtx, prompt, eventFn, files)
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

	// Clean up empty session file on shutdown.
	if ts := a.opts.TreeSession; ts != nil && ts.IsEmpty() {
		if path := ts.GetFilePath(); path != "" {
			_ = os.Remove(path)
		}
	}
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
		a.mu.Unlock()

		// Notify UI: all queued messages have been consumed into this batch.
		a.sendEvent(QueueUpdatedEvent{Length: 0})

		// Process all collected items as a single batch
		a.runQueueBatch(items)

		// Drain any unconsumed steer messages from the SDK channel.
		// These arrive when the user steered during a text-only response
		// (no tool calls, so PrepareStep didn't fire for a second step).
		// They go to the front of the queue so they run next.
		if a.opts.Kit != nil {
			if leftover := a.opts.Kit.DrainSteer(); len(leftover) > 0 {
				a.mu.Lock()
				steerItems := make([]queueItem, len(leftover))
				for i, sm := range leftover {
					steerItems[i] = queueItem{Prompt: sm.Text, Files: sm.Files}
				}
				a.queue = append(steerItems, a.queue...)
				a.mu.Unlock()
				// Notify UI about the consumed steer messages.
				a.sendEvent(SteerConsumedEvent{})
			}
		}

		// Check if more items were queued while we were processing
		a.mu.Lock()
		hasMore := len(a.queue) > 0
		if hasMore {
			// Start a new batch with the newly queued items
			items = a.queue
			a.queue = a.queue[:0]
		}
		a.mu.Unlock()

		if hasMore {
			// Notify UI: these newly queued messages have been consumed into the next batch.
			a.sendEvent(QueueUpdatedEvent{Length: 0})
		}

		if !hasMore {
			// No more items, we're done
			break
		}
		// Process the new batch
	}

	// Mark as no longer busy
	a.mu.Lock()
	a.setBusyLocked(false)
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
			// Step was cancelled by the user (double-ESC). The SDK
			// preserves the user message and any completed tool
			// call/result pairs; only the in-progress message or tool
			// call is discarded. Sync the in-memory store to match.
			if ts := a.opts.TreeSession; ts != nil {
				a.store.Replace(ts.GetLLMMessages())
			}
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
func (a *App) executeStep(ctx context.Context, prompt string, eventFn func(tea.Msg), files []kit.LLMFilePart) (*kit.TurnResult, error) {
	// Test hook: bypass SDK entirely.
	if a.opts.PromptFunc != nil {
		return a.opts.PromptFunc(ctx, prompt)
	}

	sendFn := func(msg tea.Msg) {
		if eventFn != nil {
			eventFn(msg)
		}
	}

	// Subscribe to SDK events for TUI rendering and per-step usage updates.
	// The subscription is temporary — it lives only for the duration of this step.
	var sawStepUsage atomic.Bool
	unsub := a.subscribeSDKEvents(sendFn, &sawStepUsage)
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

	// Update usage tracker. If per-step usage was already recorded from
	// StepUsageEvent callbacks, avoid double-counting totals.
	a.updateUsageFromTurnResult(result, prompt, sawStepUsage.Load())

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

	// Subscribe to SDK events for TUI rendering and per-step usage updates.
	// The subscription is temporary — it lives only for the duration of this step.
	var sawStepUsage atomic.Bool
	unsub := a.subscribeSDKEvents(sendFn, &sawStepUsage)
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

		// File attachments are not supported in batch mode; fall back to
		// processing only the first item that carries files.
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

	// Update usage tracker (using last item's prompt for fallback estimation).
	// If per-step usage was already recorded from StepUsageEvent callbacks,
	// avoid double-counting totals.
	a.updateUsageFromTurnResult(result, items[len(items)-1].Prompt, sawStepUsage.Load())

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
// SDK events to tea.Msg events and dispatch them via sendFn. When stepUsageSeen
// is provided, it is set to true after any non-zero StepUsageEvent is observed.
// Returns an unsubscribe function that removes all listeners.
func (a *App) subscribeSDKEvents(sendFn func(tea.Msg), stepUsageSeen *atomic.Bool) func() {
	k := a.opts.Kit
	var unsubs []func()

	unsubs = append(unsubs, k.Subscribe(func(e kit.Event) {
		switch ev := e.(type) {
		case kit.ToolCallEvent:
			sendFn(ToolCallStartedEvent{ToolCallID: ev.ToolCallID, ToolName: ev.ToolName, ToolArgs: ev.ToolArgs})
		case kit.ToolCallStartEvent:
			sendFn(ToolCallInputStartEvent{ToolCallID: ev.ToolCallID, ToolName: ev.ToolName, ToolKind: ev.ToolKind})
		case kit.ToolCallDeltaEvent:
			sendFn(ToolCallInputDeltaEvent{ToolCallID: ev.ToolCallID, Delta: ev.Delta})
		case kit.ToolCallEndEvent:
			sendFn(ToolCallInputEndEvent{ToolCallID: ev.ToolCallID})
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
		case kit.ReasoningCompleteEvent:
			sendFn(ReasoningCompleteEvent{})
		case kit.ToolOutputEvent:
			sendFn(ToolOutputEvent{
				ToolCallID: ev.ToolCallID,
				ToolName:   ev.ToolName,
				Chunk:      ev.Chunk,
				IsStderr:   ev.IsStderr,
			})
		case kit.SteerConsumedEvent:
			sendFn(SteerConsumedEvent{})
		case kit.StepUsageEvent:
			a.recordStepUsage(ev, stepUsageSeen, sendFn)
		case kit.PasswordPromptEvent:
			// Convert SDK PasswordPromptEvent to app PasswordPromptEvent
			// The TUI will handle this and send the response back
			responseCh := make(chan PasswordPromptResponse, 1)
			sendFn(PasswordPromptEvent{
				Prompt:     ev.Prompt,
				ResponseCh: responseCh,
			})
			// Wait for TUI response and forward to SDK
			resp := <-responseCh
			ev.ResponseCh <- kit.PasswordPromptResponse{
				Password:  resp.Password,
				Cancelled: resp.Cancelled,
			}
		case kit.TurnEndEvent:
			a.handleTurnEnd(ev, sendFn)
		}
	}))

	return func() {
		for _, unsub := range unsubs {
			unsub()
		}
	}
}

// handleTurnEnd inspects a turn's final StopReason and surfaces actionable
// feedback to the user when the turn ended in a state they can act on.
//
// Today the only surfaced case is FinishReasonLength — the model hit its
// configured max_output_tokens budget and the reply was truncated. Without
// this banner the TUI used to swallow the truncation silently, leading to
// "ghost" cut-offs with no indication of why.
//
// Separated from subscribeSDKEvents so tests can exercise it directly via a
// stubbed sendFn without standing up a full Kit.
func (a *App) handleTurnEnd(ev kit.TurnEndEvent, sendFn func(tea.Msg)) {
	if sendFn == nil {
		return
	}
	if ev.StopReason != kit.FinishReasonLength {
		return
	}
	sendFn(ExtensionPrintEvent{
		Level: "info",
		Text:  a.formatMaxTokensTruncatedMessage(),
	})
}

// formatMaxTokensTruncatedMessage builds the user-facing explanation for a
// truncated turn. It reports the active max_output_tokens budget and, when
// known, the model's catalog output ceiling so the user can judge how much
// headroom is available.
func (a *App) formatMaxTokensTruncatedMessage() string {
	k := a.opts.Kit
	if k == nil {
		// Extremely early / test-stub case: still emit a useful generic hint.
		return "⚠ Response truncated: the model hit the configured max_output_tokens limit. " +
			"Raise it with --max-tokens N, KIT_MAX_TOKENS=N, or per-model " +
			"modelSettings[provider/model].maxTokens in config."
	}
	current := k.MaxTokens()
	ceiling := k.MaxOutputLimit()
	model := k.GetModelString()

	msg := "⚠ Response truncated: "
	if model != "" {
		msg += fmt.Sprintf("%s hit the configured max_output_tokens limit", model)
	} else {
		msg += "the model hit the configured max_output_tokens limit"
	}
	if current > 0 {
		msg += fmt.Sprintf(" (%d)", current)
	}
	msg += "."
	if ceiling > 0 && current > 0 && ceiling > current {
		msg += fmt.Sprintf(" This model supports up to %d output tokens.", ceiling)
	}
	msg += "\n\nRaise it with --max-tokens N, KIT_MAX_TOKENS=N, " +
		"or per-model modelSettings[provider/model].maxTokens in your config. " +
		"Re-run the last prompt after raising it to get the full response."
	return msg
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
// appropriate renderer. In non-interactive mode it falls back to stderr with
// a level prefix so errors are distinguishable from plain output.
func (a *App) PrintFromExtension(level, text string) {
	a.mu.Lock()
	prog := a.program
	a.mu.Unlock()
	if prog != nil {
		prog.Send(ExtensionPrintEvent{Text: text, Level: level})
		return
	}
	// Non-interactive fallback: write to stderr with a level prefix so that
	// errors and info messages are distinguishable from plain output.
	switch level {
	case "error":
		fmt.Fprintf(os.Stderr, "[ERROR] %s\n", text)
	case "info":
		fmt.Fprintf(os.Stderr, "[INFO] %s\n", text)
	default:
		fmt.Println(text)
	}
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

// RequestNewSessionFromExtension sends a NewSessionRequestEvent to the TUI
// to end the current session and start a fresh one. If initialPrompt is
// non-empty it is submitted as the first user turn of the new session.
//
// If the agent is currently busy (e.g. the caller is an OnAgentEnd hook that
// fires before drainQueue clears the busy flag, or there are queued prompts
// still being processed) the call blocks until the agent becomes idle, up to
// DefaultNewSessionIdleWait. If that deadline elapses, ErrAgentBusy is
// returned and callers can detect it with errors.Is. This wait-then-send
// behavior fixes the v0.79.0 phase-handoff race documented in issue #63.
//
// Returns an error when running headless (no TUI attached), when the wait
// for idle times out (ErrAgentBusy), when the app is shutting down, or when
// a BeforeSessionSwitch extension hook cancels the switch.
//
// This is the implementation behind ctx.NewSession(prompt) for the
// interactive TUI. It blocks the caller until the TUI processes the
// switch, so it must be invoked from a goroutine outside Update().
func (a *App) RequestNewSessionFromExtension(initialPrompt string) error {
	a.mu.Lock()
	prog := a.program
	a.mu.Unlock()
	if prog == nil {
		return fmt.Errorf("new session unavailable: no interactive TUI attached")
	}
	if err := a.WaitForIdle(DefaultNewSessionIdleWait); err != nil {
		if errors.Is(err, ErrAgentBusy) {
			return fmt.Errorf("cannot start new session: %w", err)
		}
		return err
	}
	ch := make(chan error, 1)
	prog.Send(NewSessionRequestEvent{InitialPrompt: initialPrompt, ResponseCh: ch})
	return <-ch
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
//
// Coalescing (leading + trailing edge): the first call in an idle period
// fires immediately for responsiveness. Subsequent calls within a ~16 ms
// debounce window are batched into a single trailing event delivered at
// the end of the window. The trailing send is essential for correctness:
// extensions routinely make tight SetWidget→RemoveWidget pairs (e.g. on
// SubagentEnd) and silently dropping the second call would leave the TUI's
// layout stuck on stale widget dimensions until some other event happens
// to trigger a re-render.
func (a *App) NotifyWidgetUpdate() {
	if !a.widgetUpdatePending.CompareAndSwap(false, true) {
		// A leading-edge event is already in flight — mark that the runner
		// state has changed again so the trailing send below picks it up.
		a.widgetUpdateTrailing.Store(true)
		return
	}
	a.mu.Lock()
	prog := a.program
	a.mu.Unlock()
	if prog == nil {
		// No program registered (non-interactive mode); clear the flag so
		// future calls are never permanently blocked.
		a.widgetUpdatePending.Store(false)
		return
	}
	prog.Send(WidgetUpdateEvent{})
	go func() {
		time.Sleep(16 * time.Millisecond) // ~1 frame at 60 fps
		// If any extra calls came in during the debounce window, deliver
		// one trailing event so the TUI sees the latest widget state. We
		// swap-and-test instead of plain-load so concurrent calls after
		// the trailing send still race correctly with the pending reset.
		if a.widgetUpdateTrailing.Swap(false) {
			a.mu.Lock()
			p := a.program
			a.mu.Unlock()
			if p != nil {
				p.Send(WidgetUpdateEvent{})
			}
		}
		a.widgetUpdatePending.Store(false)
	}()
}

// NotifyContentReload sends a ContentReloadEvent to the TUI so it refreshes
// prompt templates and skills from their provider callbacks. Called by file
// watchers when .md/.txt files change in prompt or skill directories.
// In non-interactive mode this is a no-op.
func (a *App) NotifyContentReload() {
	a.mu.Lock()
	prog := a.program
	a.mu.Unlock()
	if prog != nil {
		prog.Send(ContentReloadEvent{})
	}
}

// NotifyMCPToolsReady sends an MCPToolsReadyEvent to the TUI so it refreshes
// tool names and MCP tool count from provider callbacks. Called when background
// MCP tool loading completes. In non-interactive mode this is a no-op.
func (a *App) NotifyMCPToolsReady() {
	a.mu.Lock()
	prog := a.program
	a.mu.Unlock()
	if prog != nil {
		prog.Send(MCPToolsReadyEvent{})
	}
}

// NotifyMCPServerLoaded sends an MCPServerLoadedEvent to the TUI so it can
// display a system message when a single MCP server finishes loading. Called
// per server as background MCP tool loading progresses.
func (a *App) NotifyMCPServerLoaded(serverName string, toolCount int, err error) {
	a.mu.Lock()
	prog := a.program
	a.mu.Unlock()
	if prog != nil {
		prog.Send(MCPServerLoadedEvent{
			ServerName: serverName,
			ToolCount:  toolCount,
			Error:      err,
		})
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
	// Non-interactive fallback: render a simple framed block to stderr so
	// it is visually distinct from plain stdout output.
	if opts.Subtitle != "" {
		fmt.Fprintf(os.Stderr, "--- %s ---\n%s\n", opts.Subtitle, opts.Text)
	} else {
		fmt.Fprintf(os.Stderr, "---\n%s\n---\n", opts.Text)
	}
}

// recordStepUsage applies token/cost usage reported for a completed step.
// Step usage events arrive even when a turn is later cancelled, so this keeps
// the usage widget accurate on all stop paths.
//
// Both session totals (cost, token counts) and the context window fill level
// are updated here so the status bar reflects progress after every LLM call,
// not just at the end of the full turn. Context fill monotonically increases
// across steps because each step re-sends the entire conversation plus any
// new tool results, so the numbers only go up.
//
// sendFn is called with a UsageUpdatedEvent to trigger a TUI re-render so
// the updated values are visible immediately.
func (a *App) recordStepUsage(ev kit.StepUsageEvent, stepUsageSeen *atomic.Bool, sendFn func(tea.Msg)) {
	hasUsage := ev.InputTokens > 0 || ev.OutputTokens > 0 || ev.CacheReadTokens > 0 || ev.CacheWriteTokens > 0
	if a.opts.Debug {
		log.Printf("[DEBUG] recordStepUsage: hasUsage=%v input=%d output=%d cacheRead=%d cacheWrite=%d",
			hasUsage, ev.InputTokens, ev.OutputTokens, ev.CacheReadTokens, ev.CacheWriteTokens)
	}
	if !hasUsage {
		return
	}
	if stepUsageSeen != nil {
		stepUsageSeen.Store(true)
	}
	if a.opts.UsageTracker == nil {
		return
	}
	a.opts.UsageTracker.UpdateUsage(
		int(ev.InputTokens),
		int(ev.OutputTokens),
		int(ev.CacheReadTokens),
		int(ev.CacheWriteTokens),
	)
	// Update context window fill from this step's usage. Each step sends
	// the full conversation to the LLM, so the reported token counts
	// represent the actual context utilization at that point.
	contextFill := int(ev.InputTokens) + int(ev.CacheReadTokens) + int(ev.CacheWriteTokens) + int(ev.OutputTokens)
	if contextFill > 0 {
		if a.opts.Debug {
			log.Printf("[DEBUG] recordStepUsage: SetContextTokens=%d (Input=%d + CacheRead=%d + CacheWrite=%d + Output=%d)",
				contextFill, ev.InputTokens, ev.CacheReadTokens, ev.CacheWriteTokens, ev.OutputTokens)
		}
		a.opts.UsageTracker.SetContextTokens(contextFill)
	}
	// Notify the TUI so it re-renders the status bar with updated values.
	if sendFn != nil {
		sendFn(UsageUpdatedEvent{})
	}
}

// updateUsageFromTurnResult records token usage from an SDK TurnResult into the
// configured UsageTracker. Called once per turn after the turn completes.
//
// When sawStepUsage is true, totals were already accumulated incrementally via
// StepUsageEvent callbacks; in that case this method only updates context fill.
// Otherwise it falls back to TotalUsage from the API response.
//
// NOTE: We only use ACTUAL token counts from API responses for cost tracking.
// Estimation is never used for costs - only API-reported tokens are accurate.
func (a *App) updateUsageFromTurnResult(result *kit.TurnResult, userPrompt string, sawStepUsage bool) {
	if a.opts.UsageTracker == nil || result == nil {
		return
	}

	// Debug logging for token tracking
	if a.opts.Debug {
		if result.TotalUsage != nil {
			log.Printf("[DEBUG] updateUsageFromTurnResult TotalUsage: input=%d output=%d cacheRead=%d cacheCreate=%d",
				result.TotalUsage.InputTokens, result.TotalUsage.OutputTokens,
				result.TotalUsage.CacheReadTokens, result.TotalUsage.CacheCreationTokens)
		} else {
			log.Printf("[DEBUG] updateUsageFromTurnResult: TotalUsage=nil")
		}
		if result.FinalUsage != nil {
			log.Printf("[DEBUG] updateUsageFromTurnResult FinalUsage: input=%d output=%d cacheRead=%d cacheCreate=%d",
				result.FinalUsage.InputTokens, result.FinalUsage.OutputTokens,
				result.FinalUsage.CacheReadTokens, result.FinalUsage.CacheCreationTokens)
		} else {
			log.Printf("[DEBUG] updateUsageFromTurnResult: FinalUsage=nil")
		}
		log.Printf("[DEBUG] updateUsageFromTurnResult: sawStepUsage=%v", sawStepUsage)
	}

	// --- Accumulate cost/token totals for the session ---
	// Only use actual API-reported tokens for cost tracking.
	// If sawStepUsage is true, totals were already updated via StepUsageEvent.
	// Check any token field > 0 (not just InputTokens) because cached prompts
	// can result in InputTokens=0 while OutputTokens>0 (OpenAI-compatible behavior).
	hasTotalUsage := result.TotalUsage != nil &&
		(result.TotalUsage.InputTokens > 0 ||
			result.TotalUsage.OutputTokens > 0 ||
			result.TotalUsage.CacheReadTokens > 0 ||
			result.TotalUsage.CacheCreationTokens > 0)
	if a.opts.Debug {
		log.Printf("[DEBUG] updateUsageFromTurnResult: hasTotalUsage=%v", hasTotalUsage)
	}

	// Surface whether the provider reported ANY usage this turn. When neither
	// incremental StepUsageEvents nor TotalUsage carried real token counts, the
	// provider (often an OpenAI-compatible proxy that omits `usage` from the
	// final streaming chunk) gave us nothing to bill or meter. Flag this so
	// the status bar shows an honest "⚠ usage not reported by provider"
	// notice instead of a misleading bare zero.
	providerReportedUsage := sawStepUsage || hasTotalUsage
	a.opts.UsageTracker.SetUsageUnreported(!providerReportedUsage)
	if a.opts.Debug {
		log.Printf("[DEBUG] updateUsageFromTurnResult: providerReportedUsage=%v -> usageUnreported=%v",
			providerReportedUsage, !providerReportedUsage)
	}

	if !sawStepUsage && hasTotalUsage {
		if a.opts.Debug {
			log.Printf("[DEBUG] updateUsageFromTurnResult: calling UpdateUsage input=%d output=%d cacheRead=%d cacheCreate=%d",
				result.TotalUsage.InputTokens, result.TotalUsage.OutputTokens,
				result.TotalUsage.CacheReadTokens, result.TotalUsage.CacheCreationTokens)
		}
		a.opts.UsageTracker.UpdateUsage(
			int(result.TotalUsage.InputTokens),
			int(result.TotalUsage.OutputTokens),
			int(result.TotalUsage.CacheReadTokens),
			int(result.TotalUsage.CacheCreationTokens),
		)
	}

	// --- Context window fill (drives the % bar) ---
	// Calculate context fill from the LAST API call's usage. The context
	// window is filled by everything sent to and received from the model:
	//
	//   InputTokens       — non-cached input (may be small with prompt caching)
	//   CacheReadTokens   — input tokens served from cache
	//   CacheCreationTokens — input tokens written to cache this call
	//   OutputTokens      — assistant output (becomes input next turn)
	//
	// With Anthropic prompt caching, InputTokens can drop to near-zero while
	// CacheReadTokens holds the bulk of the context. We must sum all four to
	// get the true context window utilization.
	//
	// We use FinalUsage (last step only), NOT TotalUsage, because TotalUsage
	// sums across all tool-calling steps — and each step re-sends the full
	// conversation, so TotalUsage massively overstates the actual window fill.
	if result.FinalUsage != nil {
		u := result.FinalUsage
		contextFill := int(u.InputTokens) + int(u.CacheReadTokens) + int(u.CacheCreationTokens) + int(u.OutputTokens)
		if contextFill > 0 {
			if a.opts.Debug {
				log.Printf("[DEBUG] updateUsageFromTurnResult: SetContextTokens=%d (Input=%d + CacheRead=%d + CacheCreate=%d + Output=%d)",
					contextFill, u.InputTokens, u.CacheReadTokens, u.CacheCreationTokens, u.OutputTokens)
			}
			a.opts.UsageTracker.SetContextTokens(contextFill)
		}
	}
}
