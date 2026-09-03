# syntax=docker/dockerfile:1
#
# Official Kit sandbox image.
#
# A ready-to-run Linux userland for the Kit coding agent, designed to be
# registered as a workdir.dev custom image (`POST /v1/images` with
# `source.type = "oci"`). It pre-bakes the toolchain that the dev-sandbox
# integration otherwise installs at runtime on every boot:
#
#   * Go (compiler + toolchain) and the Kit CLI (`kit`)
#   * The three git-forge CLIs: gh (GitHub), glab (GitLab), tea (Gitea)
#   * uv (Python package manager) and bun (JS runtime + package manager)
#   * nix (single-user, flakes enabled) + direnv for flake/.envrc dev shells
#   * git + openssh-client for SSH-based clones
#   * the `127.0.0.1 localhost` /etc/hosts entry Kit's OAuth listener needs
#
# The base apt layer mirrors workdir's curated base image
# (deploy/images/base/Dockerfile in mv37-org/workdir). We deliberately do NOT
# COPY in workdir's `sandbox-guest-agent` / `sandbox-init`: workdir's custom
# image builder injects the (static musl) guest agent + init itself when it
# converts this OCI image to an ext4 rootfs.
#
# ---------------------------------------------------------------------------
# Layout / build-cache strategy
# ---------------------------------------------------------------------------
# Every third-party tool is fetched in its **own** stage and the resulting
# binary is COPYed into the final image. That buys three things:
#
#   1. Independent invalidation. Bumping GH_VERSION rebuilds only `fetch-gh`
#      (plus its one cheap COPY) — it no longer forces a re-download of glab,
#      tea, uv, bun, direnv and a full re-install of nix, as it did when every
#      tool lived in one linear RUN chain.
#   2. Parallelism. BuildKit schedules independent stages concurrently, so a
#      cold build fetches all the CLIs (and installs nix, and compiles the Go
#      tooling) at the same time instead of serially.
#   3. Small, clean layers. Tarballs, the Go module cache and the Go build
#      cache never exist in the final image's layers — they stay in throwaway
#      stages / cache mounts.
#
# Within the final stage, COPYs are ordered least-volatile -> most-volatile:
# nix and the pinned CLIs first, `kit` (rebuilt on every release) last, so a
# release bump only invalidates the final two layers.
#
# Build (linux/amd64 only — workdir runs x86_64 Firecracker, and the pinned
# SHA256 checksums below are amd64-specific):
#   docker buildx build --platform linux/amd64 -f deploy/sandbox/Dockerfile -t kit-sandbox .
#
# Pinned tool versions — override at build time with --build-arg. Each download
# is verified against the matching SHA256 so a tampered mirror/asset fails the
# build instead of shipping. When bumping a version, update its *_SHA256 too
# (the upstream checksums: go.dev/dl JSON, gh_<v>_checksums.txt,
# glab checksums.txt, tea-<v>-linux-amd64.sha256). The ARGs are declared inside
# the stage that consumes them so a bump can't invalidate unrelated stages.

ARG UBUNTU_VERSION=24.04

# Where the `kit` binary comes from:
#   proxy — `go install github.com/mark3labs/kit/cmd/kit@${KIT_VERSION}`
#           (default; verified through the Go checksum database, and the build
#           context is never even transferred)
#   local — build from this repo checkout (`--build-arg KIT_SOURCE=local`),
#           useful for testing an unreleased kit inside the sandbox image
ARG KIT_SOURCE=proxy

# ---------------------------------------------------------------------------
# base — apt userland shared by every stage below.
# ---------------------------------------------------------------------------
FROM --platform=linux/amd64 ubuntu:${UBUNTU_VERSION} AS base

ENV DEBIAN_FRONTEND=noninteractive

# Base userland — mirrors workdir's curated base image apt layer, plus
# openssh-client (SSH clones) and unzip/jq/sudo conveniences.
#
# The apt archive + package lists live on BuildKit cache mounts: they are
# reused across local rebuilds (no re-download when the package set changes)
# and, because a mount is not part of the layer, they add nothing to the image.
# docker-clean is removed first — it tells apt to discard the .debs we cache.
RUN --mount=type=cache,target=/var/cache/apt,sharing=locked \
    --mount=type=cache,target=/var/lib/apt/lists,sharing=locked \
    rm -f /etc/apt/apt.conf.d/docker-clean \
    && apt-get update && apt-get install -y --no-install-recommends \
      bash coreutils curl wget ca-certificates git openssh-client \
      socat iproute2 iputils-ping unzip jq sudo \
      python3 python3-pip python3-venv nodejs npm build-essential

# ---------------------------------------------------------------------------
# golang — Go toolchain in /usr/local/go. Base for every Go build stage *and*
# for the final image (the agent compiles Go code at runtime).
# ---------------------------------------------------------------------------
FROM base AS golang

