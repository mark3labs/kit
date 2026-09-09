# Kit SDK: Configuration

> Part of the `kit-sdk` skill. Read `SKILL.md` first for the overview and critical constraints.

## Configuration

The SDK loads config identically to the CLI:

1. Explicit `ConfigFile` in Options (highest priority)
2. `.kit.yml` in current directory
3. `~/.kit.yml` in home directory
4. Environment variables with `KIT_` prefix (`KIT_MODEL`, etc.)
5. Provider-specific env vars (`ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, etc.)

Config files support `${ENV_VAR}` expansion.

```go
// Initialize config manually (usually not needed — kit.New handles this)
kit.InitConfig("/path/to/config.yml", false)
// Same, but skip project-local .kit.yml discovery
kit.InitConfigWithOptions(kit.ConfigInitOptions{ConfigFile: "/path/to/config.yml", Bare: true})
```

