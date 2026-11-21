// Package version holds the commands that change the data repo's history on the skills' behalf: the skills
// edit files, and every commit, merge, revert and push goes through `jeeves commit`, `jeeves merge`,
// `jeeves revert` or `jeeves push`, each checking what it is given (the paths a commit may hold, the
// branches a merge may take, a push only after the secrets and risk checks). The machine's worker runs
// them, one at a time with the other commands that change things; `jeeves git` only reads.
package version

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/WinuxNomacs/jeeves/internal/lib"
)

// session is one command's run: the data repo (found, locked, key loaded) and how it speaks.
type session struct {
	e      lib.Env
	r      *lib.Repo
	name   string
	stdout io.Writer
	stderr io.Writer
	lock   *lib.Lock
}

func (s *session) say(f string, a ...any) { fmt.Fprintf(s.stdout, "jeeves "+s.name+": "+f+"\n", a...) }

// open finds the data repo, refuses a link in it (unless allowLinks: a commit may remove one), takes the
// data repo's lock and loads the key when it is encrypted. nil (having said why) when it cannot go on.
func open(e lib.Env, name string, a lib.DataRepoArg, allowLinks bool, stdout, stderr io.Writer) *session {
	s := &session{e: e, name: name, stdout: stdout, stderr: stderr}
	r, err := lib.FindDataRepo(e, a, stderr)
	if err != nil {
		s.say("%s", err)
		return nil
	}
	s.r = r
	if msg := r.LinkProblem(); msg != "" && !allowLinks {
		s.say("%s", msg)
		return nil
	}
	if r.Encrypted() && !e.LoadKey(r) {
		s.say("the data repo is encrypted and this device is locked: run /jeeves:encryption; nothing changed")
		return nil
	}
	if s.lock = lib.TakeLock(r.CryptDir, 30); s.lock == nil {
		s.say("another Jeeves run is still at work on %s; nothing changed, try again shortly", r.ShellDir)
		return nil
	}
	return s
}

func (s *session) close() {
	if s != nil {
		s.lock.Release()
	}
}

// git runs git in the data repo with the scripts' options; its output goes to the command's standard
// error only when it fails (quiet otherwise). Returns the exit status and the output.
func (s *session) git(args ...string) (int, string) {
	cmd := exec.Command("git", append([]string{"-C", s.r.Dir, "-c", "core.autocrlf=false",
		"-c", "core.safecrlf=false", "-c", "core.excludesFile=", "-c", "core.hooksPath=" + s.r.NoHooksDir(), "-c", "core.fsmonitor=false"}, args...)...)
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	rc := lib.ExitCode(cmd.Run())
	return rc, out.String()
}

// mustGit runs git and, when it fails, shows git's own words and returns false.
func (s *session) mustGit(args ...string) bool {
	rc, out := s.git(args...)
	if rc != 0 {
		fmt.Fprint(s.stderr, out)
	}
	return rc == 0
}

func (s *session) out(args ...string) string {
	_, o := s.git(args...)
	return strings.TrimRight(o, "\n")
}

func (s *session) branch() string { return s.r.Branch() }

func (s *session) short(rev string) string { return s.out("rev-parse", "--short", rev) }

// staged lists what the index holds that HEAD does not.
func (s *session) staged() []string {
	o := s.out("-c", "core.quotepath=false", "diff", "--cached", "--name-only")
	if o == "" {
		return nil
	}
	return strings.Split(o, "\n")
}

// unmerged lists the files a stopped merge or revert left in conflict.
func (s *session) unmerged() []string {
	o := s.out("-c", "core.quotepath=false", "diff", "--name-only", "--diff-filter=U")
	if o == "" {
		return nil
	}
	return strings.Split(o, "\n")
}

// branchName is a branch a skill may make, merge or remove: pref/ (a preference change), conf/ (a
// setting in jeeves.conf) or review/ (the rules review), then a plain name.
var branchName = regexp.MustCompile(`^(pref|conf|review)/[A-Za-z0-9][A-Za-z0-9._-]*$`)

func validBranch(b string) bool {
	return branchName.MatchString(b) && !strings.Contains(b, "..") && !strings.HasSuffix(b, ".lock")
}

// committable reports why a path may not be committed by a skill ("" when it may): only the shared files
// (home/), the settings every device reads (jeeves.conf) and the data repo's own CLAUDE.md.
func committable(p string) string {
	if p == "" || strings.HasPrefix(p, "/") || strings.HasPrefix(p, "-") || strings.Contains(p, `\`) ||
		strings.ContainsAny(p, "\x00\n\r") || path.Clean(p) != strings.TrimSuffix(p, "/") ||
		strings.HasPrefix(path.Clean(p), "../") || path.Clean(p) == ".." {
		return "not a plain path inside the data repo"
	}
	c := path.Clean(p)
	switch {
	case c == "jeeves.conf", c == "CLAUDE.md", c == ".claude/CLAUDE.md":
		return ""
	case strings.HasPrefix(c, "home/") && lib.BadName(c) == "":
		return ""
	case strings.HasPrefix(c, "home/"):
		return lib.BadName(c)
	}
	return "only files under home/, jeeves.conf and the data repo's CLAUDE.md are committed this way"
}

// linkUnder names a symbolic link at p or inside it (p a folder), or "".
func linkUnder(root, p string) string {
	found := ""
	filepath.WalkDir(filepath.Join(root, filepath.FromSlash(p)), func(q string, d os.DirEntry, err error) error {
		if err == nil && d.Type()&os.ModeSymlink != 0 {
			rel, _ := filepath.Rel(root, q)
			found = filepath.ToSlash(rel)
			return filepath.SkipAll
		}
		return nil
	})
	return found
}

// usage prints a command's usage and returns 2.
func usage(stderr io.Writer, text string) int {
	fmt.Fprintln(stderr, "usage: "+text)
	return 2
}

// flag takes "-m <value>" style options out of args: the values by name, the rest in order.
func parse(args []string, withValue map[string]bool) (map[string]string, []string, bool) {
	vals := map[string]string{}
	var rest []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			rest = append(rest, args[i:]...)
			break
		}
		if withValue[a] {
			if i+1 >= len(args) {
				return nil, nil, false
			}
			vals[a] = args[i+1]
			i++
			continue
		}
		rest = append(rest, a)
	}
	return vals, rest, true
}
