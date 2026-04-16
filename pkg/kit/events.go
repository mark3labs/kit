package kit

import (
	"encoding/json"
	"sync"
)

// ---------------------------------------------------------------------------
// Event types
// ---------------------------------------------------------------------------

// EventType identifies the kind of lifecycle event.
type EventType string

const (
	// EventTurnStart fires before the agent begins processing a prompt.
	EventTurnStart EventType = "turn_start"
	// EventTurnEnd fires after the agent finishes processing (success or error).
	EventTurnEnd EventType = "turn_end"
	// EventMessageStart fires when a new assistant message begins.
	EventMessageStart EventType = "message_start"
	// EventMessageUpdate fires for each streaming text chunk.
	EventMessageUpdate EventType = "message_update"
	// EventMessageEnd fires when the assistant message is complete.
	EventMessageEnd EventType = "message_end"
	// EventToolCall fires when a tool call has been parsed and is about to execute.
	EventToolCall EventType = "tool_call"
	// EventToolExecutionStart fires when a tool begins executing.
	EventToolExecutionStart EventType = "tool_execution_start"
	// EventToolExecutionEnd fires when a tool finishes executing.
	EventToolExecutionEnd EventType = "tool_execution_end"
	// EventToolResult fires after a tool execution completes with its result.
	EventToolResult EventType = "tool_result"
	// EventToolCallContent fires when a step includes text alongside tool calls.
	EventToolCallContent EventType = "tool_call_content"
	// EventResponse fires when the LLM produces a final response.
	EventResponse EventType = "response"
	// EventCompaction fires after a successful compaction.
	EventCompaction EventType = "compaction"
	// EventReasoningDelta fires for each streaming reasoning/thinking chunk.
	EventReasoningDelta EventType = "reasoning_delta"
	// EventReasoningComplete fires when reasoning/thinking is finished,
	// after the last reasoning token has been processed.
	EventReasoningComplete EventType = "reasoning_complete"
	// EventToolOutput fires when a tool produces streaming output chunks.
	EventToolOutput EventType = "tool_output"
	EventStepUsage  EventType = "step_usage"
	// EventPasswordPrompt fires when a sudo command needs a password.
	EventPasswordPrompt EventType = "password_prompt"
	// EventSteerConsumed fires when one or more steering messages have been
	// injected into the agent turn via PrepareStep.
	EventSteerConsumed EventType = "steer_consumed"
)

// ---------------------------------------------------------------------------
// Event interface
// ---------------------------------------------------------------------------

// Event is the interface implemented by all lifecycle events. Each concrete
// event type returns its EventType via this method.
type Event interface {
	EventType() EventType
}

// ---------------------------------------------------------------------------
// Tool kind constants
// ---------------------------------------------------------------------------

// ToolKind constants classify what a tool does, enabling UIs to render
// appropriate visualizations (e.g. diff view for edit tools, command+output
// for execute tools) and file trackers to identify which results contain
// modifications.
const (
	ToolKindExecute  = "execute" // Shell execution (bash)
	ToolKindEdit     = "edit"    // File modification (edit, write)
	ToolKindRead     = "read"    // File reading (read, ls)
	ToolKindSearch   = "search"  // Content/file search (grep, find)
	ToolKindSubagent = "agent"   // Subagent spawning (subagent)
)

// coreToolKinds maps built-in tool names to their kind. MCP and extension
// tools without an entry default to ToolKindExecute.
var coreToolKinds = map[string]string{
	"bash":     ToolKindExecute,
	"edit":     ToolKindEdit,
	"write":    ToolKindEdit,
	"read":     ToolKindRead,
	"ls":       ToolKindRead,
	"grep":     ToolKindSearch,
	"find":     ToolKindSearch,
	"subagent": ToolKindSubagent,
}

// toolKindFor returns the ToolKind for a given tool name, defaulting to
// ToolKindExecute for unknown tools.
func toolKindFor(toolName string) string {
	if kind, ok := coreToolKinds[toolName]; ok {
		return kind
	}
	return ToolKindExecute
}

// parseToolArgs attempts to parse a JSON-encoded tool args string into a map.
// Returns nil on failure (non-fatal convenience parsing).
func parseToolArgs(toolArgs string) map[string]any {
	var parsed map[string]any
	if json.Unmarshal([]byte(toolArgs), &parsed) == nil {
		return parsed
	}
	return nil
}

// ---------------------------------------------------------------------------
// Finish reason constants
// ---------------------------------------------------------------------------

