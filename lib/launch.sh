# Sourced by each ported command's bin/ wrapper as `. <bin>/../lib/launch.sh <command> "$@"`, with
# jeeves_bin set to the wrapper's folder (as the wrapper found it): runs the Go build,
# dist/jeeves-<os>-<arch>[.exe] <command>. While a command still has its bash version, lib/<command>.sh,
# that runs where no build fits or the build cannot start, saying why once per reason (silently when dist/ is
# missing altogether: such a copy ships bash only), and JEEVES_NO_GO=1 forces it. A command without one
# (its bash version removed once the Go build passed parity on every OS) stops there, naming the machines
# Jeeves has builds for.
# Every process counts here (each costs tens of milliseconds in Git Bash), so the folder and the platform
# come from bash itself: no dirname, uname or $(cd ... && pwd).

jeeves_cmd=$1; shift

# The plugin folder as `cd <bin>/.. && pwd` gives it (logical: . and .. removed by name, symlinks kept), so a
# path the Go build records (git's filter settings) reads the same as the bash version's.
jeeves_root() {
  local p=${jeeves_bin//\\//} seg IFS=/ out=() d
  case "$p" in
    /*) p=$p/.. ;;
    # A Windows path (git starts a filter by the path in its config, which a bash script's git config call
    # wrote as D:/...): Git Bash's form, /d/..., with no process started.
    [A-Za-z]:/*)
      d=${p%%:*}
      case "$d" in A) d=a ;; B) d=b ;; C) d=c ;; D) d=d ;; E) d=e ;; F) d=f ;; G) d=g ;; H) d=h ;; I) d=i ;;
        J) d=j ;; K) d=k ;; L) d=l ;; M) d=m ;; N) d=n ;; O) d=o ;; P) d=p ;; Q) d=q ;; R) d=r ;; S) d=s ;;
        T) d=t ;; U) d=u ;; V) d=v ;; W) d=w ;; X) d=x ;; Y) d=y ;; Z) d=z ;; esac
      p=/$d${p#?:}/.. ;;
    *) p=$PWD/$p/.. ;;
  esac
  set -f
  for seg in $p; do
    case "$seg" in
      ''|.) ;;
      ..) [ ${#out[@]} -gt 0 ] && unset "out[$((${#out[@]} - 1))]" ;;
      *) out+=("$seg") ;;
    esac
  done
  set +f
  printf -v JEEVES_ROOT '/%s' "${out[*]}"
}
jeeves_root

jeeves_bash() { exec bash "$JEEVES_ROOT/lib/$jeeves_cmd.sh" "$@"; }
jeeves_has_bash() { [ -f "$JEEVES_ROOT/lib/$jeeves_cmd.sh" ]; }
# Says, once per reason, that the bash version runs instead of the Go build; with no bash version, says why
# the command cannot run and exits 1.
jeeves_fallback() {
  jeeves_has_bash || {
    echo "jeeves-$jeeves_cmd: $1; Jeeves runs on macOS and Linux (amd64 or arm64) and Windows (amd64)" >&2; exit 1; }
  local mark=${CLAUDE_HOME:-${CLAUDE_CONFIG_DIR:-$HOME/.claude}}/jeeves/go-fallback
  if [ "$(cat "$mark" 2>/dev/null)" != "$1" ]; then
    echo "jeeves: running the bash version of $jeeves_cmd: $1" >&2
    { mkdir -p "${mark%/*}" && printf '%s\n' "$1" > "$mark"; } 2>/dev/null
  fi
}

{ [ "$JEEVES_NO_GO" = 1 ] || [ ! -d "$JEEVES_ROOT/dist" ]; } && jeeves_has_bash && jeeves_bash "$@"
case "$OSTYPE" in
  darwin*) jeeves_os=darwin ;; linux*) jeeves_os=linux ;; msys*|cygwin*) jeeves_os=windows ;; *) jeeves_os=$OSTYPE ;;
esac
case "$HOSTTYPE" in x86_64|amd64) jeeves_arch=amd64 ;; arm64|aarch64) jeeves_arch=arm64 ;; *) jeeves_arch=$HOSTTYPE ;; esac
jeeves_exe=$JEEVES_ROOT/dist/jeeves-$jeeves_os-$jeeves_arch
[ $jeeves_os = windows ] && jeeves_exe=$jeeves_exe.exe
if [ ! -f "$jeeves_exe" ]; then
  jeeves_fallback "no Go build for $jeeves_os-$jeeves_arch in dist/"; jeeves_bash "$@"
fi

# A build that cannot start falls back: not executable (an install copy that lost the bit), checked first
# with a builtin; then the run's own 126 (a wrong format) or 127 (not found), after bash's message (which,
# unlike the note, shows at each run: such a build is not one a release ships). Run directly, not captured:
# a subshell would cost as much as the program on Windows.
if [ ! -x "$jeeves_exe" ]; then
  jeeves_fallback "the Go build dist/${jeeves_exe##*/} is not executable"; jeeves_bash "$@"
fi
# A supervisor (jeeves-auto: its wrapper sets jeeves_exec=1) replaces this shell, so a signal sent to the PID
# its caller knows (a SIGTERM to the run) reaches the program, and its lock PID is the program's own; Git
# Bash keeps a stand-in process with that PID for as long as the program runs. A build that cannot start
# falls through to the checks below (execfail).
if [ "$jeeves_exec" = 1 ]; then
  shopt -s execfail
  JEEVES_ROOT=$JEEVES_ROOT JEEVES_LOCK_PID=$$ exec "$jeeves_exe" "$jeeves_cmd" "$@"
  shopt -u execfail
fi
# JEEVES_LOCK_PID: this shell's $$, which the program writes into the data repo's lock: the bash scripts
# check a holder with kill -0, which in Git Bash knows no Windows PID (this shell lives as long as the program).
JEEVES_ROOT=$JEEVES_ROOT JEEVES_LOCK_PID=$$ "$jeeves_exe" "$jeeves_cmd" "$@"
jeeves_rc=$?
if [ $jeeves_rc = 126 ] || [ $jeeves_rc = 127 ]; then
  jeeves_fallback "the Go build dist/${jeeves_exe##*/} did not start"; jeeves_bash "$@"
fi
exit $jeeves_rc
