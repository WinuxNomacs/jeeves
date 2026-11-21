#!/usr/bin/env bash
# The memories /jeeves:update's session review reads for operational knowledge worth promoting to CLAUDE.md,
# and the record of those it has reviewed, shared by all devices: hosts/.memory-review in the data repo
# (encrypted with the rest of hosts/ when encryption is on). Memories themselves are never changed.
#
#   jeeves-memories list     memory files not reviewed yet, or changed since, one per line:
#                            <hash> TAB <source> TAB <project> TAB <file>
#                            source: "this-machine", or the host whose backup in hosts/ holds it; project: the
#                            project folder's name under projects/; file: the absolute path to read.
#                            This machine's live memories come first; MEMORY.md (an index) is left out, and a
#                            file found in several places is listed once.
#   jeeves-memories record <hash> <outcome> [<hash> <outcome>...]
#                            adds each reviewed memory and what came of it (such as "promoted 2, kept 3,
#                            left out 1") to the record, and commits it on main
#   jeeves-memories list --all
#                            every memory file, reviewed or not, in the same form (for /jeeves:review)
#
# The data repo is found as lib/lib.sh describes.

. "$(dirname "$0")/../lib/lib.sh"

say() { echo "jeeves memories: $*"; }
usage() { echo "usage: jeeves-memories list [--all] | record <hash> <outcome> [<hash> <outcome>...]" >&2; exit 2; }
[ $# -ge 1 ] || usage
cmd=$1; shift
all=
case "$cmd" in list) [ $# = 0 ] || { [ $# = 1 ] && [ "$1" = --all ] && all=1; } || usage ;; record) [ $# -ge 2 ] && [ $(( $# % 2 )) = 0 ] || usage ;; esac
find_data_repo || { say "$DATA_REPO_PROBLEM"; exit 1; }
RECORD=$REPO/hosts/.memory-review

# memory_files <projects folder> <source>: "<hash>\t<source>\t<project>\t<file>" for each memory file.
memory_files() {
  local dir key f
  for dir in "$1"/*/memory; do
    [ -d "$dir" ] || continue
    key=$(basename "$(dirname "$dir")")
    find "$dir" -type f -name '*.md' ! -name MEMORY.md | sort | while IFS= read -r f; do
      printf '%s\t%s\t%s\t%s\n' "$(blob_hash -- "$f")" "$2" "$key" "$f"
    done
  done
}

case "$cmd" in
  list)
    {
      memory_files "$CLAUDE_HOME/projects" this-machine
      for h in "$REPO"/hosts/*/; do
        h=$(basename "$h")
        [ "$h" = "$HOST" ] && continue   # this machine's backup: its live memories are listed already
        memory_files "$REPO/hosts/$h/projects" "$h"
      done
    } | awk -F'\t' -v rec="$RECORD" -v all="$all" '
      BEGIN { if (!all) while ((getline line < rec) > 0) { split(line, a, "\t"); done[a[1]] = 1 } }
      !($1 in done) && !seen[$1]++' ;;

  record)
    [ "$(git_repo symbolic-ref --short -q HEAD)" = main ] || { say "$REPO is not on main; nothing recorded"; exit 1; }
    [ ! -f "$REPO/.git/MERGE_HEAD" ] || { say "$REPO is in the middle of a merge; nothing recorded"; exit 1; }
    take_lock 30 || { say "another Jeeves run is still at work on $REPO; nothing recorded, try again shortly"; exit 1; }
    new=$CRYPT_DIR/memory-review.new old=$CRYPT_DIR/memory-review.old
    lines=
    while [ $# -gt 0 ]; do
      case "$1" in *[!0-9a-f]*|'') say "not a memory hash: $1; nothing recorded"; exit 1 ;; esac
      [ ${#1} -ge 40 ] && [ ${#1} -le 64 ] || { say "not a memory hash: $1; nothing recorded"; exit 1; }
      lines="$lines$1	$(printf '%s' "$2" | tr '[:cntrl:]' ' ' | cut -c1-200)
"
      shift 2
    done
    lines=$(printf '%s' "$lines")
    n=$(printf '%s\n' "$lines" | wc -l | tr -d ' ')
    day=$(date +%Y-%m-%d)
    rm -f "$old"; [ -f "$RECORD" ] && { cp "$RECORD" "$old" || exit 1; }
    # One line per memory hash, the newest outcome kept, sorted by hash (so two devices rarely touch the same
    # lines; a merge keeps both sides' lines).
    { [ -f "$RECORD" ] && awk -F'\t' 'NR == FNR { skip[$1] = 1; next } !($1 in skip)' <(printf '%s\n' "$lines") "$RECORD"
      printf '%s\n' "$lines" | awk -F'\t' -v d="$day" -v h="$HOST" '{ print $1 "\t" $2 "\t" d "\t" h }'
    } | sort -t "$(printf '\t')" -k1,1 -u > "$new" && mkdir -p "$REPO/hosts" && mv "$new" "$RECORD" ||
      { rm -f "$new"; say "could not write the record (disk full?); nothing recorded"; exit 1; }
    if git_repo add -- hosts/.memory-review &&
       git_repo commit -q -m "memory review($HOST): $n memories reviewed" -- hosts/.memory-review; then
      rm -f "$old"; say "recorded $n reviewed memories"
    else
      # Put the record back as it was, so no uncommitted change blocks the next snapshot.
      git_repo reset -q -- hosts/.memory-review
      if [ -f "$old" ]; then mv "$old" "$RECORD"; else rm -f "$RECORD"; fi
      say "could not commit the record; nothing recorded"; exit 1
    fi ;;

  *) usage ;;
esac
