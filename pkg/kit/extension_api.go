package kit

import (
	"fmt"

	"github.com/mark3labs/kit/internal/extensions"
	"github.com/mark3labs/kit/internal/message"
	"github.com/mark3labs/kit/internal/session"
)

// ExtensionAPI provides grouped access to all extension-related functionality.
// This cleans up the main Kit API surface while keeping all extension capabilities available.
type ExtensionAPI interface {
	// Context management
	SetContext(ctx extensions.Context)
	GetContext() extensions.Context
	UpdateContextModel(model string)

	// Widgets
	SetWidget(config extensions.WidgetConfig)
	RemoveWidget(id string)
	GetWidgets(placement extensions.WidgetPlacement) []extensions.WidgetConfig

	// Header/Footer
	SetHeader(config extensions.HeaderFooterConfig)
	RemoveHeader()
	GetHeader() *extensions.HeaderFooterConfig
	SetFooter(config extensions.HeaderFooterConfig)
	RemoveFooter()
	GetFooter() *extensions.HeaderFooterConfig

	// Editor
	SetEditor(config extensions.EditorConfig)
	ResetEditor()
	GetEditor() *extensions.EditorConfig

	// UI Visibility
	SetUIVisibility(v extensions.UIVisibility)
	GetUIVisibility() *extensions.UIVisibility

	// Tool rendering
	GetToolRenderer(toolName string) *extensions.ToolRenderConfig
	GetMessageRenderer(name string) *extensions.MessageRendererConfig

	// Session data
	GetSessionMessages() []extensions.SessionMessage
	AppendEntry(extType, data string) (string, error)
	GetEntries(extType string) []extensions.ExtensionEntry

	// Status bar
	SetStatus(entry extensions.StatusBarEntry)
	RemoveStatus(key string)
	GetStatusEntries() []extensions.StatusBarEntry

	// Shortcuts
	GetShortcuts() map[string]func()

	// Tools
	GetToolInfos() []extensions.ToolInfo
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
	Commands() []extensions.CommandDef

	// Lifecycle
	Reload() error
	HasExtensions() bool
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

func (e *extensionAPI) SetContext(ctx extensions.Context) {
	if e.kit.extRunner != nil {
		e.kit.extRunner.SetContext(ctx)
	}
}

func (e *extensionAPI) GetContext() extensions.Context {
	if e.kit.extRunner != nil {
		return e.kit.extRunner.GetContext()
	}
	return extensions.Context{}
}

func (e *extensionAPI) UpdateContextModel(model string) {
	if e.kit.extRunner != nil {
		ctx := e.kit.extRunner.GetContext()
		ctx.Model = model
		e.kit.extRunner.SetContext(ctx)
	}
}

// Widgets

func (e *extensionAPI) SetWidget(config extensions.WidgetConfig) {
	if e.kit.extRunner != nil {
		e.kit.extRunner.SetWidget(config)
	}
}

func (e *extensionAPI) RemoveWidget(id string) {
	if e.kit.extRunner != nil {
		e.kit.extRunner.RemoveWidget(id)
	}
}

func (e *extensionAPI) GetWidgets(placement extensions.WidgetPlacement) []extensions.WidgetConfig {
	if e.kit.extRunner == nil {
		return nil
	}
	return e.kit.extRunner.GetWidgets(placement)
}

// Header/Footer

func (e *extensionAPI) SetHeader(config extensions.HeaderFooterConfig) {
	if e.kit.extRunner != nil {
		e.kit.extRunner.SetHeader(config)
	}
}

func (e *extensionAPI) RemoveHeader() {
	if e.kit.extRunner != nil {
		e.kit.extRunner.RemoveHeader()
	}
}

func (e *extensionAPI) GetHeader() *extensions.HeaderFooterConfig {
	if e.kit.extRunner == nil {
		return nil
	}
	return e.kit.extRunner.GetHeader()
}

func (e *extensionAPI) SetFooter(config extensions.HeaderFooterConfig) {
	if e.kit.extRunner != nil {
		e.kit.extRunner.SetFooter(config)
	}
}

func (e *extensionAPI) RemoveFooter() {
	if e.kit.extRunner != nil {
		e.kit.extRunner.RemoveFooter()
	}
}

func (e *extensionAPI) GetFooter() *extensions.HeaderFooterConfig {
	if e.kit.extRunner == nil {
		return nil
	}
	return e.kit.extRunner.GetFooter()
}

// Editor

func (e *extensionAPI) SetEditor(config extensions.EditorConfig) {
	if e.kit.extRunner != nil {
		e.kit.extRunner.SetEditor(config)
	}
}

func (e *extensionAPI) ResetEditor() {
	if e.kit.extRunner != nil {
		e.kit.extRunner.ResetEditor()
	}
}

func (e *extensionAPI) GetEditor() *extensions.EditorConfig {
	if e.kit.extRunner == nil {
		return nil
	}
	return e.kit.extRunner.GetEditor()
}

// UI Visibility

func (e *extensionAPI) SetUIVisibility(v extensions.UIVisibility) {
	if e.kit.extRunner != nil {
		e.kit.extRunner.SetUIVisibility(v)
	}
}

func (e *extensionAPI) GetUIVisibility() *extensions.UIVisibility {
	if e.kit.extRunner == nil {
		return nil
	}
	return e.kit.extRunner.GetUIVisibility()
}

// Tool rendering

func (e *extensionAPI) GetToolRenderer(toolName string) *extensions.ToolRenderConfig {
	if e.kit.extRunner == nil {
		return nil
	}
	return e.kit.extRunner.GetToolRenderer(toolName)
}

func (e *extensionAPI) GetMessageRenderer(name string) *extensions.MessageRendererConfig {
	if e.kit.extRunner == nil {
		return nil
	}
	return e.kit.extRunner.GetMessageRenderer(name)
}

// Session data

func (e *extensionAPI) GetSessionMessages() []extensions.SessionMessage {
	if e.kit.session == nil {
		return nil
	}

	// Try to use the legacy iterBranchMessages for backward compatibility
	// with the default TreeManager adapter
	if adapter, ok := e.kit.session.(*treeManagerAdapter); ok {
		return iterBranchMessages(adapter.inner, func(me *session.MessageEntry, msg message.Message) extensions.SessionMessage {
			return extensions.SessionMessage{
				ID:        me.ID,
				Role:      string(msg.Role),
				Content:   msg.Content(),
				Timestamp: me.Timestamp.Format("2006-01-02T15:04:05Z07:00"),
			}
		})
	}

	// For custom SessionManagers, use the public interface
	branch := e.kit.session.GetCurrentBranch()
	var result []extensions.SessionMessage
	for _, entry := range branch {
		if entry.Type == EntryTypeMessage {
			result = append(result, extensions.SessionMessage{
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

func (e *extensionAPI) GetEntries(extType string) []extensions.ExtensionEntry {
	if e.kit.session == nil {
		return nil
	}
	entries := e.kit.session.GetExtensionData(extType)
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

// Status bar

func (e *extensionAPI) SetStatus(entry extensions.StatusBarEntry) {
	if e.kit.extRunner != nil {
		e.kit.extRunner.SetStatusEntry(entry)
	}
}

func (e *extensionAPI) RemoveStatus(key string) {
	if e.kit.extRunner != nil {
		e.kit.extRunner.RemoveStatusEntry(key)
	}
}

func (e *extensionAPI) GetStatusEntries() []extensions.StatusBarEntry {
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

func (e *extensionAPI) GetToolInfos() []extensions.ToolInfo {
	agentTools := e.kit.agent.GetTools()
	coreCount := e.kit.agent.GetCoreToolCount()
	mcpCount := e.kit.agent.GetMCPToolCount()

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
		if e.kit.extRunner != nil && e.kit.extRunner.IsToolDisabled(info.Name) {
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

func (e *extensionAPI) Commands() []extensions.CommandDef {
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
