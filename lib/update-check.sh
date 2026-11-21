#!/usr/bin/env bash
# Tells the user when a newer Jeeves release is out than the one running.
#   jeeves-update-check          at most once a day: one line when a newer release is out, otherwise nothing
#   jeeves-update-check --now    the same, without the daily limit
# It reads the release the marketplace pins new installs to (the "ref" in .claude-plugin/marketplace.json on
# the public repo's main, fetched anonymously with curl, with a deadline) and compares it with
# .claude-plugin/plugin.json. Not the newest tag: a tag comes before its pin, and until the pin moves the
# update commands still deliver the previous release. Any failure (no network, no curl, odd output) is
# silent and waits for the next day. Skipped on a development checkout (listed in ~/.claude/jeeves/dev-checkouts) and
# with JEEVES_UPDATE_CHECK=off. Always exits 0: it runs from the session-start hook.
# Portable bash: macOS bash 3.2, Git Bash, Linux.

. "$(dirname "$0")/lib.sh"

now=0
[ "$1" = --now ] && now=1
[ "$JEEVES_UPDATE_CHECK" = off ] && exit 0

ROOT=$(cd "$(dirname "$0")/.." 2>/dev/null && pwd -P) || exit 0
STATE=$CLAUDE_HOME/jeeves
RECORD=$STATE/update-check
# A path instead of an https URL reads that file (the tests).
URL=${JEEVES_RELEASES_URL:-https://raw.githubusercontent.com/WinuxNomacs/jeeves/main/.claude-plugin/marketplace.json}

is_dev_checkout "$ROOT" && exit 0

# Once a day: the record holds the time of the last check, in seconds. One that does not read, or lies in
# the future (a clock set back), counts as never checked.
t=$(date +%s)
if [ $now = 0 ] && [ -f "$RECORD" ]; then
  last=$(head -n 1 "$RECORD" 2>/dev/null)
  case "$last" in ''|*[!0-9]*) last= ;; esac
  [ -n "$last" ] && [ "$last" -le "$t" ] && [ $((t - last)) -lt 86400 ] && exit 0
fi
# Recorded before the check, so a failing network costs one try a day, not one per session.
mkdir -p "$STATE" 2>/dev/null && printf '%s\n' "$t" > "$RECORD" 2>/dev/null

ours=$(sed -n 's/^[[:space:]]*"version"[[:space:]]*:[[:space:]]*"\([0-9]*\.[0-9]*\.[0-9]*\)".*/\1/p' \
  "$ROOT/.claude-plugin/plugin.json" 2>/dev/null | head -n 1)
[ -n "$ours" ] || exit 0

# Only a ref of exactly vX.Y.Z counts, so nothing else from the remote reaches the message. curl -q skips the
# user's .curlrc.
case "$URL" in
  https://*) pinned=$(with_deadline "${JEEVES_UPDATE_DEADLINE:-10}" \
               curl -q -fsSL --proto =https --max-time "${JEEVES_UPDATE_DEADLINE:-10}" "$URL" 2>/dev/null) ;;
  *) pinned=$(cat "$URL" 2>/dev/null) ;;
esac
latest=$(printf '%s\n' "$pinned" |
  sed -n 's/^[[:space:]]*"ref"[[:space:]]*:[[:space:]]*"v\([0-9]\{1,9\}\.[0-9]\{1,9\}\.[0-9]\{1,9\}\)".*/\1/p' | head -n 1)
[ -n "$latest" ] || exit 0
[ "$latest" = "$ours" ] && exit 0
version_at_least "$ours" "$latest" && exit 0

echo "jeeves: Jeeves $latest is out (this is $ours). To update: 'claude plugin marketplace update jeeves', then" \
  "'claude plugin update jeeves@jeeves', then /reload-plugins; or turn on auto-update in /plugin > Marketplaces." \
  "What changed: https://github.com/WinuxNomacs/jeeves/blob/main/CHANGELOG.md"
exit 0
