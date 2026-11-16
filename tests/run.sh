#!/usr/bin/env bash
# Tests for the plugin's scripts (bin/jeeves-*) and hook files. Each test builds a scratch ~/.claude, a
# scratch data repo and a local bare "remote" in a temp folder, so nothing real is touched.
#
#   tests/run.sh            run all tests
#   tests/run.sh <pattern>  run tests whose name contains <pattern>

SRC=$(cd "$(dirname "$0")/.." && pwd)
SCRATCH=$(mktemp -d "${TMPDIR:-/tmp}/jeeves-tests.XXXXXX")
trap 'rm -rf "${SCRATCH:?}"' EXIT
filter=$1
passed=0 failed=0

export GIT_AUTHOR_NAME=test GIT_AUTHOR_EMAIL=test@example.invalid
export GIT_COMMITTER_NAME=test GIT_COMMITTER_EMAIL=test@example.invalid
export JEEVES_HOST=testhost
# Settings from the environment that runs the tests must not leak into them.
unset JEEVES_DATA CLAUDE_PLUGIN_OPTION_DATA_REPO

# Fresh world: $W/remote.git, $W/repo (the data repo, a clone with home/), $W/claude (CLAUDE_HOME).
setup() {
  W=$SCRATCH/$1; mkdir -p "$W/claude"
  git init -q --bare -b main "$W/remote.git"
  git clone -q "$W/remote.git" "$W/repo" 2>/dev/null
  mkdir -p "$W/repo/home"
  printf '.sync-state/\n*.sync-tmp\n' > "$W/repo/.gitignore"
  echo "rules v1" > "$W/repo/home/CLAUDE.md"
  echo '{"a":1}' > "$W/repo/home/settings.json"
  echo "rules v1" > "$W/claude/CLAUDE.md"
  echo '{"a":1}' > "$W/claude/settings.json"
  git -C "$W/repo" checkout -q -b main
  git -C "$W/repo" add -A && git -C "$W/repo" commit -q -m init && git -C "$W/repo" push -q origin main 2>/dev/null
  export CLAUDE_HOME=$W/claude JEEVES_DATA=$W/repo
  R=$W/repo
}
snap() { "$SRC/bin/jeeves-snapshot" "$@"; }
apply() { "$SRC/bin/jeeves-apply" "$@"; }
# Run the edit hook with the JSON Claude Code sends after a Write or Edit of the given path.
edit_hook() { printf '{"session_id":"x","hook_event_name":"PostToolUse","tool_name":"Edit","tool_input":{"file_path":"%s","old_string":"a","new_string":"b"}}' "$1" | "$SRC/bin/jeeves-hook-edit"; }
commits() { git -C "$R" rev-list --count HEAD; }

# Failed checks are collected, so one test reports every expectation it missed.
check() { # check <description> <command...>
  if "${@:2}"; then return 0; fi
  echo "expected: $1"; FAILS=$((FAILS + 1)); return 0
}

run_test() { # run_test <name> <function>
  case "$1" in *"$filter"*) ;; *) return ;; esac
  local out
  if out=$( FAILS=0; setup "$2"; "$2" 2>&1; exit $FAILS ); then
    passed=$((passed + 1)); echo "ok   $1"
  else
    failed=$((failed + 1)); echo "FAIL $1"; printf '%s\n' "$out" | sed 's/^/    /'
  fi
}

t_nothing_changed() {
  snap >/dev/null; n=$(commits)
  out=$(snap)
  check "no output" [ -z "$out" ]
  check "no new commit" [ "$(commits)" = "$n" ]
}

t_disk_change_committed() {
  snap >/dev/null; n=$(commits)
  echo "rules v2" > "$W/claude/CLAUDE.md"
  check "says committed" grep -q "committed CLAUDE.md" <<< "$(snap)"
  check "one new commit" [ "$(commits)" = $((n + 1)) ]
  check "repo copy updated" grep -q v2 "$R/home/CLAUDE.md"
  check "commit message names the host" git -C "$R" log -1 --format=%s | grep -q "snapshot(testhost)"
}

t_dirty_repo_skips() {
  echo x > "$R/stray.txt"; n=$(commits)
  echo "rules v2" > "$W/claude/CLAUDE.md"
  out=$(snap); rc=$?
  check "says skipped" grep -q "uncommitted changes" <<< "$out"
  check "exit status 1" [ $rc = 1 ]
  check "no commit" [ "$(commits)" = "$n" ]
  snap --hook >/dev/null; check "hook mode exits 0" [ $? = 0 ]
}

