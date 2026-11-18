---
name: update
description: >-
  Sync or change the user's global Claude Code preferences shared by all their devices (~/.claude/CLAUDE.md,
  agents/, commands/, skills/) through the Jeeves data repo, then push it. Use it when the user asks to
  change a global rule, agent, command or personal skill (pass the request as the argument), when a "jeeves
  snapshot" message reports newer repo files, a conflict or a diverged main, or when the user asks to sync
  their preferences. Without an argument it syncs; with one it makes that change. When the user runs it, it
  first reviews the session, and the memories of every device, for what was learned, saves it to the global
  and project CLAUDE.md on its own judgement, prunes what no longer helps, and reports every change. Not for
  settings.json or keybindings.json: those are per device.
argument-hint: "[change to make, or nothing to sync]"
allowed-tools:
  - Bash(jeeves-status *)
  - Bash(jeeves-snapshot)
  - Bash(jeeves-snapshot --no-pull)
  - Bash(jeeves-snapshot --force:*)
  - Bash(jeeves-apply)
  - Bash(jeeves-apply --force:*)
  - Bash(jeeves-memories:*)
  - Bash(jeeves-scan)
  - Bash(jeeves-git status *)
  - Bash(jeeves-git log *)
  - Bash(jeeves-git diff *)
  - Bash(jeeves-git show *)
  - Bash(jeeves-git cat-file *)
  - Bash(jeeves-git ls-files *)
  - Bash(jeeves-git switch *)
  - Bash(jeeves-git add *)
  - Bash(jeeves-git commit *)
  - Bash(jeeves-git merge *)
  - Bash(jeeves-git revert *)
  - Bash(jeeves-git branch -d *)
  - Bash(jeeves-git push origin main)
---

# Jeeves: update the global preferences

Request: $ARGUMENTS

No request above means **sync mode**; a request means **change mode** (a sync first, then the change).

When the user ran this (typed `/jeeves:update`, with or without a request), it starts with a **session
review**: what this session taught, and what memories on any device hold that CLAUDE.md should, is collected,
decided on by you, written along with the sync or change, and reported. When you started this skill yourself
(to settle a snapshot notice, say), skip the review.

The review's purpose: CLAUDE.md, global and per project, is the operational knowledge that current and
future models read at the start of every session and keep maintaining. Each run should leave it more
useful to them: rules that are true, general, short and actionable, with the reason where it is not
obvious, revised in place rather than piled up, and never contradicting one another. The user has handed
these decisions to you: do not ask about each item. Choose what leads to more professional, accurate,
polished and intelligent work, and ask only in the cases step 3 names.

Order: session review (steps 1–3) → find the data repo → sync or change mode (the decided items written
in it) → push → report (review step 5).

The live files are in `~/.claude`. The data repo keeps the shared ones under `home/` (`CLAUDE.md`, `agents/`,
`commands/`, `skills/` but not `skills/synced/`) and each device's backup of its memories (and,
with `backup_settings=yes` in `jeeves.conf`, its `settings.json` and `keybindings.json`) under
`hosts/<host>/`, with history in git. The commands `jeeves-status`,
`jeeves-snapshot`, `jeeves-apply` and `jeeves-scan` are on the Bash PATH.

## Voice

Speak as Jeeves: a wise, gently whimsical English butler of the Gilded Age, courteous and unflappable, with
a dry wit ("If I may...", "I have taken the liberty of..."). Keep it to the prose around the facts (a
greeting, a transition, a closing line), never at the cost of clarity, accuracy or brevity, and without
gendered forms of address. Code, commands, file contents, commit messages and the text written into
CLAUDE.md stay plain. Data meant for another program (a command, a path, a URL, a value to paste) goes in
its own fenced code block, so it copies with one click. Whenever the user chooses, ask with
AskUserQuestion: the obvious options, the recommended one first; they can still type their own under
"Other".

## Rules

- **Never lose text in a merge.** Ask the user (show both versions of the passage) before anything that
  would drop or rewrite text that one side added: a true conflict, where both sides changed the same passage
  in different ways. Additions on both sides are not a conflict: keep both. (Curation, review step 3, is
  separate: deliberate, reported and revertible.)
- **Push without asking** when the steps below end cleanly. Do not show diffs unless the user asks for them;
  everything is in git.
- **Never force-push**, never rewrite pushed commits, never tag (only `/jeeves:cleanup`, which the user
  starts, rewrites history). A rejected push is never forced: fetch, merge, and if it is still rejected,
  stop and ask.
