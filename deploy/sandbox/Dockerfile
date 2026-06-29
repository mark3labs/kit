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
# Build (linux/amd64 only — workdir runs x86_64 Firecracker, and the pinned
# SHA256 checksums below are amd64-specific):
#   docker buildx build --platform linux/amd64 -f deploy/sandbox/Dockerfile -t kit-sandbox .
#
FROM --platform=linux/amd64 ubuntu:24.04

ENV DEBIAN_FRONTEND=noninteractive

# Pinned tool versions — override at build time with --build-arg. Each download
# is verified against the matching SHA256 so a tampered mirror/asset fails the
# build instead of shipping. When bumping a version, update its *_SHA256 too
# (the upstream checksums: go.dev/dl JSON, gh_<v>_checksums.txt,
# glab checksums.txt, tea-<v>-linux-amd64.sha256).
ARG GO_VERSION=1.26.4
ARG GO_SHA256=1153d3d50e0ac764b447adfe05c2bcf08e889d42a02e0fe0259bd47f6733ad7f
ARG KIT_VERSION=latest
ARG GH_VERSION=2.95.0
ARG GH_SHA256=25d1e4729e8808c9ed3d613e96ebd3f3e44446f2d368c89d878a71a36ddb3d8c
ARG GLAB_VERSION=1.105.0
ARG GLAB_SHA256=d8c92c640d7adf84c9dd01d1621fca45471fb61454f025c2fa3046dfc52d0d2f
ARG TEA_VERSION=0.14.1
ARG TEA_SHA256=3cf7c5d1c20808c9ba2efb9ac125cee10d969daf398e653ea2b33cde201ea317
ARG UV_VERSION=0.11.25
ARG UV_SHA256=1db18b5e76fa645a7f3865773139bdec8e2d46adbdbb35e7410b34fa8015ccd2
ARG BUN_VERSION=1.3.14
ARG BUN_SHA256=951ee2aee855f08595aeec6225226a298d3fea83a3dcd6465c09cbccdf7e848f
ARG DIRENV_VERSION=2.37.1
ARG DIRENV_SHA256=1f1b93dd6f38523fde26dfac96151ef9d31a374e3005cd3345fb93555ae0c9b5
# Nix release tarball checksum: releases.nixos.org/nix/nix-<v>/...-x86_64-linux.tar.xz.sha256
ARG NIX_VERSION=2.31.2
ARG NIX_SHA256=d1f67c86eed016214864ba08bfb9529c307aea7e8fafb74853f96fcc3bfd8a60

# amd64 only (checksums above are amd64; FROM pins the platform).
ARG TARGETARCH=amd64

