# Kit SDK: Options Reference

> Part of the `kit-sdk` skill. Read `SKILL.md` first for the overview and critical constraints.


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

## Options Reference

All fields are optional. Zero values use CLI defaults.

```go
host, err := kit.New(ctx, &kit.Options{
    // Model
    Model:        "anthropic/claude-sonnet-4-5-20250929", // "provider/model" format
    SystemPrompt: "You are a helpful assistant",
    ConfigFile:   "/path/to/config.yml",                  // default: ~/.kit.yml

    // Behavior
    MaxSteps:  10,   // 0 = use the configured default
    Streaming: true, // stream LLM output (default from config)
    Quiet:     true, // suppress debug output
    Debug:     true, // enable debug logging

    // Generation parameters — override env/config/per-model defaults.
    // Leaving a field at its zero/nil value lets the precedence chain
    // resolve a value (KIT_* env → .kit.yml → modelSettings/customModels →
    // 8192 floor for MaxTokens, provider defaults for samplers).
    MaxTokens:        16384,             // 0 = auto-resolve; non-zero suppresses right-sizing
    ThinkingLevel:    "medium",          // model-specific effort ("" = default)
    Temperature:      ptrFloat32(0.2),   // pointer so explicit 0.0 != unset
    TopP:             nil,                // nil = leave provider/per-model default
    TopK:             nil,                // nil = leave provider/per-model default
    FrequencyPenalty: nil,
    PresencePenalty:  nil,

    // Provider configuration — override env/config without viper.Set workarounds.
    ProviderAPIKey: "sk-...",                    // "" = use config / provider env var
    ProviderURL:    "https://proxy.internal/v1", // "" = provider default endpoint
    TLSSkipVerify:  false,                       // true only; can't force-disable via Options

    // Session
    SessionDir:  "/path/to/project",  // base dir for session discovery (default: cwd)
    SessionPath: "/path/to/session.jsonl", // open specific session file
    Continue:    true,                // resume most recent session for SessionDir
    NoSession:   true,                // ephemeral in-memory session, no disk persistence
    SessionManager: myCustomSession,  // custom SessionManager implementation (advanced)

    // Tools
    Tools:            []kit.Tool{kit.NewShellTool()}, // REPLACES entire default tool set
    ExtraTools:       []kit.Tool{myTool},            // ADDS alongside core/MCP/extension tools
    DisableCoreTools: true,                        // Use no core tools (0 tools, for chat-only)
    CoreToolList:     nil,                         // nil = use config/defaults; with no configured names, include all core tools

    // Configuration
    SkipConfig:   true,                        // Skip .kit.yml files (viper defaults + env vars still apply)

    // Skills
    Skills:    []string{"/path/to/skill.md"}, // explicit skill files (empty = auto-discover)
    SkillsDir: "/path/to/skills",             // override project-local skills dir
    NoSkills:  true,                          // disable skill loading entirely

    // Feature toggles
    NoExtensions:   true,                     // disable Yaegi extension loading entirely
    NoContextFiles: true,                     // disable automatic AGENTS.md loading

    // Compaction
    AutoCompact:       true,                        // auto-compact near context limit
    CompactionOptions: &kit.CompactionOptions{...}, // nil = defaults

    // MCP OAuth — both fields are opt-in. If MCPAuthHandler is nil,
    // remote MCP servers that require OAuth will fail to connect with
    // an authorization-required error instead of silently opening a
    // browser. CLI consumers use NewCLIMCPAuthHandler; other embedders
    // implement MCPAuthHandler or configure DefaultMCPAuthHandler.
    MCPAuthHandler: mcpAuthHandler,             // nil = OAuth disabled
    MCPTokenStoreFactory: func(serverURL string) (kit.MCPTokenStore, error) {
        return myCustomStore(serverURL), nil  // custom OAuth token storage
    },

    // In-Process MCP Servers
    InProcessMCPServers: map[string]*kit.MCPServer{
        "docs": mcpSrv,  // *server.MCPServer from mcp-go — no subprocess needed
    },
})

// Tiny helper to take the address of a literal for pointer fields.
func ptrFloat32(v float32) *float32 { return &v }
```

**Critical distinction**: `Tools` replaces default core tools, not MCP or extension tools. `ExtraTools` adds tools alongside the defaults. Use `Tools` to replace core tools; use `ExtraTools` to extend them.

