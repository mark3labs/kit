# Plan 00: Create `pkg/kit` SDK Package & Extract Init from `cmd`

**Priority**: P0
**Effort**: Medium-High
**Goal**: Create `pkg/kit` as the canonical SDK package; extract shared logic from `cmd/` so both the CLI and external users consume the same API

## Background

Currently the SDK lives in `sdk/` and imports `cmd/` to access `InitConfig`, `SetupAgent`, etc. This creates a circular dependency problem: if the CLI app wants to consume the SDK, `cmd` would import `sdk` which imports `cmd`.

The fix is two-fold:
1. Move the SDK to `pkg/kit/` (idiomatic Go for public library packages)
2. Extract configuration/agent-setup logic from `cmd/` into `pkg/kit/` so both the CLI and SDK share the same code path without circular deps

### Architecture Before

```
main.go → cmd/ → internal/agent, internal/session, internal/config, ...

sdk/kit.go → cmd.InitConfig()     ← SDK depends on cmd (problem!)
           → cmd.SetupAgent()
           → internal/session
```

### Architecture After

```
cmd/kit/main.go → cmd/ → pkg/kit/ → internal/agent, internal/session, ...
                                   ← CLI consumes SDK

pkg/kit/        → internal/agent, internal/session, internal/config, ...
                ← External users consume SDK

internal/app/   → pkg/kit/        ← App consumes SDK (gradual migration)
                → internal/ui/    ← App owns UI only
```

## Prerequisites

- None. This is the foundation for all other plans.

## Step-by-Step

### Step 1: Create `pkg/kit/` directory

```bash
mkdir -p pkg/kit
```

### Step 2: Extract config-loading logic from `cmd/root.go` into `pkg/kit/config.go`

The two functions `InitConfig()` and `LoadConfigWithEnvSubstitution()` currently live in `cmd/root.go` and depend on package-level variables (`configFile`, `debugMode`). Extract them as pure functions that accept parameters.

**File**: Create `pkg/kit/config.go`

```go
package kit

import (
    "fmt"
    "os"
    "strings"

    "github.com/mark3labs/kit/internal/config"
    "github.com/spf13/viper"
)

// InitConfig initializes the viper configuration system.
// It searches for config files in standard locations and loads them with
// environment variable substitution.
//
// configFile: explicit config file path (empty = search defaults)
// debug: if true, print warnings about missing configs
func InitConfig(configFile string, debug bool) error {
    if configFile != "" {
        return LoadConfigWithEnvSubstitution(configFile)
    }

    // Ensure a config file exists (create default if none found)
    if err := config.EnsureConfigExists(); err != nil {
        if debug {
            fmt.Fprintf(os.Stderr, "Warning: Could not create default config file: %v\n", err)
        }
    }

    home, err := os.UserHomeDir()
    if err != nil {
        return fmt.Errorf("error finding home directory: %w", err)
    }

    viper.AddConfigPath(".")
    viper.AddConfigPath(home)

    configNames := []string{".kit"}
    configLoaded := false

    for _, name := range configNames {
        viper.SetConfigName(name)
        if err := viper.ReadInConfig(); err == nil {
            configPath := viper.ConfigFileUsed()
            if err := LoadConfigWithEnvSubstitution(configPath); err != nil {
                if strings.Contains(err.Error(), "environment variable substitution failed") {
                    return fmt.Errorf("error reading config file '%s': %w", configPath, err)
                }
                continue
            }
            configLoaded = true
            break
        }
    }

    if !configLoaded && debug {
        fmt.Fprintf(os.Stderr, "No config file found in current directory or home directory\n")
    }

    viper.SetEnvPrefix("KIT")
    viper.AutomaticEnv()
    return nil
}

// LoadConfigWithEnvSubstitution loads a config file with ${ENV_VAR} expansion.
func LoadConfigWithEnvSubstitution(configPath string) error {
    rawContent, err := os.ReadFile(configPath)
    if err != nil {
        return fmt.Errorf("failed to read config file: %w", err)
    }

    substituter := &config.EnvSubstituter{}
    processedContent, err := substituter.SubstituteEnvVars(string(rawContent))
    if err != nil {
        return fmt.Errorf("config env substitution failed: %w", err)
    }

    configType := "yaml"
    if strings.HasSuffix(configPath, ".json") {
        configType = "json"
    }

    config.SetConfigPath(configPath)
    viper.SetConfigType(configType)
    return viper.ReadConfig(strings.NewReader(processedContent))
}
```