// Finish reasons reported by the LLM provider on a completed turn. These
// mirror fantasy.FinishReason string values so comparisons against
// TurnEndEvent.StopReason / TurnResult.StopReason are stable across
// providers.
const (
	// FinishReasonStop: the model produced a natural stop (e.g. stop sequence
	// or end-of-turn signal).
	FinishReasonStop = "stop"
	// FinishReasonLength: the model hit the configured max_output_tokens
	// budget. The response is truncated. Surface this to the user and
	// consider raising --max-tokens / KIT_MAX_TOKENS / modelSettings[...]
	// .maxTokens.
	FinishReasonLength = "length"
	// FinishReasonToolCalls: the model stopped to emit tool calls (normal
	// mid-turn state during agentic loops).
	FinishReasonToolCalls = "tool-calls"
	// FinishReasonContentFilter: the provider's safety filter stopped
	// generation.
	FinishReasonContentFilter = "content-filter"
	// FinishReasonError: the model stopped because of an error.
	FinishReasonError = "error"
	// FinishReasonOther: provider-specific reason that doesn't map to any of
	// the above.
	FinishReasonOther = "other"
	// FinishReasonUnknown: the provider didn't report a finish reason.
	FinishReasonUnknown = "unknown"
)

// ---------------------------------------------------------------------------
// Concrete event structs
// ---------------------------------------------------------------------------

// TurnStartEvent fires before the agent begins processing a prompt.
type TurnStartEvent struct {
	Prompt string
}

// EventType implements Event.
func (e TurnStartEvent) EventType() EventType { return EventTurnStart }

// TurnEndEvent fires after the agent finishes processing.
type TurnEndEvent struct {
	Response string
	Error    error
	// StopReason is the LLM provider's finish reason for the final step of
	// the turn. Compare against the FinishReason* constants — in particular,
	// FinishReasonLength indicates the response was truncated because the
	// agent hit its max_output_tokens budget.
	StopReason string
}

// EventType implements Event.
func (e TurnEndEvent) EventType() EventType { return EventTurnEnd }

// MessageStartEvent fires when a new assistant message begins.
type MessageStartEvent struct{}

// EventType implements Event.
func (e MessageStartEvent) EventType() EventType { return EventMessageStart }

// MessageUpdateEvent fires for each streaming text chunk.
type MessageUpdateEvent struct {
	Chunk string
}

// EventType implements Event.
func (e MessageUpdateEvent) EventType() EventType { return EventMessageUpdate }

// ReasoningDeltaEvent fires for each streaming reasoning/thinking chunk.
type ReasoningDeltaEvent struct {
	Delta string
}

// EventType implements Event.
func (e ReasoningDeltaEvent) EventType() EventType { return EventReasoningDelta }

// ReasoningCompleteEvent fires when reasoning/thinking is finished, after the
// last reasoning token has been processed.
type ReasoningCompleteEvent struct{}

// EventType implements Event.
func (e ReasoningCompleteEvent) EventType() EventType { return EventReasoningComplete }

// ToolOutputEvent fires when a tool produces streaming output chunks (e.g., bash output).
type ToolOutputEvent struct {
	ToolCallID string
	ToolName   string
	Chunk      string
	IsStderr   bool
}

// EventType implements Event.
func (e ToolOutputEvent) EventType() EventType { return EventToolOutput }

// MessageEndEvent fires when the assistant message is complete.
type MessageEndEvent struct {
	Content string
}

// EventType implements Event.
func (e MessageEndEvent) EventType() EventType { return EventMessageEnd }

// ToolCallEvent fires when a tool call has been parsed.
type ToolCallEvent struct {
	ToolCallID string // Stable ID for correlating tool lifecycle events
	ToolName   string
	ToolKind   string         // Tool classification: "execute", "edit", "read", "search", "agent"
	ToolArgs   string         // JSON-encoded arguments
	ParsedArgs map[string]any // Pre-parsed arguments for convenience (nil on parse failure)
}

// EventType implements Event.
func (e ToolCallEvent) EventType() EventType { return EventToolCall }

// ToolExecutionStartEvent fires when a tool begins executing.
type ToolExecutionStartEvent struct {
	ToolCallID string
	ToolName   string
	ToolKind   string
	ToolArgs   string
}

// EventType implements Event.
func (e ToolExecutionStartEvent) EventType() EventType { return EventToolExecutionStart }

// ToolExecutionEndEvent fires when a tool finishes executing.
type ToolExecutionEndEvent struct {
	ToolCallID string
	ToolName   string
	ToolKind   string
}

// EventType implements Event.
func (e ToolExecutionEndEvent) EventType() EventType { return EventToolExecutionEnd }

