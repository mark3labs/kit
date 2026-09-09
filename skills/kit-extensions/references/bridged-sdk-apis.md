# Kit Extensions: Bridged SDK APIs

> Part of the `kit-extensions` skill. Read `SKILL.md` first for the overview and critical constraints.

## Bridged SDK APIs (New)

Extensions can now access powerful internal SDK capabilities that enable advanced features like conversation tree navigation, dynamic skill loading, template parsing, and model resolution.

### Tree Navigation

Navigate the conversation tree, summarize branches, and implement "fresh context" loops:

```go
// Get a specific node by ID with full metadata and children
node := ctx.GetTreeNode("entry-id")
// node.ID, node.ParentID, node.Type ("message"/"branch_summary"/etc)
// node.Role, node.Content, node.Model, node.Children ([]string)

// Get the current branch from root to leaf
branch := ctx.GetCurrentBranch()  // []ext.TreeNode

// Get child entry IDs of a node
children := ctx.GetChildren("entry-id")  // []string

// Navigate/fork to a different entry in the tree
result := ctx.NavigateTo("entry-id")  // ext.TreeNavigationResult{Success, Error}

// Summarize a range of the branch using LLM
summary := ctx.SummarizeBranch("from-id", "to-id")  // string

// Collapse a branch range into a summary entry (fresh context primitive)
result := ctx.CollapseBranch("from-id", "to-id", "summary text")
```

### Skill Loading

Load and inject skills dynamically at runtime:

```go
// Discover skills from standard locations
result := ctx.DiscoverSkills()  // ext.SkillLoadResult{Skills, Error}
// Standard locations: ~/.config/kit/skills/, .kit/skills/, .agents/skills/

// Load a specific skill file
skill, err := ctx.LoadSkill("/path/to/skill.md")  // (*ext.Skill, error string)
// skill.Name, skill.Description, skill.Content, skill.Tags, skill.When

// Load all skills from a directory
result := ctx.LoadSkillsFromDir("/path/to/skills")  // ext.SkillLoadResult

// Inject a skill as context (pre-loads for next turn)
err := ctx.InjectSkillAsContext("skill-name")  // error string

// Inject a skill file directly
err := ctx.InjectRawSkillAsContext("/path/to/skill.md")  // error string

// Get all discovered skills
skills := ctx.GetAvailableSkills()  // []ext.Skill
```

### Template Parsing

Parse and render templates with variable substitution:

```go
// Parse a template to extract {{variables}}
tpl := ctx.ParseTemplate("name", "Hello {{name}}, welcome to {{place}}!")
// tpl.Name, tpl.Content, tpl.Variables ([]string)

// Render a template with variable values
vars := map[string]string{"name": "Alice", "place": "Kit"}
rendered := ctx.RenderTemplate(tpl, vars)  // "Hello Alice, welcome to Kit!"

// Parse command-line style arguments
pattern := ext.ArgumentPattern{
    Positional: []string{"command", "target"},  // $1, $2
    Rest:       "args",                         // $@
    Flags:      map[string]string{"--loop": "loop", "-f": "force"},
}
result := ctx.ParseArguments("deploy staging --loop 5", pattern)
// result.Vars["command"] = "deploy"
// result.Vars["target"] = "staging"
// result.Flags["--loop"] = "5"

// Simple positional argument parsing ($1, $2, $@)
args := ctx.SimpleParseArguments("deploy staging --force", 2)
// args[0] = "deploy staging --force" (full input)
// args[1] = "deploy" ($1)
// args[2] = "staging" ($2)
// args[3] = "--force" ($@)

// Evaluate model conditionals with wildcards
matches := ctx.EvaluateModelConditional("claude-*")  // bool
// Patterns: * matches any, ? matches single char, comma = OR

// Render content with <if-model> conditionals
content := `<if-model is="claude-*">Hi Claude<else>Hi there</if-model>`
rendered := ctx.RenderWithModelConditionals(content)  // based on current model
```

### Model Resolution

Resolve model fallback chains and query capabilities:

```go
// Resolve a chain of model preferences (tries each until available)
result := ctx.ResolveModelChain([]string{
    "anthropic/claude-opus-4",
    "anthropic/claude-sonnet-4",
    "openai/gpt-4o",
})
// result.Model (selected), result.Capabilities, result.Attempted, result.Error

// Get capabilities for a specific model
caps, err := ctx.GetModelCapabilities("anthropic/claude-sonnet-4")
// caps.Provider, caps.ModelID, caps.ContextLimit, caps.Reasoning, caps.Streaming
// caps.Pricing — ext.ModelPricing, see below

// Check if a model is available (provider exists)
available := ctx.CheckModelAvailable("anthropic/claude-sonnet-4")  // bool

// Get current provider/model ID
provider := ctx.GetCurrentProvider()  // "anthropic"
modelID := ctx.GetCurrentModelID()    // "claude-sonnet-4"
```

### Model Pricing

`ModelCapabilities.Pricing` and `ModelInfoEntry.Pricing` expose registry token
costs in **USD per million tokens**, so a cost is `tokens * rate / 1_000_000`.

```go
caps, _ := ctx.GetModelCapabilities("")  // empty = current model
p := caps.Pricing
// p.Input, p.Output           float64 — $ per 1M tokens
// p.CacheRead, p.CacheWrite   float64 — only valid when the Has* flag is true
// p.HasCacheRead, p.HasCacheWrite bool
// p.Known                     bool
```

**Always check `Known` before rendering cost.** It is false for local models and
custom OpenAI-compatible endpoints, where every rate is zero — without the flag
an unpriced model is indistinguishable from a free one. Likewise, check
`HasCacheRead` rather than assuming a zero `CacheRead` means free cache reads.

Computing prompt-cache savings:

```go
usage := ctx.GetSessionUsage()
if p.Known && p.HasCacheRead {
    saved := float64(usage.TotalCacheReadTokens) * (p.Input - p.CacheRead) / 1000000
}
```

