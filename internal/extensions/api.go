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

	// SetWidget places or updates a persistent widget in the TUI. Widgets
	// remain visible across agent turns until explicitly removed. The
	// widget is identified by WidgetConfig.ID; calling SetWidget with the
	// same ID replaces the previous content.
	//
	// Example:
	//
	//   ctx.SetWidget(ext.WidgetConfig{
	//       ID:        "my-status",
	//       Placement: ext.WidgetAbove,
	//       Content:   ext.WidgetContent{Text: "Build: passing"},
	//       Style:     ext.WidgetStyle{BorderColor: "#a6e3a1"},
	//   })
	SetWidget func(WidgetConfig)

	// RemoveWidget removes a previously placed widget by its ID. No-op if
	// the ID does not exist.
	RemoveWidget func(id string)

	// SetHeader places a custom header at the top of the TUI view, above
	// the stream region. Only one header can be active at a time; calling
	// SetHeader replaces any previous header. The header persists across
	// agent turns until explicitly removed.
	//
	// Example:
	//
	//   ctx.SetHeader(ext.HeaderFooterConfig{
	//       Content: ext.WidgetContent{Text: "Project: my-app | Branch: main"},
	//       Style:   ext.WidgetStyle{BorderColor: "#89b4fa"},
	//   })
	SetHeader func(HeaderFooterConfig)

	// RemoveHeader removes the custom header. No-op if no header is set.
	RemoveHeader func()

	// SetFooter places a custom footer at the bottom of the TUI view,
	// below the status bar. Only one footer can be active at a time;
	// calling SetFooter replaces any previous footer. The footer persists
	// across agent turns until explicitly removed.
	//
	// Example:
	//
	//   ctx.SetFooter(ext.HeaderFooterConfig{
	//       Content: ext.WidgetContent{Text: "Ready | 3 tasks remaining"},
	//       Style:   ext.WidgetStyle{BorderColor: "#a6e3a1"},
	//   })
	SetFooter func(HeaderFooterConfig)

	// RemoveFooter removes the custom footer. No-op if no footer is set.
	RemoveFooter func()

	// PromptSelect shows a selection list to the user and blocks until
	// they pick an option or cancel (ESC). Returns a cancelled result in
	// non-interactive mode. Safe to call from event handlers and slash
	// command handlers.
	//
	// Example:
	//
	//   result := ctx.PromptSelect(ext.PromptSelectConfig{
	//       Message: "Choose a deployment target:",
	//       Options: []string{"staging", "production", "local"},
	//   })
	//   if !result.Cancelled {
	//       fmt.Println("Selected:", result.Value)
	//   }
	PromptSelect func(PromptSelectConfig) PromptSelectResult

	// PromptConfirm shows a yes/no confirmation to the user and blocks
	// until they respond or cancel. Returns a cancelled result in
	// non-interactive mode.
	//
	// Example:
	//
	//   result := ctx.PromptConfirm(ext.PromptConfirmConfig{
	//       Message:      "Deploy to production?",
	//       DefaultValue: false,
	//   })
	//   if !result.Cancelled && result.Value {
	//       // proceed with deployment
	//   }
	PromptConfirm func(PromptConfirmConfig) PromptConfirmResult

	// PromptInput shows a text input field to the user and blocks until
	// they submit text or cancel. Returns a cancelled result in
	// non-interactive mode.
	//
	// Example:
	//
	//   result := ctx.PromptInput(ext.PromptInputConfig{
	//       Message:     "Enter the release tag:",
	//       Placeholder: "v1.0.0",
	//   })
	//   if !result.Cancelled {
	//       fmt.Println("Tag:", result.Value)
	//   }
	PromptInput func(PromptInputConfig) PromptInputResult

	// ShowOverlay displays a modal overlay dialog that blocks until the
	// user dismisses it or selects an action. The overlay renders as a
	// centered (or anchored) bordered box over the TUI. Returns a
	// cancelled result in non-interactive mode.
	//
	// Example:
	//
	//   result := ctx.ShowOverlay(ext.OverlayConfig{
	//       Title:   "Deployment Summary",
	//       Content: ext.WidgetContent{Text: "All 3 services deployed."},
	//       Style:   ext.OverlayStyle{BorderColor: "#a6e3a1"},
	//       Actions: []string{"Continue", "Rollback", "Details"},
	//   })
	//   if !result.Cancelled {
	//       fmt.Println("Selected:", result.Action)
	//   }
	ShowOverlay func(OverlayConfig) OverlayResult

	// SetEditor installs an editor interceptor that wraps the built-in
	// input editor. The interceptor can intercept keys (remap, consume,
	// submit) and modify the rendered output. Only one interceptor is
	// active at a time; calling SetEditor replaces any previous interceptor.
	//
	// Example — vim-like normal mode:
	//
	//   ctx.SetEditor(ext.EditorConfig{
	//       HandleKey: func(key, text string) ext.EditorKeyAction {
	//           switch key {
	//           case "h":
	//               return ext.EditorKeyAction{Type: ext.EditorKeyRemap, RemappedKey: "left"}
	//           case "i":
	//               ctx.ResetEditor()
	//               return ext.EditorKeyAction{Type: ext.EditorKeyConsumed}
	//           }
	//           return ext.EditorKeyAction{Type: ext.EditorKeyPassthrough}
	//       },
	//       Render: func(width int, content string) string {
	//           return "[NORMAL]\n" + content
	//       },
	//   })
	SetEditor func(EditorConfig)

	// ResetEditor removes the active editor interceptor and restores the
	// default built-in editor behavior. No-op if no interceptor is set.
	ResetEditor func()
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
	onToolCall             func(func(ToolCallEvent, Context) *ToolCallResult)
	onToolExecStart        func(func(ToolExecutionStartEvent, Context))
	onToolExecEnd          func(func(ToolExecutionEndEvent, Context))
	onToolResult           func(func(ToolResultEvent, Context) *ToolResultResult)
	onInput                func(func(InputEvent, Context) *InputResult)
	onBeforeAgentStart     func(func(BeforeAgentStartEvent, Context) *BeforeAgentStartResult)
	onAgentStart           func(func(AgentStartEvent, Context))
	onAgentEnd             func(func(AgentEndEvent, Context))
	onMessageStart         func(func(MessageStartEvent, Context))
	onMessageUpdate        func(func(MessageUpdateEvent, Context))
	onMessageEnd           func(func(MessageEndEvent, Context))
	onSessionStart         func(func(SessionStartEvent, Context))
	onSessionShutdown      func(func(SessionShutdownEvent, Context))
	registerToolFn         func(ToolDef)
	registerCmdFn          func(CommandDef)
	registerToolRendererFn func(ToolRenderConfig)
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

