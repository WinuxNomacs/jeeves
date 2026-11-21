# Sourced by each bin/ wrapper as `. <bin>/../lib/launch.sh <command> "$@"`, with jeeves_bin set to the
# wrapper's folder (as the wrapper found it): runs the Go build, dist/jeeves-<os>-<arch>[.exe] <command>, in
# place of this shell (exec), so a signal sent to the caller's PID reaches the program and a lock file holds
# a PID Git Bash's kill -0 knows (on Windows Git Bash keeps a stand-in process with this shell's PID while
# the program runs). Where no build fits, or the build cannot start, it says where Jeeves runs and exits 1.
# Every process counts here (each costs tens of milliseconds in Git Bash), so the folder and the platform
# come from bash itself: no dirname, uname or $(cd ... && pwd).

jeeves_cmd=$1; shift
jeeves_args=" $* "

# The plugin folder as `cd <bin>/.. && pwd` gives it (logical: . and .. removed by name, symlinks kept), so a
# path the Go build records (git's filter settings) reads the same however the wrapper was called.
jeeves_root() {
  local p=${jeeves_bin//\\//} seg IFS=/ out=() d
  case "$p" in
    /*) p=$p/.. ;;
    # A Windows path (git starts a filter by the path in its config, which may be in Windows form, D:/...):
    # Git Bash's form, /d/..., with no process started.
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

# jeeves_cannot <why>: the command cannot run on this machine. A hook never blocks a session or shows an
# error: it exits 0, and only the session-start snapshot says, once a session, why Jeeves does nothing here.
jeeves_cannot() {
  case "$jeeves_cmd$jeeves_args" in
    "hook-edit "*|*" --hook-stop "*|*" --session-start "*) exit 0 ;;
    *" --hook "*)
      echo "jeeves: Jeeves does nothing on this machine: $1; it runs on macOS and Linux (amd64 or arm64) and Windows (amd64)"
      exit 0 ;;
  esac
  echo "jeeves-$jeeves_cmd: $1; Jeeves runs on macOS and Linux (amd64 or arm64) and Windows (amd64)" >&2
  exit 1
}

case "$OSTYPE" in
  darwin*) jeeves_os=darwin ;; linux*) jeeves_os=linux ;; msys*|cygwin*) jeeves_os=windows ;; *) jeeves_os=$OSTYPE ;;
esac
case "$HOSTTYPE" in x86_64|amd64) jeeves_arch=amd64 ;; arm64|aarch64) jeeves_arch=arm64 ;; *) jeeves_arch=$HOSTTYPE ;; esac
jeeves_exe=$JEEVES_ROOT/dist/jeeves-$jeeves_os-$jeeves_arch
[ $jeeves_os = windows ] && jeeves_exe=$jeeves_exe.exe
[ -f "$jeeves_exe" ] || jeeves_cannot "no Go build for $jeeves_os-$jeeves_arch in dist/"
# Not executable (an install copy that lost the bit): checked with a builtin first.
[ -x "$jeeves_exe" ] || jeeves_cannot "the Go build dist/${jeeves_exe##*/} is not executable"
# JEEVES_LOCK_PID: this shell's $$, which exec keeps and the program writes into lock files. A build of the
# wrong format fails the exec after bash's own message; execfail lets this script go on to say why.
shopt -s execfail
JEEVES_ROOT=$JEEVES_ROOT JEEVES_LOCK_PID=$$ exec "$jeeves_exe" "$jeeves_cmd" "$@"
jeeves_cannot "the Go build dist/${jeeves_exe##*/} did not start"
