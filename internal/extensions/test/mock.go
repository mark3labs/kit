package test

import (
	"sync"

	"github.com/mark3labs/kit/internal/extensions"
)

// MockContext records all interactions with the extension context.
// It provides a Context object that captures Print calls, widget settings,
// and other context operations for verification in tests.
type MockContext struct {
	mu sync.RWMutex

	// Recorded calls
	Prints      []string
	PrintInfos  []string
	PrintErrors []string
	PrintBlocks []extensions.PrintBlockOpts
	Messages    []string
	CancelSends []string

	// Widget state
	Widgets       map[string]extensions.WidgetConfig
	RemovedIDs    []string
	Header        *extensions.HeaderFooterConfig
	Footer        *extensions.HeaderFooterConfig
	HeaderRemoved bool
	FooterRemoved bool

	// Context properties
	SessionID   string
	CWD         string
	Model       string
	Interactive bool

	// UI visibility
	UIVisibility *extensions.UIVisibility

	// Status entries
	StatusEntries map[string]extensions.StatusBarEntry
	RemovedStatus []string

	// Editor
	EditorConfig *extensions.EditorConfig
	EditorReset  bool
	EditorTexts  []string

	// Options
	Options map[string]string

	// Prompt results (configurable for testing)
	PromptSelectResult      extensions.PromptSelectResult
	PromptConfirmResult     extensions.PromptConfirmResult
	PromptInputResult       extensions.PromptInputResult
	PromptMultiSelectResult extensions.PromptMultiSelectResult

	// Overlay
	Overlays []extensions.OverlayConfig
}

// StatusBarEntry represents a recorded status bar entry
type StatusBarEntry struct {
	Key      string
	Text     string
	Priority int
}

// NewMockContext creates a new mock context with default values.
func NewMockContext() *MockContext {
	return &MockContext{
		Prints:        make([]string, 0),
		PrintInfos:    make([]string, 0),
		PrintErrors:   make([]string, 0),
		PrintBlocks:   make([]extensions.PrintBlockOpts, 0),
		Messages:      make([]string, 0),
		CancelSends:   make([]string, 0),
		Widgets:       make(map[string]extensions.WidgetConfig),
		RemovedIDs:    make([]string, 0),
		StatusEntries: make(map[string]extensions.StatusBarEntry),
		RemovedStatus: make([]string, 0),
		EditorTexts:   make([]string, 0),
		Options:       make(map[string]string),
		Overlays:      make([]extensions.OverlayConfig, 0),
		Interactive:   true,
		SessionID:     "test-session",
		CWD:           "/test",
		Model:         "test-model",
	}
}

// ToContext returns a extensions.Context wired to record all interactions.
func (m *MockContext) ToContext() extensions.Context {
	return extensions.Context{
		SessionID:         m.SessionID,
		CWD:               m.CWD,
		Model:             m.Model,
		Interactive:       m.Interactive,
		Print:             m.recordPrint,
		PrintInfo:         m.recordPrintInfo,
		PrintError:        m.recordPrintError,
		PrintBlock:        m.recordPrintBlock,
		SendMessage:       m.recordSendMessage,
		CancelAndSend:     m.recordCancelAndSend,
		SetWidget:         m.recordSetWidget,
		RemoveWidget:      m.recordRemoveWidget,
		SetHeader:         m.recordSetHeader,
		RemoveHeader:      m.recordRemoveHeader,
		SetFooter:         m.recordSetFooter,
		RemoveFooter:      m.recordRemoveFooter,
		PromptSelect:      m.recordPromptSelect,
		PromptConfirm:     m.recordPromptConfirm,
		PromptInput:       m.recordPromptInput,
		PromptMultiSelect: m.recordPromptMultiSelect,
		SetEditor:         m.recordSetEditor,
		ResetEditor:       m.recordResetEditor,
		SetEditorText:     m.recordSetEditorText,
		SetUIVisibility:   m.recordUIVisibility,
		GetContextStats:   m.getContextStats,
		GetMessages:       m.getMessages,
		GetSessionPath:    m.getSessionPath,
		AppendEntry:       m.appendEntry,
		GetEntries:        m.getEntries,
		SetStatus:         m.recordSetStatus,
		RemoveStatus:      m.recordRemoveStatus,
		GetOption:         m.getOption,
		SetOption:         m.setOption,
		SetModel:          m.setModel,
		GetAllTools:       m.getAllTools,
		SetActiveTools:    m.setActiveTools,
		Exit:              m.exit,
		Complete:          m.complete,
		SuspendTUI:        m.suspendTUI,
		RenderMessage:     m.renderMessage,
		RegisterTheme:     m.registerTheme,
		SetTheme:          m.setTheme,
		ListThemes:        m.listThemes,
		ReloadExtensions:  m.reloadExtensions,
		SpawnSubagent:     m.spawnSubagent,
		ShowOverlay:       m.showOverlay,
	}
}

