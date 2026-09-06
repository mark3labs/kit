---
description: Fix bot review findings on the current PR, push, and poll until the review loop is clean
---

Resolve all automated review-bot findings (CodeRabbit and similar) on a pull request: verify each finding against the current code, fix the valid ones, push, wait for the bot's re-review, and repeat until no actionable comments remain. Extra context from the user: $@

## Identify the PR

- If the user input names a PR number, use it; otherwise resolve the PR for the current branch:

      gh pr view --json number,headRefName,state -q '{number: .number, branch: .headRefName, state: .state}'

- If the PR is merged or closed, stop and tell the user
- If the working tree is dirty with unrelated changes, stop and suggest `/commit-push` first
- Capture the repo slug once: `gh repo view --json nameWithOwner -q .nameWithOwner`

## Fetch the findings

Pull **both** comment surfaces — bots use them differently:

1. **Review-level bodies** (summary + "Actionable comments posted: N"):

       gh pr view <pr> --json reviews --jq '.reviews[] | select(.author.login | test("coderabbit|copilot|bot")) | {state, body}'

2. **Line comments** (the individual findings):

       gh api repos/<owner>/<repo>/pulls/<pr>/comments --jq '.[] | select(.user.login | endswith("[bot]")) | "=== \(.path):\(.line) ===\n\(.body)"'

CodeRabbit line comments embed a `🤖 Prompt for AI Agents` block — a ready-made instruction for each finding. Read the **full body**, not just the summary: severity markers (🟠 Major / 🟡 Minor), committable suggestions, and "Also applies to" line lists all matter.

## Triage each finding — verify before fixing

For every finding, check it against the **current** code (the comment may be outdated):

- **Still valid** → fix it, keeping the change minimal and scoped to the finding
- **Already addressed** (by a later commit or a previous loop iteration) → skip, note the commit that fixed it
- **Intentional behavior** the bot misread → skip, and reply on the thread explaining why:

      gh api repos/<owner>/<repo>/pulls/<pr>/comments/<comment-id>/replies -f body="..."

- **Wrong or out of scope** → skip with a brief reason in your report; don't silently ignore

Never blind-apply a bot's committable suggestion — read the surrounding code first. Bots regularly miss context like deliberate design decisions (check nearby comments and linked issues before "fixing" them away).

## Fix, validate, push

1. Apply the fixes; add or extend tests when a finding exposed a real gap (a fixed bug deserves a regression test)
2. Validate everything the repo's CI would:
   - `go build ./...` / `go vet ./...` / `gofmt -l .`
   - `go test -race ./...` (use an isolated `HOME` if local dotfiles pollute tests)
   - `golangci-lint run` on the touched packages
3. Commit with a Conventional Commit subject that references the review, e.g.:

       git commit -m "fix(<scope>): address <bot> review on <topic> (#<issue>)"

   Body: one bullet per finding fixed, one line per finding skipped with the reason.
4. `git push`

## Poll for the re-review

The bot re-reviews the new HEAD automatically after push. Poll — do not spam re-review requests:

1. Get the pushed SHA: `git log -1 --format=%H`
2. Poll the commit status until the bot's context reports completion (CodeRabbit sets a commit status):

       gh api repos/<owner>/<repo>/commits/<sha>/status --jq '{state, statuses: [.statuses[] | {context, state, description}]}'

   Wait for `context: "CodeRabbit"` → `state: "success"` with `description: "Review completed"`. Poll with `sleep 90`–`sleep 240` between checks; reviews typically land in 2–5 minutes.
3. Also confirm CI on the same commit: `gh pr checks <pr>`
4. If no review lands after ~3 polls, check for a rate-limit pause (see below) before you poll again.

## Handle a rate-limit pause

CodeRabbit pauses reviews when the repo hits its rate limit and posts an issue comment that says the review is skipped, plus a cooldown (for example "Please wait 14 minutes and 32 seconds before requesting another review").

