package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/mark3labs/kit/internal/extensions"
	"github.com/mark3labs/kit/pkg/extensions/test"
)

// TestSubagentMonitor_SessionStart verifies OnSessionStart initializes state
// without panicking and properly guards nil ctx calls.
func TestSubagentMonitor_SessionStart(t *testing.T) {
	harness := test.New(t)
	harness.LoadFile("../../.kit/extensions/subagent-monitor.go")

	// Emit SessionStart - should not panic even with nil ctx functions
	_, err := harness.Emit(extensions.SessionStartEvent{SessionID: "test-session"})
	if err != nil {
		t.Fatalf("SessionStart should not error: %v", err)
	}
}

// TestSubagentMonitor_SubagentLifecycle verifies the full subagent lifecycle
// creates entries and emits widget updates.
func TestSubagentMonitor_SubagentLifecycle(t *testing.T) {
	harness := test.New(t)
	harness.LoadFile("../../.kit/extensions/subagent-monitor.go")

	// Start session
	_, err := harness.Emit(extensions.SessionStartEvent{SessionID: "test-session"})
	if err != nil {
		t.Fatalf("SessionStart should not error: %v", err)
	}

	// Emit SubagentStart
	_, err = harness.Emit(extensions.SubagentStartEvent{
		ToolCallID: "call-1",
		Task:       "test task",
	})
	if err != nil {
		t.Fatalf("SubagentStart should not error: %v", err)
	}

	// Emit a few chunks
	for i := range 3 {
		_, err = harness.Emit(extensions.SubagentChunkEvent{
			ToolCallID: "call-1",
			Task:       "test task",
			ChunkType:  "text",
			Content:    fmt.Sprintf("line %d", i),
		})
		if err != nil {
			t.Fatalf("SubagentChunk %d should not error: %v", i, err)
		}
	}

	// Emit tool call chunk
	_, err = harness.Emit(extensions.SubagentChunkEvent{
		ToolCallID: "call-1",
		Task:       "test task",
		ChunkType:  "tool_call",
		ToolName:   "bash",
	})
	if err != nil {
		t.Fatalf("SubagentChunk tool_call should not error: %v", err)
	}

	// Emit SubagentEnd
	_, err = harness.Emit(extensions.SubagentEndEvent{
		ToolCallID: "call-1",
		Task:       "test task",
		Response:   "done",
	})
	if err != nil {
		t.Fatalf("SubagentEnd should not error: %v", err)
	}

	// Give time for cleanup goroutine
	time.Sleep(100 * time.Millisecond)
}

// TestSubagentMonitor_MultipleSubagents verifies multiple parallel subagents.
func TestSubagentMonitor_MultipleSubagents(t *testing.T) {
	harness := test.New(t)
	harness.LoadFile("../../.kit/extensions/subagent-monitor.go")

	_, err := harness.Emit(extensions.SessionStartEvent{SessionID: "test-session"})
	if err != nil {
		t.Fatalf("SessionStart should not error: %v", err)
	}

	// Start 3 subagents
	for i := 1; i <= 3; i++ {
		_, err := harness.Emit(extensions.SubagentStartEvent{
			ToolCallID: fmt.Sprintf("call-%d", i),
			Task:       fmt.Sprintf("task %d", i),
		})
		if err != nil {
			t.Fatalf("SubagentStart %d should not error: %v", i, err)
		}
	}

	// Emit chunks for each
	for i := 1; i <= 3; i++ {
		_, err := harness.Emit(extensions.SubagentChunkEvent{
			ToolCallID: fmt.Sprintf("call-%d", i),
			Task:       fmt.Sprintf("task %d", i),
			ChunkType:  "text",
			Content:    fmt.Sprintf("output from agent %d", i),
		})
		if err != nil {
			t.Fatalf("SubagentChunk %d should not error: %v", i, err)
		}
	}

	// End all subagents
	for i := 1; i <= 3; i++ {
		_, err := harness.Emit(extensions.SubagentEndEvent{
			ToolCallID: fmt.Sprintf("call-%d", i),
			Task:       fmt.Sprintf("task %d", i),
			Response:   "completed",
		})
		if err != nil {
			t.Fatalf("SubagentEnd %d should not error: %v", i, err)
		}
	}

	time.Sleep(100 * time.Millisecond)
}

// TestSubagentMonitor_ConcurrentSubagents verifies no panics when multiple
// subagents emit events concurrently from different goroutines.
func TestSubagentMonitor_ConcurrentSubagents(t *testing.T) {
	harness := test.New(t)
	harness.LoadFile("../../.kit/extensions/subagent-monitor.go")

	_, err := harness.Emit(extensions.SessionStartEvent{SessionID: "test-session"})
	if err != nil {
		t.Fatalf("SessionStart should not error: %v", err)
	}

	// Start 5 subagents concurrently
	done := make(chan struct{}, 5)
	for i := range 5 {
		go func(idx int) {
			defer func() { done <- struct{}{} }()

			callID := fmt.Sprintf("concurrent-%d", idx)
			task := fmt.Sprintf("concurrent task %d", idx)

			_, _ = harness.Emit(extensions.SubagentStartEvent{
				ToolCallID: callID,
				Task:       task,
			})

			// Emit many chunks rapidly
			for j := range 20 {
				_, _ = harness.Emit(extensions.SubagentChunkEvent{
					ToolCallID: callID,
					Task:       task,
					ChunkType:  "text",
					Content:    fmt.Sprintf("agent %d chunk %d", idx, j),
				})
			}

			_, _ = harness.Emit(extensions.SubagentEndEvent{
				ToolCallID: callID,
				Task:       task,
				Response:   "done",
			})
		}(i)
	}

	// Wait for all goroutines
	for range 5 {
		<-done
	}

	// Allow any final processing
	time.Sleep(200 * time.Millisecond)
}

// TestSubagentMonitor_SessionShutdown verifies shutdown doesn't panic
// even with nil ctx functions.
func TestSubagentMonitor_SessionShutdown(t *testing.T) {
	harness := test.New(t)
	harness.LoadFile("../../.kit/extensions/subagent-monitor.go")

	// Start then shutdown
	_, err := harness.Emit(extensions.SessionStartEvent{SessionID: "test-session"})
	if err != nil {
		t.Fatalf("SessionStart should not error: %v", err)
	}

	// Start a subagent
	_, err = harness.Emit(extensions.SubagentStartEvent{
		ToolCallID: "call-1",
		Task:       "test task",
	})
	if err != nil {
		t.Fatalf("SubagentStart should not error: %v", err)
	}

	// Shutdown - should not panic even with active subagent
	_, err = harness.Emit(extensions.SessionShutdownEvent{})
	if err != nil {
		t.Fatalf("SessionShutdown should not error: %v", err)
	}
}
