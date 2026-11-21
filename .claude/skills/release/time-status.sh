#!/usr/bin/env bash
# Time jeeves-status, the Go build against the bash version, on a scratch ~/.claude and data repo with a
# given number of skill files (the Go spike's measurement; nothing real is read or written).
#
#   bash .claude/skills/release/time-status.sh [files (50)] [runs (20)]
#
# Prints the mean time per run of each, in milliseconds. Needs dist/ built for this machine
# (bash .claude/skills/release/build-go.sh <os>/<arch>).

root=$(cd "$(dirname "$0")/../../.." && pwd)
n=${1:-50} runs=${2:-20}
W=$(cd "$(mktemp -d "${TMPDIR:-/tmp}/jeeves-time.XXXXXX")" && pwd) || exit 1
trap 'rm -rf "${W:?}"' EXIT
export CLAUDE_HOME=$W/claude JEEVES_DATA=$W/repo JEEVES_HOST=timehost
unset CLAUDE_CONFIG_DIR CLAUDE_PLUGIN_OPTION_DATA_REPO
git init -q -b main "$W/repo" && mkdir -p "$W/repo/home" "$W/claude"
printf '.sync-state/\n' > "$W/repo/.gitignore"
i=0
while [ $i -lt "$n" ]; do
  mkdir -p "$W/claude/skills/s$i" "$W/repo/home/skills/s$i"
  echo "skill $i" > "$W/claude/skills/s$i/SKILL.md"; echo "skill $i" > "$W/repo/home/skills/s$i/SKILL.md"
  i=$((i + 1))
done
echo rules > "$W/claude/CLAUDE.md"; echo "rules, changed" > "$W/repo/home/CLAUDE.md"
git -C "$W/repo" add -A && git -C "$W/repo" -c user.name=t -c user.email=t@example.invalid commit -qm init

cmp -s <("$root/bin/jeeves-status" 2>&1) <(JEEVES_NO_GO=1 "$root/bin/jeeves-status" 2>&1) ||
  echo "time-status: the two versions' outputs differ" >&2
mean() { # mean <label> <env...>: runs jeeves-status $runs times
  local label=$1 t; shift
  TIMEFORMAT=%R
  t=$( { time (j=0; while [ $j -lt "$runs" ]; do env "$@" "$root/bin/jeeves-status" >/dev/null 2>&1; j=$((j + 1)); done); } 2>&1 )
  awk -v t="$t" -v r="$runs" -v l="$label" 'BEGIN { printf "%-5s %6.0f ms per run\n", l, t * 1000 / r }'
}
echo "$n skill files, $runs runs each, $(uname -s) $(uname -m):"
mean go JEEVES_NO_GO=0
mean bash JEEVES_NO_GO=1
