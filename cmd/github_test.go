package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProviderSecretEnvVar(t *testing.T) {
	tests := []struct {
		provider string
		want     string
	}{
		{"anthropic", "ANTHROPIC_API_KEY"},
		{"openai", "OPENAI_API_KEY"},
		// Unknown provider falls back to "<PROVIDER>_API_KEY" with sanitization.
		{"my-custom.provider", "MY_CUSTOM_PROVIDER_API_KEY"},
	}

	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			got := providerSecretEnvVar(tt.provider)
			if got != tt.want {
				t.Errorf("providerSecretEnvVar(%q) = %q, want %q", tt.provider, got, tt.want)
			}
		})
	}
}

func TestRenderGitHubWorkflow(t *testing.T) {
	out := renderGitHubWorkflow("anthropic/claude-sonnet-4-5-20250929", "ANTHROPIC_API_KEY")

	wantSubstrings := []string{
		"name: kit",
		"issue_comment:",
		"pull_request_review_comment:",
		"startsWith(github.event.comment.body, '/kit')",
		"contains(github.event.comment.body, ' /kit')",
		"persist-credentials: false",
		"uses: mark3labs/kit-action@v1",
		"model: anthropic/claude-sonnet-4-5-20250929",
		"GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}",
		"ANTHROPIC_API_KEY: ${{ secrets.ANTHROPIC_API_KEY }}",
		"contents: write",
		"pull-requests: write",
		"issues: write",
	}
	for _, want := range wantSubstrings {
		if !strings.Contains(out, want) {
			t.Errorf("rendered workflow missing %q\n---\n%s", want, out)
		}
	}
}

func TestWriteGitHubWorkflow(t *testing.T) {
	dir := t.TempDir()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	// First write succeeds and creates nested directories.
	if err := writeGitHubWorkflow("anthropic/claude-sonnet-4-5", "ANTHROPIC_API_KEY", false); err != nil {
		t.Fatalf("writeGitHubWorkflow: %v", err)
	}
	data, err := os.ReadFile(githubWorkflowPath)
	if err != nil {
		t.Fatalf("reading workflow: %v", err)
	}
	if !strings.Contains(string(data), "model: anthropic/claude-sonnet-4-5") {
		t.Errorf("workflow missing model line:\n%s", data)
	}

	// Second write without force must refuse to clobber.
	if err := writeGitHubWorkflow("anthropic/claude-sonnet-4-5", "ANTHROPIC_API_KEY", false); err == nil {
		t.Error("expected error when overwriting without --force, got nil")
	}

	// With force it overwrites.
	if err := writeGitHubWorkflow("openai/gpt-5", "OPENAI_API_KEY", true); err != nil {
		t.Fatalf("writeGitHubWorkflow with force: %v", err)
	}
	data, err = os.ReadFile(githubWorkflowPath)
	if err != nil {
		t.Fatalf("reading workflow: %v", err)
	}
	if !strings.Contains(string(data), "OPENAI_API_KEY") {
		t.Errorf("forced overwrite did not update content:\n%s", data)
	}

	// Sanity: the file lives at the expected nested path.
	if _, err := os.Stat(filepath.Join(dir, githubWorkflowPath)); err != nil {
		t.Errorf("workflow not at expected path: %v", err)
	}
}
