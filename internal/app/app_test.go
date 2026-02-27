package app

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	kit "github.com/mark3labs/kit/pkg/kit"
)

// --------------------------------------------------------------------------
// Helpers
// --------------------------------------------------------------------------

// turnResult builds a minimal TurnResult with response text t.
func turnResult(t string) *kit.TurnResult {
	return &kit.TurnResult{Response: t}
}

// stubPromptFunc returns a PromptFunc that invokes successive functions from
// fns. Each function can block, return errors, etc. If fns is exhausted, a
// default success result is returned.
type stubPrompt struct {
	mu      sync.Mutex
	fns     []func(ctx context.Context) (*kit.TurnResult, error)
	callN   int
	blockCh chan struct{} // if non-nil, each call blocks until a value arrives
}

func (s *stubPrompt) fn(ctx context.Context, _ string) (*kit.TurnResult, error) {
	if s.blockCh != nil {
		select {
		case <-s.blockCh:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	s.mu.Lock()
	idx := s.callN
	s.callN++
	s.mu.Unlock()

	if idx < len(s.fns) {
		return s.fns[idx](ctx)
	}
	return turnResult("default response"), nil
}

func (s *stubPrompt) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.callN
}

// newTestApp creates an App wired with the given stub prompt function.
func newTestApp(s *stubPrompt) *App {
	return New(Options{PromptFunc: s.fn}, nil)
}

// newStub creates a stubPrompt that returns the given results in order.
func newStub(results ...string) *stubPrompt {
	s := &stubPrompt{}
	for _, r := range results {
		s.fns = append(s.fns, func(_ context.Context) (*kit.TurnResult, error) {
			return turnResult(r), nil
		})
	}
	return s
}

// newStubWithFuncs creates a stubPrompt whose calls are governed by arbitrary
// functions (each may inspect ctx, block, return errors, etc.).
func newStubWithFuncs(fns ...func(ctx context.Context) (*kit.TurnResult, error)) *stubPrompt {
	return &stubPrompt{fns: fns}
}

// waitForCondition polls fn() up to maxWait, returning true if fn returns true
// before the deadline.
func waitForCondition(maxWait time.Duration, fn func() bool) bool {
	deadline := time.Now().Add(maxWait)
	for time.Now().Before(deadline) {
		if fn() {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return false
}

// --------------------------------------------------------------------------
// Run (single prompt)
// --------------------------------------------------------------------------

// TestRun_single verifies that a single call to Run() executes the prompt
// and transitions the app back to idle (busy==false).
func TestRun_single(t *testing.T) {
	stub := newStub("hello")
	app := newTestApp(stub)
	defer app.Close()

	app.Run("hello world")

	ok := waitForCondition(2*time.Second, func() bool {
		app.mu.Lock()
		defer app.mu.Unlock()
		return !app.busy
	})
	if !ok {
		t.Fatal("app did not become idle within 2s after single Run()")
	}
	if got := stub.callCount(); got != 1 {
		t.Fatalf("expected 1 call, got %d", got)
	}
}

// --------------------------------------------------------------------------
// Run (queued prompts)
// --------------------------------------------------------------------------

// TestRun_queued verifies that a second Run() call while the first is in-flight
// enqueues the prompt rather than spawning a second goroutine, and that the
// queue is drained after the first step completes.
func TestRun_queued(t *testing.T) {
	gate := make(chan struct{})
	callCount := 0
	var mu sync.Mutex

	stub := newStubWithFuncs(
		func(ctx context.Context) (*kit.TurnResult, error) {
			mu.Lock()
			callCount++
			mu.Unlock()
			<-gate
			return turnResult("first"), nil
		},
		func(_ context.Context) (*kit.TurnResult, error) {
			mu.Lock()
			callCount++
			mu.Unlock()
			return turnResult("second"), nil
		},
	)
	app := newTestApp(stub)
	defer app.Close()

	app.Run("first prompt")
	time.Sleep(20 * time.Millisecond)
	app.Run("second prompt")

	if got := app.QueueLength(); got != 1 {
		t.Fatalf("expected queue length 1, got %d", got)
	}

	close(gate)

	ok := waitForCondition(3*time.Second, func() bool {
		app.mu.Lock()
		defer app.mu.Unlock()
		return !app.busy
	})
	if !ok {
		t.Fatal("app did not become idle within 3s after queued runs")
	}

	mu.Lock()
	total := callCount
	mu.Unlock()
	if total != 2 {
		t.Fatalf("expected 2 calls, got %d", total)
	}
	if got := app.QueueLength(); got != 0 {
		t.Fatalf("expected empty queue after drain, got %d", got)
	}
}

// --------------------------------------------------------------------------
// Queue drain ordering
// --------------------------------------------------------------------------

// TestQueueDrainOrdering verifies that queued prompts are consumed in FIFO order.
func TestQueueDrainOrdering(t *testing.T) {
	gate := make(chan struct{})
	var order []string
	var mu sync.Mutex

	stub := newStubWithFuncs(
		func(ctx context.Context) (*kit.TurnResult, error) {
			mu.Lock()
			order = append(order, "first")
			mu.Unlock()
			<-gate
			return turnResult("first"), nil
		},
		func(_ context.Context) (*kit.TurnResult, error) {
			mu.Lock()
			order = append(order, "second")
			mu.Unlock()
			return turnResult("second"), nil
		},
		func(_ context.Context) (*kit.TurnResult, error) {
			mu.Lock()
			order = append(order, "third")
			mu.Unlock()
			return turnResult("third"), nil
		},
	)

	app := newTestApp(stub)
	defer app.Close()

	app.Run("first")
	time.Sleep(20 * time.Millisecond)
	app.Run("second")
	app.Run("third")

	close(gate)

	ok := waitForCondition(3*time.Second, func() bool {
		app.mu.Lock()
		defer app.mu.Unlock()
		return !app.busy
	})
	if !ok {
		t.Fatal("app did not become idle within 3s")
	}

	mu.Lock()
	got := order
	mu.Unlock()

	if len(got) != 3 {
		t.Fatalf("expected 3 calls, got %d: %v", len(got), got)
	}
	for i, want := range []string{"first", "second", "third"} {
		if got[i] != want {
			t.Fatalf("call[%d]: expected %q, got %q", i, want, got[i])
		}
	}
}

// --------------------------------------------------------------------------
// CancelCurrentStep
// --------------------------------------------------------------------------

// TestCancelCurrentStep_cancelsInflightStep verifies that CancelCurrentStep()
// causes an in-flight step to receive a cancelled context and the app
// eventually transitions to idle.
func TestCancelCurrentStep_cancelsInflightStep(t *testing.T) {
	started := make(chan struct{}, 1)
	stub := newStubWithFuncs(
		func(ctx context.Context) (*kit.TurnResult, error) {
			started <- struct{}{}
			<-ctx.Done()
			return nil, ctx.Err()
		},
	)

	app := newTestApp(stub)
	defer app.Close()

	app.Run("cancel me")

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("step never started")
	}

	app.CancelCurrentStep()

	ok := waitForCondition(2*time.Second, func() bool {
		app.mu.Lock()
		defer app.mu.Unlock()
		return !app.busy
	})
	if !ok {
		t.Fatal("app did not become idle after CancelCurrentStep()")
	}
}

// TestCancelCurrentStep_safeWhenIdle verifies that calling CancelCurrentStep()
// when no step is in-flight is a no-op and does not panic.
func TestCancelCurrentStep_safeWhenIdle(t *testing.T) {
	app := newTestApp(newStub())
	defer app.Close()
	app.CancelCurrentStep()
}

// --------------------------------------------------------------------------
// ClearQueue
// --------------------------------------------------------------------------

// TestClearQueue_removesQueuedPrompts verifies that ClearQueue() removes all
// enqueued prompts and resets queue length to zero.
func TestClearQueue_removesQueuedPrompts(t *testing.T) {
	gate := make(chan struct{})
	stub := newStubWithFuncs(
		func(ctx context.Context) (*kit.TurnResult, error) {
			<-gate
			return turnResult("first"), nil
		},
	)
	app := newTestApp(stub)
	defer app.Close()

	app.Run("first")
	time.Sleep(20 * time.Millisecond)

	app.Run("second")
	app.Run("third")

	if got := app.QueueLength(); got != 2 {
		t.Fatalf("expected queue length 2 before clear, got %d", got)
	}

	app.ClearQueue()

	if got := app.QueueLength(); got != 0 {
		t.Fatalf("expected queue length 0 after ClearQueue(), got %d", got)
	}

	close(gate)
	ok := waitForCondition(2*time.Second, func() bool {
		app.mu.Lock()
		defer app.mu.Unlock()
		return !app.busy
	})
	if !ok {
		t.Fatal("app did not become idle after ClearQueue + first step complete")
	}
}

// --------------------------------------------------------------------------
// Close
// --------------------------------------------------------------------------

// TestClose_preventsNewRuns verifies that after Close() is called, subsequent
// Run() calls are silently dropped (no goroutine spawned).
func TestClose_preventsNewRuns(t *testing.T) {
	stub := newStub()
	app := newTestApp(stub)

	app.Close()

	app.Run("should be dropped")
	time.Sleep(50 * time.Millisecond)

	if got := stub.callCount(); got != 0 {
		t.Fatalf("expected 0 calls after Close(), got %d", got)
	}
}

// TestClose_waitsForInflightStep verifies that Close() blocks until any in-flight
// step completes, ensuring the WaitGroup is properly tracked.
func TestClose_waitsForInflightStep(t *testing.T) {
	gate := make(chan struct{})
	stepFinished := make(chan struct{}, 1)

	stub := newStubWithFuncs(
		func(_ context.Context) (*kit.TurnResult, error) {
			<-gate
			stepFinished <- struct{}{}
			return turnResult("done"), nil
		},
	)
	app := newTestApp(stub)

	app.Run("in-flight")
	time.Sleep(20 * time.Millisecond)

	closeDone := make(chan struct{})
	go func() {
		close(gate)
		app.Close()
		close(closeDone)
	}()

	select {
	case <-closeDone:
		select {
		case <-stepFinished:
		default:
			t.Error("Close() returned before step finished")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Close() timed out waiting for in-flight step")
	}
}

// TestClose_idempotent verifies that calling Close() multiple times does not
// panic or deadlock.
func TestClose_idempotent(t *testing.T) {
	app := newTestApp(newStub())
	app.Close()
	app.Close()
}

// TestClose_cancelsInflightStep verifies that Close() cancels the root context,
// causing a blocking step to unblock via ctx.Done().
func TestClose_cancelsInflightStep(t *testing.T) {
	started := make(chan struct{}, 1)
	stub := newStubWithFuncs(
		func(ctx context.Context) (*kit.TurnResult, error) {
			started <- struct{}{}
			<-ctx.Done()
			return nil, ctx.Err()
		},
	)
	app := newTestApp(stub)

	app.Run("in-flight")
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("step never started")
	}

	closeDone := make(chan struct{})
	go func() {
		app.Close()
		close(closeDone)
	}()

	select {
	case <-closeDone:
	case <-time.After(3 * time.Second):
		t.Fatal("Close() timed out after cancelling in-flight step")
	}
}

// --------------------------------------------------------------------------
// StepError handling
// --------------------------------------------------------------------------

// TestRun_stepError verifies that when the prompt returns an error, the app
// transitions back to idle (not stuck in busy state).
func TestRun_stepError(t *testing.T) {
	stub := newStubWithFuncs(
		func(_ context.Context) (*kit.TurnResult, error) {
			return nil, errors.New("agent exploded")
		},
	)
	app := newTestApp(stub)
	defer app.Close()

	app.Run("trigger error")

	ok := waitForCondition(2*time.Second, func() bool {
		app.mu.Lock()
		defer app.mu.Unlock()
		return !app.busy
	})
	if !ok {
		t.Fatal("app stuck in busy state after step error")
	}
}

// --------------------------------------------------------------------------
// ClearMessages
// --------------------------------------------------------------------------

// TestClearMessages_emptiesStore verifies that ClearMessages() empties the store.
func TestClearMessages_emptiesStore(t *testing.T) {
	app := newTestApp(newStub())
	defer app.Close()

	app.store.Add(makeTextMsg("user", "hello"))
	if app.store.Len() != 1 {
		t.Fatalf("expected 1 message before clear, got %d", app.store.Len())
	}

	app.ClearMessages()

	if app.store.Len() != 0 {
		t.Fatalf("expected 0 messages after ClearMessages(), got %d", app.store.Len())
	}
}

// --------------------------------------------------------------------------
// QueueLength
// --------------------------------------------------------------------------

// TestQueueLength_reflects verifies that QueueLength() accurately reflects
// the queue depth.
func TestQueueLength_reflects(t *testing.T) {
	app := newTestApp(newStub())
	defer app.Close()

	if got := app.QueueLength(); got != 0 {
		t.Fatalf("expected 0, got %d", got)
	}

	app.mu.Lock()
	app.queue = append(app.queue, "a", "b", "c")
	app.mu.Unlock()

	if got := app.QueueLength(); got != 3 {
		t.Fatalf("expected 3, got %d", got)
	}
}
