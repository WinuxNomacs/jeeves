#!/usr/bin/env bash
# The bash version of jeeves-encrypt (bin/jeeves-encrypt runs the Go build; this runs while it is ported).
# Encrypt the data repo with age, and unlock or lock this device.
#
#   jeeves-encrypt status       encryption off | locked | unlocked, the key store, age, and "history plain" when
#                               origin still holds the plain history from before encryption (tab-separated lines)
#   jeeves-encrypt on           make a key, ask for a new passphrase (twice), encrypt home/ and hosts/, commit
#   jeeves-encrypt unlock       ask for the passphrase, put the key in this device's key store
#   jeeves-encrypt lock         remove the key from this device's key store
#   jeeves-encrypt passphrase   ask for a new passphrase (twice) and commit identity.age encrypted with it
#
# Options for on, unlock and passphrase: --terminal reads the passphrase from the terminal (for a user
# running this in their own terminal, when no dialog can be shown); --key-file keeps the key in a file
# (~/.config/jeeves/keys/, readable only by the user) on a device with no credential store.
#
# The passphrase is asked for in a dialog of the OS, never on a command line or in Claude's chat, and goes to
# age only in the environment of that one age process. It is never stored. Exit status 3: no dialog can be
# shown here; the message says what to run in a terminal. `on` and `passphrase` never push: /jeeves:encryption
# pushes (after `on`, with jeeves-cleanup --all, so no plain copy stays in the history on the remote).

ALLOW_LOCKED=1
. "$(dirname "$0")/../lib/lib.sh"

say() { echo "jeeves encrypt: $*"; }
usage() { echo "usage: jeeves-encrypt status | on | unlock | lock | passphrase [--terminal] [--key-file]" >&2; exit 2; }
cmd=$1; [ -n "$cmd" ] && shift
ASK_TERMINAL=0 key_file=0
for arg in "$@"; do
  case "$arg" in
    --terminal) ASK_TERMINAL=1 ;;
    --key-file) key_file=1 ;;
    *) usage ;;
  esac
done
case "$cmd" in status|on|unlock|lock|passphrase) ;; *) usage ;; esac

find_data_repo || { say "$DATA_REPO_PROBLEM"; exit 1; }
mkdir -p "$CRYPT_DIR" || exit 1
tmp=$CRYPT_DIR/tmp/encrypt.$$
mkdir -p "$CRYPT_DIR/tmp" && trap 'rm -f "$tmp".*' EXIT
# on, unlock and passphrase change the data repo: one Jeeves run at a time.
case "$cmd" in on|unlock|passphrase)
  take_lock 30 || { say "another Jeeves run is still at work on $REPO; nothing changed, try again shortly"; exit 1; }
  trap 'rm -f "$tmp".*; release_lock' EXIT ;;
esac
MIN_PASS=12

# A recipient: the working tree's, or origin/main's when another device turned encryption on.
recipient_now() {
  if encrypted; then cat "$REPO/recipient"
  else git_repo show origin/main:recipient 2>/dev/null
  fi
}

check_age() {
  local v
  v=$(age_version)
  [ -n "$v" ] || { say "age is not installed (/jeeves:setup offers to install it)"; exit 1; }
  version_at_least "$v" "$AGE_MIN_VERSION" ||
    { say "age $v is too old: Jeeves needs $AGE_MIN_VERSION or later (with age-plugin-batchpass)"; exit 1; }
  command -v age-plugin-batchpass >/dev/null 2>&1 ||
    { say "age-plugin-batchpass is missing (it comes with age $AGE_MIN_VERSION or later)"; exit 1; }
}

check_store() {
  if [ $key_file = 1 ]; then printf 'file\n' > "$CRYPT_DIR/keystore"; fi
  [ "$(keystore_kind)" != none ] && return 0
  say "this device has no credential store (on Linux: secret-tool and a Secret Service such as GNOME Keyring);" \
      "run again with --key-file to keep the key in a file only you can read"
  exit 1
}

