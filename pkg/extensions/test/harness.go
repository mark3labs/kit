// Package test provides utilities for testing Kit extensions.
//
// This package allows extension authors to write standard Go tests that load
// and exercise their extensions in a controlled environment. Extensions are
// loaded into a Yaegi interpreter with all Kit API symbols available.
//
// Basic usage:
//
//	package main
//
//	import (
//	    "testing"
//	    "github.com/mark3labs/kit/pkg/extensions/test"
//	)
//
//	func TestMyExtension(t *testing.T) {
//	    // Create a test harness
//	    harness := test.New(t)
//
//	    // Load your extension file
//	    ext := harness.LoadFile("my-ext.go")
//
//	    // Emit events and check results
//	    result := harness.Emit(test.ToolCallEvent{
//	        ToolName: "my_tool",
//	        Input:    `{"key": "value"}`,
//	    })
//
//	    // Use assertion helpers
//	    test.AssertNotBlocked(t, result)
//	    test.AssertPrinted(t, harness, "expected output")
//	}
//
// The harness provides a mock Context that records all interactions,
// allowing you to verify that your extension called SetWidget, Print, etc.
package test

import (
	"os"
	"testing"

	"github.com/mark3labs/kit/internal/extensions"
	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
	"github.com/traefik/yaegi/stdlib/unrestricted"
)

// Harness provides a testing environment for Kit extensions.
// It loads extensions into an isolated Yaegi interpreter and provides
// methods to emit events and verify extension behavior.
type Harness struct {
	t       *testing.T
	runner  *extensions.Runner
	context *MockContext
	extPath string
}

// New creates a new test harness for the given test.
// The harness must be used within a single test function.
func New(t *testing.T) *Harness {
	return &Harness{
		t:       t,
		context: NewMockContext(),
	}
}

// LoadFile loads an extension from a file path.
// The extension is evaluated in a fresh Yaegi interpreter with all
// Kit API symbols available. The Init function is called automatically.
//
// Returns the loaded extension or fails the test on error.
func (h *Harness) LoadFile(path string) *extensions.LoadedExtension {
	h.t.Helper()

	// Verify file exists
	if _, err := os.Stat(path); err != nil {
		h.t.Fatalf("extension file not found: %s: %v", path, err)
	}

	// Read extension source
	src, err := os.ReadFile(path)
	if err != nil {
		h.t.Fatalf("failed to read extension file: %v", err)
	}

	return h.loadSource(string(src), path)
}

// LoadString loads an extension from a source string.
// Useful for inline extension tests. The path is used for error reporting.
func (h *Harness) LoadString(src string, path string) *extensions.LoadedExtension {
	h.t.Helper()
	return h.loadSource(src, path)
}

// loadSource is the internal implementation that loads extension source
// into a Yaegi interpreter.
func (h *Harness) loadSource(src string, path string) *extensions.LoadedExtension {
	h.t.Helper()

	// Create a fresh interpreter
	i := interp.New(interp.Options{})

	// Expose Go stdlib
	if err := i.Use(stdlib.Symbols); err != nil {
		h.t.Fatalf("failed to load stdlib symbols: %v", err)
	}
	if err := i.Use(unrestricted.Symbols); err != nil {
		h.t.Fatalf("failed to load unrestricted symbols: %v", err)
	}

	// Expose Kit extension API symbols
	if err := i.Use(extensions.Symbols()); err != nil {
		h.t.Fatalf("failed to load extension symbols: %v", err)
	}

	// Evaluate the extension source
	if _, err := i.Eval(src); err != nil {
		h.t.Fatalf("failed to evaluate extension source: %v", err)
	}

	// Extract the Init function
	initVal, err := i.Eval("Init")
	if err != nil {
		h.t.Fatalf("extension has no Init function: %v", err)
	}

	initFn, ok := initVal.Interface().(func(extensions.API))
	if !ok {
		h.t.Fatalf("Init has wrong signature (want func(ext.API), got %T)", initVal.Interface())
	}

	// Create the extension struct
	ext := &extensions.LoadedExtension{
		Path:     path,
		Handlers: make(map[extensions.EventType][]extensions.HandlerFunc),
	}

	// Create the API object using the test helper
	api := extensions.NewTestAPI(ext)

	// Call Init to register handlers
	initFn(api)

	// Create runner with the loaded extension
	h.runner = extensions.NewRunner([]extensions.LoadedExtension{*ext})
	h.extPath = path

	// Wire the mock context
	h.runner.SetContext(h.context.ToContext())

	return ext
}

// Emit sends an event to the loaded extension(s) and returns the result.
// Events are dispatched in order and blocking results stop propagation.
func (h *Harness) Emit(event extensions.Event) (extensions.Result, error) {
	h.t.Helper()

	if h.runner == nil {
		h.t.Fatal("no extension loaded, call LoadFile() or LoadString() first")
	}

	return h.runner.Emit(event)
}

// EmitJSON is a convenience method for emitting a ToolCallEvent with JSON input.
func (h *Harness) EmitJSON(toolName string, input string) (*extensions.ToolCallResult, error) {
	h.t.Helper()

	result, err := h.Emit(extensions.ToolCallEvent{
		ToolName: toolName,
		Input:    input,
	})
	if err != nil {
		return nil, err
	}

	if result == nil {
		return nil, nil
	}

	tcr, ok := result.(extensions.ToolCallResult)
	if !ok {
		h.t.Fatalf("expected ToolCallResult, got %T", result)
	}

	return &tcr, nil
}

// Context returns the mock context for inspection.
// Use this to verify Print calls, widget settings, etc.
func (h *Harness) Context() *MockContext {
	return h.context
}

// Runner returns the underlying runner for advanced use cases.
func (h *Harness) Runner() *extensions.Runner {
	return h.runner
}

// HasHandlers reports whether any handlers are registered for the given event type.
func (h *Harness) HasHandlers(eventType extensions.EventType) bool {
	if h.runner == nil {
		return false
	}
	return h.runner.HasHandlers(eventType)
}

// RegisteredTools returns all tools registered by the extension.
func (h *Harness) RegisteredTools() []extensions.ToolDef {
	if h.runner == nil {
		return nil
	}
	return h.runner.RegisteredTools()
}

// RegisteredCommands returns all commands registered by the extension.
func (h *Harness) RegisteredCommands() []extensions.CommandDef {
	if h.runner == nil {
		return nil
	}
	return h.runner.RegisteredCommands()
}

// MustLoad is like LoadFile but fails the test immediately on error.
// It returns the harness for chaining.
func (h *Harness) MustLoad(path string) *Harness {
	h.t.Helper()
	h.LoadFile(path)
	return h
}
