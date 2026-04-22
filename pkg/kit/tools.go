package kit

import (
	"context"
	"strings"

	"charm.land/fantasy"

	"github.com/mark3labs/kit/internal/core"
)

// Tool is the interface that all Kit tools implement.
type Tool = fantasy.AgentTool

// ToolOption configures tool behavior.
type ToolOption = core.ToolOption

// WithWorkDir sets the working directory for file-based tools.
// If empty, os.Getwd() is used at execution time.
var WithWorkDir = core.WithWorkDir

// --- Custom tool creation ---

// ToolOutput is the return value from custom tool handlers created with
// [NewTool] or [NewParallelTool]. It provides a dependency-free way to
// return results without importing the underlying LLM framework.
type ToolOutput struct {
	// Content is the text content returned to the LLM.
	Content string

	// IsError, when true, signals to the LLM that the tool call failed.
	IsError bool

	// Data contains optional binary data (images, audio, etc.).
	Data []byte

	// MediaType is the MIME type for binary Data (e.g. "image/png").
	MediaType string

	// Metadata is optional opaque metadata attached to the response.
	// It is not sent to the LLM but may be consumed by hooks or the UI.
	Metadata any
}

// TextResult creates a successful text [ToolOutput].
func TextResult(content string) ToolOutput {
	return ToolOutput{Content: content}
}

// ErrorResult creates an error [ToolOutput]. The LLM will see the content
// as a tool error, allowing it to retry or adjust its approach.
func ErrorResult(content string) ToolOutput {
	return ToolOutput{Content: content, IsError: true}
}

// ImageResult creates a [ToolOutput] that returns an image to the LLM.
// The data is the raw image bytes and mediaType is the MIME type
// (e.g. "image/png", "image/jpeg"). The optional text content accompanies
// the image and is visible to the LLM alongside it.
func ImageResult(content string, data []byte, mediaType string) ToolOutput {
	return ToolOutput{Content: content, Data: data, MediaType: mediaType}
}

// MediaResult creates a [ToolOutput] that returns non-image binary media
// (e.g. audio, video) to the LLM. The data is the raw bytes and mediaType
// is the MIME type (e.g. "audio/wav", "video/mp4"). The optional text
// content accompanies the media.
func MediaResult(content string, data []byte, mediaType string) ToolOutput {
	return ToolOutput{Content: content, Data: data, MediaType: mediaType}
}

// toolCallIDKey is the context key for the tool call ID.
type toolCallIDKey struct{}

// ToolCallIDFromContext extracts the tool call ID from the context.
// The call ID is set automatically by [NewTool] and [NewParallelTool]
// before invoking the handler. Returns an empty string if no ID is present.
func ToolCallIDFromContext(ctx context.Context) string {
	s, _ := ctx.Value(toolCallIDKey{}).(string)
	return s
}

// toolOutputToResponse converts a [ToolOutput] into the underlying
// framework's ToolResponse, inferring the response Type from Data/MediaType
// so that binary content (images, audio, etc.) is forwarded to the LLM
// instead of being silently dropped.
func toolOutputToResponse(result ToolOutput) fantasy.ToolResponse {
	resp := fantasy.ToolResponse{
		Content:   result.Content,
		IsError:   result.IsError,
		Data:      result.Data,
		MediaType: result.MediaType,
	}
	// Infer response type from binary data so the downstream framework
	// creates a media content block instead of a plain-text one.
	if len(result.Data) > 0 && result.MediaType != "" {
		if strings.HasPrefix(result.MediaType, "image/") {
			resp.Type = "image"
		} else {
			resp.Type = "media"
		}
	}
	if result.Metadata != nil {
		resp = fantasy.WithResponseMetadata(resp, result.Metadata)
	}
	return resp
}

