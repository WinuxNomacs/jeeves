// Package proc starts and stops the processes Jeeves supervises, the same way on every system it runs on:
// the worker (started detached by its first caller), the worker's children, and a background update's
// headless Claude Code child. Each system's details stay in proc_unix.go and proc_windows.go.
package proc

import (
	"os/exec"
	"time"
)

// Wait starts waiting for cmd (already started) and returns the channel its result arrives on, for Stop
// and a caller's own deadline.
func Wait(cmd *exec.Cmd) <-chan error {
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	return done
}

// StopWithin stops cmd, waiting for it through done: a graceful stop where the system has one, then a
// forced one after grace.
func StopWithin(cmd *exec.Cmd, done <-chan error, grace time.Duration) { stop(cmd, done, grace) }
