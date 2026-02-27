package kit

import (
	"context"
	"fmt"

	"github.com/mark3labs/kit/internal/compaction"
)

// ContextStats contains current context usage information.
type ContextStats struct {
	EstimatedTokens int     // Estimated token count of the current conversation
	ContextLimit    int     // Model's context window size (tokens), 0 if unknown
	UsagePercent    float64 // Fraction of context used (0.0–1.0), 0 if limit unknown
	MessageCount    int     // Number of messages in the conversation
}

// EstimateContextTokens returns the estimated token count of the current
// conversation based on tree session messages.
func (m *Kit) EstimateContextTokens() int {
	messages := m.treeSession.GetFantasyMessages()
	return compaction.EstimateMessageTokens(messages)
}

// ShouldCompact reports whether the conversation is near the model's context
// limit and should be compacted. Uses Pi's formula:
// contextTokens > contextWindow − reserveTokens.
// Returns false if the model's context limit is unknown.
func (m *Kit) ShouldCompact() bool {
	info := m.GetModelInfo()
	if info == nil || info.Limit.Context <= 0 {
		return false
	}

	reserveTokens := 16384
	if m.compactionOpts != nil && m.compactionOpts.ReserveTokens > 0 {
		reserveTokens = m.compactionOpts.ReserveTokens
	}

	messages := m.treeSession.GetFantasyMessages()
	return compaction.ShouldCompact(messages, info.Limit.Context, reserveTokens)
}

// GetContextStats returns current context usage statistics including
// estimated token count, context limit, usage percentage, and message count.
func (m *Kit) GetContextStats() ContextStats {
	messages := m.treeSession.GetFantasyMessages()
	estimated := compaction.EstimateMessageTokens(messages)

	stats := ContextStats{
		EstimatedTokens: estimated,
		MessageCount:    len(messages),
	}

	info := m.GetModelInfo()
	if info != nil && info.Limit.Context > 0 {
		stats.ContextLimit = info.Limit.Context
		stats.UsagePercent = float64(estimated) / float64(info.Limit.Context)
	}

	return stats
}

// Compact summarises older messages to reduce context usage. If opts is nil,
// the instance's CompactionOptions (or sensible defaults) are used. The
// model's context window is automatically populated from the model registry
// when opts.ContextWindow is 0.
//
// customInstructions is optional text appended to the summary prompt (e.g.
// "Focus on the API design decisions"). Pass "" for the default prompt.
//
// After compaction, the tree session is cleared and replaced with the
// compacted messages (summary + preserved recent messages).
func (m *Kit) Compact(ctx context.Context, opts *CompactionOptions, customInstructions string) (*CompactionResult, error) {
	if opts == nil {
		if m.compactionOpts != nil {
			opts = m.compactionOpts
		} else {
			opts = &CompactionOptions{}
		}
	}

	// Auto-populate context window from model info if not set.
	if opts.ContextWindow <= 0 {
		if info := m.GetModelInfo(); info != nil {
			opts.ContextWindow = info.Limit.Context
		}
	}

	messages := m.treeSession.GetFantasyMessages()
	if len(messages) < 2 {
		return nil, fmt.Errorf("cannot compact: need at least 2 messages")
	}

	model := m.agent.GetModel()
	result, newMessages, err := compaction.Compact(ctx, model, messages, *opts, customInstructions)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, nil
	}

	// Replace the session contents with the compacted messages.
	// Reset the tree leaf and re-add the compacted messages.
	m.treeSession.ResetLeaf()
	if err := m.treeSession.AddFantasyMessages(newMessages); err != nil {
		return nil, fmt.Errorf("failed to persist compacted messages: %w", err)
	}

	m.events.emit(CompactionEvent{
		Summary:         result.Summary,
		OriginalTokens:  result.OriginalTokens,
		CompactedTokens: result.CompactedTokens,
		MessagesRemoved: result.MessagesRemoved,
	})

	return result, nil
}
