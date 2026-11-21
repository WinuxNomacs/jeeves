#!/usr/bin/env bash
# jeeves-git <git arguments>: runs git in the data repo, so the skills need no `cd` into it (the Bash
# tool's folder is not kept outside the project, and a permission rule does not match past such a `cd`).
# The data repo is found as lib/lib.sh describes; this works on a locked device too.
. "$(dirname "$0")/lib.sh"
ALLOW_LOCKED=1
find_data_repo || { echo "jeeves git: $DATA_REPO_PROBLEM" >&2; exit 1; }
# The same options as the scripts' git_repo, so jeeves-git and jeeves-status agree on what is dirty.
exec git -C "$REPO" -c core.autocrlf=false -c core.safecrlf=false -c core.excludesFile= "$@"
