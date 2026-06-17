package extensions

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/spf13/viper"
)

// ---------------------------------------------------------------------------
// reentrantMu — a per-extension mutex that allows the same goroutine to
// re-enter (e.g. handler → ctx.EmitCustomEvent → handler in same extension).
// Different goroutines are serialized, preventing concurrent state mutation.
// ---------------------------------------------------------------------------

type reentrantMu struct {
	mu    sync.Mutex
	cond  *sync.Cond
	owner int64 // goroutine ID that holds the lock, or 0
	depth int   // re-entrancy depth
}

// initReentrantMu initializes the reentrant mutex in-place. Must be called
// after the struct is at its final memory location (not before copying).
func (r *reentrantMu) init() {
	r.cond = sync.NewCond(&r.mu)
}

// lock acquires the mutex. If the calling goroutine already holds it, the
// call succeeds immediately (re-entrant). Every call to lock must be paired
// with a call to unlock.
func (r *reentrantMu) lock() {
	gid := goroutineID()
	r.mu.Lock()
	if r.owner == gid {
		// Re-entrant: same goroutine already holds the lock.
		r.depth++
		r.mu.Unlock()
		return
	}
	// Wait for the current owner to release.
	for r.owner != 0 {
		r.cond.Wait() // releases mu, blocks, re-acquires mu on wake
	}
	r.owner = gid
	r.depth = 1
	r.mu.Unlock()
}

// unlock releases the mutex (or decrements re-entrancy depth).
func (r *reentrantMu) unlock() {
	r.mu.Lock()
	r.depth--
	if r.depth == 0 {
		r.owner = 0
		r.cond.Signal()
	}
	r.mu.Unlock()
}

// goroutineID extracts the current goroutine's ID from runtime.Stack output.
// This is a well-known technique used by Go testing infrastructure.
func goroutineID() int64 {
	var buf [64]byte
	n := runtime.Stack(buf[:], false)
	// Stack output starts with "goroutine NNN ["
	s := buf[:n]
	s = s[len("goroutine "):]
	s = s[:bytes.IndexByte(s, ' ')]
	id, _ := strconv.ParseInt(string(s), 10, 64)
	return id
}

// Runner manages loaded extensions and dispatches events to their handlers
// sequentially. Handlers execute in extension
// load order; for cancellable events the first blocking result wins.
//
// Each extension has a dedicated reentrant mutex so that handlers for the
// same extension are serialized (preventing data races on shared package-level
// state), while handlers for different extensions may execute concurrently.
type Runner struct {
	extensions      []LoadedExtension
	extMu           []reentrantMu // per-extension reentrant mutex, indexed by extension position
	ctx             Context
	widgets         map[string]WidgetConfig   // keyed by widget ID
	statusEntries   map[string]StatusBarEntry // keyed by status key
	header          *HeaderFooterConfig       // nil = no custom header
	footer          *HeaderFooterConfig       // nil = no custom footer
	customEditor    *EditorConfig             // nil = no custom editor interceptor
	uiVisibility    *UIVisibility             // nil = show everything (default)
	disabledTools   map[string]bool           // nil = all tools enabled
	customEventSubs map[string][]func(string) // inter-extension event bus
	optionOverrides map[string]string         // runtime option overrides
	configStore     *viper.Viper              // per-instance config store (nil = global)
	state           map[string]string         // session-scoped extension state (last-write-wins)
	stateMu         sync.RWMutex              // guards state independently of mu
	saverMu         sync.Mutex                // serializes stateSaver invocations so atomic-rename writes don't interleave
	stateSaver      func()                    // optional persistence hook invoked after each state mutation
	mu              sync.RWMutex
}

// SetConfigStore sets the per-instance configuration store used by GetOption
// to resolve "options.<name>" config values. When unset (nil), GetOption falls
// back to the process-global viper store. Threading a per-Kit store keeps
// extension option resolution isolated between Kit instances.
func (r *Runner) SetConfigStore(v *viper.Viper) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.configStore = v
}

// ShortcutEntry pairs a shortcut definition with its handler.
type ShortcutEntry struct {
	Def     ShortcutDef
	Handler func(Context)
}

