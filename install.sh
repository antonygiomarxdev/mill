#!/bin/sh
set -eu

# install.sh — one-line mill installer
# Usage: curl -sSf https://raw.githubusercontent.com/antonygiomarxdev/mill/main/install.sh | sh

REPO="antonygiomarxdev/mill"
BASE_URL="https://github.com/${REPO}/releases/latest/download"

# Detect OS and architecture.
detect_platform() {
  os=$(uname -s | tr '[:upper:]' '[:lower:]')
  arch=$(uname -m)

  case "$arch" in
    x86_64|amd64) arch="amd64" ;;
    aarch64|arm64) arch="arm64" ;;
    *)
      echo "install.sh: unsupported architecture: $arch" >&2
      exit 1
      ;;
  esac

  case "$os" in
    linux|darwin) ;;
    *)
      echo "install.sh: unsupported OS: $os" >&2
      exit 1
      ;;
  esac

  echo "${os}-${arch}"
}

# Try to get latest version tag from GitHub API.
get_version() {
  if command -v curl >/dev/null 2>&1; then
    v=$(curl -sSf "https://api.github.com/repos/${REPO}/releases/latest" 2>/dev/null \
      | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    if [ -n "$v" ]; then
      echo "$v"
      return
    fi
  fi
  echo "latest"
}

main() {
  platform=$(detect_platform)
  version=$(get_version)
  os=$(echo "$platform" | cut -d- -f1)
  arch=$(echo "$platform" | cut -d- -f2)

  echo "mill installer: ${version} for ${os}/${arch}"

  TARBALL="mill_${version}_${os}_${arch}.tar.gz"
  DOWNLOAD_URL="${BASE_URL}/${TARBALL}"

  TMPDIR=$(mktemp -d)
  trap 'rm -rf "$TMPDIR"' EXIT

  echo "Downloading ${DOWNLOAD_URL} ..."
  curl -sSfL -o "${TMPDIR}/${TARBALL}" "$DOWNLOAD_URL"

  echo "Extracting ..."
  tar xzf "${TMPDIR}/${TARBALL}" -C "$TMPDIR"

  # Prefer /usr/local/bin; fall back to ~/.local/bin.
  if [ -w /usr/local/bin ]; then
    DEST="/usr/local/bin"
  elif [ -d "${HOME}/.local/bin" ]; then
    DEST="${HOME}/.local/bin"
  elif mkdir -p "${HOME}/.local/bin" 2>/dev/null; then
    DEST="${HOME}/.local/bin"
  else
    echo "install.sh: no writable install directory found" >&2
    exit 1
  fi

  install -m 755 "${TMPDIR}/mill" "${DEST}/mill"
  echo "mill installed to ${DEST}/mill"
}

main
