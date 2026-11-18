# Changelog

A new version reaches installed copies only when `version` in `.claude-plugin/plugin.json` changes.

## 1.1.0 (2026-11-18)

**Your Claude Code, a little wiser with every session.** The session review now learns from your
memories too, and your data is guarded at every step.

- **Memories feed the review:** `/jeeves:update` also reads the memories Claude saved on any of your devices
  (this machine's, and the others' from their backups) and offers what holds everywhere as CLAUDE.md rules,
  saying which memory each came from. Memories themselves are never changed; a shared record
  (`hosts/.memory-review`) keeps a memory from being offered twice. Up to 25 files per run, so a long
  backlog is worked through over a few runs. New command: `jeeves-memories`.
- **Settings backup is opt-in:** `settings.json` and `keybindings.json` are backed up only with
  `backup_settings=yes` in `jeeves.conf`. Without it, the next snapshot removes the old copies from
  `hosts/<host>/` once and says so (the git history keeps them until a history cleanup). Memories are still
  backed up.
- **Nothing corrupt is committed:** what a snapshot commits is checked, decrypted, against what it copied; a
  failed copy or backup stops the run with nothing recorded; a file still being written is left for the next
  snapshot; files are kept byte for byte (existing data repos get `* -text` in a commit of their own, so a
  file with Windows line endings is stored as it is). Files holding conflict markers are never recorded,
  installed or pushed unless you name them (`--force <path>`).
- **Nothing reaches `~/.claude` from an unfinished state:** `jeeves-apply` installs only from a clean `main`
  with no merge in progress, re-checks each file just before replacing it, and records only a copy that is
  in place.
- **Folders and devices:** a shared folder that vanishes whole (moved, or a link to a missing drive) is held
  back until you confirm the deletion (`--allow-delete`); two machines with the same name no longer
  overwrite each other's backup (`--replace-host` when one replaces the other); a host name starting with a
  dot is refused.
- **Smaller fixes:** your own git ignore list (`.DS_Store`, `*.swp`) no longer keeps a shared file out of
  the data repo; a merge of an encrypted file that cannot be decrypted stops with a marked note instead of
  taking one side; cleanup, encrypt and restore take the lock too, and a stale lock is taken over safely.

## 1.0.1 (2026-11-17)

Fixes from the first full review for bugs, interaction breakdowns and secret leaks.

- **Secrets:** `jeeves-scan` now checks each commit a push would send, so a secret committed and removed
  again, or committed on a merged branch, can no longer reach GitHub unnoticed; a finding names its commit,
  and `/jeeves:update` explains how to fold the unpushed commits once the secret is removed.
- **Shared files are never deleted by accident:** a moved or emptied `~/.claude` is no longer recorded as
  "delete everything on every device"; a real deletion is reported as one, by the snapshot and on the other
  devices. `CLAUDE_CONFIG_DIR` is honoured.
- **Session start never hangs:** every call to GitHub stops after 30 seconds and opens no credential
  window; your own SSH command (`core.sshCommand`, `GIT_SSH_COMMAND`) is kept. Two sessions starting
  together take turns instead of racing. The session-start message no longer reports routine commits.
- **A project repo is never mistaken for the data repo** (one with a `home/` folder, when `data_repo` is unset).
- **Recovery:** a snapshot that failed or was stopped half-way is picked up by the next one instead of
  blocking every later snapshot; an empty host name no longer wipes the device backups.
- **Encryption:** a failed `/jeeves:encrypt` leaves nothing behind and a retry works (before, a retry could
  commit files under a key nobody kept); plain history left on GitHub after an encryption whose fold was not
  pushed is reported until `/jeeves:encrypt` removes it; `/jeeves:encrypt passphrase` says what a new
  passphrase does not protect and offers to fold the old `identity.age` out of the history; merge bases of
  encrypted files are no longer pruned; the key store accepts only a real age recipient.

## 1.0.0 (2026-11-17)

- **First stable release.** Every feature is now checked on Linux, Windows (Git Bash) and macOS (bash 3.2):
  sync, the edit hook, restore, history cleanup, encryption with the macOS Keychain and dialog, and the
  `/jeeves:setup` walk-through on a fresh machine. No change in behaviour since 0.7.0.

## 0.7.0 (2026-11-17)

- **Jeeves speaks as Jeeves.** Every `/jeeves:` command answers as a wise and gently whimsical Gilded Age
  butler, in the prose only (commands, code and files stay plain), gives anything meant to be pasted
  elsewhere as a copy block, and offers choices as clickable options.
- `/jeeves:cleanup` and `/jeeves:encrypt` say plainly that the history they fold is gone for good once this
  device's copy is replaced, and ask with "remove permanently" in the answer.

## 0.6.0 (2026-11-17)