// RegisterToolRenderer registers a custom renderer for a specific tool's
// display in the TUI. The renderer controls the header (parameter summary)
// and/or body (result display) of the tool's output block. If multiple
// extensions register renderers for the same tool name, the last one wins.
func (a *API) RegisterToolRenderer(config ToolRenderConfig) {
	a.registerToolRendererFn(config)
}

// ---------------------------------------------------------------------------
// Widget types (exposed to Yaegi — concrete structs, no interfaces)
// ---------------------------------------------------------------------------

// WidgetPlacement determines where a widget appears in the TUI layout
// relative to the input area.
type WidgetPlacement string

const (
	// WidgetAbove places the widget above the input area, between the
	// separator and queued messages.
	WidgetAbove WidgetPlacement = "above"

	// WidgetBelow places the widget below the input area, between the
	// input and the status bar.
	WidgetBelow WidgetPlacement = "below"
)

// WidgetContent describes what to render in a widget slot.
type WidgetContent struct {
	// Text is the content to display.
	Text string

	// Markdown, when true, renders Text as styled markdown instead of
	// plain text.
	Markdown bool
}

// WidgetStyle configures the visual appearance of a widget.
type WidgetStyle struct {
	// BorderColor is a hex color (e.g. "#a6e3a1") for the left border.
	// Empty uses the theme's default accent color.
	BorderColor string

	// NoBorder disables the left border entirely.
	NoBorder bool
}

