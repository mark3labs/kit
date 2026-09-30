package acpserver

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/charmbracelet/log"
	acp "github.com/coder/acp-go-sdk"

	kit "github.com/mark3labs/kit/pkg/kit"
)

// clientToolOptions returns the core tool options that route file access and
// commands through the client, for the capabilities the client advertised in
// initialize. The spec says an agent MUST NOT use a client method the client
// did not advertise.
func (a *Agent) clientToolOptions(sess *acpSession) []kit.ToolOption {
	caps := a.clientCapabilities()
	var opts []kit.ToolOption
	if caps.Fs.ReadTextFile || caps.Fs.WriteTextFile {
		opts = append(opts, kit.WithFileSystem(&clientFS{
			agent: a, sess: sess,
			read:  caps.Fs.ReadTextFile,
			write: caps.Fs.WriteTextFile,
		}))
	}
	if caps.Terminal {
		opts = append(opts, kit.WithCommandRunner(&clientTerminal{agent: a, sess: sess}))
	}
	return opts
}

// clientFS reads and writes text files through the client (fs/read_text_file
// and fs/write_text_file), so the agent sees unsaved editor buffers and the
// editor tracks the agent's changes. A method the client did not advertise
// falls back to the local disk.
type clientFS struct {
	agent       *Agent
	sess        *acpSession
	read, write bool
	local       localFS
}

func (f *clientFS) ReadTextFile(ctx context.Context, path string) (string, error) {
	if !f.read {
		return f.local.ReadTextFile(ctx, path)
	}
	resp, err := f.agent.conn.ReadTextFile(ctx, acp.ReadTextFileRequest{
		SessionId: f.sess.id(),
		Path:      path,
	})
	if err != nil {
		return "", fmt.Errorf("client read %s: %w", path, err)
	}
	return resp.Content, nil
}

func (f *clientFS) WriteTextFile(ctx context.Context, path, content string) error {
	if !f.write {
		return f.local.WriteTextFile(ctx, path, content)
	}
	if _, err := f.agent.conn.WriteTextFile(ctx, acp.WriteTextFileRequest{
		SessionId: f.sess.id(),
		Path:      path,
		Content:   content,
	}); err != nil {
		return fmt.Errorf("client write %s: %w", path, err)
	}
	return nil
}

// terminalOutputLimit caps the output the client keeps for one command. The
// shell tool truncates the result for the model further.
const terminalOutputLimit = 1 << 20

// clientTerminal runs the shell tool's commands in client terminals
// (terminal/*). The terminal is embedded in the tool call, so the user sees
// the output live in the editor.
type clientTerminal struct {
	agent *Agent
	sess  *acpSession
}

func (t *clientTerminal) RunCommand(ctx context.Context, req kit.CommandRequest) (kit.CommandResult, error) {
	if len(req.Argv) == 0 {
		return kit.CommandResult{}, errors.New("empty command")
	}
	conn := t.agent.conn
	sid := t.sess.id()

	create := acp.CreateTerminalRequest{
		SessionId:       sid,
		Command:         req.Argv[0],
		Args:            req.Argv[1:],
		OutputByteLimit: new(terminalOutputLimit),
	}
	if req.WorkDir != "" {
		create.Cwd = new(req.WorkDir)
	}
	created, err := conn.CreateTerminal(ctx, create)
	if err != nil {
		return kit.CommandResult{}, fmt.Errorf("create terminal: %w", err)
	}
	termID := created.TerminalId

	// Release the terminal in every case. The client keeps showing its
	// output in the tool call after the release. A cancelled turn must not
	// stop the release, so it uses its own context.
	defer func() {
		rctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		if _, err := conn.ReleaseTerminal(rctx, acp.ReleaseTerminalRequest{SessionId: sid, TerminalId: termID}); err != nil {
			log.Debug("acp: release terminal failed", "terminal", termID, "error", err)
		}
	}()

	// Show the terminal in the tool call. Subagent tool calls are not known
	// to this session, so they run without a visible terminal.
	if id, ok := t.sess.toolIDs.find(req.ToolCallID); ok {
		t.sess.mu.Lock()
		t.sess.terminals[id] = termID
		t.sess.mu.Unlock()
		t.agent.sendUpdate(t.sess, acp.UpdateToolCall(id,
			acp.WithUpdateContent([]acp.ToolCallContent{acp.ToolTerminalRef(termID)}),
		))
	}

	waitCtx, cancel := context.WithTimeout(ctx, req.Timeout)
	defer cancel()
	exit, waitErr := conn.WaitForTerminalExit(waitCtx, acp.WaitForTerminalExitRequest{SessionId: sid, TerminalId: termID})

	res := kit.CommandResult{}
	if waitErr != nil {
		// Timed out or cancelled: kill the command, then collect what it
		// wrote so far.
		kctx, kcancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		if _, err := conn.KillTerminal(kctx, acp.KillTerminalRequest{SessionId: sid, TerminalId: termID}); err != nil {
			log.Debug("acp: kill terminal failed", "terminal", termID, "error", err)
		}
		kcancel()
		if ctx.Err() != nil {
			return kit.CommandResult{}, ctx.Err()
		}
		if waitCtx.Err() == nil {
			return kit.CommandResult{}, fmt.Errorf("wait for terminal: %w", waitErr)
		}
		res.TimedOut = true
	} else {
		if exit.ExitCode != nil {
			res.ExitCode = *exit.ExitCode
		}
		if exit.Signal != nil {
			res.Signal = *exit.Signal
		}
	}

	octx, ocancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer ocancel()
	out, err := conn.TerminalOutput(octx, acp.TerminalOutputRequest{SessionId: sid, TerminalId: termID})
	if err != nil {
		return kit.CommandResult{}, fmt.Errorf("terminal output: %w", err)
	}
	res.Output = out.Output
	if out.Truncated {
		res.Output = "[output truncated]\n" + res.Output
	}
	return res, nil
}

// localFS is the local disk, used for client file methods the client did not
// advertise.
type localFS struct{}

func (localFS) ReadTextFile(_ context.Context, path string) (string, error) {
	b, err := os.ReadFile(path)
	return string(b), err
}

func (localFS) WriteTextFile(_ context.Context, path, content string) error {
	return os.WriteFile(path, []byte(content), 0o644)
}
