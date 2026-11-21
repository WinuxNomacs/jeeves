# Jeeves: restore steps

Followed by `/jeeves:restore` and by `/jeeves:setup` (step 4); speak as the skill that read this file says.
The host to restore from is the one that skill gives.

1. Run `jeeves-status` (the skill that read this file has recorded the data repo already) and note its
   `datarepo` line; if it says Jeeves is not set up on this machine yet, stop and point the user to `/jeeves:setup`. Commands
   find the data repo on their own: run the `jeeves-` scripts bare, and git in the data
   repo as `jeeves-git <arguments>`, never with a `cd` into it.
2. `jeeves-restore` prints this host's name and the hosts with a backup. Without a host given (the
   argument of `/jeeves:restore`, or the host chosen in `/jeeves:setup`), ask which one. Only projects that
   exist on this machine (`~/.claude/projects/<key>/`) get their memories; say so before restoring.
3. `jeeves-restore <host>`. It saves every file it replaces under `.sync-state/overwritten/<time>/` and
   never commits.
4. Report which projects' memories were restored, which were not (no such project here), and where the
   replaced copies were saved; new sessions read the restored memories. The next snapshot backs this
   machine up under its own `hosts/<this host>/`.
