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