t_other_branch_skips() {
  git -C "$R" checkout -q -b feature/x
  out=$(snap)
  check "says not main" grep -q "not main" <<< "$out"
}

t_new_dirs_tracked_synced_excluded() {
  mkdir -p "$W/claude/agents" "$W/claude/skills/mine" "$W/claude/skills/synced/x"
  echo a > "$W/claude/agents/reviewer.md"
  echo s > "$W/claude/skills/mine/SKILL.md"
  echo z > "$W/claude/skills/synced/x/SKILL.md"
  echo k > "$W/claude/keybindings.json"
  snap >/dev/null
  check "agent copied" [ -f "$R/home/agents/reviewer.md" ]
  check "own skill copied" [ -f "$R/home/skills/mine/SKILL.md" ]
  check "keybindings copied" [ -f "$R/home/keybindings.json" ]
  check "skills/synced not copied" [ ! -e "$R/home/skills/synced" ]
}

t_memories_backed_up_never_applied() {
  mkdir -p "$W/claude/projects/-home-u-proj/memory" "$W/claude/projects/-home-u-empty/memory"
  echo "fact" > "$W/claude/projects/-home-u-proj/memory/MEMORY.md"
  check "says committed memories" grep -q "committed memories" <<< "$(snap)"
  check "memory under host folder" [ -f "$R/memory/testhost/-home-u-proj/MEMORY.md" ]
  check "empty memory folders skipped" [ ! -e "$R/memory/testhost/-home-u-empty" ]
  rm "$W/claude/projects/-home-u-proj/memory/MEMORY.md"
  snap >/dev/null
  check "deleted memory removed from the repo copy" [ ! -e "$R/memory/testhost/-home-u-proj" ]
  check "deletion is in history" git -C "$R" log --format= --name-status -1 | grep -q "^D"
  apply >/dev/null
  check "apply did not restore the memory" [ ! -e "$W/claude/projects/-home-u-proj/memory/MEMORY.md" ]
}

t_disk_deletion_recorded() {
  echo a > "$W/claude/keybindings.json"; snap >/dev/null
  rm "$W/claude/keybindings.json"; snap >/dev/null
  check "removed from repo copy" [ ! -e "$R/home/keybindings.json" ]
}

t_apply_installs_repo_change() {
  snap >/dev/null
  echo "rules v3" > "$R/home/CLAUDE.md"
  check "says installed" grep -q "installed CLAUDE.md" <<< "$(apply)"
  check "disk updated" grep -q v3 "$W/claude/CLAUDE.md"
  out=$(apply)
  check "second apply has nothing to do" grep -q "nothing to install" <<< "$out"
}

t_apply_refuses_disk_change() {
  snap >/dev/null
  echo "edited by a session" > "$W/claude/CLAUDE.md"
  out=$(apply); rc=$?
  check "says run snapshot" grep -q "run jeeves-snapshot first" <<< "$out"
  check "exit status 1" [ $rc = 1 ]
  check "disk kept" grep -q "edited by a session" "$W/claude/CLAUDE.md"
}

t_both_changed_is_conflict() {
  snap >/dev/null
  echo "repo side" > "$R/home/CLAUDE.md"; git -C "$R" commit -qam "repo edit"
  echo "disk side" > "$W/claude/CLAUDE.md"
  n=$(commits); out=$(snap)
  check "snapshot reports conflict" grep -q "changed both" <<< "$out"
  check "snapshot did not commit" [ "$(commits)" = "$n" ]
  check "repo kept" grep -q "repo side" "$R/home/CLAUDE.md"
  check "apply reports conflict" grep -q "changed on both sides" <<< "$(apply)"
  check "disk kept" grep -q "disk side" "$W/claude/CLAUDE.md"
  apply --force >/dev/null
  check "force installs repo copy" grep -q "repo side" "$W/claude/CLAUDE.md"
  check "previous disk copy saved" grep -rq "disk side" "$R/.sync-state/overwritten"
}

t_snapshot_force_takes_disk() {
  snap >/dev/null
  echo "repo side" > "$R/home/CLAUDE.md"; git -C "$R" commit -qam "repo edit"
  echo "disk side" > "$W/claude/CLAUDE.md"
  snap --force >/dev/null
  check "repo copy is the disk one" grep -q "disk side" "$R/home/CLAUDE.md"
}

