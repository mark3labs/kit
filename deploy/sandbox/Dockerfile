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
#   * git + openssh-client for SSH-based clones
#   * the `127.0.0.1 localhost` /etc/hosts entry Kit's OAuth listener needs
#
# The base apt layer mirrors workdir's curated base image
# (deploy/images/base/Dockerfile in mv37-org/workdir). We deliberately do NOT
# COPY in workdir's `sandbox-guest-agent` / `sandbox-init`: workdir's custom
# image builder injects the (static musl) guest agent + init itself when it
# converts this OCI image to an ext4 rootfs.
#
# Build (multi-arch capable; workdir runs x86_64 Firecracker):
#   docker buildx build --platform linux/amd64 -f deploy/sandbox/Dockerfile -t kit-sandbox .
#
FROM ubuntu:24.04

ENV DEBIAN_FRONTEND=noninteractive

# Pinned tool versions — override at build time with --build-arg.
ARG GO_VERSION=1.26.4
ARG KIT_VERSION=latest
ARG GH_VERSION=2.95.0
ARG GLAB_VERSION=1.105.0
ARG TEA_VERSION=0.14.1

# Resolved automatically by buildx (linux/amd64 -> amd64, linux/arm64 -> arm64).
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
    && kit --version || true

# GitHub CLI (gh).
RUN curl -fsSL "https://github.com/cli/cli/releases/download/v${GH_VERSION}/gh_${GH_VERSION}_linux_${TARGETARCH}.tar.gz" \
      -o /tmp/gh.tgz \
    && tar -C /tmp -xzf /tmp/gh.tgz \
    && install -m755 "/tmp/gh_${GH_VERSION}_linux_${TARGETARCH}/bin/gh" /usr/local/bin/gh \
    && rm -rf /tmp/gh.tgz "/tmp/gh_${GH_VERSION}_linux_${TARGETARCH}"

# GitLab CLI (glab).
RUN curl -fsSL "https://gitlab.com/gitlab-org/cli/-/releases/v${GLAB_VERSION}/downloads/glab_${GLAB_VERSION}_linux_${TARGETARCH}.tar.gz" \
      -o /tmp/glab.tgz \
    && mkdir -p /tmp/glab \
    && tar -C /tmp/glab -xzf /tmp/glab.tgz \
    && install -m755 /tmp/glab/bin/glab /usr/local/bin/glab \
    && rm -rf /tmp/glab.tgz /tmp/glab

# Gitea CLI (tea) — single static binary.
RUN curl -fsSL "https://dl.gitea.com/tea/${TEA_VERSION}/tea-${TEA_VERSION}-linux-${TARGETARCH}" \
      -o /usr/local/bin/tea \
    && chmod +x /usr/local/bin/tea

# Kit's OAuth callback listener binds localhost; without this entry it fails to
# create the listener and nil-panics on cleanup. Bake it in so the runtime
# doesn't have to patch /etc/hosts on every boot.
RUN grep -q localhost /etc/hosts || echo '127.0.0.1 localhost' >> /etc/hosts

# Default workspace the agent clones into.
RUN mkdir -p /workspace
WORKDIR /workspace

# Smoke-check that every CLI resolves on PATH at build time.
RUN set -e; \
    go version; \
    kit --version || true; \
    gh --version; \
    glab --version; \
    tea --version

CMD ["/bin/bash"]
