package ui

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// TestExtEventDispatcher_PreservesOrder is the regression guard for the
// reordering hazard that `go emit(...)` per event introduced: cmd/root.go
// records the terminal size on the runner before emitting the resize, so an
// out-of-order pair leaves the runner reporting a stale size.
//
// A plain goroutine-per-event fails this test; the serialized dispatcher
// passes it.
func TestExtEventDispatcher_PreservesOrder(t *testing.T) {
	const n = 500

	d := newExtEventDispatcher()

	var mu sync.Mutex
	got := make([]int, 0, n)
	done := make(chan struct{})

	for i := range n {
		d.dispatch(func() {
			mu.Lock()
			got = append(got, i)
			if len(got) == n {
				close(done)
			}
			mu.Unlock()
		})
	}

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		mu.Lock()
		n := len(got)
		mu.Unlock()
		t.Fatalf("timed out after %d/%d callbacks", n, 500)
	}

	mu.Lock()
	defer mu.Unlock()
	for i, v := range got {
		if v != i {
			t.Fatalf("callback %d ran at position %d; dispatch order not preserved", v, i)
		}
	}
}

// TestExtEventDispatcher_LastWriteWins models the concrete failure the
// dispatcher exists to prevent: a burst of resizes must leave the observer
// holding the final size, not an arbitrary earlier one.
func TestExtEventDispatcher_LastWriteWins(t *testing.T) {
	d := newExtEventDispatcher()

	var mu sync.Mutex
	var width int
	done := make(chan struct{})

	sizes := []int{120, 100, 80, 60, 40}
	for _, w := range sizes {
		d.dispatch(func() {
			mu.Lock()
			width = w
			if w == 40 {
				close(done)
			}
			mu.Unlock()
		})
	}

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for resize callbacks")
	}

	mu.Lock()
	defer mu.Unlock()
	if width != 40 {
		t.Errorf("final width = %d; want 40 (the last dispatched resize)", width)
	}
}

// TestExtEventDispatcher_NilSafe covers models built without the constructor
// (a zero-value AppModel in a test, say): dispatch must be inert rather than
// panic.
func TestExtEventDispatcher_NilSafe(t *testing.T) {
	var d *extEventDispatcher
	d.dispatch(func() { t.Error("nil dispatcher must not run callbacks") })

	live := newExtEventDispatcher()
	live.dispatch(nil) // must not panic

	ran := make(chan struct{})
	live.dispatch(func() { close(ran) })
	select {
	case <-ran:
	case <-time.After(2 * time.Second):
		t.Fatal("dispatcher stalled after a nil callback")
	}
}

// TestExtEventDispatcher_ConcurrentEnqueue exercises the mutex under -race:
// notifyThinkingLevel now fires from provider goroutines, so dispatch is no
// longer called only from the Update goroutine.
func TestExtEventDispatcher_ConcurrentEnqueue(t *testing.T) {
	d := newExtEventDispatcher()

	const writers, each = 8, 50
	var wg sync.WaitGroup
	var mu sync.Mutex
	seen := map[string]bool{}
	done := make(chan struct{})

	for w := range writers {
		wg.Go(func() {
			for i := range each {
				key := fmt.Sprintf("%d-%d", w, i)
				d.dispatch(func() {
					mu.Lock()
					seen[key] = true
					if len(seen) == writers*each {
						close(done)
					}
					mu.Unlock()
				})
			}
		})
	}
	wg.Wait()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		mu.Lock()
		n := len(seen)
		mu.Unlock()
		t.Fatalf("timed out: %d/%d callbacks ran", n, writers*each)
	}
}
