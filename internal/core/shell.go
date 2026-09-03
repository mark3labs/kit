package core

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
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

// sudoPasswordFromContext retrieves the sudo password from context.
func sudoPasswordFromContext(ctx context.Context) string {
	if pw, ok := ctx.Value(sudoPasswordKey).(string); ok {
		return pw
	}
	return ""
}

// ---------------------------------------------------------------------------
// Shell resolution and identity
// ---------------------------------------------------------------------------

// defaultShell is the shell used when none is configured.
var defaultShell = []string{"bash"}

// errEmptyShellElement reports a shell configuration that contains an empty
// argument, which is a configuration mistake rather than a runnable shell.
var errEmptyShellElement = errors.New("shell contains an empty argument")

// posixShells are the shells whose command language is POSIX sh, so the model
// can be told to write POSIX syntax. zsh, fish, nu and the rest are absent
// deliberately: instructing POSIX syntax for them would be wrong.
var posixShells = []string{
	"sh", "dash", "ash", "busybox", "ksh", "ksh93", "mksh", "loksh", "oksh", "yash",
}

// shellResolution is the configured shell in the two forms the tool needs: the
// argument vector prefix to execute, and the path to advertise in SHELL.
type shellResolution struct {
	// argv is the shell plus its own leading arguments, e.g. ["bash"] or
	// ["busybox", "ash"].
	argv []string
	// shellPath is written to SHELL for child processes, so that programs
	// such as tmux use the configured shell rather than the login shell of
	// the user. It is argv[0] resolved through PATH, or argv[0] unchanged
	// when the lookup fails. It is empty for a multi-element vector: KIT
	// cannot know which element of a launcher vector is the shell, and a
	// wrong SHELL is worse for child programs than the inherited one, so
	// SHELL is then left alone.
	shellPath string
}

// normalizeShell applies the default: an empty or nil shell means bash.
func normalizeShell(shell []string) []string {
	if len(shell) == 0 {
		return defaultShell
	}
	return shell
}

// resolveShell turns a configured shell into a shellResolution.
//
// It is pure with respect to process state: it reads only its argument and the
// PATH lookup, and must not read environment variables, configuration files or
// package-level state other than the constant default. Each kit.New owns its
// own configuration store, and a package-level read here would let one host
// observe another's setting.
//
// A PATH lookup failure is not an error. The unresolved name is returned and
// the failure surfaces through the ordinary execution error path.
func resolveShell(shell []string) (shellResolution, error) {
	for _, a := range shell {
		if a == "" {
			return shellResolution{}, errEmptyShellElement
		}
	}
	argv := normalizeShell(shell)

	// SHELL is advertised only when the vector is the shell alone; see the
	// shellPath field comment.
	shellPath := ""
	if len(argv) == 1 {
		shellPath = argv[0]
		if resolved, err := exec.LookPath(argv[0]); err == nil {
			shellPath = resolved
		}
	}
	return shellResolution{argv: argv, shellPath: shellPath}, nil
}

// commandArgs returns the full process argument vector for one command
// string. The command is passed as exactly one argument, never split or
// re-quoted.
func (r shellResolution) commandArgs(command string) []string {
	args := make([]string, 0, len(r.argv)+2)
	args = append(args, r.argv...)
	args = append(args, "-c")
	return append(args, command)
}

// ShellCommandArgs returns the process argument vector that runs one command
// string through the configured shell, and the SHELL value to advertise to
// child processes, empty when SHELL should stay inherited. It is the same
// construction the shell tool uses, exported so that the TUI's ! and !!
// commands execute through the same shell.
func ShellCommandArgs(shell []string, command string) (args []string, shellPath string, err error) {
	r, err := resolveShell(shell)
	if err != nil {
		return nil, "", err
	}
	return r.commandArgs(command), r.shellPath, nil
}

// shellDisplayName is the shell as the tool descriptions name it: the
// configured vector joined with spaces.
func shellDisplayName(argv []string) string {
	return strings.Join(argv, " ")
}

// isBashShell reports whether the configured shell is bash, which is the one
// case that needs no dialect note.
func isBashShell(argv []string) bool {
	return filepath.Base(argv[0]) == "bash"
}

