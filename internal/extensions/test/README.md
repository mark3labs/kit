# Testing Kit Extensions

The `github.com/mark3labs/kit/internal/extensions/test` package provides utilities for testing Kit extensions using standard Go testing patterns.

## Overview

Extension tests run outside the Yaegi interpreter but load your extension code into an isolated interpreter instance. This allows you to:

- Test event handlers without running the full Kit TUI
- Verify that your extension registers tools/commands correctly
- Assert that context methods (Print, SetWidget, etc.) are called as expected
- Test blocking and non-blocking event handling

## Installation

The test package is part of the Kit codebase. Import it in your extension tests:

```go
import (
    "testing"
    "github.com/mark3labs/kit/internal/extensions/test"
    "github.com/mark3labs/kit/internal/extensions"
)
```

## Basic Usage

### Testing an Extension File

```go
package main

import (
    "testing"
    "github.com/mark3labs/kit/internal/extensions/test"
    "github.com/mark3labs/kit/internal/extensions"
)

func TestMyExtension(t *testing.T) {
    // Create a test harness
    harness := test.New(t)
    
    // Load your extension
    harness.LoadFile("my-ext.go")
    
    // Emit events and verify behavior
    _, err := harness.Emit(extensions.SessionStartEvent{SessionID: "test"})
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    
    // Verify the extension printed something
    test.AssertPrinted(t, harness, "session started")
}
```

### Testing Inline Extension Code

For quick tests, you can load extension source directly:

```go
func TestToolBlocking(t *testing.T) {
    src := `package main

import "kit/ext"

func Init(api ext.API) {
    api.OnToolCall(func(tc ext.ToolCallEvent, ctx ext.Context) *ext.ToolCallResult {
        if tc.ToolName == "dangerous" {
            return &ext.ToolCallResult{Block: true, Reason: "not allowed"}
        }
        return nil
    })
}
`
    harness := test.New(t)
    harness.LoadString(src, "test-ext.go")
    
    // Test the tool is blocked
    result, _ := harness.Emit(extensions.ToolCallEvent{
        ToolName: "dangerous",
        Input:    "{}",
    })
    
    test.AssertBlocked(t, result, "not allowed")
}
```

## Common Testing Patterns

### Testing Tool Registration

```go
func TestToolRegistration(t *testing.T) {
    harness := test.New(t)
    harness.LoadFile("my-ext.go")
    
    // Verify the tool was registered
    test.AssertToolRegistered(t, harness, "my_tool")
    
    // Or inspect tools directly
    tools := harness.RegisteredTools()
    for _, tool := range tools {
        if tool.Name == "my_tool" {
            t.Logf("Tool description: %s", tool.Description)
        }
    }
}
```

### Testing Command Registration

```go
func TestCommandRegistration(t *testing.T) {
    harness := test.New(t)
    harness.LoadFile("my-ext.go")
    
    test.AssertCommandRegistered(t, harness, "mycommand")
}
```

### Testing Widgets

```go
func TestWidgetBehavior(t *testing.T) {
    harness := test.New(t)
    harness.LoadFile("my-ext.go")
    
    // Trigger the event that creates the widget
    _, _ = harness.Emit(extensions.SessionStartEvent{SessionID: "test"})
    
    // Verify the widget was set
    test.AssertWidgetSet(t, harness, "my-widget")
    
    // Verify specific widget content
    test.AssertWidgetText(t, harness, "my-widget", "Expected Text")
    
    // Or verify partial content
    test.AssertWidgetTextContains(t, harness, "my-widget", "partial")
}
```

### Testing Input Handling

```go
func TestInputHandling(t *testing.T) {
    harness := test.New(t)
    harness.LoadFile("my-ext.go")
    
    // Test that the extension handles certain input
    result, _ := harness.Emit(extensions.InputEvent{
        Text:   "secret password",
        Source: "cli",
    })
    
    test.AssertInputHandled(t, result, "handled")
}
```

### Testing Print Functions

```go
func TestPrintOutput(t *testing.T) {
    harness := test.New(t)
    harness.LoadFile("my-ext.go")
    
    _, _ = harness.Emit(extensions.ToolCallEvent{
        ToolName: "test",
        Input:    "{}",
    })
    
    // Assert exact match
    test.AssertPrinted(t, harness, "exact output")
    
    // Or partial match
    test.AssertPrintedContains(t, harness, "partial")
    
    // Assert info/error messages
    test.AssertPrintInfo(t, harness, "info message")
    test.AssertPrintError(t, harness, "error message")
}
```

### Testing Status Bar

```go
func TestStatusBar(t *testing.T) {
    harness := test.New(t)
    harness.LoadFile("my-ext.go")
    
    _, _ = harness.Emit(extensions.AgentEndEvent{})
    
    test.AssertStatusSet(t, harness, "myext:status")
    test.AssertStatusText(t, harness, "myext:status", "Ready")
}
```

### Testing Prompt Results

Configure the mock context to return specific prompt results:

```go
func TestWithPrompts(t *testing.T) {
    harness := test.New(t)
    harness.LoadFile("my-ext.go")
    
    // Configure prompt results before emitting events
    harness.Context().SetPromptSelectResult(extensions.PromptSelectResult{
        Value: "option1",
        Index: 0,
        Cancelled: false,
    })
    
    // Now when your extension calls ctx.PromptSelect(), it will get this result
    _, _ = harness.Emit(extensions.SessionStartEvent{SessionID: "test"})
}
```

## Available Assertions

