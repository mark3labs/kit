package core

import (
	"context"
	"strings"
	"testing"
	"time"

	"charm.land/fantasy"
)

func TestValuesContext_StripsDeadlineAndCancellation(t *testing.T) {
	// Parent with a tight deadline.
	parent, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()
	time.Sleep(5 * time.Millisecond) // Let deadline expire.

	if parent.Err() == nil {
		t.Fatal("expected parent to be expired")
	}

	vc := valuesContext{parent: parent}

	if _, ok := vc.Deadline(); ok {
		t.Error("valuesContext should report no deadline")
	}
	if vc.Done() != nil {
		t.Error("valuesContext.Done() should return nil")
	}
	if vc.Err() != nil {
		t.Errorf("valuesContext.Err() should be nil, got %v", vc.Err())
	}
}

func TestValuesContext_PreservesValues(t *testing.T) {
	type testKey struct{}
	parent := context.WithValue(context.Background(), testKey{}, "hello")

	vc := valuesContext{parent: parent}

	got, ok := vc.Value(testKey{}).(string)
	if !ok || got != "hello" {
		t.Errorf("expected value 'hello', got %q (ok=%v)", got, ok)
	}
}

func TestSpawnContext_SurvivesCancelledParent(t *testing.T) {
	// Simulate the exact scenario from the bug: the parent generation
	// context is already cancelled when the subagent tool handler runs.
	parent, cancel := context.WithCancel(context.Background())
	cancel() // Cancelled before detach.

	// This is what executeSubagent now does:
	spawnCtx := context.WithoutCancel(valuesContext{parent: parent})

	// The spawn context must be alive.
	if spawnCtx.Err() != nil {
		t.Fatalf("spawnCtx should be alive, got err: %v", spawnCtx.Err())
	}

	// Adding a timeout should produce a working context.
	tCtx, tCancel := context.WithTimeout(spawnCtx, 5*time.Second)
	defer tCancel()

	if tCtx.Err() != nil {
		t.Fatalf("timeout context should be alive, got err: %v", tCtx.Err())
	}
}

func TestSpawnContext_SurvivesDeadlineExceededParent(t *testing.T) {
	// Simulate: parent had a deadline that already expired.
	parent, pCancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer pCancel()
	time.Sleep(5 * time.Millisecond)

	if parent.Err() != context.DeadlineExceeded {
		t.Fatalf("expected parent deadline exceeded, got: %v", parent.Err())
	}

	spawnCtx := context.WithoutCancel(valuesContext{parent: parent})

	if spawnCtx.Err() != nil {
		t.Fatalf("spawnCtx should be alive after deadline-exceeded parent, got: %v", spawnCtx.Err())
	}
}

func TestSpawnContext_PreservesSpawnerValue(t *testing.T) {
	// Verify the subagent spawner callback survives context detachment.
	called := false
	spawner := SubagentSpawnFunc(func(ctx context.Context, req SubagentSpawnRequest) (*SubagentSpawnResult, error) {
		called = true
		return &SubagentSpawnResult{Response: "ok"}, nil
	})

	parent := WithSubagentSpawner(context.Background(), spawner)
	// Cancel the parent.
	parentCtx, cancel := context.WithCancel(parent)
	cancel()

	spawnCtx := context.WithoutCancel(valuesContext{parent: parentCtx})

	// Should be able to retrieve the spawner from the detached context.
	recovered := getSubagentSpawner(spawnCtx)
	if recovered == nil {
		t.Fatal("spawner should be recoverable from detached context")
	}

	result, err := recovered(spawnCtx, SubagentSpawnRequest{ToolCallID: "tc1", Prompt: "test task", Timeout: time.Minute})
	if err != nil {
		t.Fatalf("spawner call failed: %v", err)
	}
	if !called {
		t.Error("spawner was not called")
	}
	if result.Response != "ok" {
		t.Errorf("expected 'ok', got %q", result.Response)
	}
}

func TestExecuteSubagent_ForwardsSessionID(t *testing.T) {
	// Issue #87: the optional session_id argument must reach the spawner so
	// the parent Kit can resume the existing subagent session.
	var captured SubagentSpawnRequest
	spawner := SubagentSpawnFunc(func(ctx context.Context, req SubagentSpawnRequest) (*SubagentSpawnResult, error) {
		captured = req
		return &SubagentSpawnResult{Response: "ok", SessionID: req.SessionID}, nil
	})

	ctx := WithSubagentSpawner(context.Background(), spawner)
	resp, err := executeSubagent(ctx, fantasy.ToolCall{
		ID:    "tc1",
		Input: `{"task":"follow-up question","session_id":"sess-123"}`,
	})
	if err != nil {
		t.Fatalf("executeSubagent: %v", err)
	}
	if resp.IsError {
		t.Fatalf("unexpected error response: %s", resp.Content)
	}
	if captured.SessionID != "sess-123" {
		t.Errorf("spawner SessionID = %q, want %q", captured.SessionID, "sess-123")
	}
	if captured.Prompt != "follow-up question" {
		t.Errorf("spawner Prompt = %q, want %q", captured.Prompt, "follow-up question")
	}
}

func TestSubagentToolDescription_MentionsResume(t *testing.T) {
	tool := NewSubagentTool()
	info := tool.Info()
	if !strings.Contains(info.Description, "session_id") {
		t.Error("tool description should document the session_id resume workflow")
	}
	if _, ok := info.Parameters["session_id"]; !ok {
		t.Error("tool parameters should include session_id")
	}
}
