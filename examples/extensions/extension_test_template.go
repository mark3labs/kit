// Extension Test Template
//
// This is a template for writing tests for your Kit extension.
// Copy this file to your extension directory, rename it to something like
// "my-ext_test.go", and customize it for your extension.
//
// Run tests with: go test -v
//
// IMPORTANT: This file should be in the same directory as your extension
// and use package main, NOT package test.

package main

import (
	"testing"

	"github.com/mark3labs/kit/internal/extensions"
	"github.com/mark3labs/kit/pkg/extensions/test"
)

// Test that your extension loads without errors
func TestExtension_Loads(t *testing.T) {
	harness := test.New(t)
	ext := harness.LoadFile("my-ext.go") // Change to your extension filename

	// Verify the extension was loaded
	if ext == nil {
		t.Fatal("extension should not be nil")
	}
}

// Test your event handlers are registered
func TestExtension_EventHandlers(t *testing.T) {
	harness := test.New(t)
	harness.LoadFile("my-ext.go")

	// Uncomment the handlers your extension uses:
	// test.AssertHasHandlers(t, harness, extensions.ToolCall)
	// test.AssertHasHandlers(t, harness, extensions.Input)
	// test.AssertHasHandlers(t, harness, extensions.SessionStart)
	// test.AssertHasHandlers(t, harness, extensions.AgentEnd)
}

// Test tool registration
func TestExtension_Tools(t *testing.T) {
	harness := test.New(t)
	harness.LoadFile("my-ext.go")

	// Test that your tools are registered
	// test.AssertToolRegistered(t, harness, "my_tool")

	// Or test all registered tools
	tools := harness.RegisteredTools()
	t.Logf("Registered %d tools", len(tools))
	for _, tool := range tools {
		t.Logf("  - %s: %s", tool.Name, tool.Description)
	}
}

// Test command registration
func TestExtension_Commands(t *testing.T) {
	harness := test.New(t)
	harness.LoadFile("my-ext.go")

	// Test that your commands are registered
	// test.AssertCommandRegistered(t, harness, "mycommand")

	// Or test all registered commands
	cmds := harness.RegisteredCommands()
	t.Logf("Registered %d commands", len(cmds))
	for _, cmd := range cmds {
		t.Logf("  - %s: %s", cmd.Name, cmd.Description)
	}
}

// Test session start behavior
func TestExtension_SessionStart(t *testing.T) {
	harness := test.New(t)
	harness.LoadFile("my-ext.go")

	// Emit session start event
	_, err := harness.Emit(extensions.SessionStartEvent{
		SessionID: "test-session",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify expected behavior:
	// - Did it print something?
	// test.AssertPrinted(t, harness, "expected output")

	// - Did it set a widget?
	// test.AssertWidgetSet(t, harness, "my-widget")
	// test.AssertWidgetText(t, harness, "my-widget", "expected text")

	// - Did it set the header/footer?
	// test.AssertHeaderSet(t, harness)
	// test.AssertFooterSet(t, harness)

	// - Did it set a status?
	// test.AssertStatusSet(t, harness, "myext:status")
}

// Test tool call handling
func TestExtension_ToolCall(t *testing.T) {
	harness := test.New(t)
	harness.LoadFile("my-ext.go")

	// Test a specific tool call
	result, err := harness.Emit(extensions.ToolCallEvent{
		ToolName: "some_tool",
		Input:    `{"key": "value"}`,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// If your extension blocks certain tools:
	// test.AssertNotBlocked(t, result)
	// OR
	// test.AssertBlocked(t, result, "expected reason")

	// Suppress unused variable warning (remove this when using result)
	_ = result

	// Check for print output
	// test.AssertPrinted(t, harness, "expected message")
}

// Test input handling
func TestExtension_InputHandling(t *testing.T) {
	harness := test.New(t)
	harness.LoadFile("my-ext.go")

	// Test input that should be handled
	result, err := harness.Emit(extensions.InputEvent{
		Text:   "test input",
		Source: "cli",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// If your extension handles/transforms input:
	// test.AssertInputHandled(t, result, "handled")
	// OR
	// test.AssertInputTransformed(t, result, "transformed text")

	// Suppress unused variable warning (remove this when using result)
	_ = result
}

// Test with configured prompt results
func TestExtension_WithPrompts(t *testing.T) {
	harness := test.New(t)
	harness.LoadFile("my-ext.go")

	// Configure what prompts should return
	harness.Context().SetPromptSelectResult(extensions.PromptSelectResult{
		Value:     "option1",
		Index:     0,
		Cancelled: false,
	})

	// Now when your extension calls ctx.PromptSelect(), it gets the configured result
	_, _ = harness.Emit(extensions.SessionStartEvent{SessionID: "test"})

	// Verify behavior based on the selected options
}
