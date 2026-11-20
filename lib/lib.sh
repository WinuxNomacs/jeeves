# Shared by bin/jeeves-snapshot and bin/jeeves-apply. Sourced, not run.
# Portable bash: works on bash 3.2 (macOS), Git Bash (Windows) and Linux. No GNU-only flags,
# no associative arrays, no `set -u` (empty arrays break it on bash < 4.4).

. "$(dirname "${BASH_SOURCE[0]}")/crypt.sh"

# Claude Code's own folder: CLAUDE_CONFIG_DIR when the user has moved it.
CLAUDE_HOME=${CLAUDE_HOME:-${CLAUDE_CONFIG_DIR:-$HOME/.claude}}
# This device's name in the data repo. Native Windows (Git Bash) gets "-win", so it stays apart from a WSL
# on the same PC, which reports the same host name.
if [ -n "$JEEVES_HOST" ]; then HOST=$JEEVES_HOST
else
  HOST=$(hostname 2>/dev/null | cut -d. -f1)
  [ -n "$HOST" ] || HOST=$(uname -n 2>/dev/null | cut -d. -f1)
  case "$(uname -s)" in MINGW*|MSYS*|CYGWIN*) HOST=$HOST-win ;; esac
fi

# Find the data repo (the git repo holding home/ and hosts/) and set REPO and the paths inside it.
# Order: $JEEVES_DATA (or `--data-repo <path>`, below), then the plugin option data_repo (hooks see it as
# CLAUDE_PLUGIN_OPTION_DATA_REPO), then the path this device last used (DATA_REPO_POINTER), then the git
# repo around the current folder if it is a data repo, then DEFAULT_DATA_REPO (the option's default, which
# an install without --config may leave unset).
# Returns 1 with DATA_REPO_PROBLEM set when none is found, or when it is encrypted and this device is locked
# (unless ALLOW_LOCKED=1).
DEFAULT_DATA_REPO=$HOME/jeeves-data
# The data repo a script was last told about explicitly (by --data-repo with a path, or the plugin option;
# not a JEEVES_DATA set for one command), so the skills can run every later command bare: Claude's Bash tool sees no plugin variables, and a permission
# rule does not match past a `cd` outside the project or a `JEEVES_DATA=` prefix, so either would ask.
DATA_REPO_POINTER=$CLAUDE_HOME/jeeves/data-repo

# `--data-repo <path>` as a script's first two arguments: the same as JEEVES_DATA, and recorded. An empty path
# (the plugin option left empty) means what it does without one, except that the record is not read and is
# removed, so later bare commands find the same repo. A placeholder left unfilled (a plugin loaded with
# CLAUDE_CODE_PLUGIN_DIRS or --plugin-dir gets no plugin options, so its skills pass the literal
# ${user_config.data_repo}) counts as no --data-repo at all. This file is sourced without arguments, so it
# shares the script's own and the shift is the script's.
DATA_REPO_GIVEN=
if [ "$1" = --data-repo ]; then
  case "$2" in '${user_config.'*) ;; *) JEEVES_DATA=$2 DATA_REPO_GIVEN=1 ;; esac
  shift; [ $# -gt 0 ] && shift
fi

# A path with a leading ~/ (as plugin options are written) made absolute.
expand_path() {
  case "$1" in "~") printf '%s\n' "$HOME" ;; "~/"*) printf '%s\n' "$HOME/${1#"~/"}" ;; *) printf '%s\n' "$1" ;; esac
}

