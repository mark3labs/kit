package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
)

// MCPTaskMode controls when the connection pool augments tools/call requests
// with MCP task metadata. See https://modelcontextprotocol.io/specification/2025-11-25/basic/utilities/tasks.
type MCPTaskMode string

const (
	// MCPTaskModeAuto augments tools/call with task metadata only when the
	// server advertises tasks/toolCalls capability during initialize.
	MCPTaskModeAuto MCPTaskMode = "auto"
	// MCPTaskModeNever forces every tools/call to be issued synchronously
	// (no Task field in the request), regardless of server capability.
	MCPTaskModeNever MCPTaskMode = "never"
	// MCPTaskModeAlways always sets a Task field on the tools/call request,
	// even when the server didn't advertise task support. The server may
	// still respond synchronously; this just opts in unconditionally on
	// the client side.
	MCPTaskModeAlways MCPTaskMode = "always"
)

// ParseTaskMode normalises a per-server tasks-mode string from
// configuration. Empty input maps to MCPTaskModeAuto. Unknown values are
// also treated as MCPTaskModeAuto so a stray config typo never breaks
// existing flows.
func ParseTaskMode(s string) MCPTaskMode {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "auto":
		return MCPTaskModeAuto
	case "never", "off", "disabled":
		return MCPTaskModeNever
	case "always", "force":
		return MCPTaskModeAlways
	default:
		return MCPTaskModeAuto
	}
}

// MCPTaskInfo is the connection-layer view of an MCP Task. It mirrors the
// upstream mcp.Task but exposes Go-native types and includes the originating
// server name. SDK-level wrappers re-export this under public-facing names.
type MCPTaskInfo struct {
	// Server is the configured MCP server name this task lives on.
	Server string
	// TaskID is the server-assigned identifier for the task.
	TaskID string
	// Status is the current task lifecycle state.
	Status mcp.TaskStatus
	// StatusMessage is an optional human-readable description.
	StatusMessage string
	// CreatedAt is the wall-clock time the task was created (best-effort
	// parsed from the server's ISO-8601 timestamp; zero on parse failure).
	CreatedAt time.Time
	// UpdatedAt is the wall-clock time the task was last updated (best-
	// effort parsed; zero on parse failure).
	UpdatedAt time.Time
	// TTL is the time-to-live the server intends to retain the task after
	// creation. Zero means the server did not advertise a TTL.
	TTL time.Duration
	// PollInterval is the suggested polling interval. Zero means use the
	// client's default.
	PollInterval time.Duration
}

// MCPTaskProgress is emitted while the connection pool is waiting on a
// task-augmented tool call. It provides minimal feedback for SDK consumers
// that want to render progress widgets without subscribing to the full
// notifications/tasks/status channel (Phase 2).
type MCPTaskProgress struct {
	Server  string
	TaskID  string
	Status  mcp.TaskStatus
	Message string
}

// MCPTaskProgressHandler is invoked once after a task is accepted and on
// every status transition observed by the polling loop. The final
// invocation always carries a terminal status. Implementations must not
// block; long work should be queued on a goroutine.
type MCPTaskProgressHandler func(MCPTaskProgress)

// MCPTaskConfig configures task-aware tool execution on the manager.
// All fields are optional; the zero value disables progress callbacks and
// applies sensible defaults.
type MCPTaskConfig struct {
	// PerServerMode overrides the per-server TasksMode resolved from
	// MCPServerConfig. Keys are server names. Missing entries fall back
	// to the value from config. Used by SDK consumers that want to set
	// modes programmatically.
	PerServerMode map[string]MCPTaskMode

	// DefaultTTL is the TTL hint sent in TaskParams when augmenting a
	// tools/call. Zero means omit the TTL — let the server pick its own.
	DefaultTTL time.Duration

	// PollInterval is the fallback interval between tasks/get requests
	// when the server does not suggest one. Zero defaults to 1 second.
	PollInterval time.Duration

	// MaxPollInterval caps the polling interval. Zero defaults to 5 seconds.
	MaxPollInterval time.Duration

	// Timeout is the maximum wall-clock duration to wait for a task to
	// reach a terminal state. Zero defaults to 15 minutes. Independent
	// of the per-call context deadline; whichever fires first wins.
	Timeout time.Duration

	// Progress, if non-nil, receives every status transition observed by
	// the polling loop.
	Progress MCPTaskProgressHandler
}

func (c MCPTaskConfig) resolved() MCPTaskConfig {
	if c.PollInterval <= 0 {
		c.PollInterval = 1 * time.Second
	}
	if c.MaxPollInterval <= 0 {
		c.MaxPollInterval = 5 * time.Second
	}
	if c.Timeout <= 0 {
		c.Timeout = 15 * time.Minute
	}
	return c
}

