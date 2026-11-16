# Shared by bin/jeeves-snapshot and bin/jeeves-apply. Sourced, not run.
# Portable bash: works on bash 3.2 (macOS), Git Bash (Windows) and Linux. No GNU-only flags,
# no associative arrays, no `set -u` (empty arrays break it on bash < 4.4).

CLAUDE_HOME=${CLAUDE_HOME:-$HOME/.claude}
# This device's name in the data repo. Native Windows (Git Bash) gets "-win", so it stays apart from a WSL
# on the same PC, which reports the same host name.
if [ -n "$JEEVES_HOST" ]; then HOST=$JEEVES_HOST
else
  HOST=$(hostname | cut -d. -f1)
  case "$(uname -s)" in MINGW*|MSYS*|CYGWIN*) HOST=$HOST-win ;; esac
fi

# Find the data repo (the git repo holding home/ and memory/) and set REPO and the paths inside it.
# Order: $JEEVES_DATA, then the plugin option data_repo (hooks see it as CLAUDE_PLUGIN_OPTION_DATA_REPO),
# then the git repo around the current folder if it has a home/ folder (Claude's Bash tool sees no
# plugin variables, but runs inside the data repo when working on it).
# Returns 1 with DATA_REPO_PROBLEM set when none is found.
find_data_repo() {
  local candidate=${JEEVES_DATA:-$CLAUDE_PLUGIN_OPTION_DATA_REPO} top
  case "$candidate" in "~/"*) candidate=$HOME/${candidate#"~/"} ;; esac
  if [ -z "$candidate" ]; then
    top=$(git rev-parse --show-toplevel 2>/dev/null) && [ -d "$top/home" ] && candidate=$top
  fi
  if [ -z "$candidate" ]; then
    DATA_REPO_PROBLEM="no data repo set: set the plugin option data_repo (/plugin, or /config), export JEEVES_DATA, or run this inside the data repo"
    return 1
  fi
  if ! git -C "$candidate" rev-parse --git-dir >/dev/null 2>&1; then
    DATA_REPO_PROBLEM="the data repo $candidate is not a git repository"
    return 1
  fi
  REPO=$(cd "$candidate" && pwd)
  HOME_COPY=$REPO/home
  STATE_DIR=$REPO/.sync-state
  STATE_FILE=$STATE_DIR/hashes
  HOST_DIR=$REPO/hosts/$HOST
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

# Record (or with "-", forget) the synced hash of a path.
set_base() {
  local path=$1 hash=$2 tmp
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
