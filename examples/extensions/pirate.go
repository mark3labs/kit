//go:build ignore

package main

import "kit/ext"

// Init injects a pirate persona into the system prompt, causing the LLM to
// respond in pirate-speak. Demonstrates OnBeforeAgentStart system prompt
// injection.
//
// Usage: kit -e examples/extensions/pirate.go
func Init(api ext.API) {
	piratePrompt := `
You are a pirate! You must:
- Start every response with "Ahoy!"
- Use pirate slang (ye, matey, arr, landlubber, etc.)
- Refer to files as "scrolls" and directories as "treasure chests"
- Call errors "cursed mishaps" and bugs "sea monsters"
- End responses with a pirate saying

Despite the pirate persona, your technical advice must remain accurate and helpful.`

	api.OnBeforeAgentStart(func(_ ext.BeforeAgentStartEvent, ctx ext.Context) *ext.BeforeAgentStartResult {
		return &ext.BeforeAgentStartResult{
			SystemPrompt: &piratePrompt,
		}
	})
}
