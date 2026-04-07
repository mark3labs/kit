---
title: SDK Options
description: Configuration options for the Kit Go SDK.
---

# SDK Options

Pass an `Options` struct to `kit.New()` to configure the Kit instance.

## Full options reference

```go
host, err := kit.New(ctx, &kit.Options{
    // Model
    Model:        "ollama/llama3",
    SystemPrompt: "You are a helpful bot",
    ConfigFile:   "/path/to/config.yml",

    // Behavior
    MaxSteps:     10,
    Streaming:    true,
    Quiet:        true,
    Debug:        true,

    // Session
    SessionPath:  "./session.jsonl",
    SessionDir:   "/custom/sessions/",
    Continue:     true,
    NoSession:    true,

    // Tools
    Tools:            []kit.Tool{...},     // Replace default tool set entirely
    ExtraTools:       []kit.Tool{...},     // Add tools alongside defaults
    DisableCoreTools: true,                // Use no core tools (0 tools, for chat-only)

    // Configuration
    SkipConfig:   true,                   // Skip .kit.yml files (viper defaults + env vars still apply)

    // Compaction
    AutoCompact:  true,

    // Skills
    Skills:       []string{"/path/to/skill.md"},
    SkillsDir:    "/path/to/skills/",
})
```

## Options fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `Model` | `string` | config default | Model string (provider/model format) |
| `SystemPrompt` | `string` | — | System prompt text or file path |
| `ConfigFile` | `string` | `~/.kit.yml` | Path to config file |
| `MaxSteps` | `int` | `0` | Max agent steps (0 = unlimited) |
| `Streaming` | `bool` | `true` | Enable streaming output |
| `Quiet` | `bool` | `false` | Suppress output |
| `Debug` | `bool` | `false` | Enable debug logging |
| `SessionPath` | `string` | — | Open a specific session file |
| `SessionDir` | `string` | — | Base directory for session discovery |
| `Continue` | `bool` | `false` | Resume most recent session |
| `NoSession` | `bool` | `false` | Ephemeral mode (no persistence) |
| `Tools` | `[]Tool` | — | Replace the entire default tool set |
| `ExtraTools` | `[]Tool` | — | Additional tools alongside core/MCP/extension tools |
| `DisableCoreTools` | `bool` | `false` | Use no core tools (0 tools, for chat-only) |
| `SkipConfig` | `bool` | `false` | Skip .kit.yml file loading |
| `AutoCompact` | `bool` | `false` | Auto-compact when near context limit |
| `CompactionOptions` | `*CompactionOptions` | — | Configuration for auto-compaction |
| `Skills` | `[]string` | — | Explicit skill files/dirs to load |
| `SkillsDir` | `string` | — | Override default skills directory |

## Tool configuration

**`Tools`** replaces ALL default tools (core + MCP + extension). **`ExtraTools`** adds tools alongside the defaults. Use `Tools` to restrict capabilities; use `ExtraTools` to extend them.

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
