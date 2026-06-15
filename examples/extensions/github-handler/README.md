# GitHub Handler Extension

The GitHub handler is the runtime half of Kit's GitHub integration (issue
[#60](https://github.com/mark3labs/kit/issues/60), Phase 2b). It is designed to
run **inside a GitHub Actions runner**, driven by the workflow that
`kit github install` scaffolds.

When a collaborator comments `/kit <request>` on an issue or pull request, the
workflow boots Kit headlessly with this extension loaded. The extension then:

1. **Parses** the triggering event from `GITHUB_EVENT_PATH`.
2. **Enforces permissions** — only comments whose `author_association` is
   `OWNER`, `MEMBER`, or `COLLABORATOR` are acted on.
3. **Reacts** with 👀 on the trigger comment so the human knows Kit is working.
4. **Gathers context** — the issue thread, or the pull request diff and
   comments (via the `gh` CLI).
5. **Drives the agent** with the request plus context.
6. **Posts the response** back as a comment, and — if the agent left
   uncommitted changes — pushes a branch as the `kit-agent[bot]` identity and
   opens a pull request.
7. **Swaps the reaction** to 🚀 (or 😕 on error) when finished.

Outside of GitHub Actions (i.e. when `GITHUB_ACTIONS != "true"`) the extension
is inert, so it is safe to keep loaded during normal local use.

## Requirements

- The [`gh` CLI](https://cli.github.com/) on `PATH`, authenticated via
  `GITHUB_TOKEN` (GitHub Actions provides this automatically).
- `git` on `PATH` with push access for opening pull requests.
- A provider API key for the model Kit runs (e.g. `ANTHROPIC_API_KEY`).

## Usage in a workflow

`kit github install` writes `.github/workflows/kit.yml`. To wire in this
handler, load it when invoking Kit headlessly inside the action, for example:

```bash
kit --quiet --no-session \
  -e path/to/github-handler/main.go \
  --model "$KIT_MODEL"
```

The extension reads the GitHub event itself, so no prompt argument is required —
it constructs the prompt from the comment and repository context and drives the
agent via the session lifecycle.

## Environment variables

| Variable             | Purpose                                                        |
|----------------------|---------------------------------------------------------------|
| `GITHUB_ACTIONS`     | Must be `true` for the handler to activate.                    |
| `GITHUB_EVENT_PATH`  | Path to the JSON event payload (set by Actions).              |
| `GITHUB_TOKEN`       | Used by `gh`/`git` for API and push operations.               |
| `KIT_GITHUB_DRY_RUN` | When set, log `gh`/`git` side effects instead of running them. |

## Options

| Option            | Default | Description                                          |
|-------------------|---------|------------------------------------------------------|
| `github.dry-run`  | `false` | Log GitHub/git side effects instead of executing.    |

## Dry-run mode

Set `KIT_GITHUB_DRY_RUN=1` (or the `github.dry-run` option) to exercise the
parsing, permission, and prompt-building logic without touching the network or
the working tree. Every `gh`/`git` mutation is printed instead of executed.
This is what the unit tests (`main_test.go`) use to stay deterministic.

## Security

- Triggers are gated to write/admin collaborators only.
- The workflow uses `persist-credentials: false` and least-privilege
  `permissions`, mirroring established practice.
- Commits are attributed to a dedicated `kit-agent[bot]` identity.
