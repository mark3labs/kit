package kit

import (
	"strings"

	"charm.land/fantasy"
)

// fantasyToLLMMessages converts a []fantasy.Message to []LLMMessage.
// Used at the boundary between internal agent/session code and the public SDK.
func fantasyToLLMMessages(msgs []fantasy.Message) []LLMMessage {
	result := make([]LLMMessage, len(msgs))
	for i, fm := range msgs {
		var b strings.Builder
		for _, part := range fm.Content {
			if tp, ok := part.(fantasy.TextPart); ok {
				b.WriteString(tp.Text)
			}
		}
		result[i] = LLMMessage{
			Role:    LLMMessageRole(fm.Role),
			Content: b.String(),
		}
	}
	return result
}

// llmToFantasyMessages converts a []LLMMessage to []fantasy.Message.
// Used when passing SDK types back into internal functions that still use fantasy.
func llmToFantasyMessages(msgs []LLMMessage) []fantasy.Message {
	result := make([]fantasy.Message, len(msgs))
	for i, m := range msgs {
		result[i] = fantasy.Message{
			Role:    fantasy.MessageRole(m.Role),
			Content: []fantasy.MessagePart{fantasy.TextPart{Text: m.Content}},
		}
	}
	return result
}

// llmMessagesToFantasy is an alias for llmToFantasyMessages, for callers that
// use the older name.
var llmMessagesToFantasy = llmToFantasyMessages

// fantasyUsageToLLM converts a fantasy.Usage to an LLMUsage.
func fantasyUsageToLLM(u fantasy.Usage) LLMUsage {
	return LLMUsage{
		InputTokens:         u.InputTokens,
		OutputTokens:        u.OutputTokens,
		TotalTokens:         u.TotalTokens,
		ReasoningTokens:     u.ReasoningTokens,
		CacheCreationTokens: u.CacheCreationTokens,
		CacheReadTokens:     u.CacheReadTokens,
	}
}

// llmFilePartsToFantasy converts []LLMFilePart to []fantasy.FilePart.
func llmFilePartsToFantasy(parts []LLMFilePart) []fantasy.FilePart {
	result := make([]fantasy.FilePart, len(parts))
	for i, p := range parts {
		result[i] = fantasy.FilePart{
			Filename:  p.Filename,
			Data:      p.Data,
			MediaType: p.MediaType,
		}
	}
	return result
}
