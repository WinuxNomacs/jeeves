# Jeeves

**Your Claude Code, a little wiser with every session.**

Install (then start a new session and run `/jeeves:setup`; details under [Install](#install)):

```sh
claude plugin marketplace add WinuxNomacs/jeeves
claude plugin install jeeves@jeeves
```

Upgrade to the latest release:

```sh
claude plugin marketplace update jeeves
claude plugin update jeeves@jeeves
```

![Jeeves, a smiling elderly butler with a monocle, white bow tie and dark tailcoat](docs/images/jeeves-butler.jpg)

> **Just work as usual.** Now and then, run `/jeeves:update` (or `/jeeves`): Jeeves reviews the session and
> the memories Claude saved in your projects, keeps what is worth learning, and writes it into your
> `CLAUDE.md`. From the next session on, on every device, Claude Code works with what it learned, and it
> keeps getting better.

A Claude Code plugin that turns what your sessions learn into lasting operational knowledge. `/jeeves:update`
reviews the session, and the memories Claude saved on any of your devices, for corrections, lessons and
setups worth keeping, and writes them as rules into your global or project `CLAUDE.md` on its own judgement,
revised in place and pruned of what no longer helps, so the file stays short and useful to the models that
read and maintain it. Every change is reported and can be undone with one revert. Everything lives in a git repo of
your own (the **data repo**), on Linux, macOS and Windows:

- **Shared** by all your devices and kept in sync: `~/.claude/CLAUDE.md`, `agents/`, `commands/`, `skills/`
  (not `skills/synced/`, which claude.ai manages, nor the `.trash/` folders where Claude Code puts what it
  removed). Copies under `home/` in the data repo, with their full history.
- **Backed up per device**, never synced: each project's memories, and, if you turn it on
  (`backup_settings=yes` in `jeeves.conf`), `settings.json` and `keybindings.json`. Copies under
  `hosts/<host>/`, which you can restore onto a new machine with `/jeeves:restore`.
- **Encrypted** if you want: with a passphrase, GitHub holds only encrypted copies, while the files on your
  devices stay plain. Each device asks for the passphrase once.

The live files stay in `~/.claude` (or in `CLAUDE_CONFIG_DIR` when you have moved Claude Code's folder). This plugin brings the scripts that move the copies, and hooks that run
them for you:

| Hook | When | What it runs |
|---|---|---|
| SessionStart | every session start | `jeeves-snapshot --hook`: once a day, says when a newer Jeeves release is out; pulls the data repo, records what changed in `~/.claude`, backs up this device, commits on `main`; then the automatic history cleanup, if it is turned on and due |
| PostToolUse (`Write\|Edit`) | right after Claude edits `~/.claude/CLAUDE.md` | `jeeves-snapshot --hook --no-pull`, reported as a system message when something needs attention |
| SessionStart | every session start | `jeeves-auto --session-start`: what background updates changed or left for you, and until you decide, a one-line reminder of `/jeeves:autoupdate` |
| Stop | each time Claude finishes a reply | `jeeves-auto --hook-stop`: only with background updates on, starts one in the background when it is due; otherwise ends at once |

In hook mode the scripts print only when something needs attention (a conflict, newer files to install, a
shared file deleted for every device, a problem), never a routine commit, and they never block a session:
every call to GitHub stops after 30 seconds.

## Skills

Every `/jeeves:` command answers as Jeeves, a wise and gently whimsical Gilded Age butler (in the prose
only: commands, code and files stay plain), gives anything you would paste elsewhere as a copy block, and
offers choices as clickable options.

| Skill | Started by | What it does |
|---|---|---|
| `/jeeves:setup [url]` | you, after installing | Walks you through setting up this machine in five steps: checks git and your GitHub sign-in (offers to install the GitHub CLI), finds your data repo on GitHub or creates a private one (offering to encrypt it), unlocks an encrypted one, offers to restore another device's backup, then runs the first sync and backup. |
| `/jeeves:update [change]` | you, or Claude | Syncs the shared files and pushes; with a change, makes it. See below. |
| `/jeeves [change]` | you | The short form of `/jeeves:update`: `/jeeves always answer in British English`. Alone, it asks what to do: discuss a rule idea, review and sync, review your rules, or undo a recent change. |
| `/jeeves:review [concern]` | you | Goes over your rules with you, your own firm rules included, for any that hold back the results you want; talks through what you meant and rewrites them once you agree. See below. |
| `/jeeves:cleanup <days>` | you | Removes history older than that many days (1 or more), keeping every file, after a fresh backup and your confirmation; `/jeeves:cleanup auto <every> [<keep>]` or `off` sets the automatic cleanup. See below. |
| `/jeeves:encryption [passphrase \| lock]` | you | Goes by the state: turns on encryption with a passphrase (and removes the plain history from GitHub), unlocks this device once with the passphrase (the key then stays in its credential store), or, on an unlocked device, changes the passphrase or locks the device (removes the key). See below. |
| `/jeeves:autoupdate [on \| off]` | you | Turns background updates on or off on this device (off until you decide; the session start reminds you once a session until you answer). See below. |
| `/jeeves:restore <host>` | you | Copies a device's backed-up `settings.json`, `keybindings.json` (and on request memories) onto this machine, then fixes paths that only worked there (`data_repo`, a local marketplace folder) and lists other machine-specific values. Replaced files are saved first. |

### The `update` skill

`/jeeves:update` syncs and pushes; `/jeeves:update <change>` (for example
`/jeeves:update always answer in British English`) makes a preference change; `/jeeves <change>` is the
same, shorter. Say what you want in any
words: Claude works out the intent and fits it into the rules as a whole, revising the rule it repeats or
contradicts rather than adding another; if the rules already cover it, Claude says which one and changes
nothing. Claude also starts it by
itself when you ask to change a global preference, or when a snapshot reports newer repo files or a conflict.

- **Session review** (only when you run it, not when Claude starts it): first, Claude goes through the
  session, most recent first, for what it learned (your corrections and constraints, approaches you
  confirmed, mistakes and how to avoid them, setups that took work). It also reads the memories no device
  has reviewed yet (this machine's, and other devices' from their backups; up to 25 files per run), where
  lessons saved in one project often hold everywhere; a shared record (`hosts/.memory-review`) keeps a
  memory from being reviewed twice, and memories themselves are never changed. It sorts each item into the global
  `CLAUDE.md` (holds in any repo), the project's `CLAUDE.md` (anyone working in that repo needs it; nothing
  private, since it may be public) or memory (personal or this machine only), worded as the rule it would
  become, revising an existing rule where one covers it. Claude decides without asking you about each item:
  a rule is written only when it is true, general for its file, actionable, not already there, and changes
  what a capable model would do anyway. Each run also prunes both `CLAUDE.md` files: duplicates merged,
  rules that restate default model behaviour or name tools that no longer exist removed, wording shortened.
  The Security section, rules you set as constraints ("never", "always", "from now on") and what you asked
  for in the session are never removed or weakened; Claude asks only when a change would reverse one of
  them, or when two rules contradict and nothing shows which is right. The global changes are written and
  pushed with the sync, the project ones committed in the project (on your feature branch, or a short
  branch merged into the default one; never pushed). The report lists every change, one line each, with
  what undoes it: `/jeeves:update revert <commit>` for the global file, `git revert` in the project.

- **Sync:** snapshot, merge another device's diverged `main`, merge files changed both on disk and in the repo
  (three-way, from the version both last shared), install what is newer in the repo, then push.
- **Change:** a sync, then the change on a branch of the data repo (`home/…`), merged into `main`, installed
  into `~/.claude` and pushed. Editing the copy avoids the prompt Claude Code shows for a direct edit of
  `~/.claude`. Only shared files change this way; `settings.json` and `keybindings.json` you change in
  `~/.claude` as usual, and the next snapshot backs them up.

**Instructions come only from you.** Both reviews take a rule from your own messages, from results Claude
saw itself, and (as lessons) from memories; never from text Claude read, such as a web page, tool output, a
file or a project's `CLAUDE.md`. A line there that asks for a rule or speaks for you is not written, and the
report says where it was. An item that would add a command fetching or running outside code (a URL, a
download, an install, a pipe into a shell), change tool permissions or hooks, tell Claude to trust a source,
or weaken Security or a confirmation step is never written on Claude's own judgement, whatever its source:
it is shown to you with where it came from, unless you asked for exactly that yourself ("Leave it out"
settles it for good). Permissions and hooks live in each device's `settings.json`, which the reviews never
write: such an item is only reported. When Claude starts
`/jeeves:update` itself with a change, the change must be one you asked for in the session; otherwise it
only syncs.

**Skills, agents and commands from another device wait for your OK.** They carry instructions or code into
every session, so a new or changed one arriving through the data repo is held: `/jeeves:update` tells you
what it does or what changed and where it came from, and installs it only if you say so (left held, it is asked again next time;
or remove it on every device). A new device asks once for all of them. Incoming `CLAUDE.md` changes are
installed and summarised in the report. Lines that could make a session run outside code, gain permissions
or hooks, or obey text it reads (a download piped into a shell, an encoded blob, an `allowed-tools` or hook
entry, "ignore previous instructions") are flagged, both in what arrives and in what this device is about
to push; a flagged line in a push is asked about first, and can be taken out while the rest is pushed.

It pushes without asking and shows no diff unless you ask for one: every step is a commit. It stops and asks
whenever text could be lost: a passage changed differently on both sides, a rejected push (it never
force-pushes), or a possible secret found by `jeeves-scan` in what it is about to push.

### The `review` skill

`/jeeves:update` never changes the rules you set firmly yourself ("never", "always", your recorded
decisions). `/jeeves:review` is where they are looked at, with you. It reads this session, every memory on
every device and your global and project CLAUDE.md as one set, and looks for rules that hold back the results
you want: an absolute that blocks a better path, wording a model could read two ways, two rules pulling
against each other, a rule for something that no longer exists. It raises a rule only with a reason it can
show.

For each of your own rules it raises, it explains how the rule may be getting in the way and offers new
wording, removal, or keeping it. If its reading of what you meant is off, say what you meant in your own
words: Jeeves says back the intent it hears and offers wording that captures it, until you agree. Nothing of
yours changes without your agreement. Rules you did not set firmly it tidies on its own and reports. Each
agreed change is its own commit, undone with `/jeeves:update revert <commit>`. `/jeeves:review <concern>`
starts from a rule or a result you are unhappy with.

### Background updates

Off until you turn them on, per device: `/jeeves:autoupdate`. When on, at most once an hour per session (once
the session has grown enough to be worth it), the end of one of Claude's replies starts a separate headless
Claude Code session in the background. It reviews what the session taught, as `/jeeves:update` does, adds or
revises global rules, syncs and pushes; your session is not interrupted. The next session start says what
changed, with the command to undo it, and what waits for you. Each run uses your plan's usage: measured at
about $0.35 to $0.80 of list-price usage per run on Opus (2026-11-20).

What it may do alone is narrower than `/jeeves:update`: it adds rules and revises them in place, and never
removes or merges one, renames a heading, or changes the Security section or a rule you set firmly. Project
rules, removals, risky items, skills from other devices and anything that needs your decision are queued for
your next `/jeeves:update`, which takes them up.

How it keeps your data safe:
- The headless session sees only a private temporary folder (in your private runtime folder, or a per-user
  temp folder, never the shared `/tmp`, since Claude Code also reads any `CLAUDE.md` on the way up from it): an extract of the conversation (your messages
  and Claude's replies, without tool output, each marked with a random code so quoted text cannot pass for
  yours), a copy of the global rules, the project's rules and up to 25 memories. It may edit only that copy
  and write its notes; it has no shell, no web access and no MCP servers, and gets none of your session's
  variables, nor your settings' permission rules, hooks or other plugins (only your model and settings
  environment). The folder is deleted when the run ends.
- A script, not the model, changes the data repo. It checks the edited copy (the Security section, headings
  and firm rules unchanged; nothing removed; at most 12 lines changed; no line the risk check flags), merges it into `main`, scans for
  secrets and risky lines, and pushes exactly the commit it checked. An edit that fails a check is set aside
  for you, never pushed.
- One run at a time per device; a run that hangs is stopped after 15 minutes; a failure pauses runs for an
  hour, a usage limit for a day. No worker is left behind: the run stops its headless session whenever it
  ends, stopped included; a watchdog beside that session stops it if the run itself is killed outright; and
  each session start removes what a crashed run left (its temp folder, and its session if still running,
  once its command line confirms it is one), even with background updates off. `jeeves-auto status` shows a
  run at work with its process. Logs hold only times, outcomes and usage figures, never conversation text.

The headless session does not read your `settings.json` beyond its model and `env` block, so a sign-in set up
there (such as `apiKeyHelper`) does not reach it: the run then fails, and the next session start says so.

Settings, in `~/.claude/jeeves/auto.conf` on each device: `enabled`, `interval_minutes` (60),
`min_new_chars` (8000), `deadline_minutes` (15). Background updates need `perl` with JSON::PP (on macOS,
Linux and Git for Windows by default) to read transcripts.

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

`/jeeves:encryption` encrypts everything under `home/` and `hosts/` with [age](https://age-encryption.org)
(1.3.0 or later), so a leaked GitHub token or a repo made public by mistake shows only encrypted files. The
data repo's folder and `~/.claude` on each device stay plain, as they must for Claude Code to read them.
Still visible on GitHub: file names (including the project paths of memories), sizes and commit messages.

- **The key** is an age key made for your data repo. `identity.age` in the repo holds it, encrypted with your
  passphrase; `recipient` holds its public half, which is all that encrypting needs.
- **Your passphrase** is asked for in a window of the system (macOS dialog, Windows credential prompt, zenity
  or kdialog on Linux; elsewhere, Jeeves gives you a command for your own terminal), never in the chat,
  which is saved in transcripts. It is never stored. Losing it on every device means the data on GitHub can't
  be read; a device that is still unlocked can set a new one (`/jeeves:encryption passphrase`). A new
  passphrase does not change the key: the old `identity.age` stays in the history and still opens with the old
  passphrase, so after a leak `/jeeves:encryption passphrase` offers to fold the history as well (a copy someone
  took before that stays readable with the old passphrase). Generating it with a password manager is
  recommended.
- **If the fold after turning encryption on is not pushed** (offline, say), the session-start message and
  `/jeeves:encryption` say that GitHub still holds the plain history, and `/jeeves:encryption` removes it.
- **Each device unlocks once** (`/jeeves:encryption`, or during `/jeeves:setup`): the key then stays in its
  credential store (macOS Keychain, Windows Credential Manager, a Linux Secret Service through
  `secret-tool`; where there is none, a file only you can read, if you choose it). Jeeves reads it from there
  at most once per command, so it works without asking again. A locked device says so at session start and
  doesn't sync; `/jeeves:encryption lock` removes the key.
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
| `jeeves-snapshot` | `~/.claude` → data repo | Fast-forwards `main` from its remote, copies changed shared files into `home/`, mirrors this device's settings, keybindings and memories into `hosts/<host>/`, commits on `main`. Never pushes. `--force [<path>...]` keeps the disk copy of a conflict; `--allow-delete <folder>` records a folder gone from `~/.claude`; `--replace-host` takes over a host name another machine used. |
| `jeeves-apply` | data repo → `~/.claude` | Installs shared files that changed in the data repo since this device last synced, from `main` only (not mid-merge, not with uncommitted files under `home/`). Never overwrites a file a session edited since then. `--force [<path>...]` installs the repo copy (of the named files, whatever their state), saving the disk copy first. A skill, agent or command in a version this device never had is held until `--accept <hash>:<path>` names that version; `--show` prints every incoming change with its diff and risk flags and installs nothing. |
| `jeeves-restore [<host> [--memories]]` | data repo → `~/.claude` | Copies a device's backup onto this machine, saving replaced files under `.sync-state/overwritten/`. Without a host, prints this machine's host name and the hosts with a backup. |
| `jeeves-cleanup <days>` | data repo → origin | Folds the history older than `<days>` days into one root commit (same files) and force-pushes `main` with a lease; `--dry-run <days>` only says what it would do; `--all` folds all of it (after encrypting); `--auto` is what the session-start hook runs. |
| `jeeves-check` | (read-only) | What setup needs to know: OS, git, the GitHub CLI and sign-in, an SSH key GitHub accepts, how to install the CLI and age here, the key store, the data repo folder's state. |
| `jeeves-encrypt status` / `on` / `unlock` / `lock` / `passphrase` | data repo, key store | Encryption: its state on this device; turn it on; put the key in this device's credential store, or remove it; set a new passphrase. Asks for the passphrase in a dialog (`--terminal` in your own terminal). |
| `jeeves-github find` / `create <name>` | GitHub | Lists your data repos on GitHub (topic `jeeves-data`, or named `jeeves-data`), or creates a private one with that topic, through the GitHub CLI. |
| `jeeves-init <folder> [<url>]` | (new repo) | Creates an empty data repo (`.gitignore`, `.gitattributes`, README), optionally with a remote. |
| `jeeves-status` | (read-only) | Lists each path out of sync (`disk`, `repo` or `conflict`) with its disk, repo and last-synced hashes, and how `main` stands against `origin/main`. |
| `jeeves-memories list` / `record <hash> <outcome>...` | data repo | The memory files the session review has not reviewed yet (this machine's and other devices' backups), and the shared record of those it has, committed on `main`. |
| `jeeves-auto status` / `on` / `off` / `queue` | `~/.claude` → data repo → origin | Background updates: the setting, the last runs and what waits; `queue` lists the items and set-aside changes left for `/jeeves:update` (`queue clear <id>...` once taken). The hooks run `--hook-stop` and `--session-start`; `hold` and `release` are for `/jeeves:update`. |
| `jeeves-git <git arguments>` | data repo | Runs git in the data repo, so the skills never `cd` into it. |
| `jeeves-verify` | (read-only) | Checks the plugin's files against its signed `MANIFEST` and the release key recorded at setup; lists every changed, missing or added file. `--trust` records the release key on first setup; `--hook` prints only a difference (the session-start check). |
| `jeeves-risk` | (read-only) | Flags lines a push would add under `home/` that could make a session run outside code, gain permissions or hooks, or obey text it reads; lists file, line, commit and the kind of risk. |
| `jeeves-scan` | (read-only) | Looks for secrets (private keys, API keys and tokens, password assignments) in each commit a push would send, so one a later commit removed is still caught; lists file, line and commit, never the value. |

Each device keeps a gitignored record (`.sync-state/` in the data repo) of the file hashes at its last sync.
This lets the scripts tell "this device changed it" from "the repo changed it". If both changed, they report a
conflict and change nothing. Settle a conflict with `jeeves-snapshot --force <path>` (keep the disk copy) or
`jeeves-apply --force <path>` (keep the repo copy; the disk copy is saved under `.sync-state/overwritten/`), or
let `/jeeves:update` merge the two.

How the data stays whole:

- What a snapshot commits is what it copied, byte for byte: each file is copied, hashed, and checked
  against the commit (decrypted, when encrypted) before it is made; that hash is what the sync record keeps.
  A file still being written is left for the next snapshot. A copy that fails (a full disk) stops the run
  with nothing recorded, so no later step mistakes the old copy for the newer one. The same goes for
  `jeeves-apply`, which also checks a file again just before replacing it.
- A file holding git conflict markers (an unfinished merge) is never installed or recorded unless you name
  it after `--force`, and `jeeves-scan` flags markers in what a push would send.
- A shared folder that disappears from `~/.claude` (moved, deleted, or a link to a drive that is not
  there) is not recorded as deleting its files on every device: the snapshot says so, and `/jeeves:update`
  asks you whether to delete them everywhere or put them back.
- Each machine has an ID (`~/.claude/.jeeves-machine-id`) that it writes into its `hosts/<host>/` backup;
  a second machine reporting the same host name is stopped before it overwrites that backup (set
  `JEEVES_HOST` to give it a name of its own).
- One Jeeves run at a time per data repo (a lock in `.git/jeeves/`), and one background update or
  `/jeeves:update` at a time per device (`~/.claude/jeeves/auto/run.lock`).

`jeeves-snapshot` only commits onto a clean `main`. While the data repo is on another branch or has
uncommitted changes (other than its own copies from a run that failed or was stopped), it skips and says so.
When `~/.claude` holds none of the shared files a device synced before (the folder moved or was emptied), it
records nothing rather than delete them on every device. A deleted shared file is reported as such, by the
snapshot that records it and by `jeeves-apply` on the other devices. It does not take a shared file whose name another system can't
hold (characters Windows forbids such as `:` or `?`, a name ending in a dot or space, or two names that
differ only in case), and says which and why.

## The data repo

A git repo with:

- `home/`: the shared files.
- `hosts/<host>/`: each device's backup, laid out as in `~/.claude` (`settings.json`, `keybindings.json`,
  `projects/<project>/memory/`). Only that device writes its folder. Memories are restored only on request
  and only for projects that exist on the new machine, because their folder names are absolute paths.
- Never copied: `.credentials.json`, `~/.claude.json`, history, transcripts, caches. `settings.json` is backed
  up as it is, so a token in its `env` block goes into the data repo: keep secrets out of it (an environment
  variable set elsewhere, or a secret manager). `jeeves-scan` stops a push that would send one.
- `hosts/.memory-review`: the memories the session review has reviewed (a hash and what came of it, one per
  line), so no device offers the same memory twice.
- `jeeves.conf` (optional), shared by all devices: the automatic history cleanup's settings, and
  `backup_settings=yes` to back up each device's `settings.json` and `keybindings.json` (off by default;
  turning it off removes the copies from `hosts/<host>/` at each device's next snapshot, while the git
  history keeps them until a history cleanup).
