package kit

import (
	"context"
	"errors"
	"fmt"

	"charm.land/fantasy"
)

// This file implements reactive compaction (issue #85): when a provider call
// fails with a context-length/overflow error, Kit automatically compacts the
// conversation and replays the turn once instead of surfacing a hard failure.
//
// Proactive compaction (ShouldCompact() before a turn) relies on token
// estimates that inevitably drift from real tokenizer counts — especially
// with tool-heavy traffic, non-English text, and images. A single huge
// mid-turn tool result can also overflow the context even when the turn
// started well under the limit. The reactive path is the safety net that
// makes an imprecise estimator acceptable in practice.

// isContextOverflow reports whether err is (or classifies as) a provider
// context-window overflow.
func isContextOverflow(err error) bool {
	return err != nil && errors.Is(ClassifyProviderError(err), ErrContextOverflow)
}

// prepareOverflowRetry runs reactive compaction after a provider
// context-overflow error and rebuilds the request context for a single
// replay. Media attachments in the rebuilt context are replaced with text
// placeholders — they are typically the largest contributors and cannot be
// shrunk by summarisation, so dropping them from the replayed request gives
// it the best chance to fit. The session itself is not modified beyond the
// appended compaction entry.
//
// Returns the replay messages on success. On failure (compaction failed,
// was cancelled, or produced no usable context) it returns a non-nil error
// describing why recovery was impossible; callers wrap it together with the
// original provider error so the surfaced failure both explains the failed
// recovery and preserves the [ErrContextOverflow] classification.
func (m *Kit) prepareOverflowRetry(ctx context.Context) ([]fantasy.Message, error) {
	if _, err := m.compactInternal(ctx, m.compactionOpts, "", true); err != nil {
		return nil, fmt.Errorf("compaction failed: %w", err)
	}

	// Rebuild the context from the session: the compaction summary now
	// replaces the summarised prefix, and any completed steps persisted
	// from the failed attempt are included so the replay resumes rather
	// than restarts.
	messages, _, _ := m.session.BuildContext()
	messages = stripMediaParts(messages)

	// Re-run ContextPrepare hooks on the rebuilt context, mirroring the
	// initial attempt so extensions observe every outgoing request. Strip
	// media from the hook result too — a hook may replace the messages and
	// reintroduce attachments, which would defeat the recovery.
	if hookResult := m.contextPrepare.run(ContextPrepareHook{Messages: messages}); hookResult != nil && hookResult.Messages != nil {
		messages = stripMediaParts(hookResult.Messages)
	}

	if len(messages) == 0 {
		return nil, fmt.Errorf("compaction produced an empty context")
	}
	return messages, nil
}

// stripMediaParts returns a copy of messages with file/media attachments
// replaced by short text placeholders. Message slices and non-file parts are
// shared, not copied; only messages containing file parts get a new Content
// slice.
func stripMediaParts(messages []fantasy.Message) []fantasy.Message {
	out := make([]fantasy.Message, len(messages))
	for i, msg := range messages {
		out[i] = msg
		hasFile := false
		for _, part := range msg.Content {
			if _, ok := part.(fantasy.FilePart); ok {
				hasFile = true
				break
			}
		}
		if !hasFile {
			continue
		}
		replaced := make([]fantasy.MessagePart, 0, len(msg.Content))
		for _, part := range msg.Content {
			fp, ok := part.(fantasy.FilePart)
			if !ok {
				replaced = append(replaced, part)
				continue
			}
			name := fp.Filename
			if name == "" {
				name = fp.MediaType
			}
			if name == "" {
				name = "attachment"
			}
			replaced = append(replaced, fantasy.TextPart{
				Text: fmt.Sprintf("[attachment %q removed after context overflow]", name),
			})
		}
		out[i].Content = replaced
	}
	return out
}
