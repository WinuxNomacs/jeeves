#!/usr/bin/env bash
# Background updates: every so often, a separate headless Claude Code session reviews what the user's session
# taught and writes it to the global rules, as /jeeves:update does when the user runs it. Off by default, per
# device; /jeeves:autoupdate turns it on or off.
#
#   jeeves-auto status            the setting, the last runs, what waits for the user
#   jeeves-auto on | off          turn background updates on or off on this device (either answers the offer)
#   jeeves-auto --hook-stop       Stop hook: reads the hook's JSON; when a run is due, starts one detached
#   jeeves-auto --session-start   SessionStart hook: the one-line offer, and notices from background runs
#   jeeves-auto run <session id> <transcript> <folder>
#                                 one background run (started by the Stop hook; runs in the foreground)
#   jeeves-auto hold | release    /jeeves:update, run by the user, holds the run lock while it works
#   jeeves-auto queue             items background runs left for the user's next /jeeves:update
#   jeeves-auto queue clear <id>...
#
# How a run keeps the user's data safe (the plan in upcoming-changes.md, "Background auto-update"):
# - The model sees only a private run folder: an extract of the session since the last run (no tool results;
#   each message between markers with a random per-run nonce, so quoted text cannot pose as the user), a copy
#   of the global CLAUDE.md to edit, the project's CLAUDE.md to read, and up to 25 memory files. It runs with
#   --permission-mode dontAsk, may edit that one copy and write its notes, and has no shell, web or MCP tools.
# - This script, not the model, changes the data repo: it checks the edited copy (only additions and
#   revisions: every Security line, every heading and every rule kept, few lines changed, nothing risk_flags
#   flags), merges it into main under the data repo's lock, runs jeeves-scan and jeeves-risk, and pushes the
#   commit it scanned. A result that fails a check is set aside with a notice, never pushed.
# - One run at a time per device (a run lock with PID and start time); a second due run skips. A deadline
#   stops a hung child (SIGTERM, then SIGKILL). Logs hold figures only, never session text.
# - The child gets none of the session's variables (messaging token, socket, plugin variables, the age key).
#
# Needs perl with JSON::PP (a core module) to read transcripts; without it, background updates stay off.

# The bash version, while the Go build's parity is checked (lib/launch.sh runs it with JEEVES_NO_GO=1 or
# where no build fits); bin/jeeves-auto is the launcher.
. "$(dirname "$0")/../lib/lib.sh"

SELF=$(cd "$(dirname "$0")/../bin" && pwd)/jeeves-auto
BIN=${SELF%/*}
AUTO_DIR=$CLAUDE_HOME/jeeves/auto
CONF=$CLAUDE_HOME/jeeves/auto.conf
LOCK=$AUTO_DIR/run.lock

say() { echo "jeeves auto: $*"; }
now() { date +%s; }

# A setting from auto.conf (this device's), or the default given.
auto_conf() {
  local v=
  [ -f "$CONF" ] && v=$(tr -d '\r' < "$CONF" | sed -n "s/^[[:space:]]*$1[[:space:]]*=[[:space:]]*//p" | sed 's/[[:space:]]*$//' | tail -1)
  printf '%s\n' "${v:-$2}"
}
enabled() { [ "$(auto_conf enabled no)" = yes ]; }
num_conf() { # num_conf <key> <default>: a whole number from auto.conf, or the default
  local v; v=$(auto_conf "$1" "$2")
  case "$v" in ''|*[!0-9]*) v=$2 ;; esac
  printf '%s\n' "$v"
}
INTERVAL=$(( $(num_conf interval_minutes 60) * 60 ))
MIN_NEW=$(num_conf min_new_chars 8000)
DEADLINE=${JEEVES_AUTO_DEADLINE_SECONDS:-$(( $(num_conf deadline_minutes 15) * 60 ))}
MAX_EXTRACT=$(num_conf max_extract_chars 200000)

have_perl() { perl -MJSON::PP -e 1 >/dev/null 2>&1; }

# The session's variables never reach a run: the supervisor keeps only what finds the data repo; the child
# (Claude Code itself) keeps only what selects its config, sign-in and provider (none of the session's: its
# messaging token and socket, plugin variables, the age key), and gets JEEVES_AUTO_CHILD=1, so its own hooks
# do nothing.
CHILD_KEEPS=" CLAUDE_CONFIG_DIR CLAUDE_CODE_OAUTH_TOKEN CLAUDE_CODE_USE_BEDROCK CLAUDE_CODE_USE_VERTEX CLAUDE_CODE_USE_FOUNDRY CLAUDE_CODE_SKIP_BEDROCK_AUTH CLAUDE_CODE_SKIP_VERTEX_AUTH CLAUDE_CODE_SKIP_FOUNDRY_AUTH CLAUDE_CODE_GIT_BASH_PATH "
scrub_env() { # scrub_env supervisor|child
  local v
  for v in $(env | sed -n 's/^\(CLAUDE[A-Za-z0-9_]*\)=.*/\1/p; s/^\(JEEVES[A-Za-z0-9_]*\)=.*/\1/p'); do
    case "$1:$v" in
      supervisor:JEEVES_AGE_KEY) unset "$v" ;;
      supervisor:CLAUDE_HOME|supervisor:CLAUDE_PLUGIN_OPTION_DATA_REPO|supervisor:JEEVES_*) ;;
      supervisor:*) case "$CHILD_KEEPS" in *" $v "*) ;; *) unset "$v" ;; esac ;;
      child:*) case "$CHILD_KEEPS" in *" $v "*) ;; *) unset "$v" ;; esac ;;
      *) unset "$v" ;;
    esac
  done
}