// ToolResultEvent fires after a tool execution completes with its result.
type ToolResultEvent struct {
	ToolCallID string
	ToolName   string
	ToolKind   string
	ToolArgs   string
	ParsedArgs map[string]any // Pre-parsed arguments for convenience
	Result     string
	IsError    bool
	Metadata   *ToolResultMetadata // Optional structured metadata from tool execution
}

// ToolResultMetadata carries structured data from tool executions.
type ToolResultMetadata struct {
	FileDiffs         []FileDiffInfo `json:"file_diffs,omitempty"`          // Present for edit/write tools
	SubagentSessionID string         `json:"subagent_session_id,omitempty"` // Present for subagent tool
}

// FileDiffInfo describes a file modification from an edit or write tool.
type FileDiffInfo struct {
	Path       string      `json:"path"`             // Absolute file path
	Additions  int         `json:"additions"`        // Lines added
	Deletions  int         `json:"deletions"`        // Lines removed
	IsNew      bool        `json:"is_new,omitempty"` // True if file was created (write only)
	DiffBlocks []DiffBlock `json:"diff_blocks,omitempty"`
}

// DiffBlock represents a single old→new text replacement within a file.
type DiffBlock struct {
	OldText string `json:"old_text"`
	NewText string `json:"new_text"`
}

// EventType implements Event.
func (e ToolResultEvent) EventType() EventType { return EventToolResult }

// ToolCallContentEvent fires when a step includes text alongside tool calls.
type ToolCallContentEvent struct {
	Content string
}

// EventType implements Event.
func (e ToolCallContentEvent) EventType() EventType { return EventToolCallContent }

// ResponseEvent fires when the LLM produces a final response.
type ResponseEvent struct {
	Content string
}

// EventType implements Event.
func (e ResponseEvent) EventType() EventType { return EventResponse }

// StepUsageEvent fires after each complete step in a multi-step agent turn,
// carrying the token usage for that specific step. This enables real-time
// cost tracking during long-running tool-calling conversations.
type StepUsageEvent struct {
	InputTokens      uint64
	OutputTokens     uint64
	CacheReadTokens  uint64
	CacheWriteTokens uint64
}

// EventType implements Event.
func (e StepUsageEvent) EventType() EventType { return EventStepUsage }

// CompactionEvent fires after a successful compaction.
type CompactionEvent struct {
	Summary         string
	OriginalTokens  int
	CompactedTokens int
	MessagesRemoved int
	ReadFiles       []string
	ModifiedFiles   []string
}

// EventType implements Event.
func (e CompactionEvent) EventType() EventType { return EventCompaction }

// SteerConsumedEvent fires when one or more steering messages have been
// injected into the agent turn via PrepareStep. The Count indicates how
// many messages were consumed in this batch.
type SteerConsumedEvent struct {
	Count int
}

// EventType implements Event.
func (e SteerConsumedEvent) EventType() EventType { return EventSteerConsumed }

// PasswordPromptEvent fires when a sudo command needs a password.
// The TUI should display a password prompt and send the result back via ResponseCh.
type PasswordPromptEvent struct {
	// Prompt is the message to display to the user.
	Prompt string
	// ResponseCh receives the password from the TUI.
	// The TUI must send exactly one value: (password, false) for submit
	// or ("", true) for cancel.
	ResponseCh chan<- PasswordPromptResponse
}

// PasswordPromptResponse carries the password prompt result.
type PasswordPromptResponse struct {
	Password  string
	Cancelled bool
}

// EventType implements Event.
func (e PasswordPromptEvent) EventType() EventType { return EventPasswordPrompt }

// ---------------------------------------------------------------------------
// EventBus
// ---------------------------------------------------------------------------

// EventListener is a callback that receives lifecycle events.
type EventListener func(event Event)

// eventBus is a thread-safe event dispatcher that supports multiple
// subscribers with unsubscribe capability.
type eventBus struct {
	mu        sync.RWMutex
	listeners map[int]EventListener
	nextID    int
}

// newEventBus creates a new event bus.
func newEventBus() *eventBus {
	return &eventBus{listeners: make(map[int]EventListener)}
}

// subscribe registers a listener and returns an unsubscribe function.
func (eb *eventBus) subscribe(listener EventListener) func() {
	eb.mu.Lock()
	id := eb.nextID
	eb.nextID++
	eb.listeners[id] = listener
	eb.mu.Unlock()
	return func() {
		eb.mu.Lock()
		delete(eb.listeners, id)
		eb.mu.Unlock()
	}
}

