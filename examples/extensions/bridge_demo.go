//go:build ignore

// bridge_demo.go - Demonstrates the new bridged SDK APIs for extensions.
// This extension showcases tree navigation, skill loading, template parsing,
// and model resolution capabilities.
package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"kit/ext"
)

var (
	discoveredSkills []ext.Skill
	currentBranch    []ext.TreeNode
)

func Init(api ext.API) {
	// Register /tree-info command to demonstrate tree navigation
	api.RegisterCommand(ext.CommandDef{
		Name:        "tree-info",
		Description: "Show current conversation tree information",
		Execute: func(args string, ctx ext.Context) (string, error) {
			branch := ctx.GetCurrentBranch()
			info := fmt.Sprintf("Current branch has %d nodes:\n", len(branch))
			for i, node := range branch {
				info += fmt.Sprintf("  [%d] %s (%s): %s...\n", i, node.Type, node.ID[:8], truncate(node.Content, 40))
			}
			ctx.PrintInfo(info)
			return "", nil
		},
	})

	// Register /discover-skills command
	api.RegisterCommand(ext.CommandDef{
		Name:        "discover-skills",
		Description: "Discover and list available skills",
		Execute: func(args string, ctx ext.Context) (string, error) {
			result := ctx.DiscoverSkills()
			if result.Error != "" {
				return "", fmt.Errorf("discovery failed: %s", result.Error)
			}
			discoveredSkills = result.Skills

			info := fmt.Sprintf("Discovered %d skills:\n", len(result.Skills))
			for _, s := range result.Skills {
				info += fmt.Sprintf("  - %s: %s\n", s.Name, s.Description)
			}
			ctx.PrintInfo(info)
			return "", nil
		},
	})

	// Register /parse-template command
	api.RegisterCommand(ext.CommandDef{
		Name:        "parse-template",
		Description: "Parse a template and show extracted variables",
		Execute: func(args string, ctx ext.Context) (string, error) {
			if args == "" {
				args = "Hello {{name}}, welcome to {{place}}!"
			}
			tpl := ctx.ParseTemplate("demo", args)
			info := fmt.Sprintf("Template: %s\nVariables: %v", tpl.Content, tpl.Variables)
			ctx.PrintInfo(info)
			return "", nil
		},
	})

	// Register /render-template command
	api.RegisterCommand(ext.CommandDef{
		Name:        "render-template",
		Description: "Render a template with variables (usage: /render-template name=John place=Kit)",
		Execute: func(args string, ctx ext.Context) (string, error) {
			tpl := ctx.ParseTemplate("demo", "Hello {{name}}, welcome to {{place}}!")
			vars := ctx.ParseArguments(args, ext.ArgumentPattern{
				Flags: map[string]string{"name": "name", "place": "place"},
			})
			rendered := ctx.RenderTemplate(tpl, vars.Vars)
			ctx.PrintInfo("Rendered: " + rendered)
			return "", nil
		},
	})

	// Register /check-model command
	api.RegisterCommand(ext.CommandDef{
		Name:        "check-model",
		Description: "Check model capabilities and availability",
		Execute: func(args string, ctx ext.Context) (string, error) {
			model := args
			if model == "" {
				model = ctx.Model
			}

			available := ctx.CheckModelAvailable(model)
			caps, err := ctx.GetModelCapabilities(model)

			info := fmt.Sprintf("Model: %s\n", model)
			info += fmt.Sprintf("Available: %v\n", available)
			if err == "" {
				info += fmt.Sprintf("Provider: %s\n", caps.Provider)
				info += fmt.Sprintf("Context Limit: %d\n", caps.ContextLimit)
				info += fmt.Sprintf("Reasoning: %v\n", caps.Reasoning)
			} else {
				info += fmt.Sprintf("Error: %s\n", err)
			}
			ctx.PrintInfo(info)
			return "", nil
		},
	})

	// Register /resolve-chain command
	api.RegisterCommand(ext.CommandDef{
		Name:        "resolve-chain",
		Description: "Resolve a model chain (usage: /resolve-chain claude-opus,gpt-4o,claude-sonnet)",
		Execute: func(args string, ctx ext.Context) (string, error) {
			if args == "" {
				args = "anthropic/claude-opus-4,anthropic/claude-sonnet-4,openai/gpt-4o"
			}
			prefs := ctx.SimpleParseArguments(args, 1)
			chain := []string{}
			if len(prefs) > 1 {
				// Split the first arg by comma
				for _, p := range strings.Split(prefs[1], ",") {
					p = strings.TrimSpace(p)
					if p != "" {
						chain = append(chain, p)
					}
				}
			}

			result := ctx.ResolveModelChain(chain)
			info, _ := json.MarshalIndent(result, "", "  ")
			ctx.PrintInfo("Resolution Result:\n" + string(info))
			return "", nil
		},
	})

	// Register /test-conditional command
	api.RegisterCommand(ext.CommandDef{
		Name:        "test-conditional",
		Description: "Test model conditional rendering",
		Execute: func(args string, ctx ext.Context) (string, error) {
			content := `<if-model is="claude-*">This is for Claude models<else>This is for other models</if-model>`
			rendered := ctx.RenderWithModelConditionals(content)
			ctx.PrintInfo("Input: " + content)
			ctx.PrintInfo("Output: " + rendered)
			ctx.PrintInfo(fmt.Sprintf("Current model matches 'claude-*': %v", ctx.EvaluateModelConditional("claude-*")))
			return "", nil
		},
	})

	// OnSessionStart: discover skills automatically
	api.OnSessionStart(func(e ext.SessionStartEvent, ctx ext.Context) {
		result := ctx.DiscoverSkills()
		if result.Error == "" && len(result.Skills) > 0 {
			discoveredSkills = result.Skills
			ctx.SetStatus("bridge-demo", fmt.Sprintf("%d skills", len(result.Skills)), 50)
		}
	})
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
