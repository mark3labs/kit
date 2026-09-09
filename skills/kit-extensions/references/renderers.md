# Kit Extensions: Tool and Message Renderers

> Part of the `kit-extensions` skill. Read `SKILL.md` first for the overview and critical constraints.

## Tool Renderers

Customize how tool calls are displayed in the TUI:

```go
api.RegisterToolRenderer(ext.ToolRenderConfig{
    ToolName:     "bash",
    DisplayName:  "Shell",           // replaces auto-capitalized name
    BorderColor:  "#89b4fa",
    Background:   "",
    BodyMarkdown: true,              // render body through markdown
    RenderHeader: func(toolArgs string, width int) string {
        var args struct{ Command string `json:"command"` }
        json.Unmarshal([]byte(toolArgs), &args)
        return "$ " + args.Command
    },
    RenderBody: func(toolResult string, isError bool, width int) string {
        if isError {
            return "ERROR: " + toolResult
        }
        return toolResult
    },
})
```

## Message Renderers

Define named output styles for `ctx.RenderMessage()`:

```go
api.RegisterMessageRenderer(ext.MessageRendererConfig{
    Name: "success",
    Render: func(content string, width int) string {
        return "  " + content  // green checkmark prefix
    },
})

// Usage in handlers:
ctx.RenderMessage("success", "All tests passed")
```

