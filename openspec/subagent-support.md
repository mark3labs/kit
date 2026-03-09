# First-Class Subagent Support

**Status:** Proposal  
**Author:** AI Assistant  
**Date:** 2026-03-09

## Summary

Add first-class subagent support to Kit, enabling the LLM and extensions to spawn, manage, and orchestrate child Kit instances for parallel task delegation. This builds on proven patterns from existing extensions (`subagent-widget.go`, `kit-kit.go`) and promotes them to SDK/core APIs.

## Motivation

### Current State

Kit already supports subagents through **two working extension implementations**:

1. **`subagent-widget.go`** (807 lines) - Full lifecycle management:
   - Tools: `subagent_create`, `subagent_continue`, `subagent_remove`, `subagent_list`
   - Live widget dashboard showing status, elapsed time, output
   - Conversation history persistence for multi-turn continuations
   - Process management with kill signals

2. **`kit-kit.go`** (870 lines) - Multi-expert parallel research:
   - Domain-specific system prompts from `.kit/agents/kit-kit/*.md`
   - Parallel subprocess execution with goroutines
   - Grid-based dashboard for concurrent agents

Both spawn Kit as subprocesses using `--json --no-session --no-extensions` and work today. However:

- Each extension reimplements subprocess spawning from scratch (~200 lines)
- No SDK API for extensions to spawn subagents easily
- No core tool for LLM-initiated subagent spawning
- No session tree relationships (subagents are ephemeral)
- Provider creation overhead (each subagent creates new HTTP clients)

### Goals

1. **SDK API** - `ctx.SpawnSubagent(config)` for extensions
2. **Core Tool** - Built-in `spawn_subagent` tool for LLM use
3. **Session Hierarchy** - Optional parent-child session linking
4. **Provider Efficiency** - Connection reuse for subagent spawns
5. **Lifecycle Management** - Cancellation propagation, timeouts

## Design

### Phase 1: SDK Convenience API (Easy)

Add a `SpawnSubagent` function to `ext.Context` that wraps the proven subprocess pattern.

#### Extension API (`ext` package)

```go
// SubagentConfig configures a subagent spawn.
type SubagentConfig struct {
    // Prompt is the task/instruction for the subagent.
    Prompt string

    // Model overrides the parent's model (e.g. "anthropic/claude-haiku-3-5-20241022").
    // Empty string uses the parent's current model.
    Model string

    // SystemPrompt provides domain-specific instructions.
    // Empty string uses the default system prompt.
    SystemPrompt string

    // Timeout limits execution time. Zero means no timeout (not recommended).
    Timeout time.Duration

    // OnOutput streams stderr output chunks as the subagent runs.
    // Called from a goroutine; must be safe for concurrent use.
    OnOutput func(chunk string)

    // OnComplete is called when the subagent finishes (success or error).
    // Called from a goroutine; must be safe for concurrent use.
    OnComplete func(result SubagentResult)

    // Blocking, when true, makes SpawnSubagent wait for completion and
    // return the result directly. When false (default), spawns in background
    // and returns immediately with a handle.
    Blocking bool
}

// SubagentResult contains the outcome of a subagent execution.
type SubagentResult struct {
    // Response is the subagent's final text response.
    Response string

    // Error is set if the subagent failed.
    Error error

    // ExitCode is the subprocess exit code (0 = success).
    ExitCode int

    // Elapsed is the total execution time.
    Elapsed time.Duration

    // Usage contains token usage if available.
    Usage *SubagentUsage
}

// SubagentUsage contains token usage from the subagent's run.
type SubagentUsage struct {
    InputTokens  int64
    OutputTokens int64
}

// SubagentHandle provides control over a running subagent.
type SubagentHandle struct {
    // ID is a unique identifier for this subagent instance.
    ID string

    // Kill terminates the subagent process.
    Kill() error

    // Wait blocks until the subagent completes and returns the result.
    Wait() SubagentResult

    // Done returns a channel that closes when the subagent completes.
    Done() <-chan struct{}
}
```

#### Context Addition