# The user's settings.json is not loaded by the child (its permission rules, hooks and other plugins would
# widen what the child may do); only its model and its env block (minus CLAUDE_CODE_PLUGIN_DIRS: the plugin
# is loaded by path) are carried over. user_settings model|env prints the model, or the env as NUL-separated
# NAME=value pairs.
user_settings() {
  perl -MJSON::PP -e 'local $/; my $d = eval { JSON::PP->new->utf8->decode(<STDIN>) }; exit 0 unless ref $d eq "HASH";
    if ($ARGV[0] eq "model") { my $m = $d->{model}; print "$m\n" if defined $m && !ref $m && $m =~ /^[A-Za-z0-9._:\[\]-]{1,100}$/; exit 0 }
    my $e = ref $d->{env} eq "HASH" ? $d->{env} : {};
    for my $k (sort keys %$e) { next unless $k =~ /^[A-Za-z_][A-Za-z0-9_]*$/ && $k ne "CLAUDE_CODE_PLUGIN_DIRS";
      next if ref $e->{$k}; my $v = $e->{$k} // ""; next if $v =~ /\0/; print "$k=$v\0" }' "$1" \
    < "$CLAUDE_HOME/settings.json" 2>/dev/null
}
# A path as native Windows programs take it (C:/...), from Git Bash; unchanged elsewhere.
native_path() { if command -v cygpath >/dev/null 2>&1; then cygpath -m "$1"; else printf '%s\n' "$1"; fi; }
# A path as Claude Code's permission rules match it: on Windows it normalizes C:\Users\... to /c/Users/...
# (docs, "permissions"), which Git Bash's /tmp is not; a rule then puts "/" before it (//c/Users/...).
rule_path() { if command -v cygpath >/dev/null 2>&1; then cygpath -u "$(cygpath -m "$1")"; else printf '%s\n' "$1"; fi; }

# ---- Notices (printed once by the session-start hook) and the log ----

notice() { # notice <name> <text>: replaces a notice of the same name
  mkdir -p "$AUTO_DIR/notices" && printf '%s\n' "$2" > "$AUTO_DIR/notices/$1"
}
log_run() { # log_run <outcome> [figures]: one line per run, the last 50 kept; never session text
  local f=$AUTO_DIR/log
  mkdir -p "$AUTO_DIR" || return 0
  printf '%s %s %s %s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "${SESSION:-?}" "$1" "$2" >> "$f"
  if [ "$(wc -l < "$f" | tr -d ' ')" -gt 50 ]; then tail -50 "$f" > "$f.tmp" && mv "$f.tmp" "$f"; fi
}

# ---- The run lock: one background run (or the user's /jeeves:update) per device ----
# "<pid> <start> <owner>". An auto run's lock is stale when its PID is gone or it is older than the deadline
# plus five minutes (a PID reused meanwhile is not mistaken for the run); a manual hold after two hours.

lock_stale() {
  local pid start owner count age
  read -r pid start owner count < "$LOCK" 2>/dev/null || { [ -n "$(find "$LOCK" -mmin +1 2>/dev/null)" ]; return; }
  case "$start" in ''|*[!0-9]*) [ -n "$(find "$LOCK" -mmin +1 2>/dev/null)" ]; return ;; esac
  age=$(( $(now) - start ))
  case "$owner" in
    manual) [ $age -gt 7200 ] ;;
    *) ! kill -0 "$pid" 2>/dev/null || [ $age -gt $(( DEADLINE + 300 )) ] ;;
  esac
}
lock_owner() { local pid start owner count; read -r pid start owner count < "$LOCK" 2>/dev/null && printf '%s\n' "$owner"; }
take_run_lock() { # take_run_lock auto|manual: 0 when taken
  local held
  mkdir -p "$AUTO_DIR" || return 1
  ( set -o noclobber; printf '%s %s %s\n' "$$" "$(now)" "$1" > "$LOCK" ) 2>/dev/null && return 0
  # Taken over by one process at a time, and only if it still holds what was judged stale.
  ( set -o noclobber; echo $$ > "$LOCK.takeover" ) 2>/dev/null || {
    [ -n "$(find "$LOCK.takeover" -mmin +1 2>/dev/null)" ] && rm -f "$LOCK.takeover"; return 1; }
  held=$(cat "$LOCK" 2>/dev/null)
  if lock_stale && [ "$(cat "$LOCK" 2>/dev/null)" = "$held" ]; then rm -f "$LOCK"; fi
  rm -f "$LOCK.takeover"
  ( set -o noclobber; printf '%s %s %s\n' "$$" "$(now)" "$1" > "$LOCK" ) 2>/dev/null
}
release_run_lock() {
  local pid start owner count
  read -r pid start owner count < "$LOCK" 2>/dev/null || return 0
  [ "$pid" = "$$" ] && [ "$owner" = auto ] && rm -f "$LOCK"
  return 0
}

# ---- Reading transcripts (perl, JSON::PP) ----

