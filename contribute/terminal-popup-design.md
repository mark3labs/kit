# Pop-up Terminal in Kit — Feasibility Study and Recommendations

Status: exploration / design proposal. No code changes made.

## 1. The request

> Spawn a terminal in a pop-up via an extension and a custom command. Use the
> user's pre-configured terminal. Toggle on or off. On first open it persists
> state until the Kit session closes.

"Pre-configured terminal" has two readings. Both are covered below:

- **(a) the user's shell** — `$SHELL`, run inside Kit. This is what a "pop-up
  terminal" normally means and is what the rest of this document assumes.
- **(b) the user's terminal emulator** — `$TERMINAL` / kitty / wezterm, opened
  as a separate OS window. Trivial today, but it is not a pop-up *in* Kit. See
  Option D.

---

## 2. What Kit has today

### 2.1 Already present and reusable

| Capability | Where | Notes |
|---|---|---|
| PTY allocation, sizing, byte pumping | `internal/daemon/server.go:1017-1047`, `client_attach.go` | `github.com/creack/pty v1.1.24` is a **direct dependency**. `pty.StartWithSize`, `pty.Setsize`, `pty.Open` all in use. |
| Cell-level overlay compositing | `internal/ui/composite.go:30-94` | `compositeOverlay` / `compositeAnchored` merge a box over the base view through an `ultraviolet.ScreenBuffer`. ANSI survives on both sides. Exactly the primitive an embedded pane needs. |
| Modal state + key capture | `internal/ui/model.go:1693-1696`, `6587-6662` | `stateOverlay` early-returns in `Update()` and swallows the whole keyboard except `ctrl+c`. A `stateTerminal` would be a copy of this. |
| Full-screen terminal handover | `App.SuspendTUI` — `internal/app/app.go:1596-1620` | `prog.ReleaseTerminal()` → callback → `prog.RestoreTerminal()`. Exposed to extensions as `ctx.SuspendTUI`. |
| Per-frame widget rendering | `WidgetContent.Render func(width int) string` + `RefreshHz` (1..30) — `internal/extensions/api.go:1710-1750` | Output used verbatim, **no ANSI stripping**, **no max-height clamp** (`distributeHeight`, `model.go:5261-5296`). `examples/extensions/bad-apple.go` drives 96x37 at 30fps through it. |
| Coalesced UI push from extensions | `App.NotifyWidgetUpdate` — `app.go:1440-1485` | 16 ms leading+trailing coalescing. The template for a `NotifyTerminalUpdate`. |
| Leader-chord keybinding | `model.go:2061-2183` | `ctrl+x` + `s`/`t`/`m`/`e` are taken. `ctrl+x` never reaches extensions, so a built-in chord is free real estate. |

### 2.2 Missing

| Gap | Detail |
|---|---|
| **VT emulator** (bytes → cell grid) | No dependency in `go.mod`/`go.sum`. `charmbracelet/ultraviolet` is a renderer + input decoder, not an emulator. |
| **Updatable overlay** | `OverlayConfig.Content` is a `WidgetContent` **value**, copied once into `overlayDialog.content` (`overlay.go:186`). Grep for `UpdateOverlay|OverlayUpdate|CloseOverlay|RefreshOverlay` → **zero matches**. No `OverlayUpdateEvent`, no runner state map, no notify. `ShowOverlay` is blocking and channel-based (`cmd/extension_context.go:216-238`). Overlays cannot animate. |
| **Non-modal / programmatic overlay dismissal** | An open overlay can only be closed by the user. An extension cannot close it. |
| **PTY from inside an extension** | See 2.3 — effectively impossible. |
| **Mouse events for extensions** | No hook. `tea.MouseWheelMsg` etc. never reach extension code. |

### 2.3 Yaegi constraint: an extension cannot allocate a PTY

Verified experimentally:

- `stdlib/unrestricted.Symbols` **does** export `syscall`, `os`, `os/exec`,
  `log` (`internal/extensions/loader.go:516-521`).
- But **`unsafe` is not available**: `import "unsafe" error: unable to find
  source related to: "unsafe"`.

`ioctl(TIOCGPTN)` / `ioctl(TIOCSPTLCK)` / `ioctl(TIOCSWINSZ)` all need a
`uintptr(unsafe.Pointer(&x))` argument. Without `unsafe` an extension cannot
unlock a pty slave or set the window size. `creack/pty` is not importable from
Yaegi either (extensions may import stdlib + `kit/ext` only).

