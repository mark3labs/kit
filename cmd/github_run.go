package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/spf13/cobra"
)

// commandToken is the mention that triggers Kit from a comment, mirroring the
// `if:` guard in the generated workflow (.github/workflows/kit.yml).
const commandToken = "/kit"

// subprocessTimeout bounds each git/gh invocation so a stalled network call or
// an unexpected auth prompt cannot hang the Actions job indefinitely.
const subprocessTimeout = 30 * time.Second

// agentTimeout bounds the headless agent run so a runaway turn cannot block the
// job forever. GitHub Actions jobs have their own ceiling, but a tighter bound
// keeps feedback fast and costs predictable.
const agentTimeout = 20 * time.Minute

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

var (
	githubRunModel  string
	githubRunDryRun bool
)

// githubRunCmd is the runtime half of the GitHub integration. It is invoked by
// the bundled composite action (action.yml) inside a GitHub Actions runner once
// a collaborator comments '/kit <request>' on an issue or pull request. It reads
// the triggering event, enforces permissions, runs the agent headlessly against
// the comment/PR context, and responds by posting a comment and — when the agent
// leaves changes — opening a pull request.
var githubRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Run Kit against the current GitHub Actions event (used by the kit action)",
	Long: `Run Kit against the current GitHub Actions event.

This command is normally invoked by the bundled composite action inside a
GitHub Actions runner; you rarely run it by hand. It reads the triggering
event from GITHUB_EVENT_PATH, verifies the commenter has write/admin access,
reacts with an emoji while it works, runs the agent non-interactively against
the issue thread or pull request, posts the response as a comment, and — if the
agent modified files — pushes a kit-agent[bot] branch and opens a pull request.

Set --dry-run (or KIT_GITHUB_DRY_RUN=1) to log every git/gh side effect and
skip the agent run instead of executing them.`,
	Args: cobra.NoArgs,
	RunE: runGitHubRun,
}

func init() {
	githubRunCmd.Flags().StringVarP(&githubRunModel, "model", "m", "", "provider/model the agent should use (falls back to $MODEL, then a default)")
	githubRunCmd.Flags().BoolVar(&githubRunDryRun, "dry-run", false, "log git/gh side effects and skip the agent run instead of executing them")
	githubCmd.AddCommand(githubRunCmd)
}

// --- GitHub event types ------------------------------------------------------

type ghUser struct {
	Login string `json:"login"`
}

type ghComment struct {
	ID                int64  `json:"id"`
	Body              string `json:"body"`
	AuthorAssociation string `json:"author_association"`
	User              ghUser `json:"user"`
}

type ghIssue struct {
	Number      int             `json:"number"`
	Title       string          `json:"title"`
	Body        string          `json:"body"`
	PullRequest json.RawMessage `json:"pull_request"`
}

type ghPull struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	Body   string `json:"body"`
}

type ghRepo struct {
	FullName      string `json:"full_name"`
	DefaultBranch string `json:"default_branch"`
}

type ghEvent struct {
	Action      string     `json:"action"`
	Comment     *ghComment `json:"comment"`
	Issue       *ghIssue   `json:"issue"`
	PullRequest *ghPull    `json:"pull_request"`
	Repository  ghRepo     `json:"repository"`
}

// trigger normalises a single invocation across issue_comment and
// pull_request_review_comment events.
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

