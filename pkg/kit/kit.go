package kit

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"charm.land/fantasy"

	"github.com/mark3labs/kit/internal/agent"
	"github.com/mark3labs/kit/internal/config"
	"github.com/mark3labs/kit/internal/core"
	"github.com/mark3labs/kit/internal/extensions"
	"github.com/mark3labs/kit/internal/kitsetup"
	"github.com/mark3labs/kit/internal/message"
	"github.com/mark3labs/kit/internal/models"
	"github.com/mark3labs/kit/internal/session"
	"github.com/mark3labs/kit/internal/skills"
	"github.com/mark3labs/kit/internal/tools"

	"github.com/spf13/viper"
)

// ContextFile represents a project context file (e.g. AGENTS.md) that was
// loaded during initialization and injected into the system prompt.
type ContextFile struct {
	Path    string // Absolute filesystem path.
	Content string // Full file content.
}

// Kit provides programmatic access to kit functionality, allowing
// integration of MCP tools and LLM interactions into Go applications. It manages
// agents, sessions, and model configurations.
type Kit struct {
	agent          *agent.Agent
	treeSession    *session.TreeManager
	modelString    string
	events         *eventBus
	autoCompact    bool
	compactionOpts *CompactionOptions
	contextFiles   []*ContextFile
	skills         []*skills.Skill
	extRunner      *extensions.Runner
	bufferedLogger *tools.BufferedDebugLogger

	// Hook registries — interception layer (see hooks.go).
	beforeToolCall  *hookRegistry[BeforeToolCallHook, BeforeToolCallResult]
	afterToolResult *hookRegistry[AfterToolResultHook, AfterToolResultResult]
	beforeTurn      *hookRegistry[BeforeTurnHook, BeforeTurnResult]
	afterTurn       *hookRegistry[AfterTurnHook, AfterTurnResult]
	contextPrepare  *hookRegistry[ContextPrepareHook, ContextPrepareResult]
	beforeCompact   *hookRegistry[BeforeCompactHook, BeforeCompactResult]

	// lastInputTokens stores the API-reported input token count from the
	// most recent turn. Used by GetContextStats() to return accurate usage
	// instead of the text-based heuristic which misses system prompts,
	// tool definitions, etc.
	lastInputTokensMu sync.RWMutex
	lastInputTokens   int
}

// Subscribe registers an EventListener that will be called for every lifecycle
// event emitted during Prompt() and PromptWithCallbacks(). Returns an
// unsubscribe function that removes the listener.
func (m *Kit) Subscribe(listener EventListener) func() {
	return m.events.subscribe(listener)
}

// GetExtRunner returns the extension runner (nil if extensions are disabled).
//
// Deprecated: Use SetExtensionContext and EmitSessionStart instead. GetExtRunner
// leaks the internal extensions.Runner type across the SDK boundary.
func (m *Kit) GetExtRunner() *extensions.Runner { return m.extRunner }

// GetBufferedLogger returns the buffered debug logger (nil if not configured).
//
// Deprecated: Use GetBufferedDebugMessages instead.
func (m *Kit) GetBufferedLogger() *tools.BufferedDebugLogger { return m.bufferedLogger }

// GetAgent returns the underlying agent.
//
// Deprecated: Use GetToolNames, GetLoadingMessage, GetLoadedServerNames,
// GetMCPToolCount, GetExtensionToolCount instead.
func (m *Kit) GetAgent() *agent.Agent { return m.agent }

// --------------------------------------------------------------------------
// Narrow accessors — prefer these over GetAgent/GetExtRunner/GetBufferedLogger
// --------------------------------------------------------------------------

// GetToolNames returns the names of all tools available to the agent.
func (m *Kit) GetToolNames() []string {
	agentTools := m.agent.GetTools()
	names := make([]string, len(agentTools))
	for i, t := range agentTools {
		names[i] = t.Info().Name
	}
	return names
}

// GetLoadingMessage returns the agent's startup info message (e.g. GPU
// fallback info), or empty string if none.
func (m *Kit) GetLoadingMessage() string {
	return m.agent.GetLoadingMessage()
}

// GetLoadedServerNames returns the names of successfully loaded MCP servers.
func (m *Kit) GetLoadedServerNames() []string {
	return m.agent.GetLoadedServerNames()
}

// GetMCPToolCount returns the number of tools loaded from external MCP servers.
func (m *Kit) GetMCPToolCount() int {
	return m.agent.GetMCPToolCount()
}

// GetExtensionToolCount returns the number of tools registered by extensions.
func (m *Kit) GetExtensionToolCount() int {
	return m.agent.GetExtensionToolCount()
}

// GetBufferedDebugMessages returns any debug messages that were buffered
// during initialization, then clears the buffer. Returns nil if no messages
// were buffered or if buffered logging was not configured.
func (m *Kit) GetBufferedDebugMessages() []string {
	if m.bufferedLogger == nil {
		return nil
	}
	return m.bufferedLogger.GetMessages()
}

// SetExtensionContext configures the extension runner with the given context
// functions. No-op if extensions are disabled.
func (m *Kit) SetExtensionContext(ctx extensions.Context) {
	if m.extRunner != nil {
		m.extRunner.SetContext(ctx)
	}
}

// GetExtensionContext returns the current extension runtime context.
// Returns a zero Context if extensions are disabled.
func (m *Kit) GetExtensionContext() extensions.Context {
	if m.extRunner != nil {
		return m.extRunner.GetContext()
	}
	return extensions.Context{}
}

// UpdateExtensionContextModel updates the Model field on the extension
// context so subsequent event handlers see the new model. This is a
// targeted update that avoids replacing the entire Context struct.
func (m *Kit) UpdateExtensionContextModel(model string) {
	if m.extRunner != nil {
		ctx := m.extRunner.GetContext()
		ctx.Model = model
		m.extRunner.SetContext(ctx)
	}
}

// EmitSessionStart fires the SessionStart event for extensions.
// No-op if extensions are disabled or no handlers are registered.
func (m *Kit) EmitSessionStart() {
	if m.extRunner != nil && m.extRunner.HasHandlers(extensions.SessionStart) {
		_, _ = m.extRunner.Emit(extensions.SessionStartEvent{})
	}
}

// ExtensionCommands returns the slash commands registered by extensions.
// Returns nil if extensions are disabled or no commands are registered.
func (m *Kit) ExtensionCommands() []extensions.CommandDef {
	if m.extRunner == nil {
		return nil
	}
	return m.extRunner.RegisteredCommands()
}

// SetExtensionWidget places or updates a persistent extension widget.
// Delegates to the extension runner. No-op if extensions are disabled.
func (m *Kit) SetExtensionWidget(config extensions.WidgetConfig) {
	if m.extRunner != nil {
		m.extRunner.SetWidget(config)
	}
}

// RemoveExtensionWidget removes a previously placed extension widget by ID.
// Delegates to the extension runner. No-op if extensions are disabled.
func (m *Kit) RemoveExtensionWidget(id string) {
	if m.extRunner != nil {
		m.extRunner.RemoveWidget(id)
	}
}

// GetExtensionWidgets returns extension widgets matching the given placement.
// Returns nil if extensions are disabled or no widgets match.
func (m *Kit) GetExtensionWidgets(placement extensions.WidgetPlacement) []extensions.WidgetConfig {
	if m.extRunner == nil {
		return nil
	}
	return m.extRunner.GetWidgets(placement)
}

// SetExtensionHeader places or replaces the custom header from extensions.
// Delegates to the extension runner. No-op if extensions are disabled.
func (m *Kit) SetExtensionHeader(config extensions.HeaderFooterConfig) {
	if m.extRunner != nil {
		m.extRunner.SetHeader(config)
	}
}

