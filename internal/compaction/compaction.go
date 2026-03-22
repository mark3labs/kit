// Package compaction provides context window management with token estimation,
// compaction triggers, and LLM-based conversation summarization.
//
// The algorithm preserves a token budget of recent
// messages (KeepRecentTokens, default 20 000) rather than a fixed message
// count. Auto-compaction fires when estimated context usage exceeds
// contextWindow − ReserveTokens.
//
// Features modelled after pi's compaction system:
//   - Tool result truncation (2000 char max) during serialisation
//   - Split turn handling: when a single turn exceeds the keep budget,
//     the turn prefix is summarised separately and merged
//   - Cumulative file tracking: read and modified files extracted from
//     tool calls and carried forward across compactions
package compaction

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"charm.land/fantasy"
)

// ---------------------------------------------------------------------------
// Token estimation
// ---------------------------------------------------------------------------

// estimateTokens provides a rough token count (~4 chars per token).
func estimateTokens(text string) int {
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
			total += estimateTokens(tp.Text)
		}
	}
	return total
}

// ---------------------------------------------------------------------------
// Auto-compact trigger
// ---------------------------------------------------------------------------

// ShouldCompact reports whether auto-compaction should fire.
// Formula: contextTokens > contextWindow − reserveTokens.
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
	Summary         string   // LLM-generated summary of compacted messages
	OriginalTokens  int      // Estimated token count before compaction
	CompactedTokens int      // Estimated token count after compaction
	MessagesRemoved int      // Number of messages replaced by the summary
	CutPoint        int      // Index in the original messages where the cut was made
	ReadFiles       []string // Files read during the compacted conversation
	ModifiedFiles   []string // Files modified during the compacted conversation
}

// CompactionOptions configures compaction behaviour. Token-based defaults
// are applied for zero-value fields.
type CompactionOptions struct {
	ContextWindow    int    // Model's context window size (tokens)
	ReserveTokens    int    // Tokens to reserve for LLM response, default 16384
	KeepRecentTokens int    // Recent tokens to preserve (not summarised), default 20000
	SummaryPrompt    string // Custom summary prompt (empty = use default)
}

// defaults fills zero-value fields with sensible defaults.
func (o *CompactionOptions) defaults() {
	if o.ReserveTokens <= 0 {
		o.ReserveTokens = 16384
	}
	if o.KeepRecentTokens <= 0 {
		o.KeepRecentTokens = 20000
	}
}

// defaultSystemPrompt is the system prompt sent to the summarisation LLM.

const defaultSystemPrompt = `You are a context summarization assistant. Your task is to read a conversation between a user and an AI coding assistant, then produce a structured summary following the exact format specified.

Do NOT continue the conversation. Do NOT respond to any questions in the conversation. ONLY output the structured summary.`

// defaultSummaryPrompt is the user prompt appended after the serialised
// conversation.
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

<read-files>
[One file path per line for files that were read during the conversation]
</read-files>

<modified-files>
[One file path per line for files that were created, edited, or written during the conversation]
</modified-files>

