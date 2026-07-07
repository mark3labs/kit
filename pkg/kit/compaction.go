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

// EstimateContextTokens returns the estimated token count of the current
// conversation based on session messages.
func (m *Kit) EstimateContextTokens() int {
	messages := m.session.GetMessages()
	return compaction.EstimateMessageTokens(messages)
}

// reserveTokensForModel returns the response reserve budget: an explicit
// CompactionOptions override when set, otherwise a value adapted to the
// model's output limit (min(16384, maxOutput)).
func (m *Kit) reserveTokensForModel(info *ModelInfo) int {
	if m.compactionOpts != nil && m.compactionOpts.ReserveTokens > 0 {
		return m.compactionOpts.ReserveTokens
	}
	maxOutput := 0
	if info != nil {
		maxOutput = info.Limit.Output
	}
	return compaction.AdaptiveReserveTokens(maxOutput)
}

// ShouldCompact reports whether the conversation is near the model's context
// limit and should be compacted.
// Formula: contextTokens > contextWindow − reserveTokens.
// Returns false if the model's context limit is unknown.
//
// When API-reported token counts are available (after at least one turn),
// the real count is used instead of the text-based heuristic. This is
// significantly more accurate because it includes system prompts, tool
// definitions, and other overhead that the heuristic cannot account for.
// After compaction the baseline is adjusted down by the estimated reduction
// rather than discarded, so the overhead stays accounted for until the next
// API response refreshes the count.
func (m *Kit) ShouldCompact() bool {
	info := m.GetModelInfo()
	if info == nil || info.Limit.Context <= 0 {
		return false
	}

	reserveTokens := m.reserveTokensForModel(info)

	// Prefer the real API-reported token count when available.
	m.lastInputTokensMu.RLock()
	realTokens := m.lastInputTokens
	m.lastInputTokensMu.RUnlock()

	if realTokens > 0 {
		return realTokens > info.Limit.Context-reserveTokens
	}

	// Fall back to text-based heuristic before first turn completes.
	messages := m.session.GetMessages()
	return compaction.ShouldCompact(convertToLLMMessages(messages), info.Limit.Context, reserveTokens)
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
	messages := m.session.GetMessages()

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
// flag distinguishes user-triggered from auto-compaction for hooks/events.
// On failure it emits a CompactionEvent carrying the error so embedders can
// observe the failure path symmetrically with the success path.
func (m *Kit) compactInternal(ctx context.Context, opts *CompactionOptions, customInstructions string, isAutomatic bool) (*CompactionResult, error) {
	result, err := m.compactImpl(ctx, opts, customInstructions, isAutomatic)
	if err != nil {
		m.events.emit(CompactionEvent{Err: err})
	}
	return result, err
}

// compactImpl performs the actual compaction work. On success it emits a
// CompactionEvent via persistAndEmitCompaction.
func (m *Kit) compactImpl(ctx context.Context, opts *CompactionOptions, customInstructions string, isAutomatic bool) (*CompactionResult, error) {
	// Work on a copy so auto-populated model limits never mutate the
	// caller's (or the instance's shared) options.
	var optsCopy CompactionOptions
	if opts != nil {
		optsCopy = *opts
	} else if m.compactionOpts != nil {
		optsCopy = *m.compactionOpts
	}
	opts = &optsCopy

	// Auto-populate model limits if not set; compaction.defaults() adapts
	// the reserve/keep budgets to them (issue #83).
	if info := m.GetModelInfo(); info != nil {
		if opts.ContextWindow <= 0 {
			opts.ContextWindow = info.Limit.Context
		}
		if opts.MaxOutputTokens <= 0 {
			opts.MaxOutputTokens = info.Limit.Output
		}
	}

	messages := m.session.GetMessages()
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

	// Carry forward file tracking and the prior summary (for anchored,
	// incremental re-summarisation) from the previous compaction.
	var prev *compaction.PreviousCompaction
	if lastCompaction := m.session.GetLastCompaction(); lastCompaction != nil {
		prev = &compaction.PreviousCompaction{
			Summary:       lastCompaction.Summary,
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
	entryIDs := m.session.GetContextEntryIDs()
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
// custom summary. It still determines the cut point (using the same
// adaptive budget defaults as regular compaction) and persists a
// CompactionEntry.
func (m *Kit) applyCustomCompaction(summary string, messages []LLMMessage, opts *CompactionOptions) (*CompactionResult, error) {
	originalTokens := compaction.EstimateMessageTokens(convertToLLMMessages(messages))

	resolved := *opts
	resolved.ApplyDefaults()

	cutPoint := compaction.FindCutPoint(convertToLLMMessages(messages), resolved.KeepRecentTokens)
	if cutPoint == 0 {
		cutPoint = len(messages) - 1
		if cutPoint < 1 {
			return nil, nil
		}
	}

	entryIDs := m.session.GetContextEntryIDs()
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
	if _, err := m.session.AppendCompaction(
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

	// Adjust the API-reported token count down by the heuristic reduction
	// instead of zeroing it. Zeroing forced ShouldCompact() and
	// GetContextStats() back onto the text-only heuristic, which ignores
	// the system prompt, tool schemas, and tool traffic — it undercounted
	// badly enough that auto-compaction never re-fired and the next API
	// call could overflow the real context window (issue #80). Keeping the
	// API baseline preserves that overhead; the next API response replaces
	// the estimate with the accurate post-compaction count.
	m.lastInputTokensMu.Lock()
	m.lastInputTokens = adjustPostCompactionTokens(m.lastInputTokens, originalTokens, compactedTokens)
	m.lastInputTokensMu.Unlock()

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

// adjustPostCompactionTokens computes the post-compaction context token
// baseline from the last API-reported count and the heuristic estimates of
// the conversation before and after compaction.
//
// The API-reported count includes overhead the text heuristic cannot see
// (system prompt, tool schemas, tool-call traffic), so rather than discarding
// it we subtract the heuristic reduction (original − compacted) from it:
//
//	adjusted = lastInputTokens − (originalTokens − compactedTokens)
//
// The result is clamped to at least compactedTokens (the context can never be
// smaller than the heuristic estimate of the messages actually kept) and, via
// the non-negative reduction, to at most lastInputTokens.
//
// Returns 0 when lastInputTokens is 0 (no API turn has completed yet), in
// which case callers fall back to the pure heuristic.
func adjustPostCompactionTokens(lastInputTokens, originalTokens, compactedTokens int) int {
	if lastInputTokens <= 0 {
		return 0
	}
	reduction := max(originalTokens-compactedTokens, 0)
	adjusted := max(lastInputTokens-reduction, compactedTokens)
	return adjusted
}

// Conversion helpers are in llm_convert.go.