// LoadedExtension represents a single extension that has been discovered,
// loaded, and initialised. It holds the registered handlers and any custom
// tools, commands, or tool renderers the extension provided.
type LoadedExtension struct {
	Path                string
	Handlers            map[EventType][]HandlerFunc
	Tools               []ToolDef
	Commands            []CommandDef
	ToolRenderers       []ToolRenderConfig
	MessageRenderers    []MessageRendererConfig   // named message renderers
	CustomEventHandlers map[string][]func(string) // inter-extension event bus
	Options             []OptionDef               // registered configuration options
	Shortcuts           []ShortcutEntry           // global keyboard shortcuts
}

// NewRunner creates a Runner from a set of loaded extensions.
func NewRunner(exts []LoadedExtension) *Runner {
	mus := make([]reentrantMu, len(exts))
	for i := range mus {
		mus[i].init()
	}
	return &Runner{extensions: exts, extMu: mus}
}

// SetContext updates the runtime context (session ID, model, etc.) that is
// passed to every handler invocation. Nil function fields are replaced with
// safe no-ops so extension handlers never panic on a missing callback.
// Thread-safe.
func (r *Runner) SetContext(ctx Context) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ctx = normalizeContext(ctx)
}

// normalizeContext replaces nil function fields in ctx with no-op stubs so
// that extension handlers can call any ctx method without a nil-function panic.
func normalizeContext(ctx Context) Context {
	if ctx.Print == nil {
		ctx.Print = func(string) {}
	}
	if ctx.PrintInfo == nil {
		ctx.PrintInfo = func(string) {}
	}
	if ctx.PrintError == nil {
		ctx.PrintError = func(string) {}
	}
	if ctx.PrintBlock == nil {
		ctx.PrintBlock = func(PrintBlockOpts) {}
	}
	if ctx.SendMessage == nil {
		ctx.SendMessage = func(string) {}
	}
	if ctx.CancelAndSend == nil {
		ctx.CancelAndSend = func(string) {}
	}
	if ctx.Abort == nil {
		ctx.Abort = func() {}
	}
	if ctx.IsIdle == nil {
		ctx.IsIdle = func() bool { return true }
	}
	if ctx.Compact == nil {
		ctx.Compact = func(CompactConfig) error { return fmt.Errorf("compact not available") }
	}
	if ctx.SendMultimodalMessage == nil {
		ctx.SendMultimodalMessage = func(string, []FilePart) {}
	}
	if ctx.NewSession == nil {
		ctx.NewSession = func(string) error { return fmt.Errorf("new session not available") }
	}
	if ctx.GetSessionUsage == nil {
		ctx.GetSessionUsage = func() SessionUsage { return SessionUsage{} }
	}
	if ctx.SetWidget == nil {
		ctx.SetWidget = func(WidgetConfig) {}
	}
	if ctx.RemoveWidget == nil {
		ctx.RemoveWidget = func(string) {}
	}
	if ctx.SetHeader == nil {
		ctx.SetHeader = func(HeaderFooterConfig) {}
	}
	if ctx.RemoveHeader == nil {
		ctx.RemoveHeader = func() {}
	}
	if ctx.SetFooter == nil {
		ctx.SetFooter = func(HeaderFooterConfig) {}
	}
	if ctx.RemoveFooter == nil {
		ctx.RemoveFooter = func() {}
	}
	if ctx.PromptSelect == nil {
		ctx.PromptSelect = func(PromptSelectConfig) PromptSelectResult {
			return PromptSelectResult{Cancelled: true}
		}
	}
	if ctx.PromptConfirm == nil {
		ctx.PromptConfirm = func(PromptConfirmConfig) PromptConfirmResult {
			return PromptConfirmResult{Cancelled: true}
		}
	}
	if ctx.PromptInput == nil {
		ctx.PromptInput = func(PromptInputConfig) PromptInputResult {
			return PromptInputResult{Cancelled: true}
		}
	}
	if ctx.PromptMultiSelect == nil {
		ctx.PromptMultiSelect = func(PromptMultiSelectConfig) PromptMultiSelectResult {
			return PromptMultiSelectResult{Cancelled: true}
		}
	}
	if ctx.ShowOverlay == nil {
		ctx.ShowOverlay = func(OverlayConfig) OverlayResult {
			return OverlayResult{Cancelled: true, Index: -1}
		}
	}
	if ctx.SetEditor == nil {
		ctx.SetEditor = func(EditorConfig) {}
	}
	if ctx.ResetEditor == nil {
		ctx.ResetEditor = func() {}
	}
	if ctx.SetEditorText == nil {
		ctx.SetEditorText = func(string) {}
	}
	if ctx.SetUIVisibility == nil {
		ctx.SetUIVisibility = func(UIVisibility) {}
	}
	if ctx.SetStatus == nil {
		ctx.SetStatus = func(string, string, int) {}
	}
	if ctx.RemoveStatus == nil {
		ctx.RemoveStatus = func(string) {}
	}
	if ctx.GetContextStats == nil {
		ctx.GetContextStats = func() ContextStats { return ContextStats{} }
	}
	if ctx.GetMessages == nil {
		ctx.GetMessages = func() []SessionMessage { return nil }
	}
	if ctx.GetSessionPath == nil {
		ctx.GetSessionPath = func() string { return "" }
	}
	if ctx.AppendEntry == nil {
		ctx.AppendEntry = func(string, string) (string, error) { return "", nil }
	}
	if ctx.GetEntries == nil {
		ctx.GetEntries = func(string) []ExtensionEntry { return nil }
	}
	if ctx.SetState == nil {
		ctx.SetState = func(string, string) {}
	}
	if ctx.GetState == nil {
		ctx.GetState = func(string) (string, bool) { return "", false }
	}
	if ctx.DeleteState == nil {
		ctx.DeleteState = func(string) {}
	}
	if ctx.ListState == nil {
		ctx.ListState = func() []string { return nil }
	}
	if ctx.GetOption == nil {
		ctx.GetOption = func(string) string { return "" }
	}
	if ctx.SetOption == nil {
		ctx.SetOption = func(string, string) {}
	}
	if ctx.SetModel == nil {
		ctx.SetModel = func(string) error { return nil }
	}
	if ctx.GetAvailableModels == nil {
		ctx.GetAvailableModels = func() []ModelInfoEntry { return nil }
	}
	if ctx.EmitCustomEvent == nil {
		ctx.EmitCustomEvent = func(string, string) {}
	}
	if ctx.GetAllTools == nil {
		ctx.GetAllTools = func() []ToolInfo { return nil }
	}
	if ctx.SetActiveTools == nil {
		ctx.SetActiveTools = func([]string) {}
	}
	if ctx.Exit == nil {
		ctx.Exit = func() {}
	}
	if ctx.Complete == nil {
		ctx.Complete = func(CompleteRequest) (CompleteResponse, error) {
			return CompleteResponse{}, nil
		}
	}
	if ctx.SuspendTUI == nil {
		ctx.SuspendTUI = func(callback func()) error { callback(); return nil }
	}
	if ctx.RenderMessage == nil {
		ctx.RenderMessage = func(string, string) {}
	}
	if ctx.RegisterTheme == nil {
		ctx.RegisterTheme = func(string, ThemeColorConfig) {}
	}
	if ctx.SetTheme == nil {
		ctx.SetTheme = func(string) error { return nil }
	}
	if ctx.ListThemes == nil {
		ctx.ListThemes = func() []string { return nil }
	}
	if ctx.ReloadExtensions == nil {
		ctx.ReloadExtensions = func() error { return nil }
	}
	if ctx.SpawnSubagent == nil {
		ctx.SpawnSubagent = func(SubagentConfig) (*SubagentHandle, *SubagentResult, error) {
			return nil, nil, nil
		}
	}

	// -------------------------------------------------------------------------
	// Tree Navigation API no-ops
	// -------------------------------------------------------------------------
	if ctx.GetTreeNode == nil {
		ctx.GetTreeNode = func(string) *TreeNode { return nil }
	}
	if ctx.GetCurrentBranch == nil {
		ctx.GetCurrentBranch = func() []TreeNode { return nil }
	}
	if ctx.GetChildren == nil {
		ctx.GetChildren = func(string) []string { return nil }
	}
	if ctx.NavigateTo == nil {
		ctx.NavigateTo = func(string) TreeNavigationResult {
			return TreeNavigationResult{Success: false, Error: "not implemented"}
		}
	}
	if ctx.SummarizeBranch == nil {
		ctx.SummarizeBranch = func(string, string) string {
			return ""
		}
	}
	if ctx.CollapseBranch == nil {
		ctx.CollapseBranch = func(string, string, string) TreeNavigationResult {
			return TreeNavigationResult{Success: false, Error: "not implemented"}
		}
	}

	// -------------------------------------------------------------------------
	// Skill Loading API no-ops
	// -------------------------------------------------------------------------
	if ctx.LoadSkill == nil {
		ctx.LoadSkill = func(string) (*Skill, string) { return nil, "" }
	}
	if ctx.LoadSkillsFromDir == nil {
		ctx.LoadSkillsFromDir = func(string) SkillLoadResult { return SkillLoadResult{} }
	}
	if ctx.DiscoverSkills == nil {
		ctx.DiscoverSkills = func() SkillLoadResult { return SkillLoadResult{} }
	}
	if ctx.InjectSkillAsContext == nil {
		ctx.InjectSkillAsContext = func(string) string { return "" }
	}
	if ctx.InjectRawSkillAsContext == nil {
		ctx.InjectRawSkillAsContext = func(string) string { return "" }
	}
	if ctx.GetAvailableSkills == nil {
		ctx.GetAvailableSkills = func() []Skill { return nil }
	}

	// -------------------------------------------------------------------------
	// Template Parsing API no-ops
	// -------------------------------------------------------------------------
	if ctx.ParseTemplate == nil {
		ctx.ParseTemplate = func(string, string) PromptTemplate { return PromptTemplate{} }
	}
	if ctx.RenderTemplate == nil {
		ctx.RenderTemplate = func(PromptTemplate, map[string]string) string { return "" }
	}
	if ctx.ParseArguments == nil {
		ctx.ParseArguments = func(string, ArgumentPattern) ParseResult { return ParseResult{} }
	}
	if ctx.SimpleParseArguments == nil {
		ctx.SimpleParseArguments = func(string, int) []string { return nil }
	}
	if ctx.EvaluateModelConditional == nil {
		ctx.EvaluateModelConditional = func(string) bool { return false }
	}
	if ctx.RenderWithModelConditionals == nil {
		ctx.RenderWithModelConditionals = func(string) string { return "" }
	}

	// -------------------------------------------------------------------------
	// Model Resolution API no-ops
	// -------------------------------------------------------------------------
	if ctx.ResolveModelChain == nil {
		ctx.ResolveModelChain = func([]string) ModelResolutionResult {
			return ModelResolutionResult{Error: "not implemented"}
		}
	}
	if ctx.GetModelCapabilities == nil {
		ctx.GetModelCapabilities = func(string) (ModelCapabilities, string) {
			return ModelCapabilities{}, "not implemented"
		}
	}
	if ctx.CheckModelAvailable == nil {
		ctx.CheckModelAvailable = func(string) bool { return false }
	}
	if ctx.GetCurrentProvider == nil {
		ctx.GetCurrentProvider = func() string { return "" }
	}
	if ctx.GetCurrentModelID == nil {
		ctx.GetCurrentModelID = func() string { return "" }
	}

	return ctx
}

