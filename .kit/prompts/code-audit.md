---
description: Read-only audit for dead code, duplication, boundary violations, and refactor opportunities
---

Perform a comprehensive **read-only** audit of this repository and report
findings. **Do not edit, rename, or delete any files.** Optional focus / scope
hints from the user: $@

## Scope

If the user supplied focus hints above (a package path, a subsystem name, a
concern like "TUI" or "extensions"), scope the audit accordingly. Otherwise
audit the whole repo, prioritising the highest-traffic packages first
(`cmd/`, `internal/`, `pkg/kit/` for this repo).

## Steps

1. **Map the repo first**:
   - `ls` / `find` the top-level layout and list every Go package
   - Read `AGENTS.md`, `README.md`, and any `pkg/*/doc.go` to understand the
     intended architectural boundaries (SDK vs internal vs TUI vs cmd vs
     extension surface)
   - Note the public SDK surface (`pkg/kit/`) and any documented invariants
     (e.g. "no dependency name leakage", "UI never imports extensions
     directly") — these define what counts as a violation

2. **Hunt for dead code**:
   - Run `go vet ./...` and capture warnings
   - Use `grep` to find exported symbols (`^func [A-Z]`, `^type [A-Z]`,
     `^var [A-Z]`, `^const [A-Z]`) and cross-reference call sites. Symbols
     with zero non-test references inside the module are suspects
   - Check for unreferenced files, `// TODO: remove` markers, commented-out
     blocks, and `_ = x` discard patterns
   - If `staticcheck`, `deadcode`, or `unused` are available on PATH, run
     them and include their output verbatim
   - **Do not delete anything** — list candidates with file:line and a
     confidence level (high / medium / low)

3. **Find unnecessary duplication**:
   - Look for near-identical function bodies, struct shapes, or switch
     statements across packages — `grep` for repeated function signatures
     and copy-pasted string literals / error messages is a fast first pass
   - Distinguish *coincidental* duplication (two things that happen to look
     alike but evolve independently) from *unnecessary* duplication (same
     intent, drifting in lockstep) — only flag the latter
   - For each cluster, propose where the extracted helper should live
     (which package, which file) and whether it crosses a boundary

4. **Check concerns / boundary violations**:
   - **SDK leakage**: grep `pkg/kit/` for imports of `internal/...` types
     in exported signatures, and for dependency-name leakage in exported
     names / godoc (e.g. library jargon appearing in `LLM*` types)
   - **UI ↔ extensions**: grep `internal/ui/` for any import of
     `internal/extensions/` — per AGENTS.md the UI must not import
     extensions directly; converters in `cmd/root.go` should bridge them
   - **cmd vs internal**: business logic living in `cmd/` that should be
     in `internal/` (and vice versa)
   - **Cyclic risk**: packages that import each other transitively or that
     reach across sibling boundaries unexpectedly
   - For each violation, cite the offending import / signature with
     file:line

5. **Spot refactor opportunities**:
   - Long functions (>80 lines) doing multiple unrelated things
   - Deeply nested conditionals that flatten well with early returns
   - Repeated `if err != nil { return fmt.Errorf("...: %w", err) }` chains
     that could become helpers — but only where the wrapping context is
     genuinely uniform
   - Structs with too many fields that hint at split responsibilities
   - Exported APIs that would be cleaner with options structs / functional
     options
   - Tests that share setup boilerplate ripe for a helper
   - Flag each with: location, current shape (1-2 lines), proposed shape
     (1-2 lines), and estimated risk (low / medium / high)

6. **Cross-check against project rules**:
   - Re-read `AGENTS.md` "Key Patterns" section and verify nothing in your
     findings contradicts the documented gotchas (Yaegi interface ban,
     `prog.Send()` from `Update()`, function-field bug, etc.) — if a
     "refactor" would reintroduce a known pitfall, drop it from the report
     and note why

7. **Write the report** as your final message (do not write it to disk)
   structured as:

   ```
   # Code Audit Report

   ## Summary
   - N dead-code candidates
   - N duplication clusters
   - N boundary violations
   - N refactor opportunities

   ## Dead Code
   ### High confidence
   - path/to/file.go:LINE — symbol — reason

   ### Medium confidence
   ...

   ## Duplication
   ### Cluster: <short name>
   - Sites: file:line, file:line, …
   - Suggested home: package/path
   - Notes: …

   ## Boundary Violations
   - Rule: <which rule from AGENTS.md / project convention>
   - Offender: file:line
   - Fix sketch: …

   ## Refactor Opportunities
   - Location: file:line
   - Current: …
   - Proposed: …
   - Risk: low/medium/high
   - Why it's worth it: …

   ## Suggested Next Steps
   1. …
   2. …
   ```

8. **End the report with an explicit reminder** that no files were modified,
   and recommend the user pick the highest-leverage items to act on
   manually (or via a follow-up `/fix-issue` style prompt) rather than
   running a sweeping refactor.

## Guidelines

- **Read-only, always**: no `edit`, no `write`, no `git commit`, no `go mod
  tidy`. Use only `read`, `grep`, `find`, `ls`, and read-only `bash`
  commands (`go vet`, `go build -o /tmp/...`, `staticcheck`, etc.)
- **Cite every finding** with `path/to/file.go:LINE` so the user can jump
  straight to it
- **Be honest about confidence**: false positives in a code audit are
  expensive — prefer "medium confidence, worth a look" over confidently
  wrong claims
- **Quantity isn't quality**: 10 sharp findings beat 100 nitpicks. Cut
  anything that's purely stylistic unless it directly causes one of the
  four issue categories above
- **Skip generated code** (`*.pb.go`, `*_gen.go`, anything under
  `vendor/`) and obvious third-party copies
- **Don't propose architectural rewrites** — stay within the existing
  shape of the repo and recommend incremental, reviewable changes
