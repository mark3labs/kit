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

	// CancelAndSend cancels the current agent turn (if running), clears
	// the message queue, and sends a new message that executes as soon as
	// cancellation completes. If the agent is idle, the message executes
	// immediately. This is the "steer" delivery mode.
	//
	// Use this for directive changes that should interrupt the current
	// operation, e.g. switching modes or redirecting the agent.
	//
	// Example:
	//
	//   ctx.CancelAndSend("Stop what you're doing and focus on the tests")
	CancelAndSend func(string)

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

	// PromptMultiSelect shows a multi-selection list to the user, allowing
	// them to toggle options with spacebar and confirm with enter. In
	// non-interactive mode, returns all options as selected.
	//
	// Example:
	//
	//   result := ctx.PromptMultiSelect(ext.PromptMultiSelectConfig{
	//       Message: "Select extensions to install:",
	//       Options: []string{"git", "todo", "weather"},
	//       DefaultSelected: []int{0, 1, 2},  // All selected by default
	//   })
	//   if !result.Cancelled {
	//       fmt.Println("Selected:", result.Values)
	//   }
	PromptMultiSelect func(PromptMultiSelectConfig) PromptMultiSelectResult

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

	// SetUIVisibility controls which built-in TUI chrome elements are
	// visible. By default all elements are shown (zero value = show all).
	// Call this during OnSessionStart to configure the initial layout.
	//
	// Example — minimal chrome:
	//
	//   ctx.SetUIVisibility(ext.UIVisibility{
	//       HideStartupMessage: true,
	//       HideStatusBar:      true,
	//       HideSeparator:      true,
	//       HideInputHint:      true,
	//   })
	SetUIVisibility func(UIVisibility)

	// GetContextStats returns current context-window usage information
	// (estimated tokens, context limit, usage percentage, message count).
	// Useful for building context meters, auto-compaction triggers, etc.
	//
	// Example:
	//
	//   stats := ctx.GetContextStats()
	//   pct := int(stats.UsagePercent * 100)
	//   fmt.Sprintf("[%s%s] %d%%", strings.Repeat("#", pct/10), strings.Repeat("-", 10-pct/10), pct)
	GetContextStats func() ContextStats

	// GetMessages returns the conversation messages on the current branch,
	// ordered from root to leaf. This is a read-only view; extensions
	// cannot modify messages directly.
	//
	// Example:
	//
	//   msgs := ctx.GetMessages()
	//   for _, m := range msgs {
	//       if m.Role == "assistant" {
	//           lastResponse = m.Content
	//       }
	//   }
	GetMessages func() []SessionMessage

	// GetSessionPath returns the file path of the current session's JSONL
	// file. Returns empty string for in-memory (ephemeral) sessions.
	GetSessionPath func() string

	// AppendEntry persists custom extension data in the session tree.
	// The data survives across session restarts and can be retrieved via
	// GetEntries. Use entryType to namespace your data (e.g. "myext:state").
	//
	// Example:
	//
	//   data, _ := json.Marshal(myState)
	//   ctx.AppendEntry("myext:state", string(data))
	AppendEntry func(entryType string, data string) (string, error)

	// GetEntries retrieves all persisted extension data entries matching
	// the given type on the current branch, ordered root to leaf. Pass
	// empty string to retrieve all extension data entries.
	//
	// Example — restore state on session resume:
	//
	//   entries := ctx.GetEntries("myext:state")
	//   if len(entries) > 0 {
	//       last := entries[len(entries)-1]
	//       json.Unmarshal([]byte(last.Data), &myState)
	//   }
	GetEntries func(entryType string) []ExtensionEntry

	// SetEditorText sets the text content of the input editor. This can
	// be used to pre-fill the editor with suggested text (e.g. extracted
	// questions, handoff prompts). The cursor is moved to the end.
	//
	// Example:
	//
	//   ctx.SetEditorText("Please review the changes in src/main.go")
	SetEditorText func(text string)

	// SetStatus places or updates a keyed entry in the TUI status bar.
	// Multiple entries from different extensions coexist; each is identified
	// by a unique key. Lower priority values render further left.
	//
	// Example:
	//
	//   ctx.SetStatus("myext:branch", "main", 50)
	SetStatus func(key string, text string, priority int)

	// RemoveStatus removes a keyed status bar entry. No-op if the key
	// does not exist.
	RemoveStatus func(key string)

	// GetOption returns the value of a named extension option. Options are
	// resolved in priority order:
	//   1. Runtime override (via SetOption)
	//   2. Environment variable: KIT_OPT_<NAME> (uppercase, dashes → underscores)
	//   3. Config file: options.<name> in .kit.yml
	//   4. Default value registered by the extension
	//
	// Returns empty string if the option was not registered.
	//
	// Example:
	//
	//   preset := ctx.GetOption("preset")
	//   if preset == "fast" {
	//       ctx.SetModel("anthropic/claude-haiku-3-5-20241022")
	//   }
	GetOption func(name string) string

	// SetOption sets a runtime override for a named extension option. This
	// takes highest priority over env vars, config, and defaults. Useful for
	// persisting user choices during a session.
	SetOption func(name string, value string)

	// SetModel changes the active LLM model at runtime. The model string
	// should be in "provider/model" format (e.g. "anthropic/claude-sonnet-4-5-20250929").
	// Existing tools, system prompt, and session are preserved. Returns an
	// error if the model string is invalid or the provider cannot be created.
	//
	// Example:
	//
	//   err := ctx.SetModel("openai/gpt-4o")
	//   if err != nil {
	//       ctx.PrintError("Failed to switch model: " + err.Error())
	//   }
	SetModel func(modelString string) error

	// GetAvailableModels returns a list of known models from the registry.
	// This is an advisory list — models not in the registry can still be
	// used by specifying their provider/model string directly.
	//
	// Example:
	//
	//   models := ctx.GetAvailableModels()
	//   for _, m := range models {
	//       fmt.Printf("%s/%s (ctx: %dk)\n", m.Provider, m.ModelID, m.ContextLimit/1000)
	//   }
	GetAvailableModels func() []ModelInfoEntry

	// EmitCustomEvent publishes a named event that other extensions can
	// subscribe to via api.OnCustomEvent(). Data is an arbitrary string
	// (JSON-encode complex payloads). Handlers run synchronously in
	// registration order.
	//
	// Example:
	//
	//   ctx.EmitCustomEvent("plan-mode:toggled", `{"active":true}`)
	EmitCustomEvent func(name string, data string)

	// GetAllTools returns information about all tools available to the agent,
	// including core tools (bash, read, write, etc.), MCP server tools, and
	// extension-registered tools. Each entry includes the tool's enabled status.
	//
	// Example — list read-only tools:
	//
	//   for _, t := range ctx.GetAllTools() {
	//       if t.Source == "core" && t.Enabled {
	//           fmt.Println(t.Name, "-", t.Description)
	//       }
	//   }
	GetAllTools func() []ToolInfo

	// SetActiveTools restricts the agent to only the named tools. Tools not
	// in the list are blocked from execution (the LLM receives an error if
	// it tries to call them). Pass nil or an empty slice to re-enable all
	// tools. Tool names are case-sensitive.
	//
	// Example — plan mode (read-only):
	//
	//   ctx.SetActiveTools([]string{"Read", "Glob", "Grep", "LS"})
	SetActiveTools func(names []string)

	// Exit triggers a graceful application shutdown. In interactive mode
	// this sends a quit signal to the TUI; in non-interactive mode it
	// cancels the current operation. Safe to call from any goroutine.
	//
	// Example:
	//
	//   api.RegisterCommand(ext.CommandDef{
	//       Name:        "quit",
	//       Description: "Exit the application",
	//       Execute: func(args string, ctx ext.Context) (string, error) {
	//           ctx.Exit()
	//           return "", nil
	//       },
	//   })
	Exit func()

	// Complete makes a standalone LLM completion call, bypassing the agent
	// tool loop. Use this for summarisation, question extraction, or any
	// sub-task that needs an LLM response without tool access.
	//
	// If Model is empty the current session model is reused (no extra
	// provider creation overhead). Specify a different model string to
	// use a cheaper/faster model for the sub-task.
	//
	// Example — summarise with a fast model:
	//
	//   resp, err := ctx.Complete(ext.CompleteRequest{
	//       Model:  "anthropic/claude-haiku-3-5-20241022",
	//       System: "You are a concise summarisation assistant.",
	//       Prompt: "Summarise this conversation:\n" + text,
	//   })
	//   if err != nil {
	//       ctx.PrintError("completion failed: " + err.Error())
	//       return
	//   }
	//   ctx.PrintInfo(resp.Text)
	//
	// Example — streaming completion:
	//
	//   resp, err := ctx.Complete(ext.CompleteRequest{
	//       Prompt: "Explain quantum computing",
	//       OnChunk: func(chunk string) {
	//           fmt.Print(chunk) // stream to stdout
	//       },
	//   })
	Complete func(CompleteRequest) (CompleteResponse, error)

	// SuspendTUI temporarily releases the terminal from the TUI, runs the
	// provided callback (which may spawn interactive processes like vim or
	// htop), and then restores the TUI. In non-interactive mode the
	// callback runs directly with no terminal changes.
	//
	// The callback has full access to stdin/stdout/stderr while the TUI is
	// suspended. Return from the callback to restore the TUI.
	//
	// Example — launch $EDITOR:
	//
	//   err := ctx.SuspendTUI(func() {
	//       editor := os.Getenv("EDITOR")
	//       if editor == "" { editor = "vim" }
	//       cmd := exec.Command(editor, "file.go")
	//       cmd.Stdin = os.Stdin
	//       cmd.Stdout = os.Stdout
	//       cmd.Stderr = os.Stderr
	//       cmd.Run()
	//   })
	SuspendTUI func(callback func()) error

	// RenderMessage outputs text using a named message renderer registered
	// by an extension via api.RegisterMessageRenderer(). If no renderer
	// with the given name exists, the content is printed as plain text.
	//
	// This allows extensions to define reusable visual styles (borders,
	// colors, formatting) for specific message categories and invoke them
	// by name at runtime.
	//
	// Example:
	//
	//   ctx.RenderMessage("build-status", "All 42 tests passed.")
	RenderMessage func(rendererName string, content string)

	// RegisterTheme adds a named theme to the runtime theme registry.
	// If a theme with the same name already exists it is replaced.
	// The theme becomes available via /theme and ctx.SetTheme().
	//
	// Example:
	//
	//   ctx.RegisterTheme("neon", ext.ThemeColorConfig{
	//       Primary:   ext.ThemeColor{Dark: "#FF00FF"},
	//       Secondary: ext.ThemeColor{Dark: "#00FFFF"},
	//       Success:   ext.ThemeColor{Dark: "#00FF00"},
	//       Warning:   ext.ThemeColor{Dark: "#FFFF00"},
	//       Error:     ext.ThemeColor{Dark: "#FF0000"},
	//       Info:      ext.ThemeColor{Dark: "#00FFFF"},
	//       Text:      ext.ThemeColor{Dark: "#FFFFFF"},
	//       Background: ext.ThemeColor{Dark: "#000000"},
	//   })
	RegisterTheme func(name string, config ThemeColorConfig)

	// SetTheme switches the active color theme by name. The name must
	// match a built-in theme, a user/project theme file, or a theme
	// registered via RegisterTheme. Returns an error if not found.
	//
	// Example:
	//
	//   err := ctx.SetTheme("neon")
	SetTheme func(name string) error

	// ListThemes returns the names of all available themes.
	ListThemes func() []string

	// ReloadExtensions hot-reloads all extensions from disk. Existing
	// extensions receive a SessionShutdown event, then new code is loaded
	// and receives a SessionStart event. Event handlers, commands,
	// renderers, and shortcuts update immediately; extension-defined tools
	// are NOT updated (they are baked into the agent at creation time).
	//
	// After calling ReloadExtensions the calling extension's code has been
	// replaced; the caller should return promptly.
	//
	// Example:
	//
	//   api.RegisterCommand(ext.CommandDef{
	//       Name: "reload",
	//       Description: "Hot-reload all extensions",
	//       Execute: func(args string, ctx ext.Context) (string, error) {
	//           if err := ctx.ReloadExtensions(); err != nil {
	//               return "", err
	//           }
	//           return "Extensions reloaded", nil
	//       },
	//   })
	ReloadExtensions func() error

	// SpawnSubagent spawns a child Kit instance to perform a task autonomously.
	// The subagent runs as a separate subprocess with full tool access but
	// isolated session and extensions (--no-session --no-extensions).
	//
	// When config.Blocking is true, blocks until completion and returns the
	// result directly (handle is nil). When false, returns immediately with
	// a handle for monitoring/cancellation.
	//
	// Example — blocking call:
	//
	//   _, result, err := ctx.SpawnSubagent(ext.SubagentConfig{
	//       Prompt:   "Research authentication patterns in this codebase",
	//       Blocking: true,
	//       Timeout:  2 * time.Minute,
	//   })
	//   if err != nil {
	//       ctx.PrintError("spawn failed: " + err.Error())
	//       return
	//   }
	//   ctx.PrintInfo("Subagent result:\n" + result.Response)
	//
	// Example — background spawn with callbacks:
	//
	//   handle, _, _ := ctx.SpawnSubagent(ext.SubagentConfig{
	//       Prompt: "Write unit tests for UserService",
	//       OnOutput: func(chunk string) {
	//           // Live output streaming
	//       },
	//       OnComplete: func(result ext.SubagentResult) {
	//           ctx.SendMessage("Subagent finished:\n" + result.Response)
	//       },
	//   })
	//   // handle.Kill() to cancel, handle.Wait() to block
	SpawnSubagent func(SubagentConfig) (*SubagentHandle, *SubagentResult, error)
}