// RemoveExtensionHeader removes the custom extension header.
// Delegates to the extension runner. No-op if extensions are disabled.
func (m *Kit) RemoveExtensionHeader() {
	if m.extRunner != nil {
		m.extRunner.RemoveHeader()
	}
}

// GetExtensionHeader returns the current custom header, or nil if none is set.
// Returns nil if extensions are disabled.
func (m *Kit) GetExtensionHeader() *extensions.HeaderFooterConfig {
	if m.extRunner == nil {
		return nil
	}
	return m.extRunner.GetHeader()
}

// SetExtensionFooter places or replaces the custom footer from extensions.
// Delegates to the extension runner. No-op if extensions are disabled.
func (m *Kit) SetExtensionFooter(config extensions.HeaderFooterConfig) {
	if m.extRunner != nil {
		m.extRunner.SetFooter(config)
	}
}

// RemoveExtensionFooter removes the custom extension footer.
// Delegates to the extension runner. No-op if extensions are disabled.
func (m *Kit) RemoveExtensionFooter() {
	if m.extRunner != nil {
		m.extRunner.RemoveFooter()
	}
}

// GetExtensionFooter returns the current custom footer, or nil if none is set.
// Returns nil if extensions are disabled.
func (m *Kit) GetExtensionFooter() *extensions.HeaderFooterConfig {
	if m.extRunner == nil {
		return nil
	}
	return m.extRunner.GetFooter()
}

// GetExtensionToolRenderer returns the custom renderer for the named tool, or
// nil if no extension registered a renderer for it. Returns nil if extensions
// are disabled.
func (m *Kit) GetExtensionToolRenderer(toolName string) *extensions.ToolRenderConfig {
	if m.extRunner == nil {
		return nil
	}
	return m.extRunner.GetToolRenderer(toolName)
}

// SetExtensionEditor installs an editor interceptor from extensions.
// Delegates to the extension runner. No-op if extensions are disabled.
func (m *Kit) SetExtensionEditor(config extensions.EditorConfig) {
	if m.extRunner != nil {
		m.extRunner.SetEditor(config)
	}
}

// ResetExtensionEditor removes the active editor interceptor from extensions.
// Delegates to the extension runner. No-op if extensions are disabled.
func (m *Kit) ResetExtensionEditor() {
	if m.extRunner != nil {
		m.extRunner.ResetEditor()
	}
}

// GetExtensionEditor returns the current editor interceptor, or nil if none
// is set. Returns nil if extensions are disabled.
func (m *Kit) GetExtensionEditor() *extensions.EditorConfig {
	if m.extRunner == nil {
		return nil
	}
	return m.extRunner.GetEditor()
}

// SetExtensionUIVisibility stores extension-provided UI visibility overrides.
// No-op if extensions are disabled.
func (m *Kit) SetExtensionUIVisibility(v extensions.UIVisibility) {
	if m.extRunner != nil {
		m.extRunner.SetUIVisibility(v)
	}
}

// GetExtensionUIVisibility returns extension-provided UI visibility overrides,
// or nil if none have been set. Returns nil if extensions are disabled.
func (m *Kit) GetExtensionUIVisibility() *extensions.UIVisibility {
	if m.extRunner == nil {
		return nil
	}
	return m.extRunner.GetUIVisibility()
}

// GetSessionMessages returns the conversation messages on the current branch
// as extension-facing SessionMessage structs, ordered root to leaf.
func (m *Kit) GetSessionMessages() []extensions.SessionMessage {
	if m.treeSession == nil {
		return nil
	}
	branch := m.treeSession.GetBranch("")
	var msgs []extensions.SessionMessage
	for _, entry := range branch {
		me, ok := entry.(*session.MessageEntry)
		if !ok {
			continue
		}
		msg, err := me.ToMessage()
		if err != nil {
			continue
		}
		// Flatten content parts into a single text string.
		var content strings.Builder
		for _, p := range msg.Parts {
			switch pt := p.(type) {
			case message.TextContent:
				content.WriteString(pt.Text)
			case message.ReasoningContent:
				content.WriteString(pt.Thinking)
			case message.ToolCall:
				fmt.Fprintf(&content, "[tool_call: %s(%s)]", pt.Name, pt.Input)
			case message.ToolResult:
				fmt.Fprintf(&content, "[tool_result: %s]", pt.Content)
			}
		}
		msgs = append(msgs, extensions.SessionMessage{
			ID:        me.ID,
			ParentID:  me.ParentID,
			Role:      string(msg.Role),
			Content:   content.String(),
			Model:     msg.Model,
			Provider:  msg.Provider,
			Timestamp: me.Timestamp.Format("2006-01-02T15:04:05Z07:00"),
		})
	}
	return msgs
}

// StructuredMessage represents a conversation message with typed content parts
// (tool calls, reasoning, finish markers, etc.) instead of flattened text.
type StructuredMessage struct {
	ID        string
	ParentID  string
	Role      MessageRole
	Parts     []ContentPart
	Model     string
	Provider  string
	Timestamp string // RFC3339 format
}

// GetStructuredMessages returns the conversation messages on the current
// branch with full typed content parts. Unlike GetSessionMessages() which
// flattens all content to a single text string, this preserves tool calls,
// tool results, reasoning blocks, and finish markers as distinct typed parts.
func (m *Kit) GetStructuredMessages() []StructuredMessage {
	if m.treeSession == nil {
		return nil
	}
	branch := m.treeSession.GetBranch("")
	var msgs []StructuredMessage
	for _, entry := range branch {
		me, ok := entry.(*session.MessageEntry)
		if !ok {
			continue
		}
		msg, err := me.ToMessage()
		if err != nil {
			continue
		}
		msgs = append(msgs, StructuredMessage{
			ID:        me.ID,
			ParentID:  me.ParentID,
			Role:      msg.Role,
			Parts:     msg.Parts,
			Model:     msg.Model,
			Provider:  msg.Provider,
			Timestamp: me.Timestamp.Format("2006-01-02T15:04:05Z07:00"),
		})
	}
	return msgs
}

// GetSessionFilePath returns the JSONL file path of the current session.
func (m *Kit) GetSessionFilePath() string {
	if m.treeSession == nil {
		return ""
	}
	return m.treeSession.GetFilePath()
}

// AppendExtensionEntry persists custom extension data in the session tree.
func (m *Kit) AppendExtensionEntry(extType, data string) (string, error) {
	if m.treeSession == nil {
		return "", fmt.Errorf("no session available")
	}
	return m.treeSession.AppendExtensionData(extType, data)
}

// GetExtensionEntries retrieves persisted extension data entries for a type.
func (m *Kit) GetExtensionEntries(extType string) []extensions.ExtensionEntry {
	if m.treeSession == nil {
		return nil
	}
	entries := m.treeSession.GetExtensionData(extType)
	result := make([]extensions.ExtensionEntry, 0, len(entries))
	for _, e := range entries {
		result = append(result, extensions.ExtensionEntry{
			ID:        e.ID,
			EntryType: e.ExtType,
			Data:      e.Data,
			Timestamp: e.Timestamp.Format("2006-01-02T15:04:05Z07:00"),
		})
	}
	return result
}

// SetExtensionStatus places or updates a keyed status bar entry.
func (m *Kit) SetExtensionStatus(entry extensions.StatusBarEntry) {
	if m.extRunner != nil {
		m.extRunner.SetStatusEntry(entry)
	}
}

// RemoveExtensionStatus removes a keyed status bar entry.
func (m *Kit) RemoveExtensionStatus(key string) {
	if m.extRunner != nil {
		m.extRunner.RemoveStatusEntry(key)
	}
}

// GetExtensionStatusEntries returns all extension status bar entries sorted by priority.
func (m *Kit) GetExtensionStatusEntries() []extensions.StatusBarEntry {
	if m.extRunner == nil {
		return nil
	}
	return m.extRunner.GetStatusEntries()
}