# extract <transcript> <offset> <nonce> <max chars> <offset file>: prints the extract of the complete lines
# after <offset> (the user's messages and Claude's text, tool calls cut to 300 characters; no tool results,
# no injected or summary messages, no subagent lines), stopping before <max chars> (0: no limit); writes the
# offset reached to <offset file>. With nonce "-", prints only the extract's length.
EXTRACT_PL='
use strict; use warnings; use JSON::PP;
my ($file, $offset, $nonce, $max, $offfile) = @ARGV;
open(my $fh, "<:raw", $file) or exit 3;
my $size = -s $fh; $offset = 0 if $offset > $size;
seek($fh, $offset, 0) or exit 3;
binmode(STDOUT, ":encoding(UTF-8)");
my $json = JSON::PP->new->utf8;
my ($pos, $chars, $count) = ($offset, 0, $nonce eq "-");
while (my $line = <$fh>) {
  last unless $line =~ /\n\z/;
  my $next = $pos + length($line);
  # A tool result (often large) is never read: skipped before decoding, which keeps the hook fast.
  # Pasted images: their base64 data is dropped before decoding (pure-Perl JSON is slow on it).
  $line =~ s/"data":"[A-Za-z0-9+\/=]{256,}"/"data":""/g;
  my $d = $line =~ /(?<!\\)"tool_use_id"/ && $line =~ /(?<!\\)"type":"user"/ ? undef : eval { $json->decode($line) };
  my $piece = "";
  if (ref $d eq "HASH" && ($d->{type} // "") =~ /^(user|assistant)$/ && !$d->{isMeta} && !$d->{isSidechain}
      && !$d->{isCompactSummary}) {
    my $role = $d->{type} eq "user" ? "USER" : "CLAUDE";
    my $c = ref $d->{message} eq "HASH" ? $d->{message}{content} : undef;
    my @blocks = ref $c eq "ARRAY" ? @$c : (defined $c && !ref $c ? ({ type => "text", text => $c }) : ());
    for my $b (@blocks) {
      next unless ref $b eq "HASH";
      my $t = $b->{type} // "";
      if ($t eq "text") {
        my $s = $b->{text} // "";
        next if $s =~ /^\s*$/;
        next if $role eq "USER" && $s =~ /^\s*<(system-reminder|local-command|command-name|command-message|command-args|task-notification|bash-input|bash-stdout|bash-stderr|user-prompt-submit-hook)/;
        $piece .= "<<<$nonce $role>>>\n$s\n<<<$nonce END>>>\n\n";
      } elsif ($t eq "tool_use" && $role eq "CLAUDE") {
        my $in = eval { JSON::PP->new->canonical->encode($b->{input} // {}) } // "";
        (my $name = $b->{name} // "") =~ s/[^A-Za-z0-9_:.-]//g;
        $piece .= "<<<$nonce TOOL $name>>>\n" . substr($in, 0, 300) . "\n<<<$nonce END>>>\n\n";
      }
    }
  }
  last if $count && $max && $chars >= $max;
  last if !$count && $max && $chars > 0 && $chars + length($piece) > $max;
  $chars += length($piece); $pos = $next;
  print $piece if !$count;
}
print "$chars\n" if $count;
if ($offfile) { open(my $o, ">", $offfile) or exit 4; print $o "$pos\n"; close $o or exit 4; }
'
extract() { perl -MJSON::PP -e "$EXTRACT_PL" -- "$@"; }

# json_field <field>: a top-level string, number or boolean field of the JSON on stdin (the hook's input,
# the child's result), or nothing.
json_field() {
  perl -MJSON::PP -e 'local $/; my $d = eval { JSON::PP->new->utf8->decode(<STDIN>) };
    my $v = ref $d eq "HASH" ? $d->{$ARGV[0]} : undef;
    if (JSON::PP::is_bool($v)) { print(($v ? "true" : "false") . "\n") }
    elsif (defined $v && !ref $v) { $v =~ s/[\r\n]/ /g; binmode(STDOUT, ":encoding(UTF-8)"); print "$v\n" }' "$1" 2>/dev/null
}

# A Windows path (C:\...) in the POSIX form Git Bash uses.
posix_path() {
  local p; p=$(printf '%s' "$1" | tr '\\' /)
  case "$p" in [A-Za-z]:/*) command -v cygpath >/dev/null 2>&1 && p=$(cygpath -u "$p") ;; esac
  printf '%s\n' "$p"
}
# The hook's inputs, checked: a session id of plain characters, a transcript under ~/.claude/projects/
# (physical paths compared, no "..", a plain .jsonl name), a folder that exists.
valid_session() { case "$1" in ''|*[!A-Za-z0-9_-]*) return 1 ;; esac; [ ${#1} -le 100 ]; }
valid_transcript() {
  local t=$1 dir base projects
  case "$t" in ''|*[[:cntrl:]]*|*/../*|../*|*/..) return 1 ;; esac
  base=${t##*/}
  case "$base" in *.jsonl) ;; *) return 1 ;; esac
  case "${base%.jsonl}" in ''|*[!A-Za-z0-9_.-]*) return 1 ;; esac
  dir=$(cd "${t%/*}" 2>/dev/null && pwd -P) || return 1
  projects=$(cd "$CLAUDE_HOME/projects" 2>/dev/null && pwd -P) || return 1
  case "$dir/" in "$projects"/*) ;; *) return 1 ;; esac
  [ -f "$t" ] && [ -r "$t" ]
}
valid_folder() { case "$1" in ''|*[[:cntrl:]]*) return 1 ;; esac; [ -d "$1" ]; }

session_file() { printf '%s\n' "$AUTO_DIR/sessions/$1"; }
read_session() { # sets OFFSET and LAST_RUN for a session
  OFFSET=0 LAST_RUN=0
  [ -f "$(session_file "$1")" ] && read -r OFFSET LAST_RUN < "$(session_file "$1")"
  case "$OFFSET" in ''|*[!0-9]*) OFFSET=0 ;; esac
  case "$LAST_RUN" in ''|*[!0-9]*) LAST_RUN=0 ;; esac
}
write_session() { # write_session <id> <offset> <last run>
  mkdir -p "$AUTO_DIR/sessions" && printf '%s %s\n' "$2" "$3" > "$(session_file "$1").tmp" &&
    mv "$(session_file "$1").tmp" "$(session_file "$1")"
}
backoff_until() {
  local until=0
  [ -f "$AUTO_DIR/backoff" ] && read -r until < "$AUTO_DIR/backoff"
  case "$until" in ''|*[!0-9]*) until=0 ;; esac
  printf '%s\n' "$until"
}
backoff_active() { [ "$(now)" -lt "$(backoff_until)" ]; }
queue_count() { ls "$AUTO_DIR/queue" 2>/dev/null | grep -c '\.md$'; }

# ---- Checking the model's edit before it goes anywhere ----

# Background runs may add rules and revise them in place, nothing more (the update skill's Auto mode). The
# structural checks, in perl: every section whose heading names Security stays exactly as it was and no new one
# appears; every heading stays; every rule the user set firmly (a bullet, with its continuation lines, saying
# never, always, from now on, or recorded as the user's decision) stays word for word, nothing appended to it
# or added under it, and every line saying so anywhere stays exactly; no hunk removes more lines than it adds
# (so a rule dropped here and another added elsewhere does not pass for a revision); at most 12 lines replaced
# per run (additions are bounded by the size limit); no HTML comment, invisible or direction-changing
# characters added.
CHECK_PL='
use strict; use warnings;
sub slurp { local $/; open(my $f, "<:raw", $_[0]) or return undef; my $t = <$f>; utf8::decode($t); return $t }
my ($old, $new) = (slurp($ARGV[0]), slurp($ARGV[1]));
defined $old && defined $new or do { print "the rules could not be read\n"; exit };
$new =~ /\S/ or do { print "the edited rules are empty\n"; exit };
my @ol = split /\n/, $old, -1; my @nl = split /\n/, $new, -1;
sub sections { my @l = @_; my (@out, $cur);
  for (@l) { if (/^#{1,6}\s/) { $cur = /security/i ? [$_] : undef; push @out, $cur if $cur; next } push @$cur, $_ if $cur }
  return join("\n---\n", map { my @s = @$_; pop @s while @s && $s[-1] =~ /^\s*$/; join("\n", @s) } @out) }
sections(@ol) eq sections(@nl) or do { print "the Security section was changed (background runs never change it)\n"; exit };
my %have = map { $_ => 1 } @nl;
for (@ol) { next unless /^#{1,6}\s/; $have{$_} or do { print "a heading was changed or removed\n"; exit } }
my (@blocks, $b);
for (@ol) {
  if (/^\s*[-*] / || /^#/ || /^\s*$/) { push @blocks, $b if defined $b; $b = /^\s*[-*] / ? $_ : undef; next }
  $b .= "\n$_" if defined $b }
push @blocks, $b if defined $b;
my $firm = qr/(^|[^a-z])(never|always|from now on)([^a-z]|$)|the user.s (decision|request|rule|standing)/i;
for (@ol) { next unless /$firm/; $have{$_} or do { print "a rule the user set firmly was reworded or removed\n"; exit } }
for my $k (@blocks) {
  next unless $k =~ $firm;
  # The block whole, at a line start, and followed by a blank line, another rule, a heading or the end: nothing
  # appended to its last line or added under it.
  my $at = -1; my $ok = 0;
  while (($at = index($new, $k, $at + 1)) >= 0) {
    next unless $at == 0 || substr($new, $at - 1, 1) eq "\n";
    my $rest = substr($new, $at + length($k));
    if ($rest eq "" || $rest =~ /^\n(\s*\n|\s*[-*] |#|\z)/) { $ok = 1; last } }
  $ok or do { print "a rule the user set firmly was reworded or added to\n"; exit } }
my %was = map { $_ => 1 } @ol;
for (@nl) { next if $was{$_}; /<!--|-->/ and do { print "an HTML comment was added (it could hide rules)\n"; exit } }
my $hidden = qr/[\x{200B}-\x{200F}\x{202A}-\x{202E}\x{2060}-\x{2069}\x{FEFF}\x{00AD}\x{E0000}-\x{E007F}]/;
my @oh = ($old =~ /$hidden/g); my @nh = ($new =~ /$hidden/g);
@nh <= @oh or do { print "invisible or direction-changing characters were added\n"; exit };
'
check_edit() {
  local old=$1 new=$2 why removed
  [ "$(wc -c < "$new")" -le $(( $(wc -c < "$old") * 2 + 20000 )) ] || { echo "the edited rules grew past the limit"; return; }
  has_conflict_markers "$new" && { echo "the edited rules hold conflict markers"; return; }
  LC_ALL=C grep -q "$(printf '[\001-\010\013\014\016-\037]')" "$new" && { echo "the edited rules hold control characters"; return; }
  why=$(perl -e "$CHECK_PL" -- "$old" "$new" 2>/dev/null) || why="the edited rules could not be checked"
  [ -n "$why" ] && { echo "$why"; return; }
  [ "$(grep -cE '^[[:space:]]*- ' "$new")" -ge "$(grep -cE '^[[:space:]]*- ' "$old")" ] ||
    { echo "a rule was removed (removals wait for the user's /jeeves:update)"; return; }
  # Hunk by hunk: removed lines counted from the diff body (a removed rule shows as "-- ...").
  removed=$(git diff --no-index --no-color -U0 -- "$old" "$new" 2>/dev/null | awk '
    /^@@/ { hunk = 1; split($2, o, ","); split($3, n, ","); oc = (o[2] == "" ? 1 : o[2]) + 0
            nc = (n[2] == "" ? 1 : n[2]) + 0; if (oc > nc) bad = 1; next }
    hunk && /^-/ { r++ }
    END { print (bad ? "removal" : r + 0) }')
  [ "$removed" = removal ] && { echo "a rule was removed (removals wait for the user's /jeeves:update)"; return; }
  [ "$removed" -le 12 ] || { echo "too many lines changed at once ($removed)"; return; }
  [ -z "$(added_lines "$old" "$new" | risk_flags)" ] ||
    { echo "an added line was flagged as risky (such items wait for the user)"; return; }
}

# ---- One run ----

RUN= CHILD= WATCH=
cleanup_run() {
  [ -n "$CHILD" ] && kill -0 "$CHILD" 2>/dev/null && stop_child "$CHILD"
  [ -n "$WATCH" ] && kill "$WATCH" 2>/dev/null && wait "$WATCH" 2>/dev/null
  [ -n "$RUN" ] && [ -d "$RUN" ] && rm -rf "${RUN:?}"
  rm -f "$AUTO_DIR/run-dir" "$AUTO_DIR/child" "$AUTO_DIR/settings-env"
  release_lock
  release_run_lock
}

# The child's command line names its run folder ("/jeeves:update auto <folder>"): a recorded PID is acted on
# only when its command line still says so, never on the PID alone (it may have been reused). Where ps cannot
# show command lines (Git Bash), nothing is stopped: the child then ends at its turn limit.
our_child() { # our_child <pid> <run folder>
  local args
  args=$(ps -o args= -p "$1" 2>/dev/null) || return 1
  case "$args" in *"jeeves:update auto "*"${2##*/}"*) return 0 ;; esac
  return 1
}
# stop_pid <pid>: SIGTERM, up to ten seconds, then SIGKILL (for a process this shell did not start).
stop_pid() {
  local i=0
  kill "$1" 2>/dev/null || return 0
  while kill -0 "$1" 2>/dev/null && [ $i -lt 100 ]; do sleep 0.1; i=$((i + 1)); done
  kill -9 "$1" 2>/dev/null
  return 0
}
# What a run killed outright (SIGKILL, a crash, a reboot) left behind: its child, if still running, and its
# temp folder (an extract of the session). Called at session start, at each run and by status, also when
# background updates are off; does nothing while a live run owns them. Prints what it did.
reap_leftovers() { # reap_leftovers [owned]: "owned" when this process holds the run lock
  local cpid cstart cdir dir
  [ -f "$AUTO_DIR/run-dir" ] || [ -f "$AUTO_DIR/child" ] || return 0
  if [ "$1" != owned ] && [ -f "$LOCK" ] && ! lock_stale; then return 0; fi
  if [ -f "$AUTO_DIR/child" ]; then
    read -r cpid cstart cdir < "$AUTO_DIR/child"
    case "$cpid" in ''|*[!0-9]*) ;; *)
      if kill -0 "$cpid" 2>/dev/null && our_child "$cpid" "$cdir"; then
        stop_pid "$cpid"; echo "stopped a background update's child left running ($cpid)"
      fi ;;
    esac
    rm -f "$AUTO_DIR/child"
  fi
  if [ -f "$AUTO_DIR/run-dir" ]; then
    read -r dir < "$AUTO_DIR/run-dir"
    case "$dir" in */jeeves-auto.??????) [ -d "$dir" ] && rm -rf "${dir:?}" && echo "removed a background update's temp folder" ;; esac
    rm -f "$AUTO_DIR/run-dir"
  fi
}

