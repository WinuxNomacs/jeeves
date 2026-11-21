// Package proc starts and stops the processes Jeeves supervises, the same way on every system it runs on:
// the worker (started detached by its first caller), the worker's children, and a background update's
// headless Claude Code child. Each system's details stay in proc_unix.go and proc_windows.go.
package proc

import (
	"os/exec"
	"time"
)

// Waited is a started process's end, which any number of goroutines may wait for (a supervisor's deadline
// loop and a signal's cleanup both do).
type Waited struct {
	done chan struct{}
	err  error
}

// Wait starts waiting for cmd (already started).
func Wait(cmd *exec.Cmd) *Waited {
	w := &Waited{done: make(chan struct{})}
	go func() { w.err = cmd.Wait(); close(w.done) }()
	return w
}

// Done is closed when the process has ended.
func (w *Waited) Done() <-chan struct{} { return w.done }

// Err waits for the process and gives what cmd.Wait gave.
func (w *Waited) Err() error { <-w.done; return w.err }

// StopWithin stops cmd, waiting for it through done: a graceful stop where the system has one, then a
// forced one after grace.
func StopWithin(cmd *exec.Cmd, done <-chan error, grace time.Duration) { stop(cmd, done, grace) }