// GetExtensionShortcuts returns a map of key bindings to handler functions
// from all loaded extensions. Returns nil if no shortcuts are registered or
// extensions are disabled. Handlers are closures that capture the runner's
// current context, so they can call Print/SetStatus/etc.
func (m *Kit) GetExtensionShortcuts() map[string]func() {
	if m.extRunner == nil {
		return nil
	}
	entries := m.extRunner.GetShortcuts()
	if entries == nil {
		return nil
	}
	result := make(map[string]func(), len(entries))
	for key, entry := range entries {
		h := entry.Handler
		r := m.extRunner
		result[key] = func() {
			ctx := r.GetContext()
			h(ctx)
		}
	}
	return result
}

// GetExtensionToolInfos returns information about all tools available to the
// agent, including enabled/disabled status from SetActiveTools. Each tool is
// categorized by source: "core", "mcp", or "extension".
func (m *Kit) GetExtensionToolInfos() []extensions.ToolInfo {
	agentTools := m.agent.GetTools()
	coreCount := m.agent.GetCoreToolCount()
	mcpCount := m.agent.GetMCPToolCount()

	result := make([]extensions.ToolInfo, 0, len(agentTools))
	for i, t := range agentTools {
		info := t.Info()
		source := "core"
		if i >= coreCount && i < coreCount+mcpCount {
			source = "mcp"
		} else if i >= coreCount+mcpCount {
			source = "extension"
		}
		enabled := true
		if m.extRunner != nil && m.extRunner.IsToolDisabled(info.Name) {
			enabled = false
		}
		result = append(result, extensions.ToolInfo{
			Name:        info.Name,
			Description: info.Description,
			Source:      source,
			Enabled:     enabled,
		})
	}
	return result
}

// SetExtensionActiveTools restricts the tool set to the named tools. All
// other tools are blocked from execution. Pass nil to re-enable all tools.
// No-op if extensions are disabled.
func (m *Kit) SetExtensionActiveTools(names []string) {
	if m.extRunner != nil {
		m.extRunner.SetActiveTools(names)
	}
}

// SetModel changes the active model at runtime. The existing tools, system
// prompt, and session are preserved. The model string should be in
// "provider/model" format (e.g. "anthropic/claude-sonnet-4-5-20250929").
// Returns an error if the model string is invalid or the provider cannot
// be created.
func (m *Kit) SetModel(ctx context.Context, modelString string) error {
	// Validate the model string first.
	if _, _, err := ParseModelString(modelString); err != nil {
		return err
	}

	// Build a provider config from current settings, overriding the model.
	config := &models.ProviderConfig{
		ModelString:    modelString,
		ProviderAPIKey: viper.GetString("provider-api-key"),
		ProviderURL:    viper.GetString("provider-url"),
		MaxTokens:      viper.GetInt("max-tokens"),
		TLSSkipVerify:  viper.GetBool("tls-skip-verify"),
		ThinkingLevel:  models.ParseThinkingLevel(viper.GetString("thinking-level")),
	}
	temperature := float32(viper.GetFloat64("temperature"))
	config.Temperature = &temperature
	topP := float32(viper.GetFloat64("top-p"))
	config.TopP = &topP
	topK := int32(viper.GetInt("top-k"))
	config.TopK = &topK

	if err := m.agent.SetModel(ctx, config); err != nil {
		return err
	}

	m.modelString = modelString

	// Update extension context's Model field.
	if m.extRunner != nil {
		extCtx := m.extRunner.GetContext()
		extCtx.Model = modelString
		m.extRunner.SetContext(extCtx)
	}

	return nil
}

// GetAvailableModels returns a list of known models from the registry. Each
// entry includes provider, model ID, context limit, and whether the model
// supports reasoning. This is an advisory list — models not in the registry
// can still be used by specifying their provider/model string.
func (m *Kit) GetAvailableModels() []extensions.ModelInfoEntry {
	registry := models.GetGlobalRegistry()
	var result []extensions.ModelInfoEntry
	for _, providerID := range registry.GetFantasyProviders() {
		modelsMap, err := registry.GetModelsForProvider(providerID)
		if err != nil {
			continue
		}
		for modelID, info := range modelsMap {
			result = append(result, extensions.ModelInfoEntry{
				Provider:     providerID,
				ModelID:      modelID,
				Name:         info.Name,
				ContextLimit: info.Limit.Context,
				OutputLimit:  info.Limit.Output,
				Reasoning:    info.Reasoning,
			})
		}
	}
	return result
}

// GetExtensionOption resolves a named extension option value.
func (m *Kit) GetExtensionOption(name string) string {
	if m.extRunner == nil {
		return ""
	}
	return m.extRunner.GetOption(name)
}

// SetExtensionOption stores a runtime override for a named extension option.
func (m *Kit) SetExtensionOption(name, value string) {
	if m.extRunner != nil {
		m.extRunner.SetOption(name, value)
	}
}

// EmitModelChange fires the ModelChange event for extensions.
// No-op if extensions are disabled or no handlers are registered.
func (m *Kit) EmitModelChange(newModel, previousModel, source string) {
	if m.extRunner != nil && m.extRunner.HasHandlers(extensions.ModelChange) {
		_, _ = m.extRunner.Emit(extensions.ModelChangeEvent{
			NewModel:      newModel,
			PreviousModel: previousModel,
			Source:        source,
		})
	}
}

// EmitExtensionCustomEvent dispatches a named event to all extension handlers.
// No-op if extensions are disabled.
func (m *Kit) EmitExtensionCustomEvent(name, data string) {
	if m.extRunner != nil {
		m.extRunner.EmitCustomEvent(name, data)
	}
}

// GetExtensionMessageRenderer returns the named message renderer, or nil
// if no extension registered a renderer with that name.
func (m *Kit) GetExtensionMessageRenderer(name string) *extensions.MessageRendererConfig {
	if m.extRunner == nil {
		return nil
	}
	return m.extRunner.GetMessageRenderer(name)
}

// ReloadExtensions hot-reloads all extensions from disk. Event handlers,
// commands, renderers, and shortcuts update immediately. Extension-defined
// tools are NOT updated (they are baked into the agent at creation time).
func (m *Kit) ReloadExtensions() error {
	if m.extRunner == nil {
		return fmt.Errorf("no extensions loaded")
	}

	// Emit shutdown to old extensions.
	if m.extRunner.HasHandlers(extensions.SessionShutdown) {
		_, _ = m.extRunner.Emit(extensions.SessionShutdownEvent{})
	}

	// Re-load from disk.
	extraPaths := viper.GetStringSlice("extension")
	loaded, err := extensions.LoadExtensions(extraPaths)
	if err != nil {
		return fmt.Errorf("reloading extensions: %w", err)
	}

	// Swap extensions on the runner (clears dynamic state).
	m.extRunner.Reload(loaded)

	// Re-set context and emit SessionStart.
	ctx := m.extRunner.GetContext()
	m.extRunner.SetContext(ctx)
	if m.extRunner.HasHandlers(extensions.SessionStart) {
		_, _ = m.extRunner.Emit(extensions.SessionStartEvent{SessionID: ctx.SessionID})
	}

	return nil
}