# The passphrase from the user, or exit (cancelled, or no dialog: then say what to run in a terminal).
passphrase() {
  local p rc
  p=$(ask_passphrase "$1"); rc=$?
  if [ $rc = 3 ]; then
    say "no password dialog can be shown here; in your own terminal, run:" >&2
    say "  JEEVES_DATA=\"$REPO\" \"$(cd "$(dirname "$0")/../bin" && pwd)/jeeves-encrypt\" $cmd --terminal" >&2
    exit 3
  fi
  [ $rc = 0 ] && [ -n "$p" ] || { say "cancelled; nothing changed" >&2; exit 1; }
  printf '%s\n' "$p"
}

new_passphrase() {
  local p q
  p=$(passphrase "New passphrase for your Jeeves data (at least $MIN_PASS characters)") || exit $?
  [ ${#p} -ge $MIN_PASS ] || { say "the passphrase is shorter than $MIN_PASS characters; nothing changed" >&2; exit 1; }
  q=$(passphrase "The same passphrase again") || exit $?
  [ "$p" = "$q" ] || { say "the two passphrases differ; nothing changed" >&2; exit 1; }
  printf '%s\n' "$p"
}

# write_identity <key> <passphrase>: identity.age, then a check that it opens with that passphrase.
write_identity() {
  printf '%s\n' "$1" | AGE_PASSPHRASE=$2 "$AGE" -e -j batchpass -a -o "$tmp.identity" || return 1
  [ "$(AGE_PASSPHRASE=$2 "$AGE" -d -j batchpass "$tmp.identity" | grep '^AGE-SECRET-KEY-')" = \
    "$(printf '%s\n' "$1" | grep '^AGE-SECRET-KEY-')" ] || return 1
  mv "$tmp.identity" "$REPO/identity.age"
}

# keep_key <recipient> <key>: into the store, read back.
keep_key() {
  printf '%s\n' "$2" | store_set "$1" && [ "$(store_get "$1")" = "$2" ] ||
    { say "could not keep the key in this device's key store ($(keystore_kind))"; return 1; }
  printf '%s\n' "$1" > "$CRYPT_DIR/unlocked"
}

# Undo a failed `on`: the files, the key and the ciphertext cache (made with a key nobody keeps).
undo_on() {
  git_repo reset -q --hard HEAD; rm -f "$REPO/recipient" "$REPO/identity.age" "$CRYPT_DIR/unlocked"
  rm -rf "$CRYPT_DIR/clean"; store_delete "$1"
}

on_main_clean() {
  [ "$(git_repo symbolic-ref --short -q HEAD)" = main ] || { say "$REPO is not on main"; exit 1; }
  [ ! -f "$REPO/.git/MERGE_HEAD" ] && [ -z "$(git_repo status --porcelain)" ] ||
    { say "$REPO has uncommitted changes; sync first (/jeeves:update)"; exit 1; }
}

case "$cmd" in
  status)
    if encrypted || encrypted_at origin/main; then
      if unlocked; then state=unlocked; else state=locked; fi
    else
      state=off
    fi
    printf 'encryption\t%s\n' "$state"
    printf 'keystore\t%s\n' "$(keystore_kind)"
    # The plain history from before encryption is still on origin (the fold did not get there).
    if encrypted_at origin/main && plain_history_at origin/main; then printf 'history\tplain\n'; fi
    v=$(age_version)
    if [ -z "$v" ]; then printf 'age\tmissing\n'
    elif version_at_least "$v" "$AGE_MIN_VERSION" && command -v age-plugin-batchpass >/dev/null 2>&1; then printf 'age\tok\t%s\n' "$v"
    else printf 'age\told\t%s\n' "$v"
    fi ;;

  on)
    if encrypted || encrypted_at origin/main; then say "the data repo is already encrypted"; exit 1; fi
    on_main_clean; check_age; check_store
    # A cache left by an earlier attempt holds ciphertext for another key.
    rm -rf "$CRYPT_DIR/clean"
    pass=$(new_passphrase) || exit $?
    key=$("$AGE_KEYGEN" 2>/dev/null | grep '^AGE-SECRET-KEY-') || { say "could not make a key"; exit 1; }
    rec=$(printf '%s\n' "$key" | "$AGE_KEYGEN" -y) || { say "could not make a key"; exit 1; }
    write_identity "$key" "$pass" || { say "could not write identity.age; nothing changed"; exit 1; }
    unset pass
    keep_key "$rec" "$key" || { rm -f "$REPO/identity.age"; exit 1; }
    JEEVES_AGE_KEY=$key; export JEEVES_AGE_KEY
    printf '%s\n' "$rec" > "$REPO/recipient"
    cat >> "$REPO/.gitattributes" <<'END'