// WidgetConfig fully describes a widget for placement in the TUI.
// Extensions identify widgets by ID; calling SetWidget with the same ID
// replaces the previous widget. IDs should be descriptive to avoid
// collisions across extensions (e.g. "myext:token-counter").
type WidgetConfig struct {
	// ID uniquely identifies this widget. Must be non-empty.
	ID string

	// Placement determines where the widget appears (above or below input).
	Placement WidgetPlacement

	// Content describes what to render.
	Content WidgetContent

	// Style configures the appearance.
	Style WidgetStyle

	// Priority controls ordering within a placement slot. Lower values
	// render first. Widgets with equal priority are ordered by insertion
	// time.
	Priority int
}

// ---------------------------------------------------------------------------
// Interactive prompt types (exposed to Yaegi — concrete structs)
// ---------------------------------------------------------------------------

// PromptSelectConfig configures a selection prompt that presents the user
// with a list of options to choose from.
type PromptSelectConfig struct {
	// Message is the question or instruction displayed to the user.
	Message string
	// Options is the list of choices the user can select from.
	Options []string
}

// PromptSelectResult is the response from a selection prompt.
type PromptSelectResult struct {
	// Value is the text of the selected option.
	Value string
	// Index is the zero-based index of the selected option.
	Index int
	// Cancelled is true if the user dismissed the prompt (ESC) or
	// the prompt was unavailable (non-interactive mode).
	Cancelled bool
}

// PromptConfirmConfig configures a yes/no confirmation prompt.
type PromptConfirmConfig struct {
	// Message is the question displayed to the user.
	Message string
	// DefaultValue is the pre-selected answer (true = Yes).
	DefaultValue bool
}

// PromptConfirmResult is the response from a confirmation prompt.
type PromptConfirmResult struct {
	// Value is true for "Yes", false for "No".
	Value bool
	// Cancelled is true if the user dismissed the prompt.
	Cancelled bool
}

// PromptInputConfig configures a free-form text input prompt.
type PromptInputConfig struct {
	// Message is the question displayed to the user.
	Message string
	// Placeholder is ghost text shown when the input is empty.
	Placeholder string
	// Default is the pre-filled value in the input field.
	Default string
}

// PromptInputResult is the response from a text input prompt.
type PromptInputResult struct {
	// Value is the text the user entered.
	Value string
	// Cancelled is true if the user dismissed the prompt.
	Cancelled bool
}

// ---------------------------------------------------------------------------
// Header/Footer types (exposed to Yaegi — concrete structs)
// ---------------------------------------------------------------------------

// HeaderFooterConfig describes a custom header or footer region that replaces
// or augments the default TUI chrome. Extensions use ctx.SetHeader/SetFooter
// to place one; only one header and one footer can be active at a time (the
// latest call wins). Reuses WidgetContent and WidgetStyle for consistency.
type HeaderFooterConfig struct {
	// Content describes what to render.
	Content WidgetContent

	// Style configures the appearance.
	Style WidgetStyle
}

// ---------------------------------------------------------------------------
// Overlay types (exposed to Yaegi — concrete structs)
// ---------------------------------------------------------------------------

// OverlayAnchor determines the vertical position of an overlay dialog
// within the TUI view.
type OverlayAnchor string

const (
	// OverlayCenter positions the dialog in the vertical center.
	OverlayCenter OverlayAnchor = "center"

	// OverlayTopCenter positions the dialog near the top of the view.
	OverlayTopCenter OverlayAnchor = "top-center"

	// OverlayBottomCenter positions the dialog near the bottom of the view.
	OverlayBottomCenter OverlayAnchor = "bottom-center"
)

// OverlayStyle configures the visual appearance of an overlay dialog.
type OverlayStyle struct {
	// BorderColor is a hex color (e.g. "#89b4fa") for the dialog border.
	// Empty uses a default blue accent.
	BorderColor string

	// Background is a hex color (e.g. "#1e1e2e") for the dialog background.
	// Empty means no explicit background (inherits terminal default).
	Background string
}