// Record methods

func (m *MockContext) recordPrint(text string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Prints = append(m.Prints, text)
}

func (m *MockContext) recordPrintInfo(text string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.PrintInfos = append(m.PrintInfos, text)
}

func (m *MockContext) recordPrintError(text string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.PrintErrors = append(m.PrintErrors, text)
}

func (m *MockContext) recordPrintBlock(opts extensions.PrintBlockOpts) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.PrintBlocks = append(m.PrintBlocks, opts)
}

func (m *MockContext) recordSendMessage(text string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Messages = append(m.Messages, text)
}

func (m *MockContext) recordCancelAndSend(text string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.CancelSends = append(m.CancelSends, text)
}

func (m *MockContext) recordSetWidget(config extensions.WidgetConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Widgets[config.ID] = config
}

func (m *MockContext) recordRemoveWidget(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.Widgets, id)
	m.RemovedIDs = append(m.RemovedIDs, id)
}

func (m *MockContext) recordSetHeader(config extensions.HeaderFooterConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Header = &config
}

func (m *MockContext) recordRemoveHeader() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Header = nil
	m.HeaderRemoved = true
}

func (m *MockContext) recordSetFooter(config extensions.HeaderFooterConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Footer = &config
}

func (m *MockContext) recordRemoveFooter() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Footer = nil
	m.FooterRemoved = true
}

func (m *MockContext) recordSetStatus(key string, text string, priority int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.StatusEntries[key] = extensions.StatusBarEntry{
		Key:      key,
		Text:     text,
		Priority: priority,
	}
}

func (m *MockContext) recordRemoveStatus(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.StatusEntries, key)
	m.RemovedStatus = append(m.RemovedStatus, key)
}

func (m *MockContext) recordSetEditor(config extensions.EditorConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.EditorConfig = &config
}

func (m *MockContext) recordResetEditor() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.EditorReset = true
	m.EditorConfig = nil
}

func (m *MockContext) recordSetEditorText(text string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.EditorTexts = append(m.EditorTexts, text)
}

func (m *MockContext) recordUIVisibility(vis extensions.UIVisibility) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.UIVisibility = &vis
}

func (m *MockContext) recordPromptSelect(config extensions.PromptSelectConfig) extensions.PromptSelectResult {
	// Return the configured result (tests can set this)
	return m.PromptSelectResult
}

func (m *MockContext) recordPromptConfirm(config extensions.PromptConfirmConfig) extensions.PromptConfirmResult {
	return m.PromptConfirmResult
}

func (m *MockContext) recordPromptInput(config extensions.PromptInputConfig) extensions.PromptInputResult {
	return m.PromptInputResult
}

func (m *MockContext) recordPromptMultiSelect(config extensions.PromptMultiSelectConfig) extensions.PromptMultiSelectResult {
	return m.PromptMultiSelectResult
}

func (m *MockContext) showOverlay(config extensions.OverlayConfig) extensions.OverlayResult {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Overlays = append(m.Overlays, config)
	return extensions.OverlayResult{Cancelled: true} // Default to cancelled for tests
}

