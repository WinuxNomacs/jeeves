//go:build !windows

package worker

import (
	"os"
	"os/exec"
	"strconv"
	"syscall"
)

// tryLock takes an exclusive lock on p without waiting; the system releases it when the process ends, so a
// dead worker never holds the queue (no PID to be reused). The file is closed on exec, so a child does not
// inherit it.
func tryLock(p string, mark bool) (*os.File, bool) {
	f, err := os.OpenFile(p, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, false
	}
	if syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB) != nil {
		f.Close()
		return nil, false
	}
	if mark { // the worker's PID, for a person looking
		f.Truncate(0)
		f.WriteString(strconv.Itoa(os.Getpid()) + "\n")
	}
	return f, true
}

// childCommand runs the request's command as this program; the child writes its own PID into the data
// repo's lock.
func childCommand(exe string, args []string) *exec.Cmd { return exec.Command(exe, args...) }

// openLog opens the worker's log for appending.
func openLog(p string) (*os.File, error) {
	return os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
}