// GetContext returns a snapshot of the current runtime context. Thread-safe.
func (r *Runner) GetContext() Context {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.ctx
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
		if len(handlers) == 0 {
			continue
		}

		r.extMu[i].lock()
		for _, handler := range handlers {
			result, err := safeCall(handler, event, ctx)
			if err != nil {
				log.Printf("WARN extension handler error: path=%s event=%s err=%v", ext.Path, event.Type(), err)
				continue
			}
			if result == nil {
				continue
			}

			// Check for blocking/short-circuit results.
			if isBlocking(result) {
				r.extMu[i].unlock()
				return result, nil
			}

			// Chain: keep the latest non-nil result. For ToolResultResult
			// the caller is responsible for applying the modifications.
			accumulated = result
		}
		r.extMu[i].unlock()
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
// Status bar management
// ---------------------------------------------------------------------------

// SetStatusEntry places or updates a keyed status bar entry. Thread-safe.
func (r *Runner) SetStatusEntry(entry StatusBarEntry) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.statusEntries == nil {
		r.statusEntries = make(map[string]StatusBarEntry)
	}
	r.statusEntries[entry.Key] = entry
}

// RemoveStatusEntry removes a status bar entry by key. Thread-safe.
func (r *Runner) RemoveStatusEntry(key string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.statusEntries, key)
}

