# Jeeves: encryption steps

Followed by `/jeeves:encryption` (from "First: the state", with its request) and by `/jeeves:setup` (the
sections it names); speak as the skill that read this file says.

## The passphrase never goes through this chat

The chat is saved in transcripts. `jeeves-encrypt` asks for the passphrase in a window of the OS. Never ask
the user to type it here, and never pass it in a command. If the user writes it in the chat anyway, don't
repeat it; say that it is now in the transcript, and by the state: before `jeeves-encrypt on` has run, to
choose a different one in the dialog; on a locked device, to unlock with it, then **Change the passphrase**;
on an unlocked one, **Change the passphrase** (which offers to fold the history, since the old one is now
known). Commands that open a dialog wait for the user: run them with a 10-minute timeout. A cancelled
dialog, a passphrase too short (under 12 characters) or two entries that differ change nothing: say which,
and offer to try again. Exit status 3 means no dialog can be shown on this machine: show the user the
command `jeeves-encrypt` printed, to run in their own terminal, wait for them to say it is done, then run
`jeeves-encrypt status`. If the state changed as expected, go on with the step that ran it; if not, say
what the status shows and stop.

## First: the state

Run `jeeves-status` (the skill that read this file has recorded the data repo already) and note its
`datarepo` line (on a locked device it says so); if it says no data repo is set, stop and point the user to
`/jeeves:setup`. Commands find the data repo on their own: run the `jeeves-` scripts bare, and git in the data repo as `jeeves-git <arguments>`, never
with a `cd` into it. Run `jeeves-encrypt status`, then go by the state and the request:

- `encryption off`: **Turn on**. With a request (`passphrase`, `lock`): say that the data repo is not
  encrypted, so there is nothing to change or lock, and that `/jeeves:encryption` alone turns it on; stop.
- `encryption locked`: **Unlock**, then go by the state again. With `lock`: already locked; say so and stop.
- `encryption unlocked` with a `history plain` line: **Plain history still on GitHub** first (only an
  unlocked device can fold it, so before a lock is the last chance here), then the request, if any.
- `encryption unlocked`: the request (`passphrase`: **Change the passphrase**; `lock`: **Lock this
  device**); with none, say that encryption is on and this device is unlocked, and ask with
  AskUserQuestion: "Leave it as it is", "Change the passphrase", "Lock this device".

## Tools (before Turn on, Unlock or Change the passphrase)

- `age missing` or `age old`: Jeeves needs age 1.3.0 or later. Run `jeeves-check`; offer its
  `age-install` command (run only after the user agrees). With `age-install none`, explain the official
  download: https://github.com/FiloSottile/age/releases, the archive for this system, and putting `age`,
  `age-keygen` and `age-plugin-batchpass` on the PATH (for example `~/.local/bin`). Once installed, run
  `jeeves-encrypt status` again and go on. If the user declines, say that encryption needs it and stop.
- `keystore none` (Turn on and Unlock only): this device has no credential store. Ask whether to keep the
  key in a file only this user can read (anything that can read that file or its backups can read the key)
  or to set up a store first (Linux: `secret-tool` with GNOME Keyring or KWallet). A file: add `--key-file`
  to the `jeeves-encrypt on` or `unlock` below. A store: stop, and say to run `/jeeves:encryption` again
  once it is set up.

## Turn on

- Sync first: `jeeves-snapshot`, then `jeeves-status`. Anything out of sync, or `main` not `even`: follow
  `/jeeves:update` (without its session review), then come back here.
- Explain before asking: a passphrase lost on every device means the data on GitHub can never be read
  again (a device that is still unlocked can set a new one without the old one). Recommend generating it
  with a trusted password manager and keeping it there. They will type or paste it twice, at least 12
  characters. Also say that turning it on removes the history from GitHub: every earlier commit is folded
  into one encrypted commit, force-pushed with a lease (this device keeps the old history in
  `.sync-state/history-before-cleanup.bundle`). GitHub may keep unreachable copies for a while, which is
  outside Jeeves' control. After the next cleanup replaces that bundle, the old history is gone for
  good. Ask with AskUserQuestion: "Turn on encryption now?", with the options "Yes, encrypt and remove
  the old history permanently" and "Not now".
- On a yes: `jeeves-encrypt on` (dialogs). Once `jeeves-encrypt status` says `unlocked`, run
  `jeeves-cleanup --all`: it snapshots, folds, scans and force-pushes. If the push fails (offline, or
  another device pushed), the encrypted commit stays local: say so, and that `/jeeves:update` pushes it
  later, after which `/jeeves:encryption` offers to fold the plain history again.
- Report what is encrypted, that other devices now need `/jeeves:encryption` with the passphrase to unlock
  (until then they say they are locked and don't sync), and that a new device runs `/jeeves:setup` as usual.

## Unlock

- `jeeves-encrypt unlock` (a dialog): it checks the passphrase and puts the key in this device's credential
  store. A wrong passphrase changes nothing: offer to try again. A passphrase no one remembers: a device
  that is still unlocked can set a new one there (**Change the passphrase**).
- Then `jeeves-snapshot`: it takes the other devices' changes. If it reports newer files, a conflict or a
  diverged `main`, follow `/jeeves:update` (without its session review). Report in one line that this
  device is unlocked and synced, then run `jeeves-encrypt status` and go by the state again, except that
  with no request an `encryption unlocked` without `history plain` ends here: the user came to unlock, so
  do not ask what else to do.

## Plain history still on GitHub

The fold after turning encryption on did not get pushed. Say that GitHub still holds the files as they were
before encryption, in its history. Ask with AskUserQuestion: "Remove the plain history from GitHub now?",
with "Yes, fold it and remove it permanently" and "Not now", saying as in **Turn on** what folding removes
and that this device keeps it in the bundle until the next cleanup. On a yes: `jeeves-cleanup --all`, then
`jeeves-encrypt status` must show no `history` line. If it still does (the push failed: offline, or another
device pushed), say so: `/jeeves:update` syncs, and `/jeeves:encryption` then offers the fold again.

## Change the passphrase

`jeeves-encrypt passphrase` (dialogs). It commits a new `identity.age`. Then `jeeves-scan` and
`jeeves-git push origin main` (a rejected push: `jeeves-snapshot`, then push again once; still rejected,
stop and ask). The key stays the same, so other devices stay unlocked; the new passphrase is needed only for
the next unlock. The old `identity.age` stays in the history, and it still opens with the old passphrase:
say so. When the old passphrase may be known to someone (it was written in a chat, say), offer to fold the
history too (`jeeves-cleanup --all`, asked as in **Plain history still on GitHub**), so GitHub no longer
holds the old `identity.age`; a copy of the repo someone took before that still opens with the old
passphrase, and only a new key would protect against it, which Jeeves does not do yet.

## Lock this device

Say what locking does and doesn't do. The key leaves this device's credential store, and Jeeves stops
syncing here until it is unlocked again (`/jeeves:encryption`, with the passphrase). The plain files stay:
`~/.claude` and the data repo's folder (delete those separately if the machine is being handed on). If
`jeeves-status` says `main ahead`, changes are not pushed yet and would stay on this device only: offer to
run `/jeeves:update` first. If this may be the only unlocked device, the passphrase is then the only way
back in: say so. Ask with AskUserQuestion: "Lock this device?" ("Lock it", "Not now"). On a yes:
`jeeves-encrypt lock`, and report its line.

## Never force-push yourself

Only `jeeves-cleanup` does, with a lease.
