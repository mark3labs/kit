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

// PasswordPromptCallback is the signature for password prompts.
// It receives a prompt message and returns the password and whether it was cancelled.
type PasswordPromptCallback func(prompt string) (password string, cancelled bool)

// contextKey is a custom type for context keys to avoid collisions.
type contextKey string

const (
	toolOutputCallbackKey contextKey = "toolOutputCallback"
	sudoPasswordKey       contextKey = "sudoPassword"
	passwordPromptKey     contextKey = "passwordPrompt"
)

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

// ContextWithPasswordPrompt returns a new context with the password prompt callback set.
// This allows the TUI to show a modal password prompt when sudo needs a password.
func ContextWithPasswordPrompt(ctx context.Context, callback PasswordPromptCallback) context.Context {
	return context.WithValue(ctx, passwordPromptKey, callback)
}

// passwordPromptFromContext retrieves the password prompt callback from context.
func passwordPromptFromContext(ctx context.Context) PasswordPromptCallback {
	if cb, ok := ctx.Value(passwordPromptKey).(PasswordPromptCallback); ok {
		return cb
	}
	return nil
}

// ContextWithSudoPassword returns a new context with the sudo password set.
// When present, the bash tool will use sudo -S to pipe this password to sudo commands.
func ContextWithSudoPassword(ctx context.Context, password string) context.Context {
	return context.WithValue(ctx, sudoPasswordKey, password)
}

// sudoPasswordFromContext retrieves the sudo password from context.
func sudoPasswordFromContext(ctx context.Context) string {
	if pw, ok := ctx.Value(sudoPasswordKey).(string); ok {
		return pw
	}
	return ""
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

// sudoCommandRe matches sudo commands that need to be rewritten for -S mode.
// It matches "sudo" as a word boundary, optionally preceded by environment variables.
var sudoCommandRe = regexp.MustCompile(`(?i)(^|[&|;|]|\|\||&&)\s*(\w+=\S+\s+)?\bsudo\b`)

// truncateCommand truncates a long command for display.
func truncateCommand(cmd string, maxLen int) string {
	if len(cmd) <= maxLen {
		return cmd
	}
	return cmd[:maxLen-3] + "..."
}

// rewriteSudoForStdin rewrites sudo commands to use -S -p ” for stdin password input.
// It transforms: sudo cmd → sudo -S -p ” cmd
func rewriteSudoForStdin(command string) string {
	// Find all matches and their positions
	matches := sudoCommandRe.FindAllStringIndex(command, -1)
	if matches == nil {
		return command
	}

	// Build result from end to start to preserve indices
	result := command
	for i := len(matches) - 1; i >= 0; i-- {
		match := matches[i]
		start, end := match[0], match[1]
		matchedText := result[start:end]

		// Extract just the "sudo" part (after any prefix)
		sudoIdx := strings.Index(strings.ToLower(matchedText), "sudo")
		if sudoIdx == -1 {
			continue
		}
		prefix := matchedText[:sudoIdx]
		sudoPart := matchedText[sudoIdx:]

		// Check if the text immediately after "sudo" in the result contains -S
		afterSudo := result[end:]
		if strings.HasPrefix(strings.TrimLeft(afterSudo, " \t"), "-S") {
			// Already has -S flag, skip
			continue
		}

		// Insert -S -p '' after "sudo"
		newSudo := strings.Replace(sudoPart, "sudo", "sudo -S -p ''", 1)
		result = result[:start] + prefix + newSudo + result[end:]
	}

	return result
}

// SudoPasswordRequiredResult is a special marker that indicates sudo needs a password.
// This is stored in tool response metadata to signal the TUI to prompt for password.
const SudoPasswordRequiredMetadata = `{"sudo_password_required":true}`

// IsSudoPasswordRequiredResult checks if a tool response indicates sudo password is needed.
func IsSudoPasswordRequiredResult(resp fantasy.ToolResponse) bool {
	return resp.Metadata == SudoPasswordRequiredMetadata
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

	// Check for sudo password in context or environment
	sudoPassword := sudoPasswordFromContext(ctx)
	if sudoPassword == "" {
		sudoPassword = os.Getenv("SUDO_PASSWORD")
	}
	command := args.Command

	// If command contains sudo and we don't have a password, check if sudo needs one
	if sudoPassword == "" && sudoCommandRe.MatchString(command) {
		// Check if sudo credentials are cached using sudo -n (non-interactive)
		testCmd := exec.CommandContext(cmdCtx, "sudo", "-n", "true")
		testCmd.Dir = workDir
		if err := testCmd.Run(); err != nil {
			// Sudo needs a password - try to prompt via callback
			if promptCallback := passwordPromptFromContext(ctx); promptCallback != nil {
				pw, cancelled := promptCallback("Sudo password required for: " + truncateCommand(args.Command, 60))
				if cancelled {
					return fantasy.NewTextErrorResponse("sudo password prompt cancelled"), nil
				}
				if pw == "" {
					return fantasy.NewTextErrorResponse("no sudo password provided"), nil
				}
				sudoPassword = pw
				command = rewriteSudoForStdin(command)
			} else {
				// No callback available - return error with helpful message
				return fantasy.NewTextErrorResponse(
					"This command requires sudo access. " +
						"Please run 'sudo -v' in your terminal first to cache credentials, " +
						"or set the SUDO_PASSWORD environment variable."), nil
			}
		}
		// Credentials are cached or password was provided, proceed
	}

	// If we have a sudo password, rewrite the command to use sudo -S
	if sudoPassword != "" && sudoCommandRe.MatchString(command) {
		command = rewriteSudoForStdin(command)
	}

	cmd := exec.CommandContext(cmdCtx, "bash", "-c", command)
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
		return executeBashStreaming(cmdCtx, call, cmd, outputCallback, sudoPassword)
	}

	// Non-streaming mode: collect all output at once (original behavior)
	return executeBashBuffered(cmdCtx, call, cmd, sudoPassword)
}