find_data_repo() {
  local candidate top told=
  candidate=$(expand_path "${JEEVES_DATA:-$CLAUDE_PLUGIN_OPTION_DATA_REPO}")
  if [ -n "$DATA_REPO_GIVEN" ]; then
    if [ -n "$JEEVES_DATA" ]; then told=1; else rm -f "$DATA_REPO_POINTER" 2>/dev/null; fi
  elif [ -z "$JEEVES_DATA" ] && [ -n "$CLAUDE_PLUGIN_OPTION_DATA_REPO" ]; then
    told=1
  fi
  if [ -z "$candidate" ] && [ -z "$DATA_REPO_GIVEN" ] && [ -f "$DATA_REPO_POINTER" ]; then
    IFS= read -r candidate < "$DATA_REPO_POINTER"
    candidate=${candidate%$'\r'}
  fi
  if [ -z "$candidate" ]; then
    # Only a data repo: home/, and .sync-state/ ignored as jeeves-init writes it (a dotfiles repo has home/ too).
    top=$(git rev-parse --show-toplevel 2>/dev/null) && [ -d "$top/home" ] &&
      tr -d '\r' < "$top/.gitignore" 2>/dev/null | grep -qx '\.sync-state/' && candidate=$top
  fi
  if [ -z "$candidate" ]; then
    candidate=$DEFAULT_DATA_REPO
    if [ ! -e "$candidate/.git" ]; then
      DATA_REPO_PROBLEM="Jeeves is not set up on this machine yet (no data repo at $candidate, the default): run /jeeves:setup"
      return 1
    fi
  fi
  if ! git -C "$candidate" rev-parse --git-dir >/dev/null 2>&1; then
    DATA_REPO_PROBLEM="the data repo $candidate is not a git repository: to clone or create it, run /jeeves:setup"
    return 1
  fi
  # hosts/<host>/ is replaced whole at each snapshot: a missing or odd name must never point elsewhere.
  # A name starting with a dot is refused too: hosts/.memory-review is the shared review record.
  case "$HOST" in ''|.*|*/*|*\\*)
    DATA_REPO_PROBLEM="this device's name could not be found (hostname and uname -n gave '$HOST'); set JEEVES_HOST"
    return 1 ;;
  esac
  REPO=$(cd "$candidate" && pwd)
  if [ -n "$told" ] && ! remember_data_repo && [ -n "$DATA_REPO_GIVEN" ]; then
    echo "jeeves: could not record the data repo in $DATA_REPO_POINTER: later commands without --data-repo may not find it" >&2
  fi
  HOME_COPY=$REPO/home
  STATE_DIR=$REPO/.sync-state
  STATE_FILE=$STATE_DIR/hashes
  HOST_DIR=$REPO/hosts/$HOST
  CRYPT_DIR=$REPO/.git/jeeves CRYPT_TOP=$REPO
  if encrypted || unlocked; then crypt_setup; fi
  if encrypted && ! unlocked && [ "$ALLOW_LOCKED" != 1 ]; then
    DATA_REPO_PROBLEM="the data repo is encrypted and this device is locked: run /jeeves:encryption"
    return 1
  fi
  return 0
}

# Records REPO in DATA_REPO_POINTER when it differs; returns 1 if it could not (find_data_repo warns when a
# skill passed --data-repo; the hook stays quiet).
remember_data_repo() {
  local old= rc=0
  [ -f "$DATA_REPO_POINTER" ] && IFS= read -r old < "$DATA_REPO_POINTER"
  [ "$old" = "$REPO" ] && return 0
  { mkdir -p "${DATA_REPO_POINTER%/*}" &&
    printf '%s\n' "$REPO" > "$DATA_REPO_POINTER.tmp.$$" &&
    mv -f "$DATA_REPO_POINTER.tmp.$$" "$DATA_REPO_POINTER"; } 2>/dev/null || rc=1
  rm -f "$DATA_REPO_POINTER.tmp.$$" 2>/dev/null
  return $rc
}

# ---- Encryption (lib/crypt.sh has the rest) ----

# The data repo's working tree is encrypted (has a recipient).
encrypted() { [ -s "$REPO/recipient" ]; }
# This device holds the key (the marker names the recipient, or encryption is coming from origin).
unlocked() { [ -s "$CRYPT_DIR/unlocked" ] && { ! encrypted || [ "$(cat "$CRYPT_DIR/unlocked")" = "$(cat "$REPO/recipient")" ]; }; }
# The given commit holds an encrypted data repo.
encrypted_at() { git_repo cat-file -e "$1:recipient" 2>/dev/null; }
# The given encrypted commit's history still holds home/ or hosts/ files from before encryption (the fold
# after turning it on did not reach origin).
plain_history_at() {
  local first
  first=$(git_repo rev-list --reverse "$1" -- recipient 2>/dev/null | head -1)
  [ -n "$first" ] && git_repo rev-parse -q --verify "$first^" >/dev/null &&
    [ -n "$(git_repo log -1 --format=%H "$first^" -- home hosts)" ]
}
PLAIN_HISTORY_MSG="the data repo's history on GitHub still holds the plain files from before encryption: run /jeeves:encryption to remove it"
# Read the key once for this script and the git commands it runs; 1 when the store has none.
load_key() {
  [ -n "$JEEVES_AGE_KEY" ] && return 0
  JEEVES_AGE_KEY=$(store_get "$(cat "$CRYPT_DIR/unlocked" 2>/dev/null)") || { JEEVES_AGE_KEY=; return 1; }
  export JEEVES_AGE_KEY
}

# Point git at this plugin's jeeves-crypt (the path changes with each plugin version). git compares only size
# and whole-second mtime: the snapshot rewrites hosts/ with the same times, and a full check would run the
# filter on every file.
crypt_setup() {
  local bin want have
  bin=$(cd "$(dirname "${BASH_SOURCE[0]}")/../bin" && pwd)/jeeves-crypt
  want="filter.jeeves.clean \"$bin\" clean %f
filter.jeeves.smudge \"$bin\" smudge %f
filter.jeeves.required true
diff.jeeves.textconv \"$bin\" textconv
merge.jeeves.name Jeeves (encrypted)
merge.jeeves.driver \"$bin\" merge %O %A %B %L
core.checkstat minimal
core.trustctime false"
  have=$(git -C "$REPO" config --get-regexp '^(filter|diff|merge)\.jeeves\.|^core\.(checkstat|trustctime)$')
  [ "$(printf '%s\n' "$have" | sort)" = "$(printf '%s\n' "$want" | sort)" ] && return 0
  printf '%s\n' "$want" | while read -r k v; do git -C "$REPO" config "$k" "$v"; done
}

# Shared files and folders under ~/.claude: synced between all devices through home/. Folders are walked
# for files.
TRACKED_FILES="CLAUDE.md"
TRACKED_DIRS="agents commands skills"
# Per-device files: backed up to hosts/<host>/ when jeeves.conf has backup_settings=yes (memories always
# are), never applied automatically.
HOST_FILES="settings.json keybindings.json"

# A setting from jeeves.conf at the top of the data repo (shared by all devices), or nothing.
conf_value() {
  [ -f "$REPO/jeeves.conf" ] || return 0
  tr -d '\r' < "$REPO/jeeves.conf" | sed -n "s/^[[:space:]]*$1[[:space:]]*=[[:space:]]*//p" | sed 's/[[:space:]]*$//' | tail -1
}
# settings.json and keybindings.json are backed up only when the user turned it on (backup_settings=yes).
backup_settings_on() { case "$(conf_value backup_settings | tr '[:upper:]' '[:lower:]')" in yes|true|on|1) return 0 ;; esac; return 1; }
# Never copied: skills/synced/ is managed by claude.ai; .trash/ holds what Claude Code removed.
EXCLUDED_PREFIXES="skills/synced/ skills/.trash/ agents/.trash/ commands/.trash/"