// runGitHubRun is the entry point wired to `kit github run`.
func runGitHubRun(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()

	if !inGitHubActions() && !githubDryRun() {
		return fmt.Errorf("kit github run is meant to run inside GitHub Actions (set GITHUB_ACTIONS=true or pass --dry-run)")
	}

	event, err := loadGitHubEvent()
	if err != nil {
		return err
	}

	tr, err := buildTrigger(event)
	if err != nil {
		// Not an actionable trigger (the workflow `if:` normally prevents this).
		log.Info("github run: nothing to do", "reason", err)
		return nil
	}

	if !writeAssociations[strings.ToUpper(tr.association)] {
		log.Warn("github run: ignoring /kit from unauthorized author",
			"author", tr.author, "association", tr.association)
		return nil
	}

	model := resolveRunModel()
	log.Info("github run: handling trigger",
		"repo", tr.repo, "number", tr.number, "pr", tr.isPR, "author", tr.author, "model", model)

	// React with 👀 so the human sees Kit picked up the request.
	addReaction(ctx, tr, "eyes")

	gathered := gatherContext(ctx, tr)
	prompt := buildPrompt(tr, gathered)

	response, runErr := runAgent(ctx, model, prompt)
	if runErr != nil {
		postComment(ctx, tr, "⚠️ Kit hit an error while processing this request:\n\n```\n"+runErr.Error()+"\n```")
		addReaction(ctx, tr, "confused")
		return runErr
	}

	response = strings.TrimSpace(response)
	if response == "" {
		response = "Kit finished without a textual response."
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
	return nil
}

// resolveRunModel picks the model: --model flag, then $MODEL, then the default.
func resolveRunModel() string {
	if m := strings.TrimSpace(githubRunModel); m != "" {
		return m
	}
	if m := strings.TrimSpace(os.Getenv("MODEL")); m != "" {
		return m
	}
	return defaultGitHubModel
}

func inGitHubActions() bool {
	return os.Getenv("GITHUB_ACTIONS") == "true"
}

// githubDryRun reports whether side effects should be logged instead of run.
func githubDryRun() bool {
	return githubRunDryRun || os.Getenv("KIT_GITHUB_DRY_RUN") != ""
}

// loadGitHubEvent reads and decodes the GitHub Actions event payload.
func loadGitHubEvent() (*ghEvent, error) {
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
		tr.commentKind = "issues"
	case event.PullRequest != nil:
		tr.number = event.PullRequest.Number
		tr.title = event.PullRequest.Title
		tr.body = event.PullRequest.Body
		tr.isPR = true
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
// the command token. It only recognizes the token at the start of a line
// (mirroring the workflow guard) or at the very end, so incidental mid-sentence
// mentions like "please review /kit behavior" do not trigger the handler. It
// returns the remainder of the matching line as the request.
func extractRequest(body string) (string, bool) {
	for line := range strings.SplitSeq(body, "\n") {
		trimmed := strings.TrimSpace(line)
		var rest string
		switch {
		case trimmed == commandToken:
			return "", true
		case strings.HasPrefix(trimmed, commandToken+" "):
			rest = trimmed[len(commandToken):]
		case strings.HasSuffix(trimmed, " "+commandToken):
			return "", true
		default:
			continue
		}
		return strings.TrimSpace(rest), true
	}
	return "", false
}

// gatherContext assembles the issue thread or PR diff to give the agent. It
// always includes the title/body from the event payload, and — outside dry-run,
// when `gh` is available — enriches with the comment thread and PR diff.
func gatherContext(ctx context.Context, tr *trigger) string {
	var b strings.Builder
	target := "Issue"
	if tr.isPR {
		target = "Pull request"
	}
	fmt.Fprintf(&b, "%s #%d: %s\n", target, tr.number, tr.title)
	if strings.TrimSpace(tr.body) != "" {
		fmt.Fprintf(&b, "\n%s\n", strings.TrimSpace(tr.body))
	}

	if githubDryRun() || !commandExists("gh") {
		return b.String()
	}

	num := fmt.Sprint(tr.number)
	if tr.isPR {
		if diff := ghOutput(ctx, "pr", "diff", num, "--repo", tr.repo); diff != "" {
			fmt.Fprintf(&b, "\n## Diff\n```diff\n%s\n```\n", strings.TrimSpace(diff))
		}
		if comments := ghOutput(ctx, "pr", "view", num, "--repo", tr.repo, "--json", "comments", "--jq", ".comments[] | \"@\\(.author.login): \\(.body)\""); comments != "" {
			fmt.Fprintf(&b, "\n## Comments\n%s\n", strings.TrimSpace(comments))
		}
	} else {
		if comments := ghOutput(ctx, "issue", "view", num, "--repo", tr.repo, "--json", "comments", "--jq", ".comments[] | \"@\\(.author.login): \\(.body)\""); comments != "" {
			fmt.Fprintf(&b, "\n## Comments\n%s\n", strings.TrimSpace(comments))
		}
	}
	return b.String()
}

// buildPrompt constructs the instruction sent to the agent.
func buildPrompt(tr *trigger, gathered string) string {
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
	fmt.Fprintf(&b, "## Context\n%s\n\n", strings.TrimSpace(gathered))
	b.WriteString("Carry out the request. If you modify files, they will be committed to a new ")
	b.WriteString("branch and a pull request will be opened automatically, so you do not need to ")
	b.WriteString("commit or push yourself. Finish with a concise summary of what you did.")
	return b.String()
}

// runAgent drives the agent headlessly by invoking this same binary in quiet,
// ephemeral mode against the constructed prompt, and returns its response. In
// dry-run it returns a canned response without spawning anything.
func runAgent(ctx context.Context, model, prompt string) (string, error) {
	if githubDryRun() {
		log.Info("github run: [dry-run] would run agent", "model", model, "promptChars", len(prompt))
		return "[dry-run] agent response", nil
	}

	exe, err := os.Executable()
	if err != nil || exe == "" {
		exe = "kit"
	}

	runCtx, cancel := context.WithTimeout(ctx, agentTimeout)
	defer cancel()

	args := []string{"--quiet", "--no-session", "--no-extensions"}
	if model != "" {
		args = append(args, "--model", model)
	}
	args = append(args, prompt)

	cmd := exec.CommandContext(runCtx, exe, args...)
	cmd.Stderr = os.Stderr // surface agent progress/errors in the Actions log
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("agent run failed: %w", err)
	}
	return string(out), nil
}

// hasUncommittedChanges reports whether the agent produced working-tree changes.
func hasUncommittedChanges(ctx context.Context) bool {
	if githubDryRun() {
		return os.Getenv("KIT_GITHUB_FAKE_DIRTY") != ""
	}
	return strings.TrimSpace(gitOutput(ctx, "status", "--porcelain")) != ""
}

// openPullRequest commits the working tree as kit-agent[bot], pushes a branch,
// and opens a PR. It returns the PR URL, or "" on failure / dry-run.
func openPullRequest(ctx context.Context, tr *trigger, summary string) string {
	branch := fmt.Sprintf("kit/issue-%d-%d", tr.number, time.Now().Unix())

	runGit(ctx, "checkout", "-b", branch)
	runGit(ctx, "add", "-A")
	runGit(ctx, "-c", "user.name="+botName, "-c", "user.email="+botEmail,
		"commit", "-m", fmt.Sprintf("kit: address #%d", tr.number))

	// `persist-credentials: false` in the workflow means the checkout left no
	// push credentials behind. Re-establish them from GITHUB_TOKEN via gh's git
	// credential helper, then push over the existing origin remote.
	if !githubDryRun() {
		runCmd(ctx, "gh", "auth", "setup-git")
	}
	runGit(ctx, "push", "origin", "HEAD:"+branch)

	title := fmt.Sprintf("kit: changes for #%d", tr.number)
	body := fmt.Sprintf("Automated changes from Kit in response to #%d.\n\n%s", tr.number, summary)
	if githubDryRun() {
		log.Info("github run: [dry-run] would open PR", "branch", branch, "base", tr.defaultBranch)
		return ""
	}
	return strings.TrimSpace(ghOutput(ctx, "pr", "create", "--repo", tr.repo,
		"--head", branch, "--base", tr.defaultBranch, "--title", title, "--body", body))
}

// addReaction adds an emoji reaction to the trigger comment.
func addReaction(ctx context.Context, tr *trigger, content string) {
	path := fmt.Sprintf("/repos/%s/%s/comments/%d/reactions", tr.repo, tr.commentKind, tr.commentID)
	if githubDryRun() || !commandExists("gh") {
		log.Info("github run: [dry-run] react", "content", content, "path", path)
		return
	}
	runCmd(ctx, "gh", "api", "-X", "POST", path, "-f", "content="+content)
}

// postComment posts a comment back on the triggering issue or pull request.
func postComment(ctx context.Context, tr *trigger, body string) {
	sub := "issue"
	if tr.isPR {
		sub = "pr"
	}
	if githubDryRun() || !commandExists("gh") {
		log.Info("github run: [dry-run] comment", "sub", sub, "number", tr.number, "chars", len(body))
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
func runGit(ctx context.Context, args ...string) {
	if githubDryRun() {
		log.Info("github run: [dry-run] git", "args", strings.Join(args, " "))
		return
	}
	runCmd(ctx, "git", args...)
}

// gitOutput runs a read-only git command and returns its stdout.
func gitOutput(ctx context.Context, args ...string) string {
	cmdCtx, cancel := context.WithTimeout(ctx, subprocessTimeout)
	defer cancel()
	out, err := exec.CommandContext(cmdCtx, "git", args...).Output()
	if err != nil {
		log.Error("github run: git failed", "args", strings.Join(args, " "), "err", err)
		return ""
	}
	return string(out)
}

// ghOutput runs a gh command and returns its stdout.
func ghOutput(ctx context.Context, args ...string) string {
	cmdCtx, cancel := context.WithTimeout(ctx, subprocessTimeout)
	defer cancel()
	out, err := exec.CommandContext(cmdCtx, "gh", args...).Output()
	if err != nil {
		log.Error("github run: gh failed", "args", strings.Join(args, " "), "err", err)
		return ""
	}
	return string(out)
}

// runCmd runs a command for its side effects, surfacing failures in the log.
func runCmd(ctx context.Context, name string, args ...string) {
	cmdCtx, cancel := context.WithTimeout(ctx, subprocessTimeout)
	defer cancel()
	if out, err := exec.CommandContext(cmdCtx, name, args...).CombinedOutput(); err != nil {
		log.Error("github run: command failed", "cmd", name, "err", err, "output", strings.TrimSpace(string(out)))
	}
}
