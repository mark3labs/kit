package agent

import (
	"charm.land/fantasy"
	"charm.land/fantasy/providers/anthropic"
)

// cacheControlOptions returns provider options for Anthropic cache control.
// This is used at the message level to avoid type conflicts with provider-level options.
func cacheControlOptions() fantasy.ProviderOptions {
	return anthropic.NewProviderCacheControlOptions(&anthropic.ProviderCacheControlOptions{
		CacheControl: anthropic.CacheControl{
			Type: "ephemeral",
		},
	})
}

// applyCacheControlToMessages adds cache control to specific messages.
// Anthropic allows max 4 cache blocks per request:
// 1. Last system message (if present)
// 2. Last 2 messages in the conversation
func applyCacheControlToMessages(messages []fantasy.Message) []fantasy.Message {
	if len(messages) == 0 {
		return messages
	}

	// Make a copy to avoid modifying the original slice
	result := make([]fantasy.Message, len(messages))
	copy(result, messages)

	cacheOpts := cacheControlOptions()

	// Find the last system message
	lastSystemIdx := -1
	for i, msg := range result {
		if msg.Role == fantasy.MessageRoleSystem {
			lastSystemIdx = i
		}
	}

	// Apply cache control to last system message (block 1)
	if lastSystemIdx >= 0 {
		result[lastSystemIdx].ProviderOptions = cacheOpts
	}

	// Apply cache control to last 2 messages (blocks 2-3)
	// Only if not the same as system message
	for i := max(len(result)-2, 0); i < len(result); i++ {
		if i != lastSystemIdx {
			result[i].ProviderOptions = cacheOpts
		}
	}

	return result
}
