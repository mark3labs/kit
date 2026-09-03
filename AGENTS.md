# KIT Agent Guidelines

Always talk in ASD-STE100 Simplified Technical English

## Build/Test Commands
- **Build**: `go build -o output/kit ./cmd/kit`
- **Test all**: `go test -race ./...`
- **Test single**: `go test -race ./cmd -run TestScriptExecution`
- **Lint**: `go vet ./...`
- **Format**: `go fmt ./...`

## Code Style
- **Imports**: stdlib → third-party → local (blank lines between)
- **Naming**: camelCase (unexported), PascalCase (exported)
- **Errors**: Always check, wrap with `fmt.Errorf("context: %w", err)`
- **Logging**: Use `github.com/charmbracelet/log` structured logging
- **Types**: Prefer `any` over `interface{}`
- **JSON**: snake_case tags with `omitempty` where appropriate
- **Context**: First parameter for blocking operations

## Architecture
- Multi-provider LLM support via `llm.Provider` interface
- MCP client-server for tool integration
- Builtin servers: bash, fetch, todo, fs
- **Extension system** (`internal/extensions/`): Yaegi-interpreted Go, 13 lifecycle events, custom tools/commands/widgets/overlays/editor interceptors
- **TUI** (`internal/ui/`): Bubble Tea v2 parent-child model (`AppModel` → `InputComponent`, `StreamComponent`, etc.)
- **Decoupling pattern**: `cmd/root.go` has converter functions (e.g. `widgetProviderForUI()`) that bridge `internal/extensions/` types to `internal/ui/` types — the UI never imports extensions directly
- **Public SDK** (`pkg/kit/`): The public-facing Go SDK for embedding Kit as a library. See rules below.

## Public SDK (`pkg/kit/`) Rules

`pkg/kit/` is the **public API surface** consumed by external Go developers. All exported symbols, types, function names, and godoc comments in this package are part of the SDK contract.

### No Dependency Name Leakage
Internal dependency names (e.g. `charm.land/fantasy`, library-specific jargon) **must not** appear in:
- **Exported function/method names** — use generic terms (`LLM`, `Provider`, `Message`) instead of library names
- **Exported type names** — type aliases should use domain names (e.g. `LLMMessage`, not `FantasyMessage`)
- **Godoc comments** on exported symbols — these are visible in `go doc` output and pkg.go.dev
- **Struct field names and tags** on exported types

Using dependency types directly in **function bodies** (private implementation) is fine — that's invisible to SDK consumers.

### Naming Conventions for SDK Symbols
- Type aliases re-exporting dependency types: use `LLM*` prefix (e.g. `LLMMessage`, `LLMUsage`, `LLMResponse`)
- Conversion helpers: use `ConvertToLLM*` / `ConvertFromLLM*` (not the dependency name)
- Provider queries: use `GetLLMProviders` (not `GetFantasyProviders`)
- When wrapping internal methods, the `pkg/kit/` name should be dependency-agnostic even if the `internal/` method still uses the old name

### Deprecation Pattern
When renaming a public SDK symbol, keep the old name as a deprecated wrapper for one release cycle:
```go
// Deprecated: Use NewName instead.
func OldName() { return NewName() }
```

## Key Patterns

### Yaegi (Extension Interpreter) Gotchas
- **No interfaces synthesized at runtime**: Yaegi crashes (`genInterfaceWrapper` nil deref) when an interpreted type must satisfy a host interface and no wrapper exists. Hand-written `interp.Exports` therefore exposes only concrete structs. Note: `yaegi extract` CAN generate working `_Iface` wrappers at build time, and the export key must match the type's real Go package path — so this is a solvable constraint, not an absolute one.
- **Forward-reference function bug**: referencing a func BY NAME from inside a function literal, where the func is declared LATER in the file, yields a callable that does nothing and returns zero values — silently. Affects struct fields AND direct args (`api.OnToolCall(fn)`). Fix by declaring helpers above their use, wrapping in a closure, or assigning to a local first:
  ```go
  // BROKEN: helper declared after this closure
  api.OnAgentStart(func(e ext.AgentStartEvent, ctx ext.Context) {
      ctx.SetEditor(ext.EditorConfig{HandleKey: myHandler})
  })
  func myHandler(k, t string) ext.EditorKeyAction { ... }

  // RIGHT: declare myHandler above, or wrap:
  // HandleKey: func(k, t string) ext.EditorKeyAction { return myHandler(k, t) }
  ```
