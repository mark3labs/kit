// Package compaction provides context window management with token estimation,
// compaction triggers, and LLM-based conversation summarization.
//
// The algorithm mirrors Pi's approach: preserve a token budget of recent
// messages (KeepRecentTokens, default 20 000) rather than a fixed message
// count. Auto-compaction fires when estimated context usage exceeds
// contextWindow − ReserveTokens.
package compaction

import (
	"context"
	"fmt"
	"strings"

	"charm.land/fantasy"
)

// ---------------------------------------------------------------------------
// Token estimation
// ---------------------------------------------------------------------------

// EstimateTokens provides a rough token count (~4 chars per token).
func EstimateTokens(text string) int {
	return len(text) / 4
}

// EstimateMessageTokens estimates total tokens across a slice of fantasy
// messages by summing the estimated tokens for every text part.
func EstimateMessageTokens(messages []fantasy.Message) int {
	total := 0
	for _, msg := range messages {
		total += estimateSingleMessageTokens(msg)
	}
	return total
}

// estimateSingleMessageTokens returns the estimated token count for one
// message.
func estimateSingleMessageTokens(msg fantasy.Message) int {
	total := 0
	for _, part := range msg.Content {
		if tp, ok := part.(fantasy.TextPart); ok {
			total += EstimateTokens(tp.Text)
		}
	}
	return total
}

// ---------------------------------------------------------------------------
// Auto-compact trigger
// ---------------------------------------------------------------------------

// ShouldCompact reports whether auto-compaction should fire. It uses Pi's
// formula: contextTokens > contextWindow − reserveTokens.
func ShouldCompact(messages []fantasy.Message, contextWindow int, reserveTokens int) bool {
	if contextWindow <= 0 || reserveTokens <= 0 {
		return false
	}
	estimated := EstimateMessageTokens(messages)
	return estimated > contextWindow-reserveTokens
}

// ---------------------------------------------------------------------------
// Options & defaults
// ---------------------------------------------------------------------------

// CompactionResult contains statistics from a compaction operation.
type CompactionResult struct {
	Summary         string // LLM-generated summary of compacted messages
	OriginalTokens  int    // Estimated token count before compaction
	CompactedTokens int    // Estimated token count after compaction
	MessagesRemoved int    // Number of messages replaced by the summary
}

// CompactionOptions configures compaction behaviour. Pi-style token-based
// defaults are applied for zero-value fields.
type CompactionOptions struct {
	ContextWindow    int    // Model's context window size (tokens)
	ReserveTokens    int    // Tokens to reserve for LLM response, default 16384
	KeepRecentTokens int    // Recent tokens to preserve (not summarised), default 20000
	SummaryPrompt    string // Custom summary prompt (empty = use default)
}

// defaults fills zero-value fields with sensible Pi-style defaults.
func (o *CompactionOptions) defaults() {
	if o.ReserveTokens <= 0 {
		o.ReserveTokens = 16384
	}
	if o.KeepRecentTokens <= 0 {
		o.KeepRecentTokens = 20000
	}
}

// defaultSystemPrompt is the system prompt sent to the summarisation LLM.
// Matches Pi's compaction system prompt.
const defaultSystemPrompt = `You are a context summarization assistant. Your task is to read a conversation between a user and an AI coding assistant, then produce a structured summary following the exact format specified.

Do NOT continue the conversation. Do NOT respond to any questions in the conversation. ONLY output the structured summary.`

// defaultSummaryPrompt is the user prompt appended after the serialised
// conversation. Matches Pi's initial-compaction format.
const defaultSummaryPrompt = `The messages above are a conversation to summarize. Create a structured context checkpoint summary that another LLM will use to continue the work.

Use this EXACT format:

## Goal
[What is the user trying to accomplish? Can be multiple items if the session covers different tasks.]

## Constraints & Preferences
- [Any constraints, preferences, or requirements mentioned by user]
- [Or "(none)" if none were mentioned]

## Progress
### Done
- [x] [Completed tasks/changes]

### In Progress
- [ ] [Current work]

### Blocked
- [Issues preventing progress, if any]

## Key Decisions
- **[Decision]**: [Brief rationale]

## Next Steps
1. [Ordered list of what should happen next]

## Critical Context
- [Any data, examples, or references needed to continue]
- [Or "(none)" if not applicable]

Keep each section concise. Preserve exact file paths, function names, and error messages.`

// ---------------------------------------------------------------------------
// Cut point (token-based, Pi-style)
// ---------------------------------------------------------------------------

// isValidCutPoint returns true if the message at index i is a valid place to
// split the conversation. Tool-role messages (tool results) must stay with
// their preceding assistant tool-call, so they are never valid cut points.
func isValidCutPoint(msg fantasy.Message) bool {
	return msg.Role != fantasy.MessageRoleTool
}

