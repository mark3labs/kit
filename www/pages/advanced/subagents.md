---
title: Subagents
description: Multi-agent orchestration with Kit subagents.
---

# Subagents

Kit supports multi-agent orchestration through both subprocess spawning and in-process subagents.

## Subprocess pattern

Spawn Kit as a subprocess for isolated agent execution:

```bash
kit "Analyze codebase" \
    --json \
    --no-session \
    --no-extensions \
    --quiet \
    --model anthropic/claude-haiku-3-5-20241022
```

Key flags for subprocess usage:

| Flag | Purpose |
|------|---------|
| `--quiet` | Stdout only, no TUI |
| `--no-session` | Ephemeral, no persistence |
| `--no-extensions` | Prevent recursive extension loading |
| `--json` | Machine-readable output |
| `--system-prompt` | Custom system prompt (string or file path) |

Positional arguments are the prompt. `@file` arguments attach file content as context.

## Built-in spawn_subagent tool

Kit includes a built-in `spawn_subagent` tool that the LLM can use to delegate tasks to independent child agents:

```
spawn_subagent(
    task: "Analyze the test files and summarize coverage",
    model: "anthropic/claude-haiku-3-5-20241022",   // optional
    system_prompt: "You are a test analysis expert.",  // optional
    timeout_seconds: 300                               // optional, max 1800
)
```

Subagents run as separate in-process Kit instances with full tool access (except spawning further subagents, to prevent infinite recursion). They can run in parallel.

## Extension subagents

Extensions can spawn subagents programmatically:

```go
result := ctx.SpawnSubagent(ext.SubagentConfig{
    Task:         "Review this code for security issues",
    Model:        "anthropic/claude-sonnet-4-5-20250929",
    SystemPrompt: "You are a security auditor.",
})
```

## Go SDK subagents

The SDK provides in-process subagent spawning:

```go
result, err := host.Subagent(ctx, kit.SubagentConfig{
    Task:         "Summarize the changes in this PR",
    Model:        "anthropic/claude-haiku-3-5-20241022",
    SystemPrompt: "You are a code reviewer.",
    Timeout:      5 * time.Minute,
})
```
