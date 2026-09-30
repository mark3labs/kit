package core

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"charm.land/fantasy"
)

// memFS is a FileSystem that keeps an overlay in memory, like an editor with
// unsaved buffers. Reads of paths not in the overlay fall back to disk.
type memFS struct {
	mu     sync.Mutex
	files  map[string]string
	reads  []string
	writes []string
}

func (m *memFS) ReadTextFile(_ context.Context, path string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.reads = append(m.reads, path)
	if s, ok := m.files[path]; ok {
		return s, nil
	}
	b, err := os.ReadFile(path)
	return string(b), err
}

func (m *memFS) WriteTextFile(_ context.Context, path, content string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.writes = append(m.writes, path)
	m.files[path] = content
	return nil
}

func call(t *testing.T, args map[string]any) fantasy.ToolCall {
	t.Helper()
	b, err := json.Marshal(args)
	if err != nil {
		t.Fatal(err)
	}
	return fantasy.ToolCall{ID: "call-1", Input: string(b)}
}

func TestFileSystemIsUsedByReadWriteEdit(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(p, []byte("on disk\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	fs := &memFS{files: map[string]string{p: "unsaved buffer\n"}}
	opts := []ToolOption{WithWorkDir(dir), WithFileSystem(fs)}
	ctx := context.Background()

	resp, err := NewReadTool(opts...).Run(ctx, call(t, map[string]any{"path": "a.txt"}))
	if err != nil || !strings.Contains(resp.Content, "unsaved buffer") {
		t.Fatalf("read = %q, %v; want the FileSystem content", resp.Content, err)
	}

	resp, err = NewEditTool(opts...).Run(ctx, call(t, map[string]any{
		"path": "a.txt", "edits": []map[string]any{{"old_text": "unsaved", "new_text": "edited"}},
	}))
	if err != nil || resp.IsError {
		t.Fatalf("edit = %q, %v", resp.Content, err)
	}
	if fs.files[p] != "edited buffer\n" {
		t.Errorf("edit wrote %q through the FileSystem", fs.files[p])
	}

	resp, err = NewWriteTool(opts...).Run(ctx, call(t, map[string]any{"path": "b.txt", "content": "new"}))
	if err != nil || resp.IsError {
		t.Fatalf("write = %q, %v", resp.Content, err)
	}
	if fs.files[filepath.Join(dir, "b.txt")] != "new" {
		t.Error("write did not go through the FileSystem")
	}
	if b, _ := os.ReadFile(p); string(b) != "on disk\n" {
		t.Errorf("disk changed to %q; all writes must go through the FileSystem", b)
	}
}

func TestReadWithoutFileSystemUsesDisk(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("disk"), 0o600); err != nil {
		t.Fatal(err)
	}
	resp, err := NewReadTool(WithWorkDir(dir)).Run(context.Background(), call(t, map[string]any{"path": "a.txt"}))
	if err != nil || !strings.Contains(resp.Content, "disk") {
		t.Fatalf("read = %q, %v", resp.Content, err)
	}
}

type fakeRunner struct {
	req CommandRequest
	res CommandResult
	err error
}

func (f *fakeRunner) RunCommand(_ context.Context, req CommandRequest) (CommandResult, error) {
	f.req = req
	return f.res, f.err
}

func TestShellUsesCommandRunner(t *testing.T) {
	r := &fakeRunner{res: CommandResult{Output: "hello\n", ExitCode: 0}}
	tool := NewShellTool(WithWorkDir("/work"), WithCommandRunner(r), WithShellTimeout(30*time.Second))
	resp, err := tool.Run(context.Background(), call(t, map[string]any{"command": "echo hello"}))
	if err != nil || resp.IsError || !strings.Contains(resp.Content, "hello") {
		t.Fatalf("shell = %q, %v", resp.Content, err)
	}
	if r.req.ToolCallID != "call-1" || r.req.Command != "echo hello" || r.req.WorkDir != "/work" || r.req.Timeout != 30*time.Second {
		t.Errorf("request = %+v", r.req)
	}
	if n := len(r.req.Argv); n < 2 || r.req.Argv[n-1] != "echo hello" {
		t.Errorf("argv = %q, want the shell followed by the command", r.req.Argv)
	}

	r.res = CommandResult{Output: "boom", ExitCode: 2}
	resp, _ = tool.Run(context.Background(), call(t, map[string]any{"command": "false"}))
	if !resp.IsError || !strings.Contains(resp.Content, "Exit code: 2") {
		t.Errorf("failed command = %q", resp.Content)
	}

	r.res = CommandResult{Output: "partial", TimedOut: true}
	resp, _ = tool.Run(context.Background(), call(t, map[string]any{"command": "sleep 99"}))
	if !resp.IsError || !strings.Contains(resp.Content, "timed out") || !strings.Contains(resp.Content, "partial") {
		t.Errorf("timed out command = %q", resp.Content)
	}

	// The tool's own checks still apply before the runner.
	r.req = CommandRequest{}
	resp, _ = tool.Run(context.Background(), call(t, map[string]any{"command": "source x"}))
	if !resp.IsError || r.req.Command != "" {
		t.Errorf("banned command reached the runner: %q", resp.Content)
	}
}

func TestReadFindsFileThatExistsOnlyInFileSystem(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "new.txt")
	fs := &memFS{files: map[string]string{p: "only in the buffer"}}
	resp, err := NewReadTool(WithWorkDir(dir), WithFileSystem(fs)).Run(context.Background(), call(t, map[string]any{"path": "new.txt"}))
	if err != nil || resp.IsError || !strings.Contains(resp.Content, "only in the buffer") {
		t.Fatalf("read = %q, %v", resp.Content, err)
	}
}

// A write over a file that exists only in the FileSystem must report the
// previous content in its diff metadata, not a new file.
func TestWriteDiffUsesFileSystemContent(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "buf.txt")
	fs := &memFS{files: map[string]string{p: "old line\n"}}
	resp, err := NewWriteTool(WithWorkDir(dir), WithFileSystem(fs)).Run(context.Background(),
		call(t, map[string]any{"path": "buf.txt", "content": "new line\n"}))
	if err != nil || resp.IsError {
		t.Fatalf("write = %q, %v", resp.Content, err)
	}
	meta := resp.Metadata
	if strings.Contains(meta, `"is_new":true`) || !strings.Contains(meta, "old line") {
		t.Errorf("metadata = %s; want the previous FileSystem content and is_new false", meta)
	}
}

// A relative WorkDir must still give the FileSystem absolute paths.
func TestRelativeWorkDirGivesAbsolutePaths(t *testing.T) {
	base := t.TempDir()
	t.Chdir(base)
	if err := os.Mkdir("project", 0o755); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(base, "project", "a.txt")
	fs := &memFS{files: map[string]string{want: "content"}}
	resp, err := NewReadTool(WithWorkDir("project"), WithFileSystem(fs)).Run(context.Background(),
		call(t, map[string]any{"path": "a.txt"}))
	if err != nil || resp.IsError {
		t.Fatalf("read = %q, %v", resp.Content, err)
	}
	for _, r := range fs.reads {
		if !filepath.IsAbs(r) {
			t.Errorf("FileSystem got a relative path %q", r)
		}
	}
	if got, _ := resolvePathWithWorkDir("a.txt", "project"); got != want {
		t.Errorf("resolvePathWithWorkDir = %q, want %q", got, want)
	}
}
