package agent

import (
	"context"
	"strings"
	"testing"

	"charm.land/fantasy"

	"github.com/mark3labs/kit/internal/core"
	"github.com/mark3labs/kit/internal/media"
	"github.com/mark3labs/kit/internal/models"
)

// Model strings from the embedded catalog. The test fails loudly if the
// catalog changes their image support, instead of silently testing nothing.
const (
	textOnlyModel = "deepseek/deepseek-v4-pro"
	visionModel   = "anthropic/claude-haiku-4-5"
	unknownModel  = "ollama/some-local-model"
)

func TestCatalogImageSupport(t *testing.T) {
	tests := []struct {
		model           string
		supported, know bool
	}{
		{textOnlyModel, false, true},
		{visionModel, true, true},
		{unknownModel, false, false},
	}
	for _, tc := range tests {
		s, k := models.LookupModelForSettings(tc.model).SupportsImageInput()
		if s != tc.supported || k != tc.know {
			t.Errorf("%s: SupportsImageInput = (%v, %v), want (%v, %v)", tc.model, s, k, tc.supported, tc.know)
		}
	}
	if !modelImageInput(unknownModel) {
		t.Error("an unknown model must be assumed to accept images")
	}
}

// fakeTool returns a fixed response.
type fakeTool struct {
	fantasy.AgentTool
	resp fantasy.ToolResponse
}

func (f *fakeTool) Run(context.Context, fantasy.ToolCall) (fantasy.ToolResponse, error) {
	return f.resp, nil
}

func TestImageGateTool(t *testing.T) {
	img := fantasy.NewImageResponse([]byte("png"), "image/png")
	img.Content = "Read image shot.png (image/png, 1x1, 3 bytes)"
	audio := fantasy.NewMediaResponse([]byte("wav"), "audio/wav")
	text := fantasy.NewTextResponse("hello")

	run := func(resp fantasy.ToolResponse, supported bool) fantasy.ToolResponse {
		gate := &imageGateTool{
			AgentTool:  &fakeTool{resp: resp},
			imageInput: func() (bool, string) { return supported, "p/text-model" },
		}
		out, err := gate.Run(context.Background(), fantasy.ToolCall{})
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
		return out
	}

	if out := run(img, true); out.Type != "image" {
		t.Errorf("vision model: Type = %q, want the image unchanged", out.Type)
	}
	out := run(img, false)
	if !out.IsError || len(out.Data) != 0 {
		t.Fatalf("text model: got %+v, want an error with no image data", out)
	}
	for _, want := range []string{"shot.png", "p/text-model", "does not support image input"} {
		if !strings.Contains(out.Content, want) {
			t.Errorf("error text %q does not contain %q", out.Content, want)
		}
	}
	if out := run(audio, false); out.Type != "media" {
		t.Error("audio result was gated; the gate is only for images")
	}
	if out := run(text, false); out.Content != "hello" {
		t.Error("text result was changed")
	}
}

func TestWithoutImages(t *testing.T) {
	imgResult := fantasy.ToolResultPart{
		ToolCallID: "c1",
		Output:     fantasy.ToolResultOutputContentMedia{Data: "QUJD", MediaType: "image/png", Text: "Read image a.png"},
	}
	audioResult := fantasy.ToolResultPart{
		ToolCallID: "c2",
		Output:     fantasy.ToolResultOutputContentMedia{Data: "QUJD", MediaType: "audio/wav"},
	}
	plain := fantasy.NewUserMessage("no images here")
	msgs := []fantasy.Message{
		plain,
		fantasy.NewUserMessage("look", fantasy.FilePart{Filename: "b.png", MediaType: "image/png", Data: []byte("x")}),
		{Role: fantasy.MessageRoleTool, Content: []fantasy.MessagePart{imgResult, audioResult}},
	}

	out := withoutImages(msgs, "p/text-model")

	if len(out) != 3 {
		t.Fatalf("len = %d, want 3", len(out))
	}
	if _, ok := out[1].Content[1].(fantasy.TextPart); !ok {
		t.Errorf("user image = %T, want a text note", out[1].Content[1])
	}
	tr := out[2].Content[0].(fantasy.ToolResultPart)
	if tr.ToolCallID != "c1" {
		t.Error("tool call ID lost; the call/result pairing must survive")
	}
	if txt, ok := tr.Output.(fantasy.ToolResultOutputContentText); !ok || !strings.Contains(txt.Text, "a.png") {
		t.Errorf("image tool result = %#v, want a text note naming the file", tr.Output)
	}
	if _, ok := out[2].Content[1].(fantasy.ToolResultPart).Output.(fantasy.ToolResultOutputContentMedia); !ok {
		t.Error("audio tool result was removed; only images must be")
	}
	// The input must not change: the session still holds the images.
	if _, ok := msgs[1].Content[1].(fantasy.FilePart); !ok {
		t.Error("input user message was modified")
	}
	if _, ok := msgs[2].Content[0].(fantasy.ToolResultPart).Output.(fantasy.ToolResultOutputContentMedia); !ok {
		t.Error("input tool result was modified")
	}

	// No images: the same slice comes back.
	same := []fantasy.Message{plain}
	if got := withoutImages(same, "m"); &got[0] != &same[0] {
		t.Error("a slice without images was copied")
	}
}