// ExecuteCompletion makes a standalone LLM completion call for extensions.
// When req.Model is empty the current agent model is reused (no provider
// creation overhead). When req.Model is set a temporary provider is created,
// used, and closed.
func (m *Kit) ExecuteCompletion(ctx context.Context, req extensions.CompleteRequest) (extensions.CompleteResponse, error) {
	var (
		llmModel    fantasy.LanguageModel
		closer      func()
		usedModel   string
		providerOps fantasy.ProviderOptions
	)

	if req.Model == "" {
		// Reuse the active agent's model.
		llmModel = m.agent.GetModel()
		usedModel = m.modelString
		closer = func() {} // nothing to clean up
	} else {
		// Create a temporary provider for the requested model.
		config := &models.ProviderConfig{
			ModelString:   req.Model,
			TLSSkipVerify: viper.GetBool("tls-skip-verify"),
		}
		if req.MaxTokens > 0 {
			config.MaxTokens = req.MaxTokens
		}
		providerResult, err := models.CreateProvider(ctx, config)
		if err != nil {
			return extensions.CompleteResponse{}, fmt.Errorf("create provider for %q: %w", req.Model, err)
		}
		llmModel = providerResult.Model
		usedModel = req.Model
		providerOps = providerResult.ProviderOptions
		closer = func() {
			if providerResult.Closer != nil {
				_ = providerResult.Closer.Close()
			}
		}
	}
	defer closer()

	// Build fantasy agent options (no tools — just a simple completion).
	var agentOpts []fantasy.AgentOption
	if req.System != "" {
		agentOpts = append(agentOpts, fantasy.WithSystemPrompt(req.System))
	}
	if req.MaxTokens > 0 {
		agentOpts = append(agentOpts, fantasy.WithMaxOutputTokens(int64(req.MaxTokens)))
	}
	if providerOps != nil {
		agentOpts = append(agentOpts, fantasy.WithProviderOptions(providerOps))
	}

	completionAgent := fantasy.NewAgent(llmModel, agentOpts...)

	// Convert extension SessionMessage history to fantasy.Message slice.
	var messages []fantasy.Message
	for _, sm := range req.Messages {
		messages = append(messages, fantasy.Message{
			Role: fantasy.MessageRole(sm.Role),
			Content: []fantasy.MessagePart{
				fantasy.TextPart{Text: sm.Content},
			},
		})
	}

	// Streaming path.
	if req.OnChunk != nil {
		result, err := completionAgent.Stream(ctx, fantasy.AgentStreamCall{
			Prompt:   req.Prompt,
			Messages: messages,
			OnTextDelta: func(_, text string) error {
				req.OnChunk(text)
				return nil
			},
		})
		if err != nil {
			return extensions.CompleteResponse{}, fmt.Errorf("streaming completion: %w", err)
		}
		return extensions.CompleteResponse{
			Text:         result.Response.Content.Text(),
			InputTokens:  int(result.Response.Usage.InputTokens),
			OutputTokens: int(result.Response.Usage.OutputTokens),
			Model:        usedModel,
		}, nil
	}

	// Non-streaming path.
	result, err := completionAgent.Generate(ctx, fantasy.AgentCall{
		Prompt:   req.Prompt,
		Messages: messages,
	})
	if err != nil {
		return extensions.CompleteResponse{}, fmt.Errorf("completion: %w", err)
	}
	return extensions.CompleteResponse{
		Text:         result.Response.Content.Text(),
		InputTokens:  int(result.Response.Usage.InputTokens),
		OutputTokens: int(result.Response.Usage.OutputTokens),
		Model:        usedModel,
	}, nil
}

// EmitBeforeFork emits a BeforeFork event to extensions and returns
// whether the fork was cancelled and the reason. No-op if extensions are
// disabled (returns false, "").
func (m *Kit) EmitBeforeFork(targetID string, isUserMsg bool, userText string) (cancelled bool, reason string) {
	if m.extRunner == nil || !m.extRunner.HasHandlers(extensions.BeforeFork) {
		return false, ""
	}
	result, _ := m.extRunner.Emit(extensions.BeforeForkEvent{
		TargetID:      targetID,
		IsUserMessage: isUserMsg,
		UserText:      userText,
	})
	if r, ok := result.(extensions.BeforeForkResult); ok && r.Cancel {
		reason := r.Reason
		if reason == "" {
			reason = "Fork cancelled by extension."
		}
		return true, reason
	}
	return false, ""
}

// EmitBeforeSessionSwitch emits a BeforeSessionSwitch event to extensions
// and returns whether the switch was cancelled and the reason. No-op if
// extensions are disabled (returns false, "").
func (m *Kit) EmitBeforeSessionSwitch(switchReason string) (cancelled bool, reason string) {
	if m.extRunner == nil || !m.extRunner.HasHandlers(extensions.BeforeSessionSwitch) {
		return false, ""
	}
	result, _ := m.extRunner.Emit(extensions.BeforeSessionSwitchEvent{
		Reason: switchReason,
	})
	if r, ok := result.(extensions.BeforeSessionSwitchResult); ok && r.Cancel {
		reason := r.Reason
		if reason == "" {
			reason = "Session switch cancelled by extension."
		}
		return true, reason
	}
	return false, ""
}

// HasExtensions returns true if the extension runner is configured and active.
func (m *Kit) HasExtensions() bool {
	return m.extRunner != nil
}

// Options configures Kit creation with optional overrides for model,
// prompts, configuration, and behavior settings. All fields are optional
// and will use CLI defaults if not specified.
type Options struct {
	Model        string // Override model (e.g., "anthropic/claude-sonnet-4-5-20250929")
	SystemPrompt string // Override system prompt
	ConfigFile   string // Override config file path
	MaxSteps     int    // Override max steps (0 = use default)
	Streaming    bool   // Enable streaming (default from config)
	Quiet        bool   // Suppress debug output
	Tools        []Tool // Custom tool set. If empty, AllTools() is used.
	ExtraTools   []Tool // Additional tools added alongside core/MCP/extension tools.

	// Session configuration
	SessionDir  string // Base directory for session discovery (default: cwd)
	SessionPath string // Open a specific session file by path
	Continue    bool   // Continue the most recent session for SessionDir
	NoSession   bool   // Ephemeral mode — in-memory session, no persistence

	// Skills
	Skills    []string // Explicit skill files/dirs to load (empty = auto-discover)
	SkillsDir string   // Override default project-local skills directory

	// Compaction
	AutoCompact       bool               // Auto-compact when near context limit
	CompactionOptions *CompactionOptions // Config for auto-compaction (nil = defaults)

	// Debug enables debug logging for the SDK.
	Debug bool

	// CLI is optional CLI-specific configuration. SDK users leave this nil.
	CLI *CLIOptions
}

// CLIOptions holds fields only relevant to the CLI binary. SDK users should
// not need these; they are separated to keep the main Options struct clean.
type CLIOptions struct {
	// MCPConfig is a pre-loaded MCP config. When set, LoadAndValidateConfig
	// is skipped during Kit creation.
	MCPConfig *config.Config
	// ShowSpinner shows a loading spinner for Ollama models.
	ShowSpinner bool
	// SpinnerFunc provides the spinner implementation (nil = no spinner).
	SpinnerFunc SpinnerFunc
	// UseBufferedLogger buffers debug messages for later display.
	UseBufferedLogger bool
}

// InitTreeSession creates or opens a tree session based on the given options.
// Both kit.New() and the CLI use this function so session initialisation
// logic lives in one place.
//
// Behaviour based on Options:
//   - NoSession:   in-memory tree session (no persistence)
//   - Continue:    resume most recent session for SessionDir (or cwd)
//   - SessionPath: open a specific JSONL session file
//   - default:     create a new tree session for SessionDir (or cwd)
func InitTreeSession(opts *Options) (*session.TreeManager, error) {
	if opts == nil {
		opts = &Options{}
	}

	sessionDir := opts.SessionDir
	if sessionDir == "" {
		sessionDir, _ = os.Getwd()
	}

	if opts.NoSession {
		return session.InMemoryTreeSession(sessionDir), nil
	}

	if opts.Continue {
		return session.ContinueRecent(sessionDir)
	}

	if opts.SessionPath != "" {
		return session.OpenTreeSession(opts.SessionPath)
	}

	// Default: create a new tree session for the working directory.
	return session.CreateTreeSession(sessionDir)
}

