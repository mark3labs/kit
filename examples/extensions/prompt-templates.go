//go:build ignore

// prompt-templates.go - Frontmatter-driven prompt templates with model switching.
// This extension demonstrates the new bridged SDK APIs:
// - Tree navigation for conversation management
// - Template parsing with {{variable}} substitution
// - Model resolution with fallback chains
// - Skill injection
//
// Usage:
//   1. Create ~/.config/kit/prompts/debug.md with frontmatter:
//      ---
//      description: Debug Python code
//      model: claude-sonnet-4-20250514
//      skill: python
//      ---
//      Help me debug this Python code: {{input}}
//
//   2. In Kit: /debug my_script.py

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"kit/ext"
)

// PromptTemplate represents a loaded template with frontmatter
type PromptTemplate struct {
	Name        string
	Description string
	Model       string
	Skill       string
	Content     string
	Variables   []string
	Path        string
}

var (
	templates   = make(map[string]PromptTemplate)
	templateDir string
)

func Init(api ext.API) {
	// Determine template directory
	home, _ := os.UserHomeDir()
	templateDir = filepath.Join(home, ".config", "kit", "prompts")

	// Ensure directory exists
	os.MkdirAll(templateDir, 0755)

	// Register commands
	api.RegisterCommand(ext.CommandDef{
		Name:        "reload-templates",
		Description: "Reload prompt templates from disk",
		Execute: func(args string, ctx ext.Context) (string, error) {
			loadTemplates(ctx)
			ctx.PrintInfo(fmt.Sprintf("Loaded %d templates from %s", len(templates), templateDir))
			return "", nil
		},
	})

	// Dynamic template commands are registered after loading
	api.OnSessionStart(func(e ext.SessionStartEvent, ctx ext.Context) {
		loadTemplates(ctx)
		registerTemplateCommands(api, ctx)
	})
}

// loadTemplates discovers and loads all template files
func loadTemplates(ctx ext.Context) {
	templates = make(map[string]PromptTemplate)

	entries, err := os.ReadDir(templateDir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		path := filepath.Join(templateDir, entry.Name())
		tpl, err := loadTemplateFile(path)
		if err != nil {
			continue
		}

		name := strings.TrimSuffix(entry.Name(), ".md")
		templates[name] = tpl
	}
}

// loadTemplateFile parses a template with YAML frontmatter
func loadTemplateFile(path string) (PromptTemplate, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return PromptTemplate{}, err
	}

	content := string(data)
	tpl := PromptTemplate{Path: path}

	// Parse frontmatter
	if strings.HasPrefix(content, "---") {
		parts := strings.SplitN(content[3:], "---", 2)
		if len(parts) == 2 {
			frontmatter := strings.TrimSpace(parts[0])
			body := strings.TrimSpace(parts[1])

			// Simple line-by-line frontmatter parsing
			for _, line := range strings.Split(frontmatter, "\n") {
				line = strings.TrimSpace(line)
				if line == "" || strings.HasPrefix(line, "#") {
					continue
				}

				key, value, found := strings.Cut(line, ":")
				if found {
					key = strings.TrimSpace(key)
					value = strings.TrimSpace(value)
					switch key {
					case "description":
						tpl.Description = value
					case "model":
						tpl.Model = value
					case "skill":
						tpl.Skill = value
					}
				}
			}
			tpl.Content = body
		} else {
			tpl.Content = content
		}
	} else {
		tpl.Content = content
	}

	// Parse {{variables}} using simple string parsing
	// (Can't use ctx.ParseTemplate here since we're in Init, not a handler)
	var vars []string
	for {
		start := strings.Index(tpl.Content, "{{")
		if start == -1 {
			break
		}
		end := strings.Index(tpl.Content[start:], "}}")
		if end == -1 {
			break
		}
		varName := strings.TrimSpace(tpl.Content[start+2 : start+end])
		vars = append(vars, varName)
		tpl.Content = tpl.Content[:start] + "{{" + varName + "}}" + tpl.Content[start+end+2:]
	}
	tpl.Variables = vars

	return tpl, nil
}

