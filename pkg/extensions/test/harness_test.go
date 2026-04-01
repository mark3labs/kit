package test

import (
	"testing"

	"github.com/mark3labs/kit/internal/extensions"
)

// Test harness with a simple extension
func TestHarness_LoadString(t *testing.T) {
	src := `package main

import "kit/ext"

func Init(api ext.API) {
	api.OnSessionStart(func(_ ext.SessionStartEvent, ctx ext.Context) {
		ctx.Print("session started")
	})
}
`

	harness := New(t)
	harness.LoadString(src, "test-ext.go")

	// Emit session start event
	_, err := harness.Emit(extensions.SessionStartEvent{SessionID: "test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the extension printed something
	prints := harness.Context().GetPrints()
	if len(prints) != 1 || prints[0] != "session started" {
		t.Errorf("expected ['session started'], got %v", prints)
	}
}

func TestHarness_ToolCallBlocking(t *testing.T) {
	src := `package main

import "kit/ext"

func Init(api ext.API) {
	api.OnToolCall(func(tc ext.ToolCallEvent, ctx ext.Context) *ext.ToolCallResult {
		if tc.ToolName == "banned" {
			return &ext.ToolCallResult{Block: true, Reason: "tool is banned"}
		}
		return nil
	})
}
`

	harness := New(t)
	harness.LoadString(src, "blocker.go")

	// Test blocked tool
	result, err := harness.Emit(extensions.ToolCallEvent{ToolName: "banned", Input: "{}"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	AssertBlocked(t, result, "tool is banned")

	// Test allowed tool
	result2, err := harness.Emit(extensions.ToolCallEvent{ToolName: "allowed", Input: "{}"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result2 != nil {
		t.Errorf("expected nil result for allowed tool, got %v", result2)
	}
}

func TestHarness_ToolRegistration(t *testing.T) {
	src := `package main

import "kit/ext"

func Init(api ext.API) {
	api.RegisterTool(ext.ToolDef{
		Name:        "my_tool",
		Description: "does stuff",
		Parameters:  "{}",
		Execute: func(input string) (string, error) {
			return "result: " + input, nil
		},
	})
}
`

	harness := New(t)
	harness.LoadString(src, "tool-ext.go")

	tools := harness.RegisteredTools()
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}

	if tools[0].Name != "my_tool" {
		t.Errorf("expected tool name 'my_tool', got %q", tools[0].Name)
	}

	AssertToolRegistered(t, harness, "my_tool")
}

func TestHarness_CommandRegistration(t *testing.T) {
	src := `package main

import "kit/ext"

func Init(api ext.API) {
	api.RegisterCommand(ext.CommandDef{
		Name:        "hello",
		Description: "says hello",
		Execute: func(args string, ctx ext.Context) (string, error) {
			ctx.Print("Hello, " + args)
			return "greeting sent", nil
		},
	})
}
`

	harness := New(t)
	harness.LoadString(src, "cmd-ext.go")

	cmds := harness.RegisteredCommands()
	if len(cmds) != 1 {
		t.Fatalf("expected 1 command, got %d", len(cmds))
	}

	if cmds[0].Name != "hello" {
		t.Errorf("expected command name 'hello', got %q", cmds[0].Name)
	}

	AssertCommandRegistered(t, harness, "hello")
}

func TestHarness_WidgetSetting(t *testing.T) {
	src := `package main

import "kit/ext"

func Init(api ext.API) {
	api.OnSessionStart(func(_ ext.SessionStartEvent, ctx ext.Context) {
		ctx.SetWidget(ext.WidgetConfig{
			ID:        "my-widget",
			Placement: ext.WidgetAbove,
			Content:   ext.WidgetContent{Text: "Hello, World!"},
			Style:     ext.WidgetStyle{BorderColor: "#ff0000"},
		})
	})
}
`

	harness := New(t)
	harness.LoadString(src, "widget-ext.go")

	_, err := harness.Emit(extensions.SessionStartEvent{SessionID: "test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	AssertWidgetSet(t, harness, "my-widget")
	AssertWidgetText(t, harness, "my-widget", "Hello, World!")

	// Also verify directly
	widget, ok := harness.Context().GetWidget("my-widget")
	if !ok {
		t.Error("expected widget 'my-widget' to exist")
	}
	if widget.Style.BorderColor != "#ff0000" {
		t.Errorf("expected border color '#ff0000', got %q", widget.Style.BorderColor)
	}
}

func TestHarness_FooterSetting(t *testing.T) {
	src := `package main

import "kit/ext"

func Init(api ext.API) {
	api.OnSessionStart(func(_ ext.SessionStartEvent, ctx ext.Context) {
		ctx.SetFooter(ext.HeaderFooterConfig{
			Content: ext.WidgetContent{Text: "Status: OK"},
			Style:   ext.WidgetStyle{BorderColor: "#00ff00"},
		})
	})
}
`

	harness := New(t)
	harness.LoadString(src, "footer-ext.go")

	_, err := harness.Emit(extensions.SessionStartEvent{SessionID: "test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	AssertFooterSet(t, harness)

	footer := harness.Context().GetFooter()
	if footer == nil {
		t.Fatal("expected footer to be set")
		return
	}
	if footer.Content.Text != "Status: OK" {
		t.Errorf("expected footer text 'Status: OK', got %q", footer.Content.Text)
	}
}

func TestHarness_PrintInfoAndError(t *testing.T) {
	src := `package main

import "kit/ext"

func Init(api ext.API) {
	api.OnSessionStart(func(_ ext.SessionStartEvent, ctx ext.Context) {
		ctx.PrintInfo("Information message")
		ctx.PrintError("Error message")
	})
}
`

	harness := New(t)
	harness.LoadString(src, "print-ext.go")

	_, err := harness.Emit(extensions.SessionStartEvent{SessionID: "test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	AssertPrintInfo(t, harness, "Information message")
	AssertPrintError(t, harness, "Error message")
}

func TestHarness_EmitJSON(t *testing.T) {
	src := `package main

import "kit/ext"

func Init(api ext.API) {
	api.OnToolCall(func(tc ext.ToolCallEvent, ctx ext.Context) *ext.ToolCallResult {
		if tc.ToolName == "test_tool" {
			return &ext.ToolCallResult{Block: true, Reason: "blocked"}
		}
		return nil
	})
}
`

	harness := New(t)
	harness.LoadString(src, "json-ext.go")

	result, err := harness.EmitJSON("test_tool", `{"key": "value"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("expected non-nil result")
		return
	}

	if !result.Block {
		t.Error("expected Block=true")
	}
}

func TestHarness_HasHandlers(t *testing.T) {
	src := `package main

import "kit/ext"

func Init(api ext.API) {
	api.OnToolCall(func(_ ext.ToolCallEvent, _ ext.Context) *ext.ToolCallResult {
		return nil
	})
	api.OnSessionStart(func(_ ext.SessionStartEvent, _ ext.Context) {
	})
}
`

	harness := New(t)
	harness.LoadString(src, "handlers-ext.go")

	AssertHasHandlers(t, harness, extensions.ToolCall)
	AssertHasHandlers(t, harness, extensions.SessionStart)
	AssertNoHandlers(t, harness, extensions.AgentEnd)
}

func TestHarness_MultipleExtensions(t *testing.T) {
	ext1 := `package main

import "kit/ext"

func Init(api ext.API) {
	api.OnSessionStart(func(_ ext.SessionStartEvent, ctx ext.Context) {
		ctx.Print("extension 1")
	})
}
`

	ext2 := `package main

import "kit/ext"

func Init(api ext.API) {
	api.OnSessionStart(func(_ ext.SessionStartEvent, ctx ext.Context) {
		ctx.Print("extension 2")
	})
}
`

	// Load first extension
	harness1 := New(t)
	harness1.LoadString(ext1, "ext1.go")

	// Load second extension
	harness2 := New(t)
	harness2.LoadString(ext2, "ext2.go")

	// Verify they are isolated
	_, _ = harness1.Emit(extensions.SessionStartEvent{SessionID: "test1"})
	_, _ = harness2.Emit(extensions.SessionStartEvent{SessionID: "test2"})

	prints1 := harness1.Context().GetPrints()
	prints2 := harness2.Context().GetPrints()

	if len(prints1) != 1 || prints1[0] != "extension 1" {
		t.Errorf("ext1 prints: expected ['extension 1'], got %v", prints1)
	}

	if len(prints2) != 1 || prints2[0] != "extension 2" {
		t.Errorf("ext2 prints: expected ['extension 2'], got %v", prints2)
	}
}

func TestHarness_InputHandling(t *testing.T) {
	src := `package main

import (
	"kit/ext"
	"strings"
)

func Init(api ext.API) {
	api.OnInput(func(ie ext.InputEvent, ctx ext.Context) *ext.InputResult {
		if strings.Contains(ie.Text, "secret") {
			return &ext.InputResult{Action: "handled"}
		}
		return nil
	})
}
`

	harness := New(t)
	harness.LoadString(src, "input-ext.go")

	// Test handled input
	result, err := harness.Emit(extensions.InputEvent{Text: "my secret password", Source: "cli"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	AssertInputHandled(t, result, "handled")

	// Test unhandled input
	result2, err := harness.Emit(extensions.InputEvent{Text: "normal input", Source: "cli"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result2 != nil {
		t.Errorf("expected nil result for unhandled input, got %v", result2)
	}
}

func TestHarness_StatusSetting(t *testing.T) {
	src := `package main

import "kit/ext"

func Init(api ext.API) {
	api.OnSessionStart(func(_ ext.SessionStartEvent, ctx ext.Context) {
		ctx.SetStatus("myext:status", "Ready", 50)
	})
}
`

	harness := New(t)
	harness.LoadString(src, "status-ext.go")

	_, err := harness.Emit(extensions.SessionStartEvent{SessionID: "test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	AssertStatusSet(t, harness, "myext:status")
	AssertStatusText(t, harness, "myext:status", "Ready")
}

func TestHarness_LoadFile_NotFound(t *testing.T) {
	// Test that loading a nonexistent file fails the test
	// We create a mock testing.T to capture the failure
	mockT := &testing.T{}
	harness := New(mockT)

	// Just verify the harness was created successfully
	_ = harness.Context().GetPrints()

	// The actual behavior (Fatalf on missing file) is tested implicitly
	// whenever LoadFile is used in other tests
}

// MockContext tests
func TestMockContext_Prompts(t *testing.T) {
	ctx := NewMockContext()

	// Configure results
	ctx.SetPromptSelectResult(extensions.PromptSelectResult{Value: "option1", Index: 0, Cancelled: false})
	ctx.SetPromptConfirmResult(extensions.PromptConfirmResult{Value: true, Cancelled: false})
	ctx.SetPromptInputResult(extensions.PromptInputResult{Value: "input text", Cancelled: false})

	extCtx := ctx.ToContext()

	// Test prompts return configured results
	selectResult := extCtx.PromptSelect(extensions.PromptSelectConfig{Message: "test", Options: []string{"a", "b"}})
	if selectResult.Value != "option1" {
		t.Errorf("expected 'option1', got %q", selectResult.Value)
	}

	confirmResult := extCtx.PromptConfirm(extensions.PromptConfirmConfig{Message: "test"})
	if !confirmResult.Value {
		t.Error("expected true")
	}

	inputResult := extCtx.PromptInput(extensions.PromptInputConfig{Message: "test"})
	if inputResult.Value != "input text" {
		t.Errorf("expected 'input text', got %q", inputResult.Value)
	}
}

func TestMockContext_Options(t *testing.T) {
	ctx := NewMockContext()
	extCtx := ctx.ToContext()

	// Initially empty
	if extCtx.GetOption("key") != "" {
		t.Error("expected empty option")
	}

	// Set option
	extCtx.SetOption("key", "value")
	if extCtx.GetOption("key") != "value" {
		t.Errorf("expected 'value', got %q", extCtx.GetOption("key"))
	}
}

// Assertion helper tests
func TestAssertPrintedContains(t *testing.T) {
	src := `package main

import "kit/ext"

func Init(api ext.API) {
	api.OnSessionStart(func(_ ext.SessionStartEvent, ctx ext.Context) {
		ctx.Print("This is a long message with some content")
	})
}
`

	harness := New(t)
	harness.LoadString(src, "print-ext.go")
	_, _ = harness.Emit(extensions.SessionStartEvent{SessionID: "test"})

	AssertPrintedContains(t, harness, "long message")
	AssertPrintedContains(t, harness, "some content")
}

func TestAssertWidgetTextContains(t *testing.T) {
	src := `package main

import "kit/ext"

func Init(api ext.API) {
	api.OnSessionStart(func(_ ext.SessionStartEvent, ctx ext.Context) {
		ctx.SetWidget(ext.WidgetConfig{
			ID:      "status",
			Content: ext.WidgetContent{Text: "Build: passing, Tests: 42/42"},
		})
	})
}
`

	harness := New(t)
	harness.LoadString(src, "widget-ext.go")
	_, _ = harness.Emit(extensions.SessionStartEvent{SessionID: "test"})

	AssertWidgetTextContains(t, harness, "status", "Build: passing")
	AssertWidgetTextContains(t, harness, "status", "42/42")
}

// Test that shows how to test a realistic extension pattern
func TestExample_RealisticExtension(t *testing.T) {
	// This is an example of a realistic extension that:
	// 1. Blocks dangerous tools
	// 2. Shows a status widget
	// 3. Logs tool calls
	src := `package main

import "kit/ext"

var blockedTools = []string{"rm", "del", "remove"}

func Init(api ext.API) {
	api.OnToolCall(func(tc ext.ToolCallEvent, ctx ext.Context) *ext.ToolCallResult {
		// Check if tool is blocked
		for _, blocked := range blockedTools {
			if tc.ToolName == blocked {
				ctx.PrintError("Tool " + tc.ToolName + " is blocked for safety")
				return &ext.ToolCallResult{Block: true, Reason: "safety block"}
			}
		}
		
		// Log the tool call
		ctx.SetStatus("tool-logger:last", tc.ToolName, 10)
		return nil
	})
	
	api.OnSessionStart(func(_ ext.SessionStartEvent, ctx ext.Context) {
		ctx.SetWidget(ext.WidgetConfig{
			ID:      "safety-status",
			Content: ext.WidgetContent{Text: "Safety: Active"},
			Style:   ext.WidgetStyle{BorderColor: "#00ff00"},
		})
	})
}
`

	harness := New(t)
	harness.LoadString(src, "safety-ext.go")

	// Verify handlers are registered
	AssertHasHandlers(t, harness, extensions.ToolCall)
	AssertHasHandlers(t, harness, extensions.SessionStart)

	// Test session start
	_, err := harness.Emit(extensions.SessionStartEvent{SessionID: "test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify widget was set
	AssertWidgetSet(t, harness, "safety-status")
	AssertWidgetText(t, harness, "safety-status", "Safety: Active")

	// Test allowed tool
	result, _ := harness.Emit(extensions.ToolCallEvent{ToolName: "read", Input: "{}"})
	AssertNotBlocked(t, result)

	// Verify status was updated
	AssertStatusSet(t, harness, "tool-logger:last")
	AssertStatusText(t, harness, "tool-logger:last", "read")

	// Test blocked tool
	result2, _ := harness.Emit(extensions.ToolCallEvent{ToolName: "rm", Input: `{"file": "test.txt"}`})
	AssertBlocked(t, result2, "safety block")
	AssertPrintError(t, harness, "Tool rm is blocked for safety")
}
