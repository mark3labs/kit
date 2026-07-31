# MVM as a Yaegi Replacement — Investigation Report

**Date:** 2026-07-30
**Subject:** Evaluating `github.com/mvm-sh/mvm` @ `60b7c9c` as a replacement for `github.com/traefik/yaegi v0.16.1` in Kit's extension system
**Verdict:** **Do not migrate now.** Re-evaluate in 2–3 Go release cycles (~9–12 months).

---

## TL;DR

| Question asked | Answer |
|---|---|
| Is MVM mature enough to replace Yaegi? | **No.** 95 days old, bus factor 1, no release in 5+ weeks, `0.x` with 54 exported symbols deleted in 3 months. |
| Would we get greater flexibility in UI/Logic? | **Partially yes** — generics, multi-file extensions, and far better panic diagnostics are real, low-risk wins. |
| Could we ditch the rigid interface definitions? | **Yes, but you'd trade one rigidity for a worse one.** See the central finding below. |

MVM is written by @mvertes — the original author of Yaegi — and it is technically
impressive. It genuinely fixes the limitation that shaped Kit's entire extension
API. But the fix lands on a mechanism that imposes a *new* constraint which is
operationally worse for Kit than the one it removes.

---

## The central finding

Kit's extension API is shaped by one Yaegi limitation, documented at
`internal/extensions/api.go:1253-1260` and `symbols.go:14-18`:

> *"Interfaces (Event, Result) and the HandlerFunc type are NOT exported because
> Yaegi cannot generate interface wrappers for them."*

This forced the current design: **99 concrete structs, a `Context` with 79
function fields, and 43 `On*` methods instead of one `On(EventType, HandlerFunc)`.**

**Yaegi's limitation is real — I reproduced it.** An interpreted type implementing
a host interface hard-crashes:

```
EVAL ERROR (interp.Panic): runtime error: invalid memory address or nil pointer dereference
github.com/traefik/yaegi/interp.genInterfaceWrapper(0xc0004a5040, {0x10b2d70, 0xe21240})
github.com/traefik/yaegi/interp.callBin(0xc0004a4b40)
```
Both pointer and value receivers. **MVM handles the identical program correctly:**
```
mode=ptr   concrete host-side type: *main.MyTool
  Name() = "grep"
  Execute(ok) = "ran grep with 2 args", err=<nil>
mode=val   concrete host-side type: main.ValTool     # value receivers too
```

So far this reads as a clean win. **It isn't**, and here is the part that decides
the recommendation.

MVM has **two different mechanisms** for crossing the host boundary, with
radically different constraints:

| Path | Mechanism | Shape constraint |
|---|---|---|
| **Func fields** (Kit's current design) | ADR-006 `WrapFunc` / `reflect.MakeFunc` | **None. Arbitrary signatures.** |
| **Interface methods** (the "win") | ADR-021 synthesized rtypes + pre-generated stub pools | **Finite, pre-registered catalog.** |

I verified this directly against unpatched MVM. Same signature `func(string) string`,
two different paths:

```
===== FUNC FIELDS (exotic shapes, unpatched mvm) =====
  Render("a")   = "R:a"
  Fetch("u")    = "body:u", <nil>
  Store("k",1)  = <nil>
  Exotic(string, []int, map[string]int, float64) (map[string][]string, error) = map[a:[x]], <nil>
  Nested(func(int) int) = 42
  => ALL SHAPES OK (no catalog entry needed)

===== INTERFACE METHOD (same func(string) string shape) =====
  EVAL ERROR: panic: reflect: Call using *main.R as type main.Renderer
```

**Kit's existing function-field design is already sitting on MVM's
constraint-free path.** Migrating to real interfaces — the entire point of the
exercise — moves Kit onto the constrained one, where:

- Every method signature must exist in a **vendored** catalog
  (`internal/stubs/gen_pools.go`). `func(string) string`,
  `func(string) (string, error)`, `func(string, any) error`, and
  `func(Struct) bool` are **all missing** from the shipped catalog. Four of the
  most ordinary signatures in Go.
- Missing shapes fail with an opaque `reflect: Call using X as type Y` panic,
  with no diagnostic unless `MVM_WORDDROPS=1` is set.
- Stub slots are **finite and monotonically consumed** — never reclaimed. Kit's
  `dev-reload` extension dies hard:
  ```
  EXHAUSTED after 256 successful reloads
  error: stubs: shape Wpi_pi stub pool exhausted (cap=256)
  ```
  Permanent failure for the life of the process.

So: yes, you can ditch the rigid interface definitions. The replacement is a
finite, forkable, silently-failing method-shape budget. For Kit that is a
**worse** rigidity than the current one, because today's constraint is *loud and
compile-time-ish*, while the new one is *quiet and runtime*.

---

## What MVM genuinely fixes (low-risk, real wins)

These do **not** touch the shape catalog and are the honest upside:

1. **Generics** — 8/8 tests pass (interpreted generic funcs/types, constraints,
   explicit instantiation, `slices`/`maps`/`cmp`). Yaegi has **none**.
2. **Multi-file extensions** — `EvalFiles([]goparser.PackageSource)` resolves
   cross-file references in any order. Kit is currently locked to a single
   `os.ReadFile` + single `Eval` (`loader.go:397-405`).
3. **Panic diagnostics** — every panic class becomes a normal `error` with
   `file:line:col` plus the source line and a caret. The interpreter stays
   reusable. Kit's extension error messages would improve markedly.
4. **Memory** — 1.05 MB/instance vs Yaegi's 1.73 MB (−39%).
5. **A call-path watchdog** (`vm.SetSafepointHook`) — Yaegi has nothing.

---

## What MVM does *not* fix

**Concurrency is no better, and the failure mode is worse than a crash.**
This matters because Kit invokes extension callbacks from many goroutines (TUI
event loop, tool execution, widget refresh). Reproduced across multiple runs:

```
mode=nolock goroutines=50 iter=200 -> wrongValues=0 panics=0 interpretedHits=9262 want 10000
mode=nolock goroutines=50 iter=200 -> wrongValues=0 panics=0 interpretedHits=9208 want 10000
mode=nolock goroutines=50 iter=200 -> wrongValues=0 panics=0 interpretedHits=9271 want 10000
```

**~8–11% of interpreted package-level state updates silently vanish.** No crash,
no wrong return values, no panic. `-race` fires immediately on
`vm.Machine.globals`, which is shared across pooled runner Machines with **no
synchronization** (the func-field side tables *are* `RWMutex`-protected; globals
are not).

This lands squarely on Kit's documented state-management pattern —
*"package-level vars in extensions captured in closures."*

A single host mutex fixes it completely (`interpretedHits=10000` ×3, race-clean)
at ~4.4× throughput cost (~47k calls/s — fine for a TUI). But it would have to
wrap **every** entry point, and must **not** be held across Kit's channel-based
blocking prompts (`PromptSelect`) or extensions deadlock.

**Runaway code** — no `context.Context`, no cancel. A `for {}` with no function
calls is uninterruptible; the safepoint hook is a process-global polled only on
the call path.

---

## Risk assessment

| Dimension | Severity | Evidence |
|---|---|---|
| **Project maturity** | 🔴 High | First commit 2026-04-26 (95 days). 637 commits, **627 by one person**. Velocity collapsed 296 → 41 commits in July; 8 commits in the last 21 days. |
| **API stability** | 🟠 Med-High | 7 tags, all `0.x`, **no release in 5+ weeks**. 54 exported symbols deleted in 3 months. README: *"the embedding API will still change. Pin a commit."* `go` directive bumped 1.24 → 1.26 *after* v0.5.0. |
| **Go version coupling** | 🔴 High | `internal/runtype` mirrors **12 unexported `internal/abi` structs byte-for-byte**. No fallback, no runtime guard — the only check is a test in MVM's own CI. Failure mode on a future Go release is **silent memory corruption / GC crashes**, not a build error. The abi mirror already broke once (Go 1.24 Swiss maps). |
| **Binary size** | 🟠 Med-High | Measured: yaegi+stdlib **26.3 MB** → mvm+`stdlib/all` **55.6 MB**. **+30 MB (2.1×)** on a binary Kit ships via npm. `stdlib/core` only saves 12 MB and drops `os`, `os/exec`, `path/filepath`. |
| **Hot-reload cap** | 🟠 Med-High | Hard failure at 256 reloads, unreclaimable. Directly affects Kit's `dev-reload`. |
| **`//go:linkname` hardening** | 🟢 Low | Verified *not* a problem — all 4 symbols are push-linknamed by std itself, exempt from Go 1.23+ `-checklinkname=1`. Builds clean with no ldflags. Credit to MVM here. |
| **Stdlib coverage** | 🟢 Low | 16/17 of Kit's actually-used packages are green. Only `runtime` (1 extension) is untested. |
| **License** | 🟢 Low | BSD-3-Clause, compatible with Kit's MIT. |
| **Escape hatch** | 🟢 Low | Only **3 files** import Yaegi (`loader.go`, `symbols.go`, `pkg/extensions/test/harness.go`). Symbol-table shapes are near-identical (`Use` ↔ `ImportPackageValues`, both `map[string]map[string]reflect.Value`). Migration is cheap to attempt *and* cheap to revert. |

### Two corrections to note

**The Windows "blocker" is one file.** Initial analysis flagged
`stdlib/all` failing on `windows/amd64` as a release-matrix blocker (Kit ships
win32 via npm). I traced it: the *only* failure is `stdlib/ext/log_syslog.go`
missing a build constraint. Removing that one file:
```
windows ./stdlib/core: OK    windows ./interp: OK    windows ./vm: OK
windows ./stdlib/all: OK  (after removing log_syslog.go)
```
This is a one-line upstream fix (`//go:build !windows && !plan9`), not a
structural problem. Downgraded from High to Low.

**Kit's documented "named function reference" bug did not reproduce.**
`AGENTS.md` and `api.go:2192-2206` warn that named func refs assigned to struct
fields return zero values under Yaegi, requiring closure wrappers everywhere.
Testing at v0.16.1 — including through `loader.go`'s exact code path with the
real `ext.EditorConfig`/`ext.Context`/`ext.EditorKeyAction` types — **named refs
worked correctly** in both engines. Either it was fixed upstream, or the trigger
is narrower than documented. **This is worth re-verifying independently**: it's
a constraint currently imposed on every extension author, and it may no longer
be necessary regardless of what happens with MVM.

---

## Recommendation

### 1. Do not migrate now — but the reason is the *combination*, not any single flaw

The interface win is real, and it is the one thing that could restructure Kit's
API. But it is gated behind a finite, vendored shape catalog that fails silently
and caps hot-reload at 256. That converts a clean architectural win into
"fork MVM and own `internal/stubs` forever."

Pair that with a 95-day-old single-maintainer project, `internal/abi` mirroring
with no runtime guard, and +30 MB on an npm-distributed binary, and the risk
budget is spent well before the payoff arrives.

### 2. Harvest the free wins now, without MVM

Two items on the wish list don't need an interpreter swap:

- **Multi-file extensions.** Kit's single-file limit is self-imposed at
  `loader.go:397-405` (`os.ReadFile` + one `Eval`). Yaegi can evaluate multiple
  sources into one interpreter. This is a contained change.
- **Re-verify the named-func-field warning.** If it no longer reproduces, delete
  the guidance from `AGENTS.md`, `api.go`, and `SKILL.md` and stop taxing every
  extension author with mandatory closure wrappers.

### 3. Reduce the cost of a future swap (cheap, do it opportunistically)

Only 3 files touch Yaegi and the two symbol-table APIs are nearly identical.
Introducing a small internal `engine` seam now — `New`, `ImportSymbols`,
`Eval`, `Lookup` — would make a future migration a contained experiment rather
than a rewrite. This is worth doing *on its own merits* for testability.

### 4. Re-evaluate when these are true

Concrete, checkable triggers:

- [ ] MVM survives **two Go releases** (1.27, 1.28) without an `internal/abi` break — the single most important signal
- [ ] A **runtime version guard** in `internal/runtype` that fails loudly instead of corrupting memory
- [ ] **≥2 sustained contributors**, or a v1.0 with a stability commitment
- [ ] The **shape catalog covers ordinary signatures** (`func(string) string` et al.) or becomes dynamically extensible
- [ ] **Stub pools become reclaimable** (removes the 256-reload cap)
- [ ] macOS **and** Windows in MVM's CI matrix — the abi mirror is the most platform-sensitive code in the project and is currently only exercised on Linux

Items 4 and 5 are the ones that would actually make the interface win usable for
Kit. Without them, migrating buys a fork.

### 5. If you want to keep a hand in

The highest-value low-cost move is a **spike, not a migration**: prototype the
"`Context` as a ~10-method interface instead of 79 function fields" design
against MVM to see what the API *could* look like. That answers the design
question — is the interface-based API actually nicer to write extensions
against? — without committing to the engine. If the answer is "not much nicer,"
the entire migration rationale evaporates and you've spent a day.

---

## Appendix — verified environment & artifacts

- MVM `60b7c9c`, Go 1.26.5 linux/amd64. Kit's `go.mod` is already `go 1.26.5`, so
  MVM's `go 1.26` floor is satisfied today.
- Real embedding API (differs from docs): type is `interp.Interp` (not
  `Interpreter`); shape catalog is at `internal/stubs/gen_pools.go` (ADR-021's
  `stdlib/stubs/` path is stale).
- `/tmp/mvm-poc/` — POC module, 20 test programs + `REPORT.md` (754 lines)
- `/tmp/yaegi-poc/` — Yaegi control module
- `/tmp/mvm-research` — clone, **reverted to pristine upstream**
- `btca.config.jsonc` — added `mvm` as a research resource (only change to the Kit repo)
