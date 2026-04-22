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
			continue
		}
		loaded = append(loaded, *ext)
		log.Debug("loaded extension", "path", p, "handlers", countHandlers(ext), "tools", len(ext.Tools), "commands", len(ext.Commands), "tool_renderers", len(ext.ToolRenderers))
	}
	return loaded, nil
}

// pathSet is a thread-safe helper for deduplicating and ordering file paths.
type pathSet struct {
	m    map[string]bool
	list []string
}

func newPathSet() *pathSet {
	return &pathSet{m: make(map[string]bool)}
}

func (ps *pathSet) add(p string) bool {
	abs, err := filepath.Abs(p)
	if err != nil {
		return false
	}
	if ps.m[abs] {
		return false
	}
	ps.m[abs] = true
	ps.list = append(ps.list, abs)
	return true
}

// discoverExtensionPaths returns deduplicated paths to extension files in
// load-order (global first, then project-local, then explicit).
func discoverExtensionPaths(extraPaths []string) []string {
	ps := newPathSet()

	// Global extensions: $XDG_CONFIG_HOME/kit/extensions/ (default ~/.config/kit/extensions/)
	globalDir := globalExtensionsDir()
	for _, p := range findExtensionsInDir(globalDir) {
		ps.add(p)
	}

	// Global installed git packages: $XDG_DATA_HOME/kit/git/
	globalGitDir := globalGitInstallRoot()
	for _, p := range findExtensionsInGitPackages(globalGitDir) {
		ps.add(p)
	}

	// Project-local extensions: .kit/extensions/
	localDir := filepath.Join(".kit", "extensions")
	for _, p := range findExtensionsInDir(localDir) {
		ps.add(p)
	}

	// Project-local installed git packages: .kit/git/
	projectGitDir := filepath.Join(".kit", "git")
	for _, p := range findExtensionsInGitPackages(projectGitDir) {
		ps.add(p)
	}

	// Explicit paths (highest precedence)
	for _, p := range extraPaths {
		info, err := os.Stat(p)
		if err != nil {
			continue
		}
		if info.IsDir() {
			for _, found := range findExtensionsInDir(p) {
				ps.add(found)
			}
		} else if strings.HasSuffix(p, ".go") {
			ps.add(p)
		}
	}

	return ps.list
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
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".go") && !strings.HasSuffix(entry.Name(), "_test.go") {
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

// findExtensionsInRepo scans a git repository for extensions using opinionated conventions.
// Extensions are ONLY recognized in:
//  1. Root-level *.go files
//  2. Files in examples/extensions/ or examples/ext/ subdirectories
//  3. Files in any top-level ext/ directory
//  4. Files in any subdirectory that ends in -ext/ or -extensions/
//
// Everything else (cmd/, internal/, pkg/, etc.) is ignored.
func findExtensionsInRepo(repoPath string) []string {
	var results []string
	multiFileDirs := make(map[string]bool)

	_ = filepath.Walk(repoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, _ := filepath.Rel(repoPath, path)
		relPath = filepath.ToSlash(relPath)

		// Skip directories we know don't contain extensions
		if info.IsDir() {
			switch info.Name() {
			case ".git", ".github", "node_modules", "vendor", "dist", "build":
				return filepath.SkipDir
			}

			// Skip internal code directories
			if strings.HasPrefix(relPath, "internal/") ||
				strings.HasPrefix(relPath, "cmd/") ||
				strings.HasPrefix(relPath, "pkg/") ||
				strings.HasPrefix(relPath, "test/") ||
				strings.HasPrefix(relPath, "tests/") {
				return filepath.SkipDir
			}

			// Root directory - scan it
			if relPath == "." {
				return nil
			}

			base := info.Name()
			isExtDir := base == "extensions" || base == "ext" ||
				strings.HasSuffix(base, "-extensions") || strings.HasSuffix(base, "-ext")

			// Allow walking into examples/ so we can reach examples/extensions/ etc,
			// but don't treat examples/ itself or non-extension subdirs as extension locations.
			if relPath == "examples" {
				return nil
			}

			if !isExtDir {
				mainPath := filepath.Join(path, "main.go")
				if _, err := os.Stat(mainPath); err == nil {
					if relPath == base { // Top-level directory
						if !multiFileDirs[relPath] {
							multiFileDirs[relPath] = true
							results = append(results, mainPath)
						}
						return filepath.SkipDir
					}
				}
				return filepath.SkipDir
			}

			// Check for main.go
			mainPath := filepath.Join(path, "main.go")
			if _, err := os.Stat(mainPath); err == nil {
				if !multiFileDirs[relPath] {
					multiFileDirs[relPath] = true
					results = append(results, mainPath)
				}
				return filepath.SkipDir
			}

			return nil
		}

		// It's a file
		if !strings.HasSuffix(info.Name(), ".go") || strings.HasSuffix(info.Name(), "_test.go") {
			return nil
		}

		if info.Name() == "main.go" {
			return nil
		}

		parentDir := filepath.Dir(relPath)
		if parentDir == "." {
			// Root-level .go file - valid extension
			results = append(results, path)
			return nil
		}

		// Must be in valid extension directory
		isValidExtDir := false
		if strings.HasPrefix(parentDir, "examples/extensions/") ||
			parentDir == "examples/extensions" {
			isValidExtDir = true
		} else if strings.HasPrefix(parentDir, "examples/ext/") ||
			parentDir == "examples/ext" {
			isValidExtDir = true
		} else if strings.HasPrefix(parentDir, "ext/") ||
			parentDir == "ext" {
			isValidExtDir = true
		} else if strings.Contains(parentDir, "-extensions/") ||
			strings.HasSuffix(parentDir, "-extensions") {
			isValidExtDir = true
		} else if strings.Contains(parentDir, "-ext/") ||
			strings.HasSuffix(parentDir, "-ext") {
			isValidExtDir = true
		}

		if !isValidExtDir {
			return nil
		}

		results = append(results, path)
		return nil
	})

	return results
}