# Files are copied byte for byte, so git must not convert line endings either (Git for Windows sets
# core.autocrlf=true), and its "LF will be replaced by CRLF" warnings must not reach a hook's message.
# The user's own ignore list (core.excludesFile: .DS_Store, *.swp...) never applies to the data repo: a file
# it names would be copied but never committed.
git_repo() { git -C "$REPO" -c core.autocrlf=false -c core.safecrlf=false -c core.excludesFile= "$@"; }

excluded() {
  local x
  for x in $EXCLUDED_PREFIXES; do case "$1" in "$x"*) return 0 ;; esac; done
  return 1
}

# Relative paths of tracked files under the given root, one per line.
tracked_under() {
  local root=$1 f d
  for f in $TRACKED_FILES; do
    [ -f "$root/$f" ] && printf '%s\n' "$f"
  done
  for d in $TRACKED_DIRS; do
    [ -d "$root/$d" ] || continue
    (cd "$root" && find "$d" -type f) | while IFS= read -r f; do
      excluded "$f" || printf '%s\n' "$f"
    done
  done
}

# Tracked files in one folder under the given root, following the folder itself when it is a symlink.
files_in_dir() {
  (cd "$1/$2" 2>/dev/null && find . -type f) | sed "s#^\./#$2/#" | while IFS= read -r f; do
    excluded "$f" || printf '%s\n' "$f"
  done
}

