package kit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"slices"
	"strings"
	"sync"

	"charm.land/fantasy"

	"github.com/mark3labs/kit/internal/core"
	"github.com/spf13/viper"
)

// Tool is the interface that all Kit tools implement.
type Tool = fantasy.AgentTool

// ToolOption configures tool behavior.
type ToolOption = core.ToolOption

// WithWorkDir sets the working directory for file-based tools.
// If empty, os.Getwd() is used at execution time.
var WithWorkDir = core.WithWorkDir

// --- Core Tool Validation ---
// processes a list of tool names, if disableCoreTools is true, return an
// empty list. Otherwise if coreTools is not empty, it will return a list of
// all valid names ie those found in ListAllCoreToolNames(),
// otherwise, it will return all of ListAllCoreToolNames().
func handleCoreToolList(coreTools []string, disableCoreTools bool) []string {
	var result []string
	if disableCoreTools {
		return result
	}
	allTools := ListAllCoreToolNames()
	if len(coreTools) > 0 {
		for _, tool := range allTools {
			for _, t := range coreTools {
				if t == tool {
					result = append(result, t)
					continue
				}
			}
		}
		return result
	} else {
		return allTools
	}
}

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

	// FinalValue, when Halt is true, is propagated to the turn's
	// [TurnResult.FinalValue] so the caller can recover a typed result
	// produced by the tool (e.g. a structured "finish" tool). The dynamic
	// type is whatever the tool handler stored. Ignored when Halt is false.
	FinalValue any

	// Halt, when true, signals that the agent loop should terminate after
	// this tool call. Content is still returned to the model for the current
	// step, but [TurnResult.FinalValue] and [TurnResult.HaltedByTool] are
	// populated so embedders building structured-result extraction patterns
	// (model calls a finish(...) tool, the loop ends, the typed value is
	// returned) no longer need a side-channel.
	Halt bool
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

// haltHolderKey is the context key for the per-turn halt holder. It is
// injected by runTurn so tool handlers created with [NewTool],
// [NewParallelTool], and [NewRawTool] can signal loop termination and carry a
// final value out to the [TurnResult] without an embedder-side side-channel.
type haltHolderKey struct{}

// haltHolder captures a Halt signal raised by a tool handler during a turn.
type haltHolder struct {
	mu       sync.Mutex
	halted   bool
	toolName string
	value    any
}

func (h *haltHolder) set(toolName string, value any) {
	h.mu.Lock()
	defer h.mu.Unlock()
	// First halt wins so the earliest finishing tool determines the result.
	if h.halted {
		return
	}
	h.halted = true
	h.toolName = toolName
	h.value = value
}

func (h *haltHolder) snapshot() (bool, string, any) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.halted, h.toolName, h.value
}

// recordHalt records a Halt signal from a tool handler onto the per-turn halt
// holder, if one is present in the context.
func recordHalt(ctx context.Context, toolName string, result ToolOutput) {
	if !result.Halt {
		return
	}
	if holder, ok := ctx.Value(haltHolderKey{}).(*haltHolder); ok && holder != nil {
		holder.set(toolName, result.FinalValue)
	}
}

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
			recordHalt(ctx, name, result)
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
			recordHalt(ctx, name, result)
			return toolOutputToResponse(result), nil
		},
	)
}

// rawToolInput is the decoded carrier used by [NewRawTool]. Using
// json.RawMessage lets the typed-tool machinery in fantasy generate a
// permissive object schema while we forward the raw arguments to the handler
// as a decoded map.
type rawToolInput = json.RawMessage

