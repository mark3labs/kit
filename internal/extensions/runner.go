package extensions

import (
	"fmt"
	"sort"
	"sync"

	"github.com/charmbracelet/log"
)

// Runner manages loaded extensions and dispatches events to their handlers
// sequentially, mirroring Pi's ExtensionRunner. Handlers execute in extension
// load order; for cancellable events the first blocking result wins.
type Runner struct {
	extensions   []LoadedExtension
	ctx          Context
	widgets      map[string]WidgetConfig // keyed by widget ID
	header       *HeaderFooterConfig     // nil = no custom header
	footer       *HeaderFooterConfig     // nil = no custom footer
	customEditor *EditorConfig           // nil = no custom editor interceptor
	uiVisibility *UIVisibility           // nil = show everything (default)
	mu           sync.RWMutex
}

// LoadedExtension represents a single extension that has been discovered,
// loaded, and initialised. It holds the registered handlers and any custom
// tools, commands, or tool renderers the extension provided.
type LoadedExtension struct {
	Path          string
	Handlers      map[EventType][]HandlerFunc
	Tools         []ToolDef
	Commands      []CommandDef
	ToolRenderers []ToolRenderConfig
}

// NewRunner creates a Runner from a set of loaded extensions.
func NewRunner(exts []LoadedExtension) *Runner {
	return &Runner{extensions: exts}
}

// SetContext updates the runtime context (session ID, model, etc.) that is
// passed to every handler invocation. Thread-safe.
func (r *Runner) SetContext(ctx Context) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ctx = ctx
}

// HasHandlers returns true if any loaded extension has at least one handler
// registered for the given event type.
func (r *Runner) HasHandlers(event EventType) bool {
	for i := range r.extensions {
		if len(r.extensions[i].Handlers[event]) > 0 {
			return true
		}
	}
	return false
}

// Emit dispatches an event to all matching handlers sequentially. It returns
// the accumulated result from all handlers, or nil if no handler responded.
//
// For blocking events (ToolCall, Input), the first blocking result short-circuits:
//   - ToolCallResult{Block: true} stops iteration and returns immediately.
//   - InputResult{Action: "handled"} stops iteration and returns immediately.
//
// For chainable events (ToolResult), each handler sees the accumulated result
// from previous handlers. The final merged result is returned.
//
// Panics in handlers are recovered and logged; they do not crash the process.
func (r *Runner) Emit(event Event) (Result, error) {
	r.mu.RLock()
	ctx := r.ctx
	r.mu.RUnlock()

	var accumulated Result

	for i := range r.extensions {
		ext := &r.extensions[i]
		handlers := ext.Handlers[event.Type()]
		for _, handler := range handlers {
			result, err := safeCall(handler, event, ctx)
			if err != nil {
				log.Warn("extension handler error",
					"path", ext.Path,
					"event", event.Type(),
					"err", err)
				continue
			}
			if result == nil {
				continue
			}

			// Check for blocking/short-circuit results.
			if isBlocking(result) {
				return result, nil
			}

			// Chain: keep the latest non-nil result. For ToolResultResult
			// the caller is responsible for applying the modifications.
			accumulated = result
		}
	}
	return accumulated, nil
}

// RegisteredTools returns all custom tools registered by loaded extensions.
func (r *Runner) RegisteredTools() []ToolDef {
	var tools []ToolDef
	for i := range r.extensions {
		tools = append(tools, r.extensions[i].Tools...)
	}
	return tools
}

// RegisteredCommands returns all slash commands registered by loaded extensions.
func (r *Runner) RegisteredCommands() []CommandDef {
	var cmds []CommandDef
	for i := range r.extensions {
		cmds = append(cmds, r.extensions[i].Commands...)
	}
	return cmds
}

// GetContext returns the current runtime context. Thread-safe.
func (r *Runner) GetContext() Context {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.ctx
}

// Extensions returns the loaded extensions for inspection (e.g. CLI list).
func (r *Runner) Extensions() []LoadedExtension {
	return r.extensions
}

// ---------------------------------------------------------------------------
// Widget management
// ---------------------------------------------------------------------------

// SetWidget places or updates a persistent widget. The widget is identified
// by config.ID; calling SetWidget with the same ID replaces the previous
// content. Thread-safe.
func (r *Runner) SetWidget(config WidgetConfig) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.widgets == nil {
		r.widgets = make(map[string]WidgetConfig)
	}
	r.widgets[config.ID] = config
}

// RemoveWidget removes a widget by ID. No-op if the ID does not exist.
// Thread-safe.
func (r *Runner) RemoveWidget(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.widgets, id)
}