// New creates a Kit instance using the same initialization as the CLI.
// It loads configuration, initializes MCP servers, creates the LLM model, and
// sets up the agent for interaction. Returns an error if initialization fails.
// viperInitMu serializes viper writes during kit.New(). Viper's global state
// is not thread-safe, so concurrent calls (e.g. parallel subagent spawns)
// must not overlap the Set()/Get() window.
var viperInitMu sync.Mutex

func New(ctx context.Context, opts *Options) (*Kit, error) {
	if opts == nil {
		opts = &Options{}
	}

	viperInitMu.Lock()
	defer viperInitMu.Unlock()

	// Set CLI-equivalent defaults for viper. When used as an SDK (without
	// cobra), these defaults are not registered via flag bindings.
	setSDKDefaults()

	// Initialize config (loads config files and env vars).
	if err := InitConfig(opts.ConfigFile, false); err != nil {
		return nil, fmt.Errorf("failed to initialize config: %w", err)
	}

	// Handle CLI debug mode.
	if opts.Debug {
		viper.Set("debug", true)
	}

	// Override viper settings with options.
	if opts.Model != "" {
		viper.Set("model", opts.Model)
	}
	if opts.SystemPrompt != "" {
		viper.Set("system-prompt", opts.SystemPrompt)
	}
	if opts.MaxSteps > 0 {
		viper.Set("max-steps", opts.MaxSteps)
	}
	viper.Set("stream", opts.Streaming)

	// Resolve working directory for context/skill discovery.
	cwd := opts.SessionDir
	if cwd == "" {
		cwd, _ = os.Getwd()
	}

	// Load context files (AGENTS.md) from the project root.
	contextFiles := loadContextFiles(cwd)

	// Load skills — either from explicit paths or via auto-discovery.
	loadedSkills, err := loadSkills(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to load skills: %w", err)
	}

	// Always compose the system prompt with runtime context: base prompt +
	// AGENTS.md context + skills metadata + date/cwd.
	{
		basePrompt := viper.GetString("system-prompt")
		pb := skills.NewPromptBuilder(basePrompt)

		// Inject AGENTS.md content as project context.
		for _, cf := range contextFiles {
			pb.WithSection("", fmt.Sprintf("Instructions from: %s\n\n%s", cf.Path, cf.Content))
		}

		// Inject skills metadata (name + description + location).
		if len(loadedSkills) > 0 {
			pb.WithSkills(loadedSkills)
		}

		// Append current date/time and working directory.
		pb.WithSection("", fmt.Sprintf(
			"Current date and time: %s\nCurrent working directory: %s",
			time.Now().Format("Monday, January 2, 2006, 3:04:05 PM MST"), cwd,
		))

		viper.Set("system-prompt", pb.Build())
	}

	// Load MCP configuration. Use pre-loaded config if provided via CLI options.
	var mcpConfig *config.Config
	if opts.CLI != nil {
		mcpConfig = opts.CLI.MCPConfig
	}
	if mcpConfig == nil {
		mcpConfig, err = config.LoadAndValidateConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to load MCP config: %w", err)
		}
	}

	// Pre-create hook registries so the tool wrapper can reference them.
	// Hooks registered after New() returns are still invoked because the
	// wrapper captures the registries by pointer.
	beforeToolCall := newHookRegistry[BeforeToolCallHook, BeforeToolCallResult]()
	afterToolResult := newHookRegistry[AfterToolResultHook, AfterToolResultResult]()
	beforeTurn := newHookRegistry[BeforeTurnHook, BeforeTurnResult]()
	afterTurn := newHookRegistry[AfterTurnHook, AfterTurnResult]()
	contextPrepare := newHookRegistry[ContextPrepareHook, ContextPrepareResult]()
	beforeCompact := newHookRegistry[BeforeCompactHook, BeforeCompactResult]()

	// Build agent setup options, pulling CLI-specific fields when available.
	setupOpts := kitsetup.AgentSetupOptions{
		MCPConfig:   mcpConfig,
		Quiet:       opts.Quiet,
		CoreTools:   opts.Tools,
		ExtraTools:  opts.ExtraTools,
		ToolWrapper: hookToolWrapper(beforeToolCall, afterToolResult),
	}
	if opts.CLI != nil {
		setupOpts.ShowSpinner = opts.CLI.ShowSpinner
		setupOpts.SpinnerFunc = opts.CLI.SpinnerFunc
		setupOpts.UseBufferedLogger = opts.CLI.UseBufferedLogger
	}

	// Create agent using shared setup with the hook tool wrapper.
	agentResult, err := kitsetup.SetupAgent(ctx, setupOpts)
	if err != nil {
		return nil, err
	}

	// Initialize tree session.
	treeSession, err := InitTreeSession(opts)
	if err != nil {
		_ = agentResult.Agent.Close()
		return nil, fmt.Errorf("failed to initialize session: %w", err)
	}

	k := &Kit{
		agent:           agentResult.Agent,
		treeSession:     treeSession,
		modelString:     viper.GetString("model"),
		events:          newEventBus(),
		autoCompact:     opts.AutoCompact,
		compactionOpts:  opts.CompactionOptions,
		contextFiles:    contextFiles,
		skills:          loadedSkills,
		extRunner:       agentResult.ExtRunner,
		bufferedLogger:  agentResult.BufferedLogger,
		beforeToolCall:  beforeToolCall,
		afterToolResult: afterToolResult,
		beforeTurn:      beforeTurn,
		afterTurn:       afterTurn,
		contextPrepare:  contextPrepare,
		beforeCompact:   beforeCompact,
	}

	// Bridge extension events to SDK hooks.
	if agentResult.ExtRunner != nil {
		k.bridgeExtensions(agentResult.ExtRunner)
	}

	return k, nil
}

// GetContextFiles returns the context files (e.g. AGENTS.md) loaded during
// initialisation. Returns nil if no context files were found.
func (m *Kit) GetContextFiles() []*ContextFile {
	return m.contextFiles
}

// GetSkills returns the skills loaded during initialisation.
func (m *Kit) GetSkills() []*Skill {
	return m.skills
}

// ---------------------------------------------------------------------------
// Context file loading
// ---------------------------------------------------------------------------

// loadContextFiles discovers and loads project context files (AGENTS.md) from
// the working directory. Returns nil if no context file is found.
func loadContextFiles(cwd string) []*ContextFile {
	path := filepath.Join(cwd, "AGENTS.md")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	return []*ContextFile{{
		Path:    path,
		Content: strings.TrimSpace(string(data)),
	}}
}

// ---------------------------------------------------------------------------
// Skill command expansion
// ---------------------------------------------------------------------------

// expandSkillCommand checks whether prompt starts with "/skill:<name>" and, if
// so, re-reads the skill file, strips its YAML frontmatter, wraps the body in
// a <skill> block with baseDir metadata, and appends any trailing user args.
// Returns the original text unchanged when the prefix is absent or the skill is
// not found.
func (m *Kit) expandSkillCommand(prompt string) string {
	if !strings.HasPrefix(prompt, "/skill:") {
		return prompt
	}

	// Parse: /skill:name [args]
	rest := prompt[len("/skill:"):]
	name, args, _ := strings.Cut(rest, " ")
	name = strings.TrimSpace(name)
	if name == "" {
		return prompt
	}

	// Find the skill by name.
	var skillPath string
	for _, s := range m.skills {
		if s.Name == name {
			skillPath = s.Path
			break
		}
	}
	if skillPath == "" {
		return prompt
	}

	// Re-read the file for freshness (user may have edited it since startup).
	loaded, err := skills.LoadSkill(skillPath)
	if err != nil {
		return prompt
	}

	baseDir := filepath.Dir(loaded.Path)
	var buf strings.Builder
	fmt.Fprintf(&buf, "<skill name=%q location=%q>\n", loaded.Name, loaded.Path)
	fmt.Fprintf(&buf, "References are relative to %s.\n\n", baseDir)
	buf.WriteString(loaded.Content)
	buf.WriteString("\n</skill>")

	args = strings.TrimSpace(args)
	if args != "" {
		buf.WriteString("\n\n")
		buf.WriteString(args)
	}

	return buf.String()
}

