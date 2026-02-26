package extensions

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/log"
	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
	"github.com/traefik/yaegi/stdlib/unrestricted"
)

// Discovery paths searched in order (lowest to highest precedence):
//
//	~/.config/kit/extensions/*.go          global single files
//	~/.config/kit/extensions/*/main.go     global subdirectories
//	.kit/extensions/*.go                   project-local single files
//	.kit/extensions/*/main.go              project-local subdirectories
//
// Explicit paths passed via --extension / -e flags are appended last.

// LoadExtensions discovers and loads extensions from standard locations and
// any extra paths. Each extension is loaded into its own Yaegi interpreter
// for isolation. Extensions that fail to load are logged and skipped.
func LoadExtensions(extraPaths []string) ([]LoadedExtension, error) {
	paths := discoverExtensionPaths(extraPaths)
	if len(paths) == 0 {
		return nil, nil
	}

	var loaded []LoadedExtension
	for _, p := range paths {
		ext, err := loadSingleExtension(p)
		if err != nil {
			log.Warn("skipping extension", "path", p, "err", err)
			continue
		}
		loaded = append(loaded, *ext)
		log.Debug("loaded extension", "path", p,
			"handlers", countHandlers(ext),
			"tools", len(ext.Tools),
			"commands", len(ext.Commands))
	}
	return loaded, nil
}

// discoverExtensionPaths returns deduplicated paths to extension files in
// load-order (global first, then project-local, then explicit).
func discoverExtensionPaths(extraPaths []string) []string {
	seen := make(map[string]bool)
	var paths []string

	add := func(p string) {
		abs, err := filepath.Abs(p)
		if err != nil {
			return
		}
		if seen[abs] {
			return
		}
		seen[abs] = true
		paths = append(paths, abs)
	}

	// Global extensions: $XDG_CONFIG_HOME/kit/extensions/ (default ~/.config/kit/extensions/)
	globalDir := globalExtensionsDir()
	for _, p := range findExtensionsInDir(globalDir) {
		add(p)
	}

	// Project-local extensions: .kit/extensions/
	localDir := filepath.Join(".kit", "extensions")
	for _, p := range findExtensionsInDir(localDir) {
		add(p)
	}

	// Explicit paths (highest precedence)
	for _, p := range extraPaths {
		info, err := os.Stat(p)
		if err != nil {
			continue
		}
		if info.IsDir() {
			for _, found := range findExtensionsInDir(p) {
				add(found)
			}
		} else if strings.HasSuffix(p, ".go") {
			add(p)
		}
	}

	return paths
}

// findExtensionsInDir returns .go files in dir and main.go in immediate subdirs.
func findExtensionsInDir(dir string) []string {
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return nil
	}

	var results []string

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	for _, entry := range entries {
		full := filepath.Join(dir, entry.Name())
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".go") {
			results = append(results, full)
		} else if entry.IsDir() {
			main := filepath.Join(full, "main.go")
			if _, err := os.Stat(main); err == nil {
				results = append(results, main)
			}
		}
	}
	return results
}

// globalExtensionsDir returns the global extensions directory, respecting
// $XDG_CONFIG_HOME. Defaults to ~/.config/kit/extensions.
func globalExtensionsDir() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "kit", "extensions")
}