```go
type Context struct {
    // ... existing fields ...

    // SpawnSubagent spawns a child Kit instance to perform a task.
    // When config.Blocking is true, blocks until completion and returns
    // the result directly (handle is nil). When false, returns immediately
    // with a handle for monitoring/cancellation.
    SpawnSubagent func(SubagentConfig) (*SubagentHandle, *SubagentResult, error)
}
```

#### Implementation (`internal/extensions/subagent.go`)

```go
package extensions

import (
    "bufio"
    "context"
    "encoding/json"
    "fmt"
    "os"
    "os/exec"
    "sync"
    "time"
)

// subagentJSONOutput matches the JSON envelope from `kit --json`.
type subagentJSONOutput struct {
    Response string `json:"response"`
    Usage    *struct {
        InputTokens  int64 `json:"input_tokens"`
        OutputTokens int64 `json:"output_tokens"`
    } `json:"usage,omitempty"`
}

// SpawnSubagent implements the subagent spawning logic.
func SpawnSubagent(cfg SubagentConfig) (*SubagentHandle, *SubagentResult, error) {
    if cfg.Prompt == "" {
        return nil, nil, fmt.Errorf("prompt is required")
    }

    // Find the kit binary.
    kitBinary := findKitBinary()

    // Build subprocess arguments.
    args := []string{
        "--json",
        "--no-session",
        "--no-extensions",
    }
    if cfg.Model != "" {
        args = append(args, "--model", cfg.Model)
    }
    if cfg.SystemPrompt != "" {
        // Write system prompt to temp file.
        tmpFile, err := os.CreateTemp("", "kit-subagent-*.txt")
        if err != nil {
            return nil, nil, fmt.Errorf("create temp file: %w", err)
        }
        if _, err := tmpFile.WriteString(cfg.SystemPrompt); err != nil {
            tmpFile.Close()
            os.Remove(tmpFile.Name())
            return nil, nil, fmt.Errorf("write system prompt: %w", err)
        }
        tmpFile.Close()
        args = append(args, "--system-prompt", tmpFile.Name())
        // Note: temp file cleanup handled after subprocess exits.
    }
    args = append(args, cfg.Prompt)

    cmd := exec.Command(kitBinary, args...)
    cmd.Env = os.Environ()

    stdout, err := cmd.StdoutPipe()
    if err != nil {
        return nil, nil, fmt.Errorf("stdout pipe: %w", err)
    }
    stderr, err := cmd.StderrPipe()
    if err != nil {
        return nil, nil, fmt.Errorf("stderr pipe: %w", err)
    }

    handle := &SubagentHandle{
        ID:   generateSubagentID(),
        done: make(chan struct{}),
    }

    // Start the subprocess.
    start := time.Now()
    if err := cmd.Start(); err != nil {
        return nil, nil, fmt.Errorf("start subprocess: %w", err)
    }
    handle.proc = cmd.Process

    // Run the subprocess monitoring in a goroutine.
    go func() {
        defer close(handle.done)

        var wg sync.WaitGroup
        var stdoutBuf strings.Builder

        // Read stderr (live output).
        wg.Add(1)
        go func() {
            defer wg.Done()
            scanner := bufio.NewScanner(stderr)
            scanner.Buffer(make([]byte, 256*1024), 256*1024)
            for scanner.Scan() {
                line := scanner.Text()
                if cfg.OnOutput != nil && strings.TrimSpace(line) != "" {
                    cfg.OnOutput(line + "\n")
                }
            }
        }()

        // Read stdout (JSON output).
        scanner := bufio.NewScanner(stdout)
        scanner.Buffer(make([]byte, 256*1024), 256*1024)
        for scanner.Scan() {
            stdoutBuf.WriteString(scanner.Text() + "\n")
        }

        wg.Wait()
        waitErr := cmd.Wait()
        elapsed := time.Since(start)

        // Parse result.
        result := SubagentResult{Elapsed: elapsed}
        if waitErr != nil {
            result.Error = waitErr
            if exitErr, ok := waitErr.(*exec.ExitError); ok {
                result.ExitCode = exitErr.ExitCode()
            } else {
                result.ExitCode = 1
            }
        }

        // Parse JSON output.
        raw := strings.TrimSpace(stdoutBuf.String())
        var parsed subagentJSONOutput
        if raw != "" && json.Unmarshal([]byte(raw), &parsed) == nil {
            result.Response = parsed.Response
            if parsed.Usage != nil {
                result.Usage = &SubagentUsage{
                    InputTokens:  parsed.Usage.InputTokens,
                    OutputTokens: parsed.Usage.OutputTokens,
                }
            }
        } else {
            result.Response = raw
        }

        handle.result = &result

        if cfg.OnComplete != nil {
            cfg.OnComplete(result)
        }
    }()

    if cfg.Blocking {
        // Wait for completion and return result directly.
        <-handle.done
        return nil, handle.result, nil
    }

    return handle, nil, nil
}
```

