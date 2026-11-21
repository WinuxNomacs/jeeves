package worker

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/WinuxNomacs/jeeves/internal/lib"
)

// maxWaiting is how many requests may wait; past it a new one is refused (a worker that cannot keep up,
// or one that never starts, must not fill the disk).
const maxWaiting = 100

// Run runs a command: through the worker when it is one the worker runs, or directly (direct is the
// command's own Run) when this process is the worker's child, when JEEVES_NO_WORKER=1, or for any other
// command.
func Run(e lib.Env, cmd string, args []string, stdout, stderr io.Writer,
	direct func(lib.Env, []string, io.Writer, io.Writer) int) int {
	if !Queued(cmd, args) || os.Getenv("JEEVES_IN_WORKER") == "1" || os.Getenv("JEEVES_NO_WORKER") == "1" {
		return direct(e, args, stdout, stderr)
	}
	return Call(e, cmd, args, stdout, stderr)
}

// Call queues the command, makes sure a worker runs, waits for the answer and gives back the command's
// output and exit status as its own. A hook's request (--hook) waits up to JEEVES_HOOK_WAIT seconds (20)
// in all, then leaves the request to finish and says nothing; a command's waits up to JEEVES_QUEUE_WAIT
// seconds (120) to start, then as long as the worker's deadline for it.
func Call(e lib.Env, cmd string, args []string, stdout, stderr io.Writer) int {
	hook := false
	for _, a := range args {
		hook = hook || a == "--hook"
	}
	fail := func(f string, a ...any) int {
		if hook {
			return 0
		}
		fmt.Fprintf(stderr, "jeeves-%s: "+f+"\n", append([]any{cmd}, a...)...)
		return 1
	}
	exe, err := os.Executable()
	if err != nil {
		return fail("cannot find its own program: %v", err)
	}
	q := Queue(e)
	if err := os.MkdirAll(q, 0o700); err != nil {
		return fail("could not queue the request (%v); nothing ran", err)
	}
	if len(pending(q)) >= maxWaiting {
		return fail("over %d requests are waiting in %s; nothing ran (is the worker stuck? its log: worker.log there)", maxWaiting, q)
	}
	dir, _ := os.Getwd()
	r := &Request{Version: Version, ID: newID(), Exe: exe, Command: cmd, Args: args, Dir: dir,
		Env: callerEnv(os.Environ()), Hook: hook}
	if r.Args == nil {
		r.Args = []string{}
	}
	if err := r.write(q); err != nil {
		return fail("could not queue the request (%v, disk full?); nothing ran", err)
	}
	base := filepath.Join(q, r.ID)

	start := time.Now()
	wait := seconds("JEEVES_QUEUE_WAIT", 120)
	if hook {
		wait = seconds("JEEVES_HOOK_WAIT", 20)
	}
	var spawned time.Time
	for {
		if a, ok := readAnswer(base + ".ans"); ok {
			if f, err := os.Open(base + ".out"); err == nil {
				replay(f, stdout, stderr)
				f.Close()
			}
			os.Remove(base + ".out")
			os.Remove(base + ".ans")
			return a.Exit
		}
		// No worker alive (none yet, or one that ended or crashed): start one, at most once a second.
		if time.Since(spawned) > time.Second && !workerAlive(q) {
			spawned = time.Now()
			if err := spawn(exe, q); err != nil && !hook {
				fmt.Fprintf(stderr, "jeeves-%s: could not start the worker: %v\n", cmd, err)
			}
		}
		limit := wait
		if _, err := os.Stat(base + ".out"); err == nil && !hook { // running: the worker's deadline applies
			limit = wait + seconds("JEEVES_WORKER_DEADLINE", 600) + 10*time.Second
		}
		if time.Since(start) > limit {
			if hook {
				return 0 // the request stays and runs; a hook never holds up the session
			}
			return fail("still queued after %s, behind another Jeeves run; it will run, check later with jeeves-status", limit.Round(time.Second))
		}
		time.Sleep(poll)
	}
}

// workerAlive reports whether a worker holds the queue's lock.
func workerAlive(q string) bool {
	f, ok := tryLock(filepath.Join(q, "worker.lock"), false)
	if ok {
		f.Close()
	}
	return !ok
}

// spawn starts `<exe> worker` detached, with no input, its output to worker.log in the queue folder.
func spawn(exe, q string) error {
	logPath := filepath.Join(q, "worker.log")
	if fi, err := os.Stat(logPath); err == nil && fi.Size() > 1<<20 {
		os.Remove(logPath)
	}
	log, err := os.OpenFile(logPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer log.Close()
	c := newCommand(exe, "worker")
	c.Stdout, c.Stderr = log, log
	c.Dir = os.TempDir() // not the caller's folder, which the worker would keep in use (Windows: undeletable)
	if err := startDetached(c); err != nil {
		return err
	}
	return c.Process.Release()
}

var newCommand = exec.Command
