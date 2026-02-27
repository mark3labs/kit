package kit

import "sync"

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

// MessageEndEvent fires when the assistant message is complete.
type MessageEndEvent struct {
	Content string
}

// EventType implements Event.
func (e MessageEndEvent) EventType() EventType { return EventMessageEnd }

// ToolCallEvent fires when a tool call has been parsed.
type ToolCallEvent struct {
	ToolName string
	ToolArgs string
}

// EventType implements Event.
func (e ToolCallEvent) EventType() EventType { return EventToolCall }

// ToolExecutionStartEvent fires when a tool begins executing.
type ToolExecutionStartEvent struct {
	ToolName string
}

// EventType implements Event.
func (e ToolExecutionStartEvent) EventType() EventType { return EventToolExecutionStart }

// ToolExecutionEndEvent fires when a tool finishes executing.
type ToolExecutionEndEvent struct {
	ToolName string
}

// EventType implements Event.
func (e ToolExecutionEndEvent) EventType() EventType { return EventToolExecutionEnd }

// ToolResultEvent fires after a tool execution completes with its result.
type ToolResultEvent struct {
	ToolName string
	ToolArgs string
	Result   string
	IsError  bool
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

// CompactionEvent fires after a successful compaction.
type CompactionEvent struct {
	Summary         string
	OriginalTokens  int
	CompactedTokens int
	MessagesRemoved int
}

// EventType implements Event.
func (e CompactionEvent) EventType() EventType { return EventCompaction }

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