// Stub methods that do nothing or return defaults

func (m *MockContext) getContextStats() extensions.ContextStats {
	return extensions.ContextStats{
		EstimatedTokens: 1000,
		ContextLimit:    200000,
		UsagePercent:    0.5,
		MessageCount:    10,
	}
}

func (m *MockContext) getMessages() []extensions.SessionMessage {
	return nil
}

func (m *MockContext) getSessionPath() string {
	return ""
}

func (m *MockContext) appendEntry(entryType string, data string) (string, error) {
	return "", nil
}

func (m *MockContext) getEntries(entryType string) []extensions.ExtensionEntry {
	return nil
}

func (m *MockContext) getOption(name string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.Options[name]
}

func (m *MockContext) setOption(name string, value string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Options[name] = value
}

func (m *MockContext) setModel(modelString string) error {
	return nil
}

func (m *MockContext) getAllTools() []extensions.ToolInfo {
	return nil
}

func (m *MockContext) setActiveTools(names []string) {}

func (m *MockContext) exit() {}

func (m *MockContext) complete(req extensions.CompleteRequest) (extensions.CompleteResponse, error) {
	return extensions.CompleteResponse{}, nil
}

func (m *MockContext) suspendTUI(callback func()) error {
	callback()
	return nil
}

func (m *MockContext) renderMessage(rendererName string, content string) {}

func (m *MockContext) registerTheme(name string, config extensions.ThemeColorConfig) {}

func (m *MockContext) setTheme(name string) error {
	return nil
}

func (m *MockContext) listThemes() []string {
	return nil
}

func (m *MockContext) reloadExtensions() error {
	return nil
}

func (m *MockContext) spawnSubagent(config extensions.SubagentConfig) (*extensions.SubagentHandle, *extensions.SubagentResult, error) {
	return nil, nil, nil
}

// Accessor methods for verification

// GetPrints returns all recorded Print calls.
func (m *MockContext) GetPrints() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]string, len(m.Prints))
	copy(result, m.Prints)
	return result
}

// GetPrintInfos returns all recorded PrintInfo calls.
func (m *MockContext) GetPrintInfos() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]string, len(m.PrintInfos))
	copy(result, m.PrintInfos)
	return result
}

// GetPrintErrors returns all recorded PrintError calls.
func (m *MockContext) GetPrintErrors() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]string, len(m.PrintErrors))
	copy(result, m.PrintErrors)
	return result
}

// GetWidget returns a recorded widget by ID.
func (m *MockContext) GetWidget(id string) (extensions.WidgetConfig, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	w, ok := m.Widgets[id]
	return w, ok
}

// HasWidget reports whether a widget with the given ID was set.
func (m *MockContext) HasWidget(id string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.Widgets[id]
	return ok
}

// GetHeader returns the recorded header configuration.
func (m *MockContext) GetHeader() *extensions.HeaderFooterConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.Header
}

// GetFooter returns the recorded footer configuration.
func (m *MockContext) GetFooter() *extensions.HeaderFooterConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.Footer
}

// GetStatus returns a recorded status entry by key.
func (m *MockContext) GetStatus(key string) (extensions.StatusBarEntry, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.StatusEntries[key]
	return s, ok
}

// SetPromptSelectResult configures the result returned by PromptSelect.
func (m *MockContext) SetPromptSelectResult(result extensions.PromptSelectResult) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.PromptSelectResult = result
}

// SetPromptConfirmResult configures the result returned by PromptConfirm.
func (m *MockContext) SetPromptConfirmResult(result extensions.PromptConfirmResult) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.PromptConfirmResult = result
}

// SetPromptInputResult configures the result returned by PromptInput.
func (m *MockContext) SetPromptInputResult(result extensions.PromptInputResult) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.PromptInputResult = result
}

// SetPromptMultiSelectResult configures the result returned by PromptMultiSelect.
func (m *MockContext) SetPromptMultiSelectResult(result extensions.PromptMultiSelectResult) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.PromptMultiSelectResult = result
}
