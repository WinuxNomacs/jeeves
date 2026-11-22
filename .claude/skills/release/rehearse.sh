#!/usr/bin/env bash
# Upgrade rehearsal for a release (release step 5): the last release's bin/ writes the state (a plain and an
# encrypted data repo, two devices, an edit left unsnapshotted), this checkout's bin/ takes it over, and a
# device still on the old release keeps syncing with it both ways. Needs age 1.3+.
#   bash .claude/skills/release/rehearse.sh <scratch folder> <last release tag, e.g. vold>
# Prints ok/FAIL per check and REHEARSAL OK at the end. Touches only the scratch folder (and a git worktree
# of the tag, removed at the end). The notice check expects plugin.json to still hold the old version
# (run it before the release branch's version bump) or names the next one. Exits 1 when a check failed.
# The scratch path must not hold spaces (a release before 2.0 runs JEEVES_ASKPASS unquoted).
[ $# = 2 ] && [ -n "$1" ] && [ -n "$2" ] || { echo "usage: rehearse.sh <scratch folder> <last release tag>" >&2; exit 2; }
case "$1" in *' '*) echo "rehearse.sh: the scratch path must not contain spaces" >&2; exit 2 ;; esac
mkdir -p "$1" && S=$(cd "$1" && pwd -P) || exit 2
TAG=$2 NEW=$(cd "$(dirname "$0")/../../.." && pwd -P)
W=$S/rehearse; rm -rf "${W:?}"; mkdir -p "$W"
git -C "$NEW" worktree add -q --detach "$W/old" "$TAG" || exit 1
OLD=$W/old
# The tag's worktree is registered in the real repo: remove it on every exit, an interrupted run included.
trap 'git -C "$NEW" worktree remove --force "$OLD" 2>/dev/null' EXIT
trap 'exit 130' INT TERM
export JEEVES_UPDATE_CHECK=off JEEVES_KEYSTORE=file PASS=rehearsal-pass-1
printf '#!/bin/sh\necho "%s"\n' "$PASS" > "$W/askpass"; chmod +x "$W/askpass"; export JEEVES_ASKPASS=$W/askpass
fail=0; ok() { echo "ok   $1"; }; bad() { echo "FAIL $1"; fail=1; }
chk() { local m=$1; shift; if "$@"; then ok "$m"; else bad "$m"; fi; }

