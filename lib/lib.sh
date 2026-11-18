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
# Order: $JEEVES_DATA, then the plugin option data_repo (hooks see it as CLAUDE_PLUGIN_OPTION_DATA_REPO),
# then the git repo around the current folder if it is a data repo (Claude's Bash tool sees no
# plugin variables, but runs inside the data repo when working on it), then DEFAULT_DATA_REPO (the
# option's default, which an install without --config may leave unset).
# Returns 1 with DATA_REPO_PROBLEM set when none is found, or when it is encrypted and this device is locked
# (unless ALLOW_LOCKED=1).
DEFAULT_DATA_REPO=$HOME/jeeves-data

# A path with a leading ~/ (as plugin options are written) made absolute.
expand_path() {
  case "$1" in "~") printf '%s\n' "$HOME" ;; "~/"*) printf '%s\n' "$HOME/${1#"~/"}" ;; *) printf '%s\n' "$1" ;; esac
}

find_data_repo() {
  local candidate top
  candidate=$(expand_path "${JEEVES_DATA:-$CLAUDE_PLUGIN_OPTION_DATA_REPO}")
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
  case "$HOST" in ''|.|..|*/*|*\\*)
    DATA_REPO_PROBLEM="this device's name could not be found (hostname and uname -n gave '$HOST'); set JEEVES_HOST"
    return 1 ;;
  esac
  REPO=$(cd "$candidate" && pwd)
  HOME_COPY=$REPO/home
  STATE_DIR=$REPO/.sync-state
  STATE_FILE=$STATE_DIR/hashes
  HOST_DIR=$REPO/hosts/$HOST
  CRYPT_DIR=$REPO/.git/jeeves CRYPT_TOP=$REPO
  if encrypted || unlocked; then crypt_setup; fi
  if encrypted && ! unlocked && [ "$ALLOW_LOCKED" != 1 ]; then
    DATA_REPO_PROBLEM="the data repo is encrypted and this device is locked: run /jeeves:unlock"
    return 1
  fi
  return 0
}

# ---- Encryption (lib/crypt.sh has the rest) ----

# The data repo's working tree is encrypted (has a recipient).
encrypted() { [ -s "$REPO/recipient" ]; }
# This device holds the key (the marker names the recipient, or encryption is coming from origin).
unlocked() { [ -s "$CRYPT_DIR/unlocked" ] && { ! encrypted || [ "$(cat "$CRYPT_DIR/unlocked")" = "$(cat "$REPO/recipient")" ]; }; }
# The given commit holds an encrypted data repo.
encrypted_at() { git_repo cat-file -e "$1:recipient" 2>/dev/null; }
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
# Per-device files: backed up to hosts/<host>/ (with the memories), never applied automatically.
HOST_FILES="settings.json keybindings.json"
# Never copied: skills/synced/ is managed by claude.ai; .trash/ holds what Claude Code removed.
EXCLUDED_PREFIXES="skills/synced/ skills/.trash/ agents/.trash/ commands/.trash/"

# Files are copied byte for byte, so git must not convert line endings either (Git for Windows sets
# core.autocrlf=true), and its "LF will be replaced by CRLF" warnings must not reach a hook's message.
git_repo() { git -C "$REPO" -c core.autocrlf=false -c core.safecrlf=false "$@"; }

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
  mkdir -p "$STATE_DIR"
  tmp=$STATE_FILE.tmp
  { [ -f "$STATE_FILE" ] && awk -F'\t' -v p="$path" '$2 != p' "$STATE_FILE"
    [ "$hash" = "-" ] || printf '%s\t%s\n' "$hash" "$path"
  } > "$tmp"
  mv "$tmp" "$STATE_FILE"
}

# Copy a file atomically (temp file in the target folder, then rename), creating folders.
copy_file() {
  local src=$1 dst=$2
  mkdir -p "$(dirname "$dst")"
  cp -p "$src" "$dst.sync-tmp" && mv "$dst.sync-tmp" "$dst"
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

# Why a shared path can't be held on every OS (Windows, macOS), or nothing when it can: characters Windows
# forbids, or a name part ending in a dot or space.
bad_name() {
  case "$1" in
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
remote_git() {
  GIT_TERMINAL_PROMPT=0 GCM_INTERACTIVE=never GIT_SSH_COMMAND="ssh -o BatchMode=yes -o ConnectTimeout=5" \
    with_deadline "${JEEVES_REMOTE_DEADLINE:-30}" git -C "$REPO" -c core.autocrlf=false -c core.safecrlf=false "$@"
}

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
  git_repo reflog expire --expire=now --all && git_repo gc -q --prune=now 2>/dev/null
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

# This machine's OS as Jeeves names it: linux, wsl, macos or windows (Git Bash).
os_name() {
  case "$(uname -s)" in
    Darwin) echo macos ;;
    MINGW*|MSYS*|CYGWIN*) echo windows ;;
    *) if grep -qi microsoft /proc/version 2>/dev/null; then echo wsl; else echo linux; fi ;;
  esac
}
