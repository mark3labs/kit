---
description: Resolve a GitHub issue by fixing, implementing, or documenting based on its type
---

Resolve GitHub issue #$1 by reading it, classifying it, and producing the appropriate change (bug fix, feature implementation, documentation update, etc.).

## Steps

1. **Fetch the issue**:
   - Run: gh issue view $1 --json number,title,body,labels,state,author,comments
   - If the issue is closed, stop and ask the user whether to proceed
2. **Classify the issue** from labels, title prefix, and body content:
   - `bug` / `fix:` → reproduce, then fix
   - `enhancement` / `feature` / `feat:` → design, then implement
   - `documentation` / `docs:` → locate and update docs
   - `question` / `discussion` → answer in a comment, do **not** open a PR
   - Anything else → ask the user how to proceed
3. **Create a working branch** off the default branch:
   - `git checkout main && git pull --ff-only`
   - Branch name: <type>/$1-<slug> (e.g. `fix/42-borderColor-ignored`, `feat/57-keyboard-clear`, `docs/63-widget-lifecycle`)
4. **Do the work** based on type:

   ### Bug (`bug` label / `fix:` title)
   - Reproduce the failure first (write a failing test if feasible) — if you cannot reproduce, comment on the issue asking for clarification and stop
   - Locate the root cause; do not patch symptoms
   - Add or extend a regression test that fails before and passes after the fix
   - Run `go test ./... -race` and `golangci-lint run`

   ### Feature (`enhancement` / `feature` label / `feat:` title)
   - Re-read the motivation and proposed implementation in the issue body
   - For large, ambiguous, or breaking changes, sketch the design in a comment on the issue and wait for sign-off before writing code
   - Implement behind sensible defaults; add godoc on every exported symbol
   - Add unit tests covering the new behaviour and edge cases
   - Update `README.md` / `docs/` if the public surface changed
   - Run `go test ./... -race` and `golangci-lint run`

   ### Documentation (`documentation` label / `docs:` title)
   - Open the file/URL referenced in the issue's "Documentation Location"
   - Apply the suggested improvement; verify code samples compile (`go build ./...`)
   - No tests required, but run `golangci-lint run` if Go files were touched

5. **Commit** with a Conventional Commit subject that references the issue:
   - fix: <summary> (#$1) / feat: <summary> (#$1) / docs: <summary> (#$1)
   - Body explains what changed and why, in the user's voice
6. **Push and open a PR** with "Fixes #$1" in the body — delegate to `/create-pr` if the user prefers, otherwise run:

       git push -u origin "$(git branch --show-current)"
       gh pr create --title "<type>: <summary> (#$1)" --body-file /tmp/pr-body-$1.md --base main

7. **Report** the branch name, commit SHA, and PR URL

## Guidelines

- Read the **entire** issue including comments before writing code — the latest comment often refines the ask
- If the issue is unclear, post a clarifying comment on the issue and stop; do not guess
- Do not close the issue manually — "Fixes #$1" in the PR handles that on merge
- Keep the change scoped to the issue; surface unrelated cleanups separately
- For breaking changes or architecture shifts, propose the design on the issue first and wait for maintainer sign-off
- If the issue is a duplicate or already fixed on `main`, comment with the reference and stop