// loadSingleExtension loads one .go file into a fresh Yaegi interpreter,
// calls the Init(ext.API) function, and returns the registered handlers.
func loadSingleExtension(path string) (*LoadedExtension, error) {
	ext := &LoadedExtension{
		Path:     path,
		Handlers: make(map[EventType][]HandlerFunc),
	}

	// Create a fresh interpreter.
	i := interp.New(interp.Options{})

	// Expose the Go stdlib. The base set covers most packages; the
	// unrestricted set adds os/exec so extensions can spawn processes.
	if err := i.Use(stdlib.Symbols); err != nil {
		return nil, fmt.Errorf("loading stdlib symbols: %w", err)
	}
	if err := i.Use(unrestricted.Symbols); err != nil {
		return nil, fmt.Errorf("loading unrestricted symbols: %w", err)
	}

	// Expose KIT's extension API types so the extension can
	// import "kit/ext" and use ext.ToolCall, ext.API, etc.
	if err := i.Use(Symbols()); err != nil {
		return nil, fmt.Errorf("loading extension symbols: %w", err)
	}

	// Read and evaluate the extension source file.
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading file: %w", err)
	}

	if _, err := i.Eval(string(src)); err != nil {
		return nil, fmt.Errorf("evaluating source: %w", err)
	}

	// Extract the Init function. Extensions must export:
	//   func Init(api ext.API)
	initVal, err := i.Eval("Init")
	if err != nil {
		return nil, fmt.Errorf("no Init function: %w", err)
	}

	initFn, ok := initVal.Interface().(func(API))
	if !ok {
		return nil, fmt.Errorf("init has wrong signature (want func(ext.API), got %T)", initVal.Interface())
	}

	// Build the API object that wires typed registration methods back to
	// the extension's internal handler map. Each method wraps the concrete
	// handler into the internal HandlerFunc type.
	reg := func(event EventType, fn HandlerFunc) {
		ext.Handlers[event] = append(ext.Handlers[event], fn)
	}

	api := API{
		onToolCall: func(h func(ToolCallEvent, Context) *ToolCallResult) {
			reg(ToolCall, func(e Event, c Context) Result {
				r := h(e.(ToolCallEvent), c)
				if r == nil {
					return nil
				}
				return *r
			})
		},
		onToolExecStart: func(h func(ToolExecutionStartEvent, Context)) {
			reg(ToolExecutionStart, func(e Event, c Context) Result {
				h(e.(ToolExecutionStartEvent), c)
				return nil
			})
		},
		onToolExecEnd: func(h func(ToolExecutionEndEvent, Context)) {
			reg(ToolExecutionEnd, func(e Event, c Context) Result {
				h(e.(ToolExecutionEndEvent), c)
				return nil
			})
		},
		onToolResult: func(h func(ToolResultEvent, Context) *ToolResultResult) {
			reg(ToolResult, func(e Event, c Context) Result {
				r := h(e.(ToolResultEvent), c)
				if r == nil {
					return nil
				}
				return *r
			})
		},
		onInput: func(h func(InputEvent, Context) *InputResult) {
			reg(Input, func(e Event, c Context) Result {
				r := h(e.(InputEvent), c)
				if r == nil {
					return nil
				}
				return *r
			})
		},
		onBeforeAgentStart: func(h func(BeforeAgentStartEvent, Context) *BeforeAgentStartResult) {
			reg(BeforeAgentStart, func(e Event, c Context) Result {
				r := h(e.(BeforeAgentStartEvent), c)
				if r == nil {
					return nil
				}
				return *r
			})
		},
		onAgentStart: func(h func(AgentStartEvent, Context)) {
			reg(AgentStart, func(e Event, c Context) Result {
				h(e.(AgentStartEvent), c)
				return nil
			})
		},
		onAgentEnd: func(h func(AgentEndEvent, Context)) {
			reg(AgentEnd, func(e Event, c Context) Result {
				h(e.(AgentEndEvent), c)
				return nil
			})
		},
		onMessageStart: func(h func(MessageStartEvent, Context)) {
			reg(MessageStart, func(e Event, c Context) Result {
				h(e.(MessageStartEvent), c)
				return nil
			})
		},
		onMessageUpdate: func(h func(MessageUpdateEvent, Context)) {
			reg(MessageUpdate, func(e Event, c Context) Result {
				h(e.(MessageUpdateEvent), c)
				return nil
			})
		},
		onMessageEnd: func(h func(MessageEndEvent, Context)) {
			reg(MessageEnd, func(e Event, c Context) Result {
				h(e.(MessageEndEvent), c)
				return nil
			})
		},
		onSessionStart: func(h func(SessionStartEvent, Context)) {
			reg(SessionStart, func(e Event, c Context) Result {
				h(e.(SessionStartEvent), c)
				return nil
			})
		},
		onSessionShutdown: func(h func(SessionShutdownEvent, Context)) {
			reg(SessionShutdown, func(e Event, c Context) Result {
				h(e.(SessionShutdownEvent), c)
				return nil
			})
		},
		registerToolFn: func(tool ToolDef) {
			ext.Tools = append(ext.Tools, tool)
		},
		registerCmdFn: func(cmd CommandDef) {
			ext.Commands = append(ext.Commands, cmd)
		},
	}

	// Call Init — the extension registers its handlers, tools, commands.
	initFn(api)

	return ext, nil
}

// countHandlers returns the total number of registered handlers across all events.
func countHandlers(ext *LoadedExtension) int {
	n := 0
	for _, handlers := range ext.Handlers {
		n += len(handlers)
	}
	return n
}
