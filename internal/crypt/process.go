package crypt

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// git's long-running filter protocol, version 2 (gitattributes(5), "Long Running Filter Process"): packets
// are a four-digit hex length (counting itself) and data; "0000" is a flush. git starts the process once
// per command and sends it every file to clean or smudge.

const maxPacketData = 65516

type pktReader struct{ r *bufio.Reader }

// read returns one packet's data; flush is true for "0000".
func (p pktReader) read() (data []byte, flush bool, err error) {
	var head [4]byte
	if _, err := io.ReadFull(p.r, head[:]); err != nil {
		return nil, false, err
	}
	n, err := strconv.ParseUint(string(head[:]), 16, 16)
	if err != nil {
		return nil, false, fmt.Errorf("bad packet length %q", head)
	}
	if n == 0 {
		return nil, true, nil
	}
	if n < 4 {
		return nil, false, fmt.Errorf("bad packet length %d", n)
	}
	data = make([]byte, n-4)
	_, err = io.ReadFull(p.r, data)
	return data, false, err
}

// text reads text packets up to a flush: key=value lines, without their line breaks.
func (p pktReader) text() ([]string, error) {
	var lines []string
	for {
		d, flush, err := p.read()
		if err != nil || flush {
			return lines, err
		}
		lines = append(lines, strings.TrimSuffix(string(d), "\n"))
	}
}

// content reads binary packets up to a flush.
func (p pktReader) content() ([]byte, error) {
	var out []byte
	for {
		d, flush, err := p.read()
		if err != nil || flush {
			return out, err
		}
		out = append(out, d...)
	}
}

type pktWriter struct{ w *bufio.Writer }

func (p pktWriter) packet(data []byte) error {
	if _, err := fmt.Fprintf(p.w, "%04x", len(data)+4); err != nil {
		return err
	}
	_, err := p.w.Write(data)
	return err
}

func (p pktWriter) flush() error {
	if _, err := p.w.WriteString("0000"); err != nil {
		return err
	}
	return p.w.Flush()
}

func (p pktWriter) text(lines ...string) error {
	for _, l := range lines {
		if err := p.packet([]byte(l + "\n")); err != nil {
			return err
		}
	}
	return p.flush()
}

func (p pktWriter) content(data []byte) error {
	for len(data) > 0 {
		n := min(len(data), maxPacketData)
		if err := p.packet(data[:n]); err != nil {
			return err
		}
		data = data[n:]
	}
	return p.flush()
}

// serve answers git until it closes the pipe: the handshake, then each clean or smudge. A file that fails
// is answered status=error (git stops, as the filter is required); the process goes on serving.
func (r *repo) serve(in io.Reader, out, errOut io.Writer) error {
	pr, pw := pktReader{bufio.NewReader(in)}, pktWriter{bufio.NewWriter(out)}
	hello, err := pr.text()
	if err != nil {
		return err
	}
	if !has(hello, "git-filter-client") || !has(hello, "version=2") {
		return errors.New("git did not offer filter protocol version 2")
	}
	if err := pw.text("git-filter-server", "version=2"); err != nil {
		return err
	}
	caps, err := pr.text()
	if err != nil {
		return err
	}
	var ours []string
	for _, c := range []string{"capability=clean", "capability=smudge"} {
		if has(caps, c) {
			ours = append(ours, c)
		}
	}
	if err := pw.text(ours...); err != nil {
		return err
	}
	for {
		head, err := pr.text()
		if err == io.EOF && len(head) == 0 {
			return nil // git is done
		}
		if err != nil {
			return err
		}
		cmd := value(head, "command")
		data, err := pr.content()
		if err != nil {
			return err
		}
		var res []byte
		switch cmd {
		case "clean":
			res, err = r.clean(data)
		case "smudge":
			res, err = r.smudge(data)
		default:
			err = fmt.Errorf("unknown command %q", cmd)
		}
		if err != nil {
			fmt.Fprintf(errOut, "jeeves crypt: could not %s %s: %v\n", map[string]string{"clean": "encrypt", "smudge": "decrypt"}[cmd], value(head, "pathname"), err)
			if err := pw.text("status=error"); err != nil {
				return err
			}
			continue
		}
		if err := pw.text("status=success"); err != nil {
			return err
		}
		if err := pw.content(res); err != nil {
			return err
		}
		if err := pw.flush(); err != nil { // the status stays success
			return err
		}
	}
}

func has(lines []string, s string) bool {
	for _, l := range lines {
		if l == s {
			return true
		}
	}
	return false
}

func value(lines []string, key string) string {
	for _, l := range lines {
		if v, ok := strings.CutPrefix(l, key+"="); ok {
			return v
		}
	}
	return ""
}
