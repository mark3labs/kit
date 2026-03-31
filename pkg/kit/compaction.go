package kit

import (
	"context"
	"errors"
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

// defaultReserveTokens is the number of tokens to keep free in the context
// window as a safety margin during compaction checks.
const defaultReserveTokens = 16384

// EstimateContextTokens returns the estimated token count of the current
// conversation based on tree session messages.
func (m *Kit) EstimateContextTokens() int {
	messages := m.treeSession.GetLLMMessages()
	return compaction.EstimateMessageTokens(messages)
}

// ShouldCompact reports whether the conversation is near the model's context
// limit and should be compacted.
// Formula: contextTokens > contextWindow − reserveTokens.
// Returns false if the model's context limit is unknown.
func (m *Kit) ShouldCompact() bool {
	info := m.GetModelInfo()
	if info == nil || info.Limit.Context <= 0 {
		return false
	}

	reserveTokens := defaultReserveTokens
	if m.compactionOpts != nil && m.compactionOpts.ReserveTokens > 0 {
		reserveTokens = m.compactionOpts.ReserveTokens
	}

	messages := m.treeSession.GetLLMMessages()
	return compaction.ShouldCompact(messages, info.Limit.Context, reserveTokens)
}

// GetContextStats returns current context usage statistics including
// estimated token count, context limit, usage percentage, and message count.
//
// When API-reported token counts are available (after at least one turn),
// EstimatedTokens uses the real input token count from the most recent API
// response. This is significantly more accurate than the text-based heuristic
// because it includes system prompts, tool definitions, and other overhead
// that the heuristic cannot account for.
func (m *Kit) GetContextStats() ContextStats {
	messages := m.treeSession.GetLLMMessages()

	// Prefer the real API-reported input token count when available.
	m.lastInputTokensMu.RLock()
	estimated := m.lastInputTokens
	m.lastInputTokensMu.RUnlock()
	if estimated == 0 {
		// Fall back to heuristic before first turn completes.
		estimated = compaction.EstimateMessageTokens(messages)
	}

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
// Compaction is non-destructive: a CompactionEntry is appended to the session
// tree recording the summary and the first kept entry ID. Old messages remain
// on disk but are skipped when building the LLM context — the summary is
// injected in their place.
func (m *Kit) Compact(ctx context.Context, opts *CompactionOptions, customInstructions string) (*CompactionResult, error) {
	return m.compactInternal(ctx, opts, customInstructions, false)
}

// compactInternal is the shared compaction implementation. The isAutomatic
// flag distinguishes auto-triggered compaction from manual /compact.
func (m *Kit) compactInternal(ctx context.Context, opts *CompactionOptions, customInstructions string, isAutomatic bool) (*CompactionResult, error) {
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

	messages := m.treeSession.GetLLMMessages()
	if len(messages) < 2 {
		return nil, fmt.Errorf("cannot compact: need at least 2 messages")
	}

	// Run before-compact hook — extensions can cancel or provide a custom summary.
	if m.beforeCompact.hasHooks() {
		stats := m.GetContextStats()
		if hookResult := m.beforeCompact.run(BeforeCompactHook{
			EstimatedTokens: stats.EstimatedTokens,
			ContextLimit:    stats.ContextLimit,
			UsagePercent:    stats.UsagePercent,
			MessageCount:    stats.MessageCount,
			IsAutomatic:     isAutomatic,
		}); hookResult != nil {
			if hookResult.Cancel {
				reason := hookResult.Reason
				if reason == "" {
					reason = "compaction cancelled by extension"
				}
				return nil, errors.New(reason)
			}
			// Extension provided a custom summary — use it directly.
			if hookResult.Summary != "" {
				return m.applyCustomCompaction(hookResult.Summary, messages, opts)
			}
		}
	}

	// Carry forward file tracking from previous compaction.
	var prev *compaction.PreviousCompaction
	if lastCompaction := m.treeSession.GetLastCompaction(); lastCompaction != nil {
		prev = &compaction.PreviousCompaction{
			ReadFiles:     lastCompaction.ReadFiles,
			ModifiedFiles: lastCompaction.ModifiedFiles,
		}
	}

	model := m.agent.GetModel()

	// Create a streaming callback to emit chunks as events.
	streamCallback := func(delta string) error {
		// Emit MessageUpdateEvent to the UI for streaming display.
		m.events.emit(MessageUpdateEvent{Chunk: delta})
		return nil
	}

	result, _, err := compaction.Compact(ctx, model, messages, *opts, customInstructions, prev, streamCallback)
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, nil
	}

	// Non-destructive: append a CompactionEntry to the session tree instead
	// of clearing and rewriting messages.
	entryIDs := m.treeSession.GetContextEntryIDs()
	firstKeptEntryID := ""
	if result.CutPoint >= 0 && result.CutPoint < len(entryIDs) {
		firstKeptEntryID = entryIDs[result.CutPoint]
	}

	if err := m.persistAndEmitCompaction(result.Summary, firstKeptEntryID, result.OriginalTokens, result.CompactedTokens, result.MessagesRemoved, result.ReadFiles, result.ModifiedFiles); err != nil {
		return nil, err
	}

	return result, nil
}

// applyCustomCompaction handles compaction when an extension provides a
// custom summary. It still determines the cut point and persists a
// CompactionEntry.
func (m *Kit) applyCustomCompaction(summary string, messages []LLMMessage, opts *CompactionOptions) (*CompactionResult, error) {
	originalTokens := compaction.EstimateMessageTokens(messages)

	cutPoint := compaction.FindCutPoint(messages, opts.KeepRecentTokens)
	if cutPoint == 0 {
		cutPoint = len(messages) - 1
		if cutPoint < 1 {
			return nil, nil
		}
	}

	entryIDs := m.treeSession.GetContextEntryIDs()
	firstKeptEntryID := ""
	if cutPoint >= 0 && cutPoint < len(entryIDs) {
		firstKeptEntryID = entryIDs[cutPoint]
	}

	// Estimate new token count.
	summaryTokens := compaction.EstimateMessageTokens([]LLMMessage{{
		Role:    LLMRoleSystem,
		Content: []LLMMessagePart{LLMTextPart{Text: summary}},
	}})
	recentTokens := compaction.EstimateMessageTokens(messages[cutPoint:])
	compactedTokens := summaryTokens + recentTokens

	result := &CompactionResult{
		Summary:         summary,
		OriginalTokens:  originalTokens,
		CompactedTokens: compactedTokens,
		MessagesRemoved: cutPoint,
	}

	if err := m.persistAndEmitCompaction(summary, firstKeptEntryID, originalTokens, compactedTokens, cutPoint, nil, nil); err != nil {
		return nil, err
	}

	return result, nil
}

// persistAndEmitCompaction writes a CompactionEntry to the session tree and
// emits a CompactionEvent. It is the single implementation shared by
// compactInternal and applyCustomCompaction.
func (m *Kit) persistAndEmitCompaction(
	summary, firstKeptEntryID string,
	originalTokens, compactedTokens, messagesRemoved int,
	readFiles, modifiedFiles []string,
) error {
	if _, err := m.treeSession.AppendCompaction(
		summary,
		firstKeptEntryID,
		originalTokens,
		compactedTokens,
		messagesRemoved,
		readFiles,
		modifiedFiles,
	); err != nil {
		return fmt.Errorf("failed to persist compaction entry: %w", err)
	}
	m.events.emit(CompactionEvent{
		Summary:         summary,
		OriginalTokens:  originalTokens,
		CompactedTokens: compactedTokens,
		MessagesRemoved: messagesRemoved,
		ReadFiles:       readFiles,
		ModifiedFiles:   modifiedFiles,
	})
	return nil
}

// Conversion helpers are in llm_convert.go.