The test package provides these assertion helpers:

**Event Results:**
- `AssertNotBlocked(t, result)` - Verify tool was not blocked
- `AssertBlocked(t, result, reason)` - Verify tool was blocked with reason
- `AssertInputHandled(t, result, action)` - Verify input was handled
- `AssertInputTransformed(t, result, text)` - Verify input transformation

**Context Interactions:**
- `AssertPrinted(t, harness, text)` - Verify exact print output
- `AssertPrintedContains(t, harness, substring)` - Verify partial print output
- `AssertPrintInfo(t, harness, text)` - Verify PrintInfo was called
- `AssertPrintError(t, harness, text)` - Verify PrintError was called
- `AssertWidgetSet(t, harness, id)` - Verify widget was set
- `AssertWidgetNotSet(t, harness, id)` - Verify widget was not set
- `AssertWidgetText(t, harness, id, text)` - Verify widget content
- `AssertWidgetTextContains(t, harness, id, substring)` - Verify widget contains text
- `AssertHeaderSet(t, harness)` - Verify header was set
- `AssertFooterSet(t, harness)` - Verify footer was set
- `AssertStatusSet(t, harness, key)` - Verify status was set
- `AssertStatusText(t, harness, key, text)` - Verify status text

**Registration:**
- `AssertToolRegistered(t, harness, name)` - Verify tool registration
- `AssertCommandRegistered(t, harness, name)` - Verify command registration
- `AssertHasHandlers(t, harness, eventType)` - Verify handlers exist
- `AssertNoHandlers(t, harness, eventType)` - Verify no handlers

**Messaging:**
- `AssertMessageSent(t, harness, text)` - Verify SendMessage was called
- `AssertCancelAndSend(t, harness, text)` - Verify CancelAndSend was called

## Advanced Usage

### Accessing the Mock Context

For custom assertions, access the mock context directly:

```go
func TestCustomAssertion(t *testing.T) {
    harness := test.New(t)
    harness.LoadFile("my-ext.go")
    
    _, _ = harness.Emit(extensions.SessionStartEvent{SessionID: "test"})
    
    // Get all recorded prints
    prints := harness.Context().GetPrints()
    
    // Check widget directly
    widget, ok := harness.Context().GetWidget("my-widget")
    if ok && widget.Style.BorderColor == "#ff0000" {
        t.Log("Widget has red border")
    }
    
    // Check options
    optionValue := harness.Context().GetOption("my-option")
}
```

### Testing Multiple Extensions

Each harness is isolated:

```go
func TestExtensionIsolation(t *testing.T) {
    // These run in completely separate interpreters
    harness1 := test.New(t)
    harness1.LoadFile("ext1.go")
    
    harness2 := test.New(t)
    harness2.LoadFile("ext2.go")
    
    // Events to one don't affect the other
}
```

### Direct Result Extraction

When you need to inspect result details:

```go
result, _ := harness.Emit(extensions.ToolCallEvent{...})
tcr := test.GetToolCallResult(result)
if tcr != nil {
    t.Logf("Block: %v, Reason: %s", tcr.Block, tcr.Reason)
}
```

## Best Practices

1. **Test one behavior per test** - Keep tests focused and readable
2. **Use inline source for simple tests** - LoadString is great for isolated tests
3. **Use LoadFile for integration tests** - Tests the actual extension file
4. **Assert on context calls** - Verify your extension interacts with the context correctly
5. **Test both positive and negative cases** - Verify tools are blocked AND allowed appropriately
6. **Test all event handlers** - Make sure all registered handlers work correctly

## Limitations

The test harness has these limitations:

1. **No TUI rendering** - Widgets are recorded but not rendered visually
2. **Prompts return configured values** - You must pre-configure prompt results in tests
3. **Subagents don't spawn real processes** - SpawnSubagent returns nil/empty results
4. **LLM completions are mocked** - Complete returns empty responses
5. **Some context methods are no-ops** - Exit, SetActiveTools, etc. don't have side effects

These limitations are intentional - the test harness focuses on testing extension logic, not the full Kit runtime.

## Example: Complete Extension Test

Here's a complete example testing a realistic extension:

```go
package main

import (
    "testing"
    "github.com/mark3labs/kit/internal/extensions/test"
    "github.com/mark3labs/kit/internal/extensions"
)

// Test that the extension properly blocks dangerous tools
func TestSafetyExtension_BlocksDangerousTools(t *testing.T) {
    harness := test.New(t)
    harness.LoadFile("safety-ext.go")
    
    // Verify it handles tool calls
    test.AssertHasHandlers(t, harness, extensions.ToolCall)
    
    // Test allowed tool
    result, _ := harness.Emit(extensions.ToolCallEvent{ToolName: "read", Input: "{}"})
    test.AssertNotBlocked(t, result)
    
    // Test blocked tool
    result, _ = harness.Emit(extensions.ToolCallEvent{ToolName: "rm", Input: "{}"})
    test.AssertBlocked(t, result, "safety block")
    test.AssertPrintError(t, harness, "Tool rm is blocked")
}

// Test that the extension shows status on agent completion
func TestSafetyExtension_ShowsStatus(t *testing.T) {
    harness := test.New(t)
    harness.LoadFile("safety-ext.go")
    
    _, _ = harness.Emit(extensions.AgentEndEvent{})
    
    test.AssertWidgetSet(t, harness, "safety-widget")
    test.AssertWidgetTextContains(t, harness, "safety-widget", "Safe")
}
```
