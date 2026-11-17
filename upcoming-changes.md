# Upcoming changes

## Phases (current and upcoming)

- **Up next:** 1. macOS device (a real Mac, by hand; the scripts already pass on macOS in CI)
- **Later:** 2. Per-device overlay (only if a shared file must differ per host) · Anthropic directory
  submission (only on the user's direct command)

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
- 2. `update` skill (2026-11-16): `skills/update/SKILL.md` (`/jeeves:update`, sync and change modes),
  `bin/jeeves-status`, `bin/jeeves-scan`; snapshot messages point to it; 28 tests in `tests/run.sh`. Checked
  live on RAIVEN: a real sync (nothing to merge, pushed jeeves-cmd `bfa6379..445a809`) and a real change (the
  global "never push" rule's exception for `/jeeves:update`, pushed `445a809..09be4a3`). Details below under
  "2. `update` skill".
- 3. Generic (2026-11-16): shared files (`CLAUDE.md`, `agents/`, `commands/`, `skills/`) synced through
  `home/`; `settings.json`, `keybindings.json` and memories backed up per device to `hosts/<host>/`;
  `jeeves-restore`, `jeeves-init`, skills `setup` and `restore`; Windows (Git Bash, `-win` host, `C:\` paths
  in the edit hook, `.gitattributes`, no autocrlf). 35 tests in `tests/run.sh`, passing on Linux and under
  Windows Git Bash. Checked live: jeeves-cmd moved to the new layout on RAIVEN (`2e54988`); RAIVEN-win set up
  through `/jeeves:setup` (first sync, backup `3466803`); edit hook on Windows (`d44f91b`, `a5f3c08`).
  Details below under "3. Generic".
- 4. Publish (2026-11-16): the repo is public; MIT `LICENSE`; manifests pass `claude plugin validate
  --strict` (`CLAUDE.md` moved to `.claude/`); `.github/workflows/test.yml` (tags and by hand; Linux,
  Windows, macOS bash 3.2, all green); README installs from GitHub; release 0.2.0 merged (`9de7f1e`,
  `CHANGELOG.md`), then 0.2.1 (marketplace source `./`: older Claude Code versions refused `"."`, found
  adding the marketplace on the Mac), to be tagged by the user. Details below under "4. Publish".

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
  with fresh history holds the plugin code (made public in the Publish phase).
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
  `${CLAUDE_PLUGIN_DATA}` in phase 3; in Generic it was kept in the clone instead, see "3. Generic").
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

## 2. `update` skill (done)

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

Built and checked (2026-11-16):
- The skill reads `${user_config.data_repo}` (substituted into skill text; the Bash tool sees no plugin
  variables) and works from inside the data repo. `jeeves-status` prints a `datarepo` line for that. The
  shell's folder is not kept between commands when the data repo is outside the session's folders, so every
  command starts with `cd <data repo> &&`.
- Scratch check (by hand, following SKILL.md, two devices): a conflict with additions on both sides merged
  three-way; then apply, a silent snapshot, scan, push; a planted GitHub token blocked the push and was not
  printed. **Not checked live:** the skill stopping to ask on overlapping changes (only its text says so);
  a headless `claude -p` run was not used because it would read the real `data_repo` option.
- Live on RAIVEN, both through `/jeeves:update` after `/reload-plugins`: sync mode, then change mode (above).
- Found on the way: `jeeves-apply` reported "previous disk copies saved" on a plain run in the same second as
  a `--force` run; fixed on its own branch with a test (`088b676`).

## 3. Generic (done)

Facts checked (2026-11-16):
- Plugin docs (code.claude.com: manifest-reference, settings-reference, skills, hooks, setup, loading):
  `pluginConfigs` (where `data_repo` is stored) is read only from the user's `~/.claude/settings.json` or
  managed settings, never project or local ones. `${CLAUDE_PLUGIN_DATA}` (`~/.claude/plugins/data/<id>/`)
  reaches hooks and skill text but not Bash-tool commands, and is deleted when the plugin is uninstalled.
  On native Windows, hooks and the Bash tool run in Git Bash; a hook's `tool_input.file_path` arrives as
  `C:\...` with backslashes; `~/.claude` is `C:\Users\<u>\.claude`.
- This PC (RAIVEN) has a native Windows Claude Code too: `C:\Users\winux\.claude`, with a global
  `CLAUDE.md` from 2026-11-04 that is a subset of the shared one (it lacks the 4 rules added since), a
  2-key `settings.json`, only `skills/synced/`. Git for Windows 2.55.0 (bash 5.3) is installed; its system
  gitconfig sets `core.autocrlf=true`, and its `hostname` also says `RAIVEN`.
- `tests/run.sh` under Windows Git Bash (run from WSL on a scratch copy in `C:`): 25 of 28 pass. Failing: the
  three JSON checks (no Python on Windows) and the edit hook's notice, which picks up git's "LF will be
  replaced by CRLF" warnings.

Decisions (2026-11-16, with the user):
- **Scope: shared vs per host.** Shared and synced (three-way merge, conflicts, change mode, as now):
  `CLAUDE.md`, `agents/`, `commands/`, `skills/` (not `skills/synced/`). Per host, backup only, never merged
  or applied automatically: `settings.json`, `keybindings.json` (modifier keys differ on macOS) and the
  memories. The goal: back up each machine's profile, and restore a host's backup onto a new machine. This
  removes the machine-specific paths problem (each host keeps its own `settings.json`) and the JSON merge.
- **Per-host backup at every session start**, committing only what changed (as the memory backup does now).
- **Native Windows host name:** `<hostname>-win`, automatically (Git Bash: `uname` MINGW/MSYS/CYGWIN), so
  WSL and Windows on one PC stay apart; Linux, WSL and macOS keep the plain name; `JEEVES_HOST` overrides.
- **Live Windows check on this PC's Windows side:** the user installs the plugin in the Windows Claude Code;
  first sync replaces its older `CLAUDE.md` with the shared one (the old copy saved).
- **Sync record stays in `.sync-state/`** in the clone (per device already), instead of moving to
  `${CLAUDE_PLUGIN_DATA}`: Bash-tool commands don't see that variable and uninstalling deletes it.

Design:
- **Data repo layout:** `home/` holds only the shared files. `hosts/<host>/` mirrors `~/.claude` for the
  per-host files, so a restore is a plain copy: `hosts/<host>/settings.json`, `keybindings.json`,
  `projects/<key>/memory/*.md` (replaces `memory/<host>/<key>/`). Migration in jeeves-cmd: `git mv
  home/settings.json hosts/RAIVEN/settings.json`, `git mv memory/RAIVEN hosts/RAIVEN/projects/...`.
- **`lib/lib.sh`:** `SHARED_FILES`/`SHARED_DIRS` (synced) and `HOST_FILES` + memories (backed up); host
  suffix on Windows. `jeeves-snapshot` syncs the shared set as now and copies the host set one way into
  `hosts/<host>/` (deleting there what was deleted on disk). `jeeves-apply` and `jeeves-status` cover only
  the shared set.
- **Names other hosts can't hold:** snapshot refuses a shared file whose name has `: ? * < > | " \`, ends in a
  dot or space, or differs only in case from another, and says why (it would break on Windows or macOS).
- **`/jeeves:restore <host>`** (new skill, `disable-model-invocation: true`): copies `hosts/<host>/`'s
  settings and keybindings into `~/.claude`, saving existing files under `.sync-state/overwritten/`; Claude
  then fixes paths that can't work here (plugin marketplace folder, `data_repo`) and lists anything else that
  looks machine-specific. Memories are offered only for projects whose folder exists on this machine.
- **`/jeeves:setup`** (new skill, `disable-model-invocation: true`): clones the data repo (or creates one:
  `git init`, `home/`, `.gitignore`, `.gitattributes`; a remote URL is optional, Jeeves creates no GitHub
  repo), then the first `CLAUDE.md`/skills sync through `/jeeves:update` and the first backup of this host;
  on a host with no backup yet it offers `/jeeves:restore`.
- **Windows:** `.gitattributes` (`* text=auto eol=lf`) in both repos; the edit hook converts the path with
  `cygpath -u` when present; the tests' JSON checks fall back when Python is missing; skills run their
  scripts as `bash script.sh` (Windows keeps no executable bit), stated in the update skill.
- **Docs:** README (shared vs per host, setup, restore, Windows), plugin `CLAUDE.md` layout, jeeves-cmd
  README and `CLAUDE.md`.

Verify:
- Tests named as sentences: settings and memories go to `hosts/<host>/` and are never applied; a deletion
  on disk is removed from the host backup; a Windows-invalid or case-colliding name is refused; the host
  name gets `-win` under a fake MINGW `uname`; the edit hook matches a `C:\...` path; restore saves the files
  it replaces. Full suite on Linux and under Windows Git Bash.
- Live: the jeeves-cmd migration on RAIVEN (WSL) through a real snapshot; then the Windows side: the user
  installs the plugin there, clones jeeves-cmd to `C:\Users\winux\gitrepos\config\jeeves-cmd`, runs
  `/jeeves:setup` (first sync, backup to `hosts/RAIVEN-win/`); the edit hook on Windows.

Built (2026-11-16, branch `feat/generic`):
- Scripts: `lib/lib.sh` (shared vs `HOST_FILES` lists, `-win` suffix, `bad_name`, `case_collisions`, hashes
  with `--no-filters`, data repo git with `core.autocrlf=false`), `jeeves-snapshot` (backup to
  `hosts/<host>/`, old layout removed, names refused), `jeeves-hook-edit` (`C:\` paths), new
  `jeeves-restore` and `jeeves-init`; skills `setup` and `restore`; update skill limited to shared files.
  `.gitattributes` in both repos. 35 tests, passing on Linux and under Windows Git Bash; every new test fails
  against `main`'s scripts.
- Found while building: Git for Windows' `core.autocrlf=true` made `git add` print CRLF warnings into the edit
  hook's message even with the new layout (the test repo has no `.gitattributes`, and a user's data repo may
  not either), hence `core.autocrlf=false` on the data repo's git calls. A Windows clone of the plugin keeps LF
  scripts thanks to `.gitattributes` (checked: `C:\Users\winux\gitrepos\config\jeeves`, cloned with
  Windows git over SSH).
- Live on RAIVEN (WSL): the real snapshot moved jeeves-cmd to the new layout in one commit (`2e54988`,
  settings and memories renamed unchanged into `hosts/RAIVEN/`); a second snapshot was silent; scan clean;
  pushed `09be4a3..2e54988`.
- Live on RAIVEN-win (Windows VS Code extension 2.1.325): the extension has no `/plugin`, so the plugin was
  installed with the `claude.exe` it ships (`resources\native-binary\`, now in the README). `/jeeves:setup`
  found the clone, merged the older Windows `CLAUDE.md` (a subset) into the shared version, saved the old
  copy, pushed the backup `hosts/RAIVEN-win/` (`3466803`). An Edit adding and removing a line in the Windows
  `~/.claude/CLAUDE.md` was committed by the edit hook each time (`d44f91b`, `a5f3c08`; net change none; not
  pushed yet, the next sync there pushes them). That session edited the global CLAUDE.md without asking
  (its permission mode), unlike Linux.
- Found live and fixed: `~/.claude/skills/.trash/` (Claude Code keeps removed skills there; 473 files on
  Windows) would have been shared, so `.trash/` under the shared folders is skipped (`52b2617`). Setup did not
  offer a restore because the session-start snapshot had already backed the new machine up; it now offers one
  whenever other hosts have backups (`6676ff6`).
- Not checked live: `/jeeves:restore` (tests only); the overlapping-conflict question (still only in the
  skill's text).

## 4. Publish (done)

Facts checked (2026-11-16, code.claude.com plugin docs: install, host-marketplace, loading,
marketplace-reference, manifest-reference, publish, anthropic-marketplaces; claude.com/docs/plugins/submit):
- Users add the marketplace with `claude plugin marketplace add WinuxNomacs/jeeves` (default branch), or pin
  `WinuxNomacs/jeeves#<tag>`. **The manifest `version` decides updates:** "If you set `"version": "1.0.0"`
  and push new commits without changing it, users don't receive them." So every release bumps
  `plugin.json`'s `version` (and the version is set only there, not also in `marketplace.json`).
- Auto-update is off by default for third-party marketplaces; users update with
  `claude plugin update jeeves@jeeves` or enable auto-update in `/plugin` > Marketplaces.
- Tags are only needed when other plugins declare version ranges on this one (`claude plugin tag` makes
  `jeeves--v<version>`); a tag is still useful as a release marker and the CI trigger.
- `bin/`: claude.ai and Cowork don't install plugins with a top-level `bin/`. Jeeves is for Claude Code (local
  files, hooks), so this is accepted; noted in the README.
- Directory submission (later): `LICENSE` or `license`, README of 40+ words, no bundled binaries, a security
  scan per commit, a reviewer publishes each version; the repo must be public before the listing goes live.
- `actions/checkout` (GitHub, MIT, not archived): latest release v7.0.1 on 2026-07-20, last push 2026-10-17;
  pinned by commit `3d3c42e5aac5ba805825da76410c181273ba90b1`. The alternative, a `git clone` step of our
  own, was weighed: checkout is GitHub's first-party action and the pin removes drift.

Decisions (2026-11-16, with the user):
- **Licence MIT.**
- **The planning file and history stay public as they are** (host name RAIVEN, user name winux, paths, the
  private data repo's name and hashes; no secrets, commits use the GitHub noreply address).
- **Test workflow on a version tag and by hand** (`workflow_dispatch`), on Linux, Windows (Git Bash) and macOS
  (bash 3.2): no push or pull-request trigger.
- **Anthropic directory: later, after the macOS phase, and only on the user's direct command** (the user,
  when making the repo public: "do not publish to anthropic without my direct command").
- The user made the repo public (2026-11-16). A scan of the whole history (24 commits) found no secrets: only
  `jeeves-scan`'s own patterns and the tests' fake token.

Design:
- `LICENSE` (MIT, "Winux Nomacs"); `plugin.json`: `license`, `homepage`; `marketplace.json`: `description`,
  `$schema` if the validator accepts it; `claude plugin validate --strict` passes for both.
- **Versioning** (Claude chooses the bump, as for the user's packages): `CHANGELOG.md` from the history
  (0.1.0 split, update skill; 0.2.0 = Generic). Release `0.2.0` prepared on `release/0.2.0` (version,
  changelog), merged into `main`; the user tags (`v0.2.0` or `claude plugin tag`'s `jeeves--v0.2.0`) and pushes.
  1.0.0 after the macOS phase.
- **Workflow** `.github/workflows/test.yml`: `on: push: tags` + `workflow_dispatch`; matrix ubuntu, windows,
  macos; `bash tests/run.sh`. On macOS, force `/bin/bash` (3.2) first on PATH, because the runner may have a
  newer Homebrew bash and the scripts start with `#!/usr/bin/env bash`. Checked by a manual run after the
  repo is public.
- **README:** install from GitHub first, updating (`claude plugin update`, auto-update), pinning a tag, the
  VS Code extension note, Claude Code only (not claude.ai or Cowork), licence.
- **Plugin `CLAUDE.md`:** the "Pushing" section changes: the repo is public, a push to `main` reaches users
  only with a version bump, tags stay the user's; the workflow runs on tags and by hand only.
- **Going public is the user's step** (GitHub settings, or `gh repo edit WinuxNomacs/jeeves --visibility
  public --accept-visibility-change-consequences`); before it, a last check of tracked files and history
  for secrets (`jeeves-scan` covers the data repo only; a grep over `git log -p` here).
- This machine (RAIVEN) and RAIVEN-win keep their `directory` marketplaces (local clones), so plugin edits
  load in place; only other users install from GitHub.

Built (2026-11-16, branch `feat/publish`): `LICENSE`, `plugin.json` (`homepage`, `license`, descriptions
of the shared vs per-host split), `marketplace.json` descriptions, `.github/workflows/test.yml`, README
install section, pushing rules. `CLAUDE.md` moved to `.claude/CLAUDE.md`: at the root it was shipped to
plugin users to no effect, and `claude plugin validate --strict` failed on it. `tests/run.sh` checks the
licence, where the version is set, and the workflow's triggers (35 tests).

Checked (2026-11-16): `claude plugin validate --strict` passes. The workflow, started by hand on `main`
(run 43981362339): Linux (bash 5.3) and Windows (Git Bash 5.3) 35/35; macOS with `/bin/bash` 3.2.57 failed 2
tests, both path comparisons in the tests themselves (macOS's `TMPDIR` ends in `/` and BSD `mktemp` keeps the
`//`); fixed by normalizing the scratch path (`fix/tests-macos-tmpdir`), rerun 43981931017 green on all three.
A fresh install from GitHub in a scratch `CLAUDE_CONFIG_DIR` (version 0.1.0 then) installed, and its
scripts created a data repo and snapshotted into it; the scratch folder was removed. Release 0.2.0 prepared
on `release/0.2.0` (version, `CHANGELOG.md`).

Verify: `claude plugin validate --strict`; the workflow run by hand on all three systems after going
public; a fresh install from GitHub in a scratch `CLAUDE_CONFIG_DIR` (marketplace add, install with
`--config data_repo=…`, the version shown), removed afterwards.

## 1. macOS device (was 2)

The scripts' half is covered: the test workflow runs the suite on macOS with its own bash 3.2 (green since
run 43981931017). Left: a real Mac with Claude Code, by hand.

Install the plugin, clone the data repo, run `tests/run.sh` (user runs it by hand), first sync, resolve any first-sync conflicts.

## 2. Per-device overlay (was 3)

Mostly obsolete since Generic made `settings.json` and `keybindings.json` per host. Only needed if a shared
file (a `CLAUDE.md` rule, a skill) ever has to differ between hosts: then decide host vs platform scope and
add an overlay folder (`home.<host>/` or `home.<os>/`) merged by `apply`.
