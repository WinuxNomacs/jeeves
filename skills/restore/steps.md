# Jeeves: restore steps

Followed by `/jeeves:restore` and by `/jeeves:setup` (step 4); speak as the skill that read this file says.
The host to restore from is the one that skill gives.

1. Run `jeeves-status` (the skill that read this file has recorded the data repo already) and note its
   `datarepo` line; if it says no data repo is set, stop and point the user to `/jeeves:setup`. Commands
   find the data repo on their own: run the `jeeves-` scripts bare, and git in the data
   repo as `jeeves-git <arguments>`, never with a `cd` into it.
2. `jeeves-restore` prints this host's name and the hosts with a backup. Without a host given (the
   argument of `/jeeves:restore`, or the host chosen in `/jeeves:setup`), ask which one. If that backup has memories (`hosts/<host>/projects/*/memory/`), ask whether to restore them too;
   only projects that exist on this machine (`~/.claude/projects/<key>/`) get theirs.
3. `jeeves-restore <host>` (add `--memories` if wanted). It saves every file it replaces under
   `.sync-state/overwritten/<time>/` and never commits.
4. **Fix what only worked on the old machine** in `~/.claude/settings.json` (keep it valid JSON; Claude Code
   asks before this edit, which is expected):
   - `pluginConfigs."jeeves@jeeves".options.data_repo`: this machine's data repo path (the `datarepo` line).
   - `extraKnownMarketplaces.*.source.path` (a `directory` source): if that folder does not exist here, ask
     whether to point it at a local clone or switch it to the GitHub source.
   - Then list, without changing them, other values that look machine-specific: absolute paths (`/home/…`,
     `/Users/…`, `C:\…`) in hooks, `statusLine`, `env`, permissions, and OS-specific commands. Ask before
     changing any of them.
   In `keybindings.json`, mention Cmd/Ctrl bindings when restoring between macOS and Linux or Windows.
5. Tell the user to restart Claude Code (or run `/reload-plugins`) so the restored settings load, and that
   the next snapshot backs this machine up under its own `hosts/<this host>/`. Report what was restored,
   what was saved, and which values you changed or left for them.