// requestIDCounter generates monotonically increasing JSON-RPC request IDs
// for low-level tools/call invocations that bypass the upstream client's
// ParseCallToolResult helper (necessary because that helper rejects task
// responses for lacking a "content" field).
//
// The counter is process-wide rather than per-manager so multiple managers
// or repeated calls within the same connection produce unique IDs.
var requestIDCounter atomic.Int64

func nextRequestID() mcp.RequestId {
	return mcp.NewRequestId(requestIDCounter.Add(1))
}

// callToolWithTask issues tools/call directly on the transport so we can
// observe both response shapes:
//
//   - {"content": [...], ...}  — synchronous CallToolResult.
//   - {"task": {...}, ...}     — asynchronous CreateTaskResult.
//
// On success exactly one of (callResult, taskResult) is non-nil. The
// upstream client.CallTool helper parses the response with
// mcp.ParseCallToolResult which requires a "content" field, so it cannot
// be used for task-augmented calls.
func callToolWithTask(
	ctx context.Context,
	c *client.Client,
	params mcp.CallToolParams,
) (callResult *mcp.CallToolResult, taskResult *mcp.CreateTaskResult, err error) {
	tr := c.GetTransport()
	if tr == nil {
		return nil, nil, errors.New("mcp client has no transport")
	}

	req := transport.JSONRPCRequest{
		JSONRPC: mcp.JSONRPC_VERSION,
		ID:      nextRequestID(),
		Method:  string(mcp.MethodToolsCall),
		Params:  params,
	}

	resp, sendErr := tr.SendRequest(ctx, req)
	if sendErr != nil {
		return nil, nil, sendErr
	}
	if resp.Error != nil {
		return nil, nil, resp.Error.AsError()
	}

	// Peek at the raw result to decide which shape we got.
	var probe struct {
		Task    json.RawMessage `json:"task"`
		Content json.RawMessage `json:"content"`
	}
	raw := resp.Result
	if len(raw) == 0 {
		return nil, nil, errors.New("empty tools/call result")
	}
	if uErr := json.Unmarshal(raw, &probe); uErr != nil {
		return nil, nil, fmt.Errorf("decode tools/call result: %w", uErr)
	}

	if len(probe.Task) > 0 && string(probe.Task) != "null" {
		// Task-augmented response.
		var ct mcp.CreateTaskResult
		if uErr := json.Unmarshal(raw, &ct); uErr != nil {
			return nil, nil, fmt.Errorf("decode CreateTaskResult: %w", uErr)
		}
		return nil, &ct, nil
	}

	// Synchronous response — defer to the upstream parser so content blocks
	// are typed correctly (TextContent, ImageContent, ResourceLink, etc.).
	cr, pErr := mcp.ParseCallToolResult(&raw)
	if pErr != nil {
		return nil, nil, fmt.Errorf("parse CallToolResult: %w", pErr)
	}
	return cr, nil, nil
}

// pollTaskUntilTerminal blocks until the task reaches a terminal status,
// the context is cancelled, or the configured timeout elapses. On
// cancellation it best-effort issues tasks/cancel before returning.
func pollTaskUntilTerminal(
	ctx context.Context,
	c *client.Client,
	serverName string,
	task mcp.Task,
	cfg MCPTaskConfig,
	progress MCPTaskProgressHandler,
) (*mcp.TaskResultResult, error) {
	cfg = cfg.resolved()
	deadline := time.Now().Add(cfg.Timeout)

	emit := func(status mcp.TaskStatus, msg string) {
		if progress != nil {
			progress(MCPTaskProgress{Server: serverName, TaskID: task.TaskId, Status: status, Message: msg})
		}
	}

	emit(task.Status, task.StatusMessage)

	current := task
	interval := cfg.PollInterval
	if current.PollInterval != nil && *current.PollInterval > 0 {
		interval = time.Duration(*current.PollInterval) * time.Millisecond
	}
	if interval > cfg.MaxPollInterval {
		interval = cfg.MaxPollInterval
	}

	for !current.Status.IsTerminal() {
		if time.Now().After(deadline) {
			cancelTaskBestEffort(c, current.TaskId)
			return nil, fmt.Errorf("task %s timed out after %s", current.TaskId, cfg.Timeout)
		}

		// Wait between polls or abort early on context cancellation.
		select {
		case <-ctx.Done():
			cancelTaskBestEffort(c, current.TaskId)
			return nil, ctx.Err()
		case <-time.After(interval):
		}

		got, err := c.GetTask(ctx, mcp.GetTaskRequest{
			Params: mcp.GetTaskParams{TaskId: current.TaskId},
		})
		if err != nil {
			// Transient transport hiccup — propagate immediately. The
			// upstream agent layer treats this like any other tool error.
			return nil, fmt.Errorf("tasks/get failed: %w", err)
		}
		current = got.Task
		if current.Status != task.Status || current.StatusMessage != task.StatusMessage {
			emit(current.Status, current.StatusMessage)
			task = current
		}

		// Honour any updated suggested poll interval, capped at the limit.
		if current.PollInterval != nil && *current.PollInterval > 0 {
			interval = min(time.Duration(*current.PollInterval)*time.Millisecond, cfg.MaxPollInterval)
		}
	}

	// Terminal state reached. Emit one last progress event and fetch the
	// definitive tool result.
	emit(current.Status, current.StatusMessage)

	if current.Status == mcp.TaskStatusCancelled {
		return nil, fmt.Errorf("task %s was cancelled", current.TaskId)
	}

	res, err := fetchTaskResult(ctx, c, current.TaskId)
	if err != nil {
		return nil, fmt.Errorf("tasks/result failed: %w", err)
	}
	if current.Status == mcp.TaskStatusFailed && res != nil && !res.IsError {
		// The server flagged the task as failed but didn't decorate the
		// result. Surface the status message so the caller still sees a
		// useful tool-error.
		return nil, fmt.Errorf("task %s failed: %s", current.TaskId, current.StatusMessage)
	}
	return res, nil
}