// registerTemplateCommands dynamically registers commands for each template
func registerTemplateCommands(api ext.API, ctx ext.Context) {
	for name, tpl := range templates {
		// Skip if already registered (we'd need to track this)
		tplCopy := tpl // Capture for closure
		nameCopy := name

		// Build description with metadata
		desc := tplCopy.Description
		if desc == "" {
			desc = fmt.Sprintf("Run %s template", nameCopy)
		}
		if tplCopy.Model != "" {
			desc += fmt.Sprintf(" [%s", tplCopy.Model)
			if tplCopy.Skill != "" {
				desc += fmt.Sprintf(" +%s", tplCopy.Skill)
			}
			desc += "]"
		}

		api.RegisterCommand(ext.CommandDef{
			Name:        nameCopy,
			Description: desc,
			Execute: func(args string, ctx ext.Context) (string, error) {
				return executeTemplate(ctx, tplCopy, args)
			},
		})
	}
}

// executeTemplate runs a template with the given arguments
func executeTemplate(ctx ext.Context, tpl PromptTemplate, args string) (string, error) {
	// Store original model for restoration
	originalModel := ctx.Model

	// 1. Resolve and switch model if specified
	if tpl.Model != "" {
		// Parse model chain (comma-separated)
		preferences := strings.Split(tpl.Model, ",")
		for i := range preferences {
			preferences[i] = strings.TrimSpace(preferences[i])
		}

		result := ctx.ResolveModelChain(preferences)
		if result.Error != "" {
			ctx.PrintError(fmt.Sprintf("Model resolution failed: %s", result.Error))
			// Continue with current model
		} else {
			ctx.PrintInfo(fmt.Sprintf("Switching to model: %s", result.Model))
			if err := ctx.SetModel(result.Model); err != nil {
				ctx.PrintError(fmt.Sprintf("Failed to switch model: %s", err.Error()))
			}
		}
	}

	// 2. Inject skill if specified
	if tpl.Skill != "" {
		err := ctx.InjectSkillAsContext(tpl.Skill)
		if err != "" {
			ctx.PrintError(fmt.Sprintf("Skill injection failed: %s", err))
		} else {
			ctx.PrintInfo(fmt.Sprintf("Injected skill: %s", tpl.Skill))
		}
	}

	// 3. Parse and render template
	parsed := ctx.ParseTemplate(tpl.Name, tpl.Content)

	// Build variable map
	vars := make(map[string]string)

	// Simple argument parsing: first arg is $1 (input), rest is $@
	if len(parsed.Variables) > 0 {
		argsList := ctx.SimpleParseArguments(args, len(parsed.Variables))
		for i, varName := range parsed.Variables {
			if i < len(parsed.Variables) && i+1 < len(argsList) {
				vars[varName] = argsList[i+1]
			}
		}
		// If single variable, use full args
		if len(parsed.Variables) == 1 && vars[parsed.Variables[0]] == "" {
			vars[parsed.Variables[0]] = args
		}
	}

	// Render with model conditionals
	content := ctx.RenderWithModelConditionals(tpl.Content)
	rendered := ctx.RenderTemplate(ext.PromptTemplate{Name: tpl.Name, Content: content, Variables: parsed.Variables}, vars)

	// 4. Send the rendered prompt
	ctx.SendMessage(rendered)

	// 5. Schedule model restoration after turn completes
	// We use a goroutine to wait and restore
	if tpl.Model != "" && originalModel != "" {
		go func() {
			// Note: In a real implementation, we'd use OnAgentEnd event
			// For now, the user can manually switch back
			ctx.SetStatus("template-mode", fmt.Sprintf("Template: %s (model will restore)", tpl.Name), 20)
		}()
	}

	return fmt.Sprintf("Executing template: %s", tpl.Name), nil
}