# Base userland — mirrors workdir's curated base image apt layer, plus
# openssh-client (SSH clones) and unzip/jq/sudo conveniences.
RUN apt-get update && apt-get install -y --no-install-recommends \
      bash coreutils curl wget ca-certificates git openssh-client \
      socat iproute2 iputils-ping unzip jq sudo \
      python3 python3-pip python3-venv nodejs npm build-essential \
    && rm -rf /var/lib/apt/lists/*

# Go toolchain -> /usr/local/go, on PATH for all later layers and at runtime.
RUN curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-${TARGETARCH}.tar.gz" \
      -o /tmp/go.tgz \
    && echo "${GO_SHA256}  /tmp/go.tgz" | sha256sum -c - \
    && tar -C /usr/local -xzf /tmp/go.tgz \
    && rm /tmp/go.tgz
ENV PATH=/usr/local/go/bin:/usr/local/bin:/root/go/bin:$PATH
ENV GOPATH=/root/go
ENV GOTOOLCHAIN=auto

# Kit CLI — compiled from source and installed onto PATH as `kit`. Inject the
# version ldflag (matching .goreleaser.yaml) so `kit --version` reports the
# build instead of "dev"; on release builds KIT_VERSION is the real tag.
RUN GOBIN=/usr/local/bin go install \
      -ldflags "-s -w -X main.version=${KIT_VERSION}" \
      "github.com/mark3labs/kit/cmd/kit@${KIT_VERSION}" \
    && kit --version

# GitHub CLI (gh).
RUN curl -fsSL "https://github.com/cli/cli/releases/download/v${GH_VERSION}/gh_${GH_VERSION}_linux_${TARGETARCH}.tar.gz" \
      -o /tmp/gh.tgz \
    && echo "${GH_SHA256}  /tmp/gh.tgz" | sha256sum -c - \
    && tar -C /tmp -xzf /tmp/gh.tgz \
    && install -m755 "/tmp/gh_${GH_VERSION}_linux_${TARGETARCH}/bin/gh" /usr/local/bin/gh \
    && rm -rf /tmp/gh.tgz "/tmp/gh_${GH_VERSION}_linux_${TARGETARCH}"

# GitLab CLI (glab).
RUN curl -fsSL "https://gitlab.com/gitlab-org/cli/-/releases/v${GLAB_VERSION}/downloads/glab_${GLAB_VERSION}_linux_${TARGETARCH}.tar.gz" \
      -o /tmp/glab.tgz \
    && echo "${GLAB_SHA256}  /tmp/glab.tgz" | sha256sum -c - \
    && mkdir -p /tmp/glab \
    && tar -C /tmp/glab -xzf /tmp/glab.tgz \
    && install -m755 /tmp/glab/bin/glab /usr/local/bin/glab \
    && rm -rf /tmp/glab.tgz /tmp/glab

# Gitea CLI (tea) — single static binary.
RUN curl -fsSL "https://dl.gitea.com/tea/${TEA_VERSION}/tea-${TEA_VERSION}-linux-${TARGETARCH}" \
      -o /tmp/tea \
    && echo "${TEA_SHA256}  /tmp/tea" | sha256sum -c - \
    && install -m755 /tmp/tea /usr/local/bin/tea \
    && rm -f /tmp/tea

# uv — Astral's fast Python package/installer manager (single static binary).
RUN curl -fsSL "https://github.com/astral-sh/uv/releases/download/${UV_VERSION}/uv-x86_64-unknown-linux-gnu.tar.gz" \
      -o /tmp/uv.tgz \
    && echo "${UV_SHA256}  /tmp/uv.tgz" | sha256sum -c - \
    && mkdir -p /tmp/uv \
    && tar -C /tmp/uv -xzf /tmp/uv.tgz \
    && install -m755 /tmp/uv/uv-x86_64-unknown-linux-gnu/uv /usr/local/bin/uv \
    && install -m755 /tmp/uv/uv-x86_64-unknown-linux-gnu/uvx /usr/local/bin/uvx \
    && rm -rf /tmp/uv.tgz /tmp/uv

# bun — fast JavaScript runtime & package manager (single static binary).
RUN curl -fsSL "https://github.com/oven-sh/bun/releases/download/bun-v${BUN_VERSION}/bun-linux-x64.zip" \
      -o /tmp/bun.zip \
    && echo "${BUN_SHA256}  /tmp/bun.zip" | sha256sum -c - \
    && unzip -q /tmp/bun.zip -d /tmp/bun \
    && install -m755 /tmp/bun/bun-linux-x64/bun /usr/local/bin/bun \
    && ln -sf /usr/local/bin/bun /usr/local/bin/bunx \
    && rm -rf /tmp/bun.zip /tmp/bun

# direnv — per-directory environment loader (single static binary). Whitelist
# /workspace so the agent's .envrc files load without a manual `direnv allow`.
RUN curl -fsSL "https://github.com/direnv/direnv/releases/download/v${DIRENV_VERSION}/direnv.linux-${TARGETARCH}" \
      -o /tmp/direnv \
    && echo "${DIRENV_SHA256}  /tmp/direnv" | sha256sum -c - \
    && install -m755 /tmp/direnv /usr/local/bin/direnv \
    && rm -f /tmp/direnv \
    && mkdir -p /root/.config/direnv \
    && printf '[whitelist]\nprefix = ["/workspace"]\n' > /root/.config/direnv/direnv.toml

# Nix — single-user install (no daemon: the workdir microVM has no systemd to
# supervise nix-daemon, and the agent runs as root, so a single-user store under
# /nix needs no background process). The installer's bundled config defaults
# build-users-group to the (nonexistent) `nixbld` group, which aborts a
# single-user install; NIX_CONFIG clears it (and enables flakes) for the
# install-time nix-env calls. We then persist the runtime config to
# /etc/nix/nix.conf so flakes + the new `nix` CLI are on for every invocation.
RUN curl -fsSL "https://releases.nixos.org/nix/nix-${NIX_VERSION}/nix-${NIX_VERSION}-x86_64-linux.tar.xz" \
      -o /tmp/nix.tar.xz \
    && echo "${NIX_SHA256}  /tmp/nix.tar.xz" | sha256sum -c - \
    && mkdir -p /tmp/nixinstall \
    && tar -xJf /tmp/nix.tar.xz -C /tmp/nixinstall --strip-components=1 \
    && NIX_CONFIG="$(printf 'build-users-group =\nexperimental-features = nix-command flakes')" \
         /tmp/nixinstall/install --no-daemon --no-channel-add --yes \
    && mkdir -p /etc/nix \
    && printf 'experimental-features = nix-command flakes\nbuild-users-group =\n' > /etc/nix/nix.conf \
    && rm -rf /tmp/nixinstall /tmp/nix.tar.xz

# Put the single-user nix profile on PATH and point nix at the system CA bundle.
# Kit's bash tool runs non-login `bash -c`, so nix's profile script (sourced from
# ~/.bash_profile) never runs — these ENVs replace what it would have exported.
ENV PATH=/root/.nix-profile/bin:$PATH
ENV NIX_SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt

# Kit's OAuth callback listener binds localhost; without this entry it fails to
# create the listener and nil-panics on cleanup. Bake it in so the runtime
# doesn't have to patch /etc/hosts on every boot.
RUN grep -q localhost /etc/hosts || echo '127.0.0.1 localhost' >> /etc/hosts

# Default workspace the agent clones into.
RUN mkdir -p /workspace
WORKDIR /workspace

# Smoke-check that every CLI resolves on PATH at build time. No `|| true`:
# `kit` is the main payload, so a broken binary must fail the build.
RUN set -e; \
    go version; \
    kit --version; \
    gh --version; \
    glab --version; \
    tea --version; \
    uv --version; \
    bun --version; \
    direnv version; \
    nix --version

CMD ["/bin/bash"]
