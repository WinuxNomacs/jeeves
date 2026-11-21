package lib

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"
)

// StateFile is the sync record: "<hash>\t<path>" for each path, the hash it had when last synced here.
func (r *Repo) StateFile() string { return r.StateDir + "/hashes" }

// BaseOf is the hash recorded for a path at the last sync on this device, "" when never synced.
func (r *Repo) BaseOf(p string) string { return r.Bases()[p] }

// SetBase records (or with "-", forgets) the synced hash of a path, as lib.sh's set_base. With the file
// holding that version, its content goes into git's objects too (unreachable, so never pushed), so `git
// cat-file -p <base>` reads it even when git stores the file encrypted. A failed write keeps the old record
// whole: it is written to a temp file and renamed.
func (r *Repo) SetBase(p, hash, file string) error {
	if r.BaseOf(p) == hash {
		return nil
	}
	if file != "" && hash != "-" {
		exec.Command("git", "-C", r.Dir, "hash-object", "-w", "--no-filters", "--", file).Run()
	}
	if err := os.MkdirAll(r.StateDir, 0o777); err != nil {
		return err
	}
	var buf bytes.Buffer
	if b, err := os.ReadFile(r.StateFile()); err == nil {
		for _, l := range strings.SplitAfter(string(b), "\n") {
			if l == "" {
				continue
			}
			f := strings.Split(strings.TrimSuffix(l, "\n"), "\t")
			if len(f) >= 2 && f[1] == p {
				continue
			}
			buf.WriteString(strings.TrimSuffix(l, "\n") + "\n")
		}
	}
	if hash != "-" {
		buf.WriteString(hash + "\t" + p + "\n")
	}
	tmp := r.StateFile() + ".tmp"
	if err := os.WriteFile(tmp, buf.Bytes(), 0o666); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, r.StateFile()); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

// KeepBases keeps every recorded base in git's objects: one local ref (refs/jeeves/bases, never pushed) to
// a tree of them, so gc does not prune them.
func (r *Repo) KeepBases() {
	b, err := os.ReadFile(r.StateFile())
	if err != nil {
		return
	}
	seen := map[string]bool{}
	var hashes []string
	for _, l := range strings.Split(strings.TrimSuffix(string(b), "\n"), "\n") {
		h, _, _ := strings.Cut(l, "\t")
		if !seen[h] {
			seen[h] = true
			hashes = append(hashes, h)
		}
	}
	sort.Strings(hashes)
	check := r.gitCmd("cat-file", "--batch-check")
	check.Stdin = strings.NewReader(strings.Join(hashes, "\n") + "\n")
	out, err := check.Output()
	if err != nil {
		return
	}
	var tree strings.Builder
	for _, l := range strings.Split(strings.TrimSuffix(string(out), "\n"), "\n") {
		if f := strings.Fields(l); len(f) >= 2 && f[1] == "blob" {
			tree.WriteString("100644 blob " + f[0] + "\t" + f[0] + "\n")
		}
	}
	mk := r.gitCmd("mktree")
	mk.Stdin = strings.NewReader(tree.String())
	t, err := mk.Output()
	if err != nil {
		return
	}
	id := strings.TrimSpace(string(t))
	if cur, _ := r.gitCmd("rev-parse", "-q", "--verify", "refs/jeeves/bases").Output(); strings.TrimSpace(string(cur)) == id {
		return
	}
	r.gitCmd("update-ref", "-m", "jeeves: merge bases", "refs/jeeves/bases", id).Run()
}

// gitCmd is git in the data repo with the scripts' options, not yet run.
func (r *Repo) gitCmd(args ...string) *exec.Cmd {
	return exec.Command("git", append([]string{"-C", r.Dir, "-c", "core.autocrlf=false",
		"-c", "core.safecrlf=false", "-c", "core.excludesFile=", "-c", "core.hooksPath=" + r.NoHooksDir(), "-c", "core.fsmonitor=false"}, args...)...)
}

// copyInto copies src to tmp with its mode and modification time (cp -p). JEEVES_TEST_COPY_HOOK, the
// tests' seam for failures, names a program run instead, with cp's arguments (-p <src> <dst>): it can fail,
// or change the files around the copy, as a full disk or a session writing meanwhile would.
func copyInto(src, tmp string) error {
	if hook := os.Getenv("JEEVES_TEST_COPY_HOOK"); hook != "" {
		cmd := Command(hook, "-p", src, tmp)
		cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
		return cmd.Run()
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	fi, err := in.Stat()
	if err != nil {
		return err
	}
	out, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, fi.Mode().Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	os.Chmod(tmp, fi.Mode().Perm())
	return os.Chtimes(tmp, fi.ModTime(), fi.ModTime())
}

// CopyFile copies a file atomically (a temp file beside the target, then a rename), creating folders. A
// failed copy leaves the target as it was.
func CopyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o777); err != nil {
		return err
	}
	tmp := dst + ".sync-tmp"
	if err := copyInto(src, tmp); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, dst); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

// WriteTarget is where a write to a ~/.claude file goes: through a link to a file (a dotfiles setup, say)
// to the file it names, so the link stays; the path itself otherwise.
func WriteTarget(p string) string {
	if fi, err := os.Lstat(p); err == nil && fi.Mode()&fs.ModeSymlink != 0 {
		if t, err := filepath.EvalSymlinks(p); err == nil {
			if ti, err := os.Stat(t); err == nil && ti.Mode().IsRegular() {
				return t
			}
		}
	}
	return p
}

// Copy results of CopySettled.
const (
	Copied   = 0
	Failed   = 1
	Settling = 2
)

