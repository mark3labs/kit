#!/usr/bin/env bash
# Record a VHS tape into www/public/.
#
#   ./www/tapes/record.sh              # records every *.tape
#   ./www/tapes/record.sh cyberpunk    # records cyberpunk.tape only
#
# Requires: vhs, ttyd, ffmpeg, and a chromium/chrome binary.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
tape_dir="$repo_root/www/tapes"

# Build a fresh kit and put it first on PATH so tapes can just call `kit`.
echo "==> building kit"
(cd "$repo_root" && go build -o output/kit ./cmd/kit)
export PATH="$repo_root/output:$PATH"

# Tapes may run stateful commands on camera (`/theme`, `/model`), which Kit
# persists to ~/.config/kit/preferences.yml. Snapshot it and restore on exit
# so recording a demo never changes the operator's real settings.
prefs="${XDG_CONFIG_HOME:-$HOME/.config}/kit/preferences.yml"
prefs_backup=""
prefs_existed=0
if [ -f "$prefs" ]; then
  prefs_existed=1
  prefs_backup="$(mktemp)"
  cp "$prefs" "$prefs_backup"
fi
restore_prefs() {
  # Kit flushes preferences during shutdown, which can land after vhs has
  # already returned. Let the recorded processes die first, or the restore
  # gets silently overwritten by the demo's `/theme`.
  sleep 2
  if [ "$prefs_existed" -eq 1 ]; then
    cp "$prefs_backup" "$prefs"
    rm -f "$prefs_backup"
    echo "==> restored $prefs"
  else
    # There was no preferences file before; don't leave the pinned theme
    # behind on a machine that never had one.
    rm -f "$prefs"
    echo "==> removed $prefs (did not exist before)"
  fi
}
trap restore_prefs EXIT INT TERM

# Pin the starting theme to Kit's default so recordings don't inherit whoever
# is recording. Without this the TUI opens in the operator's persisted theme
# and every machine produces a differently-coloured video. Tapes that want a
# different look switch to it on camera with `/theme`.
mkdir -p "$(dirname "$prefs")"
printf 'theme: kitt\n' > "$prefs"

# ttyd: vhs shells out to it. On NixOS it isn't installed globally, and the
# nixpkgs build can't find libwebsockets' evlib_uv plugin unless that lib dir
# is on LD_LIBRARY_PATH (otherwise: "lws_create_context: failed to load
# evlib_uv" -> vhs reports ERR_CONNECTION_REFUSED).
if command -v ttyd >/dev/null 2>&1; then
  runner=(bash -c)
elif command -v nix-shell >/dev/null 2>&1; then
  echo "==> ttyd not on PATH, using nix-shell"
  lws_so="$(ls -d /nix/store/*-libwebsockets-*/lib/libwebsockets.so.20 2>/dev/null | head -1 || true)"
  if [ -n "$lws_so" ]; then
    export LD_LIBRARY_PATH="$(dirname "$lws_so"):${LD_LIBRARY_PATH:-}"
  fi
  runner=(nix-shell -p ttyd --run)
else
  echo "error: ttyd is required by vhs. Install it (brew/apt/nix install ttyd)." >&2
  exit 1
fi

tapes=()
if [ $# -gt 0 ]; then
  for t in "$@"; do tapes+=("$tape_dir/${t%.tape}.tape"); done
else
  while IFS= read -r t; do tapes+=("$t"); done < <(find "$tape_dir" -name '*.tape' | sort)
fi

for tape in "${tapes[@]}"; do
  echo "==> recording $(basename "$tape")"
  (cd "$tape_dir" && "${runner[@]}" "vhs '$tape'")
done

# Sanity check: tapes must exercise the binary just built, not an installed
# one. Shells that rebuild PATH from their own config (nushell, fish) will
# silently shadow it, which produces a demo of the wrong version.
if command -v kit >/dev/null 2>&1; then
  resolved="$(command -v kit)"
  if [ "$resolved" != "$repo_root/output/kit" ]; then
    echo "    note: 'kit' resolves to $resolved outside this script;"
    echo "          tapes must prepend ../../output to PATH themselves."
  fi
fi

echo "==> optimizing"
# Lossless only. `--lossy` and `--colors` visibly wreck terminal text (thin
# antialiased glyphs turn crunchy and neon accents band), and a demo GIF that
# looks bad defeats the point of recording one. -O3 alone still merges
# identical frames and cuts redundant pixels, which is most of the win on
# terminal captures. If a GIF is over budget, shorten the tape or shrink the
# dimensions rather than degrading the pixels.
budget_bytes=$((2 * 1024 * 1024))
for gif in "$repo_root"/www/public/*.gif; do
  [ -e "$gif" ] || continue
  if command -v gifsicle >/dev/null 2>&1; then
    gifsicle -O3 "$gif" -o "$gif.tmp" && mv "$gif.tmp" "$gif"
  fi
  size=$(stat -c%s "$gif" 2>/dev/null || echo 0)
  printf '    %s  %s\n' "$(du -h "$gif" | cut -f1)" "$(basename "$gif")"
  if [ "$size" -gt "$budget_bytes" ]; then
    echo "        over 2MB — prefer the .webm on the page, or trim the tape."
  fi
done
for vid in "$repo_root"/www/public/*.webm "$repo_root"/www/public/*.mp4; do
  [ -e "$vid" ] || continue
  printf '    %s  %s\n' "$(du -h "$vid" | cut -f1)" "$(basename "$vid")"
done
