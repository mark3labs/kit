//go:build ignore

// phase-handoff.go demonstrates ctx.NewSession by automating the multi-phase
// workflow pattern: the agent works through a spec, writes a HANDOFF.md at
// the end of each phase, then a fresh session picks up where the last one
// left off.
//
// Two trigger modes are provided:
//
//  1. Automatic — when an assistant message ends with the sentinel
//     "<HANDOFF_READY>", the extension starts a new session and pre-loads
//     HANDOFF.md as the first prompt. Use this when you want the agent to
//     hand off control to itself with no user intervention.
//
//  2. Manual — the /handoff slash command starts a new session immediately
//     with the same handoff prompt. Useful when you finish a phase by hand
//     and want to clear the context window before the next one starts.
//
// Usage:
//
//	kit -e examples/extensions/phase-handoff.go
//
// Have your spec-driving agent write a HANDOFF.md at the end of each phase
// and finish its message with the literal string `<HANDOFF_READY>`. The
// next session boots automatically and reads HANDOFF.md as @file context.

package main

import (
	"strings"

	"kit/ext"
)

// HANDOFFSentinel is the marker the agent appends to its last message to
// request an automatic session switch. Change this to whatever fits your
// workflow.
const HANDOFFSentinel = "<HANDOFF_READY>"

// HANDOFFPrompt is the first prompt the new session receives. The leading
// "@HANDOFF.md" triggers Kit's @file expansion, inlining the handoff file's
// contents as XML-wrapped context.
const HANDOFFPrompt = "Read @HANDOFF.md and continue with the next phase."

func Init(api ext.API) {
	// Automatic trigger: detect the sentinel at the end of an agent turn.
	api.OnAgentEnd(func(e ext.AgentEndEvent, ctx ext.Context) {
		msgs := ctx.GetMessages()
		if len(msgs) == 0 {
			return
		}
		last := msgs[len(msgs)-1]
		if last.Role != "assistant" || !strings.Contains(last.Content, HANDOFFSentinel) {
			return
		}

		// NewSession blocks while the agent finishes settling and then while
		// the TUI completes the switch; run it in a goroutine so the agent's
		// turn-end pipeline isn't stalled. The internal wait-for-idle (added
		// in response to issue #63) makes this reliable even when post-turn
		// tooling (formatters, on-save hooks, hidden tool calls) extends the
		// busy window past AgentEnd.
		go func() {
			if err := ctx.NewSession(HANDOFFPrompt); err != nil {
				ctx.PrintError("phase-handoff: " + err.Error())
				return
			}
			ctx.PrintInfo("phase-handoff: started a fresh session from HANDOFF.md")
		}()
	})

	// Manual trigger: /handoff [optional override prompt]
	api.RegisterCommand(ext.CommandDef{
		Name:        "handoff",
		Description: "Start a new session, optionally with a custom prompt",
		Execute: func(args string, ctx ext.Context) (string, error) {
			prompt := strings.TrimSpace(args)
			if prompt == "" {
				prompt = HANDOFFPrompt
			}
			if err := ctx.NewSession(prompt); err != nil {
				return "", err
			}
			return "", nil
		},
	})

	// Optional safeguard: surface the next prompt so the user can confirm
	// before the auto-handoff proceeds. Set kit option "handoff.confirm=1"
	// to enable.
	api.OnBeforeSessionSwitch(func(e ext.BeforeSessionSwitchEvent, ctx ext.Context) *ext.BeforeSessionSwitchResult {
		if ctx.GetOption("handoff.confirm") != "1" {
			return nil
		}
		if e.InitialPrompt == "" {
			return nil
		}
		resp := ctx.PromptConfirm(ext.PromptConfirmConfig{
			Message:      "Start a new session with prompt:\n  " + e.InitialPrompt + "\n\nProceed?",
			DefaultValue: true,
		})
		if resp.Cancelled || !resp.Value {
			return &ext.BeforeSessionSwitchResult{
				Cancel: true,
				Reason: "handoff cancelled by user",
			}
		}
		return nil
	})
}
