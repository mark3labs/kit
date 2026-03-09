//go:build ignore

// Subagent Test Extension — Tests the new first-class subagent API
//
// Commands:
//
//	/subtest <task>      — spawn a blocking subagent and print result
//	/subbg <task>        — spawn a background subagent with live output
//
// Usage: kit -e examples/extensions/subagent-test.go
package main

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"kit/ext"
)

var (
	mu        sync.Mutex
	latestCtx ext.Context
	hasCtx    bool
)

func Init(api ext.API) {
	// Keep context fresh
	api.OnSessionStart(func(_ ext.SessionStartEvent, ctx ext.Context) {
		mu.Lock()
		latestCtx = ctx
		hasCtx = true
		mu.Unlock()

		ctx.PrintInfo(
			"Subagent Test Extension loaded\n\n" +
				"/subtest <task>    Spawn blocking subagent\n" +
				"/subbg <task>      Spawn background subagent\n\n" +
				"The LLM can also use the spawn_subagent tool.")
	})

	api.OnAgentEnd(func(_ ext.AgentEndEvent, ctx ext.Context) {
		mu.Lock()
		latestCtx = ctx
		mu.Unlock()
	})

	// Command: /subtest <task> — blocking subagent
	api.RegisterCommand(ext.CommandDef{
		Name:        "subtest",
		Description: "Spawn a blocking subagent: /subtest <task>",
		Execute: func(args string, ctx ext.Context) (string, error) {
			mu.Lock()
			latestCtx = ctx
			hasCtx = true
			mu.Unlock()

			task := strings.TrimSpace(args)
			if task == "" {
				return "Usage: /subtest <task>", nil
			}

			ctx.PrintInfo(fmt.Sprintf("Spawning blocking subagent for: %s", task))

			start := time.Now()
			_, result, err := ctx.SpawnSubagent(ext.SubagentConfig{
				Prompt:   task,
				Timeout:  2 * time.Minute,
				Blocking: true,
			})
			elapsed := time.Since(start)

			if err != nil {
				return fmt.Sprintf("Spawn error: %v", err), nil
			}

			if result == nil {
				return "No result returned", nil
			}

			if result.Error != nil {
				return fmt.Sprintf("Subagent failed (exit %d) after %ds: %v\n\nPartial output:\n%s",
					result.ExitCode, int(elapsed.Seconds()), result.Error, truncate(result.Response, 2000)), nil
			}

			response := fmt.Sprintf("Subagent completed in %ds", int(elapsed.Seconds()))
			if result.Usage != nil {
				response += fmt.Sprintf(" (tokens: %d in / %d out)", result.Usage.InputTokens, result.Usage.OutputTokens)
			}
			response += fmt.Sprintf("\n\nResult:\n%s", truncate(result.Response, 4000))

			return response, nil
		},
	})

	// Command: /subbg <task> — background subagent with callbacks
	api.RegisterCommand(ext.CommandDef{
		Name:        "subbg",
		Description: "Spawn a background subagent: /subbg <task>",
		Execute: func(args string, ctx ext.Context) (string, error) {
			mu.Lock()
			latestCtx = ctx
			hasCtx = true
			mu.Unlock()

			task := strings.TrimSpace(args)
			if task == "" {
				return "Usage: /subbg <task>", nil
			}

			ctx.PrintInfo(fmt.Sprintf("Spawning background subagent for: %s", task))

			start := time.Now()
			handle, _, err := ctx.SpawnSubagent(ext.SubagentConfig{
				Prompt:  task,
				Timeout: 2 * time.Minute,
				OnOutput: func(chunk string) {
					// Live output - could update a widget here
					fmt.Print(chunk)
				},
				OnComplete: func(result ext.SubagentResult) {
					elapsed := time.Since(start)

					mu.Lock()
					c := latestCtx
					ok := hasCtx
					mu.Unlock()

					if !ok {
						return
					}

					if result.Error != nil {
						c.SendMessage(fmt.Sprintf("Background subagent failed after %ds: %v",
							int(elapsed.Seconds()), result.Error))
						return
					}

					msg := fmt.Sprintf("Background subagent completed in %ds", int(elapsed.Seconds()))
					if result.Usage != nil {
						msg += fmt.Sprintf(" (tokens: %d in / %d out)", result.Usage.InputTokens, result.Usage.OutputTokens)
					}
					msg += fmt.Sprintf("\n\nResult:\n%s", truncate(result.Response, 4000))

					c.SendMessage(msg)
				},
			})

			if err != nil {
				return fmt.Sprintf("Spawn error: %v", err), nil
			}

			return fmt.Sprintf("Background subagent spawned (ID: %s). Results will be delivered when complete.", handle.ID), nil
		},
	})
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "\n\n... [truncated]"
}
