package extensions

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestRunner_State_BasicSetGetDelete(t *testing.T) {
	r := NewRunner(nil)

	if _, ok := r.GetState("missing"); ok {
		t.Fatal("expected GetState to return ok=false for missing key")
	}

	r.SetState("a", "1")
	r.SetState("b", "2")
	r.SetState("a", "3") // last-write-wins

	if v, ok := r.GetState("a"); !ok || v != "3" {
		t.Errorf("expected GetState(a)=(3,true), got (%q,%v)", v, ok)
	}
	if v, ok := r.GetState("b"); !ok || v != "2" {
		t.Errorf("expected GetState(b)=(2,true), got (%q,%v)", v, ok)
	}

	keys := r.ListState()
	if len(keys) != 2 {
		t.Errorf("expected 2 keys, got %d (%v)", len(keys), keys)
	}

	r.DeleteState("a")
	if _, ok := r.GetState("a"); ok {
		t.Error("expected key a to be gone after DeleteState")
	}
	if len(r.ListState()) != 1 {
		t.Errorf("expected 1 key after delete, got %v", r.ListState())
	}

	// Deleting missing key is a no-op.
	r.DeleteState("never-there")
}

func TestRunner_State_SaverFires(t *testing.T) {
	r := NewRunner(nil)
	var calls int
	var mu sync.Mutex
	r.SetStateSaver(func() {
		mu.Lock()
		calls++
		mu.Unlock()
	})

	r.SetState("a", "1")
	r.SetState("a", "2")
	r.DeleteState("a")
	r.DeleteState("a") // missing → no save

	mu.Lock()
	defer mu.Unlock()
	if calls != 3 {
		t.Errorf("expected saver to fire 3 times (2 sets + 1 delete), got %d", calls)
	}
}

func TestRunner_State_SaveAndLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ext-state.json")

	r1 := NewRunner(nil)
	r1.SetState("k1", "v1")
	r1.SetState("k2", `{"json":"value"}`)
	if err := r1.SaveStateToFile(path); err != nil {
		t.Fatalf("SaveStateToFile: %v", err)
	}

	// Verify file contains JSON map.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading saved file: %v", err)
	}
	var parsed map[string]string
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshalling: %v", err)
	}
	if parsed["k1"] != "v1" || parsed["k2"] != `{"json":"value"}` {
		t.Errorf("unexpected file contents: %v", parsed)
	}

	r2 := NewRunner(nil)
	if err := r2.LoadStateFromFile(path); err != nil {
		t.Fatalf("LoadStateFromFile: %v", err)
	}
	if v, ok := r2.GetState("k1"); !ok || v != "v1" {
		t.Errorf("expected k1=v1 after load, got (%q,%v)", v, ok)
	}
	if v, ok := r2.GetState("k2"); !ok || v != `{"json":"value"}` {
		t.Errorf("expected k2 to round-trip, got %q", v)
	}
}

func TestRunner_State_LoadMissingFileClearsState(t *testing.T) {
	// LoadStateFromFile is documented to "replace the in-memory state store
	// with its contents"; for a missing file that means clearing the store.
	// This is what makes session-switching safe: a new session that has not
	// yet written a sidecar must not inherit keys from a prior session.
	r := NewRunner(nil)
	r.SetState("a", "1")
	if err := r.LoadStateFromFile(filepath.Join(t.TempDir(), "does-not-exist.json")); err != nil {
		t.Errorf("expected nil error for missing file, got %v", err)
	}
	if _, ok := r.GetState("a"); ok {
		t.Error("expected pre-existing state to be cleared when target file is missing")
	}
	if keys := r.ListState(); keys != nil {
		t.Errorf("expected ListState() to be nil after clearing, got %v", keys)
	}
}

func TestRunner_State_LoadEmptyFileClearsState(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.json")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	r := NewRunner(nil)
	r.SetState("a", "1")
	if err := r.LoadStateFromFile(path); err != nil {
		t.Errorf("expected nil error for empty file, got %v", err)
	}
	if _, ok := r.GetState("a"); ok {
		t.Error("expected pre-existing state to be cleared when target file is empty")
	}
}

func TestRunner_State_LoadMalformedFileError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := NewRunner(nil)
	if err := r.LoadStateFromFile(path); err == nil {
		t.Error("expected error loading malformed JSON, got nil")
	}
}

func TestRunner_State_PersistenceViaSaver(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ext-state.json")

	r := NewRunner(nil)
	r.SetStateSaver(func() {
		_ = r.SaveStateToFile(path)
	})
	r.SetState("hello", "world")

	// File should exist with the value already.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading saved file: %v", err)
	}
	var parsed map[string]string
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshalling: %v", err)
	}
	if parsed["hello"] != "world" {
		t.Errorf("expected file to contain hello=world, got %v", parsed)
	}
}

func TestRunner_State_ConcurrentSet(t *testing.T) {
	r := NewRunner(nil)
	var wg sync.WaitGroup
	const goroutines = 16
	const iterations = 100
	wg.Add(goroutines)
	for range goroutines {
		go func() {
			defer wg.Done()
			for range iterations {
				r.SetState("k", "v")
				_, _ = r.GetState("k")
			}
		}()
	}
	wg.Wait()
	if v, ok := r.GetState("k"); !ok || v != "v" {
		t.Errorf("expected k=v after concurrent writes, got (%q,%v)", v, ok)
	}
}

func TestRunner_State_ContextNoOpsWhenUnset(t *testing.T) {
	// Verify normalizeContext installs safe no-ops for SetState/GetState/etc.
	// when not provided by the caller.
	ext := makeHandlerExt("state.go", map[EventType][]HandlerFunc{
		SessionStart: {
			func(e Event, c Context) Result {
				// All four state functions should be non-nil and safe to call.
				c.SetState("a", "b")
				if v, ok := c.GetState("a"); ok || v != "" {
					t.Errorf("no-op GetState should return (\"\", false); got (%q,%v)", v, ok)
				}
				c.DeleteState("a")
				if keys := c.ListState(); keys != nil {
					t.Errorf("no-op ListState should return nil; got %v", keys)
				}
				return nil
			},
		},
	})
	r := makeRunner(ext)
	// SetContext with empty Context to exercise normalizeContext defaults.
	r.SetContext(Context{})
	_, err := r.Emit(SessionStartEvent{})
	if err != nil {
		t.Fatalf("emit: %v", err)
	}
}

func TestRunner_State_SaverPanicReleasesSaverMu(t *testing.T) {
	// If the saver callback panics (e.g. disk full mid-write), runSaver
	// must still release saverMu so subsequent SetState/DeleteState calls
	// can make progress. Without `defer Unlock()` the lock would be
	// permanently held and the next write would deadlock.
	r := NewRunner(nil)
	var calls int
	r.SetStateSaver(func() {
		calls++
		if calls == 1 {
			panic("simulated disk-write failure")
		}
	})

	// First call panics. Recover, then verify a follow-up call still works
	// without blocking (proving saverMu was released).
	func() {
		defer func() {
			if rec := recover(); rec == nil {
				t.Fatal("expected panic from first saver invocation")
			}
		}()
		r.SetState("a", "1")
	}()

	done := make(chan struct{})
	go func() {
		r.SetState("b", "2") // would deadlock if saverMu were still held
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("SetState after saver panic blocked — saverMu was not released")
	}
	if calls != 2 {
		t.Errorf("expected saver to fire twice (panic + recovery write), got %d", calls)
	}
}