# This machine's own ID (random, made once, kept in ~/.claude outside the synced files): two devices that
# report the same host name must not share hosts/<host>/. Empty when it cannot be kept.
machine_id() {
  local f=$CLAUDE_HOME/.jeeves-machine-id
  if [ ! -s "$f" ]; then
    od -An -N8 -tx1 /dev/urandom 2>/dev/null | tr -d ' \n' > "$f.tmp" && [ -s "$f.tmp" ] && mv "$f.tmp" "$f" ||
      { rm -f "$f.tmp"; return 0; }
  fi
  cat "$f"
}
HOST_MARK=.jeeves-machine

# Union of tracked paths on disk and in the repo copy.
all_tracked() {
  { tracked_under "$CLAUDE_HOME"; tracked_under "$HOME_COPY"; } | sort -u
}

# Git blob hash of a file, or "-" when it does not exist.
hash_of() {
  if [ -f "$1" ]; then git hash-object --no-filters -- "$1"; else printf -- '-\n'; fi
}

# Hash recorded at the last sync on this device, empty when never synced.
base_of() {
  [ -f "$STATE_FILE" ] || return 0
  awk -F'\t' -v p="$1" '$2 == p { print $1 }' "$STATE_FILE"
}

# Record (or with "-", forget) the synced hash of a path. With the file holding that version, its content
# goes into git's objects too (unreachable, so never pushed), so `git cat-file -p <base>` reads it even when
# git stores the file encrypted.
set_base() {
  local path=$1 hash=$2 tmp
  [ "$(base_of "$path")" = "$hash" ] && return 0
  [ -n "$3" ] && [ "$hash" != "-" ] && git -C "$REPO" hash-object -w --no-filters -- "$3" >/dev/null
  mkdir -p "$STATE_DIR" || return 1
  tmp=$STATE_FILE.tmp
  # A failed write (a full disk) keeps the old record: a truncated one would lose every other base.
  if { { [ ! -f "$STATE_FILE" ] || awk -F'\t' -v p="$path" '$2 != p' "$STATE_FILE"; } &&
       { [ "$hash" = "-" ] || printf '%s\t%s\n' "$hash" "$path"; }; } > "$tmp"; then
    mv "$tmp" "$STATE_FILE" && return 0
  fi
  rm -f "$tmp"; return 1
}

# Keep every recorded base in git's objects: one local ref (refs/jeeves/bases, never pushed) to a tree of
# them, so gc does not prune them (they are unreachable otherwise: git stores the file encrypted, or a cleanup
# folded the commit that held it).
keep_bases() {
  local t
  [ -f "$STATE_FILE" ] || return 0
  t=$(awk -F'\t' '{ print $1 }' "$STATE_FILE" | sort -u | git_repo cat-file --batch-check |
      awk '$2 == "blob" { print "100644 blob " $1 "\t" $1 }' | git_repo mktree) || return 0
  [ "$(git_repo rev-parse -q --verify refs/jeeves/bases)" = "$t" ] ||
    git_repo update-ref -m "jeeves: merge bases" refs/jeeves/bases "$t"
}

# Copy a file atomically (temp file in the target folder, then rename), creating folders. A failed copy
# leaves the target as it was and returns 1.
copy_file() {
  local src=$1 dst=$2
  mkdir -p "$(dirname "$dst")" && cp -p "$src" "$dst.sync-tmp" && mv "$dst.sync-tmp" "$dst" && return 0
  rm -f "$dst.sync-tmp"; return 1
}

# Copy a file that may be being written (a ~/.claude file): the copy must hash the same as the source just
# after it, or it is tried again. Prints the hash of what was copied. Returns 1 when the copy failed, 2 when
# the file kept changing (it is still being written; the target is left as it was).
copy_settled() {
  local src=$1 dst=$2 i h
  mkdir -p "$(dirname "$dst")" || return 1
  for i in 1 2 3; do
    # A file deleted meanwhile is left for the next snapshot, which records the deletion.
    cp -p "$src" "$dst.sync-tmp" || { rm -f "$dst.sync-tmp"; [ -e "$src" ] || return 2; return 1; }
    h=$(hash_of "$dst.sync-tmp")
    if [ "$h" = "$(hash_of "$src")" ]; then
      mv "$dst.sync-tmp" "$dst" || { rm -f "$dst.sync-tmp"; return 1; }
      printf '%s\n' "$h"; return 0
    fi
    sleep 1
  done
  rm -f "$dst.sync-tmp"; return 2
}

