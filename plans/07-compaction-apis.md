# Plan 07: Compaction APIs

**Priority**: P2
**Effort**: Medium
**Goal**: Add context window management with token estimation, compaction triggers, and summarization. CLI `--compact` flag should use the SDK.

## Background

Pi exports `compact()`, `generateBranchSummary()`, `shouldCompact()`, `calculateContextTokens()`. Kit has no compaction — only `len(text)/4` estimation in `ui/usage_tracker.go:69` for display. This plan adds compaction from scratch, designed SDK-first so the CLI consumes it.

## Prerequisites

- Plan 00 (Create `pkg/kit/`)
- Plan 03 (Event subscriber system)
- Plan 04 (Enhanced session management — tree sessions for branch summaries)

## Step-by-Step

### Step 1: Create `internal/compaction/` package

**File**: `internal/compaction/compaction.go` (new)

```go
package compaction

// EstimateTokens provides a rough token count (~4 chars per token).
func EstimateTokens(text string) int {
    return len(text) / 4
}

// EstimateMessageTokens estimates tokens for a slice of fantasy messages.
func EstimateMessageTokens(messages []fantasy.Message) int { ... }

// ShouldCompact checks if conversation exceeds threshold percentage of limit.
func ShouldCompact(messages []fantasy.Message, contextLimit int, thresholdPct float64) bool { ... }

// CompactionResult contains statistics from a compaction.
type CompactionResult struct {
    Summary         string
    OriginalTokens  int
    CompactedTokens int
    MessagesRemoved int
}

// CompactionOptions configures compaction behavior.
type CompactionOptions struct {
    ContextLimit   int     // Model's context window (tokens)
    ThresholdPct   float64 // Trigger threshold (0.0-1.0), default 0.8
    PreserveRecent int     // Recent messages to keep, default 10
    SummaryPrompt  string  // Custom summary prompt (empty = default)
}

// FindCutPoint determines where to cut for compaction.
func FindCutPoint(messages []fantasy.Message, preserveRecent int) int { ... }

// Compact summarizes older messages using the LLM.
func Compact(ctx context.Context, model fantasy.LanguageModel, messages []fantasy.Message, opts CompactionOptions) (*CompactionResult, []fantasy.Message, error) { ... }
```

Full implementations as described in the original plan (summarize messages before cut point using LLM, return summary + preserved recent messages).

### Step 2: Export compaction in SDK

**File**: `pkg/kit/types.go` — add type aliases:

```go
type CompactionResult = compaction.CompactionResult
type CompactionOptions = compaction.CompactionOptions
```

### Step 3: Add Compact() and context methods to Kit

**File**: `pkg/kit/kit.go`

```go
// Compact summarizes older messages to reduce context usage.
func (m *Kit) Compact(ctx context.Context, opts *CompactionOptions) (*CompactionResult, error) { ... }

// EstimateContextTokens returns estimated token count of current conversation.
func (m *Kit) EstimateContextTokens() int { ... }

// ShouldCompact checks if conversation is near the context limit.
func (m *Kit) ShouldCompact() bool { ... }

// ContextStats returns current context usage statistics.
type ContextStats struct {
    EstimatedTokens int
    ContextLimit    int
    UsagePercent    float64
    MessageCount    int
}

func (m *Kit) GetContextStats() ContextStats { ... }
```

### Step 4: Add auto-compaction option

```go
type Options struct {
    // ... existing fields ...
    AutoCompact       bool              // Auto-compact when near limit
    CompactionOptions *CompactionOptions // Config for auto-compact
}
```

In `Prompt()`, check before generation:
```go
if m.autoCompact && m.ShouldCompact() {
    m.Compact(ctx, m.compactionOpts) // best-effort
}
```

### Step 5: App-as-Consumer — CLI `--compact` flag uses SDK

Currently `cmd/root.go` has a `compactMode` flag (line 37) but compaction is not implemented. After this plan:

**File**: `cmd/root.go`

```go
// Map --compact flag to SDK option
if compactMode {
    kitOpts.AutoCompact = true
}
```

The CLI could also expose a `/compact` slash command in interactive mode that calls `kit.Compact()`:

```go
// In interactive command handler:
case "/compact":
    result, err := k.Compact(ctx, nil)
    if err != nil {
        fmt.Printf("Compaction failed: %v\n", err)
    } else {
        fmt.Printf("Compacted: %d messages removed, %d -> %d tokens\n",
            result.MessagesRemoved, result.OriginalTokens, result.CompactedTokens)
    }
```

The usage tracker in `internal/ui/usage_tracker.go` should also use `kit.EstimateContextTokens()` instead of its own `len(text)/4` heuristic — single source of truth.

### Step 6: Write tests and verify

```bash
go build -o output/kit ./cmd/kit
go test -race ./...
```

## Files Changed Summary

| Action | File | Change |
|--------|------|--------|
| CREATE | `internal/compaction/compaction.go` | Core compaction logic |
| EDIT | `pkg/kit/types.go` | Export CompactionResult, CompactionOptions |
| EDIT | `pkg/kit/kit.go` | Compact(), ShouldCompact(), GetContextStats(), auto-compact |
| EDIT | `cmd/root.go` | Map --compact to SDK option |
| EDIT | `internal/ui/usage_tracker.go` | Use SDK token estimation |

## Verification Checklist

- [ ] Token estimation is reasonable
- [ ] `ShouldCompact()` triggers near context limit
- [ ] `Compact()` reduces message count and tokens
- [ ] Auto-compaction triggers before prompts
- [ ] CLI `--compact` flag maps to `kit.Options{AutoCompact: true}`
- [ ] Usage tracker uses SDK estimation
