# Kit SDK: Model Management and Dynamic MCP Servers

> Part of the `kit-sdk` skill. Read `SKILL.md` first for the overview and critical constraints.

## Model Management

### At creation time

```go
host, _ := kit.New(ctx, &kit.Options{
    Model: "openai/gpt-4o",
})
```

### At runtime

```go
err := host.SetModel(ctx, "anthropic/claude-sonnet-4-5-20250929")
modelStr := host.GetModelString()   // "provider/model"
info := host.GetModelInfo()          // *kit.ModelInfo (capabilities, limits, pricing) or nil
isReasoning := host.IsReasoningModel()
level := host.GetThinkingLevel()
err = host.SetThinkingLevel(ctx, "medium") // recreates agent with new thinking budget
```

### Model registry

```go
models := host.GetAvailableModels()      // []extensions.ModelInfoEntry
providers := kit.GetSupportedProviders() // []string
providers := kit.GetLLMProviders()       // providers with LLM support
models, _ := kit.GetModelsForProvider("anthropic") // map[string]kit.ModelInfo
info := kit.LookupModel("anthropic", "claude-sonnet-4-5-20250929") // *kit.ModelInfo
info := kit.GetProviderInfo("openai")    // *kit.ProviderInfo (env vars, API URL)
err := kit.ValidateEnvironment("anthropic", "") // check API keys
suggestions := kit.SuggestModels("anthropic", "claudee") // fuzzy match
```

### Model string format

Always `"provider/model"`: `"anthropic/claude-sonnet-4-5-20250929"`, `"openai/gpt-4o"`, `"ollama/qwen3:8b"`.

```go
provider, modelID, err := kit.ParseModelString("anthropic/claude-sonnet-4-5-20250929")
```

### Per-model system prompts

Models can have per-model system prompts configured via `modelSettings` or `customModels` in `.kit.yml`. When the user hasn't explicitly set a system prompt (via `--system-prompt`, config, or `Options.SystemPrompt`), the per-model prompt is used as the base and composed with AGENTS.md context and skills.

On `SetModel()`, if the new model has a per-model system prompt and no custom global prompt was set, the per-model prompt automatically replaces the previous one.

### Per-model generation parameters

Models can define default generation parameters (`temperature`, `top_p`, `top_k`, `frequency_penalty`, `presence_penalty`) via `modelSettings` or `customModels` `params` in `.kit.yml`. These defaults apply when the user hasn't explicitly set the parameter. Explicit CLI flags or config values always take priority.

---

## Dynamic MCP Server Management

Add, remove, and inspect MCP servers at runtime without restarting Kit:

```go
// Add a new MCP server — tools become available immediately
n, err := host.AddMCPServer(ctx, "github", kit.MCPServerConfig{
    Command:     []string{"npx", "-y", "@modelcontextprotocol/server-github"},
    Environment: map[string]string{"GITHUB_TOKEN": os.Getenv("GITHUB_TOKEN")},
})
fmt.Printf("Loaded %d tools from github server\n", n)

// Remove an MCP server — its tools are no longer available
err = host.RemoveMCPServer("github")

// List all currently loaded MCP servers
servers := host.ListMCPServers()
for _, s := range servers {
    fmt.Printf("Server %s: %d tools\n", s.Name, s.ToolCount)
}
```

`AddMCPServer` is safe to call while the agent is idle. If a turn is in progress, new tools are visible starting from the next LLM step. Tool names are prefixed with the server name (e.g. `"github__create_issue"`).

### In-Process MCP Servers

Register mcp-go servers that run in the same process — no subprocess spawning,
no network I/O:

```go
import (
    "github.com/mark3labs/mcp-go/mcp"
    "github.com/mark3labs/mcp-go/server"
)

mcpSrv := server.NewMCPServer("my-tools", "1.0.0",
    server.WithToolCapabilities(true),
)
mcpSrv.AddTool(mcp.NewTool("search_docs",
    mcp.WithDescription("Search documentation"),
    mcp.WithString("query", mcp.Required()),
), searchHandler)

// At init time
host, _ := kit.New(ctx, &kit.Options{
    InProcessMCPServers: map[string]*kit.MCPServer{
        "docs": mcpSrv,
    },
})

// Or at runtime
n, err := host.AddInProcessMCPServer(ctx, "docs", mcpSrv)
```

