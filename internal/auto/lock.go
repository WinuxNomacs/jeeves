package auto

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/WinuxNomacs/jeeves/internal/lib"
	"github.com/WinuxNomacs/jeeves/internal/proc"
)

// ---- The run lock: one background run (or the user's /jeeves:update) per device ----
// "<pid> <start> <owner> [count]". An auto run's lock is stale when its PID is gone or it is older than the
// deadline plus five minutes (a PID reused meanwhile is not mistaken for the run); a manual hold after two
// hours. PIDs are written as the data repo's lock writes them (lib.LockPID), so the bash version, while it
// exists, reads them the same.

// lockFields reads the lock's fields; ok is false when it holds no complete line.
func (s *state) lockFields() ([]string, bool) {
	b, err := os.ReadFile(s.lock)
	if err != nil || !strings.Contains(string(b), "\n") {
		return nil, false
	}
	l, _, _ := strings.Cut(string(b), "\n")
	return strings.Fields(l), true
}

func olderThanAMinute(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && time.Since(fi.ModTime()) > time.Minute
}

func (s *state) lockStale() bool {
	f, ok := s.lockFields()
	if !ok || len(f) < 2 || !isNum(f[1]) {
		return olderThanAMinute(s.lock)
	}
	age := now() - digits(f[1])
	owner := ""
	if len(f) > 2 {
		owner = f[2]
	}
	if owner == "manual" {
		return age > 7200
	}
	return !lib.Alive(f[0]) || age > s.deadline+300
}

func (s *state) lockOwner() string {
	f, ok := s.lockFields()
	if !ok || len(f) < 3 {
		return ""
	}
	return f[2]
}

// createExclusive writes text into a new file, failing when it exists.
func createExclusive(p, text string) bool {
	f, err := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o666)
	if err != nil {
		return false
	}
	_, werr := f.WriteString(text)
	return f.Close() == nil && werr == nil
}

// takeRunLock takes the run lock for owner (auto or manual): a stale one is taken over by one process at a
// time, and only if it still holds what was judged stale.
func (s *state) takeRunLock(owner string) bool {
	if os.MkdirAll(s.dir, 0o777) != nil {
		return false
	}
	line := func() string { return fmt.Sprintf("%s %d %s\n", lib.LockPID(), now(), owner) }
	if createExclusive(s.lock, line()) {
		return true
	}
	if !createExclusive(s.lock+".takeover", lib.LockPID()+"\n") {
		if olderThanAMinute(s.lock + ".takeover") {
			os.Remove(s.lock + ".takeover")
		}
		return false
	}
	held, _ := os.ReadFile(s.lock)
	if s.lockStale() {
		if now, err := os.ReadFile(s.lock); err == nil && string(now) == string(held) {
			os.Remove(s.lock)
		}
	}
	os.Remove(s.lock + ".takeover")
	return createExclusive(s.lock, line())
}

// releaseRunLock removes the lock if it is this run's own.
func (s *state) releaseRunLock() {
	f, ok := s.lockFields()
	if ok && len(f) >= 3 && f[0] == lib.LockPID() && f[2] == "auto" {
		os.Remove(s.lock)
	}
}

// ---- Leftovers of a run killed outright ----

// reapLeftovers puts right what a run killed outright (SIGKILL, a crash, a reboot) left behind: its child, if
// still running, and its temp folder (an extract of the session). Called at session start, at each run and
// by status, also when background updates are off; does nothing while a live run owns them, unless owned
// (this process holds the run lock). Returns what it did.
func (s *state) reapLeftovers(owned bool) []string {
	child, runDir := s.dir+"/child", s.dir+"/run-dir"
	if !exists(runDir) && !exists(child) {
		return nil
	}
	if !owned && exists(s.lock) && !s.lockStale() {
		return nil
	}
	var out []string
	if exists(child) {
		f := strings.Fields(firstLine(child))
		if len(f) >= 1 {
			if pid, err := strconv.Atoi(f[0]); err == nil && isNum(f[0]) {
				dir := ""
				if len(f) >= 3 {
					dir = f[2]
				}
				if proc.Alive(pid) && ourChild(pid, dir) {
					proc.StopPID(pid, 10*time.Second)
					out = append(out, fmt.Sprintf("stopped a background update's child left running (%d)", pid))
				}
			}
		}
		os.Remove(child)
	}
	if exists(runDir) {
		dir := firstLine(runDir)
		if isRunFolder(dir) {
			if fi, err := os.Stat(lib.NativePath(dir)); err == nil && fi.IsDir() && os.RemoveAll(lib.NativePath(dir)) == nil {
				out = append(out, "removed a background update's temp folder")
			}
		}
		os.Remove(runDir)
	}
	return out
}

