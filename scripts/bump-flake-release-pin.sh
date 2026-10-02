#!/usr/bin/env bash
# Point the flake at a new kit release.
#
# A flake input cannot say "latest tag": `nix profile install
# github:mark3labs/kit` evaluates the flake on the default branch, and a bare
# `github:mark3labs/kit` input would resolve that branch (unreleased master)
# instead of a release. flake.nix therefore pins the `kit-release` input to a
# full tag, and the `nix-release-pin` job in .github/workflows/release.yml
# runs this script after every tagged release so the pin on the default branch
# always names the newest tag.
#
# The tag is written once and lands in two places. The markers sit at the END
# of their lines, and no other line may end with them:
#
#   url     = "github:mark3labs/kit/v0.99.0"; # nix:kit-release-tag
#   version = "0.99.0";                       # nix:kit-release-version
#
# The version line exists because a `flake = false` input reaches the flake
# outputs as a bare store path, so the tag inside the url is not visible there
# (see the comment in flake.nix).
#
# Usage: scripts/bump-flake-release-pin.sh v1.2.3
set -euo pipefail

TAG="${1:-}"
if [[ ! "$TAG" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "usage: $0 vX.Y.Z (got: '${TAG:-<none>}')" >&2
  exit 1
fi
VERSION="${TAG#v}"

FLAKE_NIX="$(cd "$(dirname "$0")/.." && pwd)/flake.nix"

# The pinned lines, matched on the marker at end of line. Plain `grep marker`
# would also hit the prose comments that mention the markers.
pin_url()     { grep -E 'url[[:space:]]*=.*# nix:kit-release-tag$' "$FLAKE_NIX" || true; }
pin_version() { grep -E 'version[[:space:]]*=.*# nix:kit-release-version$' "$FLAKE_NIX" || true; }

# The tag and version the pins name, empty when the pin line is missing.
# `|| true` keeps an unmatched grep from tripping pipefail before the caller
# can say which marker line is broken.
url_tag() {
  local line
  line="$(pin_url)"
  { grep -oE 'kit/v[0-9.]+' <<<"$line" | head -1 | cut -d/ -f2 || true; }
}
version_value() {
  local line
  line="$(pin_version)"
  { grep -oE '"v?[0-9.]+"' <<<"$line" | head -1 | tr -d '"' || true; }
}

check_single_pin() {
  if [[ -z "$1" || "$(printf '%s\n' "$1" | wc -l)" != "1" ]]; then
    echo "error: flake.nix must have exactly one line ending in $2" >&2
    exit 1
  fi
}

CURRENT_TAG="$(url_tag)"
CURRENT_VERSION="$(version_value)"
check_single_pin "$(pin_url)" '# nix:kit-release-tag'
check_single_pin "$(pin_version)" '# nix:kit-release-version'

# Sanity check before touching anything: the two pins must agree right now, or
# the file was edited by hand and a blind rewrite would hide the drift.
if [[ "${CURRENT_TAG#v}" != "$CURRENT_VERSION" ]]; then
  echo "error: flake.nix pins disagree: tag $CURRENT_TAG vs version $CURRENT_VERSION" >&2
  exit 1
fi

if [[ "$CURRENT_TAG" == "$TAG" ]]; then
  echo "flake already installs kit $TAG"
  exit 0
fi

sed -i -E \
  "s|(url[[:space:]]*=[[:space:]]*\"github:mark3labs/kit/)${CURRENT_TAG}(\"[[:space:]]*;[[:space:]]*)# nix:kit-release-tag$|\1${TAG}\2# nix:kit-release-tag|" \
  "$FLAKE_NIX"
sed -i -E \
  "s|(version[[:space:]]*=[[:space:]]*\")${CURRENT_VERSION}(\"[[:space:]]*;[[:space:]]*)# nix:kit-release-version$|\1${VERSION}\2# nix:kit-release-version|" \
  "$FLAKE_NIX"

# The rewritten file must still parse and must name the new tag; a broken pin
# breaks `nix profile install` for everyone, so fail loudly instead of pushing.
NEW_TAG="$(url_tag)"
NEW_VERSION="$(version_value)"
check_single_pin "$(pin_url)" '# nix:kit-release-tag'
check_single_pin "$(pin_version)" '# nix:kit-release-version'
if [[ "$NEW_TAG" != "$TAG" || "$NEW_VERSION" != "$VERSION" ]]; then
  echo "error: rewrite failed, flake.nix now pins tag ${NEW_TAG:-none} version ${NEW_VERSION:-none}" >&2
  exit 1
fi
if command -v nix > /dev/null 2>&1; then
  nix eval --file "$FLAKE_NIX" description > /dev/null \
    || { echo "error: flake.nix no longer parses after the bump" >&2; exit 1; }
fi

echo "flake now installs kit $TAG"
