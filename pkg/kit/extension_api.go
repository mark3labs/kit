package kit

import (
	"fmt"
	"log"
	"strings"

	"github.com/mark3labs/kit/internal/extensions"
	"github.com/mark3labs/kit/internal/message"
	"github.com/mark3labs/kit/internal/session"
)

// ==== Extension Types ====
//
// Type aliases for internal extension types exposed through the public
// ExtensionAPI interface. External SDK consumers can use these without
// importing internal packages directly.

// ExtensionContext holds the runtime context passed to extensions, including
// callbacks for printing, sending messages, and accessing session state.
type ExtensionContext = extensions.Context

// ExtensionWidgetConfig describes a widget registered by an extension.
type ExtensionWidgetConfig = extensions.WidgetConfig

// ExtensionWidgetPlacement indicates where a widget should be rendered
// (e.g. above or below the conversation).
type ExtensionWidgetPlacement = extensions.WidgetPlacement

// ExtensionHeaderFooterConfig describes a header or footer registered by an extension.
type ExtensionHeaderFooterConfig = extensions.HeaderFooterConfig

// ExtensionEditorConfig configures editor behaviour overrides set by extensions.
type ExtensionEditorConfig = extensions.EditorConfig

// ExtensionUIVisibility controls which UI elements are visible.
type ExtensionUIVisibility = extensions.UIVisibility

// ExtensionToolRenderConfig describes custom tool output rendering registered by an extension.
type ExtensionToolRenderConfig = extensions.ToolRenderConfig

// ExtensionMessageRendererConfig describes custom message rendering registered by an extension.
type ExtensionMessageRendererConfig = extensions.MessageRendererConfig

// ExtensionSessionMessage represents a single message in the session history
// as exposed to extensions.
type ExtensionSessionMessage = extensions.SessionMessage

// ExtensionEntry represents a custom data entry stored by an extension
// in the session tree.
type ExtensionEntry = extensions.ExtensionEntry

// ExtensionStatusBarEntry describes a status bar entry registered by an extension.
type ExtensionStatusBarEntry = extensions.StatusBarEntry

// ExtensionToolInfo describes a tool available to the agent, as seen by extensions.
type ExtensionToolInfo = extensions.ToolInfo

// ExtensionCommandDef describes a slash command registered by an extension.
type ExtensionCommandDef = extensions.CommandDef

// ExtensionAPI provides grouped access to all extension-related functionality.
// This cleans up the main Kit API surface while keeping all extension capabilities available.
type ExtensionAPI interface {
	// Context management
	SetContext(ctx ExtensionContext)
	GetContext() ExtensionContext
	UpdateContextModel(model string)

	// Widgets
	SetWidget(config ExtensionWidgetConfig)
	RemoveWidget(id string)
	GetWidgets(placement ExtensionWidgetPlacement) []ExtensionWidgetConfig

	// Header/Footer
	SetHeader(config ExtensionHeaderFooterConfig)
	RemoveHeader()
	GetHeader() *ExtensionHeaderFooterConfig
	SetFooter(config ExtensionHeaderFooterConfig)
	RemoveFooter()
	GetFooter() *ExtensionHeaderFooterConfig

	// Editor
	SetEditor(config ExtensionEditorConfig)
	ResetEditor()
	GetEditor() *ExtensionEditorConfig

	// UI Visibility
	SetUIVisibility(v ExtensionUIVisibility)
	GetUIVisibility() *ExtensionUIVisibility

	// Tool rendering
	GetToolRenderer(toolName string) *ExtensionToolRenderConfig
	GetMessageRenderer(name string) *ExtensionMessageRendererConfig

	// Session data
	GetSessionMessages() []ExtensionSessionMessage
	AppendEntry(extType, data string) (string, error)
	GetEntries(extType string) []ExtensionEntry

	// Session-scoped extension state (last-write-wins key-value store).
	// Backed by an in-memory map and (optionally) a sidecar file per session;
	// state lives outside the conversation tree and is not visible to the LLM.
	SetState(key, value string)
	GetState(key string) (string, bool)
	DeleteState(key string)
	ListState() []string

	// InitStatePersistence loads any existing state from the per-session
	// sidecar file and installs a saver hook so that subsequent SetState /
	// DeleteState mutations are flushed to disk. Safe to call multiple times;
	// repeat calls simply reload and reinstall the saver.
	//
	// For ephemeral or in-memory sessions (no session file path), the call
	// is a no-op and state remains in memory for the lifetime of the runner.
	InitStatePersistence() error

	// Status bar
	SetStatus(entry ExtensionStatusBarEntry)
	RemoveStatus(key string)
	GetStatusEntries() []ExtensionStatusBarEntry

	// Shortcuts
	GetShortcuts() map[string]func()

	// Tools
	GetToolInfos() []ExtensionToolInfo
	SetActiveTools(names []string)

	// Options
	GetOption(name string) string
	SetOption(name, value string)

	// Events
	EmitSessionStart()
	EmitModelChange(newModel, previousModel, source string)
	EmitCustomEvent(name, data string)
	EmitBeforeFork(targetID string, isUserMsg bool, userText string) (cancelled bool, reason string)
	EmitBeforeSessionSwitch(switchReason string) (cancelled bool, reason string)

	// Commands
	Commands() []ExtensionCommandDef

	// Lifecycle
	Reload() error
	HasExtensions() bool

	// Loaded returns metadata about the extensions currently loaded.
	Loaded() []ExtensionInfo
}