// ---------------------------------------------------------------------------
// Skills loading
// ---------------------------------------------------------------------------

// loadSkills loads skills based on Options. If explicit paths are provided
// they are loaded directly; otherwise auto-discovery runs.
func loadSkills(opts *Options) ([]*skills.Skill, error) {
	if len(opts.Skills) > 0 {
		return loadExplicitSkills(opts.Skills)
	}

	// Auto-discover from standard directories.
	cwd := opts.SkillsDir
	if cwd == "" {
		cwd = opts.SessionDir
	}
	return skills.LoadSkills(cwd)
}

// loadExplicitSkills loads skills from a list of explicit paths. Each path
// can be a file or a directory.
func loadExplicitSkills(paths []string) ([]*skills.Skill, error) {
	seen := make(map[string]bool)
	var all []*skills.Skill

	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			return nil, fmt.Errorf("skill path %s: %w", p, err)
		}

		if info.IsDir() {
			dirSkills, err := skills.LoadSkillsFromDir(p)
			if err != nil {
				return nil, err
			}
			for _, s := range dirSkills {
				if !seen[s.Path] {
					seen[s.Path] = true
					all = append(all, s)
				}
			}
		} else {
			abs, _ := filepath.Abs(p)
			if !seen[abs] {
				seen[abs] = true
				s, err := skills.LoadSkill(p)
				if err != nil {
					return nil, err
				}
				all = append(all, s)
			}
		}
	}

	return all, nil
}

// ---------------------------------------------------------------------------
// TurnResult
// ---------------------------------------------------------------------------

// TurnResult contains the full result of a prompt turn, including usage
// statistics and the updated conversation. Use PromptResult() instead of
// Prompt() when you need access to this data.
type TurnResult struct {
	// Response is the assistant's final text response.
	Response string

	// StopReason indicates why the turn ended. Derived from the LLM
	// provider's finish reason: "stop", "length" (max tokens), "tool-calls",
	// "content-filter", "error", "other", "unknown".
	StopReason string

	// SessionID is the UUID of the session this turn belongs to.
	SessionID string

	// TotalUsage is the aggregate token usage across all steps in the turn
	// (includes tool-calling loop iterations). Nil if the provider didn't
	// report usage.
	TotalUsage *FantasyUsage

	// FinalUsage is the token usage from the last API call only. Use this
	// for context window fill estimation (InputTokens + OutputTokens ≈
	// current context size). Nil if unavailable.
	FinalUsage *FantasyUsage

	// Messages is the full updated conversation after the turn, including
	// any tool call/result messages added during the agent loop.
	Messages []FantasyMessage
}

// ---------------------------------------------------------------------------
// In-process subagent
// ---------------------------------------------------------------------------

// SubagentConfig configures an in-process subagent spawned via Kit.Subagent().
type SubagentConfig struct {
	// Prompt is the task/instruction for the subagent (required).
	Prompt string

	// Model overrides the parent's model (e.g. "anthropic/claude-haiku-3-5-20241022").
	// Empty string uses the parent's current model.
	Model string

	// SystemPrompt provides domain-specific instructions for the subagent.
	// Empty string uses a minimal default prompt.
	SystemPrompt string

	// Tools overrides the tool set. If nil, SubagentTools() is used (all
	// core tools except spawn_subagent, preventing infinite recursion).
	Tools []Tool

	// NoSession, when true, uses an in-memory ephemeral session. When false
	// (default), the subagent's session is persisted and can be loaded for
	// replay/inspection.
	NoSession bool

	// Timeout limits execution time. Zero means 5 minute default.
	Timeout time.Duration

	// OnEvent, when set, receives all events from the subagent's event bus.
	// This enables the parent to stream subagent tool calls, text chunks,
	// etc. in real time.
	OnEvent func(Event)
}

// SubagentResult contains the outcome of an in-process subagent execution.
type SubagentResult struct {
	// Response is the subagent's final text response.
	Response string
	// Error is set if the subagent failed (nil on success).
	Error error
	// SessionID is the subagent's session identifier (for replay).
	SessionID string
	// StopReason is the LLM's finish reason for the subagent's final turn.
	StopReason string
	// Usage contains token usage from the subagent's run.
	Usage *FantasyUsage
	// Elapsed is the total execution time.
	Elapsed time.Duration
}

// Subagent spawns an in-process child Kit instance to perform a task. The
// child gets its own session, event bus, and agent loop but shares the
// parent's config (API keys, provider settings) and defaults to the parent's
// model when SubagentConfig.Model is empty.
//
// This is the recommended way to run subagents in the SDK — no subprocess,
// no kit binary dependency, native Go types for results.
func (m *Kit) Subagent(ctx context.Context, cfg SubagentConfig) (*SubagentResult, error) {
	if cfg.Prompt == "" {
		return nil, fmt.Errorf("subagent prompt is required")
	}

	start := time.Now()

	// Default timeout.
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 5 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Fall back to parent's model.
	model := cfg.Model
	if model == "" {
		model = m.modelString
	}

	// Default system prompt.
	systemPrompt := cfg.SystemPrompt
	if systemPrompt == "" {
		systemPrompt = "You are a helpful coding assistant. Complete the task efficiently and thoroughly."
	}

	// Default tools: everything except spawn_subagent.
	tools := cfg.Tools
	if tools == nil {
		tools = SubagentTools()
	}

	// Create child Kit instance.
	child, err := New(ctx, &Options{
		Model:        model,
		SystemPrompt: systemPrompt,
		Tools:        tools,
		NoSession:    cfg.NoSession,
		Quiet:        true,
	})
	if err != nil {
		return &SubagentResult{
			Error:   fmt.Errorf("failed to create subagent: %w", err),
			Elapsed: time.Since(start),
		}, err
	}
	defer func() { _ = child.Close() }()

	// Forward events to parent if requested.
	if cfg.OnEvent != nil {
		child.Subscribe(cfg.OnEvent)
	}

	// Run the prompt.
	result, err := child.PromptResult(ctx, cfg.Prompt)
	elapsed := time.Since(start)

	if err != nil {
		return &SubagentResult{
			Error:     err,
			SessionID: child.GetSessionID(),
			Elapsed:   elapsed,
		}, err
	}

	subResult := &SubagentResult{
		Response:   result.Response,
		SessionID:  child.GetSessionID(),
		StopReason: result.StopReason,
		Elapsed:    elapsed,
	}
	if result.TotalUsage != nil {
		subResult.Usage = result.TotalUsage
	}

	return subResult, nil
}

// ---------------------------------------------------------------------------
// Shared generation helpers
// ---------------------------------------------------------------------------

