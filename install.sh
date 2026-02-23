#!/bin/sh
# Anvil installer — https://github.com/johnjansen/anvil
# Usage: curl -fsSL https://raw.githubusercontent.com/johnjansen/anvil/main/install.sh | sh
set -e

REPO="johnjansen/anvil"
INSTALL_DIR="${ANVIL_INSTALL_DIR:-/usr/local/bin}"
BINARY="anvil"

# Detect OS and architecture
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "$ARCH" in
    x86_64|amd64)  ARCH="amd64" ;;
    aarch64|arm64)  ARCH="arm64" ;;
    *)              echo "Unsupported architecture: $ARCH" >&2; exit 1 ;;
esac

case "$OS" in
    linux)  OS="linux" ;;
    darwin) OS="darwin" ;;
    *)      echo "Unsupported OS: $OS" >&2; exit 1 ;;
esac

# Get latest release tag
TAG=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/')

if [ -z "$TAG" ]; then
    echo "Error: could not determine latest release" >&2
    exit 1
fi

echo "Installing anvil ${TAG} (${OS}/${ARCH})..."

# Download binary
URL="https://github.com/${REPO}/releases/download/${TAG}/anvil-${OS}-${ARCH}"
TMPFILE="$(mktemp)"
trap 'rm -f "$TMPFILE"' EXIT

curl -fsSL -o "$TMPFILE" "$URL"
if [ $? -ne 0 ]; then
    echo "Error: download failed from ${URL}" >&2
    exit 1
fi

chmod +x "$TMPFILE"

# Install — try direct, fall back to sudo
if [ -w "$INSTALL_DIR" ]; then
    mv "$TMPFILE" "${INSTALL_DIR}/${BINARY}"
else
    echo "Need sudo to install to ${INSTALL_DIR}"
    sudo mv "$TMPFILE" "${INSTALL_DIR}/${BINARY}"
fi

echo "Installed anvil ${TAG} to ${INSTALL_DIR}/${BINARY}"
echo ""
echo "Get started:"
echo "  anvil serve &     # start the daemon"
echo "  anvil watch       # register current project"
echo "  anvil add -s '*/5 * * * *' 'Check for new issues'"
