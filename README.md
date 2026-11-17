# Jeeves

![Jeeves, a butler in a dark tailcoat and white bow tie](docs/images/jeeves-butler.jpg)

A Claude Code plugin that keeps the version history of your global Claude Code preferences in a git repo of
your own (the **data repo**), on Linux, macOS and Windows:

- **Shared** by all your devices and kept in sync: `~/.claude/CLAUDE.md`, `agents/`, `commands/`, `skills/`
  (not `skills/synced/`, which claude.ai manages, nor the `.trash/` folders where Claude Code puts what it
  removed). Copies under `home/` in the data repo.
- **Backed up per device**, never synced: `settings.json`, `keybindings.json` (they hold paths and key
  bindings that differ per machine) and each project's memories. Copies under `hosts/<host>/`, which you can
  restore onto a new machine with `/jeeves:restore`.
- **Encrypted** if you want: with a passphrase, GitHub holds only encrypted copies, while the files on your
  devices stay plain. Each device asks for the passphrase once.

The live files stay in `~/.claude`. This plugin brings the scripts that move the copies, and hooks that run
them for you:

| Hook | When | What it runs |
|---|---|---|
| SessionStart | every session start | `jeeves-snapshot --hook`: pulls the data repo, records what changed in `~/.claude`, backs up this device, commits on `main`; then the automatic history cleanup, if it is turned on and due |
| PostToolUse (`Write\|Edit`) | right after Claude edits `~/.claude/CLAUDE.md` | `jeeves-snapshot --hook --no-pull`, reported as a system message |

In hook mode the scripts print only when something needs attention, and they never block a session.

## Skills

Every `/jeeves:` command answers as Jeeves, a wise and gently whimsical Gilded Age butler (in the prose
only: commands, code and files stay plain), gives anything you would paste elsewhere as a copy block, and
offers choices as clickable options.

| Skill | Started by | What it does |
|---|---|---|
| `/jeeves:setup [url]` | you, after installing | Walks you through setting up this machine in five steps: checks git and your GitHub sign-in (offers to install the GitHub CLI), finds your data repo on GitHub or creates a private one (offering to encrypt it), unlocks an encrypted one, offers to restore another device's backup, then runs the first sync and backup. |
| `/jeeves:update [change]` | you, or Claude | Syncs the shared files and pushes; with a change, makes it. See below. |
| `/jeeves:cleanup <days>` | you | Removes history older than that many days (1 or more), keeping every file, after a fresh backup and your confirmation; `/jeeves:cleanup auto <every> [<keep>]` or `off` sets the automatic cleanup. See below. |
| `/jeeves:encrypt [passphrase]` | you | Encrypts the data repo with a passphrase and removes the plain history from GitHub; with `passphrase`, changes the passphrase. See below. |
| `/jeeves:unlock` / `/jeeves:lock` | you | Unlocks this device with the passphrase (once; the key then stays in its credential store), or removes the key from it. |
| `/jeeves:restore <host>` | you | Copies a device's backed-up `settings.json`, `keybindings.json` (and on request memories) onto this machine, then fixes paths that only worked there (`data_repo`, a local marketplace folder) and lists other machine-specific values. Replaced files are saved first. |

### The `update` skill

`/jeeves:update` syncs and pushes; `/jeeves:update <change>` (for example
`/jeeves:update always answer in British English`) makes a preference change. Claude also starts it by
itself when you ask to change a global preference, or when a snapshot reports newer repo files or a conflict.

- **Session review** (only when you run it, not when Claude starts it): first, Claude goes through the
  session, most recent first, for what it learned (your corrections and constraints, approaches you
  confirmed, mistakes and how to avoid them, setups that took work). It sorts each item into the global
  `CLAUDE.md` (holds in any repo), the project's `CLAUDE.md` (anyone working in that repo needs it; nothing
  private, since it may be public) or memory (personal or this machine only), worded as the rule it would
  become, revising an existing rule where one covers it. You pick from a list per file, with "All of these";
  the global ones are written and pushed with the sync, the project ones committed in the project (on your
  feature branch, or a short branch merged into the default one; never pushed). Afterwards it offers to
  discuss the ones you left out.

- **Sync:** snapshot, merge another device's diverged `main`, merge files changed both on disk and in the repo
  (three-way, from the version both last shared), install what is newer in the repo, then push.
- **Change:** a sync, then the change on a branch of the data repo (`home/…`), merged into `main`, installed
  into `~/.claude` and pushed. Editing the copy avoids the prompt Claude Code shows for a direct edit of
  `~/.claude`. Only shared files change this way; `settings.json` and `keybindings.json` you change in
  `~/.claude` as usual, and the next snapshot backs them up.

It pushes without asking and shows no diff unless you ask for one: every step is a commit. It stops and asks
whenever text could be lost: a passage changed differently on both sides, a rejected push (it never
force-pushes), or a possible secret found by `jeeves-scan` in what it is about to push.

### History cleanup

The data repo gains a commit for every change, on every device. To keep it small and readable over the
years, `/jeeves:cleanup <days>` folds the commits on `main` older than that many days into one first commit
holding the files as they were then. Newer commits stay as they were (files, authors, dates, messages), and no
file changes, on disk or in the repo.