// generate calls the agent's generation loop with event-emitting handlers.
// All prompt modes (Prompt, Steer, FollowUp, PromptWithOptions) share this
// single code path so callback wiring is never duplicated.
func (m *Kit) generate(ctx context.Context, messages []fantasy.Message) (*agent.GenerateWithLoopResult, error) {
	// Inject the in-process subagent spawner into the context so the
	// spawn_subagent core tool can create child Kit instances without
	// importing pkg/kit (which would create an import cycle).
	ctx = core.WithSubagentSpawner(ctx, func(
		spawnCtx context.Context, prompt, model, systemPrompt string, timeout time.Duration,
	) (*core.SubagentSpawnResult, error) {
		result, err := m.Subagent(spawnCtx, SubagentConfig{
			Prompt:       prompt,
			Model:        model,
			SystemPrompt: systemPrompt,
			Timeout:      timeout,
		})
		if result == nil {
			return &core.SubagentSpawnResult{Error: err}, err
		}
		sr := &core.SubagentSpawnResult{
			Response:  result.Response,
			Error:     result.Error,
			SessionID: result.SessionID,
			Elapsed:   result.Elapsed,
		}
		if result.Usage != nil {
			sr.InputTokens = result.Usage.InputTokens
			sr.OutputTokens = result.Usage.OutputTokens
		}
		return sr, err
	})

	return m.agent.GenerateWithLoopAndStreaming(ctx, messages,
		func(toolCallID, toolName, toolArgs string) {
			m.events.emit(ToolCallEvent{
				ToolCallID: toolCallID, ToolName: toolName, ToolKind: toolKindFor(toolName),
				ToolArgs: toolArgs, ParsedArgs: parseToolArgs(toolArgs),
			})
		},
		func(toolCallID, toolName, toolArgs string, isStarting bool) {
			if isStarting {
				m.events.emit(ToolExecutionStartEvent{ToolCallID: toolCallID, ToolName: toolName, ToolKind: toolKindFor(toolName), ToolArgs: toolArgs})
			} else {
				m.events.emit(ToolExecutionEndEvent{ToolCallID: toolCallID, ToolName: toolName, ToolKind: toolKindFor(toolName)})
			}
		},
		func(toolCallID, toolName, toolArgs, resultText, metadata string, isError bool) {
			evt := ToolResultEvent{
				ToolCallID: toolCallID, ToolName: toolName, ToolKind: toolKindFor(toolName),
				ToolArgs: toolArgs, ParsedArgs: parseToolArgs(toolArgs),
				Result: resultText, IsError: isError,
			}
			if metadata != "" {
				var meta ToolResultMetadata
				if err := json.Unmarshal([]byte(metadata), &meta); err == nil {
					evt.Metadata = &meta
				}
			}
			m.events.emit(evt)
		},
		func(content string) {
			m.events.emit(ResponseEvent{Content: content})
		},
		func(content string) {
			m.events.emit(ToolCallContentEvent{Content: content})
		},
		func(chunk string) {
			m.events.emit(MessageUpdateEvent{Chunk: chunk})
		},
		func(delta string) {
			m.events.emit(ReasoningDeltaEvent{Delta: delta})
		},
	)
}

// runTurn is the shared lifecycle for every prompt mode:
//  1. Run BeforeTurn hooks (can modify prompt, inject messages).
//  2. Persist pre-generation messages to the tree session.
//  3. Build context from the tree (walks leaf-to-root for current branch).
//  4. Emit turn/message start events.
//  5. Run generation.
//  6. Emit turn/message end events.
//  7. Persist post-generation messages (tool calls, results, assistant).
//  8. Run AfterTurn hooks.
//
// promptLabel is the human-readable label emitted in TurnStartEvent.Prompt.
// prompt is the raw user text passed to BeforeTurn hooks.
func (m *Kit) runTurn(ctx context.Context, promptLabel string, prompt string, preMessages []fantasy.Message) (*TurnResult, error) {
	// Expand /skill:name commands — reads the skill file, wraps it in a
	// <skill> block, and appends any trailing user args.
	if expanded := m.expandSkillCommand(prompt); expanded != prompt {
		prompt = expanded
		// Replace the last user message in preMessages with the expanded text,
		// preserving any file parts (e.g. clipboard images).
		for i := len(preMessages) - 1; i >= 0; i-- {
			if preMessages[i].Role == fantasy.MessageRoleUser {
				files := extractFileParts(preMessages[i])
				preMessages[i] = fantasy.NewUserMessage(expanded, files...)
				break
			}
		}
	}

	// Run BeforeTurn hooks — can modify the prompt, inject system/context messages.
	if m.beforeTurn.hasHooks() {
		if hookResult := m.beforeTurn.run(BeforeTurnHook{Prompt: prompt}); hookResult != nil {
			// Override prompt text in the last user message, preserving
			// any file parts (e.g. clipboard images).
			if hookResult.Prompt != nil {
				for i := len(preMessages) - 1; i >= 0; i-- {
					if preMessages[i].Role == fantasy.MessageRoleUser {
						files := extractFileParts(preMessages[i])
						preMessages[i] = fantasy.NewUserMessage(*hookResult.Prompt, files...)
						break
					}
				}
			}
			// Inject messages before the original preMessages.
			var injected []fantasy.Message
			if hookResult.SystemPrompt != nil {
				injected = append(injected, fantasy.NewSystemMessage(*hookResult.SystemPrompt))
			}
			if hookResult.InjectText != nil {
				injected = append(injected, fantasy.NewUserMessage(*hookResult.InjectText))
			}
			if len(injected) > 0 {
				preMessages = append(injected, preMessages...)
			}
		}
	}

	// Persist pre-generation messages to tree session.
	for _, msg := range preMessages {
		_, _ = m.treeSession.AppendFantasyMessage(msg)
	}

	// Auto-compact if enabled and conversation is near the context limit.
	if m.autoCompact && m.ShouldCompact() {
		_, _ = m.compactInternal(ctx, m.compactionOpts, "", true) // best-effort, automatic
	}

	// Build context from the tree so only the current branch is sent.
	messages := m.treeSession.GetFantasyMessages()

	// Run ContextPrepare hooks — extensions can filter, reorder, or inject messages.
	if m.contextPrepare.hasHooks() {
		if hookResult := m.contextPrepare.run(ContextPrepareHook{Messages: messages}); hookResult != nil && hookResult.Messages != nil {
			messages = hookResult.Messages
		}
	}

	sentCount := len(messages)

	m.events.emit(TurnStartEvent{Prompt: promptLabel})
	m.events.emit(MessageStartEvent{})

	result, err := m.generate(ctx, messages)
	if err != nil {
		m.events.emit(TurnEndEvent{Error: err})
		// Run AfterTurn hooks even on error.
		if m.afterTurn.hasHooks() {
			m.afterTurn.run(AfterTurnHook{Error: err})
		}
		return nil, err
	}

	responseText := result.FinalResponse.Content.Text()

	// Persist new messages (tool calls, tool results, assistant response)
	// BEFORE emitting events so that extension handlers calling
	// GetContextStats() see up-to-date token counts.
	if len(result.ConversationMessages) > sentCount {
		for _, msg := range result.ConversationMessages[sentCount:] {
			_, _ = m.treeSession.AppendFantasyMessage(msg)
		}
	}

	// Store the API-reported token count so GetContextStats() matches the
	// built-in status bar (which uses input + output tokens). The
	// text-based heuristic misses system prompts, tool definitions, etc.
	if result.FinalResponse != nil {
		u := result.FinalResponse.Usage
		m.lastInputTokensMu.Lock()
		m.lastInputTokens = int(u.InputTokens) + int(u.OutputTokens)
		m.lastInputTokensMu.Unlock()
	}

	stopReason := result.StopReason

	m.events.emit(MessageEndEvent{Content: responseText})
	m.events.emit(TurnEndEvent{Response: responseText, StopReason: stopReason})

	// Run AfterTurn hooks.
	if m.afterTurn.hasHooks() {
		m.afterTurn.run(AfterTurnHook{Response: responseText})
	}

	// Build TurnResult with usage stats.
	turnResult := &TurnResult{
		Response:   responseText,
		StopReason: stopReason,
		SessionID:  m.GetSessionID(),
		Messages:   result.ConversationMessages,
	}
	totalUsage := result.TotalUsage
	turnResult.TotalUsage = &totalUsage
	if result.FinalResponse != nil {
		finalUsage := result.FinalResponse.Usage
		turnResult.FinalUsage = &finalUsage
	}

	return turnResult, nil
}

// ---------------------------------------------------------------------------
// Prompt modes
// ---------------------------------------------------------------------------

