package extensions

// ---------------------------------------------------------------------------
// Internal types (used by runner, NOT exposed to Yaegi)
// ---------------------------------------------------------------------------

// Event is the interface satisfied by all event types internally.
type Event interface {
	Type() EventType
}

// Result is the interface satisfied by all result types internally.
type Result interface {
	isResult()
}

// HandlerFunc is the internal handler signature used by the runner.
type HandlerFunc func(event Event, ctx Context) Result

// ---------------------------------------------------------------------------
// Context (exposed to Yaegi — concrete struct, no interfaces)
// ---------------------------------------------------------------------------

// Context provides runtime information to handlers about the current session.
type Context struct {
	SessionID   string
	CWD         string
	Model       string
	Interactive bool

	// Print outputs plain text to the user. In interactive mode this
	// routes through BubbleTea's scrollback (tea.Println); in
	// non-interactive mode it writes to stdout. Extensions must use
	// this instead of fmt.Println, which is swallowed by BubbleTea.
	Print func(string)

	// PrintInfo outputs text as a styled system message block (bordered,
	// themed). Use this for informational notices the user should see.
	PrintInfo func(string)

	// PrintError outputs text as a styled error block (red border, bold).
	// Use this for error messages or warnings.
	PrintError func(string)

	// PrintBlock outputs text as a custom styled block with caller-chosen
	// border color and optional subtitle. Example:
	//
	//   ctx.PrintBlock(ext.PrintBlockOpts{
	//       Text:        "Deployment complete!",
	//       BorderColor: "#a6e3a1",
	//       Subtitle:    "my-extension",
	//   })
	PrintBlock func(PrintBlockOpts)

	// SendMessage injects a message into the conversation and triggers a
	// new agent turn. If the agent is currently busy the message is queued
	// and processed after the current turn completes.
	//
	// This is safe to call from goroutines. Common pattern:
	//
	//   go func() {
	//       out, _ := exec.Command("kit", "-p", task).Output()
	//       ctx.SendMessage("Subagent result:\n" + string(out))
	//   }()
	SendMessage func(string)
}

// PrintBlockOpts configures a custom styled block for PrintBlock.
type PrintBlockOpts struct {
	// Text is the main content to display.
	Text string
	// BorderColor is a hex color string (e.g. "#a6e3a1") for the left border.
	// Defaults to the theme's system color if empty.
	BorderColor string
	// Subtitle is optional text shown below the content in muted style
	// (e.g. extension name, timestamp). Empty means no subtitle line.
	Subtitle string
}

// ---------------------------------------------------------------------------
// API — the object passed to each extension's Init function.
//
// Instead of a generic On(EventType, HandlerFunc) that uses interfaces,
// we expose event-specific methods with concrete function signatures.
// This avoids Yaegi's genInterfaceWrapper crash entirely — no interfaces
// cross the Yaegi boundary.
// ---------------------------------------------------------------------------

// API is passed to each extension's Init function. Extensions use it to
// register typed event handlers, custom tools, and slash commands.
type API struct {
	// Event-specific registration functions (wired by the loader).
	onToolCall         func(func(ToolCallEvent, Context) *ToolCallResult)
	onToolExecStart    func(func(ToolExecutionStartEvent, Context))
	onToolExecEnd      func(func(ToolExecutionEndEvent, Context))
	onToolResult       func(func(ToolResultEvent, Context) *ToolResultResult)
	onInput            func(func(InputEvent, Context) *InputResult)
	onBeforeAgentStart func(func(BeforeAgentStartEvent, Context) *BeforeAgentStartResult)
	onAgentStart       func(func(AgentStartEvent, Context))
	onAgentEnd         func(func(AgentEndEvent, Context))
	onMessageStart     func(func(MessageStartEvent, Context))
	onMessageUpdate    func(func(MessageUpdateEvent, Context))
	onMessageEnd       func(func(MessageEndEvent, Context))
	onSessionStart     func(func(SessionStartEvent, Context))
	onSessionShutdown  func(func(SessionShutdownEvent, Context))
	registerToolFn     func(ToolDef)
	registerCmdFn      func(CommandDef)
}

// OnToolCall registers a handler that fires before a tool executes.
// Return a non-nil ToolCallResult with Block=true to prevent execution.
func (a *API) OnToolCall(handler func(ToolCallEvent, Context) *ToolCallResult) {
	a.onToolCall(handler)
}

// OnToolExecutionStart registers a handler for tool execution start.
func (a *API) OnToolExecutionStart(handler func(ToolExecutionStartEvent, Context)) {
	a.onToolExecStart(handler)
}

// OnToolExecutionEnd registers a handler for tool execution end.
func (a *API) OnToolExecutionEnd(handler func(ToolExecutionEndEvent, Context)) {
	a.onToolExecEnd(handler)
}

// OnToolResult registers a handler that fires after tool execution.
// Return a non-nil ToolResultResult to modify the output.
func (a *API) OnToolResult(handler func(ToolResultEvent, Context) *ToolResultResult) {
	a.onToolResult(handler)
}

// OnInput registers a handler that fires when user input is received.
// Return a non-nil InputResult to transform or handle the input.
func (a *API) OnInput(handler func(InputEvent, Context) *InputResult) {
	a.onInput(handler)
}

// OnBeforeAgentStart registers a handler that fires before the agent loop.
func (a *API) OnBeforeAgentStart(handler func(BeforeAgentStartEvent, Context) *BeforeAgentStartResult) {
	a.onBeforeAgentStart(handler)
}