// ---------------------------------------------------------------------------
// Session types (exposed to Yaegi — concrete structs for session access)
// ---------------------------------------------------------------------------

// SessionMessage represents a conversation message exposed to extensions.
// This is a simplified, read-only view of the internal message structures.
type SessionMessage struct {
	// ID is the unique entry identifier in the session tree.
	ID string
	// ParentID links this entry to its parent in the tree.
	ParentID string
	// Role is the message role: "user", "assistant", "tool", or "system".
	Role string
	// Content is the text content of the message (tool calls and results
	// are serialized as text summaries).
	Content string
	// Model is the model that generated this message (empty for user messages).
	Model string
	// Provider is the provider used (empty for user messages).
	Provider string
	// Timestamp is the RFC3339-formatted creation time.
	Timestamp string
}

// ExtensionEntry represents persisted extension data stored in the session.
// Extensions use AppendEntry to save custom state and GetEntries to retrieve
// it on session resume.
type ExtensionEntry struct {
	// ID is the unique entry identifier.
	ID string
	// EntryType is the extension-defined type string (e.g. "plan-mode:state").
	EntryType string
	// Data is the extension-defined payload (JSON or plain text).
	Data string
	// Timestamp is the RFC3339-formatted creation time.
	Timestamp string
}

// ---------------------------------------------------------------------------
// Context filtering types (exposed to Yaegi — concrete structs)
// ---------------------------------------------------------------------------

