# Sourced by each ported command's bin/ wrapper as `. <bin>/../lib/launch.sh <command> "$@"`, with
# jeeves_bin set to the wrapper's folder (as the wrapper found it): runs the Go
# build, dist/jeeves-<os>-<arch>[.exe] <command>, when dist/ holds one for this machine; otherwise, or when
# it cannot start, the bash version, lib/<command>.sh, saying why once per reason (not when dist/ is missing
# altogether: such a copy ships bash only). JEEVES_NO_GO=1 always runs the bash version.
# Every process counts here (each costs tens of milliseconds in Git Bash), so the folder and the platform
# come from bash itself: no dirname, uname or $(cd ... && pwd).

jeeves_cmd=$1; shift

# The plugin folder as `cd <bin>/.. && pwd` gives it (logical: . and .. removed by name, symlinks kept), so a
# path the Go build records (git's filter settings) reads the same as the bash version's.
jeeves_root() {
  local p=$jeeves_bin seg IFS=/ out=()
  case "$p" in /*) p=$p/.. ;; *) p=$PWD/$p/.. ;; esac
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
# Says, once per reason, that the bash version runs instead of the Go build.
jeeves_fallback() {
  local mark=${CLAUDE_HOME:-${CLAUDE_CONFIG_DIR:-$HOME/.claude}}/jeeves/go-fallback
  if [ "$(cat "$mark" 2>/dev/null)" != "$1" ]; then
    echo "jeeves: running the bash version of $jeeves_cmd: $1" >&2
    { mkdir -p "${mark%/*}" && printf '%s\n' "$1" > "$mark"; } 2>/dev/null
  fi
}

{ [ "$JEEVES_NO_GO" = 1 ] || [ ! -d "$JEEVES_ROOT/dist" ]; } && jeeves_bash "$@"
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
JEEVES_ROOT=$JEEVES_ROOT "$jeeves_exe" "$jeeves_cmd" "$@"
jeeves_rc=$?
if [ $jeeves_rc = 126 ] || [ $jeeves_rc = 127 ]; then
  jeeves_fallback "the Go build dist/${jeeves_exe##*/} did not start"; jeeves_bash "$@"
fi
exit $jeeves_rc