// CopySettled copies a file that may be being written (a ~/.claude file): the copy must hash as the source
// just after it, or it is tried again, three times a second apart. Returns the hash of what was copied and
// Copied, or Failed (the copy failed), or Settling (the file kept changing, or was deleted meanwhile; the
// target is left as it was).
func CopySettled(src, dst string) (string, int) {
	if os.MkdirAll(filepath.Dir(dst), 0o777) != nil {
		return "", Failed
	}
	tmp := dst + ".sync-tmp"
	for i := 0; i < 3; i++ {
		if err := copyInto(src, tmp); err != nil {
			os.Remove(tmp)
			if _, serr := os.Lstat(src); serr != nil {
				return "", Settling
			}
			return "", Failed
		}
		h, _ := HashOf(tmp)
		if s, _ := HashOf(src); h == s {
			if os.Rename(tmp, dst) != nil {
				os.Remove(tmp)
				return "", Failed
			}
			return h, Copied
		}
		if i < 2 {
			time.Sleep(time.Second)
		}
	}
	os.Remove(tmp)
	return "", Settling
}

// StagedIs: the staged content of a data repo path (decrypted) hashes to the given plain hash, so what a
// commit would record is what was copied. Through the clean cache when the file is stored encrypted, so no
// key is read.
func (r *Repo) StagedIs(p, hash string) bool {
	out, err := r.gitCmd("rev-parse", "-q", "--verify", ":"+p).Output()
	if err != nil {
		return false
	}
	blob := strings.TrimSpace(string(out))
	if blob == hash {
		return true
	}
	c := r.CryptDir + "/clean/" + hash
	if fi, err := os.Stat(c); err == nil && fi.Size() > 0 {
		if h, _ := HashOf(c); h == blob {
			return true
		}
	}
	plain, err := r.gitCmd("cat-file", "--filters", ":"+p).Output()
	if err != nil {
		plain = nil
	}
	return BlobHash(plain) == hash
}

// HasConflictMarkers: the file holds git conflict markers (an unfinished merge): such a file is never
// installed or recorded unless the user names it.
func HasConflictMarkers(p string) bool {
	fi, err := os.Stat(p)
	if err != nil || !fi.Mode().IsRegular() {
		return false
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return false
	}
	var open, close, mid bool
	for _, l := range strings.Split(string(b), "\n") {
		switch {
		case l == "=======":
			mid = true
		case strings.HasPrefix(l, "<<<<<<<") && (len(l) == 7 || l[7] == ' '):
			open = true
		case strings.HasPrefix(l, ">>>>>>>") && (len(l) == 7 || l[7] == ' '):
			close = true
		}
	}
	return open && close && mid
}

// BadName is why a shared path can't be held on every OS (Windows, macOS), or "" when it can.
func BadName(p string) string {
	for _, c := range p {
		if unicode.IsControl(c) {
			return "its name has a control character (such as a tab or a line break)"
		}
	}
	if strings.ContainsAny(p, `:*?"<>|\`) {
		return "its name has a character Windows does not allow"
	}
	if strings.HasSuffix(p, ".") || strings.HasSuffix(p, " ") || strings.Contains(p, "./") || strings.Contains(p, " /") {
		return "a part of its name ends in a dot or space, which Windows does not allow"
	}
	for _, part := range strings.Split(p, "/") {
		base, _, _ := strings.Cut(strings.ToUpper(part), ".")
		if reservedName(base) {
			return "a part of its name (" + part + ") is a device name Windows reserves"
		}
	}
	return ""
}

// reservedName: a name Windows keeps for a device, whatever extension follows (CON, NUL, COM1, LPT9...).
func reservedName(n string) bool {
	switch n {
	case "CON", "PRN", "AUX", "NUL":
		return true
	}
	if len(n) == 4 && (strings.HasPrefix(n, "COM") || strings.HasPrefix(n, "LPT")) && n[3] >= '1' && n[3] <= '9' {
		return true
	}
	return false
}

// CaseCollisions are the paths that differ only in case from another: they collide on Windows and macOS.
func CaseCollisions(paths []string) map[string]bool {
	n := map[string]int{}
	for _, p := range paths {
		n[strings.ToLower(p)]++
	}
	out := map[string]bool{}
	for _, p := range paths {
		if n[strings.ToLower(p)] > 1 {
			out[p] = true
		}
	}
	return out
}

// ConfValue is a setting from jeeves.conf at the top of the data repo (shared by all devices), the last
// line that sets it, or "".
func (r *Repo) ConfValue(name string) string {
	b, err := os.ReadFile(r.Dir + "/jeeves.conf")
	if err != nil {
		return ""
	}
	v := ""
	for _, l := range strings.Split(strings.ReplaceAll(string(b), "\r", ""), "\n") {
		t := strings.TrimLeft(l, " \t\v\f")
		if !strings.HasPrefix(t, name) {
			continue
		}
		t = strings.TrimLeft(t[len(name):], " \t\v\f")
		if !strings.HasPrefix(t, "=") {
			continue
		}
		v = strings.TrimRight(strings.TrimLeft(t[1:], " \t\v\f"), " \t\v\f")
	}
	return v
}

// MachineID is this machine's own ID (random, made once, kept in ~/.claude outside the synced files): two
// devices that report the same host name must not share hosts/<host>/. "" when it cannot be kept.
func (e Env) MachineID() string {
	f := e.ClaudeHome + "/.jeeves-machine-id"
	if fi, err := os.Stat(f); err != nil || fi.Size() == 0 {
		b := make([]byte, 8)
		if _, err := rand.Read(b); err != nil {
			return ""
		}
		tmp := f + ".tmp"
		if os.WriteFile(tmp, []byte(fmt.Sprintf("%x", b)), 0o666) != nil || os.Rename(tmp, f) != nil {
			os.Remove(tmp)
			return ""
		}
	}
	b, _ := os.ReadFile(f)
	return strings.TrimRight(string(b), "\n")
}
