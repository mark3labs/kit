# Kit Extensions: Tools, Slash Commands, Keyboard Shortcuts, and Options

> Part of the `kit-extensions` skill. Read `SKILL.md` first for the overview and critical constraints.

## Registering Tools

Tools are functions the LLM can invoke:

```go
api.RegisterTool(ext.ToolDef{
    Name:        "current_time",
    Description: "Get the current date and time",
    Parameters:  `{"type":"object","properties":{}}`,
    Execute: func(input string) (string, error) {
        return time.Now().Format(time.RFC3339), nil
    },
})
```

For long-running tools with cancellation and progress:

```go
api.RegisterTool(ext.ToolDef{
    Name:        "slow_task",
    Description: "A long-running task with progress reporting",
    Parameters:  `{"type":"object","properties":{"query":{"type":"string"}}}`,
    ExecuteWithContext: func(input string, tc ext.ToolContext) (string, error) {
        for i := 0; i < 10; i++ {
            if tc.IsCancelled() {
                return "cancelled", nil
            }
            tc.OnProgress(fmt.Sprintf("Step %d/10...", i+1))
            time.Sleep(time.Second)
        }
        return "done", nil
    },
})
```

Parameters must be a JSON Schema string. The `input` argument is the JSON-encoded parameters from the LLM.

---

## Registering Slash Commands

Commands are user-facing actions invoked with `/name` in the input:

```go
api.RegisterCommand(ext.CommandDef{
    Name:        "echo",
    Description: "Echo back the provided text",
    Execute: func(args string, ctx ext.Context) (string, error) {
        ctx.PrintInfo("You said: " + args)
        return "", nil
    },
    // Optional tab-completion:
    Complete: func(prefix string, ctx ext.Context) []string {
        return []string{"hello", "world"}
    },
})
```

Slash commands run in a dedicated goroutine (not a `tea.Cmd`), so they can safely block on prompts, I/O, etc.

---

## Registering Keyboard Shortcuts

```go
api.RegisterShortcut(ext.ShortcutDef{
    Key:         "ctrl+alt+p",
    Description: "Toggle plan mode",
}, func(ctx ext.Context) {
    // handler runs when shortcut is pressed
})
```

Handlers run in a goroutine, so they can safely block on prompts or I/O.
`/shortcuts` lists every registered binding, grouped by extension file.

**Key names are normalized.** Modifier order and casing don't matter:
`"Ctrl+Shift+S"`, `"control+shift+s"` and `"shift+ctrl+s"` are the same binding.
`control`, `option`/`opt`, `cmd`/`command` and `win` alias to `ctrl`, `alt`,
`meta` and `super`. Key-name spellings fold too (`escape` → `esc`,
`return` → `enter`, `pgdn`/`pagedown` → `pgdown`).

A shifted key works spelled either way — `"shift+a"` and `"A"` match the same
press, as do `"shift+/"` and `"?"`. A bare single character keeps its case,
since `"A"` and `"a"` are different presses.

| Key | Behaviour |
|-----|-----------|
| `ctrl+c` | **Rejected at load** — Kit consumes it before extensions are consulted, so the handler could never fire |
| `esc`, `ctrl+x`, `pgup`, `pgdown`, `ctrl+home`, `ctrl+end`, `shift+tab`, `enter`, `tab`, `up`, `down` | Accepted, but logs a warning: the shortcut shadows Kit's built-in binding |
| everything else | Fires normally |

An armed `Ctrl+X` leader chord beats shortcuts, so binding `"s"` won't break
`Ctrl+X s`. Shortcuts don't fire during modal prompts, overlays, or message
navigation.

**Prefer modifier combinations.** A bare `"s"` fires on every press of that key
outside a modal — including while typing a slash command.

---

## Registering Options

Options are configurable values resolved from env vars, config, or defaults:

```go
api.RegisterOption(ext.OptionDef{
    Name:        "my-setting",
    Description: "Controls something",
    Default:     "false",
})

// Read at runtime (resolution: env KIT_OPT_MY_SETTING > config options.my-setting > default):
val := ctx.GetOption("my-setting")

// Set at runtime:
ctx.SetOption("my-setting", "true")
```

