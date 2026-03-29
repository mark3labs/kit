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

// applyCacheControlToMessages adds cache control to specific messages in the conversation.
// Following Crush's strategy:
// 1. The last system message gets cache control
// 2. The last 2 messages get cache control
// This ensures optimal caching for the most expensive parts of the context.
func applyCacheControlToMessages(messages []fantasy.Message) []fantasy.Message {
	if len(messages) == 0 {
		return messages
	}

	// Make a copy to avoid modifying the original slice
	result := make([]fantasy.Message, len(messages))
	copy(result, messages)

	cacheOpts := cacheControlOptions()

	// Find the last system message and add cache control
	lastSystemIdx := -1
	for i, msg := range result {
		if msg.Role == fantasy.MessageRoleSystem {
			lastSystemIdx = i
		}
	}

	// Apply cache control to the last system message
	if lastSystemIdx >= 0 {
		result[lastSystemIdx].ProviderOptions = cacheOpts
	}

	// Apply cache control to the last 2 messages
	startIdx := len(result) - 2
	if startIdx < 0 {
		startIdx = 0
	}
	for i := startIdx; i < len(result); i++ {
		// Only apply if not already set (avoid double-setting system message)
		if i != lastSystemIdx {
			result[i].ProviderOptions = cacheOpts
		}
	}

	return result
}
