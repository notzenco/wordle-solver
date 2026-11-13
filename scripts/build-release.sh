#!/usr/bin/env bash
# Cross-compiles release archives and checksums.txt into dist/.
# usage: scripts/build-release.sh VERSION   (for example v0.2.0)
set -euo pipefail

version=${1:?usage: scripts/build-release.sh VERSION}
targets="darwin/amd64 darwin/arm64 linux/amd64 linux/arm64 windows/amd64 windows/arm64"

rm -rf dist
mkdir -p dist
for target in $targets; do
  os=${target%/*}
  arch=${target#*/}
  name="wordle-solver_${version#v}_${os}_${arch}"
  stage="dist/$name"
  mkdir -p "$stage"
  exe=wordle-solver
  [ "$os" = windows ] && exe=wordle-solver.exe
  CGO_ENABLED=0 GOOS=$os GOARCH=$arch go build -trimpath \
    -ldflags "-s -w -X main.version=$version" -o "$stage/$exe" ./cmd/wordle-solver
  cp README.md LICENSE "$stage/"
  # wsolve is a shorter name for the same program. A symlink keeps
  # 'update' replacing the one real binary; zip can't hold one, so Windows
  # users add the alias themselves (see the README).
  [ "$os" = windows ] || ln -s wordle-solver "$stage/wsolve"
  if [ "$os" = windows ]; then
    (cd dist && zip -qr "$name.zip" "$name")
  else
    tar -C dist -czf "dist/$name.tar.gz" "$name"
  fi
  rm -rf "$stage"
done
(cd dist && sha256sum -- *.tar.gz *.zip > checksums.txt)
ls -l dist
