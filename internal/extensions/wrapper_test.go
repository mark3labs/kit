package extensions

import (
	"context"
	"testing"

	"charm.land/fantasy"
)

// mockTool implements fantasy.AgentTool for testing.
type mockTool struct {
	name    string
	runFn   func(ctx context.Context, call fantasy.ToolCall) (fantasy.ToolResponse, error)
	provOpt fantasy.ProviderOptions
}

func (m *mockTool) Info() fantasy.ToolInfo {
	return fantasy.ToolInfo{Name: m.name, Description: "mock tool"}
}
func (m *mockTool) ProviderOptions() fantasy.ProviderOptions     { return m.provOpt }
func (m *mockTool) SetProviderOptions(o fantasy.ProviderOptions) { m.provOpt = o }
func (m *mockTool) Run(ctx context.Context, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
	if m.runFn != nil {
		return m.runFn(ctx, call)
	}
	return fantasy.NewTextResponse("ok"), nil
}

func newMockTool(name string) *mockTool {
	return &mockTool{name: name}
}

func TestWrapToolsWithExtensions_NilRunner(t *testing.T) {
	tools := []fantasy.AgentTool{newMockTool("test")}
	result := WrapToolsWithExtensions(tools, nil)
	if len(result) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(result))
	}
	// Should be the same pointer (unwrapped).
	if result[0] != tools[0] {
		t.Error("expected original tool when runner is nil")
	}
}

func TestWrapToolsWithExtensions_NoRelevantHandlers(t *testing.T) {
	r := makeRunner(makeHandlerExt("other.go", map[EventType][]HandlerFunc{
		SessionStart: {func(e Event, c Context) Result { return nil }},
	}))
	tools := []fantasy.AgentTool{newMockTool("test")}
	result := WrapToolsWithExtensions(tools, r)
	if result[0] != tools[0] {
		t.Error("expected original tool when no tool handlers exist")
	}
}

func TestWrapToolsWithExtensions_WrapsWhenHandlersExist(t *testing.T) {
	r := makeRunner(makeHandlerExt("tc.go", map[EventType][]HandlerFunc{
		ToolCall: {func(e Event, c Context) Result { return nil }},
	}))
	tools := []fantasy.AgentTool{newMockTool("test")}
	result := WrapToolsWithExtensions(tools, r)
	if result[0] == tools[0] {
		t.Error("expected wrapped tool when ToolCall handlers exist")
	}
	// Verify Info() is passed through.
	if result[0].Info().Name != "test" {
		t.Errorf("expected name 'test', got %q", result[0].Info().Name)
	}
}