// GetStatusEntries returns all status bar entries, sorted by priority
// (ascending). Thread-safe.
func (r *Runner) GetStatusEntries() []StatusBarEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]StatusBarEntry, 0, len(r.statusEntries))
	for _, e := range r.statusEntries {
		result = append(result, e)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Priority != result[j].Priority {
			return result[i].Priority < result[j].Priority
		}
		return result[i].Key < result[j].Key
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
// Message renderer management
// ---------------------------------------------------------------------------

// GetMessageRenderer returns the named message renderer, or nil if no
// extension registered a renderer with that name. If multiple extensions
// register the same name, the last one (by load order) wins.
func (r *Runner) GetMessageRenderer(name string) *MessageRendererConfig {
	for i := len(r.extensions) - 1; i >= 0; i-- {
		for j := len(r.extensions[i].MessageRenderers) - 1; j >= 0; j-- {
			if r.extensions[i].MessageRenderers[j].Name == name {
				config := r.extensions[i].MessageRenderers[j]
				return &config
			}
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Extension state store (session-scoped, last-write-wins)
// ---------------------------------------------------------------------------

// SetState records a key-value pair in the runner's session-scoped extension
// state store. The store is in-memory; callers wire SetStateSaver to persist
// changes to a sidecar file. Thread-safe.
//
// When a saver is installed, concurrent SetState/DeleteState invocations are
// serialized through saverMu so that overlapping snapshot-and-rename writes
// cannot interleave (which would otherwise race on the shared tmp file and
// risk persisting an older snapshot after a newer one).
func (r *Runner) SetState(key, value string) {
	r.stateMu.Lock()
	if r.state == nil {
		r.state = make(map[string]string)
	}
	r.state[key] = value
	saver := r.stateSaver
	r.stateMu.Unlock()
	r.runSaver(saver)
}

// GetState returns the value previously stored via SetState, plus a bool
// indicating whether the key was present. Thread-safe.
func (r *Runner) GetState(key string) (string, bool) {
	r.stateMu.RLock()
	defer r.stateMu.RUnlock()
	v, ok := r.state[key]
	return v, ok
}

// DeleteState removes a key from the state store. No-op if the key is
// missing. Thread-safe. Saver invocations are serialized via saverMu — see
// SetState for the rationale.
func (r *Runner) DeleteState(key string) {
	r.stateMu.Lock()
	_, existed := r.state[key]
	if existed {
		delete(r.state, key)
	}
	saver := r.stateSaver
	r.stateMu.Unlock()
	if !existed {
		return
	}
	r.runSaver(saver)
}

// runSaver invokes the optional persistence callback under saverMu so
// concurrent SetState/DeleteState writers cannot race on the shared tmp
// file used by SaveStateToFile's atomic rename. The deferred Unlock
// guarantees saverMu is released even if the saver panics.
func (r *Runner) runSaver(saver func()) {
	if saver == nil {
		return
	}
	r.saverMu.Lock()
	defer r.saverMu.Unlock()
	saver()
}

// ListState returns all keys currently in the state store, in unspecified
// order. Thread-safe.
func (r *Runner) ListState() []string {
	r.stateMu.RLock()
	defer r.stateMu.RUnlock()
	if len(r.state) == 0 {
		return nil
	}
	keys := make([]string, 0, len(r.state))
	for k := range r.state {
		keys = append(keys, k)
	}
	return keys
}

// SetStateSaver installs an optional persistence hook invoked after each
// mutation to the state store (SetState / DeleteState / LoadStateFromFile).
// Pass nil to disable persistence. Thread-safe.
func (r *Runner) SetStateSaver(saver func()) {
	r.stateMu.Lock()
	defer r.stateMu.Unlock()
	r.stateSaver = saver
}

// SnapshotState returns a copy of the current state store as a
// fresh map. Useful for persisting to disk without holding the lock.
// Thread-safe.
func (r *Runner) SnapshotState() map[string]string {
	r.stateMu.RLock()
	defer r.stateMu.RUnlock()
	if len(r.state) == 0 {
		return nil
	}
	copyMap := make(map[string]string, len(r.state))
	maps.Copy(copyMap, r.state)
	return copyMap
}

// LoadStateFromFile reads a JSON map from path and replaces the in-memory
// state store with its contents. Missing or empty files are treated as
// "no prior state": the in-memory store is replaced with an empty map so
// callers can safely switch sessions without leaking keys from a prior
// session into a new one. Malformed JSON returns the parse error without
// touching the existing store. Thread-safe.
func (r *Runner) LoadStateFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			r.stateMu.Lock()
			r.state = map[string]string{}
			r.stateMu.Unlock()
			return nil
		}
		return fmt.Errorf("reading extension state: %w", err)
	}
	if len(data) == 0 {
		r.stateMu.Lock()
		r.state = map[string]string{}
		r.stateMu.Unlock()
		return nil
	}
	var loaded map[string]string
	if err := json.Unmarshal(data, &loaded); err != nil {
		return fmt.Errorf("parsing extension state: %w", err)
	}
	r.stateMu.Lock()
	r.state = loaded
	r.stateMu.Unlock()
	return nil
}

// SaveStateToFile writes the current state store to path as JSON, creating
// parent directories as needed. An empty store writes an empty object so
// that consumers can distinguish "loaded but empty" from "never saved".
// Writes are atomic via a tmp-file-and-rename sequence. Thread-safe.
func (r *Runner) SaveStateToFile(path string) error {
	snap := r.SnapshotState()
	if snap == nil {
		snap = map[string]string{}
	}
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling extension state: %w", err)
	}
	if dir := filepath.Dir(path); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("creating state directory: %w", err)
		}
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("writing extension state: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("renaming extension state: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Hot-reload
// ---------------------------------------------------------------------------

// Reload replaces the loaded extensions with a fresh set and clears all
// dynamic state (widgets, status, header/footer, editor, visibility,
// disabled tools, custom event subscriptions). Option overrides are
// preserved across reloads since they represent user intent.
//
// The caller is responsible for emitting SessionShutdown before calling
// Reload and SessionStart after.
func (r *Runner) Reload(exts []LoadedExtension) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.extensions = exts
	r.widgets = nil
	r.statusEntries = nil
	r.header = nil
	r.footer = nil
	r.customEditor = nil
	r.uiVisibility = nil
	r.disabledTools = nil
	r.customEventSubs = nil
	// optionOverrides and state are intentionally preserved across reloads:
	// they represent user/session intent (not extension code) and would be
	// surprising to lose on a hot-reload.
}

