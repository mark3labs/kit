# Kit SDK: Tools

> Part of the `kit-sdk` skill. Read `SKILL.md` first for the overview and critical constraints.

## Tools

### Creating custom tools

Use `kit.NewTool` to create custom tools. The JSON schema is auto-generated from the input struct — no external dependencies required:

```go
type WeatherInput struct {
    City string `json:"city" description:"City name, e.g. 'San Francisco'"`
}

weatherTool := kit.NewTool("get_weather", "Get current weather for a city",
    func(ctx context.Context, input WeatherInput) (kit.ToolOutput, error) {
        // Your logic here (API calls, database lookups, etc.)
        return kit.TextResult("72°F, sunny in " + input.City), nil
    },
)

host, _ := kit.New(ctx, &kit.Options{
    ExtraTools: []kit.Tool{weatherTool},
})
```

**Struct tags** control the generated schema:

| Tag | Purpose | Example |
|-----|---------|---------|
| `json:"name"` | Parameter name | `json:"city"` |
| `description:"..."` | Description shown to the LLM | `description:"City name"` |
| `enum:"a,b,c"` | Restrict valid values | `enum:"json,text,csv"` |
| `omitempty` | Marks parameter as optional | `json:"limit,omitempty"` |

**Return helpers:**

| Function | Description |
|----------|-------------|
| `kit.TextResult(content)` | Successful text result |
| `kit.ErrorResult(content)` | Error result (LLM sees it as a tool error) |
| `kit.ImageResult(content, data, mediaType)` | Image result with binary data (e.g. `"image/png"`) |
| `kit.MediaResult(content, data, mediaType)` | Non-image media result (e.g. `"audio/mpeg"`) |

**ToolOutput fields** (for advanced use):

```go
kit.ToolOutput{
    Content:   "result text",     // text returned to the LLM
    IsError:   false,             // true = LLM sees this as an error
    Data:      pngBytes,          // optional binary data (images, audio)
    MediaType: "image/png",       // MIME type for binary Data
    Metadata:  map[string]any{},  // opaque metadata for hooks/UI (not sent to LLM)
}
```

**Parallel tools** — mark as safe for concurrent execution:

```go
searchTool := kit.NewParallelTool("search", "Search the web",
    func(ctx context.Context, input SearchInput) (kit.ToolOutput, error) {
        return kit.TextResult("results..."), nil
    },
)
```

**Tool call ID** — available in context for logging/tracing:

```go
tool := kit.NewTool("my_tool", "...",
    func(ctx context.Context, input MyInput) (kit.ToolOutput, error) {
        callID := kit.ToolCallIDFromContext(ctx) // correlation ID from the LLM
        log.Printf("[%s] my_tool called", callID)
        return kit.TextResult("ok"), nil
    },
)
```

### Built-in tool constructors

```go
kit.NewReadTool(opts...)  // file reading
kit.NewWriteTool(opts...) // file writing
kit.NewEditTool(opts...)  // surgical text editing
kit.NewShellTool(opts...) // shell command execution (bash by default)
kit.NewGrepTool(opts...) // content search (uses ripgrep when available)
kit.NewFindTool(opts...) // file search (uses fd when available)
kit.NewLsTool(opts...)   // directory listing
```

### Tool bundles

```go
kit.AllTools(opts...)       // all 7 core tools
kit.CodingTools(opts...)    // bash, read, write, edit
kit.ReadOnlyTools(opts...)  // read, grep, find, ls
kit.SubagentTools(opts...)  // all except subagent (prevents recursion)
```

### Tool options

```go
kit.WithWorkDir("/path/to/dir") // override working directory for file-based tools
```

### Using tools in Options

```go
// Restricted: agent can ONLY run shell commands
host, _ := kit.New(ctx, &kit.Options{
    Tools: []kit.Tool{kit.NewShellTool()},
})

// Extended: all defaults PLUS a custom tool
host, _ := kit.New(ctx, &kit.Options{
    ExtraTools: []kit.Tool{myCustomTool},
})
```

### Querying tools at runtime

```go
names := host.GetToolNames()       // []string of all tool names
tools := host.GetTools()           // []kit.Tool (full tool objects)
mcpCount := host.GetMCPToolCount() // tools from MCP servers
extCount := host.GetExtensionToolCount() // tools from extensions
ready := host.MCPToolsReady()      // true when async MCP tool loading is complete
```

## Code mode

`Options.CodeMode` points to `CodeModeOptions`. Code mode is default-on, but default enablement respects the resolved tool list and disabled core tools. Explicit `Enabled: true` can offer code mode with MCP-only tools; naming `codemode` in the core tool list selects it. `Disabled: true` takes precedence over enablement and config. For a custom `Tools` or `ExtraTools` set, create the tool with `NewCodeModeTool`, check its error, and add the returned tool. Kit supplies the live wrapped tool set, so nested calls still pass through hooks.

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

