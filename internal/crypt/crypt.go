// Package crypt is `jeeves crypt`, git's filter, diff and merge driver for an encrypted data repo (set up in
// its .git/config by lib's cryptSetup; not for running by hand):
//
//	jeeves crypt process                  git's long-running filter: clean and smudge for every file of a
//	                                      checkout or add, in one process
//	jeeves crypt clean [<path>]           plain text on stdin -> age output (needs only the recipient)
//	jeeves crypt smudge [<path>]          age output on stdin -> plain text (needs the key)
//	jeeves crypt textconv <file>          plain text of a stored file, for git diff and log
//	jeeves crypt merge <O> <A> <B> <L>    three-way merge of stored files; the result, encrypted, in <A>
//
// Content that is not age output (history from before encryption) passes through as it is. age output
// differs on every run, so clean keeps a cache (plain hash -> ciphertext, in .git/jeeves/clean/) and gives
// the same ciphertext for the same text; smudge fills it too, so a checked-out file shows no change. A
// failure exits non-zero (or answers error), so git stops (the filter is required) or reports a conflict:
// plain text never goes into a commit.
package crypt

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/WinuxNomacs/jeeves/internal/lib"
)

// repo is the data repo git runs the filter in: its top (the recipient file) and .git/jeeves.
type repo struct {
	e        lib.Env
	top      string
	cryptDir string
	key      func() (string, error) // read once, on first need
}

func openRepo(e lib.Env) (*repo, error) {
	out, err := exec.Command("git", "rev-parse", "--path-format=absolute", "--git-dir", "--show-toplevel").Output()
	if err != nil {
		return nil, errors.New("not in a git repository")
	}
	l := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	if len(l) < 2 {
		return nil, errors.New("not in a git working tree")
	}
	r := &repo{e: e, top: l[1], cryptDir: l[0] + "/jeeves"}
	r.key = sync.OnceValues(r.readKey)
	if err := os.MkdirAll(r.cache(), 0o700); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *repo) cache() string { return r.cryptDir + "/clean" }

// readKey: JEEVES_AGE_KEY (a Jeeves command loaded it), else this device's key store (git run by hand).
func (r *repo) readKey() (string, error) {
	if k := os.Getenv("JEEVES_AGE_KEY"); k != "" {
		return k, nil
	}
	rec := trimmed(r.top + "/recipient")
	if rec == "" {
		rec = trimmed(r.cryptDir + "/unlocked")
	}
	if k := r.e.StoreGet(lib.KeystoreKind(r.cryptDir), rec); k != "" {
		return k, nil
	}
	return "", errors.New("the data repo's key is not in this device's key store: run /jeeves:encryption")
}

func trimmed(p string) string {
	b, _ := os.ReadFile(p)
	return strings.TrimSpace(string(b))
}

// remember keeps a ciphertext for its plain text's hash, written atomically.
func (r *repo) remember(hash string, enc []byte) {
	tmp := fmt.Sprintf("%s/%s.%d", r.cache(), hash, os.Getpid())
	if os.WriteFile(tmp, enc, 0o600) == nil && os.Rename(tmp, r.cache()+"/"+hash) != nil {
		os.Remove(tmp)
	}
}

// clean: plain text -> age output, the cached ciphertext when there is one. A cached entry is checked
// against the plain text when the key is at hand (a Jeeves command loaded it), and dropped when it does
// not decrypt to it; without the key (git run by hand on a locked device) it is served as it is.
func (r *repo) clean(plain []byte) ([]byte, error) {
	if lib.IsAge(plain) {
		return plain, nil
	}
	// The tests' seam: encrypting fails for content holding this text ("*": all), as a broken store or a
	// full disk would.
	if f := os.Getenv("JEEVES_TEST_CRYPT_FAIL"); f != "" && (f == "*" || bytes.Contains(plain, []byte(f))) {
		return nil, errors.New("encryption failed (JEEVES_TEST_CRYPT_FAIL)")
	}
	h := lib.BlobHash(plain)
	if enc, err := os.ReadFile(r.cache() + "/" + h); err == nil && len(enc) > 0 {
		key := os.Getenv("JEEVES_AGE_KEY")
		if key == "" {
			return enc, nil
		}
		if back, err := lib.DecryptWith(key, enc); err == nil && bytes.Equal(back, plain) {
			return enc, nil
		}
		os.Remove(r.cache() + "/" + h)
	}
	rec := trimmed(r.top + "/recipient")
	if rec == "" {
		return nil, errors.New("no recipient file at the top of the data repo")
	}
	// A recipient another device (or anyone who can push) changed is not this device's key: the files
	// would go to a key it never unlocked, perhaps someone else's.
	if unl := trimmed(r.cryptDir + "/unlocked"); unl != "" && unl != rec {
		return nil, errors.New("the data repo's recipient file differs from the key this device unlocked; nothing encrypted: run /jeeves:encryption")
	}
	enc, err := lib.EncryptTo(rec, plain)
	if err != nil {
		return nil, err
	}
	r.remember(h, enc)
	return enc, nil
}

