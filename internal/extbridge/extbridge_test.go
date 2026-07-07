package extbridge

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/kit/internal/extensions"
)

// Regression tests for issue #86: SpawnSubagent with Blocking=false must
// return a usable non-nil handle and run in the background, and
// OnComplete must fire in both modes.

func TestDispatchSubagent_BlockingReturnsResultAndNilHandle(t *testing.T) {
	want := &extensions.SubagentResult{Response: "done"}
	completed := false

	run := func(context.Context) (*extensions.SubagentResult, error) {
		return want, nil
	}
	cfg := extensions.SubagentConfig{
		Blocking: true,
		OnComplete: func(r extensions.SubagentResult) {
			completed = true
			if r.Response != "done" {
				t.Errorf("OnComplete got response %q, want %q", r.Response, "done")
			}
		},
	}

	handle, result, err := dispatchSubagent(context.Background(), run, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if handle != nil {
		t.Error("blocking spawn should return nil handle")
	}
	if result == nil || result.Response != "done" {
		t.Errorf("blocking spawn result = %+v, want Response %q", result, "done")
	}
	if !completed {
		t.Error("OnComplete should fire before blocking spawn returns")
	}
}

func TestDispatchSubagent_NonBlockingReturnsHandleImmediately(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	completed := make(chan extensions.SubagentResult, 1)

	run := func(context.Context) (*extensions.SubagentResult, error) {
		close(started)
		<-release
		return &extensions.SubagentResult{Response: "background done"}, nil
	}
	cfg := extensions.SubagentConfig{
		Blocking: false,
		OnComplete: func(r extensions.SubagentResult) {
			completed <- r
		},
	}

	handle, result, err := dispatchSubagent(context.Background(), run, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if handle == nil {
		t.Fatal("non-blocking spawn must return a non-nil handle (issue #86)")
	}
	if handle.ID == "" {
		t.Error("handle.ID should be populated")
	}
	if result != nil {
		t.Errorf("non-blocking spawn should return nil result, got %+v", result)
	}

	// The run must have started in the background but not completed.
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("background run never started")
	}
	select {
	case <-handle.Done():
		t.Fatal("handle reported done while run still in flight")
	default:
	}

	close(release)

	// Wait delivers the result.
	got := handle.Wait()
	if got.Response != "background done" {
		t.Errorf("Wait() response = %q, want %q", got.Response, "background done")
	}

	// OnComplete fires asynchronously with the same result.
	select {
	case r := <-completed:
		if r.Response != "background done" {
			t.Errorf("OnComplete response = %q, want %q", r.Response, "background done")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("OnComplete never fired for background spawn")
	}
}

func TestDispatchSubagent_KillCancelsRunContext(t *testing.T) {
	run := func(ctx context.Context) (*extensions.SubagentResult, error) {
		<-ctx.Done()
		return &extensions.SubagentResult{Error: ctx.Err(), ExitCode: 1}, ctx.Err()
	}

	handle, _, err := dispatchSubagent(context.Background(), run, extensions.SubagentConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if handle == nil {
		t.Fatal("expected non-nil handle")
	}

	if err := handle.Kill(); err != nil {
		t.Fatalf("Kill() error: %v", err)
	}

	select {
	case <-handle.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("Kill() did not terminate the background run")
	}

	got := handle.Wait()
	if !errors.Is(got.Error, context.Canceled) {
		t.Errorf("Wait() error = %v, want context.Canceled", got.Error)
	}
	if got.ExitCode != 1 {
		t.Errorf("Wait() exit code = %d, want 1", got.ExitCode)
	}
}

func TestDispatchSubagent_NilResultFallback(t *testing.T) {
	run := func(context.Context) (*extensions.SubagentResult, error) {
		return nil, errors.New("boom")
	}

	handle, _, err := dispatchSubagent(context.Background(), run, extensions.SubagentConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := handle.Wait()
	if got.ExitCode != 1 {
		t.Errorf("nil-result fallback exit code = %d, want 1", got.ExitCode)
	}
}

func TestNewSubagentID(t *testing.T) {
	a, b := newSubagentID(), newSubagentID()
	if !strings.HasPrefix(a, "subagent-") {
		t.Errorf("ID %q missing subagent- prefix", a)
	}
	if a == b {
		t.Errorf("IDs should be unique, both were %q", a)
	}
}
