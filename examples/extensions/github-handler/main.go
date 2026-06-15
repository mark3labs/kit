//go:build ignore

// Package main implements the Kit GitHub handler extension.
//
// This is the Phase 2b "GitHub handler" piece of Kit's GitHub integration
// (issue #60). It is designed to run *inside a GitHub Actions runner*, driven
// by the workflow scaffolded by `kit github install`. When a collaborator
// comments `/kit <request>` on an issue or pull request, the workflow boots Kit
// headlessly with this extension loaded; the extension then:
//
//   - parses the triggering GitHub event from GITHUB_EVENT_PATH,
//   - enforces that the comment author has write/admin access
//     (author_association in OWNER / MEMBER / COLLABORATOR),
//   - reacts with 👀 on the trigger comment while it works,
//   - gathers context (issue thread or PR diff) and drives the agent with it,
//   - posts the agent's response back as a comment, and
//   - if the agent left uncommitted changes, pushes a branch as the
//     `kit-agent[bot]` identity and opens a pull request.
//
// Outside of GitHub Actions (i.e. when GITHUB_ACTIONS != "true") the extension
// is inert, so it is safe to keep loaded during normal local use.
//
// Set KIT_GITHUB_DRY_RUN=1 (or the `github.dry-run` option) to exercise the
// parsing / permission / prompt-building logic without shelling out to `gh` or
// `git` — every side effect is logged instead of executed. This is what the
// unit tests use.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"kit/ext"
)

// commandToken is the mention that triggers Kit from a comment, mirroring the
// `if:` guard in the generated workflow (.github/workflows/kit.yml).
const commandToken = "/kit"

// botName / botEmail are the dedicated identity commits are attributed to, so
// Kit's changes are clearly distinguishable from human authors in history.
const (
	botName  = "kit-agent[bot]"
	botEmail = "kit-agent[bot]@users.noreply.github.com"
)

// writeAssociations are the GitHub author_association values that imply
// write/admin access. Only these may trigger the handler.
var writeAssociations = map[string]bool{
	"OWNER":        true,
	"MEMBER":       true,
	"COLLABORATOR": true,
}

// ghUser is a GitHub user as embedded in event payloads.
type ghUser struct {
	Login string `json:"login"`
}

// ghComment is the triggering comment.
type ghComment struct {
	ID                int64  `json:"id"`
	Body              string `json:"body"`
	AuthorAssociation string `json:"author_association"`
	User              ghUser `json:"user"`
}

// ghIssue is the issue (or PR, since GitHub models PRs as issues) the comment
// was posted on. PullRequest is non-nil when the issue is actually a PR.
type ghIssue struct {
	Number      int             `json:"number"`
	Title       string          `json:"title"`
	Body        string          `json:"body"`
	PullRequest json.RawMessage `json:"pull_request"`
}

// ghPull is the pull request for pull_request_review_comment events.
type ghPull struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	Body   string `json:"body"`
}

// ghRepo identifies the repository the event originated from.
type ghRepo struct {
	FullName      string `json:"full_name"`
	DefaultBranch string `json:"default_branch"`
}

// ghEvent is the subset of the GitHub Actions event payload the handler reads.
type ghEvent struct {
	Action      string     `json:"action"`
	Comment     *ghComment `json:"comment"`
	Issue       *ghIssue   `json:"issue"`
	PullRequest *ghPull    `json:"pull_request"`
	Repository  ghRepo     `json:"repository"`
}

// trigger captures everything the handler needs about a single invocation,
// normalised across issue_comment and pull_request_review_comment events.
type trigger struct {
	repo          string
	defaultBranch string
	number        int    // issue or PR number
	isPR          bool   // true when the target is a pull request
	commentID     int64  // triggering comment id (for reactions)
	commentKind   string // "issues" or "pulls" — reaction API path segment
	author        string
	association   string
	request       string // the user's instruction (comment body minus the token)
	title         string
	body          string
}

func Init(api ext.API) {
	api.RegisterOption(ext.OptionDef{
		Name:        "github.dry-run",
		Description: "Log GitHub/git side effects instead of executing them",
		Default:     "false",
	})

	api.OnSessionStart(func(_ ext.SessionStartEvent, ctx ext.Context) {
		if !inGitHubActions() {
			return
		}
		handleSessionStart(ctx)
	})

	api.OnAgentEnd(func(e ext.AgentEndEvent, ctx ext.Context) {
		if !inGitHubActions() || activeTrigger == nil {
			return
		}
		handleAgentEnd(e, ctx)
	})
}

