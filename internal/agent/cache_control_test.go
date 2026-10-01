package agent

import (
	"context"
	"testing"

	"charm.land/fantasy"
	"charm.land/fantasy/providers/anthropic"

	"github.com/mark3labs/kit/internal/models"
)

// cacheTestTool is a minimal AgentTool used by the cache-control tests. It
// records its own provider options so a test can prove that wrapping a tool
// does not mutate it.
type cacheTestTool struct {
	name string
	opts fantasy.ProviderOptions
}

func (t *cacheTestTool) Info() fantasy.ToolInfo { return fantasy.ToolInfo{Name: t.name} }

func (t *cacheTestTool) Run(context.Context, fantasy.ToolCall) (fantasy.ToolResponse, error) {
	return fantasy.ToolResponse{}, nil
}

func (t *cacheTestTool) ProviderOptions() fantasy.ProviderOptions { return t.opts }

func (t *cacheTestTool) SetProviderOptions(opts fantasy.ProviderOptions) { t.opts = opts }

// conversation builds a message list of the given size with a system message
// first when withSystem is true.
func conversation(withSystem bool, n int) []fantasy.Message {
	var msgs []fantasy.Message
	if withSystem {
		msgs = append(msgs, fantasy.NewSystemMessage("system"))
	}
	for i := 0; i < n; i++ {
		if i%2 == 0 {
			msgs = append(msgs, fantasy.NewUserMessage("user"))
			continue
		}
		msgs = append(msgs, fantasy.Message{
			Role:    fantasy.MessageRoleAssistant,
			Content: []fantasy.MessagePart{fantasy.TextPart{Text: "assistant"}},
		})
	}
	return msgs
}

func countCacheBlocks(msgs []fantasy.Message) int {
	n := 0
	for _, m := range msgs {
		if hasCacheControl(m) {
			n++
		}
	}
	return n
}

func TestApplyCacheControlToMessages_ReservesToolBlock(t *testing.T) {
	// 1 tool block + 1 system block + 2 message blocks == Anthropic's limit.
	msgs := conversation(true, 5)
	out := applyCacheControlToMessages(msgs, toolCacheBlocks)

	if got, want := countCacheBlocks(out), 3; got != want {
		t.Fatalf("cache blocks = %d, want %d", got, want)
	}
	if !hasCacheControl(out[0]) {
		t.Error("system message must carry a cache block")
	}
	if !hasCacheControl(out[len(out)-1]) || !hasCacheControl(out[len(out)-2]) {
		t.Error("the two newest messages must carry a cache block")
	}
	if hasCacheControl(out[1]) || hasCacheControl(out[2]) {
		t.Error("older messages must not carry a cache block")
	}
	if got, want := countCacheBlocks(out)+toolCacheBlocks, anthropicMaxCacheBlocks; got != want {
		t.Fatalf("total blocks = %d, want %d", got, want)
	}
}

func TestApplyCacheControlToMessages_NoToolReservationUsesFullBudget(t *testing.T) {
	msgs := conversation(true, 5)
	out := applyCacheControlToMessages(msgs, 0)

	// 1 system + last 3 messages == 4.
	if got, want := countCacheBlocks(out), 4; got != want {
		t.Fatalf("cache blocks = %d, want %d", got, want)
	}
}

func TestApplyCacheControlToMessages_NoSystemMessage(t *testing.T) {
	msgs := conversation(false, 5)
	out := applyCacheControlToMessages(msgs, toolCacheBlocks)

	// No system message, so all three remaining blocks go to the tail.
	if got, want := countCacheBlocks(out), 3; got != want {
		t.Fatalf("cache blocks = %d, want %d", got, want)
	}
	if !hasCacheControl(out[len(out)-1]) || !hasCacheControl(out[len(out)-2]) || !hasCacheControl(out[len(out)-3]) {
		t.Error("the three newest messages must carry a cache block")
	}
}

func TestApplyCacheControlToMessages_CountsExistingBlocks(t *testing.T) {
	msgs := conversation(true, 5)
	// One message already carries a cache block.
	msgs[1].ProviderOptions = cacheControlOptions()

	out := applyCacheControlToMessages(msgs, toolCacheBlocks)

	// budget 4 - reserved 1 - existing 1 = 2 new blocks.
	if got, want := countCacheBlocks(out), 3; got != want {
		t.Fatalf("cache blocks = %d, want %d", got, want)
	}
}

