// Package worker runs the commands that change the data repo or ~/.claude one at a time per machine. A
// caller writes a request into ~/.claude/jeeves/queue/ and waits for its answer; one worker process per
// queue runs the requests in order, each as a child process of its own (`jeeves <command> <arguments>`,
// the same code a direct run uses), and answers with the child's output and exit status. Nothing listens
// on a socket: the queue is a folder, and every file in it is written to a temporary name and renamed.
//
// Files in the queue, by request ID (time-ordered, so a name sort is arrival order):
//
//	<id>.req   the request (JSON), removed once answered
//	<id>.out   the child's output, framed (frame.go); <id>.raw what it wrote past the frames (a crash)
//	<id>.ans   the answer (JSON: exit status), written last
//	worker.lock  held (an OS lock) by the live worker; rejected/ requests that could not be run
package worker

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/WinuxNomacs/jeeves/internal/lib"
)

// Version is the request format's version; a worker runs only requests of its own.
const Version = 1

// Request is one queued command.
type Request struct {
	Version int               `json:"version"`
	ID      string            `json:"id"`
	Exe     string            `json:"exe"` // the program that queued it; only a worker of the same program runs it
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Dir     string            `json:"dir"`
	Env     map[string]string `json:"env"` // the passed variables the caller had set (passed.go)
	Hook    bool              `json:"hook"`
}

// Answer is a request's outcome; the output is in <id>.out.
type Answer struct {
	Exit int `json:"exit"`
}

// Queue is the queue folder of a ~/.claude.
func Queue(e lib.Env) string { return e.ClaudeHome + "/jeeves/queue" }

// Queued reports whether a command runs through the worker: those that change the data repo or ~/.claude.
func Queued(cmd string, args []string) bool {
	switch cmd {
	case "snapshot", "apply", "restore", "cleanup":
		return true
	case "memories":
		_, rest := lib.ParseDataRepoArg(args)
		return len(rest) > 0 && rest[0] == "record"
	}
	return false
}

// newID is time-ordered (nanoseconds, zero-padded), then random, so two callers never collide.
func newID() string {
	b := make([]byte, 4)
	rand.Read(b)
	return fmt.Sprintf("%020d-%s", time.Now().UnixNano(), hex.EncodeToString(b))
}

// writeAtomic writes data to a temporary name in the same folder and renames it over p, so a reader never
// sees half a file.
func writeAtomic(p string, data []byte) error {
	tmp := filepath.Join(filepath.Dir(p), ".tmp-"+filepath.Base(p))
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, p); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

func (r *Request) write(q string) error {
	data, err := json.Marshal(r)
	if err != nil {
		return err
	}
	return writeAtomic(filepath.Join(q, r.ID+".req"), data)
}

// readRequest reads and checks a request file; an error says why it cannot be run.
func readRequest(p string) (*Request, error) {
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	var r Request
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, errors.New("it does not read")
	}
	switch {
	case r.Version != Version:
		return &r, fmt.Errorf("its format version %d is not this worker's %d", r.Version, Version)
	case r.ID != strings.TrimSuffix(filepath.Base(p), ".req"):
		return &r, errors.New("its ID does not match its file name")
	case !Queued(r.Command, r.Args):
		return &r, fmt.Errorf("%q is not a command the worker runs", r.Command)
	}
	return &r, nil
}

// pending lists the request IDs waiting in q, oldest first.
func pending(q string) []string {
	ents, _ := os.ReadDir(q)
	var ids []string
	for _, d := range ents {
		if n := d.Name(); strings.HasSuffix(n, ".req") && !strings.HasPrefix(n, ".") && d.Type().IsRegular() {
			ids = append(ids, strings.TrimSuffix(n, ".req"))
		}
	}
	sort.Strings(ids)
	return ids
}

func readAnswer(p string) (Answer, bool) {
	data, err := os.ReadFile(p)
	if err != nil {
		return Answer{}, false
	}
	var a Answer
	if json.Unmarshal(data, &a) != nil {
		return Answer{Exit: 1}, true
	}
	return a, true
}

// same reports whether two requests would do the same work (the same command, arguments, folder and
// environment), so the worker can run them once.
func same(a, b *Request) bool {
	if a.Command != b.Command || a.Dir != b.Dir || a.Exe != b.Exe || a.Hook != b.Hook ||
		strings.Join(a.Args, "\x00") != strings.Join(b.Args, "\x00") || len(a.Env) != len(b.Env) {
		return false
	}
	for k, v := range a.Env {
		if w, ok := b.Env[k]; !ok || w != v {
			return false
		}
	}
	return true
}
