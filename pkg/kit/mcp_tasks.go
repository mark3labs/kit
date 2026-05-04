package kit

import (
	"context"
	"fmt"
	"time"

	"github.com/mark3labs/kit/internal/tools"
	"github.com/mark3labs/mcp-go/mcp"
)

// MCPTaskStatus represents the lifecycle state of a task-augmented MCP
// tool call. See https://modelcontextprotocol.io/specification/2025-11-25/basic/utilities/tasks
// for the underlying spec.
type MCPTaskStatus string

const (
	// MCPTaskStatusWorking indicates the task is currently being processed.
	MCPTaskStatusWorking MCPTaskStatus = MCPTaskStatus(mcp.TaskStatusWorking)
	// MCPTaskStatusInputRequired indicates the server is waiting for client
	// input before it can proceed (rare; typically surfaced via elicitation).
	MCPTaskStatusInputRequired MCPTaskStatus = MCPTaskStatus(mcp.TaskStatusInputRequired)
	// MCPTaskStatusCompleted indicates the task finished successfully.
	MCPTaskStatusCompleted MCPTaskStatus = MCPTaskStatus(mcp.TaskStatusCompleted)
	// MCPTaskStatusFailed indicates the task ended in error.
	MCPTaskStatusFailed MCPTaskStatus = MCPTaskStatus(mcp.TaskStatusFailed)
	// MCPTaskStatusCancelled indicates the task was cancelled before completion.
	MCPTaskStatusCancelled MCPTaskStatus = MCPTaskStatus(mcp.TaskStatusCancelled)
)

// IsTerminal reports whether the status represents a final state — that is,
// the task will not change again. Terminal states are completed, failed,
// and cancelled.
func (s MCPTaskStatus) IsTerminal() bool {
	return mcp.TaskStatus(s).IsTerminal()
}

// MCPTaskMode controls when Kit augments tools/call requests with MCP task
// metadata for a specific server.
type MCPTaskMode string

const (
	// MCPTaskModeAuto augments tools/call with task metadata only when the
	// server advertises tasks/toolCalls capability during initialize.
	// This is the default and is safe to leave unconfigured for any
	// existing MCP server.
	MCPTaskModeAuto MCPTaskMode = MCPTaskMode(tools.MCPTaskModeAuto)
	// MCPTaskModeNever forces every tools/call to be issued synchronously
	// (no Task field), regardless of server capability.
	MCPTaskModeNever MCPTaskMode = MCPTaskMode(tools.MCPTaskModeNever)
	// MCPTaskModeAlways always opts into task augmentation, even when the
	// server didn't advertise the capability. The server may still respond
	// synchronously; this just expresses client intent unconditionally.
	MCPTaskModeAlways MCPTaskMode = MCPTaskMode(tools.MCPTaskModeAlways)
)

// MCPTask is the SDK-level view of an MCP Task. Timestamps are best-effort
// parsed from the server's ISO-8601 strings; they may be the zero time when
// the server omitted them or used a non-RFC3339 format.
type MCPTask struct {
	// Server is the configured MCP server name this task lives on.
	Server string
	// TaskID is the server-assigned identifier for the task.
	TaskID string
	// Status is the current task lifecycle state.
	Status MCPTaskStatus
	// StatusMessage is an optional human-readable description provided by
	// the server.
	StatusMessage string
	// CreatedAt is when the task was created on the server.
	CreatedAt time.Time
	// UpdatedAt is when the task was last updated on the server.
	UpdatedAt time.Time
	// TTL is how long the server intends to retain this task after creation.
	// Zero means the server did not advertise a TTL.
	TTL time.Duration
	// PollInterval is the suggested time between status checks. Zero means
	// the client should use its own default.
	PollInterval time.Duration
}

// MCPTaskProgress is a single status update emitted while Kit is waiting
// on a task-augmented tool call.
type MCPTaskProgress struct {
	// Server is the configured MCP server name.
	Server string
	// TaskID is the server-assigned identifier for the in-flight task.
	TaskID string
	// Status is the most recent task status observed.
	Status MCPTaskStatus
	// Message is the optional human-readable status message from the server.
	Message string
}

// MCPTaskProgressHandler is called once when a task is accepted and again
// on every observed status transition. The final invocation always carries
// a terminal status. Implementations must not block; long work should be
// dispatched on a goroutine.
type MCPTaskProgressHandler func(MCPTaskProgress)

// mcpTaskOptions carries SDK consumer configuration into the agent setup.
// Stored on Options as a single value so the public surface stays compact;
// individual fields are exposed via WithMCP* builder functions.
type mcpTaskOptions struct {
	perServer       map[string]MCPTaskMode
	defaultTTL      time.Duration
	pollInterval    time.Duration
	maxPollInterval time.Duration
	timeout         time.Duration
	progress        MCPTaskProgressHandler
}

