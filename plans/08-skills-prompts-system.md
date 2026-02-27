# Plan 08: Skills & Prompts System

**Priority**: P2
**Effort**: Medium
**Goal**: Expose skills loading, prompt templates, and dynamic system prompt management. CLI and SDK share the same skills infrastructure.

## Background

Pi exports `loadSkills()`, `formatSkillsForPrompt()`, `PromptTemplate`, `expandPromptTemplate()`. Kit has an extension system but no "skills" concept (markdown-based instruction files) or prompt template system. This plan introduces a skills layer designed SDK-first.

## Prerequisites

- Plan 00 (Create `pkg/kit/`)
- Plan 02 (Richer type exports)

## Step-by-Step

### Step 1: Create `internal/skills/` package

**File**: `internal/skills/skills.go` — Skill loading and parsing

```go
type Skill struct {
    Name        string
    Description string
    Content     string
    Path        string
    Tags        []string
    When        string // "always", "on-demand", "file:*.go"
}

func LoadSkill(path string) (*Skill, error) { ... }       // Markdown with YAML frontmatter
func LoadSkillsFromDir(dir string) ([]*Skill, error) { ... } // .md/.txt files + SKILL.md subdirs
func LoadSkills(cwd string) ([]*Skill, error) { ... }     // Auto-discover .kit/skills/ + ~/.config/kit/skills/
func FormatForPrompt(skills []*Skill) string { ... }      // Format for system prompt
```

**File**: `internal/skills/templates.go` — Prompt templates

```go
type PromptTemplate struct {
    Name      string
    Content   string
    Variables []string
}

func NewPromptTemplate(name, content string) *PromptTemplate { ... }
func LoadPromptTemplate(path string) (*PromptTemplate, error) { ... }
func (t *PromptTemplate) Expand(values map[string]string) string { ... }
func (t *PromptTemplate) ExpandStrict(values map[string]string) (string, error) { ... }
```

**File**: `internal/skills/prompt_builder.go` — System prompt composition

```go
type PromptBuilder struct { ... }

func NewPromptBuilder(basePrompt string) *PromptBuilder { ... }
func (pb *PromptBuilder) WithSkills(skills []*Skill) *PromptBuilder { ... }
func (pb *PromptBuilder) WithSection(name, content string) *PromptBuilder { ... }
func (pb *PromptBuilder) Build() string { ... }
```

### Step 2: Export in SDK

**File**: `pkg/kit/skills.go` (new)

```go
package kit

import "github.com/mark3labs/kit/internal/skills"

type Skill = skills.Skill
type PromptTemplate = skills.PromptTemplate
type PromptBuilder = skills.PromptBuilder

func LoadSkill(path string) (*Skill, error) { return skills.LoadSkill(path) }
func LoadSkillsFromDir(dir string) ([]*Skill, error) { return skills.LoadSkillsFromDir(dir) }
func LoadSkills(cwd string) ([]*Skill, error) { return skills.LoadSkills(cwd) }
func FormatSkillsForPrompt(s []*Skill) string { return skills.FormatForPrompt(s) }
func NewPromptTemplate(name, content string) *PromptTemplate { return skills.NewPromptTemplate(name, content) }
func LoadPromptTemplate(path string) (*PromptTemplate, error) { return skills.LoadPromptTemplate(path) }
func NewPromptBuilder(basePrompt string) *PromptBuilder { return skills.NewPromptBuilder(basePrompt) }
```

### Step 3: Integrate skills into Kit Options

```go
type Options struct {
    // ... existing fields ...
    Skills    []string // Skill files/dirs to load (empty = auto-discover)
    SkillsDir string   // Override default skills directory
}
```

In `New()`, load skills and compose system prompt via `PromptBuilder`.

### Step 4: App-as-Consumer — CLI uses SDK for skills

Currently Kit's extension loader (`internal/extensions/loader.go`) discovers extensions from `.kit/extensions/` and `~/.config/kit/extensions/`. The skills system follows the same pattern but for instruction files.

The CLI should:
1. Use `kit.LoadSkills(cwd)` to discover skills
2. Pass them via `kit.Options{Skills: ...}` or let auto-discovery handle it
3. A `/skills` slash command in interactive mode could list loaded skills

The existing `.agents/skills/` directory in the repo (used by btca) aligns with this convention. The SDK auto-discovers from `.kit/skills/` to avoid conflict with the `.agents/` convention used by other tools.

### Step 5: Write tests and verify

```bash
go build -o output/kit ./cmd/kit
go test -race ./...
```

## Files Changed Summary

| Action | File | Change |
|--------|------|--------|
| CREATE | `internal/skills/skills.go` | Skill loading/parsing |
| CREATE | `internal/skills/templates.go` | PromptTemplate |
| CREATE | `internal/skills/prompt_builder.go` | System prompt composition |
| CREATE | `pkg/kit/skills.go` | Public SDK exports |
| EDIT | `pkg/kit/kit.go` | Skills option, auto-loading |

## Verification Checklist

- [ ] Skills with YAML frontmatter parse correctly
- [ ] Skills without frontmatter load (name from filename)
- [ ] PromptTemplate expansion works
- [ ] PromptBuilder composes multi-section prompts
- [ ] Auto-discovery finds skills in standard directories
- [ ] CLI uses SDK for skill loading