// NewTool creates a custom [Tool] with automatic JSON schema generation from
// the TInput struct type. The handler receives a typed input (deserialized
// from the LLM's JSON arguments) and returns a [ToolOutput].
//
// Struct tags on TInput control the generated schema:
//
//	json:"name"         → parameter name
//	description:"..."   → parameter description shown to the LLM
//	enum:"a,b,c"        → restrict valid values
//	omitempty            → marks the parameter as optional
//
// The tool call ID is injected into the context and can be retrieved with
// [ToolCallIDFromContext].
//
// Binary results: When [ToolOutput.Data] and [ToolOutput.MediaType] are set,
// the response type is automatically inferred so the LLM receives the binary
// content (e.g. an image) instead of only the text. Use [ImageResult] or
// [MediaResult] for convenience.
//
// Example:
//
//	type WeatherInput struct {
//	    City string `json:"city" description:"City name"`
//	}
//
//	tool := kit.NewTool("get_weather", "Get weather for a city",
//	    func(ctx context.Context, input WeatherInput) (kit.ToolOutput, error) {
//	        return kit.TextResult("72°F, sunny in " + input.City), nil
//	    },
//	)
func NewTool[TInput any](name, description string, fn func(ctx context.Context, input TInput) (ToolOutput, error)) Tool {
	return fantasy.NewAgentTool(name, description,
		func(ctx context.Context, input TInput, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			ctx = context.WithValue(ctx, toolCallIDKey{}, call.ID)
			result, err := fn(ctx, input)
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}
			return toolOutputToResponse(result), nil
		},
	)
}

// NewParallelTool is like [NewTool] but marks the tool as safe for concurrent
// execution alongside other tools. Use this when the tool has no side effects
// or when concurrent calls are safe.
func NewParallelTool[TInput any](name, description string, fn func(ctx context.Context, input TInput) (ToolOutput, error)) Tool {
	return fantasy.NewParallelAgentTool(name, description,
		func(ctx context.Context, input TInput, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			ctx = context.WithValue(ctx, toolCallIDKey{}, call.ID)
			result, err := fn(ctx, input)
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}
			return toolOutputToResponse(result), nil
		},
	)
}

// --- Individual tool constructors ---

// NewReadTool creates a file-reading tool.
func NewReadTool(opts ...ToolOption) Tool { return core.NewReadTool(opts...) }

// NewWriteTool creates a file-writing tool.
func NewWriteTool(opts ...ToolOption) Tool { return core.NewWriteTool(opts...) }

// NewEditTool creates a surgical text-editing tool.
func NewEditTool(opts ...ToolOption) Tool { return core.NewEditTool(opts...) }

// NewBashTool creates a bash command execution tool.
func NewBashTool(opts ...ToolOption) Tool { return core.NewBashTool(opts...) }

// NewGrepTool creates a content search tool (uses ripgrep when available).
func NewGrepTool(opts ...ToolOption) Tool { return core.NewGrepTool(opts...) }

// NewFindTool creates a file search tool (uses fd when available).
func NewFindTool(opts ...ToolOption) Tool { return core.NewFindTool(opts...) }

// NewLsTool creates a directory listing tool.
func NewLsTool(opts ...ToolOption) Tool { return core.NewLsTool(opts...) }

// --- Tool bundles ---

// AllTools returns all available core tools.
func AllTools(opts ...ToolOption) []Tool { return core.AllTools(opts...) }

// CodingTools returns the default set of core tools for a coding agent:
// bash, read, write, edit.
func CodingTools(opts ...ToolOption) []Tool { return core.CodingTools(opts...) }

// ReadOnlyTools returns tools for read-only exploration:
// read, grep, find, ls.
func ReadOnlyTools(opts ...ToolOption) []Tool { return core.ReadOnlyTools(opts...) }

// SubagentTools returns all core tools except subagent. Use this when
// creating child Kit instances (in-process subagents) to prevent infinite
// recursion.
func SubagentTools(opts ...ToolOption) []Tool { return core.SubagentTools(opts...) }
