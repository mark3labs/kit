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
// Anthropic allows a maximum of 4 blocks with cache_control per request.
// We prioritize: last system message first, then most recent user messages.
func applyCacheControlToMessages(messages []fantasy.Message) []fantasy.Message {
	if len(messages) == 0 {
		return messages
	}

	// Make a copy to avoid modifying the original slice
	result := make([]fantasy.Message, len(messages))
	copy(result, messages)

	cacheOpts := cacheControlOptions()
	cacheCount := 0
	maxCacheBlocks := 4

	// First: find and cache the last system message (most important for context)
	lastSystemIdx := -1
	for i, msg := range result {
		if msg.Role == fantasy.MessageRoleSystem {
			lastSystemIdx = i
		}
	}

	if lastSystemIdx >= 0 && cacheCount < maxCacheBlocks {
		result[lastSystemIdx].ProviderOptions = cacheOpts
		cacheCount++
	}

	// Second: cache the most recent messages (up to remaining limit)
	// Work backwards from the end to prioritize recent context
	for i := len(result) - 1; i >= 0 && cacheCount < maxCacheBlocks; i-- {
		// Skip if already cached (system message) or if it's the first message
		// (we want to spread cache across the conversation)
		if result[i].ProviderOptions != nil || i == 0 {
			continue
		}
		result[i].ProviderOptions = cacheOpts
		cacheCount++
	}

	return result
}