**Source**: Extracted from `cmd/root.go:119-213`

### Step 3: Extract agent setup logic from `cmd/setup.go` into `pkg/kit/setup.go`

Move `BuildProviderConfig`, `AgentSetupOptions`, `AgentSetupResult`, `SetupAgent`, and `setupExtensions` to the SDK. The key change: replace the `quietFlag` package-level variable dependency with an explicit `Quiet` field on `AgentSetupOptions`.

**File**: Create `pkg/kit/setup.go`

```go
package kit

import (
    "context"
    "fmt"

    "charm.land/fantasy"
    "github.com/mark3labs/kit/internal/agent"
    "github.com/mark3labs/kit/internal/config"
    "github.com/mark3labs/kit/internal/extensions"
    "github.com/mark3labs/kit/internal/hooks"
    "github.com/mark3labs/kit/internal/models"
    "github.com/mark3labs/kit/internal/tools"
    "github.com/spf13/viper"
)

// AgentSetupOptions configures agent creation.
type AgentSetupOptions struct {
    MCPConfig         *config.Config
    ShowSpinner       bool
    SpinnerFunc       agent.SpinnerFunc
    UseBufferedLogger bool
    Quiet             bool // Replaces cmd's quietFlag package var
}

// AgentSetupResult contains the created agent and related components.
type AgentSetupResult struct {
    Agent          *agent.Agent
    BufferedLogger *tools.BufferedDebugLogger
    ExtRunner      *extensions.Runner
}

// BuildProviderConfig creates a ProviderConfig from the current viper state.
func BuildProviderConfig() (*models.ProviderConfig, string, error) {
    systemPrompt, err := config.LoadSystemPrompt(viper.GetString("system-prompt"))
    if err != nil {
        return nil, "", fmt.Errorf("failed to load system prompt: %w", err)
    }

    temperature := float32(viper.GetFloat64("temperature"))
    topP := float32(viper.GetFloat64("top-p"))
    topK := int32(viper.GetInt("top-k"))
    numGPU := int32(viper.GetInt("num-gpu-layers"))
    mainGPU := int32(viper.GetInt("main-gpu"))

    cfg := &models.ProviderConfig{
        ModelString:    viper.GetString("model"),
        SystemPrompt:   systemPrompt,
        ProviderAPIKey: viper.GetString("provider-api-key"),
        ProviderURL:    viper.GetString("provider-url"),
        MaxTokens:      viper.GetInt("max-tokens"),
        Temperature:    &temperature,
        TopP:           &topP,
        TopK:           &topK,
        StopSequences:  viper.GetStringSlice("stop-sequences"),
        NumGPU:         &numGPU,
        MainGPU:        &mainGPU,
        TLSSkipVerify:  viper.GetBool("tls-skip-verify"),
    }

    return cfg, systemPrompt, nil
}

// SetupAgent creates an agent from the current configuration state.
func SetupAgent(ctx context.Context, opts AgentSetupOptions) (*AgentSetupResult, error) {
    modelConfig, systemPrompt, err := BuildProviderConfig()
    if err != nil {
        return nil, err
    }

    var debugLogger tools.DebugLogger
    var bufferedLogger *tools.BufferedDebugLogger
    if viper.GetBool("debug") {
        if opts.UseBufferedLogger {
            bufferedLogger = tools.NewBufferedDebugLogger(true)
            debugLogger = bufferedLogger
        } else {
            debugLogger = tools.NewSimpleDebugLogger(true)
        }
    }

    var extRunner *extensions.Runner
    var extOpts extensionCreationOpts
    if !viper.GetBool("no-extensions") {
        var extErr error
        extRunner, extOpts, extErr = loadExtensions()
        if extErr != nil {
            fmt.Printf("Warning: Failed to load extensions: %v\n", extErr)
        }
    }

    a, err := agent.CreateAgent(ctx, &agent.AgentCreationOptions{
        ModelConfig:      modelConfig,
        MCPConfig:        opts.MCPConfig,
        SystemPrompt:     systemPrompt,
        MaxSteps:         viper.GetInt("max-steps"),
        StreamingEnabled: viper.GetBool("stream"),
        ShowSpinner:      opts.ShowSpinner,
        Quiet:            opts.Quiet,
        SpinnerFunc:      opts.SpinnerFunc,
        DebugLogger:      debugLogger,
        ToolWrapper:      extOpts.toolWrapper,
        ExtraTools:       extOpts.extraTools,
    })
    if err != nil {
        return nil, fmt.Errorf("failed to create agent: %w", err)
    }

    return &AgentSetupResult{
        Agent:          a,
        ExtRunner:      extRunner,
        BufferedLogger: bufferedLogger,
    }, nil
}

// unexported helpers

type extensionCreationOpts struct {
    toolWrapper func([]fantasy.AgentTool) []fantasy.AgentTool
    extraTools  []fantasy.AgentTool
}

func loadExtensions() (*extensions.Runner, extensionCreationOpts, error) {
    extraPaths := viper.GetStringSlice("extension")
    loaded, err := extensions.LoadExtensions(extraPaths)
    if err != nil {
        return nil, extensionCreationOpts{}, err
    }

    hooksCfg, _ := hooks.LoadHooksConfig()
    if hooksCfg != nil && len(hooksCfg.Hooks) > 0 {
        compat := extensions.HooksAsExtension(hooksCfg)
        if compat != nil {
            loaded = append([]extensions.LoadedExtension{*compat}, loaded...)
        }
    }

    if len(loaded) == 0 {
        return nil, extensionCreationOpts{}, nil
    }

    runner := extensions.NewRunner(loaded)
    wrapper := func(tools []fantasy.AgentTool) []fantasy.AgentTool {
        return extensions.WrapToolsWithExtensions(tools, runner)
    }
    extTools := extensions.ExtensionToolsAsFantasy(runner.RegisteredTools())

    return runner, extensionCreationOpts{
        toolWrapper: wrapper,
        extraTools:  extTools,
    }, nil
}
```

