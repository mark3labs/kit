# Kit SDK: Extension API, Authentication, and Skills

> Part of the `kit-sdk` skill. Read `SKILL.md` first for the overview and critical constraints.

## Extension API

The `Extensions()` method returns an `ExtensionAPI` interface that groups all extension-related functionality. This is the primary way to interact with extension state from the SDK.

Fragment prerequisites: `host` is a Kit instance; import `kit "github.com/mark3labs/kit/pkg/kit"`. Ellipses mark fields to supply.

```go
extAPI := host.Extensions()

// Check if extensions are loaded
if extAPI.HasExtensions() {
    // Context management
    extAPI.SetContext(kit.ExtensionContext{...})
    ctx := extAPI.GetContext()
    extAPI.UpdateContextModel("anthropic/claude-sonnet-4-5-20250929")

    // Widgets, headers, footers
    extAPI.SetWidget(kit.ExtensionWidgetConfig{...})
    extAPI.RemoveWidget("widget-id")
    extAPI.SetHeader(kit.ExtensionHeaderFooterConfig{...})
    extAPI.SetFooter(kit.ExtensionHeaderFooterConfig{...})

    // Status bar
    extAPI.SetStatus(kit.ExtensionStatusBarEntry{...})
    extAPI.RemoveStatus("key")

    // Options
    extAPI.SetOption("name", "value")
    val := extAPI.GetOption("name")

    // Tools
    tools := extAPI.GetToolInfos()
    extAPI.SetActiveTools([]string{"bash", "read"})

    // Session-scoped extension state (last-write-wins key-value store).
    // Backed by an in-memory map and a per-session sidecar file
    // (<session>.ext-state.json) outside the conversation tree.
    extAPI.SetState("myext:budget-cap", "10.00")
    val, ok := extAPI.GetState("myext:budget-cap")
    extAPI.DeleteState("myext:budget-cap")
    keys := extAPI.ListState()

    // Load any existing state from the sidecar and install a saver hook so
    // subsequent SetState/DeleteState mutations are flushed atomically.
    // No-op for ephemeral / in-memory sessions. Safe to call multiple times.
    _ = extAPI.InitStatePersistence()

    // Events
    extAPI.EmitSessionStart()
    extAPI.EmitModelChange("new/model", "old/model", "extension")
    extAPI.EmitCustomEvent("my-event", "data")

    // Commands and lifecycle
    cmds := extAPI.Commands()
    err := extAPI.Reload()
}
```

All methods are no-ops when extensions are disabled (nil runner), so callers don't need nil checks.

---

# SDK authentication

## Stored credentials

`SetProviderAPIKey(providerID, apiKey)` stores a key in `$XDG_CONFIG_HOME/.kit/credentials.json` (0600 for a newly created file; existing permissions are preserved), or `~/.config/.kit/credentials.json`. Handle its error. Stored keys take precedence over provider environment variables. Explicit `Options.ProviderAPIKey` / `WithProviderAPIKey` overrides stored API keys and provider environment variables. Exception: stored OpenAI OAuth credentials take precedence at the default OpenAI destination or a trusted Codex endpoint (https://chatgpt.com/backend-api/codex); configuration can also supply provider settings. Do not log keys, tokens or the credential store.

`GetProviderAPIKey` reads only the stored key (empty when absent), not environment fallback. `HasProviderCredentials` reports stored API-key or OAuth presence, not remote validity. `RemoveProviderCredentials` deletes all stored credentials for that provider. Anthropic and OpenAI use dedicated slots; Copilot is OAuth-only and rejects API-key storage. `NewCredentialManager` exposes the store for advanced use; see its [Go reference](https://pkg.go.dev/github.com/mark3labs/kit/pkg/kit#CredentialManager) for provider-specific helpers.

## Startup without credentials

By default `New` fails fast. `IsMissingCredentialsError(err)` detects a wrapped `MissingCredentialsError`. `WithAllowMissingCredentials` / `Options.AllowMissingCredentials` tolerates provider-creation failures more broadly: missing credentials, failed refresh, unsupported providers and similar failures. It does not tolerate a malformed model string. Check `host.ProviderError()`; prompts fail with that error until `SetModel` successfully creates a provider. After storing credentials, call `SetModel` or create a new instance. This option does not authenticate or silently retry prompts.

## Host-controlled OpenAI login

`LoginProvider(ctx, "openai", LoginOptions{...})` supports only ChatGPT/Codex subscription authentication, not general OpenAI API OAuth. Anthropic and Copilot login are not supported by this function. The required `OnAuthorization(context.Context, LoginInstructions) error` callback receives a URL after the listener is ready. The host displays it or opens a browser; the SDK does neither. Honor cancellation and return callback errors.

The flow is bounded by five minutes or the caller's earlier deadline. It binds loopback `127.0.0.1:1455` and `[::1]:1455`, with redirect `http://localhost:1455/auth/callback`. At least one listener must bind, and the browser must reach it. Remote/headless hosts need loopback port forwarding before login. There is no manual code-entry fallback. Keep the credential store writable for token refresh. Existing instances need provider recreation through `SetModel` (or a new Kit).

The compile-checked [login example](https://github.com/mark3labs/kit/blob/master/pkg/kit/provider_login_example_test.go) shows a three-minute caller timeout and context-aware URL display. Normal tests compile but do not execute it. `LoginOptions.CredentialManager` selects a custom store; `LoginOptions.UserAgent` is independent of any instance identity.

Provider login is distinct from MCP OAuth. MCP servers use `MCPAuthHandler` and separate token storage: see [MCP OAuth](https://go-kit.dev/sdk/options#mcp-oauth-authorization) and the [public API index](https://go-kit.dev/sdk/api-reference).

---

## Skills

```go
// Load a single skill file
skill, _ := kit.LoadSkill("/path/to/SKILL.md")
// skill.Name, skill.Description, skill.Content, skill.Path

// Load from directory
skills, _ := kit.LoadSkillsFromDir("/path/to/skills")

// Auto-discover (global + project-local)
skills, _ := kit.LoadSkills("/path/to/project")

// Prompt building with skills
pb := kit.NewPromptBuilder("You are an assistant")
pb.WithSkills(skills)
pb.WithSection("", "Extra context here")
systemPrompt := pb.Build()
```

### Runtime registration and activation

`AddSkill`, `SetSkills`, `RemoveSkill`, `DisableSkill`, `EnableSkill`, and reload
update the next prepared request, including mutations completed inside
`OnPrepareStep` in the same turn. Qualified names are allowed. Kit owns copies
of skill values, Metadata and Tags; `GetSkills` returns owned copies.
`SetSkills` uses the last entry for duplicate names and rejects invalid batches
without mutation.

Registered Content is authoritative, even when empty or without Path. Built-in
activation does not read newer disk bytes: use `ReloadSkills` or
`LoadAndAddSkill` to select disk content. Explicit `/name` activation keeps its
existing disk-freshness behavior.

Each request freezes sorted, unique eligible names and selected body/path.
Removed and model-disabled names disappear. With no eligible skills the managed
`activate_skill` is omitted; adding from zero restores it next request. Stale
selections fail with a fresh-request error rather than returning replacement
bytes. Deduplication tracks the latest successfully activated body/path per
name, so edited content can activate again in the same turn. Removing a skill
does not erase past conversation content. Resources remain filesystem-based
and best-effort.

