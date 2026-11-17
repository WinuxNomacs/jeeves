#!/usr/bin/env bash
# Tests for the plugin's scripts (bin/jeeves-*) and hook files. Each test builds a scratch ~/.claude, a
# scratch data repo and a local bare "remote" in a temp folder, so nothing real is touched.
#
#   tests/run.sh            run all tests
#   tests/run.sh <pattern>  run tests whose name contains <pattern>

SRC=$(cd "$(dirname "$0")/.." && pwd)
# Normalized as the scripts print paths (macOS's TMPDIR ends in "/", and BSD mktemp keeps the "//").
SCRATCH=$(cd "$(mktemp -d "${TMPDIR:-/tmp}/jeeves-tests.XXXXXX")" && pwd)
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
  mkdir -p "$W/repo/home/agents" "$W/claude/agents"
  echo "agent v1" > "$W/repo/home/agents/a.md"
  echo "agent v1" > "$W/claude/agents/a.md"
  echo "rules v1" > "$W/claude/CLAUDE.md"
  echo '{"a":1}' > "$W/claude/settings.json"
  mkdir -p "$W/repo/hosts/testhost" && echo '{"a":1}' > "$W/repo/hosts/testhost/settings.json"
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
  mkdir -p "$W/claude/skills/.trash/123-gone" && echo t > "$W/claude/skills/.trash/123-gone/SKILL.md"
  echo k > "$W/claude/keybindings.json"
  snap >/dev/null
  check "agent copied" [ -f "$R/home/agents/reviewer.md" ]
  check "own skill copied" [ -f "$R/home/skills/mine/SKILL.md" ]
  check "keybindings not shared" [ ! -e "$R/home/keybindings.json" ]
  check "keybindings backed up for this host" [ -f "$R/hosts/testhost/keybindings.json" ]
  check "settings backed up for this host" [ -f "$R/hosts/testhost/settings.json" ]
  check "skills/synced not copied" [ ! -e "$R/home/skills/synced" ]
  check "skills Claude Code removed (skills/.trash) not copied" [ ! -e "$R/home/skills/.trash" ]
}

t_memories_backed_up_never_applied() {
  mkdir -p "$W/claude/projects/-home-u-proj/memory" "$W/claude/projects/-home-u-empty/memory"
  echo "fact" > "$W/claude/projects/-home-u-proj/memory/MEMORY.md"
  check "says committed memories" grep -q "committed memories" <<< "$(snap)"
  check "memory under the host folder" [ -f "$R/hosts/testhost/projects/-home-u-proj/memory/MEMORY.md" ]
  check "empty memory folders skipped" [ ! -e "$R/hosts/testhost/projects/-home-u-empty" ]
  rm "$W/claude/projects/-home-u-proj/memory/MEMORY.md"
  snap >/dev/null
  check "deleted memory removed from the repo copy" [ ! -e "$R/hosts/testhost/projects/-home-u-proj" ]
  check "deletion is in history" git -C "$R" log --format= --name-status -1 | grep -q "^D"
  apply >/dev/null
  check "apply did not restore the memory" [ ! -e "$W/claude/projects/-home-u-proj/memory/MEMORY.md" ]
}

