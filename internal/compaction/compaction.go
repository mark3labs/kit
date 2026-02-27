// Package compaction provides context window management with token estimation,
// compaction triggers, and LLM-based conversation summarization.
package compaction

import (
	"context"
	"fmt"
	"strings"

	"charm.land/fantasy"
)

// EstimateTokens provides a rough token count (~4 chars per token).
func EstimateTokens(text string) int {
	return len(text) / 4
}

// EstimateMessageTokens estimates total tokens across a slice of fantasy messages
// by summing the estimated tokens for every text part.
func EstimateMessageTokens(messages []fantasy.Message) int {
	total := 0
	for _, msg := range messages {
		for _, part := range msg.Content {
			if tp, ok := part.(fantasy.TextPart); ok {
				total += EstimateTokens(tp.Text)
			}
		}
	}
	return total
}

// ShouldCompact reports whether the conversation exceeds the threshold
// percentage of the context limit. thresholdPct should be in the range 0.0–1.0
// (e.g. 0.8 means 80%).
func ShouldCompact(messages []fantasy.Message, contextLimit int, thresholdPct float64) bool {
	if contextLimit <= 0 || thresholdPct <= 0 {
		return false
	}
	estimated := EstimateMessageTokens(messages)
	return float64(estimated) >= float64(contextLimit)*thresholdPct
}

// CompactionResult contains statistics from a compaction operation.
type CompactionResult struct {
	Summary         string // LLM-generated summary of compacted messages
	OriginalTokens  int    // Estimated token count before compaction
	CompactedTokens int    // Estimated token count after compaction
	MessagesRemoved int    // Number of messages replaced by the summary
}

// CompactionOptions configures compaction behaviour.
type CompactionOptions struct {
	ContextLimit   int     // Model's context window size (tokens)
	ThresholdPct   float64 // Trigger threshold (0.0–1.0), default 0.8
	PreserveRecent int     // Number of recent messages to keep, default 10
	SummaryPrompt  string  // Custom summary prompt (empty = use default)
}

// defaults fills zero-value fields with sensible defaults.
func (o *CompactionOptions) defaults() {
	if o.ThresholdPct <= 0 {
		o.ThresholdPct = 0.8
	}
	if o.PreserveRecent <= 0 {
		o.PreserveRecent = 10
	}
}

// defaultSummaryPrompt is the system prompt used to summarise older messages.
const defaultSummaryPrompt = `You are a conversation summarizer. Summarize the following conversation messages into a concise summary that preserves:
1. Key decisions and conclusions reached
2. Important context and facts established
3. Current task state and progress
4. Any pending actions or open questions

Be concise but thorough. Output only the summary text, no preamble.`

// FindCutPoint determines the index at which to cut messages for compaction.
// Messages before the cut point will be summarised; messages from the cut
// point onward are preserved. Returns 0 if no compaction is needed.
func FindCutPoint(messages []fantasy.Message, preserveRecent int) int {
	if preserveRecent <= 0 {
		preserveRecent = 10
	}
	if len(messages) <= preserveRecent {
		return 0 // not enough messages to compact
	}
	return len(messages) - preserveRecent
}

// Compact summarises older messages using the LLM, returning the compaction
// result and a new message slice (summary message + preserved recent messages).
//
// The model parameter is the same fantasy.LanguageModel used for regular
// generation — compaction creates a disposable fantasy agent with no tools to
// produce the summary.
func Compact(
	ctx context.Context,
	model fantasy.LanguageModel,
	messages []fantasy.Message,
	opts CompactionOptions,
) (*CompactionResult, []fantasy.Message, error) {
	opts.defaults()

	cutPoint := FindCutPoint(messages, opts.PreserveRecent)
	if cutPoint == 0 {
		return nil, messages, nil // nothing to compact
	}

	oldMessages := messages[:cutPoint]
	recentMessages := messages[cutPoint:]
	originalTokens := EstimateMessageTokens(messages)

	// Build a textual representation of the messages to summarise.
	var sb strings.Builder
	for _, msg := range oldMessages {
		sb.WriteString(string(msg.Role))
		sb.WriteString(": ")
		for _, part := range msg.Content {
			if tp, ok := part.(fantasy.TextPart); ok {
				sb.WriteString(tp.Text)
			}
		}
		sb.WriteString("\n\n")
	}
	conversationText := sb.String()

	// Use the provided (or default) summary prompt.
	summaryPrompt := opts.SummaryPrompt
	if summaryPrompt == "" {
		summaryPrompt = defaultSummaryPrompt
	}

	// Create a lightweight agent (no tools) just for summarisation.
	summaryAgent := fantasy.NewAgent(model,
		fantasy.WithSystemPrompt(summaryPrompt),
	)
	result, err := summaryAgent.Generate(ctx, fantasy.AgentCall{
		Prompt: conversationText,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("compaction summarisation failed: %w", err)
	}

	summaryText := result.Response.Content.Text()
	if summaryText == "" {
		return nil, nil, fmt.Errorf("compaction produced an empty summary")
	}

	// Build the new message list: summary as a system message + preserved recent.
	summaryMessage := fantasy.Message{
		Role: fantasy.MessageRoleSystem,
		Content: []fantasy.MessagePart{
			fantasy.TextPart{
				Text: fmt.Sprintf("[Conversation summary — earlier messages were compacted]\n\n%s", summaryText),
			},
		},
	}

	newMessages := make([]fantasy.Message, 0, 1+len(recentMessages))
	newMessages = append(newMessages, summaryMessage)
	newMessages = append(newMessages, recentMessages...)

	compactedTokens := EstimateMessageTokens(newMessages)

	return &CompactionResult{
		Summary:         summaryText,
		OriginalTokens:  originalTokens,
		CompactedTokens: compactedTokens,
		MessagesRemoved: len(oldMessages),
	}, newMessages, nil
}