// OverlayConfig fully describes a modal overlay dialog. Extensions call
// ctx.ShowOverlay(config) to display the dialog and block until the user
// dismisses it or selects an action. The dialog renders as a bordered box
// positioned within the TUI, with optional scrollable content and action
// buttons.
//
// Example:
//
//	result := ctx.ShowOverlay(ext.OverlayConfig{
//	    Title:   "Build Results",
//	    Content: ext.WidgetContent{Text: "All 42 tests passed."},
//	    Style:   ext.OverlayStyle{BorderColor: "#a6e3a1"},
//	    Width:   60,
//	    Actions: []string{"Continue", "Show Details"},
//	})
type OverlayConfig struct {
	// Title is displayed at the top of the dialog. Empty means no title.
	Title string

	// Content describes what to render inside the dialog body. The Text
	// field is required; set Markdown=true to render as styled markdown.
	Content WidgetContent

	// Style configures the appearance.
	Style OverlayStyle

	// Width is the dialog width in columns. 0 = 60% of terminal width.
	// Clamped to [30, termWidth-4].
	Width int

	// MaxHeight limits the dialog height in lines. 0 = 80% of terminal
	// height. Content exceeding this height becomes scrollable.
	MaxHeight int

	// Anchor determines vertical positioning. Default is "center".
	Anchor OverlayAnchor

	// Actions, if non-empty, shows selectable action buttons at the
	// bottom of the dialog. The user navigates with left/right arrows
	// and selects with Enter. The selected action's text and index are
	// returned in OverlayResult.
	//
	// If empty, the dialog is a simple info panel dismissed with ESC
	// or Enter (result.Cancelled=false, result.Action="", result.Index=-1).
	Actions []string
}

