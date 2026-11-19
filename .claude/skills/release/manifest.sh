#!/usr/bin/env bash
# The release's MANIFEST: the SHA-256 of every file the release ships (every tracked file but MANIFEST and
# MANIFEST.sig), as jeeves-verify checks it. Run from the repo's top, on a clean release branch.
#
#   bash .claude/skills/release/manifest.sh          write MANIFEST
#   bash .claude/skills/release/manifest.sh --check  MANIFEST matches the tracked files and MANIFEST.sig is
#                                                    signed by release-key.pub (exit 1 otherwise)
#
# Then sign it (the release skill, step 6; the key has no passphrase, by the user's decision):
#   ssh-keygen -Y sign -f ~/.ssh/jeeves-release -n jeeves-release MANIFEST </dev/null

cd "$(git rev-parse --show-toplevel)" || exit 1
if command -v sha256sum >/dev/null 2>&1; then SHA256=(sha256sum); else SHA256=(shasum -a 256); fi
files() { git ls-files | grep -vxF -e MANIFEST -e MANIFEST.sig | LC_ALL=C sort; }

case "$1" in
  "")
    [ -z "$(git status --porcelain)" ] || { echo "manifest: commit everything first (the tree is not clean)" >&2; exit 1; }
    files | tr '\n' '\0' | xargs -0 "${SHA256[@]}" > MANIFEST || exit 1
    rm -f MANIFEST.sig
    echo "manifest: wrote MANIFEST ($(wc -l < MANIFEST | tr -d ' ') files); now sign it:"
    echo "  ssh-keygen -Y sign -f ~/.ssh/jeeves-release -n jeeves-release MANIFEST </dev/null" ;;
  --check)
    ok=1
    diff <(files | tr '\n' '\0' | xargs -0 "${SHA256[@]}") MANIFEST >/dev/null ||
      { echo "manifest: MANIFEST does not match the tracked files (write it again, then sign it again)"; ok=0; }
    key=$(awk '{ print $1, $2; exit }' release-key.pub 2>/dev/null)
    if [ -z "$key" ] || [ ! -f MANIFEST.sig ] ||
       ! ssh-keygen -Y verify -f <(printf 'jeeves-release namespaces="jeeves-release" %s\n' "$key") \
           -I jeeves-release -n jeeves-release -s MANIFEST.sig < MANIFEST >/dev/null 2>&1; then
      echo "manifest: MANIFEST.sig is missing or not signed by release-key.pub"; ok=0
    fi
    [ $ok = 1 ] && echo "manifest: MANIFEST matches the tracked files and is signed by release-key.pub"
    [ $ok = 1 ] ;;
  *) echo "manifest: unknown option $1" >&2; exit 2 ;;
esac
