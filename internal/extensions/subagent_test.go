package extensions

import (
	"testing"
	"time"
)

func TestSubagentHandle_CompleteWaitDone(t *testing.T) {
	h := NewSubagentHandle("subagent-test", nil)

	select {
	case <-h.Done():
		t.Fatal("Done() closed before Complete()")
	default:
	}

	h.Complete(SubagentResult{Response: "ok"})

	select {
	case <-h.Done():
	case <-time.After(time.Second):
		t.Fatal("Done() not closed after Complete()")
	}

	if got := h.Wait(); got.Response != "ok" {
		t.Errorf("Wait() response = %q, want %q", got.Response, "ok")
	}
}

func TestSubagentHandle_CompleteIsIdempotent(t *testing.T) {
	h := NewSubagentHandle("subagent-test", nil)
	h.Complete(SubagentResult{Response: "first"})
	// A second Complete must not panic (double close) and must not
	// overwrite the recorded result.
	h.Complete(SubagentResult{Response: "second"})

	if got := h.Wait(); got.Response != "first" {
		t.Errorf("Wait() response = %q, want %q", got.Response, "first")
	}
}

func TestSubagentHandle_KillInvokesCancel(t *testing.T) {
	cancelled := false
	h := NewSubagentHandle("subagent-test", func() { cancelled = true })

	if err := h.Kill(); err != nil {
		t.Fatalf("Kill() error: %v", err)
	}
	if !cancelled {
		t.Error("Kill() did not invoke cancel func")
	}
}

func TestSubagentHandle_KillWithoutCancelIsNoop(t *testing.T) {
	h := NewSubagentHandle("subagent-test", nil)
	if err := h.Kill(); err != nil {
		t.Errorf("Kill() with nil cancel should be a no-op, got error: %v", err)
	}
}
