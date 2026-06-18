// Package skilltool provides the built-in activate_skill tool, a dedicated
// activation entry point for agentskills.io skills (issue #65, gaps #13/#14).
//
// While a skill can always be activated by reading its SKILL.md with the
// generic read tool, a dedicated tool offers an enum-constrained skill name
// (preventing hallucinated names), bundled-resource enumeration, and
// per-session deduplication so the same skill is not re-injected repeatedly.
package skilltool

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	"charm.land/fantasy"

	"github.com/mark3labs/kit/internal/skills"
)

// SkillProvider returns the skills currently available for activation. It is
// queried on every call so runtime skill mutations are reflected.
type SkillProvider func() []*skills.Skill

type activateArgs struct {
	Name string `json:"name"`
}

// activateSkillTool implements fantasy.AgentTool.
type activateSkillTool struct {
	info            fantasy.ToolInfo
	provider        SkillProvider
	providerOptions fantasy.ProviderOptions

	mu        sync.Mutex
	activated map[string]bool // session-level dedup tracking
}

func (t *activateSkillTool) Info() fantasy.ToolInfo                   { return t.info }
func (t *activateSkillTool) ProviderOptions() fantasy.ProviderOptions { return t.providerOptions }
func (t *activateSkillTool) SetProviderOptions(opts fantasy.ProviderOptions) {
	t.providerOptions = opts
}

// New builds the activate_skill tool. names is the initial set of skill names
// used to populate the enum constraint on the name parameter; provider is
// queried at call time to resolve the skill by name (so runtime additions
// resolve even if absent from the enum). Returns nil when no skill names are
// available.
func New(names []string, provider SkillProvider) fantasy.AgentTool {
	if len(names) == 0 || provider == nil {
		return nil
	}
	sorted := append([]string(nil), names...)
	sort.Strings(sorted)
	enum := make([]any, len(sorted))
	for i, n := range sorted {
		enum[i] = n
	}

	return &activateSkillTool{
		info: fantasy.ToolInfo{
			Name: "activate_skill",
			Description: "Activate a skill by name to load its full instructions into context. " +
				"Use this when a task matches a skill listed in <available_skills>. " +
				"The skill body and a list of its bundled resources are returned.",
			Parameters: map[string]any{
				"name": map[string]any{
					"type":        "string",
					"description": "The exact name of the skill to activate.",
					"enum":        enum,
				},
			},
			Required: []string{"name"},
			Parallel: false,
		},
		provider:  provider,
		activated: map[string]bool{},
	}
}

func (t *activateSkillTool) Run(_ context.Context, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
	var args activateArgs
	if call.Input != "" && call.Input != "{}" {
		if err := json.Unmarshal([]byte(call.Input), &args); err != nil {
			return fantasy.NewTextErrorResponse(fmt.Sprintf("invalid arguments: %v", err)), nil
		}
	}
	name := strings.TrimSpace(args.Name)
	if name == "" {
		return fantasy.NewTextErrorResponse("name is required"), nil
	}

	// Hold the lock across the whole activation so the dedup check and the
	// subsequent mark are atomic — two concurrent calls cannot both pass the
	// check and double-activate the same skill (gap #14). The skill is only
	// marked activated on success, so a failed load can be retried.
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.activated[name] {
		return fantasy.NewTextResponse(
			fmt.Sprintf("Skill %q was already loaded earlier in this session.", name)), nil
	}

	// Resolve the skill path from the current provider snapshot. Skills with
	// disable-model-invocation set are not activatable by the model (they
	// remain available via the /skill: command), mirroring their exclusion
	// from the catalog and the tool's name enum.
	var path string
	for _, s := range t.provider() {
		if s.Name == name && !s.DisableModelInvocation {
			path = s.Path
			break
		}
	}
	if path == "" {
		return fantasy.NewTextErrorResponse(fmt.Sprintf("unknown skill %q", name)), nil
	}

	// Re-read the file for freshness, stripping frontmatter.
	loaded, err := skills.LoadSkill(path)
	if err != nil {
		return fantasy.NewTextErrorResponse(fmt.Sprintf("failed to load skill %q: %v", name, err)), nil
	}

	var buf strings.Builder
	fmt.Fprintf(&buf, "<skill_content name=%q location=%q>\n", loaded.Name, loaded.Path)
	fmt.Fprintf(&buf, "References are relative to %s.\n\n", loaded.BaseDir())
	buf.WriteString(loaded.Content)
	if res := skills.FormatResources(loaded.Resources()); res != "" {
		buf.WriteString("\n\n")
		buf.WriteString(res)
	}
	buf.WriteString("\n</skill_content>")

	t.activated[name] = true

	return fantasy.NewTextResponse(buf.String()), nil
}