func TestPromptWithoutImages(t *testing.T) {
	files := []fantasy.FilePart{
		{Filename: "a.png", MediaType: "image/png"},
		{Filename: "doc.pdf", MediaType: "application/pdf"},
	}
	prompt, kept := promptWithoutImages("describe", files, "p/m")
	if len(kept) != 1 || kept[0].Filename != "doc.pdf" {
		t.Errorf("kept = %+v, want only the PDF", kept)
	}
	if !strings.HasPrefix(prompt, "describe") || !strings.Contains(prompt, "a.png") {
		t.Errorf("prompt = %q, want the text plus a note naming a.png", prompt)
	}
}

// TestE2EReadImageTextOnlyModel runs the real read tool through the agent
// with a text-only model: the provider must never receive the image, and the
// model must get the reason.
func TestE2EReadImageTextOnlyModel(t *testing.T) {
	ctx := context.Background()
	path := writeTestPNG(t, t.TempDir(), "chart.png", 32, 32, false)

	srv := newRecordingServer(t, toolCallSSE(path), answerSSE)
	a := newImageAgent(ctx, t, srv.Server, media.Limits{})
	a.currentModel = newActiveModel(&models.ProviderConfig{ModelString: textOnlyModel})
	// Rebuild with the gate, as NewAgent does.
	gated := gateImageTools([]fantasy.AgentTool{core.NewReadTool()}, a.currentModel.imageInput)
	a.coreTools = gated
	a.fantasyAgent = fantasy.NewAgent(a.model, fantasy.WithTools(gated...))

	var resultText string
	var isErr bool
	if _, err := a.GenerateWithCallbacks(ctx,
		[]fantasy.Message{fantasy.NewUserMessage("what is in chart.png?")},
		GenerateCallbacks{OnToolResult: func(_, _, _, text, _ string, e bool) { resultText, isErr = text, e }},
	); err != nil {
		t.Fatalf("generate: %v", err)
	}
	if urls := imagePartsIn(t, srv.body(1)); len(urls) != 0 {
		t.Fatalf("image parts sent to a text-only model = %d, want 0", len(urls))
	}
	if !isErr || !strings.Contains(resultText, "does not support image input") {
		t.Errorf("tool result = %q (error %v), want the reason", resultText, isErr)
	}
	if !strings.Contains(string(srv.body(1)), "does not support image input") {
		t.Error("the model never got the reason")
	}

	// Switch to a vision model: the same read must now send the image.
	a.currentModel.set(visionModel)
	srv2 := newRecordingServer(t, toolCallSSE(path), answerSSE)
	a2 := newImageAgent(ctx, t, srv2.Server, media.Limits{})
	a2.currentModel = a.currentModel
	gated2 := gateImageTools([]fantasy.AgentTool{core.NewReadTool()}, a2.currentModel.imageInput)
	a2.coreTools = gated2
	a2.fantasyAgent = fantasy.NewAgent(a2.model, fantasy.WithTools(gated2...))
	if _, err := a2.GenerateWithCallbacks(ctx,
		[]fantasy.Message{fantasy.NewUserMessage("what is in chart.png?")}, GenerateCallbacks{}); err != nil {
		t.Fatalf("generate: %v", err)
	}
	if urls := imagePartsIn(t, srv2.body(1)); len(urls) != 1 {
		t.Errorf("image parts after switch to a vision model = %d, want 1", len(urls))
	}
}

// TestE2EHistoryImagesRemovedForTextOnlyModel checks that images already in
// the history (read under a vision model) are not sent after a switch to a
// text-only model.
func TestE2EHistoryImagesRemovedForTextOnlyModel(t *testing.T) {
	ctx := context.Background()
	srv := newRecordingServer(t, answerSSE)
	a := newImageAgent(ctx, t, srv.Server, media.Limits{})
	a.currentModel = newActiveModel(&models.ProviderConfig{ModelString: textOnlyModel})

	history := []fantasy.Message{
		fantasy.NewUserMessage("read it"),
		{Role: fantasy.MessageRoleAssistant, Content: []fantasy.MessagePart{
			fantasy.ToolCallPart{ToolCallID: "c1", ToolName: "read", Input: `{"path":"old.png"}`},
		}},
		{Role: fantasy.MessageRoleTool, Content: []fantasy.MessagePart{
			fantasy.ToolResultPart{ToolCallID: "c1", Output: fantasy.ToolResultOutputContentMedia{
				Data: "iVBORw0KGgo=", MediaType: "image/png", Text: "Read image old.png",
			}},
		}},
		fantasy.NewUserMessage("now summarise", fantasy.FilePart{Filename: "new.png", MediaType: "image/png", Data: []byte("x")}),
	}
	if _, err := a.GenerateWithCallbacks(ctx, history, GenerateCallbacks{OnStepMessages: func([]fantasy.Message) {}}); err != nil {
		t.Fatalf("generate: %v", err)
	}
	body := string(srv.body(0))
	if urls := imagePartsIn(t, srv.body(0)); len(urls) != 0 {
		t.Fatalf("image parts sent = %d, want 0", len(urls))
	}
	for _, want := range []string{"old.png", "new.png"} {
		if !strings.Contains(body, want) {
			t.Errorf("request does not mention %s; the model must know an image was removed", want)
		}
	}
	// The caller's history is not modified.
	if _, ok := history[2].Content[0].(fantasy.ToolResultPart).Output.(fantasy.ToolResultOutputContentMedia); !ok {
		t.Error("the session history was modified")
	}
}
