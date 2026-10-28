---
title: SDK Options
description: Configuration options for the Kit Go SDK.
ogImage: /og-image.png
---

# SDK Options

Pass an `Options` struct to `kit.New()` to configure the Kit instance.

> **Tip:** for simple setups, `kit.NewAgent(ctx, ...Option)` provides
> functional-options helpers (`WithModel`, `WithStreaming`, `Ephemeral`, ...)
> over the same `Options` struct. See
> [Functional options](/sdk/overview#functional-options-newagent).
>
> To embed Kit with no config, context files, skills, extensions, named
> agents, session files or core tools from the host, use
> `kit.NewIsolatedAgent`. It sets the fields below for you. See
> [Isolated agents](/sdk/overview#isolated-agents).

Each `kit.New` / `kit.NewAgent` call owns an isolated configuration store, so
these options never leak between Kit instances in the same process. See
[Per-instance config isolation](/sdk/overview#per-instance-config-isolation).


## HTTP request identity

Kit-controlled HTTP requests use `User-Agent: KIT` by default. Set a per-instance
identity with either API:

```go
host, err := kit.New(ctx, &kit.Options{UserAgent: "MyApp/1.0"})
// Or use functional options:
host, err = kit.NewAgent(ctx, kit.WithUserAgent("MyApp/1.0"))
```

Empty means `KIT`, not an opt-out. Valid values are preserved without trimming,
including spaces. HTTP control characters (including CR, LF, NUL and DEL) are
rejected before construction or login does network work. This setting has no
CLI flag, environment variable, or config-file key.

Each instance keeps its own identity. It passes to model changes, temporary
completions and subagents, including isolated/bare children and provider changes.
It covers built-in model requests (including streams), Ollama setup, remote MCP
and their OAuth requests, and provider token refresh. The identity replaces
conflicting user-agent headers from model defaults, catalog headers, per-call
headers and MCP configuration, case-insensitively. Other headers stay unchanged.
Custom provider factories receive `ProviderConfig.UserAgent` and must enforce it
on their own requests; Kit does not intercept arbitrary factory traffic.

Two inference paths keep their fixed compatibility identities, even with an
SDK override. Their token endpoints still use the configured Kit identity.
These exceptions follow existing source requirements; they were not live-tested.

| Inference path | Fixed user agent |
| --- | --- |
| Codex | `Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36` |
| Copilot | `GitHubCopilotChat/0.31.0` |

Standalone `kit.LoginProvider` has a separate `kit.LoginOptions.UserAgent` field
(default `KIT`), independent of any Kit instance or supplied credential manager.
Standalone CLI OAuth and model-catalog downloads use `KIT`.

This policy does not change global HTTP defaults. It does not cover subprocesses
(shell, git/gh/curl, installers, npm or MCP server processes), extension/application
code, browser navigation, or remote services acting for Kit. Third-party cloud
credential discovery (metadata services or default credential chains) inherits
the identity only when it uses the HTTP client Kit supplies; model endpoint
requests use the configured identity. A custom transport can rewrite headers
after Kit's boundary and is responsible for its own policy.

## Full options reference

```go
host, err := kit.New(ctx, &kit.Options{
    // Model
    Model:        "ollama/llama3",
    SystemPrompt: "You are a helpful bot",
    ConfigFile:   "/path/to/config.yml",

    // Behavior
    MaxSteps:     10,
    Streaming:    ptrBool(true), // *bool: nil = unset (default true), &false = off
    Quiet:        true,
    Debug:        true,
    DebugLogger:  myLogger,       // optional; overrides Debug + built-in logger when non-nil

    // Generation parameters (override env/config/per-model defaults)
    MaxTokens:        16384,              // 0 = auto-resolve; non-zero suppresses right-sizing
    ThinkingLevel:    "medium",           // model-specific; see SupportedThinkingLevels
    Temperature:      ptrFloat32(0.2),    // pointer so explicit 0.0 != unset
    TopP:             nil,                 // nil = provider/per-model default
    TopK:             nil,
    FrequencyPenalty: nil,
    PresencePenalty:  nil,

    // Provider configuration
    ProviderAPIKey: "sk-...",                      // "" = use config / provider env var
    ProviderURL:    "https://proxy.internal/v1",  // "" = provider default endpoint
    ProviderWire:   "anthropic",                  // "" = infer wire from model database
    TLSSkipVerify:  false,                         // only effective when true
    Providers: map[string]kit.ProviderFactory{    // app-supplied backends for "local/<model>"
        "local": localFactory,
    },

    // Session
    SessionPath:  "./session.jsonl",
    SessionDir:   "/home/user/project", // project identity for discovery, not storage root
    Continue:     true,
    NoSession:    true,

    // Tools
    Tools:            []kit.Tool{...},     // Replace default core tools; MCP and extension tools remain
    ExtraTools:       []kit.Tool{...},     // Add tools alongside defaults (mutate at runtime via host.AddTools/RemoveTools)
    DisableCoreTools: true,                // Use no core tools (0 tools, for chat-only)
    CoreToolList:     []string,            // List of core tools to include, if empty (default) include all
    WorkDir:          "/path/to/project",  // Base directory of the file and shell tools (default: process cwd)
    CoreToolOptions:  []kit.ToolOption{...}, // Extra options for the core tools, e.g. kit.WithFileSystem(fs)

    // Configuration
    SkipConfig:   true,                   // Skip .kit.yml files (viper defaults + env vars still apply)

    // Compaction
    AutoCompact:  true,

    // Skills
    Skills:        []string{"/path/to/skill.md"},
    SkillsDir:     "/path/to/skills/",
    SkillsDisable: []string{"noisy-skill"},
    NoSkills:      true,

    // Feature toggles
    NoExtensions:   true,               // disable Yaegi extension loading
    NoContextFiles: true,               // disable automatic AGENTS.md loading
    NoAgents:       true,               // disable named agent discovery (.agents/agents/, .kit/agents/, ~/.config/kit/agents/, and built-ins)

    // Session (advanced)
    SessionManager: myCustomSession,    // custom SessionManager implementation

    // MCP OAuth — both opt-in. Leave MCPAuthHandler nil to disable
    // OAuth entirely (remote MCP 401s bubble up as errors). CLI apps
    // pass kit.NewCLIMCPAuthHandler(); custom UX embedders implement
    // MCPAuthHandler or configure DefaultMCPAuthHandler + OnAuthURL.
    MCPAuthHandler: authHandler,                  // nil = OAuth disabled
    MCPTokenStoreFactory: func(serverURL string) (kit.MCPTokenStore, error) {
        return myStore(serverURL), nil
    },

    // In-Process MCP Servers
    InProcessMCPServers: map[string]*kit.MCPServer{
        "docs": mcpSrv,  // *server.MCPServer from mcp-go
    },
})
```

## Options fields

### Core

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `Model` | `string` | config default | Model string (provider/model format) |
| `DecisionModel` | `string` | config default | Default [decision model](/sdk/decisions) for `Kit.Decide` and extension `ctx.Decide` when a request names no model |
| `SystemPrompt` | `string` | — | System prompt text or file path |
| `ConfigFile` | `string` | empty | Explicit path; when unset, discover project config before home config |
| `MaxSteps` | `int` | `0` | Max agent steps (0 = use the configured default) |
| `Streaming` | `*bool` | `nil` | Enable streaming output. `nil` leaves it to the precedence chain (env → config → default `true`); `&true`/`&false` forces it. Pointer so unset is distinct from explicit `false`. |
| `Quiet` | `bool` | `false` | Suppress output |
| `Debug` | `bool` | `false` | Enable debug logging via the built-in console / buffered logger. Ignored when `DebugLogger` is non-nil. |
| `DebugLogger` | `DebugLogger` | `nil` | Caller-supplied logger that receives low-level engine + MCP tool plumbing debug output. When non-nil this overrides `Debug` — the supplied logger's `IsDebugEnabled()` controls downstream emission. See [Custom debug logger](#custom-debug-logger). |

### Generation parameters

These fields override the corresponding values from `.kit.yml` / `KIT_*`
environment variables. Leaving a field at its zero/nil value lets the
precedence chain resolve a value (`KIT_*` env → config file → per-model
defaults from `modelSettings`/`customModels` → an 8192 SDK floor for
`MaxTokens` (matching the CLI `--max-tokens` default) and provider-level
defaults for samplers).

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `MaxTokens` | `int` | auto-resolved | Max output tokens per response. `0` = auto-resolve; non-zero suppresses automatic right-sizing (same semantics as `--max-tokens`). |
| `ThinkingLevel` | `string` | auto-resolved | Case-sensitive, model-specific effort. `off` omits the field; `none` is literal. `""` falls through to config/env/per-model/`"off"`. Explicit unsupported selections return an error; use [`SupportedThinkingLevels`](/sdk/overview#reasoning-support-per-model) for catalog choices (which may include future names). |
| `Temperature` | `*float32` | — | Sampling randomness. Pointer type so explicit `0.0` is distinguishable from "unset". |
| `TopP` | `*float32` | — | Nucleus sampling cutoff. `nil` leaves provider/per-model default. |
| `TopK` | `*int32` | — | Top-K sampling limit. `nil` leaves provider/per-model default. |
| `FrequencyPenalty` | `*float32` | — | OpenAI-family frequency penalty. `nil` leaves provider default. |
| `PresencePenalty` | `*float32` | — | OpenAI-family presence penalty. `nil` leaves provider default. |

Pointer-typed fields (`Streaming` and the samplers) are populated via tiny helpers:

```go
func ptrBool(v bool) *bool          { return &v }
func ptrFloat32(v float32) *float32 { return &v }
```

These fields eliminate the need for `viper.Set()` calls before `kit.New()`
when embedding Kit as a library.

### Provider configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `ProviderAPIKey` | `string` | — | API key used to authenticate with the provider. `""` falls back to config / provider-specific env var (e.g. `ANTHROPIC_API_KEY`). When set, it takes precedence over config and env values on this instance's store. |
| `ProviderURL` | `string` | — | Override the provider endpoint (e.g. LiteLLM, vLLM, Azure OpenAI, internal proxy). `""` = provider default. |
| `ProviderWire` | `string` | — | Override the wire protocol for auto-routed providers: `openai` (Responses API), `openai-compat` (chat completions), `anthropic`, or `google`. `""` = infer from the model database. Takes precedence over per-provider `wire` declarations in the [`providers` config section](/configuration#provider-overrides). Combine with `ProviderURL` to target providers not in the database. |
| `TLSSkipVerify` | `bool` | `false` | Disable TLS certificate verification on the provider HTTP client. Only effective when `true`; to force-disable, use config file or env var instead. For self-signed dev certs only. |
| `Providers` | `map[string]ProviderFactory` | — | Provider factories for this instance, keyed by provider name (case-insensitive, no `/`). A model string `name/<model>` is built by the factory. Wins over `kit.RegisterProvider` and the built-in providers; subagents inherit it. `kit.New` copies the map and rejects nil factories and names that differ only in case. See [Custom provider backends](/sdk/overview#custom-provider-backends). |
| `DecisionProviders` | `map[string]DecisionProviderFactory` | — | Decision provider factories for this instance, with the same name rules as `Providers`. `Kit.Decide` (and extension `ctx.Decide`) sends requests for `name/<model>` to the factory model instead of the model database. Wins over `kit.RegisterDecisionProvider`; subagents inherit it. See [custom decision providers](/sdk/decisions#custom-decision-providers). |

### Session

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `SessionPath` | `string` | — | Open a specific session file |
| `SessionDir` | `string` | — | Base directory for session discovery |
| `Continue` | `bool` | `false` | Resume most recent session |
| `NoSession` | `bool` | `false` | Ephemeral mode (no persistence) |
| `SessionManager` | `SessionManager` | — | Custom session backend (advanced) |

### Tools & extensions

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `Tools` | `[]Tool` | — | Replace default core tools; MCP and extension tools remain |
| `ExtraTools` | `[]Tool` | — | Additional tools alongside core/MCP/extension tools |
| `DisableCoreTools` | `bool` | `false` | Use no core tools (0 tools, for chat-only) |
| `CoreToolList` | `[]string` | — | Allow-list of core tool names; empty/nil means all. Build with [`FilterCoreToolNames`](/sdk/overview#filtering-core-tools) from include/exclude filters. |
| `WorkDir` | `string` | process cwd | Directory the core file and shell tools resolve relative paths against and run commands in. Subagents inherit it. |
| `CoreToolOptions` | `[]ToolOption` | — | Extra options for the core tools, applied last. Use [`WithFileSystem`](#routing-file-access-and-commands) and [`WithCommandRunner`](#routing-file-access-and-commands) here. Subagents inherit them. |
| `NoExtensions` | `bool` | `false` | Disable Yaegi extension loading |
| `NoContextFiles` | `bool` | `false` | Disable automatic AGENTS.md loading |
| `NoAgents` | `bool` | `false` | Disable named agent discovery (built-ins and `.agents/agents/` / `.kit/agents/` / `~/.config/kit/agents/` files); see [Subagents](/advanced/subagents#named-agents) |

Unless `NoExtensions` is set, an embedded Kit uses the same
[auto-discovery paths](/extensions/loading#auto-discovery) as the CLI, which
include the system-wide `/usr/share/kit/extensions` directory. Set
`NoExtensions: true` for a hermetic embed that must not pick up extensions
from the host machine. [`kit.Isolated()`](/sdk/overview#isolated-agents)
sets it together with the other discovery switches.

### Skills & configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `SkipConfig` | `bool` | `false` | Skip `.kit.yml` file loading (viper defaults + env vars still apply) |
| `Skills` | `[]string` | — | Explicit skill files/dirs to load |
| `SkillsDir` | `string` | — | Scan this directory directly for skills (overrides auto-discovery; scanned as-is) |
| `SkillsDisable` | `[]string` | — | Skill names to hide from the model catalog (still usable via `/<name>`) |
| `SkillTrustPrompt` | `func(projectDir string, skillCount int) TrustDecision` | `nil` | Callback gating project-local skill loading on a trust decision (see below) |
| `NoSkills` | `bool` | `false` | Disable skill loading entirely |

#### Project-skill trust gate

Project-local skills (under `<project>/.agents/skills/` or `<project>/.kit/skills/`)
are injected into the system prompt, so loading them from an untrusted, freshly
cloned repository is a prompt-injection vector. Set `SkillTrustPrompt` to gate
that first load on an explicit decision. When `nil` (the default), project
skills load without prompting — preserving historical behaviour.

```go
opts := &kit.Options{
    SkillTrustPrompt: func(projectDir string, skillCount int) kit.TrustDecision {
        // Consult your own UI / policy here.
        if userApproves(projectDir, skillCount) {
            return kit.TrustProject     // load and persist the directory as trusted
        }
        return kit.SkipProjectSkills    // do not load project skills
    },
}
```

The callback returns one of three `TrustDecision` values:

| Decision | Effect |
|----------|--------|
| `kit.TrustProject` | Load project skills and persist `projectDir` to `~/.config/kit/trusted-projects.json` (not prompted again) |
| `kit.TrustProjectOnce` | Load project skills for this run only, without persisting |
| `kit.SkipProjectSkills` | Do not load project skills |

A directory already on the persisted allowlist is trusted without invoking the
callback. The Kit CLI wires this to an interactive terminal prompt automatically
for TTY sessions.

These fields only control the **initial** skill and context-file set picked
up by `New()`. To add, remove, or replace skills and `AGENTS.md`-style
context files at runtime (e.g. per user or per session), use the
`AddSkill` / `LoadAndAddSkill` / `RemoveSkill` / `SetSkills` /
`DisableSkill` / `EnableSkill` and
`AddContextFile` / `AddContextFileContent` / `RemoveContextFile` /
`SetContextFiles` methods on `*kit.Kit`. See
[Runtime skills and context files](/sdk/overview#runtime-skills-and-context-files).

### Compaction & MCP

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `AutoCompact` | `bool` | `false` | Compact proactively before turns that near the context limit. Independent of this setting, Kit always compacts **reactively** and replays the turn once when a provider call fails with a context-overflow error. |
| `CompactionOptions` | `*CompactionOptions` | — | Configuration for compaction; zero-value budget fields adapt to the model's limits. See [CompactionOptions](#compactionoptions) below. |
| `MCPConfig` | `*Config` | — | Pre-loaded MCP configuration. When set, `LoadAndValidateConfig` is skipped during Kit creation — useful for in-process subagents (inheriting a parent's loaded config) and programmatic setups that build config without reading `.kit.yml`. |
| `MCPAuthHandler` | `MCPAuthHandler` | — | OAuth handler for remote MCP servers. `nil` disables OAuth (servers returning 401 fail with the authorization-required error). See [MCP OAuth](#mcp-oauth-authorization) below. |
| `MCPTokenStoreFactory` | `func` | — | Custom OAuth token storage for MCP servers (default: JSON file in `$XDG_CONFIG_HOME/.kit/mcp_tokens.json`). |
| `InProcessMCPServers` | `map[string]*MCPServer` | — | In-process mcp-go servers (no subprocess) |
| `MCPTaskMode` | `map[string]MCPTaskMode` | — | Per-server override for task-augmented `tools/call`. Keys are server names; missing entries fall back to the `tasksMode` field of the matching `MCPServerConfig`. See [MCP Tasks](#mcp-tasks). |
| `MCPTaskTimeout` | `time.Duration` | `15m` | Maximum wall-clock to wait for a task to reach a terminal state. Independent of any per-call context deadline. |
| `MCPTaskTTL` | `time.Duration` | — | TTL hint sent in `TaskParams` for every task-augmented call. Zero omits the field and lets the server pick. |
| `MCPTaskPollInterval` | `time.Duration` | `1s` | Fallback interval between `tasks/get` requests when the server does not suggest one. |
| `MCPTaskMaxPollInterval` | `time.Duration` | `5s` | Cap on the polling interval (a server-supplied `pollInterval` can otherwise grow without bound). |
| `MCPTaskProgress` | `MCPTaskProgressHandler` | — | Optional callback invoked once when a task is accepted and on every observed status transition. The final invocation always carries a terminal status. |

### CompactionOptions

`CompactionOptions` controls how conversations are summarized. The token
budgets are adaptive: leave a field at zero and it scales to the model's
limits, or set an explicit value to override.

| Field | Type | Zero-value behaviour | Description |
|-------|------|----------------------|-------------|
| `ContextWindow` | `int` | auto-populated from the model registry | Model's context window size in tokens |
| `MaxOutputTokens` | `int` | auto-populated from the model registry | Model's max output tokens; scales the adaptive `ReserveTokens` |
| `ReserveTokens` | `int` | `min(16384, MaxOutputTokens)` | Tokens reserved for the LLM response |
| `KeepRecentTokens` | `int` | a quarter of the usable context (`ContextWindow − ReserveTokens`), floored at 2000 | Recent tokens preserved verbatim (not summarized) |
| `SummaryPrompt` | `string` | built-in structured checkpoint prompt | Custom summarization prompt |

When a session compacts more than once, the previous summary is fed back into
the summarization prompt and updated incrementally (anchored summaries), so
detail is preserved across compaction generations.

#### Reactive compaction on context overflow

Token estimates inevitably drift from real tokenizer counts, and a single
huge mid-turn tool result can overflow the context even when the turn started
well under the limit. When the provider rejects a request with a
context-overflow error, Kit automatically:

1. Compacts the conversation (persisting any completed steps first, so the
   replay resumes rather than restarts).
2. Replays the turn once with the rebuilt context, replacing media
   attachments with text placeholders (media cannot be shrunk by
   summarization).
3. If the replay still overflows, fails the turn with `kit.ErrContextOverflow`
   ("conversation too large to compact").

This safety net is always on — it does not require `AutoCompact`, which only
controls the *proactive* check before each turn.

## MCP OAuth Authorization

When a remote MCP server (SSE or Streamable HTTP) requires OAuth, Kit runs
the full authorization flow (dynamic client registration → PKCE → user
consent → token exchange → token persistence) but delegates the **user-facing
step** — displaying the authorization URL and receiving the callback — to
an `MCPAuthHandler`.

The SDK is deliberately inert when `MCPAuthHandler` is `nil`: it does **not**
auto-construct a default handler, bind a local TCP port, or open a browser.
This keeps library, daemon, and web-app embedders free of surprise I/O.
Consumers opt in by passing a handler explicitly.

| Building block | When to use |
|---|---|
| `MCPAuthHandler = nil` (default) | OAuth disabled. Remote MCP servers requiring auth fail with a clear error. Correct for libraries, daemons, and web apps. |
| `kit.NewCLIMCPAuthHandler()` | CLI/TUI apps. Opens the system browser, prints status to stderr (or via `NotifyFunc`), runs a localhost callback server. Used by the `kit` binary. |
| `kit.NewDefaultMCPAuthHandler()` + `OnAuthURL` | Custom UX. Use the SDK's port reservation and callback server; plug in your own presentation via the `OnAuthURL(serverName, authURL)` closure. |
| Implement `kit.MCPAuthHandler` directly | Full control. No localhost binding — e.g. return the URL from an HTTP endpoint and have the consumer POST the callback URL back. |

**CLI-style embedder:**

```go
authHandler, err := kit.NewCLIMCPAuthHandler()
if err != nil {
    log.Fatal(err)
}
defer authHandler.Close() // release the reserved port

host, _ := kit.New(ctx, &kit.Options{
    MCPAuthHandler: authHandler,
})
```

**Custom UX embedder (TUI modal, QR code, web redirect, etc.):**

```go
authHandler, _ := kit.NewDefaultMCPAuthHandler()
authHandler.OnAuthURL = func(serverName, authURL string) {
    // No browser or terminal assumptions — render however you like.
    myUI.ShowAuthPrompt(serverName, authURL)
}
defer authHandler.Close()

host, _ := kit.New(ctx, &kit.Options{
    MCPAuthHandler: authHandler,
})
```

**Fully custom handler (no local port binding at all):**

```go
type WebAuthHandler struct {
    redirectURI string
    callbacks   chan string
}

func (h *WebAuthHandler) RedirectURI() string { return h.redirectURI }

func (h *WebAuthHandler) HandleAuth(ctx context.Context, serverName, authURL string) (string, error) {
    // Push the URL to the user's existing browser session via your web app,
    // then block on the callback that your HTTP handler pushes onto the channel.
    h.pushToUserSession(serverName, authURL)
    select {
    case callbackURL := <-h.callbacks:
        return callbackURL, nil
    case <-ctx.Done():
        return "", ctx.Err()
    }
}
```

> **Warning:** `DefaultMCPAuthHandler` with no `OnAuthURL` set will silently
> drop the authorization URL and hang until the 2-minute callback timeout
> fires. Always set `OnAuthURL`, or use a higher-level wrapper like
> `CLIMCPAuthHandler`.

## MCP Tasks

The [MCP Tasks utility](https://modelcontextprotocol.io/specification/2025-11-25/basic/utilities/tasks)
turns a synchronous `tools/call` into a pollable async job: the server
returns a `taskId` with status `working` immediately, and the client polls
`tasks/get` / `tasks/result` until the task reaches a terminal state.

Kit advertises task support during `initialize` and, by default, augments
`tools/call` with task metadata only when the server advertises
`tasks/toolCalls` capability — so any existing MCP server keeps its previous
synchronous behaviour bit-for-bit. Long-running tools (builds, deployments,
batch jobs, sub-agent runs) get HTTP/SSE timeout-resistance and clean
cancellation "for free" once both sides opt in.

### Per-server mode

```go
import "time"

host, _ := kit.New(ctx, &kit.Options{
    MCPTaskMode: map[string]kit.MCPTaskMode{
        "build-server": kit.MCPTaskModeAlways, // force task-augmented calls
        "chat-server":  kit.MCPTaskModeNever,  // force synchronous calls
        // any server not in the map honours its `tasksMode` config field
        // (default "auto")
    },
})
```

| Mode | Behaviour |
|---|---|
| `MCPTaskModeAuto` (default) | Augment `tools/call` with `TaskParams` only when the server advertised `tasks/toolCalls`. |
| `MCPTaskModeNever` | Always issue `tools/call` synchronously, ignoring server capability. |
| `MCPTaskModeAlways` | Always opt in, even when the server didn't advertise the capability. The server may still respond synchronously. |

### Progress callbacks

```go
host, _ := kit.New(ctx, &kit.Options{
    MCPTaskTimeout:  15 * time.Minute,        // total wall-clock cap
    MCPTaskTTL:      30 * time.Minute,        // server retention hint
    MCPTaskProgress: func(p kit.MCPTaskProgress) {
        log.Printf("%s/%s: %s %s", p.Server, p.TaskID, p.Status, p.Message)
    },
})
```

The handler fires once when a task is accepted and again on every observed
status transition. The final call always carries a terminal status
(`MCPTaskStatusCompleted`, `MCPTaskStatusFailed`, or `MCPTaskStatusCancelled`).
Do not block in the handler — dispatch long work on a goroutine.

### Inspecting and cancelling tasks

```go
tasks, _ := host.ListMCPTasks(ctx, "build-server")
for _, t := range tasks {
    fmt.Printf("%s: %s (%s)\n", t.TaskID, t.Status, t.StatusMessage)
}

t, _ := host.GetMCPTask(ctx, "build-server", taskID)
if !t.Status.IsTerminal() {
    _, _ = host.CancelMCPTask(ctx, "build-server", taskID)
}
```

`Kit.ListMCPTasks`, `Kit.GetMCPTask`, and `Kit.CancelMCPTask` work against any
loaded MCP server that advertises the corresponding capability.
`MCPTaskStatus.IsTerminal()` is the canonical check for completion.

Context cancellation also works end-to-end: cancelling the `ctx` passed to a
tool execution triggers a best-effort `tasks/cancel` before the call returns.

## Custom debug logger

Kit's engine and MCP tool plumbing emit low-level debug output through a
`DebugLogger` interface. By default, setting `Debug: true` (or calling
`WithDebug()`) installs the built-in console logger. To route the same output
into your application's logging system instead, provide a custom
implementation via `Options.DebugLogger` or `WithDebugLogger`.

```go
type DebugLogger interface {
    LogDebug(message string)
    IsDebugEnabled() bool
}
```

When `DebugLogger` is non-nil it takes precedence over `Debug` — the
supplied logger's `IsDebugEnabled()` reports whether downstream code should
bother formatting messages.

**Example: forward to `log/slog`:**

```go
import "log/slog"

type slogDebugLogger struct{ l *slog.Logger }

func (s *slogDebugLogger) LogDebug(m string)    { s.l.Debug(m) }
func (s *slogDebugLogger) IsDebugEnabled() bool { return true }

host, _ := kit.NewAgent(ctx,
    kit.WithModel("anthropic/claude-sonnet-4-5-20250929"),
    kit.WithDebugLogger(&slogDebugLogger{l: slog.Default()}),
)
```

Implementations must be safe for concurrent use — messages can arrive
from the engine goroutine, MCP connection pool, and tool execution paths
simultaneously.

## Precedence

For any given generation or provider field, the effective value is resolved
in this order (highest priority first):

1. `Options.X` (SDK caller)
2. `KIT_X` environment variable
3. `.kit.yml` (project-local then `~/.kit.yml`)
4. Per-model defaults (`modelSettings[provider/model]` or `customModels[...].params`)
5. Provider-level defaults (e.g. Anthropic's own temperature default)
6. SDK last-resort floor (currently: `MaxTokens = 8192`, matching the CLI `--max-tokens` default)

Sampling params that remain `nil` after the SDK resolution step are left out
of the provider call entirely, so the LLM library applies its own default.

## Tool configuration

**`Tools`** replaces default core tools, not MCP or extension tools. **`ExtraTools`** adds tools alongside the defaults. Use `Tools` to replace core tools; use `ExtraTools` to extend them.

To keep the default tool set but narrow the built-in core tools, set
**`CoreToolList`** (an allow-list of names such as `"bash"`, `"read"`,
`"write"`, `"edit"`, `"grep"`, `"find"`, `"ls"`, `"subagent"`). Build the list
from include/exclude filters with
[`kit.FilterCoreToolNames`](/sdk/overview#filtering-core-tools):

```go
list, _ := kit.FilterCoreToolNames(nil, []string{"bash", "write"}) // drop two
host, _ := kit.New(ctx, &kit.Options{CoreToolList: list})
```

`DisableCoreTools: true` is the chat-only shortcut for an empty core set.

Create custom tools with `kit.NewTool` — no external dependencies needed:

```go
type LookupInput struct {
    ID string `json:"id" description:"Record ID to look up"`
}

lookupTool := kit.NewTool("lookup", "Look up a record by ID",
    func(ctx context.Context, input LookupInput) (kit.ToolOutput, error) {
        record := db.Find(input.ID)
        return kit.TextResult(record.String()), nil
    },
)

host, _ := kit.New(ctx, &kit.Options{
    ExtraTools: []kit.Tool{lookupTool},
})
```

See [Overview](/sdk/overview#custom-tools) for full custom tool documentation.

### Routing file access and commands

By default the core tools read and write the local disk and run commands as
local child processes. Two tool options change this, for example to let an
editor provide unsaved buffers and run commands in its own terminal:

- **`kit.WithFileSystem(fs)`**: the `read`, `write` and `edit` tools read and
  write text through `fs` (a `kit.FileSystem`). Images are still read from
  the local disk.
- **`kit.WithCommandRunner(r)`**: the `shell` tool runs commands through `r`
  (a `kit.CommandRunner`). The tool still checks the command and the timeout
  first. Interactive sudo prompts are not available through a runner.

```go
host, _ := kit.New(ctx, &kit.Options{
    WorkDir:         "/path/to/project",
    CoreToolOptions: []kit.ToolOption{kit.WithFileSystem(myFS), kit.WithCommandRunner(myRunner)},
})
```

`kit acp` uses these options to implement the ACP `fs/*` and `terminal/*`
client methods.

## Options and functional helper inventory

This inventory links configuration to the [authoritative Go contract](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options). Fields retain their documented zero/unset semantics. Functional options apply in order; later values win. Use `New` with `Options` for fields without a helper.

| Name | Contract |
| --- | --- |
| `WithProvider` | Registers a provider factory for this Kit instance only; see `Providers`. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#WithProvider) |
| `WithDecisionProvider` | Registers a decision provider factory for this Kit instance only; see `DecisionProviders`. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#WithDecisionProvider) |
| `AllowMissingCredentials` | AllowMissingCredentials lets New succeed when the configured provider cannot be created — no API key or OAuth token, a failed OAuth token refresh, an unsupported provider, and so on. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |
| `AlwaysOfferSkillActivation` | AlwaysOfferSkillActivation keeps the built-in activate_skill tool offered with an empty catalogue. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |
| `AutoCompact` | Compaction AutoCompact enables proactive compaction before turns that near the context limit. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |
| `Bare` | Bare disables every form of automatic context discovery, so starting Kit inside a directory does not implicitly load that directory's instructions, tooling or configuration. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |
| `CLI` | CLI is optional CLI-specific configuration. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |
| `CodeMode` | CodeMode configures code mode: one tool that runs a JavaScript program which calls the other tools and returns a small result. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |
| `CompactionOptions` | Config for auto-compaction (nil = defaults). [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |
| `ConfigFile` | Override config file path. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |
| `Continue` | Continue the newest session for SessionDir. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |
| `CoreToolList` | List of tools to include, when empty, include all available core tools. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |
| `CoreToolOptions` | CoreToolOptions are applied to the built-in core tools after the options Kit derives from its own settings, so they win on conflict. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |
| `DebugLogger` | DebugLogger, if non-nil, routes low-level debug output from the engine and the MCP tool plumbing to a caller-supplied implementation. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |
| `Debug` | Debug enables debug logging for the SDK. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |
| `DisableCoreTools` | DisableCoreTools, when true, prevents loading any core tools. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |
| `Ephemeral` | Ephemeral configures an in-memory session with no persistence (equivalent to Options.NoSession = true). [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Ephemeral) |
| `ExtraTools` | Additional tools added alongside core/MCP/extension tools. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |
| `FrequencyPenalty` | FrequencyPenalty discourages repeated tokens (OpenAI-family models). [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |
| `HTTPTransport` | HTTPTransport wraps each built-in provider's base HTTP transport. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |
| `ImageMaxBytes` | ImageMaxBytes caps the encoded size, in bytes, of an image the read tool attaches to a message. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |
| `ImageMaxEdge` | ImageMaxEdge caps the width and height, in pixels, of an image the read tool attaches to a message. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |
| `ImageNoResize` | ImageNoResize makes the read tool reject an oversized image instead of scaling it down. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |
| `InProcessMCPServers` | InProcessMCPServers registers mcp-go servers that run in the same process. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |
| `Isolated` | Isolated turns off everything that Kit otherwise discovers or writes on the host by itself. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Isolated) |
| `MCPAuthHandler` | MCPAuthHandler handles OAuth authorization for remote MCP servers. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |
| `MCPConfig` | MCPConfig provides a pre-loaded MCP configuration. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |
| `MCPTaskMaxPollInterval` | MCPTaskMaxPollInterval caps the polling interval (a server-supplied pollInterval can otherwise grow without bound). [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |
| `MCPTaskMode` | MCPTaskMode overrides the per-server [MCPTaskMode] for task-augmented tools/call execution. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |
| `MCPTaskPollInterval` | MCPTaskPollInterval is the fallback interval between tasks/get requests when the server does not suggest one. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |
| `MCPTaskProgress` | MCPTaskProgress, if non-nil, is invoked once when a task is accepted and on every status transition observed by the polling loop. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |
| `MCPTaskTTL` | MCPTaskTTL is the TTL hint sent in TaskParams for every task-augmented tools/call. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |
| `MCPTaskTimeout` | MCPTaskTimeout is the maximum wall-clock duration to wait for a task-augmented tool call to reach a terminal state. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |
| `MCPTokenStoreFactory` | MCPTokenStoreFactory, if non-nil, is called to create a token store for each remote MCP server that requires OAuth. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |
| `MaxSteps` | Override max steps (0 = use default). [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |
| `MaxTokens` | MaxTokens overrides the maximum output tokens per LLM response. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |
| `ModelMode` | Model mode to select (empty selects the model default). [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |
| `Model` | Override model (e.g., "anthropic/claude-sonnet-4-5-20250929"). [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |
| `DecisionModel` | Default decision model in "provider/model" format, used by Kit.Decide and extensions when a request names no model. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |
| `NewAgent` | NewAgent creates a Kit using an ergonomic functional-options API. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#NewAgent) |
| `NewIsolatedAgent` | NewIsolatedAgent is [NewAgent] with [Isolated] applied before opts. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#NewIsolatedAgent) |
| `NoAgents` | NoAgents disables discovery of named agent definitions (built-ins and .agents/agents/ / .kit/agents/ / ~/.config/kit/agents/ files). [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |
| `NoContextFiles` | NoContextFiles disables automatic loading of project context files (for example AGENTS.md). [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |
| `NoExtensions` | NoExtensions disables Yaegi extension loading entirely. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |
| `NoSession` | In-memory session without file persistence. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |
| `NoSkills` | Disable skill loading entirely (auto-discovery and explicit). [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |
| `OnMCPServerLoaded` | OnMCPServerLoaded, if non-nil, is called when each MCP server finishes loading during Kit initialization. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |
| `PresencePenalty` | PresencePenalty discourages repeating topics (OpenAI-family models). [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |
| `ProviderAPIKey` | ProviderAPIKey overrides the API key used to authenticate with the model provider. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |
| `ProviderURL` | ProviderURL overrides the provider endpoint. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |
| `ProviderWire` | ProviderWire overrides the wire protocol used for auto-routed providers: "openai" (Responses API), "openai-compat" (chat completions), "anthropic", or "google". [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |
| `Providers` | Providers registers provider factories for this Kit instance, keyed by provider name (case-insensitive, must not contain "/"). [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |
| `DecisionProviders` | DecisionProviders registers decision provider factories for this Kit instance, keyed by provider name with the same rules as Providers. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |
| `Quiet` | Suppress debug output. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |
| `RetryPolicy` | RetryPolicy limits retries per model request. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |
| `SessionDir` | Project directory identity for session discovery, not a storage root. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |
| `SessionManager` | SessionManager allows custom session storage backends. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |
| `SessionPath` | Open a specific session file. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |
| `ShellMaxTimeout` | ShellMaxTimeout caps the maximum timeout (in seconds) a shell tool call may request via its timeout argument. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |
| `ShellTimeout` | ShellTimeout sets the default per-call timeout (in seconds) for the shell tool, applied when the model does not specify one. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |
| `Shell` | Shell is the shell the shell tool runs a command string through, plus its own leading arguments (for example bash or busybox ash). [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |
| `SkillTrustPrompt` | SkillTrustPrompt is an optional callback invoked the first time Kit auto-discovers project-local skills (under <project>/.agents/skills or <project>/.kit/skills) in a directory that is not yet on the trust allowlist. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |
| `SkillsDir` | Scan this directory directly for skills. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |
| `SkillsDisable` | SkillsDisable names skills (by Name) to exclude from the model-facing catalog. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |
| `Skills` | Explicit skill files or directories; empty enables discovery. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |
| `SkipConfig` | SkipConfig, when true, skips loading .kit.yml configuration files. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |
| `Streaming` | Streaming enables or disables streaming output. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |
| `SystemPrompt` | Override system prompt. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |
| `TLSSkipVerify` | TLSSkipVerify disables TLS certificate verification on provider HTTP clients. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |
| `Temperature` | Temperature controls sampling randomness (typically 0.0–2.0). [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |
| `ThinkingLevel` | ThinkingLevel sets the reasoning effort for models that support extended thinking. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |
| `Tools` | Custom tool set. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |
| `TopK` | TopK limits sampling to the top K tokens. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |
| `TopP` | TopP is the nucleus-sampling cutoff (0.0–1.0). [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |
| `UserAgent` | UserAgent identifies Kit-controlled HTTP requests. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |
| `WithAgents` | WithAgents enables discovery of named agent definitions again. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#WithAgents) |
| `WithAllowMissingCredentials` | WithAllowMissingCredentials lets New succeed when the configured provider cannot be created (no credentials, a failed OAuth token refresh, an unsupported provider, and so on). [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#WithAllowMissingCredentials) |
| `WithConfigFile` | WithConfigFile sets an explicit config file path, overriding the default .kit.yml search. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#WithConfigFile) |
| `WithConfig` | WithConfig enables .kit.yml discovery again (the home and project config files). [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#WithConfig) |
| `WithContextFiles` | WithContextFiles enables loading of project context files (AGENTS.md) from the working directory again. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#WithContextFiles) |
| `WithCoreTools` | WithCoreTools enables the core tools again. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#WithCoreTools) |
| `WithDebugLogger` | WithDebugLogger installs a caller-supplied [DebugLogger] for low-level engine and MCP tool plumbing output. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#WithDebugLogger) |
| `WithDebug` | WithDebug enables SDK debug logging. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#WithDebug) |
| `WithExtensions` | WithExtensions enables loading of extensions again. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#WithExtensions) |
| `WithExtraTools` | WithExtraTools adds tools alongside the core/MCP/extension tools rather than replacing them. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#WithExtraTools) |
| `WithHTTPTransport` | WithHTTPTransport wraps the base HTTP transport of built-in providers. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#WithHTTPTransport) |
| `WithMaxTokens` | WithMaxTokens sets the maximum output tokens per LLM response. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#WithMaxTokens) |
| `WithModelMode` | WithModelMode selects a named mode for the selected model. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#WithModelMode) |
| `WithModel` | WithModel sets the model in "provider/model" format (for example openai/gpt-4.1). [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#WithModel) |
| `WithDecisionModel` | WithDecisionModel sets the default decision model in "provider/model" format (e.g. openrouter/typesafe/jev-1.14). [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#WithDecisionModel) |
| `WithProviderAPIKey` | WithProviderAPIKey overrides the API key used to authenticate with the model provider. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#WithProviderAPIKey) |
| `WithProviderURL` | WithProviderURL overrides the provider endpoint URL. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#WithProviderURL) |
| `WithProviderWire` | WithProviderWire overrides the wire protocol used for auto-routed providers: "openai" (Responses API), "openai-compat" (chat completions), "anthropic", or "google". [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#WithProviderWire) |
| `WithRetryPolicy` | WithRetryPolicy sets the retry limits for model requests. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#WithRetryPolicy) |
| `WithSessions` | WithSessions enables persistent sessions again. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#WithSessions) |
| `WithSkillActivation` | WithSkillActivation keeps activate_skill available even before any skills are registered. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#WithSkillActivation) |
| `WithSkills` | WithSkills enables skills again. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#WithSkills) |
| `WithStreaming` | WithStreaming enables or disables streaming responses. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#WithStreaming) |
| `WithSystemPrompt` | WithSystemPrompt sets the system prompt. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#WithSystemPrompt) |
| `WithThinkingLevel` | WithThinkingLevel sets the reasoning effort for models that support extended thinking. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#WithThinkingLevel) |
| `WithTools` | WithTools sets the agent's tool set, replacing the default core tools. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#WithTools) |
| `WithUserAgent` | WithUserAgent sets the per-instance HTTP identity. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#WithUserAgent) |
| `WorkDir` | WorkDir is the directory the built-in file and shell tools resolve relative paths against and run commands in. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options) |

## Model modes

`ModelMode` describes catalog modes, including `Experimental`, `Supported`, `UnsupportedReason`, and estimated input/output USD per million tokens. `host.ModelModes()` includes unavailable modes; `host.SupportedModelModes()` filters them. Query these methods rather than assuming every model offers a fixed mode.

Select with `Options.ModelMode` / `WithModelMode`, or `host.SetModelMode(ctx, name)`. Check errors with `errors.Is(err, kit.ErrUnsupportedModelMode)` for unsupported modes. `CurrentModelMode` reports the selection; an empty string selects the model default. Changes during an active turn fail, including reset. Finish or cancel the turn first.

## Code mode

`Options.CodeMode` points to `CodeModeOptions`. Code mode is default-on, but default enablement respects the resolved tool list and disabled core tools. Explicit `Enabled: true` can offer code mode with MCP-only tools; naming `codemode` in the core tool list selects it. `Disabled: true` takes precedence over enablement and config. For a custom `Tools` or `ExtraTools` set, create the tool with `NewCodeModeTool`, check its error, and add the returned tool. Kit supplies the current tool set with hooks applied, so nested calls still pass through hooks.

Zero option fields fall back to the `codemode` config section, then defaults:

| Field | Default and unit |
| --- | --- |
| `Timeout` | 30 seconds (`time.Duration` in Go, seconds in YAML) |
| `MaxToolCalls` | 50 tool calls per script |
| `MaxOutputBytes` | 100000 bytes; larger captured output may be saved to a temporary file, but combined script output and logs are capped at 4 MiB and excess is truncated |
| `MaxConcurrency` | 8 parallel tool calls per script |
| `MemoryLimitMB` | 256 MiB approximate heap growth (value shifted by 20 bits); negative disables check |
| `CatalogBudget` | 12000 bytes in the tool catalog description |
| `MCPExposure` | `CodeModeExposureDirect` for unmatched MCP tools |
| `Exposure` | Tool-name glob rules; exact names win over globs, then longer patterns win |

Exposure constants and their config values:

- `CodeModeExposureDirect` (`direct`): model sees the tool; scripts can call it.
- `CodeModeExposureScriptOnly` (`codemode`): scripts only; full signature in catalog.
- `CodeModeExposureDeferred` (`deferred`): scripts only; discover with `searchTools()`.
- `CodeModeExposureModelOnly` (`model-only`): model only; scripts cannot call it.

`CodeModeToolName` is `codemode`. See [code mode](https://go-kit.dev/advanced/code-mode) for JavaScript APIs. JavaScript limits and discovery/isolation switches are not OS sandboxing. Tool access has the host process permissions; restrict the runtime when inputs are untrusted.

## Managed skill activation

`AlwaysOfferSkillActivation` / `WithSkillActivation` keeps `activate_skill` offered even with an empty catalog. It does not enable discovery or override `NoSkills`. Combine with `Isolated` and register application-owned skills. Use `Skill.NoHostIO` for content that must not be reopened from host paths. Caller-owned activation tools and explicit preparation tool overrides retain precedence. See the [Go contract](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#WithSkillActivation) for per-step snapshot and cancellation rules.
