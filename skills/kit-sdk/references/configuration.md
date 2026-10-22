# Kit SDK: Configuration

> Part of the `kit-sdk` skill. Read `SKILL.md` first for the overview and critical constraints.

## Configuration

File selection is separate from field precedence. An explicit `ConfigFile` loads only that file. Otherwise discovery prefers project `.kit.yml`/`.kit.yaml`/`.kit.json` to the home configuration. `SkipConfig` skips files but not environment variables.

Effective generation/provider settings use explicit SDK values, then environment, then configuration, per-model defaults and final fallbacks. Zero/nil fields can mean unset; use pointer fields for explicit zero sampling values. Stored credentials beat provider environment variables; explicit provider API-key overrides win over resolved credentials. See [SDK option precedence](https://go-kit.dev/sdk/options#precedence).

Config files support `${ENV_VAR}` expansion.

```go
// Initialize config manually (usually not needed — kit.New handles this)
kit.InitConfig("/path/to/config.yml", false)
// Same, but skip project-local .kit.yml discovery
kit.InitConfigWithOptions(kit.ConfigInitOptions{ConfigFile: "/path/to/config.yml", Bare: true})
```

