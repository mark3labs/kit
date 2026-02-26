package extensions

import (
	"fmt"
	"sync"

	"github.com/charmbracelet/log"
)

// Runner manages loaded extensions and dispatches events to their handlers
// sequentially, mirroring Pi's ExtensionRunner. Handlers execute in extension
// load order; for cancellable events the first blocking result wins.
type Runner struct {
	extensions []LoadedExtension
	ctx        Context
	mu         sync.RWMutex
}

// LoadedExtension represents a single extension that has been discovered,
// loaded, and initialised. It holds the registered handlers and any custom
// tools or commands the extension provided.
type LoadedExtension struct {
	Path     string
	Handlers map[EventType][]HandlerFunc
	Tools    []ToolDef
	Commands []CommandDef
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
