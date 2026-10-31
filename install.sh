#!/usr/bin/env sh
# novram automated installation script for Linux and macOS
set -e

REPO="novram-project/novram"
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "$ARCH" in
    x86_64|amd64) ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

ASSET="novram-${OS}-${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/latest/download/${ASSET}"
DEST="/usr/local/bin/novram"

echo "==> Downloading novram for ${OS}/${ARCH}..."
TMP_DIR="$(mktemp -d)"
curl -fsSL "$URL" -o "${TMP_DIR}/${ASSET}"

echo "==> Extracting binary..."
tar -xzf "${TMP_DIR}/${ASSET}" -C "${TMP_DIR}"

echo "==> Installing novram to ${DEST}..."
sudo install -m 755 "${TMP_DIR}/novram-${OS}-${ARCH}" "$DEST" 2>/dev/null || sudo cp "${TMP_DIR}/novram-${OS}-${ARCH}" "$DEST"
rm -rf "$TMP_DIR"

echo "==> novram installed successfully!"
echo "==> Run 'novram doctor' to verify hardware configuration."