Kit does not own the server lifecycle — the caller handles cleanup. Tools are prefixed as usual (e.g. `"docs__search_docs"`).

### MCP Prompts

Query and expand prompts defined by connected MCP servers:

```go
// List all prompts from all connected MCP servers
prompts := host.ListMCPPrompts()
for _, p := range prompts {
    fmt.Printf("%s/%s: %s\n", p.ServerName, p.Name, p.Description)
    for _, arg := range p.Arguments {
        fmt.Printf("  arg: %s (required: %v)\n", arg.Name, arg.Required)
    }
}

// Expand a specific prompt with arguments
result, err := host.GetMCPPrompt(ctx, "myserver", "code-review", map[string]string{
    "language": "go",
    "style":    "thorough",
})
// result.Description — optional server description
// result.Messages — []MCPPromptMessage with Role, Content, and FileParts
for _, msg := range result.Messages {
    fmt.Printf("[%s] %s\n", msg.Role, msg.Content)
    // msg.FileParts contains binary attachments (images, embedded resources)
}
```

### MCP Resources

Read and subscribe to resources exposed by MCP servers:

```go
// List all resources from connected servers
resources := host.ListMCPResources()
for _, r := range resources {
    fmt.Printf("%s: %s (%s)\n", r.URI, r.Name, r.MIMEType)
}

// Read a specific resource
content, err := host.ReadMCPResource(ctx, "myserver", "file:///path/to/file")
if content.IsBlob {
    // Binary content in content.BlobData
} else {
    // Text content in content.Text
}

// Subscribe to resource change notifications
err = host.SubscribeMCPResource(ctx, "myserver", "file:///path/to/file")
// Unsubscribe later
err = host.UnsubscribeMCPResource(ctx, "myserver", "file:///path/to/file")
```

### MCP OAuth Authorization

When a remote MCP server requires OAuth, Kit runs the full authorization flow
(dynamic client registration → PKCE → user consent → token exchange → token
persistence) but delegates the **user-facing step** — displaying the
authorization URL and receiving the callback — to an `MCPAuthHandler`.

The SDK ships three building blocks:

| Building block | When to use |
|---|---|
| **No handler** (`Options.MCPAuthHandler = nil`) | Default. OAuth is disabled; 401s from remote MCP servers surface as errors. Correct for library, daemon, and web-app embedders that don't want side effects. |
| **`kit.NewCLIMCPAuthHandler()`** | CLI/TUI apps. Opens the system browser, prints status to stderr (or via `NotifyFunc`), runs a localhost callback server. This is what the `kit` binary uses. |
| **`kit.NewDefaultMCPAuthHandler()` + `OnAuthURL`** | Custom UX. Get the transport mechanics (port reservation + callback server) from the SDK; wire your own presentation in the `OnAuthURL(serverName, authURL)` closure. |
| **Implement `kit.MCPAuthHandler` directly** | Full control. No localhost binding — e.g. return the URL from an HTTP endpoint and have the consumer POST the callback URL back. |

**CLI-style embedder (browser + stderr):**

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
    // Render the URL however you like — no browser or terminal assumptions.
    myUI.ShowAuthPrompt(serverName, authURL)
}
defer authHandler.Close()

host, _ := kit.New(ctx, &kit.Options{
    MCPAuthHandler: authHandler,
})
```

**Important:** `DefaultMCPAuthHandler` with no `OnAuthURL` set will silently
drop the authorization URL and block until the 2-minute callback timeout
fires. Always set `OnAuthURL`, or use a higher-level wrapper like
`CLIMCPAuthHandler`.

### MCP OAuth Token Storage

Once authorization succeeds, the resulting access/refresh tokens are persisted
by an `MCPTokenStore`. By default tokens are written to
`$XDG_CONFIG_HOME/.kit/mcp_tokens.json` (fallback `~/.config/.kit/mcp_tokens.json`),
keyed by server URL, with `0600` file permissions.

Provide a custom store for encrypted storage, database persistence, or
in-memory-only flows:

```go
host, _ := kit.New(ctx, &kit.Options{
    MCPTokenStoreFactory: func(serverURL string) (kit.MCPTokenStore, error) {
        return &MyDatabaseTokenStore{serverURL: serverURL}, nil
    },
})
```

The `MCPTokenStore` interface requires `GetToken`/`SetToken`/`DeleteToken` methods. Return `kit.ErrMCPNoToken` from `GetToken` when no token is stored.

