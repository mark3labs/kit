# Plan 01: Export Tools and Tool Factories

**Priority**: P0
**Effort**: Medium
**Goal**: Expose built-in tools as public APIs with pre-built instances and factory functions. The Kit CLI app should also consume these exports instead of reaching into `internal/core` directly.

## Background

Pi SDK exports individual tools and tool factories:
- Pre-built: `readTool`, `bashTool`, `editTool`, etc.
- Factories: `createReadTool(cwd)`, `createBashTool(cwd)`, etc.
- Bundles: `allTools`, `codingTools`, `readOnlyTools`

Kit currently keeps all tools internal (`internal/core/`). The agent setup in `internal/agent/agent.go:97` calls `core.AllTools()` directly. After this plan, both SDK users AND the agent use the same public tool constructors.

## Prerequisites

- Plan 00 (Create `pkg/kit/` package)

## Architecture

```
pkg/kit/
├── kit.go              # Kit struct, New(), Prompt(), etc.
├── types.go            # Type aliases
├── tools.go            # NEW: Public tool exports, factories, bundles
├── config.go           # Extracted from cmd
├── setup.go            # Extracted from cmd
internal/core/
├── tools.go            # MODIFY: Add WithWorkDir option
├── read.go             # MODIFY: Accept workdir param
├── write.go            # MODIFY: Accept workdir param
├── bash.go             # MODIFY: Accept workdir param + cmd.Dir
├── edit.go             # MODIFY: Accept workdir param
├── grep.go             # MODIFY: Accept workdir param + cmd.Dir
├── find.go             # MODIFY: Accept workdir param + cmd.Dir
├── ls.go               # MODIFY: Accept workdir param
├── truncate.go         # Unchanged
internal/agent/
├── agent.go            # MODIFY: Use public constructors via core package
```

## Step-by-Step

### Step 1: Add ToolOption pattern to `internal/core/tools.go`

**File**: `internal/core/tools.go`

Add a functional options pattern for tool creation:

```go
// ToolOption configures tool behavior.
type ToolOption func(*toolConfig)

type toolConfig struct {
    workDir string
}

// WithWorkDir sets the working directory for file-based tools.
// If empty, os.Getwd() is used at execution time.
func WithWorkDir(dir string) ToolOption {
    return func(c *toolConfig) {
        c.workDir = dir
    }
}

func applyOptions(opts []ToolOption) toolConfig {
    var cfg toolConfig
    for _, o := range opts {
        o(&cfg)
    }
    return cfg
}
```

Update all collection functions to accept variadic options:

```go
func CodingTools(opts ...ToolOption) []fantasy.AgentTool { ... }
func ReadOnlyTools(opts ...ToolOption) []fantasy.AgentTool { ... }
func AllTools(opts ...ToolOption) []fantasy.AgentTool { ... }
```

### Step 2: Update path resolution to accept workDir

**File**: `internal/core/read.go`

Replace `resolvePath()` at line 134-144 with configurable version:

```go
func resolvePathWithWorkDir(path, workDir string) (string, error) {
    if filepath.IsAbs(path) {
        return filepath.Clean(path), nil
    }
    baseDir := workDir
    if baseDir == "" {
        var err error
        baseDir, err = os.Getwd()
        if err != nil {
            return "", fmt.Errorf("failed to get working directory: %w", err)
        }
    }
    return filepath.Clean(filepath.Join(baseDir, path)), nil
}

// Backward-compat wrapper
func resolvePath(path string) (string, error) {
    return resolvePathWithWorkDir(path, "")
}
```

### Steps 3-9: Update each tool constructor

For each tool (`read.go`, `write.go`, `edit.go`, `bash.go`, `grep.go`, `find.go`, `ls.go`):
- Change `NewXxxTool()` to `NewXxxTool(opts ...ToolOption)`
- Apply `cfg := applyOptions(opts)` in the constructor
- Pass `cfg.workDir` to path resolution or `cmd.Dir`
- For bash/grep/find (subprocess tools): set `cmd.Dir = cfg.workDir` on `exec.CommandContext`
- Existing callers pass no args, so they get default behavior (backward compatible)

### Step 10: Create `pkg/kit/tools.go`

**File**: `pkg/kit/tools.go`

