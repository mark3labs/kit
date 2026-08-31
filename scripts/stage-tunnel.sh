#!/usr/bin/env bash
# Build the kit-tunnel sidecar for one Go platform and stage it for go:embed.
#
# Usage:
#   stage-tunnel.sh [goos goarch out-path]
#
# With no arguments, reads the target from $GOOS/$GOARCH (goreleaser build
# hooks run with these set) and stages internal/daemon/embedded/kit-tunnel-
# <goos>-<goarch>. With arguments, builds for the given platform and writes
# to the given path.
#
# Host builds use plain cargo. Cross builds use cargo-zigbuild (zig as the
# cross toolchain), which covers every release target from a Linux CI box.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
EMBED_DIR="$REPO_ROOT/internal/daemon/embedded"

# Forms:
#   stage-tunnel.sh <goos> <goarch> [out-path]   (explicit)
#   stage-tunnel.sh <goos> <goarch>              (stage for go:embed)
#   GOOS=... GOARCH=... stage-tunnel.sh          (env fallback)
GOOS="${1:-${GOOS:-}}"
GOARCH="${2:-${GOARCH:-}}"
if [ -z "$GOOS" ] || [ -z "$GOARCH" ]; then
    echo "stage-tunnel: pass <goos> <goarch> [out-path] or set GOOS/GOARCH" >&2
    exit 1
fi
OUT="${3:-$EMBED_DIR/kit-tunnel-$GOOS-$GOARCH}"

# Go platform -> Rust target triple.
case "$GOOS/$GOARCH" in
    linux/amd64)  TRIPLE="x86_64-unknown-linux-gnu" ;;
    linux/arm64)  TRIPLE="aarch64-unknown-linux-gnu" ;;
    darwin/amd64) TRIPLE="x86_64-apple-darwin" ;;
    darwin/arm64) TRIPLE="aarch64-apple-darwin" ;;
    windows/amd64) TRIPLE="x86_64-pc-windows-gnu" ;;
    *)
        echo "stage-tunnel: no Rust target mapped for $GOOS/$GOARCH" >&2
        exit 1
        ;;
esac

HOST_GOOS="$(go env GOOS)"
HOST_GOARCH="$(go env GOARCH)"

cd "$REPO_ROOT/contrib/kit-tunnel"

if [ "$GOOS" = "$HOST_GOOS" ] && [ "$GOARCH" = "$HOST_GOARCH" ]; then
    cargo build --release
    ARTIFACT="target/release/kit-tunnel"
else
    if ! command -v cargo-zigbuild >/dev/null 2>&1; then
        echo "stage-tunnel: cross-building $GOOS/$GOARCH needs cargo-zigbuild." >&2
        echo "  install it with:  cargo install cargo-zigbuild --locked" >&2
        echo "  (CI installs it via taiki-e/install-action)" >&2
        exit 1
    fi
    rustup target add "$TRIPLE" >/dev/null 2>&1 || true
    # zig cannot link Apple frameworks by itself; cargo-zigbuild picks them up
    # from a macOS SDK given by SDKROOT. Fail early with a clear message
    # instead of a linker error deep in the dependency graph.
    if [ "${TRIPLE##*-}" = "darwin" ] && [ "$(uname -s)" != "Darwin" ] && [ -z "${SDKROOT:-}" ]; then
        echo "stage-tunnel: cross-building $TRIPLE needs a macOS SDK." >&2
        echo "  export SDKROOT=/path/to/MacOSX<version>.sdk" >&2
        echo "  SDKs: https://github.com/joseluisq/macosx-sdks/releases" >&2
        echo "  (CI stages one in .github/workflows/release.yml)" >&2
        exit 1
    fi
    cargo zigbuild --release --target "$TRIPLE"
    ARTIFACT="target/$TRIPLE/release/kit-tunnel"
fi

[ "$GOOS" = "windows" ] && ARTIFACT="$ARTIFACT.exe"

mkdir -p "$(dirname "$OUT")"
cp "$ARTIFACT" "$OUT"
echo "stage-tunnel: $GOOS/$GOARCH ($TRIPLE) -> $OUT"