// OverlayResult is the response from a ShowOverlay call.
type OverlayResult struct {
	// Action is the text of the selected action, or "" if no actions
	// were configured or the dialog was dismissed without selection.
	Action string

	// Index is the zero-based index of the selected action, or -1 if
	// no action was selected.
	Index int

	// Cancelled is true if the user dismissed the dialog with ESC.
	Cancelled bool
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
// Custom tool rendering (exposed to Yaegi — concrete structs)
// ---------------------------------------------------------------------------

// ToolRenderConfig provides custom rendering functions for a tool's display
// in the TUI. Extensions register tool renderers via API.RegisterToolRenderer()
// during Init. Both render functions are optional — if nil or if they return
// an empty string, the builtin renderer (or default) is used as a fallback.
//
// Example:
//
//	api.RegisterToolRenderer(ext.ToolRenderConfig{
//	    ToolName: "my-tool",
//	    RenderHeader: func(toolArgs string, width int) string {
//	        // Parse args and return a compact summary for the header
//	        return "my-tool: doing something"
//	    },
//	    RenderBody: func(toolResult string, isError bool, width int) string {
//	        // Return custom formatted result body
//	        if isError {
//	            return "ERROR: " + toolResult
//	        }
//	        return "Result: " + toolResult
//	    },
//	})
type ToolRenderConfig struct {
	// ToolName is the name of the tool this renderer applies to. Must match
	// the tool's registered name exactly (e.g. "bash", "read", "my-tool").
	ToolName string

	// DisplayName, if non-empty, replaces the auto-capitalized tool name
	// shown in the header line (e.g. "Shell" instead of "Bash").
	DisplayName string

	// BorderColor, if non-empty, overrides the default border color for
	// the tool result block. Accepts a hex color string (e.g. "#89b4fa").
	// By default, the border is green for success and red for error.
	BorderColor string

	// Background, if non-empty, sets a background color for the entire
	// tool result block. Accepts a hex color string (e.g. "#1e1e2e").
	// By default, no background is applied.
	Background string

	// BodyMarkdown, when true, passes the RenderBody output through the
	// glamour markdown renderer before display. This lets extensions return
	// markdown-formatted text without needing access to Kit's internal
	// rendering functions. Ignored when RenderBody is nil or returns empty.
	BodyMarkdown bool

	// RenderHeader, if non-nil, replaces the default parameter formatting
	// in the tool header line. Receives the JSON-encoded arguments and the
	// maximum width in columns. Return a short summary string for display
	// after the tool name, or empty string to fall back to default formatting.
	RenderHeader func(toolArgs string, width int) string

	// RenderBody, if non-nil, replaces the default tool result body rendering.
	// Receives the result text, error flag, and available width in columns.
	// Return the full styled body content, or empty string to fall back to
	// the builtin renderer (or default).
	RenderBody func(toolResult string, isError bool, width int) string
}

// ---------------------------------------------------------------------------
// Editor interceptor types (exposed to Yaegi — concrete structs)
// ---------------------------------------------------------------------------

// EditorKeyActionType defines the outcome of an editor key interception.
type EditorKeyActionType string

const (
	// EditorKeyPassthrough lets the built-in editor handle the key normally.
	EditorKeyPassthrough EditorKeyActionType = "passthrough"

	// EditorKeyConsumed means the extension handled the key. The editor
	// should re-render but not process the key further.
	EditorKeyConsumed EditorKeyActionType = "consumed"

	// EditorKeyRemap transforms the key into a different key before passing
	// it to the built-in editor. Use RemappedKey to specify the target
	// (e.g., "left", "right", "up", "down", "backspace", "delete", "enter",
	// "tab", "home", "end", or a single character like "a").
	EditorKeyRemap EditorKeyActionType = "remap"

	// EditorKeySubmit forces immediate text submission. The SubmitText field
	// specifies the text to submit (empty = use editor's current text).
	EditorKeySubmit EditorKeyActionType = "submit"
)

// EditorKeyAction is returned by an editor interceptor's HandleKey function
// to indicate how a key press should be handled.
type EditorKeyAction struct {
	// Type determines the action taken.
	Type EditorKeyActionType

	// RemappedKey is the target key name for EditorKeyRemap. Must be a
	// recognized key name (e.g., "left", "right", "up", "down", "backspace",
	// "delete", "enter", "tab", "home", "end", "esc", "space", or a single
	// printable character).
	RemappedKey string

	// SubmitText is the text to submit for EditorKeySubmit. If empty, the
	// editor's current content is submitted instead.
	SubmitText string
}

// EditorConfig defines an editor interceptor/decorator that wraps the built-in
// input editor. Extensions can intercept key events (remap, consume, or force
// submit) and/or modify the rendered output (add mode indicators, apply visual
// effects).
//
// This follows Pi's extension editor pattern (modal editor, rainbow editor)
// but uses concrete function fields instead of interfaces for Yaegi safety.
//
// IMPORTANT (Yaegi limitation): Function fields MUST be set using anonymous
// function literals (closures), NOT bare function references. Yaegi does not
// correctly propagate return values from named function references assigned to
// struct fields. Wrap any named function in a closure:
//
//	// WRONG — Yaegi returns zero values:
//	ctx.SetEditor(ext.EditorConfig{HandleKey: myHandler, Render: myRender})
//
//	// CORRECT — closure wrapper works:
//	ctx.SetEditor(ext.EditorConfig{
//	    HandleKey: func(k string, t string) ext.EditorKeyAction { return myHandler(k, t) },
//	    Render:    func(w int, c string) string { return myRender(w, c) },
//	})
type EditorConfig struct {
	// HandleKey intercepts key presses before they reach the built-in editor.
	// It receives the key name (e.g., "a", "enter", "ctrl+c", "backspace")
	// and the editor's current text content. Return an EditorKeyAction to
	// control how the key is handled.
	//
	// If nil, all keys pass through to the built-in editor unchanged.
	HandleKey func(key string, currentText string) EditorKeyAction

	// Render wraps the built-in editor's rendered output. It receives the
	// available width and the default-rendered content (including title,
	// textarea, popup, and help text). Return the modified content to display.
	//
	// If nil, the default rendering is used unchanged.
	Render func(width int, defaultContent string) string
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