// ---------------------------------------------------------------------------
// Inter-extension event bus
// ---------------------------------------------------------------------------

// SubscribeCustomEvent registers a handler for a named custom event. Handlers
// execute in registration order when EmitCustomEvent is called. Thread-safe.
func (r *Runner) SubscribeCustomEvent(name string, handler func(string)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.customEventSubs == nil {
		r.customEventSubs = make(map[string][]func(string))
	}
	r.customEventSubs[name] = append(r.customEventSubs[name], handler)
}

// EmitCustomEvent dispatches a named event to all subscribed handlers.
// Handlers run synchronously in extension load order. Panics are recovered
// and logged. Thread-safe.
func (r *Runner) EmitCustomEvent(name, data string) {
	// Collect handlers: extension-registered (Init-time) + dynamic subs.
	r.mu.RLock()
	dynamicHandlers := r.customEventSubs[name]
	r.mu.RUnlock()

	safeInvoke := func(h func(string)) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("WARN custom event handler panicked: event=%s err=%v", name, rec)
			}
		}()
		h(data)
	}

	// Extension-registered handlers first (in load order).
	for i := range r.extensions {
		extHandlers := r.extensions[i].CustomEventHandlers[name]
		if len(extHandlers) == 0 {
			continue
		}
		r.extMu[i].lock()
		for _, h := range extHandlers {
			safeInvoke(h)
		}
		r.extMu[i].unlock()
	}
	// Then dynamic subscriptions (not extension-scoped, no per-ext lock).
	for _, h := range dynamicHandlers {
		safeInvoke(h)
	}
}

