---
name: kit-sdk
description: Guide for building Go applications with the Kit SDK. Use when the user asks to create a program, service, script, or application that uses Kit programmatically as a Go library — e.g. embedding LLM interactions, building agents, creating CLI tools powered by Kit, or integrating Kit into backend services. Do NOT use for Kit extensions (use kit-extensions skill instead).
---

# Kit SDK Development Guide

The Kit SDK (`pkg/kit`) lets you embed Kit's full agent capabilities — LLM interactions, tool execution, session management, streaming, hooks — into any Go application. Unlike extensions (which are interpreted scripts running inside Kit's TUI), SDK programs are standalone compiled Go binaries.

This file is the entry point. It gives the installation, a quick start, the core lifecycle, and the rules you must never miss. Detailed material lives in the `references/` directory (see [Reference files](#reference-files) below). Read only the reference files that apply to your task.

## Installation

```bash
go get github.com/mark3labs/kit
```

Import path (alias recommended):

```go
import kit "github.com/mark3labs/kit/pkg/kit"
```

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"

    kit "github.com/mark3labs/kit/pkg/kit"
)

func main() {
    ctx := context.Background()

    host, err := kit.New(ctx, nil) // nil = load ~/.kit.yml defaults
    if err != nil {
        log.Fatal(err)
    }
    defer func() { _ = host.Close() }()

    response, err := host.Prompt(ctx, "What is 2+2?")
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println(response)
}
```

## Core Lifecycle

1. **Create**: `kit.New(ctx, opts)` — loads config, initializes MCP servers, creates LLM provider, sets up agent
2. **Interact**: `host.Prompt(ctx, msg)` — send messages, agent uses tools as needed
3. **Close**: `host.Close()` — cleans up MCP connections, model resources, session file handle

Always defer `Close()`:

```go
defer func() { _ = host.Close() }()
```

## API Map

| Area | Entry points | Reference |
|------|--------------|-----------|
| Configure the instance | `kit.Options{Model, SystemPrompt, MaxSteps, Tools, ExtraTools, NoSession, ...}` | `references/options.md` |
| Talk to the agent | `Prompt`, `PromptResult`, `PromptResultWithFiles`, `PromptWithOptions`, `Steer`, `FollowUp`, `PromptResultWithMessages` | `references/prompt-methods.md` |
| Observe (read-only) | `host.OnToolCall(...)`, `host.OnMessageUpdate(...)`, `host.Subscribe(...)` | `references/events.md` |
| Intercept (read-write) | `host.OnBeforeToolCall(priority, fn)`, `OnAfterToolResult`, `OnBeforeTurn`, `OnPrepareStep`, `OnContextPrepare`, `OnBeforeCompact` | `references/hooks.md` |
| Add capabilities | `kit.NewTool(name, desc, fn)`, `kit.TextResult`, built-in tool constructors and bundles | `references/tools.md` |
| Persist conversations | `SessionPath`, `Continue`, `NoSession`, instance and package-level session methods | `references/sessions.md` |
| Models and MCP | `host.SetModel`, model registry, `AddMCPServer`, in-process servers, MCP OAuth | `references/models-and-mcp.md` |
| Manage the context window | `EstimateContextTokens`, `Compact`, `AutoCompact`, `host.Subagent` | `references/context-compaction-subagents.md` |

A minimal custom tool, which is the most common extension point:

```go
type WeatherInput struct {
    City string `json:"city" description:"City name, e.g. 'San Francisco'"`
}

weatherTool := kit.NewTool("get_weather", "Get current weather for a city",
    func(ctx context.Context, input WeatherInput) (kit.ToolOutput, error) {
        return kit.TextResult("72°F, sunny in " + input.City), nil
    },
)

host, _ := kit.New(ctx, &kit.Options{
    ExtraTools: []kit.Tool{weatherTool},
})
```

---

## Critical Rules (never skip)

1. **`Tools` replaces, `ExtraTools` adds.** `Options.Tools` replaces ALL default tools (core + MCP + extension). `Options.ExtraTools` adds tools alongside the defaults. Use `Tools` to restrict the agent's capabilities; use `ExtraTools` to extend them.
2. **Register events and hooks before calling `Prompt`.** Subscribers and hooks return an unsubscribe function — `defer unsub()` if the handler must not outlive a scope.
3. **Events are read-only; hooks are read-write.** Use `host.On<Event>` to observe. Use `host.On<Hook>(priority, fn)` to block, modify, or inject. In hooks, return `nil` to allow/pass through; the first non-nil result wins. Lower priority values run first (`kit.HookPriorityHigh = 0`, `Normal = 50`, `Low = 100`).
4. **Pointer fields mean "unset".** Generation parameters like `Temperature`, `TopP`, `TopK` are pointers so an explicit `0.0` differs from "leave the provider default". Zero-value `MaxTokens` auto-resolves; a non-zero value suppresses right-sizing.
5. **Sessions persist automatically** as JSONL tree files. No explicit save call exists or is needed. Use `NoSession: true` for ephemeral, in-memory agents (scripts, subagents, tests).
6. **Context-window fill is not `InputTokens` alone.** With prompt caching, sum `InputTokens + CacheReadTokens + CacheCreationTokens + OutputTokens` from `result.FinalUsage`.
7. **Reactive compaction is always on.** A context-overflow error triggers one compact-and-replay. `AutoCompact: true` adds proactive compaction before turns that near the limit. If the replay still overflows, the turn fails with `kit.ErrContextOverflow`.
8. **MCP OAuth is opt-in.** If `MCPAuthHandler` is nil, remote MCP servers that require OAuth fail with an authorization-required error. They do not open a browser silently.
9. **No dependency-name leakage.** Use the `kit.LLM*` type aliases (`LLMMessage`, `LLMUsage`, ...) and `kit.ConvertToLLM*` helpers. Do not import the underlying LLM library into SDK-facing code.
10. **Configuration precedence** is explicit `ConfigFile` → `.kit.yml` in cwd → `~/.kit.yml` → `KIT_*` env vars → provider env vars (`ANTHROPIC_API_KEY`, ...). `kit.New` handles this; call `kit.InitConfig` only for special cases.

---

## Reference files

Read the file that matches the task. Paths are relative to this skill's root directory.

| File | Read it when you need... |
|------|--------------------------|
| `references/options.md` | Every `kit.Options` field with comments (model, behavior, generation parameters, provider overrides, session, tools, skills, feature toggles, compaction, MCP OAuth, in-process MCP) and the generation/provider cheat-sheet table. |
| `references/prompt-methods.md` | All prompt variants: simple string, full result with usage stats, multimodal file attachments, per-call system message injection, system-level steering, continue without new input, multiple user messages in one turn. |
| `references/events.md` | Typed convenience subscribers, the generic subscriber, the full list of event types with their fields, and tool-kind constants. |
| `references/hooks.md` | Each hook (`BeforeToolCall`, `AfterToolResult`, `BeforeTurn`, `AfterTurn`, `PrepareStep`, `ContextPrepare`, `BeforeCompact`) with result types, plus hook priorities. |
| `references/tools.md` | Creating custom tools with `kit.NewTool` (schema auto-generation, struct tags, output helpers), built-in tool constructors, tool bundles, tool options, using tools in `Options`, querying tools at runtime. |
| `references/sessions.md` | Session modes, instance methods, package-level session operations, and implementing a custom `SessionManager`. |
| `references/models-and-mcp.md` | Model management at creation and runtime, the model registry, model string format, per-model system prompts and generation parameters; dynamic MCP server management, in-process MCP servers, MCP prompts, MCP resources, MCP OAuth authorization and token storage. |
| `references/context-compaction-subagents.md` | Context token estimation, `GetContextStats`, manual and automatic compaction, reactive compaction; in-process subagents, subscribing to subagent events, named agents. |
| `references/extension-api-auth-skills.md` | The `ExtensionAPI` (`kit.Extensions()`), credential management (`auth`), and skill loading/prompt building. |
| `references/types.md` | The full list of re-exported types (`LLMMessage`, `LLMUsage`, tool types, event types, errors, ...). |
| `references/common-patterns.md` | Recipes: scripting/CLI pipe, long-running autonomous agent, streaming output to terminal, multi-turn conversation with memory, tool execution monitoring, guard rails with hooks, parallel subagents, read-only analysis agent. |
| `references/configuration.md` | Config file discovery order, `${ENV_VAR}` expansion, `kit.InitConfig` and `kit.InitConfigWithOptions`. |

---

## Key Files for Reference

- [`pkg/kit/kit.go`](https://github.com/mark3labs/kit/blob/main/pkg/kit/kit.go) — Kit struct, New(), Prompt methods, Subagent, Close
- [`pkg/kit/extension_api.go`](https://github.com/mark3labs/kit/blob/main/pkg/kit/extension_api.go) — ExtensionAPI interface, kit.Extensions() accessor
- [`pkg/kit/types.go`](https://github.com/mark3labs/kit/blob/main/pkg/kit/types.go) — Re-exported types from internal packages
- [`pkg/kit/tools.go`](https://github.com/mark3labs/kit/blob/main/pkg/kit/tools.go) — Tool constructors and bundles
- [`pkg/kit/events.go`](https://github.com/mark3labs/kit/blob/main/pkg/kit/events.go) — Event types, EventBus, typed subscribers
- [`pkg/kit/hooks.go`](https://github.com/mark3labs/kit/blob/main/pkg/kit/hooks.go) — Hook system (BeforeToolCall, AfterToolResult, etc.)
- [`pkg/kit/sessions.go`](https://github.com/mark3labs/kit/blob/main/pkg/kit/sessions.go) — Session management
- [`pkg/kit/compaction.go`](https://github.com/mark3labs/kit/blob/main/pkg/kit/compaction.go) — Context compaction
- [`pkg/kit/models.go`](https://github.com/mark3labs/kit/blob/main/pkg/kit/models.go) — Model registry lookups
- [`pkg/kit/config.go`](https://github.com/mark3labs/kit/blob/main/pkg/kit/config.go) — Config initialization and defaults
- [`pkg/kit/skills.go`](https://github.com/mark3labs/kit/blob/main/pkg/kit/skills.go) — Skills loading and prompt building
- [`pkg/kit/auth.go`](https://github.com/mark3labs/kit/blob/main/pkg/kit/auth.go) — Credential management
- [`examples/sdk/`](https://github.com/mark3labs/kit/tree/main/examples/sdk) — Working example programs
