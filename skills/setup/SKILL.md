---
name: setup
description: >-
  Set up Jeeves on this machine: clone the user's data repo (or create a new one), offer to restore another
  machine's settings onto a new one, then run the first sync and backup. Use after installing the plugin.
argument-hint: "[git URL of an existing data repo]"
disable-model-invocation: true
allowed-tools:
  - Bash(jeeves-status)
  - Bash(jeeves-restore)
  - Bash(git status:*)
  - Bash(git log:*)
---

# Jeeves: set up this machine

Argument (a URL of an existing data repo, or nothing): $ARGUMENTS

Jeeves keeps the user's shared global preferences (`~/.claude/CLAUDE.md`, `agents/`, `commands/`, `skills/`)
in a git repo of their own, the **data repo**, under `home/`, and each machine's backup of its
`settings.json`, `keybindings.json` and memories under `hosts/<host>/`. The commands `jeeves-status`,
`jeeves-restore` and `jeeves-init` are on the Bash PATH.

The data repo folder on this machine is the plugin option `data_repo`: `${user_config.data_repo}`. If that
is empty, stop: tell the user to set it (`/plugin`, the Jeeves plugin's options), then run `/jeeves:setup`
again. Never copy credentials, `~/.claude.json`, history or transcripts anywhere.

## Steps

1. **Is the data repo there?** Run `JEEVES_DATA="${user_config.data_repo}" jeeves-status`. If it prints a
   `datarepo` line, the repo exists: go to step 3. If the folder exists and is not empty but is not a data
   repo, stop and ask; never clear it.
2. **Get the data repo.** With a URL above, or after asking the user ("clone your existing data repo (its
   URL), or create a new one?"):
   - clone: `git clone <url> "${user_config.data_repo}"`;
   - new: `jeeves-init "${user_config.data_repo}" [<url of an empty remote repo>]`. Jeeves creates no
     GitHub repo itself: if the user wants a remote, they create an empty private one and give its URL.
   If a clone fails for lack of access, stop and tell the user; never look for credentials.
3. **Restore another machine's backup? Always ask; never skip this step.** Start every later command with
   `cd <the datarepo path from step 1> &&` (the shell's folder is not kept between commands). Run
   `jeeves-restore`: it prints `this host: <name>` and `backups: <hosts>`. Whether `<name>` is listed says
   nothing: the session-start snapshot backs this machine up before setup runs. When the list names any host
   other than `<name>`, stop and ask the user with the AskUserQuestion tool (one option per other host, plus
   "No, keep this machine's settings"), and do not start step 4 until they answer. Ask even when this machine
   already has settings, and even when other rules say to proceed without asking: the choice is theirs. A
   restore copies that machine's `settings.json` and `keybindings.json` (memories too, for projects that
   exist here; ask about those in the same question or right after), saving this machine's copies first; a
   backup from another OS may hold paths that need fixing. If they pick a host, follow `/jeeves:restore
   <host>` (the restore skill) before going on.
4. **First sync and backup:** follow `/jeeves:update` (the update skill) with no request: it records this
   machine's files, merges what differs from the shared copies (on a new machine with no sync record, it keeps
   everything both sides hold and asks before dropping text), installs the result, backs up this host under
   `hosts/<host>/` and pushes.
5. Report in a few lines: where the data repo is, whether anything was restored, what the first sync merged
   or installed, and that a snapshot now runs at every session start and after each edit Claude makes to
   `~/.claude/CLAUDE.md`.