// ContextMessage represents a single message in the LLM context window.
// Used by OnContextPrepare to let extensions inspect and modify the messages
// that will be sent to the LLM.
type ContextMessage struct {
	// Index is the position of this message in the original context array
	// (0-based). When returning messages from a ContextPrepareResult,
	// messages with Index >= 0 reuse the original fantasy.Message at that
	// position (preserving tool calls, reasoning, and other complex parts).
	// Set Index to -1 for newly injected messages (created from Role + Content).
	Index int

	// Role is the message role: "user", "assistant", "system", or "tool".
	Role string

	// Content is the text content of the message. For assistant messages
	// with tool calls, this includes a text summary of the calls.
	Content string
}

// ---------------------------------------------------------------------------
// LLM completion types (exposed to Yaegi — concrete structs)
// ---------------------------------------------------------------------------

// CompleteRequest configures a standalone LLM completion call. Extensions use
// this with ctx.Complete() to make direct LLM calls without the agent tool loop.
type CompleteRequest struct {
	// Model is the model to use in "provider/model" format (e.g.
	// "anthropic/claude-haiku-3-5-20241022"). Empty string uses the current
	// session model, avoiding extra provider creation overhead.
	Model string

	// Prompt is the user input text sent to the model.
	Prompt string

	// System is an optional system prompt. Empty uses no system prompt.
	System string

	// Messages is optional conversation history. If provided, Prompt is
	// appended as the final user message.
	Messages []SessionMessage

	// MaxTokens limits the response length (0 = provider default).
	MaxTokens int

	// OnChunk is called for each streaming text delta. When set, the
	// completion is performed in streaming mode. When nil, the call blocks
	// until the full response is available.
	OnChunk func(chunk string)
}

