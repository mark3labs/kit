package test

import (
	"slices"
	"strings"
	"testing"

	"github.com/mark3labs/kit/internal/extensions"
)

// AssertNotBlocked fails the test if the tool call result indicates the tool was blocked.
func AssertNotBlocked(t *testing.T, result extensions.Result) {
	t.Helper()
	if result == nil {
		return
	}
	if tcr, ok := result.(extensions.ToolCallResult); ok {
		if tcr.Block {
			t.Errorf("expected tool to not be blocked, but it was blocked with reason: %q", tcr.Reason)
		}
	}
}

// AssertBlocked fails the test if the tool call result does not indicate the tool was blocked.
func AssertBlocked(t *testing.T, result extensions.Result, expectedReason string) {
	t.Helper()
	if result == nil {
		t.Error("expected tool to be blocked, but result was nil")
		return
	}
	tcr, ok := result.(extensions.ToolCallResult)
	if !ok {
		t.Errorf("expected ToolCallResult, got %T", result)
		return
	}
	if !tcr.Block {
		t.Error("expected tool to be blocked, but it was not blocked")
		return
	}
	if expectedReason != "" && tcr.Reason != expectedReason {
		t.Errorf("expected block reason %q, got %q", expectedReason, tcr.Reason)
	}
}

// AssertInputHandled fails the test if the input result does not indicate the input was handled.
func AssertInputHandled(t *testing.T, result extensions.Result, expectedAction string) {
	t.Helper()
	if result == nil {
		t.Error("expected input to be handled, but result was nil")
		return
	}
	ir, ok := result.(extensions.InputResult)
	if !ok {
		t.Errorf("expected InputResult, got %T", result)
		return
	}
	if ir.Action != expectedAction {
		t.Errorf("expected action %q, got %q", expectedAction, ir.Action)
	}
}

// AssertInputTransformed fails the test if the input was not transformed to the expected text.
func AssertInputTransformed(t *testing.T, result extensions.Result, expectedText string) {
	t.Helper()
	if result == nil {
		t.Errorf("expected input to be transformed to %q, but result was nil", expectedText)
		return
	}
	ir, ok := result.(extensions.InputResult)
	if !ok {
		t.Errorf("expected InputResult, got %T", result)
		return
	}
	if ir.Action != "transform" {
		t.Errorf("expected action 'transform', got %q", ir.Action)
	}
	if ir.Text != expectedText {
		t.Errorf("expected transformed text %q, got %q", expectedText, ir.Text)
	}
}

// AssertPrinted fails the test if the expected text was not printed.
func AssertPrinted(t *testing.T, harness *Harness, expected string) {
	t.Helper()
	prints := harness.Context().GetPrints()
	if slices.Contains(prints, expected) {
		return
	}
	t.Errorf("expected text %q to be printed, but it was not found in prints: %v", expected, prints)
}

// AssertPrintedContains fails the test if no printed text contains the expected substring.
func AssertPrintedContains(t *testing.T, harness *Harness, substring string) {
	t.Helper()
	prints := harness.Context().GetPrints()
	for _, p := range prints {
		if strings.Contains(p, substring) {
			return
		}
	}
	t.Errorf("expected printed text to contain %q, but it was not found in prints: %v", substring, prints)
}

// AssertPrintInfo fails the test if the expected info message was not printed.
func AssertPrintInfo(t *testing.T, harness *Harness, expected string) {
	t.Helper()
	infos := harness.Context().GetPrintInfos()
	if slices.Contains(infos, expected) {
		return
	}
	t.Errorf("expected info message %q, but it was not found in PrintInfos: %v", expected, infos)
}

// AssertPrintError fails the test if the expected error message was not printed.
func AssertPrintError(t *testing.T, harness *Harness, expected string) {
	t.Helper()
	errors := harness.Context().GetPrintErrors()
	if slices.Contains(errors, expected) {
		return
	}
	t.Errorf("expected error message %q, but it was not found in PrintErrors: %v", expected, errors)
}

// AssertWidgetSet fails the test if the widget with the given ID was not set.
func AssertWidgetSet(t *testing.T, harness *Harness, id string) {
	t.Helper()
	if !harness.Context().HasWidget(id) {
		t.Errorf("expected widget %q to be set, but it was not", id)
	}
}

// AssertWidgetNotSet fails the test if the widget with the given ID was set.
func AssertWidgetNotSet(t *testing.T, harness *Harness, id string) {
	t.Helper()
	if harness.Context().HasWidget(id) {
		t.Errorf("expected widget %q to not be set, but it was", id)
	}
}

// AssertWidgetText fails the test if the widget with the given ID does not have the expected text.
func AssertWidgetText(t *testing.T, harness *Harness, id string, expected string) {
	t.Helper()
	widget, ok := harness.Context().GetWidget(id)
	if !ok {
		t.Errorf("expected widget %q to be set, but it was not", id)
		return
	}
	if widget.Content.Text != expected {
		t.Errorf("expected widget %q to have text %q, got %q", id, expected, widget.Content.Text)
	}
}