// Prompt sends a message to the agent and returns the response. The agent may
// use tools as needed to generate the response. The conversation history is
// automatically maintained in the tree session. Lifecycle events are emitted
// to all registered subscribers. Returns an error if generation fails.
func (m *Kit) Prompt(ctx context.Context, message string) (string, error) {
	result, err := m.runTurn(ctx, message, message, []fantasy.Message{
		fantasy.NewUserMessage(message),
	})
	if err != nil {
		return "", err
	}
	return result.Response, nil
}

// Steer injects a system-level instruction and triggers a new agent turn.
// Use Steer to dynamically adjust agent behavior mid-conversation without a
// visible user message — for example, changing tone, focus, or constraints.
//
// Under the hood, Steer appends a system message (the instruction) followed by
// a synthetic user message so the agent acknowledges and follows the directive.
// Both messages are persisted to the session.
func (m *Kit) Steer(ctx context.Context, instruction string) (string, error) {
	result, err := m.runTurn(ctx, "[steer] "+instruction, instruction, []fantasy.Message{
		fantasy.NewSystemMessage(instruction),
		fantasy.NewUserMessage("Please acknowledge and follow the above instruction."),
	})
	if err != nil {
		return "", err
	}
	return result.Response, nil
}

// FollowUp continues the conversation without explicit new user input.
// If text is empty, "Continue." is used as the prompt. Use FollowUp when the
// agent's previous response was truncated or you want the agent to elaborate.
//
// Returns an error if there are no previous messages in the session.
func (m *Kit) FollowUp(ctx context.Context, text string) (string, error) {
	// Verify there is conversation history to follow up on.
	if len(m.treeSession.GetFantasyMessages()) == 0 {
		return "", fmt.Errorf("cannot follow up: no previous messages")
	}

	if text == "" {
		text = "Continue."
	}

	result, err := m.runTurn(ctx, "[follow-up]", text, []fantasy.Message{
		fantasy.NewUserMessage(text),
	})
	if err != nil {
		return "", err
	}
	return result.Response, nil
}

// PromptOptions configures a single PromptWithOptions call.
type PromptOptions struct {
	// SystemMessage is prepended as a system message before the user prompt.
	// Use it to inject per-call instructions or context without permanently
	// modifying the agent's system prompt.
	SystemMessage string
}

// PromptWithOptions sends a message with per-call configuration. It behaves
// like Prompt but allows injecting an additional system message before the
// user prompt. Both messages are persisted to the session.
func (m *Kit) PromptWithOptions(ctx context.Context, msg string, opts PromptOptions) (string, error) {
	var preMessages []fantasy.Message
	if opts.SystemMessage != "" {
		preMessages = append(preMessages, fantasy.NewSystemMessage(opts.SystemMessage))
	}
	preMessages = append(preMessages, fantasy.NewUserMessage(msg))

	result, err := m.runTurn(ctx, msg, msg, preMessages)
	if err != nil {
		return "", err
	}
	return result.Response, nil
}

// PromptWithCallbacks sends a message with callbacks for monitoring tool
// execution and streaming responses. Lifecycle events are also emitted to all
// registered subscribers (via Subscribe).
//
// Deprecated: Use Subscribe/OnToolCall/OnToolResult/OnStreaming instead of
// inline callbacks. PromptWithCallbacks is retained for backward compatibility.
func (m *Kit) PromptWithCallbacks(
	ctx context.Context,
	message string,
	onToolCall func(name, args string),
	onToolResult func(name, args, result string, isError bool),
	onStreaming func(chunk string),
) (string, error) {
	// Register temporary subscribers for the inline callbacks.
	var unsubs []func()
	if onToolCall != nil {
		unsubs = append(unsubs, m.OnToolCall(func(e ToolCallEvent) {
			onToolCall(e.ToolName, e.ToolArgs)
		}))
	}
	if onToolResult != nil {
		unsubs = append(unsubs, m.OnToolResult(func(e ToolResultEvent) {
			onToolResult(e.ToolName, e.ToolArgs, e.Result, e.IsError)
		}))
	}
	if onStreaming != nil {
		unsubs = append(unsubs, m.OnStreaming(func(e MessageUpdateEvent) {
			onStreaming(e.Chunk)
		}))
	}
	defer func() {
		for _, unsub := range unsubs {
			unsub()
		}
	}()

	return m.Prompt(ctx, message)
}

// PromptResult sends a message and returns the full turn result including
// usage statistics and conversation messages. Use this instead of Prompt()
// when you need more than just the response text.
func (m *Kit) PromptResult(ctx context.Context, message string) (*TurnResult, error) {
	return m.runTurn(ctx, message, message, []fantasy.Message{
		fantasy.NewUserMessage(message),
	})
}

// PromptResultWithFiles sends a multimodal message (text + images) and returns
// the full turn result. The files parameter carries binary file data (e.g.
// clipboard images) that are included alongside the text in the user message.
func (m *Kit) PromptResultWithFiles(ctx context.Context, message string, files []fantasy.FilePart) (*TurnResult, error) {
	return m.runTurn(ctx, message, message, []fantasy.Message{
		fantasy.NewUserMessage(message, files...),
	})
}

// ClearSession resets the tree session's leaf pointer to the root, starting
// a fresh conversation branch.
func (m *Kit) ClearSession() {
	m.treeSession.ResetLeaf()
}

// GetModelString returns the current model string identifier (e.g.,
// "anthropic/claude-sonnet-4-5-20250929" or "openai/gpt-4") being used by the agent.
func (m *Kit) GetModelString() string {
	return m.modelString
}

// GetModelInfo returns detailed information about the current model
// (capabilities, pricing, limits). Returns nil if the model is not in the
// registry — this is expected for new models or custom fine-tunes.
func (m *Kit) GetModelInfo() *ModelInfo {
	provider, modelID, err := ParseModelString(m.modelString)
	if err != nil {
		return nil
	}
	return LookupModel(provider, modelID)
}

// IsReasoningModel returns true if the current model supports extended thinking / reasoning.
func (m *Kit) IsReasoningModel() bool {
	info := m.GetModelInfo()
	return info != nil && info.Reasoning
}

// GetThinkingLevel returns the current thinking level.
func (m *Kit) GetThinkingLevel() string {
	return viper.GetString("thinking-level")
}

// SetThinkingLevel changes the thinking level and recreates the agent with
// the new thinking budget. Returns an error if provider recreation fails.
func (m *Kit) SetThinkingLevel(ctx context.Context, level string) error {
	viper.Set("thinking-level", level)
	// Recreate agent with new thinking config by re-running SetModel
	// with the same model string. SetModel rebuilds the provider and
	// passes the updated viper config (including thinking-level).
	return m.SetModel(ctx, m.modelString)
}

// GetTools returns all tools available to the agent (core + MCP + extensions).
func (m *Kit) GetTools() []Tool {
	return m.agent.GetTools()
}

// extractFileParts returns all FilePart entries from a message's Content.
// Used to preserve image attachments when replacing user message text.
func extractFileParts(msg fantasy.Message) []fantasy.FilePart {
	var files []fantasy.FilePart
	for _, part := range msg.Content {
		if fp, ok := part.(fantasy.FilePart); ok {
			files = append(files, fp)
		}
	}
	return files
}

// Close cleans up resources including MCP server connections, model resources,
// and the tree session file handle. Should be called when the Kit instance is
// no longer needed. Returns an error if cleanup fails.
func (m *Kit) Close() error {
	// Emit SessionShutdown for extensions.
	if m.extRunner != nil && m.extRunner.HasHandlers(extensions.SessionShutdown) {
		_, _ = m.extRunner.Emit(extensions.SessionShutdownEvent{})
	}
	if m.treeSession != nil {
		_ = m.treeSession.Close()
	}
	return m.agent.Close()
}
