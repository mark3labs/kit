# Kit SDK: Options Reference

> Part of the `kit-sdk` skill. Read `SKILL.md` first for the overview and critical constraints.

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
    CoreToolList      []string,                    // List of core tools to include, if empty (default) include all

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

**In-process MCP servers** bypass subprocess spawning entirely. Pass `*server.MCPServer` instances from mcp-go via `InProcessMCPServers` or call `AddInProcessMCPServer()` at runtime.

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

These fields eliminate the old `viper.Set("max-tokens", 16384)` dance many
downstream embedders used to do before calling `kit.New()`. Everything is
now discoverable via godoc on `kit.Options`.

