package core

import (
	"context"
	"os"
	"time"
)

// FileSystem reads and writes text files for the read, write and edit
// tools. The default reads and writes the local disk. An embedder can route
// file access elsewhere, for example to an editor that has unsaved changes.
//
// Only text content goes through a FileSystem. The read tool still reads
// images from the local disk, and the tools still check paths (existence,
// directories) on the local disk.
type FileSystem interface {
	// ReadTextFile returns the full content of the text file at the absolute
	// path. It returns an error that satisfies errors.Is(err, os.ErrNotExist)
	// when the file does not exist, if it can tell.
	ReadTextFile(ctx context.Context, path string) (string, error)
	// WriteTextFile replaces the content of the file at the absolute path,
	// creating the file when it does not exist.
	WriteTextFile(ctx context.Context, path, content string) error
}

// CommandRequest describes one command the shell tool wants to run.
type CommandRequest struct {
	// ToolCallID is the ID of the shell tool call that runs the command.
	ToolCallID string
	// Command is the command string the model wrote.
	Command string
	// Argv is the full argument vector to run: the resolved shell, its
	// arguments, and Command, e.g. ["bash", "-c", "go test ./..."].
	Argv []string
	// WorkDir is the directory to run in. Empty means the process working
	// directory.
	WorkDir string
	// Timeout is how long the command may run.
	Timeout time.Duration
}

// CommandResult is the outcome of a command run by a CommandRunner.
type CommandResult struct {
	// Output is the combined stdout and stderr.
	Output string
	// ExitCode is the exit status. It is ignored when Signal is set.
	ExitCode int
	// Signal is the name of the signal that ended the command, if any.
	Signal string
	// TimedOut reports that the command was stopped at Timeout.
	TimedOut bool
}

// CommandRunner runs the shell tool's commands somewhere other than a local
// child process, for example in an editor's terminal. A runner receives the
// command after the tool's own checks (banned builtins, timeout limits).
// Interactive sudo password prompts are not available through a runner.
type CommandRunner interface {
	RunCommand(ctx context.Context, req CommandRequest) (CommandResult, error)
}

// WithFileSystem routes the text file access of the read, write and edit
// tools through fs. A nil fs keeps the local disk.
func WithFileSystem(fs FileSystem) ToolOption {
	return func(c *ToolConfig) {
		c.FileSystem = fs
	}
}

// WithCommandRunner makes the shell tool run commands through r. A nil r
// keeps local child processes.
func WithCommandRunner(r CommandRunner) ToolOption {
	return func(c *ToolConfig) {
		c.CommandRunner = r
	}
}

// localFS is the default FileSystem: the local disk.
type localFS struct{}

func (localFS) ReadTextFile(_ context.Context, path string) (string, error) {
	b, err := os.ReadFile(path)
	return string(b), err
}

func (localFS) WriteTextFile(_ context.Context, path, content string) error {
	return os.WriteFile(path, []byte(content), 0644)
}

// fileSystem returns the configured FileSystem or the local disk.
func (c ToolConfig) fileSystem() FileSystem {
	if c.FileSystem != nil {
		return c.FileSystem
	}
	return localFS{}
}