**Custom provider backends**: `Providers: map[string]kit.ProviderFactory{"local": f}` (or `kit.WithProvider`) routes `local/<model>` to your factory. See `references/models-and-mcp.md`.

**In-process MCP servers** bypass subprocess spawning entirely. Pass `*server.MCPServer` instances from mcp-go via `InProcessMCPServers` or call `AddInProcessMCPServer()` at runtime.

### Isolated agents (functional options)

By default an embedded Kit discovers `.kit.yml` (and its MCP servers), `AGENTS.md`, skills, extensions and named agents on the host, writes session files, and enables the core file and shell tools. For an agent that uses only what your application passes in, use `kit.NewIsolatedAgent` (or `kit.Isolated()` as the first option to `kit.NewAgent`):

```go
host, err := kit.NewIsolatedAgent(ctx,
    kit.WithModel("anthropic/claude-sonnet-4-5-20250929"),
    kit.WithExtraTools(myTool),
)
```

`Isolated()` sets `SkipConfig`, `NoContextFiles`, `NoSkills`, `NoExtensions`, `NoAgents`, `NoSession` and `DisableCoreTools`. It does not set `Bare`. Options apply in order, so put opt-ins after it:

| Opt-in | Effect |
|--------|--------|
| `WithConfig()` | `SkipConfig = false` (`.kit.yml` discovery) |
| `WithConfigFile(path)` | Loads only that file; also clears `SkipConfig` |
| `WithContextFiles()` | `NoContextFiles = false` |
| `WithSkills(paths...)` | `NoSkills = false`; sets `Skills` when paths are given |
| `WithExtensions()` | `NoExtensions = false` |
| `WithAgents()` | `NoAgents = false` |
| `WithSessions()` | `NoSession = false` |
| `WithCoreTools(names...)` | `DisableCoreTools = false`; sets `CoreToolList` when names are given |

`KIT_*` environment variables still apply. Subagents inherit the discovery switches (`Bare`, `SkipConfig`, `NoContextFiles`, `NoSkills`, `NoExtensions`, `NoAgents`) and `NoSession`, and a subagent without explicit `Tools` gets only the parent's enabled core tools, so they stay isolated too.

### Generation & provider Options (cheat sheet)

| Field | Type | Empty/nil means | Notes |
|-------|------|-----------------|-------|
| `MaxTokens` | `int` | Auto-resolve (env → config → per-model → 8192 floor) | Non-zero suppresses `rightSizeMaxTokens` |
| `ThinkingLevel` | `string` | Auto-resolve (→ `"off"`) | Model-specific catalog choices, including future names. `off` omits effort; `none` is literal. Use `SupportedThinkingLevels`; explicit unsupported graded selections return an error. Default ladder: off, none, minimal, low, medium, high, xhigh |
| `Temperature` | `*float32` | Leave provider/per-model default | Pointer so explicit `0.0` ≠ unset |
| `TopP` | `*float32` | Leave provider/per-model default | |
| `TopK` | `*int32` | Leave provider/per-model default | |
| `FrequencyPenalty` | `*float32` | Leave provider/per-model default | OpenAI-family |
| `PresencePenalty` | `*float32` | Leave provider/per-model default | OpenAI-family |
| `ProviderAPIKey` | `string` | Use config / provider env var | Overrides pre-existing viper state |
| `ProviderURL` | `string` | Use provider default endpoint | Same base URL flag as `--provider-url` |
| `TLSSkipVerify` | `bool` | — | Only effective when `true`; cannot force-disable via Options |
| `Providers` | `map[string]ProviderFactory` | No instance factories | App-supplied backends; wins over `kit.RegisterProvider` and built-ins; copied by `kit.New` |

These fields eliminate the old `viper.Set("max-tokens", 16384)` dance many
downstream embedders used to do before calling `kit.New()`. Everything is
now discoverable via godoc on `kit.Options`.


## Options and functional helper inventory

This inventory links configuration to the [authoritative Go contract](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#Options). Fields retain their documented zero/unset semantics. Functional options apply in order; later values win. Use `New` with `Options` for fields without a helper.

| Name | Contract |
| --- | --- |
| `WithProvider` | Registers a provider factory for this Kit instance only; see `Providers`. [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#WithProvider) |
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
