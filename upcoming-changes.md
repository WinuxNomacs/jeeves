# Upcoming changes

## Phases (current and upcoming)

- **Up next:** 1. `update` skill: sync and change modes, three-way review, push after confirmation
- **Then:** 2. Generic: setup skill, Windows, machine-specific paths in `settings.json`
- **Later:** 3. Publish through own GitHub marketplace · 4. macOS device · 5. Per-device overlay (when needed)

## Done

- 0. Initial import: `~/.claude/CLAUDE.md` and `settings.json` recorded in `home/` on `main`
  (2026-11-16, commit `82dd02f`).
- Sync scripts, tests and docs: `bin/snapshot`, `bin/apply`, `bin/lib.sh`; tests in `tests/run.sh`
  (15 tests, scratch `~/.claude` + clone + bare remote); README, repo CLAUDE.md (2026-11-16).
- 1. Split into plugin repo `jeeves` (this repo, fresh history) + data repo `jeeves-cmd` (2026-11-16).
  Scripts `bin/jeeves-snapshot`, `bin/jeeves-apply`, `bin/jeeves-hook-edit`, shared `lib/lib.sh`; hooks in
  `hooks/hooks.json`; manifests in `.claude-plugin/`; 22 tests in `tests/run.sh`. Installed on RAIVEN from
  the local clone. Checked live: a new session's SessionStart hook committed `snapshot(RAIVEN): settings.json`
  in `jeeves-cmd`; a session's Edit of a (scratch) global CLAUDE.md was committed by the PostToolUse hook and
  shown as a notice; the real one checked live too (2026-11-16, see "Live check of the edit hook" under 1. Split).
  Details below under "1. Split".

---

## Decisions (2026-11-16, with the user)

- **Copy model, not symlinks.** The repo mirrors `~/.claude/` under `home/`. Claude Code rewrites
  `settings.json` atomically, which can silently replace a symlink with a plain file.
- **Snapshots commit straight to `main`** as `snapshot(<host>): …`, a deliberate exception to "every change on
  a branch": they record what already happened on disk. Deliberate edits still go branch → merge → `bin/apply`.
- **Tracked:** `CLAUDE.md`, `settings.json`, `keybindings.json`, `agents/`, `commands/`, `skills/` (not
  `skills/synced/`, which claude.ai manages). **Never:** `.credentials.json`, `~/.claude.json`, history,
  sessions, transcripts, caches, backups, plugins.
- **Memories** (`~/.claude/projects/*/memory/`) are backed up per device to `memory/<host>/<project-key>/`
  and never applied: the folder names are absolute paths (they differ per OS) and memories hold
  machine-specific facts. Benefit: an audit trail of what Claude believed or deleted, and a backup.
- **Several devices** (Linux now, macOS and Windows later): `snapshot` fetches and fast-forwards `main` first;
  a per-device record of the last synced hashes (`.sync-state/`, gitignored) tells "this device changed it"
  from "another device changed it"; both → reported as a conflict, never merged automatically.
- **Diverged `main`:** snapshot still commits locally (recording disk is the priority) and reports the
  divergence; the user merges before pushing. (The first plan said "stop"; changed while building, since
  stopping would leave the disk state unrecorded.)
- **Per-device differences:** one shared copy until a real difference appears; then decide whether the rule is
  host-specific or platform-general.
- **Scripts in bash** (portable to macOS bash 3.2 and Git Bash): bash is present wherever git is. No
  dependencies beyond git; tests use a small runner in the repo rather than a test framework.

## Name and repos (2026-11-16)

- Project, plugin and commands: **jeeves** (display name **Jeeves**), invoked as `/jeeves:<skill>`. Checked
  against the 3,731 plugins in the marketplace directory (none named jeeves) and GitHub (a few small,
  unrelated repos).
- GitHub: the original repo `WinuxNomacs/claude` was renamed `WinuxNomacs/jeeves` (the 2016 private repo that
  held the name became `WinuxNomacs/jeeves-discord`), then `WinuxNomacs/jeeves-cmd` (the data repo) at the
  split; `WinuxNomacs/jeeves` is now this plugin repo, with fresh history. This file moved here from
  `jeeves-cmd` at the split; its earlier history is in that repo.

## Existing plugins checked (2026-11-16)

None covers snapshot-before-change plus a Claude-reviewed merge of diverging CLAUDE.md versions.
- `claude-md-management` (Anthropic, reviewed): audits CLAUDE.md quality and captures session learnings.
  No versioning or sync. Complementary; worth installing separately.
- `cync` (github.com/dev-gyus/claude-code-sync): push/pull `~/.claude` to a git repo. 0 stars, one main
  author, last commit 2026-04-06, needs Node 18, commits compiled `dist/`; conflicts are only
  "overwrite or keep" per file (3-way merge listed as future work). **Borrow:** secret scan before every
  push, dry-run mode. Don't depend on it.