// Each git package is stored at <gitRoot>/<host>/<owner>/<repo>/ and can contain
// .go files or a main.go in subdirectories.
// If a package has a manifest with Include field, only those paths are loaded.
func findExtensionsInGitPackages(gitRoot string) []string {
	info, err := os.Stat(gitRoot)
	if err != nil || !info.IsDir() {
		return nil
	}

	var results []string

	// Load the manifest if it exists
	manifestPath := filepath.Join(gitRoot, "packages.json")
	manifest, _ := loadManifestFromPath(manifestPath)
	// Build a map of package identity -> include list
	includeMap := make(map[string][]string)
	if manifest != nil {
		for _, entry := range manifest.Packages {
			if len(entry.Include) > 0 {
				identity := fmt.Sprintf("%s/%s", entry.Host, entry.Path)
				includeMap[identity] = entry.Include
			}
		}
	}

	// Walk through host directories (e.g., github.com/)
	hosts, err := os.ReadDir(gitRoot)
	if err != nil {
		return nil
	}

	for _, host := range hosts {
		if !host.IsDir() {
			continue
		}
		hostPath := filepath.Join(gitRoot, host.Name())

		// Walk through owner directories (e.g., github.com/user/)
		owners, err := os.ReadDir(hostPath)
		if err != nil {
			continue
		}

		for _, owner := range owners {
			if !owner.IsDir() {
				continue
			}
			ownerPath := filepath.Join(hostPath, owner.Name())

			// Walk through repo directories (e.g., github.com/user/repo/)
			repos, err := os.ReadDir(ownerPath)
			if err != nil {
				continue
			}

			for _, repo := range repos {
				if !repo.IsDir() {
					continue
				}
				repoPath := filepath.Join(ownerPath, repo.Name())

				// Check if there's an include filter for this package
				identity := fmt.Sprintf("%s/%s/%s", host.Name(), owner.Name(), repo.Name())
				includes, hasFilter := includeMap[identity]

				if hasFilter {
					// Only include specific paths
					for _, include := range includes {
						// Convert relative path to absolute
						include = strings.TrimPrefix(include, "./")
						fullPath := filepath.Join(repoPath, filepath.FromSlash(include))
						if _, err := os.Stat(fullPath); err == nil {
							results = append(results, fullPath)
						}
					}
				} else {
					// Find all extensions within this repo using convention-based scanning
					results = append(results, findExtensionsInRepo(repoPath)...)
				}
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
		onToolCallInputStart: func(h func(ToolCallInputStartEvent, Context)) {
			reg(ToolCallInputStart, func(e Event, c Context) Result {
				h(e.(ToolCallInputStartEvent), c)
				return nil
			})
		},
		onToolCallInputDelta: func(h func(ToolCallInputDeltaEvent, Context)) {
			reg(ToolCallInputDelta, func(e Event, c Context) Result {
				h(e.(ToolCallInputDeltaEvent), c)
				return nil
			})
		},
		onToolCallInputEnd: func(h func(ToolCallInputEndEvent, Context)) {
			reg(ToolCallInputEnd, func(e Event, c Context) Result {
				h(e.(ToolCallInputEndEvent), c)
				return nil
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
		onToolOutput: func(h func(ToolOutputEvent, Context)) {
			reg(ToolOutput, func(e Event, c Context) Result {
				h(e.(ToolOutputEvent), c)
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
		onModelChange: func(h func(ModelChangeEvent, Context)) {
			reg(ModelChange, func(e Event, c Context) Result {
				h(e.(ModelChangeEvent), c)
				return nil
			})
		},
		onContextPrepare: func(h func(ContextPrepareEvent, Context) *ContextPrepareResult) {
			reg(ContextPrepare, func(e Event, c Context) Result {
				r := h(e.(ContextPrepareEvent), c)
				if r == nil {
					return nil
				}
				return *r
			})
		},
		onBeforeFork: func(h func(BeforeForkEvent, Context) *BeforeForkResult) {
			reg(BeforeFork, func(e Event, c Context) Result {
				r := h(e.(BeforeForkEvent), c)
				if r == nil {
					return nil
				}
				return *r
			})
		},
		onBeforeSessionSwitch: func(h func(BeforeSessionSwitchEvent, Context) *BeforeSessionSwitchResult) {
			reg(BeforeSessionSwitch, func(e Event, c Context) Result {
				r := h(e.(BeforeSessionSwitchEvent), c)
				if r == nil {
					return nil
				}
				return *r
			})
		},
		onBeforeCompact: func(h func(BeforeCompactEvent, Context) *BeforeCompactResult) {
			reg(BeforeCompact, func(e Event, c Context) Result {
				r := h(e.(BeforeCompactEvent), c)
				if r == nil {
					return nil
				}
				return *r
			})
		},
		registerToolFn: func(tool ToolDef) {
			ext.Tools = append(ext.Tools, tool)
		},
		registerCmdFn: func(cmd CommandDef) {
			ext.Commands = append(ext.Commands, cmd)
		},
		registerToolRendererFn: func(config ToolRenderConfig) {
			ext.ToolRenderers = append(ext.ToolRenderers, config)
		},
		registerMessageRendererFn: func(config MessageRendererConfig) {
			ext.MessageRenderers = append(ext.MessageRenderers, config)
		},
		onCustomEvent: func(name string, handler func(string)) {
			if ext.CustomEventHandlers == nil {
				ext.CustomEventHandlers = make(map[string][]func(string))
			}
			ext.CustomEventHandlers[name] = append(ext.CustomEventHandlers[name], handler)
		},
		registerOption: func(opt OptionDef) {
			ext.Options = append(ext.Options, opt)
		},
		registerShortcutFn: func(def ShortcutDef, handler func(Context)) {
			ext.Shortcuts = append(ext.Shortcuts, ShortcutEntry{Def: def, Handler: handler})
		},
		onSubagentStart: func(h func(SubagentStartEvent, Context)) {
			reg(SubagentStart, func(e Event, c Context) Result {
				h(e.(SubagentStartEvent), c)
				return nil
			})
		},
		onSubagentChunk: func(h func(SubagentChunkEvent, Context)) {
			reg(SubagentChunk, func(e Event, c Context) Result {
				h(e.(SubagentChunkEvent), c)
				return nil
			})
		},
		onSubagentEnd: func(h func(SubagentEndEvent, Context)) {
			reg(SubagentEnd, func(e Event, c Context) Result {
				h(e.(SubagentEndEvent), c)
				return nil
			})
		},
		onStepStart: func(h func(StepStartEvent, Context)) {
			reg(StepStart, func(e Event, c Context) Result {
				h(e.(StepStartEvent), c)
				return nil
			})
		},
		onStepFinish: func(h func(StepFinishEvent, Context)) {
			reg(StepFinish, func(e Event, c Context) Result {
				h(e.(StepFinishEvent), c)
				return nil
			})
		},
		onReasoningStart: func(h func(ReasoningStartEvent, Context)) {
			reg(ReasoningStart, func(e Event, c Context) Result {
				h(e.(ReasoningStartEvent), c)
				return nil
			})
		},
		onWarnings: func(h func(WarningsEvent, Context)) {
			reg(Warnings, func(e Event, c Context) Result {
				h(e.(WarningsEvent), c)
				return nil
			})
		},
		onSource: func(h func(SourceEvent, Context)) {
			reg(Source, func(e Event, c Context) Result {
				h(e.(SourceEvent), c)
				return nil
			})
		},
		onError: func(h func(ErrorEvent, Context)) {
			reg(Error, func(e Event, c Context) Result {
				h(e.(ErrorEvent), c)
				return nil
			})
		},
		onRetry: func(h func(RetryEvent, Context)) {
			reg(Retry, func(e Event, c Context) Result {
				h(e.(RetryEvent), c)
				return nil
			})
		},
		onPrepareStep: func(h func(PrepareStepEvent, Context) *PrepareStepResult) {
			reg(PrepareStep, func(e Event, c Context) Result {
				r := h(e.(PrepareStepEvent), c)
				if r == nil {
					return nil
				}
				return *r
			})
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