ARG GO_VERSION=1.27.0
ARG GO_SHA256=675c26c449cbb18fc24b74650de1eabbae6e16f64326fd85a283fb3b58280685
# amd64 only (checksums here are amd64; FROM pins the platform).
ARG TARGETARCH=amd64

RUN curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-${TARGETARCH}.tar.gz" \
      -o /tmp/go.tgz \
    && echo "${GO_SHA256}  /tmp/go.tgz" | sha256sum -c - \
    && tar -C /usr/local -xzf /tmp/go.tgz \
    && rm /tmp/go.tgz

ENV PATH=/usr/local/go/bin:/usr/local/bin:/root/go/bin:$PATH
ENV GOPATH=/root/go
ENV GOTOOLCHAIN=auto

# ---------------------------------------------------------------------------
# gotools — Go developer tooling, compiled once and shipped as bare binaries.
#
# Installed with `go install` so each module + its dependencies are verified
# against the Go checksum database (sum.golang.org) during the build:
#   * gopls          — the Go language server (LSP)
#   * golangci-lint  — aggregate linter runner
#   * dlv            — Delve debugger
#   * staticcheck    — advanced static analysis
#   * goimports      — import management / formatting
#
# One RUN per tool so bumping one version doesn't recompile the ones above it,
# and the module/build caches are mounts (shared with the kit stage, never
# baked into a layer) instead of something we have to `go clean` away.
# ---------------------------------------------------------------------------
FROM golang AS gotools

ARG GOPLS_VERSION=v0.22.0
ARG GOLANGCI_LINT_VERSION=v2.12.2
ARG DELVE_VERSION=v1.27.0
ARG STATICCHECK_VERSION=2026.1
ARG GOIMPORTS_VERSION=latest

ENV GOBIN=/out/bin

RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/root/go/pkg/mod \
    go install "golang.org/x/tools/gopls@${GOPLS_VERSION}"
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/root/go/pkg/mod \
    go install "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@${GOLANGCI_LINT_VERSION}"
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/root/go/pkg/mod \
    go install "github.com/go-delve/delve/cmd/dlv@${DELVE_VERSION}"
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/root/go/pkg/mod \
    go install "honnef.co/go/tools/cmd/staticcheck@${STATICCHECK_VERSION}"
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/root/go/pkg/mod \
    go install "golang.org/x/tools/cmd/goimports@${GOIMPORTS_VERSION}"

# ---------------------------------------------------------------------------
# kit — the main payload, built two possible ways (see ARG KIT_SOURCE).
#
# Either way the version ldflag is injected (matching .goreleaser.yaml) so
# `kit --version` reports the build instead of "dev"; on release builds
# KIT_VERSION is the real tag.
# ---------------------------------------------------------------------------
FROM golang AS kit-proxy

ARG KIT_VERSION=latest
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/root/go/pkg/mod \
    GOBIN=/out/bin go install \
      -ldflags "-s -w -X main.version=${KIT_VERSION}" \
      "github.com/mark3labs/kit/cmd/kit@${KIT_VERSION}"

# Local-source build. go.mod/go.sum are copied (and downloaded) *before* the
# rest of the tree, so editing Go files re-links kit without re-resolving the
# module graph. Keep .dockerignore tight — everything it lets through busts
# this stage's cache.
FROM golang AS kit-local

ARG KIT_VERSION=dev
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/root/go/pkg/mod \
    go mod download
COPY . .
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/root/go/pkg/mod \
    go build -trimpath -ldflags "-s -w -X main.version=${KIT_VERSION}" \
      -o /out/bin/kit ./cmd/kit

# Selects kit-proxy or kit-local; the unselected stage is never built.
FROM kit-${KIT_SOURCE} AS kit

# ---------------------------------------------------------------------------
# Single-binary CLI fetch stages. Each is independent, so they download in
# parallel and a version bump invalidates exactly one of them.
# ---------------------------------------------------------------------------

# GitHub CLI (gh).
FROM base AS fetch-gh
ARG GH_VERSION=2.95.0
ARG GH_SHA256=25d1e4729e8808c9ed3d613e96ebd3f3e44446f2d368c89d878a71a36ddb3d8c
ARG TARGETARCH=amd64
RUN curl -fsSL "https://github.com/cli/cli/releases/download/v${GH_VERSION}/gh_${GH_VERSION}_linux_${TARGETARCH}.tar.gz" \
      -o /tmp/gh.tgz \
    && echo "${GH_SHA256}  /tmp/gh.tgz" | sha256sum -c - \
    && tar -C /tmp -xzf /tmp/gh.tgz \
    && install -D -m755 "/tmp/gh_${GH_VERSION}_linux_${TARGETARCH}/bin/gh" /out/bin/gh

