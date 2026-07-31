# Kit sandbox image

An official, pre-baked Linux image for running the [Kit](https://github.com/mark3labs/kit)
coding agent inside a [workdir.dev](https://workdir.dev) sandbox.

It exists so the dev-sandbox integration no longer has to `apt-get install` and
`go install` the toolchain on **every** sandbox boot — everything is baked in.

## What's inside

Based on `ubuntu:24.04`, mirroring workdir's curated base apt layer, plus:

| Tool | Purpose |
| --- | --- |
| **Go** (`/usr/local/go`) | toolchain for building/running Go code |
| **kit** | the Kit coding agent CLI (`go install github.com/mark3labs/kit/cmd/kit`) |
| **gh** | GitHub CLI — open PRs, manage repos |
| **glab** | GitLab CLI — open MRs |
| **tea** | Gitea CLI |
| **nix** | single-user install (root-owned store), flakes + new `nix` CLI enabled. Symlinked into `/usr/local/bin` so it's on the default PATH for every user/shell. See the note below. |
| **direnv** | per-directory env loader; `/workspace` is whitelisted so `.envrc` loads without `direnv allow` |
| **git**, **openssh-client** | SSH-based clones |
| python3, node/npm, build-essential, jq, curl, … | general dev userland |

It also bakes the `127.0.0.1 localhost` `/etc/hosts` entry Kit's OAuth listener
requires, and creates `/workspace`.

> **Nix is single-user / root-only.** The store lives at `/nix` owned by root
> with no `nix-daemon` (these microVMs have no systemd to supervise one). The
> `nix` CLIs are symlinked into `/usr/local/bin` so they resolve on the default
> PATH for **every** user and every shell type (non-login `bash -c`, login
> shells, relocated `$HOME`) — not just via root's `~/.nix-profile`. A non-root
> user can *find and run* `nix`, but **cannot build into the root-owned store**
> (nix chmods its own profile dirs and rejects other UIDs). If you need non-root
> builds, switch to a multi-user **daemon** install and start `nix-daemon` from
> the platform's init.

> **Note:** this image intentionally does **not** ship workdir's
> `sandbox-guest-agent` / `sandbox-init`. workdir's custom-image builder injects
> the (static musl) guest agent and init when it converts this OCI image to an
> ext4 rootfs.

## Build locally

```bash
# from the repo root
docker buildx build --platform linux/amd64 \
  -f deploy/sandbox/Dockerfile \
  -t kit-sandbox .
```

Override pinned versions with `--build-arg` (`GO_VERSION`, `KIT_VERSION`,
`GH_VERSION`, `GLAB_VERSION`, `TEA_VERSION`, `UV_VERSION`, `BUN_VERSION`,
`DIRENV_VERSION`, `NIX_VERSION`, plus `GOPLS_VERSION`, `GOLANGCI_LINT_VERSION`,
`DELVE_VERSION`, `STATICCHECK_VERSION`, `GOIMPORTS_VERSION`).

### Building an unreleased `kit`

By default `kit` is installed from the module proxy
(`go install github.com/mark3labs/kit/cmd/kit@${KIT_VERSION}`), so the build
never reads the build context at all. To bake in the *working tree* instead:

```bash
docker buildx build --platform linux/amd64 \
  -f deploy/sandbox/Dockerfile \
  --build-arg KIT_SOURCE=local \
  --build-arg KIT_VERSION=dev-$(git rev-parse --short HEAD) \
  -t kit-sandbox .
```

That stage copies `go.mod`/`go.sum` and runs `go mod download` before copying
the rest of the tree, so editing Go files re-links `kit` without re-resolving
the module graph. The root `.dockerignore` keeps the context small (~7 MB) —
anything it lets through becomes part of that stage's cache key.

## Build-cache layout

The Dockerfile is multi-stage on purpose. Every third-party tool is fetched in
its **own** stage and only the resulting binary is `COPY`ed into the final
image:

- **Independent invalidation.** Bumping `GH_VERSION` rebuilds `fetch-gh` and
  one cheap `COPY` — not glab/tea/uv/bun/direnv/nix, as it did when everything
  lived in a single linear `RUN` chain.
- **Volatility ordering.** `kit` is the last thing copied into the final stage,
  because it changes on every release. A `KIT_VERSION` bump rebuilds only the
  `kit` stage plus the final `COPY` + smoke test (~30 s instead of ~3 min).
- **Parallelism.** BuildKit schedules the independent stages concurrently, so a
  cold build downloads the CLIs, installs nix and compiles the Go tooling at
  the same time.
- **Smaller image.** Tarballs, the Go module cache and the Go build cache live
  in throwaway stages / `--mount=type=cache` mounts, so they never end up in a
  layer (~1.5 GB instead of ~2.9 GB).

Nix is installed in its own stage and the `/nix` store is copied in
(a single-user store is self-contained, so this is equivalent to installing in
place); the profile symlinks and `/etc/nix/nix.conf` are recreated in the final
stage. The build-time smoke test runs `nix eval` to prove the copied store is
usable.

CI keeps this cache across runs with `cache-from/cache-to: type=gha,mode=max`,
which exports every stage. Note that `--mount=type=cache` contents are *not*
exported to the GHA cache — they only speed up local rebuilds.

## CI / publishing

`.github/workflows/sandbox-image.yml` builds the image and pushes it to GHCR
(`ghcr.io/mark3labs/kit-sandbox`) on:

- pushes to `master` touching `deploy/sandbox/**`,
- published releases (the image is rebuilt against the released `kit` tag),
- manual `workflow_dispatch` (optionally pinning `kit_version`).

Tags published: `latest` (default branch), `sha-<short>`, branch name, and
`vX.Y.Z` / `vX.Y` on releases.

## Register it as a workdir custom image

Once published, register the OCI image with workdir
([API reference](https://github.com/mv37-org/workdir/blob/main/docs/API.md#images-spec-10-11)).
Pin an **immutable** tag (a release `vX.Y.Z` or a `sha-<short>` tag) rather than
`latest`, so the workdir image definition is reproducible:

```bash
curl -fsSL -X POST https://api.workdir.dev/v1/images \
  -H "Authorization: Bearer $WORKDIR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "source": { "type": "oci",
                "image_ref": "ghcr.io/mark3labs/kit-sandbox:v0.82.1" },
    "name": "custom/mark3labs/kit-sandbox",
    "resources_hint": { "cpu": 2, "memory_mb": 4096, "disk_gb": 16 }
  }'
```

The build is asynchronous (`202 Accepted`); poll `GET /v1/images/:id` for status
and `build_log`. Then create sandboxes against it:

```jsonc
POST /v1/sandboxes
{ "image": "custom/mark3labs/kit-sandbox",
  "image_version": "2026-06-10-ab12cd" }
```

> If the GHCR package is private, grant workdir's builder pull access (make the
> package public, or configure registry credentials on the workdir side).
