// Package extensions provides subagent spawning capabilities for Kit extensions.
package extensions

import (
	"fmt"
	"os"
	"sync"
	"time"
)

// ---------------------------------------------------------------------------
// Subagent types
// ---------------------------------------------------------------------------
// SubagentConfig configures a subagent spawn.
type SubagentConfig struct {
	// Prompt is the task/instruction for the subagent (required).
	Prompt string

	// Model overrides the parent's model (e.g. "anthropic/claude-haiku-3-5-20241022").
	// Empty string uses the parent's current model.
	Model string

	// SystemPrompt provides domain-specific instructions.
	// Empty string uses the default system prompt.
	SystemPrompt string

	// Timeout limits execution time. Zero means 5 minute default.
	Timeout time.Duration

	// OnOutput streams stderr output chunks as the subagent runs.
	// Called from a goroutine; must be safe for concurrent use.
	OnOutput func(chunk string)

	// OnEvent receives real-time events from the subagent's execution:
	// text chunks, tool calls, tool results, reasoning deltas, etc.
	// Called synchronously from the subagent's event loop.
	OnEvent func(SubagentEvent)

	// OnComplete is called when the subagent finishes (success or error).
	// Called from a goroutine; must be safe for concurrent use.
	OnComplete func(result SubagentResult)

	// Blocking, when true, makes SpawnSubagent wait for completion and
	// return the result directly. When false (default), spawns in background
	// and returns immediately with a handle.
	Blocking bool

	// NoSession, when true, runs the subagent without persisting a session
	// file. By default (false), subagent sessions are persisted so they can
	// be loaded for replay/inspection. Set to true for ephemeral tasks
	// where session history is not needed.
	NoSession bool

	// ParentSessionID links the subagent's session to the parent (optional).
	// When set, the subagent's session header includes a parent reference
	// so viewers can navigate the session tree.
	ParentSessionID string
}

// SubagentEvent carries a real-time event from a running subagent. Extensions
// use the Type field to determine what happened and read the relevant fields.
// This is a concrete struct (not an interface) for Yaegi compatibility.
type SubagentEvent struct {
	// Type identifies the event: "text", "reasoning", "tool_call",
	// "tool_result", "tool_execution_start", "tool_execution_end",
	// "turn_start", "turn_end".
	Type string

	// Content carries text for "text" and "reasoning" events.
	Content string

	// ToolCallID is set on tool_call, tool_result, tool_execution_start,
	// and tool_execution_end events.
	ToolCallID string
	// ToolName is set on tool-related events.
	ToolName string
	// ToolKind is set on tool-related events.
	ToolKind string
	// ToolArgs is set on tool_call events (JSON-encoded).
	ToolArgs string
	// ToolResult is set on tool_result events.
	ToolResult string
	// IsError is set on tool_result events.
	IsError bool
}

// SubagentResult contains the outcome of a subagent execution.
type SubagentResult struct {
	// Response is the subagent's final text response.
	Response string

	// Error is set if the subagent failed (nil on success).
	Error error

	// ExitCode is the subprocess exit code (0 = success).
	ExitCode int

	// Elapsed is the total execution time.
	Elapsed time.Duration

	// Usage contains token usage if available.
	Usage *SubagentUsage

	// SessionID is the subagent's session identifier, if available.
	// Populated when the subagent persists its session (requires running
	// without --no-session). Empty for ephemeral sessions.
	SessionID string
}

// SubagentUsage contains token usage from the subagent's run.
type SubagentUsage struct {
	InputTokens  int64
	OutputTokens int64
}

// SubagentHandle provides control over a running subagent.
type SubagentHandle struct {
	// ID is a unique identifier for this subagent instance.
	ID string

	proc   *os.Process
	done   chan struct{}
	result *SubagentResult
	mu     sync.Mutex
}

// Kill terminates the subagent process.
func (h *SubagentHandle) Kill() error {
	h.mu.Lock()
	proc := h.proc
	h.mu.Unlock()
	if proc != nil {
		return proc.Kill()
	}
	return nil
}

// Wait blocks until the subagent completes and returns the result.
func (h *SubagentHandle) Wait() SubagentResult {
	<-h.done
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.result != nil {
		return *h.result
	}
	return SubagentResult{Error: fmt.Errorf("subagent completed without result")}
}

// Done returns a channel that closes when the subagent completes.
func (h *SubagentHandle) Done() <-chan struct{} {
	return h.done
}
