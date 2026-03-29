// Package extensions provides subagent spawning capabilities for Kit extensions.
package extensions

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
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

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// subagentJSONOutput matches the JSON envelope produced by `kit --json`.
type subagentJSONOutput struct {
	Response   string `json:"response"`
	StopReason string `json:"stop_reason,omitempty"`
	SessionID  string `json:"session_id,omitempty"`
	Usage      *struct {
		InputTokens  int64 `json:"input_tokens"`
		OutputTokens int64 `json:"output_tokens"`
	} `json:"usage,omitempty"`
}

var subagentCounter atomic.Uint64

func generateSubagentID() string {
	n := subagentCounter.Add(1)
	return fmt.Sprintf("sub-%d-%d", time.Now().UnixNano(), n)
}

func findKitBinary() string {
	// Try the current process executable first.
	if exe, err := os.Executable(); err == nil {
		if _, err := os.Stat(exe); err == nil {
			return exe
		}
	}
	// Fall back to PATH lookup.
	if p, err := exec.LookPath("kit"); err == nil {
		return p
	}
	return "kit"
}

// ---------------------------------------------------------------------------
// SpawnSubagent implementation
// ---------------------------------------------------------------------------

// SpawnSubagent spawns a child Kit instance to perform a task.
//
// When config.Blocking is true, blocks until completion and returns the result
// directly (handle is nil). When false, returns immediately with a handle for
// monitoring/cancellation.
//
// The subagent runs with --json --no-session --no-extensions flags by default,
// ensuring isolation from the parent's extensions and session state.
func SpawnSubagent(cfg SubagentConfig) (*SubagentHandle, *SubagentResult, error) {
	if cfg.Prompt == "" {
		return nil, nil, fmt.Errorf("prompt is required")
	}

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 5 * time.Minute
	}

	kitBinary := findKitBinary()

	// Build subprocess arguments.
	args := []string{
		"--json",
		"--no-extensions",
	}
	if cfg.NoSession {
		args = append(args, "--no-session")
	}
	if cfg.Model != "" {
		args = append(args, "--model", cfg.Model)
	}

	// Handle system prompt - write to temp file if provided.
	var tmpFile *os.File
	if cfg.SystemPrompt != "" {
		var err error
		tmpFile, err = os.CreateTemp("", "kit-subagent-*.txt")
		if err != nil {
			return nil, nil, fmt.Errorf("create temp file: %w", err)
		}
		if _, err := tmpFile.WriteString(cfg.SystemPrompt); err != nil {
			_ = tmpFile.Close()
			_ = os.Remove(tmpFile.Name())
			return nil, nil, fmt.Errorf("write system prompt: %w", err)
		}
		_ = tmpFile.Close()
		args = append(args, "--system-prompt", tmpFile.Name())
	}

	// Add the prompt as a positional argument.
	args = append(args, cfg.Prompt)

	// Create command with timeout context.
	ctx, cancel := context.WithTimeout(context.Background(), timeout)

	cmd := exec.CommandContext(ctx, kitBinary, args...)
	cmd.Env = os.Environ()

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		if tmpFile != nil {
			_ = os.Remove(tmpFile.Name())
		}
		return nil, nil, fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		if tmpFile != nil {
			_ = os.Remove(tmpFile.Name())
		}
		return nil, nil, fmt.Errorf("stderr pipe: %w", err)
	}

	handle := &SubagentHandle{
		ID:   generateSubagentID(),
		done: make(chan struct{}),
	}

	// Start the subprocess.
	start := time.Now()
	if err := cmd.Start(); err != nil {
		cancel()
		if tmpFile != nil {
			_ = os.Remove(tmpFile.Name())
		}
		return nil, nil, fmt.Errorf("start subprocess: %w", err)
	}

	handle.mu.Lock()
	handle.proc = cmd.Process
	handle.mu.Unlock()

	// Run the subprocess monitoring in a goroutine.
	go func() {
		defer close(handle.done)
		defer cancel()
		if tmpFile != nil {
			defer func() { _ = os.Remove(tmpFile.Name()) }()
		}

		var wg sync.WaitGroup
		var stdoutBuf strings.Builder

		// Read stderr (live output).
		wg.Go(func() {
			scanner := bufio.NewScanner(stderr)
			scanner.Buffer(make([]byte, 256*1024), 256*1024)
			for scanner.Scan() {
				line := scanner.Text()
				if cfg.OnOutput != nil && strings.TrimSpace(line) != "" {
					cfg.OnOutput(line + "\n")
				}
			}
		})

		// Read stdout (JSON output).
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 256*1024), 256*1024)
		for scanner.Scan() {
			stdoutBuf.WriteString(scanner.Text() + "\n")
		}

		wg.Wait()
		waitErr := cmd.Wait()
		elapsed := time.Since(start)

		// Build result.
		result := SubagentResult{Elapsed: elapsed}
		if waitErr != nil {
			result.Error = waitErr
			if exitErr, ok := waitErr.(*exec.ExitError); ok {
				result.ExitCode = exitErr.ExitCode()
			} else {
				result.ExitCode = 1
			}
		}

		// Parse JSON output.
		raw := strings.TrimSpace(stdoutBuf.String())
		var parsed subagentJSONOutput
		if raw != "" && json.Unmarshal([]byte(raw), &parsed) == nil {
			result.Response = parsed.Response
			result.SessionID = parsed.SessionID
			if parsed.Usage != nil {
				result.Usage = &SubagentUsage{
					InputTokens:  parsed.Usage.InputTokens,
					OutputTokens: parsed.Usage.OutputTokens,
				}
			}
		} else {
			// Fallback: use raw stdout.
			result.Response = raw
		}

		handle.mu.Lock()
		handle.result = &result
		handle.proc = nil
		handle.mu.Unlock()

		if cfg.OnComplete != nil {
			cfg.OnComplete(result)
		}
	}()

	if cfg.Blocking {
		// Wait for completion and return result directly.
		<-handle.done
		handle.mu.Lock()
		r := handle.result
		handle.mu.Unlock()
		return nil, r, nil
	}

	return handle, nil, nil
}