# A step a kill could leave half done in the data repo is marked first, so the next run puts it right: a
# home/CLAUDE.md written but not committed (put back, if it still holds what this run wrote), a merge of
# origin/main started but not finished (aborted). Without that, every snapshot would skip a dirty repo.
MARK=$AUTO_DIR/in-progress
recover_steps() {
  local step h
  [ -f "$MARK" ] || return 0
  read -r step h < "$MARK"
  case "$step" in
    commit) [ "$(hash_of "$HOME_COPY/CLAUDE.md")" = "$h" ] && [ -n "$(git_repo status --porcelain -- home/CLAUDE.md)" ] &&
              { git_repo reset -q -- home/CLAUDE.md; git_repo checkout -q -- home/CLAUDE.md; } ;;
    merge) [ -f "$REPO/.git/MERGE_HEAD" ] && git_repo merge --abort >/dev/null 2>&1 ;;
  esac
  rm -f "$MARK"
}

# The data repo's lock (lib.sh), keeping this run's own cleanup on exit (take_lock sets its own trap).
data_lock() { take_lock 30 || return 1; trap cleanup_run EXIT; }

# stop_child <pid>: SIGTERM, ten seconds, then SIGKILL.
stop_child() {
  local i=0
  kill "$1" 2>/dev/null
  while kill -0 "$1" 2>/dev/null && [ $i -lt 100 ]; do sleep 0.1; i=$((i + 1)); done
  kill -9 "$1" 2>/dev/null
  wait "$1" 2>/dev/null
}

