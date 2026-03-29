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
// Anthropic allows max 4 cache blocks per request.
// Counts existing cache blocks and only adds new ones up to the limit.
func applyCacheControlToMessages(messages []fantasy.Message) []fantasy.Message {
	if len(messages) == 0 {
		return messages
	}

	// Make a copy to avoid modifying the original slice
	result := make([]fantasy.Message, len(messages))
	copy(result, messages)

	cacheOpts := cacheControlOptions()
	maxCacheBlocks := 4

	// Helper to check if message already has cache control
	hasCache := func(msg fantasy.Message) bool {
		if msg.ProviderOptions == nil {
			return false
		}
		if _, ok := msg.ProviderOptions["anthropic"]; ok {
			return true
		}
		return false
	}

	// Count existing cache blocks
	existingCacheCount := 0
	for _, msg := range result {
		if hasCache(msg) {
			existingCacheCount++
		}
	}

	// If we're already at or over the limit, don't add more
	if existingCacheCount >= maxCacheBlocks {
		return result
	}

	// How many new cache blocks can we add?
	remaining := maxCacheBlocks - existingCacheCount

	// First: find and cache the last system message (most important)
	lastSystemIdx := -1
	for i, msg := range result {
		if msg.Role == fantasy.MessageRoleSystem {
			lastSystemIdx = i
		}
	}

	if lastSystemIdx >= 0 && remaining > 0 && !hasCache(result[lastSystemIdx]) {
		result[lastSystemIdx].ProviderOptions = cacheOpts
		remaining--
	}

	// Second: cache the most recent messages (up to remaining limit)
	// Work backwards from the end to prioritize recent context
	for i := len(result) - 1; i >= 0 && remaining > 0; i-- {
		if hasCache(result[i]) {
			continue
		}
		result[i].ProviderOptions = cacheOpts
		remaining--
	}

	return result
}
