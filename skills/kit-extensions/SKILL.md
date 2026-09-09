---
name: kit-extensions
description: Guide for creating Kit extensions. Use when the user asks to build, create, or modify a Kit extension, add a custom tool, slash command, widget, keyboard shortcut, editor interceptor, tool renderer, or hook into any Kit lifecycle event.
---

# Kit Extensions Development Guide

Kit extensions are single-file Go programs interpreted at runtime by Yaegi. They hook into Kit's lifecycle, register custom tools and slash commands, display widgets, intercept editor input, render tool output, register and switch color themes, and more.

Extensions can be distributed via git repositories using `kit install`. Repos can contain single extensions or collections of multiple extensions.

This file is the entry point. It gives the structure, a minimal example, and the constraints you must never miss. Detailed material lives in the `references/` directory (see [Reference files](#reference-files) below). Read only the reference files that apply to your task.

## Extension Structure

Every extension must export a `package main` with an `Init(api ext.API)` function:

```go
//go:build ignore

package main

import "kit/ext"

func Init(api ext.API) {
    // Register event handlers, tools, commands, etc.
}
```

The `//go:build ignore` tag prevents `go build` from compiling the file directly.

## Extension Locations

Extensions are auto-loaded from these directories:

- `/usr/share/kit/extensions/*.go` (system-wide, single files)
- `/usr/share/kit/extensions/*/main.go` (system-wide, subdirectories)
- `~/.config/kit/extensions/*.go` (user, single files)
- `~/.config/kit/extensions/*/main.go` (user, subdirectories)
- `.kit/extensions/*.go` (project-local, single files)
- `.kit/extensions/*/main.go` (project-local, subdirectories)

Or loaded explicitly:

```bash
kit -e path/to/extension.go
kit --extension path/to/extension.go
```

## Import Path

Extensions import the Kit API as `"kit/ext"`. The full standard library is available plus `os/exec` for subprocess spawning.

## API Overview

The `Init` function receives an `ext.API` object for registering handlers, and event handlers receive an `ext.Context` with runtime capabilities.

- **`api.On*`** — subscribe to one of 30 lifecycle events (`OnSessionStart`, `OnToolCall`, `OnAgentEnd`, ...). See `references/lifecycle-events.md`.
- **`api.RegisterTool` / `RegisterCommand` / `RegisterShortcut` / `RegisterOption`** — add LLM tools, `/slash` commands, key bindings, and config options. See `references/tools-commands-shortcuts.md`.
- **`ctx.*`** — runtime capabilities: print output, inject messages, widgets, header/footer, prompts, overlays, editor interceptor, session data/state, model and tool management, LLM completions, themes. See `references/context-api.md`.
- **`api.RegisterToolRenderer` / `api.RegisterMessageRenderer`** — custom rendering of tool calls and messages. See `references/renderers.md`.

## Minimal Working Example

A tool, a slash command, and an event handler in one file:

```go
//go:build ignore

package main

import (
    "time"

    "kit/ext"
)

var toolCalls int // package-level vars hold state across callbacks

func Init(api ext.API) {
    api.RegisterTool(ext.ToolDef{
        Name:        "current_time",
        Description: "Get the current date and time",
        Parameters:  `{"type":"object","properties":{}}`,
        Execute: func(input string) (string, error) {
            return time.Now().Format(time.RFC3339), nil
        },
    })

    api.RegisterCommand(ext.CommandDef{
        Name:        "echo",
        Description: "Echo back the provided text",
        Execute: func(args string, ctx ext.Context) (string, error) {
            ctx.PrintInfo("You said: " + args)
            return "", nil
        },
    })

    api.OnToolCall(func(e ext.ToolCallEvent, ctx ext.Context) *ext.ToolCallResult {
        toolCalls++
        return nil // nil = allow; return &ext.ToolCallResult{Block: true, Reason: "..."} to block
    })
}
```

Run it with `kit -e my-ext.go`. Validate syntax with `kit extensions validate`.

---

## Critical Yaegi Constraints (condensed — never skip)

Yaegi silently miscompiles some valid Go. Failures do not show an error; the code just does nothing. The full version with complete examples is in `references/yaegi-constraints.md`.

### 1. No named function references in struct fields or handler arguments

A named function assigned to a struct field (or passed directly as an argument) returns zero values across the interpreter boundary. Always use anonymous closure literals:

```go
// WRONG - will silently return zero values:
func myHandler(key, text string) ext.EditorKeyAction { ... }
ctx.SetEditor(ext.EditorConfig{HandleKey: myHandler})

// CORRECT - use anonymous closure:
ctx.SetEditor(ext.EditorConfig{
    HandleKey: func(key, text string) ext.EditorKeyAction { return myHandler(key, text) },
})
```

This applies to ALL struct fields that take function values: `ToolDef.Execute`, `CommandDef.Execute`, `EditorConfig.HandleKey`, `EditorConfig.Render`, `ToolRenderConfig.RenderHeader`, `ToolRenderConfig.RenderBody`, etc.

### 2. No comma-separated case lists in a tagless switch

In `switch { case a, b, c: }` Yaegi evaluates **only the first expression**. Join the conditions with `||` into one case expression, or use an `if`/`else` chain. A switch WITH a tag (`switch n { case 1, 2, 3: }`) is fine.

```go
// WRONG - only the first condition is ever checked:
case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':

// CORRECT:
case (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9'):
```

### 3. No interfaces across the boundary

All extension-facing API types are concrete structs, never interfaces. Yaegi crashes on interface wrapper generation.

### 4. Package-level variables for state

Yaegi supports package-level variables captured in closures. This is the standard way to maintain state across event callbacks (`var callCount int` at file scope, then mutate inside handlers).

---

## Other Rules You Must Know

- **Return `nil` to pass through.** Every `On*` handler that returns a `*Result` pointer treats `nil` as "no change". Return a non-nil pointer only to modify or block.
- **Slash commands and shortcut handlers run in their own goroutine.** They can block on `ctx.PromptSelect`, I/O, or `exec.Command` safely.
- **Do not bind `ctrl+c`** as a shortcut (rejected at load). Prefer modifier combinations over bare keys.
- **`e.StopReason == "error"`** is the error check in `OnAgentEnd`. Do NOT compare against `"completed"` for success.
- **Tool `Parameters` is a JSON Schema string.** The `input` argument to `Execute` is the JSON-encoded parameters from the LLM.
- **Use rune counts (`len([]rune(s))`) not byte length** when aligning widget text that contains box-drawing or multi-byte characters.

---

## Reference files

Read the file that matches the task. Paths are relative to this skill's root directory.

| File | Read it when you need... |
|------|--------------------------|
| `references/lifecycle-events.md` | The full list of all 30 events (session, agent turn, tool, tool-call streaming, input, streaming, model, UI, context filtering, session control, custom events), their fields, and return types. |
| `references/tools-commands-shortcuts.md` | Registering tools (including `ExecuteWithContext` with cancellation/progress), slash commands with tab-completion, keyboard shortcuts (key-name normalization and reserved keys), and options. |
| `references/context-api.md` | The complete `ext.Context` API: output, message injection, widgets, header/footer, status bar, prompts, overlays, editor interceptor, terminal size, thinking level, UI visibility, session data/state, model and tool management, LLM completions, TUI suspension, themes, application control, context fields. |
| `references/renderers.md` | Custom tool renderers (`RenderHeader` / `RenderBody`) and message renderers. |
| `references/yaegi-constraints.md` | The full Yaegi constraints section with complete code examples for each pitfall. |
| `references/common-patterns.md` | Recipes: tool call blocking, system prompt injection, background processing with `SendMessage`, ephemeral context injection, live widget updates, custom theme with slash command, spawning Kit as a sub-agent. |
| `references/testing-and-distribution.md` | The `pkg/extensions/test` harness and assertions, CLI testing commands, and distributing extensions via git repositories (`kit install`, repo structure, README template, storage locations). |
| `references/plan-mode-example.md` | A complete, end-to-end extension (Plan Mode) that combines shortcuts, widgets, tool blocking, and state. |
| `references/bridged-sdk-apis.md` | Bridged SDK capabilities: conversation tree navigation, skill loading, template parsing, model resolution, model pricing. |

## Key Files for Reference

- [`internal/extensions/api.go`](https://github.com/mark3labs/kit/blob/main/internal/extensions/api.go) — Complete API type definitions
- [`internal/extensions/runner.go`](https://github.com/mark3labs/kit/blob/main/internal/extensions/runner.go) — Event dispatch and state management
- [`internal/extensions/loader.go`](https://github.com/mark3labs/kit/blob/main/internal/extensions/loader.go) — Yaegi interpreter setup
- [`internal/extensions/symbols.go`](https://github.com/mark3labs/kit/blob/main/internal/extensions/symbols.go) — All types exported to extensions
- [`pkg/extensions/test/`](https://github.com/mark3labs/kit/tree/main/pkg/extensions/test) — Testing package with harness, mocks, and assertions
- [`examples/extensions/tool-logger_test.go`](https://github.com/mark3labs/kit/blob/main/examples/extensions/tool-logger_test.go) — Complete test example
- [`examples/extensions/`](https://github.com/mark3labs/kit/tree/main/examples/extensions) — 25+ working example extensions
