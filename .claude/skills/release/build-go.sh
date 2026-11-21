#!/usr/bin/env bash
# Build the Go program for every platform Jeeves supports, reproducibly: the same source and Go version give
# the same bytes on any machine (no cgo, no file paths, no VCS stamp, no build ID). The builds in dist/ are
# committed with the release, and the signed MANIFEST holds their hashes.
#
#   bash .claude/skills/release/build-go.sh             build all five into dist/, then print their sizes and
#                                                       SHA-256 (after any change to the Go code or Go version)
#   bash .claude/skills/release/build-go.sh <os>/<arch>   build one into dist/ (such as linux/amd64)
#   bash .claude/skills/release/build-go.sh --check     build all five into a temp folder; exit 1 unless they
#                                                       match dist/ byte for byte (CI runs this on each OS)
#
# Needs the Go version go.mod names (GOTOOLCHAIN=local: never a toolchain download behind the build's back).

cd "$(git rev-parse --show-toplevel)" || exit 1
TARGETS="darwin/arm64 darwin/amd64 linux/amd64 linux/arm64 windows/amd64"
out=dist check=
case "$1" in
  --check) check=1 ;;
  "") ;;
  *) TARGETS=$* ;;
esac

export GOTOOLCHAIN=local CGO_ENABLED=0 GOFLAGS=-mod=readonly
want=$(sed -n 's/^toolchain go//p' go.mod)
have=$(go env GOVERSION 2>/dev/null); have=${have#go}
[ "$have" = "$want" ] || { echo "build-go: go.mod wants Go $want; this machine has '${have:-none}'" >&2; exit 1; }

if [ -n "$check" ]; then
  out=$(mktemp -d "${TMPDIR:-/tmp}/jeeves-build.XXXXXX") || exit 1
  trap 'rm -rf "${out:?}"' EXIT
fi
mkdir -p "$out" || exit 1
name() { local f=jeeves-${1%/*}-${1#*/}; [ "${1%/*}" = windows ] && f=$f.exe; printf '%s\n' "$f"; }
for t in $TARGETS; do
  GOOS=${t%/*} GOARCH=${t#*/} go build -trimpath -buildvcs=false -ldflags='-s -w -buildid=' \
    -o "$out/$(name "$t")" ./cmd/jeeves || { echo "build-go: $t failed" >&2; exit 1; }
done

if [ -n "$check" ]; then
  bad=0
  for t in $TARGETS; do
    f=$(name "$t")
    cmp -s "$out/$f" "dist/$f" || { echo "build-go: dist/$f differs from a build of this source" >&2; bad=1; }
  done
  [ $bad = 0 ] && echo "build-go: all five builds in dist/ match this source"
  exit $bad
fi
if command -v sha256sum >/dev/null 2>&1; then SHA256=(sha256sum); else SHA256=(shasum -a 256); fi
for t in $TARGETS; do
  f=dist/$(name "$t")
  printf '%9s  %s\n' "$(wc -c < "$f" | tr -d ' ')" "$("${SHA256[@]}" "$f" | cut -d' ' -f1)  $f"
done
