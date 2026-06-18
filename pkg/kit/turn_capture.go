package kit

import (
	"context"
	"sync"
)

// streamCollectorKey is the context key carrying the per-turn stream collector
// so the agent callbacks in generate can capture delta events into
// TurnResult.Stream without re-implementing an OnMessageUpdate handler.
type streamCollectorKey struct{}

// streamCollector accumulates StreamEvents observed during a single turn in
// emit order. It is safe for concurrent use because tool-call deltas and text
// deltas may be emitted from different goroutines.
type streamCollector struct {
	mu     sync.Mutex
	events []StreamEvent
}

func (c *streamCollector) add(e StreamEvent) {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.events = append(c.events, e)
	c.mu.Unlock()
}

func (c *streamCollector) drain() []StreamEvent {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.events) == 0 {
		return nil
	}
	out := make([]StreamEvent, len(c.events))
	copy(out, c.events)
	return out
}

// streamCollectorFromContext returns the per-turn stream collector if present.
func streamCollectorFromContext(ctx context.Context) *streamCollector {
	c, _ := ctx.Value(streamCollectorKey{}).(*streamCollector)
	return c
}