// isPosixShell reports whether the configured shell speaks POSIX sh.
func isPosixShell(argv []string) bool {
	return slices.Contains(posixShells, filepath.Base(argv[0]))
}

// shellToolDescription is the tool description for the configured shell.
//
// The model writes the command, so it has to know which shell it is writing
// for. Described as bash while a POSIX shell runs, it emits bash-only syntax
// and cannot diagnose the errors, because it believes it is talking to bash.
// Told to write POSIX syntax for fish or nu it would be wrong the other way,
// which is why the note has three cases rather than two.
func shellToolDescription(shell []string) string {
	const tail = "Returns stdout and stderr. Output is truncated to the last 2000 lines or 50KB. Optionally provide a timeout in seconds."

	argv := normalizeShell(shell)
	name := shellDisplayName(argv)
	desc := fmt.Sprintf("Execute a command through %s. %s", name, tail)

	switch {
	case isBashShell(argv):
		return desc
	case isPosixShell(argv):
		return desc + fmt.Sprintf(
			" IMPORTANT: the shell is %s, not bash. Write POSIX sh syntax only. "+
				"Bash-only constructs are unavailable and will fail: [[ ]] tests (use [ ]), "+
				"arrays, process substitution <(...), brace expansion {a,b}, 'source' (use '.'), "+
				"the 'function' keyword, 'local -a', ${var^^} case conversion, and $'...' quoting.", name)
	default:
		return desc + fmt.Sprintf(
			" IMPORTANT: the shell is %s, not bash. Write syntax that %s accepts; "+
				"bash-only constructs are unavailable and will fail.", name, name)
	}
}

// shellCommandParamDescription describes the command parameter, which also
// names the shell.
func shellCommandParamDescription(shell []string) string {
	argv := normalizeShell(shell)
	if isPosixShell(argv) && !isBashShell(argv) {
		return fmt.Sprintf("Command to execute with %s (POSIX sh syntax)", shellDisplayName(argv))
	}
	return fmt.Sprintf("Command to execute with %s", shellDisplayName(argv))
}

const defaultShellTimeout = 120 * time.Second
const maxShellTimeout = 600 * time.Second

// bannedCmdRe matches shell builtin commands that are not allowed for security
// reasons.
var bannedCmdRe = regexp.MustCompile(`^(alias|bg|bind|builtin|caller|command|compgen|complete|compopt|coproc|dirs|disown|enable|fc|fg|hash|help|history|jobs|kill|logout|mapfile|popd|pushd|readonly|select|set|shopt|source|suspend|times|trap|type|typeset|ulimit|umask|unalias|wait)\s`)

type shellArgs struct {
	Command string  `json:"command"`
	Timeout float64 `json:"timeout,omitempty"`
}