# True when the staged content of a data repo path (decrypted) hashes to the given plain hash: what a commit
# would record is what was copied. Through the clean cache when the file is stored encrypted, so no key is read.
staged_is() {
  local blob c
  blob=$(git_repo rev-parse -q --verify ":$1") || return 1
  [ "$blob" = "$2" ] && return 0
  c=$CRYPT_DIR/clean/$2
  [ -s "$c" ] && [ "$(git hash-object --no-filters -- "$c")" = "$blob" ] && return 0
  [ "$(git_repo cat-file --filters ":$1" 2>/dev/null | git hash-object --stdin --no-filters)" = "$2" ]
}

# True when a file holds git conflict markers (an unfinished merge): such a file is never installed or
# recorded unless the user names it (--force <path>).
has_conflict_markers() {
  [ -f "$1" ] && grep -qE '^<<<<<<<( |$)' "$1" && grep -qE '^>>>>>>>( |$)' "$1" && grep -qx '=======' "$1"
}

# Classify a path: insync | disk | repo | conflict.
#   disk: only the ~/.claude copy changed since the last sync (snapshot takes it)
#   repo: only the repo copy changed (apply takes it)
classify() {
  local d=$1 r=$2 b=$3
  if [ "$d" = "$r" ]; then echo insync
  elif [ -z "$b" ]; then
    # Never synced on this device: a file present on one side only is a plain add.
    if [ "$r" = "-" ]; then echo disk
    elif [ "$d" = "-" ]; then echo repo
    else echo conflict
    fi
  elif [ "$r" = "$b" ]; then echo disk
  elif [ "$d" = "$b" ]; then echo repo
  else echo conflict
  fi
}

join_lines() { tr '\n' ' ' | sed 's/ $//'; }

