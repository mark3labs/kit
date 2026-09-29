package agent

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"

	"charm.land/fantasy"

	"github.com/mark3labs/kit/internal/models"
)

// This file stops image tool results before they reach a model that cannot
// read images.
//
// Without this check, the image is sent and one of two things happens: the
// provider rejects the request (the recovery in media_recovery.go then
// handles it, at the cost of one failed request), or the provider silently
// drops the image and the model answers about an image it never saw. The
// second case gives no error to recover from, so the check must happen
// before the request.
//
// The check applies only when the model catalog is sure the model has no
// image input. An unknown model keeps the image, and the provider decides.

// imageGateTool wraps a tool and replaces an image result with an error
// result when the active model cannot read images.
type imageGateTool struct {
	fantasy.AgentTool
	// imageInput reports whether the active model accepts images and, if
	// not, its name for the error text. It is called for every result,
	// so a model switch during a session takes effect at once.
	imageInput func() (supported bool, model string)
}

// Run executes the inner tool and gates an image result.
func (t *imageGateTool) Run(ctx context.Context, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
	resp, err := t.AgentTool.Run(ctx, call)
	if err != nil || resp.IsError || !isImageResponse(resp) {
		return resp, err
	}
	supported, model := t.imageInput()
	if supported {
		return resp, nil
	}
	return fantasy.NewTextErrorResponse(imageUnsupportedText(resp, model)), nil
}

// isImageResponse reports whether resp carries an image for the model. The
// type test is the one the LLM library uses to build a media tool result.
func isImageResponse(resp fantasy.ToolResponse) bool {
	return (resp.Type == "image" || resp.Type == "media") && strings.HasPrefix(resp.MediaType, "image/")
}

// imageUnsupportedText builds the error text for an image the active model
// cannot read. It keeps the tool summary, which names the file.
func imageUnsupportedText(resp fantasy.ToolResponse, model string) string {
	what := resp.Content
	if what == "" {
		what = resp.MediaType + " content"
	}
	return fmt.Sprintf("cannot show this result to the model: %s. The current model (%s) does not support image input. "+
		"Tell the user, and use a text-based approach (for example, read the image metadata or ask the user to describe it).",
		what, model)
}

// activeModel holds the model string of the active model. Tools read it while
// they run, possibly in parallel with SetModel, so it is atomic.
type activeModel struct {
	v atomic.Pointer[string]
}

func newActiveModel(cfg *models.ProviderConfig) *activeModel {
	m := &activeModel{}
	if cfg != nil {
		m.set(cfg.ModelString)
	}
	return m
}

func (m *activeModel) set(modelString string) { m.v.Store(&modelString) }

// imageInput reports whether the active model accepts images, and its name.
func (m *activeModel) imageInput() (bool, string) {
	p := m.v.Load()
	if p == nil || *p == "" {
		return true, ""
	}
	return modelImageInput(*p), *p
}

// gateImageTools wraps every tool with an image gate that uses imageInput.
func gateImageTools(tools []fantasy.AgentTool, imageInput func() (bool, string)) []fantasy.AgentTool {
	out := make([]fantasy.AgentTool, len(tools))
	for i, tool := range tools {
		out[i] = &imageGateTool{AgentTool: tool, imageInput: imageInput}
	}
	return out
}

// modelImageInput reports whether the model named by modelString accepts
// images. An unknown model is assumed to accept them.
func modelImageInput(modelString string) bool {
	supported, known := models.LookupModelForSettings(modelString).SupportsImageInput()
	return supported || !known
}

// withoutImages returns a copy of msgs for a model that cannot read images:
// every image tool result and every image file part is replaced by a short
// text note. It changes only the request, not the session, so the images are
// sent again after a switch back to a model that reads them. Messages without
// images are shared, not copied.
func withoutImages(msgs []fantasy.Message, model string) []fantasy.Message {
	var out []fantasy.Message
	for i, msg := range msgs {
		var content []fantasy.MessagePart
		for j, part := range msg.Content {
			replacement, ok := imagePartNote(part, model)
			if !ok {
				if content != nil {
					content = append(content, part)
				}
				continue
			}
			if content == nil {
				content = make([]fantasy.MessagePart, j, len(msg.Content))
				copy(content, msg.Content[:j])
			}
			content = append(content, replacement)
		}
		if content == nil {
			if out != nil {
				out = append(out, msg)
			}
			continue
		}
		if out == nil {
			out = make([]fantasy.Message, i, len(msgs))
			copy(out, msgs[:i])
		}
		msg.Content = content
		out = append(out, msg)
	}
	if out == nil {
		return msgs
	}
	return out
}

// promptWithoutImages removes image attachments from the prompt of a turn
// and adds a note for each one to the prompt text.
func promptWithoutImages(prompt string, files []fantasy.FilePart, model string) (string, []fantasy.FilePart) {
	var kept []fantasy.FilePart
	var notes []string
	for _, f := range files {
		note, ok := imagePartNote(f, model)
		if !ok {
			kept = append(kept, f)
			continue
		}
		notes = append(notes, note.(fantasy.TextPart).Text)
	}
	if len(notes) == 0 {
		return prompt, files
	}
	return strings.Join(append([]string{prompt}, notes...), "\n\n"), kept
}

// imagePartNote returns the text replacement for part when part carries an
// image. The second return value is false when part has no image.
func imagePartNote(part fantasy.MessagePart, model string) (fantasy.MessagePart, bool) {
	switch p := part.(type) {
	case fantasy.ToolResultPart:
		m, ok := p.Output.(fantasy.ToolResultOutputContentMedia)
		if !ok || !strings.HasPrefix(m.MediaType, "image/") {
			return nil, false
		}
		what := m.Text
		if what == "" {
			what = m.MediaType
		}
		p.Output = fantasy.ToolResultOutputContentText{
			Text: fmt.Sprintf("[image not shown: %s. The current model (%s) does not support image input.]", what, model),
		}
		return p, true
	case fantasy.FilePart:
		if !strings.HasPrefix(p.MediaType, "image/") {
			return nil, false
		}
		name := p.Filename
		if name == "" {
			name = p.MediaType
		}
		return fantasy.TextPart{
			Text: fmt.Sprintf("[image %q not shown: the current model (%s) does not support image input. Tell the user.]", name, model),
		}, true
	}
	return nil, false
}
