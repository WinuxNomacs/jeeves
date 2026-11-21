//go:build !windows

package auto

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/WinuxNomacs/jeeves/internal/proc"
)

// commandLine is a process's command line, as ps shows it.
func commandLine(pid int) (string, bool) {
	out, err := exec.Command("ps", "-o", "args=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(string(out)), true
}

// stopGroup ends a child started in its own process group, and the group: SIGTERM, up to grace, SIGKILL.
func stopGroup(pid int, grace time.Duration) {
	syscall.Kill(-pid, syscall.SIGTERM)
	for end := time.Now().Add(grace); time.Now().Before(end) && proc.Alive(pid); {
		time.Sleep(100 * time.Millisecond)
	}
	syscall.Kill(-pid, syscall.SIGKILL)
}

// needsWatchdog: on Unix a separate process stops the child if the supervisor is killed outright.
const needsWatchdog = true

// ownedBy: the folder is owned by this user (the shell's -O).
func ownedBy(fi interface{ Sys() any }) bool {
	st, ok := fi.Sys().(*syscall.Stat_t)
	return ok && int(st.Uid) == syscall.Geteuid()
}

// umask makes what the run writes private to this user.
func umask() { syscall.Umask(0o077) }

// private: only this user may enter the folder (mode 0700, the shell's find -perm 0700).
func private(fi interface{ Mode() os.FileMode }) bool { return fi.Mode().Perm() == 0o700 }
