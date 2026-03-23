package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

// skillCmd installs Kit skills via the skills.sh CLI (npx skills).
var skillCmd = &cobra.Command{
	Use:   "skill",
	Short: "Install Kit skills via skills.sh",
	Long: `Install Kit skills that teach AI agents how to build with Kit.
Uses the skills.sh CLI (npx skills) to install all skills from the Kit repository.

Two skills are provided:

  1. Extensions — creating Kit extensions with full knowledge of the extension
     API, lifecycle events, widgets, tools, commands, editor interceptors,
     tool renderers, and Yaegi interpreter constraints.

  2. SDK — building AI-powered applications with the Kit Go SDK, including
     providers, agents, tools, and MCP integration.

Example:
  kit skill`,
	RunE: runSkill,
}

func init() {
	rootCmd.AddCommand(skillCmd)
}

func runSkill(_ *cobra.Command, _ []string) error {
	npx, err := exec.LookPath("npx")
	if err != nil {
		return fmt.Errorf("npx not found in PATH — install Node.js to use this command: %w", err)
	}

	args := []string{
		"skills",
		"add",
		"mark3labs/kit",
	}

	cmd := exec.Command(npx, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("skills install failed: %w", err)
	}

	return nil
}
