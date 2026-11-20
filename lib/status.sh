#!/usr/bin/env bash
# Report, without changing anything, what is out of sync between ~/.claude and the data repo.
#
#   jeeves-status   one line per tracked path that is not in sync, then the state of the data repo:
#
#     <state>  <path>  <disk>  <repo>  <base>     state: disk | repo | conflict (as jeeves-snapshot means them)
#     datarepo <absolute path of the data repo>
#     branch   <name>  clean | dirty | merging
#     main     even | ahead | behind | diverged | no-remote  <ahead>  <behind>
#
# Fields are tab-separated. disk, repo and base are git blob hashes ("-" = no file, "none" = never synced on
# this device). Read a version with: disk `cat ~/.claude/<path>`, repo `cat <data repo>/home/<path>`,
# base `git -C <data repo> cat-file -p <base>` (present when a commit ever held that version).
# main is compared with the local origin/main ref: run jeeves-snapshot first to fetch.
# The data repo is found as lib/lib.sh describes.

. "$(dirname "$0")/../lib/lib.sh"

[ $# = 0 ] || { echo "status: takes no options" >&2; exit 2; }
find_data_repo || { echo "jeeves status: $DATA_REPO_PROBLEM"; exit 1; }

while IFS= read -r p; do
  [ -n "$p" ] || continue
  d=$(hash_of "$CLAUDE_HOME/$p"); r=$(hash_of "$HOME_COPY/$p"); b=$(base_of "$p")
  state=$(classify "$d" "$r" "$b")
  [ "$state" = insync ] || printf '%s\t%s\t%s\t%s\t%s\n' "$state" "$p" "$d" "$r" "${b:-none}"
done < <(all_tracked)

branch=$(git_repo symbolic-ref --short -q HEAD)
if [ -f "$REPO/.git/MERGE_HEAD" ]; then tree=merging
elif [ -n "$(git_repo status --porcelain)" ]; then tree=dirty
else tree=clean
fi
printf 'datarepo\t%s\n' "$REPO"
printf 'branch\t%s\t%s\n' "${branch:-detached}" "$tree"

if git_repo rev-parse -q --verify refs/remotes/origin/main >/dev/null; then
  counts=$(git_repo rev-list --left-right --count main...origin/main)
  ahead=${counts%%[[:space:]]*}; behind=${counts##*[[:space:]]}
  if [ "$ahead" = 0 ] && [ "$behind" = 0 ]; then rel=even
  elif [ "$behind" = 0 ]; then rel=ahead
  elif [ "$ahead" = 0 ]; then rel=behind
  else rel=diverged
  fi
  printf 'main\t%s\t%s\t%s\n' "$rel" "$ahead" "$behind"
else
  printf 'main\tno-remote\t0\t0\n'
fi
