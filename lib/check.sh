#!/usr/bin/env bash
# What /jeeves:setup needs to know about this machine, one fact per line (tab-separated), read-only:
#
#   os           linux | wsl | macos | windows
#   git          ok <version> | missing
#   gh           ok <version> | missing                 the GitHub CLI
#   gh-auth      ok <login> | fail <why> | skip          signed in to GitHub through gh
#   gh-protocol  https | ssh | skip                      how gh clones (its git_protocol setting)
#   ssh-github   ok <login> | fail <why> | skip          an SSH key GitHub accepts
#   gh-install   <command> | none                        how to install gh here (run only after the user agrees)
#   age          ok <version> | old <version> | missing    age, for encryption (1.3.0 or later, with age-plugin-batchpass)
#   age-install  <command> | none                        how to install age here (none: the release download)
#   keystore     keychain | wincred | secret-service | none  where an encryption key would be kept
#   datarepo     <path> repo | empty | missing | other
#
# datarepo: the data repo folder ($JEEVES_DATA, the plugin option data_repo, or ~/jeeves-data), ~/ expanded. repo: a git
# repo; empty: an empty folder; missing: nothing there; other: a folder with files that is not a git repo.
# Each network check has a deadline. JEEVES_GH and JEEVES_SSH name the gh and ssh commands (tests use them).

. "$(dirname "$0")/lib.sh"

[ $# = 0 ] || { echo "check: takes no options" >&2; exit 2; }
gh=${JEEVES_GH:-gh} ssh=${JEEVES_SSH:-ssh}
line() { local IFS='	'; printf '%s\n' "$*"; }
first_word() { sed -n '1s/^[^0-9]*\([0-9][0-9.]*\).*/\1/p'; }

os=$(os_name)
line os "$os"

if command -v git >/dev/null 2>&1; then line git ok "$(git --version | first_word)"; else line git missing; fi

if command -v "$gh" >/dev/null 2>&1; then
  line gh ok "$("$gh" --version 2>/dev/null | first_word)"
  if login=$(with_deadline 15 "$gh" api user --jq .login 2>/dev/null) && [ -n "$login" ]; then
    line gh-auth ok "$login"
  else
    line gh-auth fail "not signed in, or GitHub could not be reached"
  fi
  protocol=$("$gh" config get git_protocol 2>/dev/null)
  case "$protocol" in ssh) line gh-protocol ssh ;; *) line gh-protocol https ;; esac
else
  line gh missing; line gh-auth skip; line gh-protocol skip
fi

# GitHub answers an accepted key with "Hi <login>! You've successfully authenticated" and exit status 1.
# StrictHostKeyChecking=yes: never adds GitHub's host key to known_hosts on the user's behalf.
if command -v "$ssh" >/dev/null 2>&1; then
  out=$(with_deadline 10 "$ssh" -T -o BatchMode=yes -o ConnectTimeout=5 -o StrictHostKeyChecking=yes git@github.com 2>&1)
  rc=$?
  login=$(printf '%s\n' "$out" | sed -n 's/^Hi \([^!]*\)! You.ve successfully authenticated.*/\1/p' | head -1)
  if [ -n "$login" ]; then line ssh-github ok "$login"
  elif [ $rc = 124 ]; then line ssh-github fail "no answer from github.com within 10 seconds"
  elif printf '%s' "$out" | grep -qi "host key verification failed"; then line ssh-github fail "github.com is not in known_hosts"
  else line ssh-github fail "no SSH key that GitHub accepts"
  fi
else
  line ssh-github skip
fi

install=none
case "$os" in
  macos) command -v brew >/dev/null 2>&1 && install="brew install gh" ;;
  windows) { command -v winget >/dev/null 2>&1 || command -v winget.exe >/dev/null 2>&1; } && install="winget install --id GitHub.cli -e" ;;
  *)
    if command -v apt-get >/dev/null 2>&1; then install="sudo apt install gh"
    elif command -v dnf >/dev/null 2>&1; then install="sudo dnf install gh"
    elif command -v pacman >/dev/null 2>&1; then install="sudo pacman -S github-cli"
    elif command -v brew >/dev/null 2>&1; then install="brew install gh"
    fi ;;
esac
line gh-install "$install"

v=$(age_version)
if [ -z "$v" ]; then line age missing
elif version_at_least "$v" "$AGE_MIN_VERSION" && command -v age-plugin-batchpass >/dev/null 2>&1; then line age ok "$v"
else line age old "$v"
fi
# Linux packages are often older than 1.3 (Ubuntu 24.04 apt: 1.2.1), so only Homebrew is offered there.
install=none
case "$os" in
  windows) { command -v winget >/dev/null 2>&1 || command -v winget.exe >/dev/null 2>&1; } && install="winget install --id FiloSottile.age -e" ;;
  *) command -v brew >/dev/null 2>&1 && install="brew install age" ;;
esac
line age-install "$install"
line keystore "$(keystore_kind)"

dir=$(expand_path "${JEEVES_DATA:-${CLAUDE_PLUGIN_OPTION_DATA_REPO:-$DEFAULT_DATA_REPO}}")
if [ ! -e "$dir" ]; then line datarepo "$dir" missing
elif [ -e "$dir/.git" ]; then line datarepo "$dir" repo
elif [ -d "$dir" ] && [ -z "$(ls -A "$dir" 2>/dev/null)" ]; then line datarepo "$dir" empty
else line datarepo "$dir" other
fi
exit 0