```go
package kit

import (
    "charm.land/fantasy"
    "github.com/mark3labs/kit/internal/core"
)

// Tool is the interface that all Kit tools implement.
type Tool = fantasy.AgentTool

// ToolOption configures tool behavior.
type ToolOption = core.ToolOption

// WithWorkDir sets the working directory for file-based tools.
var WithWorkDir = core.WithWorkDir

// Individual tool constructors
func NewReadTool(opts ...ToolOption) Tool  { return core.NewReadTool(opts...) }
func NewWriteTool(opts ...ToolOption) Tool { return core.NewWriteTool(opts...) }
func NewEditTool(opts ...ToolOption) Tool  { return core.NewEditTool(opts...) }
func NewBashTool(opts ...ToolOption) Tool  { return core.NewBashTool(opts...) }
func NewGrepTool(opts ...ToolOption) Tool  { return core.NewGrepTool(opts...) }
func NewFindTool(opts ...ToolOption) Tool  { return core.NewFindTool(opts...) }
func NewLsTool(opts ...ToolOption) Tool    { return core.NewLsTool(opts...) }

// Tool bundles
func AllTools(opts ...ToolOption) []Tool        { return core.AllTools(opts...) }
func CodingTools(opts ...ToolOption) []Tool     { return core.CodingTools(opts...) }
func ReadOnlyTools(opts ...ToolOption) []Tool   { return core.ReadOnlyTools(opts...) }
```

### Step 11: Add GetTools() to Kit struct

**File**: `pkg/kit/kit.go`

```go
// GetTools returns all tools available to the agent (core + MCP + extensions).
func (m *Kit) GetTools() []Tool {
    return m.agent.GetTools()
}
```

### Step 12: App-as-Consumer — Agent uses SDK tool constructors

This is the key "dog-fooding" step. Currently `internal/agent/agent.go:97` calls `core.AllTools()` directly. After this change, the agent setup should get its tool list from the caller (via `AgentConfig.Tools`) rather than hardcoding `core.AllTools()`.

**File**: `internal/agent/agent.go`

Change the `AgentConfig` struct to accept tools explicitly:

```go
type AgentConfig struct {
    // ... existing fields ...
    CoreTools []fantasy.AgentTool // NEW: if empty, defaults to core.AllTools()
}
```

In `NewAgent()` at line 96-97, change:
```go
// Before:
coreTools := core.AllTools()

// After:
coreTools := agentConfig.CoreTools
if len(coreTools) == 0 {
    coreTools = core.AllTools() // Default fallback
}
```

Then in `pkg/kit/setup.go`, the `SetupAgent` function passes tools from the SDK:

```go
a, err := agent.CreateAgent(ctx, &agent.AgentCreationOptions{
    // ... existing fields ...
    CoreTools: core.AllTools(), // Explicit — could be customized via Options
})
```

And in `pkg/kit/kit.go`, the `Options` struct gets a `Tools` field:

```go
type Options struct {
    // ... existing fields ...
    Tools []Tool // Custom tool set. If empty, AllTools() is used.
}
```

This allows SDK users to pass custom tools:

```go
k, _ := kit.New(ctx, &kit.Options{
    Tools: kit.CodingTools(kit.WithWorkDir("/my/project")),
})
```

### Step 13: Write tests and verify

```bash
go build -o output/kit ./cmd/kit
go test -race ./...
go vet ./...
```

## Files Changed Summary

| Action | File | Change |
|--------|------|--------|
| EDIT | `internal/core/tools.go` | Add ToolOption, WithWorkDir, update collection funcs |
| EDIT | `internal/core/read.go` | resolvePathWithWorkDir, accept opts |
| EDIT | `internal/core/write.go` | Accept opts |
| EDIT | `internal/core/edit.go` | Accept opts |
| EDIT | `internal/core/bash.go` | Accept opts, set cmd.Dir |
| EDIT | `internal/core/grep.go` | Accept opts, set cmd.Dir |
| EDIT | `internal/core/find.go` | Accept opts, set cmd.Dir |
| EDIT | `internal/core/ls.go` | Accept opts |
| CREATE | `pkg/kit/tools.go` | Public tool exports and factories |
| EDIT | `pkg/kit/kit.go` | Add GetTools(), Tools option |
| EDIT | `internal/agent/agent.go` | Accept CoreTools in config instead of hardcoding |
| EDIT | `pkg/kit/setup.go` | Pass tools through to agent creation |

## Verification Checklist

- [ ] `go build -o output/kit ./cmd/kit` succeeds
- [ ] `go test -race ./...` passes (agent still gets default tools)
- [ ] Tools with `WithWorkDir("/tmp")` resolve paths relative to `/tmp`
- [ ] Tools with no options use `os.Getwd()` (backward compatible)
- [ ] SDK users can pass custom tool sets via `kit.Options{Tools: ...}`
- [ ] Agent accepts injected tools instead of hardcoding `core.AllTools()`
