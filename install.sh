#!/usr/bin/env bash
set -euo pipefail

REPO="pavecraft/pave"
INSTALL_DIR="${PAVE_INSTALL_DIR:-$HOME/.local/bin}"
VERSION="${PAVE_VERSION:-}"

# --- detect OS ---
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$OS" in
  linux|darwin) ;;
  *) echo "Unsupported OS: $OS"; exit 1 ;;
esac

# --- detect architecture ---
ARCH=$(uname -m)
case "$ARCH" in
  x86_64)        ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

# --- resolve latest version from GitHub API ---
if [ -z "$VERSION" ]; then
  if command -v curl >/dev/null 2>&1; then
    VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
      | grep '"tag_name"' \
      | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/')
  elif command -v wget >/dev/null 2>&1; then
    VERSION=$(wget -qO- "https://api.github.com/repos/${REPO}/releases/latest" \
      | grep '"tag_name"' \
      | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/')
  else
    echo "curl or wget is required to download pave."
    exit 1
  fi
fi

if [ -z "$VERSION" ]; then
  echo "Could not determine the latest pave version. Set PAVE_VERSION to install a specific version."
  exit 1
fi

# GoReleaser strips the leading 'v' from the version in asset filenames.
VERSION_BARE="${VERSION#v}"
ASSET="pave_${VERSION_BARE}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/${VERSION}/${ASSET}"

echo "Installing pave ${VERSION} (${OS}/${ARCH}) → ${INSTALL_DIR}/pave"

# --- download & extract to temp dir ---
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

if command -v curl >/dev/null 2>&1; then
  curl -fsSL "$URL" | tar -xz -C "$TMP"
elif command -v wget >/dev/null 2>&1; then
  wget -qO- "$URL" | tar -xz -C "$TMP"
fi

if [ ! -f "$TMP/pave" ]; then
  echo "Binary not found in archive. The asset may not exist for ${OS}/${ARCH}."
  exit 1
fi

# --- remove any existing pave installation ---
EXISTING=$(command -v pave 2>/dev/null || true)
if [ -n "$EXISTING" ]; then
  echo "Found existing pave at ${EXISTING} — removing..."
  rm -f "$EXISTING"
fi
# Also clean the target dir in case it isn't on PATH yet.
[ -f "${INSTALL_DIR}/pave" ] && rm -f "${INSTALL_DIR}/pave"

# --- install ---
mkdir -p "$INSTALL_DIR"
mv "$TMP/pave" "$INSTALL_DIR/pave"
chmod +x "$INSTALL_DIR/pave"

# --- PATH hint ---
if ! echo ":${PATH}:" | grep -q ":${INSTALL_DIR}:"; then
  echo ""
  echo "  ${INSTALL_DIR} is not in your PATH. Add it by running:"
  echo ""
  echo '    export PATH="'"${INSTALL_DIR}"':$PATH"'
  echo ""
  echo "  To make it permanent, add the line above to ~/.bashrc, ~/.zshrc, or ~/.profile."
fi

# --- verify ---
echo ""
INSTALLED_VERSION=$("${INSTALL_DIR}/pave" --version 2>&1 || true)
echo "  ${INSTALLED_VERSION}"
echo ""
echo "Installation complete. Run: pave --help"