t_pulls_other_device() {
  snap >/dev/null
  git clone -q "$W/remote.git" "$W/other" 2>/dev/null
  echo "from laptop" > "$W/other/home/CLAUDE.md"
  git -C "$W/other" commit -qam "laptop edit" && git -C "$W/other" push -q origin main 2>/dev/null
  out=$(snap)
  check "fast-forwarded" grep -q "from laptop" "$R/home/CLAUDE.md"
  check "asks to apply" grep -q "run jeeves-apply" <<< "$out"
  apply >/dev/null
  check "applied" grep -q "from laptop" "$W/claude/CLAUDE.md"
}

t_diverged_still_records() {
  snap >/dev/null
  git clone -q "$W/remote.git" "$W/other" 2>/dev/null
  echo x > "$W/other/home/settings.json"
  git -C "$W/other" commit -qam "laptop" && git -C "$W/other" push -q origin main 2>/dev/null
  echo y > "$R/home/CLAUDE.md"; git -C "$R" commit -qam "local"; echo y > "$W/claude/CLAUDE.md"; snap >/dev/null
  echo z > "$W/claude/CLAUDE.md"
  out=$(snap)
  check "reports divergence" grep -q "diverged" <<< "$out"
  check "still committed locally" grep -q z "$R/home/CLAUDE.md"
}

t_offline_records_locally() {
  snap >/dev/null
  git -C "$R" remote set-url origin "$W/missing.git"
  echo "rules v2" > "$W/claude/CLAUDE.md"
  out=$(snap)
  check "mentions offline" grep -q "could not reach origin" <<< "$out"
  check "committed anyway" grep -q v2 "$R/home/CLAUDE.md"
  out=$(echo "rules v3" > "$W/claude/CLAUDE.md"; snap --hook)
  check "hook mode does not mention offline" bash -c '! grep -q "could not reach" <<< "$1"' _ "$out"
}

t_new_device_first_sync() {
  # A device whose ~/.claude differs from the repo before it ever synced: reported, not guessed.
  echo "this machine" > "$W/claude/CLAUDE.md"
  out=$(snap)
  check "conflict reported" grep -q "changed both" <<< "$out"
  check "repo unchanged" grep -q "rules v1" "$R/home/CLAUDE.md"
}

t_option_finds_data_repo() {
  snap >/dev/null
  unset JEEVES_DATA; export CLAUDE_PLUGIN_OPTION_DATA_REPO=$R
  echo "rules v2" > "$W/claude/CLAUDE.md"
  check "says committed" grep -q "committed CLAUDE.md" <<< "$(cd "$SCRATCH" && snap)"
  check "repo copy updated" grep -q v2 "$R/home/CLAUDE.md"
}

t_folder_finds_data_repo() {
  snap >/dev/null
  unset JEEVES_DATA
  echo "rules v2" > "$W/claude/CLAUDE.md"
  check "says committed" grep -q "committed CLAUDE.md" <<< "$(cd "$R/home" && snap)"
  check "repo copy updated" grep -q v2 "$R/home/CLAUDE.md"
}

t_no_data_repo() {
  unset JEEVES_DATA
  out=$(cd "$SCRATCH" && snap); rc=$?
  check "says how to set it" grep -q "set the plugin option data_repo" <<< "$out"
  check "exit status 1" [ $rc = 1 ]
  (cd "$SCRATCH" && snap --hook >/dev/null); check "hook mode exits 0" [ $? = 0 ]
  out=$(cd "$SCRATCH" && apply); rc=$?
  check "apply says how to set it" grep -q "set the plugin option data_repo" <<< "$out"
  check "apply exit status 1" [ $rc = 1 ]
  export JEEVES_DATA=$W/claude
  check "a folder that is not a git repo is refused" grep -q "not a git repository" <<< "$(snap)"
}

t_edit_hook_snapshots_claude_md() {
  snap >/dev/null; n=$(commits)
  echo "rules v2" > "$W/claude/CLAUDE.md"
  out=$(edit_hook "$W/claude/CLAUDE.md"); rc=$?
  check "exit status 0" [ $rc = 0 ]
  check "one new commit" [ "$(commits)" = $((n + 1)) ]
  check "repo copy updated" grep -q v2 "$R/home/CLAUDE.md"
  check "reports through systemMessage" grep -q '^{"systemMessage": "jeeves snapshot: committed CLAUDE.md"}$' <<< "$out"
  echo "rules v3" > "$W/claude/CLAUDE.md"
  edit_hook "$W/claude/../claude/CLAUDE.md" >/dev/null
  check "a path written another way still matches" grep -q v3 "$R/home/CLAUDE.md"
}