// isRunFolder: a path whose last part is a run folder's name, jeeves-auto. and six characters.
func isRunFolder(p string) bool {
	i := strings.LastIndex(p, "/")
	if i < 0 {
		return false
	}
	n := p[i+1:]
	return strings.HasPrefix(n, "jeeves-auto.") && len([]rune(n)) == len("jeeves-auto.")+6
}

// ourChild: the process's command line still names the run folder ("/jeeves:update auto <folder>"), so a
// recorded PID is acted on only while it is the run's child, never on the PID alone (it may have been
// reused). Where command lines cannot be read (Windows, where the job object has ended the child anyway),
// nothing is stopped.
func ourChild(pid int, dir string) bool {
	args, ok := commandLine(pid)
	if !ok {
		return false
	}
	base := dir
	if i := strings.LastIndex(dir, "/"); i >= 0 {
		base = dir[i+1:]
	}
	i := strings.Index(args, "jeeves:update auto ")
	return i >= 0 && strings.Contains(args[i+len("jeeves:update auto "):], base)
}

// ---- Steps a kill could leave half done in the data repo ----

// recoverSteps puts right a step marked in-progress: a home/CLAUDE.md written but not committed (put back, if
// it still holds what the run wrote), a merge of origin/main started but not finished (aborted). Without it,
// every snapshot would skip a dirty repo.
func (s *state) recoverSteps(r *lib.Repo) {
	mark := s.dir + "/in-progress"
	if !exists(mark) {
		return
	}
	f := strings.Fields(firstLine(mark))
	step, h := "", ""
	if len(f) > 0 {
		step = f[0]
	}
	if len(f) > 1 {
		h = f[1]
	}
	switch step {
	case "commit":
		if got, _ := lib.HashOf(r.HomeCopy + "/CLAUDE.md"); got == h {
			if st, _ := r.Git("status", "--porcelain", "--", "home/CLAUDE.md"); st != "" {
				r.Git("reset", "-q", "--", "home/CLAUDE.md")
				r.Git("checkout", "-q", "--", "home/CLAUDE.md")
			}
		}
	case "merge":
		if r.Merging() {
			r.Git("merge", "--abort")
		}
	}
	os.Remove(mark)
}

// ---- The watchdog (Unix) ----

// watch is `jeeves auto --watch <child pid> <supervisor pid> <end> <run folder> <auto folder>`: beside a
// run's child, it stops the child (with its process group) if the supervisor dies without cleaning up
// (SIGKILL, a crash), or a minute past the deadline, then removes the run folder and the run's records
// while they are still this run's (a new run may have taken over meanwhile). It ends with the child.
func watch(args []string) int {
	if len(args) != 5 {
		return 2
	}
	child, _ := strconv.Atoi(args[0])
	sup, _ := strconv.Atoi(args[1])
	end, _ := strconv.ParseInt(args[2], 10, 64)
	run, dir := args[3], args[4]
	if child <= 0 || sup <= 0 {
		return 2
	}
	for proc.Alive(child) {
		if !proc.Alive(sup) || now() >= end+60 {
			stopGroup(child, 10*time.Second)
			if fi, err := os.Stat(run); err == nil && fi.IsDir() && isRunFolder(filepath.ToSlash(run)) {
				os.RemoveAll(run)
			}
			if f := strings.Fields(firstLine(dir + "/child")); len(f) > 0 && f[0] == strconv.Itoa(child) {
				os.Remove(dir + "/child")
			}
			if lib.NativePath(firstLine(dir+"/run-dir")) == run {
				os.Remove(dir + "/run-dir")
			}
			return 0
		}
		time.Sleep(2 * time.Second)
	}
	return 0
}