for kind in plain crypt; do
  echo "== $kind"
  K=$W/$kind; mkdir -p "$K/claudeA/agents" "$K/claudeB"
  git init -q --bare -b main "$K/remote.git"
  echo "rules v1" > "$K/claudeA/CLAUDE.md"; echo "agent v1" > "$K/claudeA/agents/a.md"
  A() { JEEVES_HOST=hosta CLAUDE_HOME=$K/claudeA JEEVES_DATA=$K/repoA JEEVES_KEY_DIR=$K/keysA "$@"; }
  B() { JEEVES_HOST=hostb CLAUDE_HOME=$K/claudeB JEEVES_DATA=$K/repoB JEEVES_KEY_DIR=$K/keysB "$@"; }
  # --- old writes the state
  "$OLD/bin/jeeves-init" "$K/repoA" "$K/remote.git" >/dev/null
  A "$OLD/bin/jeeves-snapshot" >/dev/null
  if [ $kind = crypt ]; then A "$OLD/bin/jeeves-encrypt" on >/dev/null || bad "old encrypt on"; fi
  git -C "$K/repoA" push -q -u origin main 2>/dev/null
  git clone -q "$K/remote.git" "$K/repoB" 2>/dev/null
  if [ $kind = crypt ]; then B "$OLD/bin/jeeves-encrypt" unlock >/dev/null || bad "old unlock on B"; fi
  B "$OLD/bin/jeeves-snapshot" >/dev/null
  hb=$(B "$OLD/bin/jeeves-apply" --show 2>&1 | sed -n 's/^=== .*--accept \([^)]*\))$/\1/p')
  B "$OLD/bin/jeeves-apply" $(for x in $hb; do printf -- "--accept %s " "$x"; done) >/dev/null 2>&1
  chk "B (old) has A's agent (accepted)" grep -q "agent v1" "$K/claudeB/agents/a.md"
  chk "B (old) has A's rules" grep -q "rules v1" "$K/claudeB/CLAUDE.md"
  echo "rules v2 from old A" > "$K/claudeA/CLAUDE.md"; A "$OLD/bin/jeeves-snapshot" >/dev/null
  # work in progress during the upgrade: an edit not yet snapshotted
  echo "rules v3 in progress" > "$K/claudeA/CLAUDE.md"
  # --- upgrade A to the new bin
  out=$(A "$NEW/bin/jeeves-snapshot" --hook 2>&1); rc=$?
  chk "new hook on old state exits 0" [ $rc = 0 ]
  chk "new hook records the work in progress" A bash -c 'git -C "$1" cat-file --textconv main:home/CLAUDE.md | grep -q "v3 in progress"' _ "$K/repoA"
  st=$(A "$NEW/bin/jeeves-status" 2>&1)
  chk "new status: clean main" grep -q "^branch	main	clean" <<< "$st"
  # the Go build (2.0): runs on the old state with no fallback note, prints what the old release's bash
  # version prints on the same state, and points git's filter settings at this checkout's bin/
  chk "new status runs the Go build" bash -c '! grep -q "running the bash version" <<< "$1"' _ "$st"
  bst=$(A "$OLD/bin/jeeves-status" 2>&1)
  st=$(A "$NEW/bin/jeeves-status" 2>&1)
  chk "new status: the same output as the old release's on the old state" [ "$st" = "$bst" ]
  if [ $kind = crypt ]; then
    chk "new status: git's filter runs this checkout's jeeves-crypt" bash -c 'git -C "$1" config filter.jeeves.clean | grep -qF "$2"' _ "$K/repoA" "'$NEW/bin/jeeves-crypt' clean"
  fi
  # background updates (1.5.0): off on an upgraded device until the user decides; their hooks change nothing
  n=$(git -C "$K/repoA" rev-list --count main)
  chk "new: background updates offered, undecided" A bash -c '"$1/bin/jeeves-auto" --session-start | grep -q /jeeves:autoupdate' _ "$NEW"
  printf '{"session_id":"s1","transcript_path":"%s/projects/p/s1.jsonl","cwd":"%s"}' "$K/claudeA" "$K" | A "$NEW/bin/jeeves-auto" --hook-stop
  chk "new: the Stop hook does nothing while undecided" bash -c '[ ! -e "$1/jeeves/auto/log" ] && [ "$(git -C "$2" rev-list --count main)" = "$3" ]' _ "$K/claudeA" "$K/repoA" "$n"
  if [ $kind = crypt ]; then
    chk "new: encryption unlocked on the old key" grep -q "^encryption	unlocked" <<< "$(A "$NEW/bin/jeeves-encrypt" status)"
    chk "new: stored encrypted" bash -c '[ "$(git -C "$1" cat-file -p main:home/CLAUDE.md | head -c 21)" = age-encryption.org/v1 ]' _ "$K/repoA"
    chk "new scan passes" A "$NEW/bin/jeeves-scan" >/dev/null
  fi
  git -C "$K/repoA" push -q origin main 2>/dev/null || bad "push A"
  # --- B still on old takes the new version's commits
  B "$OLD/bin/jeeves-snapshot" >/dev/null 2>&1; B "$OLD/bin/jeeves-apply" >/dev/null 2>&1
  chk "B (old) gets new A's change" grep -q "v3 in progress" "$K/claudeB/CLAUDE.md"
  # --- B upgrades too, A on new edits again, B pulls
  echo "agent v2 from B" > "$K/claudeB/agents/a.md" 2>/dev/null || { mkdir -p "$K/claudeB/agents"; echo "agent v2 from B" > "$K/claudeB/agents/a.md"; }
  out=$(B "$NEW/bin/jeeves-snapshot" --hook 2>&1); chk "new B hook exit 0" [ $? = 0 ]
  git -C "$K/repoB" push -q origin main 2>/dev/null || bad "push B"
  A "$NEW/bin/jeeves-snapshot" >/dev/null 2>&1
  show=$(A "$NEW/bin/jeeves-apply" --show 2>&1)
  h=$(printf '%s\n' "$show" | sed -n 's/^=== .*--accept \([^)]*\))$/\1/p' | head -1)
  A "$NEW/bin/jeeves-apply" ${h:+--accept "$h"} >/dev/null 2>&1
  chk "new A gets B's agent (accepted)" grep -q "agent v2 from B" "$K/claudeA/agents/a.md"
  if [ $kind = crypt ]; then
    A "$NEW/bin/jeeves-encrypt" lock >/dev/null; chk "new lock" grep -q "^encryption	locked" <<< "$(A "$NEW/bin/jeeves-encrypt" status)"
    out=$(A "$NEW/bin/jeeves-snapshot" 2>&1); chk "locked message names /jeeves:encryption" grep -q "/jeeves:encryption" <<< "$out"
    A "$NEW/bin/jeeves-encrypt" unlock >/dev/null; chk "new unlock" grep -q "^encryption	unlocked" <<< "$(A "$NEW/bin/jeeves-encrypt" status)"
  fi
done
# update notice against a local tag repo
T=$W/tags; mkdir -p "$T"
ours=$(sed -n 's/.*"version": "\([0-9.]*\)".*/\1/p' "$NEW/.claude-plugin/plugin.json")
printf '{\n  "plugins": [\n    {\n      "source": {\n        "ref": "v%s"\n      }\n    }\n  ]\n}\n' "99.0.0" > "$T/marketplace.json"
out=$(JEEVES_UPDATE_CHECK= JEEVES_RELEASES_URL=$T/marketplace.json CLAUDE_HOME=$W/plain/claudeA JEEVES_HOST=hosta JEEVES_DATA=$W/plain/repoA "$NEW/bin/jeeves-snapshot" --hook 2>&1)
chk "the notice names a newer pinned release" grep -q "Jeeves 99.0.0 is out (this is $ours)" <<< "$out"
[ $fail = 0 ] && echo "REHEARSAL OK" || echo "REHEARSAL FAILED"
exit $fail
