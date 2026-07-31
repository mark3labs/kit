# Addendum — Third-Party Imports & Arbitrary UI

**Follow-up to `mvm-evaluation.md`.** Two requirements were raised after the
initial investigation: (1) extensions importing third-party libraries, and
(2) rendering arbitrary UI elements rather than predefined ones.

**Headline: neither requirement actually depends on switching to MVM. Both are
blocked by Kit's own choices, and both are fixable on Yaegi today.**

---

## 1. Third-party imports

### Kit's current inability is self-imposed

`internal/extensions/loader.go:380` is the whole story:

```go
i := interp.New(interp.Options{Env: os.Environ()})
```

`GoPath` is never set. Neither is `SourcecodeFilesystem`. That single omission —
not a Yaegi limitation — is why extensions today see only stdlib + `kit/ext`.

### Two ways to give extensions lipgloss, measured

| Approach | Time to first render | Per render | RSS | Works with charm's API? |
|---|---|---|---|---|
| Native Go (baseline) | 28–42 µs | 1.4–1.8 µs | 5.6 MB | — |
| **Yaegi + `yaegi extract` bindings** | **2.0–2.9 ms** | **1.6–2.4 µs** | **22.8 MB** | ✅ **yes** |
| MVM + `cmd/extract` bindings | 3.8–4.1 ms | 3.2–3.4 µs | 49.2 MB | ❌ **no** (see below) |
| MVM interpreting lipgloss *source* (local) | 790–830 ms | 89–107 µs | 158–172 MB | ✅ yes |
| MVM interpreting source (cold proxy) | **9.19 s**, 46 reqs, 4.9 MiB | — | — | ✅ yes |
| Yaegi interpreting lipgloss *source* | **impossible** | — | — | — |

**Compiled bindings are ~200× faster to first render and ~50× faster per render
than interpreting source, at 1/7 the memory.** For a shipped CLI this isn't close.

### MVM's genuinely impressive result — and why it doesn't win

MVM interpreted **20 packages of the real charm v2 stack** from source
(`bubbles/v2` spinner + `bubbletea/v2` + `lipgloss/v2` + ultraviolet + termios +
cancelreader), driving a real `tea.Program`, output byte-identical to native, in
1.54 s. Yaegi cannot do this at all (no generics, no `//go:build`).