// toToolsConfig converts the SDK-level config to the internal tools-package
// representation. Keeps the dependency arrow internal-only.
func (o mcpTaskOptions) toToolsConfig() tools.MCPTaskConfig {
	cfg := tools.MCPTaskConfig{
		DefaultTTL:      o.defaultTTL,
		PollInterval:    o.pollInterval,
		MaxPollInterval: o.maxPollInterval,
		Timeout:         o.timeout,
	}
	if len(o.perServer) > 0 {
		cfg.PerServerMode = make(map[string]tools.MCPTaskMode, len(o.perServer))
		for k, v := range o.perServer {
			cfg.PerServerMode[k] = tools.MCPTaskMode(v)
		}
	}
	if o.progress != nil {
		h := o.progress
		cfg.Progress = func(p tools.MCPTaskProgress) {
			h(MCPTaskProgress{
				Server:  p.Server,
				TaskID:  p.TaskID,
				Status:  MCPTaskStatus(p.Status),
				Message: p.Message,
			})
		}
	}
	return cfg
}

// ListMCPTasks queries tasks/list on the named MCP server and returns the
// active and recent tasks the server is willing to disclose. Returns an
// error when the server isn't loaded, doesn't expose tasks/list, or the
// underlying transport fails.
func (m *Kit) ListMCPTasks(ctx context.Context, serverName string) ([]MCPTask, error) {
	mgr, err := m.mcpToolManager()
	if err != nil {
		return nil, err
	}
	infos, err := mgr.ListServerTasks(ctx, serverName)
	if err != nil {
		return nil, err
	}
	out := make([]MCPTask, len(infos))
	for i, t := range infos {
		out[i] = mcpTaskFromInternal(t)
	}
	return out, nil
}

// GetMCPTask queries tasks/get for a single in-flight task on the named
// server. The returned MCPTask reflects the server's current view of the
// task.
func (m *Kit) GetMCPTask(ctx context.Context, serverName, taskID string) (MCPTask, error) {
	mgr, err := m.mcpToolManager()
	if err != nil {
		return MCPTask{}, err
	}
	info, err := mgr.GetServerTask(ctx, serverName, taskID)
	if err != nil {
		return MCPTask{}, err
	}
	return mcpTaskFromInternal(info), nil
}

// CancelMCPTask issues tasks/cancel for an in-flight task on the named
// server. Returns the post-cancel task state when the server responded
// with one. Cancelling an already-terminal task is a no-op on most
// servers.
func (m *Kit) CancelMCPTask(ctx context.Context, serverName, taskID string) (MCPTask, error) {
	mgr, err := m.mcpToolManager()
	if err != nil {
		return MCPTask{}, err
	}
	info, err := mgr.CancelServerTask(ctx, serverName, taskID)
	if err != nil {
		return MCPTask{}, err
	}
	return mcpTaskFromInternal(info), nil
}

// mcpToolManager returns the underlying MCP tool manager or an error when
// no MCP servers are configured.
func (m *Kit) mcpToolManager() (*tools.MCPToolManager, error) {
	if m == nil || m.agent == nil {
		return nil, fmt.Errorf("kit instance has no agent")
	}
	mgr := m.agent.GetMCPToolManager()
	if mgr == nil {
		return nil, fmt.Errorf("no MCP servers configured")
	}
	return mgr, nil
}

// mcpTaskFromInternal adapts the internal tools.MCPTaskInfo to the
// SDK-level MCPTask type. Keeps the public surface independent of
// internal package types.
func mcpTaskFromInternal(t tools.MCPTaskInfo) MCPTask {
	return MCPTask{
		Server:        t.Server,
		TaskID:        t.TaskID,
		Status:        MCPTaskStatus(t.Status),
		StatusMessage: t.StatusMessage,
		CreatedAt:     t.CreatedAt,
		UpdatedAt:     t.UpdatedAt,
		TTL:           t.TTL,
		PollInterval:  t.PollInterval,
	}
}

// inheritMCPTaskOptions copies every MCP task-related field from parent
// onto child. Used by Kit.Subagent so child instances observe the same
// per-server modes, timeouts, and progress callback as their parent.
// A nil parent is a no-op so callers don't have to guard at the call site.
func inheritMCPTaskOptions(child, parent *Options) {
	if child == nil || parent == nil {
		return
	}
	child.MCPTaskMode = parent.MCPTaskMode
	child.MCPTaskTimeout = parent.MCPTaskTimeout
	child.MCPTaskTTL = parent.MCPTaskTTL
	child.MCPTaskPollInterval = parent.MCPTaskPollInterval
	child.MCPTaskMaxPollInterval = parent.MCPTaskMaxPollInterval
	child.MCPTaskProgress = parent.MCPTaskProgress
}