// AssertWidgetTextContains fails the test if the widget text does not contain the expected substring.
func AssertWidgetTextContains(t *testing.T, harness *Harness, id string, substring string) {
	t.Helper()
	widget, ok := harness.Context().GetWidget(id)
	if !ok {
		t.Errorf("expected widget %q to be set, but it was not", id)
		return
	}
	if !strings.Contains(widget.Content.Text, substring) {
		t.Errorf("expected widget %q text to contain %q, but got %q", id, substring, widget.Content.Text)
	}
}

// AssertHeaderSet fails the test if no header was set.
func AssertHeaderSet(t *testing.T, harness *Harness) {
	t.Helper()
	if harness.Context().GetHeader() == nil {
		t.Error("expected header to be set, but it was not")
	}
}

// AssertFooterSet fails the test if no footer was set.
func AssertFooterSet(t *testing.T, harness *Harness) {
	t.Helper()
	if harness.Context().GetFooter() == nil {
		t.Error("expected footer to be set, but it was not")
	}
}

// AssertStatusSet fails the test if the status with the given key was not set.
func AssertStatusSet(t *testing.T, harness *Harness, key string) {
	t.Helper()
	_, ok := harness.Context().GetStatus(key)
	if !ok {
		t.Errorf("expected status %q to be set, but it was not", key)
	}
}

// AssertStatusText fails the test if the status with the given key does not have the expected text.
func AssertStatusText(t *testing.T, harness *Harness, key string, expected string) {
	t.Helper()
	status, ok := harness.Context().GetStatus(key)
	if !ok {
		t.Errorf("expected status %q to be set, but it was not", key)
		return
	}
	if status.Text != expected {
		t.Errorf("expected status %q to have text %q, got %q", key, expected, status.Text)
	}
}

// AssertHasHandlers fails the test if no handlers are registered for the given event type.
func AssertHasHandlers(t *testing.T, harness *Harness, eventType extensions.EventType) {
	t.Helper()
	if !harness.HasHandlers(eventType) {
		t.Errorf("expected handlers for event type %q, but none were registered", eventType)
	}
}

// AssertNoHandlers fails the test if any handlers are registered for the given event type.
func AssertNoHandlers(t *testing.T, harness *Harness, eventType extensions.EventType) {
	t.Helper()
	if harness.HasHandlers(eventType) {
		t.Errorf("expected no handlers for event type %q, but some were registered", eventType)
	}
}

// AssertToolRegistered fails the test if the tool with the given name was not registered.
func AssertToolRegistered(t *testing.T, harness *Harness, toolName string) {
	t.Helper()
	tools := harness.RegisteredTools()
	for _, tool := range tools {
		if tool.Name == toolName {
			return
		}
	}
	t.Errorf("expected tool %q to be registered, but it was not found in %v", toolName, tools)
}

// AssertCommandRegistered fails the test if the command with the given name was not registered.
func AssertCommandRegistered(t *testing.T, harness *Harness, cmdName string) {
	t.Helper()
	cmds := harness.RegisteredCommands()
	for _, cmd := range cmds {
		if cmd.Name == cmdName {
			return
		}
	}
	t.Errorf("expected command %q to be registered, but it was not found in %v", cmdName, cmds)
}

// AssertMessageSent fails the test if the expected message was not sent.
func AssertMessageSent(t *testing.T, harness *Harness, expected string) {
	t.Helper()
	ctx := harness.Context()
	if slices.Contains(ctx.Messages, expected) {
		return
	}
	t.Errorf("expected message %q to be sent, but it was not found in messages: %v", expected, ctx.Messages)
}

// AssertCancelAndSend fails the test if the expected text was not sent via CancelAndSend.
func AssertCancelAndSend(t *testing.T, harness *Harness, expected string) {
	t.Helper()
	ctx := harness.Context()
	if slices.Contains(ctx.CancelSends, expected) {
		return
	}
	t.Errorf("expected CancelAndSend with %q, but it was not found: %v", expected, ctx.CancelSends)
}

// Helper functions

// GetToolCallResult extracts a ToolCallResult from a Result, or nil if not applicable.
func GetToolCallResult(result extensions.Result) *extensions.ToolCallResult {
	if result == nil {
		return nil
	}
	if tcr, ok := result.(extensions.ToolCallResult); ok {
		return &tcr
	}
	return nil
}

// GetInputResult extracts an InputResult from a Result, or nil if not applicable.
func GetInputResult(result extensions.Result) *extensions.InputResult {
	if result == nil {
		return nil
	}
	if ir, ok := result.(extensions.InputResult); ok {
		return &ir
	}
	return nil
}

// GetToolResultResult extracts a ToolResultResult from a Result, or nil if not applicable.
func GetToolResultResult(result extensions.Result) *extensions.ToolResultResult {
	if result == nil {
		return nil
	}
	if trr, ok := result.(extensions.ToolResultResult); ok {
		return &trr
	}
	return nil
}
