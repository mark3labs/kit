package kit

import "charm.land/fantasy"

// ==== Custom Provider Types ====
//
// These aliases name every type that an [LLMLanguageModel] implementation
// touches, so an application can write a custom model for a
// [ProviderFactory] without importing the provider package directly.

// LLMCall is the request that Kit passes to [LLMLanguageModel] Generate and
// Stream. It carries the prompt, generation parameters, tool definitions
// and provider options for one model step.
type LLMCall = fantasy.Call

// LLMCallWarning reports a non-fatal problem with a call, such as a setting
// or tool that the model does not support. The call still runs.
type LLMCallWarning = fantasy.CallWarning

// LLMCallWarningType classifies an [LLMCallWarning].
type LLMCallWarningType = fantasy.CallWarningType

// Call warning type constants.
const (
	// LLMCallWarningTypeUnsupportedSetting reports an unsupported setting.
	LLMCallWarningTypeUnsupportedSetting = fantasy.CallWarningTypeUnsupportedSetting
	// LLMCallWarningTypeUnsupportedTool reports an unsupported tool.
	LLMCallWarningTypeUnsupportedTool = fantasy.CallWarningTypeUnsupportedTool
	// LLMCallWarningTypeOther reports any other warning.
	LLMCallWarningTypeOther = fantasy.CallWarningTypeOther
)

// LLMToolDefinition is a tool definition that Kit sends to the model in
// [LLMCall].Tools. It is an [LLMFunctionTool] or an [LLMProviderDefinedTool].
// It is not the executable [Tool] type.
type LLMToolDefinition = fantasy.Tool

// LLMToolType classifies an [LLMToolDefinition].
type LLMToolType = fantasy.ToolType

// Tool definition type constants.
const (
	// LLMToolTypeFunction identifies an [LLMFunctionTool].
	LLMToolTypeFunction = fantasy.ToolTypeFunction
	// LLMToolTypeProviderDefined identifies an [LLMProviderDefinedTool].
	LLMToolTypeProviderDefined = fantasy.ToolTypeProviderDefined
)

// LLMFunctionTool is a function tool definition with a name, a description
// and a JSON Schema for its input.
type LLMFunctionTool = fantasy.FunctionTool

// LLMProviderDefinedTool is a tool whose schema and behavior the model
// provider defines, such as a hosted web search.
type LLMProviderDefinedTool = fantasy.ProviderDefinedTool

// LLMStreamResponse is the sequence of [LLMStreamPart] values that
// [LLMLanguageModel] Stream returns.
type LLMStreamResponse = fantasy.StreamResponse

// LLMStreamPart is one event in an [LLMStreamResponse]. Its Type field
// tells which other fields are set.
type LLMStreamPart = fantasy.StreamPart

// LLMStreamPartType classifies an [LLMStreamPart].
type LLMStreamPartType = fantasy.StreamPartType

// Stream part type constants. A text or reasoning block is a Start part, zero
// or more Delta parts and an End part, all with the same ID. A stream ends
// with an [LLMStreamPartTypeFinish] part.
const (
	// LLMStreamPartTypeWarnings carries call warnings.
	LLMStreamPartTypeWarnings = fantasy.StreamPartTypeWarnings
	// LLMStreamPartTypeTextStart starts a text block.
	LLMStreamPartTypeTextStart = fantasy.StreamPartTypeTextStart
	// LLMStreamPartTypeTextDelta carries text for an open text block.
	LLMStreamPartTypeTextDelta = fantasy.StreamPartTypeTextDelta
	// LLMStreamPartTypeTextEnd ends a text block.
	LLMStreamPartTypeTextEnd = fantasy.StreamPartTypeTextEnd
	// LLMStreamPartTypeReasoningStart starts a reasoning block.
	LLMStreamPartTypeReasoningStart = fantasy.StreamPartTypeReasoningStart
	// LLMStreamPartTypeReasoningDelta carries text for an open reasoning block.
	LLMStreamPartTypeReasoningDelta = fantasy.StreamPartTypeReasoningDelta
	// LLMStreamPartTypeReasoningEnd ends a reasoning block.
	LLMStreamPartTypeReasoningEnd = fantasy.StreamPartTypeReasoningEnd
	// LLMStreamPartTypeToolInputStart starts the streamed input of a tool call.
	LLMStreamPartTypeToolInputStart = fantasy.StreamPartTypeToolInputStart
	// LLMStreamPartTypeToolInputDelta carries part of a tool call input.
	LLMStreamPartTypeToolInputDelta = fantasy.StreamPartTypeToolInputDelta
	// LLMStreamPartTypeToolInputEnd ends the streamed input of a tool call.
	LLMStreamPartTypeToolInputEnd = fantasy.StreamPartTypeToolInputEnd
	// LLMStreamPartTypeToolCall carries a complete tool call.
	LLMStreamPartTypeToolCall = fantasy.StreamPartTypeToolCall
	// LLMStreamPartTypeToolResult carries the result of a provider-executed tool.
	LLMStreamPartTypeToolResult = fantasy.StreamPartTypeToolResult
	// LLMStreamPartTypeSource carries a source citation.
	LLMStreamPartTypeSource = fantasy.StreamPartTypeSource
	// LLMStreamPartTypeFinish ends the stream with a finish reason and usage.
	LLMStreamPartTypeFinish = fantasy.StreamPartTypeFinish
	// LLMStreamPartTypeError reports a stream error.
	LLMStreamPartTypeError = fantasy.StreamPartTypeError
	// LLMStreamPartTypeKeepalive carries no content. It only shows that the
	// stream is still alive.
	LLMStreamPartTypeKeepalive = fantasy.StreamPartTypeKeepalive
)

