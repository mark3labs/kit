package extensions

// NewTestAPI creates an API object wired for testing.
// This is used by the test harness to load extensions and verify behavior.
// The registration functions wire handlers directly to the provided extension.
func NewTestAPI(ext *LoadedExtension) API {
	reg := func(event EventType, fn HandlerFunc) {
		ext.Handlers[event] = append(ext.Handlers[event], fn)
	}

	return API{
		onToolCall: func(h func(ToolCallEvent, Context) *ToolCallResult) {
			reg(ToolCall, func(e Event, c Context) Result {
				r := h(e.(ToolCallEvent), c)
				if r == nil {
					return nil
				}
				return *r
			})
		},
		onToolExecStart: func(h func(ToolExecutionStartEvent, Context)) {
			reg(ToolExecutionStart, func(e Event, c Context) Result {
				h(e.(ToolExecutionStartEvent), c)
				return nil
			})
		},
		onToolExecEnd: func(h func(ToolExecutionEndEvent, Context)) {
			reg(ToolExecutionEnd, func(e Event, c Context) Result {
				h(e.(ToolExecutionEndEvent), c)
				return nil
			})
		},
		onToolOutput: func(h func(ToolOutputEvent, Context)) {
			reg(ToolOutput, func(e Event, c Context) Result {
				h(e.(ToolOutputEvent), c)
				return nil
			})
		},
		onToolResult: func(h func(ToolResultEvent, Context) *ToolResultResult) {
			reg(ToolResult, func(e Event, c Context) Result {
				r := h(e.(ToolResultEvent), c)
				if r == nil {
					return nil
				}
				return *r
			})
		},
		onInput: func(h func(InputEvent, Context) *InputResult) {
			reg(Input, func(e Event, c Context) Result {
				r := h(e.(InputEvent), c)
				if r == nil {
					return nil
				}
				return *r
			})
		},
		onBeforeAgentStart: func(h func(BeforeAgentStartEvent, Context) *BeforeAgentStartResult) {
			reg(BeforeAgentStart, func(e Event, c Context) Result {
				r := h(e.(BeforeAgentStartEvent), c)
				if r == nil {
					return nil
				}
				return *r
			})
		},
		onAgentStart: func(h func(AgentStartEvent, Context)) {
			reg(AgentStart, func(e Event, c Context) Result {
				h(e.(AgentStartEvent), c)
				return nil
			})
		},
		onAgentEnd: func(h func(AgentEndEvent, Context)) {
			reg(AgentEnd, func(e Event, c Context) Result {
				h(e.(AgentEndEvent), c)
				return nil
			})
		},
		onMessageStart: func(h func(MessageStartEvent, Context)) {
			reg(MessageStart, func(e Event, c Context) Result {
				h(e.(MessageStartEvent), c)
				return nil
			})
		},
		onMessageUpdate: func(h func(MessageUpdateEvent, Context)) {
			reg(MessageUpdate, func(e Event, c Context) Result {
				h(e.(MessageUpdateEvent), c)
				return nil
			})
		},
		onMessageEnd: func(h func(MessageEndEvent, Context)) {
			reg(MessageEnd, func(e Event, c Context) Result {
				h(e.(MessageEndEvent), c)
				return nil
			})
		},
		onSessionStart: func(h func(SessionStartEvent, Context)) {
			reg(SessionStart, func(e Event, c Context) Result {
				h(e.(SessionStartEvent), c)
				return nil
			})
		},
		onSessionShutdown: func(h func(SessionShutdownEvent, Context)) {
			reg(SessionShutdown, func(e Event, c Context) Result {
				h(e.(SessionShutdownEvent), c)
				return nil
			})
		},
		onModelChange: func(h func(ModelChangeEvent, Context)) {
			reg(ModelChange, func(e Event, c Context) Result {
				h(e.(ModelChangeEvent), c)
				return nil
			})
		},
		onContextPrepare: func(h func(ContextPrepareEvent, Context) *ContextPrepareResult) {
			reg(ContextPrepare, func(e Event, c Context) Result {
				r := h(e.(ContextPrepareEvent), c)
				if r == nil {
					return nil
				}
				return *r
			})
		},
		onBeforeFork: func(h func(BeforeForkEvent, Context) *BeforeForkResult) {
			reg(BeforeFork, func(e Event, c Context) Result {
				r := h(e.(BeforeForkEvent), c)
				if r == nil {
					return nil
				}
				return *r
			})
		},
		onBeforeSessionSwitch: func(h func(BeforeSessionSwitchEvent, Context) *BeforeSessionSwitchResult) {
			reg(BeforeSessionSwitch, func(e Event, c Context) Result {
				r := h(e.(BeforeSessionSwitchEvent), c)
				if r == nil {
					return nil
				}
				return *r
			})
		},
		onBeforeCompact: func(h func(BeforeCompactEvent, Context) *BeforeCompactResult) {
			reg(BeforeCompact, func(e Event, c Context) Result {
				r := h(e.(BeforeCompactEvent), c)
				if r == nil {
					return nil
				}
				return *r
			})
		},
		registerToolFn: func(tool ToolDef) {
			ext.Tools = append(ext.Tools, tool)
		},
		registerCmdFn: func(cmd CommandDef) {
			ext.Commands = append(ext.Commands, cmd)
		},
		registerToolRendererFn: func(config ToolRenderConfig) {
			ext.ToolRenderers = append(ext.ToolRenderers, config)
		},
		onCustomEvent: func(name string, handler func(string)) {
			if ext.CustomEventHandlers == nil {
				ext.CustomEventHandlers = make(map[string][]func(string))
			}
			ext.CustomEventHandlers[name] = append(ext.CustomEventHandlers[name], handler)
		},
		registerOption: func(opt OptionDef) {
			ext.Options = append(ext.Options, opt)
		},
		registerShortcutFn: func(def ShortcutDef, handler func(Context)) {
			ext.Shortcuts = append(ext.Shortcuts, ShortcutEntry{Def: def, Handler: handler})
		},
		registerMessageRendererFn: func(config MessageRendererConfig) {
			ext.MessageRenderers = append(ext.MessageRenderers, config)
		},
		onSubagentStart: func(h func(SubagentStartEvent, Context)) {
			reg(SubagentStart, func(e Event, c Context) Result {
				h(e.(SubagentStartEvent), c)
				return nil
			})
		},
		onSubagentChunk: func(h func(SubagentChunkEvent, Context)) {
			reg(SubagentChunk, func(e Event, c Context) Result {
				h(e.(SubagentChunkEvent), c)
				return nil
			})
		},
		onSubagentEnd: func(h func(SubagentEndEvent, Context)) {
			reg(SubagentEnd, func(e Event, c Context) Result {
				h(e.(SubagentEndEvent), c)
				return nil
			})
		},
	}
}
