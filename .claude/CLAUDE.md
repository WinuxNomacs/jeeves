# CLAUDE.md (this repo)

Jeeves (`jeeves`, display name Jeeves) is a Claude Code plugin that versions the user's global Claude Code
preferences (`~/.claude/CLAUDE.md`, `settings.json` and friends) in a separate **data repo** (the user's own:
`WinuxNomacs/jeeves-cmd`). This repo is the plugin code only; it holds no preferences.

## Purpose (the user's, 2026-11-18)

Jeeves exists to improve Claude Code's operational knowledge over time, across many separate sessions and
devices: `/jeeves:update` keeps the global and project CLAUDE.md (and the skills they point to) meaningful
and effective for the current and future models that read and maintain them. Backing up settings is
secondary. Weigh every change against that. What Jeeves writes must only ever improve how Claude works,
never hinder it (2026-11-19): it captures the intent of what the user asks and fits it to the whole ruleset,
or says which rule already covers it, without asking the user to approve wording (a later run may refine it).

## Start of every session here

Check `new.md` and `upcoming-changes.md` (phases) as the global rules describe. The SessionStart hook has
already snapshotted the data repo when the plugin is installed.

## Layout

- `.claude-plugin/plugin.json` — manifest (`userConfig.data_repo`); `.claude-plugin/marketplace.json` — the repo
  is its own marketplace; its plugin entry is pinned to the last release's tag and commit (a `github` source),
  so installs get only signed releases. `MANIFEST`, `MANIFEST.sig`, `release-key.pub` — the signed release.
- `hooks/hooks.json` — SessionStart → `jeeves-snapshot --hook`; PostToolUse `Write|Edit` → `jeeves-hook-edit`.
- `bin/` — on the Bash tool's PATH while the plugin is enabled, so every name starts with `jeeves-`.
  `jeeves-snapshot` (`~/.claude` → data repo, commits on its `main`), `jeeves-apply` (data repo → `~/.claude`),
  `jeeves-hook-edit` (snapshots after an edit to `~/.claude/CLAUDE.md`), `jeeves-status` (read-only: what is
  out of sync, with hashes), `jeeves-scan` (secrets in the outgoing commits), `jeeves-risk` (lines in the outgoing commits that could plant instructions or code; `jeeves-apply` holds incoming skills, agents and commands until accepted), `jeeves-restore` (a host's
  backup → `~/.claude`), `jeeves-init` (new data repo), `jeeves-check` (read-only facts for setup: tools,
  GitHub sign-in, age, data repo folder), `jeeves-github` (find or create the data repo on GitHub through
  `gh`), `jeeves-cleanup` (folds old history into one root commit and force-pushes with a lease; `--auto`
  runs from the session-start snapshot per `jeeves.conf`; `--all` after encrypting), `jeeves-encrypt`
  (encryption on, unlock, lock, passphrase), `jeeves-crypt` (git's filter, textconv and merge driver for an
  encrypted data repo; git runs it once per file, so it sources only `lib/crypt.sh`), `jeeves-memories`
  (the memories the session review has not reviewed yet, and the shared record `hosts/.memory-review`),
  `jeeves-git` (git in the data repo), `jeeves-verify` (the plugin's files against its signed `MANIFEST`,
  the release key recorded at setup in `~/.claude/jeeves/allowed_signers`; standalone, sources nothing).