- `.gitignore`: `.sync-state/` and `*.sync-tmp`; `.gitattributes`: `* -text`, so files are kept byte for
  byte, line endings included (and, when encrypted, the filter for `home/` and `hosts/`). A data repo made
  before 1.1 had `* text=auto eol=lf`; its next snapshot changes that in a commit of its own.
- When encrypted: `recipient` (the public key) and `identity.age` (the key, encrypted with your passphrase).

`<host>` is the machine's host name; on native Windows it gets `-win` (`mypc-win`), so a WSL and a Windows
Claude Code on one PC stay apart. `JEEVES_HOST` overrides it. A data repo made before per-device backups
(with `home/settings.json` and `memory/<host>/`) is moved to this layout by each device's next snapshot.

The scripts find it, in this order: `$JEEVES_DATA` (or `--data-repo <path>` as a script's first arguments);
the plugin option `data_repo`; the data repo this device last used, recorded in `~/.claude/jeeves/data-repo`
whenever a script is given one of the first two (the session-start hook sees the plugin option, so it keeps
the record current); the git repo around the current folder, if it has a `home/` folder; `~/jeeves-data`, the
option's default. Claude's Bash tool sees no plugin variables, so each skill passes the option once with
`--data-repo` and runs everything after it bare: a permission rule does not match past a `cd` outside the
project or a `JEEVES_DATA=` prefix, so commands written that way would each ask for approval.

## What Jeeves runs, sends and changes

Everything Jeeves does runs from the bash scripts in `bin/` (readable source, no binaries) and from the
skills' instructions to Claude. In full:

**Network.** Jeeves sends nothing to the plugin's author and collects no usage data. It sends nothing to
Anthropic itself, except through Claude Code when you turn background updates on (below).
- Your data repo's own remote (the GitHub repo you chose at setup), through your git and its credentials:
  - `git fetch` at each session start (the hook), and whenever a command syncs, unlocks or cleans up; a
    `git clone` in `/jeeves:setup` on a new device.
  - `git push` from `/jeeves:update` (also run by `/jeeves:setup`'s first sync), `/jeeves:review`,
    `/jeeves:cleanup` and `/jeeves:encryption` (after a passphrase change or turning encryption on).
  - From a background update, when on: `git push` of the data repo's `main` (after the secrets and risk
    checks), and the fetch and merge a refused push needs.
  - A force-push, always with a lease, from the history cleanup: by hand (`/jeeves:cleanup`, or turning
    encryption on, after you confirm), or from the session-start hook once you turn the automatic cleanup
    on in `jeeves.conf`, without asking again each time.
  - What is pushed: copies of the shared files and this device's backups listed at the top, checked for
    secrets first (`jeeves-scan`). With encryption on, file contents leave the device only encrypted; file
    names (including project paths), sizes and commit messages stay readable.
- `raw.githubusercontent.com`, once a day from the session-start hook: an anonymous `curl` of this repo's
  `.claude-plugin/marketplace.json`, to say when a newer release is out. Nothing is sent but the request
  itself. Turn it off with `JEEVES_UPDATE_CHECK=off`.
- Anthropic, only with background updates on (`/jeeves:autoupdate`; off by default): the Stop hook starts
  `claude -p` (headless Claude Code, under your account and plan), which sends what it reads to Anthropic's
  API as any Claude Code session does: an extract of the session's conversation (no tool output), your
  global rules, the project's CLAUDE.md and up to 25 memory files.
- GitHub during `/jeeves:setup` only: your GitHub CLI (`gh`) checks you are signed in, finds or creates your
  private data repo and tags it with a topic; `ssh -T git@github.com` checks whether your SSH key is
  accepted (it never adds GitHub's host key for you).

**Software it may install.** None by itself. When `gh` or `age` is missing, `/jeeves:setup` and
`/jeeves:encryption` show your system's install command (such as `brew install gh` or `sudo apt install gh`)
and run it only after you agree. Setup may also offer `gh auth setup-git`, which changes your global git
config, again only on a yes.

**Files it changes.**
- Under `~/.claude` (or `CLAUDE_CONFIG_DIR`):
  - `CLAUDE.md`, `agents/`, `commands/` and `skills/`, when installing shared copies from the data repo; a
    file deleted from the data repo for every device is deleted here too. New or changed skills, agents and
    commands from another device are held until you accept them.
  - Project memories: `/jeeves:update` may save new ones, as Claude Code saves memories.
  - `jeeves/`: Jeeves's own state on this device (the data repo's path, the trusted release key, the last
    verified manifest, the daily check's time, development checkouts), and with background updates the
    setting (`auto.conf`), the log, notices, the queue of items for you and set-aside changes (`auto/`).
  - `settings.json`, `keybindings.json` and memories: only by `/jeeves:restore`, which you start, from the
    backup of a device you pick (settings exist there only with `backup_settings=yes`). That device's
    `settings.json` comes back whole, its permissions and hooks included; every file replaced is kept under
    the data repo's `.sync-state/overwritten/`, and Claude Code asks before the follow-up edit of
    machine-specific values.
  - **Permission settings:** besides a restore, only when you say yes at the end of `/jeeves:setup`, which
    adds two `permissions.deny` rules to `settings.json` so no session's Edit or Write tool can change the
    installed plugin or its trusted key. Nothing else in Jeeves adds, removes or changes permissions or
    hooks in your settings.
- Project `CLAUDE.md` files: from `/jeeves:update` and `/jeeves:review`, committed in that project's git
  (never pushed unless its own CLAUDE.md allows it); each change is reported with its revert.
- The data repo folder: commits on its `main`, and Jeeves's sync record in `.sync-state/`.
- With encryption on: the key in your system's credential store (Keychain, Windows Credential Manager,
  `secret-tool`) or, if you choose a key file, in `~/.config/jeeves/keys/`.

It never copies secrets: not `.credentials.json`, `~/.claude.json`, transcripts or history into the data repo.
A background update reads the session's transcript to make its extract, which it keeps in a private temporary
folder (deleted when the run ends) and never commits.

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
`/plugin` > Marketplaces. The marketplace installs only signed releases: its entry is pinned to the latest
release's tag and commit, so a new version arrives with each release; see [CHANGELOG.md](CHANGELOG.md).
Once a day, the session-start hook reads which release the marketplace on GitHub installs (anonymously,
`.claude-plugin/marketplace.json` on `main`, with curl, at most 10 seconds) and says so in one line when it
is newer than yours, with the commands above; it says nothing otherwise, on a development checkout, or with
`JEEVES_UPDATE_CHECK=off` in Claude Code's environment. To stay on a release, leave auto-update off and don't
run the update commands (set `JEEVES_UPDATE_CHECK=off` to silence the notice too).

**Working on Jeeves:** on a machine where Jeeves is already installed and set up, there is no need to
uninstall it. Load a clone in its place instead, and the installed release takes over again as soon as you
stop loading the clone.

1. Clone the repo:

   ```sh
   git clone https://github.com/WinuxNomacs/jeeves.git ~/gitrepos/config/jeeves
   ```

2. List the clone's folder in `~/.claude/jeeves/dev-checkouts`, so the release check leaves it alone (see
   below). Without this, every session start warns, since a clone matches the signed release only right
   after one:

   ```sh
   mkdir -p ~/.claude/jeeves && echo "$HOME/gitrepos/config/jeeves" >> ~/.claude/jeeves/dev-checkouts
   ```