// ExtensionInfo describes a single loaded extension for display purposes
// (e.g. the startup banner or `kit extensions list`).
type ExtensionInfo struct {
	// Path is the absolute path of the extension's .go file.
	Path string
	// ToolCount is the number of tools registered by the extension.
	ToolCount int
	// CommandCount is the number of slash commands registered.
	CommandCount int
	// HandlerCount is the total number of event handlers registered.
	HandlerCount int
}

// extensionAPI implements ExtensionAPI by wrapping a Kit instance.
type extensionAPI struct {
	kit *Kit
}

// Extensions returns the ExtensionAPI for accessing all extension-related functionality.
func (m *Kit) Extensions() ExtensionAPI {
	return &extensionAPI{kit: m}
}

// Context management

func (e *extensionAPI) SetContext(ctx ExtensionContext) {
	if e.kit.extRunner != nil {
		e.kit.extRunner.SetContext(ctx)
	}
}

func (e *extensionAPI) GetContext() ExtensionContext {
	if e.kit.extRunner != nil {
		return e.kit.extRunner.GetContext()
	}
	return ExtensionContext{}
}

func (e *extensionAPI) UpdateContextModel(model string) {
	if e.kit.extRunner != nil {
		ctx := e.kit.extRunner.GetContext()
		ctx.Model = model
		e.kit.extRunner.SetContext(ctx)
	}
}

// Widgets

func (e *extensionAPI) SetWidget(config ExtensionWidgetConfig) {
	if e.kit.extRunner != nil {
		e.kit.extRunner.SetWidget(config)
	}
}

func (e *extensionAPI) RemoveWidget(id string) {
	if e.kit.extRunner != nil {
		e.kit.extRunner.RemoveWidget(id)
	}
}

func (e *extensionAPI) GetWidgets(placement ExtensionWidgetPlacement) []ExtensionWidgetConfig {
	if e.kit.extRunner == nil {
		return nil
	}
	return e.kit.extRunner.GetWidgets(placement)
}

// Header/Footer

func (e *extensionAPI) SetHeader(config ExtensionHeaderFooterConfig) {
	if e.kit.extRunner != nil {
		e.kit.extRunner.SetHeader(config)
	}
}

func (e *extensionAPI) RemoveHeader() {
	if e.kit.extRunner != nil {
		e.kit.extRunner.RemoveHeader()
	}
}

func (e *extensionAPI) GetHeader() *ExtensionHeaderFooterConfig {
	if e.kit.extRunner == nil {
		return nil
	}
	return e.kit.extRunner.GetHeader()
}

func (e *extensionAPI) SetFooter(config ExtensionHeaderFooterConfig) {
	if e.kit.extRunner != nil {
		e.kit.extRunner.SetFooter(config)
	}
}

func (e *extensionAPI) RemoveFooter() {
	if e.kit.extRunner != nil {
		e.kit.extRunner.RemoveFooter()
	}
}

func (e *extensionAPI) GetFooter() *ExtensionHeaderFooterConfig {
	if e.kit.extRunner == nil {
		return nil
	}
	return e.kit.extRunner.GetFooter()
}

// Editor

func (e *extensionAPI) SetEditor(config ExtensionEditorConfig) {
	if e.kit.extRunner != nil {
		e.kit.extRunner.SetEditor(config)
	}
}

func (e *extensionAPI) ResetEditor() {
	if e.kit.extRunner != nil {
		e.kit.extRunner.ResetEditor()
	}
}

func (e *extensionAPI) GetEditor() *ExtensionEditorConfig {
	if e.kit.extRunner == nil {
		return nil
	}
	return e.kit.extRunner.GetEditor()
}

// UI Visibility

func (e *extensionAPI) SetUIVisibility(v ExtensionUIVisibility) {
	if e.kit.extRunner != nil {
		e.kit.extRunner.SetUIVisibility(v)
	}
}

func (e *extensionAPI) GetUIVisibility() *ExtensionUIVisibility {
	if e.kit.extRunner == nil {
		return nil
	}
	return e.kit.extRunner.GetUIVisibility()
}

