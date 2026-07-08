package kit

import (
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/kit/internal/agents"
)

func TestApplyAgentDefinition_FillsDefaults(t *testing.T) {
	temp := float32(0.2)
	def := &AgentDefinition{
		Name:         "reviewer",
		Model:        "anthropic/claude-sonnet-4",
		SystemPrompt: "Review carefully.",
		Timeout:      2 * time.Minute,
		Temperature:  &temp,
	}
	cfg := SubagentConfig{Prompt: "review this", Agent: "reviewer"}

	applyAgentDefinition(&cfg, def)

	if cfg.Model != "anthropic/claude-sonnet-4" {
		t.Errorf("Model = %q", cfg.Model)
	}
	if cfg.SystemPrompt != "Review carefully." {
		t.Errorf("SystemPrompt = %q", cfg.SystemPrompt)
	}
	if cfg.Timeout != 2*time.Minute {
		t.Errorf("Timeout = %v", cfg.Timeout)
	}
	if cfg.Temperature == nil || *cfg.Temperature != 0.2 {
		t.Errorf("Temperature = %v", cfg.Temperature)
	}
}

func TestApplyAgentDefinition_ExplicitArgsWin(t *testing.T) {
	defTemp := float32(0.2)
	cfgTemp := float32(0.9)
	def := &AgentDefinition{
		Name:         "reviewer",
		Model:        "anthropic/claude-sonnet-4",
		SystemPrompt: "Review carefully.",
		Timeout:      2 * time.Minute,
		Temperature:  &defTemp,
	}
	cfg := SubagentConfig{
		Prompt:       "review this",
		Agent:        "reviewer",
		Model:        "openai/gpt-4o",
		SystemPrompt: "Custom prompt.",
		Timeout:      time.Minute,
		Temperature:  &cfgTemp,
	}

	applyAgentDefinition(&cfg, def)

	if cfg.Model != "openai/gpt-4o" {
		t.Errorf("explicit model should win, got %q", cfg.Model)
	}
	if cfg.SystemPrompt != "Custom prompt." {
		t.Errorf("explicit system prompt should win, got %q", cfg.SystemPrompt)
	}
	if cfg.Timeout != time.Minute {
		t.Errorf("explicit timeout should win, got %v", cfg.Timeout)
	}
	if cfg.Temperature == nil || *cfg.Temperature != 0.9 {
		t.Errorf("explicit temperature should win, got %v", cfg.Temperature)
	}
}

func TestFilterToolsByName(t *testing.T) {
	tools := SubagentTools()
	filtered := filterToolsByName(tools, []string{"read", "grep", "nonexistent"})

	if len(filtered) != 2 {
		t.Fatalf("filtered %d tools, want 2", len(filtered))
	}
	names := map[string]bool{}
	for _, tl := range filtered {
		names[tl.Info().Name] = true
	}
	if !names["read"] || !names["grep"] {
		t.Errorf("filtered set = %v, want read+grep", names)
	}
}

func TestFilterToolsByName_EmptyResultIsNotNil(t *testing.T) {
	filtered := filterToolsByName(SubagentTools(), []string{"nonexistent"})
	if filtered == nil {
		t.Fatal("empty allowlist match must return a non-nil slice (explicit empty tool set)")
	}
	if len(filtered) != 0 {
		t.Errorf("filtered = %v, want empty", filtered)
	}
}

func TestNamedAgentSpecs_ExcludesHidden(t *testing.T) {
	defs := []*AgentDefinition{
		{Name: "visible", Description: "shown", Tools: []string{"read"}},
		{Name: "secret", Description: "hidden", Hidden: true},
	}

	specs := namedAgentSpecs(defs)

	if len(specs) != 1 {
		t.Fatalf("got %d specs, want 1", len(specs))
	}
	if specs[0].Name != "visible" || specs[0].Description != "shown" {
		t.Errorf("spec = %+v", specs[0])
	}
	if len(specs[0].Tools) != 1 || specs[0].Tools[0] != "read" {
		t.Errorf("spec tools = %v", specs[0].Tools)
	}
}

func TestResolveAgentDefinition_Unknown(t *testing.T) {
	k := &Kit{namedAgents: []*AgentDefinition{
		{Name: "general", Description: "g"},
		{Name: "explore", Description: "e"},
		{Name: "secret", Description: "s", Hidden: true},
	}}
	cfg := SubagentConfig{Prompt: "p", Agent: "nope"}

	_, err := k.resolveAgentDefinition(&cfg)
	if err == nil {
		t.Fatal("expected error for unknown agent")
	}
	msg := err.Error()
	if !strings.Contains(msg, `"nope"`) {
		t.Errorf("error should name the unknown agent: %v", err)
	}
	if !strings.Contains(msg, "general") || !strings.Contains(msg, "explore") {
		t.Errorf("error should list available agents: %v", err)
	}
	if strings.Contains(msg, "secret") {
		t.Errorf("error should not reveal hidden agents: %v", err)
	}
}

func TestResolveAgentDefinition_AppliesAllowlist(t *testing.T) {
	k := &Kit{namedAgents: []*AgentDefinition{
		{Name: "explore", Description: "e", Tools: []string{"read", "grep", "find", "ls"}},
	}}
	cfg := SubagentConfig{Prompt: "p", Agent: "explore"}

	restricted, err := k.resolveAgentDefinition(&cfg)
	if err != nil {
		t.Fatalf("resolve failed: %v", err)
	}
	if !restricted {
		t.Error("allowlisted agent should report restricted=true")
	}
	if len(cfg.Tools) != 4 {
		t.Fatalf("cfg.Tools has %d tools, want 4", len(cfg.Tools))
	}
	for _, tl := range cfg.Tools {
		switch tl.Info().Name {
		case "read", "grep", "find", "ls":
		default:
			t.Errorf("unexpected tool %q in restricted set", tl.Info().Name)
		}
	}
}