But it required **three patches to upstream source**, each an MVM defect:
generic inference through a multi-type type-switch case (x/ansi, 9 sites);
missing overlays for assembly-implemented `x/sys/unix` functions; and a **hard
compiler panic** on map literals with converted keys + elided-type values
(ultraviolet's 298-entry `key_table.go`) that `Eval` doesn't even recover unless
the undocumented `LenientCompile` is set.

Shipping that means shipping patched copies of charm's source.

### Two hard MVM ceilings for Kit specifically

- **`View() tea.View` is unimplementable** by interpreted code through MVM's
  host-interface path — 19 ABI words against a 9-word (amd64) budget.
  `MVM_WORDDROPS` reports `over word budget`, so unlike the shape-catalog gaps
  this **cannot be fixed by adding a catalog entry**. Kit uses bubbletea v2.
- **MVM's binding bridge cannot call variadic-of-func parameters at all** — even
  with zero variadic args or explicit `slice...` expansion:
  ```
  reflect: CallSlice using main.Opt as type []main.Opt      ← zero args!
  ```
  That is the entire functional-options idiom: `tea.NewProgram(m, tea.WithAltScreen())`,
  `lipgloss.Place(…, WithWhitespaceChars(…))`, `huh.NewForm(…)`. Yaegi handles
  all of these correctly.

So for MVM the fast path (bindings) is the *less capable* one for exactly the
libraries Kit uses, and the capable path (source) is 200× slower and needs
patched dependencies.

### Also worth knowing

`@latest`-only resolution with no `go.mod` walking: 8 of 13 modules resolved
*off* lipgloss's own `go.mod`, and 3 extra modules were pulled in. It compiled by
luck. The nested major-version flaw (`foo/bar/v2/sub`) reproduces deterministically
and silently. Cold import is 9.2 s of network at extension-load time.

---

## 2. Arbitrary UI

### The blocker is Kit's API, not the interpreter

Kit's extension UI is a **"string in a Kit-owned frame"** model. Of ~10 UI
surfaces, only **five** accept a render *function*, and only **one** —
`EditorConfig.Render` — is a per-frame, width-aware callback whose output Kit
uses verbatim (`internal/ui/model.go:3364-3375`).

Everything else takes a **static snapshot string plus 1–2 style scalars**:

```go
type WidgetContent struct { Text string; Markdown bool }
type WidgetStyle   struct { BorderColor string; NoBorder bool }
```

Kit then owns all geometry (`renderWidgetSlot`, `model.go:3379-3413`). Widgets,
header, footer, overlays, and status entries have **no render callback at all**.

The design intent is explicit and deliberate — `internal/ui/messages.go:236`:

> *"Extensions get the same geometry as every other attributed block … and choose
> only the stripe color. Letting them choose more would let a single extension
> make the transcript look inconsistent."*

So "arbitrary UI" is partly a **product decision to revisit**, not a technical
limit. Notably, Kit already has the exact primitive needed —
`overlayDialog.body func(width int) string` (`internal/ui/overlay.go:105`) — it
is simply not exposed to extensions.

### Incidental findings worth filing

Several declared extension-UI fields are **dead code**: `WidgetContent.Markdown`
(ignored for widgets/header/footer), `ToolRenderConfig.BorderColor` and
`.Background` (converted in `cmd/root.go:579-580`, mirrored into
`ui.ToolRendererData`, never read), and **`PromptMultiSelect` is documented
(`api.go:296-310`, `SKILL.md:514`) but never wired** — it always returns
`Cancelled: true`. Also `MessageRendererConfig.Render`'s doc comment claims
output is emitted via `tea.Println`, but it is actually re-wrapped twice through
`AlertBody` + a herald Note gutter (`render/blocks.go:123-130`).

---

## 3. The finding that changes the recommendation

**Kit's stated reason for its entire API shape is incorrect.**

`internal/extensions/symbols.go:14-18` says:

> *"Interfaces … are NOT exported because Yaegi cannot generate interface
> wrappers for them."*

Yaegi cannot *synthesise* wrappers at runtime — that's the `genInterfaceWrapper`
crash I reproduced. But **`yaegi extract` generates them at build time, and they
work.** I verified this against **Kit's real `internal/extensions` package**,
under Kit's exact `loader.go` setup (`stdlib` + `unrestricted` + `ext.Symbols()`):

```
tool: hostiface._KitTool
  Name()        = "grep"
  Execute(ok)   = "ran with 2 args (call 1)" err=<nil>
  Execute(fail) = "" err=grep failed on call 2
  Execute(3rd)  = "ran with 1 args (call 3)"  <- ptr-receiver state persisted
widget: hostiface._WidgetRenderer  ID()="spinner"
  Render(64) frame 0 = "\x1b[1;35m| working ========\x1b[0m"
  Render(64) frame 1 = "\x1b[1;35m/ working ========\x1b[0m"
  Render(64) frame 2 = "\x1b[1;35m- working ========\x1b[0m"
```

A stateful pointer-receiver tool **and** a stateful per-frame `Render(width int) string`
widget — the exact "arbitrary UI" primitive — both implemented by interpreted
code, both driven by the host. Independently, an interpreted `tea.Model` drove a
real bubbletea program under Yaegi.

### Two constraints I found that the API redesign must respect

1. **The export key must match the type's REAL Go package path.** This is the
   critical one. Kit's virtual `kit/ext` path works for *structs* but **fails for
   interfaces**:
   ```
   === A. virtual path 'kit/hostiface' (what Kit does today) ===
   virtual, wrap, val, main    EVAL ERROR: invalid memory address or nil pointer dereference
   virtual, wrap, ptr, Init    EVAL ERROR: invalid memory address or nil pointer dereference

   === B. REAL package path ===
   real, wrap, val, main       OK hostiface._KitTool
   real, wrap, ptr, Init       OK hostiface._KitTool
   real, NO wrap, val, main    EVAL ERROR: reflect: call of reflect.Value.Type on zero Value  (control)
   ```
   Extensions would import `github.com/mark3labs/kit/ext` rather than `kit/ext`.
   This argues for promoting the extension API out of `internal/` into a real
   published package — which also fits the existing `pkg/kit/` SDK direction.

2. **Nil funcs through a wrapper panic.** `return nil` for a func-typed result
   (idiomatic bubbletea `Init() tea.Cmd`) panics with `reflect: New(nil)`.
   Survivable (`return func() tea.Msg { return nil }`) but hostile, and it would
   need documenting.

---

## 4. Revised recommendation

The original verdict stands — **don't migrate to MVM** — but the reasoning is now
stronger, because the two capabilities that motivated the question are reachable
without it.

### Do this instead, in order

1. **Replace hand-written `symbols.go` with `yaegi extract`-generated bindings,
   including `_Iface` wrappers.** This is the highest-leverage change in the
   entire investigation. It unlocks host interfaces for extensions and removes
   the documented reason for the 79-function-field `Context` and the 43 `On*`
   methods. It also eliminates the silent-zero-value failure mode that
   `symbols.go` maintenance currently risks.

2. **Move the extension API out of `internal/` to a real import path.** Required
   for interfaces to work (finding #1 above), and aligned with `pkg/kit/`.

3. **Add `Render func(width int) string` to `WidgetConfig` / `OverlayConfig` /
   header / footer.** Pure Kit-side change, no interpreter involvement. The
   primitive already exists at `internal/ui/overlay.go:105`. This is "arbitrary
   UI" — decide deliberately how much of `messages.go:236`'s consistency policy
   you want to relax.

4. **Ship lipgloss to extensions as generated bindings** (`yaegi extract
   github.com/charmbracelet/lipgloss`), ~2 ms and ~23 MB RSS. Optionally set
   `GoPath` for pure, non-generic packages. Do **not** pursue source
   interpretation of the charm stack.

5. **File the dead-code findings**: `WidgetContent.Markdown`,
   `ToolRenderConfig.BorderColor`/`.Background`, unwired `PromptMultiSelect`,
   and the incorrect `MessageRendererConfig.Render` doc comment.

### What would still require MVM

Only one thing: extensions importing **arbitrary third-party source** that has no
pre-generated bindings — particularly generic libraries, which Yaegi cannot parse
at all. That is a real capability gap. But today it costs patched upstream
sources, 9 s cold imports, `@latest` non-reproducibility, a broken variadic-of-func
binding bridge, and an unimplementable `View() tea.View`.

Revisit when the triggers in `mvm-evaluation.md` §4 are met — plus two more:
- [ ] Variadic-of-func parameters callable through the native binding bridge
- [ ] Method-shape word budget raised, or `View() tea.View`-sized signatures supported

---

## Appendix — artifacts

- `/tmp/mvm-poc/REPORT-thirdparty.md` (729 lines) — tests A–G, full detail
- `/tmp/yteapoc/` — Yaegi control module; `testF3` drives a real bubbletea
  program from an interpreted `tea.Model`; `testF6` is the interface-wrapper matrix
- Kit interface probe was run inside the Kit module against the real
  `internal/extensions` package, then **removed**; repo is clean and builds.
- `/tmp/mvm-research` verified pristine at `60b7c9c`.

---

## Correction (post-implementation)

**The named-function-reference warning is REAL.** An earlier pass in this
investigation failed to reproduce it and this document recommended deleting the
guidance. That was wrong. Adversarial re-testing found the exact trigger, and it
requires three conditions simultaneously:

1. A bare identifier naming a `func` declaration or method value, AND
2. the destination is a host type — a struct field *or a direct argument*, AND
3. the reference sits inside a function literal that appears **before** the
   declaration in the file.

Minimal repro (verified independently):

```
helper declared AFTER Init    FAIL got="" err=<nil> (want "LE:in")
helper declared BEFORE Init   OK   got="LE:in"
```

Root cause: `interp/cfg.go:1547` generates function-literal bodies eagerly in
source order, while top-level `funcDecl` bodies are generated later. The wrapper
snapshots an empty frame, so `reflect.MakeFunc` returns zero values for every
result. Present in yaegi v0.12.0–v0.16.1; never fixed.

The docs were also **too narrow**: it affects direct arguments too
(`api.OnToolCall(fn)`, `api.RegisterShortcut(def, fn)`), which they never
mentioned. Docs in `AGENTS.md`, `api.go` and `SKILL.md` have been rewritten to
describe the precise trigger and the three valid fixes.

**Interfaces, measured precisely.** A generic `On(EventType, HandlerFunc)` is
*nearly* viable — a handler returning a concrete `Result` works fine. But the
idiomatic `return nil` panics in `genInterfaceWrapper` during `Eval`, and that
panic was **unrecovered**, killing the whole Kit process. That is why the generic
event API is still not offered, and the `symbols.go` comment has been corrected
to say this precisely rather than "Yaegi cannot generate interface wrappers".