// activeTrigger holds the parsed trigger between OnSessionStart and OnAgentEnd.
// Yaegi supports package-level state captured by the handler closures.
var activeTrigger *trigger

func inGitHubActions() bool {
	return os.Getenv("GITHUB_ACTIONS") == "true"
}

// dryRun reports whether side effects should be logged instead of executed.
func dryRun(ctx ext.Context) bool {
	if os.Getenv("KIT_GITHUB_DRY_RUN") != "" {
		return true
	}
	return strings.EqualFold(ctx.GetOption("github.dry-run"), "true")
}

// handleSessionStart parses the event, enforces permissions, reacts on the
// trigger comment, builds the prompt, and drives the agent.
func handleSessionStart(ctx ext.Context) {
	event, err := loadEvent()
	if err != nil {
		ctx.PrintError("kit-github: " + err.Error())
		ctx.Exit()
		return
	}

	tr, err := buildTrigger(event)
	if err != nil {
		// Not an actionable trigger (e.g. a comment without /kit). Stay quiet
		// and let the run finish; the workflow `if:` normally prevents this.
		ctx.PrintInfo("kit-github: " + err.Error())
		ctx.Exit()
		return
	}

	if !writeAssociations[strings.ToUpper(tr.association)] {
		ctx.PrintError(fmt.Sprintf(
			"kit-github: ignoring /kit from @%s — author_association %q lacks write access",
			tr.author, tr.association))
		ctx.Exit()
		return
	}

	activeTrigger = tr

	// React with 👀 so the human sees Kit picked up the request.
	addReaction(ctx, tr, "eyes")

	context := gatherContext(ctx, tr)
	prompt := buildPrompt(tr, context)
	ctx.SendMessage(prompt)
}

// loadEvent reads and decodes the GitHub Actions event payload.
func loadEvent() (*ghEvent, error) {
	path := os.Getenv("GITHUB_EVENT_PATH")
	if path == "" {
		return nil, fmt.Errorf("GITHUB_EVENT_PATH is not set")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading event payload: %w", err)
	}
	var event ghEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return nil, fmt.Errorf("parsing event payload: %w", err)
	}
	return &event, nil
}

// buildTrigger normalises an event into a trigger, or returns an error when the
// event is not an actionable `/kit` comment.
func buildTrigger(event *ghEvent) (*trigger, error) {
	if event.Comment == nil {
		return nil, fmt.Errorf("event has no comment; nothing to do")
	}

	request, ok := extractRequest(event.Comment.Body)
	if !ok {
		return nil, fmt.Errorf("comment does not contain the %q command", commandToken)
	}

	tr := &trigger{
		repo:          event.Repository.FullName,
		defaultBranch: event.Repository.DefaultBranch,
		commentID:     event.Comment.ID,
		author:        event.Comment.User.Login,
		association:   event.Comment.AuthorAssociation,
		request:       request,
	}
	if tr.defaultBranch == "" {
		tr.defaultBranch = "main"
	}

	switch {
	case event.Issue != nil:
		tr.number = event.Issue.Number
		tr.title = event.Issue.Title
		tr.body = event.Issue.Body
		tr.isPR = len(event.Issue.PullRequest) > 0
		// issue_comment fires for both issues and PRs; reactions for PR
		// comments posted on the conversation tab still use the issues path.
		tr.commentKind = "issues"
	case event.PullRequest != nil:
		tr.number = event.PullRequest.Number
		tr.title = event.PullRequest.Title
		tr.body = event.PullRequest.Body
		tr.isPR = true
		// pull_request_review_comment reactions use the pulls path.
		tr.commentKind = "pulls"
	default:
		return nil, fmt.Errorf("event has no issue or pull_request target")
	}

	if tr.repo == "" {
		return nil, fmt.Errorf("event is missing repository.full_name")
	}
	return tr, nil
}

