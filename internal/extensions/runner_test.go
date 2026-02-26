package extensions

import (
	"testing"
)

// makeRunner builds a Runner with the given extensions for testing.
func makeRunner(exts ...LoadedExtension) *Runner {
	return NewRunner(exts)
}

// makeHandlerExt creates a LoadedExtension with handlers registered for the given events.
func makeHandlerExt(path string, handlers map[EventType][]HandlerFunc) LoadedExtension {
	return LoadedExtension{
		Path:     path,
		Handlers: handlers,
	}
}

func TestRunner_EmitNoHandlers(t *testing.T) {
	r := makeRunner()
	result, err := r.Emit(ToolCallEvent{ToolName: "test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Fatalf("expected nil result, got %v", result)
	}
}

func TestRunner_EmitSequentialOrder(t *testing.T) {
	var order []int
	ext1 := makeHandlerExt("ext1.go", map[EventType][]HandlerFunc{
		SessionStart: {
			func(e Event, c Context) Result { order = append(order, 1); return nil },
		},
	})
	ext2 := makeHandlerExt("ext2.go", map[EventType][]HandlerFunc{
		SessionStart: {
			func(e Event, c Context) Result { order = append(order, 2); return nil },
		},
	})
	ext3 := makeHandlerExt("ext3.go", map[EventType][]HandlerFunc{
		SessionStart: {
			func(e Event, c Context) Result { order = append(order, 3); return nil },
		},
	})

	r := makeRunner(ext1, ext2, ext3)
	_, err := r.Emit(SessionStartEvent{SessionID: "test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(order) != 3 || order[0] != 1 || order[1] != 2 || order[2] != 3 {
		t.Fatalf("expected sequential order [1,2,3], got %v", order)
	}
}

func TestRunner_EmitMultipleHandlersPerExtension(t *testing.T) {
	var calls int
	ext := makeHandlerExt("multi.go", map[EventType][]HandlerFunc{
		SessionStart: {
			func(e Event, c Context) Result { calls++; return nil },
			func(e Event, c Context) Result { calls++; return nil },
		},
	})

	r := makeRunner(ext)
	_, err := r.Emit(SessionStartEvent{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 2 {
		t.Fatalf("expected 2 calls, got %d", calls)
	}
}

func TestRunner_EmitToolCallBlocking(t *testing.T) {
	var secondCalled bool
	ext1 := makeHandlerExt("blocker.go", map[EventType][]HandlerFunc{
		ToolCall: {
			func(e Event, c Context) Result {
				return ToolCallResult{Block: true, Reason: "denied"}
			},
		},
	})
	ext2 := makeHandlerExt("second.go", map[EventType][]HandlerFunc{
		ToolCall: {
			func(e Event, c Context) Result {
				secondCalled = true
				return nil
			},
		},
	})

	r := makeRunner(ext1, ext2)
	result, err := r.Emit(ToolCallEvent{ToolName: "bash", Input: "{}"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if secondCalled {
		t.Error("second handler should not have been called after block")
	}
	tcr, ok := result.(ToolCallResult)
	if !ok {
		t.Fatalf("expected ToolCallResult, got %T", result)
	}
	if !tcr.Block {
		t.Error("expected Block=true")
	}
	if tcr.Reason != "denied" {
		t.Errorf("expected reason 'denied', got %q", tcr.Reason)
	}
}

func TestRunner_EmitToolCallNonBlocking(t *testing.T) {
	ext := makeHandlerExt("allow.go", map[EventType][]HandlerFunc{
		ToolCall: {
			func(e Event, c Context) Result {
				return ToolCallResult{Block: false}
			},
		},
	})

	r := makeRunner(ext)
	result, err := r.Emit(ToolCallEvent{ToolName: "bash"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tcr, ok := result.(ToolCallResult)
	if !ok {
		t.Fatalf("expected ToolCallResult, got %T", result)
	}
	if tcr.Block {
		t.Error("expected Block=false for non-blocking result")
	}
}

func TestRunner_EmitInputBlocking(t *testing.T) {
	ext := makeHandlerExt("input-handler.go", map[EventType][]HandlerFunc{
		Input: {
			func(e Event, c Context) Result {
				return InputResult{Action: "handled"}
			},
		},
	})

	r := makeRunner(ext)
	result, err := r.Emit(InputEvent{Text: "secret", Source: "interactive"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ir, ok := result.(InputResult)
	if !ok {
		t.Fatalf("expected InputResult, got %T", result)
	}
	if ir.Action != "handled" {
		t.Errorf("expected Action 'handled', got %q", ir.Action)
	}
}

func TestRunner_EmitInputTransform(t *testing.T) {
	ext := makeHandlerExt("transform.go", map[EventType][]HandlerFunc{
		Input: {
			func(e Event, c Context) Result {
				ie := e.(InputEvent)
				return InputResult{Action: "transform", Text: ie.Text + " transformed"}
			},
		},
	})

	r := makeRunner(ext)
	result, err := r.Emit(InputEvent{Text: "hello", Source: "cli"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ir, ok := result.(InputResult)
	if !ok {
		t.Fatalf("expected InputResult, got %T", result)
	}
	if ir.Action != "transform" {
		t.Errorf("expected Action 'transform', got %q", ir.Action)
	}
	if ir.Text != "hello transformed" {
		t.Errorf("expected transformed text, got %q", ir.Text)
	}
}

func TestRunner_EmitToolResultChaining(t *testing.T) {
	modified := "modified content"
	ext := makeHandlerExt("modifier.go", map[EventType][]HandlerFunc{
		ToolResult: {
			func(e Event, c Context) Result {
				return ToolResultResult{Content: &modified}
			},
		},
	})

	r := makeRunner(ext)
	result, err := r.Emit(ToolResultEvent{ToolName: "read", Content: "original"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	trr, ok := result.(ToolResultResult)
	if !ok {
		t.Fatalf("expected ToolResultResult, got %T", result)
	}
	if trr.Content == nil || *trr.Content != "modified content" {
		t.Error("expected content to be modified")
	}
}

func TestRunner_EmitPanicRecovery(t *testing.T) {
	var secondCalled bool
	ext := makeHandlerExt("panicker.go", map[EventType][]HandlerFunc{
		SessionStart: {
			func(e Event, c Context) Result { panic("boom") },
			func(e Event, c Context) Result { secondCalled = true; return nil },
		},
	})

	r := makeRunner(ext)
	result, err := r.Emit(SessionStartEvent{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// After a panic, the runner should continue to the next handler.
	if !secondCalled {
		t.Error("second handler should still be called after panic in first")
	}
	if result != nil {
		t.Errorf("expected nil result, got %v", result)
	}
}

func TestRunner_EmitEventPassedCorrectly(t *testing.T) {
	var receivedName string
	var receivedInput string
	ext := makeHandlerExt("inspect.go", map[EventType][]HandlerFunc{
		ToolCall: {
			func(e Event, c Context) Result {
				tc := e.(ToolCallEvent)
				receivedName = tc.ToolName
				receivedInput = tc.Input
				return nil
			},
		},
	})

	r := makeRunner(ext)
	_, _ = r.Emit(ToolCallEvent{ToolName: "bash", ToolCallID: "123", Input: `{"cmd":"ls"}`})
	if receivedName != "bash" {
		t.Errorf("expected tool name 'bash', got %q", receivedName)
	}
	if receivedInput != `{"cmd":"ls"}` {
		t.Errorf("expected input '{\"cmd\":\"ls\"}', got %q", receivedInput)
	}
}

func TestRunner_SetContext(t *testing.T) {
	var receivedCtx Context
	ext := makeHandlerExt("ctx.go", map[EventType][]HandlerFunc{
		SessionStart: {
			func(e Event, c Context) Result {
				receivedCtx = c
				return nil
			},
		},
	})

	r := makeRunner(ext)
	r.SetContext(Context{
		SessionID:   "sess-123",
		CWD:         "/tmp",
		Model:       "claude-4",
		Interactive: true,
	})

	_, _ = r.Emit(SessionStartEvent{})
	if receivedCtx.SessionID != "sess-123" {
		t.Errorf("expected SessionID 'sess-123', got %q", receivedCtx.SessionID)
	}
	if receivedCtx.CWD != "/tmp" {
		t.Errorf("expected CWD '/tmp', got %q", receivedCtx.CWD)
	}
	if receivedCtx.Model != "claude-4" {
		t.Errorf("expected Model 'claude-4', got %q", receivedCtx.Model)
	}
	if !receivedCtx.Interactive {
		t.Error("expected Interactive=true")
	}
}

func TestRunner_HasHandlers(t *testing.T) {
	ext := makeHandlerExt("test.go", map[EventType][]HandlerFunc{
		ToolCall: {
			func(e Event, c Context) Result { return nil },
		},
	})

	r := makeRunner(ext)
	if !r.HasHandlers(ToolCall) {
		t.Error("expected HasHandlers(ToolCall) = true")
	}
	if r.HasHandlers(SessionStart) {
		t.Error("expected HasHandlers(SessionStart) = false")
	}
}

func TestRunner_RegisteredTools(t *testing.T) {
	ext := LoadedExtension{
		Path:     "tools.go",
		Handlers: make(map[EventType][]HandlerFunc),
		Tools: []ToolDef{
			{Name: "tool1", Description: "first"},
			{Name: "tool2", Description: "second"},
		},
	}

	r := makeRunner(ext)
	tools := r.RegisteredTools()
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(tools))
	}
	if tools[0].Name != "tool1" || tools[1].Name != "tool2" {
		t.Error("tools not returned in expected order")
	}
}

func TestRunner_RegisteredCommands(t *testing.T) {
	ext := LoadedExtension{
		Path:     "cmds.go",
		Handlers: make(map[EventType][]HandlerFunc),
		Commands: []CommandDef{
			{Name: "cmd1", Description: "first"},
		},
	}

	r := makeRunner(ext)
	cmds := r.RegisteredCommands()
	if len(cmds) != 1 {
		t.Fatalf("expected 1 command, got %d", len(cmds))
	}
	if cmds[0].Name != "cmd1" {
		t.Errorf("expected command name 'cmd1', got %q", cmds[0].Name)
	}
}

func TestRunner_Extensions(t *testing.T) {
	ext1 := makeHandlerExt("a.go", map[EventType][]HandlerFunc{})
	ext2 := makeHandlerExt("b.go", map[EventType][]HandlerFunc{})
	r := makeRunner(ext1, ext2)
	if len(r.Extensions()) != 2 {
		t.Fatalf("expected 2 extensions, got %d", len(r.Extensions()))
	}
}

func TestRunner_EmitOnlyMatchingEvent(t *testing.T) {
	var called bool
	ext := makeHandlerExt("mismatch.go", map[EventType][]HandlerFunc{
		ToolCall: {
			func(e Event, c Context) Result { called = true; return nil },
		},
	})

	r := makeRunner(ext)
	_, _ = r.Emit(SessionStartEvent{}) // different event type
	if called {
		t.Error("ToolCall handler should not be called for SessionStart event")
	}
}

func TestRunner_EmitBeforeAgentStartResult(t *testing.T) {
	injected := "extra context"
	ext := makeHandlerExt("inject.go", map[EventType][]HandlerFunc{
		BeforeAgentStart: {
			func(e Event, c Context) Result {
				return BeforeAgentStartResult{InjectText: &injected}
			},
		},
	})

	r := makeRunner(ext)
	result, err := r.Emit(BeforeAgentStartEvent{Prompt: "hello"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	bar, ok := result.(BeforeAgentStartResult)
	if !ok {
		t.Fatalf("expected BeforeAgentStartResult, got %T", result)
	}
	if bar.InjectText == nil || *bar.InjectText != "extra context" {
		t.Error("expected InjectText to be set")
	}
}

func TestRunner_LastResultWins(t *testing.T) {
	// When multiple handlers return non-nil, non-blocking results,
	// the last one should be returned (accumulated).
	first := "first"
	second := "second"
	ext := makeHandlerExt("chain.go", map[EventType][]HandlerFunc{
		ToolResult: {
			func(e Event, c Context) Result {
				return ToolResultResult{Content: &first}
			},
			func(e Event, c Context) Result {
				return ToolResultResult{Content: &second}
			},
		},
	})

	r := makeRunner(ext)
	result, _ := r.Emit(ToolResultEvent{ToolName: "test", Content: "orig"})
	trr := result.(ToolResultResult)
	if trr.Content == nil || *trr.Content != "second" {
		t.Errorf("expected last result to win, got %v", trr.Content)
	}
}

func TestRunner_ContextPrint(t *testing.T) {
	var printed []string
	var receivedCtx Context
	ext := makeHandlerExt("print.go", map[EventType][]HandlerFunc{
		Input: {
			func(e Event, c Context) Result {
				receivedCtx = c
				if c.Print != nil {
					c.Print("hello from extension")
				}
				return nil
			},
		},
	})

	r := makeRunner(ext)
	r.SetContext(Context{
		Print: func(text string) {
			printed = append(printed, text)
		},
	})

	_, _ = r.Emit(InputEvent{Text: "test"})
	if receivedCtx.Print == nil {
		t.Fatal("expected Print to be non-nil in context")
	}
	if len(printed) != 1 || printed[0] != "hello from extension" {
		t.Errorf("expected Print to capture 'hello from extension', got %v", printed)
	}
}

func TestRunner_ContextPrintInfo(t *testing.T) {
	var infos []string
	ext := makeHandlerExt("info.go", map[EventType][]HandlerFunc{
		SessionStart: {
			func(e Event, c Context) Result {
				if c.PrintInfo != nil {
					c.PrintInfo("extension loaded successfully")
				}
				return nil
			},
		},
	})

	r := makeRunner(ext)
	r.SetContext(Context{
		PrintInfo: func(text string) {
			infos = append(infos, text)
		},
	})

	_, _ = r.Emit(SessionStartEvent{})
	if len(infos) != 1 || infos[0] != "extension loaded successfully" {
		t.Errorf("expected PrintInfo to capture message, got %v", infos)
	}
}

func TestRunner_ContextPrintError(t *testing.T) {
	var errors []string
	ext := makeHandlerExt("err.go", map[EventType][]HandlerFunc{
		ToolResult: {
			func(e Event, c Context) Result {
				tr := e.(ToolResultEvent)
				if tr.IsError && c.PrintError != nil {
					c.PrintError("tool failed: " + tr.ToolName)
				}
				return nil
			},
		},
	})

	r := makeRunner(ext)
	r.SetContext(Context{
		PrintError: func(text string) {
			errors = append(errors, text)
		},
	})

	_, _ = r.Emit(ToolResultEvent{ToolName: "bash", IsError: true, Content: "exit 1"})
	if len(errors) != 1 || errors[0] != "tool failed: bash" {
		t.Errorf("expected PrintError to capture message, got %v", errors)
	}
}

func TestRunner_ContextPrintBlock(t *testing.T) {
	var captured []PrintBlockOpts
	ext := makeHandlerExt("block.go", map[EventType][]HandlerFunc{
		Input: {
			func(e Event, c Context) Result {
				if c.PrintBlock != nil {
					c.PrintBlock(PrintBlockOpts{
						Text:        "deploy complete",
						BorderColor: "#a6e3a1",
						Subtitle:    "deploy-ext",
					})
				}
				return InputResult{Action: "handled"}
			},
		},
	})

	r := makeRunner(ext)
	r.SetContext(Context{
		PrintBlock: func(opts PrintBlockOpts) {
			captured = append(captured, opts)
		},
	})

	_, _ = r.Emit(InputEvent{Text: "!deploy"})
	if len(captured) != 1 {
		t.Fatalf("expected 1 PrintBlock call, got %d", len(captured))
	}
	if captured[0].Text != "deploy complete" {
		t.Errorf("expected text 'deploy complete', got %q", captured[0].Text)
	}
	if captured[0].BorderColor != "#a6e3a1" {
		t.Errorf("expected border '#a6e3a1', got %q", captured[0].BorderColor)
	}
	if captured[0].Subtitle != "deploy-ext" {
		t.Errorf("expected subtitle 'deploy-ext', got %q", captured[0].Subtitle)
	}
}

func TestRunner_ContextPrintNilSafe(t *testing.T) {
	// When Print/PrintInfo/PrintError/PrintBlock are not set (nil), guarded calls should not panic.
	ext := makeHandlerExt("nilprint.go", map[EventType][]HandlerFunc{
		Input: {
			func(e Event, c Context) Result {
				if c.Print != nil {
					c.Print("should not happen")
				}
				if c.PrintInfo != nil {
					c.PrintInfo("should not happen")
				}
				if c.PrintError != nil {
					c.PrintError("should not happen")
				}
				if c.PrintBlock != nil {
					c.PrintBlock(PrintBlockOpts{Text: "nope"})
				}
				return nil
			},
		},
	})

	r := makeRunner(ext)
	// Context without any Print functions set.
	r.SetContext(Context{Model: "test"})
	_, err := r.Emit(InputEvent{Text: "test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