set_aside() { # set_aside <why>: keeps the proposed change for the user, never pushed
  local id; id=$(date +%Y%m%d-%H%M%S)
  mkdir -p "$AUTO_DIR/set-aside" &&
    { echo "# set aside: $1"; git diff --no-index --no-color -- "$RUN/orig/CLAUDE.md" "$RUN/edit/CLAUDE.md"; } \
      > "$AUTO_DIR/set-aside/$id.patch" 2>/dev/null
  notice "set-aside-$id" "jeeves: a background update was set aside, not pushed: $1. The proposed change is in $AUTO_DIR/set-aside/$id.patch; the next /jeeves:update takes it up."
  OUTCOME="set-aside($1)"
}

# push_scanned: scan and risk-check what main would send, then push exactly the commit checked; one retry
# after merging what arrived. Prints the outcome: pushed | nothing | local | scan | risk.
push_scanned() {
  local sha try
  for try in 1 2 3; do
    sha=$(git_repo rev-parse -q --verify main) || { echo local; return; }
    [ "$(git_repo rev-parse -q --verify refs/remotes/origin/main)" = "$sha" ] && { echo nothing; return; }
    "$BIN/jeeves-scan" >/dev/null 2>&1 || { echo scan; return; }
    "$BIN/jeeves-risk" >/dev/null 2>&1 || { echo risk; return; }
    [ "$(git_repo rev-parse main)" = "$sha" ] || continue   # a snapshot committed meanwhile: check again
    if remote_git push -q origin "$sha:refs/heads/main" >/dev/null 2>&1; then
      git_repo update-ref refs/remotes/origin/main "$sha"; echo pushed; return
    fi
    [ $try = 1 ] || break
    # Refused: what another device pushed is merged in (a conflict leaves it all for the user), then again.
    remote_git fetch -q origin main >/dev/null 2>&1 || break
    take_lock 30 || break
    echo merge > "$MARK"
    if [ "$(git_repo symbolic-ref --short -q HEAD)" = main ] && [ -z "$(git_repo status --porcelain)" ] &&
       git_repo merge -q --no-ff -m "Merge origin/main (background update on $HOST)" origin/main >/dev/null 2>&1; then
      rm -f "$MARK"; release_lock
    else
      git_repo merge --abort >/dev/null 2>&1; rm -f "$MARK"; release_lock; break
    fi
  done
  echo local
}

backoff() { echo $(( $(now) + $1 )) > "$AUTO_DIR/backoff"; }

# Where run folders go. Claude Code reads every CLAUDE.md on the way up from the child's folder (checked
# headless), so no folder on that path may be one another user can write: XDG_RUNTIME_DIR when it is this
# user's and private, else a per-user TMPDIR (macOS, Windows), else ~/.cache/jeeves (never under ~/.claude,
# where Claude Code refuses edits even with an allow rule). Never the shared /tmp.
run_base() {
  local d
  if [ -n "$XDG_RUNTIME_DIR" ] && [ -d "$XDG_RUNTIME_DIR" ] && [ -O "$XDG_RUNTIME_DIR" ] &&
     [ -n "$(find "$XDG_RUNTIME_DIR" -maxdepth 0 -perm 0700 2>/dev/null)" ]; then printf '%s\n' "$XDG_RUNTIME_DIR"; return; fi
  d=${TMPDIR%/}
  case "$d" in ''|/tmp|/var/tmp|/private/tmp) ;; *) [ -d "$d" ] && [ -O "$d" ] && { printf '%s\n' "$d"; return; } ;; esac
  d=${XDG_CACHE_HOME:-$HOME/.cache}/jeeves
  mkdir -p "$d" && chmod 700 "$d" && printf '%s\n' "$d"
}

