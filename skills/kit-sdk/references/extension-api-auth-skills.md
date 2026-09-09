# Kit SDK: Extension API, Authentication, and Skills

> Part of the `kit-sdk` skill. Read `SKILL.md` first for the overview and critical constraints.

## Extension API

The `Extensions()` method returns an `ExtensionAPI` interface that groups all extension-related functionality. This is the primary way to interact with extension state from the SDK.

```go
extAPI := host.Extensions()

// Check if extensions are loaded
if extAPI.HasExtensions() {
    // Context management
    extAPI.SetContext(extensions.Context{...})
    ctx := extAPI.GetContext()
    extAPI.UpdateContextModel("anthropic/claude-sonnet-4-5-20250929")

    // Widgets, headers, footers
    extAPI.SetWidget(extensions.WidgetConfig{...})
    extAPI.RemoveWidget("widget-id")
    extAPI.SetHeader(extensions.HeaderFooterConfig{...})
    extAPI.SetFooter(extensions.HeaderFooterConfig{...})

    // Status bar
    extAPI.SetStatus(extensions.StatusBarEntry{...})
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

## Authentication

```go
cm, _ := kit.NewCredentialManager()
hasKey := kit.HasAnthropicCredentials()
apiKey := kit.GetAnthropicAPIKey() // stored creds → ANTHROPIC_API_KEY env var
```

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