func TestResolveAgentDefinition_ExplicitToolsIntersectAllowlist(t *testing.T) {
	// Explicit cfg.Tools is the base set: the agent allowlist narrows it
	// and cfg.Tools can never widen access beyond the allowlist. This is
	// deliberate — the internal spawner always passes inherited tools, and
	// a full override would bypass the allowlist.
	k := &Kit{namedAgents: []*AgentDefinition{
		{Name: "explore", Description: "e", Tools: []string{"read", "grep"}},
	}}
	cfg := SubagentConfig{
		Prompt: "p",
		Agent:  "explore",
		// Base set includes tools outside the allowlist (bash, write).
		Tools: SubagentTools(),
	}

	restricted, err := k.resolveAgentDefinition(&cfg)
	if err != nil {
		t.Fatalf("resolve failed: %v", err)
	}
	if !restricted {
		t.Error("allowlisted agent should report restricted=true")
	}
	if len(cfg.Tools) != 2 {
		t.Fatalf("cfg.Tools has %d tools, want 2 (intersection)", len(cfg.Tools))
	}
	for _, tl := range cfg.Tools {
		switch tl.Info().Name {
		case "read", "grep":
		default:
			t.Errorf("tool %q escaped the allowlist", tl.Info().Name)
		}
	}
}

func TestResolveAgentDefinition_HiddenIsResolvable(t *testing.T) {
	k := &Kit{namedAgents: []*AgentDefinition{
		{Name: "secret", Description: "s", Hidden: true, SystemPrompt: "shh"},
	}}
	cfg := SubagentConfig{Prompt: "p", Agent: "secret"}

	restricted, err := k.resolveAgentDefinition(&cfg)
	if err != nil {
		t.Fatalf("hidden agents must stay resolvable: %v", err)
	}
	if restricted {
		t.Error("no allowlist, should not be restricted")
	}
	if cfg.SystemPrompt != "shh" {
		t.Errorf("SystemPrompt = %q", cfg.SystemPrompt)
	}
}

func TestResolveAgentDefinition_NoAllowlistLeavesToolsNil(t *testing.T) {
	k := &Kit{namedAgents: []*AgentDefinition{
		{Name: "general", Description: "g"},
	}}
	cfg := SubagentConfig{Prompt: "p", Agent: "general"}

	restricted, err := k.resolveAgentDefinition(&cfg)
	if err != nil {
		t.Fatalf("resolve failed: %v", err)
	}
	if restricted {
		t.Error("unrestricted agent should report restricted=false")
	}
	if cfg.Tools != nil {
		t.Errorf("cfg.Tools should remain nil, got %d tools", len(cfg.Tools))
	}
}

func TestKitGetAgents(t *testing.T) {
	defs := []*AgentDefinition{
		{Name: "a", Description: "aa"},
		{Name: "b", Description: "bb"},
	}
	k := &Kit{namedAgents: defs}

	got := k.GetAgents()
	if len(got) != 2 {
		t.Fatalf("GetAgents returned %d, want 2", len(got))
	}
	// Snapshot semantics: mutating the returned slice must not affect Kit.
	got[0] = nil
	if k.namedAgents[0] == nil {
		t.Error("GetAgents must return a copy")
	}

	if a, ok := k.GetAgent("b"); !ok || a.Description != "bb" {
		t.Errorf("GetAgent(b) = %+v, %v", a, ok)
	}
	if _, ok := k.GetAgent("missing"); ok {
		t.Error("GetAgent(missing) should report false")
	}

	empty := &Kit{}
	if empty.GetAgents() != nil {
		t.Error("GetAgents on empty Kit should return nil")
	}
}

func TestKitGetAgents_DeepCopy(t *testing.T) {
	temp := float32(0.3)
	k := &Kit{namedAgents: []*AgentDefinition{
		{Name: "a", Description: "aa", Tools: []string{"read", "grep"}, Temperature: &temp},
	}}

	got := k.GetAgents()

	// Mutating the returned definition's fields must not leak into Kit state.
	got[0].Name = "mutated"
	got[0].Tools[0] = "bash"
	*got[0].Temperature = 0.9

	orig := k.namedAgents[0]
	if orig.Name != "a" {
		t.Errorf("struct field mutation leaked: Name = %q", orig.Name)
	}
	if orig.Tools[0] != "read" {
		t.Errorf("Tools slice mutation leaked: %v", orig.Tools)
	}
	if *orig.Temperature != 0.3 {
		t.Errorf("Temperature pointer mutation leaked: %v", *orig.Temperature)
	}
}

func TestAgentDefinitionAliasCompatibility(t *testing.T) {
	// AgentDefinition must remain an alias of the internal type so SDK
	// consumers can round-trip values through LoadAgentDefinitions.
	takeDef := func(d *AgentDefinition) string { return d.Name }
	if got := takeDef(&agents.Agent{Name: "x", Description: "y"}); got != "x" {
		t.Errorf("alias round-trip failed: %q", got)
	}
}