### Phase 2: Core Tool (Easy)

Add a built-in `spawn_subagent` tool that the LLM can invoke directly.

#### Tool Definition (`internal/tools/subagent.go`)

```go
package tools

import (
    "context"
    "encoding/json"
    "fmt"
    "time"

    "charm.land/fantasy"
    "github.com/mark3labs/kit/internal/extensions"
)

// SubagentTool returns a tool that spawns Kit subagents.
func SubagentTool() fantasy.AgentTool {
    return fantasy.NewTool(
        "spawn_subagent",
        `Spawn a background subagent to perform a task autonomously.

The subagent runs as a separate Kit instance with full tool access. Use this to:
- Delegate independent subtasks that can run in parallel
- Perform research or analysis without blocking your main work
- Execute tasks that benefit from a fresh context window

The subagent result is returned when it completes. For long-running tasks,
consider breaking them into smaller focused subtasks.

Example use cases:
- "Research the authentication patterns in this codebase"
- "Write unit tests for the UserService class"
- "Analyze the performance bottlenecks in the database queries"`,
        func(ctx context.Context, input SubagentToolInput) (string, error) {
            if input.Task == "" {
                return "", fmt.Errorf("task is required")
            }

            timeout := 5 * time.Minute // Default timeout
            if input.TimeoutSeconds > 0 {
                timeout = time.Duration(input.TimeoutSeconds) * time.Second
            }

            _, result, err := extensions.SpawnSubagent(extensions.SubagentConfig{
                Prompt:       input.Task,
                Model:        input.Model,
                SystemPrompt: input.SystemPrompt,
                Timeout:      timeout,
                Blocking:     true,
            })
            if err != nil {
                return "", fmt.Errorf("spawn subagent: %w", err)
            }
            if result.Error != nil {
                return fmt.Sprintf("Subagent failed (exit %d): %v\n\nPartial output:\n%s",
                    result.ExitCode, result.Error, result.Response), nil
            }

            return fmt.Sprintf("Subagent completed in %ds.\n\nResult:\n%s",
                int(result.Elapsed.Seconds()), result.Response), nil
        },
    )
}

// SubagentToolInput defines the parameters for the spawn_subagent tool.
type SubagentToolInput struct {
    // Task is the complete task description for the subagent.
    Task string `json:"task" jsonschema:"description=The complete task description for the subagent to perform"`

    // Model optionally overrides the model (e.g. "anthropic/claude-haiku-3-5-20241022" for faster/cheaper tasks).
    Model string `json:"model,omitempty" jsonschema:"description=Optional model override for the subagent"`

    // SystemPrompt optionally provides domain-specific instructions.
    SystemPrompt string `json:"system_prompt,omitempty" jsonschema:"description=Optional system prompt for domain-specific guidance"`

    // TimeoutSeconds limits execution time (default: 300 = 5 minutes).
    TimeoutSeconds int `json:"timeout_seconds,omitempty" jsonschema:"description=Maximum execution time in seconds (default: 300)"`
}
```

#### Registration

Add to `internal/tools/core.go`:

```go
func CoreTools() []fantasy.AgentTool {
    return []fantasy.AgentTool{
        BashTool(),
        ReadTool(),
        WriteTool(),
        EditTool(),
        // ... existing tools ...
        SubagentTool(), // NEW
    }
}
```

### Phase 3: Session Hierarchy (Medium)

Add optional parent-child session linking so subagent conversations can be:
- Persisted and resumed
- Queried from the parent session
- Visualized in session tree views

#### CLI Flag

```
--parent-session <id>    Link this session to a parent session ID
```

