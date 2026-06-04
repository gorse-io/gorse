#!/usr/bin/env bash

set -euo pipefail

REPO="${GORSE_REPO:-gorse-io/gorse}"
VERSION="${GORSE_CLI_VERSION:-latest}"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
BINARY_NAME="${BINARY_NAME:-gorse-cli}"

log() {
  printf '%s\n' "$*"
}

fail() {
  printf 'error: %s\n' "$*" >&2
  exit 1
}

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || fail "required command not found: $1"
}

normalize_os() {
  case "$(uname -s)" in
    Linux)
      printf 'linux'
      ;;
    Darwin)
      printf 'darwin'
      ;;
    *)
      fail "unsupported operating system: $(uname -s)"
      ;;
  esac
}

normalize_arch() {
  case "$(uname -m)" in
    x86_64 | amd64)
      printf 'amd64'
      ;;
    arm64 | aarch64)
      printf 'arm64'
      ;;
    riscv64)
      printf 'riscv64'
      ;;
    *)
      fail "unsupported architecture: $(uname -m)"
      ;;
  esac
}

download() {
  url="$1"
  output="$2"
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$url" -o "$output"
  elif command -v wget >/dev/null 2>&1; then
    wget -q "$url" -O "$output"
  else
    fail "required command not found: curl or wget"
  fi
}

install_binary() {
  src="$1"
  dst="$2"

  if [ -w "$INSTALL_DIR" ] || { [ ! -e "$INSTALL_DIR" ] && [ -w "$(dirname "$INSTALL_DIR")" ]; }; then
    mkdir -p "$INSTALL_DIR"
    install -m 0755 "$src" "$dst"
  elif command -v sudo >/dev/null 2>&1; then
    sudo mkdir -p "$INSTALL_DIR"
    sudo install -m 0755 "$src" "$dst"
  else
    fail "install directory is not writable: $INSTALL_DIR"
  fi
}

main() {
  need_cmd uname

  os="$(normalize_os)"
  arch="$(normalize_arch)"

  case "${os}_${arch}" in
    linux_amd64 | linux_arm64 | linux_riscv64 | darwin_arm64)
      ;;
    darwin_amd64)
      fail "unsupported platform: darwin_amd64. Release builds currently include darwin_arm64 only."
      ;;
    *)
      fail "unsupported platform: ${os}_${arch}"
      ;;
  esac

  asset="gorse-cli_${os}_${arch}"
  if [ "$VERSION" = "latest" ]; then
    url="https://github.com/${REPO}/releases/latest/download/${asset}"
  else
    url="https://github.com/${REPO}/releases/download/${VERSION}/${asset}"
  fi

  tmp_dir="$(mktemp -d)"
  trap 'rm -rf "$tmp_dir"' EXIT
  tmp_file="${tmp_dir}/${BINARY_NAME}"

  log "Downloading ${asset} from ${REPO} (${VERSION})..."
  download "$url" "$tmp_file"
  chmod +x "$tmp_file"

  install_binary "$tmp_file" "${INSTALL_DIR}/${BINARY_NAME}"
  log "Installed ${BINARY_NAME} to ${INSTALL_DIR}/${BINARY_NAME}"
}

main "$@"
