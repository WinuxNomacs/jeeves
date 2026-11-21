package worker

import (
	"encoding/binary"
	"io"
	"os"
	"sync"
)

// The child's output goes to one file as frames, so the caller can give back standard output and standard
// error in the order they were written: a stream byte ('o' or 'e'), the length (4 bytes, big-endian), then
// the bytes. Each frame is one write to a file opened for appending.

type framer struct {
	mu sync.Mutex
	f  *os.File
}

type frameWriter struct {
	fr     *framer
	stream byte
}

func (w frameWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	buf := make([]byte, 5+len(p))
	buf[0] = w.stream
	binary.BigEndian.PutUint32(buf[1:5], uint32(len(p)))
	copy(buf[5:], p)
	w.fr.mu.Lock()
	defer w.fr.mu.Unlock()
	if _, err := w.fr.f.Write(buf); err != nil {
		return 0, err
	}
	return len(p), nil
}

// ChildOutput is the child's side: when this process runs a request (JEEVES_IN_WORKER=1 and
// JEEVES_WORKER_OUT names its output file), the writers its command prints to. The variable is removed,
// so a command this one runs prints to its own output, as it would outside the worker.
func ChildOutput() (stdout, stderr io.Writer, ok bool) {
	p := os.Getenv("JEEVES_WORKER_OUT")
	if p == "" || os.Getenv("JEEVES_IN_WORKER") != "1" {
		return nil, nil, false
	}
	os.Unsetenv("JEEVES_WORKER_OUT")
	f, err := os.OpenFile(p, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		return nil, nil, false
	}
	fr := &framer{f: f}
	return frameWriter{fr, 'o'}, frameWriter{fr, 'e'}, true
}

// replay writes the frames in r to stdout and stderr; a frame cut short (a child killed mid-write) ends it.
func replay(r io.Reader, stdout, stderr io.Writer) {
	var head [5]byte
	for {
		if _, err := io.ReadFull(r, head[:]); err != nil {
			return
		}
		n := binary.BigEndian.Uint32(head[1:])
		w := stdout
		if head[0] == 'e' {
			w = stderr
		}
		if n > 64<<20 { // not a frame this program wrote
			return
		}
		buf := make([]byte, n)
		if _, err := io.ReadFull(r, buf); err != nil {
			return
		}
		w.Write(buf)
	}
}