// OnAgentStart registers a handler for when the agent loop begins.
func (a *API) OnAgentStart(handler func(AgentStartEvent, Context)) {
	a.onAgentStart(handler)
}

// OnAgentEnd registers a handler for when the agent finishes responding.
func (a *API) OnAgentEnd(handler func(AgentEndEvent, Context)) {
	a.onAgentEnd(handler)
}

// OnMessageStart registers a handler for when an assistant message begins.
func (a *API) OnMessageStart(handler func(MessageStartEvent, Context)) {
	a.onMessageStart(handler)
}

// OnMessageUpdate registers a handler for streaming text chunks.
func (a *API) OnMessageUpdate(handler func(MessageUpdateEvent, Context)) {
	a.onMessageUpdate(handler)
}

// OnMessageEnd registers a handler for when the assistant message is complete.
func (a *API) OnMessageEnd(handler func(MessageEndEvent, Context)) {
	a.onMessageEnd(handler)
}

// OnSessionStart registers a handler for when a session is loaded or created.
func (a *API) OnSessionStart(handler func(SessionStartEvent, Context)) {
	a.onSessionStart(handler)
}

// OnSessionShutdown registers a handler for when the application is closing.
func (a *API) OnSessionShutdown(handler func(SessionShutdownEvent, Context)) {
	a.onSessionShutdown(handler)
}

// RegisterTool adds a custom tool that the LLM can invoke.
func (a *API) RegisterTool(tool ToolDef) {
	a.registerToolFn(tool)
}

// RegisterCommand adds a slash command available in interactive mode.
func (a *API) RegisterCommand(cmd CommandDef) {
	a.registerCmdFn(cmd)
}

// ---------------------------------------------------------------------------
// ToolDef / CommandDef
// ---------------------------------------------------------------------------

// ToolDef describes a custom tool registered by an extension.
type ToolDef struct {
	Name        string
	Description string
	Parameters  string // JSON Schema string
	Execute     func(input string) (string, error)
}

// CommandDef describes a slash command registered by an extension.
type CommandDef struct {
	Name        string
	Description string
	Execute     func(args string, ctx Context) (string, error)
}

// ---------------------------------------------------------------------------
// Typed events (all concrete structs — safe for Yaegi)
// ---------------------------------------------------------------------------

// ToolCallEvent fires before a tool executes.
type ToolCallEvent struct {
	ToolName   string
	ToolCallID string
	Input      string // JSON-encoded tool parameters
}

func (e ToolCallEvent) Type() EventType { return ToolCall }

// ToolCallResult controls whether the tool call proceeds.
type ToolCallResult struct {
	Block  bool
	Reason string
}

func (ToolCallResult) isResult() {}

// ToolExecutionStartEvent fires when a tool begins executing.
type ToolExecutionStartEvent struct {
	ToolName string
}

func (e ToolExecutionStartEvent) Type() EventType { return ToolExecutionStart }

// ToolExecutionEndEvent fires when a tool finishes executing.
type ToolExecutionEndEvent struct {
	ToolName string
}

func (e ToolExecutionEndEvent) Type() EventType { return ToolExecutionEnd }

// ToolResultEvent fires after tool execution with the output.
type ToolResultEvent struct {
	ToolName string
	Input    string
	Content  string
	IsError  bool
}

func (e ToolResultEvent) Type() EventType { return ToolResult }

// ToolResultResult can modify the tool's output before it reaches the LLM.
type ToolResultResult struct {
	Content *string // nil = unchanged
	IsError *bool   // nil = unchanged
}

func (ToolResultResult) isResult() {}

// InputEvent fires when user input is received.
type InputEvent struct {
	Text   string
	Source string // "interactive", "cli", "script", "queue"
}

func (e InputEvent) Type() EventType { return Input }

// InputResult controls what happens with user input.
//
//	Action: "continue" (default), "transform", "handled"
type InputResult struct {
	Action string
	Text   string // replacement text when Action="transform"
}

func (InputResult) isResult() {}

// BeforeAgentStartEvent fires before the agent loop begins.
type BeforeAgentStartEvent struct {
	Prompt string
}

func (e BeforeAgentStartEvent) Type() EventType { return BeforeAgentStart }

// BeforeAgentStartResult can inject context before the agent runs.
type BeforeAgentStartResult struct {
	InjectText   *string
	SystemPrompt *string
}

func (BeforeAgentStartResult) isResult() {}

// AgentStartEvent fires when the agent loop begins.
type AgentStartEvent struct {
	Prompt string
}

func (e AgentStartEvent) Type() EventType { return AgentStart }

// AgentEndEvent fires when the agent finishes responding.
type AgentEndEvent struct {
	Response   string
	StopReason string // "completed", "cancelled", "error"
}

func (e AgentEndEvent) Type() EventType { return AgentEnd }

// MessageStartEvent fires when a new assistant message begins.
type MessageStartEvent struct{}

func (e MessageStartEvent) Type() EventType { return MessageStart }

// MessageUpdateEvent fires for each streaming text chunk.
type MessageUpdateEvent struct {
	Chunk string
}

func (e MessageUpdateEvent) Type() EventType { return MessageUpdate }

// MessageEndEvent fires when the assistant message is complete.
type MessageEndEvent struct {
	Content string
}

func (e MessageEndEvent) Type() EventType { return MessageEnd }

// SessionStartEvent fires when a session is loaded or created.
type SessionStartEvent struct {
	SessionID string
}

func (e SessionStartEvent) Type() EventType { return SessionStart }

// SessionShutdownEvent fires when the application is closing.
type SessionShutdownEvent struct{}

func (e SessionShutdownEvent) Type() EventType { return SessionShutdown }
