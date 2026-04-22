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

// TestImageResult verifies the ImageResult convenience constructor.
func TestImageResult(t *testing.T) {
	data := []byte{0x89, 0x50, 0x4E, 0x47}
	r := kit.ImageResult("here is the image", data, "image/png")
	if r.Content != "here is the image" {
		t.Errorf("Content = %q, want %q", r.Content, "here is the image")
	}
	if len(r.Data) != 4 {
		t.Errorf("Data len = %d, want 4", len(r.Data))
	}
	if r.MediaType != "image/png" {
		t.Errorf("MediaType = %q, want %q", r.MediaType, "image/png")
	}
	if r.IsError {
		t.Error("ImageResult should not set IsError")
	}
}

// TestMediaResult verifies the MediaResult convenience constructor.
func TestMediaResult(t *testing.T) {
	data := []byte{0xFF, 0xFB, 0x90, 0x00}
	r := kit.MediaResult("audio clip", data, "audio/mpeg")
	if r.Content != "audio clip" {
		t.Errorf("Content = %q, want %q", r.Content, "audio clip")
	}
	if len(r.Data) != 4 {
		t.Errorf("Data len = %d, want 4", len(r.Data))
	}
	if r.MediaType != "audio/mpeg" {
		t.Errorf("MediaType = %q, want %q", r.MediaType, "audio/mpeg")
	}
	if r.IsError {
		t.Error("MediaResult should not set IsError")
	}
}

// TestNewTool_BinaryImageResponse verifies that NewTool correctly infers the
// response type for image data so binary content is forwarded to the LLM
// (issue #17).
func TestNewTool_BinaryImageResponse(t *testing.T) {
	type Input struct {
		Path string `json:"path"`
	}

	imgData := []byte{0x89, 0x50, 0x4E, 0x47} // PNG magic bytes

	tool := kit.NewTool("read_image", "Read an image file",
		func(ctx context.Context, input Input) (kit.ToolOutput, error) {
			return kit.ImageResult("Here is the image", imgData, "image/png"), nil
		},
	)

	// Run the tool and inspect the raw ToolResponse via the AgentTool interface.
	resp, err := tool.Run(context.Background(), kit.LLMToolCall{
		ID:    "call_1",
		Name:  "read_image",
		Input: `{"path": "test.png"}`,
	})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	// The Type field must be "image" so the downstream framework creates a
	// media content block instead of discarding the binary data.
	if resp.Type != "image" {
		t.Errorf("ToolResponse.Type = %q, want %q", resp.Type, "image")
	}
	if len(resp.Data) != 4 {
		t.Errorf("ToolResponse.Data len = %d, want 4", len(resp.Data))
	}
	if resp.MediaType != "image/png" {
		t.Errorf("ToolResponse.MediaType = %q, want %q", resp.MediaType, "image/png")
	}
	if resp.Content != "Here is the image" {
		t.Errorf("ToolResponse.Content = %q, want %q", resp.Content, "Here is the image")
	}
}

// TestNewTool_BinaryMediaResponse verifies type inference for non-image media.
func TestNewTool_BinaryMediaResponse(t *testing.T) {
	type Input struct{}

	tool := kit.NewTool("get_audio", "Get audio",
		func(ctx context.Context, input Input) (kit.ToolOutput, error) {
			return kit.MediaResult("audio clip", []byte{0xFF, 0xFB}, "audio/mpeg"), nil
		},
	)

	resp, err := tool.Run(context.Background(), kit.LLMToolCall{
		ID:    "call_2",
		Name:  "get_audio",
		Input: `{}`,
	})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}
	if resp.Type != "media" {
		t.Errorf("ToolResponse.Type = %q, want %q", resp.Type, "media")
	}
}

// TestNewTool_TextResponseTypeNotSet verifies that text-only responses do NOT
// get an inferred type (preserving existing behavior).
func TestNewTool_TextResponseTypeNotSet(t *testing.T) {
	type Input struct{}

	tool := kit.NewTool("echo", "Echo",
		func(ctx context.Context, input Input) (kit.ToolOutput, error) {
			return kit.TextResult("hello"), nil
		},
	)

	resp, err := tool.Run(context.Background(), kit.LLMToolCall{
		ID: "call_3", Name: "echo", Input: `{}`,
	})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}
	// Text responses should not have Type set (the framework treats "" as text).
	if resp.Type != "" {
		t.Errorf("ToolResponse.Type = %q, want empty string for text responses", resp.Type)
	}
}

// TestNewParallelTool_BinaryImageResponse mirrors the NewTool binary test for
// NewParallelTool.
func TestNewParallelTool_BinaryImageResponse(t *testing.T) {
	type Input struct{}

	tool := kit.NewParallelTool("snap", "Take a snapshot",
		func(ctx context.Context, input Input) (kit.ToolOutput, error) {
			return kit.ImageResult("snapshot", []byte{0xFF, 0xD8}, "image/jpeg"), nil
		},
	)

	resp, err := tool.Run(context.Background(), kit.LLMToolCall{
		ID: "call_4", Name: "snap", Input: `{}`,
	})
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}
	if resp.Type != "image" {
		t.Errorf("ToolResponse.Type = %q, want %q", resp.Type, "image")
	}
}