// GetWidgets returns all widgets matching the given placement, sorted by
// priority (ascending). Thread-safe.
func (r *Runner) GetWidgets(placement WidgetPlacement) []WidgetConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []WidgetConfig
	for _, w := range r.widgets {
		if w.Placement == placement {
			result = append(result, w)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Priority != result[j].Priority {
			return result[i].Priority < result[j].Priority
		}
		return result[i].ID < result[j].ID // stable tie-break
	})
	return result
}

// ---------------------------------------------------------------------------
// Header/Footer management
// ---------------------------------------------------------------------------

// SetHeader places or replaces the custom header. Thread-safe.
func (r *Runner) SetHeader(config HeaderFooterConfig) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.header = &config
}

// RemoveHeader removes the custom header. No-op if none is set. Thread-safe.
func (r *Runner) RemoveHeader() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.header = nil
}

// GetHeader returns the current custom header, or nil if none is set.
// Thread-safe.
func (r *Runner) GetHeader() *HeaderFooterConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.header == nil {
		return nil
	}
	// Return a copy to avoid races on the caller side.
	h := *r.header
	return &h
}

// SetFooter places or replaces the custom footer. Thread-safe.
func (r *Runner) SetFooter(config HeaderFooterConfig) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.footer = &config
}

// RemoveFooter removes the custom footer. No-op if none is set. Thread-safe.
func (r *Runner) RemoveFooter() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.footer = nil
}

// GetFooter returns the current custom footer, or nil if none is set.
// Thread-safe.
func (r *Runner) GetFooter() *HeaderFooterConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.footer == nil {
		return nil
	}
	// Return a copy to avoid races on the caller side.
	f := *r.footer
	return &f
}

// ---------------------------------------------------------------------------
// Editor interceptor management
// ---------------------------------------------------------------------------

// SetEditor installs an editor interceptor that wraps the built-in input
// editor. Only one interceptor is active at a time; calling SetEditor replaces
// any previous interceptor. Thread-safe.
func (r *Runner) SetEditor(config EditorConfig) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.customEditor = &config
}

// ResetEditor removes the active editor interceptor and restores the default
// built-in editor behavior. No-op if no interceptor is set. Thread-safe.
func (r *Runner) ResetEditor() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.customEditor = nil
}

// GetEditor returns the current editor interceptor, or nil if none is set.
// Thread-safe. Returns a shallow copy — function fields are reference types
// so the copy is safe.
func (r *Runner) GetEditor() *EditorConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.customEditor == nil {
		return nil
	}
	e := *r.customEditor
	return &e
}

// ---------------------------------------------------------------------------
// UI visibility management
// ---------------------------------------------------------------------------

// SetUIVisibility updates the UI visibility overrides. Thread-safe.
func (r *Runner) SetUIVisibility(v UIVisibility) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.uiVisibility = &v
}

// GetUIVisibility returns the current UI visibility overrides, or nil if
// none have been set (meaning show everything). Thread-safe.
func (r *Runner) GetUIVisibility() *UIVisibility {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.uiVisibility == nil {
		return nil
	}
	v := *r.uiVisibility
	return &v
}

// ---------------------------------------------------------------------------
// Tool renderer management
// ---------------------------------------------------------------------------

// GetToolRenderer returns the custom renderer for the named tool, or nil if
// no extension registered a renderer for it. If multiple extensions register
// renderers for the same tool, the last one (by load order) wins. Thread-safe
// (extensions are immutable after loading).
func (r *Runner) GetToolRenderer(toolName string) *ToolRenderConfig {
	// Walk extensions in reverse so last-registered wins.
	for i := len(r.extensions) - 1; i >= 0; i-- {
		for j := len(r.extensions[i].ToolRenderers) - 1; j >= 0; j-- {
			if r.extensions[i].ToolRenderers[j].ToolName == toolName {
				config := r.extensions[i].ToolRenderers[j]
				return &config
			}
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// safeCall invokes a handler, recovering from panics.
func safeCall(handler HandlerFunc, event Event, ctx Context) (result Result, err error) {
	defer func() {
		if rec := recover(); rec != nil {
			err = fmt.Errorf("extension panicked: %v", rec)
		}
	}()
	return handler(event, ctx), nil
}

// isBlocking returns true if the result should short-circuit further handlers.
func isBlocking(result Result) bool {
	switch r := result.(type) {
	case ToolCallResult:
		return r.Block
	case InputResult:
		return r.Action == "handled"
	}
	return false
}
