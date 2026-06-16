package cmd

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setupEvent writes a GitHub event payload to a temp file, points
// GITHUB_EVENT_PATH at it, and forces dry-run + Actions mode. It also resets
// the run command's package-level flag state so tests are independent.
func setupEvent(t *testing.T, payload string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "event.json")
	if err := os.WriteFile(path, []byte(payload), 0o644); err != nil {
		t.Fatalf("write event: %v", err)
	}
	t.Setenv("GITHUB_ACTIONS", "true")
	t.Setenv("KIT_GITHUB_DRY_RUN", "1")
	t.Setenv("GITHUB_EVENT_PATH", path)
	t.Cleanup(func() {
		githubRunModel = ""
		githubRunDryRun = false
	})
}

const issueCommentEvent = `{
  "action": "created",
  "comment": {
    "id": 555,
    "body": "/kit fix the broken parser",
    "author_association": "OWNER",
    "user": {"login": "alice"}
  },
  "issue": {"number": 42, "title": "Parser crashes on empty input", "body": "It panics."},
  "repository": {"full_name": "acme/widgets", "default_branch": "main"}
}`

func TestExtractRequest(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		want    string
		wantHit bool
	}{
		{"start with request", "/kit fix the bug", "fix the bug", true},
		{"bare token", "/kit", "", true},
		{"trailing token", "hey /kit", "", true},
		{"mid-sentence ignored", "please review /kit behavior in the docs", "", false},
		{"no token", "just a normal comment", "", false},
		{"token in second line", "thanks!\n/kit add tests", "add tests", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, hit := extractRequest(tt.body)
			if hit != tt.wantHit || got != tt.want {
				t.Errorf("extractRequest(%q) = (%q, %v), want (%q, %v)", tt.body, got, hit, tt.want, tt.wantHit)
			}
		})
	}
}

func TestBuildTrigger_IssueComment(t *testing.T) {
	event, err := func() (*ghEvent, error) {
		setupEvent(t, issueCommentEvent)
		return loadGitHubEvent()
	}()
	if err != nil {
		t.Fatalf("loadGitHubEvent: %v", err)
	}
	tr, err := buildTrigger(event)
	if err != nil {
		t.Fatalf("buildTrigger: %v", err)
	}
	if tr.repo != "acme/widgets" || tr.number != 42 || tr.isPR || tr.request != "fix the broken parser" {
		t.Errorf("unexpected trigger: %+v", tr)
	}
	if tr.commentKind != "issues" {
		t.Errorf("commentKind = %q, want issues", tr.commentKind)
	}
}

func TestBuildPrompt_ContainsContext(t *testing.T) {
	setupEvent(t, issueCommentEvent)
	event, _ := loadGitHubEvent()
	tr, _ := buildTrigger(event)

	prompt := buildPrompt(tr, gatherContext(context.Background(), tr))
	for _, want := range []string{
		"fix the broken parser",         // the request
		"acme/widgets",                  // the repo
		"issue #42",                     // the target
		"@alice",                        // the author
		"Parser crashes on empty input", // context: title
		"It panics.",                    // context: body
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt missing %q\n---\n%s", want, prompt)
		}
	}
}

func TestRunGitHub_AuthorizedIssueComment(t *testing.T) {
	setupEvent(t, issueCommentEvent)
	if err := runGitHubRun(githubRunCmd, nil); err != nil {
		t.Fatalf("runGitHubRun: %v", err)
	}
}

func TestRunGitHub_UnauthorizedAssociation(t *testing.T) {
	setupEvent(t, strings.Replace(issueCommentEvent, `"OWNER"`, `"NONE"`, 1))
	// Should return nil (no-op) without attempting the agent run.
	if err := runGitHubRun(githubRunCmd, nil); err != nil {
		t.Fatalf("runGitHubRun should be a no-op for unauthorized authors, got: %v", err)
	}
}

func TestRunGitHub_CommentWithoutToken(t *testing.T) {
	setupEvent(t, strings.Replace(issueCommentEvent,
		`"/kit fix the broken parser"`, `"just a normal comment"`, 1))
	if err := runGitHubRun(githubRunCmd, nil); err != nil {
		t.Fatalf("runGitHubRun should be a no-op without /kit, got: %v", err)
	}
}

func TestRunGitHub_MidSentenceMentionIgnored(t *testing.T) {
	setupEvent(t, strings.Replace(issueCommentEvent,
		`"/kit fix the broken parser"`, `"please review /kit behavior in the docs"`, 1))
	if err := runGitHubRun(githubRunCmd, nil); err != nil {
		t.Fatalf("runGitHubRun should ignore mid-sentence mentions, got: %v", err)
	}
}

func TestRunGitHub_PullRequestReviewComment(t *testing.T) {
	setupEvent(t, `{
  "action": "created",
  "comment": {
    "id": 999,
    "body": "/kit review this change",
    "author_association": "COLLABORATOR",
    "user": {"login": "bob"}
  },
  "pull_request": {"number": 7, "title": "Add caching", "body": "Speeds things up."},
  "repository": {"full_name": "acme/widgets", "default_branch": "main"}
}`)
	event, _ := loadGitHubEvent()
	tr, err := buildTrigger(event)
	if err != nil {
		t.Fatalf("buildTrigger: %v", err)
	}
	if !tr.isPR || tr.number != 7 || tr.commentKind != "pulls" {
		t.Errorf("unexpected PR trigger: %+v", tr)
	}
	if err := runGitHubRun(githubRunCmd, nil); err != nil {
		t.Fatalf("runGitHubRun (PR): %v", err)
	}
}

func TestRunGitHub_RequiresActionsOrDryRun(t *testing.T) {
	// Neither GITHUB_ACTIONS nor dry-run set → must error rather than act.
	t.Setenv("GITHUB_ACTIONS", "")
	t.Setenv("KIT_GITHUB_DRY_RUN", "")
	githubRunDryRun = false
	t.Cleanup(func() { githubRunDryRun = false })
	if err := runGitHubRun(githubRunCmd, nil); err == nil {
		t.Fatal("expected an error when run outside Actions without --dry-run")
	}
}

func TestResolveRunModel(t *testing.T) {
	t.Cleanup(func() { githubRunModel = "" })

	t.Setenv("MODEL", "")
	githubRunModel = ""
	if got := resolveRunModel(); got != defaultGitHubModel {
		t.Errorf("default model = %q, want %q", got, defaultGitHubModel)
	}

	t.Setenv("MODEL", "openai/gpt-5")
	if got := resolveRunModel(); got != "openai/gpt-5" {
		t.Errorf("MODEL env model = %q, want openai/gpt-5", got)
	}

	githubRunModel = "anthropic/claude-sonnet-4-5"
	if got := resolveRunModel(); got != "anthropic/claude-sonnet-4-5" {
		t.Errorf("flag model = %q, want anthropic/claude-sonnet-4-5", got)
	}
}
