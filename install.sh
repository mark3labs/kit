#!/usr/bin/env bash
# Kit install script
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/mark3labs/kit/master/install.sh | bash
#   bash install.sh [--bin-dir <dir>] [--version <tag>] [--no-provider-check]
set -euo pipefail

# ── Colour helpers ──────────────────────────────────────────────────────────
if [ -t 1 ] && command -v tput &>/dev/null && tput colors &>/dev/null; then
  BOLD=$(tput bold); RESET=$(tput sgr0)
  RED=$(tput setaf 1); GREEN=$(tput setaf 2)
  YELLOW=$(tput setaf 3); CYAN=$(tput setaf 6)
else
  BOLD=""; RESET=""; RED=""; GREEN=""; YELLOW=""; CYAN=""
fi

info()    { printf "%s  %s%s\n"        "${CYAN}→${RESET}"  "$*" "${RESET}"; }
success() { printf "%s  %s%s\n"        "${GREEN}✓${RESET}" "$*" "${RESET}"; }
warn()    { printf "%s  %s%s\n"        "${YELLOW}⚠${RESET}" "$*" "${RESET}"; }
die()     { printf "%s  %s%s\n" >&2    "${RED}✗${RESET}"   "$*" "${RESET}"; exit 1; }
header()  { printf "\n%s%s%s\n\n"      "${BOLD}" "$*" "${RESET}"; }

# ── Defaults ────────────────────────────────────────────────────────────────
REPO="mark3labs/kit"
BIN_NAME="kit"
VERSION="${KIT_VERSION:-}"   # empty = latest release tag
BIN_DIR="${KIT_BIN_DIR:-}"   # empty = auto-detect
CHECK_PROVIDER=true

# ── Argument parsing ─────────────────────────────────────────────────────────
while [[ $# -gt 0 ]]; do
  case "$1" in
    --bin-dir)
      [[ $# -ge 2 ]] || die "--bin-dir needs a value"
      BIN_DIR="$2"; shift 2 ;;
    --version)
      [[ $# -ge 2 ]] || die "--version needs a value"
      VERSION="$2"; shift 2 ;;
    --no-provider-check) CHECK_PROVIDER=false; shift ;;
    -h|--help)
      cat <<EOF
Usage: install.sh [options]

Options:
  --bin-dir <dir>       Directory to install kit into  (default: auto-detect, ~/.local/bin)
  --version <tag>       Release tag to install         (default: latest)
  --no-provider-check   Skip the LLM provider API key reminder
  -h, --help            Show this help

Environment:
  KIT_BIN_DIR           Same as --bin-dir
  KIT_VERSION           Same as --version

EOF
      exit 0 ;;
    *) die "Unknown option: $1" ;;
  esac
done

# ── Platform detection ───────────────────────────────────────────────────────
detect_platform() {
  local os arch

  case "$(uname -s)" in
    Linux)  os="linux"  ;;
    Darwin) os="darwin" ;;
    MINGW*|MSYS*|CYGWIN*)
      die "Windows is not supported by this script. Use 'npm install -g @mark3labs/kit' or download the zip from https://github.com/${REPO}/releases" ;;
    *) die "Unsupported OS: $(uname -s)" ;;
  esac

  case "$(uname -m)" in
    x86_64|amd64)  arch="amd64" ;;
    aarch64|arm64) arch="arm64" ;;
    *) die "Unsupported architecture: $(uname -m)" ;;
  esac

  echo "${os}_${arch}"
}

# ── Resolve install directory ─────────────────────────────────────────────────
resolve_bin_dir() {
  if [[ -n "$BIN_DIR" ]]; then
    echo "$BIN_DIR"
    return
  fi

  # Prefer a writable directory that already exists
  local candidates=("$HOME/.local/bin" "$HOME/bin" "/usr/local/bin")
  for dir in "${candidates[@]}"; do
    if [[ -d "$dir" && -w "$dir" ]]; then
      echo "$dir"
      return
    fi
  done

  # Fall back to ~/.local/bin and create it
  echo "$HOME/.local/bin"
}

# ── Fetch helper (stdout) ─────────────────────────────────────────────────────
fetch() {
  local url="$1"
  if command -v curl &>/dev/null; then
    curl -fsSL "$url"
  elif command -v wget &>/dev/null; then
    wget -qO- "$url"
  else
    die "Neither curl nor wget is available. Install one and retry."
  fi
}

