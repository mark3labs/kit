# Kit Extensions: Complete Example — Plan Mode

> Part of the `kit-extensions` skill. Read `SKILL.md` first for the overview and critical constraints.

## Complete Example: Plan Mode

A full extension that restricts the agent to read-only tools, with a slash command, keyboard shortcut, option, status bar indicator, and system prompt injection:

```go
//go:build ignore

package main

import (
    "strings"
    "kit/ext"
)

func Init(api ext.API) {
    readOnlyTools := []string{"read", "grep", "find", "ls"}
    var planActive bool

    api.RegisterOption(ext.OptionDef{
        Name:        "plan",
        Description: "Start in plan mode (read-only tools)",
        Default:     "false",
    })

    api.RegisterShortcut(ext.ShortcutDef{
        Key:         "ctrl+alt+p",
        Description: "Toggle plan/explore mode",
    }, func(ctx ext.Context) {
        planActive = !planActive
        applyMode(ctx, planActive, readOnlyTools)
    })

    api.RegisterCommand(ext.CommandDef{
        Name:        "plan",
        Description: "Toggle plan/explore mode",
        Execute: func(args string, ctx ext.Context) (string, error) {
            planActive = !planActive
            applyMode(ctx, planActive, readOnlyTools)
            return "", nil
        },
    })

    api.OnSessionStart(func(_ ext.SessionStartEvent, ctx ext.Context) {
        if strings.ToLower(ctx.GetOption("plan")) == "true" {
            planActive = true
            applyMode(ctx, true, readOnlyTools)
        }
    })

    api.OnBeforeAgentStart(func(_ ext.BeforeAgentStartEvent, ctx ext.Context) *ext.BeforeAgentStartResult {
        if !planActive {
            return nil
        }
        prompt := `You are in PLAN MODE (read-only). You can ONLY read and search.
Focus on understanding, analysis, and generating plans.`
        return &ext.BeforeAgentStartResult{SystemPrompt: &prompt}
    })
}

func applyMode(ctx ext.Context, active bool, tools []string) {
    if active {
        ctx.SetActiveTools(tools)
        ctx.SetStatus("plan-mode", "PLAN MODE (read-only)", 10)
        ctx.PrintInfo("Plan mode ON")
    } else {
        ctx.SetActiveTools(nil)
        ctx.RemoveStatus("plan-mode")
        ctx.PrintInfo("Plan mode OFF")
    }
}
```