// NewRawTool is the schema-driven counterpart to [NewTool]. Use it when the
// tool's input shape isn't known at compile time — for example tools loaded
// from JSON Schema definitions in skill files or MCP server catalogs.
//
// schema must be a valid JSON Schema describing the tool's input object; it is
// advertised to the model as the tool's parameter schema. fn receives the
// decoded JSON arguments as a map and returns a [ToolOutput]. Like [NewTool],
// the tool call ID is injected into the context and can be retrieved with
// [ToolCallIDFromContext], and [ToolOutput.Halt] is honored.
//
// If the model sends arguments that are not a valid JSON object the call
// short-circuits with an error [ToolResponse] before fn is invoked.
func NewRawTool(
	name, description string,
	schema map[string]any,
	fn func(ctx context.Context, args map[string]any) (ToolOutput, error),
) Tool {
	tool := fantasy.NewAgentTool(name, description,
		func(ctx context.Context, input rawToolInput, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			ctx = context.WithValue(ctx, toolCallIDKey{}, call.ID)
			args := map[string]any{}
			// Normalise whitespace before the null/empty guard so values like
			// " null " or "\tnull\n" take the same skip-unmarshal path as the
			// bare "null" and the handler always receives a non-nil empty map.
			// (fantasy currently trims via its RawMessage decode, but this keeps
			// the guard correct independent of that upstream behaviour.)
			trimmed := bytes.TrimSpace(input)
			if len(trimmed) > 0 && !bytes.Equal(trimmed, []byte("null")) {
				if err := json.Unmarshal(trimmed, &args); err != nil {
					return fantasy.NewTextErrorResponse(fmt.Sprintf("invalid arguments for tool %q: %v", name, err)), nil
				}
			}
			result, err := fn(ctx, args)
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}
			recordHalt(ctx, name, result)
			return toolOutputToResponse(result), nil
		},
	)
	// Override the auto-generated schema with the caller-supplied one so the
	// model sees the real input shape instead of the permissive raw-message
	// schema.
	if len(schema) > 0 {
		info := tool.Info()
		info.Parameters = schema
		info.Required = requiredFromSchema(schema)
		tool = &schemaOverrideTool{AgentTool: tool, info: info}
	}
	return tool
}

// schemaOverrideTool wraps an [fantasy.AgentTool] to advertise a
// caller-supplied JSON Schema instead of the auto-generated one. Used by
// [NewRawTool].
type schemaOverrideTool struct {
	fantasy.AgentTool
	info fantasy.ToolInfo
}

// Info returns the tool info carrying the overridden parameter schema.
func (t *schemaOverrideTool) Info() fantasy.ToolInfo { return t.info }

// requiredFromSchema extracts the top-level "required" array from a JSON
// Schema object, tolerating both []string and []any element types.
func requiredFromSchema(schema map[string]any) []string {
	raw, ok := schema["required"]
	if !ok {
		return nil
	}
	switch v := raw.(type) {
	case []string:
		return v
	case []any:
		out := make([]string, 0, len(v))
		for _, e := range v {
			if s, ok := e.(string); ok {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
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
func ListAllCoreToolNames() []string { return core.ListAllCoreToolNames() }

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

// FilterCoreToolNames resolves the effective core tool list from include and
// exclude name lists. At most one of include/exclude may be non-empty.
// Unknown tool names are skipped with a warning. If both lists are empty the
// returned list is nil, meaning "all core tools".
func FilterCoreToolNames(includeTools, excludeTools []string) ([]string, error) {
	if len(includeTools) > 0 && len(excludeTools) > 0 {
		return nil, fmt.Errorf("cannot use both include-core-tools and exclude-core-tools options")
	}

	var coreToolList []string
	if len(includeTools) > 0 || len(excludeTools) > 0 {
		allCoreTools := ListAllCoreToolNames()
		if len(includeTools) > 0 {
			for _, t := range includeTools {
				if !slices.Contains(allCoreTools, t) {
					log.Printf("Warning: invalid core tool: %s", t)
					continue
				}
			}
			coreToolList = includeTools
		} else {
			for _, t := range excludeTools {
				if !slices.Contains(allCoreTools, t) {
					log.Printf("Warning: invalid core tool: %s", t)
					continue
				}
			}
			for _, t := range allCoreTools {
				if !slices.Contains(excludeTools, t) {
					coreToolList = append(coreToolList, t)
				}
			}
		}
	}
	return coreToolList, nil
}

// CoreToolFilterHelper reads the include-core-tools/exclude-core-tools keys
// from a configuration store and resolves the effective core tool list.
//
// Deprecated: Use FilterCoreToolNames instead, which takes the include and
// exclude lists directly and does not expose the configuration library.
func CoreToolFilterHelper(v *viper.Viper) ([]string, error) {
	return FilterCoreToolNames(v.GetStringSlice("include-core-tools"), v.GetStringSlice("exclude-core-tools"))
}
