---
name: jeeves
description: >-
  The short form of /jeeves:update that the user types as /jeeves <rule or preference>: hands the request to
  the update skill, which reviews the session, fits the request to the rules and syncs. Without a request it
  syncs.
argument-hint: "[rule or preference, or nothing to sync]"
disable-model-invocation: true
---

# Jeeves

Request: $ARGUMENTS

The user typed `/jeeves`, the short form of `/jeeves:update`. Invoke the Skill tool with skill `jeeves:update`
and args set to the request above, exactly as given (empty when there is none), then follow that skill in
full. The user ran it, so its session review applies. Do nothing before the hand-off.