func TestWrappedTool_NormalExecution(t *testing.T) {
	var toolCallSeen, toolResultSeen bool
	r := makeRunner(makeHandlerExt("observe.go", map[EventType][]HandlerFunc{
		ToolCall: {func(e Event, c Context) Result {
			toolCallSeen = true
			return nil
		}},
		ToolResult: {func(e Event, c Context) Result {
			toolResultSeen = true
			return nil
		}},
	}))

	mock := newMockTool("bash")
	mock.runFn = func(_ context.Context, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
		return fantasy.NewTextResponse("output"), nil
	}

	tools := WrapToolsWithExtensions([]fantasy.AgentTool{mock}, r)
	resp, err := tools[0].Run(context.Background(), fantasy.ToolCall{ID: "1", Input: "{}"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "output" {
		t.Errorf("expected 'output', got %q", resp.Content)
	}
	if !toolCallSeen {
		t.Error("ToolCall handler was not invoked")
	}
	if !toolResultSeen {
		t.Error("ToolResult handler was not invoked")
	}
}

func TestWrappedTool_BlockExecution(t *testing.T) {
	var toolRan bool
	r := makeRunner(makeHandlerExt("blocker.go", map[EventType][]HandlerFunc{
		ToolCall: {func(e Event, c Context) Result {
			return ToolCallResult{Block: true, Reason: "forbidden"}
		}},
	}))

	mock := newMockTool("danger")
	mock.runFn = func(_ context.Context, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
		toolRan = true
		return fantasy.NewTextResponse("bad"), nil
	}

	tools := WrapToolsWithExtensions([]fantasy.AgentTool{mock}, r)
	resp, err := tools[0].Run(context.Background(), fantasy.ToolCall{ID: "1"})
	if toolRan {
		t.Error("tool should not have run after block")
	}
	if err == nil {
		t.Error("expected error from blocked tool")
	}
	if resp.IsError != true {
		t.Error("expected IsError=true from blocked response")
	}
}

func TestWrappedTool_ModifyResult(t *testing.T) {
	modified := "redacted"
	r := makeRunner(makeHandlerExt("redactor.go", map[EventType][]HandlerFunc{
		ToolCall: {func(e Event, c Context) Result { return nil }},
		ToolResult: {func(e Event, c Context) Result {
			return ToolResultResult{Content: &modified}
		}},
	}))

	mock := newMockTool("read")
	mock.runFn = func(_ context.Context, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
		return fantasy.NewTextResponse("secret data"), nil
	}

	tools := WrapToolsWithExtensions([]fantasy.AgentTool{mock}, r)
	resp, err := tools[0].Run(context.Background(), fantasy.ToolCall{ID: "1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "redacted" {
		t.Errorf("expected 'redacted', got %q", resp.Content)
	}
}

func TestWrappedTool_ExecutionStartEnd(t *testing.T) {
	var startSeen, endSeen bool
	r := makeRunner(makeHandlerExt("lifecycle.go", map[EventType][]HandlerFunc{
		ToolCall:           {func(e Event, c Context) Result { return nil }},
		ToolExecutionStart: {func(e Event, c Context) Result { startSeen = true; return nil }},
		ToolExecutionEnd:   {func(e Event, c Context) Result { endSeen = true; return nil }},
	}))

	tools := WrapToolsWithExtensions([]fantasy.AgentTool{newMockTool("test")}, r)
	_, _ = tools[0].Run(context.Background(), fantasy.ToolCall{ID: "1"})
	if !startSeen {
		t.Error("ToolExecutionStart not emitted")
	}
	if !endSeen {
		t.Error("ToolExecutionEnd not emitted")
	}
}

func TestExtensionToolsAsFantasy(t *testing.T) {
	defs := []ToolDef{
		{
			Name:        "greet",
			Description: "greets someone",
			Parameters:  `{"type":"object"}`,
			Execute:     func(input string) (string, error) { return "hello " + input, nil },
		},
	}

	tools := ExtensionToolsAsFantasy(defs)
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}

	info := tools[0].Info()
	if info.Name != "greet" {
		t.Errorf("expected name 'greet', got %q", info.Name)
	}
	if info.Description != "greets someone" {
		t.Errorf("expected description 'greets someone', got %q", info.Description)
	}

	resp, err := tools[0].Run(context.Background(), fantasy.ToolCall{Input: "world"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "hello world" {
		t.Errorf("expected 'hello world', got %q", resp.Content)
	}
}

func TestExtensionTool_Error(t *testing.T) {
	defs := []ToolDef{
		{
			Name:    "fail",
			Execute: func(input string) (string, error) { return "", context.DeadlineExceeded },
		},
	}

	tools := ExtensionToolsAsFantasy(defs)
	resp, err := tools[0].Run(context.Background(), fantasy.ToolCall{Input: "x"})
	if err == nil {
		t.Error("expected error")
	}
	if !resp.IsError {
		t.Error("expected IsError=true")
	}
}

func TestExtensionTool_ProviderOptions(t *testing.T) {
	defs := []ToolDef{{Name: "test", Execute: func(string) (string, error) { return "", nil }}}
	tools := ExtensionToolsAsFantasy(defs)

	// Initially nil.
	opts := tools[0].ProviderOptions()
	if opts != nil {
		t.Error("expected nil ProviderOptions initially")
	}

	// SetProviderOptions round-trips.
	po := fantasy.ProviderOptions{}
	tools[0].SetProviderOptions(po)
	got := tools[0].ProviderOptions()
	if got == nil {
		t.Error("expected non-nil ProviderOptions after set")
	}
}