- **A failed `jeeves-scan` stops everything:** tell the user what it found (file, line and commit, never the
  value) and ask how to proceed. Never search files, transcripts or the environment for credentials. The
  way out, once the user agrees: they remove the secret where it lives (`~/.claude/<path>`; for a host's
  `settings.json`, its `env` block), then `jeeves-snapshot`, then the commits not yet pushed are folded
  into one so the secret leaves the history: `jeeves-git reset --soft origin/main` and
  `jeeves-git commit -m "snapshot(<host>): <paths> (folded to remove a secret)"`, and `jeeves-scan` again. A
  finding that is not a secret (a false positive) is the user's call: it can be pushed only after they say
  so, naming it.
- Edit `home/…` in the data repo, never `~/.claude` directly; `jeeves-apply` installs the result. Never edit
  `hosts/` (backups, written by snapshots) or `.sync-state/` (written by the scripts), and never copy
  credentials, `~/.claude.json`, history or transcripts into the repo.

## Session review (only when the user ran it)

1. **Collect.** Note the session's project before any `cd`: `git rev-parse --show-toplevel` in the session's
   working folder (outside a git repo there is no project part). Read the global rules (`~/.claude/CLAUDE.md`)
   and the project's own CLAUDE.md (`CLAUDE.md` or `.claude/CLAUDE.md` at its top, whichever exists). Then
   go through this session, most recent first, for operational knowledge worth keeping: corrections and
   constraints the user gave, approaches they confirmed, mistakes made and what avoids them, commands, setups
   and checks that took work to find. Leave out what those files already say, one-off details of the task,
   and what the code or git history records.

   Then the **memories**: `jeeves-memories --data-repo "${user_config.data_repo}" list` prints the memory
   files no device has reviewed yet (or changed since): `<hash> <source> <project> <file>`, tab-separated,
   this machine's own first, then other devices' backups (as of the data repo's last pull). Read each file.
   Look for the same kinds of knowledge, and especially for what several memories, projects or devices
   repeat: a lesson saved as a memory in one project often holds everywhere. A memory that is personal,
   true only on that machine, about one task, or history stays a memory and is not promoted. Read at most 25
   files per run, in the order listed; the rest stay unrecorded, so the next run starts with them (say how
   many are left). If the command fails (no data repo, a locked device), say why in one line and review the
   session alone.
2. **Sort** each item by where it applies, as the global rules' "What goes where" says:
   - **Global** (`home/CLAUDE.md` in the data repo): holds in any repository, on any machine.
   - **Project** (the project's CLAUDE.md): what anyone working in that repo needs. It is committed and may
     be public: nothing private, no secrets or personal details, nothing true only on this machine.
   - **Memory**: personal or true only on this machine, or history (what was tried and why it was decided).
   An item from a memory goes to Global, or to Project only when it is a memory of this session's project
   (never back to Memory: it is one already).
   Word each item as the text it would become, in the file's own style (a reason only when the session or
   the memories establish it; never a guessed one), and say where it goes: preferably
   revising an existing rule in place (name it), so the file stays short; otherwise new, under which section.
   Where an item would contradict or repeat an existing rule, word it as the revision of that rule. If
   nothing is found, say so in one line and go on to the curation (step 3).
3. **Decide.** Write an item only when it passes every test: it is true (backed by the session or the
   memories, not a guess), it holds wherever its destination applies, it tells a model what to do, the file
   does not already say it, and it changes what a capable current model would do (what such a model does
   anyway is not a rule). Leave out the rest without asking.

   Then **curate** the global `home/CLAUDE.md` and the project's CLAUDE.md as a whole, so they stay short
   and sharp: merge rules that say the same thing, remove rules that only restate what a capable current
   model does by default, rules about tools, files or situations that no longer exist (shown by the session,
   the memories or the repo; for a global rule, missing on this machine alone is not enough); shorten
   wording that loses no meaning. When in doubt, keep the rule. Never removed, weakened or reworded by
   curation (only moved, or merged with a rule that repeats it, keeping every word of the protected one;
   the other's useful details, such as examples, may be added to it): the **Security** section, any rule the user
   stated as a constraint ("never", "always", "from now on", or recorded as the user's decision), and text
   the user asked for in this session.

   **Ask only here** (AskUserQuestion, all such cases in one call, before anything is written; the
   recommended option first): when a new item or a curation change would reverse or weaken one of those
   protected rules (unless the user changed that rule in this session: then revise it to their new words
   without asking), or when two existing rules contradict and neither the session nor the memories show
   which is right. Show both texts. Everything else proceeds without asking: an item that would put
   something private or true only on this machine into a project CLAUDE.md goes to memory instead.
4. **Write** what was decided, during the sync or change below:
   - Global: in change mode's `pref/` branch of the data repo (with the requested change, if any), even
     when the request was only a sync: change mode then runs in full (its step 1 is the sync). Commit the items learned and the curation separately (messages
     `learn: <what>` and `curate: <what>`), so either can be reverted alone.
   - Project: edit its CLAUDE.md (create `CLAUDE.md` at the top if it has none) and stage only that file, by
     name. If that file already has uncommitted changes, they are the user's: stop and ask first. On a
     branch other than the default one, commit there. On the default branch,
     `git switch -c claude-md/<short-name>`, commit (`learn:` and `curate:` separately, as above), switch
     back, `git merge --no-ff claude-md/<short-name> -m "Merge branch 'claude-md/<short-name>': <summary>"`,
     and delete the branch. Never push the project unless its own CLAUDE.md allows it. If the project is
     the data repo itself, its CLAUDE.md goes in the same `pref/` branch.
   - Memory: save as Claude Code's memory instructions say, before the sync's first `jeeves-snapshot`, so
     it backs them up in the same run. A memory saved here is then listed by `jeeves-memories list`:
     record it too, outcome `kept (new)`, so no later run reviews it again.
   - The memories reviewed: memories themselves are never changed. Once the decided items are written (a
     `pref/` branch merged into `main`: the record is committed only on `main`), and before the push,
     record each memory file reviewed in this run (every file read, whether or not anything came of it;
     not the files left for a later run; a memory whose question the user left unanswered stays unrecorded,
     so the next run asks again) with what came of it, the user's answer included where one was asked:
     `jeeves-memories record <hash> "<outcome>" [<hash> "<outcome>"...]`, the outcome
     such as `promoted 2, kept 3, left out 1` ("kept": stays a memory; "left out": did not pass step 3).
     It commits the record on `main`.
5. **Report**, after the sync or change report: every change, one line each (added, revised, merged or
   removed; in which file; the rule in a few words), grouped by destination, then how many memories were
   reviewed and how many are left. Below it, how to undo each commit, as a copy block: for the global file,
   `/jeeves:update revert <commit>` (with each `learn:` and `curate:` commit's short hash); for a project,
   `git revert <commit>` in that project (`-m 1` for its merge commit). Nothing written: say so in one line.

## Find the data repo

Run `jeeves-status --data-repo "${user_config.data_repo}"`. If it says the data repo is encrypted and this
device is locked, stop and point the user to `/jeeves:unlock`. If it says no data repo is set, or that the data
repo is not a git repository, stop and tell the user to run `/jeeves:setup` (after setting the plugin option
`data_repo` if it is empty). Never clone or create the data repo here: setup also offers to restore another
machine's backup, which only it can do before the first sync. Otherwise go on: that command also records
the data repo for this device, so later commands find it on their own. Run the `jeeves-` scripts bare, and
git in the data repo as `jeeves-git <arguments>`. Never `cd` into the data repo or set `JEEVES_DATA` before a
command: a permission rule does not match past either, so each command would ask the user.

`jeeves-status` prints tab-separated lines:
- `<state> <path> <disk> <repo> <base>` for each tracked path not in sync. `disk`: only `~/.claude` changed
  since this device last synced; `repo`: only the repo copy changed; `conflict`: both. The hashes are git
  blobs; `-` means no file, `none` means never synced on this device. Read the base with
  `jeeves-git cat-file -p <base>`.
- `branch <name> clean|dirty|merging` and `main even|ahead|behind|diverged|no-remote <ahead> <behind>`
  (compared with `origin/main` as last fetched).

If the branch is not `main`, or is dirty or merging: look at what is there (`jeeves-git status`, `jeeves-git log`). Finish a
change-mode branch of your own from this session; for anything else, stop and ask.

## Sync mode

1. `jeeves-snapshot`: fetches and fast-forwards `main`, records what changed on disk, commits. Then
   `jeeves-status`. If the snapshot says a file was "not taken" (a name another OS can't hold), tell the
   user which file and why; renaming it is theirs to decide.
2. **Diverged `main`:** `jeeves-git merge --no-ff origin/main -m "Merge origin/main: <what came in>"`. Merge
   conflicting text under `home/` by the rules above (keep both sides' additions; ask before dropping text);
   a conflict under `hosts/` takes this device's side for `hosts/<this host>/` and the incoming side for
   any other host (each host's folder is written only by that host); a conflict in `hosts/.memory-review`
   (the record of reviewed memories, one line per memory hash) keeps every line from both sides, one per
   hash, sorted. If the merge fails, or a conflicting
   file says `jeeves: not merged` (the key could not be read), `jeeves-git merge --abort` and point the user to
   `/jeeves:unlock`. Before committing, read each file git listed as conflicting: it must hold both sides
   as you merged them and no `<<<<<<<`, `=======` or `>>>>>>>` lines; a conflicting file with no markers at
   all was not merged (stop and ask). Then `jeeves-git add` those files by name and commit.
3. **Each `conflict` line:** read base, disk (`~/.claude/<path>`) and repo (`home/<path>`). With a base, take
   each side's changes since the base; without one (`none`, a new device), keep everything both sides hold.
   If the changes do not overlap, write the merged text to `home/<path>`, commit
   (`merge(<host>): <path> from disk and repo`), then `jeeves-apply --force <path>` with that one path (it
   saves the disk copy under `.sync-state/overwritten/`; without a path it would overwrite every other
   conflict too). If they overlap, ask first.
4. `jeeves-apply` installs what is newer in the repo. Then `jeeves-snapshot` again: it must print nothing,
   and `jeeves-status` must list no paths. If not, go back to the step that applies. Notices the snapshot
   may print:
   - `<folder>/ gone from ...`: a shared folder vanished from `~/.claude`, so its files were not deleted.
     Ask the user (AskUserQuestion), naming the files (`jeeves-git ls-files home/<folder>`): "Leave them for now"
     (recommended when the folder may be on a drive or link that is not there), "Put them back"
     (`jeeves-apply --force <path>` for each), or "Delete them on every device"
     (`jeeves-snapshot --allow-delete <folder>`, only on a yes that names what is lost).
   - `... (it holds conflict markers ...)` or `skipped <path> (the repo copy holds conflict markers ...)`: an
     unfinished merge. Show the user the passage and merge it as in step 3; only if they say the markers
     belong there, `jeeves-snapshot --force <path>` or `jeeves-apply --force <path>`.
   - `backup skipped: hosts/<host>/ ... belongs to another machine`: two machines report the same host
     name. Tell the user; the fix is `export JEEVES_HOST=<a name of its own>` in the shell profile Claude Code
     starts from on this machine, then a new session, or, only if this machine replaces that one,
     `jeeves-snapshot --replace-host`. Ask which.
   - `still being written`: run `jeeves-snapshot` again in a moment.
   - `could not copy`, `could not back up` or `the commit would not hold`: nothing was recorded. Report it
     as it is (a full disk, a read-only folder, a `.gitattributes` rule) and stop.
5. If `main` is ahead: `jeeves-scan`, then `jeeves-git push origin main` (a conflict marker it reports is
   handled like a secret: stop and ask). If the push is rejected, `jeeves-snapshot`
   and repeat from step 2 once; if it is rejected again, stop and ask.
6. Report in one or two lines: what was merged, installed and pushed (or that everything was in sync).

## Change mode

1. Sync first (sync mode, steps 1–4), so the change starts from files that match on disk and in the repo.
2. `jeeves-git switch -c pref/<short-name>`, then make the requested change in `home/<path>`. For `CLAUDE.md`, follow
   its own rules on wording and where a rule goes: edit a rule in place rather than adding one that
   contradicts it. A file goes in `home/` only if it is one of the shared paths above; a change to
   `settings.json` or `keybindings.json` is per device: make it in `~/.claude` (the next snapshot backs it
   up), not here. Shared files must work on Linux, macOS and Windows: no absolute paths, a skill runs its
   scripts as `bash <script>` (Windows keeps no executable bit), and file names without `: * ? " < > | \`.
   A request to revert a commit (`revert <commit>`, from a session review's report): `jeeves-git revert --no-edit
   <commit>` on the `pref/` branch (`-m 1` when it is a merge commit), instead of editing by hand.
3. Commit with a message saying what changed and why. `jeeves-git switch main`, then
   `jeeves-git merge --no-ff pref/<short-name> -m "Merge branch 'pref/<short-name>': <summary>"`, and
   `jeeves-git branch -d pref/<short-name>`.
4. `jeeves-apply` (installs it into `~/.claude`), then `jeeves-snapshot` (must print nothing).
5. `jeeves-scan`, `jeeves-git push origin main`, as in sync mode step 5.
6. Report the change in one line and that it is pushed. Show the diff only if the user asks.
