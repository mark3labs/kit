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
| **nix** | single-user install, flakes + new `nix` CLI enabled (for `nix develop` dev shells) |
| **direnv** | per-directory env loader; `/workspace` is whitelisted so `.envrc` loads without `direnv allow` |
| **git**, **openssh-client** | SSH-based clones |
| python3, node/npm, build-essential, jq, curl, … | general dev userland |

It also bakes the `127.0.0.1 localhost` `/etc/hosts` entry Kit's OAuth listener
requires, and creates `/workspace`.

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
`GH_VERSION`, `GLAB_VERSION`, `TEA_VERSION`, `DIRENV_VERSION`, `NIX_VERSION`).

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
