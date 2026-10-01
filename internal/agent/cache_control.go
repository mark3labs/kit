package agent

import (
	"maps"
	"os"

	"charm.land/fantasy"
	"charm.land/fantasy/providers/anthropic"

	"github.com/mark3labs/kit/internal/models"
)

// Anthropic allows at most 4 cache_control blocks per request. Kit spends one
// on the last tool definition (see applyCacheControlToTools), so the message
// breakpoints must fit in the remaining budget. The constants are kept together
// so the two files cannot drift out of sync.
const (
	// anthropicMaxCacheBlocks is the hard limit enforced by the Anthropic API.
	anthropicMaxCacheBlocks = 4
	// toolCacheBlocks is the number of breakpoints the tool list uses. The
	// caller reserves this many blocks when it applies message breakpoints,
	// because the tool breakpoint lives outside the message list.
	toolCacheBlocks = 1
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

// cachingDisabled reports whether prompt caching is turned off, either globally
// (KIT_DISABLE_CACHE) or for this provider configuration.
func cachingDisabled(config *models.ProviderConfig) bool {
	if os.Getenv("KIT_DISABLE_CACHE") != "" {
		return true
	}
	return config != nil && config.DisableCaching
}

// messageCachingEnabled reports whether Anthropic message- and tool-level
// prompt caching should be applied for the active model. The provider must use
// Anthropic cache_control (ProviderResult.MessageCacheControl), and caching
// must not be disabled by KIT_DISABLE_CACHE or ProviderConfig.DisableCaching.
func (a *Agent) messageCachingEnabled() bool {
	return a.anthropicCaching && !cachingDisabled(a.modelConfig)
}

// hasCacheControl reports whether msg already carries an Anthropic cache
// control block. It checks the concrete option type so unrelated Anthropic
// provider options are not mistaken for a cache block and counted against the
// four-block budget.
func hasCacheControl(msg fantasy.Message) bool {
	if msg.ProviderOptions == nil {
		return false
	}
	_, ok := msg.ProviderOptions[anthropic.Name].(*anthropic.ProviderCacheControlOptions)
	return ok
}

// cacheControlTool wraps a tool and reports Anthropic cache control as its
// provider options. It wraps instead of mutating the tool in place because a
// tool instance can be shared with a subagent (which may use a different
// model), and mutating it during PrepareStep would race.
type cacheControlTool struct {
	fantasy.AgentTool
	opts fantasy.ProviderOptions
}

// ProviderOptions returns the wrapped tool's own options with the cache control
// options layered on top.
func (t *cacheControlTool) ProviderOptions() fantasy.ProviderOptions {
	merged := fantasy.ProviderOptions{}
	maps.Copy(merged, t.AgentTool.ProviderOptions())
	maps.Copy(merged, t.opts)
	return merged
}

// SetProviderOptions keeps the override local to the wrapper so a later
// SetProviderOptions call cannot mutate the shared inner tool.
func (t *cacheControlTool) SetProviderOptions(opts fantasy.ProviderOptions) {
	t.opts = opts
}

// applyCacheControlToTools adds Anthropic cache control to the last tool
// definition. Anthropic's cache prefix starts with the tools, so a breakpoint
// on the last tool caches every tool definition independently of the system
// prompt. That dedicated breakpoint keeps the tool cache valid when only the
// system prompt changes (for example after Kit recomposes it for a skill or
// context-file update).
//
// The returned slice shares the tools but replaces the last element with a
// wrapper, so the caller's tools are never mutated.
func applyCacheControlToTools(tools []fantasy.AgentTool) []fantasy.AgentTool {
	if len(tools) == 0 {
		return tools
	}
	out := make([]fantasy.AgentTool, len(tools))
	copy(out, tools)
	out[len(out)-1] = &cacheControlTool{
		AgentTool: tools[len(tools)-1],
		opts:      cacheControlOptions(),
	}
	return out
}

// applyCacheControlToMessages adds cache control to specific messages.
// Anthropic allows max 4 cache blocks per request. reservedBlocks is the number
// of blocks already used outside the message list (currently the tool
// breakpoint); the function counts existing cache blocks and only adds new ones
// up to the limit.
func applyCacheControlToMessages(messages []fantasy.Message, reservedBlocks int) []fantasy.Message {
	if len(messages) == 0 {
		return messages
	}

	// Make a copy to avoid modifying the original slice
	result := make([]fantasy.Message, len(messages))
	copy(result, messages)

	cacheOpts := cacheControlOptions()

	// Count existing cache blocks
	existingCacheCount := 0
	for _, msg := range result {
		if hasCacheControl(msg) {
			existingCacheCount++
		}
	}

	// How many new cache blocks can we add?
	remaining := anthropicMaxCacheBlocks - reservedBlocks - existingCacheCount
	if remaining <= 0 {
		return result
	}

	// First: find and cache the last system message (most important)
	lastSystemIdx := -1
	for i, msg := range result {
		if msg.Role == fantasy.MessageRoleSystem {
			lastSystemIdx = i
		}
	}

	if lastSystemIdx >= 0 && !hasCacheControl(result[lastSystemIdx]) {
		result[lastSystemIdx].ProviderOptions = cacheOpts
		remaining--
	}

	// Second: cache the most recent messages (up to remaining limit)
	// Work backwards from the end to prioritize recent context
	for i := len(result) - 1; i >= 0 && remaining > 0; i-- {
		if hasCacheControl(result[i]) {
			continue
		}
		result[i].ProviderOptions = cacheOpts
		remaining--
	}

	return result
}