// ---------------------------------------------------------------------------
// Tool management
// ---------------------------------------------------------------------------

// SetActiveTools restricts the tool set to the named tools. All tools not in
// the list are disabled. Passing nil or an empty slice re-enables all tools.
// Thread-safe.
func (r *Runner) SetActiveTools(names []string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(names) == 0 {
		r.disabledTools = nil
		return
	}
	active := make(map[string]bool, len(names))
	for _, n := range names {
		active[n] = true
	}
	r.disabledTools = active // non-nil = only these tools are allowed
}

// IsToolDisabled returns true if the tool has been disabled via SetActiveTools.
// Thread-safe.
func (r *Runner) IsToolDisabled(toolName string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.disabledTools == nil {
		return false // no filter = all enabled
	}
	return !r.disabledTools[toolName]
}

// ---------------------------------------------------------------------------
// Extension options
// ---------------------------------------------------------------------------

// GetOption resolves a named option value in priority order:
//  1. Runtime override (via SetOption)
//  2. Environment variable: KIT_OPT_<NAME> (uppercased, dashes → underscores)
//  3. Viper config: options.<name>
//  4. Default value from RegisterOption
//
// Returns empty string if the option was never registered.
// Thread-safe.
func (r *Runner) GetOption(name string) string {
	// 1. Runtime override.
	r.mu.RLock()
	if v, ok := r.optionOverrides[name]; ok {
		r.mu.RUnlock()
		return v
	}
	r.mu.RUnlock()

	// 2. Environment variable: KIT_OPT_<NAME>
	envKey := "KIT_OPT_" + strings.ToUpper(strings.ReplaceAll(name, "-", "_"))
	if v := os.Getenv(envKey); v != "" {
		return v
	}

	// 3. Viper config: options.<name>
	configKey := "options." + name
	r.mu.RLock()
	store := r.configStore
	r.mu.RUnlock()
	if store == nil {
		store = viper.GetViper()
	}
	if v := store.GetString(configKey); v != "" {
		return v
	}

	// 4. Default from registered option defs.
	for i := range r.extensions {
		for _, opt := range r.extensions[i].Options {
			if opt.Name == name {
				return opt.Default
			}
		}
	}

	return ""
}