// NewShellTool creates the shell core tool. It runs one command string through
// the configured shell, which defaults to bash.
func NewShellTool(opts ...ToolOption) fantasy.AgentTool {
	cfg := ApplyOptions(opts)

	// Resolve effective timeouts, falling back to the built-in defaults.
	maxTimeout := maxShellTimeout
	if cfg.ShellMaxTimeout > 0 {
		maxTimeout = cfg.ShellMaxTimeout
	}
	defTimeout := defaultShellTimeout
	if cfg.ShellTimeout > 0 {
		defTimeout = cfg.ShellTimeout
	}
	// The default must never exceed the ceiling.
	if defTimeout > maxTimeout {
		defTimeout = maxTimeout
	}

	return &coreTool{
		info: fantasy.ToolInfo{
			// The name does not vary with the shell: a transcript, a
			// permission rule and a configuration entry all say the same
			// thing. The descriptions do vary, because that is where the
			// model learns which shell it is writing for.
			Name:        ShellToolName,
			Description: shellToolDescription(cfg.Shell),
			Parameters: map[string]any{
				"command": map[string]any{
					"type":        "string",
					"description": shellCommandParamDescription(cfg.Shell),
				},
				"timeout": map[string]any{
					"type":        "number",
					"description": fmt.Sprintf("Timeout in seconds (optional, default %ds, max %ds)", int(defTimeout.Seconds()), int(maxTimeout.Seconds())),
				},
			},
			Required: []string{"command"},
		},
		handler: func(ctx context.Context, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return executeShell(ctx, call, cfg.WorkDir, cfg.Shell, defTimeout, maxTimeout)
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
	for _, match := range slices.Backward(matches) {
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

func executeShell(ctx context.Context, call fantasy.ToolCall, workDir string, shell []string, defaultTimeout, maxTimeout time.Duration) (fantasy.ToolResponse, error) {
	var args shellArgs
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
	timeout := defaultTimeout
	if args.Timeout > 0 {
		timeout = time.Duration(args.Timeout) * time.Second
		timeout = min(timeout, maxTimeout)
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

	// Resolve the configured shell. With nothing configured, the argument
	// vector and the SHELL value are the ones this tool has always used.
	resolution, err := resolveShell(shell)
	if err != nil {
		return fantasy.NewTextErrorResponse(fmt.Sprintf("invalid shell configuration: %v", err)), nil
	}
	cmdArgs := resolution.commandArgs(command)

	cmd := exec.CommandContext(cmdCtx, cmdArgs[0], cmdArgs[1:]...)
	if workDir != "" {
		cmd.Dir = workDir
	}

	// Ensure SHELL is set to the resolved shell so child processes (e.g. tmux)
	// use it rather than the user's login shell (which may be nushell, fish,
	// etc.). A launcher vector leaves SHELL inherited; see resolveShell.
	if resolution.shellPath != "" {
		cmd.Env = append(os.Environ(), "SHELL="+resolution.shellPath)
	}

	// Get the output callback if present (for streaming support)
	outputCallback := toolOutputCallbackFromContext(ctx)

	if outputCallback != nil {
		// Streaming mode: use pipes to capture output as it arrives
		return executeShellStreaming(cmdCtx, call, cmd, outputCallback, sudoPassword)
	}

	// Non-streaming mode: collect all output at once (original behavior)
	return executeShellBuffered(cmdCtx, call, cmd, sudoPassword)
}

// pipeDrainGrace bounds how long the parent waits for the output readers to
// finish after the child process has exited. It only matters when a grandchild
// process inherited the write end and is holding it open, in which case the
// readers never observe EOF. Normal commands drain immediately and never touch
// this timer.
const pipeDrainGrace = 500 * time.Millisecond

// shellPipes holds the parent's read ends of the child's output pipes. They are
// created with os.Pipe and assigned to cmd.Stdout/cmd.Stderr rather than
// obtained from cmd.StdoutPipe, which matters for correctness:
//
// Cmd.Wait closes the pipes it creates as soon as it observes the child exit,
// without waiting for the reader to drain them. Any output still sitting in
// the kernel buffer is then lost. Owning the read ends here keeps them open
// until the readers reach EOF, so Wait can never truncate output.
//
// The cost is that Cmd.WaitDelay no longer force-closes these descriptors, so
// the caller must bound the drain itself — see waitForDrain.
type shellPipes struct {
	stdout *os.File
	stderr *os.File

	// forced records that close was called deliberately by waitForDrain
	// rather than the pipes reaching EOF. Readers use it to tell a real
	// truncation from the expected shutdown of a lingering grandchild.
	forced atomic.Bool
}

// close releases the parent's read ends, unblocking any reader still waiting
// on a pipe that a surviving grandchild holds open. Safe to call more than
// once.
func (p *shellPipes) close() {
	if p == nil {
		return
	}
	if p.stdout != nil {
		_ = p.stdout.Close()
	}
	if p.stderr != nil {
		_ = p.stderr.Close()
	}
}

// waitForDrain waits for the output readers to finish after the child has
// exited. It returns once readersDone is closed, or force-closes the pipes
// after pipeDrainGrace so a grandchild holding the write end cannot hang the
// call. Either way it does not return until the readers have stopped, so the
// caller can read the collected output without a race.
func (p *shellPipes) waitForDrain(readersDone <-chan struct{}) {
	select {
	case <-readersDone:
		// Normal path: both readers reached EOF and drained everything.
	case <-time.After(pipeDrainGrace):
		// A grandchild still holds a write end. Closing the read ends makes
		// the pending reads fail, which ends the reader loops. Flag it first
		// so those failures are not misreported as truncated output.
		p.forced.Store(true)
		p.close()
		<-readersDone
	}
}

// setupShellPipes opens stdout/stderr pipes (plus an optional sudo stdin),
// starts the command, and asynchronously writes the sudo password if any.
// Returns the readers ready for the caller to consume. If setup fails,
// errResp is non-nil and the readers must not be used; the caller should
// return the response directly.
//
// The caller owns the returned pipes and must drain them via
// [shellPipes.waitForDrain] after cmd.Wait returns.
func setupShellPipes(cmd *exec.Cmd, sudoPassword string) (pipes *shellPipes, errResp *fantasy.ToolResponse) {
	fail := func(msg string) (*shellPipes, *fantasy.ToolResponse) {
		r := fantasy.NewTextErrorResponse(msg)
		return nil, &r
	}

	// os.Pipe rather than cmd.StdoutPipe: see the shellPipes doc comment. The
	// write ends are handed to the child and closed in the parent right after
	// Start, so the readers see EOF once every writer has exited.
	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		return fail("failed to create stdout pipe")
	}
	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		_ = stdoutR.Close()
		_ = stdoutW.Close()
		return fail("failed to create stderr pipe")
	}
	cmd.Stdout = stdoutW
	cmd.Stderr = stderrW

	closeAll := func() {
		_ = stdoutR.Close()
		_ = stdoutW.Close()
		_ = stderrR.Close()
		_ = stderrW.Close()
	}

	var stdinPipe io.WriteCloser
	if sudoPassword != "" {
		stdinPipe, err = cmd.StdinPipe()
		if err != nil {
			closeAll()
			return fail("failed to create stdin pipe")
		}
	}

	if err := cmd.Start(); err != nil {
		closeAll()
		return fail(fmt.Sprintf("failed to start command: %v", err))
	}

	// Drop the parent's copies of the write ends. Without this the readers
	// would never see EOF, because this process would still count as a writer.
	_ = stdoutW.Close()
	_ = stderrW.Close()

	if sudoPassword != "" && stdinPipe != nil {
		go func() {
			defer func() { _ = stdinPipe.Close() }()
			_, _ = io.WriteString(stdinPipe, sudoPassword+"\n")
		}()
	}

	return &shellPipes{stdout: stdoutR, stderr: stderrR}, nil
}

// interpretShellExit decodes cmd.Wait()'s error into an exit code, mapping
// context-deadline-exceeded to a friendly "command timed out" response.
// errResp is non-nil only when the caller should short-circuit and return
// it directly (e.g. timeout).
func interpretShellExit(waitErr error, cmdCtx context.Context) (exitCode int, errResp *fantasy.ToolResponse) {
	if waitErr == nil {
		return 0, nil
	}
	if exitErr, ok := waitErr.(*exec.ExitError); ok {
		return exitErr.ExitCode(), nil
	}
	if cmdCtx.Err() == context.DeadlineExceeded {
		r := fantasy.NewTextErrorResponse("command timed out")
		return 0, &r
	}
	return 0, nil
}

// executeShellBuffered collects all output before returning (original behavior).
// It uses explicit pipes (not cmd.Stdout) so that cmd.WaitDelay can forcibly
// close them when grandchild processes hold pipe handles open after the
// direct child exits.
func executeShellBuffered(cmdCtx context.Context, _ fantasy.ToolCall, cmd *exec.Cmd, sudoPassword string) (fantasy.ToolResponse, error) {
	pipes, errResp := setupShellPipes(cmd, sudoPassword)
	if errResp != nil {
		return *errResp, nil
	}
	defer pipes.close()

	// Read pipes concurrently
	var wg sync.WaitGroup
	var stdout, stderr strings.Builder

	wg.Add(2)
	go func() {
		defer wg.Done()
		_, _ = io.Copy(&stdout, pipes.stdout)
	}()
	go func() {
		defer wg.Done()
		_, _ = io.Copy(&stderr, pipes.stderr)
	}()

	readersDone := make(chan struct{})
	go func() {
		wg.Wait()
		close(readersDone)
	}()

	// Wait for the process to exit. cmd.Cancel plus cmd.WaitDelay ensure a
	// runaway process tree is killed rather than blocking here forever.
	// Because the output pipes are owned by us rather than by Cmd, this call
	// cannot close them out from under the readers above.
	waitErr := cmd.Wait()

	// Let the readers finish draining, bounded so a grandchild holding a write
	// end open cannot hang the call.
	pipes.waitForDrain(readersDone)

	exitCode, errResp := interpretShellExit(waitErr, cmdCtx)
	if errResp != nil {
		return *errResp, nil
	}

	return buildShellResponse(stdout.String(), stderr.String(), exitCode)
}

// executeShellStreaming streams output as it arrives via the callback.
func executeShellStreaming(cmdCtx context.Context, call fantasy.ToolCall, cmd *exec.Cmd, outputCallback ToolOutputCallback, sudoPassword string) (fantasy.ToolResponse, error) {
	pipes, errResp := setupShellPipes(cmd, sudoPassword)
	if errResp != nil {
		return *errResp, nil
	}
	defer pipes.close()

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
			outputCallback(call.ID, ShellToolName, chunk, isStderr)
			// Collect for final result
			mu.Lock()
			if isStderr {
				stderrChunks = append(stderrChunks, chunk)
			} else {
				stdoutChunks = append(stdoutChunks, chunk)
			}
			mu.Unlock()
		}

		// A scan error ends the loop early and leaves data in the pipe. The
		// common case is bufio.ErrTooLong: a single line longer than the 1 MB
		// limit set above, which any minified JSON or packed asset produces.
		//
		// Draining is not optional. If the remainder is left unread the child
		// blocks writing to a full pipe and cmd.Wait() below never returns, so
		// the whole call hangs until the command timeout fires and every byte
		// of output is lost. Discard the rest so the process can exit, and
		// report the truncation instead of failing silently.
		//
		// A read failure caused by our own deliberate close is not truncation:
		// the foreground command already finished and only a backgrounded
		// grandchild still holds the pipe. Reporting it would put a spurious
		// notice on the output of every `cmd &` invocation.
		if err := scanner.Err(); err != nil && !pipes.forced.Load() {
			discarded, _ := io.Copy(io.Discard, reader)
			notice := fmt.Sprintf("[output truncated: %v; discarded %d further bytes]", err, discarded)
			outputCallback(call.ID, ShellToolName, notice, isStderr)
			mu.Lock()
			if isStderr {
				stderrChunks = append(stderrChunks, notice)
			} else {
				stdoutChunks = append(stdoutChunks, notice)
			}
			mu.Unlock()
		}
	}

	wg.Add(2)
	go streamOutput(pipes.stdout, false)
	go streamOutput(pipes.stderr, true)

	readersDone := make(chan struct{})
	go func() {
		wg.Wait()
		close(readersDone)
	}()

	// Wait for the process to exit. cmd.Cancel plus cmd.WaitDelay ensure a
	// runaway process tree is killed rather than blocking here forever.
	// Because the output pipes are owned by us rather than by Cmd, this call
	// cannot close them out from under the scanners above — which previously
	// truncated output whenever the child exited while a scanner was still
	// draining the kernel buffer.
	waitErr := cmd.Wait()

	// Let the scanners finish draining, bounded so a grandchild holding a write
	// end open cannot hang the call.
	pipes.waitForDrain(readersDone)

	exitCode, errResp := interpretShellExit(waitErr, cmdCtx)
	if errResp != nil {
		return *errResp, nil
	}

	return buildShellResponse(strings.Join(stdoutChunks, "\n"), strings.Join(stderrChunks, "\n"), exitCode)
}

// buildShellResponse constructs the final tool response from stdout/stderr.
func buildShellResponse(stdout, stderr string, exitCode int) (fantasy.ToolResponse, error) {
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

	// Truncate from tail (keep last N lines, most relevant for command output)
	tr := TruncateTail(output, defaultMaxLines, defaultMaxBytes)

	if exitCode != 0 {
		return fantasy.NewTextErrorResponse(tr.Content), nil
	}
	return fantasy.NewTextResponse(tr.Content), nil
}
