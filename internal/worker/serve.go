package worker

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/WinuxNomacs/jeeves/internal/lib"
)

// Settings with the tests' overrides (seconds), from this process's environment.
func seconds(name string, def int) time.Duration { return secondsOf(os.Getenv(name), def) }

func secondsOf(v string, def int) time.Duration {
	var n int
	if _, err := fmt.Sscanf(v, "%d", &n); err == nil && n > 0 {
		return time.Duration(n) * time.Second
	}
	return time.Duration(def) * time.Second
}

const (
	poll      = 25 * time.Millisecond
	answerAge = 24 * time.Hour // an answer nobody collected is removed after this
)

// Serve is `jeeves worker`: it takes the queue's lock (another worker has it: exit at once), runs the
// requests in order, and exits after JEEVES_WORKER_IDLE seconds (10) with none waiting, or when the oldest
// waiting request is another program's (an updated plugin: the worker of that program serves it).
func Serve(e lib.Env, _ []string, _, stderr io.Writer) int {
	q := Queue(e)
	if os.MkdirAll(q, 0o700) != nil {
		return 1
	}
	lock, ok := tryLock(filepath.Join(q, "worker.lock"), true)
	if !ok {
		return 0
	}
	defer lock.Close()
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(stderr, "jeeves worker: cannot find its own program: %v\n", err)
		return 1
	}
	os.Unsetenv("JEEVES_AGE_KEY") // a session's key never reaches a request's child
	idle := seconds("JEEVES_WORKER_IDLE", 10)
	last := time.Now()
	collect(q)
	for {
		ids := pending(q)
		if len(ids) == 0 {
			if time.Since(last) > idle {
				return 0
			}
			time.Sleep(poll)
			continue
		}
		if !serveOne(q, exe, ids, stderr) {
			return 0
		}
		collect(q)
		last = time.Now()
	}
}

// serveOne answers the oldest request of this program (with any identical snapshot requests waiting
// behind it), setting aside one that cannot be run. Requests another program queued (an updated plugin)
// are left to that program's worker: false when only those wait, so this worker exits and that one can
// start; one waiting over a day (its program gone) is set aside.
func serveOne(q, exe string, ids []string, stderr io.Writer) bool {
	var r *Request
	at := 0
	for i, id := range ids {
		base := filepath.Join(q, id)
		if exists(base + ".ans") { // answered before a crash removed the request
			os.Remove(base + ".req")
			return true
		}
		req, err := readRequest(base + ".req")
		if os.IsNotExist(err) {
			return true
		}
		if err != nil {
			reject(q, id, err)
			return true
		}
		if req.Exe == exe {
			r, at = req, i
			break
		}
		if fi, err := os.Stat(base + ".req"); err == nil && time.Since(fi.ModTime()) > answerAge {
			reject(q, id, fmt.Errorf("it waited over a day for its program %s", req.Exe))
			return true
		}
	}
	if r == nil {
		return false
	}
	batch := []string{r.ID}
	if r.Command == "snapshot" {
		for _, other := range ids[at+1:] {
			if o, err := readRequest(filepath.Join(q, other+".req")); err == nil && same(r, o) {
				batch = append(batch, other)
			}
		}
	}
	exit := run(q, exe, r, stderr)
	base := filepath.Join(q, r.ID)
	for i, b := range batch {
		if i > 0 {
			copyFile(base+".out", filepath.Join(q, b+".out"))
		}
		answer(q, b, exit)
	}
	return true
}

// run runs a request's command as a child of its own, with the request's environment and folder, its
// output framed into <id>.out, and a deadline (JEEVES_WORKER_DEADLINE, 10 minutes).
func run(q, exe string, r *Request, stderr io.Writer) int {
	base := filepath.Join(q, r.ID)
	out, err := os.OpenFile(base+".out", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return 1
	}
	out.Close()
	raw, err := os.OpenFile(base+".raw", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return 1
	}
	defer func() { raw.Close(); os.Remove(base + ".raw") }()
	cmd := childCommand(exe, append([]string{r.Command}, r.Args...))
	cmd.Env = append(childEnv(os.Environ(), r), "JEEVES_IN_WORKER=1", "JEEVES_WORKER_OUT="+base+".out")
	cmd.Dir = r.Dir
	if fi, err := os.Stat(r.Dir); err != nil || !fi.IsDir() {
		cmd.Dir = ""
	}
	cmd.Stdout, cmd.Stderr = raw, raw
	deadline := secondsOf(r.Env["JEEVES_WORKER_DEADLINE"], 600) // the request's: its caller waits as long
	if err := cmd.Start(); err != nil {
		note(base, "jeeves-%s: the worker could not start it: %v\n", r.Command, err)
		return 1
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	var exit int
	select {
	case err := <-done:
		exit = lib.ExitCode(err)
	case <-time.After(deadline):
		stop(cmd, done, 5*time.Second)
		note(base, "jeeves-%s: stopped after %s; nothing more was done, run it again\n", r.Command, deadline)
		exit = 124
	}
	// What the child wrote outside the frames (a crash's message) follows its output.
	if data, err := os.ReadFile(base + ".raw"); err == nil && len(data) > 0 {
		note(base, "%s", data)
	}
	return exit
}

// note appends a message to a request's output as standard error.
func note(base, f string, a ...any) {
	out, err := os.OpenFile(base+".out", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o600)
	if err != nil {
		return
	}
	defer out.Close()
	frameWriter{&framer{f: out}, 'e'}.Write([]byte(fmt.Sprintf(f, a...)))
}

func answer(q, id string, exit int) {
	base := filepath.Join(q, id)
	if writeAtomic(base+".ans", []byte(fmt.Sprintf("{\"exit\":%d}\n", exit))) == nil {
		os.Remove(base + ".req")
	}
}

// reject sets a request that cannot be run aside in rejected/ and answers it with why.
func reject(q, id string, why error) {
	base := filepath.Join(q, id)
	os.MkdirAll(filepath.Join(q, "rejected"), 0o700)
	if os.Rename(base+".req", filepath.Join(q, "rejected", id+".req")) != nil {
		os.Remove(base + ".req")
	}
	os.Remove(base + ".out")
	note(base, "jeeves: the worker did not run this request: %v; nothing changed\n", why)
	writeAtomic(base+".ans", []byte("{\"exit\":1}\n"))
}

// collect removes what callers left behind: answers (and their output) older than a day, and temporary
// files from a write that was cut short.
func collect(q string) {
	ents, _ := os.ReadDir(q)
	for _, d := range ents {
		n := d.Name()
		fi, err := d.Info()
		if err != nil || time.Since(fi.ModTime()) < answerAge {
			continue
		}
		if strings.HasSuffix(n, ".ans") {
			id := strings.TrimSuffix(n, ".ans")
			os.Remove(filepath.Join(q, id+".out"))
			os.Remove(filepath.Join(q, n))
		} else if strings.HasPrefix(n, ".tmp-") || strings.HasSuffix(n, ".out") && !exists(filepath.Join(q, strings.TrimSuffix(n, ".out")+".req")) {
			os.Remove(filepath.Join(q, n))
		}
	}
}

func copyFile(src, dst string) {
	if data, err := os.ReadFile(src); err == nil {
		os.WriteFile(dst, data, 0o600)
	}
}

func exists(p string) bool { _, err := os.Stat(p); return err == nil }
