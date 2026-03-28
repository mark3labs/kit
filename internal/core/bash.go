package core

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"

	"charm.land/fantasy"
)

// ToolOutputCallback is the signature for streaming tool output.
// It receives tool call ID, tool name, output chunk, and whether it's stderr.
type ToolOutputCallback func(toolCallID, toolName, chunk string, isStderr bool)

// contextKey is a custom type for context keys to avoid collisions.
type contextKey string

const toolOutputCallbackKey contextKey = "toolOutputCallback"

// ContextWithToolOutputCallback returns a new context with the tool output callback set.
func ContextWithToolOutputCallback(ctx context.Context, callback ToolOutputCallback) context.Context {
	return context.WithValue(ctx, toolOutputCallbackKey, callback)
}

// toolOutputCallbackFromContext retrieves the tool output callback from context.
func toolOutputCallbackFromContext(ctx context.Context) ToolOutputCallback {
	if cb, ok := ctx.Value(toolOutputCallbackKey).(ToolOutputCallback); ok {
		return cb
	}
	return nil
}

const defaultBashTimeout = 120 * time.Second
const maxBashTimeout = 600 * time.Second

// bannedCmdRe matches bash builtin commands that are not allowed for security reasons.
var bannedCmdRe = regexp.MustCompile(`^(alias|bg|bind|builtin|caller|command|compgen|complete|compopt|coproc|dirs|disown|enable|fc|fg|hash|help|history|jobs|kill|logout|mapfile|popd|pushd|readonly|select|set|shopt|source|suspend|times|trap|type|typeset|ulimit|umask|unalias|wait)\s`)

type bashArgs struct {
	Command string  `json:"command"`
	Timeout float64 `json:"timeout,omitempty"`
}

// NewBashTool creates the bash core tool.
func NewBashTool(opts ...ToolOption) fantasy.AgentTool {
	cfg := ApplyOptions(opts)
	return &coreTool{
		info: fantasy.ToolInfo{
			Name:        "bash",
			Description: "Execute a bash command. Returns stdout and stderr. Output is truncated to the last 2000 lines or 50KB. Optionally provide a timeout in seconds.",
			Parameters: map[string]any{
				"command": map[string]any{
					"type":        "string",
					"description": "Bash command to execute",
				},
				"timeout": map[string]any{
					"type":        "number",
					"description": "Timeout in seconds (optional, default 120s, max 600s)",
				},
			},
			Required: []string{"command"},
		},
		handler: func(ctx context.Context, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return executeBash(ctx, call, cfg.WorkDir)
		},
	}
}

func executeBash(ctx context.Context, call fantasy.ToolCall, workDir string) (fantasy.ToolResponse, error) {
	var args bashArgs
	if err := parseArgs(call.Input, &args); err != nil {
		return fantasy.NewTextErrorResponse("command parameter is required"), nil
	}
	if args.Command == "" {
		return fantasy.NewTextErrorResponse("command parameter is required"), nil
	}

	// Check for banned commands
	if bannedCmdRe.MatchString(args.Command) {
		return fantasy.NewTextErrorResponse(fmt.Sprintf("command '%s' is not allowed", args.Command)), nil
	}

	// Determine timeout
	timeout := defaultBashTimeout
	if args.Timeout > 0 {
		timeout = time.Duration(args.Timeout) * time.Second
		timeout = min(timeout, maxBashTimeout)
	}

	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, "bash", "-c", args.Command)
	if workDir != "" {
		cmd.Dir = workDir
	}

	// Ensure SHELL is set to bash so child processes (e.g. tmux) use bash
	// rather than the user's login shell (which may be nushell, fish, etc.).
	bashPath, err := exec.LookPath("bash")
	if err != nil {
		bashPath = "/bin/bash"
	}
	cmd.Env = append(os.Environ(), "SHELL="+bashPath)

	// Get the output callback if present (for streaming support)
	outputCallback := toolOutputCallbackFromContext(ctx)

	if outputCallback != nil {
		// Streaming mode: use pipes to capture output as it arrives
		return executeBashStreaming(cmdCtx, call, cmd, outputCallback)
	}

	// Non-streaming mode: collect all output at once (original behavior)
	return executeBashBuffered(cmdCtx, call, cmd)
}

