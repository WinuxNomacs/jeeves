#!/usr/bin/env bash
# Remove old history from the data repo. The commits on main older than <days> days become one parentless
# commit holding the files as they were then; every later commit is recreated on top of it with the same
# files, authors, dates and messages; then main is force-pushed with a lease. No file changes.
#
#   jeeves-cleanup <days>            clean up (days: 1 or more), after a fresh snapshot
#   jeeves-cleanup --dry-run <days>  say what would be removed, change nothing
#   jeeves-cleanup --all             fold all history into one commit (after turning encryption on, so no
#                                    plain copy stays in the history on the remote)
#   jeeves-cleanup --auto            for the session-start hook, after its snapshot: clean up when jeeves.conf
#                                    says one is due; always exits 0, prints only what needs attention
#
# jeeves.conf, at the top of the data repo and shared by all devices:
#   keep_days=<n>         the days of history a cleanup keeps (1 or more)
#   auto_every_days=<m>   how often the automatic cleanup runs (m >= n); without it, there is none
# A cleanup is due when the history's first commit is m days old: a cleanup's date is its root commit's.
#
# The removed history stays on this device as .sync-state/history-before-cleanup.bundle (never pushed) until
# the next cleanup replaces it. Other devices switch to the new history at their next snapshot. This is the
# only force-push in Jeeves: only main, and only with a lease, so a push another device made meanwhile is
# never overwritten. If anything fails, main is put back and nothing is pushed.

. "$(dirname "$0")/lib.sh"