cmd_run() {
  local sid=$1 tp=$2 folder=$3 nonce top proj f rc result cost ms turns err summary why hash= out end model run_n run_r prescan
  SESSION=$(printf '%s' "$sid" | cut -c1-8)
  umask 077
  { valid_session "$sid" && valid_transcript "$tp" && valid_folder "$folder"; } || { log_run "skipped" "(input refused)"; exit 0; }
  enabled || exit 0
  take_run_lock auto || exit 0   # another run, or the user's /jeeves:update, is at work
  trap cleanup_run EXIT
  trap 'exit 143' TERM INT HUP
  scrub_env supervisor

  # A run killed earlier left its child or folder: put right now.
  out=$(reap_leftovers owned); [ -n "$out" ] && log_run "cleaned" "($(printf '%s\n' "$out" | join_lines))"
  command -v claude >/dev/null 2>&1 || {
    notice problem-claude "jeeves: background updates could not start: the claude command is not on the PATH."
    log_run "skipped" "(no claude)"; backoff 3600; exit 0; }
  have_perl || { notice problem-perl "jeeves: background updates need perl with JSON::PP, which is not installed here."
    log_run "skipped" "(no perl)"; backoff 3600; exit 0; }
  find_data_repo || { notice problem-repo "jeeves: a background update was skipped: $DATA_REPO_PROBLEM."
    log_run "skipped" "(no data repo)"; backoff 3600; exit 0; }
  data_lock && { recover_steps; release_lock; }

  # Sync first: what other devices pushed, and what changed on disk.
  "$BIN/jeeves-snapshot" >/dev/null 2>&1
  # Whether what main would send passes the secrets scan before this run commits anything: a later failure is
  # then this run's own commit.
  prescan=fail; "$BIN/jeeves-scan" >/dev/null 2>&1 && prescan=ok
  if [ "$(git_repo symbolic-ref --short -q HEAD)" != main ] || [ -f "$REPO/.git/MERGE_HEAD" ] ||
     [ -n "$(git_repo status --porcelain -- home)" ]; then
    notice problem-repo "jeeves: background updates are waiting: the data repo is not on a clean main (run /jeeves:update)."
    log_run "skipped" "(data repo not on a clean main)"; backoff 3600; exit 0
  fi

  read_session "$sid"
  RUN=$(mktemp -d "$(run_base)/jeeves-auto.XXXXXX") && RUN=$(cd "$RUN" && pwd -P) || { RUN=; log_run "failed" "(no temp folder)"; exit 0; }
  printf '%s\n' "$RUN" > "$AUTO_DIR/run-dir"
  mkdir -p "$RUN/edit" "$RUN/orig" "$RUN/out" "$RUN/memories" || exit 0
  nonce=$(od -An -N16 -tx1 /dev/urandom 2>/dev/null | tr -d ' \n')
  [ ${#nonce} = 32 ] || { log_run "failed" "(no random nonce)"; exit 0; }
  extract "$tp" "$OFFSET" "$nonce" "$MAX_EXTRACT" "$RUN/offset" > "$RUN/extract.txt" ||
    { log_run "skipped" "(transcript unreadable)"; exit 0; }
  read -r NEW_OFFSET < "$RUN/offset"
  [ -s "$RUN/extract.txt" ] || { write_session "$sid" "$NEW_OFFSET" "$(now)"; log_run "nothing" "(no new messages)"; exit 0; }

  { copy_file "$HOME_COPY/CLAUDE.md" "$RUN/orig/CLAUDE.md" && copy_file "$HOME_COPY/CLAUDE.md" "$RUN/edit/CLAUDE.md"; } ||
    { log_run "failed" "(could not copy the rules)"; exit 0; }
  top=$(git -C "$folder" rev-parse --show-toplevel 2>/dev/null) proj=
  if [ -n "$top" ]; then
    for f in "$top/CLAUDE.md" "$top/.claude/CLAUDE.md"; do
      [ -f "$f" ] && copy_file "$f" "$RUN/project-CLAUDE.md" && { proj=$f; break; }
    done
  fi
  "$BIN/jeeves-memories" list 2>/dev/null | head -25 | {
    i=0
    while IFS="$(printf '\t')" read -r hash src key f; do
      i=$((i + 1))
      cp "$f" "$RUN/memories/$(printf '%02d' $i).md" 2>/dev/null &&
        printf '%02d\t%s\t%s\t%s\n' $i "$hash" "$src" "$key" >> "$RUN/memories/index.tsv"
    done; }
  hash=
  cat > "$RUN/brief.md" <<EOF
# Background update brief

- nonce: $nonce
- device: $HOST
- session extract: $RUN/extract.txt
- global rules to edit: $RUN/edit/CLAUDE.md
- project: ${top:-(none)}
- project rules (read only): $( [ -n "$proj" ] && echo "$RUN/project-CLAUDE.md (a copy of $proj)" || echo "(none)" )
- memories: $RUN/memories/ (index.tsv: number, hash, source, project)
- write: $RUN/out/summary.txt, $RUN/out/queue.md, $RUN/out/memories.tsv
EOF

  # The child: Claude Code headless, confined to the run folder, with no shell, web or MCP tools. The user's
  # settings are not loaded (their permission rules and hooks would widen it): this plugin is loaded by path,
  # and only the settings' model and env carried over. Deny rules win over a skill's allowed-tools (checked
  # headless), so the update skill's own grants give it no shell either.
  run_n=$(native_path "$RUN") run_r=$(rule_path "$RUN") model=$(user_settings model)
  # The settings' env (it may hold keys) stays outside the run folder, which the child can read, and is
  # removed before the child starts.
  user_settings env > "$AUTO_DIR/settings-env"
  (
    scrub_env child
    while IFS= read -r -d '' f; do export "$f"; done < "$AUTO_DIR/settings-env"
    rm -f "$AUTO_DIR/settings-env"
    export JEEVES_AUTO_CHILD=1
    cd "$RUN" || exit 1
    exec claude -p "/jeeves:update auto $run_n" --setting-sources project --plugin-dir "$(native_path "${BIN%/bin}")" \
      ${model:+--model "$model"} --permission-mode dontAsk --strict-mcp-config \
      --no-session-persistence --max-turns 40 --output-format json \
      --allowedTools "Read(/$run_r/**)" "Edit(/$run_r/edit/**)" "Write(/$run_r/out/**)" "Edit(/$run_r/out/**)" \
      --disallowedTools Bash WebFetch WebSearch Agent NotebookEdit \
      < /dev/null > "$RUN/result.json" 2> /dev/null
  ) &
  CHILD=$!
  end=$(( $(now) + DEADLINE ))
  printf '%s %s %s\n' "$CHILD" "$(now)" "$RUN" > "$AUTO_DIR/child"
  # A watchdog beside the child: if this supervisor dies without cleaning up (SIGKILL, a crash), or a minute
  # past the deadline, it stops the child; it ends with the child. Without it a killed supervisor would leave
  # the child running unsupervised.
  (
    trap - EXIT TERM INT HUP
    sup=$$
    while kill -0 "$CHILD" 2>/dev/null; do
      if ! kill -0 "$sup" 2>/dev/null || [ "$(now)" -ge $(( end + 60 )) ]; then
        stop_pid "$CHILD"; [ -d "$RUN" ] && rm -rf "${RUN:?}"
        # The records only while they are still this run's (a new run may have taken over meanwhile).
        [ "$(cut -d' ' -f1 "$AUTO_DIR/child" 2>/dev/null)" = "$CHILD" ] && rm -f "$AUTO_DIR/child"
        [ "$(cat "$AUTO_DIR/run-dir" 2>/dev/null)" = "$RUN" ] && rm -f "$AUTO_DIR/run-dir"
        exit 0
      fi
      sleep 2
    done
  ) < /dev/null > /dev/null 2>&1 &
  WATCH=$!
  # The deadline by the clock (a count of sleeps drifts, most on Windows).
  while kill -0 "$CHILD" 2>/dev/null; do
    if [ "$(now)" -ge "$end" ]; then
      stop_child "$CHILD"; CHILD=; rm -f "$AUTO_DIR/child"
      backoff 3600
      log_run "failed" "(stopped at the deadline, ${DEADLINE}s)"; exit 0
    fi
    sleep 0.5
  done
  wait "$CHILD"; rc=$?; CHILD=
  rm -f "$AUTO_DIR/child"
  kill "$WATCH" 2>/dev/null; wait "$WATCH" 2>/dev/null; WATCH=
  err=$(json_field is_error < "$RUN/result.json")
  cost=$(json_field total_cost_usd < "$RUN/result.json") ms=$(json_field duration_ms < "$RUN/result.json")
  turns=$(json_field num_turns < "$RUN/result.json")
  if [ $rc != 0 ] || [ "$err" != false ]; then
    result=$(json_field result < "$RUN/result.json")
    if printf '%s' "$result" | grep -qiE 'usage limit|rate limit|limit reached|quota|credit balance'; then
      backoff 86400
      notice problem-usage "jeeves: background updates are paused for a day: the plan's usage limit was reached."
      log_run "failed" "(usage limit; paused a day) cost=$cost"
    else
      backoff 3600
      notice problem-child "jeeves: a background update failed (Claude Code exit $rc; signed out, or the plugin not found?) and is paused for an hour; jeeves-auto status shows the last runs."
      log_run "failed" "(child exit $rc) cost=$cost ms=$ms turns=$turns"
    fi
    exit 0
  fi

  OUTCOME=reviewed
  summary=$(tr -d '\r' < "$RUN/out/summary.txt" 2>/dev/null | tr -d '\000-\010\013-\037' | grep -v '^[[:space:]]*$' | head -20 |
    perl -CSD -ne 'print substr($_, 0, 200) =~ s/\n?$/\n/r' 2>/dev/null)
  # It goes into the next session's context (the notice) and a commit message: withheld if anything in it is
  # flagged, looks like a secret, or holds invisible characters.
  if [ -n "$summary" ]; then
    why=
    [ -n "$(printf '%s\n' "$summary" | sed 's/^/summary	/' | risk_flags)" ] && why=1
    printf '%s\n' "$summary" | grep -qE 'sk-ant-|gh[pousr]_|github_pat_|A(KIA|SIA)[0-9A-Z]{16}|xox[abprs]-|PRIVATE KEY|[A-Za-z0-9_./+=-]{40,}' && why=1
    printf '%s' "$summary" | perl -CS -ne 'exit 1 if /[\x{200B}-\x{200F}\x{202A}-\x{202E}\x{2060}-\x{2069}\x{FEFF}\x{00AD}\x{E0000}-\x{E007F}]/' 2>/dev/null || why=1
    [ -n "$why" ] && summary="revised the global rules (the run's summary was withheld: it held flagged text)"
    why=
  fi
  if ! cmp -s "$RUN/orig/CLAUDE.md" "$RUN/edit/CLAUDE.md"; then
    why=$(check_edit "$RUN/orig/CLAUDE.md" "$RUN/edit/CLAUDE.md")
    if [ -n "$why" ]; then set_aside "$why"
    elif ! data_lock; then set_aside "the data repo stayed busy"
    elif [ "$(git_repo symbolic-ref --short -q HEAD)" != main ] || [ -f "$REPO/.git/MERGE_HEAD" ] ||
         [ -n "$(git_repo status --porcelain -- home)" ]; then
      release_lock; set_aside "the data repo was not on a clean main"
    else
      # main may have moved since the copy (a pull, a disk edit): a three-way merge, refused on a conflict.
      if cmp -s "$HOME_COPY/CLAUDE.md" "$RUN/orig/CLAUDE.md"; then cp "$RUN/edit/CLAUDE.md" "$RUN/merged"
      else cp "$HOME_COPY/CLAUDE.md" "$RUN/merged" && git merge-file -q "$RUN/merged" "$RUN/orig/CLAUDE.md" "$RUN/edit/CLAUDE.md" 2>/dev/null ||
        { rm -f "$RUN/merged"; }
      fi
      if [ ! -f "$RUN/merged" ]; then release_lock; set_aside "the global rules changed meanwhile in the same place"
      else
        encrypted && load_key
        [ -n "$summary" ] || summary="revised the global rules"
        echo "commit $(hash_of "$RUN/merged")" > "$MARK"
        if copy_file "$RUN/merged" "$HOME_COPY/CLAUDE.md" && git_repo add -- home/CLAUDE.md &&
           git_repo commit -q -m "learn: $(printf '%s\n' "$summary" | head -1) (background update on $HOST)" \
             -m "$summary" -- home/CLAUDE.md; then
          hash=$(git_repo rev-parse --short HEAD); OUTCOME=learned
        else
          git_repo reset -q -- home/CLAUDE.md 2>/dev/null; git_repo checkout -q -- home/CLAUDE.md 2>/dev/null
          set_aside "the commit failed"
        fi
        rm -f "$MARK"
        unset JEEVES_AGE_KEY
        release_lock
      fi
    fi
  fi

  # Memories reviewed: only hashes this run was given are recorded.
  if [ -s "$RUN/out/memories.tsv" ] && [ -s "$RUN/memories/index.tsv" ]; then
    set --
    while IFS="$(printf '\t')" read -r f result; do
      f=$(printf '%s' "$f" | tr -d ' \r')
      awk -F'\t' -v h="$f" '$2 == h { found = 1 } END { exit !found }' "$RUN/memories/index.tsv" || continue
      set -- "$@" "$f" "$(printf '%s' "${result:-reviewed}" | tr -d '\r' | tr '[:cntrl:]' ' ' | cut -c1-180) (background)"
    done < "$RUN/out/memories.tsv"
    [ $# -gt 0 ] && "$BIN/jeeves-memories" record "$@" >/dev/null 2>&1
  fi

  # Items for the user: project rules and decisions only the user makes.
  if [ -s "$RUN/out/queue.md" ] &&
     grep -qE 'sk-ant-[A-Za-z0-9_-]{20,}|gh[pousr]_[A-Za-z0-9]{30,}|github_pat_|A(KIA|SIA)[0-9A-Z]{16}|xox[abprs]-|PRIVATE KEY' "$RUN/out/queue.md"; then
    echo "- (a queued item was withheld: it held text that looked like a secret)" > "$RUN/out/queue.md"
  fi
  if [ -s "$RUN/out/queue.md" ]; then
    mkdir -p "$AUTO_DIR/queue" &&
    { printf 'session: %s\nproject: %s\nqueued: %s\n\n' "$sid" "${top:-(none)}" "$(date +%Y-%m-%d)"
      tr -d '\r' < "$RUN/out/queue.md" | tr -d '\000-\010\013-\037' | head -c 20000; echo; } \
      > "$AUTO_DIR/queue/$(date +%Y%m%d-%H%M%S)-$SESSION.md"
  fi

  pushed=$(push_scanned)
  # The scan stopped it and this run's own commit is the tip (never pushed): that commit is taken back out
  # and set aside, and the rest is tried again.
  if [ "$pushed" = scan ] && [ -n "$hash" ] && [ "$prescan" = ok ] && data_lock; then
    if [ "$(git_repo rev-parse --short HEAD)" = "$hash" ] && git_repo reset -q --keep HEAD~1; then
      hash=; set_aside "a line looked like a secret"
      release_lock; pushed=$(push_scanned)
    else release_lock
    fi
  fi
  case "$pushed" in
    pushed) OUTCOME="$OUTCOME,pushed" ;;
    scan) OUTCOME="$OUTCOME,not-pushed(scan)"
      notice problem-push "jeeves: a background update found what may be a secret in the outgoing commits and pushed nothing: run /jeeves:update." ;;
    risk) OUTCOME="$OUTCOME,not-pushed(risk)"
      notice problem-push "jeeves: flagged lines in the outgoing commits wait for the user's OK, so a background update pushed nothing: run /jeeves:update." ;;
    local) OUTCOME="$OUTCOME,not-pushed"
      notice problem-push "jeeves: a background update could not push (offline, or GitHub refused); its commits are kept, and the next run or /jeeves:update pushes them." ;;
  esac
  out=$("$BIN/jeeves-apply" 2>&1)
  printf '%s\n' "$out" | grep -q '^jeeves apply: held ' &&
    notice problem-held "jeeves: skills, agents or commands from another device wait for the user's OK: run /jeeves:update."
  [ -n "$hash" ] && notice "change-$hash" "jeeves: a background update changed the global rules: $(printf '%s\n' "$summary" | head -3 | join_lines). Undo: /jeeves:update revert $hash"
  write_session "$sid" "$NEW_OFFSET" "$(now)"
  rm -f "$AUTO_DIR/backoff"
  log_run "$OUTCOME" "cost=$cost ms=$ms turns=$turns"
}