t_edit_hook_ignores_other_files() {
  snap >/dev/null; n=$(commits)
  echo "rules v2" > "$W/claude/CLAUDE.md"
  echo x > "$W/claude/settings.json"
  mkdir -p "$W/proj"; echo p > "$W/proj/CLAUDE.md"
  out=$( { edit_hook "$W/claude/settings.json"; edit_hook "$W/proj/CLAUDE.md"; edit_hook ""; } ); rc=$?
  check "no output" [ -z "$out" ]
  check "exit status 0" [ $rc = 0 ]
  check "no commit" [ "$(commits)" = "$n" ]
}

t_edit_hook_escapes_json() {
  snap >/dev/null
  echo "repo side" > "$R/home/CLAUDE.md"; git -C "$R" commit -qam "repo edit"
  echo "disk side" > "$W/claude/CLAUDE.md"
  out=$(edit_hook "$W/claude/CLAUDE.md")
  check "conflict reported" grep -q "changed both" <<< "$out"
  check "quotes escaped, one line" [ "$(printf '%s\n' "$out" | wc -l | tr -d ' ')" = 1 ]
  if command -v python3 >/dev/null; then
    check "valid JSON" python3 -c 'import json,sys; json.loads(sys.argv[1])' "$out"
  fi
}

t_plugin_files() {
  for f in .claude-plugin/plugin.json .claude-plugin/marketplace.json hooks/hooks.json; do
    if command -v python3 >/dev/null; then
      check "$f is valid JSON" python3 -c 'import json,sys; json.load(open(sys.argv[1]))' "$SRC/$f"
    fi
  done
  check "SessionStart runs jeeves-snapshot --hook" grep -q 'bin/jeeves-snapshot --hook' "$SRC/hooks/hooks.json"
  check "PostToolUse matches Write|Edit" grep -q '"matcher": "Write|Edit"' "$SRC/hooks/hooks.json"
  check "PostToolUse runs the edit hook" grep -q 'bin/jeeves-hook-edit' "$SRC/hooks/hooks.json"
  check "plugin option data_repo declared" grep -q '"data_repo"' "$SRC/.claude-plugin/plugin.json"
  for f in "$SRC"/bin/*; do check "$f is executable" [ -x "$f" ]; done
  check "lib.sh is not on the PATH" [ ! -e "$SRC/bin/lib.sh" ]
}

run_test "snapshot does nothing when disk matches the repo" t_nothing_changed
run_test "snapshot commits a changed CLAUDE.md" t_disk_change_committed
run_test "snapshot skips when the repo has uncommitted changes" t_dirty_repo_skips
run_test "snapshot skips when the repo is not on main" t_other_branch_skips
run_test "agents, own skills and keybindings are tracked, skills/synced is not" t_new_dirs_tracked_synced_excluded
run_test "memories go under the host folder and are never applied" t_memories_backed_up_never_applied
run_test "a file deleted on disk is deleted from the repo copy" t_disk_deletion_recorded
run_test "apply installs a file changed in the repo" t_apply_installs_repo_change
run_test "apply refuses a file changed on disk since the last sync" t_apply_refuses_disk_change
run_test "a file changed on both sides is reported, not merged" t_both_changed_is_conflict
run_test "snapshot --force keeps the disk copy of a conflict" t_snapshot_force_takes_disk
run_test "snapshot fast-forwards to another device's push and asks to apply" t_pulls_other_device
run_test "snapshot still records locally when main has diverged" t_diverged_still_records
run_test "snapshot records locally when the remote is unreachable" t_offline_records_locally
run_test "a new device that differs from the repo gets a conflict, not a guess" t_new_device_first_sync
run_test "the data repo comes from the plugin option when JEEVES_DATA is unset" t_option_finds_data_repo
run_test "the data repo is found from the current folder inside it" t_folder_finds_data_repo
run_test "without a data repo, the scripts say how to set one and hook mode exits 0" t_no_data_repo
run_test "the edit hook snapshots after an edit to the global CLAUDE.md" t_edit_hook_snapshots_claude_md
run_test "the edit hook ignores edits to other files" t_edit_hook_ignores_other_files
run_test "the edit hook's systemMessage is valid JSON" t_edit_hook_escapes_json
run_test "the manifests and hooks.json are valid and wire up the hooks" t_plugin_files

echo "$passed passed, $failed failed"
[ $failed = 0 ]