// LLMResponseContent is the ordered content of an [LLMResponse].
type LLMResponseContent = fantasy.ResponseContent

// LLMContent is one item of [LLMResponseContent]. Use [AsLLMContent] to get
// the concrete type.
type LLMContent = fantasy.Content

// LLMContentType classifies an [LLMContent] value.
type LLMContentType = fantasy.ContentType

// Response content type constants.
const (
	// LLMContentTypeText identifies [LLMTextContent].
	LLMContentTypeText = fantasy.ContentTypeText
	// LLMContentTypeReasoning identifies [LLMReasoningContent].
	LLMContentTypeReasoning = fantasy.ContentTypeReasoning
	// LLMContentTypeFile identifies [LLMFileContent].
	LLMContentTypeFile = fantasy.ContentTypeFile
	// LLMContentTypeSource identifies [LLMSourceContent].
	LLMContentTypeSource = fantasy.ContentTypeSource
	// LLMContentTypeToolCall identifies [LLMToolCallContent].
	LLMContentTypeToolCall = fantasy.ContentTypeToolCall
	// LLMContentTypeToolResult identifies [LLMToolResultContent].
	LLMContentTypeToolResult = fantasy.ContentTypeToolResult
)

// LLMTextContent is text that the model generated.
type LLMTextContent = fantasy.TextContent

// LLMReasoningContent is reasoning that the model generated.
type LLMReasoningContent = fantasy.ReasoningContent

// LLMFileContent is a file that the model generated.
type LLMFileContent = fantasy.FileContent

// LLMSourceContent is a source that the model used for its response.
type LLMSourceContent = fantasy.SourceContent

// LLMSourceType classifies an [LLMSourceContent] or a source [LLMStreamPart].
type LLMSourceType = fantasy.SourceType

// Source type constants.
const (
	// LLMSourceTypeURL identifies a URL source.
	LLMSourceTypeURL = fantasy.SourceTypeURL
	// LLMSourceTypeDocument identifies a document source.
	LLMSourceTypeDocument = fantasy.SourceTypeDocument
)

// LLMToolCallContent is a tool call that the model generated. Input is the
// JSON-encoded tool arguments.
type LLMToolCallContent = fantasy.ToolCallContent

// LLMToolResultContent is the result of a tool that the provider executed.
type LLMToolResultContent = fantasy.ToolResultContent

// AsLLMContent converts content to the concrete content type T. It accepts
// both value and pointer forms and reports false when content is not a T.
func AsLLMContent[T LLMContent](content LLMContent) (T, bool) {
	return fantasy.AsContentType[T](content)
}

// LLMObjectCall is the request that Kit passes to [LLMLanguageModel]
// GenerateObject and StreamObject for structured output.
type LLMObjectCall = fantasy.ObjectCall

// LLMObjectResponse is the result of [LLMLanguageModel] GenerateObject.
type LLMObjectResponse = fantasy.ObjectResponse

// LLMObjectStreamResponse is the sequence of [LLMObjectStreamPart] values
// that [LLMLanguageModel] StreamObject returns.
type LLMObjectStreamResponse = fantasy.ObjectStreamResponse

// LLMObjectStreamPart is one event in an [LLMObjectStreamResponse].
type LLMObjectStreamPart = fantasy.ObjectStreamPart

// LLMObjectStreamPartType classifies an [LLMObjectStreamPart].
type LLMObjectStreamPartType = fantasy.ObjectStreamPartType

// Object stream part type constants.
const (
	// LLMObjectStreamPartTypeObject carries a new partial object.
	LLMObjectStreamPartTypeObject = fantasy.ObjectStreamPartTypeObject
	// LLMObjectStreamPartTypeTextDelta carries generated text.
	LLMObjectStreamPartTypeTextDelta = fantasy.ObjectStreamPartTypeTextDelta
	// LLMObjectStreamPartTypeError reports a stream error.
	LLMObjectStreamPartTypeError = fantasy.ObjectStreamPartTypeError
	// LLMObjectStreamPartTypeFinish ends the stream.
	LLMObjectStreamPartTypeFinish = fantasy.ObjectStreamPartTypeFinish
)
