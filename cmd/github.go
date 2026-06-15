package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"charm.land/huh/v2"
	"github.com/charmbracelet/log"
	kit "github.com/mark3labs/kit/pkg/kit"
	"github.com/spf13/cobra"
)

// defaultGitHubModel is the model written into the generated workflow when the
// user does not specify one and runs non-interactively.
const defaultGitHubModel = "anthropic/claude-sonnet-4-5-20250929"

// githubWorkflowPath is the repository-relative location of the generated
// GitHub Actions workflow that wires Kit into a repository as a collaborator.
const githubWorkflowPath = ".github/workflows/kit.yml"

var (
	githubInstallModel    string
	githubInstallForce    bool
	githubInstallNoSecret bool
)

// githubCmd is the parent command for GitHub integration subcommands. It groups
// the turnkey setup tooling that wires Kit into a repository as an automated
// collaborator/reviewer driven by GitHub Actions.
var githubCmd = &cobra.Command{
	Use:   "github",
	Short: "Set up Kit as a GitHub collaborator/reviewer",
	Long: `Set up Kit as an automated collaborator/reviewer in a GitHub repository.

Kit runs inside a GitHub Actions runner, reads the relevant context (an issue
thread or pull request), runs the agent non-interactively, and responds by
posting comments and opening pull requests.

Use 'kit github install' to scaffold the GitHub Actions workflow.`,
}

// githubInstallCmd scaffolds the GitHub Actions workflow that runs Kit on
// '/kit' comment triggers. It writes .github/workflows/kit.yml and, when the
// 'gh' CLI is available, offers to set the provider API key as a repository
// secret.
var githubInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Scaffold the GitHub Actions workflow that runs Kit",
	Long: `Scaffold the GitHub Actions workflow that runs Kit as a collaborator.

This writes .github/workflows/kit.yml configured to trigger when someone
comments '/kit ...' on an issue or pull request review. The workflow runs Kit
inside an ephemeral Actions runner with least-privilege permissions and
'persist-credentials: false', mirroring established security practice.

If the GitHub CLI ('gh') is detected on your PATH, you will be offered the
option to store your provider API key as a repository secret automatically.

Flags:
  --model       Provider/model to write into the workflow (e.g. anthropic/claude-sonnet-4-5)
  --force       Overwrite an existing workflow file
  --no-secret   Skip the offer to set the provider secret via the gh CLI

Examples:
  kit github install
  kit github install --model anthropic/claude-sonnet-4-5-20250929
  kit github install --force --no-secret`,
	Args: cobra.NoArgs,
	RunE: runGitHubInstall,
}

func init() {
	githubInstallCmd.Flags().StringVarP(&githubInstallModel, "model", "m", "", "provider/model to write into the workflow")
	githubInstallCmd.Flags().BoolVar(&githubInstallForce, "force", false, "overwrite an existing workflow file")
	githubInstallCmd.Flags().BoolVar(&githubInstallNoSecret, "no-secret", false, "skip setting the provider secret via the gh CLI")

	githubCmd.AddCommand(githubInstallCmd)
	rootCmd.AddCommand(githubCmd)
}

func runGitHubInstall(_ *cobra.Command, _ []string) error {
	model, err := resolveGitHubModel()
	if err != nil {
		return err
	}

	provider, _, err := kit.ParseModelString(model)
	if err != nil {
		return fmt.Errorf("invalid model %q: %w", model, err)
	}

	secretName := providerSecretEnvVar(provider)

	if err := writeGitHubWorkflow(model, secretName, githubInstallForce); err != nil {
		return err
	}
	fmt.Printf("✅ Wrote %s\n", githubWorkflowPath)

	maybeSetProviderSecret(secretName)

	printGitHubInstallNextSteps(secretName)
	log.Info("github workflow scaffolded", "model", model, "secret", secretName)
	return nil
}

// resolveGitHubModel determines the model to embed in the workflow. The
// --model flag takes precedence; otherwise an interactive prompt is shown
// (pre-filled with the default), and non-interactive runs use the default.
func resolveGitHubModel() (string, error) {
	if githubInstallModel != "" {
		return strings.TrimSpace(githubInstallModel), nil
	}

	if !isInteractive() {
		return defaultGitHubModel, nil
	}

	model := defaultGitHubModel
	err := huh.NewInput().
		Title("Model").
		Description("Provider/model Kit should use in CI (e.g. anthropic/claude-sonnet-4-5)").
		Value(&model).
		Run()
	if err != nil {
		return "", fmt.Errorf("model selection cancelled: %w", err)
	}

	model = strings.TrimSpace(model)
	if model == "" {
		return "", fmt.Errorf("model cannot be empty")
	}
	return model, nil
}