// FindCutPoint walks backward from the end of messages, accumulating tokens
// until the keepRecentTokens budget is filled. Returns the index that
// separates "old" messages (0..cutPoint-1, to be summarised) from "recent"
// messages (cutPoint..end, to be preserved).
//
// Returns 0 if there are fewer than 2 messages or all messages fit within
// the keep budget.
func FindCutPoint(messages []fantasy.Message, keepRecentTokens int) int {
	if len(messages) < 2 {
		return 0
	}
	if keepRecentTokens <= 0 {
		keepRecentTokens = 20000
	}

	accumulated := 0

	for i := len(messages) - 1; i >= 0; i-- {
		accumulated += estimateSingleMessageTokens(messages[i])
		if accumulated > keepRecentTokens {
			cut := i + 1

			// If the last message alone exceeds the budget, keep it
			// anyway and summarise everything before it.
			if cut >= len(messages) {
				cut = len(messages) - 1
			}

			// Land on a valid cut point — scan forward past tool-result
			// messages (they must stay with their preceding tool call).
			for cut < len(messages) && !isValidCutPoint(messages[cut]) {
				cut++
			}
			if cut >= len(messages) {
				return 0
			}

			// Need at least 2 messages before the cut to produce a
			// meaningful summary.
			if cut < 2 {
				return 0
			}
			return cut
		}
	}

	// All messages fit within the budget — nothing to compact.
	return 0
}

// forceCutPoint returns a cut point that keeps only the last non-tool
// message, summarising everything before it. Used when the budget-based
// FindCutPoint returns 0 but the caller wants to compact anyway (manual
// /compact). Returns 0 if no valid cut exists.
func forceCutPoint(messages []fantasy.Message) int {
	// Walk backward to find the last valid (non-tool) message boundary.
	for i := len(messages) - 1; i >= 2; i-- {
		if isValidCutPoint(messages[i]) {
			return i
		}
	}
	return 0
}

// ---------------------------------------------------------------------------
// Message serialisation (Pi-style)
// ---------------------------------------------------------------------------

// roleLabel returns a human-readable label for a fantasy message role,
// matching Pi's serialisation format.
func roleLabel(role fantasy.MessageRole) string {
	switch role {
	case fantasy.MessageRoleUser:
		return "[User]"
	case fantasy.MessageRoleAssistant:
		return "[Assistant]"
	case fantasy.MessageRoleTool:
		return "[Tool result]"
	case fantasy.MessageRoleSystem:
		return "[System]"
	default:
		return "[" + string(role) + "]"
	}
}

// serializeMessages converts a slice of fantasy messages into a plain-text
// representation suitable for sending to the summarisation LLM. The format
// mirrors Pi's compaction serialisation.
func serializeMessages(messages []fantasy.Message) string {
	var sb strings.Builder
	for _, msg := range messages {
		sb.WriteString(roleLabel(msg.Role))
		sb.WriteString(":\n")
		for _, part := range msg.Content {
			if tp, ok := part.(fantasy.TextPart); ok {
				sb.WriteString(tp.Text)
			}
		}
		sb.WriteString("\n\n")
	}
	return sb.String()
}

// ---------------------------------------------------------------------------
// Compact
// ---------------------------------------------------------------------------

// Compact summarises older messages using the LLM, returning the compaction
// result and a new message slice (summary message + preserved recent
// messages).
//
// The model parameter is the same fantasy.LanguageModel used for regular
// generation — compaction creates a disposable fantasy agent with no tools to
// produce the summary.
//
// customInstructions is optional text appended to the summary prompt (e.g.
// "Focus on the API design decisions"). Pass "" to use the default prompt
// only.
func Compact(
	ctx context.Context,
	model fantasy.LanguageModel,
	messages []fantasy.Message,
	opts CompactionOptions,
	customInstructions string,
) (*CompactionResult, []fantasy.Message, error) {
	opts.defaults()

	if len(messages) < 2 {
		return nil, messages, nil
	}

	cutPoint := FindCutPoint(messages, opts.KeepRecentTokens)
	if cutPoint == 0 {
		// All messages fit within the keep budget. Force a cut that
		// keeps only the last non-tool message — matching Pi, which
		// always compacts when the user explicitly requests it.
		cutPoint = forceCutPoint(messages)
		if cutPoint == 0 {
			return nil, messages, nil
		}
	}

	oldMessages := messages[:cutPoint]
	recentMessages := messages[cutPoint:]
	originalTokens := EstimateMessageTokens(messages)

	// Serialise old messages to text, matching Pi's format.
	conversationText := serializeMessages(oldMessages)

	// Build the user-facing prompt: conversation text + summary instructions.
	userPrompt := opts.SummaryPrompt
	if userPrompt == "" {
		userPrompt = defaultSummaryPrompt
	}
	if customInstructions != "" {
		userPrompt += "\n\nAdditional instructions: " + customInstructions
	}

	// Create a lightweight agent (no tools) just for summarisation.
	summaryAgent := fantasy.NewAgent(model,
		fantasy.WithSystemPrompt(defaultSystemPrompt),
	)
	result, err := summaryAgent.Generate(ctx, fantasy.AgentCall{
		Prompt: conversationText + "\n\n" + userPrompt,
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