// extractRequest pulls the instruction text out of a comment body that mentions
// the command token. It accepts the token at the start of the body or after a
// leading space (mirroring the workflow guard), and returns the remainder of
// that line as the request.
func extractRequest(body string) (string, bool) {
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		var rest string
		switch {
		case trimmed == commandToken:
			return "", true
		case strings.HasPrefix(trimmed, commandToken+" "):
			rest = trimmed[len(commandToken):]
		default:
			if idx := strings.Index(trimmed, " "+commandToken+" "); idx >= 0 {
				rest = trimmed[idx+len(commandToken)+1:]
			} else if strings.HasSuffix(trimmed, " "+commandToken) {
				return "", true
			} else {
				continue
			}
		}
		return strings.TrimSpace(rest), true
	}
	return "", false
}

// gatherContext assembles the issue thread or PR diff to give the agent. It
// always includes the title/body from the event payload, and — outside dry-run,
// when `gh` is available — enriches with the full comment thread and PR diff.
func gatherContext(ctx ext.Context, tr *trigger) string {
	var b strings.Builder
	target := "Issue"
	if tr.isPR {
		target = "Pull request"
	}
	fmt.Fprintf(&b, "%s #%d: %s\n", target, tr.number, tr.title)
	if strings.TrimSpace(tr.body) != "" {
		fmt.Fprintf(&b, "\n%s\n", strings.TrimSpace(tr.body))
	}

	if dryRun(ctx) || !commandExists("gh") {
		return b.String()
	}

	if tr.isPR {
		if diff := ghOutput(ctx, "pr", "diff", fmt.Sprint(tr.number), "--repo", tr.repo); diff != "" {
			fmt.Fprintf(&b, "\n## Diff\n```diff\n%s\n```\n", strings.TrimSpace(diff))
		}
		if comments := ghOutput(ctx, "pr", "view", fmt.Sprint(tr.number), "--repo", tr.repo, "--json", "comments", "--jq", ".comments[] | \"@\\(.author.login): \\(.body)\""); comments != "" {
			fmt.Fprintf(&b, "\n## Comments\n%s\n", strings.TrimSpace(comments))
		}
	} else {
		if comments := ghOutput(ctx, "issue", "view", fmt.Sprint(tr.number), "--repo", tr.repo, "--json", "comments", "--jq", ".comments[] | \"@\\(.author.login): \\(.body)\""); comments != "" {
			fmt.Fprintf(&b, "\n## Comments\n%s\n", strings.TrimSpace(comments))
		}
	}
	return b.String()
}

// buildPrompt constructs the instruction sent to the agent.
func buildPrompt(tr *trigger, context string) string {
	target := "issue"
	if tr.isPR {
		target = "pull request"
	}
	request := tr.request
	if request == "" {
		request = "(no explicit instruction — review the " + target + " and respond helpfully)"
	}

	var b strings.Builder
	fmt.Fprintf(&b, "You are Kit, operating as an automated collaborator on the GitHub repository %s.\n\n", tr.repo)
	fmt.Fprintf(&b, "@%s (access: %s) triggered you on %s #%d with this request:\n\n", tr.author, tr.association, target, tr.number)
	fmt.Fprintf(&b, "%s\n\n", request)
	fmt.Fprintf(&b, "## Context\n%s\n\n", strings.TrimSpace(context))
	b.WriteString("Carry out the request. If you modify files, they will be committed to a new ")
	b.WriteString("branch and a pull request will be opened automatically, so you do not need to ")
	b.WriteString("commit or push yourself. Finish with a concise summary of what you did.")
	return b.String()
}

// handleAgentEnd posts the agent's response, opens a PR for any uncommitted
// changes, and swaps the reaction to signal completion.
func handleAgentEnd(e ext.AgentEndEvent, ctx ext.Context) {
	tr := activeTrigger
	response := strings.TrimSpace(e.Response)
	if response == "" {
		response = "Kit finished without a textual response."
	}

	if e.StopReason == "error" {
		comment := "⚠️ Kit hit an error while processing this request:\n\n" + response
		postComment(ctx, tr, comment)
		addReaction(ctx, tr, "confused")
		ctx.Exit()
		return
	}

	prURL := ""
	if hasUncommittedChanges(ctx) {
		prURL = openPullRequest(ctx, tr, response)
	}

	comment := response
	if prURL != "" {
		comment += "\n\n---\nOpened a pull request with the changes: " + prURL
	}
	postComment(ctx, tr, comment)
	addReaction(ctx, tr, "rocket")
	ctx.Exit()
}