func TestApplyCacheControlToMessages_StopsAtBudget(t *testing.T) {
	msgs := conversation(true, 5)
	// Three existing blocks plus the reserved tool block exhaust the budget.
	msgs[1].ProviderOptions = cacheControlOptions()
	msgs[2].ProviderOptions = cacheControlOptions()
	msgs[3].ProviderOptions = cacheControlOptions()

	out := applyCacheControlToMessages(msgs, toolCacheBlocks)

	if got, want := countCacheBlocks(out), 3; got != want {
		t.Fatalf("cache blocks = %d, want %d (no new blocks)", got, want)
	}
	if hasCacheControl(out[len(out)-1]) {
		t.Error("no new block may be added when the budget is exhausted")
	}
}

func TestApplyCacheControlToMessages_DoesNotMutateInput(t *testing.T) {
	msgs := conversation(true, 3)
	before := countCacheBlocks(msgs)

	_ = applyCacheControlToMessages(msgs, toolCacheBlocks)

	if got := countCacheBlocks(msgs); got != before {
		t.Fatalf("input was mutated: blocks %d -> %d", before, got)
	}
}

func TestApplyCacheControlToMessages_Empty(t *testing.T) {
	if out := applyCacheControlToMessages(nil, toolCacheBlocks); out != nil {
		t.Fatalf("expected nil for empty input, got %v", out)
	}
}

func TestHasCacheControl_IgnoresUnrelatedAnthropicOptions(t *testing.T) {
	msg := fantasy.Message{
		Role:            fantasy.MessageRoleAssistant,
		ProviderOptions: fantasy.ProviderOptions{anthropic.Name: &anthropic.ProviderOptions{}},
	}
	if hasCacheControl(msg) {
		t.Error("an unrelated Anthropic provider option must not count as a cache block")
	}

	msg.ProviderOptions = cacheControlOptions()
	if !hasCacheControl(msg) {
		t.Error("a cache control option must count as a cache block")
	}
}

func TestApplyCacheControlToTools_WrapsLastTool(t *testing.T) {
	first := &cacheTestTool{name: "first"}
	inner := &cacheTestTool{name: "last"}
	inner.SetProviderOptions(fantasy.ProviderOptions{"other": &anthropic.ProviderOptions{}})

	out := applyCacheControlToTools([]fantasy.AgentTool{first, inner})

	if len(out) != 2 {
		t.Fatalf("tool count = %d, want 2", len(out))
	}
	if out[0] != first {
		t.Error("tools before the last must not be wrapped")
	}
	last := out[len(out)-1]
	if _, ok := last.ProviderOptions()[anthropic.Name]; !ok {
		t.Error("the last tool must report an Anthropic cache option")
	}
	if _, ok := last.ProviderOptions()["other"]; !ok {
		t.Error("wrapped tool lost its own options")
	}
	if _, ok := inner.ProviderOptions()[anthropic.Name]; ok {
		t.Error("the shared inner tool must not be mutated")
	}
}

func TestApplyCacheControlToTools_Empty(t *testing.T) {
	if out := applyCacheControlToTools(nil); out != nil {
		t.Fatalf("expected nil for empty input, got %v", out)
	}
}

func TestCachingDisabled(t *testing.T) {
	if cachingDisabled(nil) {
		t.Error("caching must be enabled by default")
	}
	if !cachingDisabled(&models.ProviderConfig{DisableCaching: true}) {
		t.Error("ProviderConfig.DisableCaching must disable caching")
	}

	t.Setenv("KIT_DISABLE_CACHE", "1")
	if !cachingDisabled(&models.ProviderConfig{}) {
		t.Error("KIT_DISABLE_CACHE must disable caching")
	}
}

func TestMessageCachingEnabled(t *testing.T) {
	t.Setenv("KIT_DISABLE_CACHE", "")

	a := &Agent{anthropicCaching: true}
	if !a.messageCachingEnabled() {
		t.Error("caching must be enabled for an Anthropic provider")
	}

	a.anthropicCaching = false
	if a.messageCachingEnabled() {
		t.Error("caching must be disabled for a non-Anthropic provider")
	}

	a.anthropicCaching = true
	a.modelConfig = &models.ProviderConfig{DisableCaching: true}
	if a.messageCachingEnabled() {
		t.Error("caching must honor ProviderConfig.DisableCaching")
	}

	a.modelConfig = nil
	t.Setenv("KIT_DISABLE_CACHE", "1")
	if a.messageCachingEnabled() {
		t.Error("caching must honor KIT_DISABLE_CACHE")
	}
}
