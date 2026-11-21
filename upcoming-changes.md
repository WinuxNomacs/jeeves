# Upcoming changes

## Phases (current and upcoming)

- **No release before 2.0** (the user's decision, 2026-11-21): 2.0 is the completed Go rewrite. Work merges
  into `main` as each phase is done; nothing is cut or tagged until then, except the 1.4.2 security fix.
- **Up next:** 1. Go port, Phase 4: background updates in Go (`feat/go-4`)
- **Then:** 2. Phase 5: the last bash versions removed · 3. Release 2.0
- **The user's, any time:** submit 1.4.2 in the Anthropic portal (below, "Anthropic directory submission")
- **Later:** 0. Review text gaps (below, under "Review text gaps") · 0. Injection docs: threat model in the README, plugin repo protection · 1. Key rotation (a new key after a leaked passphrase) · 2. Follow symlinked folders under
  `~/.claude` · 3. Recover a cleanup killed after moving `main` · 4. Per-device overlay (only if a shared file
  must differ per host)

## Bug: /jeeves hand-off to review (found 2026-11-19)

`/jeeves` alone offers "Review my rules", which invokes `jeeves:review` with the Skill tool; Claude Code
refuses, since `review` has `disable-model-invocation: true` ("Ask the user to run /jeeves:review
themselves"). Shipped in 1.3.0 and 1.4.0. Fix to plan: the option tells the user to type `/jeeves:review`
(or drop it from the menu); check every other hand-off between skills for the same refusal (`jeeves` to
`update`, `review` and `setup` to `update`, setup to `restore` and `encryption`).

Decided with the user (2026-11-19): the menu option tells the user to type `/jeeves:review`; setup's
hand-offs to restore and encryption (both user-only, so likely refused too) go through shared step files
beside those skills (`skills/<name>/steps.md`), read by the skill and by setup. Hand-offs to `update` are
fine (it is not user-only).

Also seen (development loads only): with the plugin loaded inline, `${user_config.data_repo}` is not
filled in, and inside double quotes bash rejects it before the script runs ("Parse error"); the model
retried with single quotes. Installed copies get the real path. Not fixed.

## Go port (decided and planned with the user, 2026-11-20)

The spike (below) met every go/no-go criterion; the user chose "go". Decided with the user (2026-11-20):
- **Builds are committed:** `dist/` holds the five builds in each release, covered by the signed `MANIFEST`,
  so `jeeves-verify` checks them as it checks every file. About 10 MB more history per release that changes
  Go code. Not checked: whether Claude Code's install clones only the release (shallow).
- **Bash goes, and 2.0 is Go only** (the user's decisions, 2026-11-21, replacing "one release after"): no
  release comes before 2.0, the completed rewrite; each command's bash version is deleted on `main` once its
  Go version passes the parity tests on all three OSes. 2.0 holds the builds, the wrappers and
  `jeeves-verify`, and runs only where a build fits (macOS, Linux and Windows on amd64 or arm64; Windows
  arm64 has none yet). The launcher's fallback stays until the last bash version is gone.
- **Background updates (built as 1.5.0, never published) ship in 2.0.**
- **Knowledge files only** (the user's decision, 2026-11-20): Jeeves syncs CLAUDE.md, `skills/`, `agents/`
  and `commands/`, and backs up each device's memories. `settings.json` and `keybindings.json` are no
  longer backed up or restored; `backup_settings` goes. Copies already in data repos
  (`hosts/<host>/settings.json`, `keybindings.json`) stop being updated and stay until the user deletes
  them or runs `/jeeves:cleanup`: nothing is removed silently. Built in Phase 2 (README, skills and
  disclosure updated with it).
- **Design:** `jeeves-verify` stays a short bash script, never ported: it checks that the builds are the
  signed ones, so it cannot be one of them, and users can read it.

**One Jeeves worker per machine** (the user's request and decisions, 2026-11-20): instead of each session's
hooks, background runs and skill commands changing the data repo themselves (serialized today only per
script call by `take_lock`, per run by the run lock), every machine has one worker that alone commits,
merges, pulls and pushes: the skill manages the content, the worker manages the versions.
- **Per machine:** all sessions on a device use that device's worker, which owns its data repo folder;
  devices still sync through GitHub.
- **A queue folder, on demand:** hooks and commands write a request into `~/.claude/jeeves/queue/` (a temp
  file, then a rename, so a half-written request is never read); whoever finds no live worker starts one
  (the noclobber lock, PID and start time); it works through the queue in order and exits when idle. No
  socket, no listener, no process left running; a crash loses no request.
- **When:** built with Phase 2 (the sync core is ported around it from the start, and the update skill's git
  steps are rewritten to hand versioning to it); Phase 4 moves background updates onto it. Nothing is
  ported twice.
- To settle when Phase 2 is planned: how a caller waits for its result (the session-start hook and the skill
  need an answer, with a deadline), where the skill writes its text (a worktree the worker gives it, or
  `home/` under the worker's lock), and the failure list (the worker killed mid-request, two starters at
  once, a stale lock, a request from an older or newer plugin version, a hung git, a refused push, a full
  disk, a queue that grows).

Phases (each ships only when the suite passes on all three OSes, with the parity test covering every ported
command against its bash version until that version is deleted):
- **Phase 0, distribution:** planned below ("Go port, Phase 0"). Not released (2.0 is).
- **Phase 1, the small commands:** `git`, `risk`, `scan`, `check`, `memories`, `update-check`, `github`,
  `init`; `status`'s bash version removed once these pass parity on all three OSes.
- **Phase 2, the sync core:** `snapshot`, `apply`, `hook-edit`, `restore`, `cleanup`, and the settings
  backup removed (above). Built in: the link rule from "Bug: a link committed to the data repo" (below),
  with its tests. The data integrity rules hold unchanged (checked copies, the sync record, a
  clean `main`); the full upgrade rehearsal, plain and encrypted, two devices, one on the old release.
- **Phase 3, encryption:** `crypt` (git's filter, run once per file: Go's start time helps most here),
  `encrypt`, the key stores; `age` built in, so users no longer install age and its plugin.
  `filippo.io/age` is vetted first (primary sources, dated, recorded here), and must read and write the
  format today's repos hold; key stores through the same system tools as now (`security`, PowerShell,
  `secret-tool`) unless a vetted alternative needs no cgo.
- **Phase 4, background updates:** `jeeves-auto`, reading JSON with the standard library (perl no longer
  needed); its eight safeguards and failure list carried over with their tests.
- **Phase 5:** the last bash versions and the launcher's fallback removed; `bin/` holds only the wrappers
  and `jeeves-verify`.

## Bug: a link committed to the data repo (found by the 1.4.2 review, 2026-11-21; fixed in step 2b)

Reproduced on 1.4.1 and 1.4.2 (Linux; Windows Git does not make links by default): someone who can push
to the data repo commits `home/CLAUDE.md` as a symbolic link (say `../../.ssh/id_ed25519` from
`~/jeeves-data/home`). `jeeves-apply` follows it (`[ -f ]`, `hash_of` and the copy all follow links) and
installs that file's content as `~/.claude/CLAUDE.md` with no prompt (CLAUDE.md is not held); the next
snapshot commits it and `/jeeves:update` pushes it (`jeeves-scan` catches only secret patterns). A
committed folder link (`home/commands -> /elsewhere`) makes the snapshot write through it. Not fixed in bash (the
user's decision, 2026-11-21: the rewrite replaces that code, and no release comes before 2.0); the Go sync
core (Phase 2) is built to it from the start: Jeeves never follows a link from the data repo: every command that syncs (and `status`)
refuses while `home/` or `hosts/` holds one (`git ls-files -s` mode 120000, or a link on disk
under them), naming the path to remove; Jeeves itself never commits links (its copies follow the disk side's
links). Tests: a committed file link and a folder link, seen failing first. Also: `jeeves-restore`'s
`find` under a memory folder has the line-break pattern (one folder up at most, only when the user
restores): read its paths through `inside`. The user chose (2026-11-21) to ship 1.4.2 without these; 2.0
carries them, in Go.

## Release 1.4.2 (the user's decision, 2026-11-21)

The release review of the Go work found that 1.4.1, the published version, lets a folder whose name holds a
line break, planted in the data repo, make a sync write outside `~/.claude` (fixed on `main`, 3582644). The
user chose a security release before 2.0: a branch from the `v1.4.1` tag with only that fix and its test,
version 1.4.2, the CHANGELOG, the signed manifest, the upgrade rehearsal from 1.4.1, CI, and the tag on the
user's word; then the marketplace pin. `main` already holds the fix: the release branch is merged into
`main` keeping `main`'s files (its CHANGELOG gains the 1.4.2 entry).

## Go port, Phase 2: the sync core and the worker (planned 2026-11-21)

The user asked (2026-11-21) for the rest of the port to go on without stopping, asking only what is
really theirs, to professional standards for pluggability, extensibility, data integrity and security; a
code review comes before 2.0. Phase 2 is built in five steps, each merged into `main` when its suite and
CI on three OSes pass, so each lands working:

**2a. Port `snapshot`, `apply`, `hook-edit`, `restore` and `cleanup`** as Phase 1 did: the same output,
messages and exit statuses, byte for byte; parity tests (`both_versions`) over each command's cases; the
bash versions deleted once they pass on three OSes. Shared code joins `internal/lib`: the sync record
(`Bases`, `SetBase`, `KeepBases`, through a temp file and a rename, as now), atomic copies (`CopyFile`,
`CopySettled`), `StagedIs`, `RemoteGit` (deadline, no prompts, the key kept out), `ConfValue`, `BadName`,
`CaseCollisions`, `MachineID`, `CleanedRoot`, `ShrinkRepo`, `PlainHistoryAt`. As built, each command
prints its messages to the writers it is given, as the bash version did, with no separate report layer:
the worker (2d) runs each request as a child process and hands back that output, so nothing else needs
the engine's results in a structure (a later caller that does can add one then). The data integrity rules hold
unchanged: every copy checked, the record holds what was copied and committed, nothing reaches
`~/.claude` from a data repo not on a clean `main`, a failure leaves nothing recorded. `jeeves-verify`
stays bash (the snapshot hook runs it as now); `jeeves-encrypt`, `jeeves-crypt` (Phase 3) and
`jeeves-auto` (Phase 4) stay bash meanwhile and call the ported commands through their wrappers.

**2b. The link rule** ("Bug: a link committed to the data repo", below): every command that syncs, and
`status`, refuses while `home/` or `hosts/` holds a link (`git ls-files -s` mode 120000, or a link on disk
under them), naming the path to remove; Jeeves never follows a link from the data repo and never commits
one. Tests first, seen failing: a committed file link, a committed folder link, a link made on disk only.

**2c. Knowledge files only** (the user's decision, 2026-11-20): `settings.json` and `keybindings.json` are
no longer backed up or restored; `backup_settings` goes from `jeeves.conf`'s reading; copies already in
`hosts/<host>/` stay (never removed silently) until the user deletes them or runs `/jeeves:cleanup`.
`restore` restores memories only. README, the setup, restore and update skills, and the disclosure say so.

**2d. One worker per machine** (built; as built, where it differs from this plan: the worker's liveness
is an OS lock on `queue/worker.lock` (flock; on Windows an open with no sharing), which the system
releases when the process dies, so no PID or start time is compared; the passed variables are a fixed
list plus every `JEEVES_*` but the age key and the caller's lock PID, and the rest are the worker's own
(from the session that started it); a worker serves its own program's requests and leaves others' (set
aside after a day), exiting when only those wait; on Windows each child runs under a Git Bash shell whose
`$$` is the lock PID the bash scripts can check, and a deadline stops the whole tree with `taskkill /T`;
the output comes back framed (standard output and error in order), with anything written outside the
frames (a crash) after it.) Commands that change the data repo or `~/.claude` (`snapshot`, `apply`,
`restore`, `cleanup`, `memories record`, and the versioning commands of 2e) no longer run in the caller:
the caller writes a request into `~/.claude/jeeves/queue/` and waits for its answer; one worker per
machine runs the requests in order.
- **Request:** a JSON file written to a temp name and renamed (a half-written request is never read):
  format version, ID, the command (only those listed above), its arguments, the caller's folder and a
  fixed list of environment variables (`HOME`, `CLAUDE_HOME`, `CLAUDE_CONFIG_DIR`, `JEEVES_DATA`,
  `JEEVES_HOST`, `CLAUDE_PLUGIN_OPTION_DATA_REPO`, `JEEVES_KEYSTORE`, `JEEVES_KEY_DIR`, `PATH` and the
  tests' `JEEVES_*`), never a secret: the key store is read by the worker's own child.
- **Worker:** `jeeves worker`, started by the first caller that finds none alive (its lock file: PID and
  start time, `O_EXCL`; a lock whose process is gone, or whose start time differs from that PID's, is
  taken over), detached from the caller's session. It runs each request as a child process, `jeeves
  <command> <arguments>` with the request's environment and folder (the same code a direct run uses, in
  isolation: one request cannot change another's state), captures its output and exit status, writes
  the answer next to the request (temp name, rename) and removes the request. Identical snapshot
  requests waiting together run once. It exits after 10 idle seconds. Nothing listens on a socket.
- **Caller:** waits for the answer with a deadline (a hook 20 seconds, a command 120); past it a hook
  says nothing and leaves the request to finish (an edit is never lost, unlike today's "skipped, another
  run at work"), a command says the request is still queued and exits 1. The answer's output is printed
  as the command's own and its exit status returned, so the skills and the hooks see what they see today.
- **Failure list** (each with a test that injects it): the worker killed mid-request (the request stays;
  the next worker runs it again: every command is safe to rerun, as each starts from the data repo's
  state); a child that hangs (a deadline per request, 10 minutes, then SIGTERM and SIGKILL; answered
  "stopped after 10 minutes"); two callers starting a worker at once (the lock: one wins, the other
  waits for it); a stale lock (taken over); a request that does not read, or from an unknown format
  version or command (set aside in `queue/rejected/` and answered with why, never run); a full disk
  (the request cannot be written: the caller says so and exits 1, or a hook stays quiet); an answer
  nobody collects (removed after a day); a queue that grows (more than 100 requests: new ones refused
  with a message); a worker from an older plugin version (requests carry the plugin root; a worker serves
  only its own, so after an update the old worker exits when idle and a new one starts).
- `JEEVES_NO_WORKER=1` runs a command in the caller, as today (tests of the commands themselves, and a
  last resort the README names).

**2e. The skill manages the content, the worker the versions.** The skills edit files; every change to
the data repo's history goes through commands the worker runs, each checking what it is given, and
`jeeves-git` becomes read-only. Commands (one Go package, `internal/version`, a subcommand each; wrappers
in `bin/`):
- `jeeves-commit [--branch <kind>/<name>] -m <message> -- <path>...`: with `--branch`, from a clean `main`,
  starts that branch (`pref/`, `conf/` or `review/` only); stages exactly the paths named, as they are on
  disk (a deletion included), and commits on the current branch. Paths only under `home/`, or
  `jeeves.conf` (no `hosts/`, `.sync-state/`, `.git`, `..`, links); nothing staged otherwise is
  committed (the index must hold nothing else). Prints the commit's short hash.
- `jeeves-commit --fold -m <message>`: the commits not yet pushed folded into one (`reset --soft
  origin/main`, then commit), for a secret that must leave the history before a push; only on `main`.
- `jeeves-merge <branch> -m <summary>`: from `main`, `merge --no-ff` of a `pref/`, `conf/` or `review/`
  branch, message `Merge branch '<branch>': <summary>`, then deletes the branch.
- `jeeves-merge origin -m <what came in>`: merges `origin/main` into `main`; on conflicts it stops and
  lists the files (exit 3) for the skill to resolve by editing them; `jeeves-merge --continue` checks each
  listed file has no conflict markers, stages them and commits; `jeeves-merge --abort` puts `main` back.
- `jeeves-revert <commit>`: `revert --no-edit` on the current branch; a conflict is aborted and reported.
- `jeeves-push`: on a clean `main` only: `jeeves-scan` and `jeeves-risk` over what it would send (a
  finding stops it, with the scan's own report), then `push origin main`, never forced; a refused push
  says to run `jeeves-merge origin`. `jeeves-cleanup` keeps its own lease push.
- `jeeves-git`: only commands that change nothing (`log`, `show`, `diff`, `status`, `cat-file`,
  `ls-files`, `rev-parse`, `branch --list`, `merge-base`, `blame`); anything else is refused, naming the
  command to use. The README's "remove it with `jeeves-git rm`" becomes deleting the file and
  `jeeves-commit`.
The `update`, `review`, `cleanup`, `encryption`, `setup` and `restore` skills use them; their allow lists
drop `jeeves-git` writes, and the tests that read skill text follow. An agent follows each changed skill
against scratch folders (the repo's CLAUDE.md, "Working on the plugin") before the merge.

Docs with each step: README (commands, the worker, "What Jeeves runs, sends and changes": the worker
process, its queue folder), `.claude/CLAUDE.md` layout, CHANGELOG 2.0.0.

## Go port, Phase 3: encryption with age built in (planned 2026-11-21)

`jeeves-encrypt` (`status`, `on`, `unlock`, `lock`, `passphrase`) and `jeeves-crypt` (git's clean and
smudge filter, textconv and merge driver) in Go, with age built in: users no longer install `age` or
`age-plugin-batchpass`. Parity tests against the bash versions over each case, then the bash versions and
the parts of `lib/crypt.sh` only they use are deleted once green on three OSes. As built: `jeeves-crypt`
never had a bash version (a filter must not fall back); `lib/crypt.sh` became `lib/keys.sh`, which keeps
only `store_get` for `jeeves-auto`; `lib/encrypt.sh` was deleted after its parity test passed on three OSes
(CI run 44945877107), and the parity steps became a Go-only test of each step and misstep
(`t_encrypt_answers_each_step_and_misstep`); the key store's name check has a Go test.
- **The library, vetted 2026-11-21 from primary sources** (GitHub API): `filippo.io/age` v1.3.2
  (2026-08-29; releases yearly, v1.3.0 2025-12, v1.2.1 2024-12); BSD-3-Clause; one main maintainer
  (Filippo Valsorda, 333 commits; others under 10), issues answered this month; one advisory,
  GHSA-32gq-x56h-299c (2024-12, plugin names running a program), fixed in 1.2.1 and only in the plugin
  code, which Jeeves does not use. It pulls in `golang.org/x/crypto`, `x/sys`, `x/term` and the author's
  `edwards25519`, `hpke` and `nistec`. Size measured: +545 KB on the linux/amd64 build (3.88 to 4.43 MB,
  the release flags). Building our own age is not an option (cryptography); keeping the CLI keeps the
  install step this phase removes. The README's list of what the programs are built with gains age (2d
  added the Go team's `golang.org/x/sys`, for Windows' `LockFileEx`: the same module age pulls in). Pinned exactly in `go.mod`; `go.sum` committed; the reproducible build check covers it.
- **Formats unchanged:** the data repo key is an X25519 identity; `identity.age` holds it encrypted with
  the passphrase as a standard scrypt stanza, armored (checked: `age -e -j batchpass -a` writes
  `-> scrypt <salt> 18`), which the library reads and writes; files are age v1. A data repo encrypted by
  1.4.x opens unchanged, and one written by 2.0 opens with the age CLI (a test does both ways with the
  CLI when it is on the PATH; CI installs it for that).
- **The filter:** git's long-running filter protocol (`filter.jeeves.process`, git 2.11+), so one process
  serves a whole checkout or add instead of one per file (a process start costs tens of milliseconds on
  Windows); `required = true`, so a failure stops git and plain text never reaches a commit. The clean
  side keeps the cache (plain hash to ciphertext under `.git/jeeves/clean/`) so an unchanged file cleans
  to the same bytes; an entry that does not decrypt to its plain text is dropped, never served. The
  textconv and merge driver stay one process per call (git's interface); the merge driver writes
  `jeeves: not merged` and fails when the key cannot be read, as now.
- **Passphrases:** the same dialogs (osascript, zenity or kdialog, PowerShell) and `--terminal` (read
  with no echo through `x/term`, already in the build with age); the passphrase stays in the process's
  memory only, never in a command line, the environment or a file; the key goes to the stores on stdin as
  now (`lib.StoreGet`'s counterpart `StorePut`).
- **What can fail** (each with a test): the key store missing or refusing (nothing changed, said why);
  a wrong passphrase; a dialog cancelled; `identity.age` not matching `recipient`; the filter process
  killed mid-checkout (git stops; the next run redoes it); a cache entry corrupted or for another key;
  a merge with the key unreadable; a disk full while writing `identity.age` (written to a temporary file
  and checked by decrypting before it replaces the old one, as now).

## Go port, Phase 1: the small commands (planned 2026-11-21)

Ports `git`, `risk`, `scan`, `check`, `memories`, `update-check`, `github` and `init` into `cmd/jeeves`, one
package each under `internal/`, shared code in `internal/lib`. Each `bin/` script becomes the three-line
wrapper (as `jeeves-status`), its bash body moved to `lib/<command>.sh`. Behaviour, messages and exit
statuses stay the same, byte for byte; the README changes only where noted. Steps, one commit each:

1. **`internal/lib` additions:** `GitRepo` (the scripts' `git_repo` options), `WithDeadline` (the command's
   own process: a graceful stop, then a forced one after a second, status 124; `exec.CommandContext`
   with `Cancel` and `WaitDelay`), `OSName` (linux, wsl, macos, windows), `KeystoreKind`, `StoreGet` (the
   key read through the same system tools as `lib/crypt.sh`: `security`, `secret-tool`, the same
   PowerShell text, a key file; reused by Phase 3), `EncryptedAt`, `IsDevCheckout`, `VersionAtLeast`,
   `TakeLock` / `ReleaseLock`.
2. **`git`, `init`, `github`:** git and gh are still run as programs (gh with its own `--jq`).
3. **`scan` and `risk`:** `outgoing_added` (the `git log -p --cc -U0` parser), the secret patterns and
   `risk_flags` in Go regexps (POSIX classes and `(?i)` match grep's for these patterns; `go test` holds
   each pattern's matching and non-matching samples). An encrypted repo: the key from `StoreGet` into
   `JEEVES_AGE_KEY` for git's own processes only; git still runs the bash `jeeves-crypt` (Phase 3 ports
   it). `risk_flags` stays in `lib/lib.sh` too until Phase 2 (`jeeves-apply` uses it); the parity test
   keeps both in step.
4. **`check`, `update-check`:** `check`'s network checks keep their deadlines. `update-check` still runs
   curl, with the same arguments: Go's own HTTPS client (`net/http`) was measured at 6.2 MB a build
   against 2.2 MB without it (linux-amd64, 2026-11-21), about 20 MB more per release across the five.
5. **`memories`:** `record` takes the data repo's lock, which the bash sync scripts share, so the lock file
   keeps its format (the holder's PID; takeover under `lock.takeover`). On Windows a Go PID is not one
   Git Bash's `kill -0` knows: the launcher passes its own `$$` (`JEEVES_LOCK_PID`; the wrapper lives as
   long as the program), which Go writes into the lock, and Go checks a bash holder's PID through
   `kill -0` in bash (only while a lock is held). Tests: a Go record waits for a bash-held lock and the
   reverse, and a stale lock from either is taken over.
6. **Tests:** each command's existing tests run the Go build (the wrappers' default); a parity test per
   command (`both_versions`) runs its cases through both versions and compares stdout, stderr and exit
   status: odd names (spaces, accents, a line break), an encrypted repo, a merge commit, no
   `origin/main`, gh and ssh stand-ins that fail or hang, an unreadable record. The full suite also runs
   once locally with `JEEVES_NO_GO=1` before the bash versions go.
7. **Bash versions removed** (the user's rule: once parity passes on all three OSes): CI by hand on the
   branch; when green, `lib/status.sh` and the eight `lib/<command>.sh` are deleted, with their parity
   tests. The launcher, finding no bash version and no build that fits, says which machines 2.0 has
   builds for and exits 1 (a test). Then `dist/` rebuilt and checked, the full suite, merge.

Failure list (`update-check` runs from the session-start hook, unattended): no network or a hung server
(the deadline; silent, tried the next day), an odd or huge answer (the size limit, the strict `vX.Y.Z`
match), an unwritable record (silent, as now), a clock set back (counted as never checked). Each has a test
today; they run against the Go build. `memories record` while another run holds the lock: waits 30
seconds, then says so with nothing recorded (a test, both lock holders).

Docs: README (the Go notes list the ported commands), `.claude/CLAUDE.md`
Go bullet (the ported commands), CHANGELOG 2.0.0.

## Go port, Phase 0: distribution (planned with the user, 2026-11-21)

Goal: the Go build is committed, signed with each release and checked by CI, with `status` as its only
command, so later phases only add commands. Not released on its own: 2.0 is (the user's decision,
2026-11-21).

Decided with the user (2026-11-21): **Claude builds the release's five builds on this PC** (as it signs the
manifest), commits them on the release branch, and CI on Linux, macOS and Windows rebuilds from source and
fails unless every byte matches. No new CI action.

Build:
1. **One launcher**, `lib/launch.sh`, sourced by each ported command's wrapper
   (`. "$(dirname "$0")/../lib/launch.sh" status "$@"`), so a wrapper is two lines and adds no process.
   It picks the build from bash's own `$OSTYPE` and `$HOSTTYPE` instead of two `uname` calls (each a
   process: slow on Windows, where the spike's Go run took 662 ms, most of it outside the program).
   Measured again on Windows with `time-status.sh`, and trimmed where it is the launcher (or the program's
   `cygpath` calls) that costs. The fallback and its once-per-reason note stay as the spike built them.
2. **`dist/` committed:** out of `.gitignore`; `.gitattributes` marks `dist/**` binary (no line-ending
   conversion, no text diffs). `go-builds.sha256` goes: the committed builds are the record, and the signed
   `MANIFEST` holds their hashes. `build-go.sh --check` builds into a temp folder and compares with
   `dist/`; `--record` is replaced by a plain build into `dist/`.
3. **Release skill:** step 4 adds gofmt, `go vet`, `go test` and `build-go.sh --check`; step 6 builds `dist/`
   right after the version bump and commits it ("Release X.Y.Z: Go builds"), before the manifest, so the
   signature covers the builds; step 7's CI run then reproduces them on three OSes.
4. **Rehearsal** (`rehearse.sh` and the published-plugin check in a scratch `CLAUDE_CONFIG_DIR`): after the
   upgrade from 1.4.1, the cached copy holds `dist/` with the executable bit kept (not yet checked: whether
   Claude Code's install copy keeps file modes; the scripts in `bin/` rely on it already), `jeeves-status`
   runs the Go build (no fallback note), and `jeeves-verify` passes; and on a copy with this machine's
   build removed, the bash version runs.
5. **Docs:** README's "What Jeeves runs, sends and changes" (compiled programs in `dist/`, built from
   `cmd/` and `internal/`, the same bytes from any machine, and how to check that:
   `build-go.sh --check` with the Go version `go.mod` names), "Working on Jeeves" (Go needed only to change
   Go code; rebuild `dist/` before committing a Go change), "Tests" (`go test ./...`). CHANGELOG's
   unreleased entry gains the Go `status`, the sort fix and the uninstall section. The directory note: compiled files
   over 256 KiB are held for a reviewer from 2.0 on.

Failure list (the launcher runs wherever a ported command runs; once hooks are ported, at every session
start), each with a test that makes it happen where one can:
- No build for this OS or processor: the bash version, a note once (tested in the spike).
- A build that cannot start (lost its executable bit in an install copy, a wrong format): the bash version,
  a note once (tested in the spike; the rehearsal checks the bit on a real install).
- A build removed or changed on disk (an antivirus quarantine, tampering): the launcher runs the bash
  version when it is missing; `jeeves-verify` reports it either way at session start, and the hooks stop.
- The program crashes (a Go panic exits 2, as a usage error does): no fallback, since the two cannot be told
  apart; its message reaches the caller. A test makes it panic through a test-only setting.
- A build that does not match the source (stale, or not reproducible): CI's `--check` and the release
  step fail; the suite's parity test fails when a stale build differs from bash.
- A development checkout with a Go change but no rebuild: the same checks; "Working on Jeeves" says to
  rebuild.

Not in Phase 0: porting more commands (Phase 1), and the per-machine worker (Phase 2).

As built (2026-11-21, branch `feat/go-dist`):
- `lib/launch.sh` finds the plugin folder by name (the same string as `cd .. && pwd`, checked for relative,
  absolute, symlinked and doubled-slash calls and a bare name) and the platform from `$OSTYPE` and
  `$HOSTTYPE`; it checks the executable bit with a builtin and runs the build directly rather than
  capturing its messages (a subshell cost about 100 ms on Windows), so a build in a wrong format falls back
  after bash's own error line. The program converts the plugin folder with `cygpath` only when a command
  needs it (an encrypted repo's filter settings). Windows, 50 files, Go `status`: 662 ms (spike) to
  391 ms; the program alone 318 ms to 172 ms. The rest is bash starting the wrapper and the program's git
  calls (about 110 ms each there).
- `dist/` is kept current on `main` with each Go change rather than rebuilt at release (the builds hold no
  version), and release step 4 checks it with `build-go.sh --check`; `go-builds.sha256` and `--record`
  are gone.
- Tests: a Go crash (`JEEVES_TEST_PANIC=1`) exits 2 with its message and no fallback; the launcher finds the
  plugin folder however it is called; a build that is not executable falls back with only the note. The
  rehearsal from 1.4.1 checks that the Go build runs on the old state, agrees with the bash version, and
  leaves git's filter settings as the bash version writes them (plain and encrypted): REHEARSAL OK.
- Release review (an independent agent, 2026-11-21), each finding checked before fixing:
  - Fixed on `main` (`fix/bash-path-handling`, older bash bugs): a name with a line break made `find` give a
    second "path" such as `../x`, and apply wrote outside `~/.claude` (reproduced by the new test on the old
    code); `base_of` and `set_base` read paths through `ENVIRON` (awk `-v` ate backslashes); `git
    hash-object` runs outside any repository (a SHA-256 repo around the current folder changed the hashes).
  - Fixed here: a bare `--data-repo` (given and empty, as bash reads it); `build-go.sh` pins `GOAMD64`,
    `GOARM64`, `GOEXPERIMENT`, `GOENV` and `GOWORK` (an inherited `GOAMD64=v3` changed the amd64 builds);
    a broken `~/jeeves-data/.git` link reads as none, as bash's `-e`; paths that reach the program
    unconverted on Windows are converted; the wrapper never sources a launcher relative to the caller's
    folder (a bare name found on the PATH did); README discloses `~/.claude/jeeves/go-fallback`.
  - Left as they are: stderr differs from bash's in error cases (an unreadable file, folder or record; the
    parity test covers stdout, exit status, and stderr in normal runs); `//` at the start of a path is
    collapsed by Go; Go refuses an empty host name on Windows where bash used "-win"; a build in a wrong
    format shows bash's own error at each run (no release ships one); `JEEVES_TEST_PANIC` stays (it only
    makes the program crash). With Git Bash's path conversion off (`MSYS_NO_PATHCONV` set, even empty),
    the bash version cannot sync either (Git for Windows cannot open `/tmp/...` paths), so that setting is
    unsupported, as before.
  - Also found in testing the fixes: the wrapper refused a relative call (`bin/jeeves-status`); fixed, with
    a test.

## Go rewrite: spike (requested and planned 2026-11-20)

The request: the user questioned bash (hard to debug; tools that differ between systems change output) and
asked whether another language and deployment would be better. Assessed with the user (2026-11-20):
- Not a uv/brew/PyPI package: skills and hooks reach users only through the plugin marketplace, so a package
  would be a second update channel that can drift from the plugin (Claude Code docs: plugins have no
  dependency mechanism for Python; only npm from a lockfile installs itself).
- Python inside the plugin would keep one channel and readable code, but needs a Python install on Windows
  and on Macs without Apple's Command Line Tools.
- Go, as precompiled binaries inside the plugin: no runtime to install, one code path on every OS, and
  `age` could be built in (`filippo.io/age`, to vet) so users no longer install age and its plugin. The cost
  is trust: users no longer read the code that runs. Rust was set aside (cross-building for macOS needs
  Apple's toolchain; its memory-safety gains matter little here).
- Evidence for the change: about 3,300 lines of bash and 2,600 of tests; commit messages mention Windows 31
  times, macOS 21, sed 14, JSON 11, mkdir locks 7: most of the work went to tools that differ between systems.

The spike answers the questions a full port depends on, on a branch, released to no one:
1. **Scope:** the shared library (finding the data repo, the tracked paths, hashes, the three-way
   classification, `jeeves.conf`, git with the same settings `git_repo` uses) and `jeeves-status`, ported to
   Go, standard library only. One binary with subcommands (`jeeves status`), so a release holds one file per
   platform, not one per command.
2. **Layout:** `go.mod` at the root (module `github.com/WinuxNomacs/jeeves`), `cmd/jeeves/`, `internal/...`.
   `bin/jeeves-status` becomes a few lines of bash that run `dist/jeeves-<os>-<arch>[.exe] status "$@"`. The
   other `bin/` scripts stay bash. Binaries are built into `dist/`, which stays out of git during the spike:
   committing them would grow the history for good, even if the port is dropped.
3. **Parity:** the existing `jeeves-status` tests run unchanged against the Go version (they call
   `bin/jeeves-status`), so its output must match the bash version byte for byte, as the skills rely on it.
   Go unit tests for the classification and hashing, with failures injected (a missing repo, a locked
   device, an unreadable file).
4. **Builds:** five targets (macOS arm64 and amd64, Linux amd64 and arm64, Windows amd64) from Linux with
   `CGO_ENABLED=0`, `-trimpath`, `-buildvcs=false` and an empty build ID, by a script beside the release
   scripts. Reproducible: two builds, and CI's build, give the same hashes.
5. **Measured and reported to the user:** each binary's size; the five together (what each release would add
   to the repo); `jeeves-status` time, Go against bash, on Linux, macOS and Windows (the skills run it at
   each sync; the session-start hook runs `jeeves-snapshot`, which sources `lib.sh` but not `jeeves-status`,
   so this measures the port of the library the hooks will use).
6. **Real platforms:**
   - macOS: the user runs the launcher from a checkout of the branch. Does Gatekeeper question an unsigned
     binary that arrived through git (git sets no quarantine flag, so probably not; unverified)?
   - Windows: run from WSL through the PC's Git Bash, Defender on. Is an unsigned Go binary flagged? Plus
     the parity tests under Windows Git Bash.
   - CI: the workflow gains a Go setup step (not a new trigger or workflow); `actions/setup-go` is at v7.1.0,
     published 2026-11-17, three days before this plan: vet it, and pin a release at least two weeks old by
     commit hash.
7. **Directory:** compiled programs and files over 256 KiB are held for a reviewer (pre-submission checklist).
   Submitting stays the user's act; nothing is submitted in this phase.

Setup: Go is not installed on this machine. The latest stable release is 1.27.4 (go.dev/dl, 2026-11-20).
Installing it changes the system, so the exact command waits for the user's yes when the build starts; the
version is pinned in `go.mod` (`go 1.27`, with a `toolchain` line).

Not in the spike: encryption (`filippo.io/age` and a key store library are vetted in the phase that ports
`jeeves-crypt`), the other commands, and the decision to commit binaries or download them on first use.

Go/no-go, the user's call at the end, with the measurements in hand:
- **Size:** what each release adds to the repo (all five binaries) is acceptable.
- **Platforms:** macOS runs the binary with no Gatekeeper prompt, and Windows with no Defender block.
- **Reproducible:** CI's build matches the local one.
- **Parity:** the status tests pass on all three OSes, and the session-start time is no worse than bash.
If go: the full port is planned as phases (the library and sync scripts, encryption with `age` built in,
background updates, the rest), each released only when its tests pass on all three OSes. If no-go: the
spike's findings are recorded here and bash stays.

Failure list for the launcher (the skills run it at each sync; once the hooks' scripts are ported, at every
session start): no binary for
this OS or architecture (fall back to the bash script, and say so once); a binary that fails to start (same
fallback); a binary that does not match the signed manifest (`jeeves-verify` covers `dist/` as it does
`bin/`). Each gets a test that makes it happen.

### Spike results (built 2026-11-20, branch `spike/go`)

Built: `go.mod`, `cmd/jeeves/` (one program, `jeeves status` for now), `internal/lib/` (data repo discovery,
the record, encryption state and `crypt_setup`'s git settings, tracked paths, blob hashes, the sync record,
classification), `internal/status/`. `bin/jeeves-status` is now a launcher; the bash version moved to
`lib/status.sh`. `.claude/skills/release/build-go.sh` (five targets; `--record` and `--check` against
`go-builds.sha256`), `time-status.sh` (the timing). Go 1.27.4 installed on this machine in
`~/.local/go-1.27.4`, linked from `~/.local/bin` (the user's yes, 2026-11-20).

- **Parity:** the suite's status tests, plus a new test that runs both versions on every state (disk, repo,
  conflict, no base, a name with a space and an accent, excluded folders, CRLF and empty files, a merge in
  progress, the refusals) and compares stdout, stderr and exit status byte for byte. Full suite, encryption
  tests included: 145 passed, 0 failed on Linux here, and in CI on all three OSes (macOS with bash 3.2 and
  the arm64 build; Windows Git Bash, 32.6 minutes, so the job's limit went from 30 to 45 on `main`).
  Windows Git Bash on this PC: the status tests 5 passed. The parity
  test was seen failing against a build that changes one word.
- **Found by the parity test on Windows:** bash's `sort -u` in `all_tracked` followed the locale (Git Bash
  put `CLAUDE.md` after `agents/`), so the bash version's own order differed between machines; fixed in bash
  (`LC_ALL=C`), its own commit. Also found in the design: Git Bash hands a native program its paths in
  Windows form (`C:\...`), while the scripts print and record `/c/...` or `/tmp/...`; the Go build converts
  with `cygpath` (both ways) where a path is printed, recorded or read from a record, and the parity test
  passes on Windows with the data repo under Git Bash's `/tmp`.
- **Sizes** (stripped, `-s -w`): macOS arm64 2,011,666 bytes, macOS amd64 2,143,424, Linux amd64 2,183,292,
  Linux arm64 2,097,276, Windows amd64 2,296,320; all five 10,731,982 bytes (about 10.2 MiB), each over the
  directory's 256 KiB "held for a reviewer" line. Every release that changes the Go code adds about that
  much to the repo if binaries are committed (git compresses them somewhat; unmeasured). More commands add
  little: most of the size is Go's runtime.
- **Reproducible:** two local builds, one with an empty build cache and one from another folder, gave the same
  five SHA-256 hashes, and so did CI's builds on its Linux, macOS (arm64) and Windows runners: the same bytes
  from any of the three OSes (`build-go.sh --check`).
- **Time** (`time-status.sh`, the launcher included; nothing else heavy running):

  | Machine | Files | Go | bash |
  |---|---|---|---|
  | Windows Git Bash (this PC) | 50 | 662 ms | 18,589 ms |
  | macOS (the user's Mac, arm64) | 50 | 34 ms | 556 ms |
  | Linux (WSL, this PC) | 500 | 51 ms | 2,672 ms |
  | Linux (WSL, this PC) | 50 | 34 ms | 304 ms |

  On Windows most of the Go build's time is the launcher's own bash and the processes it starts (uname,
  cygpath, git); trimming them is for the full port.
- **Windows Defender** (real-time and tamper protection on, signatures of 2026-11-20): the unsigned
  `jeeves-windows-amd64.exe`, copied in from WSL, ran repeatedly with no block and no detection
  (`Get-MpThreatDetection`: none in the last day). A file that arrives through git carries no
  Mark-of-the-Web, as an install's clone does.
- **macOS Gatekeeper** (the user's Mac, 2026-11-20): the arm64 build, fetched through git from the throwaway
  branch `spike/go-dist` (as an install's files arrive; the branch is deleted), ran with no dialog. `spctl --assess --type execute`
  says "rejected": what Gatekeeper would do to the same file carrying a quarantine flag (a browser download),
  so builds offered as downloads would need an Apple Developer ID signature and notarization (paid
  program); builds that arrive through git do not.
- **CI:** not `actions/setup-go` (v7.1.0 was three days old; v7.0.0 of 2026-07-16 would have passed the
  two-week rule): Go is installed as `age` already is, from go.dev's archive, pinned by its published
  SHA-256, so no action and no dependencies are added. Then gofmt, vet, the unit tests and
  `build-go.sh --check` on each OS, before the suite.

## Background auto-update (requested and planned 2026-11-20)

The request: a toggle (`/jeeves:autoupdate`, default off) and, when on, `/jeeves:update` running every so
often in the background of each session, as silently as possible, resolving what it can on its own with the
judgement the update skill already uses.

Decided with the user (2026-11-20):
- **Mechanism: detached headless.** A Stop hook (runs each time Claude finishes a reply) checks whether a
  run is due and, if so, starts a detached `claude -p` that runs the update in auto mode on the parent
  session's transcript, then exits at once. The user's session is never interrupted and its context is not
  spent; each run costs plan usage.
- **Asking: once per device.** While off and never answered on this device, the session-start hook prints a
  one-line offer; after a yes or no it stays quiet. `/jeeves:autoupdate` (user-only) turns it on or off.
- **Setting: per device** (`~/.claude/jeeves/`), not `jeeves.conf`.
- **Project CLAUDE.md: queued.** Auto mode writes only the global rules (data repo) and memories; project
  items are kept for the next manual `/jeeves:update`, which offers them. It never touches a working tree
  the user may be in.

What auto mode decides alone vs. leaves for the user (the user's rules, not relaxed by this phase):
- Alone: sync, non-overlapping merges, `learn:` items and revisions in place (a curation that removes a rule is queued, safeguard 8),
  memory review.
- Left for the next manual run, with a one-line notice at session start: risky items; held skills, agents
  and commands from another device (never auto-accepted); a true text conflict; a conflict with a rule the
  user set; `jeeves-risk` flags (nothing pushed until the user's run settles them; as built); a `jeeves-scan` finding (stops the run, nothing
  pushed); a locked device; a push rejected twice.

Collisions found (2026-11-20, in the code): `take_lock` (`lib/lib.sh`) guards one script call, not a skill
run, and the update switches branches in the one data repo folder, so two runs on one machine would switch
branches under each other. Needed: a run lock for the whole auto run (PID and start time, stale after the
deadline or a dead PID), and a second due run skips rather than waits. A manual `/jeeves:update` while an
auto run holds it waits briefly, then says so. Other devices: already safe (fetch, merge, push; never forced).

Design (proposed, to settle in 1a):
- `jeeves-auto` script: `status`, `on`, `off`, `due` (from the hook: on, idle interval passed, lock free),
  `start` (spawns the child detached, logs to `~/.claude/jeeves/auto/`), and the per-session watermark
  (how far each transcript was reviewed).
- Child: `claude -p "/jeeves:update auto <transcript path>"` with a fixed `--allowedTools` list, a deadline
  and stop escalation; `JEEVES_AUTO_CHILD=1` so its own hooks do nothing (no recursion); the update skill
  gets an "auto" section (no AskUserQuestion; defer instead; queue project items and deferred items in a
  file the manual run and the session-start notice read).
- Interval (the user's decision, 2026-11-20, after 1a): at most once an hour per session, and only when the
  session has grown by a set amount since its watermark (the threshold settled in 1b); idle or light sessions
  start no run.
- Model (the user's decision, 2026-11-20): the same model as the session (no `--model` override).

Security and integrity safeguards (all eight decided with the user, 2026-11-20; each gets a test):
1. **Prompt injection.** The child reads only an extract: no tool results; each message wrapped in a marker
   with a per-run random nonce, so text inside a message cannot pose as the user ("USER:" in quoted text
   passes today). Risky items are deferred, never written. The script check (3) runs before any push.
2. **Least privilege.** `--permission-mode dontAsk` (anything not allowed is denied), with an allowlist:
   Read, Edit/Write limited to the child's worktree, and named `jeeves-` commands. No web tools, no general
   Bash, never `bypassPermissions`. To verify in 1b: `dontAsk` honours path-scoped Edit rules.
3. **The model writes text; a script does the rest.** The child edits in a separate git worktree of the
   data repo (never the main folder, so session snapshots and the auto run never switch branches under each
   other). `jeeves-auto` then checks the result: only `home/CLAUDE.md` and the queue file changed; nothing
   under `hosts/` or `.sync-state/`; the Security section and protected rules unchanged or only added to
   (compared with the version before). Only then does it merge into `main`, run `jeeves-scan` and push,
   under the run lock. A result that fails is set aside (not pushed) with a session-start notice. To verify
   in 1b: the encryption filters work in a worktree.
4. **Clean environment.** The child starts with only the variables it needs (`env -i` plus `HOME`, `PATH`,
   locale, `JEEVES_AUTO_CHILD=1` and the like); the parent's `CLAUDE_CODE_MESSAGING_TOKEN`, socket and
   session variables are not passed on (seen in the hook's environment in 1a).
5. **Input validation.** Without `jq`, the transcript path from the hook's JSON is accepted only under
   `~/.claude/projects/` and matching a strict pattern (no `..`, no newlines, no shell metacharacters);
   always quoted, never in an `eval` or the prompt unvalidated. Missing or odd fields: skip and log.
6. **Private, minimal logs.** `umask 077`; per run only the usage figures, outcome, and commits made; never
   transcript text or the child's final message. Kept to the last N runs.
7. **A run lock for the whole run.** Recording PID and start time (a reused PID is not mistaken for the
   holder), stale after the run's deadline or when the PID is gone. A second due run skips; a manual
   `/jeeves:update` waits briefly, then says an auto run holds it.
8. **Nothing silent, nothing removed.** The next session start shows a one-line notice listing what the
   auto run changed, with the `/jeeves:update revert <commit>` command. Auto runs may add and revise rules,
   never remove one: removals (curation) are queued for the next manual run, where the report shows them.

Failure list (global rule for unattended work; each gets an injected-failure test): no network or GitHub
down (fetch or push fails: nothing lost, retried next run); the child hangs (deadline, SIGTERM then
SIGKILL, lock released); the child is killed mid-run (stale lock taken over; a `pref/` branch left behind is
finished or removed by the next run); the parent session ends (child keeps going to its deadline); two
sessions due at once (one runs, one skips); a manual run meanwhile (waits, then says so); disk full (copy
checks stop with nothing recorded, as today); the transcript moved or unreadable (skip, log); `claude` not
on the PATH or signed out (log once, notice at session start); a locked encrypted device (skip, notice);
plan usage exhausted (child fails: log, back off a day); the log grows (kept to the last N runs).

1a results (2026-11-20, Claude Code 2.1.304; checked with a scratch plugin and `claude -p`; docs:
https://code.claude.com/docs/en/hooks, /headless):
- The Stop hook fires in `-p` mode too. Its stdin holds `session_id`, `transcript_path`, `cwd`, `prompt_id`,
  `permission_mode`, `effort`, `stop_hook_active`, `last_assistant_message`, `background_tasks` and
  `session_crons`. The docs say the transcript "may lag the in-memory conversation" (a run reviews up to
  its watermark, so a lag only delays an item to the next run). SessionEnd fires too (`reason: other`;
  docs: a 1.5 s budget shared by all SessionEnd hooks): too short to start work, enough to record an end.
- A child started with `nohup ... </dev/null >log 2>&1 &` survives the hook and `claude` exiting (a `setsid`
  one too; macOS has no `setsid` command, so nohup it is, checked on the Mac in 1b). Docs: an `async: true`
  hook is killed at `-p` teardown, and they recommend "a fully detached process" for work that must outlive
  the session; output not redirected holds the hook up to 10 s.
- An env variable set for the child (`JEEVES_AUTO_CHILD=1`) reaches the child's own hooks: the recursion
  guard works.
- A user-only plugin skill with arguments works as the `-p` prompt (`/plugin:skill auto <path>`, arguments
  and `${CLAUDE_PLUGIN_ROOT}` filled in).
- `--output-format json` gives `total_cost_usd`, `duration_ms`, `num_turns`, `usage`, `modelUsage` and
  `permission_denials`: the child logs these per run.
- Transcripts are large: this session's was 3.1 MB, of which an extract (the user's messages, Claude's text,
  tool calls cut to 300 characters; no tool results) is 111 KB. The child must read an extract, not the raw
  file. Open for 1b: a portable extractor (no `jq`; macOS ships no `python3` by default).

Measured usage (list-price equivalent from `total_cost_usd`; on a subscription it is plan usage, not a
bill), review steps 1-3 only, read-only, no memories pending; sync, writing and push not measured:

| Session reviewed              | Model      | Usage    | Time | Turns |
|-------------------------------|------------|----------|------|-------|
| full long session (111 KB)    | Opus 5.5   | $0.80    | 50 s | 5     |
| full long session (111 KB)    | Sonnet 5.5 | $0.49    | 26 s | 6     |
| empty (one exchange)          | Opus 5.5   | $0.35    | 18 s | 4     |

The floor (about $0.35 on Opus) is the skill and the two CLAUDE.md files, paid on every run whatever the
session held: a run should start only when the session has new activity worth reviewing, not on a clock
alone. Opus found one useful item that Sonnet left out; both left out what was already saved.


As built (1b, 2026-11-20, branch `feat/auto-update`), where it differs from the plan above:
- No git worktree: the model never touches the data repo at all. The script copies into a private temp
  folder (`mktemp -d`, removed on exit, a leftover removed by the next run) the extract, `home/CLAUDE.md` to
  edit, the project's CLAUDE.md and up to 25 memory files; it merges the edited copy into `main` itself
  (three-way with `git merge-file` when `main` moved meanwhile; a conflict sets the edit aside). This also
  removes the open question about encryption in a worktree.
- The child's permissions (checked headless with 2.1.304): `dontAsk`, `Read(//<run>/**)`,
  `Edit(//<run>/edit/**)`, `Write`/`Edit(//<run>/out/**)`, `--disallowedTools Bash WebFetch WebSearch Agent
  NotebookEdit`, `--strict-mcp-config` (no MCP servers), `--no-session-persistence` (no transcript of its
  own). Reads, globs and greps outside the folder were denied; edits and shell writes outside it too.
- Clean environment by removal, not `env -i` (macOS and Windows need many system variables): every
  `CLAUDE*` and `JEEVES*` variable is dropped but those selecting config, sign-in and provider
  (`CLAUDE_CONFIG_DIR`, `CLAUDE_CODE_OAUTH_TOKEN`, `CLAUDE_CODE_USE_BEDROCK`/`VERTEX`/`FOUNDRY` and their
  `SKIP_*_AUTH`, `CLAUDE_CODE_GIT_BASH_PATH`); `JEEVES_AUTO_CHILD=1` is set.
- The child does not load the user's `settings.json` (`--setting-sources project`; the run folder has
  none): a user's allow rules, `additionalDirectories`, hooks or other plugins would widen it. The plugin is
  loaded with `--plugin-dir`, and the settings' `model` and `env` (minus `CLAUDE_CODE_PLUGIN_DIRS`) are carried
  over. Checked headless: the skill resolves and sign-in works that way, and a denied tool stays denied even
  when the skill's `allowed-tools` grants it (so the update skill's push and commit grants give the child
  nothing).
- The edit check also keeps every line with "never", "always", "from now on" or "the user's decision"
  word for word, and refuses any diff hunk that removes more lines than it adds (a removal plus an addition
  elsewhere kept the rule count, so the count alone missed it; seen in a test).
- A secret in the run's own commit: the commit is taken back out (checked again under the lock, `reset
  --keep`), the edit set aside, and the rest pushed.
- Independent review (2026-11-20, an agent given the code and the checklist): 18 findings, all fixed or
  noted. Fixed: bullets missed by the changed-lines count (now at most 12, counted from the diff body);
  additions to the Security section or a second Security heading; firm rules' continuation lines; invisible
  characters; the user's settings reaching the child; the rollback race; a flagged summary reaching the next
  session's context and a commit message; an orphaned child when the supervisor is stopped; the deadline
  counted in sleeps; a run killed mid-commit or mid-merge blocking every snapshot (markers, put right by the
  next run); early skips starting a run after every reply (now an hour's back-off and a notice); shell-mode
  lines in the extract; an 8 s count on a 30 MB transcript (tool results skipped before decoding: 0.03 s);
  a Stop hook timeout; hook input over 64 KB; `\\` in queue ids; manual hold staleness (two hours, refreshed by
  a second hold). Left as is: a rule rewritten in place into a different one is a revision the check cannot
  tell apart (reported with its undo); queued items for a project the user never reopens stay listed.
- Worker cleanup (the user's request, 2026-11-20): a watchdog process beside the child stops it if the
  supervisor dies without cleaning up (SIGKILL, a crash) or a minute past the deadline, and ends with the
  child; the child's PID, start and folder are recorded, and each session start, run and `status` stops a
  leftover child (only when `ps` shows its command line naming the run folder; Git Bash cannot, so there it
  ends at its turn limit) and removes a leftover temp folder, also with background updates off; never while a
  live run holds the lock. Tests: the supervisor SIGKILLed, a leftover child, a PID that is not ours, a live
  run's folder, cleanup with updates off.
- Release review (2026-11-20, a second independent agent, of the fixes and the worker cleanup): fixed a
  regression where a firm rule could be appended to, have a line added under it, or (outside a bullet) be
  reworded, and an added HTML comment could hide rules; the settings' env written where the child could read
  it (now outside the run folder, removed before the child starts); pasted images making the Stop hook slow
  (base64 dropped before decoding; the count stops at the threshold); a planted `/tmp/CLAUDE.md` reaching the
  child (confirmed headless: Claude Code reads CLAUDE.md files up from the child's folder; run folders now go
  in `XDG_RUNTIME_DIR`, a per-user `TMPDIR`, or `~/.cache/jeeves`, never `/tmp`; `~/.claude` refuses edits even
  with an allow rule, checked); a secret in an earlier commit blamed on the run's own (a scan before the run
  decides); a dying watchdog removing a new run's records; `hold` at the Bash tool's two-minute limit (now
  100 s); a second `/jeeves:update`'s release freeing the first's hold (holds counted); more invisible
  ranges (bidi isolates, tag characters); multibyte-safe summary cut; a secret-looking queued item withheld;
  a SessionStart timeout. Third live run: 18 s, $0.24, a lesson written past the stricter checks.
- Second live run after the fixes: 17.6 s, $0.24; an injected "ignore previous instructions" in a quoted
  README was reported, not written.
- Transcripts and hook input are read with `perl` and JSON::PP; without it the feature stays off.
- Live end-to-end (real `claude -p` child, scratch data repo with a copy of the real global rules, a made-up
  session): 21 s, $0.35 list-price; the lesson became a rule in the right section and was pushed; a quoted
  page posing as the user (wrong nonce) was reported in the queue, not written; the project item was queued.
- macOS checked by the user (2026-11-20): a `nohup ... &` child started by a real Claude Code Stop hook
  (headless, scratch plugin) outlived the hook and `claude`, and the hook input carried `transcript_path`.
- Not checked: the detached run on Windows (CI covers the supervisor's logic with a stand-in `claude`); on Windows the child gets the folder as `C:/...` in its prompt
  and as `//c/...` in its permission rules (the docs: Windows paths are matched in `/c/...` form), not yet run
  against a real Windows Claude Code.

## Anthropic directory submission (planned 2026-11-20)

The user asked whether Jeeves is listed on Anthropic's marketplace and chose to plan the preparation as a
phase. Submitting stays the user's own act, on their direct command.

Findings (2026-11-20):
- Not listed: `anthropics/claude-plugins-official`'s `marketplace.json` has no `jeeves` (gh, 2026-11-20).
- No "official" listing for third parties: that repo's README says `/plugins` holds plugins by Anthropic team
  members, its `marketplace.json` "is being phased out for third-party plugins", and pull requests are not
  monitored. Third-party plugins go through the developer portal, claude.ai/directory/manage ("Plugin
  bundle", pointed at the GitHub repo); a web search names `claude-community` as where they land (not in the
  docs read). Sources: https://claude.com/docs/plugins/submit,
  https://claude.com/docs/plugins/pre-submission-checklist,
  https://github.com/anthropics/claude-plugins-official (README).
- Checklist outcomes: **blocks**, **held** for a reviewer, or passes. A security scan runs per commit
  (undisclosed data transmission, hidden code, changes to Claude's permission settings); a first submission
  that fails it is rejected. The README becomes the listing text and "must describe everything the plugin
  runs, sends, or fetches". By default a reviewer publishes each version; the directory follows a branch or
  tag; `version` must rise with each release. 10 submissions per organization per day.
- Already met: MIT `LICENSE` and `license`; README well over 40 words; public repo; name `jeeves` valid and
  not reserved; only text files and two JPEGs (largest 2.2 MB, under 5 MiB; no other file over 256 KiB);
  documented hook events with paths from `${CLAUDE_PLUGIN_ROOT}`; no credentials; readable source.
- Likely **held** (not blocking): hook scripts source `lib/lib.sh` and call other plugin files. The quoted
  hook path `"${CLAUDE_PLUGIN_ROOT}"/bin/...` with an argument is probably accepted; only the portal's
  Validate confirms it.
- Not checked: which plans may submit (the "who can submit" page was not read).

Work to build:
1. A README section, "What Jeeves runs, sends and changes": every network call (git fetch and push of the
   data repo, the daily curl of the marketplace pin on GitHub, `gh` in setup) and every write under
   `~/.claude` (CLAUDE.md, agents, commands, skills; `settings.json` and keybindings on restore or with
   `backup_settings=yes`).
2. `claude plugin validate .` and `--strict` pass; no file trips a "held" rule (256 KiB, binaries,
   `.DS_Store`).
3. The directory follows release tags, not `main` (recommended: matches the signed-release pin).

Open decisions (the user's):
- Which account and plan submits.
- The publish setting (default: a reviewer approves every version).
- Whether `tests/`, `upcoming-changes.md` and `docs/images/jeeves-butler-full.jpg` ship in the plugin folder
  (the plugin is the repo root, so they do now).

Then, the user only: Validate and Submit in the portal.

Built (2026-11-20, branch `feat/directory-prep`): README section "What Jeeves runs, sends and changes" (every
network call, every write under `~/.claude`, the restore that brings back a device's `settings.json` with its
permissions and hooks, and setup's optional deny rules). `claude plugin validate --strict` passes for
`plugin.json` and `marketplace.json` (note: `claude plugin validate .` reads only the marketplace when both
exist). 48 files; none over 256 KiB but the images; no OS junk, symlinks or submodules; `MANIFEST.sig` is
ASCII. Kept as is, to be **held** for a reviewer: hook scripts source `lib/lib.sh` and run other `bin/`
scripts (the checklist: "The validator follows only plain shell scripts"); folding them into one file per
hook would undo the shared library for a hold that is not a block. Hook paths are unquoted-literal rules in
the checklist ("write each path in full from `${CLAUDE_PLUGIN_ROOT}`"); ours is quoted with an argument,
which the portal's Validate will confirm. The open decisions above are the user's at submission time.

Submission steps (for the user): claude.ai/directory/manage, Plugin bundle, repo `WinuxNomacs/jeeves`,
folder the root, track the tag `v1.4.1` (move it with each release), Validate, fill in data handling (no
personal data sent anywhere but the user's own GitHub repo; nothing retained by the author), Submit.

## One encryption command (requested 2026-11-19)

Request: the `/` menu lists all nine Jeeves commands, `/jeeves:lock` and `/jeeves:unlock` included on a
device whose data repo was never encrypted; list only what can be used, to clutter the menu less.

Checked (Claude Code docs, 2026-11-19, code.claude.com/docs/en/skills.md): a plugin cannot show or hide a
skill by state. The only per-skill switch is `user-invocable: false` (fixed); `skillOverrides` does not apply
to plugin skills. So the menu gets shorter only with fewer commands. The user's choice: merge the three
encryption commands; setup, restore and cleanup stay as they are (folding them into `/jeeves` would make
them model-invoked, losing the "destructive skills are user-only" safeguard).

Design (proposed):
- `/jeeves:encryption` replaces `encrypt`, `lock` and `unlock`. It runs `jeeves-encrypt status` first and acts on the
  state: off: offer to encrypt (today's encrypt flow); on and locked: unlock; on and unlocked: ask "Change
  the passphrase", "Lock this device" or nothing; plain history left on GitHub: offer its removal (today's
  `PLAIN_HISTORY_MSG` case). `disable-model-invocation: true` stays; `allowed-tools` the union of the three.
- Every message that names `/jeeves:unlock`, `/jeeves:lock` or `/jeeves:encrypt` (`lib/lib.sh`,
  `lib/crypt.sh`, `bin/jeeves-snapshot`, `-scan`, `-risk`, `-crypt`, `-encrypt`), the setup, update and
  review skills' references ("follow /jeeves:encrypt steps 1 and 2", "/jeeves:unlock steps 1 and 2"), the
  README and the tests name the new command.
- 9 commands become 7. Version 1.4.0 (the user's choice: the data repo and saved state do not change);
  CHANGELOG says which command replaces the three.
- Tests: the existing encryption tests keep passing; greps of skill text updated; a check that no script
  message names a removed command.

Decided with the user (2026-11-19):
- Merge only the three encryption commands (setup, restore, cleanup stay).
- Name: `/jeeves:encryption` (skill folder `skills/encryption/`; `encrypt`, `lock`, `unlock` removed).
- Version 1.4.0, not 2.0.0, though a typed command goes away.

## Update notice (requested 2026-11-19)

Request: Jeeves checks for a newer release and tells the user to update. Today nothing does: auto-update is
off for third-party marketplaces, so an install stays on its version until the user runs
`claude plugin marketplace update jeeves` and `claude plugin update jeeves@jeeves`, or turns on auto-update
in `/plugin` > Marketplaces.

Design:
- The session-start hook (`jeeves-snapshot --hook`), at most once a day, runs `git ls-remote --tags` on
  `https://github.com/WinuxNomacs/jeeves` (public, anonymous, the hook's existing 30 s deadline), takes the
  highest `vX.Y.Z` tag and compares it with `.claude-plugin/plugin.json`'s version. Newer: one line naming
  both versions and the two commands. Otherwise silent.
- The time of the last check in `~/.claude/jeeves/update-check` (outside the plugin and the data repo); a
  file that does not read counts as never checked. Network failure, no git, bad output: silent, retried the
  next day. Skipped on a development checkout (`dev-checkouts`) and with `JEEVES_UPDATE_CHECK=off`.
- Tag names are parsed strictly (`v` digits `.` digits `.` digits); anything else is ignored, so a
  repository's output never reaches the message unchecked.
- `/jeeves:setup` mentions auto-update in one line in its report (`/plugin` > Marketplaces); it changes no
  settings itself.
- README ("Updates"), repo CLAUDE.md (the hook's network calls), tests with a stand-in `git ls-remote`:
  newer, same, older, failing, junk tags, checked today already, dev checkout.

What can fail (runs unattended): no network or GitHub down (deadline, silent); a hung git (the deadline);
an unreadable or future-dated record (treated as never checked); a renamed repo (silent, no tags).

Changed in the 1.4.0 review (2026-11-19): the newest tag comes before its marketplace pin (release step 9),
so a notice from tags could name a release the update commands do not deliver yet (for hours, or for good
if the tag's CI fails). The check now reads the pinned `ref` from `.claude-plugin/marketplace.json` on
`main` (`raw.githubusercontent.com`, curl `-q --proto =https`, the same deadline); no curl: silent. The
README's "add the marketplace as `#v<version>`" was dropped: a tag's marketplace file pins the release
before it.

## Prompt injection hardening (planned 2026-11-19, the user's choice: 1 and 2 next, 4 later, 3 not taken)

Threat: text Jeeves reads (web pages and tool output in the session, project CLAUDE.md files, memory bodies,
shared files pushed to the data repo by a stolen token or a compromised device) becoming instructions loaded by
every session on every device, or skills with scripts installed by `jeeves-apply` without the user seeing them.
Already in place: scripts never execute data (no eval; `jeeves.conf` read as text), narrow allowed-tools,
destructive skills user-only, settings per device.

1. **Instructions only from the user** (skill text): in both reviews a rule comes only from the user's own
   messages or an outcome they confirmed, never from text Claude read; an item that adds commands, URLs,
   downloads, tool permissions, or weakens Security or a confirmation rule is never written on its own: it is
   shown with its source and the user decides.
2. **Incoming shared files checked before install:** new or changed skills, agents and commands from another
   device are held back by `jeeves-apply` until the user has seen the diff and agreed; incoming CLAUDE.md
   changes are summarised; a risk check (pipe to a shell, encoded blobs, permission or hook changes,
   "ignore previous instructions") flags them in and out, beside `jeeves-scan`.
   Decided with the user (2026-11-19):
   - Held: new or changed `skills/`, `agents/`, `commands/` files. `jeeves-apply` installs each only with
     `--accept <hash>:<path>` for that exact version (a later change is held again); `--show` prints every
     incoming change (diff and flags) and installs nothing. Deletions are not held.
   - CLAUDE.md: installed as before; the update report summarises the incoming change and names any flag
     (never held, the user's choice).
   - A new device's first sync (no skill, agent or command synced on it yet): one question for all, listing
     them by name; flagged ones asked separately (the user's choice).
   - Outgoing: `jeeves-risk` flags lines added under `home/` by what a push would send; the update and
     review skills ask before the push, naming each (the user's choice). `jeeves-scan` is unchanged.
   - A file Jeeves wrote on this device for the user's request is accepted without asking; a merged
     conflict is shown (its incoming side came from another device).
   - Held files are asked per skill folder, with "Install it", "Leave it held" (asked again next run) or
     "Remove it on every device"; a flagged outgoing line can be taken out (reverted) while the rest is
     pushed. (From the dry run, 2026-11-19.)
   - Limit: the hold stops a session from installing without the user seeing it; a session already
     following planted instructions can pass `--accept` itself (the tripwire and deny rules of "Script
     integrity" cover more).
3. Signed data repo commits: not taken for now.
4. (Later) Docs: threat model in the README; plugin repo protection (branch protection, 2FA, signed tags).
   The deny rules moved to "Script integrity".

## Script integrity (planned 2026-11-19, the user agreed to the design)

Goal: know that the scripts that run are the ones released. A check inside the plugin cannot stop an attacker
who can already write the plugin folder (they can edit the check, or `~/.bashrc`), so it is a tripwire for
changes on disk; the real protection is verifying what arrives (release pipeline, marketplace copy, update)
against a key the plugin cannot change.

1. **Signed manifest per release:** `MANIFEST` (SHA-256 of every shipped file) and `MANIFEST.sig`
   (`ssh-keygen -Y sign`, namespace e.g. `jeeves-release`) made on the release branch; the signature needs
   the user's SSH key at release time (Claude prepares the rest).
2. **Trust anchor outside the plugin:** `~/.claude/jeeves/allowed_signers`, written by `/jeeves:setup` from
   the key fingerprint printed in the README; an update that rewrites the plugin cannot swap the key.
3. **`jeeves-verify`:** signature, every file's hash, no extra files; run after install or update and on
   demand. Clone installs on a tag may check `git verify-tag` + a clean `git diff <tag>` instead.
4. **Tripwire at session start:** the same check from the hook (measured ~13 ms for hashes + signature on
   Linux, 28 files; Windows Git Bash not measured yet); on a mismatch it warns loudly and runs nothing else
   from the plugin. A development checkout (not on a signed release) is marked as such and skips it.
5. **Deny rules** so a prompt-injected session cannot edit the installed plugin: `Edit`/`Write` on
   `~/.claude/plugins/**` (and, in this repo, `bin/`, `lib/`, `hooks/` for sessions other than development);
   suggested by setup, written only with the user's yes (they change global settings). Bash writes are not
   fully covered: the tripwire catches those afterwards.

Decided with the user (2026-11-19):
- **Key:** a new release-only ed25519 key (not `~/.ssh/id_rsa`, which also pushes to
  GitHub and signs tags), created by the user in their own terminal; no passphrase, and Claude signs each
  release the same way (the user's decision at the first signed release, 2026-11-19; the key is backed up in
  the user's vault); its public key ships as
  `release-key.pub`, its fingerprint is printed in the README, and setup writes `allowed_signers` from it
  on first install (trust on first use; later updates cannot swap it).
- **Mismatch:** a loud notice at session start, and the hook runs nothing else; `/jeeves:update` and
  `/jeeves:review` stop before syncing until `jeeves-verify` passes again.
- **Development checkout:** only a folder the user lists in `~/.claude/jeeves/dev-checkouts` (outside the
  plugin) skips the check, silently; any other unsigned copy warns.
- **Deny rules:** setup shows them and writes them to the global `settings.json` only on the user's yes.
- **Windows timing** (measured 2026-11-19, Git Bash, 41 files, 3 runs): hashes 205-226 ms, signature
  100-120 ms. So the signature is checked again only when `MANIFEST` changed since the last pass (recorded
  outside the plugin); the hashes run each session.
- Before the first signed release, nothing warns: no `allowed_signers` means the check is not set up. With
  `allowed_signers` present, a missing `MANIFEST` warns (a stripped or downgraded copy).

- **Installs on releases only** (the user's choice): new installs took `main`, whose later commits would
  not match the signed `MANIFEST`. The plugin entry in `marketplace.json` becomes
  `{"source": "github", "repo": "WinuxNomacs/jeeves", "ref": "vX.Y.Z", "sha": "<tag commit>"}` (the docs,
  code.claude.com/docs/en/plugins/marketplace-reference.md, 2026-11-19: `github`, `url` and `git-subdir`
  sources take `ref` and `sha`; a relative `./` source takes neither). After each tag, one commit on `main`
  moves the pin; pushes to `main` reach nobody until a release. This machine's in-place install would then
  fetch from GitHub too, so development here needs a separate local dev marketplace: to work out and test
  (it changes the user's plugin setup, so each command is confirmed first). The docs do not say whether the
  cache copy holds `.git`, or whether `plugin update` re-fetches a ref pinned without `sha`: check on a real
  install in the rehearsal.

Build order: (1) `jeeves-verify`, the release skill's manifest and signing steps, the session-start check,
the skills' stop, setup's anchor and deny rules, tests; (2) at the first signed release, with the user: the
release key, the pinned marketplace entry, the dev marketplace here.

## Preference requests (planned 2026-11-19)

The user's design: `/jeeves:update <preference>` captures the intent of the user's preference and commits
it in the way that works best for the whole ruleset; CLAUDE.md must only ever improve Claude's operation,
never hinder it. No approval of the wording (any later run may refine it). If the rules already cover the
request, Jeeves explains which rule does and changes nothing. Built in `skills/update/SKILL.md` (change
mode step 2: weigh the intent against the file as review step 3 does; revise in place, add, or explain;
ask only to weaken Security), README's `update` section. Patch release (skill text only).
Checked by an agent applying the text to three sample requests; its findings were fixed: a covered rule the
user asks for again is sharpened when it was evidently missed, a direct request is never dropped as
default behaviour, mixed requests are split, scope defaults to global, the request is its own `pref:`
commit (found by message after a cleanup), and the report quotes the rule written and how to undo it.
Suite 105 passed (2026-11-19).

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
- Data integrity (2026-11-18, from a review of the code and skills): snapshot records what it copied and
  committed (checked before the commit, decrypted), stops on any failed copy or backup, leaves a file still
  being written; apply only from a clean `main`, re-checks a file before replacing it, records only a
  verified copy; `--force <path>`; conflict markers never installed, recorded or pushed unless named; the
  merge driver fails with a marked note; a vanished tracked folder held until `--allow-delete`; a machine ID
  in `hosts/<host>/` (`--replace-host`); `* -text` for new and existing data repos; an atomic stale-lock
  takeover, and the lock in cleanup, encrypt and restore. Code in `lib/lib.sh`, `bin/`; update skill step 2
  to 5; README "How the data stays whole". 97 tests in `tests/run.sh` (19 new, each injecting its failure,
  each seen failing on the old scripts). Details below under "Data integrity".
- Memories feed the review, opt-in settings backup, tagline (2026-11-18, the user's requests):
  `bin/jeeves-memories` (`list`, `record`) and the shared record `hosts/.memory-review`; the update skill's
  review reads every device's unreviewed memories (25 files per run) and states its purpose;
  `backup_settings=yes` in `jeeves.conf` (off by default; old copies removed once, kept in history);
  `conf_value` in `lib/lib.sh`; README tagline "Your Claude Code, a little wiser with every session." and
  an introduction that leads with the knowledge loop; manifests likewise. 101 tests in `tests/run.sh`.
  Details below under "Memories, opt-in settings backup, tagline".
- Release 1.1.0 (2026-11-18): both phases above. Review by an independent agent and my own pass; fixed on
  `fix/release-review` (104 tests): a copy refused by the byte-for-byte check was committed unchecked by the
  next snapshot (leftover home/ copies now go through the same check); the user's `core.excludesFile` kept
  files out of commits while they were recorded as synced (`git_repo` clears it); a host name starting with a
  dot (`.memory-review`) is refused; a file deleted while copied is left for the next snapshot; `backup_settings`
  is case-insensitive; a review record left uncommitted no longer blocks snapshots; the update skill records
  reviewed memories on `main` and falls back to the session when the memory listing fails. Not fixed: new
  files under a symlinked shared folder are not synced (Later 2); `Bash(jeeves-snapshot --force:*)` in the
  update skill's allowed-tools also matches `--allow-delete` and `--replace-host` (the skill runs those only
  after the user chose); a `merge=union` attribute for `hosts/.memory-review` (the skill merges it by hand).
  Upgrade rehearsed from v1.0.1's scripts (attributes migrated, settings copies removed once, a CRLF file
  stored byte for byte, a second snapshot silent).
- Autonomous review (2026-11-18): `skills/update/SKILL.md` steps 3 to 5 (decide against the written bar,
  curate both CLAUDE.md files within the guard, ask only on a protected rule or an unclear contradiction,
  `learn:` and `curate:` commits, a report with each revert; `/jeeves:update revert <commit>` in change
  mode). README and the repo CLAUDE.md updated. No script change (a comment in `bin/jeeves-memories`).
  Checked by an agent running the skill against scratch folders (a session with a correction, a new user
  constraint, a private detail, a one-off task; memories repeated across two projects, one contradicting a
  user constraint, one personal; a global file with a duplicate, default-behaviour rules and a missing
  script): everything decided as designed, one question held back, report with reverts. Its seven unclear
  passages were fixed: a memory with an unanswered question stays unrecorded; merging into a protected rule
  keeps its words; a global rule is not removed because a tool is missing on one machine; no guessed reasons;
  memories saved before the first snapshot and recorded `kept (new)`; a sync with global items runs change
  mode in full. The permission prompt it noted is fixed in the next entry.
- Permission prompts (2026-11-18): tested with `claude -p --permission-mode default --allowedTools`, every
  skill command written `cd <data repo> && ...` or `JEEVES_DATA=... jeeves-...` asked for approval (an allow
  rule matches neither past a `cd` outside the project nor past an unlisted variable; the user runs auto
  mode, so it went unseen). Now `lib/lib.sh` takes `--data-repo <path>` as any script's first arguments and
  records the data repo in `~/.claude/jeeves/data-repo` whenever a script is told it (the session-start
  hook, from the plugin option); later bare commands read that record. New `bin/jeeves-git` runs git in the
  data repo. All seven skills pass the option once (`jeeves-status --data-repo`, `jeeves-check --data-repo`
  in setup, `jeeves-memories --data-repo ... list` in the review) and run everything else bare, with
  `jeeves-git` allow rules that end in `*` (no middle wildcard, which can span arguments; `jeeves-git
  reset` still asks). Tests: the record and `jeeves-git` (failing on the old `lib.sh`), the skills' commands
  and allow rules, and a scratch `CLAUDE_HOME` for the whole suite. Checked headless: the update skill's
  commands pass, `jeeves-git reset` is refused. Not covered: Read and Edit of data repo files outside the
  project still ask in the default mode.
- Release 1.2.0 review fixes (2026-11-18): the data repo record is written only from an explicit path
  (`--data-repo <path>` or the plugin option, not a one-off `JEEVES_DATA`), an empty `--data-repo` removes
  it, and a record that can't be written is reported; `jeeves-git` uses `git_repo`'s options (the user's
  global ignore list no longer hides files from it). Skills: allow rules narrowed to the forms used
  (`switch main`, `switch -c pref/*`, `add -- *`, `commit -m *`, `merge --no-ff *`, ...; checked headless:
  `switch -C`, `add -f`, `merge -s ours`, `commit --amend` are refused), the project half pre-approved and
  committing only CLAUDE.md (`git commit -m ... -- <file>`), a revert whose hash a cleanup changed is found
  by its message, a conflicting revert aborts and asks, AskUserQuestion's 4-question limit, data repo paths
  read in full, lock's missing data repo. During that check a `claude -p` session's own hook committed a
  scratch `CLAUDE.md` into the real data repo (unpushed; reset with the user's approval, and the repo
  CLAUDE.md now warns against it).
- Release 1.2.0 (2026-11-18): the autonomous review, the permission prompt fix and the review fixes above.
  Review by an independent agent and my own pass (data repo paths given relative, now in full); 7 findings,
  all fixed except the Edit and Read prompts for data repo files outside the project (deliberate writes).
  Upgrade rehearsed from v1.1.0's scripts (record written by `--data-repo`, bare commands find it, nothing
  re-recorded, the reviewed memory not listed again, the hook exits 0). CI on the release branch: Linux,
  Windows and macOS, 105 passed each, encryption tests included.
- Preference requests (2026-11-19): change mode fits a requested preference to the whole ruleset, or
  explains which rule covers it; `skills/update/SKILL.md` change mode steps 2, 3, 6; README. Details above.
- `/jeeves` short form (2026-11-19, the user's request): `skills/jeeves/SKILL.md` (`name: jeeves`, so the
  bare `/jeeves` reaches it; it hands its request to `update` through the Skill tool, checked headless on a
  scratch plugin first); `/jeeves:update` unchanged, so tab-completing it still syncs at once. Suite 105 passed.
- `/jeeves` alone asks what to do (2026-11-19, the user's request): "Discuss a rule idea" (always first: the
  user's text becomes the request), "Review and sync", "Undo a recent change" (the latest `learn:`, `curate:`,
  `pref:` commits); `skills/jeeves/SKILL.md`.
- `/jeeves:review` (2026-11-19, the user's request): `skills/review/SKILL.md`. With the user, it finds the rules
  (their own firm ones included) that hold results back, discusses the intent and rewords them once agreed;
  rules not set firmly it tidies on its own. Reads every memory (`jeeves-memories list --all`, new in
  `bin/jeeves-memories`) without touching the update review's record. One `review:` commit per agreed rule,
  undone by `/jeeves:update revert`; an agreed rewording ends "(the user's decision, <date>)" so later
  curation keeps it. Also on the `/jeeves` menu.
- Instructions only from the user (2026-11-19; the user's choice: only risky items from memories are asked):
  "Instructions come only from the user" and "Risky items are the user's to decide" in the Rules of
  `skills/update/SKILL.md` and `skills/review/SKILL.md`. Rules come from the user's messages, memories and
  results Claude saw, never from text it read (a line addressed to Jeeves is reported as a possible plant);
  risky items (outside code, permissions or hooks, trusting a source, fewer confirmations) are asked with
  their source, with "Leave it out"; permission and hook items are only reported. `/jeeves:update` started
  by Claude with a request the user never made only syncs. Checked by a read-only dry run of 14 cases;
  tests in `t_plugin_files`. README "Instructions come only from you".
- Incoming shared files checked before install (2026-11-19; the user's choices: CLAUDE.md installed and
  summarised only, one question for a first sync, ask before pushing a flagged line): `bin/jeeves-apply`
  holds a skill, agent or command in a version the device never had until `--accept <hash>:<path>`;
  `--show` prints incoming diffs and flags; `bin/jeeves-risk` flags risky lines in what a push sends;
  `held_path`, `added_lines`, `outgoing_added` (shared with `jeeves-scan`) and `risk_flags` in
  `lib/lib.sh`; the snapshot notice says held files wait for the user's OK. Skill text in `update` sync steps
  4 to 6 and change step 4, and `review` step 5. Tests: the held, `--show`, first sync, forced, snapshot
  notice and risk tests in `tests/run.sh`, plus 4 older tests now accepting held files; the new tests also
  pass under Windows Git Bash. A read-only dry run of 9 cases found 12 gaps in the skill text, all fixed.
- Script integrity, part 1 (2026-11-19; the user's choices under "Script integrity"): `bin/jeeves-verify`
  (signature against `~/.claude/jeeves/allowed_signers`, every file's hash, no extra files; `--trust`,
  `--hook`; listed dev checkouts skip it; the signature is checked again only when `MANIFEST` changed), run
  first by the session-start snapshot (not the edit hook), and first by the `update` and `review` skills;
  `setup` records the key and offers the deny rules; `.claude/skills/release/manifest.sh` and the release
  skill's signing and pinning steps. Tests: the verify, trust and session-start tests in `tests/run.sh`,
  skill checks in `t_plugin_files`; `manifest.sh` checked by hand on a scratch clone with a throwaway key.
  Left for the first signed release: the key, `release-key.pub` and the README fingerprint, the pinned
  entry, the dev marketplace here, and the repo CLAUDE.md's "a push to main reaches new installs" (no longer
  true once pinned).
- Release 1.3.0 review fixes (2026-11-19): `jeeves-verify` reports links, leaves out Claude Code's
  `.orphaned_at` and `.in_use/` (seen in a real cache copy, which otherwise matched git's file list exactly,
  no `.git`) and a worktree's `.git` file, takes `~/` in `dev-checkouts`, and without OpenSSH 8.1 says so
  (exit 3) instead of a false mismatch; the edit hook runs the check too; names with control characters are
  not taken; the review skill's allowed-tools cover its apply and revert steps. Upgrade rehearsed: 1.2.0's
  scripts on two scratch devices, then the new ones (held, flagged, accepted). Suite 118 passed, the changed
  tests also under Windows Git Bash. CI on the release branch then caught an older race in `take_lock`
  (Windows): a waiter removed a fresh lock after its holder's predecessor exited; fixed (removed only if it
  still names the dead PID), with a deterministic test. Suite 119 passed.
- Release 1.3.0 (2026-11-19), the first signed release: `/jeeves`, `/jeeves:review`, preference requests,
  instructions only from the user, incoming shared files held until accepted, `jeeves-risk`, and the plugin
  checked against its signed `MANIFEST` (release key `SHA256:tXt+0kjefgqd/mSjv7xDjFj6/u9LxpK+vv9gZhxgZy8`).
  Tagged `v1.3.0` (`0b91dfb`) on the user's word.
- First signed release, the rest (2026-11-19): the marketplace entry pinned to `v1.3.0` and its commit (a
  `github` source; test in `t_plugin_files`); development loads the checkout with `CLAUDE_CODE_PLUGIN_DIRS`
  in the device's settings `env` (no dev marketplace needed: checked headless that it replaces the installed
  copy, from settings `env` too) plus `~/.claude/jeeves/dev-checkouts`; such a load gets no plugin options,
  so `--data-repo` ignores an unfilled `${user_config...}` placeholder (test in the bare-commands test).
  Set up on the user's development machine with their OK. README and repo CLAUDE.md updated.
- README blurb (2026-11-19, from new.md): a callout below the butler image: work as usual, run
  `/jeeves:update` now and then, Claude Code works with what it learned from the next session on (the
  user's placement). "From the next session", not "right away": checked headless (`claude -p`, one process,
  two turns) that a project `CLAUDE.md` changed by a Bash command mid-session is not in context on the same
  turn nor the next. Docs only: no rehearsal; installs get it with the next release.
- One encryption command (2026-11-19): `skills/encryption/SKILL.md` replaces `encrypt`, `lock` and
  `unlock` (the `/` menu: 9 Jeeves commands to 7); it goes by `jeeves-encrypt status`. Every message in
  `bin/`, `lib/`, the setup, update and review skills, README and tests names `/jeeves:encryption`; checks in
  `t_plugin_files` (old names gone, seen failing with one planted). Skill text read through by an agent
  against nine states; its findings fixed. Suite 119 passed.
- Update notice (2026-11-19): `bin/jeeves-update-check`, run by `jeeves-snapshot --hook` at session start
  (not after an edit); `is_dev_checkout` in `lib/lib.sh`; `JEEVES_UPDATE_CHECK=off` for the whole test
  suite. Test `t_the_update_notice_names_a_newer_release_once_a_day` against a local tag repo (newer,
  same, junk tags, daily limit, future and unreadable records, off, dev checkout, unreachable, the hook;
  the hook check seen failing without the wiring). Checked once against GitHub. README, setup's report,
  repo CLAUDE.md. Suite 120 passed.
- New splash image (2026-11-19, from new.md): the user's new pair (an elderly butler with a monocle, on
  green) replaces `docs/images/jeeves-butler.jpg` (1366x675, in the README) and `jeeves-butler-full.jpg`
  (2912x1440) under the same names; README alt text updated. The full image carries Google's AI content
  credentials (C2PA), no personal data.
- Release 1.4.0 (2026-11-19): `/jeeves:encryption`, the update notice, the new splash image; review fixes
  (the notice follows the marketplace pin, `jeeves-verify` reads a last line with no newline, no question
  after a plain unlock, the test runner fails on a missing test function).
- /jeeves hand-off fix (2026-11-19): `skills/jeeves/SKILL.md` tells the user to type `/jeeves:review`;
  `skills/restore/steps.md` and `skills/encryption/steps.md` hold those skills' steps, read by them
  (`${CLAUDE_SKILL_DIR}`) and by setup (`${CLAUDE_PLUGIN_ROOT}`). Tests in `t_plugin_files`: no skill
  invokes or follows a user-only skill (seen failing on the old menu), the step files exist and are read,
  no step file uses a substitution. Checked headless: `/jeeves:restore` read its `steps.md` and listed the
  backups.
- Release 1.4.1 (2026-11-20): the hand-off fix; the README section "What Jeeves runs, sends and changes"
  (corrected by the release review: the automatic cleanup's force-push, pushes from encryption and setup,
  the SSH probe, offered installs, where the sync record and key live, review's project edits, update's
  memories); setup goes on unencrypted when the encryption steps stop, and sends a locked clone to
  `/jeeves:encryption` then setup again (tests in `t_plugin_files`); `rehearse.sh` takes a relative path,
  removes its worktree on any exit and exits 1 on a failure. Checked: `${CLAUDE_PLUGIN_ROOT}` is filled in
  in skill text and Read opens plugin files without a prompt (scratch plugin, headless). Not fixed, minor:
  `lib/crypt.sh` runs `$JEEVES_ASKPASS` unquoted (a test hook; a path with spaces breaks it); the
  user-only hand-off test misses "invoke/run `/jeeves:X`" wordings.
  Tagged `v1.4.1` (CI green on all three OSes) and pinned in the marketplace (`fb537d7`); a fresh install
  from GitHub gets 1.4.1 and `jeeves-verify` passes.
- Release 1.5.0 (2026-11-20): background updates (below), the worker cleanup, both independent reviews'
  fixes, the friendlier splash image. Rehearsed from 1.4.1 (plain and encrypted, two devices; background
  updates off and undecided after the upgrade). Merged on `main` (8050958), never tagged: by the user's
  decision (2026-11-21) nothing is released before 2.0, which carries it.
- Background updates (2026-11-20; planned and measured with the user, "Background auto-update" above):
  `bin/jeeves-auto` (Stop hook, session-start notices, the run, the run lock, queue, hold and release),
  `skills/autoupdate/`, the "Auto mode" section and the hold and queue steps in `skills/update/SKILL.md`,
  the `JEEVES_AUTO_CHILD` guard in `jeeves-snapshot` and `jeeves-hook-edit`, `hooks/hooks.json`. Tests in
  `tests/run.sh` (16, with a stand-in `claude` that misbehaves on request: hang, fail, usage limit, removed
  Security line, removed or reworded rule, renamed heading, risky line, secret, another device's change
  meanwhile and on the same line, a dead remote, a held lock, odd hook input, forged markers). README
  "Background updates" and the disclosure section; repo CLAUDE.md.

- Go spike (2026-11-20; "Go rewrite: spike" above, its results, and the user's "go"): `go.mod`,
  `cmd/jeeves/`, `internal/lib/`, `internal/status/`, `bin/jeeves-status` (launcher) and `lib/status.sh`
  (bash version), `.claude/skills/release/build-go.sh`, `go-builds.sha256`, `time-status.sh`; CI's Go
  step; tests: Go unit tests, and the suite's parity and launcher-fallback tests. Also fixed: the locale
  order of tracked paths in bash, and CI's Windows time limit (45 minutes).

- Release 1.4.2 (2026-11-21; "Release 1.4.2" above): the line-break path fix only, from `v1.4.1`; review
  (it found the link hole, recorded above), suite 121 passed, REHEARSAL OK, CI green on three OSes; tagged
  `v1.4.2` on 91ef89f by Claude (the user's authorization for this release), merged into `main` keeping
  `main`'s files, marketplace pinned; a fresh scratch install gets 1.4.2 and `jeeves-verify` passes.
- Go port Phase 0 (2026-11-21; "Go port, Phase 0" above): `lib/launch.sh`, the wrapper in
  `bin/jeeves-status`, `dist/` committed, `build-go.sh --check`, release steps and rehearsal checks, README
  and CHANGELOG; tests in `tests/run.sh` (parity, fallback, crash, the launcher's folder) and `go test`.
  Full suite 150 passed on all three OSes in CI. Not released (2.0 is).
- Go port Phase 1 (2026-11-21; "Go port, Phase 1" above): `git`, `scan`, `risk`, `check`, `memories`,
  `update-check`, `github` and `init` in `internal/<command>/` (shared code in `internal/lib/`: `run.go`,
  `crypt.go`, `lock.go`, `outgoing.go`); their bash versions and `status`'s removed once the parity tests
  passed on all three OSes, and the launcher names where Jeeves runs when no build fits. `update-check`
  kept curl (net/http measured at +4 MB a build). The parity tests found two bash bugs, fixed in both:
  line 0 under a heading holding a `+word`, and names with a space garbled by git's trailing tab. Tests:
  the lock shared with the bash scripts, the launcher with and without a bash version, `go test`. CI
  green on three OSes (runs 44927817230 with parity, 44927823930 after the removal). Not released.
- Go port Phase 2 (2026-11-21; "Go port, Phase 2" above): `snapshot`, `apply`, `hook-edit`,
  `restore` and `cleanup` in Go (`internal/lib/record.go`, `remote.go`), their bash versions removed after
  parity on three OSes; the link rule (`internal/lib/links.go`); knowledge files only (memories backed up
  and restored; settings and keybindings no longer); one worker per machine (`internal/worker/`). Tests:
  the link cases, the 2c rewrites, eight worker failure tests and `go test ./internal/worker`. 2e: the versioning commands `jeeves-commit`, `jeeves-merge`,
  `jeeves-revert`, `jeeves-push` (`internal/version/`), `jeeves-git` read-only, the skills rewritten to use
  them. Merged 2026-11-21 (CI run 44943357225 green on three OSes). Not released.
- Go port Phase 3 (2026-11-21; "Go port, Phase 3" above): age built in (`filippo.io/age` v1.3.2,
  `internal/lib/agecrypt.go`), `jeeves-crypt` as git's long-running filter process (`internal/crypt/`, no
  bash version), `jeeves-encrypt` in Go (`internal/encrypt/`, passphrase dialogs in
  `internal/lib/passphrase.go`), its bash version removed after parity on three OSes (CI runs 44945877107,
  44947431241); `lib/keys.sh` keeps only the key store read `jeeves-auto` needs. Tests: the encryption
  tests without the age command line, each step and misstep of `jeeves-encrypt`, the filter's failure
  tests, `go test ./internal/lib ./internal/crypt`. Not released.

## Review text gaps (found by the 2026-11-19 dry run; older text, not this phase's)

1. Update's session review drops a lesson the files already state even when the session shows it was not
   followed; change mode sharpens such a rule instead (step 3 should too).
2. Update's step 2 has no default scope (change mode: global unless it names the project).
3. The firm or protected rule differs: update protects "never", "always", "from now on" and recorded
   decisions; review adds "every" and "only", so it may ask about procedural rules update rewords freely.
4. Update step 3's "keeping every word of the protected one" is undefined when both merged rules are
   protected.
5. A memory that would reverse an unmarked rule the user likely set (a merge or branch policy) is rewritten
   silently by update; it should be asked or left to `/jeeves:review`.

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
- Directory submission: see "Anthropic directory submission (planned 2026-11-20)".
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
- Releases before 1.0.1 stay untagged (2026-11-17, the user: "still very early on"); the procedure checks
  only the previous release's tag.

## Data integrity (done 2026-11-18)

From a review of the code and the skills for paths that lose or corrupt data (two passes: mine and an
independent reviewer's; each finding reproduced in scratch folders or shown in the code). "Recoverable from
git" holds only until the next history cleanup folds it away, so none of these count as harmless.

### Findings

1. Snapshot records the base from the live disk file after committing (`jeeves-snapshot:191`) and ignores a
   failed copy (`:148`): an edit during the snapshot, a torn read or a failed copy leaves base = disk while the
   repo holds the older copy; the next `jeeves-apply` installs the older copy over the newest edit, no backup.
2. Apply ignores a failed copy but records the repo version as the base (`jeeves-apply:247-248`): the next
   snapshot commits the old disk copy over the other device's newer one, which reaches every device.
3. `jeeves-apply --force` (and `jeeves-snapshot --force`) act on every conflict; update skill step 3 runs it
   after merging one file, so a second conflict's disk edit is overwritten (kept only in `overwritten/`).
4. Encrypted repo: a merge driver that cannot read the key exits 1, git marks a conflict holding only our
   side with no markers; update skill step 2 says "`git add` the result and commit".
5. `jeeves-apply` checks neither branch nor merge state: it installs conflict markers or an unmerged `pref/`
   branch into `~/.claude`. Nothing checks for conflict markers before a commit or a push.
6. A vanished tracked folder, or a symlinked one whose target is gone, is recorded as deleting all its files
   on every device (the guard covers only "all shared files gone").
7. A nested `.git` (a skill installed by `git clone`) is committed as an empty gitlink; a nested `.gitignore`
   keeps files out; `jeeves-status` still says in sync.
8. The `hosts/<host>/` backup is removed and copied again with no check: a failed copy commits the deletion
   of memories from the latest backup.
9. Encrypted repos: `core.checkstat minimal` plus `cp -p` hide a same-size change within the same second;
   a later pull overwrites it without a conflict.
10. `jeeves-init` writes `* text=auto eol=lf`: CRLF files are committed as LF, against the byte-for-byte rule
    (`jeeves-cmd` holds no CRLF file today).
11. `take_lock`'s takeover of a stale lock is not atomic (two holders in 12 of 15 trials with 6 takers); the
    EXIT trap removes a lock it may not own; cleanup (by hand), encrypt, restore and the skills' git commands
    take no lock.
12. Two devices with the same host name share and wipe one `hosts/<host>/`.
13. Lesser: `switch_to_cleaned_history` does not check its bundle before `reset --hard` and `gc --prune=now`;
    cleanup's `undo` can fail silently; a failed write of `.sync-state/hashes` replaces it truncated; apply
    `--force` and restore overwrite even when saving the old copy failed.

### Design (proposed)

- **Record what was committed.** Copy, then hash the copy; that hash is the base. Any failed copy (shared
  files, backups) stops the run: nothing staged, no base recorded, the copies left as leftovers. After the
  commit, each taken path's committed content (decrypted) must hash to its base, or the run stops and says
  so (also catches findings 9 and 10). Same in apply: base only after a verified copy; just before
  overwriting a disk file, its hash is checked again.
- **Guards.** Apply refuses unless the data repo is on `main`, not merging, and `home/` is clean. Snapshot
  and apply refuse a shared file holding conflict markers; `jeeves-scan` refuses them in outgoing commits.
  `--force` takes the paths it applies to (the skill passes the one it merged). The merge driver fails so
  that git stops instead of returning one side, and the skill checks that no file is left unmerged or
  without its markers before it commits.
- **Deletions.** A tracked folder that disappears whole, or is a dangling symlink, is not recorded: the
  snapshot records the rest and prints a notice; `/jeeves:update` asks, and on a yes naming the files runs
  the snapshot with an option that records that folder's deletion.
- **What git sees.** Nested `.git` and `.gitignore`: unchanged (the user's decision). `jeeves-init` writes
  `* -text`; the snapshot replaces an existing `* text=auto eol=lf` line with it once and commits that
  alone. `core.checkstat minimal` kept only if the post-commit check covers it.
- **Same host name.** Each device keeps a random machine ID in `.git/jeeves/` (local) and writes it into
  `hosts/<host>/`; a snapshot that finds another machine's ID there skips the backup and says to set
  `JEEVES_HOST` to a distinct name (setup does the same check before the first sync).
- **Locking.** Atomic takeover (rename a fresh lock into place, then check it holds our PID), release only
  our own lock; cleanup, encrypt and restore take the lock (the auto cleanup inherits the snapshot's).
  The skills' own git commands stay outside it: the snapshot already skips a dirty or merging repo.
- Lesser items fixed as listed: bundle checked, `undo` failure reported, `set_base` write checked, no
  overwrite without its saved copy.

### What can fail (snapshot and apply run unattended from hooks)

Disk full or read-only target (copy, state file, bundle); a file written while it is read; a killed run
(stale lock, leftover temp files); a second run at once; the key store unreadable (merge driver, smudge);
a missing folder or dangling symlink; git's index missing a change. Each gets a test that injects it (stand-in
`cp` that fails or edits the source, read-only folders, a moved key, several takers of a stale lock), seen
failing before its fix.

### As built (2026-11-18)

- `copy_settled` (lib) copies a `~/.claude` file and retries while its hash changes (three tries, then
  "still being written"); `staged_is` checks a staged path against the copy's hash through the clean cache
  (no key read), else `git cat-file --filters`; taken paths are re-added with `git add --renormalize`, so a
  same-size, same-second change is not missed (finding 9).
- The `hosts/<host>/` backup is built in `.sync-state/host.sync-tmp/` and moved into place.
- The machine ID lives in `~/.claude/.jeeves-machine-id` (outside the synced files, so a re-cloned data repo
  keeps it); it goes into `hosts/<host>/.jeeves-machine` only with a backup that holds something.
- On git 2.44+ the merge driver's exit status 129 stops the merge with nothing changed; older git reports a
  conflict whose file carries the "not merged" markers (the test accepts either).
- For key rotation (Later 1): the clean cache (`.git/jeeves/clean/`) is keyed by the plain text's hash only,
  so a new key must clear it, as `encrypt on` does.
- Not changed: the skills' own git commands run outside the lock; nested `.git`/`.gitignore` (the user's
  decision); `.sync-state/overwritten/` stays local and unpruned.

### Decisions (2026-11-18, with the user)

- Plan a fix phase, ahead of anything else, rather than quick fixes.
- A vanished or dangling tracked folder: held until the user confirms in `/jeeves:update`. As built, an
  empty folder that is still there counts as its files deleted (recorded as before); only a missing folder
  or a link that points nowhere is held.
- A skill with its own `.git` or `.gitignore`: left as today (not fully synced, no notice).
- Line endings in existing data repos: the snapshot migrates `.gitattributes` to `* -text` once.
- Same host name: a machine ID per device; the snapshot detects a clash and stops the backup.

## Memories, opt-in settings backup, tagline (done 2026-11-18, the user's requests)

Purpose, as the user put it: `/jeeves:update` is about improving CLAUDE.md over time, to make it meaningful
and effective for the current and future models that maintain it (recorded in `.claude/CLAUDE.md`).

### 1. Memories feed the review

Request: the session review reads only the current session; memories (`~/.claude/projects/*/memory/`) hold
much operational knowledge from other sessions and are never considered for CLAUDE.md. Builds on the
session review (Done 6) and the memory backups in `hosts/<host>/`.

Design (proposed): when the user runs `/jeeves:update`, the review also reads memory files, sorts each
item worth keeping as Global, Project or "stays a memory", words it as CLAUDE.md text (revising an
existing rule in place where one covers it), and offers it in the same AskUserQuestion flow as session
items. A record of what was reviewed keeps the same memory from being offered every run. Open decisions
below.

### 2. Settings and keybindings backup opt-in

Request: back up `settings.json` and `keybindings.json` only when the user turns it on (the backup may be
dropped later). Memories stay backed up by default. Design (proposed): a `jeeves.conf` setting
(`backup_settings=yes`), off by default; snapshot, restore, setup step 4 and the README follow it.

### 3. README tagline

Request (from `new.md`): a tagline at the top of the README that says Jeeves improves Claude Code's
operational knowledge across sessions, with backup as secondary.

### Decisions (2026-11-18, with the user)

- Memories read: this machine's `~/.claude/projects/*/memory/` and every other device's backup in the data
  repo (`hosts/*/projects/*/memory/`).
- A promoted memory is always kept as it is; the review only records that it was promoted.
- A shared record in the data repo lists each reviewed memory's hash and outcome (promoted, declined, stays
  a memory); every device skips those, and a memory that changes is reviewed again.
- Settings and keybindings backup: off by default, `backup_settings=yes` in `jeeves.conf` turns it on. When
  off, the next snapshot removes the existing copies from `hosts/<host>/` (git history keeps them until a
  cleanup folds them) and says so once.
- Tagline: "Your Claude Code, a little wiser with every session."

### As built (2026-11-18)

- The record holds one line per memory hash: hash, outcome, date, host; sorted by hash. It sits in
  `hosts/` (a file, not a host folder) so it is encrypted with the backups; it records hashes only, no paths.
- The review reads at most 25 memory files per run (this machine had 105 on the first count), in list
  order, and records every file it read, so declined items are not offered again until the file changes.
- `record` takes its pairs as arguments, so the skill's `Bash(jeeves-memories:*)` permission covers it.
- An item from a memory goes to Global, or to Project only for the session's own project.

## Autonomous review (built 2026-11-18, the user's request of the same day)

**The request (new.md):** Jeeves learns from the session and the memories automatically, without
prompting the user for each item: it judges what operational knowledge improves Claude Code sessions
globally, and over time removes cruft (statements that do not improve how the current generation of models
works). It asks only in an extreme case of conflicting rules; otherwise it makes the choice that leads to
more professional, accurate, polished and intelligent results. At its core, Jeeves makes Claude Code more
intelligent over time.

**Builds on:** the session review in `skills/update/SKILL.md` (steps 1 to 5) and the memory review
(`jeeves-memories`, `hosts/.memory-review`). Today (checked in the skill text): the review runs only when the
user types `/jeeves:update`; every item is offered through AskUserQuestion, one question per destination
(Global, Project, Memory); only what the user selects is written; unselected items get a follow-up question;
nothing is ever removed from CLAUDE.md except by revising a rule in place.

**Proposed design:**

1. **Decide, then report.** Steps 3 and 5 (ask, follow-up) are replaced by Jeeves's own judgement against a
   written bar: an item is written when it is true, general for its destination, actionable, not already
   said, and changes what a model would do (something a capable model would do anyway is not a rule). The
   report lists every change, one line each (added, revised, removed, where), with the commit to revert it
   (`git revert <commit>` in the data repo; the project's merge commit).
2. **Curation pass (cruft removal).** Each run also re-reads the destination files and removes or merges: rules that repeat another,
   rules that restate default model behaviour, rules about tools or situations that no longer exist, and
   wording that can be shorter without losing meaning. Never removed or weakened automatically: the
   Security section, and rules the user stated as constraints ("never", "always", "from now on"); those are
   the "extreme case" that asks.
3. **When it still asks (the only prompts):** a new item or a removal would reverse or weaken a rule the user
   set; two existing rules contradict and the right one is not clear from the session or memories; text
   would go into a project CLAUDE.md that is private or true only on this machine (it goes to memory instead,
   without asking, when that is clearly where it belongs).
4. **Memories:** decided items are recorded as today (`promoted`, `kept`); a memory superseded by a rule is
   still never changed (decision recorded 2026-11-18: "Always keep it").
5. The skill's "Never lose text" rule stays for merges (sync), where it protects text a device added.
6. Tests: the scripts do not change unless decision 1 adds a hook; the skill text is checked by a run against
   scratch folders (a session with a correction, a duplicate rule, a rule restating default behaviour, a
   user constraint the new item contradicts).

**Decisions (2026-11-18, with the user):**

1. **Trigger:** on `/jeeves:update` only, with no per-item questions (no hook, no offer at session end).
2. **Cruft removal:** automatic, within the guard (never the Security section, never a constraint the user
   stated); every removal listed in the report and revertible from git.
3. **Project CLAUDE.md:** always automatic, like the global file, keeping the rule that nothing private, no
   secret and no personal detail goes there (the repo may be public).
4. **No cap on edits per run** (the user: the first memory review is the big one; later runs come in
   manageable chunks). The limit of 25 memory files read per run stays: it bounds what one run reads into
   context, not what it writes. The first curation pass over an existing CLAUDE.md may also be large; its
   report lists every change, and one revert of that run's commit undoes it.

## 4. Per-device overlay

Mostly obsolete since Generic made `settings.json` and `keybindings.json` per host. Only needed if a shared
file (a `CLAUDE.md` rule, a skill) ever has to differ between hosts: then decide host vs platform scope and
add an overlay folder (`home.<host>/` or `home.<os>/`) merged by `apply`.
