#!/usr/bin/env bash
# Build the Go program for every platform Jeeves supports, into dist/, reproducibly: the same source and Go
# version give the same bytes on any machine (no cgo, no file paths, no VCS stamp, no build ID).
#
#   bash .claude/skills/release/build-go.sh            build all five, then print their sizes and SHA-256
#   bash .claude/skills/release/build-go.sh <os>/<arch>  build one (such as linux/amd64)
#   bash .claude/skills/release/build-go.sh --record   build all five and record their SHA-256 in
#                                                      go-builds.sha256 beside this script (after a change
#                                                      to the Go code or the Go version; commit it)
#   bash .claude/skills/release/build-go.sh --check    build all five; exit 1 unless they match the record
#                                                      (CI runs this on each OS: the same bytes everywhere)
#
# Needs the Go version go.mod names (GOTOOLCHAIN=local: never a toolchain download behind the build's back).

cd "$(git rev-parse --show-toplevel)" || exit 1
TARGETS="darwin/arm64 darwin/amd64 linux/amd64 linux/arm64 windows/amd64"
RECORD=.claude/skills/release/go-builds.sha256
mode=print
case "$1" in --record|--check) mode=${1#--} ;; "") ;; *) TARGETS=$* ;; esac
if command -v sha256sum >/dev/null 2>&1; then SHA256=(sha256sum); else SHA256=(shasum -a 256); fi

export GOTOOLCHAIN=local CGO_ENABLED=0 GOFLAGS=-mod=readonly
want=$(sed -n 's/^toolchain go//p' go.mod)
have=$(go env GOVERSION 2>/dev/null); have=${have#go}
[ "$have" = "$want" ] || { echo "build-go: go.mod wants Go $want; this machine has '${have:-none}'" >&2; exit 1; }

mkdir -p dist || exit 1
for t in $TARGETS; do
  os=${t%/*} arch=${t#*/}
  out=dist/jeeves-$os-$arch; [ "$os" = windows ] && out=$out.exe
  GOOS=$os GOARCH=$arch go build -trimpath -buildvcs=false -ldflags='-s -w -buildid=' -o "$out" ./cmd/jeeves ||
    { echo "build-go: $t failed" >&2; exit 1; }
done
names() { for t in $TARGETS; do f=jeeves-${t%/*}-${t#*/}; [ "${t%/*}" = windows ] && f=$f.exe; echo "$f"; done; }
# "<sha256>  <name>" per build (Git Bash's sha256sum marks a binary file " *<name>"; dropped).
hashes() { (cd dist && names | while read -r f; do "${SHA256[@]}" "$f"; done) | awk '{ sub(/^\*/, "", $2); print $1 "  " $2 }'; }
case $mode in
  record) hashes > "$RECORD" && echo "build-go: recorded $RECORD"; exit ;;
  check) if hashes | diff "$RECORD" -; then
           echo "build-go: all five builds match $RECORD"; exit 0
         fi
         echo "build-go: the builds differ from $RECORD (lines above: < recorded, > built here)" >&2; exit 1 ;;
esac
cd dist && for t in $TARGETS; do
  f=jeeves-${t%/*}-${t#*/}; [ "${t%/*}" = windows ] && f=$f.exe
  printf '%9s  %s\n' "$(wc -c < "$f" | tr -d ' ')" "$("${SHA256[@]}" "$f")"
done
