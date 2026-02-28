//go:build ignore

package main

import (
	"fmt"
	"strings"

	"kit/ext"
)

// Init demonstrates the interactive prompt system. It registers three slash
// commands that show each prompt type (select, confirm, input), plus a
// combined workflow command that chains prompts together.
func Init(api ext.API) {

	// /demo-select — shows a selection list.
	api.RegisterCommand(ext.CommandDef{
		Name:        "demo-select",
		Description: "Demo: pick from a list",
		Execute: func(args string, ctx ext.Context) (string, error) {
			result := ctx.PromptSelect(ext.PromptSelectConfig{
				Message: "Choose your deployment target:",
				Options: []string{"local", "staging", "production"},
			})
			if result.Cancelled {
				return "Selection cancelled.", nil
			}
			return fmt.Sprintf("Selected: %s (index %d)", result.Value, result.Index), nil
		},
	})

	// /demo-confirm — shows a yes/no confirmation.
	api.RegisterCommand(ext.CommandDef{
		Name:        "demo-confirm",
		Description: "Demo: yes/no confirmation",
		Execute: func(args string, ctx ext.Context) (string, error) {
			result := ctx.PromptConfirm(ext.PromptConfirmConfig{
				Message:      "Are you sure you want to deploy?",
				DefaultValue: false,
			})
			if result.Cancelled {
				return "Confirmation cancelled.", nil
			}
			if result.Value {
				return "Confirmed! Deploying...", nil
			}
			return "Declined. Deployment aborted.", nil
		},
	})

	// /demo-input — shows a text input.
	api.RegisterCommand(ext.CommandDef{
		Name:        "demo-input",
		Description: "Demo: free-form text input",
		Execute: func(args string, ctx ext.Context) (string, error) {
			result := ctx.PromptInput(ext.PromptInputConfig{
				Message:     "Enter the release tag:",
				Placeholder: "v1.0.0",
			})
			if result.Cancelled {
				return "Input cancelled.", nil
			}
			return fmt.Sprintf("Release tag: %s", result.Value), nil
		},
	})

	// /demo-workflow — chains multiple prompts into a workflow.
	api.RegisterCommand(ext.CommandDef{
		Name:        "demo-workflow",
		Description: "Demo: chained prompt workflow",
		Execute: func(args string, ctx ext.Context) (string, error) {
			// Step 1: select environment
			env := ctx.PromptSelect(ext.PromptSelectConfig{
				Message: "Step 1/3: Select environment:",
				Options: []string{"development", "staging", "production"},
			})
			if env.Cancelled {
				return "Workflow cancelled at step 1.", nil
			}

			// Step 2: enter version tag
			tag := ctx.PromptInput(ext.PromptInputConfig{
				Message:     "Step 2/3: Enter the version tag:",
				Placeholder: "v1.0.0",
			})
			if tag.Cancelled {
				return "Workflow cancelled at step 2.", nil
			}

			// Step 3: confirm
			confirm := ctx.PromptConfirm(ext.PromptConfirmConfig{
				Message: fmt.Sprintf(
					"Step 3/3: Deploy %s to %s?",
					tag.Value, env.Value),
				DefaultValue: false,
			})
			if confirm.Cancelled {
				return "Workflow cancelled at step 3.", nil
			}
			if !confirm.Value {
				return "Deployment declined.", nil
			}

			var summary strings.Builder
			summary.WriteString("Deployment summary:\n")
			fmt.Fprintf(&summary, "  Environment: %s\n", env.Value)
			fmt.Fprintf(&summary, "  Version:     %s\n", tag.Value)
			summary.WriteString("  Status:      initiated")
			return summary.String(), nil
		},
	})
}
