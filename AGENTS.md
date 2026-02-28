<!-- OPENSPEC:START -->
# OpenSpec Instructions

These instructions are for AI assistants working in this project.

Always open `@/openspec/AGENTS.md` when the request:
- Mentions planning or proposals (words like proposal, spec, change, plan)
- Introduces new capabilities, breaking changes, architecture shifts, or big performance/security work
- Sounds ambiguous and you need the authoritative spec before coding

Use `@/openspec/AGENTS.md` to learn:
- How to create and apply change proposals
- Spec format and conventions
- Project structure and guidelines

Keep this managed block so 'openspec update' can refresh the instructions.

<!-- OPENSPEC:END -->

# KIT Agent Guidelines

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

## Key Patterns

### Yaegi (Extension Interpreter) Gotchas
- **No interfaces across boundary**: All extension-facing API types must be concrete structs, never interfaces. Yaegi crashes on interface wrapper generation.
- **Function field bug**: Named function references assigned to struct fields return zero values across the interpreter boundary. Always use anonymous closure literals:
  ```go
  // WRONG: ctx.SetEditor(ext.EditorConfig{HandleKey: myHandler})
  // RIGHT: ctx.SetEditor(ext.EditorConfig{HandleKey: func(k, t string) ext.EditorKeyAction { return myHandler(k, t) }})
  ```
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
kit --prompt "question" --quiet --no-session --no-extensions --system-prompt /path/to/prompt.txt --model provider/model
```
Key flags: `--quiet` (stdout only, no TUI), `--no-session` (ephemeral), `--no-extensions` (prevent recursive loading), `--system-prompt` (string or file path).

## External Repo Research
- **ALWAYS use `btca`** to search external repos (e.g. iteratr, other reference codebases)
- Never guess or manually search the filesystem for external projects
- Example: `btca ask -r https://github.com/user/repo -q "How does X work?"`
- See `.agents/skills/btca-cli/SKILL.md` for full btca usage