# ---- Hooks ----

cmd_hook_stop() {
  local input sid tp folder size n
  [ "$JEEVES_AUTO_CHILD" = 1 ] && exit 0
  enabled || exit 0
  have_perl || exit 0
  input=$(cat)
  sid=$(printf '%s' "$input" | json_field session_id)
  tp=$(posix_path "$(printf '%s' "$input" | json_field transcript_path)")
  folder=$(posix_path "$(printf '%s' "$input" | json_field cwd)")
  { valid_session "$sid" && valid_transcript "$tp" && valid_folder "$folder"; } || exit 0
  backoff_active && exit 0
  read_session "$sid"
  [ $(( $(now) - LAST_RUN )) -ge "$INTERVAL" ] || exit 0
  [ -f "$LOCK" ] && ! lock_stale && exit 0
  size=$(wc -c < "$tp" | tr -d ' ')
  [ "$OFFSET" -gt "$size" ] && OFFSET=0
  [ $(( size - OFFSET )) -ge "$MIN_NEW" ] || exit 0
  n=$(extract "$tp" "$OFFSET" - "$MIN_NEW" "" 2>/dev/null)
  case "$n" in ''|*[!0-9]*) exit 0 ;; esac
  [ "$n" -ge "$MIN_NEW" ] || exit 0
  # Detached: it outlives this hook and the session; its output goes nowhere, so the hook ends at once.
  ( scrub_env supervisor; exec nohup "$SELF" run "$sid" "$tp" "$folder" < /dev/null > /dev/null 2>&1 ) &
  exit 0
}

