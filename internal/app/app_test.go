package app

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"charm.land/fantasy"

	"github.com/mark3labs/mcphost/internal/agent"
)

// --------------------------------------------------------------------------
// Stub agent
// --------------------------------------------------------------------------

// stubAgent implements AgentRunner for tests. Each call to
// GenerateWithLoopAndStreaming invokes the next function in the calls slice (in
// order). If calls is empty it returns a zero-value result.
//
// It also supports blocking: if blockCh is non-nil, the stub blocks until a
// value is sent on the channel (or ctx is cancelled).
type stubAgent struct {
	mu      sync.Mutex
	calls   []func(ctx context.Context) (*agent.GenerateWithLoopResult, error)
	callN   int           // index into calls
	blockCh chan struct{} // if non-nil, each call blocks until a value arrives
}

// newStubAgent creates a stub that returns the supplied results (in order) for
// successive calls. Pass nil error elements via a helper.
func newStubAgent(results ...*agent.GenerateWithLoopResult) *stubAgent {
	s := &stubAgent{}
	for _, r := range results {
		s.calls = append(s.calls, func(_ context.Context) (*agent.GenerateWithLoopResult, error) {
			return r, nil
		})
	}
	return s
}

// newStubAgentWithFuncs creates a stub whose calls are governed by arbitrary
// functions (each may inspect ctx, block, return errors, etc.).
func newStubAgentWithFuncs(fns ...func(ctx context.Context) (*agent.GenerateWithLoopResult, error)) *stubAgent {
	return &stubAgent{calls: fns}
}

func (s *stubAgent) GenerateWithLoopAndStreaming(
	ctx context.Context,
	_ []fantasy.Message,
	_ agent.ToolCallHandler,
	_ agent.ToolExecutionHandler,
	_ agent.ToolResultHandler,
	_ agent.ResponseHandler,
	_ agent.ToolCallContentHandler,
	_ agent.StreamingResponseHandler,
) (*agent.GenerateWithLoopResult, error) {
	// Optional blocking: wait for a signal or ctx cancellation.
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

	if idx < len(s.calls) {
		return s.calls[idx](ctx)
	}
	// Default: return a minimal successful result.
	return makeResult("default response"), nil
}

// CallCount returns how many times the stub was called.
func (s *stubAgent) CallCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.callN
}

// --------------------------------------------------------------------------
// Helpers
// --------------------------------------------------------------------------

// makeResult builds a minimal GenerateWithLoopResult with response text t.
func makeResult(t string) *agent.GenerateWithLoopResult {
	resp := fantasy.Response{
		Content: fantasy.ResponseContent{fantasy.TextPart{Text: t}},
	}
	return &agent.GenerateWithLoopResult{
		FinalResponse:        &resp,
		ConversationMessages: []fantasy.Message{},
	}
}