**Source**: Extracted from `cmd/setup.go:28-185`

### Step 4: Move SDK core (`sdk/kit.go`, `sdk/types.go`) into `pkg/kit/`

Move the files and update them to import from the local package (no more `cmd` import):

**File**: Move `sdk/kit.go` to `pkg/kit/kit.go`

Key changes:
- `package sdk` → `package kit`
- Remove `import "github.com/mark3labs/kit/cmd"` entirely
- Replace `cmd.InitConfig()` → `InitConfig(...)` (same package)
- Replace `cmd.LoadConfigWithEnvSubstitution(...)` → `LoadConfigWithEnvSubstitution(...)` (same package)
- Replace `cmd.SetupAgent(...)` → `SetupAgent(...)` (same package)
- Replace `cmd.AgentSetupOptions{...}` → `AgentSetupOptions{...}` (same package)

**File**: Move `sdk/types.go` to `pkg/kit/types.go`

Key change: `package sdk` → `package kit`

### Step 5: Move `main.go` to `cmd/kit/main.go`

**File**: Create `cmd/kit/main.go` with the current `main.go` contents

```go
package main

import (
    "context"
    "fmt"
    "os"

    "github.com/charmbracelet/fang"
    "github.com/mark3labs/kit/cmd"
)

var version = "dev"

func main() {
    if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
        fmt.Println(version)
        os.Exit(0)
    }
    ctx := context.Background()
    rootCmd := cmd.GetRootCommand(version)
    if err := fang.Execute(ctx, rootCmd); err != nil {
        os.Exit(1)
    }
}
```

Delete root `main.go`.

### Step 6: Update `cmd/root.go` to delegate to `pkg/kit`

**File**: `cmd/root.go`

Replace the `InitConfig` function body with a call to the SDK:

```go
import kit "github.com/mark3labs/kit/pkg/kit"

func InitConfig() {
    if err := kit.InitConfig(configFile, debugMode); err != nil {
        fmt.Fprintf(os.Stderr, "%v\n", err)
        os.Exit(1)
    }
}
```

Keep `LoadConfigWithEnvSubstitution` as a thin wrapper or remove it entirely (callers use `kit.LoadConfigWithEnvSubstitution` directly).

### Step 7: Update `cmd/setup.go` to delegate to `pkg/kit`

**File**: `cmd/setup.go`

Replace `BuildProviderConfig`, `SetupAgent`, etc. with thin wrappers that inject CLI-specific state:

