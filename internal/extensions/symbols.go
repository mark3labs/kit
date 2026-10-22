package extensions

import (
	"reflect"

	"github.com/mark3labs/kit/internal/yaegi/interp"
)

// Symbols returns the Yaegi export table that makes KIT's extension API
// available to interpreted Go code. Extensions import these types as:
//
//	import "kit/ext"
//
// IMPORTANT: Only concrete types (structs, constants) are exported.
//
// Yaegi cannot SYNTHESIZE an interface wrapper at runtime: when interpreted
// code must produce a value of a host interface type, it panics in
// genInterfaceWrapper. Two consequences shape this table:
//
//  1. The Event, Result and HandlerFunc types are not exported, so there is
//     no generic On(EventType, HandlerFunc). Extensions use event-specific
//     methods like api.OnToolCall() with concrete function signatures.
//     Measured: a handler returning a concrete Result works, but the
//     idiomatic `return nil` panics during Eval — which is why the generic
//     form is not offered.
//  2. Build-time wrappers (the `_Iface` structs that `yaegi extract`
//     generates) DO work, and would let interpreted types satisfy host
//     interfaces. Adopting them additionally requires the export key to
//     match the type's real Go package path — the virtual "kit/ext" path
//     works for structs but yields a nil dereference for interfaces.
func Symbols() interp.Exports {
	return interp.Exports{
		"kit/ext/ext": map[string]reflect.Value{
			// Concrete provider protocol types and typed constants.
			"ProviderDef":              reflect.ValueOf((*ProviderDef)(nil)),
			"ProviderCapabilities":     reflect.ValueOf((*ProviderCapabilities)(nil)),
			"ProviderModel":            reflect.ValueOf((*ProviderModel)(nil)),
			"ProviderPrice":            reflect.ValueOf((*ProviderPrice)(nil)),
			"ProviderAuth":             reflect.ValueOf((*ProviderAuth)(nil)),
			"ProviderAuthMode":         reflect.ValueOf((*ProviderAuthMode)(nil)),
			"ProviderAuthRequest":      reflect.ValueOf((*ProviderAuthRequest)(nil)),
			"ProviderAuthResult":       reflect.ValueOf((*ProviderAuthResult)(nil)),
			"ProviderAuthStatus":       reflect.ValueOf((*ProviderAuthStatus)(nil)),
			"ProviderCreateRequest":    reflect.ValueOf((*ProviderCreateRequest)(nil)),
			"ProviderCreateResult":     reflect.ValueOf((*ProviderCreateResult)(nil)),
			"ProviderRuntime":          reflect.ValueOf((*ProviderRuntime)(nil)),
			"ProviderResult":           reflect.ValueOf((*ProviderResult)(nil)),
			"ProviderError":            reflect.ValueOf((*ProviderError)(nil)),
			"ProviderParameters":       reflect.ValueOf((*ProviderParameters)(nil)),
			"ProviderRequest":          reflect.ValueOf((*ProviderRequest)(nil)),
			"ProviderTool":             reflect.ValueOf((*ProviderTool)(nil)),
			"ProviderToolChoice":       reflect.ValueOf((*ProviderToolChoice)(nil)),
			"ProviderToolChoiceType":   reflect.ValueOf((*ProviderToolChoiceType)(nil)),
			"ProviderReplayData":       reflect.ValueOf((*ProviderReplayData)(nil)),
			"ProviderMessage":          reflect.ValueOf((*ProviderMessage)(nil)),
			"ProviderPart":             reflect.ValueOf((*ProviderPart)(nil)),
			"ProviderPartType":         reflect.ValueOf((*ProviderPartType)(nil)),
			"ProviderEvent":            reflect.ValueOf((*ProviderEvent)(nil)),
			"ProviderEventType":        reflect.ValueOf((*ProviderEventType)(nil)),
			"ProviderFinishReason":     reflect.ValueOf((*ProviderFinishReason)(nil)),
			"ProviderUsage":            reflect.ValueOf((*ProviderUsage)(nil)),
			"ProviderAuthNone":         reflect.ValueOf(ProviderAuthNone),
			"ProviderAuthAPIKey":       reflect.ValueOf(ProviderAuthAPIKey),
			"ProviderAuthExternal":     reflect.ValueOf(ProviderAuthExternal),
			"ProviderAuthConfigured":   reflect.ValueOf(ProviderAuthConfigured),
			"ProviderAuthUnconfigured": reflect.ValueOf(ProviderAuthUnconfigured),
			"ProviderAuthUnknown":      reflect.ValueOf(ProviderAuthUnknown),
			"ProviderToolAuto":         reflect.ValueOf(ProviderToolAuto),
			"ProviderToolNone":         reflect.ValueOf(ProviderToolNone),
			"ProviderToolRequired":     reflect.ValueOf(ProviderToolRequired),
			"ProviderToolNamed":        reflect.ValueOf(ProviderToolNamed),
			"ProviderPartText":         reflect.ValueOf(ProviderPartText),
			"ProviderPartReasoning":    reflect.ValueOf(ProviderPartReasoning),
			"ProviderPartFile":         reflect.ValueOf(ProviderPartFile),
			"ProviderPartToolCall":     reflect.ValueOf(ProviderPartToolCall),
			"ProviderPartToolResult":   reflect.ValueOf(ProviderPartToolResult),
			"ProviderTextStart":        reflect.ValueOf(ProviderTextStart),
			"ProviderTextDelta":        reflect.ValueOf(ProviderTextDelta),
			"ProviderTextEnd":          reflect.ValueOf(ProviderTextEnd),
			"ProviderReasoningStart":   reflect.ValueOf(ProviderReasoningStart),
			"ProviderReasoningDelta":   reflect.ValueOf(ProviderReasoningDelta),
			"ProviderReasoningEnd":     reflect.ValueOf(ProviderReasoningEnd),
			"ProviderToolStart":        reflect.ValueOf(ProviderToolStart),
			"ProviderToolDelta":        reflect.ValueOf(ProviderToolDelta),
			"ProviderToolEnd":          reflect.ValueOf(ProviderToolEnd),
			"ProviderUsageEvent":       reflect.ValueOf(ProviderUsageEvent),
			"ProviderFinish":           reflect.ValueOf(ProviderFinish),
			"ProviderFinishStop":       reflect.ValueOf(ProviderFinishStop),
			"ProviderFinishLength":     reflect.ValueOf(ProviderFinishLength),
			"ProviderFinishToolCalls":  reflect.ValueOf(ProviderFinishToolCalls),

			// Struct types (nil pointer trick for type registration)
			"API":            reflect.ValueOf((*API)(nil)),
			"Context":        reflect.ValueOf((*Context)(nil)),
			"ToolDef":        reflect.ValueOf((*ToolDef)(nil)),
			"ToolContext":    reflect.ValueOf((*ToolContext)(nil)),
			"ShortcutDef":    reflect.ValueOf((*ShortcutDef)(nil)),
			"CommandDef":     reflect.ValueOf((*CommandDef)(nil)),
			"PrintBlockOpts": reflect.ValueOf((*PrintBlockOpts)(nil)),

			// Sentinel errors. Extensions detect them with errors.Is:
			//
			//   if errors.Is(err, ext.ErrAgentBusy) { ... }
			"ErrAgentBusy": reflect.ValueOf(&ErrAgentBusy).Elem(),

			// Session types
			"SessionMessage": reflect.ValueOf((*SessionMessage)(nil)),
			"ExtensionEntry": reflect.ValueOf((*ExtensionEntry)(nil)),
			"SessionUsage":   reflect.ValueOf((*SessionUsage)(nil)),

			// Option types
			"OptionDef": reflect.ValueOf((*OptionDef)(nil)),

			// Model info types
			"ModelInfoEntry": reflect.ValueOf((*ModelInfoEntry)(nil)),

			// Tool info types
			"ToolInfo": reflect.ValueOf((*ToolInfo)(nil)),

			// LLM completion types
			"CompleteRequest":  reflect.ValueOf((*CompleteRequest)(nil)),
			"CompleteResponse": reflect.ValueOf((*CompleteResponse)(nil)),
			"CompactConfig":    reflect.ValueOf((*CompactConfig)(nil)),
			"FilePart":         reflect.ValueOf((*FilePart)(nil)),

			// Status bar types
			"StatusBarEntry": reflect.ValueOf((*StatusBarEntry)(nil)),

			// Widget types
			"WidgetConfig":    reflect.ValueOf((*WidgetConfig)(nil)),
			"WidgetContent":   reflect.ValueOf((*WidgetContent)(nil)),
			"WidgetStyle":     reflect.ValueOf((*WidgetStyle)(nil)),
			"WidgetPlacement": reflect.ValueOf((*WidgetPlacement)(nil)),
			"WidgetAbove":     reflect.ValueOf(WidgetAbove),
			"WidgetBelow":     reflect.ValueOf(WidgetBelow),

			// Header/Footer types
			"HeaderFooterConfig": reflect.ValueOf((*HeaderFooterConfig)(nil)),

			// UI visibility
			"UIVisibility": reflect.ValueOf((*UIVisibility)(nil)),

			// Context stats
			"ContextStats": reflect.ValueOf((*ContextStats)(nil)),

			// Overlay types
			"OverlayAnchor":       reflect.ValueOf((*OverlayAnchor)(nil)),
			"OverlayCenter":       reflect.ValueOf(OverlayCenter),
			"OverlayTopCenter":    reflect.ValueOf(OverlayTopCenter),
			"OverlayBottomCenter": reflect.ValueOf(OverlayBottomCenter),
			"OverlayStyle":        reflect.ValueOf((*OverlayStyle)(nil)),
			"OverlayConfig":       reflect.ValueOf((*OverlayConfig)(nil)),
			"OverlayResult":       reflect.ValueOf((*OverlayResult)(nil)),

			// Tool renderer types
			"ToolRenderConfig": reflect.ValueOf((*ToolRenderConfig)(nil)),

			// Message renderer types
			"MessageRendererConfig": reflect.ValueOf((*MessageRendererConfig)(nil)),

			// Editor interceptor types
			"EditorKeyActionType":  reflect.ValueOf((*EditorKeyActionType)(nil)),
			"EditorKeyPassthrough": reflect.ValueOf(EditorKeyPassthrough),
			"EditorKeyConsumed":    reflect.ValueOf(EditorKeyConsumed),
			"EditorKeyRemap":       reflect.ValueOf(EditorKeyRemap),
			"EditorKeySubmit":      reflect.ValueOf(EditorKeySubmit),
			"EditorKeyAction":      reflect.ValueOf((*EditorKeyAction)(nil)),
			"EditorConfig":         reflect.ValueOf((*EditorConfig)(nil)),

			// Prompt types
			"PromptSelectConfig":      reflect.ValueOf((*PromptSelectConfig)(nil)),
			"PromptSelectResult":      reflect.ValueOf((*PromptSelectResult)(nil)),
			"PromptConfirmConfig":     reflect.ValueOf((*PromptConfirmConfig)(nil)),
			"PromptConfirmResult":     reflect.ValueOf((*PromptConfirmResult)(nil)),
			"PromptInputConfig":       reflect.ValueOf((*PromptInputConfig)(nil)),
			"PromptInputResult":       reflect.ValueOf((*PromptInputResult)(nil)),
			"PromptMultiSelectConfig": reflect.ValueOf((*PromptMultiSelectConfig)(nil)),
			"PromptMultiSelectResult": reflect.ValueOf((*PromptMultiSelectResult)(nil)),

			// Context filtering types
			"ContextMessage":       reflect.ValueOf((*ContextMessage)(nil)),
			"ContextPrepareEvent":  reflect.ValueOf((*ContextPrepareEvent)(nil)),
			"ContextPrepareResult": reflect.ValueOf((*ContextPrepareResult)(nil)),

			// Session lifecycle types
			"BeforeForkEvent":           reflect.ValueOf((*BeforeForkEvent)(nil)),
			"BeforeForkResult":          reflect.ValueOf((*BeforeForkResult)(nil)),
			"BeforeSessionSwitchEvent":  reflect.ValueOf((*BeforeSessionSwitchEvent)(nil)),
			"BeforeSessionSwitchResult": reflect.ValueOf((*BeforeSessionSwitchResult)(nil)),
			"BeforeCompactEvent":        reflect.ValueOf((*BeforeCompactEvent)(nil)),
			"BeforeCompactResult":       reflect.ValueOf((*BeforeCompactResult)(nil)),

			// Subagent types
			"SubagentConfig": reflect.ValueOf((*SubagentConfig)(nil)),
			"SubagentResult": reflect.ValueOf((*SubagentResult)(nil)),
			"SubagentUsage":  reflect.ValueOf((*SubagentUsage)(nil)),
			"SubagentHandle": reflect.ValueOf((*SubagentHandle)(nil)),
			"SubagentEvent":  reflect.ValueOf((*SubagentEvent)(nil)),

			// Subagent lifecycle events
			"SubagentStartEvent": reflect.ValueOf((*SubagentStartEvent)(nil)),
			"SubagentChunkEvent": reflect.ValueOf((*SubagentChunkEvent)(nil)),
			"SubagentEndEvent":   reflect.ValueOf((*SubagentEndEvent)(nil)),

			// Theme types
			"ThemeColor":       reflect.ValueOf((*ThemeColor)(nil)),
			"ThemeColorConfig": reflect.ValueOf((*ThemeColorConfig)(nil)),
			"ThemeColors":      reflect.ValueOf((*ThemeColors)(nil)),

			// Tree navigation types
			"TreeNode":             reflect.ValueOf((*TreeNode)(nil)),
			"TreeNavigationResult": reflect.ValueOf((*TreeNavigationResult)(nil)),

			// Skill types
			"Skill":           reflect.ValueOf((*Skill)(nil)),
			"SkillLoadResult": reflect.ValueOf((*SkillLoadResult)(nil)),

			// Template parsing types
			"PromptTemplate":   reflect.ValueOf((*PromptTemplate)(nil)),
			"ArgumentPattern":  reflect.ValueOf((*ArgumentPattern)(nil)),
			"ParseResult":      reflect.ValueOf((*ParseResult)(nil)),
			"ModelConditional": reflect.ValueOf((*ModelConditional)(nil)),

			// Model resolution types
			"ModelCapabilities":     reflect.ValueOf((*ModelCapabilities)(nil)),
			"ModelPricing":          reflect.ValueOf((*ModelPricing)(nil)),
			"ModelResolutionResult": reflect.ValueOf((*ModelResolutionResult)(nil)),

			// Event structs
			"ToolCallEvent":            reflect.ValueOf((*ToolCallEvent)(nil)),
			"ToolCallResult":           reflect.ValueOf((*ToolCallResult)(nil)),
			"ToolCallInputStartEvent":  reflect.ValueOf((*ToolCallInputStartEvent)(nil)),
			"ToolCallInputDeltaEvent":  reflect.ValueOf((*ToolCallInputDeltaEvent)(nil)),
			"ToolCallInputEndEvent":    reflect.ValueOf((*ToolCallInputEndEvent)(nil)),
			"ToolExecutionStartEvent":  reflect.ValueOf((*ToolExecutionStartEvent)(nil)),
			"ToolExecutionEndEvent":    reflect.ValueOf((*ToolExecutionEndEvent)(nil)),
			"ToolOutputEvent":          reflect.ValueOf((*ToolOutputEvent)(nil)),
			"ToolResultEvent":          reflect.ValueOf((*ToolResultEvent)(nil)),
			"ToolResultResult":         reflect.ValueOf((*ToolResultResult)(nil)),
			"InputEvent":               reflect.ValueOf((*InputEvent)(nil)),
			"InputResult":              reflect.ValueOf((*InputResult)(nil)),
			"BeforeAgentStartEvent":    reflect.ValueOf((*BeforeAgentStartEvent)(nil)),
			"BeforeAgentStartResult":   reflect.ValueOf((*BeforeAgentStartResult)(nil)),
			"AgentStartEvent":          reflect.ValueOf((*AgentStartEvent)(nil)),
			"AgentEndEvent":            reflect.ValueOf((*AgentEndEvent)(nil)),
			"MessageStartEvent":        reflect.ValueOf((*MessageStartEvent)(nil)),
			"MessageUpdateEvent":       reflect.ValueOf((*MessageUpdateEvent)(nil)),
			"MessageEndEvent":          reflect.ValueOf((*MessageEndEvent)(nil)),
			"SessionStartEvent":        reflect.ValueOf((*SessionStartEvent)(nil)),
			"SessionShutdownEvent":     reflect.ValueOf((*SessionShutdownEvent)(nil)),
			"ModelChangeEvent":         reflect.ValueOf((*ModelChangeEvent)(nil)),
			"ThinkingLevelChangeEvent": reflect.ValueOf((*ThinkingLevelChangeEvent)(nil)),
			"TerminalResizeEvent":      reflect.ValueOf((*TerminalResizeEvent)(nil)),
			"TurnStateChangeEvent":     reflect.ValueOf((*TurnStateChangeEvent)(nil)),
			"MCPToolsReadyEvent":       reflect.ValueOf((*MCPToolsReadyEvent)(nil)),
			"MCPServerInfo":            reflect.ValueOf((*MCPServerInfo)(nil)),

			// Step lifecycle events
			"StepStartEvent":      reflect.ValueOf((*StepStartEvent)(nil)),
			"StepFinishEvent":     reflect.ValueOf((*StepFinishEvent)(nil)),
			"ReasoningStartEvent": reflect.ValueOf((*ReasoningStartEvent)(nil)),
			"WarningsEvent":       reflect.ValueOf((*WarningsEvent)(nil)),
			"SourceEvent":         reflect.ValueOf((*SourceEvent)(nil)),
			"ErrorEvent":          reflect.ValueOf((*ErrorEvent)(nil)),
			"RetryEvent":          reflect.ValueOf((*RetryEvent)(nil)),
			"StepResumeEvent":     reflect.ValueOf((*StepResumeEvent)(nil)),
			"PrepareStepEvent":    reflect.ValueOf((*PrepareStepEvent)(nil)),
			"PrepareStepResult":   reflect.ValueOf((*PrepareStepResult)(nil)),
			"LLMUsageEvent":       reflect.ValueOf((*LLMUsageEvent)(nil)),
		},
	}
}