// executeBashBuffered collects all output before returning (original behavior).
// It uses explicit pipes (not cmd.Stdout) so that cmd.WaitDelay can forcibly
// close them when grandchild processes hold pipe handles open after the
// direct child exits.
func executeBashBuffered(cmdCtx context.Context, call fantasy.ToolCall, cmd *exec.Cmd, sudoPassword string) (fantasy.ToolResponse, error) {
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return fantasy.NewTextErrorResponse("failed to create stdout pipe"), nil
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return fantasy.NewTextErrorResponse("failed to create stderr pipe"), nil
	}

	// If we have a sudo password, create a stdin pipe and write the password
	var stdinPipe io.WriteCloser
	if sudoPassword != "" {
		stdinPipe, err = cmd.StdinPipe()
		if err != nil {
			return fantasy.NewTextErrorResponse("failed to create stdin pipe"), nil
		}
	}

	if err := cmd.Start(); err != nil {
		return fantasy.NewTextErrorResponse(fmt.Sprintf("failed to start command: %v", err)), nil
	}

	// Write password to stdin if needed, then close stdin
	if sudoPassword != "" && stdinPipe != nil {
		go func() {
			defer func() { _ = stdinPipe.Close() }()
			_, _ = io.WriteString(stdinPipe, sudoPassword+"\n")
		}()
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
func executeBashStreaming(cmdCtx context.Context, call fantasy.ToolCall, cmd *exec.Cmd, outputCallback ToolOutputCallback, sudoPassword string) (fantasy.ToolResponse, error) {
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return fantasy.NewTextErrorResponse("failed to create stdout pipe"), nil
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return fantasy.NewTextErrorResponse("failed to create stderr pipe"), nil
	}

	// If we have a sudo password, create a stdin pipe
	var stdinPipe io.WriteCloser
	if sudoPassword != "" {
		stdinPipe, err = cmd.StdinPipe()
		if err != nil {
			return fantasy.NewTextErrorResponse("failed to create stdin pipe"), nil
		}
	}

	// Start command execution
	if err := cmd.Start(); err != nil {
		return fantasy.NewTextErrorResponse(fmt.Sprintf("failed to start command: %v", err)), nil
	}

	// Write password to stdin if needed, then close stdin
	if sudoPassword != "" && stdinPipe != nil {
		go func() {
			defer func() { _ = stdinPipe.Close() }()
			_, _ = io.WriteString(stdinPipe, sudoPassword+"\n")
		}()
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