# Encrypted by Jeeves with age: stored encrypted in git, plain in the working tree.
home/** filter=jeeves diff=jeeves merge=jeeves -text
hosts/** filter=jeeves diff=jeeves merge=jeeves -text
END
    crypt_setup
    git_repo add --renormalize .
    git_repo add -- recipient identity.age .gitattributes
    # Nothing under home/ or hosts/ may be staged in plain text.
    plain=$(git_repo ls-files -s -- home hosts | while read -r _ blob _ path; do
      [ "$(git_repo cat-file blob "$blob" | head -c ${#AGE_MAGIC})" = "$AGE_MAGIC" ] || printf '%s\n' "$path"
    done)
    if [ -n "$plain" ]; then
      undo_on "$rec"
      say "these files did not encrypt, so nothing changed: $(printf '%s\n' "$plain" | join_lines)"; exit 1
    fi
    n=$(git_repo ls-files -- home hosts | wc -l | tr -d ' ')
    git_repo commit -q -m "Encrypt the data repo with age" \
      -m "home/ and hosts/ are stored encrypted from here on; identity.age holds the key, encrypted with the passphrase." ||
      { undo_on "$rec"; say "commit failed, so nothing changed"; exit 1; }
    say "encrypted $n files and committed; this device is unlocked (key in its $(keystore_kind) store)" ;;

  unlock)
    # Encryption another device turned on is known from origin.
    encrypted || remote_git fetch -q origin main 2>/dev/null
    rec=$(recipient_now)
    [ -n "$rec" ] || { say "the data repo is not encrypted"; exit 1; }
    if unlocked && load_key; then say "this device is already unlocked"; exit 0; fi
    check_age; check_store
    if encrypted; then cp "$REPO/identity.age" "$tmp.id"; else git_repo show origin/main:identity.age > "$tmp.id"; fi ||
      { say "identity.age is missing"; exit 1; }
    pass=$(passphrase "Passphrase of your Jeeves data") || exit $?
    key=$(AGE_PASSPHRASE=$pass "$AGE" -d -j batchpass "$tmp.id" 2>/dev/null | grep '^AGE-SECRET-KEY-')
    unset pass
    [ -n "$key" ] || { say "wrong passphrase; nothing changed"; exit 1; }
    [ "$(printf '%s\n' "$key" | "$AGE_KEYGEN" -y)" = "$rec" ] || { say "identity.age does not match the recipient file"; exit 1; }
    keep_key "$rec" "$key" || exit 1
    JEEVES_AGE_KEY=$key; export JEEVES_AGE_KEY
    crypt_setup
    # Files a clone wrote before git could decrypt them: check them out again, decrypted.
    raw=()
    if encrypted; then
      while IFS= read -r f; do is_age "$REPO/$f" && raw+=("$f"); done < <(git_repo ls-files -- home hosts)
    fi
    if [ ${#raw[@]} -gt 0 ]; then
      (cd "$REPO" && rm -f -- "${raw[@]}") && git_repo checkout -q -- "${raw[@]}" ||
        { say "unlocked, but could not decrypt the files in $REPO"; exit 1; }
    fi
    say "unlocked: the key is in this device's $(keystore_kind) store" ;;

  lock)
    rec=$(recipient_now)
    [ -n "$rec" ] || { say "the data repo is not encrypted"; exit 1; }
    store_delete "$rec"; rm -f "$CRYPT_DIR/unlocked"
    say "locked: the key is no longer in this device's key store (the files in ~/.claude and $REPO stay as they are)" ;;

  passphrase)
    encrypted || { say "the data repo is not encrypted"; exit 1; }
    unlocked && load_key || { say "this device is locked: run /jeeves:encryption first"; exit 1; }
    on_main_clean; check_age
    pass=$(new_passphrase) || exit $?
    write_identity "$JEEVES_AGE_KEY" "$pass" || { say "could not write identity.age; nothing changed"; exit 1; }
    unset pass
    git_repo add -- identity.age && git_repo commit -q -m "Change the passphrase of the data repo's key" ||
      { say "commit failed"; exit 1; }
    say "new passphrase set and committed; the key is unchanged, so other devices stay unlocked" ;;
esac
