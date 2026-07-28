# Demo recordings

Terminal demos for the docs site are recorded with
[**VHS**](https://github.com/charmbracelet/vhs) — Charm's "GIF as code" tool.
It drives a real terminal (via `ttyd`) from a scripted `.tape` file, so every
recording is deterministic, reviewable in a diff, and re-recordable when the
UI changes. No screen-capture software, no mouse, no retakes.

## Recording

```bash
./www/tapes/record.sh             # record every *.tape
./www/tapes/record.sh cyberpunk   # record cyberpunk.tape only
```

The script builds a fresh `kit` binary, puts it on `PATH`, runs VHS, and
writes the output into `www/public/`, then losslessly optimizes any GIFs with
`gifsicle` if it's installed.

> **Always record through `record.sh`, not `vhs <tape>` directly.**
> `cyberpunk.tape` runs `/theme` on camera, and Kit persists theme and model
> selections to `~/.config/kit/preferences.yml`. `record.sh` snapshots that
> file and restores it afterwards; calling `vhs` directly skips the snapshot
> and silently leaves your real theme changed.

## Requirements

| Tool | Purpose |
| --- | --- |
| `vhs` | tape runner |
| `ttyd` | headless terminal VHS drives |
| `ffmpeg` | frame encoding |
| chromium / chrome | headless renderer |

```bash
# macOS
brew install vhs ttyd ffmpeg
# Nix
nix profile install nixpkgs#vhs nixpkgs#ttyd nixpkgs#ffmpeg
# Go
go install github.com/charmbracelet/vhs@latest
```

> **NixOS note:** nixpkgs' `ttyd` fails with `lws_create_context: failed to
> load evlib_uv`, which VHS surfaces as `ERR_CONNECTION_REFUSED`. Fix by
> putting the libwebsockets lib dir on `LD_LIBRARY_PATH` — `record.sh` does
> this automatically.

## Writing a tape

Keep demos **short**. One idea per tape; make several small tapes rather than
one long one.

```tape
Output ../public/mydemo.gif
Set Shell "nu"
Set FontFamily "Operator-caskabold"
Set FontSize 16
Set Width 1200
Set Height 820
Set Padding 20
Set TypingSpeed 55ms
Set Framerate 24

Hide                       # setup not shown in the recording
Sleep 3s
Type "$env.PATH = ($env.PATH | prepend ('../../output' | path expand))" Enter
Type "cd demo" Enter
Type "clear" Enter
Show

Type "kit" Enter
Sleep 3s
Type "explain this repo" Enter
Sleep 12s
```

Useful directives: `Hide`/`Show` (trim setup), `Sleep`, `Set TypingSpeed`,
`Set PlaybackSpeed`, `Wait`, `Ctrl+C`, `Escape`, `Tab`, `Screenshot f.png`.
Run `vhs manual` for the full list.

### Tips

- **Hide the setup.** Wrap `cd`, `clear`, and prompt tweaks in `Hide` / `Show`
  so the recording starts clean.
- **Put configuration in `.kit.yml`, not flags.** Kit reads `.kit.yml` from the
  working directory, so model, system prompt and `no-session` can live there
  and the tape only has to type `kit`. See `demo/.kit.yml`.
- **Prepend the build to `PATH` inside the tape.** See the shell notes below —
  otherwise you may record an entirely different `kit`.
- **Pin anything inherited from local state** (model, theme, thinking level),
  or the same tape produces a different video on another machine.
- **Prefer `Wait` over `Sleep` for agent replies.** LLM latency varies; a fixed
  `Sleep` either truncates the answer or wastes seconds.
- **1200×820 @ FontSize 16** reads well on desktop and stays legible on mobile.

### Syncing on agent replies

A tape can't know how long a model will take. `Wait+Screen` blocks until the
terminal matches a regex, so the recording tracks the real reply:

```tape
Wait+Screen@60s /(?s)legacy code\?.*LINK SEVERED/
```

Two traps, both hit while building `cyberpunk.tape`:

- **A bare sentinel matches the previous reply.** Earlier output stays on
  screen, so `/LINK SEVERED/` matches instantly on exchange 2 and the tape
  races ahead. Anchor each `Wait` to its own question text.
- **Counting sentinels is unreliable.** `/(LINK SEVERED.*){3}/` only matches
  while all three are still visible; one long reply scrolls the first off the
  top and the tape hangs until timeout. This fails *intermittently*, so record
  a tape two or three times before trusting it.

The sentinel itself comes from the system prompt (`demo/netrunner.md`), which
ends every reply with a fixed line. Because it is never typed on camera, it
cannot self-match.

## Matching the local terminal

`cyberpunk.tape` reproduces the dev environment rather than VHS defaults, so
the recording looks like the terminal it was authored in.

| Setting | Value | Source |
| --- | --- | --- |
| `Set Shell` | `nu` | nushell + starship prompt |
| `Set FontFamily` | `Operator-caskabold` | `~/.config/ghostty/config` |
| `Set Theme` | synthwave (matches Kit's built-in theme) | `internal/ui/style/themes.go` |

VHS renders through headless Chromium, so **`Set FontFamily` must name a font
fontconfig can resolve** — check with `fc-list : family | grep -i <name>`. If
the name is wrong VHS silently falls back to a default monospace rather than
erroring. To confirm a font actually applied, record twice (once with a
deliberately bogus family name) and compare a frame:

```bash
ffmpeg -i cyberpunk.gif -vf "select=eq(n\,120)" -vframes 1 f.png -y
ffmpeg -i f.png -f rawvideo -pix_fmt rgb24 - | md5sum   # hashes must differ
```

The theme is inlined as JSON rather than named, so the terminal background
matches the palette Kit renders the TUI with. Keep the two in sync if
`internal/ui/style/themes.go` changes.

### Shell notes

VHS accepts any string for `Set Shell` and only fails at record time, so typos
are not caught by `vhs validate`. With `nu`:

- **There is no `PS1`.** Use `$env.PROMPT_INDICATOR` / `$env.PROMPT_COMMAND`.
- **Blank the right prompt** — `$env.PROMPT_COMMAND_RIGHT = {|| '' }` — it
  renders a clock, which makes every re-recording differ.
- **`export PATH=...` from `record.sh` does not survive.** nushell rebuilds
  `$env.PATH` from its own config, so `kit` resolves to whatever is installed
  system-wide. Two recordings were silently made against a stale v0.91.1
  binary before this was caught. Prepend it inside the tape:
  `$env.PATH = ($env.PATH | prepend ('../../output' | path expand))`
- **Allow ~3s of startup** in the `Hide` block; `env.nu` decrypts sops-nix
  secrets on launch, which is slower than a bash prompt.

### VHS parser quirks

- `Output` rejects absolute paths and names starting with `_`. Use a relative
  path, and `./name.gif` if it would otherwise start with punctuation.
- `vhs validate` does **not** catch a bad `Set Shell` or an unresolvable
  `Set FontFamily`; both fail (or silently fall back) only at record time.

## Embedding on the site

Static assets in `www/public/` are served from the site root:

```html
<img src="/cyberpunk.gif" alt="Kit demo" style="width: 100%; border-radius: 8px;" />
```

**Prefer video over GIF when the clip is longer than a few seconds.** A `.webm`
is typically smaller than the equivalent GIF at better quality. Emit both from
one tape (VHS supports multiple `Output` lines) and use the video with a GIF
fallback:

```html
<video autoplay loop muted playsinline
       style="width: 100%; border-radius: 8px;">
  <source src="/cyberpunk.webm" type="video/webm" />
  <img src="/cyberpunk.gif" alt="Kit demo" />
</video>
```

`autoplay loop muted playsinline` is what makes a video behave like a GIF —
`muted` and `playsinline` are both required for iOS to autoplay inline.

## Size budget

Aim for **under ~2 MB** for a hero GIF. `record.sh` runs `gifsicle -O3`, which
is **lossless** — it merges identical frames and drops redundant pixels
without touching the palette.

Resist the urge to add `--lossy` or `--colors`:

```bash
gifsicle -O3 --lossy=80 --colors 128 out.gif -o out.gif   # DON'T
```

On terminal captures that visibly wrecks the output — antialiased glyph edges
go crunchy and neon accents band into flat blocks. It's a bad trade for a file
whose entire job is to look good.

When a GIF is over budget, shrink the *content*, not the pixels:

- shorten the tape (fewer scenes, tighter `Sleep`s)
- lower `Set Framerate` (24 is plenty; VHS defaults to 50)
- reduce `Set Width` / `Set Height`
- or just ship the `.webm` and let the GIF be the fallback

Full-screen animation is worst-case for GIF: a full-bleed effect measured
**13 MB** where a mostly-static TUI clip of similar length was under 1 MB.
