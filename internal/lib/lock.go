package lib

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Lock is the data repo's lock, shared with the bash scripts' take_lock: one Jeeves run that changes the
// data repo or ~/.claude at a time. The file holds the holder's PID as bash knows it (on Windows the
// launcher's $$, passed in JEEVES_LOCK_PID: Git Bash's kill -0 knows no Windows PID).
type Lock struct {
	path string
	pid  string
	held bool
}

func lockPID() string {
	if p := os.Getenv("JEEVES_LOCK_PID"); p != "" {
		return p
	}
	return strconv.Itoa(os.Getpid())
}

// createExclusive writes pid into a new file, failing when it exists (bash's noclobber).
func createExclusive(p, pid string) bool {
	f, err := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o666)
	if err != nil {
		return false
	}
	_, werr := f.WriteString(pid + "\n")
	return f.Close() == nil && werr == nil
}

func olderThanAMinute(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && time.Since(fi.ModTime()) > time.Minute
}

// alive is bash's `kill -0 <pid>`.
func alive(pid string) bool {
	n, err := strconv.Atoi(pid)
	if err != nil || n <= 0 {
		return false
	}
	return processAlive(n, pid)
}

// TakeLock waits up to secs for the lock in cryptDir, taking over one whose process is gone (or an empty
// one over a minute old), one waiter at a time under lock.takeover; nil when it is not had in time.
func TakeLock(cryptDir string, secs int) *Lock {
	if os.MkdirAll(cryptDir, 0o777) != nil {
		return nil
	}
	l := &Lock{path: cryptDir + "/lock", pid: lockPID()}
	for i := 0; !createExclusive(l.path, l.pid); i++ {
		if createExclusive(l.path+".takeover", l.pid) {
			pid := catTrim(l.path)
			stale := (pid != "" && !alive(pid)) || (pid == "" && olderThanAMinute(l.path))
			if _, err := os.Stat(l.path); stale && err == nil && catTrim(l.path) == pid {
				os.Remove(l.path)
			}
			os.Remove(l.path + ".takeover")
		} else if olderThanAMinute(l.path + ".takeover") {
			os.Remove(l.path + ".takeover")
		}
		if i >= secs*10 {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	l.held = true
	return l
}

// Release removes the lock if it is still this run's own.
func (l *Lock) Release() {
	if l != nil && l.held && strings.TrimRight(catTrim(l.path), "\n") == l.pid {
		os.Remove(l.path)
	}
	if l != nil {
		l.held = false
	}
}
