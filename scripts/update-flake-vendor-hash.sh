#!/usr/bin/env bash
# Refresh the go-modules hash (vendorHash) in flake.nix.
#
# The nix build fetches the module tree of the *pinned release* in a
# fixed-output derivation, and vendorHash pins that tree. It has to move
# whenever the pinned release ships different dependencies. Build with a fake
# hash, read the hash Nix reports as expected out of the failure, write it
# back, then prove the build goes through.
#
# The `nix-release-pin` job in .github/workflows/release.yml runs this after
# bumping the release pin; run it by hand when you bump the pin yourself:
#
#   scripts/bump-flake-release-pin.sh v1.2.3
#   scripts/update-flake-vendor-hash.sh
#
# Any extra arguments are passed through to `nix build`, for example to work
# against a local nixpkgs:
#
#   scripts/update-flake-vendor-hash.sh --override-input nixpkgs /path/to/nixpkgs
set -euo pipefail

FLAKE_NIX="$(cd "$(dirname "$0")/.." && pwd)/flake.nix"
FAKE_HASH="sha256-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="

if ! grep -qE 'vendorHash[[:space:]]*=' "$FLAKE_NIX"; then
  echo "error: no vendorHash line found in $FLAKE_NIX" >&2
  exit 1
fi

sed -i -E "s|(vendorHash[[:space:]]*=[[:space:]]*\")[^\"]*(\"[[:space:]]*;)|\1${FAKE_HASH}\2|" "$FLAKE_NIX"

log="$(nix build --no-link "$@" .#default 2>&1 || true)"
hash="$(grep -oE 'got:[[:space:]]+sha256-[A-Za-z0-9+/=]+' <<<"$log" | head -1 | awk '{print $2}')"
if [[ -z "$hash" ]]; then
  echo "error: could not read the expected module hash from the build output:" >&2
  echo "$log" | tail -20 >&2
  exit 1
fi

sed -i -E "s|(vendorHash[[:space:]]*=[[:space:]]*\")[^\"]*(\"[[:space:]]*;)|\1${hash}\2|" "$FLAKE_NIX"

# Prove the flake builds with the new hash before announcing it.
nix build --no-link "$@" .#default

echo "vendorHash is now $hash"
