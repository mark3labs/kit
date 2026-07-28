package ui

import (
	"sync"

	"github.com/charmbracelet/log"
)

// extEventDispatcher runs extension event callbacks on a single background
// goroutine, in the order they were enqueued.
//
// Two constraints pull against each other. Extension handlers must not run on
// BubbleTea's Update goroutine — a slow handler would stall the event loop —
// but a bare `go emit(...)` per event lets the scheduler interleave them, so
// two rapid events can reach the extension runner out of order.
//
// That reordering is not cosmetic. cmd/root.go records the terminal size on
// the runner *before* emitting the resize event, so a reordered pair leaves
// the runner reporting the older size to every extension that later calls
// GetTerminalSize — the live-size contract quietly breaks after a fast
// sequence of resizes. Turn-state transitions have the same hazard: a
// working/idle pair delivered backwards leaves a footer timer running.
//
// Serializing through one consumer keeps handlers off the UI goroutine while
// preserving emission order. Enqueue never blocks and never drops.
//
// Known limitation: because one consumer serves every event kind, a callback
// that blocks forever stalls all later events. Extension handlers already run
// behind Runner.safeCall, which recovers panics, and a hung handler would
// previously have wedged only its own goroutine. Bounding each callback would
// mean either abandoning ordering or leaking goroutines on timeout, so the
// stall is accepted: it requires a pathological handler, and the ordering
// guarantee is what correctness depends on.
type extEventDispatcher struct {
	mu    sync.Mutex
	queue []func()
	wake  chan struct{}
}

// newExtEventDispatcher starts the consumer goroutine. The consumer runs for
// the lifetime of the process, parked on a channel receive when idle; there is
// no teardown because the TUI model itself lives until exit.
func newExtEventDispatcher() *extEventDispatcher {
	d := &extEventDispatcher{wake: make(chan struct{}, 1)}
	go d.run()
	return d
}

// dispatch queues fn for asynchronous execution. Safe to call from the
// BubbleTea Update goroutine: it takes a short mutex and returns without
// waiting for fn to run.
func (d *extEventDispatcher) dispatch(fn func()) {
	if d == nil || fn == nil {
		return
	}

	d.mu.Lock()
	d.queue = append(d.queue, fn)
	d.mu.Unlock()

	// Non-blocking nudge: a pending wake already guarantees the consumer will
	// observe everything currently queued, so a dropped signal is harmless.
	select {
	case d.wake <- struct{}{}:
	default:
	}
}

func (d *extEventDispatcher) run() {
	for range d.wake {
		for {
			d.mu.Lock()
			if len(d.queue) == 0 {
				d.mu.Unlock()
				break
			}
			fn := d.queue[0]
			// Release the slot so a long queue does not pin memory.
			d.queue[0] = nil
			d.queue = d.queue[1:]
			d.mu.Unlock()

			d.invoke(fn)
		}
	}
}

// invoke runs one callback, isolating the consumer from panics.
//
// Extension handlers are already panic-guarded by the runner, so this is
// defence in depth for the dispatcher's own invariant: the consumer goroutine
// must never die. If it did, every later resize, turn-state, and
// thinking-level event would be dropped silently for the rest of the session.
func (d *extEventDispatcher) invoke(fn func()) {
	defer func() {
		if rec := recover(); rec != nil {
			log.Error("extension event callback panicked", "recover", rec)
		}
	}()

	fn()
}
