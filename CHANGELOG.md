# Changelog

A new version reaches installed copies only when `version` in `.claude-plugin/plugin.json` changes.

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