# Shared files that carry instructions or code into every session on every device: a new or changed one from
# the data repo is installed only once the user has accepted that exact version (jeeves-apply --accept).
held_path() { case "$1" in skills/*|agents/*|commands/*) return 0 ;; esac; return 1; }

# added_lines <old file> <new file>: "line <n>\t<text>" for each line the new file adds (all of them when
# the old file does not exist).
added_lines() {
  local old=$1
  [ -f "$old" ] || old=/dev/null
  git -c core.autocrlf=false -c core.quotepath=false diff --no-index --no-color --no-ext-diff --no-textconv \
    -U0 -- "$old" "$2" 2>/dev/null | awk '
    /^@@/ { for (i = 2; i <= NF; i++) if (substr($i, 1, 1) == "+") { split(substr($i, 2), a, ","); n = a[1] + 0 }
            hunk = 1; next }
    !hunk || /^\\/ { next }
    /^\+/ { print "line " n "\t" substr($0, 2); n++ }'
}

# outgoing_added <range>: "<file>\t<line>\t<commit>\t<text>" for each line the commits in <range> add, each
# commit's own diff (a merge's combined diff: only lines that match none of its parents, as the rest come
# from commits checked anyway).
outgoing_added() {
  git_repo -c core.quotepath=false log --no-color --no-ext-diff --format='commit %h' -p --cc -U0 "$1" -- |
  awk '
  /^commit / { c = $2; hunk = 0; next }
  /^diff / { hunk = 0; next }
  !hunk && /^\+\+\+ / { f = substr($0, 7); next }
  /^@@/ { match($0, /^@+/); np = RLENGTH - 1; hunk = 1
          for (i = 2; i <= NF; i++) if (substr($i, 1, 1) == "+") { split(substr($i, 2), a, ","); n = a[1] + 0 }
          next }
  /^\\/ { next }
  hunk { m = substr($0, 1, np)
         if (m ~ /-/) next
         if (m ~ /^\++$/) print f "\t" n "\t" c "\t" substr($0, np + 1)
         n++ }'
}

# risk_flags: reads "<where>\t<text>" lines on stdin and prints "<where>: <kind>" for each line that could
# make a session run outside code, gain permissions or hooks, or obey text it reads. A flag is for the user
# to look at, not proof of harm.
risk_flags() {
  local lines
  lines=$(cat)
  [ -n "$lines" ] || return 0
  risk_look() { # risk_look <kind> <grep flags> <extended regex>
    local i
    for i in $(printf '%s\n' "$lines" | cut -f2- | grep -n $2 -e "$3" | cut -d: -f1); do
      printf '%s: %s\n' "$(printf '%s\n' "$lines" | sed -n "${i}p" | cut -f1)" "$1"
    done
  }
  risk_look "a download run by a shell or interpreter" -Ei \
    '(curl|wget|iwr|irm|invoke-webrequest|invoke-restmethod)[^|]*\|[[:space:]]*(sudo[[:space:]]+)?([a-z]*sh|iex|invoke-expression|python[0-9.]*|node|perl|ruby)([[:space:]]|$)'
  risk_look "encoded text decoded (possibly to run it)" -Ei 'base64[[:space:]]+(-d|-D|--decode)|frombase64string'
  risk_look "a long encoded blob" -E '[A-Za-z0-9+/_=-]{200,}'
  risk_look "a tool permission or permission mode" -Ei \
    'allowed-?tools|bypasspermissions|dangerously-skip-permissions|permission-mode|defaultmode|"(allow|deny|ask)"[[:space:]]*:'
  risk_look "a hook" -Ei '"hooks"[[:space:]]*:|^[[:space:]]*hooks[[:space:]]*:'
  risk_look "text telling the model to drop its instructions" -Ei \
    '(ignore|disregard|forget|override)([[:space:]]+(all|any|the|your|of|these|previous|prior|above|earlier|preceding|system))+[[:space:]]+(instructions|rules|prompts?|guidelines)'
}

# Why a shared path can't be held on every OS (Windows, macOS), or nothing when it can: characters Windows
# forbids, or a name part ending in a dot or space.
bad_name() {
  case "$1" in
    *[[:cntrl:]]*) echo "its name has a control character (such as a tab or a line break)" ;;
    *[':*?"<>|\\']*) echo "its name has a character Windows does not allow" ;;
    *'.' | *' ' | *'./'* | *' /'*) echo "a part of its name ends in a dot or space, which Windows does not allow" ;;
  esac
}

# Paths (from stdin, one per line) that differ only in case from another: they collide on Windows and macOS.
case_collisions() {
  awk '{ k = tolower($0); n[k]++; p[NR] = $0; l[NR] = k } END { for (i = 1; i <= NR; i++) if (n[l[i]] > 1) print p[i] }'
}

# Git commands that contact origin: never wait for a password prompt or a credential window, give up on a
# dead ssh host fast, and stop after a deadline (a stalled HTTPS server would otherwise hold up the
# session-start hook for as long as it stalls). git runs directly under with_deadline, so a stop reaches it.
# The user's own ssh command (GIT_SSH_COMMAND, core.sshCommand, GIT_SSH) is kept. Fetch and push never
# decrypt, so the key stays out of them (and of hooks and ssh).
remote_git() (
  if [ -z "$GIT_SSH_COMMAND" ] && [ -z "$GIT_SSH" ] && [ -z "$(git -C "$REPO" config core.sshCommand)" ]; then
    export GIT_SSH_COMMAND="ssh -o BatchMode=yes -o ConnectTimeout=5"
  fi
  unset JEEVES_AGE_KEY
  export GIT_TERMINAL_PROMPT=0 GCM_INTERACTIVE=never
  with_deadline "${JEEVES_REMOTE_DEADLINE:-30}" git -C "$REPO" -c core.autocrlf=false -c core.safecrlf=false "$@"
)

# The first line jeeves-cleanup gives the parentless commit that replaces the removed history.
CLEANUP_MARK="jeeves-cleanup:"
# Where the removed history is kept on this device (never pushed) until the next cleanup replaces it.
CLEANUP_BUNDLE_NAME=history-before-cleanup.bundle

# True when the history of the given commit starts at a jeeves-cleanup root.
cleaned_root() {
  git_repo rev-list --max-parents=0 "$1" 2>/dev/null | while IFS= read -r r; do
    git_repo log -1 --format=%s "$r"
  done | grep -q "^$CLEANUP_MARK"
}

# Drop the objects no branch reaches any more (removed history, kept in the bundle), so .git shrinks.
shrink_repo() {
  keep_bases
  git_repo reflog expire --expire=now --all && git_repo gc -q --prune=now 2>/dev/null
}

# True when the given plugin folder is a development checkout the user lists, one per line, in
# ~/.claude/jeeves/dev-checkouts (`~` and `~/` taken as the home folder). jeeves-verify, which sources
# nothing, has its own copy of this loop.
is_dev_checkout() {
  local d f=$CLAUDE_HOME/jeeves/dev-checkouts
  [ -f "$f" ] || return 1
  while IFS= read -r d || [ -n "$d" ]; do
    d=${d%$'\r'}
    case "$d" in "~") d=$HOME ;; "~/"*) d=$HOME/${d#"~/"} ;; esac
    [ -n "$d" ] && [ "$(cd "$d" 2>/dev/null && pwd -P)" = "$1" ] && return 0
  done < "$f"
  return 1
}

# Run a command with a deadline in seconds (no `timeout` on macOS): past it, the command is stopped by its own
# PID, gracefully and then by force, and this returns 124.
with_deadline() {
  local limit=$(( $1 * 10 )) i=0 pid
  shift
  "$@" & pid=$!
  while kill -0 "$pid" 2>/dev/null; do
    if [ $i -ge $limit ]; then
      kill "$pid" 2>/dev/null; sleep 1; kill -9 "$pid" 2>/dev/null; wait "$pid" 2>/dev/null
      return 124
    fi
    sleep 0.1; i=$((i + 1))
  done
  wait "$pid"
}

# One Jeeves run that changes the data repo or ~/.claude at a time per data repo (two sessions starting
# together would race on the same temp files and lose sync records). Waits up to <seconds>, then returns 1.
# The lock is a file holding the PID, created by bash's noclobber (an exclusive open; some mkdir builds, such
# as uutils', do not fail when another process creates the same folder at the same moment). A lock whose
# process is gone, or an empty one over a minute old, is taken over, by one waiter at a time (a second lock,
# lock.takeover, held only for that check), so two waiters never both remove it. Released when the script
# exits, and only if it is still this script's own.
take_lock() {
  local lock=$CRYPT_DIR/lock i=0 pid
  mkdir -p "$CRYPT_DIR" || return 1
  while ! ( set -o noclobber; echo $$ > "$lock" ) 2>/dev/null; do
    if ( set -o noclobber; echo $$ > "$lock.takeover" ) 2>/dev/null; then
      # Read again under the takeover lock: another waiter may have replaced the stale lock meanwhile.
      pid=$(cat "$lock" 2>/dev/null)
      # Removed only if it still names the same PID after the check: the holder read above may have released
      # it and exited meanwhile, and another waiter taken a fresh lock that must stay (seen once on Windows).
      if { { [ -n "$pid" ] && ! kill -0 "$pid" 2>/dev/null; } ||
           { [ -z "$pid" ] && [ -n "$(find "$lock" -mmin +1 2>/dev/null)" ]; }; } &&
         [ -e "$lock" ] && [ "$(cat "$lock" 2>/dev/null)" = "$pid" ]; then
        rm -f "$lock"
      fi
      rm -f "$lock.takeover"
    elif [ -n "$(find "$lock.takeover" -mmin +1 2>/dev/null)" ]; then
      rm -f "$lock.takeover"   # left by a waiter killed during its check, which takes milliseconds
    fi
    [ $i -ge $(( $1 * 10 )) ] && return 1
    sleep 0.1; i=$((i + 1))
  done
  LOCK_HELD=1
  trap release_lock EXIT
}

# Remove the lock if this script holds it (a script with its own EXIT trap calls this from it).
release_lock() {
  [ "$LOCK_HELD" = 1 ] && [ "$(cat "$CRYPT_DIR/lock" 2>/dev/null)" = "$$" ] && rm -f "$CRYPT_DIR/lock"
  LOCK_HELD=0
}

# This machine's OS as Jeeves names it: linux, wsl, macos or windows (Git Bash).
os_name() {
  case "$(uname -s)" in
    Darwin) echo macos ;;
    MINGW*|MSYS*|CYGWIN*) echo windows ;;
    *) if grep -qi microsoft /proc/version 2>/dev/null; then echo wsl; else echo linux; fi ;;
  esac
}