#### Session Header Extension

```go
// SessionHeader in internal/session/entry.go
type SessionHeader struct {
    Type          EntryType `json:"type"`
    ID            string    `json:"id"`
    Version       string    `json:"version"`
    Cwd           string    `json:"cwd"`
    Timestamp     time.Time `json:"timestamp"`
    ParentSession string    `json:"parent_session,omitempty"` // existing
    
    // NEW: For subagent sessions
    ParentSessionID string `json:"parent_session_id,omitempty"` // UUID of parent
    SubagentTask    string `json:"subagent_task,omitempty"`     // Original task prompt
}
```

#### Query Functions

```go
// ListChildSessions returns all sessions that have this session as their parent.
func ListChildSessions(parentID string) ([]SessionInfo, error) {
    allSessions, err := ListAllSessions()
    if err != nil {
        return nil, err
    }
    
    var children []SessionInfo
    for _, s := range allSessions {
        if s.ParentSessionID == parentID {
            children = append(children, s)
        }
    }
    return children, nil
}
```

#### Subagent Config Extension

```go
type SubagentConfig struct {
    // ... existing fields ...
    
    // ParentSessionID links the subagent's session to the parent.
    // When set, the subagent's session is persisted (not ephemeral)
    // and can be queried/resumed later.
    ParentSessionID string
}
```

### Phase 4: Provider Pooling (Medium)

Reduce overhead when spawning multiple subagents by reusing provider connections.

#### Provider Pool (`internal/models/pool.go`)

```go
package models

import (
    "context"
    "sync"
    "time"

    "charm.land/fantasy"
)

// ProviderPool manages reusable LLM provider instances.
type ProviderPool struct {
    mu       sync.RWMutex
    providers map[string]*pooledProvider
    ttl      time.Duration
}

type pooledProvider struct {
    model   fantasy.LanguageModel
    closer  func() error
    created time.Time
    refs    int
}

// NewProviderPool creates a provider pool with the given TTL for idle providers.
func NewProviderPool(ttl time.Duration) *ProviderPool {
    p := &ProviderPool{
        providers: make(map[string]*pooledProvider),
        ttl:       ttl,
    }
    go p.cleanupLoop()
    return p
}

// Get returns a provider for the model string, creating one if needed.
func (p *ProviderPool) Get(ctx context.Context, modelString string) (fantasy.LanguageModel, func(), error) {
    p.mu.Lock()
    defer p.mu.Unlock()
    
    if pp, ok := p.providers[modelString]; ok {
        pp.refs++
        return pp.model, func() { p.release(modelString) }, nil
    }
    
    // Create new provider.
    config := &ProviderConfig{ModelString: modelString}
    result, err := CreateProvider(ctx, config)
    if err != nil {
        return nil, nil, err
    }
    
    p.providers[modelString] = &pooledProvider{
        model:   result.Model,
        closer:  result.Closer.Close,
        created: time.Now(),
        refs:    1,
    }
    
    return result.Model, func() { p.release(modelString) }, nil
}

func (p *ProviderPool) release(modelString string) {
    p.mu.Lock()
    defer p.mu.Unlock()
    
    if pp, ok := p.providers[modelString]; ok {
        pp.refs--
    }
}

func (p *ProviderPool) cleanupLoop() {
    ticker := time.NewTicker(p.ttl / 2)
    defer ticker.Stop()
    
    for range ticker.C {
        p.mu.Lock()
        now := time.Now()
        for key, pp := range p.providers {
            if pp.refs == 0 && now.Sub(pp.created) > p.ttl {
                pp.closer()
                delete(p.providers, key)
            }
        }
        p.mu.Unlock()
    }
}
```

### Phase 5: Advanced Context Sharing (Hard - Future)

> **Note:** This phase requires significant design work and is deferred to a future proposal.

Options to explore:

1. **Selective History Injection** - Pass compressed parent history to subagent
2. **Shared Memory** - IPC mechanism for context sharing between processes
3. **In-Process Subagents** - Run subagents as goroutines sharing the same provider

## Implementation Plan

