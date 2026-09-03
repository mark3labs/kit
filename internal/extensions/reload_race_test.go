package extensions

import (
	"sync"
	"testing"
)

// mkExts builds n minimal extensions that each carry one SessionStart
// handler, so Emit reaches the per-extension mutex.
func mkExts(n int) []LoadedExtension {
	exts := make([]LoadedExtension, n)
	for i := range exts {
		exts[i] = LoadedExtension{
			Path: "ext.go",
			Handlers: map[EventType][]HandlerFunc{
				SessionStart: {func(Event, Context) Result { return nil }},
			},
		}
	}
	return exts
}

// TestReloadGrowingExtensionCountDoesNotPanic reproduces a crash seen when a
// new extension file is dropped into ~/.config/kit/extensions while Kit is
// running: the content watcher hot-reloads, the extension count grows, and
// the next event dispatch panics with
//
//	index out of range [3] with length 3
//
// at r.extMu[i].lock(). NewRunner sizes extMu to len(exts), but Reload
// replaced r.extensions without resizing extMu, so any reload that ADDS an
// extension leaves the mutex slice too short.
func TestReloadGrowingExtensionCountDoesNotPanic(t *testing.T) {
	r := NewRunner(mkExts(3))

	// Simulate the watcher picking up a newly added extension file.
	r.Reload(mkExts(4))

	if got, want := len(r.extMu), len(r.extensions); got != want {
		t.Fatalf("after Reload: len(extMu)=%d, len(extensions)=%d — they must match", got, want)
	}

	// This is the call that panicked in the wild.
	if _, err := r.Emit(SessionStartEvent{SessionID: "s"}); err != nil {
		t.Fatalf("Emit after growing reload: %v", err)
	}
	r.EmitCustomEvent("evt", "data")
}

// TestReloadShrinkingExtensionCount covers the reverse direction. This never
// panicked, but a stale oversized extMu would silently pair extensions with
// the wrong mutex after a later reload, so the invariant is asserted here too.
func TestReloadShrinkingExtensionCount(t *testing.T) {
	r := NewRunner(mkExts(5))
	r.Reload(mkExts(2))

	if got, want := len(r.extMu), len(r.extensions); got != want {
		t.Fatalf("after Reload: len(extMu)=%d, len(extensions)=%d — they must match", got, want)
	}
	if _, err := r.Emit(SessionStartEvent{SessionID: "s"}); err != nil {
		t.Fatalf("Emit after shrinking reload: %v", err)
	}
}

// TestReloadToZeroExtensions covers removing the last extension file.
func TestReloadToZeroExtensions(t *testing.T) {
	r := NewRunner(mkExts(2))
	r.Reload(nil)

	if len(r.extMu) != 0 {
		t.Fatalf("len(extMu)=%d after reloading to zero extensions, want 0", len(r.extMu))
	}
	if _, err := r.Emit(SessionStartEvent{SessionID: "s"}); err != nil {
		t.Fatalf("Emit after empty reload: %v", err)
	}
}

// TestConcurrentReloadAndEmit is the race the panic actually came from: the
// watcher goroutine calls Reload while the UI goroutine dispatches events.
// Emit iterated r.extensions and indexed r.extMu without holding r.mu, so it
// could observe a new (longer) extensions slice against an old mutex slice.
//
// Run with -race to catch the unsynchronised access as well as the panic.
func TestConcurrentReloadAndEmit(t *testing.T) {
	r := NewRunner(mkExts(3))

	var wg sync.WaitGroup
	stop := make(chan struct{})

	// Reloader: oscillate the extension count so a torn read pairs a long
	// extensions slice with a short extMu slice.
	wg.Go(func() {
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			default:
			}
			r.Reload(mkExts(1 + i%8))
		}
	})

	// Emitters.
	for range 4 {
		wg.Go(func() {
			for range 2000 {
				_, _ = r.Emit(SessionStartEvent{SessionID: "s"})
				r.EmitCustomEvent("evt", "data")
				_ = r.HasHandlers(SessionStart)
			}
		})
	}

	// Let the emitters finish, then stop the reloader.
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	for range 4 {
	}
	close(stop)
	<-done
}
