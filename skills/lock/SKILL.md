---
name: lock
description: >-
  Remove the Jeeves data repo's key from this device's credential store (for example before handing the
  machine on); Jeeves then stops syncing here until /jeeves:unlock.
disable-model-invocation: true
allowed-tools:
  - Bash(jeeves-encrypt status)
---

# Jeeves: lock this device

1. Run `JEEVES_DATA="${user_config.data_repo}" jeeves-encrypt status`. `encryption off`: nothing to lock;
   say so and stop. `locked`: already locked; say so and stop.
2. Say what locking does and doesn't do. The key leaves this device's credential store, and Jeeves stops
   syncing here until `/jeeves:unlock` with the passphrase. The plain files stay: `~/.claude` and the data
   repo's folder (delete those separately if the machine is being handed on). Changes not yet pushed stay
   on this device only. Ask with AskUserQuestion: "Lock this device?"
3. On a yes: `cd <datarepo path> && jeeves-encrypt lock`, and report its line.
