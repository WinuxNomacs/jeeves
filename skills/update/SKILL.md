---
name: update
description: >-
  Sync or change the user's global Claude Code preferences shared by all their devices (~/.claude/CLAUDE.md,
  agents/, commands/, skills/) through the Jeeves data repo, then push it. Use it when the user asks to
  change a global rule, agent, command or personal skill (pass the request as the argument), when a "jeeves
  snapshot" message reports newer repo files, a conflict or a diverged main, or when the user asks to sync
  their preferences. Without an argument it syncs; with one it makes that change. Not for settings.json or
  keybindings.json: those are per device, only backed up.
argument-hint: "[change to make, or nothing to sync]"
allowed-tools:
  - Bash(jeeves-status)
  - Bash(jeeves-snapshot)
  - Bash(jeeves-snapshot --no-pull)
  - Bash(jeeves-apply)
  - Bash(jeeves-apply --force)
  - Bash(jeeves-scan)
  - Bash(git status:*)
  - Bash(git log:*)
  - Bash(git diff:*)
  - Bash(git show:*)
  - Bash(git cat-file:*)
  - Bash(git push origin main)
---

# Jeeves: update the global preferences

Request: $ARGUMENTS

No request above means **sync mode**; a request means **change mode** (a sync first, then the change).

The live files are in `~/.claude`. The data repo keeps the shared ones under `home/` (`CLAUDE.md`, `agents/`,
`commands/`, `skills/` but not `skills/synced/`) and each device's backup of its `settings.json`,
`keybindings.json` and memories under `hosts/<host>/`, with history in git. The commands `jeeves-status`,
`jeeves-snapshot`, `jeeves-apply` and `jeeves-scan` are on the Bash PATH.

## Rules

- **Never lose text.** Ask the user (show both versions of the passage) before anything that would drop or
  rewrite text that one side added: a true conflict, where both sides changed the same passage in different
  ways. Additions on both sides are not a conflict: keep both.
- **Push without asking** when the steps below end cleanly. Do not show diffs unless the user asks for them;
  everything is in git.
- **Never force-push**, never rewrite pushed commits, never tag. A rejected push is never forced: fetch, merge,
  and if it is still rejected, stop and ask.
- **A failed `jeeves-scan` stops everything:** tell the user what it found (file and line, never the value)
  and ask how to proceed. Never search files, transcripts or the environment for credentials.
- Edit `home/…` in the data repo, never `~/.claude` directly; `jeeves-apply` installs the result. Never edit
  `hosts/` (backups, written by snapshots) or `.sync-state/` (written by the scripts), and never copy
  credentials, `~/.claude.json`, history or transcripts into the repo.

## Find the data repo

Run `JEEVES_DATA="${user_config.data_repo}" jeeves-status`. If it says no data repo is set, stop and tell the
user to set the plugin option `data_repo` (`/plugin` or `/config`). Otherwise start every later command
with `cd <the path on its datarepo line> &&` (the shell may not keep its folder between commands), so the
scripts and git act on the data repo.

`jeeves-status` prints tab-separated lines:
- `<state> <path> <disk> <repo> <base>` for each tracked path not in sync. `disk`: only `~/.claude` changed
  since this device last synced; `repo`: only the repo copy changed; `conflict`: both. The hashes are git
  blobs; `-` means no file, `none` means never synced on this device. Read the base with
  `git cat-file -p <base>`.
- `branch <name> clean|dirty|merging` and `main even|ahead|behind|diverged|no-remote <ahead> <behind>`
  (compared with `origin/main` as last fetched).

If the branch is not `main`, or is dirty or merging: look at what is there (`git status`, `git log`). Finish a
change-mode branch of your own from this session; for anything else, stop and ask.

## Sync mode

1. `jeeves-snapshot`: fetches and fast-forwards `main`, records what changed on disk, commits. Then
   `jeeves-status`. If the snapshot says a file was "not taken" (a name another OS can't hold), tell the
   user which file and why; renaming it is theirs to decide.
2. **Diverged `main`:** `git merge --no-ff origin/main -m "Merge origin/main: <what came in>"`. Merge
   conflicting text under `home/` by the rules above (keep both sides' additions; ask before dropping text);
   a conflict under `hosts/` takes this device's side for `hosts/<this host>/` and the incoming side for
   any other host (each host's folder is written only by that host). `git add` the result and commit.
3. **Each `conflict` line:** read base, disk (`~/.claude/<path>`) and repo (`home/<path>`). With a base, take
   each side's changes since the base; without one (`none`, a new device), keep everything both sides hold.
   If the changes do not overlap, write the merged text to `home/<path>`, commit
   (`merge(<host>): <path> from disk and repo`), then `jeeves-apply --force` (it saves the disk copy under
   `.sync-state/overwritten/`). If they overlap, ask first.
4. `jeeves-apply` installs what is newer in the repo. Then `jeeves-snapshot` again: it must print nothing,
   and `jeeves-status` must list no paths. If not, go back to the step that applies.
5. If `main` is ahead: `jeeves-scan`, then `git push origin main`. If the push is rejected, `jeeves-snapshot`
   and repeat from step 2 once; if it is rejected again, stop and ask.
6. Report in one or two lines: what was merged, installed and pushed (or that everything was in sync).

## Change mode

1. Sync first (sync mode, steps 1–4), so the change starts from files that match on disk and in the repo.
2. `git switch -c pref/<short-name>`, then make the requested change in `home/<path>`. For `CLAUDE.md`, follow
   its own rules on wording and where a rule goes: edit a rule in place rather than adding one that
   contradicts it. A file goes in `home/` only if it is one of the shared paths above; a change to
   `settings.json` or `keybindings.json` is per device: make it in `~/.claude` (the next snapshot backs it
   up), not here. Shared files must work on Linux, macOS and Windows: no absolute paths, a skill runs its
   scripts as `bash <script>` (Windows keeps no executable bit), and file names without `: * ? " < > | \`.
3. Commit with a message saying what changed and why. `git switch main`, then
   `git merge --no-ff pref/<short-name> -m "Merge branch 'pref/<short-name>': <summary>"`, and
   `git branch -d pref/<short-name>`.
4. `jeeves-apply` (installs it into `~/.claude`), then `jeeves-snapshot` (must print nothing).
5. `jeeves-scan`, `git push origin main`, as in sync mode step 5.
6. Report the change in one line and that it is pushed. Show the diff only if the user asks.