Keep each section concise. Preserve exact file paths, function names, and error messages.`

// ---------------------------------------------------------------------------
// Tool result truncation
// ---------------------------------------------------------------------------

// maxToolResultChars is the maximum length of tool result text preserved
// during serialisation. Longer results are truncated with a marker.
const maxToolResultChars = 2000

// truncateToolResult truncates text to maxToolResultChars, appending a
// marker indicating how many characters were removed.
func truncateToolResult(text string) string {
	if len(text) <= maxToolResultChars {
		return text
	}
	truncated := len(text) - maxToolResultChars
	return text[:maxToolResultChars] + fmt.Sprintf("\n[...%d chars truncated]", truncated)
}

// ---------------------------------------------------------------------------
// Cut point (token-based)
// ---------------------------------------------------------------------------

// isValidCutPoint returns true if the message at index i is a valid place to
// split the conversation. Tool-role messages (tool results) must stay with
// their preceding assistant tool-call, so they are never valid cut points.
func isValidCutPoint(msg fantasy.Message) bool {
	return msg.Role != fantasy.MessageRoleTool
}

// findTurnStart returns the index of the user message that starts the turn
// containing messages[idx]. A "turn" starts with a user message and includes
// all subsequent assistant/tool messages until the next user message.
func findTurnStart(messages []fantasy.Message, idx int) int {
	for i := idx; i >= 0; i-- {
		if messages[i].Role == fantasy.MessageRoleUser {
			return i
		}
	}
	return 0
}

// FindCutPoint walks backward from the end of messages, accumulating tokens
// until the keepRecentTokens budget is filled. Returns the index that
// separates "old" messages (0..cutPoint-1, to be summarised) from "recent"
// messages (cutPoint..end, to be preserved).
//
// The cut point prefers turn boundaries (user messages). When a single turn
// exceeds the budget, the cut lands mid-turn (IsSplitTurn returns true).
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

// IsSplitTurn returns true if the cut point lands in the middle of a turn
// (i.e. the message at cutPoint is not a user message, meaning we're
// splitting a single turn's assistant/tool messages).
func IsSplitTurn(messages []fantasy.Message, cutPoint int) bool {
	if cutPoint <= 0 || cutPoint >= len(messages) {
		return false
	}
	// If the cut point is at a user message, it's a clean turn boundary.
	if messages[cutPoint].Role == fantasy.MessageRoleUser {
		return false
	}
	// Otherwise we're cutting mid-turn — check if the turn started before
	// the cut point.
	turnStart := findTurnStart(messages, cutPoint)
	return turnStart < cutPoint
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
// File tracking
// ---------------------------------------------------------------------------

// fileOps contains cumulative file operation tracking.
type fileOps struct {
	ReadFiles     map[string]bool
	ModifiedFiles map[string]bool
}

func newFileOps() *fileOps {
	return &fileOps{
		ReadFiles:     make(map[string]bool),
		ModifiedFiles: make(map[string]bool),
	}
}

// extractFileOps scans messages for tool calls and extracts file paths.
// It recognises the built-in Kit tools: read, write, edit, bash, grep, find, ls.
func extractFileOps(messages []fantasy.Message) *fileOps {
	ops := newFileOps()
	for _, msg := range messages {
		for _, part := range msg.Content {
			tc, ok := part.(fantasy.ToolCallPart)
			if !ok {
				continue
			}

			// Parse the JSON input to extract path arguments.
			var args map[string]any
			if err := json.Unmarshal([]byte(tc.Input), &args); err != nil {
				continue
			}

			path, _ := args["path"].(string)
			if path == "" {
				continue
			}

			switch tc.ToolName {
			case "read", "grep", "find", "ls":
				ops.ReadFiles[path] = true
			case "write", "edit":
				ops.ModifiedFiles[path] = true
			}
		}
	}
	return ops
}

// merge combines another fileOps into this one (for cumulative tracking).
func (f *fileOps) merge(other *fileOps) {
	if other == nil {
		return
	}
	for k := range other.ReadFiles {
		f.ReadFiles[k] = true
	}
	for k := range other.ModifiedFiles {
		f.ModifiedFiles[k] = true
	}
}

// mergeSlices adds previously tracked file lists (from a prior compaction).
func (f *fileOps) mergeSlices(readFiles, modifiedFiles []string) {
	for _, p := range readFiles {
		f.ReadFiles[p] = true
	}
	for _, p := range modifiedFiles {
		f.ModifiedFiles[p] = true
	}
}

// sortedKeys returns the keys of a bool map sorted alphabetically.
func sortedKeys(m map[string]bool) []string {
	if len(m) == 0 {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	// Simple sort — no need for sort package for small lists.
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			if keys[j] < keys[i] {
				keys[i], keys[j] = keys[j], keys[i]
			}
		}
	}
	return keys
}

// ---------------------------------------------------------------------------
// Message serialisation
// ---------------------------------------------------------------------------

// roleLabel returns a human-readable label for a fantasy message role.
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
// representation suitable for sending to the summarisation LLM. Tool result
// text is truncated to maxToolResultChars to keep the summarisation request
// within reasonable token budgets.
func serializeMessages(messages []fantasy.Message) string {
	var sb strings.Builder
	for _, msg := range messages {
		sb.WriteString(roleLabel(msg.Role))
		sb.WriteString(":\n")
		for _, part := range msg.Content {
			switch p := part.(type) {
			case fantasy.TextPart:
				if msg.Role == fantasy.MessageRoleTool {
					sb.WriteString(truncateToolResult(p.Text))
				} else {
					sb.WriteString(p.Text)
				}
			case fantasy.ToolCallPart:
				fmt.Fprintf(&sb, "[Tool call: %s(%s)]", p.ToolName, truncateToolResult(p.Input))
			case fantasy.ReasoningPart:
				fmt.Fprintf(&sb, "[Thinking]: %s", truncateToolResult(p.Text))
			}
		}
		sb.WriteString("\n\n")
	}
	return sb.String()
}

// ---------------------------------------------------------------------------
// Compact
// ---------------------------------------------------------------------------

// PreviousCompaction carries file tracking state from a prior compaction so
// that file operations accumulate across multiple compactions.
type PreviousCompaction struct {
	ReadFiles     []string
	ModifiedFiles []string
}

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
//
// prev carries file tracking from a previous compaction for cumulative
// tracking. Pass nil if there is no prior compaction.
func Compact(
	ctx context.Context,
	model fantasy.LanguageModel,
	messages []fantasy.Message,
	opts CompactionOptions,
	customInstructions string,
	prev *PreviousCompaction,
) (*CompactionResult, []fantasy.Message, error) {
	opts.defaults()

	if len(messages) < 2 {
		return nil, messages, nil
	}

	cutPoint := FindCutPoint(messages, opts.KeepRecentTokens)
	if cutPoint == 0 {
		// All messages fit within the keep budget. Force a cut that
		// keeps only the last non-tool message — always compact when
		// the user explicitly requests it.
		cutPoint = forceCutPoint(messages)
		if cutPoint == 0 {
			return nil, messages, nil
		}
	}

	oldMessages := messages[:cutPoint]
	recentMessages := messages[cutPoint:]
	originalTokens := EstimateMessageTokens(messages)

	// Extract file operations from old messages.
	ops := extractFileOps(oldMessages)
	// Accumulate from previous compaction if present.
	if prev != nil {
		ops.mergeSlices(prev.ReadFiles, prev.ModifiedFiles)
	}
	// Also scan recent messages for file ops (they'll be carried forward).
	recentOps := extractFileOps(recentMessages)
	ops.merge(recentOps)

	// Handle split turns: when the cut lands mid-turn, summarise the turn
	// prefix separately and merge with the history summary.
	var summaryText string
	var err error

	if IsSplitTurn(messages, cutPoint) {
		summaryText, err = compactSplitTurn(ctx, model, oldMessages, messages, cutPoint, opts, customInstructions)
	} else {
		summaryText, err = compactNormal(ctx, model, oldMessages, opts, customInstructions)
	}
	if err != nil {
		return nil, nil, err
	}

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
		CutPoint:        cutPoint,
		ReadFiles:       sortedKeys(ops.ReadFiles),
		ModifiedFiles:   sortedKeys(ops.ModifiedFiles),
	}, newMessages, nil
}

// compactNormal generates a summary for a clean turn-boundary cut.
func compactNormal(
	ctx context.Context,
	model fantasy.LanguageModel,
	oldMessages []fantasy.Message,
	opts CompactionOptions,
	customInstructions string,
) (string, error) {
	conversationText := serializeMessages(oldMessages)
	return generateSummary(ctx, model, conversationText, opts, customInstructions)
}

// compactSplitTurn handles the case where the cut point lands mid-turn.
// It generates two summaries and merges them:
//  1. History summary: all complete turns before the split turn
//  2. Turn prefix summary: the early part of the split turn (from the turn's
//     user message up to the cut point)
//
// The merged result preserves context from both the older history and the
// beginning of the current long turn.
func compactSplitTurn(
	ctx context.Context,
	model fantasy.LanguageModel,
	oldMessages []fantasy.Message,
	allMessages []fantasy.Message,
	cutPoint int,
	opts CompactionOptions,
	customInstructions string,
) (string, error) {
	// Find where the split turn starts.
	turnStart := findTurnStart(allMessages, cutPoint)

	// Messages before the turn are the "history" portion.
	historyMessages := oldMessages
	if turnStart > 0 && turnStart < len(oldMessages) {
		historyMessages = oldMessages[:turnStart]
	}

	// The turn prefix: from turnStart to cutPoint.
	turnPrefixMessages := allMessages[turnStart:cutPoint]

	var historySummary string
	var err error

	// Generate history summary if there are complete turns before the split.
	if len(historyMessages) >= 2 {
		historySummary, err = generateSummary(ctx, model,
			serializeMessages(historyMessages), opts, "")
		if err != nil {
			return "", fmt.Errorf("split turn history summary failed: %w", err)
		}
	}

	// Generate turn prefix summary.
	turnPrefixText := serializeMessages(turnPrefixMessages)
	turnPrefixPrompt := "The messages above are the BEGINNING of a long turn that was split. " +
		"Summarize the work done so far in this turn, preserving tool call results, " +
		"file changes, and progress. Another LLM will continue this turn."
	if customInstructions != "" {
		turnPrefixPrompt += "\n\nAdditional instructions: " + customInstructions
	}

	summaryAgent := fantasy.NewAgent(model,
		fantasy.WithSystemPrompt(defaultSystemPrompt),
	)
	result, err := summaryAgent.Generate(ctx, fantasy.AgentCall{
		Prompt: turnPrefixText + "\n\n" + turnPrefixPrompt,
	})
	if err != nil {
		return "", fmt.Errorf("split turn prefix summary failed: %w", err)
	}
	turnPrefixSummary := result.Response.Content.Text()

	// Merge the two summaries.
	if historySummary != "" && turnPrefixSummary != "" {
		return historySummary + "\n\n---\n\n## Current Turn (in progress)\n\n" + turnPrefixSummary, nil
	}
	if turnPrefixSummary != "" {
		return turnPrefixSummary, nil
	}
	return historySummary, nil
}

// generateSummary calls the LLM to produce a structured summary.
func generateSummary(
	ctx context.Context,
	model fantasy.LanguageModel,
	conversationText string,
	opts CompactionOptions,
	customInstructions string,
) (string, error) {
	userPrompt := opts.SummaryPrompt
	if userPrompt == "" {
		userPrompt = defaultSummaryPrompt
	}
	if customInstructions != "" {
		userPrompt += "\n\nAdditional instructions: " + customInstructions
	}

	summaryAgent := fantasy.NewAgent(model,
		fantasy.WithSystemPrompt(defaultSystemPrompt),
	)
	result, err := summaryAgent.Generate(ctx, fantasy.AgentCall{
		Prompt: conversationText + "\n\n" + userPrompt,
	})
	if err != nil {
		return "", fmt.Errorf("compaction summarisation failed: %w", err)
	}

	return result.Response.Content.Text(), nil
}
