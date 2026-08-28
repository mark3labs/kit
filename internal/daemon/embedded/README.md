# Embedded kit-tunnel sidecar

This directory holds platform-specific `kit-tunnel` binaries that are
compiled into the kit binary via `go:embed`, so a kit build always carries
its transport sidecar — no separate download needed.

- Naming: `kit-tunnel-<GOOS>-<GOARCH>` (Go naming, e.g. `kit-tunnel-linux-amd64`,
  `kit-tunnel-darwin-arm64`, `kit-tunnel-windows-amd64`).
- Staging: `task build` / `task tunnel` copies the cargo release build here
  for the current platform before compiling kit.
- The binaries are NOT committed (see `.gitignore`); a checkout without them
  still builds — kit just falls back to an external `kit-tunnel` on PATH.
- At runtime kit extracts the embedded binary to the user cache dir
  (`~/.cache/kit/tunnel/`), keyed by its SHA-256, and executes that copy.

This README exists so the `go:embed all:embedded` pattern always matches,
even on a fresh clone with no staged sidecar.