| Phase | Component | Effort | Files |
|-------|-----------|--------|-------|
| 1 | SDK API | 2-3 days | `internal/extensions/subagent.go`, `internal/extensions/api.go`, `ext/types.go` |
| 2 | Core Tool | 1 day | `internal/tools/subagent.go`, `internal/tools/core.go` |
| 3 | Session Hierarchy | 2-3 days | `internal/session/entry.go`, `internal/session/store.go`, `cmd/root.go` |
| 4 | Provider Pooling | 2-3 days | `internal/models/pool.go`, `internal/agent/agent.go` |
| 5 | Context Sharing | 1-2 weeks | TBD (future proposal) |

**Total for production-ready (Phases 1-4):** ~1-1.5 weeks

## Migration Path

Existing extensions (`subagent-widget.go`, `kit-kit.go`) can migrate incrementally:

```go
// Before: ~150 lines of subprocess management
cmd := exec.Command(kitBinary, args...)
stdout, _ := cmd.StdoutPipe()
stderr, _ := cmd.StderrPipe()
// ... pipe reading, JSON parsing, process lifecycle ...

// After: 10 lines
handle, _, _ := ctx.SpawnSubagent(ext.SubagentConfig{
    Prompt:   task,
    OnOutput: func(chunk string) { state.appendChunk(chunk) },
    OnComplete: func(result ext.SubagentResult) {
        state.setStatus("done")
        ctx.SendMessage(fmt.Sprintf("Subagent finished: %s", result.Response))
    },
})
```

## Testing Strategy

1. **Unit Tests** - `internal/extensions/subagent_test.go`
   - Config validation
   - Timeout handling
   - JSON parsing edge cases

2. **Integration Tests** - `internal/extensions/subagent_integration_test.go`
   - Actual subprocess spawning
   - Cancellation propagation
   - Multiple concurrent subagents

3. **Extension Tests** - Update `subagent-widget.go` to use new API
   - Verify backward compatibility
   - Measure code reduction

4. **tmux TUI Tests** - Per AGENTS.md testing pattern
   ```bash
   tmux new-session -d -s subtest "output/kit -e examples/extensions/subagent-widget.go"
   tmux send-keys -t subtest '/sub research authentication patterns' Enter
   sleep 10
   tmux capture-pane -t subtest -p | grep -q "Subagent #1"
   ```

## Security Considerations

1. **Process Isolation** - Subagents run as separate processes (existing behavior)
2. **No Extension Inheritance** - `--no-extensions` prevents recursive loading
3. **Timeout Enforcement** - Default 5-minute timeout prevents runaway processes
4. **Resource Limits** - Consider adding `--max-steps` to subagent invocations

## Open Questions

1. **Should subagents inherit MCP servers?** Currently no (`--no-extensions`). Adding `--inherit-mcp` flag could enable tool sharing.

2. **Result size limits?** Current extensions truncate at 8-16KB. Should the SDK have a configurable limit?

3. **Parallel execution limits?** Should there be a max concurrent subagents setting to prevent resource exhaustion?

4. **Billing/quota tracking?** For teams with usage limits, should parent sessions aggregate subagent token usage?

## Appendix: Existing Extension Patterns

### subagent-widget.go Key Patterns

```go
// Process lifecycle management
type subState struct {
    Proc    *os.Process // active process for killing
    Status  string      // "running", "done", "error"
    History string      // conversation history for /subcont
}

// Subprocess invocation
args := []string{"--json", "--no-session", "--no-extensions", prompt}
cmd := exec.Command(kitBinary, args...)
cmd.Start()

// Dual pipe reading
go func() { /* stderr -> live widget updates */ }()
go func() { /* stdout -> JSON result */ }()

// Result delivery
ctx.SendMessage(fmt.Sprintf("Subagent #%d finished: %s", id, result))
```

### kit-kit.go Key Patterns

```go
// Parallel expert execution
var wg sync.WaitGroup
for _, q := range queries {
    wg.Add(1)
    go func(expert, question string) {
        defer wg.Done()
        out, code, elapsed := queryExpert(expert, question)
        // ...
    }(q.Expert, q.Question)
}
wg.Wait()

// Domain-specific system prompts from files
def := parseAgentFile(filepath.Join(dir, "ext-expert.md"))
args = append(args, "--system-prompt", tmpFile.Name())
```