// SetOption stores a runtime override for a named option. This takes highest
// priority over env vars, config, and defaults. Thread-safe.
func (r *Runner) SetOption(name, value string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.optionOverrides == nil {
		r.optionOverrides = make(map[string]string)
	}
	r.optionOverrides[name] = value
}

// RegisteredOptions returns all option definitions from all loaded extensions.
func (r *Runner) RegisteredOptions() []OptionDef {
	var opts []OptionDef
	for i := range r.extensions {
		opts = append(opts, r.extensions[i].Options...)
	}
	return opts
}

// ---------------------------------------------------------------------------
// Keyboard shortcuts
// ---------------------------------------------------------------------------

// GetShortcuts returns all registered keyboard shortcuts as a map of
// key binding → handler. If multiple extensions register the same key,
// the last registration wins. Thread-safe (reads extension list which is
// immutable after loading).
func (r *Runner) GetShortcuts() map[string]ShortcutEntry {
	result := make(map[string]ShortcutEntry)
	for i := range r.extensions {
		for _, sc := range r.extensions[i].Shortcuts {
			result[sc.Def.Key] = sc
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

// RegisteredShortcuts returns all shortcut definitions from all loaded
// extensions. Used for help/listing commands.
func (r *Runner) RegisteredShortcuts() []ShortcutDef {
	var defs []ShortcutDef
	seen := make(map[string]bool)
	// Iterate in reverse so last registration for a key wins.
	for i := len(r.extensions) - 1; i >= 0; i-- {
		for _, sc := range r.extensions[i].Shortcuts {
			if !seen[sc.Def.Key] {
				seen[sc.Def.Key] = true
				defs = append(defs, sc.Def)
			}
		}
	}
	return defs
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
	case BeforeForkResult:
		return r.Cancel
	case BeforeSessionSwitchResult:
		return r.Cancel
	case BeforeCompactResult:
		return r.Cancel
	}
	return false
}