// Tool rendering

func (e *extensionAPI) GetToolRenderer(toolName string) *ExtensionToolRenderConfig {
	if e.kit.extRunner == nil {
		return nil
	}
	return e.kit.extRunner.GetToolRenderer(toolName)
}

func (e *extensionAPI) GetMessageRenderer(name string) *ExtensionMessageRendererConfig {
	if e.kit.extRunner == nil {
		return nil
	}
	return e.kit.extRunner.GetMessageRenderer(name)
}

// Session data

func (e *extensionAPI) GetSessionMessages() []ExtensionSessionMessage {
	if e.kit.session == nil {
		return nil
	}

	// Try to use the legacy iterBranchMessages for backward compatibility
	// with the default TreeManager adapter
	if adapter, ok := e.kit.session.(*treeManagerAdapter); ok {
		return iterBranchMessages(adapter.inner, func(me *session.MessageEntry, msg message.Message) ExtensionSessionMessage {
			return ExtensionSessionMessage{
				ID:        me.ID,
				Role:      string(msg.Role),
				Content:   msg.Content(),
				Timestamp: me.Timestamp.Format("2006-01-02T15:04:05Z07:00"),
			}
		})
	}

	// For custom SessionManagers, use the public interface
	branch := e.kit.session.GetCurrentBranch()
	var result []ExtensionSessionMessage
	for _, entry := range branch {
		if entry.Type == EntryTypeMessage {
			result = append(result, ExtensionSessionMessage{
				ID:        entry.ID,
				Role:      entry.Role,
				Content:   entry.Content,
				Timestamp: entry.Timestamp.Format("2006-01-02T15:04:05Z07:00"),
			})
		}
	}
	return result
}

func (e *extensionAPI) AppendEntry(extType, data string) (string, error) {
	if e.kit.session == nil {
		return "", fmt.Errorf("no session available")
	}
	return e.kit.session.AppendExtensionData(extType, data)
}

func (e *extensionAPI) SetState(key, value string) {
	if e.kit.extRunner != nil {
		e.kit.extRunner.SetState(key, value)
	}
}

func (e *extensionAPI) GetState(key string) (string, bool) {
	if e.kit.extRunner == nil {
		return "", false
	}
	return e.kit.extRunner.GetState(key)
}

func (e *extensionAPI) DeleteState(key string) {
	if e.kit.extRunner != nil {
		e.kit.extRunner.DeleteState(key)
	}
}

func (e *extensionAPI) ListState() []string {
	if e.kit.extRunner == nil {
		return nil
	}
	return e.kit.extRunner.ListState()
}

func (e *extensionAPI) InitStatePersistence() error {
	if e.kit.extRunner == nil {
		return nil
	}
	path := extStateSidecarPath(e.kit.GetSessionPath())
	if path == "" {
		// Ephemeral or in-memory session; no on-disk state.
		e.kit.extRunner.SetStateSaver(nil)
		return nil
	}
	if err := e.kit.extRunner.LoadStateFromFile(path); err != nil {
		return err
	}
	runner := e.kit.extRunner
	runner.SetStateSaver(func() {
		if err := runner.SaveStateToFile(path); err != nil {
			log.Printf("WARN extension state save failed: path=%s err=%v", path, err)
		}
	})
	return nil
}

// extStateSidecarPath returns the path to the per-session extension state
// sidecar file derived from the session's JSONL path. Returns empty for
// ephemeral / in-memory sessions where no JSONL is being written.
func extStateSidecarPath(sessionPath string) string {
	if sessionPath == "" {
		return ""
	}
	if trimmed, ok := strings.CutSuffix(sessionPath, ".jsonl"); ok {
		return trimmed + ".ext-state.json"
	}
	return sessionPath + ".ext-state.json"
}

func (e *extensionAPI) GetEntries(extType string) []ExtensionEntry {
	if e.kit.session == nil {
		return nil
	}
	entries := e.kit.session.GetExtensionData(extType)
	result := make([]ExtensionEntry, 0, len(entries))
	for _, e := range entries {
		result = append(result, ExtensionEntry{
			ID:        e.ID,
			EntryType: e.ExtType,
			Data:      e.Data,
			Timestamp: e.Timestamp.Format("2006-01-02T15:04:05Z07:00"),
		})
	}
	return result
}

// Status bar

func (e *extensionAPI) SetStatus(entry ExtensionStatusBarEntry) {
	if e.kit.extRunner != nil {
		e.kit.extRunner.SetStatusEntry(entry)
	}
}

func (e *extensionAPI) RemoveStatus(key string) {
	if e.kit.extRunner != nil {
		e.kit.extRunner.RemoveStatusEntry(key)
	}
}