// providerSecretEnvVar returns the environment variable / repository secret
// name that holds the API key for the given provider. It consults the model
// registry and falls back to "<PROVIDER>_API_KEY" for unknown providers.
func providerSecretEnvVar(provider string) string {
	if info := kit.GetProviderInfo(provider); info != nil && len(info.Env) > 0 {
		return info.Env[0]
	}
	sanitized := strings.ToUpper(strings.NewReplacer("-", "_", ".", "_").Replace(provider))
	return sanitized + "_API_KEY"
}

// renderGitHubWorkflow builds the workflow YAML for the given model and
// provider secret name.
func renderGitHubWorkflow(model, secretName string) string {
	return fmt.Sprintf(`name: kit
on:
  issue_comment:
    types: [created]
  pull_request_review_comment:
    types: [created]
jobs:
  kit:
    if: |
      startsWith(github.event.comment.body, '/kit') ||
      contains(github.event.comment.body, ' /kit')
    runs-on: ubuntu-latest
    permissions:
      contents: write
      pull-requests: write
      issues: write
    steps:
      - uses: actions/checkout@v4
        with:
          persist-credentials: false
      - uses: mark3labs/kit-action@v1
        with:
          model: %s
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          %s: ${{ secrets.%s }}
`, model, secretName, secretName)
}

// writeGitHubWorkflow writes the generated workflow to githubWorkflowPath,
// creating parent directories as needed. It refuses to overwrite an existing
// file unless force is true.
func writeGitHubWorkflow(model, secretName string, force bool) error {
	if _, err := os.Stat(githubWorkflowPath); err == nil && !force {
		return fmt.Errorf("%s already exists; re-run with --force to overwrite", githubWorkflowPath)
	} else if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("checking %s: %w", githubWorkflowPath, err)
	}

	if err := os.MkdirAll(filepath.Dir(githubWorkflowPath), 0o755); err != nil {
		return fmt.Errorf("creating %s: %w", filepath.Dir(githubWorkflowPath), err)
	}

	content := renderGitHubWorkflow(model, secretName)
	if err := os.WriteFile(githubWorkflowPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", githubWorkflowPath, err)
	}
	return nil
}

// maybeSetProviderSecret offers to set the provider API key as a repository
// secret via the gh CLI when it is available, interactive, the secret value is
// present in the environment, and the user did not pass --no-secret.
func maybeSetProviderSecret(secretName string) {
	if githubInstallNoSecret || !isInteractive() {
		return
	}

	if _, err := exec.LookPath("gh"); err != nil {
		return
	}

	value := os.Getenv(secretName)
	if value == "" {
		fmt.Printf("ℹ️  %s is not set in your environment; set the repository secret manually with:\n", secretName)
		fmt.Printf("     gh secret set %s\n", secretName)
		return
	}

	var confirm bool
	if err := huh.NewConfirm().
		Title(fmt.Sprintf("Set the %s repository secret via gh?", secretName)).
		Description("Uses the value from your current environment.").
		Value(&confirm).
		Run(); err != nil || !confirm {
		return
	}

	cmd := exec.Command("gh", "secret", "set", secretName, "--body", value)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("⚠️  Failed to set secret via gh: %v\n", err)
		fmt.Printf("     Set it manually with: gh secret set %s\n", secretName)
		return
	}
	fmt.Printf("✅ Set repository secret %s\n", secretName)
}

// printGitHubInstallNextSteps prints the manual follow-up actions a user must
// take after the workflow is scaffolded.
func printGitHubInstallNextSteps(secretName string) {
	fmt.Println("\nNext steps:")
	fmt.Printf("  1. Commit the workflow:  git add %s && git commit -m \"ci: add kit workflow\"\n", githubWorkflowPath)
	fmt.Printf("  2. Set the %s repository secret (Settings → Secrets → Actions), if not already set.\n", secretName)
	fmt.Println("  3. Comment '/kit <your request>' on an issue or pull request to trigger Kit.")
}