```go
import kit "github.com/mark3labs/kit/pkg/kit"

// BuildProviderConfig delegates to the SDK.
func BuildProviderConfig() (*models.ProviderConfig, string, error) {
    return kit.BuildProviderConfig()
}

// SetupAgent delegates to the SDK, injecting CLI-specific quiet flag.
func SetupAgent(ctx context.Context, opts AgentSetupOptions) (*AgentSetupResult, error) {
    result, err := kit.SetupAgent(ctx, kit.AgentSetupOptions{
        MCPConfig:         opts.MCPConfig,
        ShowSpinner:       opts.ShowSpinner,
        SpinnerFunc:       opts.SpinnerFunc,
        UseBufferedLogger: opts.UseBufferedLogger,
        Quiet:             quietFlag, // Inject CLI package-level state
    })
    if err != nil {
        return nil, err
    }
    // Map SDK result back to cmd types (or make cmd use SDK types directly)
    return &AgentSetupResult{
        Agent:          result.Agent,
        BufferedLogger: result.BufferedLogger,
        ExtRunner:      result.ExtRunner,
    }, nil
}
```

**Alternative (cleaner)**: Remove `cmd` wrapper types entirely and have all callers in `cmd/` use `kit.AgentSetupOptions` and `kit.AgentSetupResult` directly. This is the app-as-consumer pattern.

### Step 8: Update `.goreleaser.yaml`

Add `main: ./cmd/kit`:

```yaml
builds:
  - id: kit
    main: ./cmd/kit
    binary: kit
    ldflags:
      - -s -w -X main.version={{.Version}}
```

### Step 9: Update examples and tests

**Move**: `sdk/examples/` → `pkg/kit/examples/`

Update all imports:
- `"github.com/mark3labs/kit/sdk"` → `kit "github.com/mark3labs/kit/pkg/kit"`
- All `sdk.` → `kit.`

**Move**: `sdk/kit_test.go` → `pkg/kit/kit_test.go`
- `package sdk_test` → `package kit_test`
- Update import path

### Step 10: Clean up old `sdk/` directory

Remove `sdk/` entirely after all files are moved.

### Step 11: Update documentation

- `README.md`: Update import paths to `"github.com/mark3labs/kit/pkg/kit"`
- Move `sdk/README.md` → `pkg/kit/README.md` with updated paths

### Step 12: Verify

```bash
go build -o output/kit ./cmd/kit
go test -race ./...
go vet ./...
```

Confirm no remaining imports of `"github.com/mark3labs/kit/sdk"` or `"github.com/mark3labs/kit/cmd"` from `pkg/kit/`.

## Files Changed Summary

| Action | File | Change |
|--------|------|--------|
| CREATE | `pkg/kit/config.go` | Extracted InitConfig, LoadConfigWithEnvSubstitution |
| CREATE | `pkg/kit/setup.go` | Extracted BuildProviderConfig, SetupAgent, AgentSetupOptions/Result |
| MOVE | `sdk/kit.go` → `pkg/kit/kit.go` | Change package, remove cmd import |
| MOVE | `sdk/types.go` → `pkg/kit/types.go` | Change package |
| MOVE | `sdk/kit_test.go` → `pkg/kit/kit_test.go` | Change package and imports |
| MOVE | `sdk/examples/` → `pkg/kit/examples/` | Update imports |
| CREATE | `cmd/kit/main.go` | New CLI entrypoint |
| DELETE | `main.go` | Moved to cmd/kit/ |
| EDIT | `cmd/root.go` | Delegate InitConfig to pkg/kit |
| EDIT | `cmd/setup.go` | Delegate SetupAgent to pkg/kit (or use SDK types directly) |
| EDIT | `.goreleaser.yaml` | Add `main: ./cmd/kit` |
| DELETE | `sdk/` | Entire directory after moves |

## Dependency Graph After

```
cmd/kit/main.go  → cmd/
cmd/             → pkg/kit/     (CLI uses SDK)
                 → internal/app/ (CLI uses app for TUI)
                 → internal/ui/  (CLI uses UI)
pkg/kit/         → internal/agent, internal/session, internal/config, ...
                 (SDK uses internals, never cmd)
internal/app/    → pkg/kit/     (App uses SDK — gradual migration)
                 → internal/ui/  (App owns TUI)
```

**No circular dependencies.**

## Verification Checklist

- [ ] `go build -o output/kit ./cmd/kit` succeeds
- [ ] `go test -race ./...` passes
- [ ] `go vet ./...` clean
- [ ] No `pkg/kit/` file imports `cmd/`
- [ ] `cmd/` files import `pkg/kit/` for shared logic
- [ ] No remaining references to `"github.com/mark3labs/kit/sdk"`
- [ ] Examples compile with new import path
- [ ] `.goreleaser.yaml` builds from `./cmd/kit`
- [ ] CI passes (`go test ./...`)
