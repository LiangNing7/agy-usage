#!/usr/bin/env bash
set -euo pipefail

REPO="LiangNing7/agy-usage"
BIN_NAME="agy-usage"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64|amd64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *)
    echo "Unsupported architecture: $ARCH" >&2
    exit 1
    ;;
esac

mkdir -p "$INSTALL_DIR"

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

LATEST_TAG="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" 2>/dev/null | grep -m1 '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/' || true)"

if [ -n "$LATEST_TAG" ]; then
  VER="${LATEST_TAG#v}"
  URL="https://github.com/${REPO}/releases/download/${LATEST_TAG}/${BIN_NAME}_${VER}_${OS}_${ARCH}.tar.gz"
  echo "Downloading ${BIN_NAME} ${LATEST_TAG} (${OS}/${ARCH}) ..."
  if curl -fsSL "$URL" -o "$TMP_DIR/archive.tar.gz"; then
    tar -xzf "$TMP_DIR/archive.tar.gz" -C "$TMP_DIR"
    install -m 0755 "$TMP_DIR/$BIN_NAME" "$INSTALL_DIR/$BIN_NAME"
    echo "Installed to ${INSTALL_DIR}/${BIN_NAME}"
    exit 0
  fi
fi

if command -v go >/dev/null 2>&1; then
  echo "Building ${BIN_NAME} from source via go install ..."
  GOBIN="$INSTALL_DIR" CGO_ENABLED=0 go install "github.com/${REPO}/cmd/${BIN_NAME}@latest"
  echo "Installed to ${INSTALL_DIR}/${BIN_NAME}"
  exit 0
fi

echo "Failed to find a prebuilt release and 'go' is not installed." >&2
exit 1
