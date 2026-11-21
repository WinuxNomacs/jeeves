#!/usr/bin/env bash
# Copy a device's backup (hosts/<host>/ in the data repo) onto this machine's ~/.claude.
#
#   jeeves-restore                        this machine's host name, and the hosts that have a backup
#   jeeves-restore <host>                 restore its settings.json and keybindings.json
#   jeeves-restore <host> --memories      also its memories, for projects that exist on this machine
#                                         (~/.claude/projects/<key>/ is there)
#
# Files it replaces are saved first under .sync-state/overwritten/<time>/. It never commits: the next
# jeeves-snapshot backs up this machine as it is then. Paths inside the restored settings (the plugin
# marketplace folder, data_repo) may need fixing on a new machine: /jeeves:restore does that.
# The data repo is found as lib/lib.sh describes.

. "$(dirname "$0")/lib.sh"

say() { echo "jeeves restore: $*"; }
host= memories=0
for arg in "$@"; do
  case "$arg" in
    --memories) memories=1 ;;
    -*) echo "restore: unknown option $arg" >&2; exit 2 ;;
    *) [ -z "$host" ] || { echo "restore: one host only" >&2; exit 2; }; host=$arg ;;
  esac
done
find_data_repo || { say "$DATA_REPO_PROBLEM"; exit 1; }

hosts=$(cd "$REPO" && ls -d hosts/*/ 2>/dev/null | sed -e 's#^hosts/##' -e 's#/$##' | join_lines)
if [ -z "$host" ]; then
  say "this host: $HOST; backups: ${hosts:-none}"
  exit 0
fi
case "$host" in */* | . | ..) say "not a host name: $host"; exit 2 ;; esac
src=$REPO/hosts/$host
[ -d "$src" ] || { say "no backup for $host (backups: ${hosts:-none})"; exit 1; }
take_lock 30 || { say "another Jeeves run is still at work on $REPO; nothing restored, try again shortly"; exit 1; }

saved_dir=$STATE_DIR/overwritten/$(date +%Y%m%d-%H%M%S)
restored=() skipped=() failed=() saved=0
# put <path relative to ~/.claude>: copy it from the backup, saving a different disk copy first; a file whose
# disk copy could not be saved is left as it is.
put() {
  if [ -f "$CLAUDE_HOME/$1" ] && ! cmp -s "$src/$1" "$CLAUDE_HOME/$1"; then
    copy_file "$CLAUDE_HOME/$1" "$saved_dir/$1" || { failed+=("$1 (its disk copy could not be saved first)"); return 1; }
    saved=1
  fi
  copy_file "$src/$1" "$CLAUDE_HOME/$1" || { failed+=("$1 (could not be written)"); return 1; }
}

for f in $HOST_FILES; do
  [ -f "$src/$f" ] || continue
  put "$f" && restored+=("$f")
done

if [ $memories = 1 ]; then
  for dir in "$src"/projects/*/memory; do
    [ -d "$dir" ] || continue
    key=$(basename "$(dirname "$dir")")
    if [ ! -d "$CLAUDE_HOME/projects/$key" ]; then skipped+=("$key"); continue; fi
    while IFS= read -r f; do put "projects/$key/memory/$f"; done < <(cd "$dir" && find . -type f | sed 's#^\./##')
    restored+=("memories of $key")
  done
fi

if [ ${#restored[@]} -gt 0 ]; then say "restored from $host: $(printf '%s, ' "${restored[@]}" | sed 's/, $//')"
else say "nothing to restore from $host"
fi
[ $saved = 1 ] && say "previous copies saved in $saved_dir"
[ ${#skipped[@]} -gt 0 ] && say "memories not restored (no such project on this machine): $(printf '%s\n' "${skipped[@]}" | join_lines)"
if [ ${#failed[@]} -gt 0 ]; then
  say "not restored, left as they were: $(printf '%s; ' "${failed[@]}" | sed 's/; $//')"; exit 1
fi
exit 0