# ── Fetch latest tag from GitHub ──────────────────────────────────────────────
latest_version() {
  local tag
  tag=$(fetch "https://api.github.com/repos/${REPO}/releases/latest" \
        | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": *"\(.*\)".*/\1/') || true

  [[ -n "$tag" ]] || die "Could not determine latest release. Use --version to pin one."
  echo "$tag"
}

# ── Download helper ───────────────────────────────────────────────────────────
download() {
  local url="$1" dest="$2"
  if command -v curl &>/dev/null; then
    curl -fsSL "$url" -o "$dest"
  elif command -v wget &>/dev/null; then
    wget -q "$url" -O "$dest"
  else
    die "Neither curl nor wget is available. Install one and retry."
  fi
}

# ── Checksum helper ──────────────────────────────────────────────────────────
verify_checksum() {
  local file="$1" asset="$2" version="$3" tmp_dir="$4"
  local checksums="${tmp_dir}/checksums.txt"
  local url="https://github.com/${REPO}/releases/download/${version}/checksums.txt"

  download "$url" "$checksums" 2>/dev/null || \
    die "Could not download checksums.txt for ${version}; refusing to install an unverified binary"

  local expected
  expected=$(awk -v asset="$asset" '$2 == asset { print $1; exit }' "$checksums")
  [[ -n "$expected" ]] || \
    die "No checksum found for ${asset}; refusing to install an unverified binary"

  local actual
  if command -v sha256sum &>/dev/null; then
    actual=$(sha256sum "$file" | awk '{print $1}')
  elif command -v shasum &>/dev/null; then
    actual=$(shasum -a 256 "$file" | awk '{print $1}')
  else
    die "SHA-256 verification requires sha256sum or shasum"
  fi

  [[ "$actual" == "$expected" ]] || die "Checksum mismatch for ${asset}"
  success "Verified checksum"
}

# ── Install via pre-built release binary ─────────────────────────────────────
TMP_DIR=""
cleanup() { [[ -n "$TMP_DIR" ]] && rm -rf "$TMP_DIR"; }
trap cleanup EXIT

install_from_release() {
  local version="$1" platform="$2" bin_dir="$3"

  # GoReleaser asset name pattern: kit_<version-without-v>_<os>_<arch>.tar.gz
  local asset="${BIN_NAME}_${version#v}_${platform}.tar.gz"
  local url="https://github.com/${REPO}/releases/download/${version}/${asset}"
  TMP_DIR=$(mktemp -d)

  info "Downloading ${asset} (${version})…"
  download "$url" "${TMP_DIR}/${asset}" 2>/dev/null || \
    die "Could not download release asset: ${url}"

  verify_checksum "${TMP_DIR}/${asset}" "$asset" "$version" "$TMP_DIR"

  command -v tar &>/dev/null || die "tar is required to extract ${asset}"
  tar -xzf "${TMP_DIR}/${asset}" -C "$TMP_DIR"

  local extracted="${TMP_DIR}/${BIN_NAME}"
  if [[ ! -f "$extracted" ]]; then
    extracted=$(find "$TMP_DIR" -type f -name "${BIN_NAME}" | head -n 1)
  fi
  [[ -n "$extracted" && -f "$extracted" ]] || die "Archive did not contain ${BIN_NAME}"

  mkdir -p "$bin_dir" || die "Could not create ${bin_dir}. Use --bin-dir to choose another directory."
  [[ -w "$bin_dir" ]] || die "${bin_dir} is not writable. Use --bin-dir or run with sudo."
  install -m 0755 "$extracted" "${bin_dir}/${BIN_NAME}"
  success "Installed ${BIN_NAME} → ${bin_dir}/${BIN_NAME}"
}

# ── PATH nudge ────────────────────────────────────────────────────────────────
check_path() {
  local bin_dir="$1"
  if ! echo "$PATH" | tr ':' '\n' | grep -qx "$bin_dir"; then
    echo ""
    warn "${bin_dir} is not on your \$PATH."
    echo "    Add this line to your shell rc file:"
    echo ""
    printf "    %sexport PATH=\"%s:\$PATH\"%s\n" "${BOLD}" "$bin_dir" "${RESET}"
    echo ""
    echo "    Then reload your shell:  source ~/.bashrc  (or ~/.zshrc)"
  fi
}

# ── Provider API key reminder ─────────────────────────────────────────────────
check_provider() {
  if [[ "$CHECK_PROVIDER" == false ]]; then return; fi

  local keys=(ANTHROPIC_API_KEY OPENAI_API_KEY GOOGLE_API_KEY GEMINI_API_KEY OPENROUTER_API_KEY)
  for key in "${keys[@]}"; do
    if [[ -n "${!key:-}" ]]; then
      success "${key} is set"
      return
    fi
  done

  echo ""
  warn "No LLM provider API key found."
  echo "    Export a key for your preferred provider, for example:"
  echo ""
  printf "    %sexport ANTHROPIC_API_KEY=\"sk-ant-...\"%s\n" "${BOLD}" "${RESET}"
  echo ""
  echo "    Or log in interactively:  kit auth login anthropic"
  echo "    See https://github.com/${REPO}#readme for all providers."
}

# ── Main ──────────────────────────────────────────────────────────────────────
main() {
  header "Installing Kit"

  local platform
  platform=$(detect_platform)
  info "Detected platform: ${platform}"

  local bin_dir
  bin_dir=$(resolve_bin_dir)

  if [[ -z "$VERSION" ]]; then
    info "Resolving latest release…"
    VERSION=$(latest_version)
  fi
  [[ "$VERSION" == v* ]] || VERSION="v${VERSION}"
  info "Version: ${VERSION}"

  install_from_release "$VERSION" "$platform" "$bin_dir"

  check_path "$bin_dir"
  check_provider

  echo ""
  success "Done!  Run ${BOLD}kit${RESET}${GREEN} to start.${RESET}"
  echo ""
}

main "$@"
