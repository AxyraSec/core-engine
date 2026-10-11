#!/bin/sh
# Installs the Axyra engine on Linux and macOS.
#
#   curl -fsSL https://raw.githubusercontent.com/AxyraSec/core-engine/main/install.sh | sh
#   sh install.sh -v v0.1.1 -d /usr/local/bin
#
# Options (or environment variables):
#   -v VERSION   AXYRA_VERSION      release tag, default: latest release
#   -d DIR       AXYRA_INSTALL_DIR  install directory, default: /usr/local/bin if
#                                   writable, else ~/.local/bin
#
# The binary is checked against checksums.txt from the same release. If cosign
# is installed, the signature of checksums.txt is checked too.
# With the GitHub CLI logged in, private releases can be installed as well.
set -eu

REPO="AxyraSec/core-engine"
VERSION="${AXYRA_VERSION:-}"
INSTALL_DIR="${AXYRA_INSTALL_DIR:-}"
# Base URL of release downloads. Only meant for mirrors and tests.
DOWNLOAD_URL="${AXYRA_DOWNLOAD_URL:-https://github.com/$REPO/releases/download}"
# HTTPS only, unless a custom mirror is set.
PROTO="=https"
[ -n "${AXYRA_DOWNLOAD_URL:-}" ] && PROTO="=https,http"

say() { printf 'axyra: %s\n' "$*"; }
die() { printf 'axyra: error: %s\n' "$*" >&2; exit 1; }
has() { command -v "$1" >/dev/null 2>&1; }

while getopts "v:d:h" opt; do
  case $opt in
    v) VERSION=$OPTARG ;;
    d) INSTALL_DIR=$OPTARG ;;
    h) sed -n '2,16p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) exit 2 ;;
  esac
done

case "$(uname -s)" in
  Linux) os=linux ;;
  Darwin) os=darwin ;;
  MINGW* | MSYS* | CYGWIN*) die "on Windows, download engine_windows_amd64.exe from https://github.com/$REPO/releases" ;;
  *) die "unsupported OS: $(uname -s)" ;;
esac
case "$(uname -m)" in
  x86_64 | amd64) arch=amd64 ;;
  aarch64 | arm64) arch=arm64 ;;
  *) die "unsupported architecture: $(uname -m)" ;;
esac
asset="engine_${os}_${arch}"

use_gh=false
if [ -z "${AXYRA_DOWNLOAD_URL:-}" ] && has gh && gh auth status >/dev/null 2>&1; then
  use_gh=true
fi

fetch() { # fetch URL FILE
  if has curl; then
    curl -fsSL --proto "$PROTO" -o "$2" "$1"
  elif has wget; then
    wget -q -O "$2" "$1"
  else
    die "curl or wget is required"
  fi
}

if [ -z "$VERSION" ]; then
  if $use_gh; then
    VERSION=$(gh release view -R "$REPO" --json tagName -q .tagName) || die "cannot find the latest release"
  elif has curl; then
    VERSION=$(curl -fsSLI -o /dev/null -w '%{url_effective}' "https://github.com/$REPO/releases/latest" | sed 's#.*/##')
  fi
  case "$VERSION" in v*) ;; *) die "cannot find the latest release, use -v <version>" ;; esac
fi

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT INT TERM

say "downloading $asset $VERSION"
if $use_gh; then
  gh release download "$VERSION" -R "$REPO" -D "$tmp" \
    -p "$asset" -p checksums.txt -p checksums.txt.sigstore.json ||
    die "download failed"
else
  fetch "$DOWNLOAD_URL/$VERSION/$asset" "$tmp/$asset" || die "download failed: $DOWNLOAD_URL/$VERSION/$asset"
  fetch "$DOWNLOAD_URL/$VERSION/checksums.txt" "$tmp/checksums.txt" || die "cannot download checksums.txt"
  fetch "$DOWNLOAD_URL/$VERSION/checksums.txt.sigstore.json" "$tmp/checksums.txt.sigstore.json" 2>/dev/null || true
fi

if has cosign && [ -s "$tmp/checksums.txt.sigstore.json" ]; then
  cosign verify-blob "$tmp/checksums.txt" \
    --bundle "$tmp/checksums.txt.sigstore.json" \
    --certificate-identity-regexp "^https://github.com/$REPO/\.github/workflows/release\.yml@refs/tags/v" \
    --certificate-oidc-issuer https://token.actions.githubusercontent.com >/dev/null 2>&1 ||
    die "signature of checksums.txt is not valid"
  say "signature OK (cosign)"
else
  say "cosign not found: skipping the signature check"
fi

want=$(awk -v f="$asset" '$2 == f || $2 == "*"f { print $1 }' "$tmp/checksums.txt")
[ -n "$want" ] || die "$asset is not listed in checksums.txt"
if has sha256sum; then
  got=$(sha256sum "$tmp/$asset" | awk '{ print $1 }')
elif has shasum; then
  got=$(shasum -a 256 "$tmp/$asset" | awk '{ print $1 }')
else
  die "sha256sum or shasum is required"
fi
[ "$got" = "$want" ] || die "checksum mismatch for $asset"
say "checksum OK"

if [ -z "$INSTALL_DIR" ]; then
  if [ -w /usr/local/bin ]; then
    INSTALL_DIR=/usr/local/bin
  else
    INSTALL_DIR="$HOME/.local/bin"
  fi
fi
mkdir -p "$INSTALL_DIR"
chmod 0755 "$tmp/$asset"
mv "$tmp/$asset" "$INSTALL_DIR/engine"
say "installed $INSTALL_DIR/engine"

"$INSTALL_DIR/engine"
case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *) say "add $INSTALL_DIR to your PATH" ;;
esac