**Consequence:** a real terminal must be hosted by Kit core. An extension can
own the *policy* (command, toggle, styling) but not the *PTY*.

### 2.4 Key-routing constraints for extensions

From `model.go:1886-2265`, in order. Keys an extension's editor interceptor
(`ctx.SetEditor(HandleKey:)`) **never** sees:

- `ctrl+c` (always, `1892`), `ctrl+x` and its chord suffix (`2203`, `2061-2183`)
- any key while an overlay or prompt is open (`1689-1696`)
- `pgup` / `pgdown` / `ctrl+home` / `ctrl+end` while `stateInput` (`1949-1970`)
- `shift+tab` on reasoning models (`1975-1982`)
- `esc` while `stateWorking` (`2186-2199`)
- any registered extension shortcut (`1936-1945`)
- **pasted text** — arrives as `tea.PasteMsg`, which has no case in the key
  path and goes straight to the textarea (`model.go:3140`, `input.go:598`)

Key strings are `msg.String()`: printable runes come through as literal text
(`"A"`, not `"shift+a"`), space is `"space"`, modified keys are `"ctrl+a"`.
`HandleKey` runs **synchronously on the Bubble Tea event loop** — blocking it
freezes the TUI.

So a pure-extension terminal cannot receive `ctrl+c` (the single most important
key in a shell) or paste. That alone disqualifies Option B as a real terminal.

---

## 3. Options

### Option A — `SuspendTUI` + a multiplexer (works today, zero core changes)

A `/term` command that calls `ctx.SuspendTUI` and runs the user's shell under
`tmux`/`abduco`/`dtach`, keyed to the Kit session ID:

```go
api.RegisterCommand(ext.CommandDef{
    Name: "term", Description: "Toggle a persistent shell",
    Execute: func(args string, ctx ext.Context) (string, error) {
        name := "kit-" + ctx.SessionID
        return "", ctx.SuspendTUI(func() {
            // new-session -A: attach if it exists, create if it does not.
            c := exec.Command("tmux", "new-session", "-A", "-s", name)
            c.Dir = ctx.CWD
            c.Stdin, c.Stdout, c.Stderr = os.Stdin, os.Stdout, os.Stderr
            c.Run()
        })
    },
})
```

- **Persistence**: solved properly. The tmux server owns the shell; cwd, env,
  history, running jobs, scrollback all survive. Kill it in
  `OnSessionShutdown` with `tmux kill-session -t kit-<id>`.
- **Toggle**: `/term` attaches, `ctrl-b d` detaches back into Kit instantly.
- **Fidelity**: 100%. It *is* a real terminal, with real `ctrl+c`, real paste,
  real mouse, real 256-colour, real full-screen apps.
- **Cost**: ~40 lines. No new dependency, no core change.
- **Downside**: full-screen handover — Kit's UI is gone while attached
  (`prog.ReleaseTerminal` exits the alt screen). It is not a *pop-up*.
- **Downside**: requires tmux/abduco/dtach installed. Degrade gracefully to a
  plain `$SHELL` (no persistence) when absent.
- Prior art in-repo: `examples/extensions/interactive-shell.go`.

### Option B — Widget + editor interceptor "console" (works today, not a terminal)

A tall widget (`Render` + `RefreshHz`) drawn above the input, fed by a
long-lived `bash` with pipe stdio, keys captured via `SetEditor.HandleKey`.

- **Verdict: do not ship this as a terminal.** No `ctrl+c`, no paste, no PTY
  (so no job control, no prompt colours, no `vim`/`htop`/`less`, and most
  programs switch to non-interactive line-buffered mode when `isatty` is
  false — same limitation `internal/core/bash.go` already documents).
- It is a fine *command console* (type a command, see buffered output inline),
  which is a genuinely useful but different feature.
- A halfway house: shell out to `script -qfc "$SHELL" /dev/null` to borrow a
  PTY from util-linux. That fixes isatty, but leaves the key-routing gaps and
  still needs a VT emulator to interpret the output.

### Option C — Core PTY pane + extension API (recommended for a real pop-up)

Host the terminal in Kit core; expose a thin, non-blocking API to extensions.