- **Tagless-switch comma case lists are miscompiled**: in `switch { case a, b, c: }` Yaegi evaluates ONLY the first expression and silently ignores the rest — no error, no panic. Bit both `status-footer.go` (CJK counted as 1 column) and `popup-terminal.go` (digits stripped from session names). A switch WITH a tag (`switch n { case 1, 2, 3: }`) is fine.
  ```go
  // BROKEN: only `r >= 'a' && r <= 'z'` is ever evaluated
  switch {
  case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':

  // RIGHT: join with || into ONE case expression
  switch {
  case (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9'):
  ```
  Regression test: `pkg/extensions/test/tagless_switch_yaegi_test.go` (fails loudly if Yaegi ever fixes it).
- **Symbol exports**: Every new type exposed to extensions must be added to `internal/extensions/symbols.go`

### BubbleTea Integration
- **No `prog.Send()` from inside `Update()`**: Calling `prog.Send()` synchronously within a BubbleTea `Update()` handler deadlocks the event loop. Use `go appInstance.NotifyWidgetUpdate()` (async goroutine) instead.
- **Height measurement**: `distributeHeight()` in `model.go` must measure using the same render path as `View()`. If an interceptor wraps rendering, measure with the wrapper too, or layout will mismatch.
- **Channel-based prompts**: Extension prompt calls (PromptSelect, etc.) block on a `chan PromptResponse`. Extension slash commands run in dedicated goroutines (not `tea.Cmd`) to avoid stalling BubbleTea's Cmd scheduler.

### Extension State Management
- **Thread-safe maps on Runner**: Widget/header/footer/editor state lives on the Runner with `sync.RWMutex`, queried by UI via callbacks
- **Context function fields**: The `Context` struct uses function fields (`Print func(string)`, `SetWidget func(WidgetConfig)`) wired by closures in `cmd/root.go`
- **Package-level vars in extensions**: Yaegi supports package-level variables captured in closures — this is how extensions maintain state across event callbacks

### Unicode in Widget Text
- Widget content renders through `lipgloss.Style.Render()` which preserves ANSI escape codes
- Use rune-based width calculations (`len([]rune(s))`) not byte length (`len(s)`) when aligning box-drawing characters or multi-byte symbols

## Testing

### Interactive TUI Testing with tmux
Use tmux to test Kit interactively without blocking the agent:
```bash
tmux new-session -d -s kittest -x 120 -y 40 "output/kit -e examples/extensions/my-ext.go --no-session 2>kit_stderr.log"
sleep 3
tmux capture-pane -t kittest -p          # read screen
tmux send-keys -t kittest '/command' Enter  # send input
tmux kill-session -t kittest              # cleanup
```

### Non-Interactive Kit (Subprocess Spawning)
Extensions can spawn Kit as a subprocess for sub-agent patterns:
```bash
kit --quiet --no-session --no-extensions --system-prompt /path/to/prompt.txt --model provider/model "question"
```
Positional args are the prompt. `@file` args attach file content. Key flags: `--quiet` (stdout only, no TUI), `--no-session` (ephemeral), `--no-extensions` (prevent recursive loading), `--system-prompt` (string or file path).

## External Repo Research
- **ALWAYS use `btca`** to search external repos (e.g. iteratr, other reference codebases)
- Never guess or manually search the filesystem for external projects
- Example: `btca ask -r https://github.com/user/repo -q "How does X work?"`
- See `.agents/skills/btca-cli/SKILL.md` for full btca usage

## BTCA Configured Resources
The following external repositories are configured in `btca.config.jsonc` for research:

- bubbletea
- lipgloss
- bubbles
- glamour
- fantasy
- catwalk
- crush
- pi
- iteratr
- yaegi
- acp-go-sdk
- opencode
- herald
- herald-md