// executeBashBuffered collects all output before returning (original behavior).
// It uses explicit pipes (not cmd.Stdout) so that cmd.WaitDelay can forcibly
// close them when grandchild processes hold pipe handles open after the
// direct child exits.
func executeBashBuffered(cmdCtx context.Context, call fantasy.ToolCall, cmd *exec.Cmd) (fantasy.ToolResponse, error) {
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return fantasy.NewTextErrorResponse("failed to create stdout pipe"), nil
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return fantasy.NewTextErrorResponse("failed to create stderr pipe"), nil
	}

	if err := cmd.Start(); err != nil {
		return fantasy.NewTextErrorResponse(fmt.Sprintf("failed to start command: %v", err)), nil
	}

	// Read pipes concurrently
	var wg sync.WaitGroup
	var stdout, stderr strings.Builder
	var stdoutErr, stderrErr error

	wg.Add(2)
	go func() {
		defer wg.Done()
		_, stdoutErr = io.Copy(&stdout, stdoutPipe)
	}()
	go func() {
		defer wg.Done()
		_, stderrErr = io.Copy(&stderr, stderrPipe)
	}()

	// Wait for the process to exit first. cmd.WaitDelay ensures that if
	// pipes remain open (held by grandchild processes), they'll be forcibly
	// closed after the grace period, which unblocks the io.Copy goroutines.
	waitErr := cmd.Wait()

	// Wait for pipe readers to finish draining.
	wg.Wait()

	// Ignore pipe read errors caused by WaitDelay force-closing —
	// we still have whatever was read before the close.
	_ = stdoutErr
	_ = stderrErr

	exitCode := 0
	if waitErr != nil {
		if exitErr, ok := waitErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else if cmdCtx.Err() == context.DeadlineExceeded {
			return fantasy.NewTextErrorResponse("command timed out"), nil
		}
	}

	return buildBashResponse(stdout.String(), stderr.String(), exitCode)
}

// executeBashStreaming streams output as it arrives via the callback.
func executeBashStreaming(cmdCtx context.Context, call fantasy.ToolCall, cmd *exec.Cmd, outputCallback ToolOutputCallback) (fantasy.ToolResponse, error) {
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return fantasy.NewTextErrorResponse("failed to create stdout pipe"), nil
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return fantasy.NewTextErrorResponse("failed to create stderr pipe"), nil
	}

	// Start command execution
	if err := cmd.Start(); err != nil {
		return fantasy.NewTextErrorResponse(fmt.Sprintf("failed to start command: %v", err)), nil
	}

	// Stream stdout and stderr concurrently
	var wg sync.WaitGroup
	var mu sync.Mutex
	var stdoutChunks, stderrChunks []string

	streamOutput := func(reader io.Reader, isStderr bool) {
		defer wg.Done()
		scanner := bufio.NewScanner(reader)
		// Use larger buffer for long lines
		buf := make([]byte, 0, 64*1024)
		scanner.Buffer(buf, 1024*1024)

		for scanner.Scan() {
			chunk := scanner.Text()
			// Send chunk to UI
			outputCallback(call.ID, "bash", chunk, isStderr)
			// Collect for final result
			mu.Lock()
			if isStderr {
				stderrChunks = append(stderrChunks, chunk)
			} else {
				stdoutChunks = append(stdoutChunks, chunk)
			}
			mu.Unlock()
		}
	}

	wg.Add(2)
	go streamOutput(stdoutPipe, false)
	go streamOutput(stderrPipe, true)

	// Wait for the process to exit. cmd.WaitDelay ensures that if pipes
	// remain open (held by grandchild processes), they'll be forcibly closed
	// after the grace period, which unblocks the scanners above.
	err = cmd.Wait()

	// Wait for the pipe readers to finish draining. This will complete
	// quickly since cmd.Wait() (with WaitDelay) has already ensured
	// the pipes are closed.
	wg.Wait()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else if cmdCtx.Err() == context.DeadlineExceeded {
			return fantasy.NewTextErrorResponse("command timed out"), nil
		}
	}

	return buildBashResponse(strings.Join(stdoutChunks, "\n"), strings.Join(stderrChunks, "\n"), exitCode)
}

// buildBashResponse constructs the final tool response from stdout/stderr.
func buildBashResponse(stdout, stderr string, exitCode int) (fantasy.ToolResponse, error) {
	var result strings.Builder
	if stdout != "" {
		result.WriteString(stdout)
	}
	if stderr != "" {
		if result.Len() > 0 {
			result.WriteString("\n")
		}
		result.WriteString("STDERR:\n")
		result.WriteString(stderr)
	}
	if exitCode != 0 {
		if result.Len() > 0 {
			result.WriteString("\n")
		}
		fmt.Fprintf(&result, "Exit code: %d", exitCode)
	}

	output := result.String()
	if output == "" {
		output = "(no output)"
	}

	// Truncate from tail (keep last N lines, most relevant for bash)
	tr := TruncateTail(output, defaultMaxLines, defaultMaxBytes)

	if exitCode != 0 {
		return fantasy.NewTextErrorResponse(tr.Content), nil
	}
	return fantasy.NewTextResponse(tr.Content), nil
}