// CompleteResponse contains the LLM response and usage metadata from a
// standalone completion call.
type CompleteResponse struct {
	// Text is the complete response text.
	Text string

	// InputTokens is the number of tokens in the request.
	InputTokens int

	// OutputTokens is the number of tokens in the response.
	OutputTokens int

	// Model is the actual model used (useful when CompleteRequest.Model was empty).
	Model string
}

// ---------------------------------------------------------------------------
// Status bar types (exposed to Yaegi — concrete structs)
// ---------------------------------------------------------------------------

// StatusBarEntry represents a keyed entry in the TUI status bar. Extensions
// can set multiple independent entries that render alongside the built-in
// model name and token usage display.
type StatusBarEntry struct {
	// Key uniquely identifies this entry (e.g. "myext:git-branch").
	Key string
	// Text is the rendered content shown in the status bar.
	Text string
	// Priority controls ordering. Lower values render further left.
	// Built-in entries (model, usage) have implicit priority 100-110.
	Priority int
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
	onToolCall                func(func(ToolCallEvent, Context) *ToolCallResult)
	onToolExecStart           func(func(ToolExecutionStartEvent, Context))
	onToolExecEnd             func(func(ToolExecutionEndEvent, Context))
	onToolOutput              func(func(ToolOutputEvent, Context))
	onToolResult              func(func(ToolResultEvent, Context) *ToolResultResult)
	onInput                   func(func(InputEvent, Context) *InputResult)
	onBeforeAgentStart        func(func(BeforeAgentStartEvent, Context) *BeforeAgentStartResult)
	onAgentStart              func(func(AgentStartEvent, Context))
	onAgentEnd                func(func(AgentEndEvent, Context))
	onMessageStart            func(func(MessageStartEvent, Context))
	onMessageUpdate           func(func(MessageUpdateEvent, Context))
	onMessageEnd              func(func(MessageEndEvent, Context))
	onSessionStart            func(func(SessionStartEvent, Context))
	onSessionShutdown         func(func(SessionShutdownEvent, Context))
	registerToolFn            func(ToolDef)
	registerCmdFn             func(CommandDef)
	registerToolRendererFn    func(ToolRenderConfig)
	onModelChange             func(func(ModelChangeEvent, Context))
	onContextPrepare          func(func(ContextPrepareEvent, Context) *ContextPrepareResult)
	onBeforeFork              func(func(BeforeForkEvent, Context) *BeforeForkResult)
	onBeforeSessionSwitch     func(func(BeforeSessionSwitchEvent, Context) *BeforeSessionSwitchResult)
	onBeforeCompact           func(func(BeforeCompactEvent, Context) *BeforeCompactResult)
	onCustomEvent             func(name string, handler func(string))
	registerOption            func(OptionDef)
	registerShortcutFn        func(ShortcutDef, func(Context))
	registerMessageRendererFn func(MessageRendererConfig)
	onSubagentStart           func(func(SubagentStartEvent, Context))
	onSubagentChunk           func(func(SubagentChunkEvent, Context))
	onSubagentEnd             func(func(SubagentEndEvent, Context))
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

// OnToolOutput registers a handler for streaming tool output chunks.
// This fires for each output line as it arrives from tools like bash,
// allowing extensions to observe or process output in real-time.
func (a *API) OnToolOutput(handler func(ToolOutputEvent, Context)) {
	a.onToolOutput(handler)
}

// OnToolResult registers a handler that fires after tool execution.
// Return a non-nil ToolResultResult to modify the output.
func (a *API) OnToolResult(handler func(ToolResultEvent, Context) *ToolResultResult) {
	a.onToolResult(handler)
}

// OnSubagentStart registers a handler that fires when a spawn_subagent tool
// call begins executing. Use the ToolCallID to correlate with subsequent
// OnSubagentChunk and OnSubagentEnd events for the same subagent.
func (a *API) OnSubagentStart(handler func(SubagentStartEvent, Context)) {
	a.onSubagentStart(handler)
}

// OnSubagentChunk registers a handler for real-time events from a running
// subagent. ChunkType identifies the kind of event ("text", "tool_call",
// "tool_result", "tool_execution_start", "tool_execution_end", etc.).
// Correlate with OnSubagentStart via the ToolCallID field.
func (a *API) OnSubagentChunk(handler func(SubagentChunkEvent, Context)) {
	a.onSubagentChunk(handler)
}

// OnSubagentEnd registers a handler that fires when a spawn_subagent call
// completes. ErrorMsg is non-empty when the subagent failed.
func (a *API) OnSubagentEnd(handler func(SubagentEndEvent, Context)) {
	a.onSubagentEnd(handler)
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

// OnModelChange registers a handler that fires after the active model is
// changed via ctx.SetModel(). The handler receives the new and previous model
// strings plus the source of the change.
func (a *API) OnModelChange(handler func(ModelChangeEvent, Context)) {
	a.onModelChange(handler)
}

// OnContextPrepare registers a handler that fires after the context window is
// built from the session tree (including compaction) and before the messages
// are sent to the LLM. The handler can inspect the context and return a
// modified message set to filter, reorder, or inject messages.
//
// Return nil to leave the context unchanged. Return a non-nil result with
// a Messages slice to replace the context window entirely. Messages with a
// non-negative Index reuse the original message at that position (preserving
// tool calls, reasoning parts, etc.); messages with Index < 0 are created
// fresh from Role + Content.
//
// Example — inject a RAG context message:
//
//	api.OnContextPrepare(func(e ext.ContextPrepareEvent, ctx ext.Context) *ext.ContextPrepareResult {
//	    ragContext := fetchRelevantDocs(e.Messages[len(e.Messages)-1].Content)
//	    injected := ext.ContextMessage{Index: -1, Role: "system", Content: ragContext}
//	    msgs := append([]ext.ContextMessage{injected}, e.Messages...)
//	    return &ext.ContextPrepareResult{Messages: msgs}
//	})
func (a *API) OnContextPrepare(handler func(ContextPrepareEvent, Context) *ContextPrepareResult) {
	a.onContextPrepare(handler)
}

// RegisterTool adds a custom tool that the LLM can invoke.
func (a *API) RegisterTool(tool ToolDef) {
	a.registerToolFn(tool)
}

// RegisterCommand adds a slash command available in interactive mode.
func (a *API) RegisterCommand(cmd CommandDef) {
	a.registerCmdFn(cmd)
}

// RegisterOption declares a named configuration option. The option can be set
// via environment variables (KIT_OPT_<NAME>) or config file (options.<name>).
// Multiple extensions can register options with the same name; the last default
// wins.
func (a *API) RegisterOption(opt OptionDef) {
	a.registerOption(opt)
}

// RegisterShortcut registers a global keyboard shortcut that fires across
// all app states except modal prompts/overlays. Use modifier combinations
// like "ctrl+p", "alt+t", or "f1" — avoid bare characters that conflict
// with text input. If multiple extensions register the same key, the last
// registration wins. The handler runs in a goroutine so it can call blocking
// APIs like PromptSelect without stalling the TUI event loop.
func (a *API) RegisterShortcut(def ShortcutDef, handler func(Context)) {
	if a.registerShortcutFn != nil {
		a.registerShortcutFn(def, handler)
	}
}

// OnCustomEvent registers a handler for a custom inter-extension event.
// The handler receives the data string published by EmitCustomEvent.
// Multiple handlers can subscribe to the same event name; they execute
// in registration order.
func (a *API) OnCustomEvent(name string, handler func(string)) {
	a.onCustomEvent(name, handler)
}

// OnBeforeFork registers a handler that fires before the session tree is
// branched to a different entry point. Return a non-nil BeforeForkResult
// with Cancel=true to prevent the fork.
func (a *API) OnBeforeFork(handler func(BeforeForkEvent, Context) *BeforeForkResult) {
	a.onBeforeFork(handler)
}

// OnBeforeSessionSwitch registers a handler that fires before the session
// is switched to a new branch (e.g. /new command). Return a non-nil
// BeforeSessionSwitchResult with Cancel=true to prevent the switch.
func (a *API) OnBeforeSessionSwitch(handler func(BeforeSessionSwitchEvent, Context) *BeforeSessionSwitchResult) {
	a.onBeforeSessionSwitch(handler)
}

// OnBeforeCompact registers a handler that fires before context compaction
// runs. Return a non-nil BeforeCompactResult with Cancel=true to prevent
// compaction from proceeding.
func (a *API) OnBeforeCompact(handler func(BeforeCompactEvent, Context) *BeforeCompactResult) {
	a.onBeforeCompact(handler)
}

// RegisterToolRenderer registers a custom renderer for a specific tool's
// display in the TUI. The renderer controls the header (parameter summary)
// and/or body (result display) of the tool's output block. If multiple
// extensions register renderers for the same tool name, the last one wins.
func (a *API) RegisterToolRenderer(config ToolRenderConfig) {
	a.registerToolRendererFn(config)
}

// RegisterMessageRenderer registers a named message renderer that extensions
// can invoke via ctx.RenderMessage(name, content). Use this to define
// reusable visual styles for branded output, progress reports, or custom
// notification formats. If multiple extensions register the same name, the
// last one wins.
func (a *API) RegisterMessageRenderer(config MessageRendererConfig) {
	if a.registerMessageRendererFn != nil {
		a.registerMessageRendererFn(config)
	}
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

// PromptMultiSelectConfig configures a multi-selection prompt that allows
// the user to toggle multiple options and confirm their selection.
type PromptMultiSelectConfig struct {
	// Message is the question or instruction displayed to the user.
	Message string
	// Options is the list of choices the user can select from.
	Options []string
	// DefaultSelected contains indices of options that should be
	// pre-selected when the prompt appears. If nil, all options are selected.
	DefaultSelected []int
}

// PromptMultiSelectResult is the response from a multi-selection prompt.
type PromptMultiSelectResult struct {
	// Values contains the text of selected options.
	Values []string
	// Indices contains the zero-based indices of selected options.
	Indices []int
	// Cancelled is true if the user dismissed the prompt (ESC) or
	// the prompt was unavailable (non-interactive mode).
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
// UI visibility (exposed to Yaegi — concrete struct)
// ---------------------------------------------------------------------------

// UIVisibility controls which built-in TUI chrome elements are visible.
// The zero value shows everything (backward compatible). Extensions call
// ctx.SetUIVisibility to customise the layout — for example, a "minimal"
// theme can hide the startup banner, status bar, and input hint and replace
// them with a single custom footer.
type UIVisibility struct {
	HideStartupMessage bool // Hide the "Model loaded..." startup block
	HideStatusBar      bool // Hide the "provider · model  Tokens: ..." line
	HideSeparator      bool // Hide the "────────" divider between stream and input
	HideInputHint      bool // Hide the "enter submit · ctrl+j..." hint below input
}

// ---------------------------------------------------------------------------
// Context stats (exposed to Yaegi — concrete struct)
// ---------------------------------------------------------------------------

// ContextStats contains current context-window usage information.
// Extensions can poll this via ctx.GetContextStats() to build usage
// meters, auto-compaction triggers, etc.
type ContextStats struct {
	EstimatedTokens int     // Estimated token count of the current conversation
	ContextLimit    int     // Model's context window size (tokens), 0 if unknown
	UsagePercent    float64 // Fraction of context used (0.0–1.0), 0 if limit unknown
	MessageCount    int     // Number of messages in the conversation
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
// Model info types (exposed to Yaegi — concrete structs)
// ---------------------------------------------------------------------------

// ModelInfoEntry represents a known model from the registry. Used by
// GetAvailableModels to let extensions discover which models are available.
type ModelInfoEntry struct {
	// Provider is the provider ID (e.g. "anthropic", "openai").
	Provider string
	// ModelID is the model identifier (e.g. "claude-sonnet-4-5-20250929").
	ModelID string
	// Name is the human-readable model name.
	Name string
	// ContextLimit is the maximum context window in tokens (0 if unknown).
	ContextLimit int
	// OutputLimit is the maximum output tokens (0 if unknown).
	OutputLimit int
	// Reasoning is true if the model supports extended thinking.
	Reasoning bool
}

// ---------------------------------------------------------------------------
// Tool info types (exposed to Yaegi — concrete structs)
// ---------------------------------------------------------------------------

// ToolInfo provides read-only information about a tool available to the agent.
// Used by GetAllTools to let extensions inspect and filter the tool set.
type ToolInfo struct {
	// Name is the tool's unique identifier.
	Name string
	// Description is the tool's human-readable description.
	Description string
	// Source indicates where the tool came from: "core", "mcp", or "extension".
	Source string
	// Enabled is true if the tool is currently active.
	Enabled bool
}

// ---------------------------------------------------------------------------
// ToolDef / CommandDef
// ---------------------------------------------------------------------------

// ToolContext provides runtime context to a tool's ExecuteWithContext handler.
// It allows tools to check for cancellation and report progress while running.
type ToolContext struct {
	// IsCancelled returns true when the tool's execution has been cancelled
	// (e.g. the user interrupted the agent or the request timed out).
	// Long-running tools should poll this periodically and return early.
	IsCancelled func() bool
	// OnProgress sends a progress message that is displayed in the TUI
	// while the tool is executing. Useful for long-running operations
	// that want to show incremental status.
	OnProgress func(text string)
}

// ToolDef describes a custom tool registered by an extension.
type ToolDef struct {
	Name        string
	Description string
	Parameters  string // JSON Schema string
	// Execute is the simple handler — receives JSON input, returns text result.
	// Use this for tools that don't need cancellation or progress reporting.
	Execute func(input string) (string, error)
	// ExecuteWithContext is the rich handler — receives JSON input plus a
	// ToolContext that provides cancellation checking and progress reporting.
	// If both Execute and ExecuteWithContext are set, ExecuteWithContext wins.
	ExecuteWithContext func(input string, tc ToolContext) (string, error)
}

// CommandDef describes a slash command registered by an extension.
type CommandDef struct {
	Name        string
	Description string
	Execute     func(args string, ctx Context) (string, error)
	// Complete provides argument tab-completion for this command.
	// Called with the partial argument text typed so far; returns
	// candidate completions. Nil means no argument completion.
	Complete func(prefix string, ctx Context) []string
}

// ---------------------------------------------------------------------------
// Keyboard shortcuts (exposed to Yaegi — concrete structs)
// ---------------------------------------------------------------------------

// ShortcutDef describes a global keyboard shortcut registered by an extension.
// Shortcuts fire across all app states except modal prompts/overlays.
// Use modifier combinations (e.g., "ctrl+p", "alt+t", "f1") — avoid bare
// characters like "a" or "x" which conflict with text input.
type ShortcutDef struct {
	// Key is the key binding (e.g., "ctrl+p", "alt+t", "f1", "ctrl+shift+s").
	Key string
	// Description explains what the shortcut does (shown in /shortcuts help).
	Description string
}

// ---------------------------------------------------------------------------
// Custom message rendering (exposed to Yaegi — concrete structs)
// ---------------------------------------------------------------------------

// MessageRendererConfig provides a named rendering function that extensions
// can invoke via ctx.RenderMessage(name, content). Unlike tool renderers
// (which hook into the automatic tool result display), message renderers are
// invoked explicitly by extension code for branded status updates, progress
// reports, or any custom visual output.
//
// Example:
//
//	api.RegisterMessageRenderer(ext.MessageRendererConfig{
//	    Name: "build-status",
//	    Render: func(content string, width int) string {
//	        border := strings.Repeat("─", width-4)
//	        return "╭" + border + "╮\n│ " + content + "\n╰" + border + "╯"
//	    },
//	})
type MessageRendererConfig struct {
	// Name uniquely identifies this renderer. Used by ctx.RenderMessage
	// to look it up at call time. Should be namespaced to avoid collisions
	// (e.g. "myext:build-status").
	Name string

	// Render produces the styled output string from raw content. Receives
	// the content and the terminal width in columns. Return the final
	// ANSI-styled string to print; it will be emitted via tea.Println
	// (or plain stdout in non-interactive mode).
	Render func(content string, width int) string
}

// ---------------------------------------------------------------------------
// Extension options (exposed to Yaegi — concrete structs)
// ---------------------------------------------------------------------------

// OptionDef describes a configuration option that an extension can register.
// Options are resolved from env vars, config file, or default value.
type OptionDef struct {
	// Name is the option identifier. Used as:
	//   - Env var: KIT_OPT_<NAME> (uppercased, dashes → underscores)
	//   - Config key: options.<name> in .kit.yml
	Name string
	// Description explains what the option controls.
	Description string
	// Default is the fallback value if not set via env or config.
	Default string
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
// Uses concrete function fields instead of interfaces for Yaegi safety.
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
	ToolKind   string         // Tool classification: "execute", "edit", "read", "search", "agent"
	Input      string         // JSON-encoded tool parameters
	ParsedArgs map[string]any // Pre-parsed arguments for convenience (nil on parse failure)
	// Source indicates who initiated the tool call.
	// Currently always "llm" (all tool calls originate from the LLM agent loop).
	// Future user-initiated tool features may set this to "user".
	Source string
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
	ToolCallID string
	ToolName   string
	ToolKind   string
}

func (e ToolExecutionStartEvent) Type() EventType { return ToolExecutionStart }

// ToolExecutionEndEvent fires when a tool finishes executing.
type ToolExecutionEndEvent struct {
	ToolCallID string
	ToolName   string
	ToolKind   string
}

func (e ToolExecutionEndEvent) Type() EventType { return ToolExecutionEnd }

// ToolOutputEvent fires when a tool produces streaming output chunks.
// This is primarily used for long-running tools like bash to show output
// in real-time as it arrives, before the tool completes.
type ToolOutputEvent struct {
	ToolCallID string
	ToolName   string
	ToolKind   string
	Chunk      string // Output text chunk
	IsStderr   bool   // Whether this chunk came from stderr
}

func (e ToolOutputEvent) Type() EventType { return ToolOutput }

// ToolResultEvent fires after tool execution with the output.
type ToolResultEvent struct {
	ToolCallID string
	ToolName   string
	ToolKind   string
	Input      string
	Content    string
	IsError    bool
	Metadata   string // Optional JSON-encoded structured metadata (e.g. file diffs)
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

// ModelChangeEvent fires after the active model is changed via ctx.SetModel().
type ModelChangeEvent struct {
	// NewModel is the model string that was set (e.g. "anthropic/claude-sonnet-4-5-20250929").
	NewModel string
	// PreviousModel is the model string before the change.
	PreviousModel string
	// Source indicates what triggered the change: "extension" for ctx.SetModel(),
	// "user" for interactive model selection.
	Source string
}

func (e SessionShutdownEvent) Type() EventType { return SessionShutdown }

func (e ModelChangeEvent) Type() EventType { return ModelChange }

// ContextPrepareEvent fires after the context window is built from the session
// tree and before the messages are sent to the LLM. Handlers can inspect the
// messages and return a modified set to filter, reorder, or inject context.
type ContextPrepareEvent struct {
	// Messages is the current context window that will be sent to the LLM.
	// Each ContextMessage includes an Index field that maps back to the
	// position in the original message array (for identity-preserving edits).
	Messages []ContextMessage
}

func (e ContextPrepareEvent) Type() EventType { return ContextPrepare }

// ContextPrepareResult allows extensions to replace the context window.
// Return nil to leave the context unchanged.
type ContextPrepareResult struct {
	// Messages replaces the entire context window. Each entry with a
	// non-negative Index reuses the original message at that position
	// (preserving tool calls, reasoning, etc.); entries with Index < 0
	// are created fresh from Role + Content.
	Messages []ContextMessage
}

func (ContextPrepareResult) isResult() {}

// BeforeForkEvent fires before the session tree is branched to a different
// entry point (via the tree selector or /fork command).
type BeforeForkEvent struct {
	// TargetID is the session entry ID being branched to.
	TargetID string
	// IsUserMessage is true if the selected entry is a user message
	// (which causes the fork to target the parent entry).
	IsUserMessage bool
	// UserText is the user message text (non-empty only when IsUserMessage is true).
	UserText string
}

func (e BeforeForkEvent) Type() EventType { return BeforeFork }

// BeforeForkResult controls whether the fork proceeds. Return Cancel=true
// with an optional Reason to block the fork.
type BeforeForkResult struct {
	// Cancel, when true, prevents the fork from proceeding.
	Cancel bool
	// Reason is a human-readable explanation shown to the user when
	// Cancel is true. Empty string uses a default message.
	Reason string
}

func (BeforeForkResult) isResult() {}

// BeforeSessionSwitchEvent fires before the session is switched to a new
// branch (e.g. /new or /clear commands).
type BeforeSessionSwitchEvent struct {
	// Reason describes why the switch is happening: "new" for /new command,
	// "clear" for /clear command.
	Reason string
}

func (e BeforeSessionSwitchEvent) Type() EventType { return BeforeSessionSwitch }

// BeforeSessionSwitchResult controls whether the session switch proceeds.
// Return Cancel=true with an optional Reason to block the switch.
type BeforeSessionSwitchResult struct {
	// Cancel, when true, prevents the session switch from proceeding.
	Cancel bool
	// Reason is a human-readable explanation shown to the user when
	// Cancel is true. Empty string uses a default message.
	Reason string
}

func (BeforeSessionSwitchResult) isResult() {}

// BeforeCompactEvent fires before context compaction runs. Provides
// information about the current context state to help extensions decide
// whether to allow or block compaction.
type BeforeCompactEvent struct {
	// EstimatedTokens is the estimated token count of the conversation.
	EstimatedTokens int
	// ContextLimit is the model's context window size in tokens.
	ContextLimit int
	// UsagePercent is the fraction of context used (0.0–1.0).
	UsagePercent float64
	// MessageCount is the number of messages in the conversation.
	MessageCount int
	// IsAutomatic is true when compaction was triggered automatically
	// (as opposed to manual /compact command).
	IsAutomatic bool
}

func (e BeforeCompactEvent) Type() EventType { return BeforeCompact }

// BeforeCompactResult controls whether compaction proceeds. Return
// Cancel=true with an optional Reason to block compaction, or provide
// a custom Summary to replace the default LLM-generated one.
type BeforeCompactResult struct {
	// Cancel, when true, prevents compaction from proceeding.
	Cancel bool
	// Reason is a human-readable explanation shown to the user when
	// Cancel is true. Empty string uses a default message.
	Reason string
	// Summary, when non-empty, replaces the default LLM-generated summary.
	// The extension is responsible for generating a useful summary.
	// Ignored when Cancel is true.
	Summary string
}

func (BeforeCompactResult) isResult() {}

// ---------------------------------------------------------------------------
// Subagent lifecycle events (exposed to Yaegi — concrete structs)
// ---------------------------------------------------------------------------

// SubagentStartEvent fires when a spawn_subagent tool call begins executing.
type SubagentStartEvent struct {
	// ToolCallID is the LLM-assigned ID of the spawn_subagent tool call.
	// Use this to correlate SubagentChunkEvent and SubagentEndEvent.
	ToolCallID string
	// Task is the task description passed to the subagent.
	Task string
}

func (e SubagentStartEvent) Type() EventType { return SubagentStart }

// SubagentChunkEvent fires for each real-time event from a running subagent.
// Type field indicates the kind of event; read the relevant fields accordingly.
type SubagentChunkEvent struct {
	// ToolCallID matches the SubagentStartEvent.ToolCallID for this subagent.
	ToolCallID string
	// Task is the task description (repeated for convenience).
	Task string
	// ChunkType identifies the event kind:
	//   "text"                 — LLM text chunk (read Content)
	//   "reasoning"            — reasoning/thinking delta (read Content)
	//   "tool_call"            — subagent called a tool (read ToolName, ToolArgs)
	//   "tool_result"          — tool returned a result (read ToolName, ToolResult, IsError)
	//   "tool_execution_start" — tool began executing (read ToolName)
	//   "tool_execution_end"   — tool finished executing (read ToolName)
	//   "turn_start"           — subagent turn began
	//   "turn_end"             — subagent turn ended
	ChunkType string
	// Content carries text for "text" and "reasoning" chunk types.
	Content string
	// ToolName is set on tool-related chunk types.
	ToolName string
	// ToolArgs is the JSON-encoded tool arguments for "tool_call" chunks.
	ToolArgs string
	// ToolResult is the tool output for "tool_result" chunks.
	ToolResult string
	// IsError is true when a "tool_result" chunk represents an error.
	IsError bool
}

func (e SubagentChunkEvent) Type() EventType { return SubagentChunk }

// SubagentEndEvent fires when a spawn_subagent tool call completes.
type SubagentEndEvent struct {
	// ToolCallID matches the SubagentStartEvent.ToolCallID for this subagent.
	ToolCallID string
	// Task is the task description.
	Task string
	// Response is the subagent's final text response (empty on error).
	Response string
	// ErrorMsg is non-empty when the subagent failed.
	ErrorMsg string
}

func (e SubagentEndEvent) Type() EventType { return SubagentEnd }

// ThemeColor is an adaptive color pair with light and dark hex values.
// Either field may be empty to inherit from the default theme.
type ThemeColor struct {
	Light string
	Dark  string
}

// ThemeColorConfig defines a complete color theme that extensions can register
// programmatically via ctx.RegisterTheme(). Uses plain hex strings (not
// color.Color) so the type is safe to pass across the Yaegi boundary.
type ThemeColorConfig struct {
	Primary     ThemeColor
	Secondary   ThemeColor
	Success     ThemeColor
	Warning     ThemeColor
	Error       ThemeColor
	Info        ThemeColor
	Text        ThemeColor
	Muted       ThemeColor
	VeryMuted   ThemeColor
	Background  ThemeColor
	Border      ThemeColor
	MutedBorder ThemeColor
	System      ThemeColor
	Tool        ThemeColor
	Accent      ThemeColor
	Highlight   ThemeColor

	// Markdown/syntax highlighting overrides.
	MdHeading ThemeColor
	MdLink    ThemeColor
	MdKeyword ThemeColor
	MdString  ThemeColor
	MdNumber  ThemeColor
	MdComment ThemeColor
}
