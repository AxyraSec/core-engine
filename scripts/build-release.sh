#!/usr/bin/env bash
# Builds the release binaries for every supported platform into dist/,
# with LICENSE, NOTICE and checksums.txt. Used by `make dist` and release.yml.
#
#   scripts/build-release.sh <version>
set -euo pipefail
cd "$(dirname "$0")/.."

version=${1:?usage: scripts/build-release.sh <version>}
platforms=(
  linux/amd64 linux/arm64
  darwin/amd64 darwin/arm64
  windows/amd64 windows/arm64
)

rm -rf dist
mkdir -p dist

for p in "${platforms[@]}"; do
  os=${p%/*} arch=${p#*/}
  out="dist/engine_${os}_${arch}"
  [ "$os" = windows ] && out="$out.exe"
  CGO_ENABLED=0 GOOS=$os GOARCH=$arch go build -trimpath \
    -ldflags "-s -w -X main.version=${version}" -o "$out" ./cmd/engine
  echo "built $out"
done

cp LICENSE NOTICE dist/
(cd dist && sha256sum engine_* > checksums.txt)
echo "wrote dist/checksums.txt"
