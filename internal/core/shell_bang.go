package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// This file runs a bang-mode shell command — the `!cmd` and `!!cmd` prefixes in
// the composer — and streams its output while it runs.
//
// It is separate from the shell tool in shell.go on purpose. The tool has to
// return one response for the model, so it accumulates per stream, attributes
// each chunk to a tool call, and applies its own truncation. A bang command has
// no model waiting on it: it draws into the transcript as bytes arrive and
// keeps everything the user might scroll back to. Merging the two would mean one
// set of rules serving two opposite requirements.

// ErrShellCommandTimedOut reports a command that exceeded its timeout. It is a
// distinct error because the bang path shows it as a status line rather than as
// output: the command produced nothing useful, and saying so is more useful than
// showing the empty buffer.
var ErrShellCommandTimedOut = errors.New("command timed out")

// BangShellTimeout is the ceiling for a bang command when the caller sets no
// timeout of its own. It matches the shell tool's default so that `!go build`
// and the same command from the model behave the same way.
const BangShellTimeout = 120 * time.Second

// ShellChunkSink receives output as it arrives. isStderr says which stream the
// bytes came from.
//
// The call happens on a reader goroutine, one of two running concurrently, so an
// implementation must be safe for concurrent use. It must also not block: a sink
// that waits for the UI while the UI waits for the command deadlocks both. The
// caller here drops a chunk when its channel is full, which is the correct
// trade — the full text is still collected for the final result.
type ShellChunkSink func(chunk string, isStderr bool)

// ShellRunOptions configures one bang-mode shell command.
type ShellRunOptions struct {
	// Command is the command string to run through the configured shell.
	Command string

	// Shell is the configured shell vector. Empty means the default shell.
	Shell []string

	// WorkDir is the working directory. Empty means the process working
	// directory.
	WorkDir string

	// Timeout bounds the whole run. Zero means BangShellTimeout. The command
	// and its process group are killed when it expires.
	Timeout time.Duration

	// OnChunk receives output as it arrives. Nil collects the output without
	// streaming it, which is what a caller with nothing live to draw wants.
	OnChunk ShellChunkSink
}

// ShellRunResult is the outcome of one bang-mode shell command.
type ShellRunResult struct {
	// Command is the command that was run.
	Command string

	// Output is stdout followed by stderr, in that order, with a newline
	// between them when both are present. It holds the escape sequences the
	// program emitted, unmodified: the display layer and the model layer each
	// need a different treatment and neither can be undone from the other.
	Output string

	// ExitCode is the process exit status, or -1 when the command never
	// produced one.
	ExitCode int

	// TimedOut reports that the timeout fired. Output collected before the
	// kill is still present.
	TimedOut bool
}

// RunShellCommand runs one command through the configured shell, streams its
// output to opts.OnChunk as it arrives, and returns the complete output.
//
// It applies the same shell resolution, environment and exit handling as the
// shell tool, so a bang command and the model's own call to the same command
// differ only in who is watching.
func RunShellCommand(ctx context.Context, opts ShellRunOptions) (ShellRunResult, error) {
	result := ShellRunResult{Command: opts.Command, ExitCode: -1}

	resolution, err := resolveShell(opts.Shell)
	if err != nil {
		return result, fmt.Errorf("invalid shell configuration: %w", err)
	}

	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = BangShellTimeout
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Cancel kills the process; WaitDelay then bounds how long the kill can be
	// ignored. Without the ceiling a process that traps SIGTERM holds the whole
	// call open until the parent is killed.
	cmd := exec.CommandContext(runCtx, resolution.argv[0], resolution.argv[1:]...)
	cmd.Args = append(cmd.Args, "-c", opts.Command) //nolint:gosec // running the user's own command is the point
	cmd.WaitDelay = 2 * time.Second
	if opts.WorkDir != "" {
		cmd.Dir = opts.WorkDir
	}
	cmd.Env = shellEnv(os.Environ(), resolution.shellPath)

	pipes, err := openShellPipes(cmd, "")
	if err != nil {
		return result, err
	}
	defer pipes.close()

	var mu sync.Mutex
	var stdout, stderr strings.Builder

	sink := opts.OnChunk
	// pump reads one stream in fixed-size pieces and forwards each piece
	// verbatim. The chunk boundary is deliberately not a line boundary: a
	// progress bar repaints with carriage returns and writes no newline until
	// it is done, so a line-based reader would show nothing at all until the
	// command exited. A split can land inside an escape sequence, which the
	// display layer repairs on each frame; see style.CompleteANSIOnly.
	pump := func(r *os.File, isStderr bool, dst *strings.Builder) {
		buf := make([]byte, 32*1024)
		for {
			n, err := r.Read(buf)
			if n > 0 {
				chunk := string(buf[:n])
				mu.Lock()
				dst.WriteString(chunk)
				mu.Unlock()
				if sink != nil {
					sink(chunk, isStderr)
				}
			}
			if err != nil {
				return
			}
		}
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); pump(pipes.stdout, false, &stdout) }()
	go func() { defer wg.Done(); pump(pipes.stderr, true, &stderr) }()

	readersDone := make(chan struct{})
	go func() {
		wg.Wait()
		close(readersDone)
	}()

	waitErr := cmd.Wait()
	pipes.waitForDrain(readersDone)

	mu.Lock()
	result.Output = stdout.String()
	errText := stderr.String()
	mu.Unlock()
	if result.Output != "" && errText != "" && !strings.HasSuffix(result.Output, "\n") {
		result.Output += "\n"
	}
	result.Output += errText

	if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
		result.TimedOut = true
		result.ExitCode = 124 // conventional timeout status
		return result, ErrShellCommandTimedOut
	}

	result.ExitCode = exitCodeOf(waitErr)
	return result, nil
}

// exitCodeOf decodes the error from cmd.Wait into an exit status. A command that
// was signalled reports 128 plus the signal number, which is the shell
// convention and is more informative than a bare failure.
func exitCodeOf(waitErr error) int {
	if waitErr == nil {
		return 0
	}
	if exitErr, ok := errors.AsType[*exec.ExitError](waitErr); ok {
		if code := exitErr.ExitCode(); code >= 0 {
			return code
		}
		// ExitCode reports -1 when the process died from a signal, so the
		// status is reconstructed here.
		if status, ok := exitErr.Sys().(interface{ Signaled() bool }); ok && status.Signaled() {
			return 128
		}
		return 1
	}
	return 1
}