- `skills/` — `update` (`/jeeves:update`: when the user runs it, a review of the session and of every
  device's unreviewed memories: it decides on its own what to save to the global and project CLAUDE.md,
  prunes them, asks only on a conflict with a rule the user set, and reports each change with its revert;
  then sync, or a change to a shared file, then push), `jeeves` (`/jeeves <request>`, the short form: it
  hands the request to `update`; alone, it asks: a rule idea, review and sync, the rules review, or undo;
  named after the plugin, so the bare `/jeeves` reaches it), `review` (`/jeeves:review`: with the user, the
  rules that hold results back, the user's firm ones included; rewords them once the user agrees; reads
  every memory through `jeeves-memories list --all`), `setup`, `restore`,
  `cleanup` (`/jeeves:cleanup`: history cleanup and its `jeeves.conf` settings), `encrypt`, `unlock`, `lock`.
  They get the data repo from `${user_config.data_repo}` (substituted into skill text; the Bash tool sees no
  plugin variables), passed once as `jeeves-status --data-repo "..."`, which records it in
  `~/.claude/jeeves/data-repo`; every later command is bare (`jeeves-git` for git), never after a `cd` into
  the data repo or a `JEEVES_DATA=` prefix: an allow rule does not match past either, so each would prompt
  in the default permission mode (tested headless with `claude -p --permission-mode default --allowedTools`).
  Such a `claude -p` session runs the installed plugin's hooks with the real `data_repo` option: never give
  it a scratch `CLAUDE_HOME` or `JEEVES_HOST` (one test did, and its session-start snapshot committed the
  scratch `CLAUDE.md` into the real data repo); point only `PATH` at a worktree's `bin/`.
- Shared files (`home/`, synced) vs per-device files (`hosts/<host>/`, backup only; settings and keybindings
  only with `backup_settings=yes` in `jeeves.conf`): the lists are at the top of `lib/lib.sh`.
- `lib/lib.sh` — shared code: finding the data repo, tracked paths list, hashes, three-way classification.
  `lib/crypt.sh` — encryption: the key stores (Keychain, Windows Credential Manager through PowerShell,
  `secret-tool`, a key file), the passphrase dialogs, age helpers.
- `tests/run.sh` — tests; each runs against a scratch `~/.claude`, data repo and bare remote.
- `.claude/skills/release/` — the release procedure (a skill for working on the repo, not shipped as one).
- This file is `.claude/CLAUDE.md`, for working on the repo; Claude Code loads it here as project context.
  At the root it would be shipped with the plugin to no effect, and `claude plugin validate --strict` fails.

## Working on the plugin

- To develop, load the checkout over the installed copy: `"env": {"CLAUDE_CODE_PLUGIN_DIRS": "<checkout>"}`
  in that device's `~/.claude/settings.json` (or `claude --plugin-dir <checkout>` for one session), and list
  the checkout in `~/.claude/jeeves/dev-checkouts` so `jeeves-verify` leaves it alone. It loads in place:
  `/reload-plugins` picks up edits. Hook scripts are live as soon as a file is saved, so run `bash -n` on a
  changed script before saving it over the working copy, and keep `main` working. Loaded that way the plugin
  gets no plugin options: skills pass the literal `${user_config.data_repo}`, which the scripts ignore in
  favour of the recorded data repo (checked headless, 2026-11-19).
- To run a `bin/` script by hand against scratch folders, `export CLAUDE_HOME=<scratch>/.claude
  JEEVES_DATA=<scratch>/data` (and `JEEVES_HOST=<name>` to act as another host) first; never put `env ...`
  in a variable and run `$VAR cmd`, which zsh (the macOS shell) does not split.
