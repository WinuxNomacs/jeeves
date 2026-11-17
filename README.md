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

The live files stay in `~/.claude`. This plugin brings the scripts that move the copies, and hooks that run
them for you:

| Hook | When | What it runs |
|---|---|---|
| SessionStart | every session start | `jeeves-snapshot --hook`: pulls the data repo, records what changed in `~/.claude`, backs up this device, commits on `main`; then the automatic history cleanup, if it is turned on and due |
| PostToolUse (`Write\|Edit`) | right after Claude edits `~/.claude/CLAUDE.md` | `jeeves-snapshot --hook --no-pull`, reported as a system message |

In hook mode the scripts print only when something needs attention, and they never block a session.

## Skills

| Skill | Started by | What it does |
|---|---|---|
| `/jeeves:setup [url]` | you, after installing | Clones your data repo (or creates one), offers to restore another device's backup on a new machine, then runs the first sync and backup. |
| `/jeeves:update [change]` | you, or Claude | Syncs the shared files and pushes; with a change, makes it. See below. |
| `/jeeves:cleanup <days>` | you | Removes history older than that many days (1 or more), keeping every file, after a fresh backup and your confirmation; `/jeeves:cleanup auto <every> [<keep>]` or `off` sets the automatic cleanup. See below. |
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

- It takes a fresh backup first, says how many commits it will fold and up to which date, and asks you.
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

## Commands

The plugin's `bin/` is on the PATH of Claude's Bash tool, so Claude (or you, by giving the full path) runs:

| Command | Direction | What it does |
|---|---|---|
| `jeeves-snapshot` | `~/.claude` → data repo | Fast-forwards `main` from its remote, copies changed shared files into `home/`, mirrors this device's settings, keybindings and memories into `hosts/<host>/`, commits on `main`. Never pushes. |
| `jeeves-apply` | data repo → `~/.claude` | Installs shared files that changed in the data repo since this device last synced. Never overwrites a file a session edited since then. |
| `jeeves-restore [<host> [--memories]]` | data repo → `~/.claude` | Copies a device's backup onto this machine, saving replaced files under `.sync-state/overwritten/`. Without a host, prints this machine's host name and the hosts with a backup. |
| `jeeves-cleanup <days>` | data repo → origin | Folds the history older than `<days>` days into one root commit (same files) and force-pushes `main` with a lease; `--dry-run <days>` only says what it would do; `--auto` is what the session-start hook runs. |
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
- `.gitignore`: `.sync-state/` and `*.sync-tmp`; `.gitattributes`: `* text=auto eol=lf`.

`<host>` is the machine's host name; on native Windows it gets `-win` (`mypc-win`), so a WSL and a Windows
Claude Code on one PC stay apart. `JEEVES_HOST` overrides it. A data repo made before per-device backups
(with `home/settings.json` and `memory/<host>/`) is moved to this layout by each device's next snapshot.

The scripts find it, in this order: `$JEEVES_DATA`; the plugin option `data_repo`; the git repo around the
current folder, if it has a `home/` folder. Hooks see the plugin option; Claude's Bash tool does not, so run
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

Claude Code asks for `data_repo`: the folder where your data repo is (or will be) on this machine, for
example `~/gitrepos/config/jeeves-data`. Then run `/jeeves:setup` (with your data repo's git URL if you
already have one) in a new session.

The VS Code extension has no `/plugin` command: run the commands above in a terminal, with `claude` if it is
installed, or else with the executable the extension ships (on Windows,
`%USERPROFILE%\.vscode\extensions\anthropic.claude-code-<version>-win32-x64\resources\native-binary\claude.exe`),
adding `--config data_repo=<path>` to the install, then start a new conversation.

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

Install the plugin, set `data_repo` to where the data repo should be on this machine, and run
`/jeeves:setup <url of your data repo>`. It clones it, offers to restore another device's settings, and
merges this machine's shared files with the repo's (keeping what both hold, asking before dropping text).

## Tests

`tests/run.sh` (or `tests/run.sh <part of a test name>`). Each test uses a scratch `~/.claude`, data repo and
bare remote, so nothing real is touched.

## License

[MIT](LICENSE)