// hasUncommittedChanges reports whether the working tree has changes the agent
// produced. In dry-run it reports the value of KIT_GITHUB_FAKE_DIRTY so tests
// stay deterministic.
func hasUncommittedChanges(ctx ext.Context) bool {
	if dryRun(ctx) {
		return os.Getenv("KIT_GITHUB_FAKE_DIRTY") != ""
	}
	out := gitOutput(ctx, "status", "--porcelain")
	return strings.TrimSpace(out) != ""
}

// openPullRequest commits the working tree as kit-agent[bot], pushes a branch,
// and opens a PR. It returns the PR URL, or "" on failure / dry-run.
func openPullRequest(ctx ext.Context, tr *trigger, summary string) string {
	branch := fmt.Sprintf("kit/issue-%d-%d", tr.number, time.Now().Unix())

	runGit(ctx, "checkout", "-b", branch)
	runGit(ctx, "add", "-A")
	runGit(ctx, "-c", "user.name="+botName, "-c", "user.email="+botEmail,
		"commit", "-m", fmt.Sprintf("kit: address #%d", tr.number))
	runGit(ctx, "push", "origin", "HEAD:"+branch)

	title := fmt.Sprintf("kit: changes for #%d", tr.number)
	body := fmt.Sprintf("Automated changes from Kit in response to #%d.\n\n%s", tr.number, summary)
	if dryRun(ctx) {
		ctx.Print(fmt.Sprintf("[dry-run] gh pr create --head %s --base %s", branch, tr.defaultBranch))
		return ""
	}
	url := ghOutput(ctx, "pr", "create", "--repo", tr.repo,
		"--head", branch, "--base", tr.defaultBranch,
		"--title", title, "--body", body)
	return strings.TrimSpace(url)
}

// addReaction adds an emoji reaction to the trigger comment.
func addReaction(ctx ext.Context, tr *trigger, content string) {
	path := fmt.Sprintf("/repos/%s/%s/comments/%d/reactions", tr.repo, tr.commentKind, tr.commentID)
	if dryRun(ctx) || !commandExists("gh") {
		ctx.Print(fmt.Sprintf("[dry-run] react %q on %s", content, path))
		return
	}
	runCmd(ctx, "gh", "api", "-X", "POST", path, "-f", "content="+content)
}

// postComment posts a comment back on the triggering issue or pull request.
func postComment(ctx ext.Context, tr *trigger, body string) {
	sub := "issue"
	if tr.isPR {
		sub = "pr"
	}
	if dryRun(ctx) || !commandExists("gh") {
		ctx.Print(fmt.Sprintf("[dry-run] gh %s comment %d --body <%d chars>", sub, tr.number, len(body)))
		return
	}
	runCmd(ctx, "gh", sub, "comment", fmt.Sprint(tr.number), "--repo", tr.repo, "--body", body)
}

// --- thin subprocess helpers -------------------------------------------------

func commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// runGit runs a mutating git command, logging instead of executing in dry-run.
func runGit(ctx ext.Context, args ...string) {
	if dryRun(ctx) {
		ctx.Print("[dry-run] git " + strings.Join(args, " "))
		return
	}
	runCmd(ctx, "git", args...)
}

// gitOutput runs a read-only git command and returns its stdout.
func gitOutput(ctx ext.Context, args ...string) string {
	cmd := exec.Command("git", args...)
	out, err := cmd.Output()
	if err != nil {
		ctx.PrintError(fmt.Sprintf("kit-github: git %s failed: %v", strings.Join(args, " "), err))
		return ""
	}
	return string(out)
}

// ghOutput runs a gh command and returns its stdout.
func ghOutput(ctx ext.Context, args ...string) string {
	cmd := exec.Command("gh", args...)
	out, err := cmd.Output()
	if err != nil {
		ctx.PrintError(fmt.Sprintf("kit-github: gh %s failed: %v", strings.Join(args, " "), err))
		return ""
	}
	return string(out)
}

// runCmd runs a command for its side effects, surfacing failures via PrintError.
func runCmd(ctx ext.Context, name string, args ...string) {
	cmd := exec.Command(name, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		ctx.PrintError(fmt.Sprintf("kit-github: %s failed: %v\n%s", name, err, strings.TrimSpace(string(out))))
	}
}