**The one new dependency is already validated.** `github.com/charmbracelet/x/vt`
is a full VT emulator built on `ultraviolet` — the same cell library Kit
already uses in `composite.go`. It compiles and runs against Kit's *exact*
ultraviolet pin (`v0.0.0-20260812204455-68fa937c71be`), verified with a probe
build. Relevant surface:

```go
e := vt.NewSafeEmulator(w, h)   // concurrency-safe wrapper
e.Write(ptyBytes)               // io.Writer — feed the PTY read pump
e.Render() string               // ANSI string → drop into compositeAnchored
e.Draw(scr uv.Screen, area uv.Rectangle) // or draw straight into the ScreenBuffer
e.SendKey(uv.KeyEvent)          // key → bytes, no hand-rolled encoder needed
e.Paste(text)                   // bracketed paste
e.InputPipe() io.Writer         // wire to the ptmx
e.Resize(w, h)
e.SetCallbacks(vt.Callbacks{Bell: ..., Title: ..., AltScreen: ...})
e.Scrollback() / SetScrollbackSize(n)  // default 10000 lines
```

`SendKey` taking a `uv.KeyEvent` is the decisive detail: `tea.KeyPressMsg` in
bubbletea v2 *is* an ultraviolet key event, so key forwarding is a direct
hand-off with no ANSI encoding table to write and maintain.

#### Sketch

**1. `internal/terminal/session.go`** — host-side PTY session.

```go
type Session struct {
    mu     sync.RWMutex
    cmd    *exec.Cmd
    ptmx   *os.File
    emu    *vt.SafeEmulator
    cols, rows int
    alive  bool
    title  string
}

func Open(cfg Config) (*Session, error)   // pty.StartWithSize + go readPump
func (s *Session) Write(p []byte) (int, error)
func (s *Session) SendKey(k uv.KeyEvent)
func (s *Session) Resize(cols, rows int) error  // pty.Setsize + emu.Resize
func (s *Session) Render() string
func (s *Session) Close() error
```

`readPump` copies `ptmx → emu` and calls a damage callback, coalesced at ~30 Hz
(mirror `App.NotifyWidgetUpdate`'s leading+trailing 16 ms window,
`app.go:1440-1485`).

**2. Ownership → persistence.** The `*Session` lives on `App`, not on
`AppModel`. Showing and hiding only flips `AppModel.state`; the PTY, the child
process, and the emulator's scrollback keep running underneath. That is
"persists until the Kit session closes", for free. `App.Close` kills it.

**3. `internal/ui/terminal_pane.go` + `stateTerminal`.** Copy the `stateOverlay`
shape:

- `View()`: `finalContent = compositeAnchored(finalContent, pane.Render(), anchor, m.width, m.height)` — insert next to `model.go:3352-3356`.
- `Update()`: early-return branch next to `model.go:1693-1696`; forward
  `tea.KeyPressMsg` to `session.SendKey`, `tea.PasteMsg` to `emu.Paste`,
  `tea.MouseWheelMsg`/click to `emu.SendMouse`, `tea.WindowSizeMsg` to
  `session.Resize`.
- **Reserve exactly one escape key** to close the pane; everything else goes to
  the child. `ctrl+c` must reach the shell, so the escape key cannot be
  `ctrl+c`. Two workable choices: a leader chord (`ctrl+x` is already the
  leader and `s`/`t`/`m`/`e` are taken — `ctrl+x !` is free), or a tmux-style
  prefix owned by the pane. Note `ctrl+c` is currently intercepted
  unconditionally at `model.go:1892`; that branch needs a `stateTerminal`
  exemption.
- Cursor: `emu.CursorPosition()` + `emu.IsAltScreen()` are available if you
  want a real cursor inside the pane. The composite path returns a plain
  string, so a visible hardware cursor needs `tea.View.Cursor` plumbing — a
  reverse-video block cell is the cheap first version.

**4. Extension API** (`internal/extensions/api.go` + `symbols.go`) — concrete
structs only, function fields, no interfaces:

```go
type TerminalConfig struct {
    ID       string   // stable key; reopening the same ID reattaches
    Command  []string // empty → $SHELL, then /bin/sh
    Env      []string
    Cwd      string
    Title    string
    Width    int      // 0 = 80% of terminal
    Height   int      // 0 = 70% of terminal
    Anchor   OverlayAnchor
    Style    OverlayStyle
    OnExit   func(code int)
    OnTitle  func(string)
}

// Context additions — all non-blocking:
OpenTerminal  func(TerminalConfig) (*TerminalHandle, error)
ShowTerminal  func(id string) error
HideTerminal  func(id string) error
ToggleTerminal func(id string) error
CloseTerminal func(id string) error
ListTerminals func() []TerminalInfo   // ID, Title, Running, Visible

// TerminalHandle: struct with unexported state + methods, like SubagentHandle
// (internal/extensions/subagent.go:134). Show/Hide/Toggle/Write/Resize/
// Close/IsRunning/Done.
```

Sessions live in a `map[string]*terminal.Session` on the `Runner` guarded by
`sync.RWMutex` — the same pattern as widgets/header/footer/editor. Mutators
call `go appInstance.NotifyTerminalUpdate()` (never `prog.Send()` from inside
`Update()`).

**5. The extension then becomes trivial:**

```go
api.RegisterCommand(ext.CommandDef{
    Name: "term", Description: "Toggle the pop-up terminal",
    Execute: func(args string, ctx ext.Context) (string, error) {
        if err := ctx.ToggleTerminal("popup"); err == nil { return "", nil }
        _, err := ctx.OpenTerminal(ext.TerminalConfig{
            ID: "popup", Cwd: ctx.CWD, Anchor: ext.OverlayCenter,
        })
        return "", err
    },
})
api.RegisterShortcut(ext.ShortcutDef{Key: "ctrl+alt+t", Description: "Toggle terminal"},
    func(ctx ext.Context) { ctx.ToggleTerminal("popup") })
```

#### Effort and risk

| Piece | Effort | Risk |
|---|---|---|
| `internal/terminal` (PTY + emulator + damage coalescing) | ~250 LOC | Low — `internal/daemon` is a working precedent |
| `stateTerminal` + pane render + composite | ~200 LOC | Low — mirrors `stateOverlay` |
| Key/mouse/paste/resize forwarding | ~150 LOC | **Medium** — the escape-key design and the `ctrl+c` exemption need care |
| Extension API + symbols + wiring | ~200 LOC | Low — mirrors widgets/subagent |
| `charmbracelet/x/vt` dependency | — | **Medium** — untagged (pseudo-version only), and its own `ultraviolet` pin is 5 months behind Kit's. Verified compatible today; pin it and add a build test. |
| Windows | — | **Medium** — `creack/pty` is thin there. `charmbracelet/x/xpty` (already in the module graph via go.sum) wraps ConPTY and is the drop-in fix. |

Total: roughly 800 LOC plus tests, one new direct dependency.

### Option D — External emulator window (5 minutes, different feature)

```go
term := firstNonEmpty(os.Getenv("KIT_TERMINAL"), os.Getenv("TERMINAL"))
exec.Command(term, "-e", os.Getenv("SHELL")).Start()  // detached, not waited on
```

Zero integration cost, and it is genuinely what some users mean by "my
pre-configured terminal". But the `-e`/`--` flag differs per emulator
(kitty/alacritty/wezterm/gnome-terminal all disagree), it needs a display
server, and it does nothing on a remote SSH session. Worth adding as a
one-line fallback inside whichever option you pick, not as the feature.

---

## 4. Recommendation

**Ship Option A now, as an extension. Build Option C if the pop-up form factor
is a hard requirement.**

Rationale:

1. Option A already satisfies three of the four stated requirements — extension,
   custom command, toggle, and *state persists until the Kit session closes* —
   at ~40 lines and zero core risk. It fails only "pop-up": it is full-screen.
2. Option A's fidelity is unbeatable, because it hands over the real terminal.
   Any in-TUI pane is a re-implementation that will be subtly worse (cursor
   shape, mouse reporting, OSC 52 clipboard, sixel/kitty graphics, `TERM`
   capability mismatches).
3. Option C is the only path to a genuine pop-up, and it is now clearly
   *tractable* rather than speculative: `creack/pty` is already a direct
   dependency, `compositeAnchored` already does cell-level pane merging, the
   `stateOverlay` modal pattern is already there to copy, and
   `charmbracelet/x/vt` supplies the one missing piece and is verified to
   compile against Kit's ultraviolet pin.