t_disk_deletion_recorded() {
  echo a > "$W/claude/agents/b.md"; snap >/dev/null
  rm "$W/claude/agents/b.md"; snap >/dev/null
  check "removed from repo copy" [ ! -e "$R/home/agents/b.md" ]
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

t_apply_reports_saved_copies_only_when_it_saved() {
  # A fixed clock puts both runs in the same second, so they share the overwritten/ folder name.
  mkdir -p "$W/bin"; printf '#!/bin/sh\necho 20260101-000000\n' > "$W/bin/date"; chmod +x "$W/bin/date"
  snap >/dev/null
  echo "repo side" > "$R/home/CLAUDE.md"; git -C "$R" commit -qam "repo edit"
  echo "disk side" > "$W/claude/CLAUDE.md"
  check "force run says it saved" grep -q "previous disk copies saved" <<< "$(PATH=$W/bin:$PATH apply --force)"
  out=$(PATH=$W/bin:$PATH apply)
  check "next run saved nothing and says nothing about it" bash -c '! grep -q "saved" <<< "$1"' _ "$out"
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
  echo x > "$W/other/home/agents/a.md"
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
  if have_python; then
    check "valid JSON" python3 -c 'import json,sys; json.loads(sys.argv[1])' "$out"
  fi
}

status() { "$SRC/bin/jeeves-status" "$@"; }
# Python validates JSON when it is there (on Windows, "python3" may be a stub that only prints a hint).
have_python() { python3 -c pass >/dev/null 2>&1; }
scan() { "$SRC/bin/jeeves-scan" "$@"; }

t_status_lists_out_of_sync_paths() {
  snap >/dev/null
  out=$(status)
  check "in sync: only the repo, branch and main lines" [ "$(printf '%s\n' "$out" | wc -l | tr -d ' ')" = 3 ]
  check "names the data repo" grep -q "^datarepo	$R$" <<< "$out"
  check "clean main" grep -q "^branch	main	clean$" <<< "$out"
  check "even with origin" grep -q "^main	even	0	0$" <<< "$out"
  b=$(git -C "$R" hash-object home/CLAUDE.md)
  echo "disk side" > "$W/claude/CLAUDE.md"
  echo "agent v2" > "$R/home/agents/a.md"
  mkdir -p "$W/claude/commands" "$R/home/commands"
  echo k > "$W/claude/commands/c.md"; echo other > "$R/home/commands/c.md"; git -C "$R" add -A
  git -C "$R" commit -qam "repo edits"
  out=$(status)
  d=$(git hash-object "$W/claude/CLAUDE.md")
  check "disk change with its three hashes" grep -q "^disk	CLAUDE.md	$d	$b	$b$" <<< "$out"
  check "repo change" grep -q "^repo	agents/a.md	" <<< "$out"
  check "never-synced file on both sides is a conflict with no base" grep -q "^conflict	commands/c.md	.*	none$" <<< "$out"
  check "base is readable from git" [ "$(git -C "$R" cat-file -p "$b")" = "rules v1" ]
  check "ahead of origin" grep -q "^main	ahead	1	0$" <<< "$out"
  check "status changed nothing" grep -q "rules v1" "$R/home/CLAUDE.md"
}

t_status_reports_diverged_and_dirty() {
  snap >/dev/null
  git clone -q "$W/remote.git" "$W/other" 2>/dev/null
  echo x > "$W/other/home/agents/a.md"
  git -C "$W/other" commit -qam "laptop" && git -C "$W/other" push -q origin main 2>/dev/null
  echo y > "$R/home/CLAUDE.md"; git -C "$R" commit -qam "local"
  git -C "$R" fetch -q origin
  echo stray > "$R/stray.txt"
  out=$(status)
  check "diverged" grep -q "^main	diverged	1	1$" <<< "$out"
  check "dirty" grep -q "^branch	main	dirty$" <<< "$out"
}

t_scan_flags_secrets_without_printing_them() {
  snap >/dev/null
  tok="sk-ant-api03-$(printf 'x%.0s' 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20 21 22 23 24)"
  printf 'line one\nexport ANTHROPIC_API_KEY=%s\n' "$tok" > "$R/home/CLAUDE.md"
  git -C "$R" commit -qam "oops"
  printf '%s\n' '{"env":{"SERVICE_TOKEN":"abcdefghijklmnop1234"}}' > "$W/claude/settings.json"
  snap >/dev/null
  out=$(scan); rc=$?
  check "exit status 1" [ $rc = 1 ]
  check "names file and line" grep -q "possible Anthropic API key at home/CLAUDE.md:2" <<< "$out"
  check "catches a token in this host's settings backup" grep -q "value at hosts/testhost/settings.json:1" <<< "$out"
  check "never prints the secret" bash -c '! grep -Eq "x{10}|abcdefghijklmnop" <<< "$1"' _ "$out"
}

t_scan_passes_clean_diff() {
  snap >/dev/null
  printf 'rules v2\nUse a token budget of 2000 when summarising.\n' > "$R/home/CLAUDE.md"
  git -C "$R" commit -qam "clean"
  out=$(scan); rc=$?
  check "exit status 0" [ $rc = 0 ]
  check "says nothing found" grep -q "nothing found in 2 added lines" <<< "$out"
  git -C "$R" push -q origin main 2>/dev/null
  check "nothing outgoing" grep -q "nothing found in 0 added lines" <<< "$(scan)"
}

t_snapshot_points_to_update_skill() {
  snap >/dev/null
  echo "repo side" > "$R/home/CLAUDE.md"; echo "agent v2" > "$R/home/agents/a.md"
  git -C "$R" commit -qam "repo edit"
  echo "disk side" > "$W/claude/CLAUDE.md"
  out=$(snap)
  check "newer files mention /jeeves:update" grep -q "run jeeves-apply to install them, or /jeeves:update" <<< "$out"
  check "conflicts mention /jeeves:update" grep -q "merge them with /jeeves:update" <<< "$out"
}

restore() { "$SRC/bin/jeeves-restore" "$@"; }
# A command on PATH that prints fixed output (or runs a small script), standing in for a system tool.
fake() { # fake <name> <script body>
  mkdir -p "$W/fakebin"; printf '#!/bin/sh\n%s\n' "$2" > "$W/fakebin/$1"; chmod +x "$W/fakebin/$1"
}

t_host_files_never_applied() {
  snap >/dev/null
  echo '{"other":1}' > "$R/hosts/testhost/settings.json"
  mkdir -p "$R/hosts/laptop"; echo '{"laptop":1}' > "$R/hosts/laptop/settings.json"
  git -C "$R" add -A && git -C "$R" commit -qm "edits to backups"
  apply >/dev/null
  check "this host's settings untouched by apply" grep -q '"a":1' "$W/claude/settings.json"
  check "status lists no paths" [ "$(status | wc -l | tr -d ' ')" = 3 ]
  snap >/dev/null
  check "the next snapshot puts back this host's own backup" grep -q '"a":1' "$R/hosts/testhost/settings.json"
  check "other hosts' backups are left alone" [ -f "$R/hosts/laptop/settings.json" ]
}

t_old_layout_moved_to_host_folder() {
  echo '{"a":1}' > "$R/home/settings.json"
  mkdir -p "$R/memory/testhost/-home-u-proj" "$R/memory/laptop/-home-l"
  echo old > "$R/memory/testhost/-home-u-proj/MEMORY.md"; echo l > "$R/memory/laptop/-home-l/MEMORY.md"
  git -C "$R" add -A && git -C "$R" commit -qm "old layout"
  mkdir -p "$W/claude/projects/-home-u-proj/memory"; echo fact > "$W/claude/projects/-home-u-proj/memory/MEMORY.md"
  snap >/dev/null
  check "shared settings.json removed" [ ! -e "$R/home/settings.json" ]
  check "this host's old memory folder removed" [ ! -e "$R/memory/testhost" ]
  check "memory now in the host folder" [ -f "$R/hosts/testhost/projects/-home-u-proj/memory/MEMORY.md" ]
  check "another host's old memory folder is left for that host" [ -f "$R/memory/laptop/-home-l/MEMORY.md" ]
  check "status lists no paths" [ "$(status | wc -l | tr -d ' ')" = 3 ]
}

t_names_other_systems_cannot_hold_refused() {
  snap >/dev/null; n=$(commits)
  mkdir -p "$W/claude/skills/s"
  if ( echo x > "$W/claude/skills/s/a:b.md" ) 2>/dev/null && [ -f "$W/claude/skills/s/a:b.md" ]; then
    out=$(snap); rc=$?
    check "says not taken and why" grep -q "not taken.*skills/s/a:b.md (its name has a character Windows does not allow)" <<< "$out"
    check "exit status 1" [ $rc = 1 ]
    check "not copied" [ ! -e "$R/home/skills/s/a:b.md" ]
    check "no commit" [ "$(commits)" = "$n" ]
    rm "$W/claude/skills/s/a:b.md"
  fi
  echo x > "$W/claude/agents/Big.md"
  if [ ! -e "$W/claude/agents/big.md" ]; then # case-sensitive file system
    echo y > "$W/claude/agents/big.md"
    out=$(snap)
    check "names differing only in case not taken" grep -q "agents/Big.md (another file's name differs from it only in case" <<< "$out"
    check "neither copied" [ ! -e "$R/home/agents/Big.md" ] && check "neither copied" [ ! -e "$R/home/agents/big.md" ]
    rm "$W/claude/agents/big.md"
  fi
  out=$(snap)
  check "a valid name is taken" [ -f "$R/home/agents/Big.md" ]
  check "nothing refused then" bash -c '! grep -q "not taken" <<< "$1"' _ "$out"
}

t_native_windows_host_name_gets_suffix() {
  unset JEEVES_HOST
  fake hostname 'echo box.example.org'
  fake uname 'echo MINGW64_NT-10.0-26100'
  echo "rules v2" > "$W/claude/CLAUDE.md"
  PATH=$W/fakebin:$PATH snap >/dev/null
  check "commit names box-win" git -C "$R" log -1 --format=%s | grep -q "^snapshot(box-win)"
  check "backup under hosts/box-win" [ -f "$R/hosts/box-win/settings.json" ]
  fake uname 'echo Linux'
  echo "rules v3" > "$W/claude/CLAUDE.md"
  PATH=$W/fakebin:$PATH snap >/dev/null
  check "Linux keeps the plain name" git -C "$R" log -1 --format=%s | grep -q "^snapshot(box)"
}

t_edit_hook_takes_a_windows_path() {
  snap >/dev/null; n=$(commits)
  fake cygpath "printf '%s\n' \"\$2\" | sed 's#^C:/X#$W#'"
  echo "rules v2" > "$W/claude/CLAUDE.md"
  out=$(PATH=$W/fakebin:$PATH edit_hook 'C:\\X\\claude\\CLAUDE.md')
  check "snapshot ran" [ "$(commits)" = $((n + 1)) ]
  check "reports through systemMessage" grep -q 'committed CLAUDE.md' <<< "$out"
}

t_restore_copies_a_host_backup() {
  mkdir -p "$R/hosts/old/projects/-home-u-here/memory" "$R/hosts/old/projects/-home-u-gone/memory"
  echo '{"old":1}' > "$R/hosts/old/settings.json"; echo kb > "$R/hosts/old/keybindings.json"
  echo m > "$R/hosts/old/projects/-home-u-here/memory/MEMORY.md"
  echo g > "$R/hosts/old/projects/-home-u-gone/memory/MEMORY.md"
  git -C "$R" add -A && git -C "$R" commit -qm "old host backup"
  mkdir -p "$W/claude/projects/-home-u-here"
  check "lists the backups" grep -q "this host: testhost; backups: old testhost" <<< "$(restore)"
  out=$(restore nosuch); rc=$?
  check "unknown host: exit 1 and names the backups" [ $rc = 1 ] && check "unknown host message" grep -q "no backup for nosuch (backups: old testhost)" <<< "$out"
  out=$(restore old)
  check "settings restored" grep -q old "$W/claude/settings.json"
  check "keybindings restored" [ "$(cat "$W/claude/keybindings.json")" = kb ]
  check "the replaced settings saved" grep -rq '"a":1' "$R/.sync-state/overwritten"
  check "says what it restored" grep -q "restored from old: settings.json, keybindings.json" <<< "$out"
  check "memories only on request" [ ! -e "$W/claude/projects/-home-u-here/memory" ]
  out=$(restore old --memories)
  check "memory of a project here restored" [ -f "$W/claude/projects/-home-u-here/memory/MEMORY.md" ]
  check "memory of a missing project not restored" [ ! -e "$W/claude/projects/-home-u-gone" ]
  check "says which were not restored" grep -q "not restored (no such project on this machine): -home-u-gone" <<< "$out"
  check "restore never commits" [ -z "$(git -C "$R" status --porcelain)" ]
}

t_init_creates_a_data_repo() {
  out=$("$SRC/bin/jeeves-init" "$W/new" "$W/remote.git")
  check "says created" grep -q "created $W/new with origin" <<< "$out"
  check "on main" [ "$(git -C "$W/new" symbolic-ref --short HEAD)" = main ]
  check "one commit" [ "$(git -C "$W/new" rev-list --count HEAD)" = 1 ]
  check "LF line endings everywhere" grep -q 'eol=lf' "$W/new/.gitattributes"
  check "sync record ignored" grep -qx '.sync-state/' "$W/new/.gitignore"
  check "origin set" bash -c '[[ "$(git -C "$1" remote get-url origin)" == */remote.git ]]' _ "$W/new"
  "$SRC/bin/jeeves-init" "$W/new" >/dev/null; rc=$?
  check "refuses a folder that is not empty" [ $rc = 1 ]
  JEEVES_DATA=$W/new snap --no-pull >/dev/null
  check "the first snapshot fills it" [ -f "$W/new/home/CLAUDE.md" ] && check "and backs up the host" [ -f "$W/new/hosts/testhost/settings.json" ]
}

t_plugin_files() {
  for f in .claude-plugin/plugin.json .claude-plugin/marketplace.json hooks/hooks.json; do
    if have_python; then
      check "$f is valid JSON" python3 -c 'import json,sys; json.load(open(sys.argv[1]))' "$SRC/$f"
    fi
  done
  check "SessionStart runs jeeves-snapshot --hook" grep -q 'bin/jeeves-snapshot --hook' "$SRC/hooks/hooks.json"
  check "PostToolUse matches Write|Edit" grep -q '"matcher": "Write|Edit"' "$SRC/hooks/hooks.json"
  check "PostToolUse runs the edit hook" grep -q 'bin/jeeves-hook-edit' "$SRC/hooks/hooks.json"
  check "plugin option data_repo declared" grep -q '"data_repo"' "$SRC/.claude-plugin/plugin.json"
  check "MIT licence file" grep -q '^MIT License$' "$SRC/LICENSE"
  check "licence declared in plugin.json" grep -q '"license": "MIT"' "$SRC/.claude-plugin/plugin.json"
  check "version set in plugin.json only, not in marketplace.json" bash -c '! grep -q "\"version\"" "$1"' _ "$SRC/.claude-plugin/marketplace.json"
  check "the marketplace gives the plugin source as ./ (older Claude Code versions reject \".\")" grep -q '"source": "\./"' "$SRC/.claude-plugin/marketplace.json"
  check "no CLAUDE.md at the plugin root (shipped to users to no effect)" [ ! -e "$SRC/CLAUDE.md" ]
  wf=$SRC/.github/workflows/test.yml
  check "the workflow has no pull-request or schedule trigger" bash -c '! grep -Eq "^  (pull_request|pull_request_target|schedule):" "$1"' _ "$wf"
  check "the workflow's push trigger is for tags only" bash -c '[ "$(grep -A1 "^  push:" "$1" | tail -1 | sed "s/^ *//" | cut -d: -f1)" = tags ]' _ "$wf"
  for f in "$SRC"/bin/*; do check "$f is executable" [ -x "$f" ]; done
  check "lib.sh is not on the PATH" [ ! -e "$SRC/bin/lib.sh" ]
  sk=$SRC/skills/update/SKILL.md
  check "update skill exists" [ -f "$sk" ]
  check "update skill is named update" grep -q '^name: update$' "$sk"
  check "Claude may start the update skill" bash -c '! grep -q "^disable-model-invocation: true" "$1"' _ "$sk"
  check "update skill gets the data repo from the plugin option" grep -q 'JEEVES_DATA="${user_config.data_repo}"' "$sk"
  for k in setup restore; do
    sk=$SRC/skills/$k/SKILL.md
    check "$k skill is named $k" grep -q "^name: $k\$" "$sk"
    check "only the user starts the $k skill" grep -q '^disable-model-invocation: true$' "$sk"
    check "$k skill gets the data repo from the plugin option" grep -q 'JEEVES_DATA="${user_config.data_repo}"' "$sk"
  done
}

run_test "snapshot does nothing when disk matches the repo" t_nothing_changed
run_test "snapshot commits a changed CLAUDE.md" t_disk_change_committed
run_test "snapshot skips when the repo has uncommitted changes" t_dirty_repo_skips
run_test "snapshot skips when the repo is not on main" t_other_branch_skips
run_test "agents and own skills are shared, skills/synced and .trash are not, settings and keybindings are backed up per host" t_new_dirs_tracked_synced_excluded
run_test "memories go under the host folder and are never applied" t_memories_backed_up_never_applied
run_test "a file deleted on disk is deleted from the repo copy" t_disk_deletion_recorded
run_test "apply installs a file changed in the repo" t_apply_installs_repo_change
run_test "apply refuses a file changed on disk since the last sync" t_apply_refuses_disk_change
run_test "a file changed on both sides is reported, not merged" t_both_changed_is_conflict
run_test "apply mentions saved disk copies only on the run that saved them" t_apply_reports_saved_copies_only_when_it_saved
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
run_test "status lists out-of-sync paths with their disk, repo and base hashes" t_status_lists_out_of_sync_paths
run_test "status reports a diverged main and a dirty data repo" t_status_reports_diverged_and_dirty
run_test "scan flags secrets in outgoing commits without printing them" t_scan_flags_secrets_without_printing_them
run_test "scan passes a clean outgoing diff" t_scan_passes_clean_diff
run_test "snapshot points to /jeeves:update for newer files and conflicts" t_snapshot_points_to_update_skill
run_test "per-host backups are never applied, and only their own host rewrites them" t_host_files_never_applied
run_test "the old layout (shared settings, memory/<host>) moves to the host folder" t_old_layout_moved_to_host_folder
run_test "names Windows or macOS cannot hold are not taken" t_names_other_systems_cannot_hold_refused
run_test "native Windows gets -win after the host name" t_native_windows_host_name_gets_suffix
run_test "the edit hook takes a Windows C:\\ path" t_edit_hook_takes_a_windows_path
run_test "restore copies a host's backup and saves what it replaces" t_restore_copies_a_host_backup
run_test "init creates an empty data repo that a snapshot fills" t_init_creates_a_data_repo
run_test "the manifests and hooks.json are valid and wire up the hooks" t_plugin_files

echo "$passed passed, $failed failed"
[ $failed = 0 ]
