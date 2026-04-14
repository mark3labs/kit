//go:build ignore

package main

import (
	"os/exec"
	"regexp"
	"strings"

	"kit/ext"
)

// re matches !{...} with non-greedy content.
var re = regexp.MustCompile(`!\{([^}]+)\}`)

// Init expands inline bash expressions in user prompts before they reach the
// LLM. Text like !{git rev-parse --abbrev-ref HEAD} is replaced with the
// command's stdout.
//
// In interactive mode the expansion happens at submit time via an editor
// interceptor, so the expanded text is also visible in the user message
// block on screen. In non-interactive mode (CLI, script, queue) the
// expansion happens via OnInput transform.
//
// Examples:
//
//	"Fix the tests on !{git rev-parse --abbrev-ref HEAD}"
//	  → "Fix the tests on main"
//
//	"The current directory is !{pwd}"
//	  → "The current directory is /home/user/project"
//
// Usage: kit -e examples/extensions/inline-bash.go
func Init(api ext.API) {
	// ── Interactive mode: editor interceptor ──────────────────────────
	// Intercept Enter / Ctrl+D so we can expand !{...} BEFORE the
	// SubmitMsg is created. This ensures the expanded text appears in
	// the user message block on screen as well as in the LLM prompt.
	api.OnSessionStart(func(_ ext.SessionStartEvent, ctx ext.Context) {
		if !ctx.Interactive {
			return
		}
		ctx.SetEditor(ext.EditorConfig{
			HandleKey: func(key string, currentText string) ext.EditorKeyAction {
				if (key == "enter" || key == "ctrl+d") && re.MatchString(currentText) {
					expanded := expand(currentText)
					// Clear the textarea asynchronously — calling
					// SetEditorText synchronously from inside Update()
					// would deadlock the BubbleTea event loop.
					go ctx.SetEditorText("")
					return ext.EditorKeyAction{
						Type:       ext.EditorKeySubmit,
						SubmitText: expanded,
					}
				}
				return ext.EditorKeyAction{Type: ext.EditorKeyPassthrough}
			},
		})
	})

	// ── Non-interactive fallback: OnInput transform ──────────────────
	// For CLI, script, and queue sources the editor interceptor is not
	// active, so we fall back to OnInput which still rewrites the
	// prompt text sent to the LLM.
	api.OnInput(func(ev ext.InputEvent, ctx ext.Context) *ext.InputResult {
		if ev.Source == "interactive" || !re.MatchString(ev.Text) {
			return nil
		}

		return &ext.InputResult{
			Action: "transform",
			Text:   expand(ev.Text),
		}
	})
}

// expand replaces every !{cmd} in text with the command's stdout.
// On error the original !{cmd} token is preserved.
func expand(text string) string {
	return re.ReplaceAllStringFunc(text, func(match string) string {
		cmd := re.FindStringSubmatch(match)[1]
		cmd = strings.TrimSpace(cmd)

		out, err := exec.Command("bash", "-c", cmd).Output()
		if err != nil {
			return match // keep original on error
		}
		return strings.TrimSpace(string(out))
	})
}
