#!/usr/bin/env bash
# Create a new, empty data repo for Jeeves.
#
#   jeeves-init <folder> [<remote url>]
#
# The folder must not exist yet, or be empty. Makes a git repo on main with .gitignore, .gitattributes and a
# README, commits them, and sets origin when a remote URL (of an empty repo) is given. It never pushes:
# /jeeves:update does, after the first snapshot. To use an existing data repo, clone it instead.

say() { echo "jeeves init: $*"; }
[ $# = 1 ] || [ $# = 2 ] || { echo "usage: jeeves-init <folder> [<remote url>]" >&2; exit 2; }
dir=$1 url=$2
case "$dir" in "~/"*) dir=$HOME/${dir#"~/"} ;; esac
if [ -e "$dir" ] && [ -n "$(ls -A "$dir" 2>/dev/null)" ]; then
  say "$dir is not empty; clone an existing data repo there instead, or pick another folder"
  exit 1
fi
mkdir -p "$dir" && cd "$dir" || exit 1
git init -q && git symbolic-ref HEAD refs/heads/main || exit 1

cat > .gitignore <<'END'
# Per-device sync record (hashes at the last sync, overwritten copies); never shared.
.sync-state/
# Interrupted atomic copies.
*.sync-tmp
END
cat > .gitattributes <<'END'
# Byte for byte: no line-ending conversion, also in a Windows checkout (core.autocrlf).
* -text
END
cat > README.md <<'END'
# Jeeves data repo

Global Claude Code preferences, versioned by the [Jeeves](https://github.com/WinuxNomacs/jeeves) plugin.

- `home/`: shared by every device: `~/.claude/` `CLAUDE.md`, `agents/`, `commands/`, `skills/`.
- `hosts/<host>/`: each device's backup of its `settings.json`, `keybindings.json` and memories, laid out
  as in `~/.claude`. Restore one onto a machine with `/jeeves:restore <host>`.
END
git add -A && git commit -q -m "Data repo for Jeeves" || { say "commit failed"; exit 1; }
[ -z "$url" ] || git remote add origin "$url" || exit 1
say "created $(pwd)${url:+ with origin $url}"
