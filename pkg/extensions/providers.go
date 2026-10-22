package extensions

import internalext "github.com/mark3labs/kit/internal/extensions"

// ProviderDef is the concrete extension provider protocol type.
type ProviderDef = internalext.ProviderDef

// ProviderCapabilities is the concrete extension provider protocol type.
type ProviderCapabilities = internalext.ProviderCapabilities

// ProviderModel is the concrete extension provider protocol type.
type ProviderModel = internalext.ProviderModel

// ProviderPrice is the concrete extension provider protocol type.
type ProviderPrice = internalext.ProviderPrice

// ProviderAuth is the concrete extension provider protocol type.
type ProviderAuth = internalext.ProviderAuth

// ProviderAuthMode is the concrete extension provider protocol type.
type ProviderAuthMode = internalext.ProviderAuthMode

// ProviderAuthRequest is the concrete extension provider protocol type.
type ProviderAuthRequest = internalext.ProviderAuthRequest

// ProviderAuthResult is the concrete extension provider protocol type.
type ProviderAuthResult = internalext.ProviderAuthResult

// ProviderAuthStatus is the concrete extension provider protocol type.
type ProviderAuthStatus = internalext.ProviderAuthStatus

// ProviderCreateRequest is the concrete extension provider protocol type.
type ProviderCreateRequest = internalext.ProviderCreateRequest

// ProviderCreateResult is the concrete extension provider protocol type.
type ProviderCreateResult = internalext.ProviderCreateResult

// ProviderRuntime is the concrete extension provider protocol type.
type ProviderRuntime = internalext.ProviderRuntime

// ProviderResult is the concrete extension provider protocol type.
type ProviderResult = internalext.ProviderResult

// ProviderError is the concrete extension provider protocol type.
type ProviderError = internalext.ProviderError

// ProviderParameters is the concrete extension provider protocol type.
type ProviderParameters = internalext.ProviderParameters

// ProviderRequest is the concrete extension provider protocol type.
type ProviderRequest = internalext.ProviderRequest

// ProviderTool is the concrete extension provider protocol type.
type ProviderTool = internalext.ProviderTool

// ProviderToolChoice is the concrete extension provider protocol type.
type ProviderToolChoice = internalext.ProviderToolChoice

// ProviderToolChoiceType is the concrete extension provider protocol type.
type ProviderToolChoiceType = internalext.ProviderToolChoiceType

// ProviderReplayData is the concrete extension provider protocol type.
type ProviderReplayData = internalext.ProviderReplayData

// ProviderMessage is the concrete extension provider protocol type.
type ProviderMessage = internalext.ProviderMessage

// ProviderPart is the concrete extension provider protocol type.
type ProviderPart = internalext.ProviderPart

// ProviderPartType is the concrete extension provider protocol type.
type ProviderPartType = internalext.ProviderPartType

// ProviderEvent is the concrete extension provider protocol type.
type ProviderEvent = internalext.ProviderEvent

// ProviderEventType is the concrete extension provider protocol type.
type ProviderEventType = internalext.ProviderEventType

// ProviderFinishReason is the concrete extension provider protocol type.
type ProviderFinishReason = internalext.ProviderFinishReason

// ProviderUsage is the concrete extension provider protocol type.
type ProviderUsage = internalext.ProviderUsage

const (
	// ProviderAuthNone is the corresponding extension protocol value.
	ProviderAuthNone = internalext.ProviderAuthNone
	// ProviderAuthAPIKey is the corresponding extension protocol value.
	ProviderAuthAPIKey = internalext.ProviderAuthAPIKey
	// ProviderAuthExternal is the corresponding extension protocol value.
	ProviderAuthExternal = internalext.ProviderAuthExternal
	// ProviderAuthConfigured is the corresponding extension protocol value.
	ProviderAuthConfigured = internalext.ProviderAuthConfigured
	// ProviderAuthUnconfigured is the corresponding extension protocol value.
	ProviderAuthUnconfigured = internalext.ProviderAuthUnconfigured
	// ProviderAuthUnknown is the corresponding extension protocol value.
	ProviderAuthUnknown = internalext.ProviderAuthUnknown
	// ProviderToolAuto is the corresponding extension protocol value.
	ProviderToolAuto = internalext.ProviderToolAuto
	// ProviderToolNone is the corresponding extension protocol value.
	ProviderToolNone = internalext.ProviderToolNone
	// ProviderToolRequired is the corresponding extension protocol value.
	ProviderToolRequired = internalext.ProviderToolRequired
	// ProviderToolNamed is the corresponding extension protocol value.
	ProviderToolNamed = internalext.ProviderToolNamed
	// ProviderPartText is the corresponding extension protocol value.
	ProviderPartText = internalext.ProviderPartText
	// ProviderPartReasoning is the corresponding extension protocol value.
	ProviderPartReasoning = internalext.ProviderPartReasoning
	// ProviderPartFile is the corresponding extension protocol value.
	ProviderPartFile = internalext.ProviderPartFile
	// ProviderPartToolCall is the corresponding extension protocol value.
	ProviderPartToolCall = internalext.ProviderPartToolCall
	// ProviderPartToolResult is the corresponding extension protocol value.
	ProviderPartToolResult = internalext.ProviderPartToolResult
	// ProviderTextStart is the corresponding extension protocol value.
	ProviderTextStart = internalext.ProviderTextStart
	// ProviderTextDelta is the corresponding extension protocol value.
	ProviderTextDelta = internalext.ProviderTextDelta
	// ProviderTextEnd is the corresponding extension protocol value.
	ProviderTextEnd = internalext.ProviderTextEnd
	// ProviderReasoningStart is the corresponding extension protocol value.
	ProviderReasoningStart = internalext.ProviderReasoningStart
	// ProviderReasoningDelta is the corresponding extension protocol value.
	ProviderReasoningDelta = internalext.ProviderReasoningDelta
	// ProviderReasoningEnd is the corresponding extension protocol value.
	ProviderReasoningEnd = internalext.ProviderReasoningEnd
	// ProviderToolStart is the corresponding extension protocol value.
	ProviderToolStart = internalext.ProviderToolStart
	// ProviderToolDelta is the corresponding extension protocol value.
	ProviderToolDelta = internalext.ProviderToolDelta
	// ProviderToolEnd is the corresponding extension protocol value.
	ProviderToolEnd = internalext.ProviderToolEnd
	// ProviderUsageEvent is the corresponding extension protocol value.
	ProviderUsageEvent = internalext.ProviderUsageEvent
	// ProviderFinish is the corresponding extension protocol value.
	ProviderFinish = internalext.ProviderFinish
	// ProviderFinishStop is the corresponding extension protocol value.
	ProviderFinishStop = internalext.ProviderFinishStop
	// ProviderFinishLength is the corresponding extension protocol value.
	ProviderFinishLength = internalext.ProviderFinishLength
	// ProviderFinishToolCalls is the corresponding extension protocol value.
	ProviderFinishToolCalls = internalext.ProviderFinishToolCalls
)