1. Look for the pause comment on the PR:

       gh api repos/<owner>/<repo>/issues/<pr>/comments --jq '.[] | select(.user.login | test("coderabbit")) | select(.body | test("rate limit|Please wait|paused"; "i")) | {id, created_at, body}'

2. **Read the exact cooldown that CodeRabbit posted** in that comment — do not guess a default. Convert it to seconds and add a small buffer (~60s), then account for the time that already passed since `created_at`.
3. `sleep` for that cooldown. Use one `sleep <seconds>` call with a timeout large enough for the wait; split into several sleeps if the wait is longer than a single command timeout allows.
4. **After** the cooldown ends, kick off a new review by posting an issue comment:

       gh pr comment <pr> --body "@coderabbitai review"

   (`@coderabbit review` is the phrasing users type; the bot handle is `@coderabbitai`. Post it as a PR comment, not a reply on a line thread.)
5. Go back to *Poll for the re-review*. If the bot answers with another rate-limit comment, read the new cooldown and repeat this section.
6. A rate-limit pause is **not** a loop iteration and **not** a reason to stop. Never skip the cooldown and never spam `@coderabbitai review` while the pause is active — early requests reset the timer.

## Check for new findings and loop

After the re-review completes:

1. Re-fetch line comments and diff against the set you already handled (compare comment IDs / created_at timestamps — new findings have new IDs)
2. Check that old threads resolved:

       gh api graphql -f query='query { repository(owner: "<owner>", name: "<repo>") { pullRequest(number: <pr>) { reviewThreads(first: 50) { nodes { isResolved isOutdated path } } } } }'

3. **New actionable comments** → go back to *Triage* and repeat the loop
4. **Rate-limit comment instead of a review** → go to *Handle a rate-limit pause*, then resume
5. **Threads still open for findings you already fixed** → nudge the bot on each one (see below)
6. **No comments left, all threads resolved (or outdated), bot status green** → done

### Nudge unresolved threads

CodeRabbit sometimes leaves a thread open after you fixed the finding. For every such thread, reply **directly to that comment**:

       gh api repos/<owner>/<repo>/pulls/<pr>/comments/<comment-id>/replies \
         -f body="@coderabbitai please check that this has been addressed."

Then poll the thread again with the `reviewThreads` GraphQL query until `isResolved: true` or `isOutdated: true`. Nudge a given thread once per loop iteration — do not repeat the reply on the same thread while the bot is still working.

**Do not stop the loop early.** Keep looping — fix, push, poll, nudge, wait out rate limits — until CodeRabbit has no remaining comments **and** every thread is resolved or outdated. The only reasons to stop before that are: the PR closed or merged, a finding needs a human product decision (say which one and why), or the user tells you to stop. Report progress as you go so long waits stay visible.

## Report

- Findings fixed (with severity), skipped (with reasons), and any threads replied to
- Commits pushed this session (`git log --oneline` of the new commits)
- Final state: bot review status, CI status, unresolved thread count (target: 0)
- Any rate-limit pauses hit: the cooldown you waited and when you re-requested the review
- Threads you nudged with `@coderabbitai please check that this has been addressed.`
- If anything was intentionally left open, say so explicitly

## Guidelines

- Verify every finding against current code before touching anything — bots review diffs, not intent
- Keep each loop iteration a single commit; don't mix review fixes with unrelated work
- Reply on threads when skipping for "intentional behavior" — silent skips look like neglect and the bot may re-raise
- Prefer the bot's own `Prompt for AI Agents` phrasing when interpreting ambiguous findings
- Never `--force` push during the loop; the bot tracks incremental commits
- If the bot flags something CI also caught, fix once — don't attribute it twice
- On a rate-limit pause, wait the full cooldown CodeRabbit posted, then re-request the review — never abandon the loop because of a pause
- Finish the job: no remaining bot comments and zero unresolved threads is the only clean exit