- **Encryption with a passphrase.** `/jeeves:encrypt` encrypts the data repo's `home/` and `hosts/` with
  age (1.3.0 or later), so GitHub holds only encrypted files, and folds the earlier plain history into one
  encrypted commit. The files on your devices stay plain. The passphrase is asked for in a window of the
  system, never in the chat, and is never stored; each device unlocks once (`/jeeves:unlock`) and then keeps
  the key in its credential store (macOS Keychain, Windows Credential Manager, a Linux Secret Service, or a
  key file if you choose). `/jeeves:lock` removes it; `/jeeves:encrypt passphrase` changes the passphrase.
- `/jeeves:setup` offers encryption for a new data repo (before anything is pushed) and unlocks an encrypted
  one it clones.
- New commands `jeeves-encrypt` and `jeeves-crypt` (git's filter for an encrypted data repo);
  `jeeves-cleanup --all`; `jeeves-check` reports age and the key store; `jeeves-scan` refuses a file that
  would be pushed unencrypted.
- A snapshot whose `git add` fails now records nothing (it used to mark the files as synced).

## 0.5.0 (2026-11-17)

- **Guided setup.** `/jeeves:setup` walks you through five steps: tools, GitHub access (it offers to install
  the GitHub CLI and has you sign in with `! gh auth login`), your data repo (found on GitHub by its
  `jeeves-data` topic, or created as a private `jeeves-data` repo), settings from another machine, first sync.
- The `data_repo` option defaults to `~/jeeves-data`, and Jeeves uses that folder when the option is unset.
  A machine not set up yet is told so at each session start, with `/jeeves:setup` to run.
- New commands `jeeves-check` (read-only facts for setup) and `jeeves-github` (`find`, `create <name>`).
- Every call to GitHub from these commands has a deadline.

## 0.4.0 (2026-11-17)

- **Session review in `/jeeves:update`.** When you run it, Claude first goes through the session for what it
  learned and lists the rules it would save, per destination (global `CLAUDE.md`, the project's `CLAUDE.md`,
  memory), with "All of these"; it writes what you pick (global with the sync and push, the project's
  committed in the project, never pushed) and then offers to discuss the ones you left out. Not when Claude
  starts the skill by itself.

## 0.3.0 (2026-11-17)

- **History cleanup.** `/jeeves:cleanup <days>` folds the data repo's commits older than that many days into
  one first commit holding the files as they were then; newer commits and every file stay as they were. It
  asks first, keeps the removed history on that device in `.sync-state/history-before-cleanup.bundle`, and
  force-pushes `main` with a lease (refused, and `main` put back, if another device pushed meanwhile).
- **Automatic cleanup:** `jeeves.conf` in the data repo (`keep_days`, `auto_every_days`) turns it on; the
  session-start hook runs it when due. `/jeeves:cleanup auto <every> [<keep>]` and `off` set it.
- Other devices switch to a cleaned-up history at their next snapshot, recording again any change they had
  not pushed (or reporting a conflict).
- New command `jeeves-cleanup` (`<days>`, `--dry-run <days>`, `--auto`).

## 0.2.2 (2026-11-17)

- `/jeeves:setup` always asks whether to restore another device's backup, and waits for the answer.
- On a machine without a data repo, the session-start message and `/jeeves:update` point to `/jeeves:setup`
  instead of the update skill cloning the repo itself (which skipped the restore question).

## 0.2.1 (2026-11-17)

- The marketplace gives the plugin source as `./`: older Claude Code versions refused to add the marketplace
  with `"."` (`plugins.0.source: Invalid input`).

## 0.2.0 (2026-11-16)

- **Shared vs per device.** Only `CLAUDE.md`, `agents/`, `commands/` and `skills/` are synced between devices
  (`home/` in the data repo). `settings.json`, `keybindings.json` and memories are backed up per device under
  `hosts/<host>/`, laid out as in `~/.claude`, and never applied automatically. A data repo in the 0.1.0
  layout (`home/settings.json`, `memory/<host>/`) is moved by each device's next snapshot.
- New `/jeeves:setup` (clone or create the data repo, first sync) and `/jeeves:restore <host>` (copy a
  device's backup onto this machine and fix its machine-specific paths), with the commands `jeeves-init` and
  `jeeves-restore`.
- Windows: native Windows hosts are named `<host>-win`; the edit hook takes `C:\` paths; line endings stay LF
  (`.gitattributes`, no `core.autocrlf` on the data repo). Tests pass under Git Bash.
- Shared files whose names Windows or macOS can't hold are not taken, with a message saying why. The
  `.trash/` folders where Claude Code keeps removed skills are never copied.
- `/jeeves:update` covers only the shared files.
- MIT licence; install from GitHub (`claude plugin marketplace add WinuxNomacs/jeeves`); a test workflow for
  Linux, Windows and macOS.

## 0.1.0 (2026-11-16)

- First version: `jeeves-snapshot` and `jeeves-apply` with a three-way, per-device sync record; a
  SessionStart hook and an edit hook for `~/.claude/CLAUDE.md`; memories backed up per device.
- `/jeeves:update`: sync (merging diverged `main` and both-sides changes) or a preference change, then a
  secret scan (`jeeves-scan`) and a push; `jeeves-status` shows what is out of sync.