- It takes a fresh backup first, says how many commits it will fold and up to which date, and asks you,
  saying plainly that they will be gone for good once this device's copy (below) is replaced.
- It force-pushes `main` with a lease: if another device pushed in the meantime, the push is refused and
  nothing changes. Any failure puts `main` back. This is the only force-push in Jeeves.
- The removed history stays on that device as `.sync-state/history-before-cleanup.bundle` (never pushed)
  until the next cleanup replaces it; `git clone <bundle>` opens it.
- Other devices notice the new history at their next snapshot and switch to it, keeping their own old
  history in the same bundle. A change a device had not pushed is recorded again from its disk, or reported
  as a conflict if the other side changed the same file.
- **Automatic:** `jeeves.conf` at the top of the data repo, shared by all devices, holds
  `keep_days=<n>` and `auto_every_days=<m>` (`m` at least `n`). At session start, when the history's first
  commit is `m` days old, the hook cleans up and pushes. `/jeeves:cleanup auto <m> <n>` writes it.
- GitHub may keep removed commits for a while, and forks keep theirs; that is outside Jeeves. A secret that
  was ever pushed must be revoked.

### Encryption

`/jeeves:encrypt` encrypts everything under `home/` and `hosts/` with [age](https://age-encryption.org)
(1.3.0 or later), so a leaked GitHub token or a repo made public by mistake shows only encrypted files. The
data repo's folder and `~/.claude` on each device stay plain, as they must for Claude Code to read them.
Still visible on GitHub: file names (including the project paths of memories), sizes and commit messages.

- **The key** is an age key made for your data repo. `identity.age` in the repo holds it, encrypted with your
  passphrase; `recipient` holds its public half, which is all that encrypting needs.
- **Your passphrase** is asked for in a window of the system (macOS dialog, Windows credential prompt, zenity
  or kdialog on Linux; elsewhere, Jeeves gives you a command for your own terminal), never in the chat,
  which is saved in transcripts. It is never stored. Losing it on every device means the data on GitHub can't
  be read; a device that is still unlocked can set a new one (`/jeeves:encrypt passphrase`). Generating it
  with a password manager is recommended.
- **Each device unlocks once** (`/jeeves:unlock`, or during `/jeeves:setup`): the key then stays in its
  credential store (macOS Keychain, Windows Credential Manager, a Linux Secret Service through
  `secret-tool`; where there is none, a file only you can read, if you choose it). Jeeves reads it from there
  at most once per command, so it works without asking again. A locked device says so at session start and
  doesn't sync; `/jeeves:lock` removes the key.
- **Turning it on** folds all earlier history into one encrypted commit and force-pushes it with a lease, as
  the history cleanup does, so no plain copy stays in the history on GitHub (GitHub may keep unreachable
  copies for a while). A new data repo encrypted during setup never holds plain text at all.
- git does the work through a filter, a diff driver and a merge driver (`jeeves-crypt`, set up in the data
  repo's `.git/config`), so syncing, merging and `git log -p` work as before. `jeeves-scan` refuses a push
  that would send a file under `home/` or `hosts/` unencrypted.

## Commands

The plugin's `bin/` is on the PATH of Claude's Bash tool, so Claude (or you, by giving the full path) runs:

| Command | Direction | What it does |
|---|---|---|
| `jeeves-snapshot` | `~/.claude` → data repo | Fast-forwards `main` from its remote, copies changed shared files into `home/`, mirrors this device's settings, keybindings and memories into `hosts/<host>/`, commits on `main`. Never pushes. |
| `jeeves-apply` | data repo → `~/.claude` | Installs shared files that changed in the data repo since this device last synced. Never overwrites a file a session edited since then. |
| `jeeves-restore [<host> [--memories]]` | data repo → `~/.claude` | Copies a device's backup onto this machine, saving replaced files under `.sync-state/overwritten/`. Without a host, prints this machine's host name and the hosts with a backup. |
| `jeeves-cleanup <days>` | data repo → origin | Folds the history older than `<days>` days into one root commit (same files) and force-pushes `main` with a lease; `--dry-run <days>` only says what it would do; `--all` folds all of it (after encrypting); `--auto` is what the session-start hook runs. |
| `jeeves-check` | (read-only) | What setup needs to know: OS, git, the GitHub CLI and sign-in, an SSH key GitHub accepts, how to install the CLI and age here, the key store, the data repo folder's state. |
| `jeeves-encrypt status` / `on` / `unlock` / `lock` / `passphrase` | data repo, key store | Encryption: its state on this device; turn it on; put the key in this device's credential store, or remove it; set a new passphrase. Asks for the passphrase in a dialog (`--terminal` in your own terminal). |
| `jeeves-github find` / `create <name>` | GitHub | Lists your data repos on GitHub (topic `jeeves-data`, or named `jeeves-data`), or creates a private one with that topic, through the GitHub CLI. |
| `jeeves-init <folder> [<url>]` | (new repo) | Creates an empty data repo (`.gitignore`, `.gitattributes`, README), optionally with a remote. |
| `jeeves-status` | (read-only) | Lists each path out of sync (`disk`, `repo` or `conflict`) with its disk, repo and last-synced hashes, and how `main` stands against `origin/main`. |
| `jeeves-scan` | (read-only) | Looks for secrets (private keys, API keys and tokens, password assignments) in the commits a push would send; lists file and line, never the value. |

Each device keeps a gitignored record (`.sync-state/` in the data repo) of the file hashes at its last sync.
This lets the scripts tell "this device changed it" from "the repo changed it". If both changed, they report a
conflict and change nothing. Settle a conflict with `jeeves-snapshot --force` (keep the disk copy) or
`jeeves-apply --force` (keep the repo copy; the disk copy is saved under `.sync-state/overwritten/`), or let
`/jeeves:update` merge the two.

`jeeves-snapshot` only commits onto a clean `main`. While the data repo is on another branch or has
uncommitted changes, it skips and says so. It does not take a shared file whose name another system can't
hold (characters Windows forbids such as `:` or `?`, a name ending in a dot or space, or two names that
differ only in case), and says which and why.

## The data repo

A git repo with:

- `home/`: the shared files.
- `hosts/<host>/`: each device's backup, laid out as in `~/.claude` (`settings.json`, `keybindings.json`,
  `projects/<project>/memory/`). Only that device writes its folder. Memories are restored only on request
  and only for projects that exist on the new machine, because their folder names are absolute paths.
- Never copied: credentials, `~/.claude.json`, history, transcripts, caches.
- `jeeves.conf` (optional): the automatic history cleanup's settings.
- `.gitignore`: `.sync-state/` and `*.sync-tmp`; `.gitattributes`: `* text=auto eol=lf` (and, when
  encrypted, the filter for `home/` and `hosts/`).
- When encrypted: `recipient` (the public key) and `identity.age` (the key, encrypted with your passphrase).

`<host>` is the machine's host name; on native Windows it gets `-win` (`mypc-win`), so a WSL and a Windows
Claude Code on one PC stay apart. `JEEVES_HOST` overrides it. A data repo made before per-device backups
(with `home/settings.json` and `memory/<host>/`) is moved to this layout by each device's next snapshot.

The scripts find it, in this order: `$JEEVES_DATA`; the plugin option `data_repo`; the git repo around the
current folder, if it has a `home/` folder; `~/jeeves-data`, the option's default. Hooks see the plugin option; Claude's Bash tool does not, so run
the commands inside the data repo or export `JEEVES_DATA`.

## Install

Requirements: Claude Code (the CLI or an IDE extension; claude.ai and Cowork don't install plugins with a
`bin/` folder), git and bash. On Windows: Git for Windows, since Claude Code runs hooks and its Bash tool in
Git Bash. Claude Code must be recent enough for plugin options: an older one refuses the plugin with
`Unrecognized keys: "displayName", "userConfig"`; run `claude update` first (2.1.304 is known to work).

```sh
claude plugin marketplace add WinuxNomacs/jeeves
claude plugin install jeeves@jeeves
```

Claude Code asks for `data_repo`, the folder for your data repo on this machine: keep the default,
`~/jeeves-data`. Then start a new session and run `/jeeves:setup`. It walks you through the rest, step by
step: it checks your GitHub sign-in (and offers to install the GitHub CLI if you don't have it), creates
your private `jeeves-data` repo on GitHub, or finds the one another device made, and runs the first sync.

The VS Code extension has no `/plugin` command: run the commands above in a terminal, with `claude` if it is
installed, or else with the executable the extension ships (on Windows,
`%USERPROFILE%\.vscode\extensions\anthropic.claude-code-<version>-win32-x64\resources\native-binary\claude.exe`),
adding `--config data_repo=~/jeeves-data` to the install (or another folder), then start a new conversation.

**Updates:** auto-update is off for third-party marketplaces. Update with
`claude plugin marketplace update jeeves` and `claude plugin update jeeves@jeeves`, or turn on auto-update in
`/plugin` > Marketplaces. A new version arrives when `plugin.json`'s `version` changes; see
[CHANGELOG.md](CHANGELOG.md). To stay on one release, add the marketplace as `WinuxNomacs/jeeves#v<version>`.

**Working on Jeeves:** install from a local clone instead, and edits take effect on `/reload-plugins`:

```sh
git clone https://github.com/WinuxNomacs/jeeves.git
claude plugin marketplace add ./jeeves
claude plugin install jeeves@jeeves
```

## Adding a device

Install the plugin and run `/jeeves:setup`. Signed in to GitHub with the GitHub CLI, it finds your data
repo by itself (otherwise give its URL: `/jeeves:setup <url>`), clones it, asks for the passphrase if it is
encrypted, offers to restore another device's settings, and merges this machine's shared files with the repo's (keeping what both hold, asking before
dropping text).

## Tests

`tests/run.sh` (or `tests/run.sh <part of a test name>`). Each test uses a scratch `~/.claude`, data repo and
bare remote, so nothing real is touched. The encryption tests need age 1.3.0 or later on the PATH and are
skipped without it.

## License

[MIT](LICENSE)
