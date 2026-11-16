# CLAUDE.md (this repo)

Jeeves (`jeeves`, display name Jeeves) is a Claude Code plugin that versions the user's global Claude Code
preferences (`~/.claude/CLAUDE.md`, `settings.json` and friends) in a separate **data repo** (the user's own:
`WinuxNomacs/jeeves-cmd`). This repo is the plugin code only; it holds no preferences.

## Start of every session here

Check `new.md` and `upcoming-changes.md` (phases) as the global rules describe. The SessionStart hook has
already snapshotted the data repo when the plugin is installed.

## Layout

- `.claude-plugin/plugin.json` — manifest (`userConfig.data_repo`); `.claude-plugin/marketplace.json` — the repo
  is its own marketplace (plugin source `"."`).
- `hooks/hooks.json` — SessionStart → `jeeves-snapshot --hook`; PostToolUse `Write|Edit` → `jeeves-hook-edit`.
- `bin/` — on the Bash tool's PATH while the plugin is enabled, so every name starts with `jeeves-`.
  `jeeves-snapshot` (`~/.claude` → data repo, commits on its `main`), `jeeves-apply` (data repo → `~/.claude`),
  `jeeves-hook-edit` (snapshots after an edit to `~/.claude/CLAUDE.md`), `jeeves-status` (read-only: what is
  out of sync, with hashes), `jeeves-scan` (secrets in the outgoing commits).
- `skills/update/SKILL.md` — `/jeeves:update`: sync, or a preference change, then push. It gets the data repo
  from `${user_config.data_repo}` (substituted into skill text; the Bash tool sees no plugin variables).
- `lib/lib.sh` — shared code: finding the data repo, tracked paths list, hashes, three-way classification.
- `tests/run.sh` — tests; each runs against a scratch `~/.claude`, data repo and bare remote.
- This `CLAUDE.md` is for working on the repo; Claude Code does not load a plugin's `CLAUDE.md` as context
  for its users (`claude plugin validate` may warn about it).

## Working on the plugin

- Installed from this folder as a local marketplace, the plugin loads in place: `/reload-plugins` picks up
  edits. Hook scripts are live as soon as a file is saved, so run `bash -n` on a changed script before saving
  it over the working copy, and keep `main` working.
- After changing a manifest or `hooks.json`: `claude plugin validate .` and
  `claude plugin validate .claude-plugin/marketplace.json`.

## Rules for the scripts

- Portable bash: macOS bash 3.2, Git Bash, Linux. No GNU-only flags (no `sed -i`, no `\|` in sed), no
  associative arrays, no `set -u`, no `jq`.
- Hooks must never block a session: hook mode always exits 0 and prints only what needs attention.
- The scripts themselves never push; they contact the remote only for `fetch` + fast-forward in `snapshot`.
  Only the `update` skill pushes, after `jeeves-scan`.
- Never copy secrets: `.credentials.json`, `~/.claude.json`, transcripts, history.
- Tests: `tests/run.sh` (or `tests/run.sh <name part>`).

## Pushing and pulling (overrides the global "never push" rule, for this repo only)

Claude may pull from and push to `origin` (`WinuxNomacs/jeeves`, private) whenever the work needs it, without
asking each time: pull before starting work, push `main` after a merge, push branches to back them up. Still
never force-push, rewrite pushed history, or tag (a tag stays the user's call). Pushing publishes nothing
beyond the private GitHub repo: there are no workflows. Before adding any workflow, ask (Actions minutes on
private repos are billed). This changes in the Publish phase, when the repo becomes a public marketplace.
