package kit

import (
	"sync"
	"sync/atomic"
	"testing"
)

// TestEventBusSubscribeAndEmit verifies that subscribers receive emitted events.
func TestEventBusSubscribeAndEmit(t *testing.T) {
	bus := newEventBus()
	var received []Event

	bus.subscribe(func(e Event) {
		received = append(received, e)
	})

	bus.emit(TurnStartEvent{Prompt: "hello"})
	bus.emit(ToolCallEvent{ToolName: "bash", ToolArgs: `{"cmd":"ls"}`})
	bus.emit(TurnEndEvent{Response: "done"})

	if len(received) != 3 {
		t.Fatalf("expected 3 events, got %d", len(received))
	}

	if ts, ok := received[0].(TurnStartEvent); !ok || ts.Prompt != "hello" {
		t.Errorf("event 0: expected TurnStartEvent{Prompt:hello}, got %T %+v", received[0], received[0])
	}
	if tc, ok := received[1].(ToolCallEvent); !ok || tc.ToolName != "bash" {
		t.Errorf("event 1: expected ToolCallEvent{ToolName:bash}, got %T %+v", received[1], received[1])
	}
	if te, ok := received[2].(TurnEndEvent); !ok || te.Response != "done" {
		t.Errorf("event 2: expected TurnEndEvent{Response:done}, got %T %+v", received[2], received[2])
	}
}

// TestEventBusMultipleSubscribers verifies that all subscribers receive every event.
func TestEventBusMultipleSubscribers(t *testing.T) {
	bus := newEventBus()
	var count1, count2 int

	bus.subscribe(func(Event) { count1++ })
	bus.subscribe(func(Event) { count2++ })

	bus.emit(MessageStartEvent{})
	bus.emit(MessageEndEvent{Content: "test"})

	if count1 != 2 {
		t.Errorf("subscriber 1: expected 2 calls, got %d", count1)
	}
	if count2 != 2 {
		t.Errorf("subscriber 2: expected 2 calls, got %d", count2)
	}
}

// TestEventBusUnsubscribe verifies that unsubscribed listeners stop receiving events.
func TestEventBusUnsubscribe(t *testing.T) {
	bus := newEventBus()
	var count int

	unsub := bus.subscribe(func(Event) { count++ })

	bus.emit(TurnStartEvent{})
	if count != 1 {
		t.Fatalf("expected 1 call before unsub, got %d", count)
	}

	unsub()

	bus.emit(TurnEndEvent{})
	if count != 1 {
		t.Errorf("expected 1 call after unsub, got %d", count)
	}
}

// TestEventBusUnsubscribePartial verifies that only the unsubscribed listener
// is removed; others continue receiving.
func TestEventBusUnsubscribePartial(t *testing.T) {
	bus := newEventBus()
	var countA, countB int

	unsubA := bus.subscribe(func(Event) { countA++ })
	bus.subscribe(func(Event) { countB++ })

	bus.emit(MessageStartEvent{})
	unsubA()
	bus.emit(MessageEndEvent{Content: "test"})

	if countA != 1 {
		t.Errorf("listener A: expected 1 (unsubscribed after first), got %d", countA)
	}
	if countB != 2 {
		t.Errorf("listener B: expected 2, got %d", countB)
	}
}

// TestEventBusConcurrentEmit verifies thread safety of concurrent emit calls.
func TestEventBusConcurrentEmit(t *testing.T) {
	bus := newEventBus()
	var count atomic.Int64

	bus.subscribe(func(Event) {
		count.Add(1)
	})

	var wg sync.WaitGroup
	for range 100 {
		wg.Go(func() {
			bus.emit(MessageUpdateEvent{Chunk: "x"})
		})
	}
	wg.Wait()

	if got := count.Load(); got != 100 {
		t.Errorf("expected 100 events, got %d", got)
	}
}

