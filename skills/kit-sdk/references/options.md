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
    MaxSteps:  10,   // 0 = unlimited tool-calling steps
    Streaming: true, // stream LLM output (default from config)
    Quiet:     true, // suppress debug output
    Debug:     true, // enable debug logging

    // Generation parameters — override env/config/per-model defaults.
    // Leaving a field at its zero/nil value lets the precedence chain
    // resolve a value (KIT_* env → .kit.yml → modelSettings/customModels →
    // 8192 floor for MaxTokens, provider defaults for samplers).
    MaxTokens:        16384,             // 0 = auto-resolve; non-zero suppresses right-sizing
    ThinkingLevel:    "medium",          // "off", "none", "minimal", "low", "medium", "high" ("" = default)
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

**Critical distinction**: `Tools` replaces ALL default tools (core + MCP + extension). `ExtraTools` adds tools alongside the defaults. Use `Tools` to restrict the agent's capabilities; use `ExtraTools` to extend them.

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
| `ThinkingLevel` | `string` | Auto-resolve (→ `"off"`) | Valid: `"off"`, `"none"`, `"minimal"`, `"low"`, `"medium"`, `"high"` |
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