// emit dispatches an event to all registered listeners. Listeners are
// snapshotted under the read lock and called outside of it, so listeners
// may safely call subscribe/unsubscribe without deadlocking.
func (eb *eventBus) emit(event Event) {
	eb.mu.RLock()
	snapshot := make([]EventListener, 0, len(eb.listeners))
	for _, l := range eb.listeners {
		snapshot = append(snapshot, l)
	}
	eb.mu.RUnlock()
	for _, l := range snapshot {
		l(event)
	}
}

// ---------------------------------------------------------------------------
// Typed convenience subscribers
// ---------------------------------------------------------------------------

// OnToolCall registers a handler that fires only for ToolCallEvent.
// Returns an unsubscribe function.
func (m *Kit) OnToolCall(handler func(ToolCallEvent)) func() {
	return m.Subscribe(func(e Event) {
		if tc, ok := e.(ToolCallEvent); ok {
			handler(tc)
		}
	})
}

// OnToolResult registers a handler that fires only for ToolResultEvent.
// Returns an unsubscribe function.
func (m *Kit) OnToolResult(handler func(ToolResultEvent)) func() {
	return m.Subscribe(func(e Event) {
		if tr, ok := e.(ToolResultEvent); ok {
			handler(tr)
		}
	})
}

// OnToolOutput registers a handler that fires only for ToolOutputEvent
// (streaming tool output chunks, e.g., from bash). Returns an unsubscribe function.
func (m *Kit) OnToolOutput(handler func(ToolOutputEvent)) func() {
	return m.Subscribe(func(e Event) {
		if to, ok := e.(ToolOutputEvent); ok {
			handler(to)
		}
	})
}

// OnStreaming registers a handler that fires only for MessageUpdateEvent
// (streaming text chunks). Returns an unsubscribe function.
func (m *Kit) OnStreaming(handler func(MessageUpdateEvent)) func() {
	return m.Subscribe(func(e Event) {
		if mu, ok := e.(MessageUpdateEvent); ok {
			handler(mu)
		}
	})
}

// OnResponse registers a handler that fires only for ResponseEvent.
// Returns an unsubscribe function.
func (m *Kit) OnResponse(handler func(ResponseEvent)) func() {
	return m.Subscribe(func(e Event) {
		if r, ok := e.(ResponseEvent); ok {
			handler(r)
		}
	})
}

// OnTurnStart registers a handler that fires only for TurnStartEvent.
// Returns an unsubscribe function.
func (m *Kit) OnTurnStart(handler func(TurnStartEvent)) func() {
	return m.Subscribe(func(e Event) {
		if ts, ok := e.(TurnStartEvent); ok {
			handler(ts)
		}
	})
}

// OnTurnEnd registers a handler that fires only for TurnEndEvent.
// Returns an unsubscribe function.
func (m *Kit) OnTurnEnd(handler func(TurnEndEvent)) func() {
	return m.Subscribe(func(e Event) {
		if te, ok := e.(TurnEndEvent); ok {
			handler(te)
		}
	})
}

// ---------------------------------------------------------------------------
// Subagent event subscriptions
// ---------------------------------------------------------------------------

// SubscribeSubagent registers a listener for real-time events from a subagent
// identified by its tool call ID. Returns an unsubscribe function.
//
// The listener receives the same event types as Subscribe() (ToolCallEvent,
// MessageUpdateEvent, etc.) but scoped to the child agent's activity. If the
// tool call ID doesn't correspond to an active or future subagent call,
// the listener simply never fires.
//
// Typical usage — register inside an OnToolCall handler:
//
//	kit.OnToolCall(func(e kit.ToolCallEvent) {
//	    if e.ToolName == "subagent" {
//	        kit.SubscribeSubagent(e.ToolCallID, func(child kit.Event) {
//	            // real-time subagent events
//	        })
//	    }
//	})
func (m *Kit) SubscribeSubagent(toolCallID string, listener EventListener) func() {
	actual, _ := m.subagentListeners.LoadOrStore(toolCallID, newEventBus())
	return actual.(*eventBus).subscribe(listener)
}

// getSubagentListenerSet returns the listener set for a tool call, or nil.
func (m *Kit) getSubagentListenerSet(toolCallID string) *eventBus {
	if v, ok := m.subagentListeners.Load(toolCallID); ok {
		return v.(*eventBus)
	}
	return nil
}

// cleanupSubagentListeners removes the listener set for a completed tool call.
func (m *Kit) cleanupSubagentListeners(toolCallID string) {
	m.subagentListeners.Delete(toolCallID)
}
