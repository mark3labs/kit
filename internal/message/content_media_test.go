package message

import (
	"testing"

	"charm.land/fantasy"
)

// TestToolResultMediaRoundTrip checks that a media tool result survives the
// conversion to a stored Message and back. Without this, resuming a session
// would silently drop every image the agent had read.
func TestToolResultMediaRoundTrip(t *testing.T) {
	const (
		callID    = "call_abc"
		b64       = "aGVsbG8gd29ybGQ="
		mediaType = "image/png"
		summary   = "Read image shot.png (image/png, 120x80, 900 bytes)"
	)

	original := fantasy.Message{
		Role: fantasy.MessageRoleTool,
		Content: []fantasy.MessagePart{
			fantasy.ToolResultPart{
				ToolCallID: callID,
				Output: fantasy.ToolResultOutputContentMedia{
					Data:      b64,
					MediaType: mediaType,
					Text:      summary,
				},
			},
		},
	}

	stored := FromLLMMessage(original)

	results := stored.ToolResults()
	if len(results) != 1 {
		t.Fatalf("len(ToolResults()) = %d, want 1", len(results))
	}
	got := results[0]
	if got.MediaData != b64 {
		t.Errorf("MediaData = %q, want %q", got.MediaData, b64)
	}
	if got.MediaType != mediaType {
		t.Errorf("MediaType = %q, want %q", got.MediaType, mediaType)
	}
	if got.Content != summary {
		t.Errorf("Content = %q, want %q", got.Content, summary)
	}
	if got.IsError {
		t.Error("IsError = true, want false")
	}

	back := stored.ToLLMMessages()
	if len(back) != 1 {
		t.Fatalf("len(ToLLMMessages()) = %d, want 1", len(back))
	}
	parts := back[0].Content
	if len(parts) != 1 {
		t.Fatalf("len(parts) = %d, want 1", len(parts))
	}
	resultPart, ok := parts[0].(fantasy.ToolResultPart)
	if !ok {
		t.Fatalf("part type = %T, want fantasy.ToolResultPart", parts[0])
	}
	mediaOut, ok := resultPart.Output.(fantasy.ToolResultOutputContentMedia)
	if !ok {
		t.Fatalf("output type = %T, want fantasy.ToolResultOutputContentMedia; the image was lost",
			resultPart.Output)
	}
	if mediaOut.Data != b64 {
		t.Errorf("Data = %q, want %q", mediaOut.Data, b64)
	}
	if mediaOut.MediaType != mediaType {
		t.Errorf("MediaType = %q, want %q", mediaOut.MediaType, mediaType)
	}
	if mediaOut.Text != summary {
		t.Errorf("Text = %q, want %q", mediaOut.Text, summary)
	}
}

// TestToolResultTextStaysText guards the regression risk of the media branch:
// a plain text result must not become a media result.
func TestToolResultTextStaysText(t *testing.T) {
	stored := Message{
		Role: RoleTool,
		Parts: []ContentPart{
			ToolResult{ToolCallID: "c1", Content: "1: package main"},
		},
	}
	back := stored.ToLLMMessages()
	if len(back) != 1 {
		t.Fatalf("len(ToLLMMessages()) = %d, want 1", len(back))
	}
	part := back[0].Content[0].(fantasy.ToolResultPart)
	if _, ok := part.Output.(fantasy.ToolResultOutputContentText); !ok {
		t.Fatalf("output type = %T, want fantasy.ToolResultOutputContentText", part.Output)
	}
}

// TestToolResultErrorWinsOverMedia checks the branch order: an error result
// stays an error even when media fields happen to be set.
func TestToolResultErrorWinsOverMedia(t *testing.T) {
	stored := Message{
		Role: RoleTool,
		Parts: []ContentPart{
			ToolResult{
				ToolCallID: "c1",
				Content:    "cannot read image",
				IsError:    true,
				MediaData:  "aGk=",
				MediaType:  "image/png",
			},
		},
	}
	back := stored.ToLLMMessages()
	part := back[0].Content[0].(fantasy.ToolResultPart)
	if _, ok := part.Output.(fantasy.ToolResultOutputContentError); !ok {
		t.Fatalf("output type = %T, want fantasy.ToolResultOutputContentError", part.Output)
	}
}