// plain: age output -> plain text; anything else as it is.
func (r *repo) plain(stored []byte) ([]byte, error) {
	if !lib.IsAge(stored) {
		return stored, nil
	}
	key, err := r.key()
	if err != nil {
		return nil, err
	}
	return lib.DecryptWith(key, stored)
}

// smudge: age output -> plain text, remembering the ciphertext so clean gives it back unchanged.
func (r *repo) smudge(stored []byte) ([]byte, error) {
	plain, err := r.plain(stored)
	if err != nil || !lib.IsAge(stored) {
		return plain, err
	}
	r.remember(lib.BlobHash(plain), stored)
	return plain, nil
}

// Run is `jeeves crypt`.
func Run(e lib.Env, args []string, stdout, stderr io.Writer) int {
	fail := func(f string, a ...any) int {
		fmt.Fprintf(stderr, "jeeves crypt: "+f+"\n", a...)
		return 1
	}
	if len(args) == 0 {
		return fail("usage: jeeves-crypt process | clean | smudge | textconv <file> | merge <O> <A> <B> <L>")
	}
	r, err := openRepo(e)
	if err != nil {
		return fail("%v", err)
	}
	what := func() string {
		if len(args) > 1 {
			return args[1]
		}
		return "a file"
	}
	switch args[0] {
	case "process":
		if err := r.serve(os.Stdin, stdout, stderr); err != nil {
			return fail("%v", err)
		}
		return 0
	case "clean", "smudge":
		in, err := io.ReadAll(os.Stdin)
		if err != nil {
			return fail("%v", err)
		}
		var out []byte
		if args[0] == "clean" {
			out, err = r.clean(in)
		} else {
			out, err = r.smudge(in)
		}
		if err != nil {
			return fail("could not %s %s: %v", map[string]string{"clean": "encrypt", "smudge": "decrypt"}[args[0]], what(), err)
		}
		stdout.Write(out)
		return 0
	case "textconv":
		if len(args) != 2 {
			return fail("textconv takes one file")
		}
		data, err := os.ReadFile(args[1])
		if err != nil {
			return fail("%v", err)
		}
		out, err := r.plain(data)
		if err != nil {
			return fail("%v", err)
		}
		stdout.Write(out)
		return 0
	case "merge":
		if len(args) < 4 {
			return fail("merge takes <O> <A> <B> <L>")
		}
		marker := "7"
		if len(args) > 4 && args[4] != "" {
			marker = args[4]
		}
		return r.merge(args[1], args[2], args[3], marker, stderr)
	}
	return fail("usage: jeeves-crypt process | clean | smudge | textconv <file> | merge <O> <A> <B> <L>")
}

// notMerged is what a file gets when it cannot be decrypted to merge: conflict markers and a note, so Jeeves
// refuses to install or record it.
const notMerged = "<<<<<<< jeeves: not merged\n" +
	"The data repo's key could not be read, so this file could not be decrypted to merge it.\n" +
	"Abort the merge (git merge --abort), run /jeeves:encryption, then merge again.\n" +
	"=======\n>>>>>>> jeeves\n"

// merge decrypts the three sides to private temporary files, merges them with git merge-file, and writes
// the result, encrypted, over <A>; the exit status is merge-file's (the number of conflicts). Without the
// key, <A> gets the note above and the status is 129, which makes git stop the merge (git 2.44+; older git
// reports a conflict).
func (r *repo) merge(o, a, b, marker string, stderr io.Writer) int {
	tmp, err := os.MkdirTemp(r.cryptDir, "merge.")
	if err != nil {
		return 1
	}
	defer os.RemoveAll(tmp)
	sides := map[string]string{"O": o, "A": a, "B": b}
	for name, p := range sides {
		data, err := os.ReadFile(p)
		if err == nil {
			data, err = r.plain(data)
		}
		if err != nil {
			if enc, e2 := r.clean([]byte(notMerged)); e2 == nil {
				os.WriteFile(a, enc, 0o600)
			}
			fmt.Fprintf(stderr, "jeeves crypt: %v\n", err)
			return 129
		}
		if os.WriteFile(filepath.Join(tmp, name), data, 0o600) != nil {
			return 1
		}
	}
	cmd := exec.Command("git", "merge-file", "--marker-size="+marker, "-L", "ours", "-L", "base", "-L", "theirs",
		filepath.Join(tmp, "A"), filepath.Join(tmp, "O"), filepath.Join(tmp, "B"))
	cmd.Stderr = stderr
	rc := lib.ExitCode(cmd.Run())
	if rc >= 128 {
		return 1
	}
	merged, err := os.ReadFile(filepath.Join(tmp, "A"))
	if err != nil {
		return 1
	}
	enc, err := r.clean(merged)
	if err != nil || os.WriteFile(a, enc, 0o666) != nil {
		return 1
	}
	return rc
}
