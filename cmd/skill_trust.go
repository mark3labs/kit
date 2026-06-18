package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"

	"github.com/mark3labs/kit/pkg/kit"
)

// skillTrustPrompt returns a callback that gates project-local skill loading
// on an interactive trust decision (issue #65, gap #8). Project-local skills
// are injected into the system prompt, so a freshly cloned untrusted repo
// could smuggle instructions into the agent. The prompt asks the user whether
// to trust the directory before any project skill is loaded.
//
// It returns nil — meaning "load without prompting" — when Kit is not running
// interactively (a non-TTY stdin, --quiet, or a non-interactive one-shot
// prompt), so scripted and piped invocations keep their existing behaviour.
func skillTrustPrompt() func(projectDir string, skillCount int) kit.TrustDecision {
	// Only prompt for interactive terminal sessions.
	if quietFlag || positionalPrompt != "" {
		return nil
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return nil
	}

	return func(projectDir string, skillCount int) kit.TrustDecision {
		noun := "skills"
		if skillCount == 1 {
			noun = "skill"
		}
		fmt.Printf("\nThis project provides %d %s under .agents/skills or .kit/skills:\n  %s\n",
			skillCount, noun, projectDir)
		fmt.Print("Load them into the agent? [t]rust always / [o]nce / [s]kip (default skip): ")

		reader := bufio.NewReader(os.Stdin)
		line, _ := reader.ReadString('\n')
		switch strings.ToLower(strings.TrimSpace(line)) {
		case "t", "trust", "a", "always":
			return kit.TrustProject
		case "o", "once", "y", "yes":
			return kit.TrustProjectOnce
		default:
			return kit.SkipProjectSkills
		}
	}
}
