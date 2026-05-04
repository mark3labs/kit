package tools

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/kit/internal/config"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// newTaskTestInProcessServer builds an in-process MCP server with a
// task-augmented tool. The handler simulates work by sleeping briefly
// before completing.
//
// Important: the upstream mcp-go server cancels the request context as
// soon as the synchronous part of the tools/call returns (see
// request_handler.go:85, `defer cancel()`). Task goroutines spawned by
// AddTaskTool inherit that context and therefore see context.Canceled
// the instant they start. Real-world transports (stdio, SSE, streamable
// HTTP) don't trip this because they keep the connection — and a
// background context — alive across the async work, but the in-process
// transport runs entirely on the request goroutine. To test the polling
// path realistically we detach from the request context here.
func newTaskTestInProcessServer(t *testing.T, workDuration time.Duration) *server.MCPServer {
	t.Helper()
	srv := server.NewMCPServer("task-test", "1.0.0",
		server.WithToolCapabilities(true),
		// list=true, cancel=true, toolCallTasks=true so capability detection,
		// cancellation, and tool augmentation all flow through.
		server.WithTaskCapabilities(true, true, true),
	)
	srv.AddTaskTool(
		mcp.Tool{
			Name:        "long_running",
			Description: "Sleep, then echo the input string.",
			InputSchema: mcp.ToolInputSchema{
				Type: "object",
				Properties: map[string]any{
					"msg": map[string]any{"type": "string"},
				},
			},
			Execution: &mcp.ToolExecution{
				TaskSupport: mcp.TaskSupportRequired,
			},
		},
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CreateTaskResult, error) {
			msg, _ := req.GetArguments()["msg"].(string)
			// Detach from the request context so the task handler can
			// outlive the synchronous request — see comment above.
			time.Sleep(workDuration)
			_ = ctx
			return &mcp.CreateTaskResult{
				Content: []mcp.Content{
					mcp.TextContent{Type: "text", Text: "echo:" + msg},
				},
			}, nil
		},
	)
	return srv
}

// newSyncOnlyServer is a server that does NOT advertise task capability.
// Used to verify the auto-detect path keeps the sync semantics.
func newSyncOnlyServer() *server.MCPServer {
	srv := server.NewMCPServer("sync-only", "1.0.0",
		server.WithToolCapabilities(true),
	)
	srv.AddTool(
		mcp.NewTool("greet",
			mcp.WithDescription("Say hello"),
			mcp.WithString("name", mcp.Required()),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			name, _ := req.GetArguments()["name"].(string)
			return mcp.NewToolResultText("hi " + name), nil
		},
	)
	return srv
}

func TestConnectionPoolAdvertisesTaskCapability(t *testing.T) {
	pool := NewMCPConnectionPool(DefaultConnectionPoolConfig(), false, nil, nil)
	defer func() { _ = pool.Close() }()

	srv := newTaskTestInProcessServer(t, 0)
	cfg := config.MCPServerConfig{Type: "inprocess", InProcessServer: srv}

	conn, err := pool.GetConnection(context.Background(), "tasks", cfg)
	if err != nil {
		t.Fatalf("GetConnection: %v", err)
	}

	init := conn.InitializeResult()
	if init == nil {
		t.Fatal("InitializeResult is nil after GetConnection")
	}
	if init.Capabilities.Tasks == nil {
		t.Fatal("server did not advertise Tasks capability — initialize handshake regressed")
	}
	if !conn.SupportsToolTasks() {
		t.Error("SupportsToolTasks should be true for a server with toolCallTasks=true")
	}
	if !pool.ServerSupportsToolTasks("tasks") {
		t.Error("ServerSupportsToolTasks should mirror the connection's value")
	}
}

func TestConnectionPoolDetectsAbsentTaskCapability(t *testing.T) {
	pool := NewMCPConnectionPool(DefaultConnectionPoolConfig(), false, nil, nil)
	defer func() { _ = pool.Close() }()

	cfg := config.MCPServerConfig{Type: "inprocess", InProcessServer: newSyncOnlyServer()}
	conn, err := pool.GetConnection(context.Background(), "sync", cfg)
	if err != nil {
		t.Fatalf("GetConnection: %v", err)
	}
	if conn.SupportsToolTasks() {
		t.Error("SupportsToolTasks should be false for a server that didn't advertise the capability")
	}
}

