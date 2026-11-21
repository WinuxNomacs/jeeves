//go:build !windows

package proc

import (
	"os/exec"
	"syscall"
	"time"
)

// StartDetached starts cmd as its own session, so the end of the caller's (a hook's process group) does
// not end it.
func StartDetached(cmd *exec.Cmd) error {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	return cmd.Start()
}

// Contained is a started child whose whole tree a supervisor stops: on Unix its own process group.
type Contained struct{ cmd *exec.Cmd }

// StartContained starts cmd in a process group of its own, so a stop reaches what it starts too.
func StartContained(cmd *exec.Cmd) (*Contained, error) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return &Contained{cmd}, nil
}

// Stop ends the child's group: SIGTERM, then SIGKILL after grace, waiting through done.
func (c *Contained) Stop(w *Waited, grace time.Duration) {
	pg := -c.cmd.Process.Pid
	syscall.Kill(pg, syscall.SIGTERM)
	select {
	case <-w.Done():
	case <-time.After(grace):
		syscall.Kill(pg, syscall.SIGKILL)
		<-w.Done()
	}
}

// Close lets go of the child (nothing to release on Unix).
func (c *Contained) Close() {}

// stop ends a child past its deadline: SIGTERM (its cleanup runs), then SIGKILL after grace.
func stop(cmd *exec.Cmd, done <-chan error, grace time.Duration) {
	cmd.Process.Signal(syscall.SIGTERM)
	select {
	case <-done:
	case <-time.After(grace):
		cmd.Process.Kill()
		<-done
	}
}

// Alive reports whether a process with this PID exists (kill -0).
func Alive(pid int) bool { return pid > 0 && syscall.Kill(pid, 0) == nil }

// StopPID ends a process this one did not start: SIGTERM, up to grace for it to go, then SIGKILL.
func StopPID(pid int, grace time.Duration) {
	if pid <= 0 || syscall.Kill(pid, syscall.SIGTERM) != nil {
		return
	}
	for end := time.Now().Add(grace); time.Now().Before(end) && Alive(pid); {
		time.Sleep(100 * time.Millisecond)
	}
	syscall.Kill(pid, syscall.SIGKILL)
}
