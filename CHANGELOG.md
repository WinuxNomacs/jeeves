# Changelog

A new version reaches installed copies only when `version` in `.claude-plugin/plugin.json` changes.

## 0.2.2 (2026-11-17)

- `/jeeves:setup` always asks whether to restore another device's backup, and waits for the answer.
- On a machine without a data repo, the session-start message and `/jeeves:update` point to `/jeeves:setup`
  instead of the update skill cloning the repo itself (which skipped the restore question).

## 0.2.1 (2026-11-17)

- The marketplace gives the plugin source as `./`: older Claude Code versions refused to add the marketplace
  with `"."` (`plugins.0.source: Invalid input`).

## 0.2.0 (2026-11-16)

- **Shared vs per device.** Only `CLAUDE.md`, `agents/`, `commands/` and `skills/` are synced between devices
  (`home/` in the data repo). `settings.json`, `keybindings.json` and memories are backed up per device under
  `hosts/<host>/`, laid out as in `~/.claude`, and never applied automatically. A data repo in the 0.1.0
  layout (`home/settings.json`, `memory/<host>/`) is moved by each device's next snapshot.
- New `/jeeves:setup` (clone or create the data repo, first sync) and `/jeeves:restore <host>` (copy a
  device's backup onto this machine and fix its machine-specific paths), with the commands `jeeves-init` and
  `jeeves-restore`.
- Windows: native Windows hosts are named `<host>-win`; the edit hook takes `C:\` paths; line endings stay LF
  (`.gitattributes`, no `core.autocrlf` on the data repo). Tests pass under Git Bash.
- Shared files whose names Windows or macOS can't hold are not taken, with a message saying why. The
  `.trash/` folders where Claude Code keeps removed skills are never copied.
- `/jeeves:update` covers only the shared files.
- MIT licence; install from GitHub (`claude plugin marketplace add WinuxNomacs/jeeves`); a test workflow for
  Linux, Windows and macOS.

## 0.1.0 (2026-11-16)

- First version: `jeeves-snapshot` and `jeeves-apply` with a three-way, per-device sync record; a
  SessionStart hook and an edit hook for `~/.claude/CLAUDE.md`; memories backed up per device.
- `/jeeves:update`: sync (merging diverged `main` and both-sides changes) or a preference change, then a
  secret scan (`jeeves-scan`) and a push; `jeeves-status` shows what is out of sync.