3. Load the clone. For one terminal session, with no settings changed:

   ```sh
   claude --plugin-dir ~/gitrepos/config/jeeves
   ```

   For every session, the VS Code panel included, add this to that machine's `~/.claude/settings.json`
   (per device, never synced by Jeeves) and start a new session; `/reload-plugins` alone has left the
   panel's command menu without the Jeeves commands:

   ```json
   "env": { "CLAUDE_CODE_PLUGIN_DIRS": "/absolute/path/to/jeeves" }
   ```

4. Check that `claude plugin list` shows `jeeves@inline` with the clone's path.

A clone loaded this way gets no plugin options; Jeeves then uses the data repo this device recorded at
setup. Hook scripts run from the clone as soon as a file is saved, in every session on that machine: keep
the loaded clone on a working `main`, and try changes in a `git worktree`. Edits to skills take effect on
`/reload-plugins`. The tests (`bash tests/run.sh`) need age 1.3 or later for the encryption tests (on
macOS, `brew install age`). To install or try the published plugin on that machine while the setting is on,
run `claude` without it (`env -u CLAUDE_CODE_PLUGIN_DIRS claude ...`): an install made with it set has
failed to load. To go back to the installed release, remove the `env` line (or stop passing
`--plugin-dir`), start a new session, and update it with the commands above.

## Verifying the plugin

Each release carries `MANIFEST` (the SHA-256 of every file it ships) signed with a release-only SSH key.
`/jeeves:setup` records that key outside the plugin (`~/.claude/jeeves/allowed_signers`) the first time, so
a later update cannot swap it; compare the fingerprint it shows with this one:

```
SHA256:tXt+0kjefgqd/mSjv7xDjFj6/u9LxpK+vv9gZhxgZy8 jeeves-release (ED25519)
```

At every session start, `jeeves-verify` checks the plugin's files against the signed `MANIFEST`. If any file
changed, is missing or was added (a link counts), it says so and Jeeves's hooks do nothing more;
`/jeeves:update` and `/jeeves:review` stop too, until the plugin is reinstalled. The check needs OpenSSH 8.1
or later (`ssh-keygen -Y`); without it, it says so and Jeeves carries on unchecked. This catches changes on disk; it cannot stop
someone who can already write the plugin folder, who could change the check as well. Setup also offers deny
rules that stop Claude's Edit and Write tools from changing the installed plugin or the recorded key.

A copy you develop in (installed from a local folder) is not a signed release: list its folder, one per
line, in `~/.claude/jeeves/dev-checkouts` (such as `~/src/jeeves`), and the check leaves it alone.

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
