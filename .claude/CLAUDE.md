# CLAUDE.md (this repo)

Jeeves (`jeeves`, display name Jeeves) is a Claude Code plugin that versions the user's global Claude Code
preferences (`~/.claude/CLAUDE.md`, `settings.json` and friends) in a separate **data repo** (the user's own:
`WinuxNomacs/jeeves-cmd`). This repo is the plugin code only; it holds no preferences.

## Start of every session here

Check `new.md` and `upcoming-changes.md` (phases) as the global rules describe. The SessionStart hook has
already snapshotted the data repo when the plugin is installed.

## Layout

- `.claude-plugin/plugin.json` — manifest (`userConfig.data_repo`); `.claude-plugin/marketplace.json` — the repo
  is its own marketplace (plugin source `"./"`).
- `hooks/hooks.json` — SessionStart → `jeeves-snapshot --hook`; PostToolUse `Write|Edit` → `jeeves-hook-edit`.
- `bin/` — on the Bash tool's PATH while the plugin is enabled, so every name starts with `jeeves-`.
  `jeeves-snapshot` (`~/.claude` → data repo, commits on its `main`), `jeeves-apply` (data repo → `~/.claude`),
  `jeeves-hook-edit` (snapshots after an edit to `~/.claude/CLAUDE.md`), `jeeves-status` (read-only: what is
  out of sync, with hashes), `jeeves-scan` (secrets in the outgoing commits), `jeeves-restore` (a host's
  backup → `~/.claude`), `jeeves-init` (new data repo), `jeeves-cleanup` (folds old history into one root
  commit and force-pushes with a lease; `--auto` runs from the session-start snapshot per `jeeves.conf`).
- `skills/` — `update` (`/jeeves:update`: when the user runs it, a review of the session for rules to save
  to the global and project CLAUDE.md; then sync, or a change to a shared file, then push), `setup`, `restore`,
  `cleanup` (`/jeeves:cleanup`: history cleanup and its `jeeves.conf` settings).
  They get the data repo from `${user_config.data_repo}` (substituted into skill text; the Bash tool sees no
  plugin variables).
- Shared files (`home/`, synced) vs per-device files (`hosts/<host>/`, backup only): the lists are at the top
  of `lib/lib.sh`.
- `lib/lib.sh` — shared code: finding the data repo, tracked paths list, hashes, three-way classification.
- `tests/run.sh` — tests; each runs against a scratch `~/.claude`, data repo and bare remote.
- This file is `.claude/CLAUDE.md`, for working on the repo; Claude Code loads it here as project context.
  At the root it would be shipped with the plugin to no effect, and `claude plugin validate --strict` fails.

## Working on the plugin

- Installed from this folder as a local marketplace, the plugin loads in place: `/reload-plugins` picks up
  edits. Hook scripts are live as soon as a file is saved, so run `bash -n` on a changed script before saving
  it over the working copy, and keep `main` working.
- After changing a manifest or `hooks.json`: `claude plugin validate .` and
  `claude plugin validate .claude-plugin/marketplace.json`.

## Rules for the scripts

- Portable bash: macOS bash 3.2, Git Bash, Linux. Run the tests under Windows Git Bash too when a change
  could differ there (from WSL: copy the tree to a `C:` folder and run `bash.exe -lc tests/run.sh`). No GNU-only flags (no `sed -i`, no `\|` in sed), no
  associative arrays, no `set -u`, no `jq`.
- Hooks must never block a session: hook mode always exits 0 and prints only what needs attention.
- The scripts never push, with one exception: `jeeves-cleanup` force-pushes the data repo's `main`, always
  with a lease (`--force-with-lease=main:<the origin/main it checked>`), after `jeeves-scan`, and puts `main`
  back if the push fails. Otherwise they contact the remote only for `fetch` + fast-forward in `snapshot`,
  and only the `update` and `cleanup` skills push.
- Never copy secrets: `.credentials.json`, `~/.claude.json`, transcripts, history.
- Tests: `tests/run.sh` (or `tests/run.sh <name part>`).

## Pushing and pulling (overrides the global "never push" rule, for this repo only)

The repo is public (`WinuxNomacs/jeeves`) and is the plugin's marketplace. Claude may pull from `origin` and
push branches to it whenever the work needs it, and push `main` after a merge whose full test suite passed,
without asking each time. A push to `main` reaches new installs at once (they take the default branch);
existing installs get it only when `plugin.json`'s `version` changes, which happens on a `release/X.Y.Z`
branch (version and `CHANGELOG.md`), merged like any other. Still never force-push, rewrite pushed history,
or tag: a tag marks a release and starts the test workflow, so it stays the user's call.

The only workflow, `.github/workflows/test.yml`, runs on a pushed tag or by hand (Actions tab, or
`gh workflow run tests`), never on a push or pull request; Actions minutes are free on a public repo, but ask
before adding a trigger or a workflow. Never submit the plugin to Anthropic's directory unless the user
directly asks for it.
