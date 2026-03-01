package extensions

import (
	"reflect"

	"github.com/traefik/yaegi/interp"
)

// Symbols returns the Yaegi export table that makes KIT's extension API
// available to interpreted Go code. Extensions import these types as:
//
//	import "kit/ext"
//
// IMPORTANT: Only concrete types (structs, constants) are exported. Interfaces
// (Event, Result) and the HandlerFunc type are NOT exported because Yaegi
// cannot generate interface wrappers for them. Instead, extensions use
// event-specific methods like api.OnToolCall() which accept concrete function
// signatures.
func Symbols() interp.Exports {
	return interp.Exports{
		"kit/ext/ext": map[string]reflect.Value{
			// Struct types (nil pointer trick for type registration)
			"API":            reflect.ValueOf((*API)(nil)),
			"Context":        reflect.ValueOf((*Context)(nil)),
			"ToolDef":        reflect.ValueOf((*ToolDef)(nil)),
			"CommandDef":     reflect.ValueOf((*CommandDef)(nil)),
			"PrintBlockOpts": reflect.ValueOf((*PrintBlockOpts)(nil)),

			// Session types
			"SessionMessage": reflect.ValueOf((*SessionMessage)(nil)),
			"ExtensionEntry": reflect.ValueOf((*ExtensionEntry)(nil)),

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

			// Editor interceptor types
			"EditorKeyActionType":  reflect.ValueOf((*EditorKeyActionType)(nil)),
			"EditorKeyPassthrough": reflect.ValueOf(EditorKeyPassthrough),
			"EditorKeyConsumed":    reflect.ValueOf(EditorKeyConsumed),
			"EditorKeyRemap":       reflect.ValueOf(EditorKeyRemap),
			"EditorKeySubmit":      reflect.ValueOf(EditorKeySubmit),
			"EditorKeyAction":      reflect.ValueOf((*EditorKeyAction)(nil)),
			"EditorConfig":         reflect.ValueOf((*EditorConfig)(nil)),

			// Prompt types
			"PromptSelectConfig":  reflect.ValueOf((*PromptSelectConfig)(nil)),
			"PromptSelectResult":  reflect.ValueOf((*PromptSelectResult)(nil)),
			"PromptConfirmConfig": reflect.ValueOf((*PromptConfirmConfig)(nil)),
			"PromptConfirmResult": reflect.ValueOf((*PromptConfirmResult)(nil)),
			"PromptInputConfig":   reflect.ValueOf((*PromptInputConfig)(nil)),
			"PromptInputResult":   reflect.ValueOf((*PromptInputResult)(nil)),

			// Event structs
			"ToolCallEvent":           reflect.ValueOf((*ToolCallEvent)(nil)),
			"ToolCallResult":          reflect.ValueOf((*ToolCallResult)(nil)),
			"ToolExecutionStartEvent": reflect.ValueOf((*ToolExecutionStartEvent)(nil)),
			"ToolExecutionEndEvent":   reflect.ValueOf((*ToolExecutionEndEvent)(nil)),
			"ToolResultEvent":         reflect.ValueOf((*ToolResultEvent)(nil)),
			"ToolResultResult":        reflect.ValueOf((*ToolResultResult)(nil)),
			"InputEvent":              reflect.ValueOf((*InputEvent)(nil)),
			"InputResult":             reflect.ValueOf((*InputResult)(nil)),
			"BeforeAgentStartEvent":   reflect.ValueOf((*BeforeAgentStartEvent)(nil)),
			"BeforeAgentStartResult":  reflect.ValueOf((*BeforeAgentStartResult)(nil)),
			"AgentStartEvent":         reflect.ValueOf((*AgentStartEvent)(nil)),
			"AgentEndEvent":           reflect.ValueOf((*AgentEndEvent)(nil)),
			"MessageStartEvent":       reflect.ValueOf((*MessageStartEvent)(nil)),
			"MessageUpdateEvent":      reflect.ValueOf((*MessageUpdateEvent)(nil)),
			"MessageEndEvent":         reflect.ValueOf((*MessageEndEvent)(nil)),
			"SessionStartEvent":       reflect.ValueOf((*SessionStartEvent)(nil)),
			"SessionShutdownEvent":    reflect.ValueOf((*SessionShutdownEvent)(nil)),
		},
	}
}
