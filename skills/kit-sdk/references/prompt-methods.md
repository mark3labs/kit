# Kit SDK: Prompt Methods

> Part of the `kit-sdk` skill. Read `SKILL.md` first for the overview and critical constraints.

## Prompt Methods

### Simple prompt — string in, string out

```go
response, err := host.Prompt(ctx, "Explain this code")
```

### Full result with usage stats

```go
result, err := host.PromptResult(ctx, "Analyze this file")
// result.Response     — assistant's text
// result.StopReason   — "stop", "length", "tool-calls", "error", etc.
// result.SessionID    — session UUID
// result.TotalUsage   — aggregate tokens across all steps (*kit.LLMUsage)
//                        LLMUsage{InputTokens, OutputTokens, TotalTokens,
//                                 ReasoningTokens, CacheCreationTokens, CacheReadTokens}
// result.FinalUsage   — tokens from last API call only (*kit.LLMUsage)
//                        For context window fill, sum: InputTokens + CacheReadTokens +
//                        CacheCreationTokens + OutputTokens (with prompt caching,
//                        InputTokens alone understates the context)
// result.Messages     — full updated conversation ([]kit.LLMMessage)
//                        LLMMessage{Role kit.LLMMessageRole, Content string}
```

### Multimodal with file attachments

```go
files := []kit.LLMFilePart{{
    Filename:  "screenshot.png",
    MediaType: "image/png",
    Data:      imageBytes,
}}
result, err := host.PromptResultWithFiles(ctx, "What's in this image?", files)
```

### Per-call system message injection

```go
response, err := host.PromptWithOptions(ctx, "Review this PR", kit.PromptOptions{
    SystemMessage: "Focus on security vulnerabilities only.",
})
```

### System-level steering (no visible user message)

```go
response, err := host.Steer(ctx, "Switch to a more formal tone")
```

### Continue without new input

```go
response, err := host.FollowUp(ctx, "") // empty = "Continue."
```

### Multiple user messages in one turn

```go
result, err := host.PromptResultWithMessages(ctx, []string{
    "Here is the code:",
    "@file.go", // content from earlier
    "Please review it.",
})
```

