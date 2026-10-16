// Package skilltool provides snapshot-backed skill activation.
package skilltool

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	"charm.land/fantasy"
	"github.com/mark3labs/kit/internal/skills"
)

type SkillProvider func() []*skills.Skill

type activationState struct {
	mu        sync.Mutex
	activated map[string][32]byte
}

type activateSkillTool struct {
	info            fantasy.ToolInfo
	provider        SkillProvider
	providerOptions fantasy.ProviderOptions
	state           *activationState
	selected        map[string]*skills.Skill
}

func (t *activateSkillTool) Info() fantasy.ToolInfo                   { return t.info }
func (t *activateSkillTool) ProviderOptions() fantasy.ProviderOptions { return t.providerOptions }
func (t *activateSkillTool) SetProviderOptions(opts fantasy.ProviderOptions) {
	t.providerOptions = opts
}

// New retains activation state even when the catalogue is empty. The agent
// calls Snapshot before wrapping tools for each model request.
func New(_ []string, provider SkillProvider) fantasy.AgentTool {
	if provider == nil {
		return nil
	}
	t := &activateSkillTool{info: fantasy.ToolInfo{Name: "activate_skill"}, provider: provider, state: &activationState{activated: make(map[string][32]byte)}}
	return t
}

// Snapshot returns an immutable definition, or nil if no names are eligible.
func (t *activateSkillTool) Snapshot() fantasy.AgentTool {
	selected := make(map[string]*skills.Skill)
	for _, s := range t.provider() {
		if s != nil && !s.DisableModelInvocation {
			selected[s.Name] = skills.Clone(s)
		}
	}
	if len(selected) == 0 {
		return nil
	}
	names := make([]string, 0, len(selected))
	for name := range selected {
		names = append(names, name)
	}
	sort.Strings(names)
	enum := make([]any, len(names))
	for i, name := range names {
		enum[i] = name
	}
	return &activateSkillTool{
		provider: t.provider, state: t.state, selected: selected, providerOptions: t.providerOptions,
		info: fantasy.ToolInfo{Name: "activate_skill", Description: "Activate a skill listed in <available_skills> to load its full instructions and bundled resource list.", Parameters: map[string]any{"name": map[string]any{"type": "string", "description": "The exact name of the skill to activate.", "enum": enum}}, Required: []string{"name"}},
	}
}

func fingerprint(s *skills.Skill) [32]byte {
	b, _ := json.Marshal([]string{s.Name, s.Content, s.Path})
	return sha256.Sum256(b)
}

func (t *activateSkillTool) Run(ctx context.Context, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
	if err := ctx.Err(); err != nil {
		return fantasy.ToolResponse{}, err
	}
	var args struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal([]byte(call.Input), &args); err != nil {
		return fantasy.NewTextErrorResponse(fmt.Sprintf("invalid arguments: %v", err)), nil
	}
	name := strings.TrimSpace(args.Name)
	if name == "" {
		return fantasy.NewTextErrorResponse("name is required"), nil
	}
	if t.selected == nil {
		snapshot := t.Snapshot()
		if snapshot == nil {
			return fantasy.NewTextErrorResponse(fmt.Sprintf("unknown skill %q", name)), nil
		}
		return snapshot.Run(ctx, call)
	}
	t.state.mu.Lock()
	defer t.state.mu.Unlock()
	selected := t.selected[name]
	var current *skills.Skill
	for _, s := range t.provider() {
		if s.Name == name && !s.DisableModelInvocation {
			current = s
			break
		}
	}
	if current == nil || selected == nil {
		return fantasy.NewTextErrorResponse(fmt.Sprintf("unknown skill %q; prepare a fresh request", name)), nil
	}
	version := fingerprint(selected)
	if fingerprint(current) != version {
		return fantasy.NewTextErrorResponse(fmt.Sprintf("skill %q changed; prepare a fresh request", name)), nil
	}
	if err := ctx.Err(); err != nil {
		return fantasy.ToolResponse{}, err
	}
	if t.state.activated[name] == version {
		return fantasy.NewTextResponse(fmt.Sprintf("Skill %q was already loaded earlier in this session.", name)), nil
	}
	content := skills.FormatActivation(selected)
	if err := ctx.Err(); err != nil {
		return fantasy.ToolResponse{}, err
	}
	t.state.activated[name] = version
	return fantasy.NewTextResponse(content), nil
}