# GitLab CLI (glab).
FROM base AS fetch-glab
ARG GLAB_VERSION=1.105.0
ARG GLAB_SHA256=d8c92c640d7adf84c9dd01d1621fca45471fb61454f025c2fa3046dfc52d0d2f
ARG TARGETARCH=amd64
RUN curl -fsSL "https://gitlab.com/gitlab-org/cli/-/releases/v${GLAB_VERSION}/downloads/glab_${GLAB_VERSION}_linux_${TARGETARCH}.tar.gz" \
      -o /tmp/glab.tgz \
    && echo "${GLAB_SHA256}  /tmp/glab.tgz" | sha256sum -c - \
    && mkdir -p /tmp/glab \
    && tar -C /tmp/glab -xzf /tmp/glab.tgz \
    && install -D -m755 /tmp/glab/bin/glab /out/bin/glab

# Gitea CLI (tea) — single static binary.
FROM base AS fetch-tea
ARG TEA_VERSION=0.14.1
ARG TEA_SHA256=3cf7c5d1c20808c9ba2efb9ac125cee10d969daf398e653ea2b33cde201ea317
ARG TARGETARCH=amd64
RUN curl -fsSL "https://dl.gitea.com/tea/${TEA_VERSION}/tea-${TEA_VERSION}-linux-${TARGETARCH}" \
      -o /tmp/tea \
    && echo "${TEA_SHA256}  /tmp/tea" | sha256sum -c - \
    && install -D -m755 /tmp/tea /out/bin/tea

# uv — Astral's fast Python package/installer manager (single static binary).
FROM base AS fetch-uv
ARG UV_VERSION=0.11.25
ARG UV_SHA256=1db18b5e76fa645a7f3865773139bdec8e2d46adbdbb35e7410b34fa8015ccd2
RUN curl -fsSL "https://github.com/astral-sh/uv/releases/download/${UV_VERSION}/uv-x86_64-unknown-linux-gnu.tar.gz" \
      -o /tmp/uv.tgz \
    && echo "${UV_SHA256}  /tmp/uv.tgz" | sha256sum -c - \
    && mkdir -p /tmp/uv \
    && tar -C /tmp/uv -xzf /tmp/uv.tgz \
    && install -D -m755 /tmp/uv/uv-x86_64-unknown-linux-gnu/uv /out/bin/uv \
    && install -D -m755 /tmp/uv/uv-x86_64-unknown-linux-gnu/uvx /out/bin/uvx

# bun — fast JavaScript runtime & package manager (single static binary).
FROM base AS fetch-bun
ARG BUN_VERSION=1.3.14
ARG BUN_SHA256=951ee2aee855f08595aeec6225226a298d3fea83a3dcd6465c09cbccdf7e848f
RUN curl -fsSL "https://github.com/oven-sh/bun/releases/download/bun-v${BUN_VERSION}/bun-linux-x64.zip" \
      -o /tmp/bun.zip \
    && echo "${BUN_SHA256}  /tmp/bun.zip" | sha256sum -c - \
    && unzip -q /tmp/bun.zip -d /tmp/bun \
    && install -D -m755 /tmp/bun/bun-linux-x64/bun /out/bin/bun

# direnv — per-directory environment loader (single static binary).
FROM base AS fetch-direnv
ARG DIRENV_VERSION=2.37.1
ARG DIRENV_SHA256=1f1b93dd6f38523fde26dfac96151ef9d31a374e3005cd3345fb93555ae0c9b5
ARG TARGETARCH=amd64
RUN curl -fsSL "https://github.com/direnv/direnv/releases/download/v${DIRENV_VERSION}/direnv.linux-${TARGETARCH}" \
      -o /tmp/direnv \
    && echo "${DIRENV_SHA256}  /tmp/direnv" | sha256sum -c - \
    && install -D -m755 /tmp/direnv /out/bin/direnv

# ---------------------------------------------------------------------------
# nix — single-user install (no daemon: the workdir microVM has no systemd to
# supervise nix-daemon, and the agent runs as root, so a single-user store
# under /nix needs no background process). The installer's bundled config
# defaults build-users-group to the (nonexistent) `nixbld` group, which aborts
# a single-user install; NIX_CONFIG clears it (and enables flakes) for the
# install-time nix-env calls.
#
# Built in its own stage and copied in as a plain /nix store: the store is
# self-contained and relocation-free, so this is equivalent to installing in
# place — but it decouples nix from the Go toolchain (a GO_VERSION bump no
# longer re-runs the slowest step in the build).
#
# Nix release tarball checksum:
#   releases.nixos.org/nix/nix-<v>/...-x86_64-linux.tar.xz.sha256
# ---------------------------------------------------------------------------
FROM base AS nix

ARG NIX_VERSION=2.31.2
ARG NIX_SHA256=d1f67c86eed016214864ba08bfb9529c307aea7e8fafb74853f96fcc3bfd8a60