4. Option C **must** live in core. The `unsafe`-less Yaegi sandbox makes a
   PTY unreachable from extension code, so "via an extension" can only mean the
   extension drives a core-hosted pane. Plan the API accordingly.
5. Skip Option B for this feature. Reconsider it separately if the real want is
   an inline command console rather than a terminal.

### Suggested sequencing

1. **Done** — `examples/extensions/popup-terminal.go` implements Option A with
   tmux/abduco/dtach detection and a plain-`$SHELL` fallback. Verified
   end-to-end: nested-tmux attach, detach, reattach with cwd and scrollback
   intact, `/term kill`, shutdown cleanup, and the `kill-on-exit` opt-out.
2. **Next, if wanted** — land `internal/terminal` + `stateTerminal` behind an
   experiment flag. Test with tmux and hardware-cursor apps (`vim`, `htop`,
   `less`) inside the pane.
3. **Then** — expose `OpenTerminal`/`ToggleTerminal` to extensions and rewrite
   the example against it, keeping the Option A path as the fallback when the
   pane is disabled.

### Independent smaller win

Regardless of the choice above, **overlays should gain a `Render func(width,
height int) string` field and an update/close channel.** Widgets have
`Render` + `RefreshHz`; overlays are frozen at creation and can only be
refreshed by the user re-triggering an action button (see the workaround in
`examples/extensions/subagent-monitor.go:685-734`). Closing that gap is a small
change that unblocks live dashboards, progress modals, and Option C's pane
alike. `overlayDialog` already has an unexported per-width `body func(width
int) string` hook (`overlay.go:105`) used by the message inspector — it just is
not reachable from `OverlayConfig`.

---

## 6. Postscript: a Yaegi bug found while building Option A

Implementing the extension surfaced a **silent Yaegi miscompilation**. In a
tagless switch, `switch { case a, b, c: }` evaluates **only the first
expression** and ignores the rest — no load error, no panic. A switch with a
tag (`switch n { case 1, 2, 3: }`) is unaffected.

An AST scan of all extension files found two live victims:

- `popup-terminal.go` — the tmux session-name sanitizer dropped every digit and
  uppercase letter, so a session ID like `01JABCDEF` sanitized to the empty
  string and silently fell back to the pid.
- `status-footer.go:178` — `dispWidth` counted only emoji as double-width and
  silently ignored its six other ranges, so CJK, hangul and fullwidth
  characters measured 1 column instead of 2 — precisely the footer-wrap bug
  the function was written to prevent.

Both are fixed by joining conditions with `||` into a single case expression.
Documented in `AGENTS.md` and the `kit-extensions` skill, with a regression
test at `pkg/extensions/test/tagless_switch_yaegi_test.go` that asserts the
broken behaviour and fails loudly (with removal instructions) if Yaegi is ever
fixed.

---

## 7. Key file references

- `internal/ui/composite.go:30-94` — `compositeOverlay` / `compositeAnchored`
- `internal/ui/overlay.go:35-197, 208-336` — modal dialog, key handling
- `internal/ui/model.go:1671-1696` — modal early-return routing
- `internal/ui/model.go:1886-2265` — key dispatch order; interceptor at `2213`
- `internal/ui/model.go:3352-3356` — overlay composite in `View()`
- `internal/ui/model.go:5201-5302` — `distributeHeight` (widget measurement)
- `internal/ui/model.go:6587-6673` — `updateOverlayState`, `resolveOverlay`
- `internal/app/app.go:1440-1485` — `NotifyWidgetUpdate` coalescing
- `internal/app/app.go:1596-1620` — `SuspendTUI`
- `internal/daemon/server.go:1017-1047` — `pty.StartWithSize` precedent
- `internal/daemon/server.go:535-545` — PTY resize / minimum-size logic
- `internal/extensions/api.go:1710-1790` — `WidgetContent` / `WidgetConfig`
- `internal/extensions/api.go:1919-2004` — overlay types
- `internal/extensions/subagent.go:131-190` — `SubagentHandle` (handle pattern)
- `internal/extensions/loader.go:516-527` — Yaegi symbol sets
- `cmd/extension_context.go:95-148, 216-241` — notify + `ShowOverlay` + `SuspendTUI` wiring
- `examples/extensions/interactive-shell.go` — Option A prior art
- `examples/extensions/bad-apple.go:340` — 30 Hz widget rendering proof
