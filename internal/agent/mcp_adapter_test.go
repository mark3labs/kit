package agent

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"charm.land/fantasy"

	"github.com/mark3labs/kit/internal/tools"
)

// stubExecutor lets each test script the (result, err) pair returned by
// ExecuteTool. The adapter holds an mcpExecutor interface, so this is the
// only seam the tests need.
type stubExecutor struct {
	result *tools.MCPToolResult
	err    error
	// called records the last invocation for assertion.
	called bool
	name   string
	input  string
}

func (s *stubExecutor) ExecuteTool(_ context.Context, prefixedName, inputJSON string) (*tools.MCPToolResult, error) {
	s.called = true
	s.name = prefixedName
	s.input = inputJSON
	return s.result, s.err
}

func newMCPAgentTool(exec mcpExecutor, name string) *mcpAgentTool {
	return &mcpAgentTool{
		tool: tools.MCPTool{Name: name},
		exec: exec,
	}
}

// Manager-side Go errors (JSON-RPC protocol errors, transport failures,
// schema validation rejections from the MCP server) must be surfaced to
// the model as soft tool errors so the agent loop can keep going. Aborting
// the turn would discard all prior tool results — see issue #N.
func TestMCPAgentTool_RPCErrorBecomesSoftError(t *testing.T) {
	exec := &stubExecutor{
		err: errors.New("MCP error -32602: Invalid params: missing field \"task\""),
	}
	tool := newMCPAgentTool(exec, "pubmed__search")

	resp, err := tool.Run(context.Background(), fantasy.ToolCall{
		ID:    "call-1",
		Name:  "pubmed__search",
		Input: `{"query":"foo"}`,
	})

	if err != nil {
		t.Fatalf("expected nil error (soft), got %v", err)
	}
	if !resp.IsError {
		t.Fatalf("expected IsError=true, got false")
	}
	if !strings.Contains(resp.Content, "pubmed__search") {
		t.Errorf("expected tool name in error content, got %q", resp.Content)
	}
	if !strings.Contains(resp.Content, "-32602") {
		t.Errorf("expected underlying error text in content, got %q", resp.Content)
	}
}

// Context cancellation is the one error that must remain critical: it
// means the caller intentionally aborted, and the agent loop needs to
// unwind cleanly rather than burning more steps.
func TestMCPAgentTool_CtxCancelStaysCritical(t *testing.T) {
	exec := &stubExecutor{
		// Real managers typically return ctx.Err() (or a wrapper) when the
		// context is cancelled mid-call.
		err: context.Canceled,
	}
	tool := newMCPAgentTool(exec, "slow__tool")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	resp, err := tool.Run(ctx, fantasy.ToolCall{Name: "slow__tool"})

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if resp.IsError || resp.Content != "" {
		t.Errorf("expected empty response on critical error, got IsError=%v Content=%q", resp.IsError, resp.Content)
	}
}

// Deadline-exceeded behaves the same as cancellation: ctx.Err() is
// non-nil, so the adapter must propagate the critical error rather than
// converting the executor's error into a soft response.
func TestMCPAgentTool_CtxDeadlineStaysCritical(t *testing.T) {
	exec := &stubExecutor{err: context.DeadlineExceeded}
	tool := newMCPAgentTool(exec, "slow__tool")

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()

	resp, err := tool.Run(ctx, fantasy.ToolCall{Name: "slow__tool"})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context.DeadlineExceeded, got %v", err)
	}
	if resp.IsError || resp.Content != "" {
		t.Errorf("expected empty response on critical error, got IsError=%v Content=%q", resp.IsError, resp.Content)
	}
}

// Server-side soft errors (CallToolResult{ isError: true }) must continue
// to flow through as soft errors — this was the existing behavior and
// must not regress.
func TestMCPAgentTool_ServerIsErrorRemainsSoftError(t *testing.T) {
	exec := &stubExecutor{
		result: &tools.MCPToolResult{
			IsError: true,
			Content: "search service is rate limited; try again in 30s",
		},
	}
	tool := newMCPAgentTool(exec, "pubmed__search")

	resp, err := tool.Run(context.Background(), fantasy.ToolCall{Name: "pubmed__search"})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !resp.IsError {
		t.Fatalf("expected IsError=true, got false")
	}
	if resp.Content != "search service is rate limited; try again in 30s" {
		t.Errorf("expected pass-through content, got %q", resp.Content)
	}
}

// Happy path: ordinary successful tool result is passed through unchanged.
func TestMCPAgentTool_SuccessIsPassthrough(t *testing.T) {
	exec := &stubExecutor{
		result: &tools.MCPToolResult{
			IsError: false,
			Content: `{"hits":3}`,
		},
	}
	tool := newMCPAgentTool(exec, "pubmed__search")

	resp, err := tool.Run(context.Background(), fantasy.ToolCall{Name: "pubmed__search"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.IsError {
		t.Fatalf("expected IsError=false")
	}
	if resp.Content != `{"hits":3}` {
		t.Errorf("expected pass-through content, got %q", resp.Content)
	}
}
