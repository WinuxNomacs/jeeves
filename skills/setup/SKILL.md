---
name: setup
description: >-
  Set up Jeeves on this machine, step by step: check git and GitHub access, find the user's data repo on
  GitHub or create a private one, offer to restore another machine's settings, then run the first sync and
  backup. Use after installing the plugin.
argument-hint: "[git URL of an existing data repo]"
disable-model-invocation: true
allowed-tools:
  - Bash(jeeves-check)
  - Bash(jeeves-github find)
  - Bash(jeeves-status)
  - Bash(jeeves-restore)
  - Bash(git status:*)
  - Bash(git log:*)
---

# Jeeves: set up this machine

Argument (a URL of an existing data repo, or nothing): $ARGUMENTS

Jeeves keeps the user's shared global preferences (`~/.claude/CLAUDE.md`, `agents/`, `commands/`, `skills/`)
in a private git repo of their own, the **data repo**, under `home/`, and each machine's backup of its
`settings.json`, `keybindings.json` and memories under `hosts/<host>/`. The commands `jeeves-check`,
`jeeves-github`, `jeeves-status`, `jeeves-restore` and `jeeves-init` are on the Bash PATH.

## How to guide

The user may know nothing about git. Walk them through five steps, and start each with a heading line such
as **Step 2 of 5: GitHub access**, then one or two plain sentences: what this step checks, what was found,
and what happens next. Every choice is an AskUserQuestion with the recommended option first, so the user
picks rather than types; ask for typing only for a URL or a name. Never search for credentials, and never
ask for a password or token: sign-ins are the user's own, in their terminal. Never copy credentials,
`~/.claude.json`, history or transcripts anywhere. Anything that installs software or changes global
settings is shown as the exact command and run only after the user agrees.

Start by running `JEEVES_DATA="${user_config.data_repo}" jeeves-check` (an empty option is fine: it then
uses `~/jeeves-data`). Its lines (tab-separated) are the facts for the steps below; run it again after
anything changes. The `datarepo` line gives the data repo folder with `~/` expanded: use that path, quoted,
in every command, never `~`.

## Step 1 of 5: Tools

- `git missing`: say that Jeeves needs git, and how to get it (macOS: `xcode-select --install`; Windows: Git
  for Windows from git-scm.com; Linux: the system's package manager). Stop until it is installed.
- `os windows`: Claude Code runs Jeeves in Git Bash, which comes with Git for Windows; nothing more needed.
- Otherwise say in one line that the tools are there, and go on.

## Step 2 of 5: GitHub access

Jeeves keeps the data repo on GitHub, so every device can reach it. Skip this step when the data repo is
already there (`datarepo ... repo`).

- `gh-auth ok <login>`: say "signed in to GitHub as <login>" and go on.
- `gh missing`: explain that the GitHub CLI lets setup find or create the data repo without any URL. Ask:
  "Install the GitHub CLI?" with the `gh-install` command as the recommended option (say if it asks for the
  computer's password: `sudo`), "I'll install it myself" (link: https://cli.github.com), and "Continue without
  it" (then a URL is needed in step 3). `gh-install none`: offer only the last two. After an install, run
  `jeeves-check` again.
- `gh ok` but `gh-auth fail`: the user signs in themselves: tell them to type `! gh auth login` here (the `!`
  runs it in this session) and follow its prompts, choosing GitHub.com and their browser. Then run
  `jeeves-check` again. If it still fails, say what it reported and offer to continue without `gh`.
- Without `gh`, `ssh-github ok` means git can still reach GitHub over SSH; mention it.

## Step 3 of 5: Your data repo

The folder on this machine is the `datarepo` path.

- `repo`: it is already there; say so and go to step 4.
- `other`: the folder holds other files. Stop and ask; never clear it.
- `missing` or `empty`: find or create the repo.
  - A URL given as the argument: clone it (below).
  - With `gh-auth ok`: run `jeeves-github find`. Each line is `<owner/name>\t<clone url>` of a data repo
    already on GitHub (another device made it). Ask which to use, with "Create a new one" as the last option;
    when the list is empty, ask "Create your private data repo on GitHub (jeeves-data)?" with options
    "Yes, create it", "Use another name" and "I already have one (I'll paste its URL)".
  - Without `gh`: ask for the URL of an existing data repo, or explain how to make one: on github.com, New
    repository, name `jeeves-data`, Private, no README, then copy its URL (the green Code button).
- **Clone:** `git clone <url> "<datarepo path>"`. If it fails for lack of access and `gh-protocol https`
  with `gh-auth ok`, git is probably not using gh's sign-in: offer to run `gh auth setup-git` (it changes the
  user's global git settings, so ask first), then clone again. Any other failure: say what git reported and
  stop.
- **Create:** `jeeves-github create <name>` (it prints `<owner/name>\t<clone url>`; a taken name: ask for
  another), then `jeeves-init "<datarepo path>" "<clone url>"`. Nothing is pushed until step 5. Without `gh`,
  `jeeves-init "<datarepo path>" "<url>"` with the URL of the empty repo the user made.

## Step 4 of 5: Settings from another machine

**Always ask; never skip this step.** Start every later command with `cd "<datarepo path>" &&` (the shell's
folder is not kept between commands). Run `jeeves-restore`: it prints `this host: <name>` and
`backups: <hosts>`. Whether `<name>` is listed says nothing: the session-start snapshot backs this machine up
before setup runs. When the list names any host other than `<name>`, stop and ask the user with the
AskUserQuestion tool (one option per other host, plus "No, keep this machine's settings"), and do not start
step 5 until they answer. Ask even when this machine already has settings, and even when other rules say to
proceed without asking: the choice is theirs. A restore copies that machine's `settings.json` and
`keybindings.json` (memories too, for projects that exist here; ask about those in the same question or right
after), saving this machine's copies first; a backup from another OS may hold paths that need fixing. If they
pick a host, follow `/jeeves:restore <host>` (the restore skill) before going on. With no other host (a new
data repo), say there is nothing to restore yet and go on.

## Step 5 of 5: First sync

Follow `/jeeves:update` (the update skill) with no request, and without its session review: it records this
machine's files, merges what differs from the shared copies (on a new machine with no sync record, it keeps
everything both sides hold and asks before dropping text), installs the result, backs up this host under
`hosts/<host>/` and pushes.

## Done

Report in a few lines: where the data repo is (folder and GitHub repo), whether anything was restored, what
the first sync merged or installed, and what happens from now on: a snapshot at every session start and after
each edit Claude makes to `~/.claude/CLAUDE.md`; `/jeeves:update` to sync by hand or change a preference;
on another device, install Jeeves and run `/jeeves:setup`, which finds this data repo by itself when that
device is signed in to GitHub with `gh`.
