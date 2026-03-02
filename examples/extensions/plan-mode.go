//go:build ignore

package main

import (
	"strings"

	"kit/ext"
)

// Init implements a plan/explore mode that restricts the agent to read-only
// tools. Toggle with /plan (or start in plan mode via KIT_OPT_PLAN=true).
// Inspired by Pi's plan-mode extension.
//
// In plan mode the agent can only use read, grep, find, and ls — it cannot
// write files, run bash, or make edits. This is useful for exploring a
// codebase, reviewing architecture, or generating plans before executing.
//
// The status bar shows the current mode and the system prompt is augmented
// with planning instructions when active.
//
// Usage: kit -e examples/extensions/plan-mode.go
//
// Start in plan mode: KIT_OPT_PLAN=true kit -e examples/extensions/plan-mode.go
func Init(api ext.API) {
	// Read-only tool set (matches core.ReadOnlyTools).
	readOnlyTools := []string{"read", "grep", "find", "ls"}

	var planActive bool

	// Register "plan" option so users can start in plan mode via env/config.
	api.RegisterOption(ext.OptionDef{
		Name:        "plan",
		Description: "Start in plan mode (read-only tools)",
		Default:     "false",
	})

	// /plan — toggle plan mode on or off.
	api.RegisterCommand(ext.CommandDef{
		Name:        "plan",
		Description: "Toggle plan/explore mode (read-only tools)",
		Execute: func(args string, ctx ext.Context) (string, error) {
			planActive = !planActive
			applyMode(ctx, planActive, readOnlyTools)
			return "", nil
		},
	})

	// Check option at session start to enable plan mode from env/config.
	api.OnSessionStart(func(_ ext.SessionStartEvent, ctx ext.Context) {
		opt := strings.ToLower(ctx.GetOption("plan"))
		if opt == "true" || opt == "1" || opt == "yes" {
			planActive = true
			applyMode(ctx, true, readOnlyTools)
		}
	})

	// Inject planning instructions into the system prompt when active.
	api.OnBeforeAgentStart(func(_ ext.BeforeAgentStartEvent, ctx ext.Context) *ext.BeforeAgentStartResult {
		if !planActive {
			return nil
		}
		prompt := `You are in PLAN MODE (read-only exploration).
You can ONLY read, search, and explore the codebase. You CANNOT write files,
run commands, or make edits. Focus on:
- Understanding the codebase structure and architecture
- Identifying relevant files and patterns
- Generating detailed plans and recommendations
- Answering questions about how the code works

When the user is ready to execute, they will exit plan mode with /plan.`
		return &ext.BeforeAgentStartResult{
			SystemPrompt: &prompt,
		}
	})
}

func applyMode(ctx ext.Context, active bool, readOnlyTools []string) {
	if active {
		ctx.SetActiveTools(readOnlyTools)
		ctx.SetStatus("plan-mode", "PLAN MODE (read-only)", 10)
		ctx.PrintInfo("Plan mode ON — agent restricted to read-only tools (read, grep, find, ls).\nUse /plan to toggle off.")
	} else {
		ctx.SetActiveTools(nil) // re-enable all tools
		ctx.RemoveStatus("plan-mode")
		ctx.PrintInfo("Plan mode OFF — all tools re-enabled.")
	}
}
