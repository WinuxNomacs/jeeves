package worker

import (
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"time"
)

// tryLock opens p with no sharing, so a second opener fails while the worker lives; Windows closes the
// handle when the process ends, so a dead worker never holds the queue. The handle is not inheritable.
func tryLock(p string, mark bool) (*os.File, bool) {
	name, err := syscall.UTF16PtrFromString(p)
	if err != nil {
		return nil, false
	}
	h, err := syscall.CreateFile(name, syscall.GENERIC_READ|syscall.GENERIC_WRITE, 0, nil,
		syscall.OPEN_ALWAYS, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return nil, false
	}
	f := os.NewFile(uintptr(h), p)
	if mark { // the worker's PID, for a person looking
		f.Truncate(0)
		f.WriteString(strconv.Itoa(os.Getpid()) + "\n")
	}
	return f, true
}

const (
	detachedProcess        = 0x00000008
	createNewProcessGroup  = 0x00000200
	createBreakawayFromJob = 0x01000000
)

// startDetached starts the worker outside the caller's console and, where the job allows it, outside the
// caller's job, so the end of the hook that started it does not end it.
func startDetached(cmd *exec.Cmd) error {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true,
		CreationFlags: detachedProcess | createNewProcessGroup | createBreakawayFromJob}
	if err := cmd.Start(); err == nil {
		return nil
	}
	c2 := exec.Command(cmd.Path, cmd.Args[1:]...)
	c2.Env, c2.Dir, c2.Stdin, c2.Stdout, c2.Stderr = cmd.Env, cmd.Dir, cmd.Stdin, cmd.Stdout, cmd.Stderr
	c2.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: detachedProcess | createNewProcessGroup}
	*cmd = *c2
	return cmd.Start()
}

// childCommand runs the request's command under a Git Bash shell that stays for its whole run: the data
// repo's lock holds that shell's $$, which the bash scripts' kill -0 can check (it knows no Windows PID).
func childCommand(exe string, args []string) *exec.Cmd {
	return exec.Command("bash", append([]string{"-c", `export JEEVES_LOCK_PID=$$; "$@"`, "_", exe}, args...)...)
}

// stop ends a child past its deadline with its whole tree (the shell and the program under it): Windows
// has no SIGTERM to send a console program from here, so this is the forced stop.
func stop(cmd *exec.Cmd, done <-chan error, grace time.Duration) {
	exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid)).Run()
	select {
	case <-done:
	case <-time.After(grace):
		cmd.Process.Kill()
		<-done
	}
}