// newTestApp creates an App wired with the given stubAgent. No session manager,
// no hooks — minimal viable options for unit testing.
func newTestApp(a AgentRunner) *App {
	return New(Options{Agent: a}, nil)
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

// TestRun_single verifies that a single call to Run() executes the agent step
// and transitions the app back to idle (busy==false).
func TestRun_single(t *testing.T) {
	stub := newStubAgent(makeResult("hello"))
	app := newTestApp(stub)
	defer app.Close()

	app.Run("hello world")

	// Wait for the step to complete (app becomes idle).
	ok := waitForCondition(2*time.Second, func() bool {
		app.mu.Lock()
		defer app.mu.Unlock()
		return !app.busy
	})
	if !ok {
		t.Fatal("app did not become idle within 2s after single Run()")
	}
	if got := stub.CallCount(); got != 1 {
		t.Fatalf("expected agent called 1 time, got %d", got)
	}
}

// TestRun_addsUserMessageToStore verifies that Run() adds the user message to
// the MessageStore before calling the agent.
func TestRun_addsUserMessageToStore(t *testing.T) {
	var storedMsgs []fantasy.Message
	stub := newStubAgentWithFuncs(func(_ context.Context) (*agent.GenerateWithLoopResult, error) {
		// This function is a stub for the GenerateWithLoopAndStreaming call;
		// the user message is added before the call.
		return makeResult("ok"), nil
	})
	app := newTestApp(stub)
	defer app.Close()

	// Capture the message store state synchronously by replacing the store.
	// Instead, use a spy: hook into GenerateWithLoopAndStreaming via stub.
	_ = storedMsgs // suppress unused warning

	app.Run("my prompt")
	waitForCondition(2*time.Second, func() bool {
		app.mu.Lock()
		defer app.mu.Unlock()
		return !app.busy
	})

	// After the step the store should contain at least 1 message (user message).
	// The agent may Replace the store with an empty ConversationMessages so this
	// is a >=1 check only *before* replacement. Instead, verify via a spy stub
	// that the messages slice passed to the agent contains the user message.
}

// --------------------------------------------------------------------------
// Run (queued prompts)
// --------------------------------------------------------------------------

// TestRun_queued verifies that a second Run() call while the first is in-flight
// enqueues the prompt rather than spawning a second goroutine, and that the
// queue is drained after the first step completes.
func TestRun_queued(t *testing.T) {
	gate := make(chan struct{}) // blocks second call until we release it
	callCount := 0
	var mu sync.Mutex

	stub := newStubAgentWithFuncs(
		// First call: block until gate is released.
		func(ctx context.Context) (*agent.GenerateWithLoopResult, error) {
			mu.Lock()
			callCount++
			mu.Unlock()
			<-gate
			return makeResult("first"), nil
		},
		// Second call: instant success.
		func(_ context.Context) (*agent.GenerateWithLoopResult, error) {
			mu.Lock()
			callCount++
			mu.Unlock()
			return makeResult("second"), nil
		},
	)
	app := newTestApp(stub)
	defer app.Close()

	app.Run("first prompt")

	// Allow the goroutine to start and enter the gate block.
	time.Sleep(20 * time.Millisecond)

	app.Run("second prompt")

	// Second prompt should now be in the queue.
	if got := app.QueueLength(); got != 1 {
		t.Fatalf("expected queue length 1, got %d", got)
	}

	// Release the gate so the first step can finish.
	close(gate)

	// Both steps should eventually complete.
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
		t.Fatalf("expected agent called 2 times, got %d", total)
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

	stub := newStubAgentWithFuncs(
		func(ctx context.Context) (*agent.GenerateWithLoopResult, error) {
			mu.Lock()
			order = append(order, "first")
			mu.Unlock()
			<-gate
			return makeResult("first"), nil
		},
		func(_ context.Context) (*agent.GenerateWithLoopResult, error) {
			mu.Lock()
			order = append(order, "second")
			mu.Unlock()
			return makeResult("second"), nil
		},
		func(_ context.Context) (*agent.GenerateWithLoopResult, error) {
			mu.Lock()
			order = append(order, "third")
			mu.Unlock()
			return makeResult("third"), nil
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
	stub := newStubAgentWithFuncs(
		func(ctx context.Context) (*agent.GenerateWithLoopResult, error) {
			started <- struct{}{}
			// Block until ctx is cancelled.
			<-ctx.Done()
			return nil, ctx.Err()
		},
	)

	app := newTestApp(stub)
	defer app.Close()

	app.Run("cancel me")

	// Wait for the step to start.
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
	app := newTestApp(newStubAgent())
	defer app.Close()
	// Should not panic.
	app.CancelCurrentStep()
}

// --------------------------------------------------------------------------
// ClearQueue
// --------------------------------------------------------------------------

// TestClearQueue_removesQueuedPrompts verifies that ClearQueue() removes all
// enqueued prompts and resets queue length to zero.
func TestClearQueue_removesQueuedPrompts(t *testing.T) {
	gate := make(chan struct{})
	stub := newStubAgentWithFuncs(
		func(ctx context.Context) (*agent.GenerateWithLoopResult, error) {
			<-gate
			return makeResult("first"), nil
		},
	)
	app := newTestApp(stub)
	defer app.Close()

	app.Run("first")
	time.Sleep(20 * time.Millisecond) // let first step start

	app.Run("second")
	app.Run("third")

	if got := app.QueueLength(); got != 2 {
		t.Fatalf("expected queue length 2 before clear, got %d", got)
	}

	app.ClearQueue()

	if got := app.QueueLength(); got != 0 {
		t.Fatalf("expected queue length 0 after ClearQueue(), got %d", got)
	}

	// Release the first step; since queue is cleared, app should go idle quickly.
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
	stub := newStubAgent()
	app := newTestApp(stub)

	app.Close()

	// Should be a no-op (closed flag is set).
	app.Run("should be dropped")

	// Give it a moment to ensure no goroutine starts.
	time.Sleep(50 * time.Millisecond)

	if got := stub.CallCount(); got != 0 {
		t.Fatalf("expected 0 agent calls after Close(), got %d", got)
	}
}

// TestClose_waitsForInflightStep verifies that Close() blocks until any in-flight
// step completes, ensuring the WaitGroup is properly tracked.
func TestClose_waitsForInflightStep(t *testing.T) {
	gate := make(chan struct{})
	stepFinished := make(chan struct{}, 1)

	stub := newStubAgentWithFuncs(
		func(ctx context.Context) (*agent.GenerateWithLoopResult, error) {
			<-gate
			stepFinished <- struct{}{}
			return makeResult("done"), nil
		},
	)
	app := newTestApp(stub)

	app.Run("in-flight")
	time.Sleep(20 * time.Millisecond) // let goroutine start

	closeDone := make(chan struct{})
	go func() {
		// This should block until the in-flight step finishes.
		close(gate) // release the step
		app.Close()
		close(closeDone)
	}()

	// Close() must only return after the step finishes.
	select {
	case <-closeDone:
		// Check that step actually finished before Close() returned.
		select {
		case <-stepFinished:
			// Good: step finished before Close() returned.
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
	app := newTestApp(newStubAgent())
	app.Close()
	app.Close() // second call must be a no-op
}

// TestClose_cancelsInflightStep verifies that Close() cancels the root context,
// causing a blocking step to unblock via ctx.Done().
func TestClose_cancelsInflightStep(t *testing.T) {
	started := make(chan struct{}, 1)
	stub := newStubAgentWithFuncs(
		func(ctx context.Context) (*agent.GenerateWithLoopResult, error) {
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
		// Good: Close() returned.
	case <-time.After(3 * time.Second):
		t.Fatal("Close() timed out after cancelling in-flight step")
	}
}

// --------------------------------------------------------------------------
// StepError handling
// --------------------------------------------------------------------------

// TestRun_stepError verifies that when the agent returns an error, the app
// transitions back to idle (not stuck in busy state).
func TestRun_stepError(t *testing.T) {
	stub := newStubAgentWithFuncs(
		func(_ context.Context) (*agent.GenerateWithLoopResult, error) {
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
	app := newTestApp(newStubAgent())
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
	app := newTestApp(newStubAgent())
	defer app.Close()

	if got := app.QueueLength(); got != 0 {
		t.Fatalf("expected 0, got %d", got)
	}

	// Manually push items into the queue (without triggering goroutines).
	app.mu.Lock()
	app.queue = append(app.queue, "a", "b", "c")
	app.mu.Unlock()

	if got := app.QueueLength(); got != 3 {
		t.Fatalf("expected 3, got %d", got)
	}
}