func (e *extensionAPI) GetStatusEntries() []ExtensionStatusBarEntry {
	if e.kit.extRunner == nil {
		return nil
	}
	return e.kit.extRunner.GetStatusEntries()
}

// Shortcuts

func (e *extensionAPI) GetShortcuts() map[string]func() {
	if e.kit.extRunner == nil {
		return nil
	}
	entries := e.kit.extRunner.GetShortcuts()
	if entries == nil {
		return nil
	}
	result := make(map[string]func(), len(entries))
	for key, entry := range entries {
		h := entry.Handler
		r := e.kit.extRunner
		result[key] = func() {
			ctx := r.GetContext()
			h(ctx)
		}
	}
	return result
}

// Tools

func (e *extensionAPI) GetToolInfos() []ExtensionToolInfo {
	agentTools := e.kit.agent.GetTools()
	coreCount := e.kit.agent.GetCoreToolCount()
	mcpCount := e.kit.agent.GetMCPToolCount()

	result := make([]ExtensionToolInfo, 0, len(agentTools))
	for i, t := range agentTools {
		info := t.Info()
		source := "core"
		if i >= coreCount && i < coreCount+mcpCount {
			source = "mcp"
		} else if i >= coreCount+mcpCount {
			source = "extension"
		}
		enabled := true
		if e.kit.extRunner != nil && e.kit.extRunner.IsToolDisabled(info.Name) {
			enabled = false
		}
		result = append(result, ExtensionToolInfo{
			Name:        info.Name,
			Description: info.Description,
			Source:      source,
			Enabled:     enabled,
		})
	}
	return result
}

func (e *extensionAPI) SetActiveTools(names []string) {
	if e.kit.extRunner != nil {
		e.kit.extRunner.SetActiveTools(names)
	}
}

// Options

func (e *extensionAPI) GetOption(name string) string {
	if e.kit.extRunner == nil {
		return ""
	}
	return e.kit.extRunner.GetOption(name)
}

func (e *extensionAPI) SetOption(name, value string) {
	if e.kit.extRunner != nil {
		e.kit.extRunner.SetOption(name, value)
	}
}

// Events

func (e *extensionAPI) EmitSessionStart() {
	if e.kit.extRunner != nil && e.kit.extRunner.HasHandlers(extensions.SessionStart) {
		_, _ = e.kit.extRunner.Emit(extensions.SessionStartEvent{})
	}
}

func (e *extensionAPI) EmitModelChange(newModel, previousModel, source string) {
	if e.kit.extRunner != nil && e.kit.extRunner.HasHandlers(extensions.ModelChange) {
		_, _ = e.kit.extRunner.Emit(extensions.ModelChangeEvent{
			NewModel:      newModel,
			PreviousModel: previousModel,
			Source:        source,
		})
	}
}

func (e *extensionAPI) EmitCustomEvent(name, data string) {
	if e.kit.extRunner != nil {
		e.kit.extRunner.EmitCustomEvent(name, data)
	}
}

func (e *extensionAPI) EmitBeforeFork(targetID string, isUserMsg bool, userText string) (cancelled bool, reason string) {
	if e.kit.extRunner == nil || !e.kit.extRunner.HasHandlers(extensions.BeforeFork) {
		return false, ""
	}
	result, _ := e.kit.extRunner.Emit(extensions.BeforeForkEvent{
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

func (e *extensionAPI) EmitBeforeSessionSwitch(switchReason string) (cancelled bool, reason string) {
	if e.kit.extRunner == nil || !e.kit.extRunner.HasHandlers(extensions.BeforeSessionSwitch) {
		return false, ""
	}
	result, _ := e.kit.extRunner.Emit(extensions.BeforeSessionSwitchEvent{
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

// Commands

func (e *extensionAPI) Commands() []ExtensionCommandDef {
	if e.kit.extRunner == nil {
		return nil
	}
	return e.kit.extRunner.RegisteredCommands()
}

// Lifecycle

func (e *extensionAPI) Reload() error {
	return e.kit.ReloadExtensions()
}

func (e *extensionAPI) HasExtensions() bool {
	return e.kit.extRunner != nil
}

func (e *extensionAPI) Loaded() []ExtensionInfo {
	if e.kit.extRunner == nil {
		return nil
	}
	exts := e.kit.extRunner.Extensions()
	if len(exts) == 0 {
		return nil
	}
	infos := make([]ExtensionInfo, 0, len(exts))
	for _, ex := range exts {
		handlerCount := 0
		for _, hs := range ex.Handlers {
			handlerCount += len(hs)
		}
		infos = append(infos, ExtensionInfo{
			Path:         ex.Path,
			ToolCount:    len(ex.Tools),
			CommandCount: len(ex.Commands),
			HandlerCount: handlerCount,
		})
	}
	return infos
}