// TestEventBusConcurrentSubscribeEmit verifies thread safety when subscribing
// and emitting concurrently.
func TestEventBusConcurrentSubscribeEmit(t *testing.T) {
	bus := newEventBus()
	var total atomic.Int64

	var wg sync.WaitGroup
	// Spawn subscribers concurrently.
	for range 10 {
		wg.Go(func() {
			bus.subscribe(func(Event) {
				total.Add(1)
			})
		})
	}
	// Spawn emitters concurrently.
	for range 50 {
		wg.Go(func() {
			bus.emit(TurnStartEvent{Prompt: "test"})
		})
	}
	wg.Wait()

	// We can't assert an exact count because subscribe/emit ordering is
	// non-deterministic, but it must not panic or deadlock.
	t.Logf("total events received across subscribers: %d", total.Load())
}

// TestEventBusEmitNoListeners verifies emit is a no-op with no subscribers.
func TestEventBusEmitNoListeners(t *testing.T) {
	bus := newEventBus()
	// Should not panic.
	bus.emit(TurnStartEvent{Prompt: "hello"})
	bus.emit(TurnEndEvent{})
}

// TestEventTypes verifies that each event struct returns the correct EventType.
func TestEventTypes(t *testing.T) {
	tests := []struct {
		event    Event
		expected EventType
	}{
		{TurnStartEvent{}, EventTurnStart},
		{TurnEndEvent{}, EventTurnEnd},
		{MessageStartEvent{}, EventMessageStart},
		{MessageUpdateEvent{}, EventMessageUpdate},
		{MessageEndEvent{}, EventMessageEnd},
		{ToolCallEvent{}, EventToolCall},
		{ToolExecutionStartEvent{}, EventToolExecutionStart},
		{ToolExecutionEndEvent{}, EventToolExecutionEnd},
		{ToolResultEvent{}, EventToolResult},
		{ToolCallContentEvent{}, EventToolCallContent},
		{ResponseEvent{}, EventResponse},
	}

	for _, tt := range tests {
		if got := tt.event.EventType(); got != tt.expected {
			t.Errorf("%T.EventType() = %q, want %q", tt.event, got, tt.expected)
		}
	}
}

// TestEventBusListenerCanUnsubscribeInCallback verifies that a listener can
// safely call its own unsubscribe function from within the callback.
func TestEventBusListenerCanUnsubscribeInCallback(t *testing.T) {
	bus := newEventBus()
	var count int

	var unsub func()
	unsub = bus.subscribe(func(Event) {
		count++
		unsub() // Unsubscribe from within the callback.
	})

	bus.emit(TurnStartEvent{})
	bus.emit(TurnStartEvent{})

	if count != 1 {
		t.Errorf("expected listener to fire once (unsubscribed itself), got %d", count)
	}
}

// TestEventOrdering verifies that events are received in emission order
// by a single subscriber.
func TestEventOrdering(t *testing.T) {
	bus := newEventBus()
	var types []EventType

	bus.subscribe(func(e Event) {
		types = append(types, e.EventType())
	})

	expected := []EventType{
		EventTurnStart,
		EventMessageStart,
		EventMessageUpdate,
		EventToolCall,
		EventToolExecutionStart,
		EventToolExecutionEnd,
		EventToolResult,
		EventToolCallContent,
		EventMessageEnd,
		EventResponse,
		EventTurnEnd,
	}

	bus.emit(TurnStartEvent{})
	bus.emit(MessageStartEvent{})
	bus.emit(MessageUpdateEvent{Chunk: "hello"})
	bus.emit(ToolCallEvent{ToolName: "bash"})
	bus.emit(ToolExecutionStartEvent{ToolName: "bash"})
	bus.emit(ToolExecutionEndEvent{ToolName: "bash"})
	bus.emit(ToolResultEvent{ToolName: "bash", Result: "ok"})
	bus.emit(ToolCallContentEvent{Content: "I'll run bash"})
	bus.emit(MessageEndEvent{Content: "done"})
	bus.emit(ResponseEvent{Content: "done"})
	bus.emit(TurnEndEvent{Response: "done"})

	if len(types) != len(expected) {
		t.Fatalf("expected %d events, got %d", len(expected), len(types))
	}
	for i, exp := range expected {
		if types[i] != exp {
			t.Errorf("event %d: expected %q, got %q", i, exp, types[i])
		}
	}
}