- To check a change to skill text, have an agent follow the skill against scratch folders (those variables
  exported, the worktree's `bin/` on the PATH) with a made-up session and memories that hit each case; it
  finds unclear passages the tests can't. For text that only directs decisions, an agent that reads the
  skill and applies it to sample requests, running nothing, is quicker and finds the same kind of gaps.
  Some tests grep skill text, so rewording a skill can fail them.
- **Releases follow `.claude/skills/release/SKILL.md`** (`/release`): readiness, the review for bugs,
  interaction breakdowns and secret leaks, the checks, the release branch, CI, the merge, and the tag on the
  user's word.
- After changing a manifest or `hooks.json`: `claude plugin validate .` and
  `claude plugin validate .claude-plugin/marketplace.json`.
- Rehearse an upgrade in a scratch `CLAUDE_CONFIG_DIR`: `claude plugin marketplace add WinuxNomacs/jeeves`
  and `claude plugin install jeeves@jeeves --config data_repo=<scratch>` (the published version); after the
  new tag's pin is pushed, `claude plugin marketplace update jeeves` and `claude plugin update jeeves@jeeves`,
  then check `<scratch>/plugins/cache/jeeves/jeeves/<version>/` and run its `bin/jeeves-verify --trust` and
  `bin/jeeves-verify` with `CLAUDE_HOME=<scratch>` (Claude Code adds an `.in_use/` folder there; verify skips it). For scripts, also run the last tag's `bin/`
  (a `git worktree` of it) against a scratch data repo, then the new `bin/` on the state it wrote.

## Rules for the scripts

- Portable bash: macOS bash 3.2, Git Bash, Linux. Run the tests under Windows Git Bash too when a change
  could differ there (from WSL: copy the tree to a `C:` folder and run `bash.exe -lc tests/run.sh`). No
  GNU-only flags (no `sed -i`, no `\|` in sed), no `timeout`, no `date -d` or `date -r` (missing or different
  on macOS; format dates with git), no associative arrays, no `set -u`, no `jq`.
  No `mkdir` as a lock: uutils coreutils (Ubuntu 26.04's default) lets two processes create the same folder
  at once; `take_lock` in `lib/lib.sh` uses bash's `noclobber` instead.
- When a test's own git command writes files (switch, merge, checkout), give it `-c core.autocrlf=false`, as
  the scripts' `git_repo` does: Git for Windows would write CRLF and the scripts would see a dirty repo.
- Hooks must never block a session: hook mode always exits 0 and prints only what needs attention.
- Data integrity: check every copy (`copy_file`, `copy_settled`) and stop with nothing recorded when one
  fails; a sync record (`set_base`) holds the hash of what was copied and committed, never of the disk file
  read again later; nothing reaches `~/.claude` from a data repo that is not on a clean `main`.
- The scripts never push, with one exception: `jeeves-cleanup` force-pushes the data repo's `main`, always
  with a lease (`--force-with-lease=main:<the origin/main it checked>`), after `jeeves-scan`, and puts `main`
  back if the push fails. Otherwise they contact the remote only for `fetch` + fast-forward in `snapshot`,
  and only the `update` and `cleanup` skills push.
- Never copy secrets: `.credentials.json`, `~/.claude.json`, transcripts, history.
- Encryption: a passphrase never goes through the chat or a command line (a dialog, or `--terminal` in the
  user's own terminal; to age only as `AGE_PASSPHRASE` of that one process); the key goes to stores on
  stdin. Plain text must never reach a commit: the filter is required, a failure stops git, and
  `jeeves-scan` checks what a push sends. Tests for it need age 1.3+ on the PATH (skipped without it; the workflow installs a pinned age and fails
  if they are skipped); for
  Windows, unzip the release's Windows archive next to the copied tree and put it on the PATH.
- Tests: `tests/run.sh` (or `tests/run.sh <a word from the test's name>`; check how many ran). To see a new
  check fail on the old code, put back only the file under test (`git show HEAD:<file> > <file>`), never
  `git stash`, which stashes the tests too.

## Pushing and pulling (overrides the global "never push" rule, for this repo only)

The repo is public (`WinuxNomacs/jeeves`) and is the plugin's marketplace. Claude may pull from `origin` and
push branches to it whenever the work needs it, and push `main` after a merge whose full test suite passed,
without asking each time. A push to `main` reaches no install: the marketplace entry is pinned to the last
release's tag and commit, and moves (one commit on `main`, release step 9) only after the user's tag. A
release is a `release/X.Y.Z` branch (version, `CHANGELOG.md`, the signed `MANIFEST`), merged like any other. Still never force-push or rewrite pushed history.
A tag marks a release and starts the test workflow, so Claude tags and pushes a tag only when the user says
so for that release. Tags are annotated:
`git tag -a vX.Y.Z -m "Jeeves X.Y.Z"` (a plain `git tag` fails here with "no tag message").

The only workflow, `.github/workflows/test.yml`, runs on a pushed tag or by hand (Actions tab, or
`gh workflow run tests`), never on a push or pull request; Actions minutes are free on a public repo, but ask
before adding a trigger or a workflow. Never submit the plugin to Anthropic's directory unless the user
directly asks for it.
