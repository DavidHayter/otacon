#!/bin/sh
# Otacon Installer
# Usage: curl -sSL https://raw.githubusercontent.com/merthan/otacon/main/scripts/install.sh | sh

set -e

REPO="merthan/otacon"
INSTALL_DIR="/usr/local/bin"
BINARY="otacon"

# Detect OS and arch
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
  x86_64)  ARCH="amd64" ;;
  aarch64) ARCH="arm64" ;;
  arm64)   ARCH="arm64" ;;
  *)       echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

echo "🛡️  Otacon Installer"
echo "   OS:   $OS"
echo "   Arch: $ARCH"
echo ""

# Get latest version
VERSION=$(curl -sSL "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
if [ -z "$VERSION" ]; then
  echo "❌ Failed to fetch latest version"
  exit 1
fi
echo "   Version: $VERSION"

# Download
URL="https://github.com/$REPO/releases/download/$VERSION/otacon_${OS}_${ARCH}.tar.gz"
echo "   Downloading: $URL"
echo ""

TMP_DIR=$(mktemp -d)
trap "rm -rf $TMP_DIR" EXIT

curl -sSL "$URL" -o "$TMP_DIR/otacon.tar.gz"
tar -xzf "$TMP_DIR/otacon.tar.gz" -C "$TMP_DIR"

# Install
if [ -w "$INSTALL_DIR" ]; then
  mv "$TMP_DIR/$BINARY" "$INSTALL_DIR/$BINARY"
else
  echo "   Installing to $INSTALL_DIR (requires sudo)..."
  sudo mv "$TMP_DIR/$BINARY" "$INSTALL_DIR/$BINARY"
fi

chmod +x "$INSTALL_DIR/$BINARY"

echo ""
echo "✅ Otacon $VERSION installed to $INSTALL_DIR/$BINARY"
echo ""
echo "   Quick start:"
echo "     otacon version        Check installation"
echo "     otacon scan            Full cluster scan"
echo "     otacon audit --explain Audit with remediation"
echo ""
echo "   Documentation: https://github.com/$REPO"
