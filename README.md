# Jeeves

A Claude Code plugin that keeps the version history of your global Claude Code preferences
(`~/.claude/CLAUDE.md`, `settings.json`, `keybindings.json`, `agents/`, `commands/`, `skills/`) in a git repo
of your own (the **data repo**), shared across your devices.

The live files stay in `~/.claude`. The data repo keeps copies under `home/`. This plugin brings the scripts
that move the copies, and hooks that run them for you:

| Hook | When | What it runs |
|---|---|---|
| SessionStart | every session start | `jeeves-snapshot --hook`: pulls the data repo, records what changed in `~/.claude`, commits on `main` |
| PostToolUse (`Write\|Edit`) | right after Claude edits `~/.claude/CLAUDE.md` | `jeeves-snapshot --hook --no-pull`, reported as a system message |

In hook mode the scripts print only when something needs attention, and they never block a session.

## The `update` skill

`/jeeves:update` syncs and pushes; `/jeeves:update <change>` (for example
`/jeeves:update always answer in British English`) makes a preference change. Claude also starts it by
itself when you ask to change a global preference, or when a snapshot reports newer repo files or a conflict.

- **Sync:** snapshot, merge another device's diverged `main`, merge files changed both on disk and in the repo
  (three-way, from the version both last shared), install what is newer in the repo, then push.
- **Change:** a sync, then the change on a branch of the data repo (`home/…`), merged into `main`, installed
  into `~/.claude` and pushed. Editing the copy avoids the prompt Claude Code shows for a direct edit of
  `~/.claude`.

It pushes without asking and shows no diff unless you ask for one: every step is a commit. It stops and asks
whenever text could be lost: a passage changed differently on both sides, a rejected push (it never
force-pushes), or a possible secret found by `jeeves-scan` in what it is about to push.

## Commands

The plugin's `bin/` is on the PATH of Claude's Bash tool, so Claude (or you, by giving the full path) runs:

| Command | Direction | What it does |
|---|---|---|
| `jeeves-snapshot` | `~/.claude` → data repo | Fast-forwards `main` from its remote, copies changed files into `home/`, backs up memories, commits on `main`. Never pushes. |
| `jeeves-apply` | data repo → `~/.claude` | Installs files that changed in the data repo since this device last synced. Never overwrites a file a session edited since then. |
| `jeeves-status` | (read-only) | Lists each path out of sync (`disk`, `repo` or `conflict`) with its disk, repo and last-synced hashes, and how `main` stands against `origin/main`. |
| `jeeves-scan` | (read-only) | Looks for secrets (private keys, API keys and tokens, password assignments) in the commits a push would send; lists file and line, never the value. |

Each device keeps a gitignored record (`.sync-state/` in the data repo) of the file hashes at its last sync.
This lets the scripts tell "this device changed it" from "the repo changed it". If both changed, they report a
conflict and change nothing. Settle a conflict with `jeeves-snapshot --force` (keep the disk copy) or
`jeeves-apply --force` (keep the repo copy; the disk copy is saved under `.sync-state/overwritten/`), or let
`/jeeves:update` merge the two.

`jeeves-snapshot` only commits onto a clean `main`. While the data repo is on another branch or has
uncommitted changes, it skips and says so.

## The data repo

A git repo with a `home/` folder (copies of the tracked files) and a `memory/<host>/` folder (a backup of
each project's `~/.claude/projects/<project>/memory/` on that device, never applied back, because the folder
names are absolute paths and memories hold machine-specific facts). Never copied: credentials,
`~/.claude.json`, history, transcripts, caches. Its `.gitignore` should hold `.sync-state/` and `*.sync-tmp`.

The scripts find it, in this order: `$JEEVES_DATA`; the plugin option `data_repo`; the git repo around the
current folder, if it has a `home/` folder. Hooks see the plugin option; Claude's Bash tool does not, so run
the commands inside the data repo or export `JEEVES_DATA`.

## Install

From a local clone (edits to the clone take effect on `/reload-plugins`):

```sh
git clone git@github.com:WinuxNomacs/jeeves.git ~/gitrepos/config/jeeves
claude plugin marketplace add ~/gitrepos/config/jeeves
claude plugin install jeeves@jeeves
```

Or from GitHub (a private repo works with your own git credentials):
`claude plugin marketplace add WinuxNomacs/jeeves`, then `claude plugin install jeeves@jeeves`.

Then set the data repo path when Claude Code asks for `data_repo`, or later in `/plugin` or `/config`.
Requirements: git and bash (on Windows, Git Bash).

## Adding a device

Install the plugin, clone your data repo and set `data_repo` to it. The first session's snapshot reports
any file that differs from the repo as a conflict; run `/jeeves:update` to merge them, or choose per file
with `--force` on `jeeves-snapshot` or `jeeves-apply`.

## Tests

`tests/run.sh` (or `tests/run.sh <part of a test name>`). Each test uses a scratch `~/.claude`, data repo and
bare remote, so nothing real is touched.