- `claude-backup` (tombelieber): session backup (sessions are deleted after 30 days), macOS npm CLI, one
  maintainer, skill moved to another repo. Different goal.
- `sync-my-skills`: skills only, 8 commits over two days in 2026-04.

## Plugin direction (2026-11-16, from the Claude Code docs on skills, plugins and plugin components)

- A **skill** is instructions (`SKILL.md`) plus scripts; Claude follows it, nothing enforces it. So:
  deterministic work (hashes, conflicts, commits) stays in scripts; judgment (reviewing diffs, merging rule
  text, asking the user) is the skill; whatever must always happen is a **hook**.
- A **plugin** packages skills, hooks, `bin/` (on Claude's PATH), install-time `userConfig`, and
  `${CLAUDE_PLUGIN_DATA}` (survives updates: the per-device sync record goes there). Distributed through a
  marketplace (a GitHub repo with `.claude-plugin/marketplace.json`).
- Windows: hooks run through Git Bash, or PowerShell when Git Bash is missing; Jeeves requires Git Bash.
- Side-effecting skills use `disable-model-invocation: true` so only the user starts them.

## Phase 1 decisions (2026-11-16, with the user)

- **Split now**, not in phase 3. This repo's history holds preferences and memory backups, so it cannot become
  the plugin repo without rewriting pushed history. So: this repo is renamed on GitHub to
  **`WinuxNomacs/jeeves-cmd`** ("Jeeves Command Center": the data); a **new private `WinuxNomacs/jeeves`**
  with fresh history holds the plugin code (made public in phase 4).
- **Hooks:** SessionStart snapshot, plus a PostToolUse snapshot after edits to `~/.claude/CLAUDE.md`.

## 1. Split: plugin repo `jeeves`, data repo `jeeves-cmd` (done)

Facts checked in the Claude Code plugin docs (2026-11-16: manifest-reference, components, hooks, loading,
marketplace-reference, install):
- `userConfig` in `plugin.json` (types include `directory`) is asked for when the plugin is installed or
  enabled in `/plugin`; hooks get each value as `CLAUDE_PLUGIN_OPTION_<KEY>`; skills get `${user_config.KEY}`.
- Bash-tool commands see none of the plugin variables, though the plugin's `bin/` is on their PATH.
- A private GitHub repo works as a marketplace (cloned with the machine's own git credentials). A
  marketplace added from a local path loads the plugin in place: edits apply on `/reload-plugins`.
- SessionStart plain stdout goes into Claude's context; PostToolUse needs `systemMessage` to show the user
  anything. Hook stdin carries `tool_input.file_path` for Write/Edit.

Design:
- **Plugin repo** (new, private, fresh history): `.claude-plugin/plugin.json` (name `jeeves`, display name
  Jeeves, `userConfig.data_repo` of type `directory`), `.claude-plugin/marketplace.json` (the repo is its own
  marketplace, plugin source `"."`), `hooks/hooks.json`, `bin/` (snapshot, apply, lib.sh), `tests/`, README,
  CLAUDE.md, and this planning file (it describes the plugin, not the data).
- **Finding the data repo:** `lib.sh` takes `JEEVES_DATA`, else `CLAUDE_PLUGIN_OPTION_DATA_REPO`, else stops
  with a message saying how to set it. `.sync-state/` stays in the data repo for now (moves to
  `${CLAUDE_PLUGIN_DATA}` in phase 3).
- **Hooks:** SessionStart → `snapshot --hook`. PostToolUse on `Write|Edit` → a small script that reads the path
  from stdin and runs `snapshot --hook --no-pull` only when it is `~/.claude/CLAUDE.md`, reporting through
  `systemMessage`.
- **Data repo** (this one, renamed `jeeves-cmd`): keeps `home/`, `memory/`, `.sync-state/`; loses `bin/` and
  `tests/` once the plugin works; its README and CLAUDE.md point at the plugin.
- **Steps:** set this repo's remote to `jeeves-cmd`, rename on GitHub; create private `WinuxNomacs/jeeves`;
  build the plugin there (tests first, with `JEEVES_DATA`); install it from the local folder; remove the
  scripts here. After the session the user renames this folder to `~/gitrepos/config/jeeves-cmd`; the plugin
  lives in `~/gitrepos/config/jeeves`.
- **Built (2026-11-16, with the user):** scripts renamed `jeeves-snapshot` / `jeeves-apply` because the
  plugin's `bin/` is on every Bash tool PATH; `lib.sh` moved to `lib/` to stay off it. The data repo is also
  found from the current folder (the git repo around it, if it has `home/`), since the Bash tool sees no
  plugin variables. `CLAUDE_PREFS_HOST` became `JEEVES_HOST`. The edit hook parses its JSON with awk (no jq,
  no GNU sed). Installed with `claude plugin marketplace add <clone>` and
  `claude plugin install jeeves@jeeves --config data_repo=<path>`.
- **Found while building:** Claude Code treats `~/.claude/CLAUDE.md` as a sensitive file: a headless session
  cannot edit it (an interactive one asks first), so the edit hook was checked live against a scratch
  `CLAUDE_HOME` and data repo. Installing writes absolute, machine-specific paths into `settings.json`
  (`extraKnownMarketplaces.jeeves.source.path`, `pluginConfigs."jeeves@jeeves".options.data_repo`), which
  the data repo shares with every device: to settle in phase 2.
- **Live check of the edit hook (2026-11-16, interactive session):** an Edit adding a marker line to the
  real `~/.claude/CLAUDE.md` was committed at once (`b4b60b6` in jeeves-cmd); the revert too (`bfa6379`),
  leaving `home/` identical to before. Pushed.
- **Gap found:** a session started before the plugin was installed loads no hooks. One such session (in
  another project) edited `~/.claude/CLAUDE.md` with nothing recorded; the next manual `jeeves-snapshot`
  committed it (`1d81587`). Any snapshot (next session start, or `/jeeves:update`) covers this; no fix needed.
- **Verify:** tests pass in the plugin repo; a new session's SessionStart snapshot runs (seen in `git log` of
  the data repo after a deliberate change); an edit to `~/.claude/CLAUDE.md` is committed at once.

## 1. `update` skill (was 2)

Decisions (2026-11-16, with the user):
- **Push without asking.** Show the diff only when the user asks for it: everything is in git, so nothing is
  lost and a routine diff is noise. **Always ask when data could be lost:** a merge that would drop or rewrite
  text one side added (a true conflict), anything that would need a force-push (never done), a rejected
  non-fast-forward push. A failed secret scan also stops and asks.
- **Claude may start the skill** (no `disable-model-invocation`): requests to change global preferences go
  through the data repo instead of a direct edit, and a snapshot reporting newer repo files or a conflict
  leads to it.

Design:
- `skills/update/SKILL.md`, invoked `/jeeves:update [request]`: no request means sync mode, a request means
  change mode.
- `bin/jeeves-status` (new, read-only): one line per tracked path not in sync,
  `state<TAB>path<TAB>disk<TAB>repo<TAB>base` (from `classify`/`hash_of`/`base_of` in `lib/lib.sh`; the base
  is a git blob, read with `git cat-file -p`), plus `main` vs `origin/main` (ahead, behind, diverged).
- `bin/jeeves-scan` (new, our own, no extra binary on every device): greps the outgoing diff
  (`origin/main..main`) for private-key headers, `sk-ant-`, `ghp_`/`github_pat_`, `AKIA`, `xox[bp]-`, and
  key/token/secret/password assignments with long values; exit 1 with the matches.
- `jeeves-snapshot`'s "newer" and "conflict" messages also suggest `/jeeves:update`.
- **Sync mode:** `jeeves-snapshot` → if `main` diverged from `origin/main`, `git merge --no-ff origin/main`,
  Claude merging conflicting `home/` text → per disk-vs-repo conflict, Claude merges base/disk/repo into
  `home/`, commits, `jeeves-apply --force` (disk copy saved) → `jeeves-apply` → `jeeves-snapshot` (silent) →
  `jeeves-scan` → push.
- **Change mode:** branch in the data repo → edit `home/…` → commit → merge `--no-ff` → `jeeves-apply` →
  `jeeves-snapshot` (silent) → `jeeves-scan` → push. Avoids the sensitive-file prompt of a direct edit.
- After building: use change mode to add to the global "never push" rule that `/jeeves:update` pushes the
  Jeeves data repo. Docs: README, plugin `CLAUDE.md`, jeeves-cmd `CLAUDE.md` ("Changing a preference").
- **Verify:** tests (status lists disk/repo/conflict paths with their hashes; status reports a diverged
  main; scan flags a planted token and passes a clean diff); `claude plugin validate`; live in a scratch data
  repo and `CLAUDE_HOME` (change mode, then a forced conflict that must stop and ask); one real sync.

## 2. Generic (was 3)

Code and data are split already (phase 1). Setup skill that creates or clones the data repo; sync record
moves to `${CLAUDE_PLUGIN_DATA}`; Windows (Git Bash, paths, `hostname`, `.gitattributes` with `eol=lf`; the
edit hook compares POSIX paths, so a `C:\...` path from the hook needs converting there).
Machine-specific paths in the shared `settings.json` (see "Found while building" under 1. Split): options
include leaving them out of the shared copy, or a per-device overlay (phase 5) for these keys.

## 3. Publish (was 4)

Own GitHub marketplace; versioned releases; Anthropic directory optional.

## 4. macOS device (was 5)

Install the plugin, clone the data repo, run `tests/run.sh` (user runs it by hand), first sync, resolve any first-sync conflicts.

## 5. Per-device overlay (was 6)

When the first real difference appears: decide host vs platform scope, then add an overlay folder
(`home.<host>/` or `home.<os>/`) merged by `apply` (needs a JSON merge for `settings.json`).
