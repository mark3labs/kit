# Kit SDK: Context, Compaction, and In-Process Subagents

> Part of the `kit-sdk` skill. Read `SKILL.md` first for the overview and critical constraints.

## Context & Compaction

```go
tokens := host.EstimateContextTokens()  // heuristic token count
shouldCompact := host.ShouldCompact()    // true if near context limit
// ShouldCompact() uses API-reported token counts (including cache tokens)
// when available, falling back to text-based heuristic before the first turn.

stats := host.GetContextStats()
// stats.EstimatedTokens — uses API-reported count when available (more accurate;
//                          includes system prompts, tool definitions, cache tokens)
// stats.ContextLimit    — model's context window size
// stats.UsagePercent    — fraction used (0.0–1.0)
// stats.MessageCount    — number of messages

// Manual compaction
result, err := host.Compact(ctx, nil, "") // nil opts = defaults, "" = default prompt
// result.Summary, result.OriginalTokens, result.CompactedTokens, result.MessagesRemoved

// Auto-compaction via Options
host, _ := kit.New(ctx, &kit.Options{
    AutoCompact: true,
    CompactionOptions: &kit.CompactionOptions{
        ReserveTokens:   16384,
        KeepRecentTokens: 4096,
        ContextWindow:   200000,
    },
})
```

**Reactive compaction (always on):** independent of `AutoCompact`, when a
provider call fails with a context-overflow error the turn loop compacts the
conversation and replays the turn once. Media attachments in the replayed
request are replaced with text placeholders. If the replay still overflows,
the turn fails with `kit.ErrContextOverflow` ("conversation too large to
compact"). `AutoCompact: true` additionally compacts *proactively* before
turns that near the limit.

---

## In-Process Subagents

Spawn child Kit instances without subprocess overhead:

```go
result, err := host.Subagent(ctx, kit.SubagentConfig{
    Prompt:       "Analyze the test files and summarize coverage",
    Model:        "anthropic/claude-haiku-3-5-20241022", // empty = parent's model
    SystemPrompt: "You are a test analysis expert.",
    Tools:        nil,           // nil = SubagentTools() (all except subagent)
    NoSession:    true,          // ephemeral
    Timeout:      2 * time.Minute, // 0 = 5 minute default
    OnEvent: func(e kit.Event) {
        // Real-time events from the child agent
        if chunk, ok := e.(kit.MessageUpdateEvent); ok {
            fmt.Print(chunk.Chunk)
        }
    },
})
// result.Response, result.Error, result.SessionID, result.StopReason
// result.Usage (*kit.LLMUsage), result.Elapsed (time.Duration)
```

### Subscribing to subagent events from parent

```go
host.OnToolCall(func(e kit.ToolCallEvent) {
    if e.ToolName == "subagent" {
        host.SubscribeSubagent(e.ToolCallID, func(child kit.Event) {
            // Real-time events scoped to this subagent
        })
    }
})
```

### Named agents

Named agents are reusable subagent presets discovered from markdown files
(`.agents/agents/*.md`, `.kit/agents/*.md`, `~/.config/kit/agents/*.md`)
plus the built-ins `general` and `explore`. The filename is the agent name;
YAML frontmatter sets `description` (required), `model`, `tools` (allowlist),
`temperature`, `timeout` (seconds), `hidden`, `disabled`; the body is the
system prompt. They are advertised in the subagent tool description so the
LLM can delegate by name.

```go
defs := host.GetAgents()             // discovered definitions (snapshot)
def, ok := host.GetAgent("explore")  // lookup by name

result, err := host.Subagent(ctx, kit.SubagentConfig{
    Prompt: "Map out the session persistence flow",
    Agent:  "explore", // preset prompt + read-only tool allowlist
    // Explicit Model/SystemPrompt/Timeout/Temperature override the preset.
})

// Standalone discovery without a Kit instance:
defs, err := kit.LoadAgentDefinitions("") // "" = current working directory
```

Disable discovery with `--no-agents`, the `no-agents` config key, `KIT_NO_AGENTS=true`, or `kit.Options{NoAgents: true}`.