usage() { echo "usage: jeeves-cleanup <days> | --dry-run <days> | --all | --auto" >&2; exit 2; }
mode=run days= all=0
case "$1" in
  --auto) mode=auto; [ $# = 1 ] || usage ;;
  --all) all=1; [ $# = 1 ] || usage ;;
  --dry-run) mode=dry; days=$2; [ $# = 2 ] || usage ;;
  -*) usage ;;
  *) days=$1; [ $# = 1 ] || usage ;;
esac

say() { echo "jeeves cleanup: $*"; }
finish() { if [ $mode = auto ]; then exit 0; else exit "$1"; fi; }
# A check the session-start snapshot has already reported (or that will pass later) is silent in --auto.
skip() { [ $mode = auto ] || say "skipped, $*"; finish 1; }
# A whole number of 1 or more, without leading zeros (bash reads 08 as octal).
count() { case "$1" in ''|*[!0-9]*) return 1 ;; esac; [ $((10#$1)) -ge 1 ] && echo $((10#$1)); }

find_data_repo || { say "$DATA_REPO_PROBLEM"; finish 1; }
now=${JEEVES_NOW:-$(date +%s)}

if [ $mode = auto ]; then
  every=$(conf_value auto_every_days)
  [ -n "$every" ] || exit 0
  keep=$(conf_value keep_days)
  if ! every=$(count "$every") || ! days=$(count "$keep"); then
    say "automatic cleanup skipped: jeeves.conf needs keep_days and auto_every_days as whole numbers of 1 or more"
    exit 0
  fi
  if [ "$every" -lt "$days" ]; then
    say "automatic cleanup skipped: in jeeves.conf, auto_every_days ($every) must be at least keep_days ($days)"
    exit 0
  fi
  first=$(git_repo rev-list --first-parent main 2>/dev/null | tail -1)
  [ -n "$first" ] || exit 0
  [ $((now - $(git_repo log -1 --format=%ct "$first"))) -ge $((every * 86400)) ] || exit 0
elif [ $all = 0 ]; then
  days=$(count "$days") || { say "the days to keep must be a whole number of 1 or more"; exit 2; }
fi

[ $mode = run ] && "$(dirname "$0")/../bin/jeeves-snapshot"
# One Jeeves run at a time (--auto runs inside the session-start snapshot, which holds the lock already).
if [ $mode = run ] || [ $all = 1 ]; then
  take_lock 30 || { say "another Jeeves run is still at work on $REPO; nothing changed, try again shortly"; exit 1; }
fi

# Only a clean main that holds everything origin has.
branch=$(git_repo symbolic-ref --short -q HEAD)
if [ "$branch" != main ]; then skip "$REPO is on '${branch:-detached HEAD}', not main"; fi
if [ -f "$REPO/.git/MERGE_HEAD" ] || [ -n "$(git_repo status --porcelain)" ]; then
  skip "$REPO has uncommitted changes"
fi
has_origin=0
if git_repo remote get-url origin >/dev/null 2>&1; then
  has_origin=1
  if [ $mode != dry ] && ! remote_git fetch -q origin main 2>/dev/null; then
    skip "could not reach origin; nothing changed"
  fi
  origin=$(git_repo rev-parse -q --verify refs/remotes/origin/main)
  if [ -n "$origin" ] && ! git_repo merge-base --is-ancestor "$origin" main; then
    skip "main does not hold everything origin/main has; sync first (/jeeves:update)"
  fi
fi

# The newest commit on main's first line of history that is older than the cutoff becomes the new root.
if [ $all = 1 ]; then
  base=$(git_repo rev-parse main) kept_what="keeping no history"
else
  cutoff=$((now - days * 86400))
  base=$(git_repo log --first-parent --format='%H %ct' main | awk -v c="$cutoff" '$2 <= c { print $1; exit }')
  kept_what="keeping $days days"
fi
if [ -z "$base" ] || [ -z "$(git_repo rev-list --parents -n 1 "$base" | cut -s -d' ' -f2-)" ]; then
  if [ $all = 1 ]; then say "the history is already one commit"; else [ $mode = auto ] || say "nothing older than $days days to remove"; fi
  finish 0
fi
folded=$(git_repo rev-list --count "$base")
kept=$(git_repo rev-list --count "$base..main")
when=$(git_repo log -1 --format=%ci "$base")
if [ $mode = dry ]; then
  say "would fold the $folded commits up to $when into one, and keep the $kept newer ones"
  exit 0
fi

old=$(git_repo rev-parse main)
mkdir -p "$STATE_DIR"
map=$STATE_DIR/cleanup-map.sync-tmp bundle=$STATE_DIR/$CLEANUP_BUNDLE_NAME
cleanup_tmp() { rm -f "$map" "$bundle.sync-tmp"; }
fail() { cleanup_tmp; say "$*; nothing changed"; finish 1; }

root=$(printf '%s\n\n%s\n' "$CLEANUP_MARK history up to $when folded into this commit" \
  "jeeves-cleanup removed the $folded commits up to $when ($kept_what); this commit holds the files as they were then." |
  GIT_AUTHOR_DATE="$now +0000" GIT_COMMITTER_DATE="$now +0000" git_repo commit-tree "$base^{tree}") ||
  fail "could not write the new first commit"

# Recreate each later commit, parents first. A parent that was removed maps to the new root.
: > "$map"
new=$root
for c in $(git_repo rev-list --reverse --topo-order "$base..main"); do
  parents=
  for p in $(git_repo rev-list --parents -n 1 "$c" | cut -s -d' ' -f2-); do
    np=$(awk -v k="$p" '$1 == k { print $2; exit }' "$map"); [ -n "$np" ] || np=$root
    case " $parents " in *" -p $np "*) ;; *) parents="$parents -p $np" ;; esac
  done
  { IFS= read -r an; IFS= read -r ae; IFS= read -r ad; IFS= read -r cn; IFS= read -r ce; IFS= read -r cd; } <<EOF
$(git_repo log -1 --format='%an%n%ae%n%ad%n%cn%n%ce%n%cd' --date=raw "$c")
EOF
  # $parents is "-p <hash> ...", split on purpose.
  new=$(git_repo cat-file commit "$c" | sed '1,/^$/d' |
    GIT_AUTHOR_NAME=$an GIT_AUTHOR_EMAIL=$ae GIT_AUTHOR_DATE=$ad \
    GIT_COMMITTER_NAME=$cn GIT_COMMITTER_EMAIL=$ce GIT_COMMITTER_DATE=$cd \
    git_repo commit-tree "$c^{tree}" $parents) || fail "could not recreate commit $c"
  echo "$c $new" >> "$map"
done
[ "$(git_repo rev-parse "$new^{tree}")" = "$(git_repo rev-parse "main^{tree}")" ] ||
  fail "the recreated history does not end with the same files"

# Keep the old history before main moves; it replaces the previous cleanup's only once the push is done.
git_repo bundle create "$bundle.sync-tmp" main >/dev/null 2>&1 || fail "could not save the old history"

# Put main back; refused (and said) if something committed onto the new history meanwhile.
undo() {
  if git_repo update-ref -m "jeeves-cleanup: undone" refs/heads/main "$old" "$new"; then cleanup_tmp
  else
    rm -f "$map"
    say "could not put main back (it moved meanwhile); the old history is in .sync-state/$CLEANUP_BUNDLE_NAME.sync-tmp"
  fi
}
git_repo update-ref -m "jeeves-cleanup" refs/heads/main "$new" "$old" || fail "could not move main"
trap 'undo; exit 1' INT TERM HUP

if [ $has_origin = 1 ]; then
  if ! scan=$("$(dirname "$0")/../bin/jeeves-scan"); then
    undo; trap - INT TERM HUP
    printf '%s\n' "$scan"; say "not pushed, because of the possible secrets above; nothing changed"; finish 1
  fi
  if ! err=$(remote_git push -q --force-with-lease="main:$origin" origin main 2>&1); then
    undo; trap - INT TERM HUP
    say "push refused or failed, so main was put back and nothing changed (another device may have pushed" \
        "meanwhile; the cleanup runs again later): $(printf '%s' "$err" | head -3 | join_lines)"
    finish 1
  fi
fi
trap - INT TERM HUP
mv "$bundle.sync-tmp" "$bundle"; rm -f "$map"
shrink_repo
done_msg="folded the $folded commits up to $when into one and kept the $kept newer ones"
[ $has_origin = 1 ] && done_msg="$done_msg; pushed"
say "$done_msg (the old history is in .sync-state/$CLEANUP_BUNDLE_NAME on this device)"
finish 0
