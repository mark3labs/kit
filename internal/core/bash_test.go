package core

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"charm.land/fantasy"
)

// helper to create a bash tool call with the given command and optional timeout.
func bashCall(command string, timeout float64) fantasy.ToolCall {
	args := map[string]any{"command": command}
	if timeout > 0 {
		args["timeout"] = timeout
	}
	input, _ := json.Marshal(args)
	return fantasy.ToolCall{
		ID:    "test-call",
		Name:  "bash",
		Input: string(input),
	}
}

func TestBash_SimpleCommand(t *testing.T) {
	resp, err := executeBash(context.Background(), bashCall("echo hello", 0), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.IsError {
		t.Fatalf("expected success, got error: %s", resp.Content)
	}
	if resp.Content != "hello\n" {
		t.Errorf("expected 'hello\\n', got %q", resp.Content)
	}
}

func TestBash_TimeoutKillsProcess(t *testing.T) {
	start := time.Now()
	resp, err := executeBash(context.Background(), bashCall("sleep 60", 2), "")
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.IsError {
		t.Fatal("expected error response for timed-out command")
	}
	if elapsed > 10*time.Second {
		t.Errorf("command took %v, expected ~2s timeout", elapsed)
	}
}

func TestBash_BackgroundProcessDoesNotHang(t *testing.T) {
	// This command spawns a background sleep that would hold pipes open
	// forever if we didn't have process group killing + WaitDelay.
	start := time.Now()
	resp, err := executeBash(context.Background(), bashCall("echo done; sleep 3600 &", 5), "")
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// The foreground command (echo) should complete quickly
	if elapsed > 5*time.Second {
		t.Errorf("command took %v, should complete in <5s (background process should not block)", elapsed)
	}
	if resp.IsError {
		t.Fatalf("expected success, got error: %s", resp.Content)
	}
}

func TestBash_BackgroundProcessDoesNotHang_Streaming(t *testing.T) {
	// Same test but in streaming mode (with output callback).
	ctx := ContextWithToolOutputCallback(context.Background(), func(_, _, _ string, _ bool) {})
	start := time.Now()
	resp, err := executeBash(ctx, bashCall("echo streaming; sleep 3600 &", 5), "")
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if elapsed > 5*time.Second {
		t.Errorf("streaming command took %v, should complete in <5s", elapsed)
	}
	if resp.IsError {
		t.Fatalf("expected success, got error: %s", resp.Content)
	}
}

func TestBash_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = executeBash(ctx, bashCall("sleep 60", 0), "")
	}()

	// Cancel after a short delay
	time.Sleep(500 * time.Millisecond)
	cancel()

	// Should return promptly after cancellation
	select {
	case <-done:
		// success
	case <-time.After(5 * time.Second):
		t.Fatal("executeBash did not return after context cancellation")
	}
}

func TestBash_BannedCommand(t *testing.T) {
	resp, err := executeBash(context.Background(), bashCall("alias foo=bar", 0), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.IsError {
		t.Fatal("expected error for banned command")
	}
}

func TestBash_EmptyCommand(t *testing.T) {
	resp, err := executeBash(context.Background(), bashCall("", 0), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.IsError {
		t.Fatal("expected error for empty command")
	}
}
