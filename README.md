# Jeeves

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
| SessionStart | every session start | `jeeves-snapshot --hook`: pulls the data repo, records what changed in `~/.claude`, backs up this device, commits on `main` |
| PostToolUse (`Write\|Edit`) | right after Claude edits `~/.claude/CLAUDE.md` | `jeeves-snapshot --hook --no-pull`, reported as a system message |

In hook mode the scripts print only when something needs attention, and they never block a session.

## Skills

| Skill | Started by | What it does |
|---|---|---|
| `/jeeves:setup [url]` | you, after installing | Clones your data repo (or creates one), offers to restore another device's backup on a new machine, then runs the first sync and backup. |
| `/jeeves:update [change]` | you, or Claude | Syncs the shared files and pushes; with a change, makes it. See below. |
| `/jeeves:restore <host>` | you | Copies a device's backed-up `settings.json`, `keybindings.json` (and on request memories) onto this machine, then fixes paths that only worked there (`data_repo`, a local marketplace folder) and lists other machine-specific values. Replaced files are saved first. |

### The `update` skill

`/jeeves:update` syncs and pushes; `/jeeves:update <change>` (for example
`/jeeves:update always answer in British English`) makes a preference change. Claude also starts it by
itself when you ask to change a global preference, or when a snapshot reports newer repo files or a conflict.

- **Sync:** snapshot, merge another device's diverged `main`, merge files changed both on disk and in the repo
  (three-way, from the version both last shared), install what is newer in the repo, then push.
- **Change:** a sync, then the change on a branch of the data repo (`home/…`), merged into `main`, installed
  into `~/.claude` and pushed. Editing the copy avoids the prompt Claude Code shows for a direct edit of
  `~/.claude`. Only shared files change this way; `settings.json` and `keybindings.json` you change in
  `~/.claude` as usual, and the next snapshot backs them up.

It pushes without asking and shows no diff unless you ask for one: every step is a commit. It stops and asks
whenever text could be lost: a passage changed differently on both sides, a rejected push (it never
force-pushes), or a possible secret found by `jeeves-scan` in what it is about to push.

## Commands

The plugin's `bin/` is on the PATH of Claude's Bash tool, so Claude (or you, by giving the full path) runs:

| Command | Direction | What it does |
|---|---|---|
| `jeeves-snapshot` | `~/.claude` → data repo | Fast-forwards `main` from its remote, copies changed shared files into `home/`, mirrors this device's settings, keybindings and memories into `hosts/<host>/`, commits on `main`. Never pushes. |
| `jeeves-apply` | data repo → `~/.claude` | Installs shared files that changed in the data repo since this device last synced. Never overwrites a file a session edited since then. |
| `jeeves-restore [<host> [--memories]]` | data repo → `~/.claude` | Copies a device's backup onto this machine, saving replaced files under `.sync-state/overwritten/`. Without a host, prints this machine's host name and the hosts with a backup. |
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
Git Bash.

```sh
claude plugin marketplace add WinuxNomacs/jeeves
claude plugin install jeeves@jeeves
```

Claude Code asks for `data_repo`: the folder where your data repo is (or will be) on this machine, for
example `~/gitrepos/config/jeeves-data`. Then run `/jeeves:setup` (with your data repo's git URL if you
already have one) in a new session.

The VS Code extension has no `/plugin` command: run the commands above with the `claude` executable the
extension ships (on Windows,
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