RUN curl -fsSL "https://releases.nixos.org/nix/nix-${NIX_VERSION}/nix-${NIX_VERSION}-x86_64-linux.tar.xz" \
      -o /tmp/nix.tar.xz \
    && echo "${NIX_SHA256}  /tmp/nix.tar.xz" | sha256sum -c - \
    && mkdir -p /tmp/nixinstall \
    && tar -xJf /tmp/nix.tar.xz -C /tmp/nixinstall --strip-components=1 \
    && NIX_CONFIG="$(printf 'build-users-group =\nexperimental-features = nix-command flakes')" \
         /tmp/nixinstall/install --no-daemon --no-channel-add --yes \
    && rm -rf /tmp/nixinstall /tmp/nix.tar.xz

# ---------------------------------------------------------------------------
# Final image: Go toolchain + everything above, assembled least-volatile
# (rarely-bumped tools) to most-volatile (`kit`, rebuilt every release).
# ---------------------------------------------------------------------------
FROM golang AS final

# nix store + the flakes/new-CLI runtime config. Discovery must not depend on
# root's ~/.nix-profile: sandbox platforms launch processes with their own
# (often sanitized) PATH and may relocate $HOME, so a
# `PATH=/root/.nix-profile/bin` ENV alone is fragile. Instead we symlink the
# nix CLIs from the *shared* default profile into /usr/local/bin (already on
# the base PATH for every user and every shell type), so `nix` resolves
# regardless of PATH/HOME/login state. NOTE: this is still a single-user store
# owned by root — only root can build into it; multi-user (non-root) use would
# require a daemon install, which these no-systemd microVMs can't supervise.
COPY --from=nix /nix /nix
RUN mkdir -p /etc/nix \
    && printf 'experimental-features = nix-command flakes\nbuild-users-group =\n' > /etc/nix/nix.conf \
    && for b in /nix/var/nix/profiles/default/bin/*; do ln -sf "$b" /usr/local/bin/; done \
    && ln -sfn /nix/var/nix/profiles/per-user/root/profile /root/.nix-profile

# nix is found via the /usr/local/bin symlinks above (on the base PATH for all
# users). Point nix at the system CA bundle for TLS fetches: ENV covers the
# image default env; the profile.d hook re-exports it for login shells that
# reset the environment from /etc/profile.
ENV NIX_SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt
RUN printf 'export NIX_SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt\n' \
      > /etc/profile.d/nix-sandbox.sh

# Kit's OAuth callback listener binds localhost; without this entry it fails to
# create the listener and nil-panics on cleanup. Bake it in so the runtime
# doesn't have to patch /etc/hosts on every boot. Also create the default
# workspace the agent clones into, and whitelist it for direnv so the agent's
# .envrc files load without a manual `direnv allow`.
RUN grep -q localhost /etc/hosts || echo '127.0.0.1 localhost' >> /etc/hosts
RUN mkdir -p /workspace /root/.config/direnv \
    && printf '[whitelist]\nprefix = ["/workspace"]\n' > /root/.config/direnv/direnv.toml

# Pinned single-binary CLIs (one cheap layer each, in their own cache lanes).
COPY --from=fetch-gh     /out/bin/ /usr/local/bin/
COPY --from=fetch-glab   /out/bin/ /usr/local/bin/
COPY --from=fetch-tea    /out/bin/ /usr/local/bin/
COPY --from=fetch-uv     /out/bin/ /usr/local/bin/
COPY --from=fetch-bun    /out/bin/ /usr/local/bin/
COPY --from=fetch-direnv /out/bin/ /usr/local/bin/
RUN ln -sf /usr/local/bin/bun /usr/local/bin/bunx

# Go developer tooling (gopls, golangci-lint, dlv, staticcheck, goimports).
COPY --from=gotools /out/bin/ /usr/local/bin/

# The Kit CLI itself — last, because it is the one thing that changes on every
# release; nothing above it is invalidated by a KIT_VERSION bump.
COPY --from=kit /out/bin/kit /usr/local/bin/kit

WORKDIR /workspace

# Smoke-check that every CLI resolves on PATH at build time. No `|| true`:
# `kit` is the main payload, so a broken binary must fail the build. The
# `nix eval` also proves the copied-in store + /etc/nix config are usable.
RUN set -e; \
    go version; \
    gopls version; \
    golangci-lint --version; \
    dlv version; \
    staticcheck --version; \
    goimports -h 2>/dev/null || true; \
    kit --version; \
    gh --version; \
    glab --version; \
    tea --version; \
    uv --version; \
    bun --version; \
    bunx --version; \
    direnv version; \
    nix --version; \
    nix eval --expr '1 + 1' >/dev/null

CMD ["/bin/bash"]
