---
name: jeeves
description: >-
  The short form of /jeeves:update that the user types as /jeeves <rule or preference>: hands the request to
  the update skill, which reviews the session, fits the request to the rules and syncs. Without a request it
  asks what the user would like to do.
argument-hint: "[rule or preference, or nothing for a menu]"
disable-model-invocation: true
allowed-tools:
  - Bash(jeeves-git log *)
---

# Jeeves

Request: $ARGUMENTS

The user typed `/jeeves`, the short form of `/jeeves:update`. To hand a request off, invoke the Skill tool
with skill `jeeves:update` and args set to the request, exactly as given, then follow that skill in full.
The user ran it, so its session review applies.

**With a request above:** hand it off at once; do nothing before.

**Without one:** ask with AskUserQuestion, as Jeeves (a courteous English butler, briefly), "What would you
like to do?", with these options in this order:
1. "Discuss a rule idea": "Tell me a preference or rule in your own words; I'll fit it to your rules, or show
   you the rule that already covers it."
2. "Review and sync": "Review this session and the memories, then sync your preferences on every device."
3. "Undo a recent change": "Revert a rule change from an earlier review or request."

Then:
- **Discuss a rule idea:** if the user already wrote the idea (under "Other" or in a note), hand that off.
  Otherwise ask, in one line, what the idea is, and end your turn; when the user answers, hand their answer
  off as the request.
- **Review and sync:** hand off with no request.
- **Undo a recent change:** `jeeves-git log --format="%h %s" -n 40 main`, keep the commits whose message
  starts with `learn:`, `curate:` or `pref:`, and ask with AskUserQuestion which to undo (the four most
  recent, each labelled by its message in plain words, its hash in the description). Hand off
  `revert <hash>`. If the command fails (no data repo yet) or none are found, say so in one line and offer
  `/jeeves:setup` or nothing.
- **Anything typed under "Other":** treat it as the request and hand it off.
