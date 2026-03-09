package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

// skillCmd installs the kit-extensions skill via the skills.sh CLI (npx skills).
// This teaches AI agents how to create Kit extensions with full knowledge of
// the extension API, lifecycle events, widgets, tools, commands, and Yaegi constraints.
var skillCmd = &cobra.Command{
	Use:   "skill",
	Short: "Install the Kit extensions skill via skills.sh",
	Long: `Install the kit-extensions skill that teaches AI agents how to create
Kit extensions. Uses the skills.sh CLI (npx skills) to install the skill
from the Kit repository.

The skill provides comprehensive documentation of Kit's extension API including
lifecycle events, custom tools, slash commands, widgets, editor interceptors,
tool renderers, and critical Yaegi interpreter constraints.

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
		"--skill",
		"kit-extensions",
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
