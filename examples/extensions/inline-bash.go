//go:build ignore

package main

import (
	"os/exec"
	"regexp"
	"strings"

	"kit/ext"
)

// Init expands inline bash expressions in user prompts before they reach the
// LLM. Text like !{git branch --show-current} is replaced with the command's
// stdout. Inspired by Pi's inline-bash.ts.
//
// Examples:
//
//	"Fix the tests on !{git branch --show-current}"
//	  → "Fix the tests on main"
//
//	"The current directory is !{pwd}"
//	  → "The current directory is /home/user/project"
//
// Usage: kit -e examples/extensions/inline-bash.go
func Init(api ext.API) {
	// Matches !{...} with non-greedy content.
	re := regexp.MustCompile(`!\{([^}]+)\}`)

	api.OnInput(func(ev ext.InputEvent, ctx ext.Context) *ext.InputResult {
		if !re.MatchString(ev.Text) {
			return nil
		}

		expanded := re.ReplaceAllStringFunc(ev.Text, func(match string) string {
			// Extract the command between !{ and }.
			cmd := re.FindStringSubmatch(match)[1]
			cmd = strings.TrimSpace(cmd)

			out, err := exec.Command("bash", "-c", cmd).Output()
			if err != nil {
				return match // keep original on error
			}
			return strings.TrimSpace(string(out))
		})

		return &ext.InputResult{
			Action: "transform",
			Text:   expanded,
		}
	})
}