cmd_session_start() {
  local f n out
  [ "$JEEVES_AUTO_CHILD" = 1 ] && exit 0
  out=$(reap_leftovers)
  [ -n "$out" ] && log_run "cleaned" "($(printf '%s\n' "$out" | join_lines))"
  if [ ! -f "$CONF" ]; then
    echo "jeeves: background updates (a /jeeves:update review about once an hour of use, in a separate headless session; off by default) are not decided on this device. Tell the user once, in one line, that /jeeves:autoupdate turns them on or keeps them off; either answer ends this reminder."
  fi
  for f in "$AUTO_DIR"/notices/*; do
    [ -f "$f" ] || continue
    cat "$f"; rm -f "$f"
  done
  n=$(( $(queue_count) + $(ls "$AUTO_DIR/set-aside" 2>/dev/null | grep -c '\.patch$') ))
  [ "$n" -gt 0 ] && echo "jeeves: $n item(s) from background updates wait for the user's next /jeeves:update (project rules, changes set aside, or decisions only the user makes)."
  exit 0
}

# ---- Commands for the user and the skills ----

set_enabled() {
  mkdir -p "${CONF%/*}" || exit 1
  { [ -f "$CONF" ] && grep -v '^[[:space:]]*enabled[[:space:]]*=' "$CONF"; echo "enabled=$1"; } > "$CONF.tmp" &&
    mv "$CONF.tmp" "$CONF"
}

case "$1" in
  --hook-stop) cmd_hook_stop ;;
  --session-start) cmd_session_start ;;
  run) shift; [ $# = 3 ] || { echo "usage: jeeves-auto run <session id> <transcript> <folder>" >&2; exit 2; }; cmd_run "$@" ;;
  on)
    have_perl || { say "background updates need perl with JSON::PP, which is not installed here; left off"; exit 1; }
    command -v claude >/dev/null 2>&1 || { say "the claude command is not on the PATH; left off"; exit 1; }
    set_enabled yes
    say "on: a background review at most every $(( INTERVAL / 60 )) minutes per session, once it has grown by $MIN_NEW characters" ;;
  off) set_enabled no; say "off" ;;
  status)
    if [ ! -f "$CONF" ]; then say "off (not decided on this device)"; elif enabled; then say "on"; else say "off"; fi
    say "at most every $(( INTERVAL / 60 )) minutes per session, after $MIN_NEW new characters; deadline $(( DEADLINE / 60 )) minutes"
    if [ -f "$LOCK" ] && ! lock_stale; then
      say "at work now ($(lock_owner), $(( $(now) - $(cut -d' ' -f2 "$LOCK") ))s)$( [ -f "$AUTO_DIR/child" ] && echo ", child PID $(cut -d' ' -f1 "$AUTO_DIR/child")")"
    else
      out=$(reap_leftovers); [ -n "$out" ] && say "$out"
    fi
    backoff_active && say "paused after a failure for $(( ($(backoff_until) - $(now)) / 60 + 1 )) more minutes"
    say "waiting for the user: $(queue_count) queued item(s), $(ls "$AUTO_DIR/set-aside" 2>/dev/null | grep -c '\.patch$') set-aside change(s)"
    [ -f "$AUTO_DIR/log" ] && { say "last runs:"; tail -5 "$AUTO_DIR/log" | sed 's/^/  /'; }
    exit 0 ;;
  hold)
    i=0 wait=${JEEVES_AUTO_HOLD_WAIT:-100}   # under the Bash tool's two-minute limit
    while ! take_run_lock manual; do
      # Another /jeeves:update of the user's holds it: one more hold, so its release does not free this one.
      if [ "$(lock_owner)" = manual ]; then
        read -r p0 s0 o0 c0 < "$LOCK"; case "$c0" in ''|*[!0-9]*) c0=1 ;; esac
        printf '%s %s manual %s\n' "$$" "$(now)" $((c0 + 1)) > "$LOCK"; break
      fi
      [ $i -ge $(( wait * 10 )) ] && { say "a background update is still at work; try again in a few minutes"; exit 1; }
      [ $i = 0 ] && say "a background update is at work; waiting for it to finish (up to $wait seconds)"
      sleep 0.1; i=$((i + 1))
    done
    say "held" ;;
  release)
    if [ "$(lock_owner)" = manual ]; then
      read -r p0 s0 o0 c0 < "$LOCK"; case "$c0" in ''|*[!0-9]*) c0=1 ;; esac
      if [ "$c0" -gt 1 ]; then printf '%s %s manual %s\n' "$p0" "$s0" $((c0 - 1)) > "$LOCK"; else rm -f "$LOCK"; fi
    fi
    say "released" ;;
  queue)
    if [ "$2" = clear ]; then
      shift 2
      for id in "$@"; do
        case "$id" in ''|*/*|*\\*|.*) say "not a queue item: $id"; continue ;; esac
        rm -f "$AUTO_DIR/queue/${id%.md}.md"
        case "$id" in *.patch) rm -f "$AUTO_DIR/set-aside/$id" ;; esac
      done
      say "cleared"
    else
      for f in "$AUTO_DIR"/queue/*.md; do [ -f "$f" ] && { echo "=== ${f##*/}"; cat "$f"; echo; }; done
      for f in "$AUTO_DIR"/set-aside/*.patch; do [ -f "$f" ] && { echo "=== ${f##*/} (set aside)"; cat "$f"; echo; }; done
    fi
    exit 0 ;;
  *) echo "usage: jeeves-auto status | on | off | queue [clear <id>...] | hold | release" >&2; exit 2 ;;
esac
