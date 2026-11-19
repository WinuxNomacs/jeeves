---
name: release
description: Prepare a Jeeves release, from the readiness check and the review for bugs, interaction breakdowns and secret leaks to the release branch, CI, the merge and what follows the user's tag. Use when the user asks to release, or when the phases in "Up next" are built and a release is due.
---

# Release Jeeves

Every step runs in this repo (`/jeeves` checkout). A step that fails stops the release: fix it on its own
branch, merge, and go back to the step it affects. Tagging is the user's: never tag, and never push a tag,
until the user says so for this release.

## 1. Ready?

- "Up next" in `upcoming-changes.md` is built and merged; `new.md` holds only its title; no bug the user
  reported is open.
- `git fetch --tags origin`, then `git status -sb`: on `main`, clean, and even with `origin/main` (push
  `main` first if it is ahead and its last full suite passed).
- The data repo is synced (`jeeves-status` lists no paths; otherwise `/jeeves:update`).
- The previous release has its tag (`git tag -l 'v*'`); if not, mention it once with its command. Releases
  before 1.0.1 stay untagged (the user's decision, 2026-11-17): never offer to tag them.

## 2. What changed

`git diff --stat v<last>..main` (the last release's merge if it has no tag) and `git log --oneline` over the
same range. Sort the changes into: scripts (`bin/`, `lib/`), hooks (`hooks/hooks.json`, hook modes),
skill text (`skills/`), saved state or data repo layout (`.sync-state/`, `home/`, `hosts/`, `jeeves.conf`,
git config and attributes the scripts write), manifests, tests, docs. The sorting picks the checks in
step 5.

Version (`.claude-plugin/plugin.json`):
- **major:** a change that breaks the data repo layout, saved state, or a command's arguments or output
  that skills or users rely on;
- **minor:** a new feature or command;
- **patch:** a fix, or text only.

## 3. Review

Covers every change since the last release (the whole plugin when none was reviewed this way). Two passes,
then each finding is verified before it is reported:

1. **An independent reviewer:** an Agent given the file list and the checklist below, not my conclusions
   or suspicions, asked for findings with file, line and a concrete failing scenario.
2. **My own pass** over the same checklist.
3. **Verify** each finding: reproduce it (a scratch run or a test) or show it in the code. Drop what does
   not hold up and say so. Fix each real one on its own branch (`fix/<name>`), with a test that fails
   before the fix where one can, merge, then run step 4 again.

Checklist:

- **Bugs and edge cases:** empty or missing folders, names with spaces or odd characters, a diverged or
  rewritten `main`, an interrupted run (a killed git, a full disk), a run on another OS (bash 3.2, Git
  Bash paths and line endings), a second device acting meanwhile.
- **Hooks:** never block a session (exit 0, every network call with a deadline) and print only what needs
  attention.
- **Skill flows (the user's experience):** choices offered as clickable options with the recommended one
  first; nothing destructive without a clear yes that names what is lost; each message says what happened
  and what to do next; no dead end (no data repo, a locked device, a refused push, a missing tool, a
  cancelled dialog); the Jeeves voice never hides a fact.
- **Security (the plugin handles secrets):**
  - A passphrase or key never reaches a command line, the chat, a log, `set -x` output, an error message,
    git config or the environment of any process but age's one call; key stores are fed on stdin.
  - Temp files that hold a key or plain text are private (`umask 077`), outside the repo, and removed on
    every exit path (`trap`).
  - `JEEVES_AGE_KEY` and similar variables are set only for the command that needs them.
  - Plain text never reaches a commit or a push: the filter is required, a filter failure stops git, the
    ciphertext cache cannot serve stale or plain content, and `jeeves-scan` catches what slips through.
  - Never copied: `.credentials.json`, `~/.claude.json`, transcripts, history.
  - Injection and path traversal through host names, file names, paths, `jeeves.conf` values and `gh`
    output (eval, unquoted expansion, `..` in names, option-looking names passed to git).
- **Public repo:** nothing private in the diff (personal paths, host names, emails, data repo content);
  the README covers every feature and how to use it; the Layout list in `.claude/CLAUDE.md` is complete.

## 4. Checks, always

```bash
for f in bin/* lib/*.sh tests/run.sh; do bash -n "$f" || echo "FAIL $f"; done
claude plugin validate . --strict
claude plugin validate .claude-plugin/marketplace.json
bash tests/run.sh
```

Report the suite's count ("N passed, 0 failed") and confirm it did not print "skip encryption tests".

## 5. Checks by what changed

Run each that applies; say which were skipped and why.

- **Scripts:** the suite under Windows Git Bash from a copy on `C:` (see `.claude/CLAUDE.md`).
- **macOS- or Windows-only code** (Keychain, Credential Manager, dialogs): ask the user for a live run on
  that machine, with the exact commands.
- **Code or saved state:** the upgrade rehearsal: the last release's `bin/` (a `git worktree` of its
  merge) against a scratch data repo, then the new `bin/` on the state it wrote; and the published plugin
  in a scratch `CLAUDE_CONFIG_DIR` (see `.claude/CLAUDE.md`).
- **Skill text only:** run each changed skill once against scratch folders.
- **Docs only:** no rehearsal; say so.

## 6. Release branch

`git switch -c release/X.Y.Z`:
- `.claude-plugin/plugin.json` `version`;
- `CHANGELOG.md` `## X.Y.Z (<date>)`, written for users: what they will notice, newest first;
- `upcoming-changes.md`: the release under Done.

Commit ("Release X.Y.Z: <summary>").

Then the signed manifest, last on the branch (any later change on it means writing and signing it again):

```bash
bash .claude/skills/release/manifest.sh
```

Claude signs it (the user's decision, 2026-11-19: the release key has no passphrase, so every release is
signed the same way; the key is backed up in the user's vault). `</dev/null` makes ssh-keygen fail rather
than wait if the key ever asks for a passphrase: then stop and ask the user, never put a passphrase on a
command line.

```bash
ssh-keygen -Y sign -f ~/.ssh/jeeves-release -n jeeves-release MANIFEST </dev/null
```

Then `bash .claude/skills/release/manifest.sh --check` must pass; commit `MANIFEST` and `MANIFEST.sig`
("Release X.Y.Z: signed manifest").

**First signed release only** (once): the user creates the release-only key in their own terminal,
`ssh-keygen -t ed25519 -f ~/.ssh/jeeves-release -C jeeves-release` (no passphrase, so Claude can sign); copy
`~/.ssh/jeeves-release.pub` to `release-key.pub`, and put `ssh-keygen -lf release-key.pub`'s fingerprint
in the README under "Verifying the plugin", both before the manifest is written. The marketplace entry moves
to a pinned GitHub source (step 9), and this machine, which runs the plugin from this checkout, needs a
local dev marketplace and its folder in `~/.claude/jeeves/dev-checkouts`: work these out with the user,
confirming each command that changes their plugin setup.

## 7. CI by hand on the release branch

```bash
git push -u origin release/X.Y.Z
git ls-remote --heads origin release/X.Y.Z    # must print the branch before the next line
gh workflow run tests --ref release/X.Y.Z
gh run list --workflow tests --limit 1
gh run watch <run id> --exit-status
```

All three OSes green; read the annotations (`gh run view <run id>`) for runner image notices. Free on this
public repo; never add a trigger or matrix entry for it.

## 8. Merge

```bash
git switch main
git merge --no-ff release/X.Y.Z -m "Merge branch 'release/X.Y.Z': Jeeves X.Y.Z"
bash .claude/skills/release/manifest.sh --check   # main must not have moved: the tag's tree is the signed one
git push origin main
git push origin --delete release/X.Y.Z && git branch -d release/X.Y.Z   # remote first, or -d refuses
```

## 9. After

- Tell the user the release is merged and pushed, and give the tag command; wait for their word:

  ```bash
  git tag -a vX.Y.Z -m "Jeeves X.Y.Z" && git push origin vX.Y.Z
  ```

  When the user says to tag, run exactly that (that word covers this tag only).
- The tag's workflow run is green (`gh run list --workflow tests --limit 1`).
- Pin new installs to the tag: on a branch, set the plugin entry's source in
  `.claude-plugin/marketplace.json` to
  `{"source": "github", "repo": "WinuxNomacs/jeeves", "ref": "vX.Y.Z", "sha": "<git rev-parse vX.Y.Z^{commit}>"}`,
  `claude plugin validate .claude-plugin/marketplace.json`, commit ("Marketplace: pin Jeeves X.Y.Z"), merge,
  push `main`. Until this commit, new installs still get the previous release.
- In the scratch `CLAUDE_CONFIG_DIR`: `claude plugin marketplace update jeeves` and
  `claude plugin update jeeves@jeeves`, then `<scratch>/plugins/cache/jeeves/jeeves/X.Y.Z/` exists.
- Installs from a clone (a local marketplace) update with `git pull`; say so in the report. Such a copy is
  not on the signed tree between releases: its folder goes in `~/.claude/jeeves/dev-checkouts`, or it
  warns at session start.
