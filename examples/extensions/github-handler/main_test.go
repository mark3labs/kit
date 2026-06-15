package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mark3labs/kit/internal/extensions"
	"github.com/mark3labs/kit/pkg/extensions/test"
)

// writeEvent writes a GitHub event payload to a temp file and points
// GITHUB_EVENT_PATH at it. It also forces the extension into dry-run and
// pretends we are running inside GitHub Actions.
func writeEvent(t *testing.T, payload string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "event.json")
	if err := os.WriteFile(path, []byte(payload), 0o644); err != nil {
		t.Fatalf("write event: %v", err)
	}
	t.Setenv("GITHUB_ACTIONS", "true")
	t.Setenv("KIT_GITHUB_DRY_RUN", "1")
	t.Setenv("GITHUB_EVENT_PATH", path)
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

func TestGitHubHandler_RegistersHandlers(t *testing.T) {
	harness := test.New(t)
	ext := harness.LoadFile("main.go")
	if ext == nil {
		t.Fatal("extension should not be nil")
	}
	test.AssertHasHandlers(t, harness, extensions.SessionStart)
	test.AssertHasHandlers(t, harness, extensions.AgentEnd)
}

func TestGitHubHandler_InertOutsideActions(t *testing.T) {
	// No GITHUB_ACTIONS env → the handler must do nothing.
	t.Setenv("GITHUB_ACTIONS", "")
	harness := test.New(t)
	harness.LoadFile("main.go")

	if _, err := harness.Emit(extensions.SessionStartEvent{SessionID: "s1"}); err != nil {
		t.Fatalf("emit: %v", err)
	}
	if msgs := harness.Context().Messages; len(msgs) != 0 {
		t.Errorf("expected no messages outside Actions, got %v", msgs)
	}
}

func TestGitHubHandler_AuthorizedIssueComment(t *testing.T) {
	writeEvent(t, issueCommentEvent)

	harness := test.New(t)
	harness.LoadFile("main.go")
	if _, err := harness.Emit(extensions.SessionStartEvent{SessionID: "s1"}); err != nil {
		t.Fatalf("emit: %v", err)
	}

	msgs := harness.Context().Messages
	if len(msgs) != 1 {
		t.Fatalf("expected exactly one driven prompt, got %d: %v", len(msgs), msgs)
	}
	prompt := msgs[0]
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

func TestGitHubHandler_UnauthorizedAssociation(t *testing.T) {
	writeEvent(t, strings.Replace(issueCommentEvent, `"OWNER"`, `"NONE"`, 1))

	harness := test.New(t)
	harness.LoadFile("main.go")
	if _, err := harness.Emit(extensions.SessionStartEvent{SessionID: "s1"}); err != nil {
		t.Fatalf("emit: %v", err)
	}

	if msgs := harness.Context().Messages; len(msgs) != 0 {
		t.Fatalf("unauthorized author must not drive the agent, got %v", msgs)
	}
	if errs := harness.Context().GetPrintErrors(); len(errs) == 0 ||
		!strings.Contains(strings.Join(errs, "\n"), "lacks write access") {
		t.Errorf("expected a write-access error, got %v", errs)
	}
}

func TestGitHubHandler_CommentWithoutToken(t *testing.T) {
	writeEvent(t, strings.Replace(issueCommentEvent,
		`"/kit fix the broken parser"`, `"just a normal comment"`, 1))

	harness := test.New(t)
	harness.LoadFile("main.go")
	if _, err := harness.Emit(extensions.SessionStartEvent{SessionID: "s1"}); err != nil {
		t.Fatalf("emit: %v", err)
	}
	if msgs := harness.Context().Messages; len(msgs) != 0 {
		t.Fatalf("non-/kit comment must not drive the agent, got %v", msgs)
	}
}

func TestGitHubHandler_MidSentenceMentionIgnored(t *testing.T) {
	// An incidental mid-sentence mention of the token must not trigger Kit.
	writeEvent(t, strings.Replace(issueCommentEvent,
		`"/kit fix the broken parser"`, `"please review /kit behavior in the docs"`, 1))

	harness := test.New(t)
	harness.LoadFile("main.go")
	if _, err := harness.Emit(extensions.SessionStartEvent{SessionID: "s1"}); err != nil {
		t.Fatalf("emit: %v", err)
	}
	if msgs := harness.Context().Messages; len(msgs) != 0 {
		t.Fatalf("mid-sentence /kit mention must not drive the agent, got %v", msgs)
	}
}

func TestGitHubHandler_PullRequestReviewComment(t *testing.T) {
	writeEvent(t, `{
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

	harness := test.New(t)
	harness.LoadFile("main.go")
	if _, err := harness.Emit(extensions.SessionStartEvent{SessionID: "s1"}); err != nil {
		t.Fatalf("emit: %v", err)
	}
	msgs := harness.Context().Messages
	if len(msgs) != 1 {
		t.Fatalf("expected one driven prompt, got %v", msgs)
	}
	if !strings.Contains(msgs[0], "pull request #7") {
		t.Errorf("expected PR target in prompt:\n%s", msgs[0])
	}
}

func TestGitHubHandler_AgentEndPostsComment(t *testing.T) {
	writeEvent(t, issueCommentEvent)

	harness := test.New(t)
	harness.LoadFile("main.go")
	if _, err := harness.Emit(extensions.SessionStartEvent{SessionID: "s1"}); err != nil {
		t.Fatalf("emit session start: %v", err)
	}
	if _, err := harness.Emit(extensions.AgentEndEvent{
		Response:   "Fixed the parser by guarding empty input.",
		StopReason: "completed",
	}); err != nil {
		t.Fatalf("emit agent end: %v", err)
	}

	prints := strings.Join(harness.Context().GetPrints(), "\n")
	if !strings.Contains(prints, "gh issue comment 42") {
		t.Errorf("expected a dry-run comment post, got prints:\n%s", prints)
	}
}

func TestGitHubHandler_AgentEndOpensPRWhenDirty(t *testing.T) {
	writeEvent(t, issueCommentEvent)
	t.Setenv("KIT_GITHUB_FAKE_DIRTY", "1")

	harness := test.New(t)
	harness.LoadFile("main.go")
	if _, err := harness.Emit(extensions.SessionStartEvent{SessionID: "s1"}); err != nil {
		t.Fatalf("emit session start: %v", err)
	}
	if _, err := harness.Emit(extensions.AgentEndEvent{
		Response:   "Made changes.",
		StopReason: "completed",
	}); err != nil {
		t.Fatalf("emit agent end: %v", err)
	}

	prints := strings.Join(harness.Context().GetPrints(), "\n")
	if !strings.Contains(prints, "gh pr create") {
		t.Errorf("expected a dry-run PR creation, got prints:\n%s", prints)
	}
	if !strings.Contains(prints, "git checkout -b kit/issue-42-") {
		t.Errorf("expected a dry-run branch checkout, got prints:\n%s", prints)
	}
}