// fetchTaskResult issues tasks/result on the transport and parses the raw
// response. The upstream client.TaskResult helper delegates to
// mcp.ParseTaskResultResult which (as of mcp-go v0.51.0) looks for the
// content array under a nested "result" key that never exists in the
// wire format — leading to systematically empty Content. Doing the
// parse here keeps the polling path working until that is fixed upstream.
func fetchTaskResult(ctx context.Context, c *client.Client, taskID string) (*mcp.TaskResultResult, error) {
	tr := c.GetTransport()
	if tr == nil {
		return nil, errors.New("mcp client has no transport")
	}
	req := transport.JSONRPCRequest{
		JSONRPC: mcp.JSONRPC_VERSION,
		ID:      nextRequestID(),
		Method:  string(mcp.MethodTasksResult),
		Params:  mcp.TaskResultParams{TaskId: taskID},
	}
	resp, err := tr.SendRequest(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, resp.Error.AsError()
	}

	// Manually decode the wire shape: {"_meta": {...}, "content": [...],
	// "structuredContent": ..., "isError": bool}.
	var shape struct {
		Meta              json.RawMessage   `json:"_meta"`
		Content           []json.RawMessage `json:"content"`
		StructuredContent any               `json:"structuredContent"`
		IsError           bool              `json:"isError"`
	}
	if err := json.Unmarshal(resp.Result, &shape); err != nil {
		return nil, fmt.Errorf("decode tasks/result: %w", err)
	}

	out := &mcp.TaskResultResult{
		StructuredContent: shape.StructuredContent,
		IsError:           shape.IsError,
	}
	if len(shape.Meta) > 0 && string(shape.Meta) != "null" {
		var metaMap map[string]any
		if err := json.Unmarshal(shape.Meta, &metaMap); err == nil {
			out.Meta = mcp.NewMetaFromMap(metaMap)
		}
	}
	for _, raw := range shape.Content {
		var contentMap map[string]any
		if err := json.Unmarshal(raw, &contentMap); err != nil {
			return nil, fmt.Errorf("decode content block: %w", err)
		}
		parsed, err := mcp.ParseContent(contentMap)
		if err != nil {
			return nil, fmt.Errorf("parse content block: %w", err)
		}
		out.Content = append(out.Content, parsed)
	}
	return out, nil
}

// cancelTaskBestEffort issues tasks/cancel and ignores any error. Used on
// context cancellation paths where the connection is already going away.
func cancelTaskBestEffort(c *client.Client, taskID string) {
	if c == nil || taskID == "" {
		return
	}
	cancelCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, _ = c.CancelTask(cancelCtx, mcp.CancelTaskRequest{
		Params: mcp.CancelTaskParams{TaskId: taskID},
	})
}

// taskFromMCP converts a wire-format mcp.Task to our richer connection-
// layer view. Unparseable timestamps surface as the zero time.
func taskFromMCP(serverName string, t mcp.Task) MCPTaskInfo {
	out := MCPTaskInfo{
		Server:        serverName,
		TaskID:        t.TaskId,
		Status:        t.Status,
		StatusMessage: t.StatusMessage,
	}
	if t.CreatedAt != "" {
		if v, err := time.Parse(time.RFC3339, t.CreatedAt); err == nil {
			out.CreatedAt = v
		}
	}
	if t.LastUpdatedAt != "" {
		if v, err := time.Parse(time.RFC3339, t.LastUpdatedAt); err == nil {
			out.UpdatedAt = v
		}
	}
	if t.TTL != nil {
		out.TTL = time.Duration(*t.TTL) * time.Millisecond
	}
	if t.PollInterval != nil {
		out.PollInterval = time.Duration(*t.PollInterval) * time.Millisecond
	}
	return out
}
