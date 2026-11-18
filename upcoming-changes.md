# Upcoming changes

## Phases (current and upcoming)

- **Up next:** nothing planned
- **Later:** 1. Key rotation (a new key after a leaked passphrase) · 2. Follow symlinked folders under
  `~/.claude` · 3. Recover a cleanup killed after moving `main` · 4. Per-device overlay (only if a shared file
  must differ per host) · Anthropic directory submission (only on the user's direct command)

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
- 5. History cleanup (2026-11-17, from `new.md`): `bin/jeeves-cleanup` (`<days>`, `--dry-run`, `--auto`),
  `skills/cleanup/SKILL.md` (`/jeeves:cleanup`), `jeeves.conf` in the data repo, the automatic cleanup run by
  the session-start snapshot, and the snapshot's switch to a cleaned-up history; 43 tests in `tests/run.sh`
  (8 new, each failure injected), passing on Linux and under Windows Git Bash. Rules: the global CLAUDE.md
  allows a force-push only where a repo's CLAUDE.md names the task; `jeeves-cmd`'s CLAUDE.md names the
  cleanup. A dry run on `jeeves-cmd` found nothing to remove yet (its first commit was under a day old).
  Release 0.3.0. Details below under "5. History cleanup".
- 6. Session review in `/jeeves:update` (2026-11-17, the user's request): `skills/update/SKILL.md`,
  "Session review"; checks in `tests/run.sh` (the manifests test). Release 0.4.0. Details below under
  "6. Session review".
- 1. Guided setup (2026-11-17, from `new.md`): `skills/setup/SKILL.md` (five numbered steps),
  `bin/jeeves-check`, `bin/jeeves-github`, `with_deadline` and `os_name` in `lib/lib.sh`, the `data_repo`
  default `~/jeeves-data` (manifest and `find_data_repo`); 49 tests in `tests/run.sh` (stand-in `gh` and
  `ssh` that fail on request). The walk-through itself is not run live yet (the macOS phase). Release 0.5.0.
  Details below under "1. Guided setup".
- 2. Encryption (2026-11-17, from `new.md`): `lib/crypt.sh`, `bin/jeeves-crypt` (git filter, textconv,
  merge driver), `bin/jeeves-encrypt` (status, on, unlock, lock, passphrase), `jeeves-cleanup --all`, the
  lock checks in `find_data_repo` and the snapshot's pull, `jeeves-scan`'s check for unencrypted files,
  `jeeves-check`'s age and key store lines; skills `encrypt`, `unlock`, `lock`; setup offers encryption for a
  new repo and unlocks an encrypted clone. 63 tests in `tests/run.sh` (13 for encryption with real age and a
  stand-in key store and dialog, failures injected; skipped without age 1.3+), passing on Linux and under
  Windows Git Bash with the Windows `age.exe`. Checked live on RAIVEN (Windows 11, from WSL): the Windows
  Credential Manager round trip and the Windows password prompt (found and fixed: PowerShell's exit status
  lost in a pipe, a progress record on stderr, Cancel returned as an empty answer). Not run live yet: a real
  `/jeeves:encrypt` (the user's call), `secret-tool` and zenity.
  Details below under "2. Encryption".
- 3. Cleanup warning (2026-11-17, from `new.md`): `skills/cleanup/SKILL.md` step 3 and `/jeeves:encrypt`'s
  question say the folded commits go for good; README; a check in `tests/run.sh`. Details below.
- 4. Butler persona and reply conventions (2026-11-17, from `new.md`): a "Voice" section, the same in every
  skill (a check keeps them equal); README. Copy blocks and clickable options are also rules in the global
  CLAUDE.md (written through `/jeeves:update`). Release 0.7.0. Details below.
- 5. macOS device (2026-11-17, in a Claude Code session on the Mac): every earlier phase checked on macOS 26
  with bash 3.2 (63 tests, encryption included), plus live: the edit hook, restored settings' paths, the
  cleanup dry run, encryption with the Keychain and the macOS dialog (Cancel too), and the `/jeeves:setup`
  walk-through (fresh-machine paths, scratch). Found and fixed over the phase: the marketplace's plugin
  source `"."` refused by older Claude Code (0.2.1), older Claude Code refusing `userConfig` (README),
  setup's restore question never asked (0.2.2), and `jeeves-cmd` lacking the `jeeves-data` topic (added).
  No code change in the Mac session. Details below under "5. macOS device".
- Release 1.0.0 (2026-11-17): version and `CHANGELOG.md` only (no code change since 0.7.0, so no upgrade
  rehearsal); full suite and the test workflow run by hand on the release branch (Linux, Windows, macOS).
  Tags `v0.3.0` to `v0.7.0` were never pushed. The `v1.0.0` tag is on hold (2026-11-17): the release had no
  code review, so the pre-release review comes first and the release is bumped (1.0.1) if anything changes.
- 6. Release procedure (2026-11-17): `.claude/skills/release/SKILL.md` (`/release`), pointed to from
  `.claude/CLAUDE.md`; it includes the review for bugs, interaction breakdowns and secret leaks, and lists
  untagged release merges. Details below under "Release procedure (done)".
- 7. Encryption tests everywhere (2026-11-17): the workflow installs age 1.3.2 pinned by SHA-256 and fails
  if the encryption tests skip (run by hand on `ci/age`: 63 passed on Linux, Windows and macOS); age 1.3.2
  in `~/.local/bin` on RAIVEN (63 passed). Details below under "Encryption tests everywhere (done)".
- 8. Pre-release review and release 1.0.1 (2026-11-17): the whole plugin reviewed by an independent agent
  and in my own pass; all but three findings fixed on `fix/` branches, each with a test seen failing
  first where one could be written (78 tests).
  Details below under "Pre-release review and release (done)".

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

## 5. History cleanup (done)

**Request** (`new.md`, 2026-11-17): the data repo's history grows forever. (1) A cleanup that removes the
history older than a number of days the user gives, minimum 1; with 1, take a fresh backup first, then remove
every commit older than 24 hours. (2) The same cleanup automatically, every so many days, the minimum being
the cleanup's own number of days.

**Measured** (2026-11-17): `jeeves-cmd` has 45 commits after one day, `.git` is 2.5 MB (mostly loose
objects; the packed part is 4 KiB). Growth is slow (small text files), so this is housekeeping, not urgent.

**Builds on:** `jeeves-snapshot` (the fresh backup, fetch and fast-forward), `jeeves-scan`, the update skill's
push, `.sync-state/` (per-device sync record: blob hashes, which a rewrite does not change).

**Conflicts with standing rules, so the rules change (decision E):** the global CLAUDE.md ("never
force-push", "only rewrite history that has not been pushed") says that a repo's own CLAUDE.md may allow a
force-push or a rewrite of pushed history for a task it names, and that the rule holds everywhere else (done
2026-11-17 through `/jeeves:update`). When the phase is built, the data repo's CLAUDE.md ("Still never
force-push, rewrite pushed history") gets that permission for the cleanup only: only the data repo's `main`,
only with `--force-with-lease` (so a push another device made meanwhile is never overwritten), only when the
user ran it or turned on the automatic cleanup. This repo's CLAUDE.md ("the scripts never push") and the
update skill ("never force-push") name `jeeves-cleanup` as the one exception.

### Design (proposed)

- **`bin/jeeves-cleanup <days>`** (our own, git plumbing only; no `git filter-repo`, which would be a Python
  dependency on every device, Windows included):
  1. `jeeves-snapshot` (fresh backup; stops if the repo is dirty, not on `main`, or `main` has diverged).
  2. Cutoff = now − days × 24 h. The newest commit on `main` older than the cutoff becomes the new root: a
     parentless commit with the same tree, message `history before <date> removed by jeeves-cleanup`.
  3. Every later commit is recreated in order with `git commit-tree`, same tree, author, dates and message,
     its parents mapped (an old parent maps to the new root). Trees are identical, so every file and the
     `.sync-state` hashes stay valid.
  4. Check: the new `main`'s tree equals the old `main`'s tree, or stop without changing anything.
  5. The old history is kept locally as a bundle under `.sync-state/` (gitignored) until the next cleanup
     (decision B), `main` is moved, `jeeves-scan`, then
     `git push --force-with-lease=main:<the origin/main it started from> origin main`.
  6. Nothing older than the cutoff: say so and change nothing.
- **`/jeeves:cleanup <days>`** (skill, only the user starts it): explains what will be removed (how many
  commits, from which date), asks to confirm, runs `jeeves-cleanup`.
- **Other devices** after a cleanup: their `main` shares no history with the new `origin/main`. `jeeves-snapshot`
  detects that (no merge base, and the new root's message), keeps the old `main` in the same bundle, resets
  `main` to `origin/main`, then records disk as usual. As built (corrected while building): a plain reset
  would have lost a change the device never pushed, because its sync record already held that change, so the
  repo's older copy looked newer and `jeeves-apply` would install it. So, before the reset, each file changed
  in `<last fetched origin/main>..main` gets as its sync record the version in that last fetched
  `origin/main` (the one both devices last shared): the change is recorded again, or reported as a conflict
  with the right base when the other side changed the file too. With no fetched `origin/main` known, every
  differing file becomes a conflict. Both tests were seen failing without this.
- **As built, also:** after the push (or the switch), `git reflog expire` + `gc --prune=now` drop the
  removed objects, so `.git` really shrinks; the bundle is the copy. In `--auto`, the checks the snapshot
  already reported (dirty repo, diverged, offline) are silent. A push has no deadline beyond git's no-prompt
  setting and ssh's 5 s connect timeout: macOS has no `timeout` command.
- **Automatic cleanup:** the settings live in the data repo (one file, e.g. `jeeves.conf`: `keep_days`,
  `auto_every_days`), so all devices agree and only one needs to run it (decision A). The last cleanup's
  date is the root commit's, so no extra state. When due, the session-start hook runs it and pushes (decision C).
  `auto_every_days` must be ≥ `keep_days` (request (2)); a smaller value is refused with a message.
- **Out of scope: GitHub's own retention.** GitHub may keep the removed commits as unreachable objects for a
  while, and forks keep them; that is GitHub's to clean up (decision E). The aim is a data repo that stays
  small and readable over years. A secret already pushed must still be revoked, not cleaned up.

### What can fail (it runs unattended in automatic mode)

- Another device pushes between the fetch and the push: `--force-with-lease` refuses; nothing is lost; retry
  at the next session start.
- No network, or the push is rejected for any other reason: the local rewrite is undone (`main` back to the
  old commit, kept in the bundle) and the hook reports it once.
- Two devices clean up at once: the second one's lease fails; it then sees the first one's new root and
  resets as any other device does.
- A device offline during a cleanup with commits it never pushed: re-recorded from its disk at its next
  snapshot (above); its own old `main` stays as a local ref.
- Interrupted mid-rewrite (crash, Ctrl+C): the new commits are only objects until `main` moves, which is the
  last local step; a rerun starts over.
- A malformed `jeeves.conf`: the automatic cleanup is skipped with a message, never guessed.
- A merge commit whose parents straddle the cutoff: the old parent maps to the new root (tested).

### Tests (each failure above injected)

Cleanup with nothing old enough; with 1 day (commits older than 24 h gone, newer ones kept, same tree,
same authors and dates); a merge across the cutoff; the lease refusing (another push meanwhile); the push
failing (local main restored); another device resetting after a cleanup, with an unpushed commit
re-recorded; `auto_every_days` < `keep_days` refused; a malformed config skipped; automatic cleanup runs
only when due. Fake dates with `GIT_COMMITTER_DATE`.

### Decisions (2026-11-17, with the user)

- **A. Settings in the data repo**, one file shared by all devices (`jeeves.conf`: `keep_days`,
  `auto_every_days`), so they agree and only one device needs to run the cleanup.
- **B. Keep the removed history locally** as a bundle under `.sync-state/` (never pushed) until the next
  cleanup replaces it.
- **C. Fully automatic:** when due, the session-start hook cleans up and force-pushes with a lease; any
  failure is undone and reported.
- **D. Two numbers:** `keep_days` (minimum 1) and `auto_every_days` (how often the automatic cleanup runs,
  at least `keep_days`; unset means no automatic cleanup).
- **E. Rules:** the global rule allows a force-push only where a repo's own CLAUDE.md names the task (the
  data repo's, for the cleanup); GitHub's retention of removed commits is accepted as out of our control.

## 6. Session review (done)

**Request (2026-11-17):** when `/jeeves:update` runs, first assess the session for global and project
operational knowledge, revise the global CLAUDE.md with the global part and the project's CLAUDE.md with
the project part, so the global file learns over sessions instead of only when someone notices a lesson
(as with "stage files by name", found when a `git add -A` swept the user's `.gitignore` edit into a commit).

**Design:** skill text only (no script): collect from the session (most recent first) what the files don't
already say; sort by the global rules' "What goes where" into global, project and memory; word each item as
the rule it becomes, revising in place where a rule covers it; ask; write during the sync or change.

**Decisions (2026-11-17, with the user):**
- **When:** only when the user runs `/jeeves:update`, not when Claude starts the skill itself mid-task.
- **Confirm:** a selectable list per destination with an "All of these" option (AskUserQuestion holds 4
  options per question and 4 questions per call, so items are split over questions); afterwards, the
  unselected items are offered for discussion.
- **Project commits:** on the current branch when it is not the default one; on the default branch, a short
  `claude-md/<name>` branch merged back with `--no-ff`; never pushed unless that repo's CLAUDE.md allows it.

## 1. Guided setup (done)

**Request:** before release and publishing, walk a new user through installing step by step, with as little
interaction and technical knowledge as possible, always saying what is needed next; for example, check their
GitHub access and offer to create the data repo when they have none.

**Today** (read 2026-11-17): installing takes two terminal commands, and the install dialog asks for the
`data_repo` folder with no default. `/jeeves:setup` then needs the URL of an existing data repo, or the user
creates an empty private GitHub repo themselves (Jeeves creates none) and passes its URL. Nothing checks git,
GitHub access or the Claude Code version before a step fails. A machine with no data repo gets a
session-start message pointing to `/jeeves:setup` (0.2.2).

**Builds on:** `skills/setup/SKILL.md`, `jeeves-init`, `jeeves-restore`, the update skill's first sync,
`find_data_repo` (expands `~/`). From the plugin manifest reference (code.claude.com/docs/en/plugins-reference,
read 2026-11-17): a `userConfig` option takes `default`, `required` and `type: directory`; non-sensitive
values go to `settings.json` under `pluginConfigs`. `gh` is the GitHub CLI (version here: 2.46.0).

### Design (proposed)

- **Install:** `data_repo` gets the default `~/jeeves-data`, so the install dialog is accepted with Enter. The
  README's install becomes: two commands, then `/jeeves:setup` in a new session.
- **`/jeeves:setup` as a numbered walk-through** ("Step 2 of 5: GitHub access"), each step saying what it
  checks, what it found, and what comes next; every choice an AskUserQuestion with the recommended option
  first, so the user picks rather than types:
  1. **Tools:** git and bash present (Git for Windows on Windows), Claude Code recent enough.
  2. **GitHub access:** `gh auth status` (signed in as whom); without `gh`, `ssh -T git@github.com`. If `gh`
     is missing: offer to install it, showing this OS's command (Homebrew, winget, apt) and running it only
     after the user confirms; then the user signs in with `! gh auth login` (their own interactive login;
     Claude never handles the token). The manual way stays available. Never search for credentials.
  3. **Data repo:** with `gh`, look for the user's existing data repos (marked with a GitHub topic, e.g.
     `jeeves-data`) and offer to clone one: a second device needs no URL. None: offer to create a private one
     with `gh repo create jeeves-data --private` (another name on request; taken: ask) and `jeeves-init`. Without `gh`: ask for
     a URL, or explain step by step how to make an empty private repo on github.com.
  4. **Encryption:** "Secure your data with a passphrase?" (phase 2), for a new data repo; a cloned
     encrypted one asks for its passphrase here.
  5. **Restore** another machine's backup (as today: always asked).
  6. **First sync**, then a short "what happens from now on".
- **`bin/jeeves-check`** (new, read-only): one line per check (`git`, `bash`, `gh`, `gh-auth`, `ssh-github`,
  `claude-version`), each `ok`/`missing`/`fail` with a detail, so the skill reads facts instead of guessing,
  and the tests can fake each tool failing.

### As built (2026-11-17)

- **The default is Jeeves's own too.** An install without `--config` left `data_repo` unset despite the
  manifest's `default` (Claude Code 2.1.304: "1 userConfig option not yet set", nothing in `pluginConfigs`;
  with `required: true` it is "1 required"). Whether a session then substitutes the default was not
  checkable without a signed-in session, so `find_data_repo` and `jeeves-check` fall back to `~/jeeves-data`
  themselves, `required` is dropped, and the default stays for the install dialog. A machine not set up
  hears "Jeeves is not set up on this machine yet ... run /jeeves:setup" at each session start.
- **No Claude Code version check:** a plugin that loads (with `userConfig`) already proves it is new enough;
  `claude` is not on the PATH under the VS Code extension anyway.
- **Deadlines:** `with_deadline` (portable: macOS has no `timeout`) stops a hung `gh` or `ssh` by its own
  PID, TERM then KILL. `ssh` uses `StrictHostKeyChecking=yes`, so it never adds GitHub's host key itself;
  GitHub's "Hi <login>! You've successfully authenticated" comes with exit status 1, so the text is matched.
- **Arguments are checked before `gh` runs** (a test caught `gh config get` running before a bad name was
  refused).
- **Existing data repos need the topic** to be found: `WinuxNomacs/jeeves-cmd` had none; added on
  2026-11-17 in the macOS phase (`gh repo edit --add-topic jeeves-data`), and `jeeves-github find` lists it.

### What can fail

No network (each check times out and says so), `gh` installed but signed out, signed in to another account,
the repo name already taken, a clone refused (no access), a GitHub org that forbids private repos, Windows
without Git Bash, an old Claude Code (the install fails before setup: README says to update).

### Decisions (2026-11-17, with the user)

- **A. Missing `gh`:** offer to install it (the OS's command, shown and confirmed), manual way still offered.
- **B/C. Defaults:** folder `~/jeeves-data` on each machine; new GitHub repo `jeeves-data`, private, with a
  topic so other devices find it.

## 2. Encryption with a passphrase (done)

**Request:** before release and publishing, an easy option to encrypt the user's data in git, so they can
secure it with a passphrase; research options with better git/GitHub integration than ansible-vault.

**Builds on:** the data repo layout, `hash_of` (already `git hash-object --no-filters`), the update skill's
merges, `jeeves-scan`, setup (phase 1), the history cleanup (to fold away history encrypted under an old
key). What it protects: the data on GitHub (a leaked token, a repo made public by mistake). Not a device
whose account is already open: the plain files are in `~/.claude` anyway.

### Options checked (2026-11-17, primary sources: each project's repo, releases, source, packages)

| Option | Verdict for Jeeves | Evidence |
|---|---|---|
| **Our own on age** (chosen) | Fits: passphrase-protected key committed, unattended after one unlock per device | age v1.3.2 (2026-08-29), BSD-3, prebuilt for Windows/macOS/Linux, Homebrew/winget/scoop; its docs recommend a passphrase-encrypted identity file |
| transcrypt | Best ready-made fit (bash + openssl, passphrase, merge driver) but weak today | v2.3.2 (2026-05-18); key from the password via `openssl enc -md MD5` without PBKDF2 (its script, lines 298-333); fix PR #219 open since 2026-07-13 |
| git-crypt | No passphrase mode; no Windows build of the current release | 0.8.0 (2025-09-24), only a Linux binary attached; issue #75 (passphrase) open since 2016 |
| git-remote-gcrypt | Hides names and messages too, but re-uploads the whole history on every push to GitHub and every push is forced | its README; 1.5 released 2022, 1.6 unreleased |
| sops | Markdown becomes an opaque blob; working tree stays encrypted | v3.13.3 (2026-07-23) |
| ansible-vault | Python 3.13+, Windows only under WSL | ansible-core 2.22.0 (2026-11-02) |
| git-secret, git-agecrypt | Manual hide/reveal (2022); author says "you probably shouldn't" use it | their READMEs |

Also checked: `openssl enc` refuses AEAD ciphers ("enc: AEAD ciphers not supported", tested here), so an
openssl build would need its own HMAC. No GitHub-native client-side encryption was found.

### Design (proposed)

- **Keys:** `jeeves-encrypt` makes an age key pair; the private key is committed as `identity.age`,
  encrypted with the user's passphrase, and the public key as `recipient` (plain: encrypting needs only it). Each device enters the passphrase once (setup, or
  `/jeeves:unlock`); the decrypted private key goes to the **OS credential store** (decision E2): macOS
  Keychain (`security`), Windows Credential Manager (read through PowerShell, which has no built-in reader),
  Linux Secret Service (`secret-tool`); WSL probably through Windows' store via `powershell.exe` (to check).
  The passphrase itself is never stored. Where no store is available, say so and ask (a key file is the
  fallback to offer).
- **Passphrase entry (decision E4):** never through the Claude chat (it is saved in transcripts) and never
  as a command argument. An OS dialog: `osascript` on macOS, the Windows credential prompt (`Get-Credential`,
  also from WSL through `powershell.exe`), `zenity --password` on a Linux desktop; with none available, a
  command for the user to paste into their own terminal. The script hands it to age as `AGE_PASSPHRASE` of
  that one age process (`age -j batchpass`), never on a command line or in a file. Built that way rather
  than with `AGE_PASSPHRASE_FD` as first planned: a file descriptor from bash is not inherited by the Windows
  `age.exe`, and a process's environment is readable only by the same user, who can read the key store
  anyway. Needs age 1.3.0 or later with `age-plugin-batchpass` (age itself reads a passphrase only from a
  terminal, which Claude's shell lacks; checked in its source 2026-11-17). Packages (2026-11-17): Homebrew
  1.3.2, winget `FiloSottile.age` 1.3.2 (installs `age-plugin-batchpass` too, per its manifest), Ubuntu
  24.04 apt 1.2.1 (too old: setup points to the release download).
- **Making the passphrase (decision E5):** the user's own. Before asking, explain that a passphrase lost on
  every device means the data on GitHub can't be read (an unlocked device can set a new one without the old),
  and recommend generating one with a trusted password manager. Typed or pasted twice, and both must match.
  How they keep it is up to them.
- **Key once per script run (measured 2026-11-17):** a read from the Windows store costs about 1-1.5 s
  (PowerShell start from WSL 620-750 ms; `Add-Type` CredRead 1.5 s; DPAPI 0.9 s), and git starts the filter
  once per file (133 files in jeeves-cmd today). So each Jeeves script reads the key once and passes it to
  the git it runs in an environment variable of that process only; the filter, `textconv` and merge driver
  read the store only when it is missing (git run by hand).
- **Git integration:** a clean/smudge filter (`filter.jeeves.required=true`, so a missing key fails rather
  than committing plain text), a `textconv` for diffs, and a merge driver that decrypts, merges with
  `git merge-file`, and encrypts, so the working tree stays plain and the update skill's merges keep working.
  age output differs each run, so the filter keeps a plain-hash → ciphertext cache in `.git/jeeves/`
  (otherwise every `git status` sees a change); encrypting needs no private key, so `git status` never reads
  the store. `jeeves-scan` scans the plain text.
- **Scope (decision E3):** every file except the repo's own config (`.gitignore`, `.gitattributes`,
  `README.md`, `jeeves.conf`, `identity.age`). Visible on GitHub still: file names (including project paths
  under `hosts/`), sizes, commit messages.
- **Commands:** `/jeeves:encrypt` (turn on for an existing repo: re-commits every file encrypted, then
  folds all history into that one encrypted commit at once with the history cleanup, decision E6; GitHub may
  keep unreachable copies for a while, outside our control), `/jeeves:unlock`, `/jeeves:lock` (removes the key from the
  store), passphrase change (re-encrypts only `identity.age`).
- **Locked device:** hooks exit 0 and say once that the data repo is locked and how to unlock.

### As built (2026-11-17)

- **Key to age:** on stdin (`age -d -i -`), which works with the Windows `age.exe` too; the ciphertext goes
  to a temp file in `.git/jeeves/tmp/`. To the stores: on stdin as well (`security -i`, `secret-tool store`,
  PowerShell `[Console]::In`), never on a command line.
- **Windows Credential Manager:** a small C# type over `CredRead`/`CredWrite`/`CredDelete` through
  `powershell.exe -EncodedCommand`; generic credentials `jeeves:<recipient>`, persisted on this machine only.
  Measured live: 1.4-1.6 s per call.
- **Speed:** git compares only size and whole-second mtime in an encrypted data repo (`core.checkStat=minimal`,
  `core.trustctime=false`), since the snapshot rewrites `hosts/` with the same times every run. On a copy of
  jeeves-cmd (133 files): a snapshot 0.5 s encrypted vs 0.8-0.9 s plain; `encrypt on` 11 s, mostly the filter
  started once per file (48 ms each, age itself 15 ms). On Windows that one-time cost (also an unlock of a
  new device, which decrypts every file) is several times higher; not measured. If it matters, git's
  long-running filter protocol (`filter.<driver>.process`) is the fix.
- **Lazy key:** the snapshot reads the key only when origin brought something to check out, so a session
  start with nothing new costs no store read.
- **Locked device:** `find_data_repo` refuses (all scripts but `jeeves-encrypt`) with "run /jeeves:unlock";
  `.git/jeeves/unlocked` holds the recipient. A device that pulls another device's encryption while locked
  does not take it.
- **Base versions:** `set_base` writes the synced version into git's objects (`hash-object -w`, unreachable,
  never pushed), so the update skill's `git cat-file -p <base>` still works when git stores files encrypted.
  `gc` may prune them later; a merge then has no base and keeps both sides.
- **Snapshot:** stops (records nothing) when `git add` fails, and checks age before copying anything.
- **Passphrase:** at least 12 characters, typed twice.
- **Turning it on:** `jeeves-encrypt on` commits; `/jeeves:encrypt` then runs `jeeves-cleanup --all`. During
  setup, a new repo is encrypted before its first push, so there is no history to fold.
- **Cache:** `.git/jeeves/clean/` (plain hash -> ciphertext) grows by one small file per version; not pruned
  yet (about 5 KB per version on jeeves-cmd).

### What can fail

No age installed (setup offers to install it, like `gh`); no credential store (WSL, a headless Linux); a
wrong passphrase; the key missing on a device that pulls encrypted commits (filter fails loudly, snapshot
reports "locked"); the cache out of date; a merge driver failure (falls back to a conflict, never to plain
text in a commit); CRLF on Windows; speed: git runs the filter per file, and reading the key from a
credential store each time may be slow (measure; cache the key per process if needed).

### Decisions (2026-11-17, with the user)

- **E1. Approach:** our own, on age.
- **E2. Key storage:** the OS credential store, unattended after one unlock per device.
- **E3. Scope:** all data, offered during setup.
- **E4. Passphrase entry:** an OS dialog, falling back to the user's own terminal.
- **E5. Making the passphrase:** the user's own, typed twice, after explaining what losing it means and
  recommending a password manager to generate it.
- **E6. Old history:** folded away right after encrypting.

## 5. macOS device (done)

The scripts' half is covered: the test workflow runs the suite on macOS with its own bash 3.2 (green since
run 43981931017, and on tag `v0.2.2`). Done by hand on the Mac (2026-11-17): installed 0.2.1 then 0.2.2 from
GitHub, data repo cloned, first sync (its CLAUDE.md rules merged into the shared one), restore of RAIVEN's
backup run.

**The work moves to a Claude Code session on the Mac** (the user's decision, 2026-11-17). Only one session
works on this repo at a time: the one on RAIVEN is closed first. Session transcripts and memory don't move
between machines (memories are keyed by the project's path, `/home/winux/...` vs `/Users/winux/...`), so the
handoff is this file and `.claude/CLAUDE.md`; everything the Mac session needs is written here before it
starts.

Handoff steps:
1. On RAIVEN: everything committed, merged and pushed; this section current; close the session.
2. On the Mac, in a terminal (the user uninstalled the Mac's 0.2.2 on 2026-11-17, so no old Jeeves runs
   there; if a `marketplace remove` below says it is not found, that is fine): `git clone https://github.com/WinuxNomacs/jeeves.git
   ~/gitrepos/config/jeeves`; switch the plugin from the GitHub marketplace to that clone so edits load with `/reload-plugins`
   (`claude plugin marketplace remove jeeves`, `claude plugin marketplace add ~/gitrepos/config/jeeves`,
   `claude plugin install jeeves@jeeves --config data_repo="$HOME/gitrepos/config/jeeves-cmd"`).
3. Open Claude Code in `~/gitrepos/config/jeeves` and say "continue the macOS phase in upcoming-changes.md".

The Mac session then:
- runs `tests/run.sh` on the Mac's own bash and git;
- checks the edit hook: a direct edit to `~/.claude/CLAUDE.md` (not through `/jeeves:update`) must commit
  `snapshot(Winuxs-MacBook-Pro): CLAUDE.md` and show the notice;
- checks the restored `settings.json`: `data_repo` and the marketplace path point at Mac folders;
- runs the history cleanup on the Mac (built in 0.3.0): `tests/run.sh cleanup`, then
  `jeeves-cleanup --dry-run 1` on the real data repo; a real cleanup only if the user asks;
- encryption (built in 0.6.0): `brew install age` (ask first), `tests/run.sh` (the encryption tests run with
  age on the PATH), then the Keychain and the macOS dialog live on a scratch data repo
  (`JEEVES_DATA=<scratch> jeeves-encrypt on`, then `lock` and `unlock`); a Keychain entry is
  `security find-generic-password -s jeeves`; delete the scratch entry afterwards;
- then, if the user wants it, encrypts the real data repo (`/jeeves:encrypt`, from the Mac or RAIVEN) and
  unlocks the other device. RAIVEN (WSL) needs age 1.3+ first: apt has only 1.2.1, so the official release
  download into `~/.local/bin`. Every device that still syncs must run 0.6.0 or later before encrypting;
- fixes what it finds on a branch, merges, pushes `main` (allowed by `.claude/CLAUDE.md`), and moves this
  phase to Done.

Checked in the Mac session (2026-11-17; macOS 26.6 arm64, `/bin/bash` 3.2.57, git 2.56.0, Claude Code
2.1.311). Handoff step 2 had not been done, so the plugin was installed from the local clone here.
- `tests/run.sh`: 50 passed without age; after `brew install age` (1.3.2, the bottle includes
  `age-plugin-batchpass`) 63 passed, 0 failed.
- Restored settings: the Mac's only backup (`4c9680b`) predates the earlier restore, and the uninstall
  rewrote `settings.json`, so it was rehearsed on a scratch `~/.claude`: `jeeves-restore RAIVEN` wrote
  RAIVEN's `/home/winux/...` marketplace path and `data_repo`; the restore skill's step 4 pointed both at
  existing Mac folders, valid JSON, nothing else machine-specific.
- History cleanup: `jeeves-cleanup --dry-run 1` first refused (the clone was 15 commits behind
  `origin/main`: correct); after a fast-forward, "would fold the 30 commits up to 2026-11-16 15:23 into
  one, and keep the 30 newer ones". No real cleanup (not asked).
- Encryption on a scratch data repo: `on` (macOS dialog twice, key in the Keychain under service `jeeves`,
  account the public key; the commit holds age ciphertext), `lock` (entry gone; status refuses with "run
  /jeeves:unlock"), `unlock` (dialog once, entry back), and Cancel in the dialog ("cancelled; nothing
  changed", exit 1, still locked). No scratch Keychain entry left.
- The Mac's `~/.claude/CLAUDE.md` was behind the shared one (RAIVEN's later rules); `jeeves-apply`
  installed it.
- Edit hook, after `/reload-plugins`: a direct Edit of the real `~/.claude/CLAUDE.md` (one word) was
  committed within seconds as `726b2d2 snapshot(Winuxs-MacBook-Pro): backup settings.json CLAUDE.md` (the
  settings backup rode along: the install had changed it). Local only until `/jeeves:update` pushes it.
- `/jeeves:setup` walk-through, fresh-machine paths, in this session on a scratch `~/.claude` and an empty
  scratch data repo folder (`JEEVES_HOST=NewMac`): `jeeves-check` all ok (gh signed in, keychain, age);
  `jeeves-github find` listed nothing (the missing topic, above; after adding it, the first call right
  after the edit was still empty, the next one listed `jeeves-cmd`: GitHub's lag); HTTPS clone with gh's
  sign-in; not encrypted, so no unlock; `jeeves-restore` offered RAIVEN-win, RAIVEN, Winuxs-MacBook-Pro and
  saved the scratch `settings.json` first; the first snapshot committed the backup and reported
  `CLAUDE.md` as a conflict with base `none`; merged keeping both sides, `jeeves-apply --force`, the second
  snapshot printed nothing, status listed no paths, `jeeves-scan` clean. Not pushed (scratch).
- The plugin cache (`~/.claude/plugins/cache/jeeves/jeeves/0.7.0/`) is a copy of the clone, not a link,
  but it is not what runs: `/reload-plugins` reported "`jeeves@jeeves` re-read from its folder:
  `~/gitrepos/config/jeeves`", and the Bash tool's PATH holds the clone's `bin/`. An untracked probe file
  stayed out of the cache after that reload. So `.claude/CLAUDE.md`'s "loads in place" holds; the cache
  folders (`0.2.1`, `0.2.2`, `0.7.0`) are leftovers.

Facts the Mac session needs (from RAIVEN's memory, which does not travel): the data repo is
`WinuxNomacs/jeeves-cmd` (private), hosts so far `RAIVEN` (Linux, WSL), `RAIVEN-win` (Windows, same PC),
`Winuxs-MacBook-Pro`. RAIVEN-win is retired (2026-11-17): its plugin is uninstalled and its plugin clone deleted; it comes back only for Windows testing, installed fresh from the GitHub marketplace,
and its last backup stays in `hosts/RAIVEN-win/`. WSL and native Windows on one PC share one Windows
Credential Manager entry for the key, so `/jeeves:lock` on one locks both. The user tags releases (or asks Claude to, for one tag at a time; this repo's tags are
annotated: `git tag -a`); nothing goes to Anthropic's directory without the user's direct command.

## 3. Cleanup warning (done)

**Request:** "When cleanup is ran manually, double check with the user before moving forward so they
understand that commits older than the given number of days will be gone forever."

**Already built:** `/jeeves:cleanup <days>` runs a dry run and asks with AskUserQuestion, naming how many
commits are folded and up to which date (`skills/cleanup/SKILL.md` step 3); it never runs without that yes.
What is missing is saying what "fold" costs. The accurate wording: those commits are removed from GitHub and
from every device; this device keeps them in `.sync-state/history-before-cleanup.bundle` only until the next
cleanup replaces it, and after that they are gone for good (GitHub may keep unreachable copies for a while,
outside Jeeves' control). The files themselves do not change.

**Proposed:** reword step 3's question (and the README) so the answer options say it: "Yes, remove them
permanently" / "No, keep them". Same for `/jeeves:encrypt`'s fold. Text-only change: no rehearsal needed.

## 4. Butler persona and reply conventions (done)

**Requests:**
- "Can the Jeeves plugin imitate a persona so that its responses are always that of a wise & whimsical
  gilded age english butler? Let's discuss this more."
- "Make it so that Jeeves always provides data that is destined for another program a single click copy
  block to make it easier for the user to interact with it."
- "Make it so that Jeeves always presents clickable response options to make it easier for touch
  screen/mouse users. Offer the obvious suggestions where possible for them to choose with a mouse/tap if
  they desire. They can always type out a full response via Other if they so choose."

**What a plugin can do (Claude Code docs, plugins reference and components pages, read 2026-11-17):**
- **Output style:** `output-styles/<name>.md` in the plugin, listed in `/output-style` as `jeeves:<name>`;
  the user picks it (the docs describe no way for a plugin to turn it on by itself). It changes how every
  reply is phrased; `keep-coding-instructions: true` keeps Claude Code's software-engineering instructions.
- **SessionStart hook:** Jeeves already has one; it could add the persona's instructions to every session
  (always on while the plugin is enabled, unless a plugin option turns it off). To check: how a hook adds
  context (`additionalContext` in its JSON output) and how strongly that holds over a long session.
- **Only in Jeeves' own skills:** the persona in `/jeeves:*` replies only.
- **The two conventions:** a copy block is a fenced code block (the VS Code extension shows a copy button
  on them; the terminal: to check). Clickable options are the AskUserQuestion tool (up to 4 options plus
  "Other"). Both could instead be rules in the user's global CLAUDE.md (synced by Jeeves), which would apply
  with or without the persona.

**Open decisions:**
- P1. Scope: every reply in every session, or Jeeves' own skills only?
- P2. Mechanism: an output style the user selects, or always on through the SessionStart hook (with an
  option to turn it off)?
- P3. The conventions: part of the persona, or global CLAUDE.md rules for every session?
- P4. Tone limits: never at the cost of the global style rules (concise, no emojis, accuracy first)?
  Whimsy in greetings and asides only, never inside code, commands, commit messages or files?

**Decisions (2026-11-17, with the user):** P1 the `/jeeves:` skills only; P2 no output style (an output
style would change every reply, so none is shipped); P3 both: in the skills and as global CLAUDE.md rules;
P4 not asked, built as proposed (prose only, global style rules win, no gendered forms of address). Built
before the macOS phase.

**As built:** the "Voice" section sits before each skill's first section (`lock` gained a "Steps" heading).

## Release procedure, encryption tests, pre-release review (planned 2026-11-17)

**Request:** plan the procedure that leads up to every release, from what earlier releases needed. The user
added: every release gets a code review for bugs and for UI and interaction breakdowns that would hurt the
user's experience, and, since the plugin handles secrets, a security review that the code is secure and
leaks no secret anywhere.

**Builds on:** the release steps used so far (global "Packages the user owns", this repo's `CLAUDE.md`), the
test workflow, `tests/run.sh`.

### Release procedure (done)

Built as `.claude/skills/release/SKILL.md`, pointed to from `.claude/CLAUDE.md` (Working on the plugin,
Layout). The user tags when they say so; then Claude runs the tag command for that release. Planned steps:

`.claude/skills/release/SKILL.md`, pointed to from `.claude/CLAUDE.md`. Steps:

1. **Ready?** "Up next" built and merged, `new.md` empty, no reported bug open, `main` = `origin/main`, the
   data repo synced. List release merges without a tag and give the user the `git tag -a` commands.
2. **What changed:** diff since the last release, sorted into scripts, hooks, skill text, saved state or
   data repo layout, docs; this picks the checks below. Version: major for a breaking change to the data
   repo layout, saved state or commands; minor for a feature; patch for a fix or text.
3. **Review** (fixes on their own branches, then the checks again):
   - Bugs and edge cases (empty folders, odd names, diverged `main`, interrupted runs); hooks never block or
     clutter a session; skill flows: clickable choices with the recommended one first, nothing destructive
     without a clear yes, messages that say what to do next, no dead ends (no data repo, locked device,
     refused push).
   - Security: every path a passphrase or key takes (never on a command line, in the chat, a log, `set -x`,
     an error message or git config; to key stores on stdin; temp files private and always removed;
     `JEEVES_AGE_KEY` only where needed); plain text can never reach a commit or a push (required filter,
     ciphertext cache, what `jeeves-scan` catches); credentials, transcripts and `~/.claude.json` never
     copied; injection and path traversal through host names, paths, `gh` output.
   - Public repo: nothing private in the diff (personal paths, host names, emails, data repo content);
     README covers every feature; the Layout list in `.claude/CLAUDE.md` is complete.
   - An independent reviewer given the code, not my conclusions, plus my own pass; every finding verified
     before it is reported.
4. **Checks, always:** `bash -n` on every script; both manifests validate (`--strict`); the full suite,
   with the count, encryption tests run (not skipped).
5. **Checks by what changed:** scripts: Windows Git Bash from a `C:` copy; macOS- or Windows-only code: ask
   the user for a live run there; code or saved state: the upgrade rehearsal (last release's `bin/` in a
   worktree, then the new one on its state; the published plugin in a scratch `CLAUDE_CONFIG_DIR`); skill
   text only: run the skill once against scratch folders. Say which ones were skipped and why.
6. **`release/X.Y.Z`:** version, `CHANGELOG.md`, Done in this file.
7. **CI by hand on the release branch:** confirm the branch push succeeded before `gh workflow run tests
   --ref release/X.Y.Z` (1.0.0: a failed push gave "No ref found"); all three OSes green; read the
   annotations (runner image changes).
8. **Merge** `--no-ff`, push `main`, delete the branch.
9. **After:** the user tags; the tag's run is green; then `claude plugin marketplace update` and
   `claude plugin update` in the scratch config show the new version's cache folder; clone installs pull.

### Encryption tests everywhere (done)

The 13 encryption tests skip without age 1.3+: RAIVEN's apt has 1.2.1 and the workflow installs none, so
for 1.0.0 they ran only by hand on the Mac.

- CI: a step that downloads a pinned age release and checks its checksum on all three OSes. Minutes are free
  here: the repo is public, and GitHub's Actions billing page says standard runners in public repositories
  are free and "do not reduce the included compute usage" (fetched 2026-11-17).
- RAIVEN: age 1.3+ from the release download into `~/.local/bin` (user-level; the command shown first).
- Vet age's current state from its repo and releases first, and record the evidence here.

**age checked 2026-11-17** (GitHub API on `FiloSottile/age`): BSD-3-Clause, not archived; latest release
v1.3.2 (2026-08-29; v1.3.0 and v1.3.1 in 2025-12), commits up to the release day; one main maintainer
(Filippo Valsorda, 333 commits; the next contributor 6), issues answered into 2026-11; one advisory,
GHSA-32gq-x56h-299c (plugin names could run arbitrary binaries), fixed in 1.2.1. Release archives hold
`age`, `age-keygen` and `age-plugin-batchpass` (plus the pq and tag plugins), with no dependency to
install; each comes with a Sigsum `.proof`. Pinned by SHA-256, each matching GitHub's asset digest:
linux-amd64 `cbe24006...`, darwin-arm64 `e2020b07...`, windows-amd64 `f48d8f8f...`. Building our own
is not an option (the plugin already depends on age for encryption).

**As built:** the workflow's "Install age" step downloads v1.3.2 for the runner's OS, checks the SHA-256,
and puts it on the PATH; the "Tests" step fails if the encryption tests were skipped. To upgrade age, change
the version and the three hashes (from the release's asset digests). RAIVEN: age 1.3.2 (`age`,
`age-keygen`, `age-plugin-batchpass`) in `~/.local/bin` from the same verified archive; 63 passed there.

### Pre-release review and release (done)

Run the procedure on the whole plugin (not only the diff since 0.7.0: no release has had this review),
fix what it finds, then release (1.0.1 if anything changed, else tag 1.0.0 as merged).

Fixed (branch, test):
- `fix/scan-every-commit`: the scan read only the origin/main..main tree diff; it now reads each outgoing
  commit (a merge's combined diff) and checks every outgoing blob for encryption.
- `fix/remote-deadline`, `fix/crypt-hardening`: `remote_git` under `with_deadline` (30 s), `GCM_INTERACTIVE=never`,
  the user's ssh command kept, no `JEEVES_AGE_KEY` in fetch, push or `cleanup --auto`.
- `fix/recipient-check`: `store_*` take only `age1` + bech32 (PowerShell and `security -i` injection, `../`).
- `fix/snapshot-safety`: an emptied `~/.claude` (with sync records) records nothing; `CLAUDE_CONFIG_DIR`;
  `uname -n` fallback and a refused odd host name; leftovers of a failed snapshot are its own; deletions
  reported; no routine commit in hook output.
- `fix/data-repo-marker`: the current-folder fallback needs `.sync-state/` in `.gitignore`.
- `fix/crypt-hardening`: the clean cache cleared at `on` and on failure; a failed `on` commit undone;
  `umask 077` in `jeeves-crypt`; cleanup traps HUP; `jeeves-github create` refuses `-name`.
- `fix/keep-bases`: `refs/jeeves/bases` (local, never pushed) keeps merge bases from gc.
- `fix/snapshot-lock`, then in `fix/plain-history`: a lock file by noclobber. uutils mkdir 0.10.0 (Ubuntu
  26.04's default) let two processes create the same folder, so a mkdir lock failed one time in three.
- `fix/plain-history`: `plain_history_at`, reported by the snapshot and `jeeves-encrypt status`; the
  passphrase texts say the old `identity.age` stays in the history.
- `fix/scan-way-out`: README on `settings.json` `env`; the update skill's way out of a scan finding.

Left for later (in "Later"): key rotation; symlinked folders (files under them are not synced, nothing is
reported); a cleanup killed with SIGKILL between moving `main` and the push (the next snapshot says
"diverged" and a merge refuses unrelated histories). Dropped: macOS host names following DHCP (the Mac's
name stayed `Winuxs-MacBook-Pro`).

Checks: 78 passed, 0 failed (no encryption skip); both validates; upgrade rehearsal (1.0.0's `bin/` wrote a
plain then encrypted, cleaned-up data repo; the new `bin/` snapshotted, applied, scanned and pushed it, and
created `refs/jeeves/bases`); Windows Git Bash and macOS bash 3.2 through the test workflow on the release
branch.

### Decisions (2026-11-17, with the user)

- The procedure is a repo skill.
- A review for bugs, interaction breakdowns and security is part of every release.
- Encryption tests: in CI and on RAIVEN.
- The `v1.0.0` tag waits for the review.
- The procedure lists untagged releases and gives the commands; tagging stays the user's.

## 4. Per-device overlay

Mostly obsolete since Generic made `settings.json` and `keybindings.json` per host. Only needed if a shared
file (a `CLAUDE.md` rule, a skill) ever has to differ between hosts: then decide host vs platform scope and
add an overlay folder (`home.<host>/` or `home.<os>/`) merged by `apply`.
