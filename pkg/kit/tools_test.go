package kit_test

import (
	"context"
	"testing"

	kit "github.com/mark3labs/kit/pkg/kit"
)

// TestNewTool_BasicTextResult verifies that NewTool creates a working tool
// that returns text content via ToolOutput.
func TestNewTool_BasicTextResult(t *testing.T) {
	type Input struct {
		Name string `json:"name"`
	}

	tool := kit.NewTool("greet", "Greet someone",
		func(ctx context.Context, input Input) (kit.ToolOutput, error) {
			return kit.TextResult("hello " + input.Name), nil
		},
	)

	info := tool.Info()
	if info.Name != "greet" {
		t.Errorf("Info().Name = %q, want %q", info.Name, "greet")
	}
	if info.Description != "Greet someone" {
		t.Errorf("Info().Description = %q, want %q", info.Description, "Greet someone")
	}
	if info.Parallel {
		t.Error("NewTool should not mark tool as parallel")
	}
}

// TestNewParallelTool_MarkedParallel verifies that NewParallelTool marks the
// tool as safe for concurrent execution.
func TestNewParallelTool_MarkedParallel(t *testing.T) {
	type Input struct {
		Query string `json:"query"`
	}

	tool := kit.NewParallelTool("search", "Search for things",
		func(ctx context.Context, input Input) (kit.ToolOutput, error) {
			return kit.TextResult("found: " + input.Query), nil
		},
	)

	info := tool.Info()
	if info.Name != "search" {
		t.Errorf("Info().Name = %q, want %q", info.Name, "search")
	}
	if !info.Parallel {
		t.Error("NewParallelTool should mark tool as parallel")
	}
}

// TestTextResult verifies the TextResult convenience constructor.
func TestTextResult(t *testing.T) {
	r := kit.TextResult("ok")
	if r.Content != "ok" {
		t.Errorf("Content = %q, want %q", r.Content, "ok")
	}
	if r.IsError {
		t.Error("TextResult should not set IsError")
	}
}

// TestErrorResult verifies the ErrorResult convenience constructor.
func TestErrorResult(t *testing.T) {
	r := kit.ErrorResult("bad input")
	if r.Content != "bad input" {
		t.Errorf("Content = %q, want %q", r.Content, "bad input")
	}
	if !r.IsError {
		t.Error("ErrorResult should set IsError")
	}
}

// TestToolCallIDFromContext verifies round-trip context injection.
func TestToolCallIDFromContext(t *testing.T) {
	// Empty context returns empty string.
	if id := kit.ToolCallIDFromContext(context.Background()); id != "" {
		t.Errorf("expected empty string from bare context, got %q", id)
	}
}

// TestToolOutput_Metadata verifies that metadata can be set on ToolOutput.
func TestToolOutput_Metadata(t *testing.T) {
	r := kit.ToolOutput{
		Content:  "data",
		Metadata: map[string]string{"key": "value"},
	}
	if r.Metadata == nil {
		t.Error("expected non-nil Metadata")
	}
	m, ok := r.Metadata.(map[string]string)
	if !ok {
		t.Fatalf("expected map[string]string, got %T", r.Metadata)
	}
	if m["key"] != "value" {
		t.Errorf("Metadata[key] = %q, want %q", m["key"], "value")
	}
}

// TestToolOutput_BinaryData verifies that binary data fields work correctly.
func TestToolOutput_BinaryData(t *testing.T) {
	data := []byte{0x89, 0x50, 0x4E, 0x47}
	r := kit.ToolOutput{
		Content:   "image result",
		Data:      data,
		MediaType: "image/png",
	}
	if len(r.Data) != 4 {
		t.Errorf("Data len = %d, want 4", len(r.Data))
	}
	if r.MediaType != "image/png" {
		t.Errorf("MediaType = %q, want %q", r.MediaType, "image/png")
	}
}
