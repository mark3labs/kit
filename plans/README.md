# SDK Revamp Plans

## Core Architectural Principle

**The Kit CLI app is the primary consumer of the SDK.**

The SDK is not a thin wrapper for external users. The CLI is built on top of it:

1. `pkg/kit/` defines the canonical API for agents, sessions, events, and hooks
2. `cmd/` parses CLI flags, maps them to `kit.Options`, and calls `kit.New()`
3. `internal/app/` subscribes to SDK events for TUI rendering and uses SDK prompt methods
4. If the app needs a capability, it is added to the SDK first, then consumed by the app
5. External users get the exact same API the CLI uses

### Architecture

```
cmd/kit/main.go
    |
    v
cmd/              Parses flags, maps to kit.Options
    |
    v
pkg/kit/          Canonical SDK: New(), Prompt(), Subscribe(), hooks
    |
    +---> internal/agent/       Agent creation, generation loop
    +---> internal/session/     Session persistence, tree manager
    +---> internal/config/      Config loading, MCP server config
    +---> internal/core/        Built-in tools (read, write, bash, etc.)
    +---> internal/models/      Provider registry, model validation
    +---> internal/auth/        Credential management, OAuth
    +---> internal/compaction/  Context summarization (Plan 07)
    +---> internal/skills/      Skill loading, templates (Plan 08)
    +---> internal/extensions/  Yaegi extension runtime

internal/app/     TUI/interactive mode — subscribes to SDK events
    |
    +---> pkg/kit/              Uses SDK for prompts, sessions, tools
    +---> internal/ui/          Owns BubbleTea rendering only
```

**No circular dependencies.** `pkg/kit/` never imports `cmd/`. `cmd/` imports `pkg/kit/`.

### Before vs After

| Concern | Before (Parallel) | After (SDK-First) |
|---------|-------------------|-------------------|
| Config init | `cmd.InitConfig()` called by both CLI and SDK | `kit.InitConfig()` in `pkg/kit/`, `cmd/` delegates |
| Agent creation | `cmd.SetupAgent()` called by both | `kit.SetupAgent()` in `pkg/kit/`, `cmd/` delegates |
| Session setup | `cmd/root.go` has 80-line if/else chain | `kit.Options{Continue: true}`, SDK handles it |
| Events | 3 parallel systems (SDK callbacks, extension events, TUI msgs) | Single SDK EventBus, TUI bridges via `Subscribe()` |
| Tool exposure | Internal only | `kit.AllTools()`, `kit.NewReadTool(kit.WithWorkDir(...))` |
| Hooks | Only via Yaegi extensions | `kit.OnBeforeToolCall()` — extensions bridge to SDK hooks |

## Plan Execution Order

| Plan | Priority | Description | Depends On |
|------|----------|-------------|------------|
| **00** | P0 | Create `pkg/kit/`, extract init from `cmd/` | None |
| **01** | P0 | Export tools and tool factories | 00 |
| **02** | P0 | Richer type exports (40+ types) | 00 |
| **03** | P1 | Unified event/subscriber system (core done; app/ext bridge deferred) | 00, 02 |
| **04** | P1 | Enhanced session management | 00, 02 |
| **05** | P1 | Additional prompt modes (Steer, FollowUp) | 00, 03 |
| **06** | P2 | Auth & model management APIs | 00, 02 |
| **07** | P2 | Compaction APIs | 00, 03, 04 |
| **08** | P2 | Skills & prompts system | 00, 02 |
| **09** | P3 | Extension hook system | 00, 01, 02, 03 |
| **10** | P4 | App-as-SDK-consumer — complete integration | 00–09 |

### Recommended Batches

**Batch 1 — Foundation** (Plans 00, 01, 02):
Restructure package, expose tools and types. SDK is usable for basic programmatic access. CLI starts delegating to SDK.

**Batch 2 — Rich Interaction** (Plans 03, 04, 05):
Unified events, sessions, prompt modes. App migrates to SDK for event handling and session setup.

**Batch 3 — Management** (Plans 06, 07, 08):
Auth, compaction, skills. CLI commands use SDK functions.

**Batch 4 — Extensibility** (Plan 09):
Hook system with extension bridge. App's extension dispatch routes through SDK hooks.

**Batch 5 — Full Integration** (Plan 10):
CLI uses `kit.New()`, app calls `kit.PromptResult()`, extension events route through SDK EventBus. Closes all deferred items from Plans 03, 05, 09. Removes `AgentRunner` interface, `app.Options.Extensions`, and legacy `executeStep` code.

## Parity with Pi SDK

After all plans:

| Capability | Pi | Kit (After) |
|-----------|-----|-------------|
| Top-level package imports | Yes | `pkg/kit/` |
| Tool exports + factories | Yes | Plan 01 |
| Rich type surface (50+) | Yes | Plan 02 |
| Event subscriber system | Yes | Plan 03 |
| Session management (list/continue/branch) | Yes | Plan 04 |
| Multiple prompt modes | Yes | Plan 05 |
| Auth/model management | Yes | Plan 06 |
| Compaction APIs | Yes | Plan 07 |
| Skills/prompts system | Yes | Plan 08 |
| Extension hooks (20+ events) | Yes | Plan 09 |
| App built on SDK | Yes | Plan 10 (completes deferred work from 03, 05, 09) |