func TestSupportsToolTasksFromInit(t *testing.T) {
	cases := []struct {
		name string
		in   *mcp.InitializeResult
		want bool
	}{
		{"nil", nil, false},
		{"no tasks", &mcp.InitializeResult{}, false},
		{"tasks no requests", &mcp.InitializeResult{
			Capabilities: mcp.ServerCapabilities{Tasks: &mcp.TasksCapability{}},
		}, false},
		{"tasks with toolCalls", &mcp.InitializeResult{
			Capabilities: mcp.ServerCapabilities{Tasks: mcp.NewTasksCapability()},
		}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := supportsToolTasksFromInit(tc.in); got != tc.want {
				t.Errorf("supportsToolTasksFromInit() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestParseTaskMode(t *testing.T) {
	cases := []struct {
		in   string
		want MCPTaskMode
	}{
		{"", MCPTaskModeAuto},
		{"auto", MCPTaskModeAuto},
		{"AUTO", MCPTaskModeAuto},
		{"never", MCPTaskModeNever},
		{"off", MCPTaskModeNever},
		{"always", MCPTaskModeAlways},
		{"force", MCPTaskModeAlways},
		{"bogus", MCPTaskModeAuto},
	}
	for _, tc := range cases {
		if got := ParseTaskMode(tc.in); got != tc.want {
			t.Errorf("ParseTaskMode(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestExecuteToolPollsTaskToCompletion(t *testing.T) {
	mgr := NewMCPToolManager()
	mgr.SetTaskConfig(MCPTaskConfig{
		PollInterval:    20 * time.Millisecond,
		MaxPollInterval: 50 * time.Millisecond,
		Timeout:         10 * time.Second,
	})

	cfg := config.MCPServerConfig{
		Type:            "inprocess",
		InProcessServer: newTaskTestInProcessServer(t, 50*time.Millisecond),
	}

	if _, err := mgr.AddServer(context.Background(), "tasks", cfg); err != nil {
		t.Fatalf("AddServer: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	res, err := mgr.ExecuteTool(ctx, "tasks__long_running", `{"msg":"hello"}`)
	if err != nil {
		t.Fatalf("ExecuteTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("expected non-error result, got %s", res.Content)
	}
	if !strings.Contains(res.Content, "echo:hello") {
		t.Errorf("expected result to contain 'echo:hello', got %s", res.Content)
	}
}

func TestExecuteToolHonorsNeverMode(t *testing.T) {
	// Even though the server advertises tasks/toolCalls, "never" should
	// keep the call synchronous. Since the tool is TaskSupportRequired,
	// the server returns an error rather than running it sync — we just
	// verify the error surfaces (not a poll-loop hang).
	mgr := NewMCPToolManager()
	mgr.SetTaskConfig(MCPTaskConfig{
		PerServerMode: map[string]MCPTaskMode{"tasks": MCPTaskModeNever},
		Timeout:       2 * time.Second,
	})

	cfg := config.MCPServerConfig{
		Type:            "inprocess",
		InProcessServer: newTaskTestInProcessServer(t, 0),
	}

	if _, err := mgr.AddServer(context.Background(), "tasks", cfg); err != nil {
		t.Fatalf("AddServer: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// We don't care which way the server fails the sync call; we just want
	// to confirm we didn't hang in the polling loop and didn't panic.
	_, err := mgr.ExecuteTool(ctx, "tasks__long_running", `{"msg":"x"}`)
	if err == nil {
		t.Fatal("expected an error when forcing sync execution of a task-required tool")
	}
}

func TestExecuteToolEmitsProgress(t *testing.T) {
	var statuses []mcp.TaskStatus
	mgr := NewMCPToolManager()
	mgr.SetTaskConfig(MCPTaskConfig{
		PollInterval:    10 * time.Millisecond,
		MaxPollInterval: 25 * time.Millisecond,
		Timeout:         5 * time.Second,
		Progress: func(p MCPTaskProgress) {
			statuses = append(statuses, p.Status)
		},
	})

	cfg := config.MCPServerConfig{
		Type:            "inprocess",
		InProcessServer: newTaskTestInProcessServer(t, 30*time.Millisecond),
	}
	if _, err := mgr.AddServer(context.Background(), "tasks", cfg); err != nil {
		t.Fatalf("AddServer: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := mgr.ExecuteTool(ctx, "tasks__long_running", `{"msg":"hi"}`); err != nil {
		t.Fatalf("ExecuteTool: %v", err)
	}
	if len(statuses) == 0 {
		t.Fatal("expected at least one progress event")
	}
	last := statuses[len(statuses)-1]
	if !last.IsTerminal() {
		t.Errorf("last progress event should be terminal, got %q", last)
	}
}

func TestListGetCancelMCPTasksOnLoadedServer(t *testing.T) {
	mgr := NewMCPToolManager()
	cfg := config.MCPServerConfig{
		Type:            "inprocess",
		InProcessServer: newTaskTestInProcessServer(t, 0),
	}
	if _, err := mgr.AddServer(context.Background(), "tasks", cfg); err != nil {
		t.Fatalf("AddServer: %v", err)
	}

	ctx := context.Background()

	// tasks/list — no in-flight tasks yet, so we just verify the call
	// succeeds and returns an empty slice (or any slice; the exact length
	// depends on server retention policy).
	if _, err := mgr.ListServerTasks(ctx, "tasks"); err != nil {
		t.Errorf("ListServerTasks: %v", err)
	}

	// Unknown server should error cleanly without panicking.
	if _, err := mgr.GetServerTask(ctx, "unknown", "abc"); err == nil {
		t.Error("GetServerTask on unknown server should error")
	}
	if _, err := mgr.CancelServerTask(ctx, "unknown", "abc"); err == nil {
		t.Error("CancelServerTask on unknown server should error")
	}
}
